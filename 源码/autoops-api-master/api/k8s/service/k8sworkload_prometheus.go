package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	alertmodel "dodevops-api/api/alertmanager/model"
	"dodevops-api/api/k8s/model"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultPodPrometheusRange = time.Hour
	defaultPodPrometheusStep  = time.Minute
)

func (s *K8sWorkloadServiceImpl) GetPodPrometheusMetrics(c *gin.Context, clusterId uint, namespaceName string, podName string, query model.PodPrometheusMetricsQuery) {
	cluster, err := s.clusterDao.GetByID(clusterId)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, "获取集群信息失败: "+err.Error())
		return
	}
	if cluster.MonitorDatasourceID == 0 {
		result.Failed(c, http.StatusBadRequest, "当前集群未配置监控数据源")
		return
	}

	clientset, err := s.createK8sClient(cluster.Credential)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}
	pod, err := clientset.CoreV1().Pods(namespaceName).Get(c.Request.Context(), podName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Pod不存在: "+err.Error())
		return
	}
	scope := normalizePodPrometheusScope(query.Scope)
	podNames := []string{podName}
	podNamePattern := ""
	if scope == "workload" {
		podNames = s.resolvePrometheusSiblingPodNames(c.Request.Context(), clientset, namespaceName, pod)
		podNamePattern = s.resolvePrometheusWorkloadPodNamePattern(c.Request.Context(), clientset, namespaceName, pod)
	}

	ds, err := s.resolvePodMonitorDatasource(cluster.MonitorDatasourceID)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	promAPI, err := newK8SPromAPI(*ds)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "创建监控查询客户端失败: "+err.Error())
		return
	}

	start, end, rangeText, stepText, stepDuration := normalizePodPrometheusRange(query)
	queries := buildPodPrometheusQueries(namespaceName, podNames, query)
	if podNamePattern != "" {
		queries = buildPodPrometheusQueriesForPodPattern(namespaceName, podNamePattern, query)
	}
	defs := []podPrometheusChartDef{
		{Key: "cpuUsageRate", Title: "Pod容器CPU使用率", Unit: "%", Precision: 2, Query: queries["cpuUsageRate"]},
		{Key: "memoryUsageRate", Title: "Pod容器内存WSS使用率", Unit: "%", Precision: 2, Query: queries["memoryUsageRate"]},
		{Key: "restartCount", Title: "Pod容器重启次数", Unit: "次", Precision: 0, Query: queries["restartCount"]},
		{Key: "network", Title: "Pod每秒网络带宽", Unit: "MiB/s", Precision: 3, Query: queries["network"]},
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()

	charts := make([]model.PodMetricChart, len(defs))
	warningsByChart := make([]string, len(defs))
	var wg sync.WaitGroup
	for i, def := range defs {
		wg.Add(1)
		go func(index int, chartDef podPrometheusChartDef) {
			defer wg.Done()
			chart, warning := queryPodPrometheusChart(ctx, promAPI, chartDef, start, end, stepDuration)
			charts[index] = chart
			warningsByChart[index] = warning
		}(i, def)
	}
	wg.Wait()
	podNames = mergePrometheusChartPodNames(podNames, charts)

	warnings := make([]string, 0)
	for _, warning := range warningsByChart {
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}

	result.Success(c, model.PodPrometheusMetricsResponse{
		ClusterID:      cluster.ID,
		Namespace:      namespaceName,
		PodName:        podName,
		Scope:          scope,
		PodNames:       podNames,
		DatasourceID:   ds.ID,
		DatasourceType: ds.DatasourceType,
		DatasourceName: ds.Name,
		Range:          rangeText,
		TimeRange:      fmt.Sprintf("%s - %s", start.Format(time.RFC3339), end.Format(time.RFC3339)),
		Step:           stepText,
		Charts:         charts,
		Warnings:       warnings,
		Queries:        queries,
	})
}

type podPrometheusChartDef struct {
	Key       string
	Title     string
	Unit      string
	Precision int
	Query     string
}

func (s *K8sWorkloadServiceImpl) resolvePodMonitorDatasource(id uint) (*alertmodel.AlertDatasource, error) {
	var ds alertmodel.AlertDatasource
	if err := s.clusterDao.DB.First(&ds, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("监控数据源不存在")
		}
		return nil, fmt.Errorf("查询监控数据源失败: %w", err)
	}
	if ds.Enabled != alertmodel.RuleStatusEnabled {
		return nil, fmt.Errorf("监控数据源已禁用")
	}
	if !isK8SPromQLDatasource(ds.DatasourceType) {
		return nil, fmt.Errorf("监控数据源仅支持 Prometheus/VictoriaMetrics")
	}
	if strings.TrimSpace(ds.URL) == "" {
		return nil, fmt.Errorf("监控数据源 URL 为空")
	}
	return &ds, nil
}

func (s *K8sWorkloadServiceImpl) attachPodPrometheusCurrentUsage(ctx context.Context, cluster *model.KubeCluster, namespaceName string, pods []model.K8sPodInfo) []model.K8sPodInfo {
	if cluster == nil || cluster.MonitorDatasourceID == 0 || len(pods) == 0 {
		return pods
	}

	missingPodNames := make([]string, 0, len(pods))
	for _, pod := range pods {
		if isEmptyPodCurrentUsage(pod.CurrentUsage) {
			missingPodNames = append(missingPodNames, pod.Name)
		}
	}
	if len(missingPodNames) == 0 {
		return pods
	}

	ds, err := s.resolvePodMonitorDatasource(cluster.MonitorDatasourceID)
	if err != nil {
		return pods
	}
	promAPI, err := newK8SPromAPI(*ds)
	if err != nil {
		return pods
	}

	queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	selector := buildPodPrometheusResourceSelector(namespaceName, missingPodNames, model.PodPrometheusMetricsQuery{})
	cpuQuery := fmt.Sprintf(`sum by (pod) (irate(container_cpu_usage_seconds_total%s[2m]))`, selector)
	memoryQuery := fmt.Sprintf(`sum by (pod) (container_memory_working_set_bytes%s)`, selector)
	cpuByPod := queryInstantPodValues(queryCtx, promAPI, cpuQuery)
	memoryByPod := queryInstantPodValues(queryCtx, promAPI, memoryQuery)

	for i := range pods {
		if !isEmptyPodCurrentUsage(pods[i].CurrentUsage) {
			continue
		}
		cpuCores, hasCPU := cpuByPod[pods[i].Name]
		memoryBytes, hasMemory := memoryByPod[pods[i].Name]
		if !hasCPU && !hasMemory {
			continue
		}
		if hasCPU {
			cpuQuantity := resource.NewMilliQuantity(int64(math.Round(cpuCores*1000)), resource.DecimalSI)
			pods[i].CurrentUsage.CPU = formatCPUQuantity(*cpuQuantity)
		}
		if hasMemory {
			memoryQuantity := resource.NewQuantity(int64(math.Round(memoryBytes)), resource.BinarySI)
			pods[i].CurrentUsage.Memory = formatMemoryQuantity(*memoryQuantity)
		}
	}

	return pods
}

func isEmptyPodCurrentUsage(usage model.ResourceUsage) bool {
	cpu := strings.TrimSpace(usage.CPU)
	memory := strings.TrimSpace(usage.Memory)
	return (cpu == "" || cpu == "0" || cpu == "0m") && (memory == "" || memory == "0" || memory == "0Mi")
}

func queryInstantPodValues(ctx context.Context, promAPI v1.API, query string) map[string]float64 {
	values := make(map[string]float64)
	value, _, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return values
	}
	vector, ok := value.(prommodel.Vector)
	if !ok {
		return values
	}
	for _, sample := range vector {
		if sample == nil {
			continue
		}
		podName := string(sample.Metric["pod"])
		if podName == "" {
			continue
		}
		rawValue := float64(sample.Value)
		if math.IsNaN(rawValue) || math.IsInf(rawValue, 0) {
			continue
		}
		values[podName] = rawValue
	}
	return values
}

func normalizePodPrometheusScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pod", "single":
		return "pod"
	default:
		return "workload"
	}
}

func (s *K8sWorkloadServiceImpl) resolvePrometheusSiblingPodNames(ctx context.Context, clientset kubernetes.Interface, namespaceName string, pod *corev1.Pod) []string {
	if pod == nil {
		return nil
	}

	selector := s.resolvePodOwnerSelector(ctx, clientset, namespaceName, pod)
	if strings.TrimSpace(selector) == "" {
		return []string{pod.Name}
	}

	list, err := clientset.CoreV1().Pods(namespaceName).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(list.Items) == 0 {
		return []string{pod.Name}
	}

	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		if list.Items[i].Name != "" {
			names = append(names, list.Items[i].Name)
		}
	}
	sort.Strings(names)
	return names
}

func (s *K8sWorkloadServiceImpl) resolvePrometheusWorkloadPodNamePattern(ctx context.Context, clientset kubernetes.Interface, namespaceName string, pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	owner := controllerOwnerRef(pod.OwnerReferences)
	if owner == nil {
		return ""
	}

	switch owner.Kind {
	case "ReplicaSet":
		rs, err := clientset.AppsV1().ReplicaSets(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		if deploymentOwner := controllerOwnerRef(rs.OwnerReferences); deploymentOwner != nil && deploymentOwner.Kind == "Deployment" {
			return prometheusWorkloadPodNamePattern("Deployment", deploymentOwner.Name)
		}
		return prometheusWorkloadPodNamePattern("ReplicaSet", owner.Name)
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		return prometheusWorkloadPodNamePattern(owner.Kind, owner.Name)
	default:
		return ""
	}
}

func prometheusWorkloadPodNamePattern(kind, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	escapedName := regexp.QuoteMeta(name)
	switch kind {
	case "Deployment":
		return escapedName + `-[a-z0-9]+-[a-z0-9]+`
	case "StatefulSet":
		return escapedName + `-[0-9]+`
	case "ReplicaSet", "DaemonSet", "Job":
		return escapedName + `-[a-z0-9]+`
	default:
		return ""
	}
}

func (s *K8sWorkloadServiceImpl) resolvePodOwnerSelector(ctx context.Context, clientset kubernetes.Interface, namespaceName string, pod *corev1.Pod) string {
	owner := controllerOwnerRef(pod.OwnerReferences)
	if owner == nil {
		return ""
	}

	switch owner.Kind {
	case "ReplicaSet":
		rs, err := clientset.AppsV1().ReplicaSets(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		if deploymentOwner := controllerOwnerRef(rs.OwnerReferences); deploymentOwner != nil && deploymentOwner.Kind == "Deployment" {
			deployment, err := clientset.AppsV1().Deployments(namespaceName).Get(ctx, deploymentOwner.Name, metav1.GetOptions{})
			if err == nil {
				return selectorString(deployment.Spec.Selector)
			}
		}
		return selectorString(rs.Spec.Selector)
	case "Deployment":
		deployment, err := clientset.AppsV1().Deployments(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return selectorString(deployment.Spec.Selector)
	case "StatefulSet":
		statefulSet, err := clientset.AppsV1().StatefulSets(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return selectorString(statefulSet.Spec.Selector)
	case "DaemonSet":
		daemonSet, err := clientset.AppsV1().DaemonSets(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return selectorString(daemonSet.Spec.Selector)
	case "Job":
		job, err := clientset.BatchV1().Jobs(namespaceName).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return selectorString(job.Spec.Selector)
	default:
		return ""
	}
}

func controllerOwnerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	if len(refs) > 0 {
		return &refs[0]
	}
	return nil
}

func selectorString(selector *metav1.LabelSelector) string {
	if selector == nil {
		return ""
	}
	parsed, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return ""
	}
	return parsed.String()
}

func normalizePodPrometheusRange(query model.PodPrometheusMetricsQuery) (time.Time, time.Time, string, string, time.Duration) {
	rangeText, rangeDuration := podPrometheusRangeDuration(query.Range)
	end := time.Now()
	if query.End > 0 {
		end = time.UnixMilli(query.End)
	}
	start := end.Add(-rangeDuration)
	if query.Start > 0 {
		start = time.UnixMilli(query.Start)
	}
	if !start.Before(end) {
		start = end.Add(-rangeDuration)
	}

	if query.Start > 0 || query.End > 0 {
		rangeDuration = end.Sub(start)
		rangeText = formatPodPrometheusDuration(rangeDuration)
	}

	stepText, step := podPrometheusStepDuration(query.Step, rangeDuration)
	return start, end, rangeText, stepText, step
}

func podPrometheusRangeDuration(raw string) (string, time.Duration) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1h", "1hour":
		return "1h", time.Hour
	case "3h":
		return "3h", 3 * time.Hour
	case "6h":
		return "6h", 6 * time.Hour
	case "24h", "1d":
		return "24h", 24 * time.Hour
	case "3d":
		return "3d", 3 * 24 * time.Hour
	case "7d":
		return "7d", 7 * 24 * time.Hour
	case "50d":
		return "50d", 50 * 24 * time.Hour
	default:
		if parsed, ok := parsePodPrometheusDuration(raw); ok {
			return formatPodPrometheusDuration(parsed), parsed
		}
		return "1h", defaultPodPrometheusRange
	}
}

func podPrometheusStepDuration(raw string, rangeDuration time.Duration) (string, time.Duration) {
	stepText := strings.TrimSpace(raw)
	if stepText != "" {
		if parsed, ok := parsePodPrometheusDuration(stepText); ok && parsed > 0 {
			return formatPodPrometheusDuration(parsed), parsed
		}
	}

	step := defaultPodPrometheusStep
	switch {
	case rangeDuration <= time.Hour:
		step = time.Minute
	case rangeDuration <= 3*time.Hour:
		step = time.Minute
	case rangeDuration <= 6*time.Hour:
		step = 2 * time.Minute
	case rangeDuration <= 24*time.Hour:
		step = 5 * time.Minute
	case rangeDuration <= 3*24*time.Hour:
		step = 15 * time.Minute
	case rangeDuration <= 7*24*time.Hour:
		step = 30 * time.Minute
	default:
		step = 2 * time.Hour
	}
	return formatPodPrometheusDuration(step), step
}

func parsePodPrometheusDuration(raw string) (time.Duration, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return 0, false
	}
	if strings.HasSuffix(value, "d") {
		daysText := strings.TrimSuffix(value, "d")
		days, err := strconv.Atoi(daysText)
		if err != nil || days <= 0 {
			return 0, false
		}
		return time.Duration(days) * 24 * time.Hour, true
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

func formatPodPrometheusDuration(duration time.Duration) string {
	if duration <= 0 {
		return defaultPodPrometheusRange.String()
	}
	if duration%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(duration/(24*time.Hour)))
	}
	return duration.String()
}

func buildPodPrometheusResourceSelector(namespaceName string, podNames []string, query model.PodPrometheusMetricsQuery) string {
	return buildPodPrometheusResourceSelectorWithPodMatcher(namespaceName, regexValuesMatcher("pod", podNames), query)
}

func buildPodPrometheusResourceSelectorWithPodMatcher(namespaceName, podMatcher string, query model.PodPrometheusMetricsQuery) string {
	matchers := []string{
		regexMatcher("namespace", namespaceName),
		podMatcher,
		`container!=""`,
		`container!="POD"`,
	}
	if value := strings.TrimSpace(query.Container); value != "" {
		matchers = append(matchers, regexMatcher("container", value))
	}
	if value := strings.TrimSpace(query.Node); value != "" {
		matchers = append(matchers, regexMatcher("node", value))
	}
	if value := strings.TrimSpace(query.OriginPrometheus); value != "" {
		matchers = append(matchers, regexMatcher("origin_prometheus", value))
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func buildPodPrometheusNetworkSelector(namespaceName string, podNames []string, query model.PodPrometheusMetricsQuery) string {
	return buildPodPrometheusNetworkSelectorWithPodMatcher(namespaceName, regexValuesMatcher("pod", podNames), query)
}

func buildPodPrometheusNetworkSelectorWithPodMatcher(namespaceName, podMatcher string, query model.PodPrometheusMetricsQuery) string {
	matchers := []string{
		regexMatcher("namespace", namespaceName),
		podMatcher,
		`interface!="lo"`,
	}
	if value := strings.TrimSpace(query.Node); value != "" {
		matchers = append(matchers, regexMatcher("node", value))
	}
	if value := strings.TrimSpace(query.OriginPrometheus); value != "" {
		matchers = append(matchers, regexMatcher("origin_prometheus", value))
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func buildPodPrometheusRestartSelector(namespaceName string, podNames []string, query model.PodPrometheusMetricsQuery) string {
	return buildPodPrometheusRestartSelectorWithPodMatcher(namespaceName, regexValuesMatcher("pod", podNames), query)
}

func buildPodPrometheusRestartSelectorWithPodMatcher(namespaceName, podMatcher string, query model.PodPrometheusMetricsQuery) string {
	matchers := []string{
		regexMatcher("namespace", namespaceName),
		podMatcher,
		`container!=""`,
	}
	if value := strings.TrimSpace(query.Container); value != "" {
		matchers = append(matchers, regexMatcher("container", value))
	}
	if value := strings.TrimSpace(query.OriginPrometheus); value != "" {
		matchers = append(matchers, regexMatcher("origin_prometheus", value))
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func buildPodPrometheusLimitSelector(namespaceName string, podNames []string, query model.PodPrometheusMetricsQuery, resourceName, unit string) string {
	return buildPodPrometheusLimitSelectorWithPodMatcher(namespaceName, regexValuesMatcher("pod", podNames), query, resourceName, unit)
}

func buildPodPrometheusLimitSelectorWithPodMatcher(namespaceName, podMatcher string, query model.PodPrometheusMetricsQuery, resourceName, unit string) string {
	matchers := []string{
		regexMatcher("namespace", namespaceName),
		podMatcher,
		`container!=""`,
		regexMatcher("resource", resourceName),
		regexMatcher("unit", unit),
	}
	if value := strings.TrimSpace(query.Container); value != "" {
		matchers = append(matchers, regexMatcher("container", value))
	}
	if value := strings.TrimSpace(query.Node); value != "" {
		matchers = append(matchers, regexMatcher("node", value))
	}
	if value := strings.TrimSpace(query.OriginPrometheus); value != "" {
		matchers = append(matchers, regexMatcher("origin_prometheus", value))
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func regexMatcher(label, value string) string {
	pattern := escapePromQLRegexPattern(regexp.QuoteMeta(strings.TrimSpace(value)))
	return fmt.Sprintf(`%s=~"^%s$"`, label, pattern)
}

func regexValuesMatcher(label string, values []string) string {
	parts := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, regexp.QuoteMeta(value))
	}
	if len(parts) == 0 {
		return fmt.Sprintf(`%s=~"^$"`, label)
	}
	return regexPatternMatcher(label, strings.Join(parts, "|"))
}

func regexPatternMatcher(label, pattern string) string {
	pattern = escapePromQLRegexPattern(pattern)
	return fmt.Sprintf(`%s=~"^(%s)$"`, label, pattern)
}

func escapePromQLRegexPattern(pattern string) string {
	pattern = strings.ReplaceAll(pattern, `\`, `\\`)
	pattern = strings.ReplaceAll(pattern, `"`, `\"`)
	return pattern
}

func buildPodPrometheusQueries(namespaceName string, podNames []string, query model.PodPrometheusMetricsQuery) map[string]string {
	podMatcher := regexValuesMatcher("pod", podNames)
	return buildPodPrometheusQueriesWithPodMatcher(namespaceName, podMatcher, query)
}

func buildPodPrometheusQueriesForPodPattern(namespaceName, podNamePattern string, query model.PodPrometheusMetricsQuery) map[string]string {
	return buildPodPrometheusQueriesWithPodMatcher(namespaceName, regexPatternMatcher("pod", podNamePattern), query)
}

func buildPodPrometheusQueriesWithPodMatcher(namespaceName, podMatcher string, query model.PodPrometheusMetricsQuery) map[string]string {
	resourceSelector := buildPodPrometheusResourceSelectorWithPodMatcher(namespaceName, podMatcher, query)
	networkSelector := buildPodPrometheusNetworkSelectorWithPodMatcher(namespaceName, podMatcher, query)
	restartSelector := buildPodPrometheusRestartSelectorWithPodMatcher(namespaceName, podMatcher, query)
	cpuLimitSelector := buildPodPrometheusLimitSelectorWithPodMatcher(namespaceName, podMatcher, query, "cpu", "core")
	memoryLimitSelector := buildPodPrometheusLimitSelectorWithPodMatcher(namespaceName, podMatcher, query, "memory", "byte")
	cpuCores := fmt.Sprintf(`sum by (pod) (irate(container_cpu_usage_seconds_total%s[2m]))`, resourceSelector)
	cpuLimit := fmt.Sprintf(`(sum by (pod) ((container_spec_cpu_quota%s > 0) / 100000) or sum by (pod) (kube_pod_container_resource_limits%s > 0))`, resourceSelector, cpuLimitSelector)
	memoryLimit := fmt.Sprintf(`(sum by (pod) (container_spec_memory_limit_bytes%s > 0) or sum by (pod) (kube_pod_container_resource_limits%s > 0))`, resourceSelector, memoryLimitSelector)
	networkReceive := fmt.Sprintf(`label_replace(sum by (pod) (irate(container_network_receive_bytes_total%s[2m])) / 1024 / 1024, "direction", "in", "pod", ".*")`, networkSelector)
	networkTransmit := fmt.Sprintf(`label_replace(sum by (pod) (irate(container_network_transmit_bytes_total%s[2m])) / 1024 / 1024, "direction", "out", "pod", ".*")`, networkSelector)
	return map[string]string{
		"cpuUsageRate":    fmt.Sprintf(`%s / %s * 100`, cpuCores, cpuLimit),
		"memoryUsageRate": fmt.Sprintf(`sum by (pod) (container_memory_working_set_bytes%s) / %s * 100`, resourceSelector, memoryLimit),
		"restartCount":    fmt.Sprintf(`sum by (pod) (kube_pod_container_status_restarts_total%s)`, restartSelector),
		"network":         fmt.Sprintf(`%s or %s`, networkReceive, networkTransmit),
	}
}

func mergePrometheusChartPodNames(current []string, charts []model.PodMetricChart) []string {
	seen := make(map[string]struct{})
	for _, name := range current {
		name = strings.TrimSpace(name)
		if name != "" {
			seen[name] = struct{}{}
		}
	}
	for _, chart := range charts {
		for _, series := range chart.Series {
			name := strings.TrimSpace(series.Labels["pod"])
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func queryPodPrometheusChart(ctx context.Context, promAPI v1.API, def podPrometheusChartDef, start, end time.Time, step time.Duration) (model.PodMetricChart, string) {
	chart := model.PodMetricChart{
		Key:       def.Key,
		Title:     def.Title,
		Unit:      def.Unit,
		Precision: def.Precision,
		Series:    []model.PodMetricSeries{},
	}

	value, warnings, err := promAPI.QueryRange(ctx, def.Query, v1.Range{Start: start, End: end, Step: step})
	if err != nil {
		return chart, fmt.Sprintf("%s 查询失败: %v", def.Title, err)
	}
	if len(warnings) > 0 {
		return chartFromPrometheusValue(chart, value), fmt.Sprintf("%s 查询告警: %s", def.Title, strings.Join(warnings, "; "))
	}
	return chartFromPrometheusValue(chart, value), ""
}

func chartFromPrometheusValue(chart model.PodMetricChart, value prommodel.Value) model.PodMetricChart {
	matrix, ok := value.(prommodel.Matrix)
	if !ok {
		return chart
	}
	for _, stream := range matrix {
		if stream == nil {
			continue
		}
		labels := prometheusMetricLabels(stream.Metric)
		name := labels["pod"]
		if name == "" {
			name = stream.Metric.String()
		}
		if direction := labels["direction"]; direction != "" {
			switch direction {
			case "in":
				name = "流入:" + name
			case "out":
				name = "流出:" + name
			default:
				name = direction + ":" + name
			}
		}
		points := make([]model.MetricsPoint, 0, len(stream.Values))
		for _, sample := range stream.Values {
			rawValue := float64(sample.Value)
			if math.IsNaN(rawValue) || math.IsInf(rawValue, 0) {
				continue
			}
			points = append(points, model.MetricsPoint{
				Timestamp: sample.Timestamp.Time().Format(time.RFC3339),
				Value:     roundMetric(rawValue, chart.Precision),
			})
		}
		chart.Series = append(chart.Series, model.PodMetricSeries{
			Name:   name,
			Labels: labels,
			Points: points,
		})
	}
	return chart
}

func prometheusMetricLabels(metric prommodel.Metric) map[string]string {
	labels := make(map[string]string, len(metric))
	for key, value := range metric {
		labels[string(key)] = string(value)
	}
	return labels
}

func newK8SPromAPI(ds alertmodel.AlertDatasource) (v1.API, error) {
	roundTripper := http.DefaultTransport
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "", "none":
	case "basic":
		roundTripper = &k8sDatasourceBasicAuthRoundTripper{
			username: ds.Username,
			password: ds.Password,
			next:     roundTripper,
		}
	case "bearer":
		roundTripper = &k8sDatasourceBearerAuthRoundTripper{
			token: ds.Token,
			next:  roundTripper,
		}
	default:
		return nil, fmt.Errorf("不支持的认证方式: %s", ds.AuthType)
	}

	client, err := api.NewClient(api.Config{
		Address:      normalizeK8SPromQueryURL(ds.URL),
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

func normalizeK8SPromQueryURL(rawURL string) string {
	url := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	for _, suffix := range []string{"/api/v1/import/prometheus", "/api/v1/write", "/write"} {
		if strings.HasSuffix(url, suffix) {
			return strings.TrimRight(strings.TrimSuffix(url, suffix), "/")
		}
	}
	return url
}

type k8sDatasourceBasicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *k8sDatasourceBasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}

type k8sDatasourceBearerAuthRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (rt *k8sDatasourceBearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if strings.TrimSpace(rt.token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(rt.token))
	}
	return rt.next.RoundTrip(req)
}
