package service

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common/config"
)

var vectorIdentifierRegexp = regexp.MustCompile(`[^A-Za-z0-9_]+`)
var vectorFieldRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var vectorSemverRegexp = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)

var (
	vectorPackageUploadTasks   sync.Map
	vectorPackageUploadTaskSeq uint64
	vectorPackageUploadTaskMu  sync.RWMutex
)

const (
	vectorAggregatorName                = "aiops-vector-aggregator"
	vectorAggregatorConfigPath          = "/opt/aiops/vector-aggregator/vector.yaml"
	vectorAggregatorContainerConfigPath = "/etc/vector/vector.yaml"
	vectorAggregatorDataDir             = "/var/lib/aiops-vector-aggregator"
	vectorAggregatorConsumerGroup       = "aiops-victorialogs-writer"
	vectorAggregatorKubernetesFileName  = "aiops-vector-aggregator.yaml"
	// File sources need stable filesystem identity and must read rotated files
	// from their beginning to avoid checkpoint collisions and data loss.
	vectorFileFingerprintStrategy = "device_and_inode"
	vectorFileReadFrom            = "beginning"
)

func (s *LogtailService) GetVectorPackageInfo() logtailmodel.VectorPackageInfo {
	packagePath := configuredVectorPackagePath()

	info := logtailmodel.VectorPackageInfo{
		PackagePath: packagePath,
		PackageURL:  buildUploadPackageURL(packagePath),
		FileName:    filepath.Base(packagePath),
	}

	fileInfo, err := os.Stat(packagePath)
	if err != nil {
		if !os.IsNotExist(err) {
			info.Error = err.Error()
		}
		return info
	}
	if fileInfo.IsDir() {
		info.Error = "Vector 包路径指向目录，不是文件"
		return info
	}
	info.Exists = true
	info.SizeBytes = fileInfo.Size()
	info.ModTime = fileInfo.ModTime().Format(time.RFC3339)

	if sha, err := fileSHA256(packagePath); err == nil {
		info.SHA256 = sha
	} else {
		info.Error = err.Error()
	}
	info.Version = detectVectorVersion(packagePath)
	return info
}

func (s *LogtailService) UploadVectorPackage(fileHeader *multipart.FileHeader, replace bool) (logtailmodel.VectorPackageInfo, error) {
	if fileHeader == nil {
		return logtailmodel.VectorPackageInfo{}, errors.New("上传文件不能为空")
	}
	if fileHeader.Size <= 0 {
		return logtailmodel.VectorPackageInfo{}, errors.New("上传文件不能为空文件")
	}

	targetPath, err := resolveVectorPackageTargetPath()
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	if stat, statErr := os.Stat(targetPath); statErr == nil {
		if stat.IsDir() {
			return logtailmodel.VectorPackageInfo{}, errors.New("当前 Vector 包路径是目录，无法直接上传文件")
		}
		if !replace {
			return logtailmodel.VectorPackageInfo{}, errors.New("当前 Vector 包已存在，请先删除后再上传")
		}
	} else if !os.IsNotExist(statErr) {
		return logtailmodel.VectorPackageInfo{}, statErr
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	src, err := fileHeader.Open()
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}
	defer src.Close()

	tempPath := filepath.Join(filepath.Dir(targetPath), fmt.Sprintf(".vector-upload-%d.tmp", time.Now().UnixNano()))
	dst, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	copyErr := func() error {
		defer dst.Close()
		_, err := io.Copy(dst, src)
		return err
	}()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return logtailmodel.VectorPackageInfo{}, copyErr
	}

	_ = os.Chmod(tempPath, 0o755)
	if replace {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tempPath)
			return logtailmodel.VectorPackageInfo{}, err
		}
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return logtailmodel.VectorPackageInfo{}, err
	}
	_ = os.Chmod(targetPath, 0o755)

	info := s.GetVectorPackageInfo()
	if !info.Exists {
		return info, errors.New("Vector 包上传后校验失败")
	}
	return info, nil
}

func (s *LogtailService) CreateVectorPackageUploadTask(tempFilePath string, originalFileName string, replace bool) (*logtailmodel.VectorPackageUploadTask, error) {
	tempFilePath = strings.TrimSpace(tempFilePath)
	if tempFilePath == "" {
		return nil, errors.New("上传临时文件不能为空")
	}
	fileInfo, err := os.Stat(tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("获取临时文件信息失败: %w", err)
	}
	if fileInfo.IsDir() || fileInfo.Size() <= 0 {
		return nil, errors.New("上传文件不能为空文件")
	}
	if err := validateVectorPackageTargetForUpload(replace); err != nil {
		return nil, err
	}

	now := time.Now()
	task := &logtailmodel.VectorPackageUploadTask{
		TaskID:        nextVectorPackageUploadTaskID(),
		FileName:      firstNonEmpty(originalFileName, filepath.Base(tempFilePath)),
		FileSize:      fileInfo.Size(),
		Replace:       replace,
		Status:        logtailmodel.VectorPackageUploadTaskPending,
		Progress:      0,
		UploadedBytes: 0,
		TotalBytes:    fileInfo.Size(),
		Message:       "任务已创建，等待保存 Vector 包",
		CreatedAt:     now,
	}
	vectorPackageUploadTasks.Store(task.TaskID, task)

	go s.runVectorPackageUploadTask(task, tempFilePath)
	return cloneVectorPackageUploadTask(task), nil
}

func (s *LogtailService) GetVectorPackageUploadTask(taskID string) (*logtailmodel.VectorPackageUploadTask, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.New("任务ID不能为空")
	}
	taskValue, ok := vectorPackageUploadTasks.Load(taskID)
	if !ok {
		return nil, fmt.Errorf("上传任务不存在: %s", taskID)
	}
	task, ok := taskValue.(*logtailmodel.VectorPackageUploadTask)
	if !ok {
		return nil, fmt.Errorf("上传任务数据异常: %s", taskID)
	}
	return cloneVectorPackageUploadTask(task), nil
}

func (s *LogtailService) runVectorPackageUploadTask(task *logtailmodel.VectorPackageUploadTask, tempFilePath string) {
	startedAt := time.Now()
	updateVectorPackageUploadTask(task, func(t *logtailmodel.VectorPackageUploadTask) {
		t.Status = logtailmodel.VectorPackageUploadTaskUploading
		t.StartedAt = &startedAt
		t.Progress = 5
		t.Message = "正在保存 Vector 包"
	})

	info, err := s.uploadVectorPackageFromPath(tempFilePath, task.Replace, func(uploaded, total int64) {
		progress := 5.0
		if total > 0 {
			progress = 5 + float64(uploaded)/float64(total)*85
		}
		updateVectorPackageUploadTask(task, func(t *logtailmodel.VectorPackageUploadTask) {
			t.UploadedBytes = uploaded
			t.TotalBytes = total
			t.Progress = progress
			t.Message = fmt.Sprintf("正在保存 Vector 包: %.2f%%", t.Progress)
		})
	})

	completedAt := time.Now()
	updateVectorPackageUploadTask(task, func(t *logtailmodel.VectorPackageUploadTask) {
		t.CompletedAt = &completedAt
		if err != nil {
			t.Status = logtailmodel.VectorPackageUploadTaskFailed
			t.Error = err.Error()
			t.Message = "上传失败"
			return
		}
		t.Status = logtailmodel.VectorPackageUploadTaskCompleted
		t.Progress = 100
		t.UploadedBytes = t.TotalBytes
		t.Message = "上传完成"
		t.Package = &info
	})

	if removeErr := os.Remove(tempFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
		updateVectorPackageUploadTask(task, func(t *logtailmodel.VectorPackageUploadTask) {
			if t.Error == "" {
				t.Error = "清理临时文件失败: " + removeErr.Error()
			}
		})
	}
}

func (s *LogtailService) uploadVectorPackageFromPath(srcPath string, replace bool, onProgress func(uploaded, total int64)) (logtailmodel.VectorPackageInfo, error) {
	if err := validateVectorPackageTargetForUpload(replace); err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}
	targetPath, err := resolveVectorPackageTargetPath()
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	tempPath := filepath.Join(filepath.Dir(targetPath), fmt.Sprintf(".vector-upload-%d.tmp", time.Now().UnixNano()))
	dst, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	copyErr := func() error {
		defer dst.Close()
		_, err := copyWithProgress(dst, src, srcInfo.Size(), onProgress)
		if syncErr := dst.Sync(); err == nil && syncErr != nil {
			err = syncErr
		}
		return err
	}()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return logtailmodel.VectorPackageInfo{}, copyErr
	}

	_ = os.Chmod(tempPath, 0o755)
	if replace {
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tempPath)
			return logtailmodel.VectorPackageInfo{}, err
		}
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return logtailmodel.VectorPackageInfo{}, err
	}
	_ = os.Chmod(targetPath, 0o755)

	info := s.GetVectorPackageInfo()
	if !info.Exists {
		return info, errors.New("Vector 包上传后校验失败")
	}
	return info, nil
}

func (s *LogtailService) DeleteVectorPackage() (logtailmodel.VectorPackageInfo, error) {
	targetPath, err := resolveVectorPackageTargetPath()
	if err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}

	stat, statErr := os.Stat(targetPath)
	switch {
	case os.IsNotExist(statErr):
		return s.GetVectorPackageInfo(), nil
	case statErr != nil:
		return logtailmodel.VectorPackageInfo{}, statErr
	case stat.IsDir():
		return logtailmodel.VectorPackageInfo{}, errors.New("当前 Vector 包路径是目录，无法删除")
	}

	if err := os.Remove(targetPath); err != nil {
		return logtailmodel.VectorPackageInfo{}, err
	}
	return s.GetVectorPackageInfo(), nil
}

func (s *LogtailService) ListLogAgents(query logtailmodel.LogAgentQuery) ([]logtailmodel.LogAgentVO, int64, logtailmodel.LogAgentQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.agentDAO.List(query, agentHeartbeatThreshold())
	if err != nil {
		return nil, 0, query, err
	}
	for i := range items {
		if err := s.clearHealthyLogAgentLastError(&items[i]); err != nil {
			return nil, 0, query, err
		}
		applyLogAgentPresentation(&items[i])
		count, countErr := s.agentDAO.CountCollectTasksForAgent(items[i].AgentID, items[i].HostID)
		if countErr != nil {
			return nil, 0, query, countErr
		}
		items[i].CollectTaskCount = count
	}
	return items, total, query, nil
}

func (s *LogtailService) GetLogAgent(agentID uint) (*logtailmodel.LogAgentVO, error) {
	if agentID == 0 {
		return nil, errors.New("Agent ID不能为空")
	}
	item, err := s.agentDAO.GetByAgentID(agentID, agentHeartbeatThreshold())
	if err != nil {
		return nil, err
	}
	if err := s.clearHealthyLogAgentLastError(item); err != nil {
		return nil, err
	}
	applyLogAgentPresentation(item)
	count, err := s.agentDAO.CountCollectTasksForAgent(item.AgentID, item.HostID)
	if err != nil {
		return nil, err
	}
	item.CollectTaskCount = count
	return item, nil
}

func (s *LogtailService) ResolveLogAgent(agentID, hostID uint) (*logtailmodel.LogAgentVO, error) {
	if agentID > 0 {
		return s.GetLogAgent(agentID)
	}
	if hostID == 0 {
		return nil, errors.New("Agent ID或Host ID不能为空")
	}
	item, err := s.agentDAO.GetByHostID(hostID, agentHeartbeatThreshold())
	if err != nil {
		return nil, err
	}
	if err := s.clearHealthyLogAgentLastError(item); err != nil {
		return nil, err
	}
	applyLogAgentPresentation(item)
	count, err := s.agentDAO.CountCollectTasksForAgent(item.AgentID, item.HostID)
	if err != nil {
		return nil, err
	}
	item.CollectTaskCount = count
	return item, nil
}

func (s *LogtailService) ListAgentCollectTasks(agentID uint, query logtailmodel.LogCollectTaskQuery) ([]logtailmodel.LogCollectTask, int64, logtailmodel.LogCollectTaskQuery, error) {
	agent, err := s.GetLogAgent(agentID)
	if err != nil {
		return nil, 0, query, err
	}
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.agentDAO.ListCollectTasksForAgent(agent.AgentID, agent.HostID, query)
	for i := range items {
		hydrateCollectTaskPresentation(&items[i])
	}
	return items, total, query, err
}

func (s *LogtailService) PreviewAgentConfig(agentID uint) (*logtailmodel.LogAgentConfigPreview, error) {
	agent, err := s.GetLogAgent(agentID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.agentDAO.ListEnabledCollectTasksForAgent(agent.AgentID, agent.HostID)
	if err != nil {
		return nil, err
	}
	packageInfo := s.GetVectorPackageInfo()
	content, version, err := s.renderVectorConfig(agent, tasks, packageInfo.Version)
	if err != nil {
		return nil, err
	}
	hostAliases := configuredKafkaHostAliasModels()
	hash := ""
	if strings.TrimSpace(content) != "" {
		hash = vectorConfigHash(content, hostAliases)
	}
	return &logtailmodel.LogAgentConfigPreview{
		AgentID:       agent.AgentID,
		HostID:        agent.HostID,
		HostName:      agent.HostName,
		ConfigVersion: version,
		ConfigHash:    hash,
		ConfigPath:    "/opt/aiops/vector/current/vector.yaml",
		Content:       content,
		Tasks:         tasks,
		Package:       packageInfo,
		HostAliases:   hostAliases,
	}, nil
}

func (s *LogtailService) PreviewVectorAggregatorConfig() (*logtailmodel.LogVectorAggregatorConfigPreview, error) {
	kafkaOpts, err := effectiveVectorKafkaAggregatorOptions()
	if err != nil {
		return nil, err
	}
	installOpts := effectiveVectorAggregatorInstallOptions()
	legacyHTTPSink := shouldUseLegacyHTTPSinkConfig(installOpts.Image)
	content := renderVectorAggregatorConfig(kafkaOpts, legacyHTTPSink)
	hostAliases := configuredKafkaHostAliasModels()
	hash := ""
	if strings.TrimSpace(content) != "" {
		hash = vectorConfigHash(content, hostAliases)
	}
	dockerPreview := buildVectorAggregatorDockerPreview(content, installOpts)
	kubernetesPreview := buildVectorAggregatorKubernetesPreview(content, hash, installOpts)
	deliveryMode := "auto"
	kafkaEnabled := false
	if config.Config != nil {
		deliveryMode = normalizeDeliveryMode(config.Config.Logtail.DeliveryMode)
		kafkaEnabled = config.Config.Logtail.Kafka.Enabled
	}
	return &logtailmodel.LogVectorAggregatorConfigPreview{
		DeliveryMode:  deliveryMode,
		KafkaEnabled:  kafkaEnabled,
		Brokers:       kafkaOpts.Brokers,
		Topic:         firstNonEmpty(kafkaOpts.Topic, "aiops-logs"),
		ConsumerGroup: vectorAggregatorConsumerGroup,
		Image:         installOpts.Image,
		ConfigHash:    hash,
		ConfigPath:    vectorAggregatorConfigPath,
		Content:       content,
		Docker:        dockerPreview,
		Kubernetes:    kubernetesPreview,
		Package:       s.GetVectorPackageInfo(),
		HostAliases:   hostAliases,
	}, nil
}

func (s *LogtailService) CreateAgentOperation(agentID uint, req logtailmodel.LogAgentOperationRequest) (*logtailmodel.LogAgentOperation, error) {
	agent, err := s.GetLogAgent(agentID)
	if err != nil {
		return nil, err
	}
	return s.createAgentOperationForAgent(agent, req)
}

func (s *LogtailService) createAgentOperationForAgent(agent *logtailmodel.LogAgentVO, req logtailmodel.LogAgentOperationRequest) (*logtailmodel.LogAgentOperation, error) {
	req.OpType = strings.TrimSpace(req.OpType)
	if !isAllowedAgentOperation(req.OpType) {
		return nil, errors.New("不支持的Agent操作类型")
	}
	active, err := s.operationDAO.GetActive(agent.AgentID, req.OpType)
	if err != nil {
		return nil, err
	}
	if active != nil {
		if !shouldReplaceActiveAgentOperation(req, active) {
			return active, nil
		}
		if err := s.operationDAO.CancelPending(active.ID, agent.AgentID, "操作已被新的请求替换"); err != nil {
			return nil, err
		}
	}
	if req.Operator == "" {
		req.Operator = "system"
	}
	if req.RequestJSON == "" {
		payload, _ := json.Marshal(req)
		req.RequestJSON = string(payload)
	}
	item := &logtailmodel.LogAgentOperation{
		AgentID:       agent.AgentID,
		HostID:        agent.HostID,
		OpType:        req.OpType,
		OpStatus:      logtailmodel.LogAgentOperationStatusPending,
		TargetVersion: req.TargetVersion,
		ConfigVersion: req.ConfigVersion,
		RequestJSON:   req.RequestJSON,
		Operator:      req.Operator,
		ProgressText:  "等待Agent拉取执行",
	}
	if err := s.operationDAO.Create(item); err != nil {
		return nil, err
	}
	if operationCreationMarksConfigPending(req.OpType) {
		if err := s.agentDAO.SetConfigPending(agent.AgentID, agent.HostID, req.ConfigVersion); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func shouldReplaceActiveAgentOperation(req logtailmodel.LogAgentOperationRequest, active *logtailmodel.LogAgentOperation) bool {
	if active == nil || active.OpStatus != logtailmodel.LogAgentOperationStatusPending {
		return false
	}
	if req.Force {
		return true
	}
	switch strings.TrimSpace(req.OpType) {
	case logtailmodel.LogAgentOperationInstallVector, logtailmodel.LogAgentOperationUpgradeVector:
		return true
	default:
		return false
	}
}

func (s *LogtailService) ListCollectTaskAgents(taskID uint, query logtailmodel.LogAgentQuery) (*logtailmodel.LogCollectTaskAgentsResponse, error) {
	if taskID == 0 {
		return nil, errors.New("采集任务ID不能为空")
	}
	task, err := s.taskDAO.GetByID(taskID)
	if err != nil {
		return nil, err
	}
	hydrateCollectTaskPresentation(task)
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.listAgentsForCollectTask(*task, query)
	if err != nil {
		return nil, err
	}
	if err := s.fillLogAgentCollectTaskCounts(items); err != nil {
		return nil, err
	}
	applyCollectTaskAgentPresentation(task, items)

	summaryItems, _, err := s.listAgentsForCollectTask(*task, logtailmodel.LogAgentQuery{Page: 1})
	if err != nil {
		return nil, err
	}
	applyCollectTaskAgentPresentation(task, summaryItems)
	return &logtailmodel.LogCollectTaskAgentsResponse{
		Task:     *task,
		Summary:  buildCollectTaskAgentSummary(summaryItems),
		Total:    total,
		List:     items,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (s *LogtailService) ApplyCollectTask(taskID uint, req logtailmodel.LogCollectTaskApplyRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	if taskID == 0 {
		return nil, errors.New("采集任务ID不能为空")
	}
	task, err := s.taskDAO.GetByID(taskID)
	if err != nil {
		return nil, err
	}
	hydrateCollectTaskPresentation(task)
	if task.Status == logtailmodel.LogStatusDisabled && isHostCollectType(task.CollectType) {
		agents, total, err := s.listAgentsForCollectTask(*task, logtailmodel.LogAgentQuery{Page: 1})
		if err != nil {
			return nil, err
		}
		return buildDisabledCollectTaskApplyResponse(*task, agents, total), nil
	}
	if isKubernetesCollectType(task.CollectType) {
		return s.applyKubernetesCollectTask(*task, req)
	}
	agents, total, err := s.listAgentsForCollectTask(*task, logtailmodel.LogAgentQuery{Page: 1})
	if err != nil {
		return nil, err
	}
	operator := strings.TrimSpace(req.Operator)
	if operator == "" {
		operator = "admin"
	}

	resp := &logtailmodel.LogCollectTaskApplyResponse{
		Task:    *task,
		Results: make([]logtailmodel.LogCollectTaskApplyResult, 0, len(agents)),
	}
	resp.Summary.MatchedAgents = total
	for i := range agents {
		resultItem := s.applyCollectTaskToAgent(*task, &agents[i], operator, req.Force)
		switch resultItem.Status {
		case "created":
			for _, opType := range resultItem.OpTypes {
				switch opType {
				case logtailmodel.LogAgentOperationInstallVector:
					resp.Summary.InstallCreated++
				case logtailmodel.LogAgentOperationReloadVector:
					resp.Summary.ReloadCreated++
				}
			}
		case "skipped":
			resp.Summary.Skipped++
		case "failed":
			resp.Summary.Failed++
		}
		resp.Results = append(resp.Results, resultItem)
	}
	return resp, nil
}

func buildDisabledCollectTaskApplyResponse(task logtailmodel.LogCollectTask, agents []logtailmodel.LogAgentVO, total int64) *logtailmodel.LogCollectTaskApplyResponse {
	resp := &logtailmodel.LogCollectTaskApplyResponse{
		Task:    task,
		Results: make([]logtailmodel.LogCollectTaskApplyResult, 0, len(agents)),
	}
	resp.Summary.MatchedAgents = total
	for i := range agents {
		resp.Summary.Skipped++
		resp.Results = append(resp.Results, logtailmodel.LogCollectTaskApplyResult{
			AgentID:      agents[i].AgentID,
			HostID:       agents[i].HostID,
			HostName:     firstNonEmpty(agents[i].HostName, agents[i].Name),
			PrivateIP:    agents[i].PrivateIP,
			VectorStatus: agents[i].VectorStatus,
			ConfigStatus: logtailmodel.LogAgentConfigStatusDisabled,
			Status:       "skipped",
			Message:      disabledCollectTaskConfigMessage(),
		})
	}
	return resp
}

func (s *LogtailService) StopCollectTask(taskID uint, req logtailmodel.LogCollectTaskApplyRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	if taskID == 0 {
		return nil, errors.New("采集任务ID不能为空")
	}
	task, err := s.taskDAO.GetByID(taskID)
	if err != nil {
		return nil, err
	}
	hydrateCollectTaskPresentation(task)
	if !isHostCollectType(task.CollectType) {
		return nil, ErrCollectTaskStopUnsupported
	}

	agents, total, err := s.listAgentsForCollectTask(*task, logtailmodel.LogAgentQuery{Page: 1})
	if err != nil {
		return nil, err
	}
	operator := strings.TrimSpace(req.Operator)
	if operator == "" {
		operator = "admin"
	}

	if task.Status != logtailmodel.LogStatusDisabled {
		task.Status = logtailmodel.LogStatusDisabled
		task.UpdatedBy = operator
		if task.ConfigVersion <= 0 {
			task.ConfigVersion = 1
		} else {
			task.ConfigVersion++
		}
		if err := s.taskDAO.Update(task); err != nil {
			return nil, err
		}
	}
	hydrateCollectTaskPresentation(task)

	resp := &logtailmodel.LogCollectTaskApplyResponse{
		Task:    *task,
		Results: make([]logtailmodel.LogCollectTaskApplyResult, 0, len(agents)),
	}
	resp.Summary.MatchedAgents = total
	for i := range agents {
		resultItem := s.stopCollectTaskOnAgent(*task, &agents[i], operator, req.Force)
		switch resultItem.Status {
		case "created":
			resp.Summary.ReloadCreated++
		case "skipped":
			resp.Summary.Skipped++
		case "failed":
			resp.Summary.Failed++
		}
		resp.Results = append(resp.Results, resultItem)
	}
	return resp, nil
}

func (s *LogtailService) listAgentsForCollectTask(task logtailmodel.LogCollectTask, query logtailmodel.LogAgentQuery) ([]logtailmodel.LogAgentVO, int64, error) {
	agentIDs, hostIDs := parseCollectTaskTargetIDs(task.ExecuteTargetsJSON)
	items, total, err := s.agentDAO.ListAgentsForCollectTask(task, agentIDs, hostIDs, query, agentHeartbeatThreshold())
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		if err := s.clearHealthyLogAgentLastError(&items[i]); err != nil {
			return nil, 0, err
		}
		applyLogAgentPresentation(&items[i])
	}
	return items, total, nil
}

func (s *LogtailService) fillLogAgentCollectTaskCounts(items []logtailmodel.LogAgentVO) error {
	for i := range items {
		count, err := s.agentDAO.CountCollectTasksForAgent(items[i].AgentID, items[i].HostID)
		if err != nil {
			return err
		}
		items[i].CollectTaskCount = count
	}
	return nil
}

func (s *LogtailService) applyCollectTaskToAgent(task logtailmodel.LogCollectTask, agent *logtailmodel.LogAgentVO, operator string, force bool) logtailmodel.LogCollectTaskApplyResult {
	result := logtailmodel.LogCollectTaskApplyResult{
		AgentID:      agent.AgentID,
		HostID:       agent.HostID,
		HostName:     firstNonEmpty(agent.HostName, agent.Name),
		PrivateIP:    agent.PrivateIP,
		VectorStatus: agent.VectorStatus,
		ConfigStatus: agent.ConfigStatus,
	}

	preview, err := s.PreviewAgentConfig(agent.AgentID)
	if err != nil {
		result.Status = "failed"
		result.Message = "生成 Vector 配置失败: " + err.Error()
		return result
	}

	opTypes := collectTaskApplyOperationTypes(agent, preview, force)
	if len(opTypes) == 0 {
		result.Status = "skipped"
		result.Message = "Vector 已运行且配置已生效"
		return result
	}

	for _, opType := range opTypes {
		if !force {
			active, err := s.operationDAO.HasActive(agent.AgentID, opType)
			if err != nil {
				result.Status = "failed"
				result.Message = "检查待执行操作失败: " + err.Error()
				return result
			}
			if active {
				continue
			}
		}
		requestPayload, _ := json.Marshal(map[string]interface{}{
			"opType":        opType,
			"collectTaskId": task.ID,
			"configVersion": preview.ConfigVersion,
			"operator":      operator,
			"force":         force,
		})
		configVersion := 0
		if operationNeedsConfig(opType) {
			configVersion = preview.ConfigVersion
		}
		operation, err := s.createAgentOperationForAgent(agent, logtailmodel.LogAgentOperationRequest{
			OpType:        opType,
			ConfigVersion: configVersion,
			RequestJSON:   string(requestPayload),
			Operator:      operator,
		})
		if err != nil {
			result.Status = "failed"
			result.Message = "创建操作失败: " + err.Error()
			return result
		}
		result.OpTypes = append(result.OpTypes, opType)
		result.OperationIDs = append(result.OperationIDs, operation.ID)
	}

	if len(result.OperationIDs) == 0 {
		result.Status = "skipped"
		result.Message = "已存在待执行操作"
		return result
	}
	result.Status = "created"
	result.Message = "已创建待执行操作"
	return result
}

func (s *LogtailService) stopCollectTaskOnAgent(task logtailmodel.LogCollectTask, agent *logtailmodel.LogAgentVO, operator string, force bool) logtailmodel.LogCollectTaskApplyResult {
	result := logtailmodel.LogCollectTaskApplyResult{
		AgentID:      agent.AgentID,
		HostID:       agent.HostID,
		HostName:     firstNonEmpty(agent.HostName, agent.Name),
		PrivateIP:    agent.PrivateIP,
		VectorStatus: agent.VectorStatus,
		ConfigStatus: agent.ConfigStatus,
	}

	if normalizeVectorStatus(agent.VectorStatus) == logtailmodel.VectorStatusNotInstalled {
		result.Status = "skipped"
		result.Message = "Vector 未安装，无需停止采集"
		return result
	}

	preview, err := s.PreviewAgentConfig(agent.AgentID)
	if err != nil {
		result.Status = "failed"
		result.Message = "生成停止后的 Vector 配置失败: " + err.Error()
		return result
	}
	result.ConfigHash = preview.ConfigHash

	if !force {
		active, err := s.operationDAO.HasActive(agent.AgentID, logtailmodel.LogAgentOperationReloadVector)
		if err != nil {
			result.Status = "failed"
			result.Message = "检查待执行操作失败: " + err.Error()
			return result
		}
		if active {
			result.Status = "skipped"
			result.Message = "已存在待执行重载操作"
			return result
		}
	}

	requestPayload, _ := json.Marshal(map[string]interface{}{
		"opType":        logtailmodel.LogAgentOperationReloadVector,
		"action":        "stop_collect_task",
		"collectTaskId": task.ID,
		"configVersion": preview.ConfigVersion,
		"operator":      operator,
		"force":         force,
	})
	operation, err := s.createAgentOperationForAgent(agent, logtailmodel.LogAgentOperationRequest{
		OpType:        logtailmodel.LogAgentOperationReloadVector,
		ConfigVersion: preview.ConfigVersion,
		RequestJSON:   string(requestPayload),
		Operator:      operator,
	})
	if err != nil {
		result.Status = "failed"
		result.Message = "创建停止采集重载操作失败: " + err.Error()
		return result
	}
	result.OpTypes = []string{logtailmodel.LogAgentOperationReloadVector}
	result.OperationIDs = []uint{operation.ID}
	result.Status = "created"
	result.Message = "已创建停止采集重载操作"
	return result
}

func (s *LogtailService) ListAgentOperations(agentID uint, query logtailmodel.LogAgentOperationQuery) ([]logtailmodel.LogAgentOperation, int64, logtailmodel.LogAgentOperationQuery, error) {
	if agentID == 0 {
		return nil, 0, query, errors.New("Agent ID不能为空")
	}
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.operationDAO.List(agentID, query)
	if err != nil {
		return nil, 0, query, err
	}
	s.fillAgentOperationTaskStatus(items)
	return items, total, query, err
}

func (s *LogtailService) fillAgentOperationTaskStatus(items []logtailmodel.LogAgentOperation) {
	taskStatusByID := make(map[uint]int)
	for i := range items {
		taskID := parseAgentOperationCollectTaskID(items[i].RequestJSON)
		if taskID == 0 {
			continue
		}
		items[i].CollectTaskID = taskID
		if status, ok := taskStatusByID[taskID]; ok {
			items[i].TaskStatus = status
			continue
		}
		task, err := s.taskDAO.GetByID(taskID)
		if err != nil || task == nil {
			continue
		}
		taskStatusByID[taskID] = task.Status
		items[i].TaskStatus = task.Status
	}
}

func parseAgentOperationCollectTaskID(raw string) uint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	var payload struct {
		CollectTaskID uint `json:"collectTaskId"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0
	}
	return payload.CollectTaskID
}

func (s *LogtailService) GetRuntimeConfig(req logtailmodel.LogAgentRuntimeRequest) (*logtailmodel.LogAgentConfigResponse, error) {
	agent, err := s.ResolveLogAgent(req.AgentID, req.HostID)
	if err != nil {
		return nil, err
	}
	preview, err := s.PreviewAgentConfig(agent.AgentID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(preview.Content) == "" {
		return &logtailmodel.LogAgentConfigResponse{
			NeedUpdate:    false,
			AgentID:       preview.AgentID,
			HostID:        preview.HostID,
			ConfigVersion: 0,
			ConfigHash:    "",
			ConfigPath:    preview.ConfigPath,
			Content:       "",
			Package:       preview.Package,
			HostAliases:   preview.HostAliases,
		}, nil
	}
	needUpdate := req.CurrentVersion != preview.ConfigVersion
	if strings.TrimSpace(req.CurrentHash) != "" && strings.TrimSpace(req.CurrentHash) != preview.ConfigHash {
		needUpdate = true
	}
	return &logtailmodel.LogAgentConfigResponse{
		NeedUpdate:    needUpdate,
		AgentID:       preview.AgentID,
		HostID:        preview.HostID,
		ConfigVersion: preview.ConfigVersion,
		ConfigHash:    preview.ConfigHash,
		ConfigPath:    preview.ConfigPath,
		Content:       preview.Content,
		Package:       preview.Package,
		HostAliases:   preview.HostAliases,
	}, nil
}

func (s *LogtailService) ClaimRuntimeOperations(req logtailmodel.LogAgentRuntimeRequest) (*logtailmodel.LogAgentOperationDispatchResponse, error) {
	agent, err := s.ResolveLogAgent(req.AgentID, req.HostID)
	if err != nil {
		return nil, err
	}
	operations, err := s.operationDAO.ClaimPending(agent.AgentID, req.Limit)
	if err != nil {
		return nil, err
	}
	resp := &logtailmodel.LogAgentOperationDispatchResponse{
		AgentID:    agent.AgentID,
		HostID:     agent.HostID,
		Operations: make([]logtailmodel.LogAgentOperationDispatch, 0, len(operations)),
	}
	packageInfo := s.GetVectorPackageInfo()
	var configPayload *logtailmodel.LogAgentConfigResponse
	for _, op := range operations {
		dispatch := logtailmodel.LogAgentOperationDispatch{
			Operation: op,
			Package:   packageInfo,
		}
		if operationNeedsConfig(op.OpType) {
			if configPayload == nil {
				cfg, cfgErr := s.GetRuntimeConfig(logtailmodel.LogAgentRuntimeRequest{
					AgentID: agent.AgentID,
					HostID:  agent.HostID,
				})
				if cfgErr != nil {
					return nil, cfgErr
				}
				configPayload = cfg
			}
			if strings.TrimSpace(configPayload.Content) == "" {
				if _, reportErr := s.operationDAO.Report(
					op.ID,
					agent.AgentID,
					logtailmodel.LogAgentOperationStatusCanceled,
					100,
					"未绑定采集任务，跳过配置下发",
					`{"message":"no bound collect tasks"}`,
					"",
				); reportErr != nil {
					return nil, reportErr
				}
				if pruneErr := s.operationDAO.PruneCompleted(agent.AgentID, op.ID); pruneErr != nil {
					return nil, pruneErr
				}
				continue
			}
			dispatch.Config = configPayload
		}
		resp.Operations = append(resp.Operations, dispatch)
	}
	return resp, nil
}

func (s *LogtailService) ReportRuntimeStatus(req logtailmodel.LogAgentStatusReportRequest) (*logtailmodel.LogAgentStatus, error) {
	agent, err := s.ResolveLogAgent(req.AgentID, req.HostID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	status := normalizeVectorStatus(req.VectorStatus)
	lastError := strings.TrimSpace(req.LastError)
	if shouldTreatRuntimeErrorAsInstalled(status, req, lastError) {
		status = logtailmodel.VectorStatusInstalled
		lastError = ""
	}
	configStatus := strings.TrimSpace(req.ConfigStatus)
	if configStatus == "" {
		configStatus = logtailmodel.LogAgentConfigStatusPending
	}
	desiredConfigVersion := req.DesiredConfigVersion
	desiredConfigHash := req.DesiredConfigHash
	if shouldPreserveRuntimeConfigFailure(agent, req, configStatus, lastError) {
		configStatus = logtailmodel.LogAgentConfigStatusFailed
		lastError = agent.LastError
		if agent.DesiredConfigVersion > desiredConfigVersion {
			desiredConfigVersion = agent.DesiredConfigVersion
		}
		if strings.TrimSpace(desiredConfigHash) == "" {
			desiredConfigHash = agent.DesiredConfigHash
		}
	}
	runtimeType := strings.TrimSpace(req.RuntimeType)
	if runtimeType == "" {
		runtimeType = logtailmodel.LogAgentRuntimeHost
	}
	item := &logtailmodel.LogAgentStatus{
		AgentID:              agent.AgentID,
		HostID:               agent.HostID,
		RuntimeType:          runtimeType,
		ClusterID:            req.ClusterID,
		NodeName:             req.NodeName,
		VectorVersion:        req.VectorVersion,
		VectorStatus:         status,
		VectorPID:            req.VectorPID,
		InstallPath:          req.InstallPath,
		ConfigPath:           req.ConfigPath,
		DesiredConfigVersion: desiredConfigVersion,
		AppliedConfigVersion: req.AppliedConfigVersion,
		DesiredConfigHash:    desiredConfigHash,
		AppliedConfigHash:    req.AppliedConfigHash,
		ConfigStatus:         configStatus,
		LastHeartbeatAt:      &now,
		LastError:            lastError,
		MetricsJSON:          req.MetricsJSON,
	}
	if shouldClearRuntimeLastError(status, configStatus, item.LastError) {
		item.LastError = ""
		item.LastErrorAt = nil
	} else if item.LastError != "" {
		item.LastErrorAt = &now
	}
	if shouldTouchRuntimeLastReload(agent, status, configStatus, req) {
		item.LastReloadAt = &now
	}
	if err := s.agentDAO.UpsertStatus(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *LogtailService) ReportRuntimeOperation(operationID uint, req logtailmodel.LogAgentOperationReportRequest) (*logtailmodel.LogAgentOperation, error) {
	if operationID == 0 {
		return nil, errors.New("操作ID不能为空")
	}
	agent, err := s.ResolveLogAgent(req.AgentID, req.HostID)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(req.OpStatus)
	if !isAllowedOperationStatus(status) {
		return nil, errors.New("不支持的操作状态")
	}
	progress := req.Progress
	progressText := strings.TrimSpace(req.ProgressText)
	if status == logtailmodel.LogAgentOperationStatusSuccess {
		if progress <= 0 {
			progress = 100
		}
		if progressText == "" {
			progressText = "执行完成"
		}
	}
	if status == logtailmodel.LogAgentOperationStatusFailed {
		if progress >= 0 {
			progress = -1
		}
		if progressText == "" {
			progressText = "执行失败"
		}
	}
	if status == logtailmodel.LogAgentOperationStatusRunning && progressText == "" {
		progressText = "执行中"
	}
	errorMsg := operationFailureMessage(req.ErrorMsg, progressText)
	if status != logtailmodel.LogAgentOperationStatusFailed {
		errorMsg = strings.TrimSpace(req.ErrorMsg)
	}
	item, err := s.operationDAO.Report(operationID, agent.AgentID, status, progress, progressText, req.ResultJSON, errorMsg)
	if err != nil {
		return nil, err
	}
	switch {
	case status == logtailmodel.LogAgentOperationStatusSuccess && operationSuccessClearsLastError(item.OpType):
		if operationSuccessMarksConfigApplied(item.OpType) {
			if err := s.agentDAO.MarkConfigApplied(agent.AgentID, agent.HostID, item.ConfigVersion); err != nil {
				return nil, err
			}
		} else {
			if err := s.agentDAO.ClearLastError(agent.AgentID); err != nil {
				return nil, err
			}
		}
	case status == logtailmodel.LogAgentOperationStatusFailed:
		if operationFailureMarksConfigFailed(item.OpType) {
			err = s.agentDAO.SetConfigError(agent.AgentID, agent.HostID, item.ErrorMsg, item.ConfigVersion)
		} else {
			err = s.agentDAO.SetLastError(agent.AgentID, agent.HostID, item.ErrorMsg)
		}
		if err != nil {
			return nil, err
		}
	}
	if isTerminalOperationStatus(status) {
		if err := s.operationDAO.PruneCompleted(agent.AgentID, item.ID); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *LogtailService) renderVectorConfig(agent *logtailmodel.LogAgentVO, tasks []logtailmodel.LogCollectTask, packageVersion string) (string, int, error) {
	if len(tasks) == 0 {
		return "", 0, nil
	}

	var builder strings.Builder
	builder.WriteString("data_dir: /var/lib/aiops-vector\n\n")
	builder.WriteString("sources:\n")
	legacyHTTPSink := shouldUseLegacyHTTPSinkConfig(firstNonEmpty(agent.VectorVersion, packageVersion))
	for _, task := range tasks {
		writeVectorHostFileSource(&builder, task)
	}

	builder.WriteString("\ntransforms:\n")
	maxVersion := 0
	for _, task := range tasks {
		if task.ConfigVersion > maxVersion {
			maxVersion = task.ConfigVersion
		}
		sourceID := vectorComponentID(task, "file")
		transformID := vectorComponentID(task, "parse")
		store, err := s.storeDAO.GetByID(task.StoreID)
		if err != nil {
			return "", 0, fmt.Errorf("查询日志库失败 task=%s: %w", task.Code, err)
		}
		project, err := s.projectDAO.GetByID(task.ProjectID)
		if err != nil {
			return "", 0, fmt.Errorf("查询项目失败 task=%s: %w", task.Code, err)
		}
		template, _ := s.templateDAO.GetByID(task.ParseTemplateID)

		builder.WriteString(fmt.Sprintf("  %s:\n", transformID))
		builder.WriteString("    type: remap\n")
		builder.WriteString("    inputs:\n")
		builder.WriteString(fmt.Sprintf("      - %s\n", sourceID))
		builder.WriteString("    source: |\n")
		writeVectorParseScript(&builder, task, template)
		writeVectorCommonFields(&builder, agent, task, project, store)
	}

	builder.WriteString("\nsinks:\n")
	for _, task := range tasks {
		transformID := vectorComponentID(task, "parse")
		sinkID := vectorComponentID(task, "delivery")
		if err := writeVectorDeliverySink(&builder, sinkID, transformID, legacyHTTPSink); err != nil {
			return "", 0, err
		}
	}
	if maxVersion == 0 {
		maxVersion = 1
	}
	return builder.String(), maxVersion, nil
}

func writeVectorHostFileSource(builder *strings.Builder, task logtailmodel.LogCollectTask) {
	sourceID := vectorComponentID(task, "file")
	builder.WriteString(fmt.Sprintf("  %s:\n", sourceID))
	builder.WriteString("    type: file\n")
	builder.WriteString("    include:\n")
	for _, pattern := range collectTaskPathPatterns(&task) {
		builder.WriteString(fmt.Sprintf("      - %s\n", yamlQuote(pattern)))
	}
	writeVectorFileRotationSettings(builder)
	if pattern := effectiveCollectTaskMultilinePattern(task); pattern != "" {
		builder.WriteString("    multiline:\n")
		builder.WriteString(fmt.Sprintf("      start_pattern: %s\n", yamlQuote(pattern)))
		builder.WriteString("      mode: halt_before\n")
		builder.WriteString(fmt.Sprintf("      condition_pattern: %s\n", yamlQuote(pattern)))
		builder.WriteString("      timeout_ms: 5000\n")
	}
}

func effectiveCollectTaskMultilinePattern(task logtailmodel.LogCollectTask) string {
	if task.MultilineEnabledSnapshot != nil && !*task.MultilineEnabledSnapshot {
		return ""
	}
	if pattern := normalizeDefaultTextMultilinePattern(task.MultilinePatternSnapshot); pattern != "" {
		return pattern
	}
	if task.MultilineEnabledSnapshot == nil {
		return defaultMultilinePatternForMode(task.ParseModeSnapshot)
	}
	return ""
}

func writeVectorFileRotationSettings(builder *strings.Builder) {
	builder.WriteString("    fingerprint:\n")
	builder.WriteString(fmt.Sprintf("      strategy: %s\n", vectorFileFingerprintStrategy))
	builder.WriteString(fmt.Sprintf("    read_from: %s\n", vectorFileReadFrom))
}

type vectorSinkOptions struct {
	DiskBufferMaxSizeBytes int64
	BufferWhenFull         string
	BatchMaxEvents         int
	BatchMaxBytes          int
	BatchTimeoutSecs       int
	RequestTimeoutSecs     int
}

type vectorKafkaSinkOptions struct {
	Brokers []string
	Topic   string
}

type vectorAggregatorInstallOptions struct {
	Name       string
	Image      string
	Namespace  string
	Replicas   int32
	ConfigPath string
	DataDir    string
}

func effectiveVectorSinkOptions() vectorSinkOptions {
	opts := vectorSinkOptions{
		DiskBufferMaxSizeBytes: 2 * 1024 * 1024 * 1024,
		BufferWhenFull:         "block",
		BatchMaxEvents:         1000,
		BatchMaxBytes:          1024 * 1024,
		BatchTimeoutSecs:       1,
		RequestTimeoutSecs:     60,
	}
	if config.Config == nil {
		return opts
	}
	vector := config.Config.Logtail.Vector
	if vector.DiskBufferMaxSizeBytes > 0 {
		opts.DiskBufferMaxSizeBytes = vector.DiskBufferMaxSizeBytes
	}
	return opts
}

func effectiveVectorAggregatorInstallOptions() vectorAggregatorInstallOptions {
	opts := vectorAggregatorInstallOptions{
		Name:       vectorAggregatorName,
		Image:      "docker.1ms.run/timberio/vector:latest-debian",
		Namespace:  k8sVectorNamespace,
		Replicas:   1,
		ConfigPath: vectorAggregatorConfigPath,
		DataDir:    vectorAggregatorDataDir,
	}
	if config.Config == nil {
		return opts
	}
	if image := strings.TrimSpace(config.Config.Logtail.Aggregator.Image); image != "" {
		opts.Image = image
	}
	return opts
}

func effectiveVectorKafkaSinkOptions() (vectorKafkaSinkOptions, bool, error) {
	if config.Config == nil {
		return vectorKafkaSinkOptions{}, false, nil
	}
	mode := normalizeDeliveryMode(config.Config.Logtail.DeliveryMode)
	kafka := config.Config.Logtail.Kafka
	ready := kafka.Enabled && len(kafka.Brokers) > 0
	switch mode {
	case "direct":
		return vectorKafkaSinkOptions{}, false, nil
	case "kafka":
		if !ready {
			return vectorKafkaSinkOptions{}, false, fmt.Errorf("logtail delivery_mode=kafka but kafka.enabled=false or brokers empty")
		}
		return vectorKafkaSinkOptions{Brokers: kafka.Brokers, Topic: firstNonEmpty(kafka.Topic, "aiops-logs")}, true, nil
	default:
		if !ready {
			return vectorKafkaSinkOptions{}, false, nil
		}
		return vectorKafkaSinkOptions{Brokers: kafka.Brokers, Topic: firstNonEmpty(kafka.Topic, "aiops-logs")}, true, nil
	}
}

func effectiveVectorKafkaAggregatorOptions() (vectorKafkaSinkOptions, error) {
	if config.Config == nil {
		return vectorKafkaSinkOptions{}, errors.New("logtail.kafka 未配置，无法生成 Kafka Aggregator 配置")
	}
	mode := normalizeDeliveryMode(config.Config.Logtail.DeliveryMode)
	if mode == "direct" {
		return vectorKafkaSinkOptions{}, errors.New("当前 logtail.delivery_mode=direct，无需启动 Kafka Aggregator")
	}
	kafka := config.Config.Logtail.Kafka
	if !kafka.Enabled || len(kafka.Brokers) == 0 {
		return vectorKafkaSinkOptions{}, errors.New("Kafka 未启用或 brokers 为空，无法生成 Kafka Aggregator 配置")
	}
	return vectorKafkaSinkOptions{
		Brokers: kafka.Brokers,
		Topic:   firstNonEmpty(kafka.Topic, "aiops-logs"),
	}, nil
}

func renderVectorAggregatorConfig(kafkaOpts vectorKafkaSinkOptions, legacyHTTPSink bool) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("data_dir: %s\n\n", vectorAggregatorDataDir))
	builder.WriteString("sources:\n")
	builder.WriteString("  kafka_logs:\n")
	builder.WriteString("    type: kafka\n")
	builder.WriteString(fmt.Sprintf("    bootstrap_servers: %s\n", yamlQuote(strings.Join(kafkaOpts.Brokers, ","))))
	builder.WriteString(fmt.Sprintf("    group_id: %s\n", yamlQuote(vectorAggregatorConsumerGroup)))
	builder.WriteString("    topics:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", yamlQuote(firstNonEmpty(kafkaOpts.Topic, "aiops-logs"))))
	builder.WriteString("    auto_offset_reset: earliest\n")
	builder.WriteString("    decoding:\n")
	builder.WriteString("      codec: json\n\n")
	builder.WriteString("sinks:\n")
	writeVectorHTTPSink(&builder, "victorialogs", "kafka_logs", legacyHTTPSink)
	return builder.String()
}

func buildVectorAggregatorDockerPreview(configContent string, opts vectorAggregatorInstallOptions) logtailmodel.LogVectorAggregatorDockerPreview {
	return logtailmodel.LogVectorAggregatorDockerPreview{
		ContainerName: opts.Name,
		Image:         opts.Image,
		ConfigPath:    opts.ConfigPath,
		DataDir:       opts.DataDir,
		Command:       renderVectorAggregatorDockerCommand(configContent, opts),
	}
}

func renderVectorAggregatorDockerCommand(configContent string, opts vectorAggregatorInstallOptions) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("mkdir -p %s %s\n\n", linuxDir(opts.ConfigPath), opts.DataDir))
	builder.WriteString(fmt.Sprintf("cat > %s <<'EOF'\n", opts.ConfigPath))
	builder.WriteString(strings.TrimRight(configContent, "\n"))
	builder.WriteString("\nEOF\n\n")
	builder.WriteString(fmt.Sprintf("docker pull %s\n\n", shellQuote(opts.Image)))
	builder.WriteString(fmt.Sprintf("docker rm -f %s 2>/dev/null || true\n\n", shellQuote(opts.Name)))
	builder.WriteString("docker run -d \\\n")
	builder.WriteString(fmt.Sprintf("  --name %s \\\n", shellQuote(opts.Name)))
	builder.WriteString("  --restart unless-stopped \\\n")
	builder.WriteString("  --network host \\\n")
	builder.WriteString("  --security-opt seccomp=unconfined \\\n")
	builder.WriteString("  --pids-limit -1 \\\n")
	builder.WriteString("  --ulimit nproc=65535:65535 \\\n")
	builder.WriteString(fmt.Sprintf("  -v %s:%s:ro \\\n", opts.ConfigPath, vectorAggregatorContainerConfigPath))
	builder.WriteString(fmt.Sprintf("  -v %s:%s \\\n", opts.DataDir, opts.DataDir))
	builder.WriteString(fmt.Sprintf("  %s \\\n", shellQuote(opts.Image)))
	builder.WriteString(fmt.Sprintf("  --config %s\n", vectorAggregatorContainerConfigPath))
	return builder.String()
}

func buildVectorAggregatorKubernetesPreview(configContent, configHash string, opts vectorAggregatorInstallOptions) logtailmodel.LogVectorAggregatorKubernetesPreview {
	return logtailmodel.LogVectorAggregatorKubernetesPreview{
		Namespace:      opts.Namespace,
		DeploymentName: opts.Name,
		ConfigMapName:  opts.Name,
		Replicas:       opts.Replicas,
		FileName:       vectorAggregatorKubernetesFileName,
		ApplyCommand:   fmt.Sprintf("kubectl apply -f %s", vectorAggregatorKubernetesFileName),
		YAML:           renderVectorAggregatorKubernetesYAML(configContent, configHash, opts),
	}
}

func renderVectorAggregatorKubernetesYAML(configContent, configHash string, opts vectorAggregatorInstallOptions) string {
	shortHash := ""
	if strings.HasPrefix(configHash, "sha256:") && len(configHash) >= len("sha256:")+12 {
		shortHash = configHash[len("sha256:") : len("sha256:")+12]
	}
	if shortHash == "" {
		shortHash = shortConfigHash(configContent)
	}

	var builder strings.Builder
	builder.WriteString("apiVersion: v1\n")
	builder.WriteString("kind: Namespace\n")
	builder.WriteString("metadata:\n")
	builder.WriteString(fmt.Sprintf("  name: %s\n", opts.Namespace))
	builder.WriteString("---\n")
	builder.WriteString("apiVersion: v1\n")
	builder.WriteString("kind: ConfigMap\n")
	builder.WriteString("metadata:\n")
	builder.WriteString(fmt.Sprintf("  name: %s\n", opts.Name))
	builder.WriteString(fmt.Sprintf("  namespace: %s\n", opts.Namespace))
	builder.WriteString("  labels:\n")
	builder.WriteString(fmt.Sprintf("    app: %s\n", opts.Name))
	builder.WriteString("    app.kubernetes.io/managed-by: aiops-backend\n")
	builder.WriteString("data:\n")
	builder.WriteString("  vector.yaml: |\n")
	writeYAMLBlock(&builder, configContent, "    ")
	builder.WriteString("---\n")
	builder.WriteString("apiVersion: apps/v1\n")
	builder.WriteString("kind: Deployment\n")
	builder.WriteString("metadata:\n")
	builder.WriteString(fmt.Sprintf("  name: %s\n", opts.Name))
	builder.WriteString(fmt.Sprintf("  namespace: %s\n", opts.Namespace))
	builder.WriteString("  labels:\n")
	builder.WriteString(fmt.Sprintf("    app: %s\n", opts.Name))
	builder.WriteString("    app.kubernetes.io/managed-by: aiops-backend\n")
	builder.WriteString("spec:\n")
	builder.WriteString(fmt.Sprintf("  replicas: %d\n", opts.Replicas))
	builder.WriteString("  selector:\n")
	builder.WriteString("    matchLabels:\n")
	builder.WriteString(fmt.Sprintf("      app: %s\n", opts.Name))
	builder.WriteString("  template:\n")
	builder.WriteString("    metadata:\n")
	builder.WriteString("      labels:\n")
	builder.WriteString(fmt.Sprintf("        app: %s\n", opts.Name))
	builder.WriteString(fmt.Sprintf("        app.kubernetes.io/name: %s\n", opts.Name))
	builder.WriteString("        app.kubernetes.io/managed-by: aiops-backend\n")
	builder.WriteString("      annotations:\n")
	builder.WriteString(fmt.Sprintf("        aiops.dev/config-hash: %s\n", yamlQuote(shortHash)))
	builder.WriteString("    spec:\n")
	builder.WriteString("      securityContext:\n")
	builder.WriteString("        seccompProfile:\n")
	builder.WriteString("          type: Unconfined\n")
	writeKubernetesHostAliasesYAML(&builder, "      ")
	builder.WriteString("      containers:\n")
	builder.WriteString("        - name: vector\n")
	builder.WriteString(fmt.Sprintf("          image: %s\n", yamlQuote(opts.Image)))
	builder.WriteString("          imagePullPolicy: IfNotPresent\n")
	builder.WriteString("          args:\n")
	builder.WriteString("            - --config\n")
	builder.WriteString(fmt.Sprintf("            - %s\n", vectorAggregatorContainerConfigPath))
	builder.WriteString("          resources:\n")
	builder.WriteString("            requests:\n")
	builder.WriteString("              cpu: 100m\n")
	builder.WriteString("              memory: 256Mi\n")
	builder.WriteString("            limits:\n")
	builder.WriteString("              cpu: \"2\"\n")
	builder.WriteString("              memory: 2Gi\n")
	builder.WriteString("          volumeMounts:\n")
	builder.WriteString("            - name: config\n")
	builder.WriteString("              mountPath: /etc/vector\n")
	builder.WriteString("              readOnly: true\n")
	builder.WriteString("            - name: data\n")
	builder.WriteString(fmt.Sprintf("              mountPath: %s\n", opts.DataDir))
	builder.WriteString("      volumes:\n")
	builder.WriteString("        - name: config\n")
	builder.WriteString("          configMap:\n")
	builder.WriteString(fmt.Sprintf("            name: %s\n", opts.Name))
	builder.WriteString("        - name: data\n")
	builder.WriteString("          emptyDir: {}\n")
	return builder.String()
}

func writeKubernetesHostAliasesYAML(builder *strings.Builder, indent string) {
	aliases := configuredKafkaHostAliases()
	if len(aliases) == 0 {
		return
	}
	builder.WriteString(indent)
	builder.WriteString("hostAliases:\n")
	for _, alias := range aliases {
		builder.WriteString(indent)
		builder.WriteString("  - ip: ")
		builder.WriteString(yamlQuote(alias.IP))
		builder.WriteString("\n")
		builder.WriteString(indent)
		builder.WriteString("    hostnames:\n")
		for _, hostname := range alias.Hostnames {
			builder.WriteString(indent)
			builder.WriteString("      - ")
			builder.WriteString(yamlQuote(hostname))
			builder.WriteString("\n")
		}
	}
}

func configuredKafkaHostAliasModels() []logtailmodel.KafkaHostAlias {
	if config.Config == nil {
		return nil
	}
	aliases := config.Config.Logtail.Kafka.HostAliases
	if len(aliases) == 0 {
		return nil
	}
	result := make([]logtailmodel.KafkaHostAlias, 0, len(aliases))
	for _, alias := range aliases {
		ip := strings.TrimSpace(alias.IP)
		hostnames := trimStringSlice(alias.Hostnames)
		if ip == "" || len(hostnames) == 0 {
			continue
		}
		result = append(result, logtailmodel.KafkaHostAlias{
			IP:        ip,
			Hostnames: hostnames,
		})
	}
	return result
}

func vectorConfigHash(content string, hostAliases []logtailmodel.KafkaHostAlias) string {
	var builder strings.Builder
	builder.WriteString(content)
	if len(hostAliases) > 0 {
		builder.WriteString("\n# aiops-host-aliases\n")
		for _, alias := range hostAliases {
			ip := strings.TrimSpace(alias.IP)
			hostnames := trimStringSlice(alias.Hostnames)
			if ip == "" || len(hostnames) == 0 {
				continue
			}
			builder.WriteString(ip)
			builder.WriteString(" ")
			builder.WriteString(strings.Join(hostnames, " "))
			builder.WriteString("\n")
		}
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(builder.String())))
}

func normalizeDeliveryMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "kafka":
		return "kafka"
	case "direct":
		return "direct"
	default:
		return "auto"
	}
}

func writeVectorDeliverySink(builder *strings.Builder, sinkID, inputID string, legacyHTTPSink bool) error {
	kafkaOpts, useKafka, err := effectiveVectorKafkaSinkOptions()
	if err != nil {
		return err
	}
	if useKafka {
		writeVectorKafkaSink(builder, sinkID, inputID, kafkaOpts)
		return nil
	}
	writeVectorHTTPSink(builder, sinkID, inputID, legacyHTTPSink)
	return nil
}

func writeVectorKafkaSink(builder *strings.Builder, sinkID, inputID string, opts vectorKafkaSinkOptions) {
	bufferOpts := effectiveVectorSinkOptions()
	builder.WriteString(fmt.Sprintf("  %s:\n", sinkID))
	builder.WriteString("    type: kafka\n")
	builder.WriteString("    inputs:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", inputID))
	builder.WriteString(fmt.Sprintf("    bootstrap_servers: %s\n", yamlQuote(strings.Join(opts.Brokers, ","))))
	builder.WriteString(fmt.Sprintf("    topic: %s\n", yamlQuote(firstNonEmpty(opts.Topic, "aiops-logs"))))
	builder.WriteString("    encoding:\n")
	builder.WriteString("      codec: json\n")
	builder.WriteString("    compression: gzip\n")
	builder.WriteString("    librdkafka_options:\n")
	builder.WriteString("      client.id: aiops-logtail\n")
	builder.WriteString("      request.required.acks: all\n")
	builder.WriteString("    buffer:\n")
	builder.WriteString("      type: disk\n")
	builder.WriteString(fmt.Sprintf("      max_size: %d\n", bufferOpts.DiskBufferMaxSizeBytes))
	builder.WriteString(fmt.Sprintf("      when_full: %s\n", bufferOpts.BufferWhenFull))
}

func writeVectorHTTPSink(builder *strings.Builder, sinkID, inputID string, legacyHTTPSink bool) {
	opts := effectiveVectorSinkOptions()
	builder.WriteString(fmt.Sprintf("  %s:\n", sinkID))
	builder.WriteString("    type: http\n")
	builder.WriteString("    inputs:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", inputID))
	builder.WriteString(fmt.Sprintf("    uri: %s\n", yamlQuote(victoriaLogsJSONLineURI())))
	builder.WriteString("    method: post\n")
	builder.WriteString("    encoding:\n")
	if legacyHTTPSink {
		builder.WriteString("      codec: ndjson\n")
	} else {
		builder.WriteString("      codec: json\n")
		builder.WriteString("    framing:\n")
		builder.WriteString("      method: newline_delimited\n")
	}
	builder.WriteString("    batch:\n")
	builder.WriteString(fmt.Sprintf("      max_events: %d\n", opts.BatchMaxEvents))
	builder.WriteString(fmt.Sprintf("      max_bytes: %d\n", opts.BatchMaxBytes))
	builder.WriteString(fmt.Sprintf("      timeout_secs: %d\n", opts.BatchTimeoutSecs))
	builder.WriteString("    request:\n")
	builder.WriteString(fmt.Sprintf("      timeout_secs: %d\n", opts.RequestTimeoutSecs))
	if !legacyHTTPSink {
		builder.WriteString("    buffer:\n")
		builder.WriteString("      type: disk\n")
		builder.WriteString(fmt.Sprintf("      max_size: %d\n", opts.DiskBufferMaxSizeBytes))
		builder.WriteString(fmt.Sprintf("      when_full: %s\n", opts.BufferWhenFull))
	}
	builder.WriteString("    compression: gzip\n")
}

func writeVectorParseScript(builder *strings.Builder, task logtailmodel.LogCollectTask, template *logtailmodel.LogParseTemplate) {
	mode := normalizeCollectTaskParseMode(task.ParseModeSnapshot)
	if mode == "" && template != nil {
		mode = normalizeCollectTaskParseMode(template.ParseMode)
	}
	builder.WriteString("      ._raw = .message\n")
	if shouldDropVectorSelfLogs(task) {
		builder.WriteString("      _msg_lower = downcase(._raw) ?? \"\"\n")
		builder.WriteString("      if contains(_msg_lower, \" vector:\") || contains(_msg_lower, \"aiops-vector\") || contains(_msg_lower, \"victoria-logs\") || contains(_msg_lower, \" victorialogs/\") {\n")
		builder.WriteString("        abort\n")
		builder.WriteString("      }\n")
	}
	switch mode {
	case "json":
		writeVectorJSONParseScript(builder)
		if shouldNormalizeKubernetesStdoutJSONMessage(task) {
			writeKubernetesStdoutJSONMessageScript(builder)
		}
		writeVectorRegexExtractScript(builder, task.RegexPattern)
	case "nginx":
		if !writeVectorTemplateScript(builder, template) {
			writeVectorNginxParseScript(builder)
		}
		writeVectorRegexExtractScript(builder, task.RegexPattern)
	case "nginx_json":
		if !writeVectorTemplateScript(builder, template) {
			writeVectorNginxJSONParseScript(builder)
		}
		writeVectorRegexExtractScript(builder, task.RegexPattern)
	case "nginx_json_loose":
		writeVectorNginxJSONLooseParseScript(builder)
		writeVectorRegexExtractScript(builder, task.RegexPattern)
	case "db_slow_log":
		if !writeVectorTemplateScript(builder, template) {
			writeVectorDBSlowLogParseScript(builder)
		}
		writeVectorRegexExtractScriptFromField(builder, task.RegexPattern, ".sql_text")
	case "regex":
		if !writeVectorRegexExtractScript(builder, task.RegexPattern) {
			writeVectorTemplateScript(builder, template)
		}
	default:
		writeVectorTemplateScript(builder, template)
	}
	builder.WriteString("      ._msg = .message\n")
	builder.WriteString("      if !exists(.timestamp) {\n")
	builder.WriteString("        .timestamp = now()\n")
	builder.WriteString("      }\n")
	builder.WriteString("      ._time = .timestamp\n")
}

func shouldNormalizeKubernetesStdoutJSONMessage(task logtailmodel.LogCollectTask) bool {
	if task.CollectType != logtailmodel.LogCollectTypeKubernetes {
		return false
	}
	return normalizeKubernetesLogSource(task.K8SLogSource) == logtailmodel.LogK8SLogSourceStdout
}

func writeVectorJSONParseScript(builder *strings.Builder) {
	builder.WriteString("      parsed, err = parse_json(._raw)\n")
	builder.WriteString("      if err == null && is_object(parsed) {\n")
	builder.WriteString("        ., merge_err = merge(., parsed)\n")
	builder.WriteString("      }\n")
}

func writeKubernetesStdoutJSONMessageScript(builder *strings.Builder) {
	builder.WriteString("      log_text = to_string(.log) ?? \"\"\n")
	builder.WriteString("      if log_text != \"\" {\n")
	builder.WriteString("        .message = log_text\n")
	builder.WriteString("        inner, inner_err = parse_json(log_text)\n")
	builder.WriteString("        if inner_err == null && is_object(inner) {\n")
	builder.WriteString("          ., merge_err = merge(., inner)\n")
	builder.WriteString("          if exists(.msg) {\n")
	builder.WriteString("            .message = to_string(.msg) ?? log_text\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
}

func writeVectorNginxParseScript(builder *strings.Builder) {
	builder.WriteString("      parsed, err = parse_nginx_log(._raw, \"combined\")\n")
	builder.WriteString("      if err == null && is_object(parsed) {\n")
	builder.WriteString("        . = merge(., parsed)\n")
	builder.WriteString("        .remote_addr = .client\n")
	builder.WriteString("        .remote_user = .user\n")
	builder.WriteString("        .body_bytes_sent = .size\n")
	builder.WriteString("        .http_referer = .referer\n")
	builder.WriteString("        .http_user_agent = .agent\n")
	builder.WriteString("        request_parts, request_err = parse_regex(.request, r'^(?P<request_method>[A-Z]+)\\s+(?P<request_uri>\\S+)(?:\\s+HTTP/(?P<http_version>[0-9.]+))?$')\n")
	builder.WriteString("        if request_err == null && is_object(request_parts) {\n")
	builder.WriteString("          . = merge(., request_parts)\n")
	builder.WriteString("          uri_parts, uri_err = parse_regex(.request_uri, r'^(?P<request_path>[^?]+)(?:\\?(?P<request_query>.*))?$')\n")
	builder.WriteString("          if uri_err == null && is_object(uri_parts) {\n")
	builder.WriteString("            . = merge(., uri_parts)\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
}

func writeVectorNginxJSONParseScript(builder *strings.Builder) {
	writeVectorJSONParseScript(builder)
	writeVectorNginxJSONFieldMappingScript(builder)
}

func writeVectorNginxJSONLooseParseScript(builder *strings.Builder) {
	builder.WriteString("      .parse_status = \"raw_fallback\"\n")
	builder.WriteString("      parsed, err = parse_json(._raw)\n")
	builder.WriteString("      if err == null && is_object(parsed) {\n")
	builder.WriteString("        ., merge_err = merge(., parsed)\n")
	builder.WriteString("        .parse_status = \"json_ok\"\n")
	builder.WriteString("      } else {\n")
	builder.WriteString("        loose_found = false\n")
	writeVectorLooseJSONStringField(builder, "@timestamp", "@timestamp")
	writeVectorLooseJSONStringField(builder, "clientip", "clientip")
	writeVectorLooseJSONStringField(builder, "request_method", "request_method")
	writeVectorLooseJSONStringField(builder, "domain", "domain")
	writeVectorLooseJSONStringField(builder, "referer", "referer")
	writeVectorLooseJSONStringField(builder, "request", "request")
	writeVectorLooseJSONNumberField(builder, "size", "size")
	writeVectorLooseJSONNumberField(builder, "status", "status")
	writeVectorLooseJSONNumberField(builder, "responsetime", "responsetime")
	writeVectorLooseJSONStringField(builder, "upstreamtime", "upstreamtime")
	writeVectorLooseJSONStringField(builder, "upstreamaddr", "upstreamaddr")
	writeVectorLooseJSONStringField(builder, "request_id", "request_id")
	writeVectorLooseJSONStringField(builder, "http_user_agent", "http_user_agent")
	writeVectorLooseJSONStringField(builder, "lyj_openclaw_api_key", "lyj_openclaw_api_key")
	writeVectorLooseJSONStringField(builder, "x_zai_user", "x_zai_user")
	writeVectorLooseJSONStringField(builder, "x_zeroai_user", "x_zeroai_user")
	writeVectorLooseJSONStringField(builder, "x_zai_trace_id", "x_zai_trace_id")
	writeVectorLooseJSONStringField(builder, "x_zai_workspace", "x_zai_workspace")
	writeVectorLooseJSONStringField(builder, "has_x_zai_api_key", "has_x_zai_api_key")
	writeVectorLooseJSONStringField(builder, "x_zai_session_token", "x_zai_session_token")
	writeVectorLooseJSONStringField(builder, "args", "args")
	writeVectorLooseJSONBodyField(builder, "post_body", "post_body")
	writeVectorLooseJSONBodyField(builder, "request_body", "request_body")
	builder.WriteString("        if loose_found {\n")
	builder.WriteString("          .parse_status = \"loose_ok\"\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	writeVectorNginxJSONFieldMappingScript(builder)
}

func writeVectorNginxJSONFieldMappingScript(builder *strings.Builder) {
	builder.WriteString("      if exists(.clientip) && !exists(.remote_addr) {\n")
	builder.WriteString("        .remote_addr = .clientip\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.request) && !exists(.request_uri) {\n")
	builder.WriteString("        .request_uri = .request\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.request_uri) {\n")
	builder.WriteString("        uri_parts, uri_err = parse_regex(.request_uri, r'^(?P<request_path>[^?]+)(?:\\?(?P<request_query>.*))?$')\n")
	builder.WriteString("        if uri_err == null && is_object(uri_parts) {\n")
	builder.WriteString("          . = merge(., uri_parts)\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.size) && !exists(.body_bytes_sent) {\n")
	builder.WriteString("        .body_bytes_sent = .size\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.referer) && !exists(.http_referer) {\n")
	builder.WriteString("        .http_referer = .referer\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.responsetime) && !exists(.request_time) {\n")
	builder.WriteString("        .request_time = .responsetime\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.upstreamtime) && !exists(.upstream_response_time) {\n")
	builder.WriteString("        .upstream_response_time = .upstreamtime\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.status) {\n")
	builder.WriteString("        .status = to_int(.status) ?? .status\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.body_bytes_sent) {\n")
	builder.WriteString("        .body_bytes_sent = to_int(.body_bytes_sent) ?? .body_bytes_sent\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.request_time) {\n")
	builder.WriteString("        .request_time = to_float(.request_time) ?? .request_time\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.upstream_response_time) {\n")
	builder.WriteString("        .upstream_response_time = to_float(.upstream_response_time) ?? .upstream_response_time\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if exists(.\"@timestamp\") {\n")
	builder.WriteString("        .event_timestamp = .\"@timestamp\"\n")
	builder.WriteString("        timestamp_text = to_string(.\"@timestamp\") ?? \"\"\n")
	builder.WriteString("        parsed_time, parsed_time_err = parse_timestamp(timestamp_text, \"%+\")\n")
	builder.WriteString("        if parsed_time_err == null {\n")
	builder.WriteString("          .timestamp = parsed_time\n")
	builder.WriteString("        } else {\n")
	builder.WriteString("          parsed_time, parsed_time_err = parse_timestamp(timestamp_text, \"%Y-%m-%d %H:%M:%S\")\n")
	builder.WriteString("          if parsed_time_err == null {\n")
	builder.WriteString("            .timestamp = parsed_time\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
}

func writeVectorLooseJSONStringField(builder *strings.Builder, jsonKey, fieldName string) {
	vrlName := toVRLIdentifier(fieldName)
	vrlField := "." + fieldName
	if strings.ContainsAny(fieldName, "@-.") {
		vrlField = ".\"" + fieldName + "\""
	}
	builder.WriteString(fmt.Sprintf("        loose_%s_parts, loose_%s_err = parse_regex(._raw, r'\"%s\"\\s*:\\s*\"(?P<loose_%s>[^\"]*)\"')\n", vrlName, vrlName, jsonKey, vrlName))
	builder.WriteString(fmt.Sprintf("        if loose_%s_err == null && is_object(loose_%s_parts) {\n", vrlName, vrlName))
	builder.WriteString(fmt.Sprintf("          %s = loose_%s_parts.loose_%s\n", vrlField, vrlName, vrlName))
	builder.WriteString("          loose_found = true\n")
	builder.WriteString("        }\n")
}

func writeVectorLooseJSONNumberField(builder *strings.Builder, jsonKey, fieldName string) {
	vrlName := toVRLIdentifier(fieldName)
	builder.WriteString(fmt.Sprintf("        loose_%s_parts, loose_%s_err = parse_regex(._raw, r'\"%s\"\\s*:\\s*(?P<loose_%s>-?[0-9]+(?:\\.[0-9]+)?)')\n", vrlName, vrlName, jsonKey, vrlName))
	builder.WriteString(fmt.Sprintf("        if loose_%s_err == null && is_object(loose_%s_parts) {\n", vrlName, vrlName))
	builder.WriteString(fmt.Sprintf("          .%s = loose_%s_parts.loose_%s\n", fieldName, vrlName, vrlName))
	builder.WriteString("          loose_found = true\n")
	builder.WriteString("        }\n")
}

func writeVectorLooseJSONBodyField(builder *strings.Builder, jsonKey, fieldName string) {
	vrlName := toVRLIdentifier(fieldName)
	builder.WriteString(fmt.Sprintf("        loose_%s_parts, loose_%s_err = parse_regex(._raw, r'\"%s\"\\s*:\\s*\"(?P<loose_%s>.*)\"\\s*}\\s*$')\n", vrlName, vrlName, jsonKey, vrlName))
	builder.WriteString(fmt.Sprintf("        if loose_%s_err == null && is_object(loose_%s_parts) {\n", vrlName, vrlName))
	builder.WriteString(fmt.Sprintf("          .%s = loose_%s_parts.loose_%s\n", fieldName, vrlName, vrlName))
	builder.WriteString("          loose_found = true\n")
	builder.WriteString("        }\n")
}

func toVRLIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "field"
	}
	var builder strings.Builder
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			builder.WriteRune(ch)
		} else {
			builder.WriteRune('_')
		}
	}
	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return "field"
	}
	if result[0] >= '0' && result[0] <= '9' {
		return "field_" + result
	}
	return result
}

func writeVectorDBSlowLogParseScript(builder *strings.Builder) {
	builder.WriteString("      slow_time_found = false\n")
	builder.WriteString("      time_parts, time_err = parse_regex(._raw, r'(?m)^# Time:\\s*(?P<slow_time>[^\\n]+)$')\n")
	builder.WriteString("      if time_err == null && is_object(time_parts) {\n")
	builder.WriteString("        . = merge(., time_parts)\n")
	builder.WriteString("        parsed_slow_time, parsed_slow_time_err = parse_timestamp(.slow_time, \"%+\")\n")
	builder.WriteString("        if parsed_slow_time_err == null {\n")
	builder.WriteString("          .timestamp = parsed_slow_time\n")
	builder.WriteString("          slow_time_found = true\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	builder.WriteString("      user_parts, user_err = parse_regex(._raw, r'(?m)^# User@Host:\\s+(?P<db_user>[^\\[]+)\\[[^\\]]*\\]\\s+@\\s+.*\\[(?P<db_host>[^\\]]*)\\]\\s+Id:\\s+(?P<thread_id>\\d+)$')\n")
	builder.WriteString("      if user_err == null && is_object(user_parts) {\n")
	builder.WriteString("        . = merge(., user_parts)\n")
	builder.WriteString("      }\n")
	builder.WriteString("      schema_parts, schema_err = parse_regex(._raw, r'(?m)^# Schema:\\s*(?P<database>\\S+)\\s+Last_errno:\\s*(?P<last_errno>-?\\d+)\\s+Killed:\\s*(?P<killed>-?\\d+)$')\n")
	builder.WriteString("      if schema_err == null && is_object(schema_parts) {\n")
	builder.WriteString("        . = merge(., schema_parts)\n")
	builder.WriteString("      }\n")
	builder.WriteString("      query_parts, query_err = parse_regex(._raw, r'(?m)^# Query_time:\\s*(?P<query_time>[0-9.]+)\\s+Lock_time:\\s*(?P<lock_time>[0-9.]+)\\s+Rows_sent:\\s*(?P<rows_sent>-?\\d+)\\s+Rows_examined:\\s*(?P<rows_examined>-?\\d+)(?:\\s+Rows_affected:\\s*(?P<rows_affected>-?\\d+))?$')\n")
	builder.WriteString("      if query_err == null && is_object(query_parts) {\n")
	builder.WriteString("        . = merge(., query_parts)\n")
	builder.WriteString("      }\n")
	builder.WriteString("      bytes_parts, bytes_err = parse_regex(._raw, r'(?m)^# Bytes_sent:\\s*(?P<bytes_sent>-?\\d+)$')\n")
	builder.WriteString("      if bytes_err == null && is_object(bytes_parts) {\n")
	builder.WriteString("        . = merge(., bytes_parts)\n")
	builder.WriteString("      }\n")
	builder.WriteString("      sql_parts, sql_err = parse_regex(._raw, r'(?ms)(?:^|\\n)(?:use\\s+`?(?P<use_database>[^`;\\s]+)`?;\\n)?SET timestamp=(?P<unix_ts>\\d+);\\n(?P<sql_text>.*)$')\n")
	builder.WriteString("      if sql_err == null && is_object(sql_parts) {\n")
	builder.WriteString("        . = merge(., sql_parts)\n")
	builder.WriteString("        if .database == \"\" && .use_database != \"\" {\n")
	builder.WriteString("          .database = .use_database\n")
	builder.WriteString("        }\n")
	builder.WriteString("        if !slow_time_found {\n")
	builder.WriteString("          parsed_unix_time, parsed_unix_time_err = parse_timestamp(.unix_ts, \"%s\")\n")
	builder.WriteString("          if parsed_unix_time_err == null {\n")
	builder.WriteString("            .timestamp = parsed_unix_time\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	builder.WriteString("      .query_time = to_float(.query_time) ?? .query_time\n")
	builder.WriteString("      .lock_time = to_float(.lock_time) ?? .lock_time\n")
	builder.WriteString("      .rows_sent = to_int(.rows_sent) ?? .rows_sent\n")
	builder.WriteString("      .rows_examined = to_int(.rows_examined) ?? .rows_examined\n")
	builder.WriteString("      .rows_affected = to_int(.rows_affected) ?? .rows_affected\n")
	builder.WriteString("      .bytes_sent = to_int(.bytes_sent) ?? .bytes_sent\n")
	builder.WriteString("      .thread_id = to_int(.thread_id) ?? .thread_id\n")
	builder.WriteString("      .last_errno = to_int(.last_errno) ?? .last_errno\n")
	builder.WriteString("      .killed = to_int(.killed) ?? .killed\n")
	builder.WriteString("      operation_parts, operation_err = parse_regex(.sql_text, r'(?is)^\\s*(?P<sql_operation>[A-Za-z_]+)')\n")
	builder.WriteString("      if operation_err == null && is_object(operation_parts) {\n")
	builder.WriteString("        . = merge(., operation_parts)\n")
	builder.WriteString("      }\n")
	builder.WriteString("      .message = .sql_text\n")
}

func writeVectorTemplateScript(builder *strings.Builder, template *logtailmodel.LogParseTemplate) bool {
	if template == nil || strings.TrimSpace(template.VRLScript) == "" {
		return false
	}
	script, _ := repairGeneratedVRLScript(template.VRLScript, nil)
	for _, line := range strings.Split(script, "\n") {
		builder.WriteString("      ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return true
}

func writeVectorRegexExtractScript(builder *strings.Builder, pattern string) bool {
	return writeVectorRegexExtractScriptFromField(builder, pattern, "._raw")
}

func writeVectorRegexExtractScriptFromField(builder *strings.Builder, pattern, fieldExpr string) bool {
	literal, ok := buildVRLRegexLiteral(pattern)
	if !ok {
		return false
	}
	builder.WriteString(fmt.Sprintf("      parsed, err = parse_regex(%s, %s)\n", fieldExpr, literal))
	builder.WriteString("      if err == null && is_object(parsed) {\n")
	builder.WriteString("        . = merge(., parsed)\n")
	builder.WriteString("      }\n")
	return true
}

func buildVRLRegexLiteral(pattern string) (string, bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.Contains(pattern, "'") || strings.ContainsAny(pattern, "\r\n") {
		return "", false
	}
	return "r'" + pattern + "'", true
}

func shouldDropVectorSelfLogs(task logtailmodel.LogCollectTask) bool {
	for _, pattern := range collectTaskPathPatterns(&task) {
		switch strings.ToLower(strings.TrimSpace(pattern)) {
		case "/var/log/messages", "/var/log/syslog":
			return true
		}
	}
	return false
}

func writeVectorCommonFields(builder *strings.Builder, agent *logtailmodel.LogAgentVO, task logtailmodel.LogCollectTask, project *logtailmodel.LogProject, store *logtailmodel.LogStore) {
	fields := map[string]string{
		"project_code":      project.Code,
		"store_code":        store.Code,
		"collect_task_name": task.Name,
		"collect_task_code": task.Code,
		"env":               project.Env,
		"log_source":        "host_file",
		"log_type":          firstNonEmpty(displayCollectTaskParseMode(task.ParseModeSnapshot), store.StoreType, "app_log"),
		"agent_id":          strconv.FormatUint(uint64(agent.AgentID), 10),
		"host_id":           strconv.FormatUint(uint64(agent.HostID), 10),
		"host_name":         firstNonEmpty(agent.HostName, agent.Name),
		"app":               firstNonEmpty(store.Code, task.Code),
	}
	labels := parseStringMap(task.LabelsJSON)
	for key, value := range labels {
		if vectorFieldRegexp.MatchString(key) {
			fields[key] = value
		}
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if vectorFieldRegexp.MatchString(key) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		builder.WriteString(fmt.Sprintf("      .%s = %s\n", key, strconv.Quote(fields[key])))
	}
	builder.WriteString("      .file_path = .file\n")
	writeVectorLogFileNameScript(builder)
}

func writeVectorLogFileNameScript(builder *strings.Builder) {
	// Keep this hook in place for future file-name facets, but do not emit VRL
	// for now. Older Vector versions reject several otherwise valid parse_regex
	// variants at compile time, and a label enhancement must not break startup.
}

func parseStringMap(raw string) map[string]string {
	result := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return result
	}
	var generic map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &generic); err != nil {
		return result
	}
	for key, value := range generic {
		switch v := value.(type) {
		case string:
			result[key] = v
		case float64:
			result[key] = strconv.FormatFloat(v, 'f', -1, 64)
		case bool:
			result[key] = strconv.FormatBool(v)
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

func vectorComponentID(task logtailmodel.LogCollectTask, suffix string) string {
	raw := fmt.Sprintf("task_%d_%s_%s", task.ID, task.Code, suffix)
	value := vectorIdentifierRegexp.ReplaceAllString(raw, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		return fmt.Sprintf("task_%d_%s", task.ID, suffix)
	}
	return value
}

func victoriaLogsJSONLineURI() string {
	baseURL := ""
	if config.Config != nil {
		baseURL = strings.TrimRight(strings.TrimSpace(config.Config.Logtail.VictoriaLogs.URL), "/")
	}
	if baseURL == "" {
		baseURL = "http://127.0.0.1:9428"
	}
	streamFields := strings.Join([]string{
		"project_code",
		"store_code",
		"env",
		"host_name",
		"collect_task_name",
		"collect_task_code",
		"log_source",
		"k8s_log_source",
		"k8s_cluster_name",
		"k8s_namespace",
		"k8s_workload_name",
		"k8s_pod_name",
		"k8s_container_name",
		"log_file_name",
	}, ",")
	return fmt.Sprintf("%s/insert/jsonline?_stream_fields=%s&_msg_field=_msg&_time_field=_time", baseURL, streamFields)
}

func buildCollectTaskAgentSummary(items []logtailmodel.LogAgentVO) logtailmodel.LogCollectTaskAgentSummary {
	var summary logtailmodel.LogCollectTaskAgentSummary
	for _, item := range items {
		summary.Total++
		if item.AgentOnlineStatus == cmdbmodel.AgentOnlineStatusOnline {
			summary.Online++
		} else {
			summary.Offline++
		}
		switch normalizeVectorStatus(item.VectorStatus) {
		case logtailmodel.VectorStatusRunning:
			summary.VectorRunning++
		case logtailmodel.VectorStatusInstalled:
			summary.VectorInstalled++
		case logtailmodel.VectorStatusError:
			summary.VectorError++
		default:
			summary.VectorNotInstalled++
		}
		switch item.ConfigStatus {
		case logtailmodel.LogAgentConfigStatusApplied:
			summary.ConfigApplied++
		case logtailmodel.LogAgentConfigStatusFailed:
			summary.ConfigFailed++
		case logtailmodel.LogAgentConfigStatusDisabled:
			summary.ConfigDisabled++
		default:
			summary.ConfigPending++
		}
	}
	return summary
}

func applyCollectTaskAgentPresentation(task *logtailmodel.LogCollectTask, items []logtailmodel.LogAgentVO) {
	for i := range items {
		applyLogAgentPresentation(&items[i])
	}
	if task == nil || task.Status != logtailmodel.LogStatusDisabled {
		return
	}
	for i := range items {
		items[i].ConfigStatus = logtailmodel.LogAgentConfigStatusDisabled
		items[i].ConfigStatusText = disabledCollectTaskConfigMessage()
	}
}

func applyLogAgentPresentation(item *logtailmodel.LogAgentVO) {
	if item == nil {
		return
	}
	item.ConfigStatusText = logAgentConfigStatusText(*item)
}

func logAgentConfigStatusText(item logtailmodel.LogAgentVO) string {
	switch strings.TrimSpace(item.ConfigStatus) {
	case logtailmodel.LogAgentConfigStatusApplied:
		return "配置已生效"
	case logtailmodel.LogAgentConfigStatusFailed:
		if normalizeVectorStatus(item.VectorStatus) == logtailmodel.VectorStatusRunning &&
			configVersionBehind(item.DesiredConfigVersion, item.AppliedConfigVersion) {
			return "运行中，新配置未生效"
		}
		return "配置应用失败"
	case logtailmodel.LogAgentConfigStatusPending:
		if configVersionBehind(item.DesiredConfigVersion, item.AppliedConfigVersion) {
			return "新配置待生效"
		}
		return "配置待生效"
	case logtailmodel.LogAgentConfigStatusDisabled:
		return disabledCollectTaskConfigMessage()
	default:
		return "配置状态未知"
	}
}

func configVersionBehind(desired, applied int) bool {
	return desired > 0 && desired > applied
}

func disabledCollectTaskConfigMessage() string {
	return "任务禁用中，配置不生效"
}

func collectTaskApplyOperationTypes(agent *logtailmodel.LogAgentVO, preview *logtailmodel.LogAgentConfigPreview, force bool) []string {
	installed := normalizeVectorStatus(agent.VectorStatus) != logtailmodel.VectorStatusNotInstalled &&
		(strings.TrimSpace(agent.VectorVersion) != "" || strings.TrimSpace(agent.InstallPath) != "")
	if !installed || shouldReinstallVector(agent) {
		return []string{
			logtailmodel.LogAgentOperationInstallVector,
			logtailmodel.LogAgentOperationReloadVector,
		}
	}
	if force {
		return []string{logtailmodel.LogAgentOperationReloadVector}
	}
	if normalizeVectorStatus(agent.VectorStatus) != logtailmodel.VectorStatusRunning {
		return []string{logtailmodel.LogAgentOperationReloadVector}
	}
	if agent.ConfigStatus != logtailmodel.LogAgentConfigStatusApplied {
		return []string{logtailmodel.LogAgentOperationReloadVector}
	}
	if agent.AppliedConfigVersion != preview.ConfigVersion {
		return []string{logtailmodel.LogAgentOperationReloadVector}
	}
	if strings.TrimSpace(agent.AppliedConfigHash) != "" && agent.AppliedConfigHash != preview.ConfigHash {
		return []string{logtailmodel.LogAgentOperationReloadVector}
	}
	return nil
}

func parseCollectTaskTargetIDs(raw string) ([]uint, []uint) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var targets []struct {
		AgentID uint `json:"agentId"`
		HostID  uint `json:"hostId"`
	}
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return nil, nil
	}
	agentIDs := make([]uint, 0, len(targets))
	hostIDs := make([]uint, 0, len(targets))
	seenAgents := map[uint]struct{}{}
	seenHosts := map[uint]struct{}{}
	for _, target := range targets {
		if target.AgentID > 0 {
			if _, ok := seenAgents[target.AgentID]; !ok {
				agentIDs = append(agentIDs, target.AgentID)
				seenAgents[target.AgentID] = struct{}{}
			}
		}
		if target.HostID > 0 {
			if _, ok := seenHosts[target.HostID]; !ok {
				hostIDs = append(hostIDs, target.HostID)
				seenHosts[target.HostID] = struct{}{}
			}
		}
	}
	return agentIDs, hostIDs
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%X", hash.Sum(nil)), nil
}

func detectVectorVersion(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	output, err := exec.Command(absPath, "--version").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func buildUploadPackageURL(packagePath string) string {
	uploadDir := strings.TrimSpace(config.Config.ImageSettings.UploadDir)
	if uploadDir == "" {
		uploadDir = "./upload"
	}
	absUpload, err1 := filepath.Abs(uploadDir)
	absPackage, err2 := filepath.Abs(packagePath)
	if err1 != nil || err2 != nil {
		return ""
	}
	rel, err := filepath.Rel(absUpload, absPackage)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	rel = filepath.ToSlash(rel)
	path := "/api/v1/upload/" + strings.TrimLeft(rel, "/")
	publicURL := strings.TrimRight(strings.TrimSpace(config.Config.Server.PublicURL), "/")
	if publicURL == "" {
		return path
	}
	parsed, err := url.Parse(publicURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return path
	}
	return publicURL + path
}

func configuredVectorPackagePath() string {
	packagePath := ""
	if config.Config != nil {
		packagePath = strings.TrimSpace(config.Config.Logtail.Vector.PackagePath)
	}
	if packagePath == "" {
		packagePath = "./upload/logtail/vector"
	}
	return packagePath
}

func resolveVectorPackageTargetPath() (string, error) {
	packagePath := configuredVectorPackagePath()
	uploadDir := "./upload"
	if config.Config != nil {
		uploadDir = strings.TrimSpace(config.Config.ImageSettings.UploadDir)
		if uploadDir == "" {
			uploadDir = "./upload"
		}
	}

	absUpload, err := filepath.Abs(uploadDir)
	if err != nil {
		return "", err
	}
	absPackage, err := filepath.Abs(packagePath)
	if err != nil {
		return "", err
	}
	if !pathWithinBase(absUpload, absPackage) {
		return "", fmt.Errorf("Vector 包路径不在 upload 目录内: %s", absPackage)
	}
	return absPackage, nil
}

func pathWithinBase(basePath, targetPath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func validateVectorPackageTargetForUpload(replace bool) error {
	targetPath, err := resolveVectorPackageTargetPath()
	if err != nil {
		return err
	}
	stat, statErr := os.Stat(targetPath)
	switch {
	case statErr == nil:
		if stat.IsDir() {
			return errors.New("当前 Vector 包路径是目录，无法直接上传文件")
		}
		if !replace {
			return errors.New("当前 Vector 包已存在，请先删除后再上传")
		}
	case os.IsNotExist(statErr):
		return nil
	default:
		return statErr
	}
	return nil
}

func nextVectorPackageUploadTaskID() string {
	seq := atomic.AddUint64(&vectorPackageUploadTaskSeq, 1)
	return fmt.Sprintf("vector-upload-%d-%d", time.Now().UnixNano(), seq)
}

func cloneVectorPackageUploadTask(task *logtailmodel.VectorPackageUploadTask) *logtailmodel.VectorPackageUploadTask {
	if task == nil {
		return nil
	}
	vectorPackageUploadTaskMu.RLock()
	defer vectorPackageUploadTaskMu.RUnlock()
	clone := *task
	if task.Package != nil {
		packageInfo := *task.Package
		clone.Package = &packageInfo
	}
	return &clone
}

func updateVectorPackageUploadTask(task *logtailmodel.VectorPackageUploadTask, update func(*logtailmodel.VectorPackageUploadTask)) {
	if task == nil || update == nil {
		return
	}
	vectorPackageUploadTaskMu.Lock()
	defer vectorPackageUploadTaskMu.Unlock()
	update(task)
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, onProgress func(uploaded, total int64)) (int64, error) {
	buf := make([]byte, 1024*1024)
	var written int64
	if onProgress != nil {
		onProgress(0, total)
	}
	for {
		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
				if onProgress != nil {
					onProgress(written, total)
				}
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return written, readErr
		}
	}
	return written, nil
}

func agentHeartbeatThreshold() time.Time {
	timeout := 300
	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatTimeout > 0 {
		timeout = config.Config.Monitor.Agent.HeartbeatTimeout
	}
	return time.Now().Add(-time.Duration(timeout) * time.Second)
}

func isAllowedAgentOperation(opType string) bool {
	switch opType {
	case logtailmodel.LogAgentOperationInstallVector,
		logtailmodel.LogAgentOperationUpgradeVector,
		logtailmodel.LogAgentOperationValidateConfig,
		logtailmodel.LogAgentOperationReloadVector,
		logtailmodel.LogAgentOperationRestartVector,
		logtailmodel.LogAgentOperationStopVector,
		logtailmodel.LogAgentOperationUninstallVector:
		return true
	default:
		return false
	}
}

func isAllowedOperationStatus(status string) bool {
	switch status {
	case logtailmodel.LogAgentOperationStatusPending,
		logtailmodel.LogAgentOperationStatusRunning,
		logtailmodel.LogAgentOperationStatusSuccess,
		logtailmodel.LogAgentOperationStatusFailed,
		logtailmodel.LogAgentOperationStatusCanceled:
		return true
	default:
		return false
	}
}

func isTerminalOperationStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case logtailmodel.LogAgentOperationStatusSuccess,
		logtailmodel.LogAgentOperationStatusFailed,
		logtailmodel.LogAgentOperationStatusCanceled:
		return true
	default:
		return false
	}
}

func operationFailureMessage(errorMsg, progressText string) string {
	if msg := strings.TrimSpace(errorMsg); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(progressText); msg != "" {
		return msg
	}
	return "执行失败"
}

func operationNeedsConfig(opType string) bool {
	switch opType {
	case logtailmodel.LogAgentOperationValidateConfig,
		logtailmodel.LogAgentOperationReloadVector,
		logtailmodel.LogAgentOperationRestartVector:
		return true
	default:
		return false
	}
}

func yamlQuote(value string) string {
	return strconv.Quote(value)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func linuxDir(value string) string {
	value = strings.TrimSpace(value)
	idx := strings.LastIndex(value, "/")
	if idx <= 0 {
		return "."
	}
	return value[:idx]
}

func writeYAMLBlock(builder *strings.Builder, content, indent string) {
	content = strings.TrimRight(content, "\n")
	for _, line := range strings.Split(content, "\n") {
		builder.WriteString(indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeVectorStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", logtailmodel.VectorStatusNotInstalled:
		return logtailmodel.VectorStatusNotInstalled
	case logtailmodel.VectorStatusInstalled:
		return logtailmodel.VectorStatusInstalled
	case logtailmodel.VectorStatusRunning:
		return logtailmodel.VectorStatusRunning
	case "stopped", logtailmodel.VectorStatusError:
		return logtailmodel.VectorStatusError
	default:
		return logtailmodel.VectorStatusError
	}
}

func shouldClearRuntimeLastError(status, configStatus, lastError string) bool {
	if status == logtailmodel.VectorStatusRunning && configStatus == logtailmodel.LogAgentConfigStatusApplied {
		return true
	}
	if strings.TrimSpace(lastError) != "" {
		return false
	}
	return status == logtailmodel.VectorStatusNotInstalled
}

func shouldPreserveRuntimeConfigFailure(agent *logtailmodel.LogAgentVO, req logtailmodel.LogAgentStatusReportRequest, configStatus, lastError string) bool {
	if agent == nil || agent.ConfigStatus != logtailmodel.LogAgentConfigStatusFailed {
		return false
	}
	if strings.TrimSpace(agent.LastError) == "" || strings.TrimSpace(lastError) != "" {
		return false
	}
	if configStatus == logtailmodel.LogAgentConfigStatusFailed {
		return false
	}
	if configStatus == logtailmodel.LogAgentConfigStatusApplied &&
		runtimeReportCoversDesiredConfig(agent, req) {
		return false
	}
	return true
}

func runtimeReportCoversDesiredConfig(agent *logtailmodel.LogAgentVO, req logtailmodel.LogAgentStatusReportRequest) bool {
	if agent == nil {
		return false
	}
	if agent.DesiredConfigVersion > 0 && req.AppliedConfigVersion >= agent.DesiredConfigVersion {
		return true
	}
	desiredHash := strings.TrimSpace(agent.DesiredConfigHash)
	appliedHash := strings.TrimSpace(req.AppliedConfigHash)
	return desiredHash != "" && appliedHash != "" && desiredHash == appliedHash
}

func shouldTouchRuntimeLastReload(agent *logtailmodel.LogAgentVO, status, configStatus string, req logtailmodel.LogAgentStatusReportRequest) bool {
	if status != logtailmodel.VectorStatusRunning || configStatus != logtailmodel.LogAgentConfigStatusApplied {
		return false
	}
	if agent == nil {
		return true
	}
	if normalizeVectorStatus(agent.VectorStatus) != logtailmodel.VectorStatusRunning ||
		agent.ConfigStatus != logtailmodel.LogAgentConfigStatusApplied {
		return true
	}
	if req.AppliedConfigVersion > 0 && req.AppliedConfigVersion != agent.AppliedConfigVersion {
		return true
	}
	appliedHash := strings.TrimSpace(req.AppliedConfigHash)
	return appliedHash != "" && appliedHash != strings.TrimSpace(agent.AppliedConfigHash)
}

func shouldTreatRuntimeErrorAsInstalled(status string, req logtailmodel.LogAgentStatusReportRequest, lastError string) bool {
	if status != logtailmodel.VectorStatusError {
		return false
	}
	if req.DesiredConfigVersion > 0 || req.AppliedConfigVersion > 0 ||
		strings.TrimSpace(req.DesiredConfigHash) != "" || strings.TrimSpace(req.AppliedConfigHash) != "" {
		return false
	}
	if strings.TrimSpace(req.VectorVersion) == "" && strings.TrimSpace(req.InstallPath) == "" {
		return false
	}
	lower := strings.ToLower(lastError)
	if strings.Contains(lower, "vector binary check failed") ||
		strings.Contains(lower, "glibc") ||
		strings.Contains(lower, "failed") {
		return false
	}
	return lastError == "" ||
		strings.Contains(lower, "active: inactive") ||
		strings.Contains(lower, "inactive (dead)")
}

func (s *LogtailService) clearHealthyLogAgentLastError(item *logtailmodel.LogAgentVO) error {
	if !shouldClearLogAgentLastError(item) {
		return nil
	}
	if err := s.agentDAO.ClearLastError(item.AgentID); err != nil {
		return err
	}
	item.LastError = ""
	item.LastErrorAt = nil
	return nil
}

func shouldClearLogAgentLastError(item *logtailmodel.LogAgentVO) bool {
	if item == nil || strings.TrimSpace(item.LastError) == "" {
		return false
	}
	return shouldClearRuntimeLastError(normalizeVectorStatus(item.VectorStatus), item.ConfigStatus, "")
}

func operationSuccessClearsLastError(opType string) bool {
	switch strings.TrimSpace(opType) {
	case logtailmodel.LogAgentOperationInstallVector,
		logtailmodel.LogAgentOperationUpgradeVector,
		logtailmodel.LogAgentOperationValidateConfig,
		logtailmodel.LogAgentOperationReloadVector,
		logtailmodel.LogAgentOperationRestartVector,
		logtailmodel.LogAgentOperationStopVector,
		logtailmodel.LogAgentOperationUninstallVector:
		return true
	default:
		return false
	}
}

func operationSuccessMarksConfigApplied(opType string) bool {
	switch strings.TrimSpace(opType) {
	case logtailmodel.LogAgentOperationReloadVector,
		logtailmodel.LogAgentOperationRestartVector:
		return true
	default:
		return false
	}
}

func operationFailureMarksConfigFailed(opType string) bool {
	return operationNeedsConfig(opType)
}

func operationCreationMarksConfigPending(opType string) bool {
	return operationNeedsConfig(opType)
}

func shouldUseLegacyHTTPSinkConfig(version string) bool {
	major, minor, _, ok := parseVectorSemver(version)
	if !ok {
		return false
	}
	if major != 0 {
		return false
	}
	if minor < 23 {
		return true
	}
	return false
}

func parseVectorSemver(raw string) (int, int, int, bool) {
	matches := vectorSemverRegexp.FindStringSubmatch(strings.TrimSpace(raw))
	if len(matches) != 4 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

func shouldReinstallVector(agent *logtailmodel.LogAgentVO) bool {
	if agent == nil {
		return false
	}
	if normalizeVectorStatus(agent.VectorStatus) != logtailmodel.VectorStatusError {
		return false
	}
	if strings.TrimSpace(agent.VectorVersion) == "" || strings.TrimSpace(agent.InstallPath) == "" {
		return true
	}
	return agent.DesiredConfigVersion == 0 && agent.AppliedConfigVersion == 0
}
