package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	configdao "dodevops-api/api/configcenter/dao"
	configmodel "dodevops-api/api/configcenter/model"
	k8sdao "dodevops-api/api/k8s/dao"
	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"

	"gorm.io/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	businessHealthEngineSource      = "business_health_engine"
	businessHealthAnalysisWindow    = "5m"
	businessHealthK8sEventLookback  = time.Hour
	businessHealthDefaultGraphDepth = 8
)

type businessHealthEngine struct {
	cmdb            *CmdbService
	k8sClusterDao   *k8sdao.KubeClusterDao
	accountDao      *configdao.AccountAuthDao
	hostDao         cmdbdao.CmdbHostDao
	hardwareService *CmdbIdcAssetHardwareMonitorService
}

type businessHealthReport struct {
	Snapshots      map[uint][]nodeHealth
	Coverage       map[string]interface{}
	ScoreBreakdown map[string]interface{}
	Evidence       []map[string]interface{}
	GeneratedAt    time.Time
}

type healthCheckResult struct {
	Name       string
	Status     string
	Score      int
	Message    string
	Dimension  string
	Evidence   map[string]interface{}
	Diagnostic string
	Optional   bool
}

func newBusinessHealthEngine(cmdb *CmdbService) *businessHealthEngine {
	db := cmdb.dao.DB()
	return &businessHealthEngine{
		cmdb:            cmdb,
		k8sClusterDao:   k8sdao.NewKubeClusterDao(db),
		accountDao:      configdao.NewAccountAuthDao(),
		hostDao:         cmdbdao.NewCmdbHostDao(),
		hardwareService: NewCmdbIdcAssetHardwareMonitorService(),
	}
}

func (e *businessHealthEngine) Analyze(ctx context.Context, graph *businessGraphData) (*businessHealthReport, error) {
	report := &businessHealthReport{
		Snapshots:      make(map[uint][]nodeHealth),
		Coverage:       make(map[string]interface{}),
		ScoreBreakdown: make(map[string]interface{}),
		Evidence:       []map[string]interface{}{},
		GeneratedAt:    time.Now(),
	}
	if graph == nil || graph.Root == nil {
		return report, nil
	}

	resources := graph.sortedResources()
	for _, resource := range resources {
		checks := e.analyzeResource(ctx, graph, resource)
		healthByDimension := e.healthFromChecks(resource, checks)
		report.Snapshots[resource.ID] = healthByDimension
		report.Evidence = append(report.Evidence, e.evidenceEntry(resource, healthByDimension, checks))
	}

	report.Coverage = e.buildCoverage(graph, report.Snapshots)
	report.ScoreBreakdown = e.buildScoreBreakdown(graph, report.Snapshots)
	return report, nil
}

func (e *businessHealthEngine) analyzeResource(ctx context.Context, graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	if resource == nil {
		return nil
	}

	switch strings.TrimSpace(resource.ResourceType) {
	case cmdbmodel.CmdbResourceTypeBusinessSystem:
		return e.analyzeBusinessSystem(graph, resource)
	case cmdbmodel.CmdbResourceTypeBusinessService, cmdbmodel.CmdbResourceTypeApplication:
		return e.analyzeBusinessService(graph, resource)
	case cmdbmodel.CmdbResourceTypeNginx, cmdbmodel.CmdbResourceTypeGateway:
		return e.analyzeTrafficEntryService(graph, resource)
	case cmdbmodel.CmdbResourceTypeK8sCluster:
		return e.analyzeK8sCluster(ctx, resource)
	case "k8s_namespace":
		return e.analyzeK8sNamespace(ctx, resource)
	case cmdbmodel.CmdbResourceTypeK8sWorkload:
		return e.analyzeK8sWorkload(ctx, graph, resource)
	case cmdbmodel.CmdbResourceTypeHost, cmdbmodel.CmdbResourceTypeVM, cmdbmodel.CmdbResourceTypeCloudHost:
		return e.analyzeHost(resource)
	case cmdbmodel.CmdbResourceTypePhysicalServer:
		return e.analyzePhysicalServer(resource)
	case cmdbmodel.CmdbResourceTypeDatabase:
		return e.analyzeDatabase(ctx, graph, resource)
	case cmdbmodel.CmdbResourceTypeMiddleware:
		return e.analyzeMiddleware(graph, resource)
	case cmdbmodel.CmdbResourceTypeLogProject:
		return e.analyzeLogProject(resource)
	case cmdbmodel.CmdbResourceTypeLogStore:
		return e.analyzeLogStore(resource)
	default:
		return e.analyzeGeneric(resource)
	}
}

func (e *businessHealthEngine) analyzeBusinessSystem(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	descendants := graph.descendantResources()
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := fmt.Sprintf("已绑定 %d 个下游资源节点", len(descendants))
	if len(descendants) == 0 {
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		message = "业务模型未绑定下游资源，无法计算业务健康"
	}
	return []healthCheckResult{
		{
			Name:      "业务建模覆盖",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionBusiness,
			Evidence: map[string]interface{}{
				"boundResourceCount": len(descendants),
				"relationCount":      len(graph.Relations),
			},
			Diagnostic: "编辑业务模型",
		},
	}
}

func (e *businessHealthEngine) analyzeBusinessService(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	props := jsonToMap(resource.Properties)
	if strings.EqualFold(stringFromAny(props["deployMode"]), "host") || uintFromResource(resource, "hostId") > 0 {
		checks := e.analyzeHostDeployedService(resource)
		checks = append(checks, e.checkBoundLogCoverage(graph, resource, false, ""))
		return checks
	}

	children := graph.childrenOf(resource.ID)
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := fmt.Sprintf("业务服务已关联 %d 个下游资源", len(children))
	if len(children) == 0 {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 75
		message = "业务服务未继续关联应用或资源，影响链路分析完整性"
	}
	return []healthCheckResult{
		{
			Name:      "服务链路完整性",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionBusiness,
			Evidence: map[string]interface{}{
				"downstreamCount": len(children),
			},
			Diagnostic: "编辑业务服务依赖",
		},
	}
}

func (e *businessHealthEngine) analyzeHostDeployedService(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	props := jsonToMap(resource.Properties)
	hostID := uintFromResource(resource, "hostId")
	serviceName := strings.TrimSpace(firstNonEmpty(
		stringFromAny(firstPresent(props, "serviceName", "processName", "service")),
	))
	monitorAll := boolFromAny(firstPresent(props, "monitorAllServices", "allServices"))
	if !monitorAll && serviceName == "" {
		monitorAll = true
	}
	if hostID == 0 {
		return []healthCheckResult{unknownCheck("主机服务监控", cmdbmodel.CmdbHealthDimensionBusiness, "业务服务未绑定主机，无法检查服务进程", resourceIdentityEvidence(resource))}
	}

	metrics, err := NewMonitorService().GetCompleteHostMetricsData(hostID)
	if err != nil {
		return []healthCheckResult{unknownCheck("主机服务监控", cmdbmodel.CmdbHealthDimensionBusiness, "主机完整监控未接入或读取失败: "+err.Error(), map[string]interface{}{"hostId": hostID, "serviceName": serviceName})}
	}

	check := hostServiceMetricsCheck(hostID, serviceName, monitorAll, metrics.Services)
	return []healthCheckResult{check}
}

func (e *businessHealthEngine) analyzeTrafficEntryService(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	health := derivedHealthFromResource(resource)
	if health.Status == cmdbmodel.CmdbHealthStatusUnknown {
		health.Status = cmdbmodel.CmdbHealthStatusHealthy
		health.Score = 100
		health.Message = resourceTypeLabel(resource.ResourceType) + " 节点已接入业务链路"
	}
	logConfigured, logSource := resourceLogSourceConfigured(resource)
	return []healthCheckResult{
		{
			Name:      resourceTypeLabel(resource.ResourceType) + " 节点状态",
			Status:    health.Status,
			Score:     health.Score,
			Message:   health.Message,
			Dimension: cmdbmodel.CmdbHealthDimensionBusiness,
			Evidence: mergeEvidence(resourceIdentityEvidence(resource), map[string]interface{}{
				"entryType": resource.ResourceType,
			}),
			Diagnostic: "查看入口链路",
		},
		e.checkBoundLogCoverage(graph, resource, logConfigured, logSource),
	}
}

func (e *businessHealthEngine) analyzeK8sCluster(ctx context.Context, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	clusterID := uintFromResource(resource, "clusterId")
	if clusterID == 0 && resource.SourceID > 0 {
		clusterID = resource.SourceID
	}
	if clusterID == 0 {
		return []healthCheckResult{unknownCheck("K8S 集群识别", cmdbmodel.CmdbHealthDimensionContainer, "资源缺少 clusterId/sourceId，无法连接 K8S 集群", resourceIdentityEvidence(resource))}
	}

	cluster, err := e.k8sClusterDao.GetByID(clusterID)
	if err != nil {
		return []healthCheckResult{criticalCheck("K8S 集群配置", cmdbmodel.CmdbHealthDimensionContainer, "K8S 集群配置不存在或读取失败", map[string]interface{}{"clusterId": clusterID, "error": err.Error()})}
	}

	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := "K8S 集群处于运行状态"
	if cluster.Status != k8smodel.ClusterStatusRunning {
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 30
		message = fmt.Sprintf("K8S 集群状态异常: %s", cluster.GetStatusText())
	} else if cluster.NodeCount > 0 && cluster.ReadyNodes < cluster.NodeCount {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 80
		message = fmt.Sprintf("K8S 集群存在未就绪节点: %d/%d", cluster.ReadyNodes, cluster.NodeCount)
	}

	checks := []healthCheckResult{
		{
			Name:      "集群运行状态",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionContainer,
			Evidence: map[string]interface{}{
				"clusterId":   cluster.ID,
				"clusterName": cluster.Name,
				"status":      cluster.GetStatusText(),
				"nodeCount":   cluster.NodeCount,
				"readyNodes":  cluster.ReadyNodes,
				"lastSyncAt":  cluster.LastSyncAt,
			},
			Diagnostic: "查看 K8S 集群监控",
		},
	}

	clientset, err := e.createK8sClient(cluster)
	if err != nil {
		checks = append(checks, warningCheck("K8S API 连接", cmdbmodel.CmdbHealthDimensionContainer, "K8S API 连接失败，无法读取实时运行态", map[string]interface{}{"clusterId": cluster.ID, "error": err.Error()}))
		return checks
	}

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		checks = append(checks, warningCheck("K8S API 连接", cmdbmodel.CmdbHealthDimensionContainer, "K8S API Server 探测失败", map[string]interface{}{"clusterId": cluster.ID, "error": err.Error()}))
		return checks
	}
	checks = append(checks, healthCheckResult{
		Name:      "K8S API 连接",
		Status:    cmdbmodel.CmdbHealthStatusHealthy,
		Score:     100,
		Message:   "K8S API Server 可访问",
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: map[string]interface{}{
			"clusterId": cluster.ID,
			"version":   version.GitVersion,
			"deadline":  checkCtx.Err() == nil,
		},
		Diagnostic: "查看 K8S 集群详情",
	})

	return checks
}

func (e *businessHealthEngine) analyzeK8sNamespace(ctx context.Context, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	cluster, clientset, evidence, err := e.k8sClientFromResource(resource)
	if err != nil {
		return []healthCheckResult{unknownCheck("命名空间运行态", cmdbmodel.CmdbHealthDimensionContainer, "未接入 K8S 运行态数据: "+err.Error(), evidence)}
	}
	namespace := stringFromResource(resource, "namespace")
	if namespace == "" {
		namespace = lastSourceKeyPart(resource.SourceKey)
	}
	if namespace == "" {
		return []healthCheckResult{unknownCheck("命名空间识别", cmdbmodel.CmdbHealthDimensionContainer, "资源缺少 namespace，无法分析命名空间", evidence)}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	ns, err := clientset.CoreV1().Namespaces().Get(checkCtx, namespace, metav1.GetOptions{})
	if err != nil {
		status := cmdbmodel.CmdbHealthStatusWarning
		score := 70
		if apierrors.IsNotFound(err) {
			status = cmdbmodel.CmdbHealthStatusCritical
			score = 35
		}
		return []healthCheckResult{
			{
				Name:       "命名空间存在性",
				Status:     status,
				Score:      score,
				Message:    "命名空间读取失败: " + err.Error(),
				Dimension:  cmdbmodel.CmdbHealthDimensionContainer,
				Evidence:   mergeEvidence(evidence, map[string]interface{}{"namespace": namespace, "error": err.Error()}),
				Diagnostic: "查看命名空间事件",
			},
		}
	}

	events, warningEvents := e.listK8sWarningEvents(checkCtx, clientset, namespace, "", "")
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := "命名空间存在且近期未发现 Warning 事件"
	if warningEvents > 0 {
		message = fmt.Sprintf("命名空间近 1 小时存在 %d 条 Warning 事件，仅作为诊断线索", warningEvents)
	}
	return []healthCheckResult{
		{
			Name:      "命名空间运行态",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionContainer,
			Evidence: mergeEvidence(evidence, map[string]interface{}{
				"clusterId":      cluster.ID,
				"clusterName":    cluster.Name,
				"namespace":      namespace,
				"phase":          string(ns.Status.Phase),
				"warningEvents":  warningEvents,
				"recentWarnings": events,
				"affectsScore":   false,
			}),
			Diagnostic: "查看命名空间事件",
			Optional:   warningEvents > 0,
		},
	}
}

func (e *businessHealthEngine) analyzeK8sWorkload(ctx context.Context, graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	_, clientset, evidence, err := e.k8sClientFromResource(resource)
	if err != nil {
		return []healthCheckResult{unknownCheck("工作负载运行态", cmdbmodel.CmdbHealthDimensionContainer, "未接入 K8S 运行态数据: "+err.Error(), evidence)}
	}

	namespace := stringFromResource(resource, "namespace")
	workloadType := firstNonEmpty(stringFromResource(resource, "workloadType"), stringFromResource(resource, "kind"), "Deployment")
	workloadName := firstNonEmpty(stringFromResource(resource, "workloadName"), resource.Name)
	if namespace == "" || workloadName == "" {
		return []healthCheckResult{unknownCheck("工作负载识别", cmdbmodel.CmdbHealthDimensionContainer, "缺少 namespace/workloadName，无法分析工作负载", evidence)}
	}

	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	checks := make([]healthCheckResult, 0, 5)
	runtimeCheck, selector := e.checkK8sWorkloadRuntime(checkCtx, clientset, namespace, workloadType, workloadName, evidence)
	checks = append(checks, runtimeCheck)
	if selector != nil {
		checks = append(checks, e.checkK8sWorkloadPods(checkCtx, clientset, namespace, selector, evidence))
	} else if strings.EqualFold(workloadType, "Pod") {
		if podCheck := e.checkK8sSinglePod(checkCtx, clientset, namespace, workloadName, evidence); podCheck.Name != "" {
			checks = append(checks, podCheck)
		}
	}

	events, warningEvents := e.listK8sWarningEvents(checkCtx, clientset, namespace, normalizeK8sKind(workloadType), workloadName)
	eventStatus := cmdbmodel.CmdbHealthStatusHealthy
	eventScore := 100
	eventMessage := "近 1 小时未发现工作负载 Warning 事件"
	if warningEvents > 0 {
		eventMessage = fmt.Sprintf("近 1 小时存在 %d 条工作负载 Warning 事件，仅作为诊断线索", warningEvents)
	}
	checks = append(checks, healthCheckResult{
		Name:      "K8S 事件",
		Status:    eventStatus,
		Score:     eventScore,
		Message:   eventMessage,
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: mergeEvidence(evidence, map[string]interface{}{
			"namespace":      namespace,
			"workloadType":   workloadType,
			"workloadName":   workloadName,
			"warningEvents":  warningEvents,
			"recentWarnings": events,
			"affectsScore":   false,
		}),
		Diagnostic: "查看工作负载事件",
		Optional:   warningEvents > 0,
	})

	checks = append(checks, e.checkK8sLogCoverage(graph, resource, evidence))
	return checks
}

func (e *businessHealthEngine) checkK8sWorkloadRuntime(ctx context.Context, clientset *kubernetes.Clientset, namespace, workloadType, workloadName string, evidence map[string]interface{}) (healthCheckResult, *metav1.LabelSelector) {
	base := mergeEvidence(evidence, map[string]interface{}{
		"namespace":    namespace,
		"workloadType": workloadType,
		"workloadName": workloadName,
	})
	switch strings.ToLower(strings.TrimSpace(workloadType)) {
	case "deployment", "deployments":
		item, err := clientset.AppsV1().Deployments(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("Deployment", err, base), nil
		}
		desired := int32(1)
		if item.Spec.Replicas != nil {
			desired = *item.Spec.Replicas
		}
		return replicatedWorkloadCheck("Deployment", item.Status.ReadyReplicas, desired, item.Status.AvailableReplicas, base), item.Spec.Selector
	case "statefulset", "statefulsets":
		item, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("StatefulSet", err, base), nil
		}
		desired := int32(1)
		if item.Spec.Replicas != nil {
			desired = *item.Spec.Replicas
		}
		return replicatedWorkloadCheck("StatefulSet", item.Status.ReadyReplicas, desired, item.Status.AvailableReplicas, base), item.Spec.Selector
	case "daemonset", "daemonsets":
		item, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("DaemonSet", err, base), nil
		}
		return replicatedWorkloadCheck("DaemonSet", item.Status.NumberReady, item.Status.DesiredNumberScheduled, item.Status.NumberAvailable, base), item.Spec.Selector
	case "job", "jobs":
		item, err := clientset.BatchV1().Jobs(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("Job", err, base), nil
		}
		return jobWorkloadCheck(item, base), item.Spec.Selector
	case "cronjob", "cronjobs":
		item, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("CronJob", err, base), nil
		}
		status := cmdbmodel.CmdbHealthStatusHealthy
		score := 100
		message := "CronJob 调度配置正常"
		if item.Spec.Suspend != nil && *item.Spec.Suspend {
			status = cmdbmodel.CmdbHealthStatusWarning
			score = 75
			message = "CronJob 当前处于暂停状态"
		}
		return healthCheckResult{
			Name:      "工作负载副本",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionContainer,
			Evidence: mergeEvidence(base, map[string]interface{}{
				"schedule":         item.Spec.Schedule,
				"suspend":          item.Spec.Suspend,
				"activeJobs":       len(item.Status.Active),
				"lastScheduleTime": item.Status.LastScheduleTime,
			}),
			Diagnostic: "查看 CronJob",
		}, nil
	case "pod", "pods":
		item, err := clientset.CoreV1().Pods(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if err != nil {
			return k8sGetErrorCheck("Pod", err, base), nil
		}
		return podHealthCheck(*item, base), nil
	default:
		return unknownCheck("工作负载类型", cmdbmodel.CmdbHealthDimensionContainer, "暂不支持的 K8S 工作负载类型: "+workloadType, base), nil
	}
}

func (e *businessHealthEngine) checkK8sWorkloadPods(ctx context.Context, clientset *kubernetes.Clientset, namespace string, selector *metav1.LabelSelector, evidence map[string]interface{}) healthCheckResult {
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return warningCheck("Pod 选择器", cmdbmodel.CmdbHealthDimensionContainer, "工作负载 selector 解析失败: "+err.Error(), evidence)
	}
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return warningCheck("Pod 状态", cmdbmodel.CmdbHealthDimensionContainer, "Pod 列表读取失败: "+err.Error(), evidence)
	}

	total := len(pods.Items)
	notReady := 0
	failed := 0
	pending := 0
	restarts := int32(0)
	imagePullErrors := 0
	abnormalPods := make([]map[string]interface{}, 0)
	for _, pod := range pods.Items {
		podRestarts := podRestartCount(pod)
		restarts += podRestarts
		podReady := isBusinessPodReady(pod)
		if !podReady {
			notReady++
		}
		switch pod.Status.Phase {
		case corev1.PodFailed:
			failed++
		case corev1.PodPending:
			pending++
		}
		if podHasBusinessImagePullError(pod) {
			imagePullErrors++
		}
		if !podReady || podRestarts > 0 || podHasBusinessImagePullError(pod) {
			abnormalPods = append(abnormalPods, map[string]interface{}{
				"name":         pod.Name,
				"phase":        string(pod.Status.Phase),
				"ready":        podReady,
				"restartCount": podRestarts,
				"nodeName":     pod.Spec.NodeName,
			})
		}
	}

	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := fmt.Sprintf("Pod 全部就绪，共 %d 个", total)
	switch {
	case total == 0:
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 30
		message = "未找到工作负载关联 Pod"
	case failed > 0 || imagePullErrors > 0:
		status = cmdbmodel.CmdbHealthStatusCritical
		score = maxInt(20, 70-failed*15-imagePullErrors*20)
		message = fmt.Sprintf("Pod 存在严重异常: failed=%d, imagePull=%d", failed, imagePullErrors)
	case restarts >= 100:
		status = cmdbmodel.CmdbHealthStatusCritical
		score = maxInt(30, 80-int(restarts)/20)
		message = fmt.Sprintf("Pod 重启次数异常偏高: restart=%d", restarts)
	case notReady > 0 || pending > 0 || restarts >= 5:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = maxInt(60, 100-notReady*10-pending*8-int(restarts)*2)
		message = fmt.Sprintf("Pod 存在轻微问题: notReady=%d, pending=%d, restart=%d", notReady, pending, restarts)
	}

	return healthCheckResult{
		Name:      "Pod 状态",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: mergeEvidence(evidence, map[string]interface{}{
			"podTotal":        total,
			"podNotReady":     notReady,
			"podFailed":       failed,
			"podPending":      pending,
			"restartCount":    restarts,
			"imagePullErrors": imagePullErrors,
			"abnormalPods":    limitMapList(abnormalPods, 5),
		}),
		Diagnostic: "查看 Pod / 重启记录",
	}
}

func (e *businessHealthEngine) checkK8sSinglePod(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, evidence map[string]interface{}) healthCheckResult {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return k8sGetErrorCheck("Pod", err, evidence)
	}
	return podHealthCheck(*pod, evidence)
}

func (e *businessHealthEngine) checkK8sLogCoverage(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding, evidence map[string]interface{}) healthCheckResult {
	props := jsonToMap(resource.Properties)
	enabled := boolFromAny(firstPresent(props, "logEnabled", "logAnalysisEnabled", "logsEnabled", "日志接入"))
	source := stringFromAny(firstPresent(props, "logSource", "logIndex", "logProject", "日志来源"))
	return e.checkBoundLogCoverage(graph, resource, enabled || source != "", source, evidence)
}

func (e *businessHealthEngine) analyzeHost(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	hostID := uintFromResource(resource, "hostId")
	if hostID == 0 && resource.SourceID > 0 {
		hostID = resource.SourceID
	}
	host, err := e.resolveHost(resource, hostID)
	if err != nil {
		return []healthCheckResult{unknownCheck("主机识别", cmdbmodel.CmdbHealthDimensionInfra, "无法识别主机资产: "+err.Error(), resourceIdentityEvidence(resource))}
	}

	checks := make([]healthCheckResult, 0, 3)
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := "主机认证状态正常"
	if host.Status == 2 {
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		message = "主机未认证，无法采集系统指标"
	} else if host.Status == 3 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 30
		message = "主机认证失败，无法采集系统指标"
	}
	checks = append(checks, healthCheckResult{
		Name:      "主机资产状态",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionInfra,
		Evidence: map[string]interface{}{
			"hostId":    host.ID,
			"hostName":  firstNonEmpty(host.Name, host.HostName),
			"hostIp":    firstNonEmpty(host.PrivateIP, host.SSHIP, host.PublicIP),
			"hostType":  host.HostType,
			"authState": host.Status,
		},
		Diagnostic: "查看主机详情",
	})

	monitor := NewMonitorService()
	metrics, err := monitor.GetHostMetricsData(host.ID, 5, "30s")
	if err != nil {
		checks = append(checks, unknownCheck("主机基础监控", cmdbmodel.CmdbHealthDimensionInfra, "主机基础监控未接入或读取失败: "+err.Error(), map[string]interface{}{"hostId": host.ID}))
	} else {
		checks = append(checks, hostMetricsCheck(host.ID, metrics))
	}

	checks = append(checks, healthCheckResult{
		Name:       "系统日志接入",
		Status:     cmdbmodel.CmdbHealthStatusUnknown,
		Score:      0,
		Message:    "暂未发现主机系统日志健康快照，建议接入日志分析模块",
		Dimension:  cmdbmodel.CmdbHealthDimensionLog,
		Evidence:   map[string]interface{}{"hostId": host.ID, "logConfigured": false},
		Diagnostic: "查看系统日志分析",
		Optional:   true,
	})
	return checks
}

func (e *businessHealthEngine) analyzePhysicalServer(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	checks := e.analyzeHost(resource)
	assetID := uintFromResource(resource, "assetId")
	if assetID == 0 && resource.SourceID > 0 {
		assetID = resource.SourceID
	}
	if assetID == 0 {
		checks = append(checks, unknownCheck("硬件监控", cmdbmodel.CmdbHealthDimensionInfra, "物理机缺少资产 ID，无法检查硬件监控", resourceIdentityEvidence(resource)))
		return checks
	}
	monitor, err := e.hardwareService.BuildAssetHardwareMonitor(assetID, 5, 6, 5)
	if err != nil {
		checks = append(checks, unknownCheck("硬件监控", cmdbmodel.CmdbHealthDimensionInfra, "硬件监控未接入或读取失败: "+err.Error(), map[string]interface{}{"assetId": assetID}))
		return checks
	}
	status := normalizeHealthStatusName(monitor.Health.Level)
	if status == "" {
		status = cmdbmodel.CmdbHealthStatusUnknown
	}
	checks = append(checks, healthCheckResult{
		Name:      "物理机硬件健康",
		Status:    status,
		Score:     monitor.Health.Score,
		Message:   monitor.Health.Conclusion,
		Dimension: cmdbmodel.CmdbHealthDimensionInfra,
		Evidence: map[string]interface{}{
			"assetId":          assetID,
			"metricsAvailable": monitor.MetricsAvailable,
			"vendor":           monitor.Vendor,
			"criticalCount":    monitor.Health.CriticalCount,
			"warningCount":     monitor.Health.WarningCount,
			"riskCategories":   monitor.Health.RiskCategories,
			"recommendations":  monitor.Health.Recommendations,
			"alerts":           hardwareAlertEvidence(monitor.Alerts),
		},
		Diagnostic: "查看物理机硬件监控",
	})
	return checks
}

func (e *businessHealthEngine) analyzeDatabase(ctx context.Context, graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	assetID := uintFromResource(resource, "assetId")
	if assetID == 0 && resource.SourceID > 0 {
		assetID = resource.SourceID
	}
	if assetID == 0 {
		return []healthCheckResult{unknownCheck("数据库资产识别", cmdbmodel.CmdbHealthDimensionMiddleware, "数据库节点缺少 sourceId/assetId，无法分析实例健康", resourceIdentityEvidence(resource))}
	}

	checks := make([]healthCheckResult, 0, 3)
	asset := cmdbmodel.CmdbSQL{}
	assetErr := e.cmdb.dao.DB().First(&asset, assetID).Error
	if assetErr != nil {
		checks = append(checks, criticalCheck("数据库资产", cmdbmodel.CmdbHealthDimensionMiddleware, "数据库资产不存在或读取失败", map[string]interface{}{"assetId": assetID, "error": assetErr.Error()}))
		return checks
	}

	checks = append(checks, databaseAssetCheck(assetID, asset))
	checks = append(checks, e.checkDatabaseRuntimeFromPrometheus(ctx, assetID, asset)...)

	checks = append(checks, e.checkBoundLogCoverage(graph, resource, strings.TrimSpace(asset.SlowLogPath) != "", asset.SlowLogPath, map[string]interface{}{
		"assetId":           assetID,
		"slowLogConfigured": strings.TrimSpace(asset.SlowLogPath) != "",
		"slowLogPath":       asset.SlowLogPath,
		"hostId":            asset.HostID,
		"hostIp":            asset.HostIP,
	}))

	return checks
}

func (e *businessHealthEngine) analyzeMiddleware(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	props := jsonToMap(resource.Properties)
	monitorEnabled := boolFromAny(firstPresent(props, "monitorEnabled", "metricsEnabled", "prometheusEnabled"))
	statusValue := firstNonEmpty(stringFromAny(firstPresent(props, "runtimeStatus", "status", "healthStatus")), resource.Status)
	status := normalizeHealthStatusName(statusValue)
	score := 100
	message := "中间件资产状态正常"
	if status == "" || status == cmdbmodel.CmdbHealthStatusUnknown {
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		message = "中间件未接入专用监控，无法判断运行态"
	} else if status != cmdbmodel.CmdbHealthStatusHealthy {
		score = scoreByStatus(status)
		message = "中间件运行状态异常: " + statusValue
	}
	if !monitorEnabled && status == cmdbmodel.CmdbHealthStatusHealthy {
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		message = "中间件仅有资产状态，未接入运行指标"
	}
	checks := []healthCheckResult{
		{
			Name:      "中间件运行态",
			Status:    status,
			Score:     score,
			Message:   message,
			Dimension: cmdbmodel.CmdbHealthDimensionMiddleware,
			Evidence: mergeEvidence(resourceIdentityEvidence(resource), map[string]interface{}{
				"monitorEnabled": monitorEnabled,
				"runtimeStatus":  statusValue,
			}),
			Diagnostic: "查看中间件监控",
		},
	}
	checks = append(checks, e.checkBoundLogCoverage(graph, resource, false, ""))
	return checks
}

func (e *businessHealthEngine) analyzeLogProject(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	projectID := uintFromResource(resource, "projectId", "logProjectId")
	if projectID == 0 && resource.SourceID > 0 {
		projectID = resource.SourceID
	}
	if projectID == 0 {
		return []healthCheckResult{unknownCheck("日志项目识别", cmdbmodel.CmdbHealthDimensionLog, "日志项目节点缺少 sourceId/projectId，无法分析日志健康", resourceIdentityEvidence(resource))}
	}

	var project logtailmodel.LogProject
	if err := e.cmdb.dao.DB().First(&project, projectID).Error; err != nil {
		return []healthCheckResult{criticalCheck("日志项目", cmdbmodel.CmdbHealthDimensionLog, "日志项目不存在或读取失败", map[string]interface{}{"projectId": projectID, "error": err.Error()})}
	}

	var storeTotal int64
	_ = e.cmdb.dao.DB().Model(&logtailmodel.LogStore{}).Where("project_id = ?", projectID).Count(&storeTotal).Error
	events := e.recentFiringLogEvents(projectID, 0, 5)
	status, score, message := logHealthStatusFromEvents(events, project.Status == logtailmodel.LogStatusEnabled)
	if storeTotal == 0 && status == cmdbmodel.CmdbHealthStatusHealthy {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 75
		message = "日志项目未关联日志库，无法形成业务日志证据"
	}

	return []healthCheckResult{{
		Name:      "日志项目健康",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionLog,
		Evidence: mergeEvidence(resourceIdentityEvidence(resource), map[string]interface{}{
			"projectId":     project.ID,
			"projectName":   project.Name,
			"projectCode":   project.Code,
			"enabled":       project.Status == logtailmodel.LogStatusEnabled,
			"storeTotal":    storeTotal,
			"firingEvents":  len(events),
			"recentEvents":  logEventEvidence(events),
			"explicitBind":  true,
			"bindingTarget": cmdbmodel.CmdbResourceTypeLogProject,
		}),
		Diagnostic: "查看日志项目",
	}}
}

func (e *businessHealthEngine) analyzeLogStore(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	storeID := uintFromResource(resource, "storeId", "logStoreId")
	if storeID == 0 && resource.SourceID > 0 {
		storeID = resource.SourceID
	}
	if storeID == 0 {
		return []healthCheckResult{unknownCheck("日志库识别", cmdbmodel.CmdbHealthDimensionLog, "日志库节点缺少 sourceId/storeId，无法分析日志健康", resourceIdentityEvidence(resource))}
	}

	var store logtailmodel.LogStore
	if err := e.cmdb.dao.DB().First(&store, storeID).Error; err != nil {
		return []healthCheckResult{criticalCheck("日志库", cmdbmodel.CmdbHealthDimensionLog, "日志库不存在或读取失败", map[string]interface{}{"storeId": storeID, "error": err.Error()})}
	}

	events := e.recentFiringLogEvents(store.ProjectID, store.ID, 5)
	status, score, message := logHealthStatusFromEvents(events, store.Status == logtailmodel.LogStatusEnabled)
	return []healthCheckResult{{
		Name:      "日志库健康",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionLog,
		Evidence: mergeEvidence(resourceIdentityEvidence(resource), map[string]interface{}{
			"projectId":     store.ProjectID,
			"storeId":       store.ID,
			"storeName":     store.Name,
			"storeCode":     store.Code,
			"enabled":       store.Status == logtailmodel.LogStatusEnabled,
			"firingEvents":  len(events),
			"recentEvents":  logEventEvidence(events),
			"explicitBind":  true,
			"bindingTarget": cmdbmodel.CmdbResourceTypeLogStore,
		}),
		Diagnostic: "查看日志库",
	}}
}

func (e *businessHealthEngine) checkBoundLogCoverage(graph *businessGraphData, resource *cmdbmodel.CmdbBusinessModelBinding, legacyConfigured bool, legacySource string, evidenceItems ...map[string]interface{}) healthCheckResult {
	boundLogs := boundLogResources(graph, resource.ID)
	sourceConfigured, sourceName := resourceLogSourceConfigured(resource)
	if sourceConfigured {
		legacyConfigured = true
		if strings.TrimSpace(legacySource) == "" {
			legacySource = sourceName
		}
	}
	evidence := resourceIdentityEvidence(resource)
	for _, item := range evidenceItems {
		evidence = mergeEvidence(evidence, item)
	}
	evidence = mergeEvidence(evidence, map[string]interface{}{
		"logBound":         len(boundLogs) > 0,
		"boundLogCount":    len(boundLogs),
		"boundLogs":        boundLogResourceEvidence(boundLogs),
		"legacyConfigured": legacyConfigured,
		"legacySource":     legacySource,
		"sourceLogBinding": sourceConfigured,
	})
	if len(boundLogs) > 0 {
		return healthCheckResult{
			Name:       "日志绑定",
			Status:     cmdbmodel.CmdbHealthStatusHealthy,
			Score:      100,
			Message:    fmt.Sprintf("已绑定 %d 个日志资源，异常日志可作为健康证据", len(boundLogs)),
			Dimension:  cmdbmodel.CmdbHealthDimensionLog,
			Evidence:   evidence,
			Diagnostic: "查看日志分析",
		}
	}
	if legacyConfigured {
		return healthCheckResult{
			Name:       "日志绑定",
			Status:     cmdbmodel.CmdbHealthStatusHealthy,
			Score:      95,
			Message:    "已配置旧版日志来源，建议补充显式日志资源绑定",
			Dimension:  cmdbmodel.CmdbHealthDimensionLog,
			Evidence:   evidence,
			Diagnostic: "绑定日志资源",
		}
	}
	return healthCheckResult{
		Name:       "日志绑定",
		Status:     cmdbmodel.CmdbHealthStatusWarning,
		Score:      70,
		Message:    "未绑定日志资源，故障时缺少日志证据",
		Dimension:  cmdbmodel.CmdbHealthDimensionLog,
		Evidence:   evidence,
		Diagnostic: "绑定日志资源",
	}
}

func (e *businessHealthEngine) analyzeGeneric(resource *cmdbmodel.CmdbBusinessModelBinding) []healthCheckResult {
	health := derivedHealthFromResource(resource)
	if health.Status == cmdbmodel.CmdbHealthStatusHealthy {
		return []healthCheckResult{unknownCheck("资源健康接入", healthDimensionForResourceType(resource.ResourceType), "资源未接入专用健康分析器，等待监控快照", resourceIdentityEvidence(resource))}
	}
	return []healthCheckResult{
		{
			Name:       "资源资产状态",
			Status:     health.Status,
			Score:      health.Score,
			Message:    health.Message,
			Dimension:  healthDimensionForResourceType(resource.ResourceType),
			Evidence:   health.Evidence,
			Diagnostic: "查看资源详情",
		},
	}
}

func (e *businessHealthEngine) healthFromChecks(resource *cmdbmodel.CmdbBusinessModelBinding, checks []healthCheckResult) []nodeHealth {
	byDimension := make(map[string][]healthCheckResult)
	for _, check := range checks {
		dimension := normalizeHealthDimension(check.Dimension)
		if dimension == "" {
			dimension = healthDimensionForResourceType(resource.ResourceType)
		}
		check.Dimension = dimension
		byDimension[dimension] = append(byDimension[dimension], check)
	}

	result := make([]nodeHealth, 0, len(byDimension)+1)
	for dimension, items := range byDimension {
		result = append(result, checksToNodeHealth(dimension, items))
	}
	overall := checksToNodeHealth(cmdbmodel.CmdbHealthDimensionOverall, checks)
	overall.Evidence = mergeEvidence(overall.Evidence, map[string]interface{}{
		"resourceId":   resource.ID,
		"resourceName": resourceDisplayName(resource),
		"resourceType": resource.ResourceType,
	})
	result = append(result, overall)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Dimension < result[j].Dimension
	})
	return result
}

func checksToNodeHealth(dimension string, checks []healthCheckResult) nodeHealth {
	if len(checks) == 0 {
		return unknownHealth(dimension, fmt.Sprintf("暂无%s健康数据", healthDimensionLabel(dimension)))
	}
	ignoredOptionalUnknownCount := 0
	if dimension == cmdbmodel.CmdbHealthDimensionOverall {
		effectiveChecks := make([]healthCheckResult, 0, len(checks))
		for _, check := range checks {
			normalizedStatus, _ := normalizeHealthInput(check.Status, check.Score)
			if check.Optional && normalizedStatus == cmdbmodel.CmdbHealthStatusUnknown {
				ignoredOptionalUnknownCount++
				continue
			}
			effectiveChecks = append(effectiveChecks, check)
		}
		if len(effectiveChecks) > 0 && len(effectiveChecks) < len(checks) {
			checks = effectiveChecks
		} else {
			ignoredOptionalUnknownCount = 0
		}
	}
	scoreTotal := 0
	worst := checks[0]
	criticalCount := 0
	warningCount := 0
	unknownCount := 0
	checkEvidence := make([]map[string]interface{}, 0, len(checks))
	for _, check := range checks {
		check.Status, check.Score = normalizeHealthInput(check.Status, check.Score)
		scoreTotal += check.Score
		if isWorseCheck(check, worst) {
			worst = check
		}
		switch check.Status {
		case cmdbmodel.CmdbHealthStatusCritical:
			criticalCount++
		case cmdbmodel.CmdbHealthStatusWarning:
			warningCount++
		case cmdbmodel.CmdbHealthStatusUnknown:
			unknownCount++
		}
		checkEvidence = append(checkEvidence, map[string]interface{}{
			"name":       check.Name,
			"status":     check.Status,
			"score":      check.Score,
			"message":    check.Message,
			"evidence":   check.Evidence,
			"diagnostic": check.Diagnostic,
			"optional":   check.Optional,
		})
	}
	score := int(math.Round(float64(scoreTotal) / float64(len(checks))))
	status := cmdbmodel.CmdbHealthStatusHealthy
	switch {
	case criticalCount > 0:
		status = cmdbmodel.CmdbHealthStatusCritical
		score = minInt(score, worst.Score)
	case warningCount > 0:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = minInt(score, worst.Score)
	case unknownCount == len(checks):
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
	case unknownCount > 0:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = minInt(score, 80)
	}
	return nodeHealth{
		Dimension: dimension,
		Score:     clampScore(score),
		Status:    status,
		Source:    businessHealthEngineSource,
		Message:   worst.Message,
		Evidence: map[string]interface{}{
			"checks":                      checkEvidence,
			"checkCount":                  len(checks),
			"criticalCount":               criticalCount,
			"warningCount":                warningCount,
			"unknownCount":                unknownCount,
			"worstCheck":                  worst.Name,
			"ignoredOptionalUnknownCount": ignoredOptionalUnknownCount,
		},
	}
}

func (e *businessHealthEngine) buildCoverage(graph *businessGraphData, snapshots map[uint][]nodeHealth) map[string]interface{} {
	dimensionCoverage := make(map[string]map[string]int)
	for _, dimension := range append([]string{cmdbmodel.CmdbHealthDimensionOverall}, businessHealthDimensions()...) {
		dimensionCoverage[dimension] = map[string]int{
			"covered": 0,
			"unknown": 0,
			"total":   0,
		}
	}
	statusStats := map[string]int{
		cmdbmodel.CmdbHealthStatusHealthy:  0,
		cmdbmodel.CmdbHealthStatusWarning:  0,
		cmdbmodel.CmdbHealthStatusCritical: 0,
		cmdbmodel.CmdbHealthStatusUnknown:  0,
	}

	for _, resource := range graph.sortedResources() {
		nodeSnapshots := snapshots[resource.ID]
		overall := firstNodeHealthByDimension(nodeSnapshots, cmdbmodel.CmdbHealthDimensionOverall)
		statusStats[overall.Status]++
		for dimension := range dimensionCoverage {
			if dimension == cmdbmodel.CmdbHealthDimensionOverall || resourceSupportsDimension(resource.ResourceType, dimension) {
				dimensionCoverage[dimension]["total"]++
				health := firstNodeHealthByDimension(nodeSnapshots, dimension)
				if health.Status == "" || health.Status == cmdbmodel.CmdbHealthStatusUnknown {
					dimensionCoverage[dimension]["unknown"]++
				} else {
					dimensionCoverage[dimension]["covered"]++
				}
			}
		}
	}

	return map[string]interface{}{
		"resourceTotal":     len(graph.Resources),
		"relationTotal":     len(graph.Relations),
		"statusStats":       statusStats,
		"dimensionCoverage": dimensionCoverage,
		"analysisWindow":    businessHealthAnalysisWindow,
	}
}

func (e *businessHealthEngine) buildScoreBreakdown(graph *businessGraphData, snapshots map[uint][]nodeHealth) map[string]interface{} {
	dimensions := make([]map[string]interface{}, 0)
	for _, dimension := range businessHealthDimensions() {
		nodeCount := 0
		scoreTotal := 0
		worstScore := 100
		worstNode := map[string]interface{}{}
		for _, resource := range graph.descendantResources() {
			health := firstNodeHealthByDimension(snapshots[resource.ID], dimension)
			if health.Dimension == "" || health.Status == cmdbmodel.CmdbHealthStatusUnknown {
				continue
			}
			nodeCount++
			scoreTotal += health.Score
			if health.Score < worstScore {
				worstScore = health.Score
				worstNode = map[string]interface{}{
					"id":           resource.ID,
					"name":         resourceDisplayName(resource),
					"resourceType": resource.ResourceType,
					"score":        health.Score,
					"status":       health.Status,
					"message":      health.Message,
				}
			}
		}
		avgScore := 0
		if nodeCount > 0 {
			avgScore = int(math.Round(float64(scoreTotal) / float64(nodeCount)))
		}
		dimensions = append(dimensions, map[string]interface{}{
			"dimension":  dimension,
			"label":      healthDimensionLabel(dimension),
			"nodeCount":  nodeCount,
			"avgScore":   avgScore,
			"worstScore": worstScore,
			"worstNode":  worstNode,
		})
	}
	return map[string]interface{}{
		"strategy":   "node checks -> dimension average/worst -> business worst-status aggregation",
		"dimensions": dimensions,
	}
}

func (e *businessHealthEngine) evidenceEntry(resource *cmdbmodel.CmdbBusinessModelBinding, snapshots []nodeHealth, checks []healthCheckResult) map[string]interface{} {
	overall := firstNodeHealthByDimension(snapshots, cmdbmodel.CmdbHealthDimensionOverall)
	checkEntries := make([]map[string]interface{}, 0, len(checks))
	for _, check := range checks {
		checkEntries = append(checkEntries, map[string]interface{}{
			"name":       check.Name,
			"dimension":  check.Dimension,
			"status":     check.Status,
			"score":      check.Score,
			"message":    check.Message,
			"diagnostic": check.Diagnostic,
		})
	}
	return map[string]interface{}{
		"resourceId":    resource.ID,
		"resourceName":  resourceDisplayName(resource),
		"resourceType":  resource.ResourceType,
		"healthStatus":  overall.Status,
		"healthScore":   overall.Score,
		"message":       overall.Message,
		"checks":        checkEntries,
		"diagnostics":   diagnosisEntries(resource),
		"sourceType":    resource.SourceType,
		"sourceId":      resource.SourceID,
		"sourceKey":     resource.SourceKey,
		"generatedAt":   e.GeneratedAtString(),
		"analysisStage": "node_health",
	}
}

func (e *businessHealthEngine) GeneratedAtString() string {
	return time.Now().Format(time.RFC3339)
}

func (e *businessHealthEngine) createK8sClient(cluster *k8smodel.KubeCluster) (*kubernetes.Clientset, error) {
	if cluster == nil {
		return nil, fmt.Errorf("cluster is nil")
	}
	if strings.TrimSpace(cluster.Credential) == "" {
		return nil, fmt.Errorf("集群缺少 kubeconfig 配置")
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		return nil, err
	}
	config.Timeout = 10 * time.Second
	return kubernetes.NewForConfig(config)
}

func (e *businessHealthEngine) k8sClientFromResource(resource *cmdbmodel.CmdbBusinessModelBinding) (*k8smodel.KubeCluster, *kubernetes.Clientset, map[string]interface{}, error) {
	clusterID := uintFromResource(resource, "clusterId")
	if clusterID == 0 && strings.EqualFold(resource.SourceType, cmdbmodel.CmdbResourceTypeK8sCluster) && resource.SourceID > 0 {
		clusterID = resource.SourceID
	}
	if clusterID == 0 {
		clusterID = parseClusterIDFromSourceKey(resource.SourceKey)
	}
	evidence := mergeEvidence(resourceIdentityEvidence(resource), map[string]interface{}{"clusterId": clusterID})
	if clusterID == 0 {
		return nil, nil, evidence, fmt.Errorf("缺少 clusterId")
	}
	cluster, err := e.k8sClusterDao.GetByID(clusterID)
	if err != nil {
		return nil, nil, evidence, err
	}
	if cluster.Status != k8smodel.ClusterStatusRunning {
		return cluster, nil, evidence, fmt.Errorf("集群状态不是运行中: %s", cluster.GetStatusText())
	}
	clientset, err := e.createK8sClient(cluster)
	if err != nil {
		return cluster, nil, evidence, err
	}
	return cluster, clientset, evidence, nil
}

func (e *businessHealthEngine) listK8sWarningEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, kind, name string) ([]map[string]interface{}, int) {
	if clientset == nil {
		return nil, 0
	}
	options := metav1.ListOptions{}
	if kind != "" && name != "" {
		options.FieldSelector = "involvedObject.kind=" + kind + ",involvedObject.name=" + name
	} else if kind != "" {
		options.FieldSelector = "involvedObject.kind=" + kind
	} else if name != "" {
		options.FieldSelector = "involvedObject.name=" + name
	}
	events, err := clientset.CoreV1().Events(namespace).List(ctx, options)
	if err != nil {
		return []map[string]interface{}{{"error": err.Error()}}, 0
	}
	since := time.Now().Add(-businessHealthK8sEventLookback)
	warnings := make([]map[string]interface{}, 0)
	for _, event := range events.Items {
		last := eventLastTime(event)
		if last.Before(since) || event.Type != corev1.EventTypeWarning {
			continue
		}
		warnings = append(warnings, map[string]interface{}{
			"type":      event.Type,
			"reason":    event.Reason,
			"message":   event.Message,
			"count":     event.Count,
			"object":    event.InvolvedObject.Kind + "/" + event.InvolvedObject.Name,
			"lastTime":  last.Format(time.RFC3339),
			"namespace": event.Namespace,
		})
	}
	sort.Slice(warnings, func(i, j int) bool {
		left, _ := warnings[i]["lastTime"].(string)
		right, _ := warnings[j]["lastTime"].(string)
		return left > right
	})
	return limitMapList(warnings, 5), len(warnings)
}

func (g *businessGraphData) childrenOf(resourceID uint) []*cmdbmodel.CmdbBusinessModelBinding {
	children := make([]*cmdbmodel.CmdbBusinessModelBinding, 0)
	for _, relation := range g.Relations {
		if relation.SrcID != resourceID {
			continue
		}
		if child := g.Resources[relation.DstID]; child != nil {
			children = append(children, child)
		}
	}
	sort.Slice(children, func(i, j int) bool { return children[i].ID < children[j].ID })
	return children
}

func boundLogResources(graph *businessGraphData, resourceID uint) []*cmdbmodel.CmdbBusinessModelBinding {
	if graph == nil || resourceID == 0 {
		return nil
	}
	items := make([]*cmdbmodel.CmdbBusinessModelBinding, 0)
	seen := map[uint]struct{}{}
	for _, relation := range graph.Relations {
		if relation.SrcID != resourceID {
			continue
		}
		child := graph.Resources[relation.DstID]
		if child == nil || !isLogResourceType(child.ResourceType) {
			continue
		}
		if relation.RelationType != cmdbmodel.CmdbRelationEmitsLogTo &&
			relation.RelationType != cmdbmodel.CmdbRelationUses &&
			relation.RelationType != cmdbmodel.CmdbRelationDependsOn {
			continue
		}
		if _, ok := seen[child.ID]; ok {
			continue
		}
		seen[child.ID] = struct{}{}
		items = append(items, child)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func isLogResourceType(resourceType string) bool {
	return resourceType == cmdbmodel.CmdbResourceTypeLogProject || resourceType == cmdbmodel.CmdbResourceTypeLogStore
}

func resourceLogSourceConfigured(resource *cmdbmodel.CmdbBusinessModelBinding) (bool, string) {
	if resource == nil {
		return false, ""
	}
	sourceType := strings.ToLower(strings.TrimSpace(resource.SourceType))
	if resource.SourceID > 0 {
		switch sourceType {
		case cmdbmodel.CmdbResourceTypeLogProject, cmdbmodel.CmdbResourceTypeLogStore,
			"logtail_project", "logtail_store", "log-project", "log-store":
			return true, fmt.Sprintf("%s:%d", resource.SourceType, resource.SourceID)
		}
	}
	props := jsonToMap(resource.Properties)
	for _, key := range []string{"logStoreId", "logProjectId", "storeId", "projectId"} {
		if uintFromAny(props[key]) > 0 {
			return true, fmt.Sprintf("%s:%d", key, uintFromAny(props[key]))
		}
	}
	for _, key := range []string{"logSource", "logStore", "logProject", "storeCode", "projectCode", "日志来源"} {
		if value := stringFromAny(props[key]); value != "" {
			return true, value
		}
	}
	return false, ""
}

func boundLogResourceEvidence(items []*cmdbmodel.CmdbBusinessModelBinding) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"id":           item.ID,
			"name":         resourceDisplayName(item),
			"resourceType": item.ResourceType,
			"sourceType":   item.SourceType,
			"sourceId":     item.SourceID,
			"sourceKey":    item.SourceKey,
		})
	}
	return result
}

func (e *businessHealthEngine) recentFiringLogEvents(projectID, storeID uint, limit int) []logtailmodel.LogAlertEvent {
	if projectID == 0 && storeID == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 5
	}
	events := make([]logtailmodel.LogAlertEvent, 0)
	query := e.cmdb.dao.DB().Model(&logtailmodel.LogAlertEvent{}).Where("LOWER(status) = ?", "firing")
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	if storeID > 0 {
		query = query.Where("store_id = ?", storeID)
	}
	_ = query.Order("fired_at DESC, id DESC").Limit(limit).Find(&events).Error
	return events
}

func logHealthStatusFromEvents(events []logtailmodel.LogAlertEvent, enabled bool) (string, int, string) {
	if !enabled {
		return cmdbmodel.CmdbHealthStatusWarning, 70, "日志资源已停用，无法持续提供稳定性证据"
	}
	if len(events) == 0 {
		return cmdbmodel.CmdbHealthStatusHealthy, 100, "日志资源已接入，当前无 firing 日志告警"
	}
	for _, event := range events {
		if isCriticalLogSeverity(event.Severity) {
			return cmdbmodel.CmdbHealthStatusCritical, 35, fmt.Sprintf("存在 %d 条 firing 日志告警，包含严重级别事件", len(events))
		}
	}
	return cmdbmodel.CmdbHealthStatusWarning, 75, fmt.Sprintf("存在 %d 条 firing 日志告警", len(events))
}

func isCriticalLogSeverity(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical", "crit", "fatal", "error", "high", "p0", "p1", "严重", "致命":
		return true
	default:
		return false
	}
}

func logEventEvidence(events []logtailmodel.LogAlertEvent) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		item := map[string]interface{}{
			"id":            event.ID,
			"ruleId":        event.RuleID,
			"projectId":     event.ProjectID,
			"storeId":       event.StoreID,
			"sourceName":    event.SourceName,
			"severity":      event.Severity,
			"status":        event.Status,
			"summary":       event.Summary,
			"triggerReason": event.TriggerReason,
			"matchCount":    event.MatchCount,
		}
		if event.FiredAt != nil {
			item["firedAt"] = event.FiredAt.Format(time.RFC3339)
		}
		result = append(result, item)
	}
	return result
}

func (e *businessHealthEngine) resolveHost(resource *cmdbmodel.CmdbBusinessModelBinding, hostID uint) (*cmdbmodel.CmdbHost, error) {
	if hostID > 0 {
		host, err := e.hostDao.GetCmdbHostById(hostID)
		if err == nil {
			return &host, nil
		}
	}
	for _, candidate := range []string{
		stringFromResource(resource, "hostIp"),
		stringFromResource(resource, "privateIp"),
		stringFromResource(resource, "sshIp"),
		stringFromResource(resource, "ip"),
		resource.InstanceId,
		resource.SourceKey,
		resource.Name,
		resource.DisplayName,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if host, err := e.hostDao.GetCmdbHostByInstanceIdentifier(candidate); err == nil {
			return &host, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func hostMetricsCheck(hostID uint, metrics *cmdbmodel.HostMetrics) healthCheckResult {
	if metrics == nil {
		return unknownCheck("主机基础监控", cmdbmodel.CmdbHealthDimensionInfra, "主机基础监控暂无数据", map[string]interface{}{"hostId": hostID})
	}
	if metrics.OnlineStatus == 1 {
		return criticalCheck("主机基础监控", cmdbmodel.CmdbHealthDimensionInfra, "主机监控离线", map[string]interface{}{"hostId": hostID, "onlineStatus": metrics.OnlineStatus})
	}
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	risks := make([]string, 0)
	if metrics.CPUUsage >= 90 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score -= 25
		risks = append(risks, "CPU 使用率超过 90%")
	} else if metrics.CPUUsage >= 80 {
		status = cmdbmodel.CmdbHealthStatusWarning
		score -= 10
		risks = append(risks, "CPU 使用率超过 80%")
	}
	if metrics.MemoryUsage >= 90 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score -= 25
		risks = append(risks, "内存使用率超过 90%")
	} else if metrics.MemoryUsage >= 80 && status != cmdbmodel.CmdbHealthStatusCritical {
		status = cmdbmodel.CmdbHealthStatusWarning
		score -= 10
		risks = append(risks, "内存使用率超过 80%")
	}
	if metrics.DiskUsage >= 90 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score -= 30
		risks = append(risks, "磁盘使用率超过 90%")
	} else if metrics.DiskUsage >= 80 && status != cmdbmodel.CmdbHealthStatusCritical {
		status = cmdbmodel.CmdbHealthStatusWarning
		score -= 10
		risks = append(risks, "磁盘使用率超过 80%")
	}
	message := "主机 CPU / 内存 / 磁盘指标正常"
	if len(risks) > 0 {
		message = strings.Join(risks, "，")
	}
	return healthCheckResult{
		Name:      "主机基础监控",
		Status:    status,
		Score:     clampScore(score),
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionInfra,
		Evidence: map[string]interface{}{
			"hostId":       hostID,
			"onlineStatus": metrics.OnlineStatus,
			"cpuUsage":     metrics.CPUUsage,
			"memoryUsage":  metrics.MemoryUsage,
			"diskUsage":    metrics.DiskUsage,
			"load5":        metrics.Load5,
			"risks":        risks,
		},
		Diagnostic: "查看主机监控",
	}
}

func hostServiceMetricsCheck(hostID uint, serviceName string, monitorAll bool, services []cmdbmodel.ServiceInfo) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	risks := make([]string, 0)
	matched := make([]map[string]interface{}, 0)
	checked := 0

	for _, service := range services {
		if !monitorAll && !strings.EqualFold(strings.TrimSpace(service.Name), strings.TrimSpace(serviceName)) {
			continue
		}
		checked++
		serviceRisks := make([]string, 0)
		if service.CPU >= 90 {
			serviceRisks = append(serviceRisks, "CPU 使用率超过 90%")
		} else if service.CPU >= 80 {
			serviceRisks = append(serviceRisks, "CPU 使用率超过 80%")
		}
		if service.Memory >= 90 {
			serviceRisks = append(serviceRisks, "内存使用率超过 90%")
		} else if service.Memory >= 80 {
			serviceRisks = append(serviceRisks, "内存使用率超过 80%")
		}
		if len(serviceRisks) > 0 {
			risks = append(risks, fmt.Sprintf("%s: %s", service.Name, strings.Join(serviceRisks, "，")))
		}
		matched = append(matched, map[string]interface{}{
			"name":   service.Name,
			"cpu":    service.CPU,
			"memory": service.Memory,
			"risks":  serviceRisks,
		})
	}

	switch {
	case len(services) == 0:
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		risks = append(risks, "主机未返回服务进程指标")
	case !monitorAll && checked == 0:
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 35
		risks = append(risks, "指定服务未发现或已停止: "+serviceName)
	case len(risks) > 0:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 85
		for _, item := range matched {
			cpu, _ := item["cpu"].(float64)
			memory, _ := item["memory"].(float64)
			if cpu >= 90 || memory >= 90 {
				status = cmdbmodel.CmdbHealthStatusCritical
				score = 45
				break
			}
			if cpu >= 80 || memory >= 80 {
				score = minInt(score, 70)
			}
		}
	}

	target := "全部服务"
	if !monitorAll {
		target = serviceName
	}
	message := fmt.Sprintf("主机服务监控正常，目标: %s，已检查 %d 个服务", target, checked)
	if len(risks) > 0 {
		message = strings.Join(risks, "；")
	}
	return healthCheckResult{
		Name:      "主机服务监控",
		Status:    status,
		Score:     clampScore(score),
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionBusiness,
		Evidence: map[string]interface{}{
			"hostId":             hostID,
			"serviceName":        serviceName,
			"monitorAllServices": monitorAll,
			"serviceTotal":       len(services),
			"checkedCount":       checked,
			"matchedServices":    limitMapList(matched, 10),
			"risks":              risks,
		},
		Diagnostic: "查看主机进程",
	}
}

func databaseAssetCheck(assetID uint, asset cmdbmodel.CmdbSQL) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	risks := make([]string, 0)
	if asset.Status == cmdbmodel.DatabaseStatusError || asset.Status == cmdbmodel.DatabaseStatusOffline || asset.Status == cmdbmodel.DatabaseStatusStopped {
		status = cmdbmodel.CmdbHealthStatusCritical
		score -= 35
		risks = append(risks, "数据库资产状态异常")
	}
	if asset.AuthStatus == cmdbmodel.DatabaseAuthStatusFailed {
		if status != cmdbmodel.CmdbHealthStatusCritical {
			status = cmdbmodel.CmdbHealthStatusWarning
		}
		score -= 15
		risks = append(risks, "数据库管控认证失败，不代表实例运行不可用")
	} else if asset.AuthStatus != cmdbmodel.DatabaseAuthStatusSuccess {
		if status != cmdbmodel.CmdbHealthStatusCritical {
			status = cmdbmodel.CmdbHealthStatusWarning
		}
		score -= 10
		risks = append(risks, "数据库未认证")
	}
	message := "数据库资产状态正常"
	if len(risks) > 0 {
		message = strings.Join(risks, "，")
	}
	return healthCheckResult{
		Name:      "数据库资产状态",
		Status:    status,
		Score:     clampScore(score),
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionMiddleware,
		Evidence: map[string]interface{}{
			"assetId":                 assetID,
			"assetName":               firstNonEmpty(asset.Name, asset.InstanceName),
			"dbType":                  asset.Type,
			"dbStatus":                asset.Status,
			"authStatus":              asset.AuthStatus,
			"monitorStatus":           asset.MonitorStatus,
			"alertStatus":             asset.AlertStatus,
			"prometheusAccountId":     asset.PrometheusAccountID,
			"prometheusLabelSelector": asset.PrometheusLabelSelector,
			"risks":                   risks,
		},
		Diagnostic: "查看数据库监控",
	}
}

type businessPrometheusClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

type businessPrometheusAPIResponse struct {
	Status string          `json:"status"`
	Error  string          `json:"error"`
	Data   json.RawMessage `json:"data"`
}

type businessPrometheusQueryData struct {
	ResultType string                          `json:"resultType"`
	Result     []businessPrometheusQueryResult `json:"result"`
}

type businessPrometheusQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type businessPrometheusTargetsData struct {
	ActiveTargets []businessPrometheusTarget `json:"activeTargets"`
}

type businessPrometheusTarget struct {
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	Labels           map[string]string `json:"labels"`
	ScrapePool       string            `json:"scrapePool"`
	ScrapeURL        string            `json:"scrapeUrl"`
	GlobalURL        string            `json:"globalUrl"`
	Health           string            `json:"health"`
	LastError        string            `json:"lastError"`
}

type businessDBMetricRule struct {
	Key              string
	Name             string
	PromQL           string
	Unit             string
	WarnAbove        float64
	CriticalAbove    float64
	HasWarnAbove     bool
	HasCriticalAbove bool
}

type businessDBSignalSummary struct {
	Signals      []map[string]interface{}
	Risks        []string
	HasCritical  bool
	RuleCount    int
	CheckedCount int
	MissingCount int
}

type businessDatabaseRuntimeTargetResult struct {
	Row              businessPrometheusQueryResult
	Target           businessPrometheusTarget
	TargetLabels     map[string]string
	TargetScore      int
	Selector         string
	PromQL           string
	TargetMatched    bool
	SelectorsTried   []string
	CandidateTargets []map[string]interface{}
}

func (e *businessHealthEngine) checkDatabaseRuntimeFromPrometheus(ctx context.Context, assetID uint, asset cmdbmodel.CmdbSQL) []healthCheckResult {
	dbTypeName := businessDatabaseTypeName(asset.Type)
	upMetric := businessDatabaseUpMetric(asset.Type)
	evidence := map[string]interface{}{
		"assetId":                 assetID,
		"assetName":               firstNonEmpty(asset.Name, asset.InstanceName),
		"dbType":                  asset.Type,
		"dbTypeName":              dbTypeName,
		"hostIp":                  asset.HostIP,
		"endpoint":                asset.Endpoint,
		"port":                    asset.Port,
		"prometheusAccountId":     asset.PrometheusAccountID,
		"prometheusLabelSelector": strings.TrimSpace(asset.PrometheusLabelSelector),
		"analysisWindow":          businessHealthAnalysisWindow,
	}
	single := func(status string, score int, message string, evidence map[string]interface{}) []healthCheckResult {
		return []healthCheckResult{businessDatabaseRuntimeCheckResult(status, score, message, evidence)}
	}
	if upMetric == "" {
		return single(
			cmdbmodel.CmdbHealthStatusUnknown,
			0,
			"暂不支持该数据库类型的实时监控评分: "+dbTypeName,
			evidence,
		)
	}
	evidence["upMetric"] = upMetric

	if asset.PrometheusAccountID == 0 {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"未配置数据库 Prometheus 账号，无法基于运行态指标判断实例可用性",
			evidence,
		)
	}
	account, err := e.accountDao.GetByID(asset.PrometheusAccountID)
	if err != nil {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"数据库 Prometheus 账号读取失败: "+err.Error(),
			mergeEvidence(evidence, map[string]interface{}{"error": err.Error()}),
		)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"数据库 Prometheus 账号解密失败: "+err.Error(),
			mergeEvidence(evidence, map[string]interface{}{"error": err.Error()}),
		)
	}

	client := newBusinessPrometheusClient(account, password)
	if client.baseURL == "" {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"数据库 Prometheus 地址为空，无法查询运行态指标",
			evidence,
		)
	}
	evidence["prometheus"] = client.baseURL

	checkCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	selector := strings.TrimSpace(asset.PrometheusLabelSelector)
	upPromQL := businessPrometheusMetricQuery(upMetric, selector)
	rows, err := client.queryInstantMetricRows(checkCtx, upPromQL)
	if err != nil {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"数据库运行态监控查询失败: "+err.Error(),
			mergeEvidence(evidence, map[string]interface{}{
				"promql": upPromQL,
				"error":  err.Error(),
			}),
		)
	}

	row, matchScore, matched := businessSelectDatabaseMetricRow(asset, selector, rows)
	var matchedTarget businessPrometheusTarget
	var matchedTargetScore int
	targetMatched := false
	if !matched && selector == "" {
		targetRuntime, targetErr := businessResolveDatabaseRuntimeFromTargets(checkCtx, client, asset, upMetric)
		if targetErr == nil {
			row = targetRuntime.Row
			matchScore = targetRuntime.TargetScore
			matched = true
			selector = targetRuntime.Selector
			upPromQL = targetRuntime.PromQL
			matchedTarget = targetRuntime.Target
			matchedTargetScore = targetRuntime.TargetScore
			targetMatched = true
		} else if targetRuntime.TargetMatched {
			return single(
				cmdbmodel.CmdbHealthStatusWarning,
				70,
				"数据库 Prometheus target 已匹配，但运行态指标查询失败: "+targetErr.Error(),
				mergeEvidence(evidence, map[string]interface{}{
					"promql":           targetRuntime.PromQL,
					"targetMatched":    true,
					"targetScore":      targetRuntime.TargetScore,
					"targetHealth":     targetRuntime.Target.Health,
					"scrapeURL":        firstNonEmpty(targetRuntime.Target.ScrapeURL, targetRuntime.Target.GlobalURL),
					"targetLabels":     targetRuntime.TargetLabels,
					"selectorTried":    targetRuntime.SelectorsTried,
					"candidateTargets": targetRuntime.CandidateTargets,
					"error":            targetErr.Error(),
				}),
			)
		}
	}
	if !matched {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			fmt.Sprintf("未匹配到 %s 对应的数据库监控序列，请检查 Prometheus label selector 或资产 HostIP/Endpoint", upMetric),
			mergeEvidence(evidence, map[string]interface{}{
				"promql":      upPromQL,
				"seriesCount": len(rows),
			}),
		)
	}

	upValue, ok := businessPrometheusSampleValue(row.Value)
	if !ok {
		return single(
			cmdbmodel.CmdbHealthStatusWarning,
			70,
			"数据库运行态指标返回值无法解析",
			mergeEvidence(evidence, map[string]interface{}{
				"promql": upPromQL,
				"labels": row.Metric,
			}),
		)
	}

	selectorForSignals := selector
	if selectorForSignals == "" {
		selectorForSignals = businessBuildPrometheusSelector(row.Metric, asset.Type)
	}
	evidence = mergeEvidence(evidence, map[string]interface{}{
		"promql":          upPromQL,
		"upValue":         roundBusinessFloat(upValue, 2),
		"matched":         true,
		"matchScore":      matchScore,
		"matchBy":         businessMetricMatchBy(asset, row.Metric),
		"matchedLabels":   row.Metric,
		"runtimeSelector": selectorForSignals,
	})

	if !targetMatched {
		matchedTarget, matchedTargetScore, targetMatched = businessFindDatabasePrometheusTarget(asset, row.Metric, client, checkCtx)
	}
	if targetMatched {
		evidence = mergeEvidence(evidence, map[string]interface{}{
			"targetMatched": true,
			"targetScore":   matchedTargetScore,
			"targetHealth":  matchedTarget.Health,
			"scrapeURL":     firstNonEmpty(matchedTarget.ScrapeURL, matchedTarget.GlobalURL),
			"lastError":     matchedTarget.LastError,
			"targetLabels":  businessMergeTargetLabels(matchedTarget),
		})
		if strings.TrimSpace(matchedTarget.Health) != "" && !strings.EqualFold(matchedTarget.Health, "up") {
			message := "数据库 Prometheus target 异常: " + matchedTarget.Health
			if strings.TrimSpace(matchedTarget.LastError) != "" {
				message += "，" + matchedTarget.LastError
			}
			return single(
				cmdbmodel.CmdbHealthStatusCritical,
				35,
				message,
				evidence,
			)
		}
	}

	if upValue < 1 {
		return single(
			cmdbmodel.CmdbHealthStatusCritical,
			35,
			fmt.Sprintf("%s 显示数据库 exporter 无法连接实例，%s=%.0f", dbTypeName, upMetric, upValue),
			evidence,
		)
	}

	checks := []healthCheckResult{
		businessDatabaseRuntimeCheckResult(
			cmdbmodel.CmdbHealthStatusHealthy,
			100,
			fmt.Sprintf("%s 可用性正常，%s=%.0f", dbTypeName, upMetric, upValue),
			mergeEvidence(evidence, map[string]interface{}{"category": "availability"}),
		),
	}

	targetLabels := map[string]string{}
	targetScrapeURL := ""
	if targetMatched {
		targetLabels = businessMergeTargetLabels(matchedTarget)
		targetScrapeURL = firstNonEmpty(matchedTarget.ScrapeURL, matchedTarget.GlobalURL)
	}
	hostCandidates := businessDatabaseHostSignalCandidates(asset, row.Metric, targetLabels, targetScrapeURL)
	runtimeHostIP, hostSummary := businessCheckDatabaseHostSignals(checkCtx, client, hostCandidates)
	hostEvidence := mergeEvidence(evidence, map[string]interface{}{
		"category":        "host_pressure",
		"hostIp":          runtimeHostIP,
		"hostCandidates":  hostCandidates,
		"signals":         hostSummary.Signals,
		"signalRisks":     hostSummary.Risks,
		"ruleCount":       hostSummary.RuleCount,
		"checkedCount":    hostSummary.CheckedCount,
		"missingCount":    hostSummary.MissingCount,
		"runtimeSelector": selectorForSignals,
	})
	checks = append(checks, businessDatabaseSignalCheckResult(
		"数据库宿主机压力",
		cmdbmodel.CmdbHealthDimensionInfra,
		hostSummary,
		"数据库所在主机 CPU / 内存 / 磁盘压力正常",
		"未识别数据库宿主机地址，无法生成宿主机压力评分规则",
		"数据库宿主机监控指标未匹配，无法判断宿主机压力",
		hostEvidence,
		"查看主机监控",
	))

	serviceRules := businessDatabaseServiceSignalRules(asset, selectorForSignals)
	serviceSummary := businessCheckDatabaseSignalMetrics(checkCtx, client, serviceRules)
	serviceEvidence := mergeEvidence(evidence, map[string]interface{}{
		"category":        "database_service_pressure",
		"signals":         serviceSummary.Signals,
		"signalRisks":     serviceSummary.Risks,
		"ruleCount":       serviceSummary.RuleCount,
		"checkedCount":    serviceSummary.CheckedCount,
		"missingCount":    serviceSummary.MissingCount,
		"runtimeSelector": selectorForSignals,
	})
	checks = append(checks, businessDatabaseSignalCheckResult(
		"数据库服务压力",
		cmdbmodel.CmdbHealthDimensionMiddleware,
		serviceSummary,
		dbTypeName+" 服务指标正常",
		"当前数据库类型暂无可用的服务压力评分规则",
		dbTypeName+" 服务监控指标未匹配，无法判断数据库服务压力",
		serviceEvidence,
		"查看数据库监控",
	))

	return checks
}

func businessDatabaseSignalCheckResult(name, dimension string, summary businessDBSignalSummary, healthyMessage, noRuleMessage, missingMessage string, evidence map[string]interface{}, diagnostic string) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := healthyMessage
	optional := false
	switch {
	case summary.RuleCount == 0:
		status = cmdbmodel.CmdbHealthStatusUnknown
		score = 0
		message = noRuleMessage
		optional = true
	case summary.CheckedCount == 0:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 80
		message = missingMessage
	case len(summary.Risks) > 0:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 75
		message = strings.Join(summary.Risks, "；")
		if summary.HasCritical {
			status = cmdbmodel.CmdbHealthStatusCritical
			score = 45
		}
	}
	return healthCheckResult{
		Name:       name,
		Status:     status,
		Score:      clampScore(score),
		Message:    message,
		Dimension:  dimension,
		Evidence:   evidence,
		Diagnostic: diagnostic,
		Optional:   optional,
	}
}

func businessDatabaseRuntimeCheckResult(status string, score int, message string, evidence map[string]interface{}) healthCheckResult {
	return healthCheckResult{
		Name:       "数据库可用性",
		Status:     status,
		Score:      clampScore(score),
		Message:    message,
		Dimension:  cmdbmodel.CmdbHealthDimensionMiddleware,
		Evidence:   evidence,
		Diagnostic: "查看数据库监控",
	}
}

func businessResolveDatabaseRuntimeFromTargets(ctx context.Context, client *businessPrometheusClient, asset cmdbmodel.CmdbSQL, upMetric string) (businessDatabaseRuntimeTargetResult, error) {
	result := businessDatabaseRuntimeTargetResult{
		SelectorsTried:   make([]string, 0),
		CandidateTargets: make([]map[string]interface{}, 0),
	}
	candidates, err := businessListDatabasePrometheusTargetCandidates(asset, client, ctx)
	if err != nil {
		return result, err
	}
	if len(candidates) == 0 {
		return result, fmt.Errorf("no matching prometheus target found")
	}
	result.TargetMatched = true
	var lastErr error
	for _, candidate := range candidates {
		labels := businessMergeTargetLabels(candidate.Target)
		selector := businessBuildPrometheusSelector(labels, asset.Type)
		result.CandidateTargets = append(result.CandidateTargets, map[string]interface{}{
			"score":     candidate.Score,
			"selector":  selector,
			"health":    candidate.Target.Health,
			"scrapeURL": firstNonEmpty(candidate.Target.ScrapeURL, candidate.Target.GlobalURL),
			"labels":    labels,
		})
		if selector == "" {
			continue
		}
		promql := businessPrometheusMetricQuery(upMetric, selector)
		result.SelectorsTried = append(result.SelectorsTried, selector)
		rows, queryErr := client.queryInstantMetricRows(ctx, promql)
		if queryErr != nil {
			lastErr = queryErr
			continue
		}
		row, _, rowMatched := businessSelectDatabaseMetricRow(asset, selector, rows)
		if !rowMatched && len(rows) > 0 {
			row = rows[0]
			rowMatched = true
		}
		if !rowMatched {
			lastErr = fmt.Errorf("empty result")
			continue
		}
		result.Row = row
		result.Target = candidate.Target
		result.TargetLabels = labels
		result.TargetScore = candidate.Score
		result.Selector = selector
		result.PromQL = promql
		return result, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("empty result")
	}
	return result, lastErr
}

func newBusinessPrometheusClient(account *configmodel.AccountAuth, password string) *businessPrometheusClient {
	return &businessPrometheusClient{
		baseURL:  buildPrometheusEndpoint(account.Host, account.Port),
		username: account.Name,
		password: password,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *businessPrometheusClient) queryInstantMetricRows(ctx context.Context, promql string) ([]businessPrometheusQueryResult, error) {
	var data businessPrometheusQueryData
	if err := c.get(ctx, "/api/v1/query", url.Values{"query": []string{promql}}, &data); err != nil {
		return nil, err
	}
	if len(data.Result) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	return data.Result, nil
}

func (c *businessPrometheusClient) queryInstantValue(ctx context.Context, promql string) (float64, error) {
	rows, err := c.queryInstantMetricRows(ctx, promql)
	if err != nil {
		return 0, err
	}
	value, ok := businessPrometheusSampleValue(rows[0].Value)
	if !ok {
		return 0, fmt.Errorf("invalid prometheus sample")
	}
	return value, nil
}

func (c *businessPrometheusClient) targets(ctx context.Context) ([]businessPrometheusTarget, error) {
	var data businessPrometheusTargetsData
	if err := c.get(ctx, "/api/v1/targets", url.Values{"state": []string{"active"}}, &data); err != nil {
		return nil, err
	}
	return data.ActiveTargets, nil
}

func (c *businessPrometheusClient) get(ctx context.Context, path string, values url.Values, out interface{}) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = values.Encode()
	req.Header.Set("Accept", "application/json")
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("prometheus returned HTTP %d", resp.StatusCode)
	}

	var apiResp businessPrometheusAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}
	if apiResp.Status != "success" {
		if apiResp.Error != "" {
			return fmt.Errorf("%s", apiResp.Error)
		}
		return fmt.Errorf("prometheus query failed")
	}
	return json.Unmarshal(apiResp.Data, out)
}

func businessDatabaseUpMetric(dbType int) string {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return "mysql_up"
	case cmdbmodel.DatabaseTypePostgreSQL:
		return "pg_up"
	case cmdbmodel.DatabaseTypeRedis:
		return "redis_up"
	default:
		return ""
	}
}

func businessDatabaseTypeName(dbType int) string {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return "MySQL"
	case cmdbmodel.DatabaseTypePostgreSQL:
		return "PostgreSQL"
	case cmdbmodel.DatabaseTypeOracle:
		return "Oracle"
	case cmdbmodel.DatabaseTypeRedis:
		return "Redis"
	case cmdbmodel.DatabaseTypeMongoDB:
		return "MongoDB"
	case cmdbmodel.DatabaseTypeSQLServer:
		return "SQL Server"
	case cmdbmodel.DatabaseTypeElasticsearch:
		return "Elasticsearch"
	default:
		return fmt.Sprintf("数据库类型 %d", dbType)
	}
}

func businessPrometheusMetricQuery(metric, selector string) string {
	if strings.TrimSpace(selector) == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func businessSelectDatabaseMetricRow(asset cmdbmodel.CmdbSQL, selector string, rows []businessPrometheusQueryResult) (businessPrometheusQueryResult, int, bool) {
	if len(rows) == 0 {
		return businessPrometheusQueryResult{}, 0, false
	}
	if strings.TrimSpace(selector) != "" {
		best := rows[0]
		bestValue := -1.0
		for _, row := range rows {
			value, ok := businessPrometheusSampleValue(row.Value)
			if !ok {
				continue
			}
			if value > bestValue {
				best = row
				bestValue = value
			}
		}
		return best, 100, true
	}

	bestScore := 0
	var best businessPrometheusQueryResult
	for _, row := range rows {
		score := businessScoreDatabaseMetricLabels(asset, row.Metric)
		if score > bestScore {
			bestScore = score
			best = row
		}
	}
	return best, bestScore, bestScore > 0
}

func businessCheckDatabaseSignalMetrics(ctx context.Context, client *businessPrometheusClient, rules []businessDBMetricRule) businessDBSignalSummary {
	summary := businessDBSignalSummary{
		Signals:   make([]map[string]interface{}, 0, len(rules)),
		Risks:     make([]string, 0),
		RuleCount: len(rules),
	}
	signals := make([]map[string]interface{}, 0, len(rules))
	for _, rule := range rules {
		value, err := client.queryInstantValue(ctx, rule.PromQL)
		if err != nil {
			summary.MissingCount++
			signals = append(signals, map[string]interface{}{
				"key":    rule.Key,
				"name":   rule.Name,
				"promql": rule.PromQL,
				"status": "missing",
				"error":  err.Error(),
			})
			continue
		}

		summary.CheckedCount++
		status := cmdbmodel.CmdbHealthStatusHealthy
		if rule.HasCriticalAbove && value >= rule.CriticalAbove {
			status = cmdbmodel.CmdbHealthStatusCritical
			summary.HasCritical = true
			summary.Risks = append(summary.Risks, fmt.Sprintf("%s达到严重阈值: %.2f%s", rule.Name, value, rule.Unit))
		} else if rule.HasWarnAbove && value >= rule.WarnAbove {
			status = cmdbmodel.CmdbHealthStatusWarning
			summary.Risks = append(summary.Risks, fmt.Sprintf("%s偏高: %.2f%s", rule.Name, value, rule.Unit))
		}
		signals = append(signals, map[string]interface{}{
			"key":    rule.Key,
			"name":   rule.Name,
			"value":  roundBusinessFloat(value, 2),
			"unit":   rule.Unit,
			"status": status,
			"promql": rule.PromQL,
		})
	}
	summary.Signals = limitMapList(signals, 12)
	return summary
}

func businessCheckDatabaseHostSignals(ctx context.Context, client *businessPrometheusClient, candidates []string) (string, businessDBSignalSummary) {
	var bestHost string
	var bestSummary businessDBSignalSummary
	for _, host := range candidates {
		rules := businessDatabaseHostSignalRules(host)
		summary := businessCheckDatabaseSignalMetrics(ctx, client, rules)
		if bestHost == "" || summary.CheckedCount > bestSummary.CheckedCount || (summary.CheckedCount == bestSummary.CheckedCount && summary.MissingCount < bestSummary.MissingCount) {
			bestHost = host
			bestSummary = summary
		}
		if summary.CheckedCount > 0 {
			return host, summary
		}
	}
	return bestHost, bestSummary
}

func businessDatabaseHostSignalRules(hostIP string) []businessDBMetricRule {
	rules := make([]businessDBMetricRule, 0, 3)
	nodeSelector := businessNodeInstanceSelector(hostIP)
	if nodeSelector != "" {
		rules = append(rules,
			businessAboveRule("hostCpuUsage", "数据库所在主机 CPU 使用率", businessNodeCPUUsageQuery(nodeSelector), "%", 80, 90),
			businessAboveRule("hostMemoryUsage", "数据库所在主机内存使用率", businessNodeMemoryUsageQuery(nodeSelector), "%", 80, 90),
			businessAboveRule("hostDiskUsage", "数据库所在主机磁盘使用率", businessNodeDiskUsageQuery(nodeSelector), "%", 80, 90),
		)
	}
	return rules
}

func businessDatabaseServiceSignalRules(asset cmdbmodel.CmdbSQL, selector string) []businessDBMetricRule {
	rules := make([]businessDBMetricRule, 0, 7)
	switch asset.Type {
	case cmdbmodel.DatabaseTypeMySQL:
		rules = append(rules,
			businessAboveRule("mysqlConnectionsUsage", "MySQL 连接使用率", businessMySQLConnectionUsageQuery(selector), "%", 80, 90),
			businessAboveRule("mysqlAbortedConnects", "MySQL 异常连接速率", businessPrometheusSumRateQuery("mysql_global_status_aborted_connects", selector), "/s", 0.1, 1),
			businessAboveRule("mysqlLockWaits", "MySQL 行锁等待速率", businessPrometheusSumRateQuery("mysql_global_status_innodb_row_lock_waits", selector), "/s", 0.1, 2),
		)
	case cmdbmodel.DatabaseTypeRedis:
		rules = append(rules,
			businessAboveRule("redisBlockedClients", "Redis 阻塞客户端", businessPrometheusSumQuery("redis_blocked_clients", selector), "个", 1, 10),
			businessAboveRule("redisErrorReplies", "Redis 错误响应速率", businessPrometheusSumRateQuery("redis_total_error_replies", selector), "/s", 0.1, 5),
			businessAboveRule("redisEvictedKeys", "Redis Key 淘汰速率", businessPrometheusSumRateQuery("redis_evicted_keys_total", selector), "/s", 0.1, 10),
		)
	case cmdbmodel.DatabaseTypePostgreSQL:
		rules = append(rules,
			businessAboveRule("pgScrapeError", "PostgreSQL 采集错误", businessPrometheusMaxQuery("pg_exporter_last_scrape_error", selector), "", 1, 1),
			businessAboveRule("pgConnectionsUsage", "PostgreSQL 连接使用率", businessPostgreSQLConnectionUsageQuery(selector), "%", 80, 90),
			businessAboveRule("pgDeadlocks", "PostgreSQL 死锁速率", businessPrometheusSumRateQuery("pg_stat_database_deadlocks", selector), "/s", 0.01, 0.1),
		)
	}
	return rules
}

func businessAboveRule(key, name, promql, unit string, warnAbove, criticalAbove float64) businessDBMetricRule {
	return businessDBMetricRule{
		Key:              key,
		Name:             name,
		PromQL:           promql,
		Unit:             unit,
		WarnAbove:        warnAbove,
		CriticalAbove:    criticalAbove,
		HasWarnAbove:     true,
		HasCriticalAbove: true,
	}
}

func businessPrometheusSampleValue(value []interface{}) (float64, bool) {
	if len(value) < 2 {
		return 0, false
	}
	raw, ok := value[1].(string)
	if !ok {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, false
	}
	return parsed, true
}

func businessFindDatabasePrometheusTarget(asset cmdbmodel.CmdbSQL, metricLabels map[string]string, client *businessPrometheusClient, ctx context.Context) (businessPrometheusTarget, int, bool) {
	candidates, err := businessListDatabasePrometheusTargetCandidates(asset, client, ctx)
	if err != nil {
		return businessPrometheusTarget{}, 0, false
	}
	if len(candidates) == 0 {
		return businessPrometheusTarget{}, 0, false
	}
	if len(metricLabels) == 0 {
		return candidates[0].Target, candidates[0].Score, true
	}
	best := candidates[0]
	bestScore := best.Score
	for _, candidate := range candidates {
		score := candidate.Score
		labels := businessMergeTargetLabels(candidate.Target)
		if businessLabelsOverlap(metricLabels, labels, "job", "instance", "target", "addr", "__address__", "alias") {
			score += 50
		}
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best.Target, bestScore, bestScore > 0
}

type businessDatabaseTargetCandidate struct {
	Target businessPrometheusTarget
	Score  int
}

func businessListDatabasePrometheusTargetCandidates(asset cmdbmodel.CmdbSQL, client *businessPrometheusClient, ctx context.Context) ([]businessDatabaseTargetCandidate, error) {
	targets, err := client.targets(ctx)
	if err != nil {
		return nil, err
	}
	candidates := make([]businessDatabaseTargetCandidate, 0)
	for _, target := range targets {
		labels := businessMergeTargetLabels(target)
		haystack := businessPrometheusTargetHaystack(target, labels)
		if businessIsNodeExporterTarget(labels, haystack) {
			continue
		}
		score := businessScoreDatabaseMetricLabels(asset, labels)
		if businessIsDatabaseTargetCandidate(asset.Type, labels, haystack) {
			score += 50
		}
		if businessTargetAddressMatchesAsset(asset, target.ScrapeURL) || businessTargetAddressMatchesAsset(asset, target.GlobalURL) {
			score += 40
		}
		if score == 0 && asset.Type == cmdbmodel.DatabaseTypeRedis && businessRedisExporterProxyMatchesAsset(asset, target) {
			score += 60
		}
		if score > 0 {
			candidates = append(candidates, businessDatabaseTargetCandidate{Target: target, Score: score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	return candidates, nil
}

func businessScoreDatabaseMetricLabels(asset cmdbmodel.CmdbSQL, labels map[string]string) int {
	haystackParts := make([]string, 0, len(labels))
	for key, value := range labels {
		haystackParts = append(haystackParts, key+"="+value)
	}
	haystack := strings.ToLower(strings.Join(haystackParts, " "))
	score := 0
	addressMatched := false
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__", "alias", "hostname", "job"} {
		if businessTargetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			addressMatched = true
			score += 100
		}
	}
	for _, needle := range businessDatabaseTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	if !addressMatched && score < 20 {
		return 0
	}
	return score
}

func businessPrometheusTargetHaystack(target businessPrometheusTarget, labels map[string]string) string {
	parts := make([]string, 0, len(labels)+4)
	for key, value := range labels {
		parts = append(parts, key+"="+value)
	}
	parts = append(parts, target.ScrapePool, target.ScrapeURL, target.GlobalURL, target.Health)
	return strings.ToLower(strings.Join(parts, " "))
}

func businessIsNodeExporterTarget(labels map[string]string, haystack string) bool {
	job := strings.ToLower(strings.TrimSpace(labels["job"]))
	address := strings.ToLower(strings.TrimSpace(labels["__address__"]))
	if strings.Contains(job, "node") || strings.Contains(job, "consul-node") {
		return true
	}
	if strings.HasSuffix(address, ":9100") {
		return true
	}
	return strings.Contains(haystack, "node_exporter")
}

func businessIsDatabaseTargetCandidate(dbType int, labels map[string]string, haystack string) bool {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return businessLabelsOrHaystackContains(labels, haystack, "mysql")
	case cmdbmodel.DatabaseTypeRedis:
		return businessLabelsOrHaystackContains(labels, haystack, "redis")
	case cmdbmodel.DatabaseTypePostgreSQL:
		return businessLabelsOrHaystackContains(labels, haystack, "postgres", "postgresql", "pg_")
	default:
		return false
	}
}

func businessLabelsOrHaystackContains(labels map[string]string, haystack string, needles ...string) bool {
	for _, key := range []string{"job", "role", "service", "auth_module", "__param_auth_module", "instance", "alias", "hostname"} {
		value := strings.ToLower(strings.TrimSpace(labels[key]))
		for _, needle := range needles {
			if value != "" && strings.Contains(value, needle) {
				return true
			}
		}
	}
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func businessMetricMatchBy(asset cmdbmodel.CmdbSQL, labels map[string]string) string {
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__", "alias", "hostname", "job"} {
		if businessTargetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			return key
		}
	}
	return "auto"
}

func businessDatabaseHostSignalCandidates(asset cmdbmodel.CmdbSQL, metricLabels, targetLabels map[string]string, targetScrapeURL string) []string {
	candidates := make([]string, 0, 6)
	add := func(value string) {
		host := businessHostFromAddress(value)
		if host == "" {
			return
		}
		for _, existing := range candidates {
			if strings.EqualFold(existing, host) {
				return
			}
		}
		candidates = append(candidates, host)
	}

	for _, key := range []string{"__address__", "__meta_consul_service_address", "host", "host_ip"} {
		add(targetLabels[key])
	}
	add(targetScrapeURL)
	for _, key := range []string{"target", "__param_target", "addr", "__address__", "hostname", "instance"} {
		add(metricLabels[key])
	}
	add(asset.Endpoint)
	add(asset.HostIP)
	return candidates
}

func businessDatabaseHostFromLabels(labels map[string]string) string {
	for _, key := range []string{"target", "__param_target", "addr", "instance", "__address__", "hostname"} {
		if host := businessHostFromAddress(labels[key]); host != "" {
			return host
		}
	}
	return ""
}

func businessHostFromAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err == nil {
			if host := strings.TrimSpace(parsed.Hostname()); host != "" {
				return host
			}
			if parsed.Host != "" {
				return businessHostFromAddress(parsed.Host)
			}
			if parsed.Opaque != "" {
				return businessHostFromAddress(parsed.Opaque)
			}
		}
	}
	if idx := strings.LastIndex(raw, "@"); idx >= 0 && idx+1 < len(raw) {
		raw = raw[idx+1:]
	}
	if idx := strings.IndexAny(raw, "/?"); idx >= 0 {
		raw = raw[:idx]
	}
	raw = strings.Trim(raw, "[] ")
	if host, port, ok := strings.Cut(raw, ":"); ok && host != "" && port != "" {
		if _, err := strconv.Atoi(port); err == nil {
			return strings.Trim(host, "[] ")
		}
	}
	return raw
}

func businessTargetAddressMatchesAsset(asset cmdbmodel.CmdbSQL, address string) bool {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return false
	}
	for _, host := range businessAssetHostCandidates(asset) {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if address == host || strings.HasPrefix(address, host+":") || strings.Contains(address, host) {
			return true
		}
	}
	if endpoint := strings.ToLower(strings.TrimSpace(asset.Endpoint)); endpoint != "" {
		return address == endpoint || strings.Contains(address, endpoint)
	}
	return false
}

func businessRedisExporterProxyMatchesAsset(asset cmdbmodel.CmdbSQL, target businessPrometheusTarget) bool {
	if asset.Type != cmdbmodel.DatabaseTypeRedis {
		return false
	}
	for _, value := range []string{target.ScrapeURL, target.GlobalURL} {
		if businessHTTPHostMatchesAsset(asset, value) {
			return true
		}
	}
	return false
}

func businessHTTPHostMatchesAsset(asset cmdbmodel.CmdbSQL, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		host = strings.ToLower(parsed.Host)
	}
	for _, candidate := range businessAssetHostCandidates(asset) {
		candidate = strings.ToLower(strings.TrimSpace(candidate))
		if candidate != "" && host == candidate {
			return true
		}
	}
	return false
}

func businessAssetHostCandidates(asset cmdbmodel.CmdbSQL) []string {
	candidates := []string{asset.HostIP}
	if host, _, ok := strings.Cut(strings.TrimSpace(asset.Endpoint), ":"); ok {
		candidates = append(candidates, host)
	}
	return candidates
}

func businessDatabaseTargetNeedles(asset cmdbmodel.CmdbSQL) []string {
	needles := []string{asset.HostIP, asset.Endpoint, asset.Name, asset.InstanceName}
	ipParts := strings.Split(asset.HostIP, ".")
	if len(ipParts) == 4 {
		needles = append(needles,
			ipParts[2]+"."+ipParts[3],
			ipParts[2]+"_"+ipParts[3],
			"db"+ipParts[2]+"_"+ipParts[3],
			"node"+ipParts[2]+"-"+ipParts[3],
		)
		switch asset.Type {
		case cmdbmodel.DatabaseTypeMySQL:
			needles = append(needles, "mysql"+ipParts[2]+"_"+ipParts[3])
		case cmdbmodel.DatabaseTypeRedis:
			needles = append(needles, "redis"+ipParts[2]+"_"+ipParts[3])
		case cmdbmodel.DatabaseTypePostgreSQL:
			needles = append(needles, "postgres"+ipParts[2]+"_"+ipParts[3], "pg"+ipParts[2]+"_"+ipParts[3])
		}
	}
	return needles
}

func businessMergeTargetLabels(target businessPrometheusTarget) map[string]string {
	labels := make(map[string]string, len(target.DiscoveredLabels)+len(target.Labels))
	for key, value := range target.DiscoveredLabels {
		labels[key] = value
	}
	for key, value := range target.Labels {
		labels[key] = value
	}
	return labels
}

func businessLabelsOverlap(left, right map[string]string, keys ...string) bool {
	for _, key := range keys {
		leftValue := strings.TrimSpace(left[key])
		rightValue := strings.TrimSpace(right[key])
		if leftValue != "" && rightValue != "" && leftValue == rightValue {
			return true
		}
	}
	return false
}

func businessBuildPrometheusSelector(labels map[string]string, dbType int) string {
	keySets := [][]string{{"job", "instance"}, {"instance"}, {"alias"}, {"hostname"}, {"job"}}
	if dbType == cmdbmodel.DatabaseTypeRedis {
		keySets = [][]string{{"job", "instance", "target"}, {"job", "instance", "addr"}, {"job", "instance"}, {"target"}, {"addr"}, {"instance"}, {"alias"}, {"job"}}
	}
	for _, keys := range keySets {
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if value := strings.TrimSpace(labels[key]); value != "" {
				parts = append(parts, fmt.Sprintf(`%s="%s"`, key, businessEscapePrometheusLabelValue(value)))
			}
		}
		if len(parts) == len(keys) {
			return strings.Join(parts, ",")
		}
	}
	return ""
}

func businessEscapePrometheusLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func businessAppendPrometheusSelector(selector string, extra ...string) string {
	parts := make([]string, 0, len(extra)+1)
	if strings.TrimSpace(selector) != "" {
		parts = append(parts, strings.TrimSpace(selector))
	}
	for _, item := range extra {
		if strings.TrimSpace(item) != "" {
			parts = append(parts, strings.TrimSpace(item))
		}
	}
	return strings.Join(parts, ",")
}

func businessPrometheusSumQuery(metric, selector string) string {
	return "sum(" + businessPrometheusMetricQuery(metric, selector) + ")"
}

func businessPrometheusMaxQuery(metric, selector string) string {
	return "max(" + businessPrometheusMetricQuery(metric, selector) + ")"
}

func businessPrometheusSumRateQuery(metric, selector string) string {
	return "sum(rate(" + businessPrometheusMetricQuery(metric, selector) + "[5m]))"
}

func businessMySQLConnectionUsageQuery(selector string) string {
	connected := businessPrometheusSumQuery("mysql_global_status_threads_connected", selector)
	maxConnections := businessPrometheusSumQuery("mysql_global_variables_max_connections", selector)
	return "(" + connected + " / clamp_min(" + maxConnections + ", 1)) * 100"
}

func businessPostgreSQLConnectionUsageQuery(selector string) string {
	current := businessPrometheusSumQuery("pg_stat_database_numbackends", selector)
	maxConnections := businessPrometheusMaxQuery("pg_settings_max_connections", selector)
	return "(" + current + " / clamp_min(" + maxConnections + ", 1)) * 100"
}

func businessNodeInstanceSelector(hostIP string) string {
	hostIP = strings.TrimSpace(hostIP)
	if hostIP == "" {
		return ""
	}
	return fmt.Sprintf(`instance=~"%s(:.*)?"`, businessEscapePrometheusLabelValue(regexp.QuoteMeta(hostIP)))
}

func businessNodeMetricQuery(metric, selector string, extra ...string) string {
	selector = businessAppendPrometheusSelector(selector, extra...)
	return businessPrometheusMetricQuery(metric, selector)
}

func businessNodeCPUUsageQuery(selector string) string {
	return "100 * (1 - avg(rate(" + businessNodeMetricQuery("node_cpu_seconds_total", selector, `mode="idle"`) + "[5m])))"
}

func businessNodeMemoryUsageQuery(selector string) string {
	available := businessPrometheusSumQuery("node_memory_MemAvailable_bytes", selector)
	total := businessPrometheusSumQuery("node_memory_MemTotal_bytes", selector)
	return "(1 - (" + available + " / clamp_min(" + total + ", 1))) * 100"
}

func businessNodeDiskUsageQuery(selector string) string {
	fsFilter := `fstype!~"tmpfs|devtmpfs|overlay|squashfs|proc|sysfs|cgroup2?"`
	avail := businessNodeMetricQuery("node_filesystem_avail_bytes", selector, fsFilter)
	size := businessNodeMetricQuery("node_filesystem_size_bytes", selector, fsFilter)
	return "max((1 - (" + avail + " / clamp_min(" + size + ", 1))) * 100)"
}

func roundBusinessFloat(value float64, places int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if places < 0 {
		return value
	}
	pow := math.Pow10(places)
	return math.Round(value*pow) / pow
}

func replicatedWorkloadCheck(kind string, ready, desired, available int32, evidence map[string]interface{}) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := fmt.Sprintf("%s 副本就绪: %d/%d", kind, ready, desired)
	if desired <= 0 {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 70
		message = fmt.Sprintf("%s 期望副本数为 0", kind)
	} else if ready == 0 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 30
		message = fmt.Sprintf("%s 无就绪副本: %d/%d", kind, ready, desired)
	} else if ready < desired {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = maxInt(60, int(float64(ready)/float64(desired)*100))
		message = fmt.Sprintf("%s 副本未全部就绪: %d/%d", kind, ready, desired)
	}
	return healthCheckResult{
		Name:      "工作负载副本",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: mergeEvidence(evidence, map[string]interface{}{
			"kind":              kind,
			"readyReplicas":     ready,
			"desiredReplicas":   desired,
			"availableReplicas": available,
		}),
		Diagnostic: "查看工作负载详情",
	}
}

func jobWorkloadCheck(job *batchv1.Job, evidence map[string]interface{}) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := "Job 执行完成或运行正常"
	if job.Status.Failed > 0 {
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 35
		message = fmt.Sprintf("Job 存在失败 Pod: %d", job.Status.Failed)
	} else if job.Status.Active > 0 {
		status = cmdbmodel.CmdbHealthStatusHealthy
		message = fmt.Sprintf("Job 正在运行: active=%d", job.Status.Active)
	} else if job.Status.Succeeded == 0 {
		status = cmdbmodel.CmdbHealthStatusWarning
		score = 75
		message = "Job 尚未成功完成"
	}
	return healthCheckResult{
		Name:      "工作负载副本",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: mergeEvidence(evidence, map[string]interface{}{
			"kind":      "Job",
			"active":    job.Status.Active,
			"succeeded": job.Status.Succeeded,
			"failed":    job.Status.Failed,
		}),
		Diagnostic: "查看 Job",
	}
}

func podHealthCheck(pod corev1.Pod, evidence map[string]interface{}) healthCheckResult {
	restarts := podRestartCount(pod)
	ready := isBusinessPodReady(pod)
	status := cmdbmodel.CmdbHealthStatusHealthy
	score := 100
	message := "Pod 运行正常"
	switch {
	case pod.Status.Phase == corev1.PodFailed || podHasBusinessImagePullError(pod):
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 35
		message = "Pod 存在失败或镜像拉取异常"
	case !ready || pod.Status.Phase == corev1.PodPending || restarts >= 5:
		status = cmdbmodel.CmdbHealthStatusWarning
		score = maxInt(60, 90-int(restarts)*3)
		message = "Pod 未就绪、Pending 或近期重启偏多"
	}
	return healthCheckResult{
		Name:      "Pod 状态",
		Status:    status,
		Score:     score,
		Message:   message,
		Dimension: cmdbmodel.CmdbHealthDimensionContainer,
		Evidence: mergeEvidence(evidence, map[string]interface{}{
			"podName":      pod.Name,
			"phase":        string(pod.Status.Phase),
			"ready":        ready,
			"restartCount": restarts,
			"nodeName":     pod.Spec.NodeName,
		}),
		Diagnostic: "查看 Pod 详情",
	}
}

func k8sGetErrorCheck(kind string, err error, evidence map[string]interface{}) healthCheckResult {
	status := cmdbmodel.CmdbHealthStatusWarning
	score := 70
	if apierrors.IsNotFound(err) {
		status = cmdbmodel.CmdbHealthStatusCritical
		score = 35
	}
	return healthCheckResult{
		Name:       "工作负载副本",
		Status:     status,
		Score:      score,
		Message:    fmt.Sprintf("%s 读取失败: %s", kind, err.Error()),
		Dimension:  cmdbmodel.CmdbHealthDimensionContainer,
		Evidence:   mergeEvidence(evidence, map[string]interface{}{"kind": kind, "error": err.Error()}),
		Diagnostic: "查看工作负载详情",
	}
}

func warningCheck(name, dimension, message string, evidence map[string]interface{}) healthCheckResult {
	return healthCheckResult{Name: name, Status: cmdbmodel.CmdbHealthStatusWarning, Score: 75, Message: message, Dimension: dimension, Evidence: evidence, Diagnostic: "查看诊断"}
}

func criticalCheck(name, dimension, message string, evidence map[string]interface{}) healthCheckResult {
	return healthCheckResult{Name: name, Status: cmdbmodel.CmdbHealthStatusCritical, Score: 35, Message: message, Dimension: dimension, Evidence: evidence, Diagnostic: "查看诊断"}
}

func unknownCheck(name, dimension, message string, evidence map[string]interface{}) healthCheckResult {
	return healthCheckResult{Name: name, Status: cmdbmodel.CmdbHealthStatusUnknown, Score: 0, Message: message, Dimension: dimension, Evidence: evidence, Diagnostic: "配置数据源"}
}

func resourceIdentityEvidence(resource *cmdbmodel.CmdbBusinessModelBinding) map[string]interface{} {
	return map[string]interface{}{
		"resourceId":   resource.ID,
		"resourceName": resourceDisplayName(resource),
		"resourceType": resource.ResourceType,
		"sourceType":   resource.SourceType,
		"sourceId":     resource.SourceID,
		"sourceKey":    resource.SourceKey,
	}
}

func resourceSupportsDimension(resourceType, dimension string) bool {
	switch dimension {
	case cmdbmodel.CmdbHealthDimensionOverall:
		return true
	case cmdbmodel.CmdbHealthDimensionBusiness:
		return resourceType == cmdbmodel.CmdbResourceTypeBusinessSystem ||
			resourceType == cmdbmodel.CmdbResourceTypeBusinessService ||
			resourceType == cmdbmodel.CmdbResourceTypeApplication ||
			resourceType == cmdbmodel.CmdbResourceTypeNginx ||
			resourceType == cmdbmodel.CmdbResourceTypeGateway
	case cmdbmodel.CmdbHealthDimensionContainer:
		return resourceType == cmdbmodel.CmdbResourceTypeK8sCluster ||
			resourceType == cmdbmodel.CmdbResourceTypeK8sWorkload ||
			resourceType == "k8s_namespace"
	case cmdbmodel.CmdbHealthDimensionInfra:
		return resourceType == cmdbmodel.CmdbResourceTypeHost ||
			resourceType == cmdbmodel.CmdbResourceTypeVM ||
			resourceType == cmdbmodel.CmdbResourceTypeCloudHost ||
			resourceType == cmdbmodel.CmdbResourceTypePhysicalServer ||
			resourceType == cmdbmodel.CmdbResourceTypeDatabase
	case cmdbmodel.CmdbHealthDimensionMiddleware:
		return resourceType == cmdbmodel.CmdbResourceTypeDatabase ||
			resourceType == cmdbmodel.CmdbResourceTypeMiddleware
	case cmdbmodel.CmdbHealthDimensionLog:
		return resourceType == cmdbmodel.CmdbResourceTypeK8sWorkload ||
			resourceType == cmdbmodel.CmdbResourceTypeBusinessService ||
			resourceType == cmdbmodel.CmdbResourceTypeApplication ||
			resourceType == cmdbmodel.CmdbResourceTypeNginx ||
			resourceType == cmdbmodel.CmdbResourceTypeGateway ||
			resourceType == cmdbmodel.CmdbResourceTypeDatabase ||
			resourceType == cmdbmodel.CmdbResourceTypeMiddleware ||
			resourceType == cmdbmodel.CmdbResourceTypeLogProject ||
			resourceType == cmdbmodel.CmdbResourceTypeLogStore ||
			resourceType == cmdbmodel.CmdbResourceTypeHost ||
			resourceType == cmdbmodel.CmdbResourceTypeVM ||
			resourceType == cmdbmodel.CmdbResourceTypeCloudHost ||
			resourceType == cmdbmodel.CmdbResourceTypePhysicalServer
	default:
		return false
	}
}

func healthDimensionForResourceType(resourceType string) string {
	switch resourceType {
	case cmdbmodel.CmdbResourceTypeBusinessSystem, cmdbmodel.CmdbResourceTypeBusinessService, cmdbmodel.CmdbResourceTypeApplication,
		cmdbmodel.CmdbResourceTypeNginx, cmdbmodel.CmdbResourceTypeGateway:
		return cmdbmodel.CmdbHealthDimensionBusiness
	case cmdbmodel.CmdbResourceTypeK8sCluster, cmdbmodel.CmdbResourceTypeK8sWorkload, "k8s_namespace":
		return cmdbmodel.CmdbHealthDimensionContainer
	case cmdbmodel.CmdbResourceTypeHost, cmdbmodel.CmdbResourceTypeVM, cmdbmodel.CmdbResourceTypeCloudHost, cmdbmodel.CmdbResourceTypePhysicalServer:
		return cmdbmodel.CmdbHealthDimensionInfra
	case cmdbmodel.CmdbResourceTypeDatabase, cmdbmodel.CmdbResourceTypeMiddleware:
		return cmdbmodel.CmdbHealthDimensionMiddleware
	case cmdbmodel.CmdbResourceTypeLogProject, cmdbmodel.CmdbResourceTypeLogStore:
		return cmdbmodel.CmdbHealthDimensionLog
	default:
		return cmdbmodel.CmdbHealthDimensionOverall
	}
}

func diagnosisEntries(resource *cmdbmodel.CmdbBusinessModelBinding) []map[string]interface{} {
	switch resource.ResourceType {
	case cmdbmodel.CmdbResourceTypeNginx, cmdbmodel.CmdbResourceTypeGateway:
		return []map[string]interface{}{
			{"label": "查看入口日志", "target": "log_query", "sourceId": resource.SourceID, "sourceType": resource.SourceType},
			{"label": "查看日志告警", "target": "log_alert_events", "sourceId": resource.SourceID, "sourceType": resource.SourceType},
			{"label": "查看入口链路", "target": "cmdb_resource", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypeK8sCluster:
		return []map[string]interface{}{
			{"label": "查看 K8S 集群监控", "target": "k8s_cluster", "sourceId": resource.SourceID},
			{"label": "查看集群事件", "target": "k8s_events", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypeK8sWorkload:
		return []map[string]interface{}{
			{"label": "查看工作负载", "target": "k8s_workload", "sourceId": resource.SourceID},
			{"label": "查看 Pod / 重启", "target": "k8s_pods", "sourceKey": resource.SourceKey},
			{"label": "查看日志分析", "target": "log_analysis", "sourceKey": resource.SourceKey},
		}
	case cmdbmodel.CmdbResourceTypeDatabase:
		return []map[string]interface{}{
			{"label": "查看数据库监控", "target": "database_monitor", "sourceId": resource.SourceID},
			{"label": "慢 SQL 分析", "target": "slow_sql", "sourceId": resource.SourceID},
			{"label": "连接数/锁等待", "target": "database_locks", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypeHost, cmdbmodel.CmdbResourceTypeVM, cmdbmodel.CmdbResourceTypeCloudHost:
		return []map[string]interface{}{
			{"label": "查看主机监控", "target": "host_monitor", "sourceId": resource.SourceID},
			{"label": "查看进程", "target": "host_process", "sourceId": resource.SourceID},
			{"label": "查看磁盘/IO", "target": "host_disk_io", "sourceId": resource.SourceID},
			{"label": "查看日志分析", "target": "log_analysis", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypePhysicalServer:
		return []map[string]interface{}{
			{"label": "查看硬件监控", "target": "hardware_monitor", "sourceId": resource.SourceID},
			{"label": "查看主机监控", "target": "host_monitor", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypeLogProject:
		return []map[string]interface{}{
			{"label": "查看日志项目", "target": "log_project", "sourceId": resource.SourceID},
			{"label": "查看日志告警", "target": "log_alert_events", "sourceId": resource.SourceID},
		}
	case cmdbmodel.CmdbResourceTypeLogStore:
		return []map[string]interface{}{
			{"label": "查看日志库", "target": "log_store", "sourceId": resource.SourceID},
			{"label": "查询日志", "target": "log_query", "sourceId": resource.SourceID},
			{"label": "查看日志告警", "target": "log_alert_events", "sourceId": resource.SourceID},
		}
	default:
		return []map[string]interface{}{{"label": "查看资源详情", "target": "cmdb_resource", "sourceId": resource.SourceID}}
	}
}

func firstNodeHealthByDimension(items []nodeHealth, dimension string) nodeHealth {
	for _, item := range items {
		if item.Dimension == dimension {
			return item
		}
	}
	return nodeHealth{}
}

func isWorseCheck(candidate, current healthCheckResult) bool {
	if healthSeverity(candidate.Status) != healthSeverity(current.Status) {
		return healthSeverity(candidate.Status) > healthSeverity(current.Status)
	}
	return candidate.Score < current.Score
}

func scoreByStatus(status string) int {
	switch normalizeHealthStatusName(status) {
	case cmdbmodel.CmdbHealthStatusHealthy:
		return 100
	case cmdbmodel.CmdbHealthStatusWarning:
		return 75
	case cmdbmodel.CmdbHealthStatusCritical:
		return 35
	default:
		return 0
	}
}

func dbHealthScore(status string, abnormalCount, warningCount int) int {
	score := scoreByStatus(status)
	score -= abnormalCount * 8
	score -= warningCount * 3
	return clampScore(score)
}

func uintFromResource(resource *cmdbmodel.CmdbBusinessModelBinding, keys ...string) uint {
	props := jsonToMap(resource.Properties)
	for _, key := range keys {
		if value, ok := props[key]; ok {
			if parsed := uintFromAny(value); parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func stringFromResource(resource *cmdbmodel.CmdbBusinessModelBinding, keys ...string) string {
	props := jsonToMap(resource.Properties)
	for _, key := range keys {
		if value, ok := props[key]; ok {
			if parsed := stringFromAny(value); parsed != "" {
				return parsed
			}
		}
	}
	return ""
}

func uintFromAny(value interface{}) uint {
	switch typed := value.(type) {
	case uint:
		return typed
	case int:
		if typed > 0 {
			return uint(typed)
		}
	case int64:
		if typed > 0 {
			return uint(typed)
		}
	case float64:
		if typed > 0 {
			return uint(typed)
		}
	case jsonNumber:
		if parsed, err := strconv.ParseUint(string(typed), 10, 64); err == nil {
			return uint(parsed)
		}
	case string:
		if parsed, err := strconv.ParseUint(strings.TrimSpace(typed), 10, 64); err == nil {
			return uint(parsed)
		}
	}
	return 0
}

type jsonNumber string

func stringFromAny(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func boolFromAny(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y", "enabled", "on", "已接入":
			return true
		}
	case int:
		return typed != 0
	case float64:
		return typed != 0
	}
	return false
}

func firstPresent(values map[string]interface{}, keys ...string) interface{} {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func parseClusterIDFromSourceKey(sourceKey string) uint {
	parts := strings.Split(sourceKey, ":")
	for i, part := range parts {
		if strings.EqualFold(part, "cluster") && i+1 < len(parts) {
			if parsed, err := strconv.ParseUint(parts[i+1], 10, 64); err == nil {
				return uint(parsed)
			}
		}
	}
	return 0
}

func lastSourceKeyPart(sourceKey string) string {
	parts := strings.Split(sourceKey, ":")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func normalizeK8sKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "deployment", "deployments":
		return "Deployment"
	case "statefulset", "statefulsets":
		return "StatefulSet"
	case "daemonset", "daemonsets":
		return "DaemonSet"
	case "job", "jobs":
		return "Job"
	case "cronjob", "cronjobs":
		return "CronJob"
	case "pod", "pods":
		return "Pod"
	default:
		return kind
	}
}

func eventLastTime(event corev1.Event) time.Time {
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.FirstTimestamp.IsZero() {
		return event.FirstTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func isBusinessPodReady(pod corev1.Pod) bool {
	if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podRestartCount(pod corev1.Pod) int32 {
	var restarts int32
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}
	for _, status := range pod.Status.InitContainerStatuses {
		restarts += status.RestartCount
	}
	return restarts
}

func podHasBusinessImagePullError(pod corev1.Pod) bool {
	for _, status := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
		waiting := status.State.Waiting
		if waiting == nil {
			continue
		}
		reason := strings.ToLower(waiting.Reason)
		if strings.Contains(reason, "imagepull") || strings.Contains(reason, "errimagepull") || strings.Contains(reason, "backoff") {
			return true
		}
	}
	return false
}

func hardwareAlertEvidence(alerts []cmdbmodel.CmdbIdcAssetHardwareAlertVo) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(alerts))
	for _, alert := range alerts {
		result = append(result, map[string]interface{}{
			"severity":       alert.Severity,
			"category":       alert.Category,
			"metric":         alert.Metric,
			"component":      alert.Component,
			"message":        alert.Message,
			"recommendation": alert.Recommendation,
		})
		if len(result) >= 5 {
			break
		}
	}
	return result
}

func limitMapList(items []map[string]interface{}, limit int) []map[string]interface{} {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
