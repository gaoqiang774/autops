package service

import (
	cmdbmodel "dodevops-api/api/cmdb/model"
	configcentermodel "dodevops-api/api/configcenter/model"
	"dodevops-api/api/task/dao"
	"dodevops-api/api/task/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type ITaskWorkService interface {
	StartJob(taskID uint) error                                          // 启动任务
	StopJob(taskID, templateID uint) error                               // 停止任务
	GetJobLog(taskID, templateID uint) (string, error)                   // 获取任务日志
	GetJobStatus(taskID, templateID uint) (*model.TaskWorkStatus, error) // 获取任务状态
	ScheduleJob(job *model.TaskWork) error                               // 添加任务
}

type TaskWorkServiceImpl struct {
	dao            dao.TaskWorkDaoInterface
	taskJobService TaskJobService
}

func (s *TaskWorkServiceImpl) StartJob(taskID uint) error {
	parent, err := s.dao.GetParentTask(taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %v", err)
	}
	if parent.Type != model.TaskTypeImmediate {
		return fmt.Errorf("该接口只能直接执行普通任务")
	}
	if parent.Status == model.TaskStatusRunning {
		return fmt.Errorf("任务正在运行中")
	}

	// 获取任务信息
	jobs, err := s.dao.GetByTaskID(taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %v", err)
	}

	// 检查是否有任务正在运行
	for _, job := range jobs {
		if job.Status == 2 { // 2表示运行中
			return fmt.Errorf("任务正在运行中")
		}
	}

	// 已完成、失败或停止的作业在重跑前恢复为等待状态。
	if err := common.GetDB().Model(&model.TaskWork{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
		"status": model.TaskStatusPending, "log": "", "log_path": "", "start_time": nil,
		"end_time": nil, "duration": 0,
	}).Error; err != nil {
		return fmt.Errorf("重置任务状态失败: %v", err)
	}
	now := time.Now()
	if err := common.GetDB().Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status": model.TaskStatusRunning, "start_time": now, "end_time": nil, "duration": 0,
	}).Error; err != nil {
		return fmt.Errorf("更新任务状态失败: %v", err)
	}
	jobs, err = s.dao.GetByTaskID(taskID)
	if err != nil {
		return fmt.Errorf("重新读取任务失败: %v", err)
	}

	// 获取任务队列服务
	taskQueue := GetTaskQueue()

	// 将所有任务提交到 Redis 队列
	submittedCount := 0
	for _, job := range jobs {
		if job.Status != 1 { // 非等待状态跳过
			continue
		}

		// 更新状态为运行中
		if err := s.dao.UpdateStatus(job.ID, 2); err != nil {
			continue
		}

		// 获取任务优先级（可根据任务类型或其他条件决定）
		priority := s.getTaskPriority(&job)

		if taskQueue == nil || !taskQueue.IsRunning() {
			jobCopy := job
			go func() {
				if err := s.executeJob(&jobCopy); err != nil {
					fmt.Printf("执行任务失败: task=%d work=%d error=%v\n", taskID, jobCopy.ID, err)
				}
			}()
		} else if err := taskQueue.Enqueue(&job, priority); err != nil {
			s.dao.UpdateStatus(job.ID, model.TaskStatusPending)
			continue
		}

		submittedCount++
	}

	if submittedCount == 0 {
		common.GetDB().Model(&model.Task{}).Where("id = ?", taskID).Update("status", model.TaskStatusFailed)
		return fmt.Errorf("没有有效的任务可以执行")
	}

	return nil
}

// getTaskPriority 获取任务优先级
func (s *TaskWorkServiceImpl) getTaskPriority(job *model.TaskWork) string {
	// 获取父任务信息
	parentTask, err := s.dao.GetParentTask(job.TaskID)
	if err != nil {
		return "normal" // 默认优先级
	}

	// 根据任务类型确定优先级
	switch parentTask.Type {
	case model.TaskTypeImmediate: // 立即执行任务 - 高优先级
		return "high"
	case model.TaskTypeScheduled: // 定时任务 - 普通优先级
		return "normal"
	case model.TaskTypeAnsible: // Ansible任务 - 低优先级（可能耗时较长）
		return "low"
	default:
		return "normal"
	}
}

func (s *TaskWorkServiceImpl) executeJob(job *model.TaskWork) error {
	startTime := time.Now()
	if err := s.dao.UpdateTiming(job.ID, &startTime, nil, 0); err != nil {
		return fmt.Errorf("记录任务开始时间失败: %v", err)
	}
	if err := s.dao.UpdateStatus(job.ID, model.TaskStatusRunning); err != nil {
		return fmt.Errorf("更新任务状态失败: %v", err)
	}

	// 检查父任务状态，如果已停止或暂停则不再执行
	parentTask, err := s.dao.GetParentTask(job.TaskID)
	if err != nil {
		return fmt.Errorf("获取父任务状态失败: %v", err)
	}

	// 检查任务是否被停止
	if parentTask.Status == model.TaskStatusStopped {
		// 更新子任务状态为异常以匹配父任务状态
		if err := s.dao.UpdateStatus(job.ID, model.TaskStatusStopped); err != nil {
			return fmt.Errorf("更新子任务状态失败: %v", err)
		}
		return fmt.Errorf("父任务已被停止，子任务标记为异常")
	}

	// 检查任务是否被暂停（仅对定时任务有效）
	if parentTask.Status == model.TaskStatusPaused && parentTask.Type == model.TaskTypeScheduled {
		// 暂停状态的任务不执行，但不标记为异常
		return fmt.Errorf("定时任务已被暂停，跳过执行")
	}

	// 获取模板和主机信息
	template, host, err := s.getJobDetails(job)
	if err != nil {
		// 仅当获取关键信息失败时才标记为异常
		s.dao.UpdateStatus(job.ID, 4)
		return fmt.Errorf("获取任务详情失败: %v", err)
	}

	// 创建日志目录
	logDir := fmt.Sprintf("logs/task_%d", job.TaskID)
	os.MkdirAll(logDir, 0755)

	// 执行SSH任务（带重试和严格错误处理）
	var logContent string
	var sshErr error
	maxRetries := 3               // 增加重试次数
	retryDelay := 5 * time.Second // 增加重试间隔

	for i := 0; i <= maxRetries; i++ {
		logContent, sshErr = s.executeSSHTask(job, template, host)
		if sshErr == nil {
			break
		}

		if isSSHConnectionError(sshErr) {
			if i < maxRetries {
				time.Sleep(retryDelay)
				continue
			}
			logContent = fmt.Sprintf("SSH连接失败: %v\n%s", sshErr, logContent)
			break
		} else {
			logContent = fmt.Sprintf("命令执行出错: %v\n%s", sshErr, logContent)
			break
		}
	}

	// 写入日志文件
	logPath := filepath.Join(logDir, fmt.Sprintf("task_%d_template_%d.log", job.TaskID, job.TemplateID))
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		return fmt.Errorf("写入日志文件失败: %v", err)
	}

	// 记录任务结束时间和耗时
	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Seconds())

	if err := s.dao.UpdateTiming(job.ID, &startTime, &endTime, duration); err != nil {
		return fmt.Errorf("记录任务时间失败: %v", err)
	}

	// 更新任务状态和日志 - 确保这些更新不覆盖时间字段
	if err := s.dao.UpdateLog(job.ID, logContent); err != nil {
		return err
	}
	if err := s.dao.UpdateLogPath(job.ID, logPath); err != nil {
		return err
	}

	executionStatus := model.TaskStatusSuccess
	if sshErr != nil {
		executionStatus = model.TaskStatusFailed
	}
	if err := s.dao.UpdateStatus(job.ID, executionStatus); err != nil {
		return err
	}

	// 获取该任务的所有子任务
	jobs, err := s.dao.GetByTaskID(job.TaskID)
	if err != nil {
		return fmt.Errorf("获取子任务失败: %v", err)
	}

	// 获取父任务类型
	taskDao := dao.NewTaskDao(common.GetDB())
	parentTask, err = taskDao.GetById(job.TaskID)
	if err != nil {
		return fmt.Errorf("获取父任务信息失败: %v", err)
	}

	// 计算总耗时和确定最终状态
	var totalDuration int
	finalStatus := 3 // 默认成功
	allCompleted := true
	now := time.Now()

	for _, j := range jobs {
		if j.Duration > 0 {
			totalDuration += j.Duration
		}
		if j.Status == model.TaskStatusFailed || j.Status == model.TaskStatusStopped {
			finalStatus = 4
		}
		if j.Status == 1 || j.Status == 2 { // 如果有子任务未完成
			allCompleted = false
		}
	}

	// 只有所有子任务都完成时才更新父任务
	if allCompleted {
		// 对于定时任务，父任务状态保持为运行中(2)
		if parentTask.Type == 2 { // 2表示定时任务
			finalStatus = 2

			// 将所有子任务状态重置为等待中(1)，以便下次定时触发时可以再次执行
			for _, j := range jobs {
				if j.Status == 3 || j.Status == 4 { // 成功或失败状态
					s.dao.UpdateStatus(j.ID, 1)
				}
			}
		}

		// 更新父任务状态、耗时和结束时间（使用Select只更新必要字段）
		task := &model.Task{
			ID:       job.TaskID,
			Status:   finalStatus,
			Duration: totalDuration,
			EndTime:  &now,
		}

		// 使用Select明确指定要更新的字段，避免created_at被错误更新
		if err := common.GetDB().Model(task).
			Select("status", "duration", "end_time").
			Where("id = ?", job.TaskID).
			Updates(task).Error; err != nil {
			return fmt.Errorf("更新父任务失败: %v", err)
		}

		// 更新execute_count和next_run_time（对所有类型的任务）
		if err := s.taskJobService.UpdateTaskAfterExecution(job.TaskID, parentTask.CronExpr); err != nil {
			return fmt.Errorf("更新任务执行统计失败: %v", err)
		}
	}

	return sshErr
}

func (s *TaskWorkServiceImpl) StopJob(taskID, templateID uint) error {
	// 1. 获取任务详情并检查状态
	job, err := s.dao.GetByTaskAndTemplateID(taskID, templateID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %v", err)
	}

	// 2. 如果任务不是运行中状态(2)，直接返回错误
	if job.Status != 2 {
		return fmt.Errorf("任务当前状态为%d，无法停止非运行中的任务", job.Status)
	}

	// 3. 执行远程停止操作
	s.killRemoteProcess(job)

	// 4. 更新状态为已停止并记录结束时间
	now := time.Now()
	if err := s.dao.UpdateStatus(job.ID, model.TaskStatusStopped); err != nil {
		return fmt.Errorf("更新任务状态失败: %v", err)
	}

	// 5. 更新结束时间和耗时
	s.dao.UpdateTiming(job.ID, nil, &now, 0)

	// 6. 停止所有关联的子任务
	jobs, err := s.dao.GetByTaskID(taskID)
	if err == nil {
		// 首先停止当前任务
		s.dao.UpdateStatus(job.ID, model.TaskStatusStopped)

		// 停止所有子任务
		for _, j := range jobs {
			if j.ID == job.ID {
				continue // 跳过当前任务，已经处理过
			}

			// 只停止运行中或等待中的任务
			if j.Status == 1 || j.Status == 2 {
				s.dao.UpdateStatus(j.ID, model.TaskStatusStopped)
			}
		}

		// 更新父任务状态为已停止
		taskDao := dao.NewTaskDao(common.GetDB())
		taskDao.UpdateStatus(taskID, model.TaskStatusStopped)
	}

	// 7. 记录操作日志
	logContent := fmt.Sprintf("任务被手动停止于 %s", now.Format(time.RFC3339))
	s.dao.UpdateLog(job.ID, logContent)

	return nil
}

func (s *TaskWorkServiceImpl) GetJobLog(taskID, templateID uint) (string, error) {
	job, err := s.dao.GetByTaskAndTemplateID(taskID, templateID)
	if err != nil {
		return "", fmt.Errorf("获取任务失败: %v", err)
	}

	if job.LogPath == "" {
		return "", fmt.Errorf("任务日志路径为空")
	}

	content, err := os.ReadFile(job.LogPath)
	if err != nil {
		return "", fmt.Errorf("读取日志文件失败: %v", err)
	}

	return string(content), nil
}

func (s *TaskWorkServiceImpl) GetJobStatus(taskID, templateID uint) (*model.TaskWorkStatus, error) {
	job, err := s.dao.GetByTaskAndTemplateID(taskID, templateID)
	if err != nil {
		return nil, fmt.Errorf("获取任务失败: %v", err)
	}

	status := &model.TaskWorkStatus{
		Status:   job.Status,
		Log:      job.Log,
		Duration: job.Duration,
	}
	if job.StartTime != nil {
		status.StartTime = job.StartTime.Format(time.RFC3339)
	}
	if job.EndTime != nil {
		status.EndTime = job.EndTime.Format(time.RFC3339)
	}
	return status, nil
}

func (s *TaskWorkServiceImpl) ScheduleJob(job *model.TaskWork) error {
	// 获取关联的TaskJob记录
	taskJob, err := s.dao.GetTaskJobByID(job.TaskID)
	if err != nil {
		return fmt.Errorf("获取任务信息失败: %v", err)
	}

	// 立即执行任务 (普通任务)
	if taskJob.Type == 1 {
		// 更新状态为运行中
		if err := s.dao.UpdateStatus(job.ID, 2); err != nil {
			return fmt.Errorf("更新任务状态失败: %v", err)
		}

		// 获取任务队列服务
		taskQueue := GetTaskQueue()
		if taskQueue == nil {
			// 如果队列服务不可用，回退到原始方式
			go func() {
				s.executeJob(job)
			}()
			return nil
		}

		// 提交到 Redis 队列
		priority := s.getTaskPriority(job)
		if err := taskQueue.Enqueue(job, priority); err != nil {
			return fmt.Errorf("提交任务到队列失败: %v", err)
		}

		return nil
	}

	// 定时任务处理
	if taskJob.Type == 2 {
		// 从task_job获取cron表达式并计算下次执行时间
		if taskJob.CronExpr == "" {
			return fmt.Errorf("定时任务必须包含cron表达式")
		}

		// 使用cron库解析cron表达式（标准5位格式：分 时 日 月 周）
		schedule, err := cron.ParseStandard(taskJob.CronExpr)
		if err != nil {
			return fmt.Errorf("解析cron表达式失败: %v", err)
		}

		// 计算下次执行时间
		nextTime := schedule.Next(time.Now())
		if nextTime.IsZero() {
			return fmt.Errorf("无法计算有效的下次执行时间")
		}

		job.ScheduledTime = &nextTime

		// 验证执行时间格式
		if job.ScheduledTime.Before(time.Now()) {
			return fmt.Errorf("定时任务执行时间(%v)不能早于当前时间(%v)",
				job.ScheduledTime.Format("2006-01-02 15:04:05"),
				time.Now().Format("2006-01-02 15:04:05"))
		}

		// 计算延迟时间
		delay := time.Until(*job.ScheduledTime)
		if delay <= 0 {
			return fmt.Errorf("定时任务执行时间已过期，请设置未来的时间")
		}

		// 先注册到全局 cron 调度器（在数据库操作之前，避免死锁）
		scheduler := GetGlobalScheduler()
		if scheduler == nil {
			return fmt.Errorf("全局调度器未初始化")
		}

		// 检查任务是否已在调度器中（防止重复注册）
		entries := scheduler.GetEntries()
		if _, exists := entries[job.TaskID]; exists {
			// 任务已注册，只更新数据库状态即可
			taskDao := dao.NewTaskDao(common.GetDB())
			if err := taskDao.UpdateStatus(job.TaskID, 2); err != nil {
				return fmt.Errorf("更新父任务状态失败: %v", err)
			}
			return nil
		}

		// 注册定时任务（使用统一的回调函数）
		if err := scheduler.AddScheduledTask(job.TaskID, taskJob.CronExpr, createScheduledTaskHandler(job.TaskID)); err != nil {
			return fmt.Errorf("注册定时任务失败: %v", err)
		}

		// 注册成功后，再更新数据库状态（避免在持有锁的情况下操作数据库）
		taskDao := dao.NewTaskDao(common.GetDB())
		if err := taskDao.UpdateStatus(job.TaskID, 2); err != nil {
			// 如果状态更新失败，移除已注册的任务
			scheduler.RemoveScheduledTask(job.TaskID)
			return fmt.Errorf("更新父任务状态失败: %v", err)
		}

		// 更新子任务的计划执行时间
		if err := s.dao.UpdateScheduledJob(job.ID, 1, job.ScheduledTime); err != nil {
			return fmt.Errorf("更新子任务状态失败: %v", err)
		}

		return nil
	}

	return fmt.Errorf("不支持的任务类型: %d (有效类型: 1=普通任务,2=定时任务,3=ansible任务)", taskJob.Type)
}

// Helper methods
func (s *TaskWorkServiceImpl) getJobDetails(job *model.TaskWork) (*model.TaskTemplate, *cmdbmodel.CmdbHost, error) {
	// 获取模板信息
	var template model.TaskTemplate
	if err := common.GetDB().First(&template, job.TemplateID).Error; err != nil {
		return nil, nil, fmt.Errorf("获取模板失败: %v", err)
	}

	// 获取主机信息
	var host cmdbmodel.CmdbHost
	if err := common.GetDB().First(&host, job.HostID).Error; err != nil {
		return nil, nil, fmt.Errorf("获取主机失败: %v", err)
	}

	return &template, &host, nil
}

func isSSHConnectionError(err error) bool {
	// 判断是否为SSH连接错误(包括超时)
	return err != nil && (strings.Contains(err.Error(), "connection") ||
		strings.Contains(err.Error(), "authentication") ||
		strings.Contains(err.Error(), "timeout"))
}

func (s *TaskWorkServiceImpl) executeSSHTask(job *model.TaskWork, template *model.TaskTemplate, host *cmdbmodel.CmdbHost) (string, error) {
	// 1. 获取认证凭证
	var ecsAuth configcentermodel.EcsAuth
	if err := common.GetDB().First(&ecsAuth, host.SSHKeyID).Error; err != nil {
		return "", fmt.Errorf("获取认证凭证失败: %v", err)
	}
	if err := ecsAuth.DecryptSecrets(); err != nil {
		return "", fmt.Errorf("解密认证凭证失败: %v", err)
	}

	// 2. 初始化SSH配置
	sshUtil := util.NewSSHUtil()
	sshConfig := &util.SSHConfig{
		IP:       host.SSHIP,   // SSH连接IP
		Port:     host.SSHPort, // SSH端口
		Username: host.SSHName, // SSH用户名
		Type:     ecsAuth.Type, // 认证类型
	}

	// 3. 根据认证类型配置
	switch ecsAuth.Type {
	case 1: // 密码认证
		sshConfig.Password = ecsAuth.Password
	case 2: // 密钥认证
		sshConfig.PublicKey = ecsAuth.PublicKey
	case 3: // 公钥免认证 - 自动查找本地私钥
		// 不需要设置额外信息，SSHUtil会自动查找本地私钥
	case 4:
		sshConfig.BastionHost = ecsAuth.BastionHost
		sshConfig.BastionPort = ecsAuth.BastionPort
		sshConfig.BastionUsername = ecsAuth.BastionUsername
		sshConfig.BastionAuthType = ecsAuth.BastionAuthType
		sshConfig.BastionPassword = ecsAuth.BastionPassword
		sshConfig.BastionPrivateKey = ecsAuth.BastionPrivateKey
		sshConfig.TargetAuthType = ecsAuth.TargetAuthType
		sshConfig.TargetPassword = ecsAuth.TargetPassword
		sshConfig.TargetPrivateKey = ecsAuth.TargetPrivateKey
	default:
		return "", fmt.Errorf("不支持的认证类型: %d", ecsAuth.Type)
	}

	// 4. 构造远程临时文件。内容使用Base64传输，避免脚本中的引号或EOF片段
	// 破坏外层Shell命令。
	pidFile := fmt.Sprintf("/tmp/task_%d_%d.pid", job.TaskID, job.TemplateID)
	logFile := fmt.Sprintf("/tmp/task_%d_%d.log", job.TaskID, job.TemplateID)
	exitFile := fmt.Sprintf("/tmp/task_%d_%d.exit", job.TaskID, job.TemplateID)

	// 清理Windows换行符（\r\n -> \n），避免在Linux上执行脚本时出现语法错误
	scriptContent := strings.ReplaceAll(template.Content, "\r\n", "\n")
	scriptContent = strings.ReplaceAll(scriptContent, "\r", "\n")

	var scriptFile, runner string
	switch template.Type {
	case model.TemplateTypeShell:
		scriptFile = fmt.Sprintf("/tmp/task_%d_%d.sh", job.TaskID, job.TemplateID)
		scriptContent = "#!/bin/bash\n" + scriptContent
		runner = "bash " + scriptFile
	case model.TemplateTypePython:
		scriptFile = fmt.Sprintf("/tmp/task_%d_%d.py", job.TaskID, job.TemplateID)
		runner = "python3 " + scriptFile
	case model.TemplateTypeAnsible:
		scriptFile = fmt.Sprintf("/tmp/task_%d_%d.yml", job.TaskID, job.TemplateID)
		runner = "ansible-playbook " + scriptFile
	default:
		return "", fmt.Errorf("不支持的模板类型: %d", template.Type)
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(scriptContent))
	cmd := fmt.Sprintf(
		"printf '%%s' '%s' | base64 -d > %s && chmod 700 %s && rm -f %s && "+
			"nohup sh -c '%s; code=$?; printf \"%%s\" \"$code\" > %s; exit \"$code\"' > %s 2>&1 < /dev/null & echo $! > %s",
		encoded, scriptFile, scriptFile, exitFile, runner, exitFile, logFile, pidFile,
	)

	// 6. 执行远程命令
	_, err := sshUtil.ExecuteRemoteCommand(sshConfig, cmd)
	if err != nil {
		return "", fmt.Errorf("SSH执行失败: %v", err)
	}

	// 8. 验证PID文件是否创建 (带重试)
	maxRetries := 3
	retryDelay := 1 * time.Second
	var pid string

	for i := 0; i < maxRetries; i++ {
		checkCmd := fmt.Sprintf(`
			if [ -f %s ]; then
				cat %s
				exit 0
			else
				exit 1
			fi
		`, pidFile, pidFile)

		checkOutput, err := sshUtil.ExecuteRemoteCommand(sshConfig, checkCmd)
		if err == nil && checkOutput != "" {
			pid = strings.TrimSpace(checkOutput)
			break
		}
		time.Sleep(retryDelay)
	}

	if pid == "" {
		return "", fmt.Errorf("无法获取有效的进程PID")
	}

	// 9. 最长等待一小时，超时后终止远程进程并写入124退出码。
	waitCmd := fmt.Sprintf(`
		count=0
		while kill -0 %s > /dev/null 2>&1; do
			count=$((count + 1))
			if [ "$count" -ge 3600 ]; then
				kill -TERM %s > /dev/null 2>&1 || true
				printf "124" > %s
				break
			fi
			sleep 1
		done
	`, pid, pid, exitFile)

	if _, err := sshUtil.ExecuteRemoteCommand(sshConfig, waitCmd); err != nil {
		return "", fmt.Errorf("等待任务完成失败: %v", err)
	}

	// 获取完整日志，并让SSH命令携带脚本退出码。
	finalLogCmd := fmt.Sprintf("cat %s; code=$(cat %s 2>/dev/null || printf '1'); rm -f %s %s %s; exit \"$code\"",
		logFile, exitFile, scriptFile, pidFile, exitFile)
	finalOutput, err := sshUtil.ExecuteRemoteCommand(sshConfig, finalLogCmd)
	if err != nil {
		return finalOutput, fmt.Errorf("脚本执行失败: %v", err)
	}

	return finalOutput, nil
}

// 杀死远程进程
func (s *TaskWorkServiceImpl) killRemoteProcess(job *model.TaskWork) error {
	// 1. 获取主机信息和SSH认证
	_, host, err := s.getJobDetails(job)
	if err != nil {
		return fmt.Errorf("获取主机信息失败: %v", err)
	}

	// 2. 获取SSH认证凭证
	var ecsAuth configcentermodel.EcsAuth
	if err := common.GetDB().First(&ecsAuth, host.SSHKeyID).Error; err != nil {
		return fmt.Errorf("获取SSH认证凭证失败: %v", err)
	}
	if err := ecsAuth.DecryptSecrets(); err != nil {
		return fmt.Errorf("解密SSH认证凭证失败: %v", err)
	}

	// 3. 验证SSH认证信息
	if ecsAuth.Type < 1 || ecsAuth.Type > 4 {
		return fmt.Errorf("不支持的SSH认证类型: %d", ecsAuth.Type)
	}
	if ecsAuth.Type == 1 && ecsAuth.Password == "" {
		return fmt.Errorf("密码认证类型但密码为空")
	}
	if ecsAuth.Type == 2 && ecsAuth.PublicKey == "" {
		return fmt.Errorf("密钥认证类型但公钥为空")
	}

	// 4. 初始化SSH配置 (与executeSSHTask保持一致)
	sshUtil := util.NewSSHUtil()
	sshConfig := &util.SSHConfig{
		IP:       host.SSHIP,   // SSH连接IP
		Port:     host.SSHPort, // SSH端口
		Username: host.SSHName, // SSH用户名
		Type:     ecsAuth.Type, // 认证类型
	}

	// 根据认证类型配置
	switch ecsAuth.Type {
	case 1: // 密码认证
		sshConfig.Password = ecsAuth.Password
	case 2: // 密钥认证
		sshConfig.PublicKey = ecsAuth.PublicKey
	case 3: // 公钥免认证 - 自动查找本地私钥
		// 不需要设置额外信息，SSHUtil会自动查找本地私钥
	case 4:
		sshConfig.BastionHost = ecsAuth.BastionHost
		sshConfig.BastionPort = ecsAuth.BastionPort
		sshConfig.BastionUsername = ecsAuth.BastionUsername
		sshConfig.BastionAuthType = ecsAuth.BastionAuthType
		sshConfig.BastionPassword = ecsAuth.BastionPassword
		sshConfig.BastionPrivateKey = ecsAuth.BastionPrivateKey
		sshConfig.TargetAuthType = ecsAuth.TargetAuthType
		sshConfig.TargetPassword = ecsAuth.TargetPassword
		sshConfig.TargetPrivateKey = ecsAuth.TargetPrivateKey
	default:
		return fmt.Errorf("不支持的认证类型: %d", ecsAuth.Type)
	}

	// 5. 构造PID文件路径和终止命令
	pidFile := fmt.Sprintf("/tmp/task_%d_%d.pid", job.TaskID, job.TemplateID)

	// 更健壮的终止命令:
	cmd := fmt.Sprintf(`
		# 检查PID文件是否存在
		if [ ! -f %s ]; then
			echo "PID文件 %s 不存在";
			exit 0;
		fi
		
		# 读取PID
		PID=$(cat %s);
		
		# 检查进程是否存在
		if ! ps -p $PID > /dev/null 2>&1; then
			echo "进程 $PID 不存在";
			rm -f %s;
			exit 0;
		fi
		
		# 终止进程
		echo "正在终止进程 $PID";
		if kill -9 $PID; then
			# 等待进程终止
			for i in {1..5}; do
				if ! ps -p $PID > /dev/null 2>&1; then
					echo "成功终止进程 $PID";
					rm -f %s;
					exit 0;
				fi
				sleep 1;
			done
			
			# 如果进程仍然存在
			if ps -p $PID > /dev/null 2>&1; then
				echo "警告: 进程 $PID 仍然运行";
				exit 1;
			fi
		else
			echo "终止进程 $PID 失败";
			exit 1;
		fi`,
		pidFile, pidFile, pidFile, pidFile, pidFile)

	// 6. 执行终止命令
	output, err := sshUtil.ExecuteRemoteCommand(sshConfig, cmd)
	if err != nil {

		// 检查是否是连接/认证错误
		if strings.Contains(err.Error(), "authentication") ||
			strings.Contains(err.Error(), "connection") {
			return fmt.Errorf("SSH连接/认证失败: %v", err)
		}

		// 检查输出中是否有成功终止的迹象
		if strings.Contains(output, "成功终止进程") ||
			strings.Contains(output, "进程不存在") ||
			strings.Contains(output, "PID文件不存在") {
			return nil
		}
		return fmt.Errorf("终止进程失败: %v\n输出:\n%s", err, output)
	}

	return nil
}

// 辅助函数：获取任务类型名称
func getJobType(jobType int) string {
	switch jobType {
	case 1:
		return "立即执行"
	case 2:
		return "定时任务"
	default:
		return "未知类型"
	}
}

// 创建任务服务实例
func NewTaskWorkService() ITaskWorkService {
	return &TaskWorkServiceImpl{
		dao:            dao.TaskWorkDao(),
		taskJobService: NewTaskService(common.GetDB()),
	}
}
