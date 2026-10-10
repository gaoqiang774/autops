package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	k8sdao "dodevops-api/api/k8s/dao"
	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	k8sClusterCollectorName       = "aiops-vector"
	k8sVectorBaseConfigMapName    = "aiops-vector-base"
	k8sVectorBaseConfigFile       = "00-base.yaml"
	k8sVectorLegacyProjectPrefix  = "aiops-vector-project-"
	k8sVectorConfigScopeLabel     = "aiops.dev/config-scope"
	k8sVectorConfigProjectIDLabel = "aiops.dev/log-project-id"
	k8sVectorConfigScopeBase      = "base"
	k8sVectorConfigScopeProject   = "project"
)

type kubernetesVectorConfigFile struct {
	Scope     string
	Project   *logtailmodel.LogProject
	ConfigMap string
	FileName  string
	Content   string
}

type kubernetesClusterVectorConfigBundle struct {
	Base     kubernetesVectorConfigFile
	Projects []kubernetesVectorConfigFile
}

type kubernetesVectorConfigRef struct {
	ConfigMap string
	FileName  string
}

func (bundle kubernetesClusterVectorConfigBundle) files() []kubernetesVectorConfigFile {
	result := make([]kubernetesVectorConfigFile, 0, 1+len(bundle.Projects))
	result = append(result, bundle.Base)
	result = append(result, bundle.Projects...)
	return result
}

func (bundle kubernetesClusterVectorConfigBundle) hash() string {
	var builder strings.Builder
	for _, file := range bundle.files() {
		builder.WriteString(file.ConfigMap)
		builder.WriteByte('\n')
		builder.WriteString(file.FileName)
		builder.WriteByte('\n')
		builder.WriteString(file.Content)
		builder.WriteByte('\n')
	}
	return shortConfigHash(builder.String())
}

type kubernetesStdoutPipelineTarget struct {
	Task   logtailmodel.LogCollectTask
	Target indexedKubernetesCollectTarget
}

type kubernetesNodeFilePipelineTarget struct {
	Task   logtailmodel.LogCollectTask
	Target indexedKubernetesCollectTarget
}

type kubernetesProjectPipelineTargets struct {
	Stdout   []kubernetesStdoutPipelineTarget
	NodeFile []kubernetesNodeFilePipelineTarget
}

func (s *LogtailService) ListKubernetesClusterCollectors(query logtailmodel.LogK8SClusterCollectorQuery) (*logtailmodel.LogK8SClusterCollectorListResponse, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	clusters, err := listKubernetesClustersForCollector(query)
	if err != nil {
		return nil, err
	}

	clusterIDs := make([]uint, 0, len(clusters))
	for _, cluster := range clusters {
		clusterIDs = append(clusterIDs, cluster.ID)
	}
	latestOps, _ := s.k8sClusterDAO.LatestByClusters(clusterIDs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	items := make([]logtailmodel.LogK8SClusterCollectorVO, 0, len(clusters))
	clusterCache := map[uint]*k8smodel.KubeCluster{}
	clientCache := map[uint]*kubernetes.Clientset{}
	for i := range clusters {
		cluster := clusters[i]
		clusterCache[cluster.ID] = &cluster
		_, clientset, clientErr := s.getKubernetesClusterClient(cluster.ID, clusterCache, clientCache)
		if clientErr != nil {
			item := baseKubernetesClusterCollectorVO(cluster, latestOps[cluster.ID], "failed", -1, "连接K8S集群失败: "+clientErr.Error())
			if query.Status == "" || item.Status == query.Status {
				items = append(items, item)
			}
			continue
		}
		item := s.buildKubernetesClusterCollectorStatus(ctx, clientset, cluster, latestOps[cluster.ID])
		if query.Status != "" && item.Status != query.Status {
			continue
		}
		items = append(items, item)
	}

	summary := summarizeKubernetesClusterCollectors(items)
	total := int64(len(items))
	start := (query.Page - 1) * query.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + query.PageSize
	if end > len(items) {
		end = len(items)
	}

	return &logtailmodel.LogK8SClusterCollectorListResponse{
		Total:    total,
		List:     items[start:end],
		Summary:  summary,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (s *LogtailService) GetKubernetesClusterCollectorStatus(clusterID uint) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	if clusterID == 0 {
		return nil, errors.New("K8S集群ID不能为空")
	}
	cluster, err := k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
	if err != nil {
		return nil, err
	}
	latestOps, _ := s.k8sClusterDAO.LatestByClusters([]uint{clusterID})
	clusterCache := map[uint]*k8smodel.KubeCluster{cluster.ID: cluster}
	clientCache := map[uint]*kubernetes.Clientset{}
	_, clientset, err := s.getKubernetesClusterClient(clusterID, clusterCache, clientCache)
	if err != nil {
		item := baseKubernetesClusterCollectorVO(*cluster, latestOps[clusterID], "failed", -1, "连接K8S集群失败: "+err.Error())
		return &item, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	item := s.buildKubernetesClusterCollectorStatus(ctx, clientset, *cluster, latestOps[clusterID])
	s.fillKubernetesClusterCollectorConfig(ctx, clientset, &item)
	return &item, nil
}

func (s *LogtailService) InstallKubernetesClusterCollector(clusterID uint, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	return s.runKubernetesClusterCollectorAction(clusterID, logtailmodel.LogK8SCollectorOperationInstall, req)
}

func (s *LogtailService) UpdateKubernetesClusterCollector(clusterID uint, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	return s.runKubernetesClusterCollectorAction(clusterID, logtailmodel.LogK8SCollectorOperationUpdate, req)
}

func (s *LogtailService) UninstallKubernetesClusterCollector(clusterID uint, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	return s.runKubernetesClusterCollectorAction(clusterID, logtailmodel.LogK8SCollectorOperationUninstall, req)
}

func (s *LogtailService) ReinstallKubernetesClusterCollector(clusterID uint, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	return s.runKubernetesClusterCollectorAction(clusterID, logtailmodel.LogK8SCollectorOperationReinstall, req)
}

func (s *LogtailService) ListKubernetesClusterCollectorOperations(clusterID uint, query logtailmodel.LogK8SClusterCollectorOperationQuery) ([]logtailmodel.LogK8SClusterCollectorOperation, int64, logtailmodel.LogK8SClusterCollectorOperationQuery, error) {
	if clusterID == 0 {
		return nil, 0, query, errors.New("K8S集群ID不能为空")
	}
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	query.ClusterID = clusterID
	items, total, err := s.k8sClusterDAO.List(query)
	hydrateKubernetesClusterCollectorOperationsRuntime(items)
	return items, total, query, err
}

func (s *LogtailService) runKubernetesClusterCollectorAction(clusterID uint, opType string, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorVO, error) {
	if clusterID == 0 {
		return nil, errors.New("K8S集群ID不能为空")
	}
	operator := strings.TrimSpace(req.Operator)
	if operator == "" {
		operator = "admin"
	}
	cluster, err := k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("查询K8S集群失败 clusterId=%d: %w", clusterID, err)
	}
	clusterCache := map[uint]*k8smodel.KubeCluster{cluster.ID: cluster}
	clientCache := map[uint]*kubernetes.Clientset{}
	_, clientset, err := s.getKubernetesClusterClient(clusterID, clusterCache, clientCache)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if opType != logtailmodel.LogK8SCollectorOperationUninstall {
		if err := s.ensureKubernetesClusterCollectorIdle(ctx, clientset, clusterID); err != nil {
			return nil, err
		}
	}
	operation, err := s.createKubernetesClusterCollectorOperation(*cluster, opType, operator, req)
	if err != nil {
		return nil, err
	}
	_, _ = s.k8sClusterDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusRunning, 10, "开始执行K8S集群Collector操作", "", "")

	vectorImage := kubernetesClusterCollectorRequestImage(req)
	if opType != logtailmodel.LogK8SCollectorOperationUninstall {
		vectorImage = s.resolveKubernetesClusterCollectorImage(ctx, clientset, vectorImage)
	}
	err = s.executeKubernetesClusterCollectorAction(ctx, clientset, cluster, opType, vectorImage)
	if err != nil {
		_, _ = s.k8sClusterDAO.UpdateProgress(operation.ID, logtailmodel.LogAgentOperationStatusFailed, -1, "执行K8S集群Collector操作失败", "", err.Error())
		return nil, err
	}

	message := kubernetesClusterOperationSuccessMessage(opType)
	latestOp := *operation
	item := s.buildKubernetesClusterCollectorStatus(ctx, clientset, *cluster, latestOp)
	s.fillKubernetesClusterCollectorConfig(ctx, clientset, &item)
	resultItem := item
	resultItem.LastOperation = nil
	resultJSON := ""
	if payload, marshalErr := json.Marshal(resultItem); marshalErr == nil {
		resultJSON = string(payload)
	}
	operationStatus := logtailmodel.LogAgentOperationStatusRunning
	operationProgress := item.Progress
	operationMessage := firstNonEmpty(item.ProgressText, "K8S集群Collector滚动更新中")
	operationError := ""
	if item.Status == "failed" {
		operationStatus = logtailmodel.LogAgentOperationStatusFailed
		operationProgress = -1
		operationMessage = firstNonEmpty(item.LastError, item.ProgressText, "K8S集群Collector运行失败")
		operationError = operationMessage
	} else if item.Status == "running" {
		operationStatus = logtailmodel.LogAgentOperationStatusSuccess
		operationProgress = 100
		operationMessage = message
	}
	_, _ = s.k8sClusterDAO.UpdateProgress(operation.ID, operationStatus, operationProgress, operationMessage, resultJSON, operationError)
	latestOps, _ := s.k8sClusterDAO.LatestByClusters([]uint{clusterID})
	item.LastOperation = nil
	if latest, ok := latestOps[clusterID]; ok {
		hydrateKubernetesClusterCollectorOperationRuntime(&latest)
		item.LastOperation = &latest
	}
	return &item, nil
}

func hydrateKubernetesClusterCollectorOperationsRuntime(items []logtailmodel.LogK8SClusterCollectorOperation) {
	for i := range items {
		hydrateKubernetesClusterCollectorOperationRuntime(&items[i])
	}
}

func hydrateKubernetesClusterCollectorOperationRuntime(item *logtailmodel.LogK8SClusterCollectorOperation) {
	if item == nil {
		return
	}
	item.RuntimeStatus = ""
	item.RuntimeMessage = ""
	item.RuntimeProgress = 0
	item.RuntimeReady = false

	resultJSON := strings.TrimSpace(item.ResultJSON)
	if resultJSON != "" {
		var snapshot struct {
			Status       string `json:"status"`
			Progress     int    `json:"progress"`
			ProgressText string `json:"progressText"`
			LastError    string `json:"lastError"`
		}
		if err := json.Unmarshal([]byte(resultJSON), &snapshot); err == nil {
			item.RuntimeStatus = strings.TrimSpace(snapshot.Status)
			item.RuntimeProgress = snapshot.Progress
			item.RuntimeMessage = firstNonEmpty(strings.TrimSpace(snapshot.LastError), strings.TrimSpace(snapshot.ProgressText))
		}
	}
	if item.RuntimeStatus == "" && item.OpStatus == logtailmodel.LogAgentOperationStatusFailed {
		item.RuntimeStatus = "failed"
		item.RuntimeMessage = strings.TrimSpace(item.ErrorMsg)
	}

	switch item.OpType {
	case logtailmodel.LogK8SCollectorOperationUninstall:
		item.RuntimeReady = item.RuntimeStatus == "not_installed"
	default:
		item.RuntimeReady = item.RuntimeStatus == "running"
	}
	normalizeKubernetesClusterCollectorOperationDisplay(item)
}

func normalizeKubernetesClusterCollectorOperationDisplay(item *logtailmodel.LogK8SClusterCollectorOperation) {
	if item == nil || item.OpStatus != logtailmodel.LogAgentOperationStatusSuccess {
		return
	}
	switch item.RuntimeStatus {
	case "failed":
		item.OpStatus = logtailmodel.LogAgentOperationStatusFailed
		item.Progress = -1
		if item.RuntimeMessage != "" {
			item.ProgressText = item.RuntimeMessage
			if strings.TrimSpace(item.ErrorMsg) == "" {
				item.ErrorMsg = item.RuntimeMessage
			}
		}
	case "pending":
		item.OpStatus = logtailmodel.LogAgentOperationStatusRunning
		if item.RuntimeProgress != 0 {
			item.Progress = item.RuntimeProgress
		}
		if item.RuntimeMessage != "" {
			item.ProgressText = item.RuntimeMessage
		}
	case "running", "not_installed":
		if item.RuntimeProgress != 0 {
			item.Progress = item.RuntimeProgress
		}
		if item.RuntimeMessage != "" {
			item.ProgressText = item.RuntimeMessage
		}
	}
}

func (s *LogtailService) createKubernetesClusterCollectorOperation(cluster k8smodel.KubeCluster, opType, operator string, req logtailmodel.LogK8SClusterCollectorActionRequest) (*logtailmodel.LogK8SClusterCollectorOperation, error) {
	requestPayload, _ := json.Marshal(map[string]interface{}{
		"clusterId":   cluster.ID,
		"opType":      opType,
		"operator":    operator,
		"force":       req.Force,
		"vectorImage": kubernetesClusterCollectorRequestImage(req),
	})
	item := &logtailmodel.LogK8SClusterCollectorOperation{
		ClusterID:    cluster.ID,
		ClusterName:  cluster.Name,
		OpType:       opType,
		OpStatus:     logtailmodel.LogAgentOperationStatusPending,
		Progress:     0,
		ProgressText: "等待执行",
		RequestJSON:  string(requestPayload),
		Operator:     operator,
	}
	if err := s.k8sClusterDAO.Create(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *LogtailService) executeKubernetesClusterCollectorAction(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, opType, vectorImage string) error {
	switch opType {
	case logtailmodel.LogK8SCollectorOperationInstall, logtailmodel.LogK8SCollectorOperationUpdate:
		return s.applyKubernetesClusterCollector(ctx, clientset, cluster, vectorImage)
	case logtailmodel.LogK8SCollectorOperationReinstall:
		if err := s.uninstallKubernetesClusterCollector(ctx, clientset); err != nil {
			return err
		}
		return s.applyKubernetesClusterCollector(ctx, clientset, cluster, vectorImage)
	case logtailmodel.LogK8SCollectorOperationUninstall:
		return s.uninstallKubernetesClusterCollector(ctx, clientset)
	default:
		return fmt.Errorf("不支持的K8S集群Collector操作: %s", opType)
	}
}

func (s *LogtailService) applyKubernetesClusterCollector(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, vectorImage string) error {
	if err := ensureKubernetesNamespace(ctx, clientset, k8sVectorNamespace); err != nil {
		return err
	}
	resolvedImage := s.resolveKubernetesClusterCollectorImage(ctx, clientset, vectorImage)
	runtimeVersion := detectKubernetesClusterCollectorRuntimeVersion(ctx, clientset)
	legacyHTTPSink := shouldUseLegacyHTTPSinkConfig(firstNonEmpty(runtimeVersion, resolvedImage))
	bundle, err := s.renderKubernetesClusterVectorConfigBundle(ctx, clientset, cluster, legacyHTTPSink)
	if err != nil {
		return err
	}
	labels := kubernetesClusterCollectorLabels()
	if err := applyKubernetesVectorConfigBundle(ctx, clientset, bundle, labels); err != nil {
		return err
	}
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k8sClusterCollectorName,
			Namespace: k8sVectorNamespace,
			Labels:    labels,
		},
	}
	if err := applyServiceAccount(ctx, clientset, serviceAccount); err != nil {
		return err
	}
	daemonSet := buildKubernetesStdoutDaemonSet(k8sClusterCollectorName, labels, bundle.hash(), resolvedImage, kubernetesVectorConfigRefs(bundle)...)
	if err := applyDaemonSet(ctx, clientset, daemonSet); err != nil {
		return err
	}
	if err := deleteConfigMapIfExists(ctx, clientset, k8sVectorNamespace, k8sClusterCollectorName); err != nil {
		return err
	}
	return deleteStaleKubernetesProjectConfigMaps(ctx, clientset, bundle)
}

func (s *LogtailService) uninstallKubernetesClusterCollector(ctx context.Context, clientset *kubernetes.Clientset) error {
	if err := deleteDaemonSetIfExists(ctx, clientset, k8sVectorNamespace, k8sClusterCollectorName); err != nil {
		return err
	}
	if err := deleteConfigMapIfExists(ctx, clientset, k8sVectorNamespace, k8sClusterCollectorName); err != nil {
		return err
	}
	if err := deleteConfigMapIfExists(ctx, clientset, k8sVectorNamespace, k8sVectorBaseConfigMapName); err != nil {
		return err
	}
	if err := deleteKubernetesProjectConfigMaps(ctx, clientset); err != nil {
		return err
	}
	return deleteServiceAccountIfExists(ctx, clientset, k8sVectorNamespace, k8sClusterCollectorName)
}

func (s *LogtailService) renderKubernetesClusterVectorConfigBundle(ctx context.Context, clientset *kubernetes.Clientset, cluster *k8smodel.KubeCluster, legacyHTTPSink bool) (kubernetesClusterVectorConfigBundle, error) {
	stdoutTargets, err := s.listKubernetesStdoutTaskTargetsForCluster(ctx, clientset, cluster.ID)
	if err != nil {
		return kubernetesClusterVectorConfigBundle{}, err
	}
	nodeFileTargets, err := s.listKubernetesNodeFileTaskTargetsForCluster(cluster.ID)
	if err != nil {
		return kubernetesClusterVectorConfigBundle{}, err
	}
	sortKubernetesPipelineTargets(stdoutTargets, nodeFileTargets)

	bundle := kubernetesClusterVectorConfigBundle{
		Base: kubernetesVectorConfigFile{
			Scope:     k8sVectorConfigScopeBase,
			ConfigMap: k8sVectorBaseConfigMapName,
			FileName:  k8sVectorBaseConfigFile,
			Content:   renderKubernetesVectorBaseConfig(stdoutTargets, len(stdoutTargets)+len(nodeFileTargets) == 0),
		},
	}

	grouped := make(map[uint]*kubernetesProjectPipelineTargets)
	for _, item := range stdoutTargets {
		group := grouped[item.Task.ProjectID]
		if group == nil {
			group = &kubernetesProjectPipelineTargets{}
			grouped[item.Task.ProjectID] = group
		}
		group.Stdout = append(group.Stdout, item)
	}
	for _, item := range nodeFileTargets {
		group := grouped[item.Task.ProjectID]
		if group == nil {
			group = &kubernetesProjectPipelineTargets{}
			grouped[item.Task.ProjectID] = group
		}
		group.NodeFile = append(group.NodeFile, item)
	}

	projectIDs := make([]uint, 0, len(grouped))
	for projectID := range grouped {
		projectIDs = append(projectIDs, projectID)
	}
	sort.Slice(projectIDs, func(i, j int) bool { return projectIDs[i] < projectIDs[j] })
	for _, projectID := range projectIDs {
		project, err := s.projectDAO.GetByID(projectID)
		if err != nil {
			return kubernetesClusterVectorConfigBundle{}, fmt.Errorf("查询日志项目失败 projectId=%d: %w", projectID, err)
		}
		content, err := s.renderKubernetesProjectVectorConfig(cluster, project, *grouped[projectID], legacyHTTPSink)
		if err != nil {
			return kubernetesClusterVectorConfigBundle{}, err
		}
		bundle.Projects = append(bundle.Projects, kubernetesVectorConfigFile{
			Scope:     k8sVectorConfigScopeProject,
			Project:   project,
			ConfigMap: kubernetesProjectConfigMapName(project),
			FileName:  kubernetesProjectConfigFileName(project),
			Content:   content,
		})
	}
	return bundle, nil
}

func renderKubernetesVectorBaseConfig(stdoutTargets []kubernetesStdoutPipelineTarget, empty bool) string {
	sourceID := "k8s_stdout_file"
	var builder strings.Builder
	builder.WriteString("data_dir: /var/lib/aiops-vector\n\n")
	if len(stdoutTargets) > 0 {
		builder.WriteString("sources:\n")
		builder.WriteString(fmt.Sprintf("  %s:\n", sourceID))
		builder.WriteString("    type: file\n")
		builder.WriteString("    include:\n")
		for _, pattern := range kubernetesStdoutSourceIncludePatterns(stdoutTargets) {
			builder.WriteString(fmt.Sprintf("      - %s\n", yamlQuote(pattern)))
		}
		writeVectorFileRotationSettings(&builder)
	}
	if empty {
		builder.WriteString("sources:\n")
		builder.WriteString("  k8s_empty_file:\n")
		builder.WriteString("    type: file\n")
		builder.WriteString("    include:\n")
		builder.WriteString("      - /var/log/containers/.aiops-empty-*.log\n")
		builder.WriteString("    read_from: end\n\n")
		builder.WriteString("sinks:\n")
		builder.WriteString("  k8s_empty_blackhole:\n")
		builder.WriteString("    type: blackhole\n")
		builder.WriteString("    inputs:\n")
		builder.WriteString("      - k8s_empty_file\n")
	}
	return builder.String()
}

func (s *LogtailService) renderKubernetesProjectVectorConfig(cluster *k8smodel.KubeCluster, project *logtailmodel.LogProject, targets kubernetesProjectPipelineTargets, legacyHTTPSink bool) (string, error) {
	const sourceID = "k8s_stdout_file"
	var builder strings.Builder
	if len(targets.NodeFile) > 0 {
		builder.WriteString("sources:\n")
	}
	for _, item := range targets.NodeFile {
		task := item.Task
		nodeSourceID := vectorComponentID(task, fmt.Sprintf("k8s_node_file_%d", item.Target.Index))
		builder.WriteString(fmt.Sprintf("  %s:\n", nodeSourceID))
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
	}
	if len(targets.NodeFile) > 0 {
		builder.WriteString("\n")
	}

	builder.WriteString("transforms:\n")
	for _, item := range targets.Stdout {
		task := item.Task
		target := item.Target.Target
		targetIndex := item.Target.Index
		store, err := s.storeDAO.GetByID(task.StoreID)
		if err != nil {
			return "", fmt.Errorf("查询日志库失败 task=%s: %w", task.Code, err)
		}
		template, _ := s.templateDAO.GetByID(task.ParseTemplateID)
		filterID := vectorComponentID(task, fmt.Sprintf("k8s_stdout_filter_%d", targetIndex))
		parseID := vectorComponentID(task, fmt.Sprintf("k8s_stdout_parse_%d", targetIndex))

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
		stdoutTask := task
		stdoutTask.K8SLogSource = logtailmodel.LogK8SLogSourceStdout
		writeVectorParseScript(&builder, stdoutTask, template)
		writeVectorKubernetesCommonFields(&builder, cluster, stdoutTask, project, store, target)
	}
	for _, item := range targets.NodeFile {
		task := item.Task
		target := item.Target.Target
		targetIndex := item.Target.Index
		store, err := s.storeDAO.GetByID(task.StoreID)
		if err != nil {
			return "", fmt.Errorf("查询日志库失败 task=%s: %w", task.Code, err)
		}
		template, _ := s.templateDAO.GetByID(task.ParseTemplateID)
		nodeSourceID := vectorComponentID(task, fmt.Sprintf("k8s_node_file_%d", targetIndex))
		parseID := vectorComponentID(task, fmt.Sprintf("k8s_node_file_parse_%d", targetIndex))

		builder.WriteString(fmt.Sprintf("  %s:\n", parseID))
		builder.WriteString("    type: remap\n")
		builder.WriteString("    inputs:\n")
		builder.WriteString(fmt.Sprintf("      - %s\n", nodeSourceID))
		builder.WriteString("    source: |\n")
		nodeTask := task
		nodeTask.K8SLogSource = logtailmodel.LogK8SLogSourceNodeFile
		writeKubernetesNodeFileUnpackScript(&builder)
		writeVectorParseScript(&builder, nodeTask, template)
		writeVectorKubernetesCommonFields(&builder, cluster, nodeTask, project, store, target)
	}

	builder.WriteString("\nsinks:\n")
	for _, item := range targets.Stdout {
		task := item.Task
		parseID := vectorComponentID(task, fmt.Sprintf("k8s_stdout_parse_%d", item.Target.Index))
		sinkID := vectorComponentID(task, fmt.Sprintf("k8s_stdout_delivery_%d", item.Target.Index))
		if err := writeKubernetesVectorDeliverySink(&builder, sinkID, parseID, legacyHTTPSink); err != nil {
			return "", err
		}
	}
	for _, item := range targets.NodeFile {
		task := item.Task
		parseID := vectorComponentID(task, fmt.Sprintf("k8s_node_file_parse_%d", item.Target.Index))
		sinkID := vectorComponentID(task, fmt.Sprintf("k8s_node_file_delivery_%d", item.Target.Index))
		if err := writeKubernetesVectorDeliverySink(&builder, sinkID, parseID, legacyHTTPSink); err != nil {
			return "", err
		}
	}
	return builder.String(), nil
}

func sortKubernetesPipelineTargets(stdout []kubernetesStdoutPipelineTarget, nodeFile []kubernetesNodeFilePipelineTarget) {
	sort.Slice(stdout, func(i, j int) bool {
		if stdout[i].Task.ProjectID != stdout[j].Task.ProjectID {
			return stdout[i].Task.ProjectID < stdout[j].Task.ProjectID
		}
		if stdout[i].Task.ID != stdout[j].Task.ID {
			return stdout[i].Task.ID < stdout[j].Task.ID
		}
		return stdout[i].Target.Index < stdout[j].Target.Index
	})
	sort.Slice(nodeFile, func(i, j int) bool {
		if nodeFile[i].Task.ProjectID != nodeFile[j].Task.ProjectID {
			return nodeFile[i].Task.ProjectID < nodeFile[j].Task.ProjectID
		}
		if nodeFile[i].Task.ID != nodeFile[j].Task.ID {
			return nodeFile[i].Task.ID < nodeFile[j].Task.ID
		}
		return nodeFile[i].Target.Index < nodeFile[j].Target.Index
	})
}

func kubernetesProjectConfigMapName(project *logtailmodel.LogProject) string {
	if project == nil {
		return "aiops-vector-unknown"
	}
	return k8sResourceName(
		"aiops-vector",
		kubernetesProjectNameSegment(project),
		strconv.FormatUint(uint64(project.ID), 10),
	)
}

func kubernetesLegacyProjectConfigMapName(projectID uint) string {
	return k8sVectorLegacyProjectPrefix + strconv.FormatUint(uint64(projectID), 10)
}

func kubernetesProjectConfigFileName(project *logtailmodel.LogProject) string {
	if project == nil {
		return "10-project-unknown.yaml"
	}
	return k8sResourceName(
		"10",
		kubernetesProjectNameSegment(project),
		strconv.FormatUint(uint64(project.ID), 10),
	) + ".yaml"
}

func kubernetesProjectNameSegment(project *logtailmodel.LogProject) string {
	if project == nil {
		return "project"
	}
	for _, candidate := range []string{project.Name, project.Code} {
		value := strings.ToLower(strings.TrimSpace(candidate))
		value = k8sNameInvalidChars.ReplaceAllString(value, "-")
		value = strings.Trim(value, "-")
		for strings.Contains(value, "--") {
			value = strings.ReplaceAll(value, "--", "-")
		}
		if value != "" {
			return value
		}
	}
	return "project"
}

func kubernetesVectorConfigRefs(bundle kubernetesClusterVectorConfigBundle) []kubernetesVectorConfigRef {
	files := bundle.files()
	refs := make([]kubernetesVectorConfigRef, 0, len(files))
	for _, file := range files {
		refs = append(refs, kubernetesVectorConfigRef{ConfigMap: file.ConfigMap, FileName: file.FileName})
	}
	return refs
}

func applyKubernetesVectorConfigBundle(ctx context.Context, clientset *kubernetes.Clientset, bundle kubernetesClusterVectorConfigBundle, collectorLabels map[string]string) error {
	for _, file := range bundle.files() {
		configLabels := mergeStringMaps(collectorLabels, map[string]string{k8sVectorConfigScopeLabel: file.Scope})
		annotations := map[string]string{
			"aiops.dev/config-file": file.FileName,
		}
		if file.Project != nil {
			configLabels[k8sVectorConfigProjectIDLabel] = strconv.FormatUint(uint64(file.Project.ID), 10)
			annotations["aiops.dev/log-project-name"] = file.Project.Name
			annotations["aiops.dev/log-project-code"] = file.Project.Code
		}
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:        file.ConfigMap,
				Namespace:   k8sVectorNamespace,
				Labels:      configLabels,
				Annotations: annotations,
			},
			Data: map[string]string{k8sVectorConfigFile: file.Content},
		}
		if err := applyConfigMap(ctx, clientset, configMap); err != nil {
			return fmt.Errorf("下发Vector配置失败 configMap=%s: %w", file.ConfigMap, err)
		}
	}
	return nil
}

func deleteStaleKubernetesProjectConfigMaps(ctx context.Context, clientset *kubernetes.Clientset, bundle kubernetesClusterVectorConfigBundle) error {
	desired := make(map[string]struct{}, len(bundle.Projects))
	for _, file := range bundle.Projects {
		desired[file.ConfigMap] = struct{}{}
	}
	return deleteKubernetesProjectConfigMapsExcept(ctx, clientset, desired)
}

func deleteKubernetesProjectConfigMaps(ctx context.Context, clientset *kubernetes.Clientset) error {
	return deleteKubernetesProjectConfigMapsExcept(ctx, clientset, nil)
}

func deleteKubernetesProjectConfigMapsExcept(ctx context.Context, clientset *kubernetes.Clientset, keep map[string]struct{}) error {
	selector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/managed-by": k8sCollectorManagedBy,
		k8sVectorConfigScopeLabel:      k8sVectorConfigScopeProject,
	}).String()
	items, err := clientset.CoreV1().ConfigMaps(k8sVectorNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("查询项目级Vector配置失败: %w", err)
	}
	for _, item := range items.Items {
		if _, ok := keep[item.Name]; ok {
			continue
		}
		if err := deleteConfigMapIfExists(ctx, clientset, k8sVectorNamespace, item.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *LogtailService) listKubernetesStdoutTaskTargetsForCluster(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) ([]kubernetesStdoutPipelineTarget, error) {
	tasks, err := s.taskDAO.ListKubernetesCollectors(logtailmodel.LogK8SCollectorQuery{
		ClusterID:    clusterID,
		K8SLogSource: logtailmodel.LogK8SLogSourceStdout,
	})
	if err != nil {
		return nil, err
	}
	items := make([]kubernetesStdoutPipelineTarget, 0)
	for i := range tasks {
		task := tasks[i]
		if task.Status != logtailmodel.LogStatusEnabled {
			continue
		}
		if !kubernetesLogSourceIncludes(task.K8SLogSource, logtailmodel.LogK8SLogSourceStdout) {
			continue
		}
		targets, err := parseIndexedKubernetesCollectTargets(task.ExecuteTargetsJSON)
		if err != nil {
			continue
		}
		for _, target := range targets {
			if target.Target.ClusterID != clusterID {
				continue
			}
			resolved, err := resolveKubernetesWorkloadContainers(ctx, clientset, target.Target)
			if err != nil {
				return nil, fmt.Errorf("自动解析工作负载容器失败 task=%s workload=%s/%s: %w", task.Code, target.Target.Namespace, target.Target.WorkloadName, err)
			}
			target.Target = resolved
			items = append(items, kubernetesStdoutPipelineTarget{
				Task:   task,
				Target: target,
			})
		}
	}
	return items, nil
}

func (s *LogtailService) listKubernetesNodeFileTaskTargetsForCluster(clusterID uint) ([]kubernetesNodeFilePipelineTarget, error) {
	tasks, err := s.taskDAO.ListKubernetesCollectors(logtailmodel.LogK8SCollectorQuery{
		ClusterID:    clusterID,
		K8SLogSource: logtailmodel.LogK8SLogSourceNodeFile,
	})
	if err != nil {
		return nil, err
	}
	items := make([]kubernetesNodeFilePipelineTarget, 0)
	for i := range tasks {
		task := tasks[i]
		if task.Status != logtailmodel.LogStatusEnabled {
			continue
		}
		if !kubernetesLogSourceIncludes(task.K8SLogSource, logtailmodel.LogK8SLogSourceNodeFile) {
			continue
		}
		if len(collectTaskPathPatterns(&task)) == 0 {
			continue
		}
		targets, err := parseIndexedKubernetesCollectTargetsBySource(task.ExecuteTargetsJSON, logtailmodel.LogK8SLogSourceNodeFile)
		if err != nil {
			continue
		}
		for _, target := range targets {
			if target.Target.ClusterID != clusterID {
				continue
			}
			items = append(items, kubernetesNodeFilePipelineTarget{
				Task:   task,
				Target: target,
			})
		}
	}
	return items, nil
}

func (s *LogtailService) buildKubernetesClusterCollectorStatus(ctx context.Context, clientset *kubernetes.Clientset, cluster k8smodel.KubeCluster, latestOp logtailmodel.LogK8SClusterCollectorOperation) logtailmodel.LogK8SClusterCollectorVO {
	item := baseKubernetesClusterCollectorVO(cluster, latestOp, "not_installed", 0, "未安装")
	daemonSet, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		item.Events = listKubernetesWarningEvents(ctx, clientset, k8sVectorNamespace, k8sClusterCollectorName)
		latestOp = s.reconcileKubernetesClusterCollectorOperation(latestOp, item)
		applyActiveClusterOperation(&item, latestOp)
		return item
	}
	if err != nil {
		item.Status = "failed"
		item.Progress = -1
		item.ProgressText = "查询K8S集群Collector失败"
		item.LastError = err.Error()
		now := time.Now()
		item.LastErrorAt = &now
		applyActiveClusterOperation(&item, latestOp)
		return item
	}
	item.Desired = daemonSet.Status.DesiredNumberScheduled
	item.Ready = daemonSet.Status.NumberReady
	item.Updated = daemonSet.Status.UpdatedNumberScheduled
	item.Available = daemonSet.Status.NumberAvailable
	if daemonSet.Spec.Template.Annotations != nil {
		item.ConfigHash = daemonSet.Spec.Template.Annotations["aiops.dev/config-hash"]
	}
	for _, container := range daemonSet.Spec.Template.Spec.Containers {
		if container.Name == "vector" {
			item.VectorImage = container.Image
			break
		}
	}
	item.Pods = listKubernetesPods(ctx, clientset, k8sVectorNamespace, labels.SelectorFromSet(labels.Set{"app": k8sClusterCollectorName}).String())
	item.Events = listKubernetesWarningEventsForPods(ctx, clientset, k8sVectorNamespace, item.Pods)
	item.Status, item.Progress, item.ProgressText = determineKubernetesCollectorProgress(item.Desired, item.Ready, item.Updated, item.Available, firstPodError(item.Pods))
	item.VectorStatus = item.Status
	switch item.Status {
	case "running":
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusApplied
	case "failed":
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusFailed
	default:
		item.ConfigStatus = logtailmodel.LogAgentConfigStatusPending
	}
	if errMsg := firstPodError(item.Pods); errMsg != "" {
		item.LastError = errMsg
		now := time.Now()
		item.LastErrorAt = &now
	}
	latestOp = s.reconcileKubernetesClusterCollectorOperation(latestOp, item)
	applyActiveClusterOperation(&item, latestOp)
	return item
}

func (s *LogtailService) reconcileKubernetesClusterCollectorOperation(latestOp logtailmodel.LogK8SClusterCollectorOperation, item logtailmodel.LogK8SClusterCollectorVO) logtailmodel.LogK8SClusterCollectorOperation {
	if latestOp.ID == 0 || !isActiveOperation(latestOp.OpStatus) {
		return latestOp
	}
	status := strings.TrimSpace(item.Status)
	if status != "running" && status != "failed" && status != "not_installed" {
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

	opStatus := logtailmodel.LogAgentOperationStatusSuccess
	progress := item.Progress
	if progress <= 0 {
		progress = 100
	}
	progressText := kubernetesClusterOperationSuccessMessage(latestOp.OpType)
	errorMsg := ""
	if status == "failed" {
		opStatus = logtailmodel.LogAgentOperationStatusFailed
		progress = -1
		progressText = firstNonEmpty(item.LastError, item.ProgressText, "K8S集群Collector运行失败")
		errorMsg = progressText
	} else if item.ProgressText != "" && item.ProgressText != "运行中" {
		progressText = item.ProgressText
	}

	updated, err := s.k8sClusterDAO.UpdateProgress(latestOp.ID, opStatus, progress, progressText, resultJSON, errorMsg)
	if err != nil || updated == nil {
		return latestOp
	}
	hydrateKubernetesClusterCollectorOperationRuntime(updated)
	return *updated
}

func (s *LogtailService) fillKubernetesClusterCollectorConfig(ctx context.Context, clientset *kubernetes.Clientset, item *logtailmodel.LogK8SClusterCollectorVO) {
	if item == nil {
		return
	}
	configFiles := make([]logtailmodel.LogK8SVectorConfigFile, 0)
	configMap, err := clientset.CoreV1().ConfigMaps(k8sVectorNamespace).Get(ctx, k8sVectorBaseConfigMapName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		configMap, err = clientset.CoreV1().ConfigMaps(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	}
	if err == nil {
		if configFile, ok := kubernetesVectorConfigFileModel(configMap); ok {
			configFiles = append(configFiles, configFile)
		}
	}
	selector := labels.SelectorFromSet(labels.Set{
		"app.kubernetes.io/managed-by": k8sCollectorManagedBy,
		k8sVectorConfigScopeLabel:      k8sVectorConfigScopeProject,
	}).String()
	if configMaps, err := clientset.CoreV1().ConfigMaps(k8sVectorNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector}); err == nil {
		for index := range configMaps.Items {
			if configFile, ok := kubernetesVectorConfigFileModel(&configMaps.Items[index]); ok {
				configFiles = append(configFiles, configFile)
			}
		}
	}
	sort.Slice(configFiles, func(i, j int) bool {
		if configFiles[i].Scope != configFiles[j].Scope {
			return configFiles[i].Scope == k8sVectorConfigScopeBase
		}
		if configFiles[i].ProjectID != configFiles[j].ProjectID {
			return configFiles[i].ProjectID < configFiles[j].ProjectID
		}
		return configFiles[i].FileName < configFiles[j].FileName
	})
	item.ConfigFiles = configFiles
	item.ConfigYAML = mergeKubernetesVectorConfigFiles(configFiles)
}

func kubernetesVectorConfigFileModel(configMap *corev1.ConfigMap) (logtailmodel.LogK8SVectorConfigFile, bool) {
	if configMap == nil || configMap.Data == nil {
		return logtailmodel.LogK8SVectorConfigFile{}, false
	}
	content := configMap.Data[k8sVectorConfigFile]
	if strings.TrimSpace(content) == "" {
		return logtailmodel.LogK8SVectorConfigFile{}, false
	}
	scope := configMap.Labels[k8sVectorConfigScopeLabel]
	if scope == "" {
		scope = k8sVectorConfigScopeBase
	}
	projectID, _ := strconv.ParseUint(configMap.Labels[k8sVectorConfigProjectIDLabel], 10, 64)
	fileName := configMap.Annotations["aiops.dev/config-file"]
	if fileName == "" {
		if projectID > 0 {
			fileName = kubernetesProjectConfigFileName(&logtailmodel.LogProject{
				ID:   uint(projectID),
				Name: configMap.Annotations["aiops.dev/log-project-name"],
				Code: configMap.Annotations["aiops.dev/log-project-code"],
			})
		} else {
			fileName = k8sVectorBaseConfigFile
		}
	}
	return logtailmodel.LogK8SVectorConfigFile{
		Scope:              scope,
		ProjectID:          uint(projectID),
		ProjectName:        configMap.Annotations["aiops.dev/log-project-name"],
		ProjectCode:        configMap.Annotations["aiops.dev/log-project-code"],
		ConfigMapNamespace: configMap.Namespace,
		ConfigMapName:      configMap.Name,
		FileName:           fileName,
		ConfigHash:         shortConfigHash(content),
		ConfigYAML:         content,
	}, true
}

func mergeKubernetesVectorConfigFiles(files []logtailmodel.LogK8SVectorConfigFile) string {
	var builder strings.Builder
	for _, file := range files {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(fmt.Sprintf("# %s (ConfigMap: %s)\n", file.FileName, file.ConfigMapName))
		builder.WriteString(strings.TrimSpace(file.ConfigYAML))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func baseKubernetesClusterCollectorVO(cluster k8smodel.KubeCluster, latestOp logtailmodel.LogK8SClusterCollectorOperation, status string, progress int, progressText string) logtailmodel.LogK8SClusterCollectorVO {
	item := logtailmodel.LogK8SClusterCollectorVO{
		ClusterID:         cluster.ID,
		ClusterName:       cluster.Name,
		ClusterStatus:     cluster.Status,
		ResourceKind:      "DaemonSet",
		ResourceNamespace: k8sVectorNamespace,
		ResourceName:      k8sClusterCollectorName,
		Status:            status,
		Progress:          progress,
		ProgressText:      progressText,
		ConfigStatus:      logtailmodel.LogAgentConfigStatusPending,
		VectorStatus:      status,
		VectorImage:       configuredKubernetesVectorImage(),
		UpdatedAt:         cluster.UpdatedAt.Format(time.RFC3339),
	}
	if latestOp.ID > 0 {
		hydrateKubernetesClusterCollectorOperationRuntime(&latestOp)
		item.LastOperation = &latestOp
	}
	return item
}

func applyActiveClusterOperation(item *logtailmodel.LogK8SClusterCollectorVO, latestOp logtailmodel.LogK8SClusterCollectorOperation) {
	if latestOp.ID == 0 {
		return
	}
	rawOpStatus := latestOp.OpStatus
	hydrateKubernetesClusterCollectorOperationRuntime(&latestOp)
	item.LastOperation = &latestOp
	if !isActiveOperation(rawOpStatus) {
		return
	}
	item.Status = "pending"
	item.VectorStatus = "pending"
	item.Progress = latestOp.Progress
	item.ProgressText = latestOp.ProgressText
}

func summarizeKubernetesClusterCollectors(items []logtailmodel.LogK8SClusterCollectorVO) logtailmodel.LogK8SCollectorStats {
	var stats logtailmodel.LogK8SCollectorStats
	stats.Total = int64(len(items))
	for _, item := range items {
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

func listKubernetesClustersForCollector(query logtailmodel.LogK8SClusterCollectorQuery) ([]k8smodel.KubeCluster, error) {
	var clusters []k8smodel.KubeCluster
	db := common.GetDB().Model(&k8smodel.KubeCluster{})
	if query.ClusterID > 0 {
		db = db.Where("id = ?", query.ClusterID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	err := db.Order("updated_at DESC, id DESC").Find(&clusters).Error
	return clusters, err
}

func kubernetesClusterCollectorLabels() map[string]string {
	return map[string]string{
		"app":                          k8sClusterCollectorName,
		"app.kubernetes.io/name":       k8sClusterCollectorName,
		"app.kubernetes.io/instance":   k8sClusterCollectorName,
		"app.kubernetes.io/managed-by": k8sCollectorManagedBy,
		"aiops.dev/collector-scope":    "cluster",
	}
}

func kubernetesClusterCollectorRequestImage(req logtailmodel.LogK8SClusterCollectorActionRequest) string {
	return strings.TrimSpace(firstNonEmpty(req.VectorImage, req.Image))
}

func (s *LogtailService) resolveKubernetesClusterCollectorImage(ctx context.Context, clientset *kubernetes.Clientset, requestedImage string) string {
	if image := strings.TrimSpace(requestedImage); image != "" {
		return image
	}
	daemonSet, err := clientset.AppsV1().DaemonSets(k8sVectorNamespace).Get(ctx, k8sClusterCollectorName, metav1.GetOptions{})
	if err == nil {
		for _, container := range daemonSet.Spec.Template.Spec.Containers {
			if container.Name == "vector" && strings.TrimSpace(container.Image) != "" {
				return strings.TrimSpace(container.Image)
			}
		}
	}
	return configuredKubernetesVectorImage()
}

func detectKubernetesClusterCollectorRuntimeVersion(ctx context.Context, clientset *kubernetes.Clientset) string {
	if clientset == nil {
		return ""
	}
	pods, err := clientset.CoreV1().Pods(k8sVectorNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"app": k8sClusterCollectorName}).String(),
	})
	if err != nil {
		return ""
	}
	for _, pod := range pods.Items {
		if version := detectKubernetesVectorVersionFromPodLogs(ctx, clientset, pod.Name); version != "" {
			return version
		}
	}
	return ""
}

func detectKubernetesVectorVersionFromPodLogs(ctx context.Context, clientset *kubernetes.Clientset, podName string) string {
	for _, previous := range []bool{false, true} {
		logText := fetchKubernetesPodLogTail(ctx, clientset, k8sVectorNamespace, podName, "vector", previous)
		if version := detectKubernetesVectorVersionFromLog(logText); version != "" {
			return version
		}
	}
	return ""
}

func detectKubernetesVectorVersionFromLog(logText string) string {
	matches := vectorSemverRegexp.FindStringSubmatch(logText)
	if len(matches) != 4 {
		return ""
	}
	return matches[0]
}

func kubernetesClusterOperationSuccessMessage(opType string) string {
	switch opType {
	case logtailmodel.LogK8SCollectorOperationInstall:
		return "已安装 K8S 集群级 Vector"
	case logtailmodel.LogK8SCollectorOperationUpdate:
		return "已更新 K8S 集群级 Vector 配置"
	case logtailmodel.LogK8SCollectorOperationUninstall:
		return "已卸载 K8S 集群级 Vector"
	case logtailmodel.LogK8SCollectorOperationReinstall:
		return "已重装 K8S 集群级 Vector"
	default:
		return "K8S 集群级 Vector 操作完成"
	}
}
