package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	k8sdao "dodevops-api/api/k8s/dao"
	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common"
	"dodevops-api/common/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	k8sVectorNamespace       = "aiops-vector"
	k8sVectorConfigFile      = "vector.yaml"
	k8sCollectorManagedBy    = "aiops-backend"
	k8sCollectorVolumePrefix = "aiops-log-share"
	k8sVectorConfigVolume    = "aiops-vector-config"
	k8sVectorDataVolume      = "aiops-vector-data"
	k8sVectorDataDir         = "/var/lib/aiops-vector"
	k8sVectorHostDataDir     = "/var/lib/aiops-vector/k8s"

	k8sClusterCollectorBusyMessage = "集群级 Vector 正在更新中，请等待当前下发完成后再操作"
	k8sCollectorActiveWindow       = 30 * time.Minute
)

var (
	k8sNameInvalidChars       = regexp.MustCompile(`[^a-z0-9-]+`)
	kafkaFailedResolvePattern = regexp.MustCompile(`(?i)failed to resolve '([^']+)'`)
)

type kubernetesCollectTarget struct {
	ClusterID    uint
	Namespace    string
	WorkloadType string
	WorkloadName string
	Containers   []string
}

type indexedKubernetesCollectTarget struct {
	Index  int
	Target kubernetesCollectTarget
}

func (s *LogtailService) applyKubernetesCollectTask(task logtailmodel.LogCollectTask, req logtailmodel.LogCollectTaskApplyRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	targets, err := parseIndexedKubernetesCollectTargetsForTask(task)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, errors.New("K8S日志采集必须选择目标")
	}
	actionReq := logtailmodel.LogK8SCollectorActionRequest{
		Operator: req.Operator,
		Force:    req.Force,
	}
	return s.runKubernetesCollectorAction(task, targets, logtailmodel.LogK8SCollectorOperationUpdate, actionReq)
}

func (s *LogtailService) runKubernetesCollectorAction(task logtailmodel.LogCollectTask, targets []indexedKubernetesCollectTarget, opType string, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	if len(targets) == 0 {
		return nil, errors.New("K8S日志采集必须选择目标")
	}
	operator := strings.TrimSpace(req.Operator)
	if operator == "" {
		operator = "admin"
	}

	resp := &logtailmodel.LogCollectTaskApplyResponse{
		Task:    task,
		Results: make([]logtailmodel.LogCollectTaskApplyResult, 0, len(targets)),
	}
	resp.Summary.MatchedAgents = int64(len(targets))

	clusterCache := map[uint]*k8smodel.KubeCluster{}
	clientCache := map[uint]*kubernetes.Clientset{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	for _, indexedTarget := range targets {
		target := indexedTarget.Target
		resultItem := logtailmodel.LogCollectTaskApplyResult{
			ConfigStatus: logtailmodel.LogAgentConfigStatusPending,
			PrivateIP:    kubernetesCollectTargetDisplayName(target),
		}
		cluster, clientset, err := s.getKubernetesClusterClient(target.ClusterID, clusterCache, clientCache)
		if cluster != nil {
			resultItem.HostName = cluster.Name
		}
		if err != nil {
			resultItem.Status = "failed"
			resultItem.Message = err.Error()
			resp.Summary.Failed++
			resp.Results = append(resp.Results, resultItem)
			continue
		}
		resultItem.HostName = cluster.Name
		resultItem.VectorStatus = logtailmodel.VectorStatusRunning
		if isKubernetesClusterLevelCollectorMutation(task, opType) {
			if err := s.ensureKubernetesClusterCollectorIdle(ctx, clientset, target.ClusterID); err != nil {
				return nil, err
			}
		}

		operation, opErr := s.createKubernetesCollectorOperation(task, indexedTarget, opType, operator, req)
		if opErr != nil {
			resultItem.Status = "failed"
			resultItem.Message = "创建K8S Collector操作记录失败: " + opErr.Error()
			resp.Summary.Failed++
			resp.Results = append(resp.Results, resultItem)
			continue
		}
		_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusRunning, 10, "开始执行K8S Collector操作", "", "")

		originalStatus := task.Status
		if err := s.persistKubernetesTaskStatusForOperation(&task, opType); err != nil {
			resultItem.Status = "failed"
			resultItem.Message = "更新采集任务状态失败: " + err.Error()
			resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
			_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusFailed, -1, resultItem.Message, "", err.Error())
			resp.Summary.Failed++
			resp.Results = append(resp.Results, resultItem)
			continue
		}
		resp.Task = task
		err = s.executeKubernetesCollectorAction(ctx, clientset, cluster, task, target, opType)

		if err != nil {
			if task.Status != originalStatus {
				task.Status = originalStatus
				_ = s.taskDAO.Update(&task)
				resp.Task = task
			}
			resultItem.Status = "failed"
			resultItem.Message = err.Error()
			resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
			_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusFailed, -1, "执行K8S Collector操作失败", "", err.Error())
			resp.Summary.Failed++
		} else {
			status, statusErr := s.buildKubernetesCollectorStatus(ctx, clientset, cluster, task, indexedTarget)
			resultJSON := ""
			if statusErr == nil {
				resultItem.ConfigHash = status.ConfigHash
				resultItem.ConfigYAML = status.ConfigYAML
				if payload, marshalErr := json.Marshal(status); marshalErr == nil {
					resultJSON = string(payload)
				}
			}
			resultItem.OpTypes = []string{opType}
			resultItem.OperationIDs = []uint{operation.ID}
			if statusErr != nil {
				resultItem.Status = "failed"
				resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
				resultItem.Message = "查询K8S Collector状态失败: " + statusErr.Error()
				_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusFailed, -1, resultItem.Message, resultJSON, statusErr.Error())
				resp.Summary.Failed++
			} else if status.Status == "failed" || status.ConfigStatus == logtailmodel.LogAgentConfigStatusFailed {
				resultItem.Status = "failed"
				resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
				resultItem.Message = firstNonEmpty(status.LastError, status.ProgressText, "K8S Collector运行失败")
				_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusFailed, -1, resultItem.Message, resultJSON, resultItem.Message)
				resp.Summary.Failed++
			} else if status.Status == "running" && status.ConfigStatus == logtailmodel.LogAgentConfigStatusApplied {
				resultItem.Status = "created"
				resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusApplied
				resultItem.Message = kubernetesOperationSuccessMessage(task, opType)
				_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusSuccess, 100, resultItem.Message, resultJSON, "")
				resp.Summary.ReloadCreated++
			} else {
				resultItem.Status = "pending"
				resultItem.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
				resultItem.Message = firstNonEmpty(status.ProgressText, "K8S Collector滚动更新中")
				_, _ = s.k8sOpDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusRunning, status.Progress, resultItem.Message, resultJSON, "")
				resp.Summary.ReloadCreated++
			}
		}
		resp.Results = append(resp.Results, resultItem)
	}

	return resp, nil
}

func (s *LogtailService) persistKubernetesTaskStatusForOperation(task *logtailmodel.LogCollectTask, opType string) error {
	if task == nil {
		return nil
	}
	desiredStatus := task.Status
	switch opType {
	case logtailmodel.LogK8SCollectorOperationInstall, logtailmodel.LogK8SCollectorOperationReinstall:
		desiredStatus = logtailmodel.LogStatusEnabled
	case logtailmodel.LogK8SCollectorOperationUninstall:
		desiredStatus = logtailmodel.LogStatusDisabled
	default:
		return nil
	}
	if task.Status == desiredStatus {
		return nil
	}
	task.Status = desiredStatus
	return s.taskDAO.Update(task)
}

func (s *LogtailService) executeKubernetesCollectorAction(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget, opType string) error {
	switch opType {
	case logtailmodel.LogK8SCollectorOperationInstall, logtailmodel.LogK8SCollectorOperationUpdate:
		return s.applyKubernetesCollectorTarget(ctx, clientset, cluster, task, target)
	case logtailmodel.LogK8SCollectorOperationReinstall:
		if err := s.uninstallKubernetesCollectorTarget(ctx, clientset, cluster, task, target); err != nil {
			return err
		}
		return s.applyKubernetesCollectorTarget(ctx, clientset, cluster, task, target)
	case logtailmodel.LogK8SCollectorOperationUninstall:
		return s.uninstallKubernetesCollectorTarget(ctx, clientset, cluster, task, target)
	default:
		return fmt.Errorf("不支持的K8S Collector操作: %s", opType)
	}
}

func (s *LogtailService) applyKubernetesCollectorTarget(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	if normalizeKubernetesLogSource(task.K8SLogSource) != logtailmodel.LogK8SLogSourceNodeFile {
		resolved, err := resolveKubernetesWorkloadContainers(ctx, clientset, target)
		if err != nil {
			return fmt.Errorf("自动解析工作负载容器失败: %w", err)
		}
		target = resolved
	}
	switch normalizeKubernetesLogSource(task.K8SLogSource) {
	case logtailmodel.LogK8SLogSourceStdout:
		return s.applyKubernetesStdoutCollector(ctx, clientset, cluster, task, target)
	case logtailmodel.LogK8SLogSourceNodeFile:
		return s.applyKubernetesNodeFileCollector(ctx, clientset, cluster, task, target)
	case logtailmodel.LogK8SLogSourceContainerFile:
		return s.applyKubernetesContainerFileCollector(ctx, clientset, cluster, task, target)
	case logtailmodel.LogK8SLogSourceBoth:
		stdoutTask := task
		stdoutTask.K8SLogSource = logtailmodel.LogK8SLogSourceStdout
		if err := s.applyKubernetesStdoutCollector(ctx, clientset, cluster, stdoutTask, target); err != nil {
			return err
		}
		fileTask := task
		fileTask.K8SLogSource = logtailmodel.LogK8SLogSourceContainerFile
		return s.applyKubernetesContainerFileCollector(ctx, clientset, cluster, fileTask, target)
	default:
		return fmt.Errorf("不支持的K8S日志来源: %s", task.K8SLogSource)
	}
}

func isKubernetesClusterLevelCollectorMutation(task logtailmodel.LogCollectTask, opType string) bool {
	switch strings.TrimSpace(opType) {
	case logtailmodel.LogK8SCollectorOperationInstall,
		logtailmodel.LogK8SCollectorOperationUpdate,
		logtailmodel.LogK8SCollectorOperationReinstall,
		logtailmodel.LogK8SCollectorOperationUninstall:
	default:
		return false
	}
	switch normalizeKubernetesLogSource(task.K8SLogSource) {
	case logtailmodel.LogK8SLogSourceStdout, logtailmodel.LogK8SLogSourceNodeFile, logtailmodel.LogK8SLogSourceBoth:
		return true
	default:
		return false
	}
}

func (s *LogtailService) ensureKubernetesClusterCollectorIdle(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) error {
	since := time.Now().Add(-k8sCollectorActiveWindow)
	if count, err := s.k8sOpDAO.CountActiveByCluster(clusterID, since); err != nil {
		return fmt.Errorf("检查K8S Collector操作状态失败: %w", err)
	} else if count > 0 {
		return errors.New(k8sClusterCollectorBusyMessage)
	}
	if count, err := s.k8sClusterDAO.CountActiveByCluster(clusterID, since); err != nil {
		return fmt.Errorf("检查K8S集群级Collector操作状态失败: %w", err)
	} else if count > 0 {
		return errors.New(k8sClusterCollectorBusyMessage)
	}
	if clientset == nil {
		return nil
	}
	daemonSet, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询K8S集群级Vector状态失败: %w", err)
	}
	if kubernetesDaemonSetIsRolling(daemonSet) {
		return fmt.Errorf("%s，当前副本: desired=%d ready=%d updated=%d available=%d",
			k8sClusterCollectorBusyMessage,
			daemonSet.Status.DesiredNumberScheduled,
			daemonSet.Status.NumberReady,
			daemonSet.Status.UpdatedNumberScheduled,
			daemonSet.Status.NumberAvailable,
		)
	}
	return nil
}

func kubernetesDaemonSetIsRolling(daemonSet *appsv1.DaemonSet) bool {
	if daemonSet == nil {
		return false
	}
	desired := daemonSet.Status.DesiredNumberScheduled
	if desired <= 0 {
		return false
	}
	return daemonSet.Status.NumberReady != desired ||
		daemonSet.Status.UpdatedNumberScheduled != desired ||
		daemonSet.Status.NumberAvailable != desired
}

func (s *LogtailService) getKubernetesClusterClient(clusterID uint, clusterCache map[uint]*k8smodel.KubeCluster, clientCache map[uint]*kubernetes.Clientset) (*k8smodel.KubeCluster, *kubernetes.Clientset, error) {
	if clusterID == 0 {
		return nil, nil, errors.New("K8S集群ID不能为空")
	}
	if cluster, ok := clusterCache[clusterID]; ok {
		if clientset, exists := clientCache[clusterID]; exists && clientset != nil {
			return cluster, clientset, nil
		}
	}

	cluster := clusterCache[clusterID]
	if cluster == nil {
		var err error
		cluster, err = k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
		if err != nil {
			return nil, nil, fmt.Errorf("查询K8S集群失败 clusterId=%d: %w", clusterID, err)
		}
	}
	if strings.TrimSpace(cluster.Credential) == "" {
		return cluster, nil, fmt.Errorf("K8S集群 %s 未配置 kubeconfig", cluster.Name)
	}
	restConfig, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		return cluster, nil, fmt.Errorf("解析K8S集群 kubeconfig 失败: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return cluster, nil, fmt.Errorf("创建K8S客户端失败: %w", err)
	}
	clusterCache[clusterID] = cluster
	clientCache[clusterID] = clientset
	return cluster, clientset, nil
}

func (s *LogtailService) ListKubernetesCollectors(query logtailmodel.LogK8SCollectorQuery) (*logtailmodel.LogK8SCollectorListResponse, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	tasks, err := s.taskDAO.ListKubernetesCollectors(query)
	if err != nil {
		return nil, err
	}

	clusterCache := map[uint]*k8smodel.KubeCluster{}
	clientCache := map[uint]*kubernetes.Clientset{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	allItems := make([]logtailmodel.LogK8SCollectorVO, 0)
	for i := range tasks {
		task := tasks[i]
		hydrateCollectTaskPresentation(&task)
		targets, err := parseIndexedKubernetesCollectTargetsForTask(task)
		if err != nil {
			continue
		}
		latestOps, _ := s.k8sOpDAO.LatestByTaskTargets(task.ID)
		for _, indexedTarget := range targets {
			if !kubernetesCollectorMatchesQuery(task, indexedTarget.Target, query) {
				continue
			}
			item := s.buildKubernetesCollectorStatusWithCache(ctx, &task, indexedTarget, latestOps[indexedTarget.Index], clusterCache, clientCache)
			if query.Status != "" && item.Status != query.Status {
				continue
			}
			allItems = append(allItems, item)
		}
	}

	summary := summarizeKubernetesCollectors(allItems)
	total := int64(len(allItems))
	start := (query.Page - 1) * query.PageSize
	if start > len(allItems) {
		start = len(allItems)
	}
	end := start + query.PageSize
	if end > len(allItems) {
		end = len(allItems)
	}
	return &logtailmodel.LogK8SCollectorListResponse{
		Total:    total,
		List:     allItems[start:end],
		Summary:  summary,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (s *LogtailService) GetKubernetesCollectorStatus(taskID uint, targetIndex int) (*logtailmodel.LogK8SCollectorVO, error) {
	task, indexedTarget, err := s.getKubernetesTaskTarget(taskID, targetIndex)
	if err != nil {
		return nil, err
	}
	clusterCache := map[uint]*k8smodel.KubeCluster{}
	clientCache := map[uint]*kubernetes.Clientset{}
	latestOps, _ := s.k8sOpDAO.LatestByTaskTargets(task.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	item := s.buildKubernetesCollectorStatusWithCache(ctx, task, indexedTarget, latestOps[indexedTarget.Index], clusterCache, clientCache)
	return &item, nil
}

func (s *LogtailService) ListKubernetesCollectorOperations(taskID uint, query logtailmodel.LogK8SCollectorOperationQuery) ([]logtailmodel.LogK8SCollectorOperation, int64, logtailmodel.LogK8SCollectorOperationQuery, error) {
	if taskID == 0 {
		return nil, 0, query, errors.New("采集任务ID不能为空")
	}
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	query.TaskID = taskID
	items, total, err := s.k8sOpDAO.List(query)
	return items, total, query, err
}

func (s *LogtailService) InstallKubernetesCollector(taskID uint, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	return s.runKubernetesCollectorActionByTaskID(taskID, logtailmodel.LogK8SCollectorOperationInstall, req)
}

func (s *LogtailService) UpdateKubernetesCollector(taskID uint, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	return s.runKubernetesCollectorActionByTaskID(taskID, logtailmodel.LogK8SCollectorOperationUpdate, req)
}

func (s *LogtailService) UninstallKubernetesCollector(taskID uint, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	return s.runKubernetesCollectorActionByTaskID(taskID, logtailmodel.LogK8SCollectorOperationUninstall, req)
}

func (s *LogtailService) ReinstallKubernetesCollector(taskID uint, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	return s.runKubernetesCollectorActionByTaskID(taskID, logtailmodel.LogK8SCollectorOperationReinstall, req)
}

func (s *LogtailService) runKubernetesCollectorActionByTaskID(taskID uint, opType string, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogCollectTaskApplyResponse, error) {
	task, err := s.taskDAO.GetByID(taskID)
	if err != nil {
		return nil, err
	}
	hydrateCollectTaskPresentation(task)
	if !isKubernetesCollectType(task.CollectType) {
		return nil, errors.New("该采集任务不是K8S日志采集任务")
	}
	targets, err := parseIndexedKubernetesCollectTargetsForTask(*task)
	if err != nil {
		return nil, err
	}
	if req.TargetIndex != nil {
		selected := make([]indexedKubernetesCollectTarget, 0, 1)
		for _, target := range targets {
			if target.Index == *req.TargetIndex {
				selected = append(selected, target)
				break
			}
		}
		if len(selected) == 0 {
			return nil, fmt.Errorf("K8S采集目标 targetIndex=%d 不存在", *req.TargetIndex)
		}
		targets = selected
	}
	return s.runKubernetesCollectorAction(*task, targets, opType, req)
}

func (s *LogtailService) createKubernetesCollectorOperation(task logtailmodel.LogCollectTask, indexedTarget indexedKubernetesCollectTarget, opType, operator string, req logtailmodel.LogK8SCollectorActionRequest) (*logtailmodel.LogK8SCollectorOperation, error) {
	requestPayload, _ := json.Marshal(map[string]interface{}{
		"taskId":      task.ID,
		"targetIndex": indexedTarget.Index,
		"opType":      opType,
		"operator":    operator,
		"force":       req.Force,
	})
	item := &logtailmodel.LogK8SCollectorOperation{
		TaskID:       task.ID,
		TargetIndex:  indexedTarget.Index,
		ClusterID:    indexedTarget.Target.ClusterID,
		Namespace:    indexedTarget.Target.Namespace,
		WorkloadType: indexedTarget.Target.WorkloadType,
		WorkloadName: indexedTarget.Target.WorkloadName,
		K8SLogSource: normalizeKubernetesLogSource(task.K8SLogSource),
		OpType:       opType,
		OpStatus:     logtailmodel.LogAgentOperationStatusPending,
		Progress:     0,
		ProgressText: "等待执行",
		RequestJSON:  string(requestPayload),
		Operator:     operator,
	}
	if err := s.k8sOpDAO.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *LogtailService) applyKubernetesStdoutCollector(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	_, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return errors.New("K8S集群级Vector未安装，请先在K8S Collector标签页安装集群级Vector")
	}
	if err != nil {
		return err
	}
	return s.applyKubernetesClusterCollector(ctx, clientset, cluster, "")
}

func (s *LogtailService) applyKubernetesNodeFileCollector(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	if len(collectTaskPathPatterns(&task)) == 0 {
		return errors.New("节点文件日志采集必须填写宿主机日志路径")
	}
	_, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return errors.New("K8S集群级Vector未安装，请先在K8S Collector标签页安装集群级Vector")
	}
	if err != nil {
		return err
	}
	return s.applyKubernetesClusterCollector(ctx, clientset, cluster, "")
}

func (s *LogtailService) applyKubernetesContainerFileCollector(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	configContent, err := s.renderKubernetesContainerFileVectorConfig(cluster, task, target)
	if err != nil {
		return err
	}
	resourceName := kubernetesCollectorName(task, target, "file")
	labels := kubernetesCollectorLabels(task, target, resourceName)
	configHash := shortConfigHash(configContent)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: target.Namespace,
			Labels:    labels,
		},
		Data: map[string]string{k8sVectorConfigFile: configContent},
	}
	if err := applyConfigMap(ctx, clientset, configMap); err != nil {
		return err
	}

	return patchKubernetesWorkloadForSidecar(ctx, clientset, task, target, resourceName, configHash)
}

func (s *LogtailService) uninstallKubernetesCollectorTarget(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	switch normalizeKubernetesLogSource(task.K8SLogSource) {
	case logtailmodel.LogK8SLogSourceStdout:
		return s.applyKubernetesClusterCollector(ctx, clientset, cluster, "")
	case logtailmodel.LogK8SLogSourceNodeFile:
		return s.applyKubernetesClusterCollector(ctx, clientset, cluster, "")
	case logtailmodel.LogK8SLogSourceContainerFile:
		resourceName := kubernetesCollectorName(task, target, "file")
		if err := unpatchKubernetesWorkloadForSidecar(ctx, clientset, task, target); err != nil {
			return err
		}
		if err := deleteConfigMapIfExists(ctx, clientset, target.Namespace, resourceName); err != nil {
			return err
		}
	case logtailmodel.LogK8SLogSourceBoth:
		fileTask := task
		fileTask.K8SLogSource = logtailmodel.LogK8SLogSourceContainerFile
		return s.uninstallKubernetesCollectorTarget(ctx, clientset, cluster, fileTask, target)
	default:
		return fmt.Errorf("不支持的K8S日志来源: %s", task.K8SLogSource)
	}
	return nil
}

func (s *LogtailService) renderKubernetesStdoutVectorConfig(cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) (string, error) {
	store, err := s.storeDAO.GetByID(task.StoreID)
	if err != nil {
		return "", fmt.Errorf("查询日志库失败 task=%s: %w", task.Code, err)
	}
	project, err := s.projectDAO.GetByID(task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("查询项目失败 task=%s: %w", task.Code, err)
	}
	template, _ := s.templateDAO.GetByID(task.ParseTemplateID)

	var builder strings.Builder
	sourceID := vectorComponentID(task, "k8s_stdout_file")
	filterID := vectorComponentID(task, "k8s_stdout_filter")
	parseID := vectorComponentID(task, "k8s_stdout_parse")
	sinkID := vectorComponentID(task, "k8s_stdout_delivery")

	builder.WriteString("data_dir: /var/lib/aiops-vector\n\n")
	builder.WriteString("sources:\n")
	builder.WriteString(fmt.Sprintf("  %s:\n", sourceID))
	builder.WriteString("    type: file\n")
	builder.WriteString("    include:\n")
	for _, pattern := range kubernetesStdoutSourceIncludePatterns([]kubernetesStdoutPipelineTarget{
		{
			Task: task,
			Target: indexedKubernetesCollectTarget{
				Index:  0,
				Target: target,
			},
		},
	}) {
		builder.WriteString(fmt.Sprintf("      - %s\n", yamlQuote(pattern)))
	}
	writeVectorFileRotationSettings(&builder)
	builder.WriteString("\n")

	builder.WriteString("transforms:\n")
	builder.WriteString(fmt.Sprintf("  %s:\n", filterID))
	builder.WriteString("    type: remap\n")
	builder.WriteString("    inputs:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", sourceID))
	builder.WriteString("    source: |\n")
	writeKubernetesStdoutUnpackScript(&builder, target)

	builder.WriteString(fmt.Sprintf("  %s:\n", parseID))
	builder.WriteString("    type: remap\n")
	builder.WriteString("    inputs:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", filterID))
	builder.WriteString("    source: |\n")
	writeVectorParseScript(&builder, task, template)
	writeVectorKubernetesCommonFields(&builder, cluster, task, project, store, target)

	builder.WriteString("\nsinks:\n")
	legacyHTTPSink := shouldUseLegacyHTTPSinkConfig(configuredKubernetesVectorImage())
	if err := writeKubernetesVectorDeliverySink(&builder, sinkID, parseID, legacyHTTPSink); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func (s *LogtailService) renderKubernetesContainerFileVectorConfig(cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) (string, error) {
	store, err := s.storeDAO.GetByID(task.StoreID)
	if err != nil {
		return "", fmt.Errorf("查询日志库失败 task=%s: %w", task.Code, err)
	}
	project, err := s.projectDAO.GetByID(task.ProjectID)
	if err != nil {
		return "", fmt.Errorf("查询项目失败 task=%s: %w", task.Code, err)
	}
	template, _ := s.templateDAO.GetByID(task.ParseTemplateID)

	var builder strings.Builder
	sourceID := vectorComponentID(task, "k8s_container_file")
	parseID := vectorComponentID(task, "k8s_container_file_parse")
	sinkID := vectorComponentID(task, "k8s_container_file_delivery")

	builder.WriteString("data_dir: /var/lib/aiops-vector\n\n")
	builder.WriteString("sources:\n")
	builder.WriteString(fmt.Sprintf("  %s:\n", sourceID))
	builder.WriteString("    type: file\n")
	builder.WriteString("    include:\n")
	for _, pattern := range collectTaskPathPatterns(&task) {
		builder.WriteString(fmt.Sprintf("      - %s\n", yamlQuote(pattern)))
	}
	writeVectorFileRotationSettings(&builder)
	if pattern := effectiveCollectTaskMultilinePattern(task); pattern != "" {
		builder.WriteString("    multiline:\n")
		builder.WriteString(fmt.Sprintf("      start_pattern: %s\n", yamlQuote(pattern)))
		builder.WriteString("      mode: halt_before\n")
		builder.WriteString(fmt.Sprintf("      condition_pattern: %s\n", yamlQuote(pattern)))
		builder.WriteString("      timeout_ms: 5000\n")
	}
	builder.WriteString("\n")

	builder.WriteString("transforms:\n")
	builder.WriteString(fmt.Sprintf("  %s:\n", parseID))
	builder.WriteString("    type: remap\n")
	builder.WriteString("    inputs:\n")
	builder.WriteString(fmt.Sprintf("      - %s\n", sourceID))
	builder.WriteString("    source: |\n")
	writeVectorParseScript(&builder, task, template)
	writeVectorKubernetesCommonFields(&builder, cluster, task, project, store, target)

	builder.WriteString("\nsinks:\n")
	legacyHTTPSink := shouldUseLegacyHTTPSinkConfig(configuredKubernetesVectorImage())
	if err := writeKubernetesVectorDeliverySink(&builder, sinkID, parseID, legacyHTTPSink); err != nil {
		return "", err
	}
	return builder.String(), nil
}

func writeKubernetesStdoutUnpackScript(builder *strings.Builder, target kubernetesCollectTarget) {
	containerFilePattern := kubernetesContainerFileVRLPattern(target)
	podFilePattern := kubernetesPodFileVRLPattern(target)
	builder.WriteString("      file_path = to_string(.file) ?? \"\"\n")
	builder.WriteString("      .file_path = file_path\n")
	writeVectorLogFileNameScript(builder)
	builder.WriteString(fmt.Sprintf("      file_parts, file_err = parse_regex(file_path, r'%s')\n", containerFilePattern))
	builder.WriteString("      if file_err != null || !is_object(file_parts) {\n")
	builder.WriteString(fmt.Sprintf("        file_parts, file_err = parse_regex(file_path, r'%s')\n", podFilePattern))
	builder.WriteString("      }\n")
	builder.WriteString("      if file_err != null || !is_object(file_parts) {\n")
	builder.WriteString("        abort\n")
	builder.WriteString("      }\n")
	builder.WriteString("      . = merge(., file_parts)\n")
	builder.WriteString("      docker_parsed = false\n")
	builder.WriteString("      docker_parts, docker_err = parse_json(.message)\n")
	builder.WriteString("      if docker_err == null && is_object(docker_parts) {\n")
	builder.WriteString("        docker_log = to_string(docker_parts.log) ?? \"\"\n")
	builder.WriteString("        docker_stream = to_string(docker_parts.stream) ?? \"\"\n")
	builder.WriteString("        docker_time = to_string(docker_parts.time) ?? \"\"\n")
	builder.WriteString("        if docker_log != \"\" && (docker_stream == \"stdout\" || docker_stream == \"stderr\") && docker_time != \"\" {\n")
	builder.WriteString("          docker_parsed = true\n")
	builder.WriteString("          .message = docker_log\n")
	builder.WriteString("          .k8s_stream = docker_stream\n")
	builder.WriteString("          .k8s_log_time = docker_time\n")
	builder.WriteString("          parsed_docker_time, parsed_docker_time_err = parse_timestamp(docker_time, \"%+\")\n")
	builder.WriteString("          if parsed_docker_time_err == null {\n")
	builder.WriteString("            .timestamp = parsed_docker_time\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if !docker_parsed {\n")
	builder.WriteString("        cri_parts, cri_err = parse_regex(.message, r'^(?P<k8s_log_time>\\S+)\\s+(?P<k8s_stream>stdout|stderr)\\s+(?P<k8s_log_flag>\\S+)\\s(?P<k8s_message>.*)$')\n")
	builder.WriteString("        if cri_err == null && is_object(cri_parts) {\n")
	builder.WriteString("          . = merge(., cri_parts)\n")
	builder.WriteString("          .message = .k8s_message\n")
	builder.WriteString("          parsed_log_time, parsed_log_time_err = parse_timestamp(.k8s_log_time, \"%+\")\n")
	builder.WriteString("          if parsed_log_time_err == null {\n")
	builder.WriteString("            .timestamp = parsed_log_time\n")
	builder.WriteString("          }\n")
	builder.WriteString("        }\n")
	builder.WriteString("      }\n")
}

func writeKubernetesNodeFileUnpackScript(builder *strings.Builder) {
	builder.WriteString("      file_path = to_string(.file) ?? \"\"\n")
	builder.WriteString("      .file_path = file_path\n")
	writeVectorLogFileNameScript(builder)
	builder.WriteString("      app_parts, app_err = parse_regex(file_path, r'^/data/logs/tomcatlogs/(?P<app_name>[^/]+)/')\n")
	builder.WriteString("      if app_err != null || !is_object(app_parts) {\n")
	builder.WriteString("        app_parts, app_err = parse_regex(file_path, r'^/data/logs/(?P<app_name>[^/]+)/')\n")
	builder.WriteString("      }\n")
	builder.WriteString("      if app_err == null && is_object(app_parts) {\n")
	builder.WriteString("        . = merge(., app_parts)\n")
	builder.WriteString("        .app = .app_name\n")
	builder.WriteString("        .service = .app_name\n")
	builder.WriteString("      }\n")
}

func kubernetesStdoutSourceIncludePatterns(targets []kubernetesStdoutPipelineTarget) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(targets))
	for _, item := range targets {
		target := item.Target.Target
		workload := strings.TrimSpace(target.WorkloadName)
		namespace := strings.TrimSpace(target.Namespace)
		if workload == "" || namespace == "" {
			continue
		}
		containers := target.Containers
		if len(containers) == 0 {
			containers = []string{"*"}
		}
		for _, container := range containers {
			container = strings.TrimSpace(container)
			if container == "" {
				container = "*"
			}
			pattern := fmt.Sprintf("/var/log/containers/%s-*_%s_%s-*.log", workload, namespace, container)
			if _, ok := seen[pattern]; ok {
				continue
			}
			seen[pattern] = struct{}{}
			result = append(result, pattern)
		}
	}
	sort.Strings(result)
	if len(result) == 0 {
		return []string{"/var/log/containers/.aiops-empty-*.log"}
	}
	return result
}

func writeVectorKubernetesCommonFields(builder *strings.Builder, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, project *logtailmodel.LogProject, store *logtailmodel.LogStore, target kubernetesCollectTarget) {
	source := normalizeKubernetesLogSource(task.K8SLogSource)
	fields := map[string]string{
		"project_code":      project.Code,
		"store_code":        store.Code,
		"collect_task_name": task.Name,
		"collect_task_code": task.Code,
		"env":               project.Env,
		"host_name":         cluster.Name,
		"log_source":        "k8s_container",
		"k8s_log_source":    source,
		"k8s_cluster_id":    strconv.FormatUint(uint64(target.ClusterID), 10),
		"k8s_cluster_name":  cluster.Name,
		"k8s_namespace":     target.Namespace,
		"k8s_workload_type": target.WorkloadType,
		"k8s_workload_name": target.WorkloadName,
		"log_type":          firstNonEmpty(displayCollectTaskParseMode(task.ParseModeSnapshot), store.StoreType, "app_log"),
	}
	if source == logtailmodel.LogK8SLogSourceNodeFile {
		fields["log_source"] = "k8s_node_file"
		delete(fields, "k8s_namespace")
		delete(fields, "k8s_workload_type")
		delete(fields, "k8s_workload_name")
	} else {
		fields["app"] = firstNonEmpty(store.Code, task.Code)
		fields["service"] = target.WorkloadName
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
	if source == logtailmodel.LogK8SLogSourceNodeFile {
		builder.WriteString("      if !exists(.app) || .app == \"\" {\n")
		builder.WriteString(fmt.Sprintf("        .app = %s\n", strconv.Quote(firstNonEmpty(store.Code, task.Code))))
		builder.WriteString("      }\n")
		builder.WriteString("      if !exists(.service) || .service == \"\" {\n")
		builder.WriteString("        .service = .app\n")
		builder.WriteString("      }\n")
	}
	builder.WriteString("      if exists(.file) {\n")
	builder.WriteString("        .file_path = .file\n")
	builder.WriteString("      }\n")
	writeVectorLogFileNameScript(builder)
}

func writeKubernetesVectorDeliverySink(builder *strings.Builder, sinkID, inputID string, legacyHTTPSink bool) error {
	return writeVectorDeliverySink(builder, sinkID, inputID, legacyHTTPSink)
}

func buildKubernetesStdoutDaemonSet(name string, labels map[string]string, configHash, vectorImage string, configRefs ...kubernetesVectorConfigRef) *appsv1.DaemonSet {
	hostPathType := corev1.HostPathDirectory
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	readOnly := true
	privileged := false
	if strings.TrimSpace(vectorImage) == "" {
		vectorImage = configuredKubernetesVectorImage()
	}
	if len(configRefs) == 0 {
		configRefs = []kubernetesVectorConfigRef{{ConfigMap: k8sVectorBaseConfigMapName, FileName: k8sVectorBaseConfigFile}}
	}
	configProjections := make([]corev1.VolumeProjection, 0, len(configRefs))
	for _, ref := range configRefs {
		configProjections = append(configProjections, corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{Name: ref.ConfigMap},
				Items:                []corev1.KeyToPath{{Key: k8sVectorConfigFile, Path: ref.FileName}},
			},
		})
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k8sVectorNamespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: func() *intstr.IntOrString {
						value := intstr.FromString("25%")
						return &value
					}(),
					MaxSurge: func() *intstr.IntOrString {
						value := intstr.FromInt(0)
						return &value
					}(),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeStringMaps(labels, map[string]string{"app": name}),
					Annotations: map[string]string{
						"aiops.dev/config-hash": configHash,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: name,
					HostAliases:        configuredKafkaHostAliases(),
					Containers: []corev1.Container{
						{
							Name:            "vector",
							Image:           vectorImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            []string{"--config-dir", "/etc/vector"},
							Resources:       defaultKubernetesVectorResources(),
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "data-dir", MountPath: "/var/lib/aiops-vector"},
								{Name: "config", MountPath: "/etc/vector", ReadOnly: true},
								{Name: "var-log-containers", MountPath: "/var/log/containers", ReadOnly: readOnly},
								{Name: "var-log-pods", MountPath: "/var/log/pods", ReadOnly: readOnly},
								{Name: "docker-containers", MountPath: "/var/lib/docker/containers", ReadOnly: readOnly},
								{Name: "data-logs", MountPath: "/data/logs", ReadOnly: readOnly},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "data-dir",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: k8sVectorHostDataDir, Type: &hostPathDirectoryOrCreate},
							},
						},
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								Projected: &corev1.ProjectedVolumeSource{Sources: configProjections},
							},
						},
						{
							Name: "var-log-containers",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: "/var/log/containers", Type: &hostPathType},
							},
						},
						{
							Name: "var-log-pods",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: "/var/log/pods", Type: &hostPathType},
							},
						},
						{
							Name: "docker-containers",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/docker/containers", Type: &hostPathDirectoryOrCreate},
							},
						},
						{
							Name: "data-logs",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{Path: "/data/logs", Type: &hostPathDirectoryOrCreate},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists},
					},
				},
			},
		},
	}
}

func defaultKubernetesVectorResources() corev1.ResourceRequirements {
	cpu := resource.MustParse("1")
	memory := resource.MustParse("2Gi")
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    cpu,
			corev1.ResourceMemory: memory,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    cpu,
			corev1.ResourceMemory: memory,
		},
	}
}

func (s *LogtailService) buildKubernetesCollectorStatusWithCache(ctx context.Context, task *logtailmodel.LogCollectTask, indexedTarget indexedKubernetesCollectTarget, latestOp logtailmodel.LogK8SCollectorOperation, clusterCache map[uint]*k8smodel.KubeCluster, clientCache map[uint]*kubernetes.Clientset) logtailmodel.LogK8SCollectorVO {
	target := indexedTarget.Target
	cluster, clientset, err := s.getKubernetesClusterClient(target.ClusterID, clusterCache, clientCache)
	if err != nil {
		return baseKubernetesCollectorVO(task, nil, indexedTarget, latestOp, "failed", -1, "连接K8S集群失败: "+err.Error())
	}
	item, statusErr := s.buildKubernetesCollectorStatus(ctx, clientset, cluster, *task, indexedTarget)
	if statusErr != nil {
		item = baseKubernetesCollectorVO(task, cluster, indexedTarget, latestOp, "failed", -1, statusErr.Error())
	}
	latestOp = s.reconcileKubernetesCollectorOperation(*task, latestOp, item)
	applyActiveKubernetesCollectorOperation(&item, latestOp)
	return item
}

func (s *LogtailService) reconcileKubernetesCollectorOperation(task logtailmodel.LogCollectTask, latestOp logtailmodel.LogK8SCollectorOperation, item logtailmodel.LogK8SCollectorVO) logtailmodel.LogK8SCollectorOperation {
	if latestOp.ID == 0 || !isActiveOperation(latestOp.OpStatus) {
		return latestOp
	}
	status := strings.TrimSpace(item.Status)
	if status != "pending" && status != "running" && status != "failed" && status != "not_installed" {
		return latestOp
	}
	if status == "not_installed" && latestOp.OpType != logtailmodel.LogK8SCollectorOperationUninstall {
		return latestOp
	}
	if status == "running" && latestOp.OpType == logtailmodel.LogK8SCollectorOperationUninstall {
		return latestOp
	}

	resultItem := item
	resultItem.LastOperation = nil
	resultJSON := ""
	if payload, err := json.Marshal(resultItem); err == nil {
		resultJSON = string(payload)
	}
	if status == "pending" {
		updated, err := s.k8sOpDAO.UpdateProgress(
			latestOp.ID,
			logtailmodel.LogAgentOperationStatusRunning,
			item.Progress,
			item.ProgressText,
			resultJSON,
			item.LastError,
		)
		if err != nil || updated == nil {
			return latestOp
		}
		return *updated
	}

	opStatus := logtailmodel.LogAgentOperationStatusSuccess
	progress := 100
	progressText := kubernetesOperationSuccessMessage(task, latestOp.OpType)
	errorMsg := ""
	if status == "failed" {
		opStatus = logtailmodel.LogAgentOperationStatusFailed
		progress = -1
		progressText = firstNonEmpty(item.LastError, item.ProgressText, "K8S Collector运行失败")
		errorMsg = progressText
	}
	updated, err := s.k8sOpDAO.UpdateProgress(latestOp.ID, opStatus, progress, progressText, resultJSON, errorMsg)
	if err != nil || updated == nil {
		return latestOp
	}
	return *updated
}

func applyActiveKubernetesCollectorOperation(item *logtailmodel.LogK8SCollectorVO, latestOp logtailmodel.LogK8SCollectorOperation) {
	if item == nil || latestOp.ID == 0 {
		return
	}
	item.LastOperation = &latestOp
	if !isActiveOperation(latestOp.OpStatus) {
		return
	}
	// Pending runtime data was read from Kubernetes in this request and is more
	// current than the operation snapshot persisted by the background rollout.
	if item.Status == "pending" {
		return
	}
	item.Status = "pending"
	item.Progress = latestOp.Progress
	item.ProgressText = latestOp.ProgressText
}

func (s *LogtailService) buildKubernetesCollectorStatus(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, indexedTarget indexedKubernetesCollectTarget) (logtailmodel.LogK8SCollectorVO, error) {
	target := indexedTarget.Target
	source := normalizeKubernetesLogSource(task.K8SLogSource)
	if source != logtailmodel.LogK8SLogSourceNodeFile {
		resolved, err := resolveKubernetesWorkloadContainers(ctx, clientset, target)
		if err != nil {
			return baseKubernetesCollectorVO(&task, cluster, indexedTarget, logtailmodel.LogK8SCollectorOperation{}, "failed", -1, "自动解析工作负载容器失败"), err
		}
		target = resolved
		indexedTarget.Target = resolved
	}
	item := baseKubernetesCollectorVO(&task, cluster, indexedTarget, logtailmodel.LogK8SCollectorOperation{}, "not_installed", 0, "未安装")
	if task.Status == logtailmodel.LogStatusDisabled {
		fillKubernetesCollectorConfig(ctx, clientset, &item)
		return item, nil
	}
	if source == logtailmodel.LogK8SLogSourceBoth {
		return s.buildKubernetesCombinedCollectorStatus(ctx, clientset, cluster, task, indexedTarget)
	}

	switch source {
	case logtailmodel.LogK8SLogSourceStdout, logtailmodel.LogK8SLogSourceNodeFile:
		resourceName := k8sClusterCollectorName
		item.ResourceKind = "DaemonSet"
		item.ResourceNamespace = k8sVectorNamespace
		item.ResourceName = resourceName
		daemonSet, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, resourceName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			item.Events = listKubernetesWarningEvents(ctx, clientset, k8sVectorNamespace, resourceName)
			return item, nil
		}
		if err != nil {
			return item, fmt.Errorf("查询K8S Collector DaemonSet失败: %w", err)
		}
		item.Desired = daemonSet.Status.DesiredNumberScheduled
		item.Ready = daemonSet.Status.NumberReady
		item.Updated = daemonSet.Status.UpdatedNumberScheduled
		item.Available = daemonSet.Status.NumberAvailable
		if daemonSet.Spec.Template.Annotations != nil {
			item.ConfigHash = daemonSet.Spec.Template.Annotations["aiops.dev/config-hash"]
		}
		item.Pods = listKubernetesPods(ctx, clientset, k8sVectorNamespace, labels.SelectorFromSet(labels.Set{"app": resourceName}).String())
		item.Events = listKubernetesWarningEventsForPods(ctx, clientset, k8sVectorNamespace, item.Pods)
		fillKubernetesCollectorConfig(ctx, clientset, &item)
		if !kubernetesClusterConfigIncludesTaskPipeline(task, indexedTarget.Index, item.ConfigYAML) {
			item.Status = "not_installed"
			item.Progress = 0
			item.ProgressText = "任务采集配置未下发"
			item.VectorStatus = logtailmodel.VectorStatusNotInstalled
			item.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
			item.Desired = 0
			item.Ready = 0
			item.Updated = 0
			item.Available = 0
			item.Pods = nil
			return item, nil
		}
	case logtailmodel.LogK8SLogSourceContainerFile:
		resourceName := kubernetesCollectorName(task, target, "file")
		item.ResourceKind = "Sidecar"
		item.ResourceNamespace = target.Namespace
		item.ResourceName = resourceName
		desired, ready, updated, available, selector, sidecarInjected, err := getKubernetesWorkloadStatus(ctx, clientset, task, target)
		if err != nil {
			if apierrors.IsNotFound(err) {
				item.ProgressText = "目标工作负载不存在"
				item.LastError = "目标工作负载不存在"
				return item, nil
			}
			return item, err
		}
		if !sidecarInjected {
			item.Events = listKubernetesWarningEvents(ctx, clientset, target.Namespace, target.WorkloadName)
			return item, nil
		}
		item.Desired = desired
		item.Ready = ready
		item.Updated = updated
		item.Available = available
		item.Pods = listKubernetesPods(ctx, clientset, target.Namespace, selector)
		item.Events = listKubernetesWarningEvents(ctx, clientset, target.Namespace, target.WorkloadName)
		fillKubernetesCollectorConfig(ctx, clientset, &item)
	default:
		return item, fmt.Errorf("不支持的K8S日志来源: %s", task.K8SLogSource)
	}

	item.Status, item.Progress, item.ProgressText = determineKubernetesCollectorProgress(item.Desired, item.Ready, item.Updated, item.Available, firstPodError(item.Pods))
	item.VectorStatus = item.Status
	if item.Status == "running" {
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusApplied
	} else if item.Status == "failed" {
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
	} else {
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
	}
	if errMsg := firstPodError(item.Pods); errMsg != "" {
		item.LastError = errMsg
		now := time.Now()
		item.LastErrorAt = &now
	}
	return item, nil
}

func resolveKubernetesWorkloadContainers(ctx context.Context, clientset kubernetes.Interface, target kubernetesCollectTarget) (kubernetesCollectTarget, error) {
	if clientset == nil {
		return target, errors.New("K8S客户端不能为空")
	}
	var podSpec *corev1.PodSpec
	switch normalizeKubernetesWorkloadType(target.WorkloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return target, fmt.Errorf("查询Deployment失败: %w", err)
		}
		podSpec = &item.Spec.Template.Spec
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return target, fmt.Errorf("查询StatefulSet失败: %w", err)
		}
		podSpec = &item.Spec.Template.Spec
	case "daemonset":
		item, err := clientset.AppsV1().DaemonSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return target, fmt.Errorf("查询DaemonSet失败: %w", err)
		}
		podSpec = &item.Spec.Template.Spec
	default:
		return target, fmt.Errorf("暂不支持的K8S工作负载类型: %s", target.WorkloadType)
	}

	target.Containers = target.Containers[:0]
	for _, container := range podSpec.Containers {
		name := strings.TrimSpace(container.Name)
		if name == "" || strings.HasPrefix(name, "aiops-vector-") {
			continue
		}
		target.Containers = append(target.Containers, name)
	}
	if len(target.Containers) == 0 {
		return target, errors.New("工作负载没有可采集的业务容器")
	}
	sort.Strings(target.Containers)
	return target, nil
}

func (s *LogtailService) buildKubernetesCombinedCollectorStatus(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, task logtailmodel.LogCollectTask, indexedTarget indexedKubernetesCollectTarget) (logtailmodel.LogK8SCollectorVO, error) {
	stdoutTask := task
	stdoutTask.K8SLogSource = logtailmodel.LogK8SLogSourceStdout
	stdoutItem, stdoutErr := s.buildKubernetesCollectorStatus(ctx, clientset, cluster, stdoutTask, indexedTarget)

	fileTask := task
	fileTask.K8SLogSource = logtailmodel.LogK8SLogSourceContainerFile
	fileItem, fileErr := s.buildKubernetesCollectorStatus(ctx, clientset, cluster, fileTask, indexedTarget)

	return mergeKubernetesCombinedCollectorStatus(task, cluster, indexedTarget, stdoutItem, fileItem, stdoutErr, fileErr), nil
}

func mergeKubernetesCombinedCollectorStatus(task logtailmodel.LogCollectTask, cluster *k8smodel.KubeCluster, indexedTarget indexedKubernetesCollectTarget, stdoutItem, fileItem logtailmodel.LogK8SCollectorVO, stdoutErr, fileErr error) logtailmodel.LogK8SCollectorVO {
	item := baseKubernetesCollectorVO(&task, cluster, indexedTarget, logtailmodel.LogK8SCollectorOperation{}, "not_installed", 0, "未安装")
	item.ResourceKind = "DaemonSet+Sidecar"
	item.ResourceNamespace = fmt.Sprintf("%s + %s", stdoutItem.ResourceNamespace, fileItem.ResourceNamespace)
	item.ResourceName = fmt.Sprintf("%s + %s", stdoutItem.ResourceName, fileItem.ResourceName)
	item.ConfigMapNamespace = fmt.Sprintf("%s + %s", stdoutItem.ConfigMapNamespace, fileItem.ConfigMapNamespace)
	item.ConfigMapName = fmt.Sprintf("%s + %s", stdoutItem.ConfigMapName, fileItem.ConfigMapName)
	// The combined row is primarily about the running collector. When stdout is
	// enabled, the collector is the cluster DaemonSet; container_file sidecar
	// status is exposed separately in Components to avoid adding 6 + 1 = 7.
	item.Desired = stdoutItem.Desired
	item.Ready = stdoutItem.Ready
	item.Updated = stdoutItem.Updated
	item.Available = stdoutItem.Available
	item.Pods = append(item.Pods, stdoutItem.Pods...)
	item.Events = append(item.Events, stdoutItem.Events...)
	item.Events = append(item.Events, fileItem.Events...)
	item.ConfigHash = strings.Trim(strings.Join([]string{stdoutItem.ConfigHash, fileItem.ConfigHash}, "+"), "+")
	item.ConfigYAML = mergeKubernetesCollectorConfigYAML(stdoutItem.ConfigYAML, fileItem.ConfigYAML)
	item.Components = []logtailmodel.LogK8SCollectorComponent{
		kubernetesCollectorComponent(logtailmodel.LogK8SLogSourceStdout, "标准输出 / 集群级 Vector", stdoutItem),
		kubernetesCollectorComponent(logtailmodel.LogK8SLogSourceContainerFile, "容器内文件 / Sidecar", fileItem),
	}

	errMsg := firstNonEmpty(firstPodError(item.Pods), stdoutItem.LastError, fileItem.LastError)
	if stdoutErr != nil {
		errMsg = firstNonEmpty(errMsg, stdoutErr.Error())
	}
	if fileErr != nil {
		errMsg = firstNonEmpty(errMsg, fileErr.Error())
	}
	item.Status, item.Progress, item.ProgressText = determineKubernetesCollectorProgress(item.Desired, item.Ready, item.Updated, item.Available, errMsg)
	item.VectorStatus = item.Status
	switch item.Status {
	case "running":
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusApplied
	case "failed":
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
	default:
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
	}
	if errMsg != "" {
		item.LastError = errMsg
		now := time.Now()
		item.LastErrorAt = &now
	}
	return item
}

func kubernetesCollectorComponent(source, name string, item logtailmodel.LogK8SCollectorVO) logtailmodel.LogK8SCollectorComponent {
	readyPods := 0
	for _, pod := range item.Pods {
		if pod.Ready {
			readyPods++
		}
	}
	return logtailmodel.LogK8SCollectorComponent{
		Source:             source,
		Name:               name,
		ResourceKind:       item.ResourceKind,
		ResourceNamespace:  item.ResourceNamespace,
		ResourceName:       item.ResourceName,
		ConfigMapNamespace: item.ConfigMapNamespace,
		ConfigMapName:      item.ConfigMapName,
		Status:             item.Status,
		Progress:           item.Progress,
		ProgressText:       item.ProgressText,
		Desired:            item.Desired,
		Ready:              item.Ready,
		Updated:            item.Updated,
		Available:          item.Available,
		ConfigStatus:       item.ConfigStatus,
		VectorStatus:       item.VectorStatus,
		PodCount:           len(item.Pods),
		ReadyPodCount:      readyPods,
		LastError:          item.LastError,
	}
}

func mergeKubernetesCollectorConfigYAML(stdoutConfig, fileConfig string) string {
	stdoutConfig = strings.TrimSpace(stdoutConfig)
	fileConfig = strings.TrimSpace(fileConfig)
	switch {
	case stdoutConfig == "":
		return fileConfig
	case fileConfig == "":
		return stdoutConfig
	default:
		return "# stdout collector\n" + stdoutConfig + "\n\n# container_file collector\n" + fileConfig + "\n"
	}
}

func fillKubernetesCollectorConfig(ctx context.Context, clientset kubernetes.Interface, item *logtailmodel.LogK8SCollectorVO) {
	if item == nil {
		return
	}
	namespace := strings.TrimSpace(item.ConfigMapNamespace)
	name := strings.TrimSpace(item.ConfigMapName)
	if namespace == "" {
		namespace = strings.TrimSpace(item.ResourceNamespace)
	}
	if name == "" {
		name = strings.TrimSpace(item.ResourceName)
	}
	if namespace == "" || name == "" {
		return
	}
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && namespace == k8sVectorNamespace && item.ProjectID > 0 {
		selector := labels.SelectorFromSet(labels.Set{
			k8sVectorConfigScopeLabel:     k8sVectorConfigScopeProject,
			k8sVectorConfigProjectIDLabel: strconv.FormatUint(uint64(item.ProjectID), 10),
		}).String()
		if configMaps, listErr := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); listErr == nil && len(configMaps.Items) > 0 {
			selected := &configMaps.Items[0]
			legacyName := kubernetesLegacyProjectConfigMapName(item.ProjectID)
			for index := range configMaps.Items {
				if configMaps.Items[index].Name != legacyName {
					selected = &configMaps.Items[index]
					break
				}
			}
			configMap = selected
			err = nil
		}
	}
	if apierrors.IsNotFound(err) && namespace == k8sVectorNamespace && item.ProjectID > 0 {
		// Collectors before project-level ConfigMaps stored every project in aiops-vector.
		configMap, err = clientset.CoreV1().ConfigMaps(namespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	}
	if err != nil || configMap == nil || configMap.Data == nil {
		return
	}
	item.ConfigMapNamespace = configMap.Namespace
	item.ConfigMapName = configMap.Name
	item.ConfigYAML = configMap.Data[k8sVectorConfigFile]
	if item.ConfigHash == "" && item.ConfigYAML != "" {
		item.ConfigHash = shortConfigHash(item.ConfigYAML)
	}
}

func kubernetesClusterConfigIncludesTaskPipeline(task logtailmodel.LogCollectTask, targetIndex int, configYAML string) bool {
	if strings.TrimSpace(configYAML) == "" {
		return false
	}
	switch normalizeKubernetesLogSource(task.K8SLogSource) {
	case logtailmodel.LogK8SLogSourceStdout:
		return strings.Contains(configYAML, vectorComponentID(task, fmt.Sprintf("k8s_stdout_parse_%d", targetIndex)))
	case logtailmodel.LogK8SLogSourceNodeFile:
		return strings.Contains(configYAML, vectorComponentID(task, fmt.Sprintf("k8s_node_file_%d", targetIndex)))
	default:
		return true
	}
}

func baseKubernetesCollectorVO(task *logtailmodel.LogCollectTask, cluster *k8smodel.KubeCluster, indexedTarget indexedKubernetesCollectTarget, latestOp logtailmodel.LogK8SCollectorOperation, status string, progress int, progressText string) logtailmodel.LogK8SCollectorVO {
	target := indexedTarget.Target
	clusterName := ""
	if cluster != nil {
		clusterName = cluster.Name
	}
	source := normalizeKubernetesLogSource(task.K8SLogSource)
	resourceNamespace := k8sVectorNamespace
	resourceKind := "DaemonSet"
	resourceName := k8sClusterCollectorName
	configMapNamespace := resourceNamespace
	configMapName := resourceName
	if source == logtailmodel.LogK8SLogSourceContainerFile {
		resourceNamespace = target.Namespace
		resourceKind = "Sidecar"
		resourceName = kubernetesCollectorName(*task, target, "file")
		configMapNamespace = resourceNamespace
		configMapName = resourceName
	} else {
		configMapName = kubernetesLegacyProjectConfigMapName(task.ProjectID)
	}
	item := logtailmodel.LogK8SCollectorVO{
		TaskID:             task.ID,
		TaskName:           task.Name,
		TaskCode:           task.Code,
		TaskStatus:         task.Status,
		ProjectID:          task.ProjectID,
		StoreID:            task.StoreID,
		TargetIndex:        indexedTarget.Index,
		ClusterID:          target.ClusterID,
		ClusterName:        clusterName,
		Namespace:          target.Namespace,
		WorkloadType:       target.WorkloadType,
		WorkloadName:       target.WorkloadName,
		Containers:         target.Containers,
		K8SLogSource:       source,
		ResourceKind:       resourceKind,
		ResourceNamespace:  resourceNamespace,
		ResourceName:       resourceName,
		ConfigMapNamespace: configMapNamespace,
		ConfigMapName:      configMapName,
		Status:             status,
		Progress:           progress,
		ProgressText:       progressText,
		ConfigStatus:       logtailmodel.LogAgentConfigStatusPending,
		VectorStatus:       status,
	}
	if task.Status == logtailmodel.LogStatusDisabled {
		item.Status = "not_installed"
		item.Progress = 0
		item.ProgressText = "任务已停止采集"
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
		item.VectorStatus = logtailmodel.VectorStatusNotInstalled
	}
	if latestOp.ID > 0 {
		item.LastOperation = &latestOp
	}
	return item
}

func getKubernetesWorkloadStatus(ctx context.Context, clientset *kubernetes.Clientset, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) (int32, int32, int32, int32, string, bool, error) {
	sidecarName := kubernetesSidecarName(task)
	switch normalizeKubernetesWorkloadType(target.WorkloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return 0, 0, 0, 0, "", false, err
		}
		desired := int32(1)
		if item.Spec.Replicas != nil {
			desired = *item.Spec.Replicas
		}
		selector := labels.SelectorFromSet(labels.Set(item.Spec.Selector.MatchLabels)).String()
		return desired, item.Status.ReadyReplicas, item.Status.UpdatedReplicas, item.Status.AvailableReplicas, selector, podTemplateHasContainer(item.Spec.Template, sidecarName), nil
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return 0, 0, 0, 0, "", false, err
		}
		desired := int32(1)
		if item.Spec.Replicas != nil {
			desired = *item.Spec.Replicas
		}
		selector := labels.SelectorFromSet(labels.Set(item.Spec.Selector.MatchLabels)).String()
		return desired, item.Status.ReadyReplicas, item.Status.UpdatedReplicas, item.Status.AvailableReplicas, selector, podTemplateHasContainer(item.Spec.Template, sidecarName), nil
	case "daemonset":
		item, err := clientset.AppsV1().DaemonSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return 0, 0, 0, 0, "", false, err
		}
		selector := labels.SelectorFromSet(labels.Set(item.Spec.Selector.MatchLabels)).String()
		return item.Status.DesiredNumberScheduled, item.Status.NumberReady, item.Status.UpdatedNumberScheduled, item.Status.NumberAvailable, selector, podTemplateHasContainer(item.Spec.Template, sidecarName), nil
	default:
		return 0, 0, 0, 0, "", false, fmt.Errorf("暂不支持的K8S工作负载类型: %s", target.WorkloadType)
	}
}

func listKubernetesPods(ctx context.Context, clientset *kubernetes.Clientset, namespace, selector string) []logtailmodel.LogK8SCollectorPod {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil
	}
	items := make([]logtailmodel.LogK8SCollectorPod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		ready := true
		var restartSum int32
		message := ""
		logContainer := ""
		for _, status := range pod.Status.ContainerStatuses {
			restartSum += status.RestartCount
			if !status.Ready {
				ready = false
			}
			if message == "" {
				message = describeKubernetesContainerStatus(status)
				if message != "" {
					logContainer = status.Name
				}
			}
		}
		if message == "" {
			message = describeKubernetesPodSchedulingStatus(pod)
		}
		lastLog := ""
		if shouldFetchKubernetesPodLog(message) || shouldInspectKubernetesVectorPodLog(namespace, pod) {
			if logContainer == "" {
				logContainer = kubernetesVectorPodLogContainer(pod)
			}
			lastLog = fetchKubernetesPodLogTail(ctx, clientset, pod.Namespace, pod.Name, logContainer, restartSum > 0 && !ready)
		}
		if diagnostic := diagnoseKubernetesVectorLog(lastLog); diagnostic != "" {
			message = mergeKubernetesDiagnosticMessage(message, diagnostic)
		}
		items = append(items, logtailmodel.LogK8SCollectorPod{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			NodeName:   pod.Spec.NodeName,
			Phase:      string(pod.Status.Phase),
			Ready:      ready && pod.Status.Phase == corev1.PodRunning,
			RestartSum: restartSum,
			Message:    message,
			LastLog:    lastLog,
		})
	}
	return items
}

func shouldInspectKubernetesVectorPodLog(namespace string, pod corev1.Pod) bool {
	if namespace != k8sVectorNamespace {
		return false
	}
	return pod.Labels["app"] == k8sClusterCollectorName
}

func kubernetesVectorPodLogContainer(pod corev1.Pod) string {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "vector" {
			return status.Name
		}
	}
	if len(pod.Status.ContainerStatuses) > 0 {
		return pod.Status.ContainerStatuses[0].Name
	}
	return ""
}

func mergeKubernetesDiagnosticMessage(message, diagnostic string) string {
	message = strings.TrimSpace(message)
	diagnostic = strings.TrimSpace(diagnostic)
	if diagnostic == "" {
		return message
	}
	if message == "" || isKubernetesPodWaitingReason(message) {
		return diagnostic
	}
	if strings.Contains(message, diagnostic) {
		return message
	}
	return message + "；" + diagnostic
}

func describeKubernetesPodSchedulingStatus(pod corev1.Pod) string {
	for _, condition := range pod.Status.Conditions {
		if condition.Type != corev1.PodScheduled || condition.Status != corev1.ConditionFalse {
			continue
		}
		reason := firstNonEmpty(condition.Reason, "Unschedulable")
		if text := strings.TrimSpace(condition.Message); text != "" {
			return reason + ": " + text
		}
		return reason
	}
	if pod.Status.Phase == corev1.PodPending && strings.TrimSpace(pod.Spec.NodeName) == "" {
		return "Pending: 等待调度"
	}
	return ""
}

func describeKubernetesContainerStatus(status corev1.ContainerStatus) string {
	if status.Ready && status.State.Running != nil {
		return ""
	}
	if status.State.Waiting != nil {
		reason := firstNonEmpty(status.State.Waiting.Reason, status.State.Waiting.Message)
		if status.LastTerminationState.Terminated != nil {
			previous := formatKubernetesPreviousTermination(status.LastTerminationState.Terminated)
			if reason != "" && previous != "" {
				return reason + ": previous " + previous
			}
			if previous != "" {
				return previous
			}
		}
		return reason
	}
	if status.State.Terminated != nil {
		return formatKubernetesTermination(status.State.Terminated)
	}
	if status.LastTerminationState.Terminated != nil && status.RestartCount > 0 {
		previous := formatKubernetesPreviousTermination(status.LastTerminationState.Terminated)
		if previous != "" {
			return "previous " + previous
		}
	}
	return ""
}

func formatKubernetesPreviousTermination(item *corev1.ContainerStateTerminated) string {
	if item == nil || item.ExitCode == 0 {
		return ""
	}
	return formatKubernetesTermination(item)
}

func formatKubernetesTermination(item *corev1.ContainerStateTerminated) string {
	if item == nil {
		return ""
	}
	reason := firstNonEmpty(item.Reason, "Terminated")
	details := []string{fmt.Sprintf("exitCode=%d", item.ExitCode)}
	if item.Signal > 0 {
		details = append(details, fmt.Sprintf("signal=%d", item.Signal))
	}
	message := reason + "(" + strings.Join(details, ",") + ")"
	if text := strings.TrimSpace(item.Message); text != "" {
		message += ": " + text
	}
	if item.ExitCode == 78 {
		message += ": Vector配置校验失败或启动参数错误，请查看配置预览和Pod日志"
	}
	return message
}

func shouldFetchKubernetesPodLog(message string) bool {
	message = strings.TrimSpace(message)
	if message == "" || isKubernetesPodWaitingReason(message) {
		return false
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "exitcode=") ||
		strings.Contains(lower, "crashloopbackoff") ||
		strings.Contains(lower, "error")
}

func fetchKubernetesPodLogTail(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string, previous bool) string {
	if strings.TrimSpace(namespace) == "" || strings.TrimSpace(podName) == "" {
		return ""
	}
	tailLines := int64(80)
	limitBytes := int64(16 * 1024)
	options := &corev1.PodLogOptions{
		Container:  containerName,
		Previous:   previous,
		TailLines:  &tailLines,
		LimitBytes: &limitBytes,
	}
	data, err := readKubernetesPodLog(ctx, clientset, namespace, podName, options)
	if err != nil && previous {
		options.Previous = false
		data, err = readKubernetesPodLog(ctx, clientset, namespace, podName, options)
	}
	if err != nil {
		return ""
	}
	return strings.TrimSpace(data)
}

func diagnoseKubernetesVectorLog(logText string) string {
	logText = strings.TrimSpace(logText)
	if logText == "" {
		return ""
	}
	lower := strings.ToLower(logText)
	switch {
	case isKubernetesVectorKafkaLog(lower) && strings.Contains(lower, "localhost:9092") && strings.Contains(lower, "connection refused"):
		return "写入Kafka失败: Kafka advertised.listeners 返回 localhost:9092，集群内 Vector 会连接自身 localhost；请将 Kafka 广播地址改为可从主机/K8S访问的内网地址"
	case isKubernetesVectorKafkaLog(lower) && strings.Contains(lower, "failed to resolve"):
		return diagnoseKafkaResolveFailure(logText)
	case isKubernetesVectorKafkaLog(lower) && strings.Contains(lower, "connection refused"):
		return "写入Kafka失败: Broker连接被拒绝；请检查Kafka监听地址、advertised.listeners、端口映射和网络策略"
	case isKubernetesVectorKafkaLog(lower) && (strings.Contains(lower, "message production error") || strings.Contains(lower, "messagetimedout") || strings.Contains(lower, "message timed out")):
		return "写入Kafka超时: Vector已读取日志但发送到Kafka未确认；请检查Kafka advertised.listeners、Broker健康、Topic分区和网络连通性"
	case isKubernetesVectorKafkaLog(lower) && (strings.Contains(lower, "brokertransportfailure") || strings.Contains(lower, "broker transport failure")):
		return "写入Kafka失败: Broker传输异常；请检查Kafka advertised.listeners是否指向可访问地址"
	case strings.Contains(lower, "request timed out"):
		return "写入VictoriaLogs超时: Vector已读取日志，但发送到日志库未收到响应；请检查K8S Pod到VictoriaLogs地址的网络、路由、防火墙或Service暴露"
	case strings.Contains(lower, "unable to retrieve container logs"):
		return "无法读取Vector容器日志: 容器可能刚退出或运行时日志已轮转；若同时出现exitCode=78，通常需要检查Vector配置预览"
	case strings.Contains(lower, "connection refused"):
		return "写入VictoriaLogs失败: 目标地址拒绝连接；请检查VictoriaLogs服务是否监听9428端口以及地址是否正确"
	case strings.Contains(lower, "no route to host") || strings.Contains(lower, "network is unreachable") || strings.Contains(lower, "i/o timeout"):
		return "写入VictoriaLogs失败: K8S Pod无法访问日志库网络；请检查集群路由、NetworkPolicy、防火墙和日志库内网地址"
	case strings.Contains(lower, "unknown variant") && strings.Contains(lower, "ndjson"):
		return "Vector配置不兼容: 当前Vector版本不支持encoding.codec=ndjson；请重新下发最新配置"
	case strings.Contains(lower, "unknown field") && strings.Contains(lower, "framing"):
		return "Vector配置不兼容: 当前Vector版本不支持framing字段；请重新下发兼容配置"
	case strings.Contains(lower, "unknown field") && strings.Contains(lower, "healthcheck"):
		return "Vector配置不兼容: 当前Vector版本不支持healthcheck字段；请重新下发兼容配置"
	case strings.Contains(lower, "configuration error"):
		if line := findKubernetesVectorLogLine(logText, "Configuration error"); line != "" {
			return "Vector配置错误: " + line
		}
		return "Vector配置错误: 请查看lastLog中的Configuration error详情"
	case strings.Contains(lower, "http status") || strings.Contains(lower, "response status"):
		if line := findKubernetesVectorLogLine(logText, "status"); line != "" {
			return "VictoriaLogs写入被拒绝: " + line
		}
	case strings.Contains(lower, "failed reading file for fingerprinting"):
		return "读取容器日志文件失败: 日志文件可能被容器运行时轮转或删除；若持续出现请检查/var/log/containers挂载和文件匹配规则"
	}
	return ""
}

func diagnoseKafkaResolveFailure(logText string) string {
	address := ""
	if matches := kafkaFailedResolvePattern.FindStringSubmatch(logText); len(matches) == 2 {
		address = strings.TrimSpace(matches[1])
	}
	if address == "" {
		return "写入Kafka失败: Kafka advertised.listeners 返回了不可解析的Broker地址；请将Kafka广播地址改为可从K8S访问的内网IP/域名，或在集群DNS/hosts中配置解析"
	}
	return fmt.Sprintf("写入Kafka失败: Kafka advertised.listeners 返回不可解析地址 %s；K8S Pod内无法解析该主机名，请将Kafka广播地址改为可从K8S访问的内网IP/域名，或在集群DNS/hosts中配置解析", address)
}

func isKubernetesVectorKafkaLog(lowerLogText string) bool {
	return strings.Contains(lowerLogText, "component_type=kafka") ||
		strings.Contains(lowerLogText, `component_type="kafka"`) ||
		strings.Contains(lowerLogText, "kafkaerror") ||
		strings.Contains(lowerLogText, "rdkafka") ||
		strings.Contains(lowerLogText, "brokertransportfailure")
}

func findKubernetesVectorLogLine(logText, keyword string) string {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for _, line := range strings.Split(logText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if keyword == "" || strings.Contains(strings.ToLower(line), keyword) {
			return line
		}
	}
	return ""
}

func readKubernetesPodLog(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, options *corev1.PodLogOptions) (string, error) {
	stream, err := clientset.CoreV1().Pods(namespace).GetLogs(podName, options).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func listKubernetesWarningEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, keyword string) []logtailmodel.LogK8SCollectorEvent {
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	items := make([]logtailmodel.LogK8SCollectorEvent, 0, 5)
	keyword = strings.TrimSpace(keyword)
	for i := len(events.Items) - 1; i >= 0 && len(items) < 5; i-- {
		event := events.Items[i]
		if event.Type != corev1.EventTypeWarning {
			continue
		}
		ref := event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name
		if keyword != "" && !strings.Contains(event.Message, keyword) && !strings.Contains(ref, keyword) {
			continue
		}
		lastTime := ""
		if !event.LastTimestamp.IsZero() {
			lastTime = event.LastTimestamp.Time.Format(time.RFC3339)
		}
		items = append(items, logtailmodel.LogK8SCollectorEvent{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			LastTime:  lastTime,
			ObjectRef: ref,
		})
	}
	return items
}

func listKubernetesWarningEventsForPods(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pods []logtailmodel.LogK8SCollectorPod) []logtailmodel.LogK8SCollectorEvent {
	if len(pods) == 0 {
		return nil
	}
	hasProblemPod := false
	podNames := make(map[string]struct{}, len(pods))
	for _, pod := range pods {
		if pod.Name != "" {
			podNames[pod.Name] = struct{}{}
		}
		if !pod.Ready || strings.TrimSpace(pod.Message) != "" {
			hasProblemPod = true
		}
	}
	if len(podNames) == 0 || !hasProblemPod {
		return nil
	}

	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	items := make([]logtailmodel.LogK8SCollectorEvent, 0, 5)
	for i := len(events.Items) - 1; i >= 0 && len(items) < 5; i-- {
		event := events.Items[i]
		if event.Type != corev1.EventTypeWarning {
			continue
		}
		if event.InvolvedObject.Kind != "Pod" {
			continue
		}
		if _, ok := podNames[event.InvolvedObject.Name]; !ok {
			continue
		}
		lastTime := ""
		if !event.LastTimestamp.IsZero() {
			lastTime = event.LastTimestamp.Time.Format(time.RFC3339)
		}
		items = append(items, logtailmodel.LogK8SCollectorEvent{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			LastTime:  lastTime,
			ObjectRef: event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
		})
	}
	return items
}

func determineKubernetesCollectorProgress(desired, ready, updated, available int32, errMsg string) (string, int, string) {
	if desired <= 0 {
		return "pending", 30, "已下发，等待副本调度"
	}
	if ready >= desired && updated >= desired && available >= desired {
		return "running", 100, "运行中"
	}
	progress := minKubernetesCollectorProgress(desired, ready, updated, available)
	if progress < 10 {
		progress = 10
	}
	errMsg = strings.TrimSpace(errMsg)
	if errMsg != "" {
		if ready > 0 {
			return "pending", progress, fmt.Sprintf("滚动更新中: 就绪 %d/%d，已更新 %d/%d，可用 %d/%d，存在异常Pod: %s", ready, desired, updated, desired, available, desired, errMsg)
		}
		return "failed", -1, errMsg
	}
	if ready > 0 {
		return "pending", progress, fmt.Sprintf("滚动更新中: 就绪 %d/%d，已更新 %d/%d，可用 %d/%d", ready, desired, updated, desired, available, desired)
	}
	return "pending", progress, fmt.Sprintf("等待就绪: %d/%d，已更新 %d/%d", ready, desired, updated, desired)
}

func minKubernetesCollectorProgress(desired, ready, updated, available int32) int {
	minimum := desired
	for _, value := range []int32{ready, updated, available} {
		if value < minimum {
			minimum = value
		}
	}
	if minimum < 0 {
		minimum = 0
	}
	return int(float64(minimum) / float64(desired) * 100)
}

func firstPodError(pods []logtailmodel.LogK8SCollectorPod) string {
	for _, pod := range pods {
		if pod.Message != "" && !isKubernetesPodWaitingReason(pod.Message) {
			return pod.Name + ": " + pod.Message
		}
		if pod.Phase == string(corev1.PodFailed) || pod.Phase == string(corev1.PodUnknown) {
			return pod.Name + ": " + pod.Phase
		}
	}
	return ""
}

func isKubernetesPodWaitingReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	switch reason {
	case "", "ContainerCreating", "PodInitializing":
		return true
	default:
		return strings.HasPrefix(reason, "Unschedulable") ||
			strings.HasPrefix(reason, "Pending:")
	}
}

func podTemplateHasContainer(template corev1.PodTemplateSpec, containerName string) bool {
	for _, container := range template.Spec.Containers {
		if container.Name == containerName {
			return true
		}
	}
	return false
}

func patchKubernetesWorkloadForSidecar(ctx context.Context, clientset *kubernetes.Clientset, task logtailmodel.LogCollectTask, target kubernetesCollectTarget, configMapName, configHash string) error {
	switch normalizeKubernetesWorkloadType(target.WorkloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询Deployment失败: %w", err)
		}
		mutated, err := mutatePodTemplateForContainerFileCollector(&item.Spec.Template, task, target, configMapName, configHash)
		if err != nil {
			return err
		}
		if !mutated {
			return nil
		}
		_, err = clientset.AppsV1().Deployments(target.Namespace).Update(ctx, item, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("更新Deployment失败: %w", err)
		}
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询StatefulSet失败: %w", err)
		}
		mutated, err := mutatePodTemplateForContainerFileCollector(&item.Spec.Template, task, target, configMapName, configHash)
		if err != nil {
			return err
		}
		if !mutated {
			return nil
		}
		_, err = clientset.AppsV1().StatefulSets(target.Namespace).Update(ctx, item, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("更新StatefulSet失败: %w", err)
		}
	case "daemonset":
		item, err := clientset.AppsV1().DaemonSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询DaemonSet失败: %w", err)
		}
		mutated, err := mutatePodTemplateForContainerFileCollector(&item.Spec.Template, task, target, configMapName, configHash)
		if err != nil {
			return err
		}
		if !mutated {
			return nil
		}
		_, err = clientset.AppsV1().DaemonSets(target.Namespace).Update(ctx, item, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("更新DaemonSet失败: %w", err)
		}
	default:
		return fmt.Errorf("暂不支持的K8S工作负载类型: %s", target.WorkloadType)
	}
	return nil
}

func unpatchKubernetesWorkloadForSidecar(ctx context.Context, clientset *kubernetes.Clientset, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) error {
	switch normalizeKubernetesWorkloadType(target.WorkloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("查询Deployment失败: %w", err)
		}
		if !removePodTemplateContainerFileCollector(&item.Spec.Template, task, target) {
			return nil
		}
		if _, err := clientset.AppsV1().Deployments(target.Namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("更新Deployment失败: %w", err)
		}
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("查询StatefulSet失败: %w", err)
		}
		if !removePodTemplateContainerFileCollector(&item.Spec.Template, task, target) {
			return nil
		}
		if _, err := clientset.AppsV1().StatefulSets(target.Namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("更新StatefulSet失败: %w", err)
		}
	case "daemonset":
		item, err := clientset.AppsV1().DaemonSets(target.Namespace).Get(ctx, target.WorkloadName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("查询DaemonSet失败: %w", err)
		}
		if !removePodTemplateContainerFileCollector(&item.Spec.Template, task, target) {
			return nil
		}
		if _, err := clientset.AppsV1().DaemonSets(target.Namespace).Update(ctx, item, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("更新DaemonSet失败: %w", err)
		}
	default:
		return fmt.Errorf("暂不支持的K8S工作负载类型: %s", target.WorkloadType)
	}
	return nil
}

func mutatePodTemplateForContainerFileCollector(template *corev1.PodTemplateSpec, task logtailmodel.LogCollectTask, target kubernetesCollectTarget, configMapName, configHash string) (bool, error) {
	if template == nil {
		return false, errors.New("工作负载Pod模板不能为空")
	}
	logDirs := collectTaskLogDirs(&task)
	if len(logDirs) == 0 {
		return false, errors.New("容器内文件日志采集必须填写日志路径")
	}

	selectedContainers := targetContainerSet(target.Containers)
	volumeByDir := map[string]string{}
	for _, logDir := range logDirs {
		volumeName := existingVolumeForMountPath(template.Spec.Containers, selectedContainers, logDir)
		if volumeName == "" {
			volumeName = kubernetesVolumeName(task, logDir)
			template.Spec.Volumes = upsertEmptyDirVolume(template.Spec.Volumes, volumeName)
			for index := range template.Spec.Containers {
				if !shouldPatchContainer(template.Spec.Containers[index].Name, selectedContainers) {
					continue
				}
				template.Spec.Containers[index].VolumeMounts = upsertVolumeMount(template.Spec.Containers[index].VolumeMounts, corev1.VolumeMount{
					Name:      volumeName,
					MountPath: logDir,
				})
			}
		}
		volumeByDir[logDir] = volumeName
	}

	template.Spec.Volumes = upsertConfigMapVolume(template.Spec.Volumes, k8sVectorConfigVolume, configMapName)
	template.Spec.Volumes = upsertEmptyDirVolume(template.Spec.Volumes, k8sVectorDataVolume)
	template.Spec.HostAliases = upsertPodHostAliases(template.Spec.HostAliases, configuredKafkaHostAliases())
	template.Spec.Containers = upsertSidecarContainer(template.Spec.Containers, buildContainerFileSidecar(task, volumeByDir))
	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}
	template.Annotations["aiops.dev/log-collector"] = "container_file"
	template.Annotations["aiops.dev/log-collector-task"] = strconv.FormatUint(uint64(task.ID), 10)
	template.Annotations["aiops.dev/log-collector-config-hash"] = configHash
	return true, nil
}

func removePodTemplateContainerFileCollector(template *corev1.PodTemplateSpec, task logtailmodel.LogCollectTask, target kubernetesCollectTarget) bool {
	if template == nil {
		return false
	}
	mutated := false
	sidecarName := kubernetesSidecarName(task)
	containers := template.Spec.Containers[:0]
	for _, container := range template.Spec.Containers {
		if container.Name == sidecarName {
			mutated = true
			continue
		}
		containers = append(containers, container)
	}
	template.Spec.Containers = containers

	selectedContainers := targetContainerSet(target.Containers)
	managedVolumes := map[string]struct{}{
		k8sVectorConfigVolume: {},
		k8sVectorDataVolume:   {},
	}
	for _, logDir := range collectTaskLogDirs(&task) {
		managedVolumes[kubernetesVolumeName(task, logDir)] = struct{}{}
	}
	for index := range template.Spec.Containers {
		if !shouldPatchContainer(template.Spec.Containers[index].Name, selectedContainers) {
			continue
		}
		mounts := template.Spec.Containers[index].VolumeMounts[:0]
		for _, mount := range template.Spec.Containers[index].VolumeMounts {
			if _, ok := managedVolumes[mount.Name]; ok {
				mutated = true
				continue
			}
			mounts = append(mounts, mount)
		}
		template.Spec.Containers[index].VolumeMounts = mounts
	}

	volumes := template.Spec.Volumes[:0]
	for _, volume := range template.Spec.Volumes {
		if _, ok := managedVolumes[volume.Name]; ok {
			mutated = true
			continue
		}
		volumes = append(volumes, volume)
	}
	template.Spec.Volumes = volumes

	for _, key := range []string{
		"aiops.dev/log-collector",
		"aiops.dev/log-collector-task",
		"aiops.dev/log-collector-config-hash",
	} {
		if template.Annotations != nil {
			if _, ok := template.Annotations[key]; ok {
				delete(template.Annotations, key)
				mutated = true
			}
		}
	}
	return mutated
}

func buildContainerFileSidecar(task logtailmodel.LogCollectTask, volumeByDir map[string]string) corev1.Container {
	mounts := []corev1.VolumeMount{
		{Name: k8sVectorConfigVolume, MountPath: "/etc/vector", ReadOnly: true},
		{Name: k8sVectorDataVolume, MountPath: k8sVectorDataDir},
	}
	dirs := make([]string, 0, len(volumeByDir))
	for dir := range volumeByDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volumeByDir[dir],
			MountPath: dir,
			ReadOnly:  true,
		})
	}
	return corev1.Container{
		Name:            kubernetesSidecarName(task),
		Image:           configuredKubernetesVectorImage(),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args:            []string{"--config", "/etc/vector/vector.yaml"},
		VolumeMounts:    mounts,
	}
}

func configuredKafkaHostAliases() []corev1.HostAlias {
	if config.Config == nil {
		return nil
	}
	aliases := config.Config.Logtail.Kafka.HostAliases
	if len(aliases) == 0 {
		return nil
	}
	result := make([]corev1.HostAlias, 0, len(aliases))
	for _, alias := range aliases {
		ip := strings.TrimSpace(alias.IP)
		hostnames := trimStringSlice(alias.Hostnames)
		if ip == "" || len(hostnames) == 0 {
			continue
		}
		result = append(result, corev1.HostAlias{
			IP:        ip,
			Hostnames: hostnames,
		})
	}
	return result
}

func upsertPodHostAliases(current, desired []corev1.HostAlias) []corev1.HostAlias {
	if len(desired) == 0 {
		return current
	}
	result := append([]corev1.HostAlias(nil), current...)
	for _, alias := range desired {
		ip := strings.TrimSpace(alias.IP)
		hostnames := trimStringSlice(alias.Hostnames)
		if ip == "" || len(hostnames) == 0 {
			continue
		}
		found := false
		for index := range result {
			if result[index].IP != ip {
				continue
			}
			result[index].Hostnames = mergeUniqueStrings(result[index].Hostnames, hostnames)
			found = true
			break
		}
		if !found {
			result = append(result, corev1.HostAlias{IP: ip, Hostnames: hostnames})
		}
	}
	return result
}

func trimStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mergeUniqueStrings(base, extra []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)+len(extra))
	for _, value := range base {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	for _, value := range extra {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func parseKubernetesCollectTargets(raw string) ([]kubernetesCollectTarget, error) {
	indexedTargets, err := parseIndexedKubernetesCollectTargets(raw)
	if err != nil {
		return nil, err
	}
	targets := make([]kubernetesCollectTarget, 0, len(indexedTargets))
	for _, target := range indexedTargets {
		targets = append(targets, target.Target)
	}
	return targets, nil
}

func parseIndexedKubernetesCollectTargetsForTask(task logtailmodel.LogCollectTask) ([]indexedKubernetesCollectTarget, error) {
	return parseIndexedKubernetesCollectTargetsBySource(task.ExecuteTargetsJSON, normalizeKubernetesLogSource(task.K8SLogSource))
}

func parseIndexedKubernetesCollectTargets(raw string) ([]indexedKubernetesCollectTarget, error) {
	return parseIndexedKubernetesCollectTargetsBySource(raw, "")
}

func parseIndexedKubernetesCollectTargetsBySource(raw, source string) ([]indexedKubernetesCollectTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			return nil, errors.New("节点文件日志采集必须选择K8S集群")
		}
		return nil, errors.New("K8S日志采集必须选择工作负载")
	}
	var genericTargets []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &genericTargets); err != nil {
		return nil, errors.New("K8S日志采集目标格式不正确")
	}
	targets := make([]indexedKubernetesCollectTarget, 0, len(genericTargets))
	for index, rawTarget := range genericTargets {
		target := kubernetesCollectTarget{
			ClusterID:    mapUintValue(rawTarget, "clusterId", "clusterID"),
			Namespace:    mapStringValue(rawTarget, "namespace", "namespaceName"),
			WorkloadType: normalizeKubernetesWorkloadType(mapStringValue(rawTarget, "workloadType", "type")),
			WorkloadName: mapStringValue(rawTarget, "workloadName", "name"),
			Containers:   mapStringSliceValue(rawTarget, "containers", "containerNames", "containerName"),
		}
		if target.ClusterID == 0 {
			return nil, fmt.Errorf("K8S日志采集目标第%d项缺少clusterId", index+1)
		}
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			target.Namespace = k8sVectorNamespace
			target.WorkloadType = logtailmodel.LogK8SLogSourceNodeFile
			target.WorkloadName = k8sClusterCollectorName
			target.Containers = nil
			targets = append(targets, indexedKubernetesCollectTarget{Index: index, Target: target})
			continue
		}
		if target.Namespace == "" {
			return nil, fmt.Errorf("K8S日志采集目标第%d项缺少namespace", index+1)
		}
		if target.WorkloadType == "" {
			return nil, fmt.Errorf("K8S日志采集目标第%d项缺少workloadType", index+1)
		}
		if target.WorkloadName == "" {
			return nil, fmt.Errorf("K8S日志采集目标第%d项缺少workloadName", index+1)
		}
		targets = append(targets, indexedKubernetesCollectTarget{Index: index, Target: target})
	}
	return targets, nil
}

func kubernetesCollectTargetDisplayName(target kubernetesCollectTarget) string {
	if target.WorkloadType == logtailmodel.LogK8SLogSourceNodeFile {
		return fmt.Sprintf("cluster/%d/node_file", target.ClusterID)
	}
	return fmt.Sprintf("%s/%s/%s", target.Namespace, target.WorkloadType, target.WorkloadName)
}

func (s *LogtailService) getKubernetesTaskTarget(taskID uint, targetIndex int) (*logtailmodel.LogCollectTask, indexedKubernetesCollectTarget, error) {
	if taskID == 0 {
		return nil, indexedKubernetesCollectTarget{}, errors.New("采集任务ID不能为空")
	}
	task, err := s.taskDAO.GetByID(taskID)
	if err != nil {
		return nil, indexedKubernetesCollectTarget{}, err
	}
	hydrateCollectTaskPresentation(task)
	if !isKubernetesCollectType(task.CollectType) {
		return nil, indexedKubernetesCollectTarget{}, errors.New("该采集任务不是K8S日志采集任务")
	}
	targets, err := parseIndexedKubernetesCollectTargetsForTask(*task)
	if err != nil {
		return nil, indexedKubernetesCollectTarget{}, err
	}
	for _, target := range targets {
		if target.Index == targetIndex {
			return task, target, nil
		}
	}
	return nil, indexedKubernetesCollectTarget{}, fmt.Errorf("K8S采集目标 targetIndex=%d 不存在", targetIndex)
}

func kubernetesCollectorMatchesQuery(task logtailmodel.LogCollectTask, target kubernetesCollectTarget, query logtailmodel.LogK8SCollectorQuery) bool {
	if query.ClusterID > 0 && target.ClusterID != query.ClusterID {
		return false
	}
	if namespace := strings.TrimSpace(query.Namespace); namespace != "" && target.Namespace != namespace {
		return false
	}
	if workloadType := normalizeKubernetesWorkloadType(query.WorkloadType); workloadType != "" && target.WorkloadType != workloadType {
		return false
	}
	if source := normalizeKubernetesLogSource(query.K8SLogSource); source != "" && !kubernetesLogSourceIncludes(task.K8SLogSource, source) {
		return false
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		haystack := strings.Join([]string{task.Name, task.Code, target.Namespace, target.WorkloadType, target.WorkloadName, strings.Join(target.Containers, ",")}, " ")
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(keyword))
	}
	return true
}

func summarizeKubernetesCollectors(items []logtailmodel.LogK8SCollectorVO) logtailmodel.LogK8SCollectorStats {
	var stats logtailmodel.LogK8SCollectorStats
	for _, item := range items {
		stats.Total++
		switch item.Status {
		case "running":
			stats.Running++
		case "failed":
			stats.Failed++
		case "not_installed":
			stats.NotInstalled++
		default:
			stats.Pending++
		}
	}
	return stats
}

func isActiveOperation(status string) bool {
	return status == logtailmodel.LogAgentOperationStatusPending || status == logtailmodel.LogAgentOperationStatusRunning
}

func mapStringSliceValue(values map[string]interface{}, keys ...string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, key := range keys {
		raw, ok := values[key]
		if !ok {
			continue
		}
		switch typed := raw.(type) {
		case []interface{}:
			for _, item := range typed {
				value := strings.TrimSpace(fmt.Sprint(item))
				if value == "" || value == "<nil>" {
					continue
				}
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				result = append(result, value)
			}
		case []string:
			for _, item := range typed {
				value := strings.TrimSpace(item)
				if value == "" {
					continue
				}
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				result = append(result, value)
			}
		case string:
			for _, item := range strings.Split(typed, ",") {
				value := strings.TrimSpace(item)
				if value == "" {
					continue
				}
				if _, ok := seen[value]; ok {
					continue
				}
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
		if len(result) > 0 {
			break
		}
	}
	sort.Strings(result)
	return result
}

func normalizeKubernetesWorkloadType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "deployment", "deploy", "deployments":
		return "deployment"
	case "statefulset", "statefulsets", "sts":
		return "statefulset"
	case "daemonset", "daemonsets", "ds":
		return "daemonset"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func kubernetesContainerFileVRLPattern(target kubernetesCollectTarget) string {
	workload := regexp.QuoteMeta(target.WorkloadName)
	namespace := regexp.QuoteMeta(target.Namespace)
	containerPattern := ".+"
	if len(target.Containers) > 0 {
		parts := make([]string, 0, len(target.Containers))
		for _, container := range target.Containers {
			parts = append(parts, regexp.QuoteMeta(container))
		}
		containerPattern = "(?:" + strings.Join(parts, "|") + ")"
	}
	return fmt.Sprintf(`^.*/(?P<k8s_pod_name>%s(?:-[^_]*)?)_%s_(?P<k8s_container_name>%s)-(?P<k8s_container_id>[^/]+)\.log$`, workload, namespace, containerPattern)
}

func kubernetesPodFileVRLPattern(target kubernetesCollectTarget) string {
	workload := regexp.QuoteMeta(target.WorkloadName)
	namespace := regexp.QuoteMeta(target.Namespace)
	containerPattern := ".+"
	if len(target.Containers) > 0 {
		parts := make([]string, 0, len(target.Containers))
		for _, container := range target.Containers {
			parts = append(parts, regexp.QuoteMeta(container))
		}
		containerPattern = "(?:" + strings.Join(parts, "|") + ")"
	}
	return fmt.Sprintf(`^.*/%s_(?P<k8s_pod_name>%s(?:-[^_/]*)?)_[^/]+/(?P<k8s_container_name>%s)/(?P<k8s_restart_count>[^/]+)\.log$`, namespace, workload, containerPattern)
}

func collectTaskLogDirs(task *logtailmodel.LogCollectTask) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, pattern := range collectTaskPathPatterns(task) {
		dir := path.Clean(path.Dir(strings.TrimSpace(pattern)))
		if dir == "." || dir == "/" || dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		result = append(result, dir)
	}
	sort.Strings(result)
	return result
}

func targetContainerSet(containers []string) map[string]struct{} {
	if len(containers) == 0 {
		return nil
	}
	result := map[string]struct{}{}
	for _, container := range containers {
		container = strings.TrimSpace(container)
		if container != "" {
			result[container] = struct{}{}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func shouldPatchContainer(name string, selected map[string]struct{}) bool {
	if len(selected) == 0 {
		return true
	}
	_, ok := selected[name]
	return ok
}

func existingVolumeForMountPath(containers []corev1.Container, selected map[string]struct{}, mountPath string) string {
	for _, container := range containers {
		if !shouldPatchContainer(container.Name, selected) {
			continue
		}
		for _, mount := range container.VolumeMounts {
			if mount.MountPath == mountPath && mount.Name != "" {
				return mount.Name
			}
		}
	}
	return ""
}

func upsertEmptyDirVolume(volumes []corev1.Volume, name string) []corev1.Volume {
	for index := range volumes {
		if volumes[index].Name == name {
			volumes[index].VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
			return volumes
		}
	}
	return append(volumes, corev1.Volume{
		Name:         name,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
}

func upsertConfigMapVolume(volumes []corev1.Volume, name, configMapName string) []corev1.Volume {
	volume := corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
			},
		},
	}
	for index := range volumes {
		if volumes[index].Name == name {
			volumes[index] = volume
			return volumes
		}
	}
	return append(volumes, volume)
}

func upsertVolumeMount(mounts []corev1.VolumeMount, mount corev1.VolumeMount) []corev1.VolumeMount {
	for index := range mounts {
		if mounts[index].MountPath == mount.MountPath {
			mounts[index] = mount
			return mounts
		}
	}
	return append(mounts, mount)
}

func upsertSidecarContainer(containers []corev1.Container, sidecar corev1.Container) []corev1.Container {
	for index := range containers {
		if containers[index].Name == sidecar.Name {
			containers[index] = sidecar
			return containers
		}
	}
	return append(containers, sidecar)
}

func ensureKubernetesNamespace(ctx context.Context, clientset *kubernetes.Clientset, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "aiops-vector",
				"app.kubernetes.io/managed-by": k8sCollectorManagedBy,
			},
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("创建K8S日志采集命名空间失败: %w", err)
	}
	return nil
}

func applyConfigMap(ctx context.Context, clientset *kubernetes.Clientset, desired *corev1.ConfigMap) error {
	current, err := clientset.CoreV1().ConfigMaps(desired.Namespace).Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, createErr := clientset.CoreV1().ConfigMaps(desired.Namespace).Create(ctx, desired, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("创建ConfigMap失败: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询ConfigMap失败: %w", err)
	}
	current.Labels = desired.Labels
	current.Annotations = desired.Annotations
	current.Data = desired.Data
	if _, err := clientset.CoreV1().ConfigMaps(desired.Namespace).Update(ctx, current, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("更新ConfigMap失败: %w", err)
	}
	return nil
}

func applyServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, desired *corev1.ServiceAccount) error {
	current, err := clientset.CoreV1().ServiceAccounts(desired.Namespace).Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, createErr := clientset.CoreV1().ServiceAccounts(desired.Namespace).Create(ctx, desired, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("创建ServiceAccount失败: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询ServiceAccount失败: %w", err)
	}
	current.Labels = desired.Labels
	if _, err := clientset.CoreV1().ServiceAccounts(desired.Namespace).Update(ctx, current, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("更新ServiceAccount失败: %w", err)
	}
	return nil
}

func applyDaemonSet(ctx context.Context, clientset *kubernetes.Clientset, desired *appsv1.DaemonSet) error {
	current, err := clientset.AppsV1().DaemonSets(desired.Namespace).Get(ctx, desired.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, createErr := clientset.AppsV1().DaemonSets(desired.Namespace).Create(ctx, desired, metav1.CreateOptions{}); createErr != nil {
			return fmt.Errorf("创建DaemonSet失败: %w", createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询DaemonSet失败: %w", err)
	}
	current.Labels = desired.Labels
	current.Spec.Template = desired.Spec.Template
	current.Spec.UpdateStrategy = desired.Spec.UpdateStrategy
	if _, err := clientset.AppsV1().DaemonSets(desired.Namespace).Update(ctx, current, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("更新DaemonSet失败: %w", err)
	}
	return nil
}

func deleteDaemonSetIfExists(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	err := clientset.AppsV1().DaemonSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除DaemonSet失败: %w", err)
	}
	return nil
}

func deleteConfigMapIfExists(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	err := clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除ConfigMap失败: %w", err)
	}
	return nil
}

func deleteServiceAccountIfExists(ctx context.Context, clientset *kubernetes.Clientset, namespace, name string) error {
	err := clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("删除ServiceAccount失败: %w", err)
	}
	return nil
}

func kubernetesOperationSuccessMessage(task logtailmodel.LogCollectTask, opType string) string {
	source := normalizeKubernetesLogSource(task.K8SLogSource)
	switch opType {
	case logtailmodel.LogK8SCollectorOperationUninstall:
		if source == logtailmodel.LogK8SLogSourceStdout {
			return "已移除标准输出任务配置并刷新集群级Vector"
		}
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			return "已移除节点文件任务配置并刷新集群级Vector"
		}
		return "已卸载 K8S Collector"
	case logtailmodel.LogK8SCollectorOperationReinstall:
		if source == logtailmodel.LogK8SLogSourceStdout || source == logtailmodel.LogK8SLogSourceNodeFile {
			return "已刷新集群级Vector配置"
		}
		return "已重装 K8S Collector"
	case logtailmodel.LogK8SCollectorOperationInstall:
		if source == logtailmodel.LogK8SLogSourceContainerFile {
			return "已安装 K8S Vector Sidecar 并更新工作负载"
		}
		return "已刷新集群级Vector配置"
	default:
		if source == logtailmodel.LogK8SLogSourceContainerFile {
			return "已更新 K8S Vector Sidecar 并更新工作负载"
		}
		return "已刷新集群级Vector配置"
	}
}

func kubernetesCollectorLabels(task logtailmodel.LogCollectTask, target kubernetesCollectTarget, name string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "aiops-vector",
		"app.kubernetes.io/instance":   name,
		"app.kubernetes.io/managed-by": k8sCollectorManagedBy,
		"aiops.dev/collect-task-id":    strconv.FormatUint(uint64(task.ID), 10),
		"aiops.dev/collect-task-code":  sanitizeKubernetesLabelValue(task.Code),
		"aiops.dev/log-source":         normalizeKubernetesLogSource(task.K8SLogSource),
		"aiops.dev/workload":           sanitizeKubernetesLabelValue(target.WorkloadName),
	}
}

func kubernetesCollectorName(task logtailmodel.LogCollectTask, target kubernetesCollectTarget, suffix string) string {
	return k8sResourceName("aiops-vector", strconv.FormatUint(uint64(task.ID), 10), target.Namespace, target.WorkloadName, suffix)
}

func kubernetesVolumeName(task logtailmodel.LogCollectTask, logDir string) string {
	return k8sResourceName(k8sCollectorVolumePrefix, strconv.FormatUint(uint64(task.ID), 10), path.Base(logDir))
}

func kubernetesSidecarName(task logtailmodel.LogCollectTask) string {
	return k8sResourceName("aiops-vector", strconv.FormatUint(uint64(task.ID), 10))
}

func k8sResourceName(parts ...string) string {
	joined := strings.ToLower(strings.Join(parts, "-"))
	joined = k8sNameInvalidChars.ReplaceAllString(joined, "-")
	joined = strings.Trim(joined, "-")
	for strings.Contains(joined, "--") {
		joined = strings.ReplaceAll(joined, "--", "-")
	}
	if joined == "" {
		joined = "aiops-vector"
	}
	if len(joined) <= 63 {
		return joined
	}
	hash := shortConfigHash(joined)
	prefixLength := 63 - len(hash) - 1
	if prefixLength < 1 {
		return hash
	}
	return strings.Trim(joined[:prefixLength], "-") + "-" + hash
}

func sanitizeKubernetesLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 63 {
		return value
	}
	return value[:54] + "-" + shortConfigHash(value)
}

func shortConfigHash(value string) string {
	sum := fmt.Sprintf("%x", sha256Sum(value))
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}

func sha256Sum(value string) []byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(value))
	return hash.Sum(nil)
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range base {
		result[key] = value
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}

func configuredKubernetesVectorImage() string {
	if config.Config != nil {
		return strings.TrimSpace(config.Config.Logtail.Aggregator.Image)
	}
	return ""
}
