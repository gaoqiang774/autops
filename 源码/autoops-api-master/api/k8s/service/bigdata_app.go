package service

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"dodevops-api/api/k8s/dao"
	"dodevops-api/api/k8s/model"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

type IBigDataAppService interface {
	GetComponentTree(c *gin.Context, clusterID uint, namespace string)
	GetComponentDetail(c *gin.Context, clusterID uint, namespace, component string)
	GetComponentMetrics(c *gin.Context, clusterID uint, namespace, component string)
	OperateComponent(c *gin.Context, clusterID uint, namespace, component string, req *model.BigDataActionRequest)
	OperateWorkload(c *gin.Context, clusterID uint, namespace, workloadType, workloadName string, req *model.BigDataActionRequest)
	OperatePod(c *gin.Context, clusterID uint, namespace, podName string, req *model.BigDataActionRequest)
	GetWorkloadYAML(c *gin.Context, clusterID uint, namespace, workloadType, workloadName string)
	ApplyWorkloadYAML(c *gin.Context, clusterID uint, namespace string, req *model.UpdateWorkloadYAMLRequest)
	CreatePodFromYAML(c *gin.Context, clusterID uint, namespace string, req *model.CreatePodFromYAMLRequest)
}

type BigDataAppServiceImpl struct {
	clusterDao      *dao.KubeClusterDao
	workloadService *K8sWorkloadServiceImpl
}

func NewBigDataAppService(db *gorm.DB) IBigDataAppService {
	return &BigDataAppServiceImpl{
		clusterDao: dao.NewKubeClusterDao(db),
		workloadService: &K8sWorkloadServiceImpl{
			clusterDao:   dao.NewKubeClusterDao(db),
			cacheService: newK8sCacheService(),
		},
	}
}

func defaultBigDataComponentRules() []model.BigDataComponentRule {
	return []model.BigDataComponentRule{
		{Name: "dolphinscheduler", DisplayName: "DolphinScheduler", Category: "调度", Prefixes: []string{"dolphinscheduler-"}},
		{Name: "hdfs", DisplayName: "HDFS", Category: "存储", Prefixes: []string{"hadoop-datanode-132-5", "hadoop-datanode-41", "hadoop-namenode-nn1", "hadoop-namenode-nn2"}},
		{Name: "yarn", DisplayName: "YARN", Category: "资源调度", Prefixes: []string{"hadoop-nodemanager-", "hadoop-resourcemanager-"}},
		{Name: "hadoop-historyserver", DisplayName: "Hadoop-Historyserver", Category: "历史服务", Prefixes: []string{"hadoop-historyserver-"}},
		{Name: "hive", DisplayName: "Hive", Category: "数据仓库", Prefixes: []string{"hive-"}},
		{Name: "journalnode", DisplayName: "JournalNode", Category: "存储", Prefixes: []string{"journalnode-"}},
		{Name: "spark-historyserver", DisplayName: "Spark-Historyserver", Category: "计算", Prefixes: []string{"spark-historyserver-"}},
		{Name: "streampark", DisplayName: "StreamPark", Category: "流处理", Prefixes: []string{"streampark-"}},
		{Name: "zookeeper", DisplayName: "ZooKeeper", Category: "协调服务", Prefixes: []string{"zookeeper-"}},
	}
}

func (s *BigDataAppServiceImpl) GetComponentTree(c *gin.Context, clusterID uint, namespace string) {
	data, err := s.loadBigDataAppData(c.Request.Context(), clusterID, namespace)
	if err != nil {
		s.writeLoadError(c, err)
		return
	}

	tree := buildBigDataTree(data)
	response := model.BigDataAppTreeResponse{
		ClusterID:   clusterID,
		ClusterName: data.cluster.Name,
		Namespace:   namespace,
		Summary:     buildBigDataSummary(data.components),
		Tree:        tree,
		Components:  data.components,
		Rules:       data.rules,
	}
	result.Success(c, response)
}

func (s *BigDataAppServiceImpl) GetComponentDetail(c *gin.Context, clusterID uint, namespace, component string) {
	data, err := s.loadBigDataAppData(c.Request.Context(), clusterID, namespace)
	if err != nil {
		s.writeLoadError(c, err)
		return
	}

	selected, ok := findBigDataComponent(data.components, component)
	if !ok {
		result.Failed(c, http.StatusNotFound, "组件不存在或当前命名空间下没有匹配资源")
		return
	}

	metrics, metricsByPod := s.collectComponentMetrics(c.Request.Context(), data.cluster.Credential, selected.Name, selected.Pods)
	response := model.BigDataComponentDetailResponse{
		ClusterID:  clusterID,
		Component:  selected,
		Containers: buildBigDataContainerDetails(selected.Pods, metricsByPod),
		Metrics:    metrics,
	}
	result.Success(c, response)
}

func (s *BigDataAppServiceImpl) GetComponentMetrics(c *gin.Context, clusterID uint, namespace, component string) {
	data, err := s.loadBigDataAppData(c.Request.Context(), clusterID, namespace)
	if err != nil {
		s.writeLoadError(c, err)
		return
	}

	selected, ok := findBigDataComponent(data.components, component)
	if !ok {
		result.Failed(c, http.StatusNotFound, "组件不存在或当前命名空间下没有匹配资源")
		return
	}

	metrics, _ := s.collectComponentMetrics(c.Request.Context(), data.cluster.Credential, selected.Name, selected.Pods)
	result.Success(c, metrics)
}

func (s *BigDataAppServiceImpl) OperateComponent(c *gin.Context, clusterID uint, namespace, component string, req *model.BigDataActionRequest) {
	data, err := s.loadBigDataAppData(c.Request.Context(), clusterID, namespace)
	if err != nil {
		s.writeLoadError(c, err)
		return
	}

	selected, ok := findBigDataComponent(data.components, component)
	if !ok {
		result.Failed(c, http.StatusNotFound, "组件不存在或当前命名空间下没有匹配资源")
		return
	}
	if data.cluster.Status != model.ClusterStatusRunning {
		result.Failed(c, http.StatusBadRequest, "集群状态异常，无法操作组件")
		return
	}

	clientset, err := s.workloadService.createK8sClient(data.cluster.Credential)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	resp := model.BigDataActionResponse{
		ClusterID: clusterID,
		Namespace: namespace,
		Component: selected.Name,
		Action:    normalizeBigDataAction(req.Action),
		Items:     []model.BigDataActionItem{},
	}

	action := normalizeBigDataAction(req.Action)
	switch action {
	case "quickRestart":
		targetPods := filterBigDataPodsByNames(selected.Pods, req.PodNames)
		for _, pod := range targetPods {
			resp.Items = append(resp.Items, s.operatePod(c.Request.Context(), clientset, pod.Namespace, pod.Name, req))
		}
	default:
		for _, workload := range selected.Workloads {
			resp.Items = append(resp.Items, s.operateWorkload(c.Request.Context(), clientset, workload.Namespace, string(workload.Type), workload.Name, req))
		}
		if len(selected.Workloads) == 0 {
			for _, pod := range selected.Pods {
				resp.Items = append(resp.Items, s.operatePod(c.Request.Context(), clientset, pod.Namespace, pod.Name, req))
			}
		}
	}

	resp.Success = actionItemsSucceeded(resp.Items)
	resp.Message = buildActionMessage(resp.Success, resp.Action, len(resp.Items))
	result.Success(c, resp)
}

func (s *BigDataAppServiceImpl) OperateWorkload(c *gin.Context, clusterID uint, namespace, workloadType, workloadName string, req *model.BigDataActionRequest) {
	cluster, clientset, ok := s.getClusterAndClient(c, clusterID)
	if !ok {
		return
	}
	if cluster.Status != model.ClusterStatusRunning {
		result.Failed(c, http.StatusBadRequest, "集群状态异常，无法操作工作负载")
		return
	}

	item := s.operateWorkload(c.Request.Context(), clientset, namespace, workloadType, workloadName, req)
	resp := model.BigDataActionResponse{
		ClusterID: clusterID,
		Namespace: namespace,
		Action:    normalizeBigDataAction(req.Action),
		Success:   item.Status == "success",
		Message:   item.Message,
		Items:     []model.BigDataActionItem{item},
	}
	result.Success(c, resp)
}

func (s *BigDataAppServiceImpl) OperatePod(c *gin.Context, clusterID uint, namespace, podName string, req *model.BigDataActionRequest) {
	cluster, clientset, ok := s.getClusterAndClient(c, clusterID)
	if !ok {
		return
	}
	if cluster.Status != model.ClusterStatusRunning {
		result.Failed(c, http.StatusBadRequest, "集群状态异常，无法操作Pod")
		return
	}

	item := s.operatePod(c.Request.Context(), clientset, namespace, podName, req)
	resp := model.BigDataActionResponse{
		ClusterID: clusterID,
		Namespace: namespace,
		Action:    normalizeBigDataAction(req.Action),
		Success:   item.Status == "success",
		Message:   item.Message,
		Items:     []model.BigDataActionItem{item},
	}
	result.Success(c, resp)
}

func (s *BigDataAppServiceImpl) GetWorkloadYAML(c *gin.Context, clusterID uint, namespace, workloadType, workloadName string) {
	s.workloadService.GetWorkloadYaml(c, clusterID, namespace, workloadType, workloadName)
}

func (s *BigDataAppServiceImpl) ApplyWorkloadYAML(c *gin.Context, clusterID uint, namespace string, req *model.UpdateWorkloadYAMLRequest) {
	s.workloadService.UpdateWorkloadYaml(c, clusterID, namespace, req)
}

func (s *BigDataAppServiceImpl) CreatePodFromYAML(c *gin.Context, clusterID uint, namespace string, req *model.CreatePodFromYAMLRequest) {
	s.workloadService.CreatePodFromYAML(c, clusterID, namespace, req)
}

type bigDataAppData struct {
	cluster    *model.KubeCluster
	rules      []model.BigDataComponentRule
	components []model.BigDataComponent
}

func (s *BigDataAppServiceImpl) loadBigDataAppData(ctx context.Context, clusterID uint, namespace string) (*bigDataAppData, error) {
	cluster, err := s.clusterDao.GetByID(clusterID)
	if err != nil {
		return nil, err
	}

	clientset, err := s.workloadService.createK8sClient(cluster.Credential)
	if err != nil {
		return nil, err
	}

	listCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	podList, err := clientset.CoreV1().Pods(namespace).List(listCtx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	workloads, err := s.workloadService.loadWorkloads(listCtx, clusterID, namespaceOrAll(namespace), "all")
	if err != nil {
		return nil, err
	}
	workloads = filterPodWorkloads(workloads)

	rules := defaultBigDataComponentRules()
	components := buildBigDataComponents(s.workloadService, rules, workloads, podList.Items)

	return &bigDataAppData{
		cluster:    cluster,
		rules:      rules,
		components: components,
	}, nil
}

func (s *BigDataAppServiceImpl) getClusterAndClient(c *gin.Context, clusterID uint) (*model.KubeCluster, *kubernetes.Clientset, bool) {
	cluster, err := s.clusterDao.GetByID(clusterID)
	if err != nil {
		s.writeLoadError(c, err)
		return nil, nil, false
	}
	clientset, err := s.workloadService.createK8sClient(cluster.Credential)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return nil, nil, false
	}
	return cluster, clientset, true
}

func (s *BigDataAppServiceImpl) writeLoadError(c *gin.Context, err error) {
	if err == gorm.ErrRecordNotFound {
		result.Failed(c, http.StatusNotFound, "集群不存在")
		return
	}
	result.Failed(c, http.StatusInternalServerError, "获取大数据应用信息失败: "+err.Error())
}

func buildBigDataComponents(workloadService *K8sWorkloadServiceImpl, rules []model.BigDataComponentRule, workloads []model.K8sWorkload, pods []corev1.Pod) []model.BigDataComponent {
	components := make(map[string]*model.BigDataComponent, len(rules)+1)
	for _, rule := range rules {
		components[rule.Name] = &model.BigDataComponent{
			Name:        rule.Name,
			DisplayName: rule.DisplayName,
			Category:    rule.Category,
			Status:      "Empty",
			Namespaces:  []string{},
			Workloads:   []model.K8sWorkload{},
			Pods:        []model.K8sPodInfo{},
		}
	}

	for _, workload := range workloads {
		rule, ok := matchBigDataComponentRule(rules, workload.Name)
		if !ok {
			continue
		}
		component := components[rule.Name]
		component.Workloads = append(component.Workloads, workload)
		addNamespace(component, workload.Namespace)
	}

	for i := range pods {
		pod := &pods[i]
		rule, ok := matchBigDataComponentRule(rules, pod.Name)
		if !ok {
			continue
		}
		component := components[rule.Name]
		component.Pods = append(component.Pods, workloadService.convertToPodInfo(pod))
		addNamespace(component, pod.Namespace)
	}

	items := make([]model.BigDataComponent, 0, len(components))
	for _, rule := range rules {
		component := components[rule.Name]
		sortWorkloads(component.Workloads)
		sort.SliceStable(component.Pods, func(i, j int) bool {
			if component.Pods[i].Namespace == component.Pods[j].Namespace {
				return component.Pods[i].Name < component.Pods[j].Name
			}
			return component.Pods[i].Namespace < component.Pods[j].Namespace
		})
		sort.Strings(component.Namespaces)
		component.Summary = buildComponentSummary(*component)
		component.Status = componentStatus(component.Summary)
		if component.Summary.Workloads > 0 || component.Summary.Pods > 0 {
			items = append(items, *component)
		}
	}

	return items
}

func addNamespace(component *model.BigDataComponent, namespace string) {
	if namespace == "" {
		return
	}
	for _, item := range component.Namespaces {
		if item == namespace {
			return
		}
	}
	component.Namespaces = append(component.Namespaces, namespace)
}

func buildComponentSummary(component model.BigDataComponent) model.BigDataComponentSummary {
	summary := model.BigDataComponentSummary{
		Workloads: len(component.Workloads),
		Pods:      len(component.Pods),
	}
	for _, workload := range component.Workloads {
		summary.DesiredReplicas += workload.DesiredReplicas
		if workload.DesiredReplicas == 0 {
			summary.DesiredReplicas += workload.Replicas
		}
		summary.CurrentReplicas += workload.CurrentReplicas
		summary.ReadyReplicas += workload.ReadyReplicas
	}
	for _, pod := range component.Pods {
		if pod.Status == "Running" {
			summary.RunningPods++
			summary.ReadyPods++
		} else if pod.Status != "Completed" {
			summary.AbnormalPods++
		}
		summary.RestartCount += pod.RestartCount
	}
	return summary
}

func componentStatus(summary model.BigDataComponentSummary) string {
	if summary.Workloads == 0 && summary.Pods == 0 {
		return "Empty"
	}
	if summary.AbnormalPods > 0 {
		if summary.ReadyPods > 0 {
			return "Partial"
		}
		return "Abnormal"
	}
	if summary.Pods > 0 && summary.ReadyPods == summary.Pods {
		return "Running"
	}
	return "Pending"
}

func buildBigDataSummary(components []model.BigDataComponent) model.BigDataAppSummary {
	namespaceSet := map[string]struct{}{}
	summary := model.BigDataAppSummary{Components: len(components)}
	for _, component := range components {
		for _, namespace := range component.Namespaces {
			namespaceSet[namespace] = struct{}{}
		}
		summary.Workloads += component.Summary.Workloads
		summary.Pods += component.Summary.Pods
		summary.ReadyPods += component.Summary.ReadyPods
		summary.AbnormalPods += component.Summary.AbnormalPods
		for _, workload := range component.Workloads {
			if workload.Status == "Running" || workload.Status == "Active" {
				summary.RunningWorkloads++
			} else {
				summary.AbnormalWorkloads++
			}
		}
	}
	summary.Namespaces = len(namespaceSet)
	return summary
}

func buildBigDataTree(data *bigDataAppData) []model.BigDataAppTreeNode {
	tree := make([]model.BigDataAppTreeNode, 0, len(data.components))

	for _, component := range data.components {
		componentNode := model.BigDataAppTreeNode{
			ID:        "component:" + component.Name,
			Label:     component.DisplayName,
			Type:      "component",
			Status:    component.Status,
			Component: component.Name,
			Children:  []model.BigDataAppTreeNode{},
			Meta: map[string]interface{}{
				"category":   component.Category,
				"namespaces": component.Namespaces,
				"summary":    component.Summary,
			},
		}

		if len(component.Namespaces) == 1 {
			componentNode.Namespace = component.Namespaces[0]
		}

		for _, workload := range component.Workloads {
			componentNode.Children = append(componentNode.Children, model.BigDataAppTreeNode{
				ID:           fmt.Sprintf("workload:%s:%s:%s", workload.Namespace, workload.Type, workload.Name),
				Label:        workload.Name,
				Type:         "workload",
				Status:       workload.Status,
				Namespace:    workload.Namespace,
				Component:    component.Name,
				WorkloadType: string(workload.Type),
				WorkloadName: workload.Name,
				Meta: map[string]interface{}{
					"replicas":          workload.Replicas,
					"readyReplicas":     workload.ReadyReplicas,
					"desiredReplicas":   workload.DesiredReplicas,
					"currentReplicas":   workload.CurrentReplicas,
					"availableReplicas": workload.AvailableReplicas,
				},
			})
		}

		sort.SliceStable(componentNode.Children, func(i, j int) bool {
			return componentNode.Children[i].Label < componentNode.Children[j].Label
		})
		tree = append(tree, componentNode)
	}

	sort.SliceStable(tree, func(i, j int) bool {
		return tree[i].Label < tree[j].Label
	})
	return tree
}

func filterPodWorkloads(workloads []model.K8sWorkload) []model.K8sWorkload {
	filtered := make([]model.K8sWorkload, 0, len(workloads))
	for _, workload := range workloads {
		if workload.Type == model.WorkloadTypePod {
			continue
		}
		filtered = append(filtered, workload)
	}
	return filtered
}

func matchBigDataComponentRule(rules []model.BigDataComponentRule, name string) (model.BigDataComponentRule, bool) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	for _, rule := range rules {
		for _, prefix := range rule.Prefixes {
			normalizedPrefix := strings.ToLower(strings.TrimSpace(prefix))
			if normalizedPrefix == "" {
				continue
			}
			if matchesBigDataNamePrefix(lowerName, normalizedPrefix) {
				return rule, true
			}
		}
	}
	return model.BigDataComponentRule{}, false
}

func matchesBigDataNamePrefix(name, prefix string) bool {
	if name == prefix {
		return true
	}
	if strings.HasSuffix(prefix, "-") {
		if strings.HasPrefix(name, prefix) {
			return true
		}
		baseName := strings.TrimSuffix(prefix, "-")
		return baseName != "" && name == baseName
	}
	return strings.HasPrefix(name, prefix+"-")
}

func findBigDataComponent(components []model.BigDataComponent, name string) (model.BigDataComponent, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, component := range components {
		if strings.ToLower(component.Name) == normalized || strings.ToLower(component.DisplayName) == normalized {
			return component, true
		}
	}
	return model.BigDataComponent{}, false
}

func (s *BigDataAppServiceImpl) collectComponentMetrics(ctx context.Context, kubeconfig, componentName string, pods []model.K8sPodInfo) (model.BigDataComponentMetrics, map[string]model.PodMetricsInfo) {
	response := model.BigDataComponentMetrics{
		Available: true,
		Pods:      []model.PodMetricsInfo{},
	}
	metricsByPod := map[string]model.PodMetricsInfo{}

	clientset, err := s.workloadService.createK8sClient(kubeconfig)
	if err != nil {
		response.Available = false
		response.Message = "连接K8s集群失败: " + err.Error()
		return response, metricsByPod
	}
	var historyByPod map[string]*model.ResourceMetricHistory
	response.History, historyByPod = s.buildComponentHistories(ctx, clientset, componentName, pods)

	metricsClient, err := s.workloadService.createMetricsClient(kubeconfig)
	if err != nil {
		response.Available = false
		response.Message = "连接Metrics服务失败: " + err.Error()
		return response, metricsByPod
	}

	var totalCPU, totalMemory resource.Quantity
	podsByNamespace := groupBigDataPodsByNamespace(pods)
	for namespace, namespacePods := range podsByNamespace {
		podList, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		podByName := map[string]*corev1.Pod{}
		for i := range podList.Items {
			pod := &podList.Items[i]
			podByName[pod.Name] = pod
		}

		podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		podMetricsByName := map[string]*metricsv1beta1.PodMetrics{}
		for i := range podMetricsList.Items {
			podMetrics := &podMetricsList.Items[i]
			podMetricsByName[podMetrics.Name] = podMetrics
		}

		for _, podInfo := range namespacePods {
			pod := podByName[podInfo.Name]
			if pod == nil {
				pod, err = getPodByNameAcrossNamespaces(ctx, clientset, podInfo.Name)
				if err != nil {
					continue
				}
			}
			podMetrics := podMetricsByName[podInfo.Name]
			if podMetrics == nil {
				continue
			}

			item := s.workloadService.convertToPodMetricsInfo(pod, podMetrics)
			if history := historyByPod[podMetricKey(podInfo.Namespace, podInfo.Name)]; history != nil {
				item.History = history
			}
			response.Pods = append(response.Pods, item)
			metricsByPod[pod.Name] = item
			for _, cm := range podMetrics.Containers {
				if cpu, ok := cm.Usage[corev1.ResourceCPU]; ok {
					totalCPU.Add(cpu)
				}
				if memory, ok := cm.Usage[corev1.ResourceMemory]; ok {
					totalMemory.Add(memory)
				}
			}
		}
	}

	if len(response.Pods) == 0 && len(pods) > 0 {
		response.Available = false
		response.Message = "未获取到组件Pod监控数据，请确认Metrics Server已安装"
	}
	response.TotalUsage = model.ResourceUsage{
		CPU:    formatCPUQuantity(totalCPU),
		Memory: formatMemoryQuantity(totalMemory),
	}
	return response, metricsByPod
}

func groupBigDataPodsByNamespace(pods []model.K8sPodInfo) map[string][]model.K8sPodInfo {
	grouped := map[string][]model.K8sPodInfo{}
	for _, pod := range pods {
		if pod.Name == "" || pod.Namespace == "" {
			continue
		}
		grouped[pod.Namespace] = append(grouped[pod.Namespace], pod)
	}
	return grouped
}

func (s *BigDataAppServiceImpl) buildComponentHistories(ctx context.Context, clientset *kubernetes.Clientset, componentName string, pods []model.K8sPodInfo) (*model.ResourceMetricHistory, map[string]*model.ResourceMetricHistory) {
	historyByPod := map[string]*model.ResourceMetricHistory{}
	if len(pods) == 0 {
		return nil, historyByPod
	}

	allCPU := map[string][]scraperMetricPoint{}
	allMemory := map[string][]scraperMetricPoint{}
	var total podResourceTotal

	for namespace, namespacePods := range groupBigDataPodsByNamespace(pods) {
		names := make([]string, 0, len(namespacePods))
		podByName := map[string]model.K8sPodInfo{}
		for _, pod := range namespacePods {
			names = append(names, pod.Name)
			podByName[pod.Name] = pod
			podTotal := podResourceTotalFromWorkloadResources(pod.Resources)
			total.CPUMilli += podTotal.CPUMilli
			total.MemoryBytes += podTotal.MemoryBytes
		}

		series := s.workloadService.fetchPodHistorySeries(ctx, clientset, namespace, names)
		for name, points := range series.CPU {
			allCPU[podMetricKey(namespace, name)] = points
		}
		for name, points := range series.Memory {
			allMemory[podMetricKey(namespace, name)] = points
		}

		for _, name := range names {
			pod := podByName[name]
			history := buildPodMetricHistory(namespace, name, series, podResourceTotalFromWorkloadResources(pod.Resources))
			if history != nil {
				historyByPod[podMetricKey(namespace, name)] = history
			}
		}
	}

	cpuRaw := sumMetricSeries(allCPU)
	memoryRaw := sumMetricSeries(allMemory)
	cpuPoints := convertRawPoints(cpuRaw, 0.001)
	memoryPoints := convertRawPoints(memoryRaw, 1.0/(1024*1024*1024))
	if len(cpuPoints) == 0 && len(memoryPoints) == 0 {
		return nil, historyByPod
	}

	return &model.ResourceMetricHistory{
		Name:            componentName,
		Kind:            "bigdata-component",
		CPU:             cpuPoints,
		Memory:          memoryPoints,
		CPUUsageRate:    ratePoints(cpuRaw, total.CPUMilli),
		MemoryUsageRate: ratePoints(memoryRaw, total.MemoryBytes),
	}, historyByPod
}

func buildBigDataContainerDetails(pods []model.K8sPodInfo, metricsByPod map[string]model.PodMetricsInfo) []model.BigDataContainerDetail {
	items := []model.BigDataContainerDetail{}
	for _, pod := range pods {
		metrics := metricsByPod[pod.Name]
		metricByContainer := map[string]model.ContainerMetricsInfo{}
		for _, containerMetric := range metrics.Containers {
			metricByContainer[containerMetric.Name] = containerMetric
		}
		for _, container := range pod.Containers {
			detail := model.BigDataContainerDetail{
				PodName:   pod.Name,
				Namespace: pod.Namespace,
				NodeName:  pod.NodeName,
				Container: container,
			}
			if metric, ok := metricByContainer[container.Name]; ok {
				detail.Metrics = &metric
			}
			items = append(items, detail)
		}
	}
	return items
}

func filterBigDataPodsByNames(pods []model.K8sPodInfo, names []string) []model.K8sPodInfo {
	if len(names) == 0 {
		return pods
	}
	allowed := map[string]struct{}{}
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	filtered := []model.K8sPodInfo{}
	for _, pod := range pods {
		if _, ok := allowed[pod.Name]; ok {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

func (s *BigDataAppServiceImpl) operateWorkload(ctx context.Context, clientset *kubernetes.Clientset, namespace, workloadType, workloadName string, req *model.BigDataActionRequest) model.BigDataActionItem {
	action := normalizeBigDataAction(req.Action)
	item := model.BigDataActionItem{
		Namespace:    namespace,
		ResourceType: strings.ToLower(workloadType),
		ResourceName: workloadName,
		Action:       action,
		Status:       "failed",
	}

	switch action {
	case "stop":
		prev, err := s.scaleWorkload(ctx, clientset, namespace, workloadType, workloadName, 0)
		item.PreviousReplicas = &prev
		zero := int32(0)
		item.TargetReplicas = &zero
		return finishActionItem(item, err, "工作负载已停止")
	case "start":
		target := int32(1)
		if req.Replicas != nil && *req.Replicas > 0 {
			target = *req.Replicas
		}
		prev, err := s.scaleWorkload(ctx, clientset, namespace, workloadType, workloadName, target)
		item.PreviousReplicas = &prev
		item.TargetReplicas = &target
		return finishActionItem(item, err, "工作负载已启动")
	case "scale":
		if req.Replicas == nil || *req.Replicas < 0 {
			item.Message = "scale操作需要传入非负replicas"
			return item
		}
		prev, err := s.scaleWorkload(ctx, clientset, namespace, workloadType, workloadName, *req.Replicas)
		item.PreviousReplicas = &prev
		item.TargetReplicas = req.Replicas
		return finishActionItem(item, err, "工作负载副本数已调整")
	case "restart", "quickRestart":
		err := s.restartWorkload(ctx, clientset, namespace, workloadType, workloadName)
		return finishActionItem(item, err, "工作负载滚动重启已触发")
	default:
		item.Status = "skipped"
		item.Message = "不支持的工作负载操作: " + action
		return item
	}
}

func (s *BigDataAppServiceImpl) operatePod(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, req *model.BigDataActionRequest) model.BigDataActionItem {
	action := normalizeBigDataAction(req.Action)
	item := model.BigDataActionItem{
		Namespace:    namespace,
		ResourceType: "pod",
		ResourceName: podName,
		Action:       action,
		Status:       "failed",
	}

	switch action {
	case "quickRestart", "restart", "delete", "stop":
		err := clientset.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		return finishActionItem(item, err, "Pod删除成功，若由控制器管理将自动重建")
	default:
		item.Status = "skipped"
		item.Message = "Pod仅支持quickRestart/restart/delete/stop操作"
		return item
	}
}

func (s *BigDataAppServiceImpl) scaleWorkload(ctx context.Context, clientset *kubernetes.Clientset, namespace, workloadType, workloadName string, replicas int32) (int32, error) {
	switch normalizeWorkloadType(workloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		prev := int32(0)
		if item.Spec.Replicas != nil {
			prev = *item.Spec.Replicas
		}
		item.Spec.Replicas = &replicas
		_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return prev, err
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		prev := int32(0)
		if item.Spec.Replicas != nil {
			prev = *item.Spec.Replicas
		}
		item.Spec.Replicas = &replicas
		_, err = clientset.AppsV1().StatefulSets(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return prev, err
	default:
		return 0, fmt.Errorf("%s 不支持副本伸缩", workloadType)
	}
}

func (s *BigDataAppServiceImpl) restartWorkload(ctx context.Context, clientset *kubernetes.Clientset, namespace, workloadType, workloadName string) error {
	restartedAt := time.Now().Format(time.RFC3339)
	switch normalizeWorkloadType(workloadType) {
	case "deployment":
		item, err := clientset.AppsV1().Deployments(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = clientset.AppsV1().Deployments(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return err
	case "statefulset":
		item, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = clientset.AppsV1().StatefulSets(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return err
	case "daemonset":
		item, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if item.Spec.Template.Annotations == nil {
			item.Spec.Template.Annotations = map[string]string{}
		}
		item.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = restartedAt
		_, err = clientset.AppsV1().DaemonSets(namespace).Update(ctx, item, metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("%s 不支持滚动重启", workloadType)
	}
}

func finishActionItem(item model.BigDataActionItem, err error, successMessage string) model.BigDataActionItem {
	if err != nil {
		item.Status = "failed"
		item.Message = err.Error()
		return item
	}
	item.Status = "success"
	item.Message = successMessage
	return item
}

func actionItemsSucceeded(items []model.BigDataActionItem) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Status == "failed" {
			return false
		}
	}
	return true
}

func buildActionMessage(success bool, action string, count int) string {
	if success {
		return fmt.Sprintf("%s操作已提交，共处理%d个资源", action, count)
	}
	return fmt.Sprintf("%s操作部分或全部失败，共处理%d个资源", action, count)
}

func normalizeBigDataAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "apply":
		return "apply"
	case "config", "configure":
		return "config"
	case "stop":
		return "stop"
	case "start":
		return "start"
	case "quickrestart", "quick_restart", "fastrestart", "fast_restart":
		return "quickRestart"
	case "restart":
		return "restart"
	case "scale":
		return "scale"
	case "delete":
		return "delete"
	default:
		return strings.TrimSpace(action)
	}
}

func normalizeWorkloadType(workloadType string) string {
	switch strings.ToLower(strings.TrimSpace(workloadType)) {
	case "deployment", "deployments":
		return "deployment"
	case "statefulset", "statefulsets":
		return "statefulset"
	case "daemonset", "daemonsets":
		return "daemonset"
	case "job", "jobs":
		return "job"
	case "cronjob", "cronjobs":
		return "cronjob"
	default:
		return strings.ToLower(strings.TrimSpace(workloadType))
	}
}

func namespaceOrAll(namespace string) string {
	if strings.TrimSpace(namespace) == "" {
		return metav1.NamespaceAll
	}
	return namespace
}

func getPodByNameAcrossNamespaces(ctx context.Context, clientset *kubernetes.Clientset, podName string) (*corev1.Pod, error) {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + podName,
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, apierrors.NewNotFound(corev1.Resource("pods"), podName)
	}
	return &pods.Items[0], nil
}
