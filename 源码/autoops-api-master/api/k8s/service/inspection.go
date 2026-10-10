package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/k8s/dao"
	"dodevops-api/api/k8s/model"

	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type InspectionService struct {
	db            *gorm.DB
	clusterDao    *dao.KubeClusterDao
	inspectionDao *dao.InspectionDao
}

func NewInspectionService(db *gorm.DB) *InspectionService {
	return &InspectionService{
		db:            db,
		clusterDao:    dao.NewKubeClusterDao(db),
		inspectionDao: dao.NewInspectionDao(db),
	}
}

func (s *InspectionService) StartInspection(ctx context.Context, req *model.StartInspectionRequest) (*model.ClusterInspection, error) {
	if len(req.ClusterIDs) == 0 {
		return nil, fmt.Errorf("未选择集群")
	}

	clusterID := req.ClusterIDs[0]
	cluster, err := s.clusterDao.GetByID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("获取集群信息失败: %w", err)
	}
	if strings.TrimSpace(cluster.Credential) == "" {
		return nil, fmt.Errorf("集群缺少 kubeconfig 配置")
	}

	now := time.Now()
	inspection := &model.ClusterInspection{
		ClusterID:         cluster.ID,
		ClusterName:       cluster.Name,
		Status:            model.InspectionStatusRunning,
		Progress:          5,
		CurrentStep:       "巡检任务已创建",
		CompletedClusters: 0,
		TotalClusters:     len(req.ClusterIDs),
		UserID:            req.UserID,
		StartTime:         now,
	}
	if err := s.inspectionDao.Create(inspection); err != nil {
		return nil, fmt.Errorf("创建巡检记录失败: %w", err)
	}

	go s.runInspection(context.Background(), inspection.ID, req.Options)
	return inspection, nil
}

func (s *InspectionService) runInspection(ctx context.Context, inspectionID uint, options model.InspectionOptions) {
	startTime := time.Now()
	inspection, err := s.inspectionDao.GetByID(inspectionID)
	if err != nil {
		return
	}

	clientset, err := s.createClientset(ctx, inspection.ClusterID)
	if err != nil {
		s.updateInspectionFailed(inspection, err)
		return
	}

	if !hasAnyInspectionOption(options) {
		options = defaultInspectionOptions()
	}

	result := &model.InspectionResult{
		ClusterID:   inspection.ClusterID,
		ClusterName: inspection.ClusterName,
	}

	type inspectionJob struct {
		progress int
		step     string
		run      func()
	}

	var mu sync.Mutex
	jobs := make([]inspectionJob, 0, 10)
	if options.CheckCluster {
		jobs = append(jobs, inspectionJob{15, "正在检查集群连接与版本...", func() {
			data := s.checkClusterInfo(ctx, clientset)
			mu.Lock()
			result.ClusterInfo = data
			mu.Unlock()
		}})
		jobs = append(jobs, inspectionJob{25, "正在检查控制面与核心组件...", func() {
			data := s.checkComponents(ctx, clientset)
			mu.Lock()
			result.Components = data
			mu.Unlock()
		}})
	}
	if options.CheckNodes {
		jobs = append(jobs, inspectionJob{35, "正在检查节点健康状态...", func() {
			data := s.checkNodeHealth(ctx, clientset)
			mu.Lock()
			result.NodeHealth = data
			mu.Unlock()
		}})
	}
	if options.CheckWorkloads {
		jobs = append(jobs, inspectionJob{45, "正在检查工作负载...", func() {
			data := s.checkWorkloads(ctx, clientset)
			mu.Lock()
			result.Workloads = data
			mu.Unlock()
		}})
	}
	if options.CheckNetwork {
		jobs = append(jobs, inspectionJob{55, "正在检查网络资源...", func() {
			data := s.checkNetwork(ctx, clientset)
			mu.Lock()
			result.Network = data
			mu.Unlock()
		}})
	}
	if options.CheckStorage {
		jobs = append(jobs, inspectionJob{65, "正在检查存储资源...", func() {
			data := s.checkStorage(ctx, clientset)
			mu.Lock()
			result.Storage = data
			mu.Unlock()
		}})
	}
	if options.CheckSecurity {
		jobs = append(jobs, inspectionJob{75, "正在检查安全配置...", func() {
			data := s.checkSecurity(ctx, clientset)
			mu.Lock()
			result.Security = data
			mu.Unlock()
		}})
	}
	if options.CheckConfig {
		jobs = append(jobs, inspectionJob{82, "正在检查配置治理...", func() {
			data := s.checkConfig(ctx, clientset)
			mu.Lock()
			result.Config = data
			mu.Unlock()
		}})
	}
	if options.CheckCapacity {
		jobs = append(jobs, inspectionJob{90, "正在检查容量水位...", func() {
			data := s.checkCapacity(ctx, clientset)
			mu.Lock()
			result.Capacity = data
			mu.Unlock()
		}})
	}
	if options.CheckEvents {
		jobs = append(jobs, inspectionJob{96, "正在分析集群事件...", func() {
			data := s.checkEvents(ctx, clientset)
			mu.Lock()
			result.Events = data
			mu.Unlock()
		}})
	}

	var wg sync.WaitGroup
	for _, job := range jobs {
		_ = s.inspectionDao.UpdateProgress(inspectionID, job.progress, job.step)
		wg.Add(1)
		go func(job inspectionJob) {
			defer wg.Done()
			job.run()
		}(job)
	}
	wg.Wait()

	result.Summary = s.summarizeResults(result)
	result.Summary.Duration = int(time.Since(startTime).Seconds())
	result.Score = s.calculateScore(result)
	s.saveInspectionResult(inspectionID, result, startTime)
}

func (s *InspectionService) createClientset(ctx context.Context, clusterID uint) (*kubernetes.Clientset, error) {
	cluster, err := s.clusterDao.GetByID(clusterID)
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		return nil, fmt.Errorf("解析 kubeconfig 失败: %w", err)
	}
	config.Timeout = 15 * time.Second

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("创建 Kubernetes 客户端失败: %w", err)
	}

	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := clientset.Discovery().ServerVersion(); err != nil {
		select {
		case <-checkCtx.Done():
			return nil, checkCtx.Err()
		default:
			return nil, fmt.Errorf("连接 Kubernetes API Server 失败: %w", err)
		}
	}
	return clientset, nil
}

func (s *InspectionService) checkClusterInfo(ctx context.Context, clientset *kubernetes.Clientset) model.ClusterInfoResult {
	result := model.ClusterInfoResult{Items: []model.CheckItem{}}
	start := time.Now()
	version, err := clientset.Discovery().ServerVersion()
	if err != nil {
		result.ConnectionState = "failed"
		result.Items = append(result.Items, model.CheckItem{
			Category:   "集群连接",
			Name:       "API Server 连接",
			Status:     model.CheckStatusError,
			Detail:     "连接失败: " + err.Error(),
			Suggestion: "检查 kubeconfig、网络连通性以及 API Server 状态",
		})
		return result
	}

	result.ConnectionDelay = time.Since(start).Milliseconds()
	result.ConnectionState = "connected"
	result.Version = version.GitVersion
	result.Platform = version.Platform
	result.GitVersion = version.GitVersion
	result.GoVersion = version.GoVersion
	result.BuildDate = version.BuildDate

	status := model.CheckStatusSuccess
	detail := "API Server 连接正常"
	suggestion := ""
	if result.ConnectionDelay > 2000 {
		status = model.CheckStatusWarning
		detail = "API Server 连接延迟较高"
		suggestion = "检查网络质量，必要时在更近的网络位置部署后端服务"
	}
	result.Items = append(result.Items, model.CheckItem{
		Category:   "集群连接",
		Name:       "API Server 连接",
		Status:     status,
		Value:      fmt.Sprintf("%dms", result.ConnectionDelay),
		Expected:   "<2000ms",
		Detail:     detail,
		Suggestion: suggestion,
	})
	result.Items = append(result.Items, model.CheckItem{
		Category: "版本信息",
		Name:     "Kubernetes 版本",
		Status:   model.CheckStatusSuccess,
		Value:    result.Version,
		Detail:   fmt.Sprintf("平台: %s, Go: %s", result.Platform, result.GoVersion),
	})
	return result
}

func (s *InspectionService) checkNodeHealth(ctx context.Context, clientset *kubernetes.Clientset) model.NodeHealthResult {
	result := model.NodeHealthResult{
		Items:           []model.CheckItem{},
		NodeUtilization: []model.NodeResource{},
	}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Items = append(result.Items, model.CheckItem{
			Category: "节点",
			Name:     "节点列表获取",
			Status:   model.CheckStatusError,
			Detail:   "获取节点列表失败: " + err.Error(),
		})
		return result
	}

	result.TotalNodes = len(nodes.Items)
	for _, node := range nodes.Items {
		nodeResource := model.NodeResource{Name: node.Name, Status: "NotReady"}
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status == corev1.ConditionTrue {
					result.ReadyNodes++
					nodeResource.Status = "Ready"
				} else {
					result.NotReadyNodes++
					result.Items = append(result.Items, model.CheckItem{
						Category:   "节点状态",
						Name:       node.Name,
						Status:     model.CheckStatusError,
						Detail:     fmt.Sprintf("节点 NotReady: %s", cond.Message),
						Suggestion: "检查 kubelet 日志、容器运行时、网络和节点资源状态",
					})
				}
			}
			if isNodePressureCondition(cond) {
				result.PressureNodes++
				result.Items = append(result.Items, model.CheckItem{
					Category:   "资源压力",
					Name:       node.Name,
					Status:     model.CheckStatusWarning,
					Detail:     fmt.Sprintf("节点存在 %s", cond.Type),
					Suggestion: "检查节点资源水位，清理异常负载或规划扩容",
				})
			}
		}
		if len(node.Spec.Taints) > 0 {
			result.TaintedNodes++
		}

		nodeResource.CPUCapacity = node.Status.Capacity.Cpu().String()
		nodeResource.MemoryCapacity = formatInspectionMemory(node.Status.Capacity.Memory().Value())
		nodeResource.PodCapacity = int(node.Status.Capacity.Pods().Value())
		result.NodeUtilization = append(result.NodeUtilization, nodeResource)
	}

	if result.NotReadyNodes == 0 {
		result.Items = append(result.Items, model.CheckItem{
			Category: "节点状态",
			Name:     "节点健康检查",
			Status:   model.CheckStatusSuccess,
			Value:    fmt.Sprintf("%d/%d Ready", result.ReadyNodes, result.TotalNodes),
			Detail:   "所有节点处于 Ready 状态",
		})
	}
	return result
}

func (s *InspectionService) checkComponents(ctx context.Context, clientset *kubernetes.Clientset) model.ComponentsResult {
	result := model.ComponentsResult{
		ControlPlane: []model.ComponentStatus{},
		Addons:       []model.ComponentStatus{},
		Items:        []model.CheckItem{},
	}
	pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Items = append(result.Items, model.CheckItem{
			Category: "组件",
			Name:     "kube-system 组件检查",
			Status:   model.CheckStatusError,
			Detail:   "获取 kube-system Pod 失败: " + err.Error(),
		})
		return result
	}

	controlPlaneComponents := []string{"kube-apiserver", "kube-controller-manager", "kube-scheduler", "etcd"}
	addonComponents := []string{"coredns", "kube-dns", "kube-proxy", "calico", "flannel", "cilium", "metrics-server"}

	for _, pod := range pods.Items {
		name, isControlPlane, isAddon := matchComponentName(pod.Name, controlPlaneComponents, addonComponents)
		if name == "" {
			continue
		}

		status := "Healthy"
		if pod.Status.Phase != corev1.PodRunning {
			status = "Unhealthy"
		}
		restarts, ready := containerReadyStatus(pod)
		component := model.ComponentStatus{
			Name:      name,
			Namespace: pod.Namespace,
			Status:    status,
			Ready:     ready,
			Restarts:  restarts,
			Age:       formatInspectionAge(pod.CreationTimestamp.Time),
		}
		if isControlPlane {
			result.ControlPlane = append(result.ControlPlane, component)
		} else if isAddon {
			result.Addons = append(result.Addons, component)
		}
	}

	appendComponentItems(&result, result.ControlPlane, "控制面")
	appendComponentItems(&result, result.Addons, "插件组件")
	return result
}

func (s *InspectionService) checkWorkloads(ctx context.Context, clientset *kubernetes.Clientset) model.WorkloadsResult {
	result := model.WorkloadsResult{
		Items:              []model.CheckItem{},
		PodsByPhase:        make(map[string]int),
		UnhealthyWorkloads: []model.WorkloadInfo{},
	}

	if deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalDeployments = len(deployments.Items)
		for _, dep := range deployments.Items {
			if dep.Status.ReadyReplicas == dep.Status.Replicas && dep.Status.Replicas > 0 {
				result.HealthyDeployments++
				continue
			}
			if dep.Status.Replicas > 0 {
				result.UnhealthyWorkloads = append(result.UnhealthyWorkloads, workloadInfo("Deployment", dep.Namespace, dep.Name, dep.Status.ReadyReplicas, dep.Status.Replicas, "副本数不符合预期"))
			}
		}
	}
	if daemonsets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalDaemonSets = len(daemonsets.Items)
		for _, ds := range daemonsets.Items {
			if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
				result.HealthyDaemonSets++
				continue
			}
			result.UnhealthyWorkloads = append(result.UnhealthyWorkloads, workloadInfo("DaemonSet", ds.Namespace, ds.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled, "节点调度数量不符合预期"))
		}
	}
	if statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalStatefulSets = len(statefulSets.Items)
		for _, sts := range statefulSets.Items {
			desired := desiredStatefulSetReplicas(sts)
			if sts.Status.ReadyReplicas == desired && desired > 0 {
				result.HealthyStatefulSets++
				continue
			}
			if desired > 0 {
				result.UnhealthyWorkloads = append(result.UnhealthyWorkloads, workloadInfo("StatefulSet", sts.Namespace, sts.Name, sts.Status.ReadyReplicas, desired, "副本数不符合预期"))
			}
		}
	}
	if pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalPods = len(pods.Items)
		for _, pod := range pods.Items {
			result.PodsByPhase[string(pod.Status.Phase)]++
			switch pod.Status.Phase {
			case corev1.PodRunning:
				result.RunningPods++
			case corev1.PodPending:
				result.PendingPods++
			case corev1.PodFailed:
				result.FailedPods++
			}
			if podHasHighRestarts(pod, 5) {
				result.HighRestartPods++
			}
			if podHasImagePullError(pod) {
				result.ImagePullErrors++
			}
		}
	}

	status := model.CheckStatusSuccess
	if len(result.UnhealthyWorkloads) > 0 || result.FailedPods > 0 || result.ImagePullErrors > 0 {
		status = model.CheckStatusError
	} else if result.PendingPods > 0 || result.HighRestartPods > 0 {
		status = model.CheckStatusWarning
	}
	result.Items = append(result.Items, model.CheckItem{
		Category:   "工作负载",
		Name:       "工作负载健康",
		Status:     status,
		Value:      fmt.Sprintf("Pods: %d, Running: %d", result.TotalPods, result.RunningPods),
		Detail:     fmt.Sprintf("异常工作负载: %d, Pending: %d, Failed: %d, 镜像拉取异常: %d", len(result.UnhealthyWorkloads), result.PendingPods, result.FailedPods, result.ImagePullErrors),
		Suggestion: workloadSuggestion(status),
	})
	return result
}

func (s *InspectionService) checkNetwork(ctx context.Context, clientset *kubernetes.Clientset) model.NetworkResult {
	result := model.NetworkResult{Items: []model.CheckItem{}}
	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err == nil {
		result.TotalServices = len(services.Items)
		for _, svc := range services.Items {
			switch svc.Spec.Type {
			case corev1.ServiceTypeClusterIP:
				result.ClusterIPServices++
			case corev1.ServiceTypeNodePort:
				result.NodePortServices++
			case corev1.ServiceTypeLoadBalancer:
				result.LoadBalancerServices++
			}
			if serviceNeedsEndpoint(svc) {
				endpointSummary, epErr := GetServiceEndpointSummary(ctx, clientset, svc.Namespace, svc.Name)
				if epErr != nil || endpointSummary.Ready == 0 {
					result.NoEndpointServices++
				}
			}
		}
	}
	if ingresses, err := clientset.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalIngresses = len(ingresses.Items)
	}
	if policies, err := clientset.NetworkingV1().NetworkPolicies("").List(ctx, metav1.ListOptions{}); err == nil {
		result.NetworkPolicies = len(policies.Items)
	}

	status := model.CheckStatusSuccess
	if result.NoEndpointServices > 0 {
		status = model.CheckStatusWarning
	}
	result.Items = append(result.Items, model.CheckItem{
		Category:   "网络",
		Name:       "Service Endpoint 检查",
		Status:     status,
		Value:      fmt.Sprintf("%d 个无 Endpoint", result.NoEndpointServices),
		Detail:     fmt.Sprintf("Service: %d, Ingress: %d, NetworkPolicy: %d", result.TotalServices, result.TotalIngresses, result.NetworkPolicies),
		Suggestion: networkSuggestion(result.NoEndpointServices),
	})
	return result
}

func (s *InspectionService) checkStorage(ctx context.Context, clientset *kubernetes.Clientset) model.StorageResult {
	result := model.StorageResult{Items: []model.CheckItem{}}
	if pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalPVs = len(pvs.Items)
		for _, pv := range pvs.Items {
			switch pv.Status.Phase {
			case corev1.VolumeAvailable:
				result.AvailablePVs++
			case corev1.VolumeBound:
				result.BoundPVs++
			case corev1.VolumeReleased:
				result.ReleasedPVs++
			case corev1.VolumeFailed:
				result.FailedPVs++
			}
		}
	}
	if pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalPVCs = len(pvcs.Items)
		for _, pvc := range pvcs.Items {
			switch pvc.Status.Phase {
			case corev1.ClaimBound:
				result.BoundPVCs++
			case corev1.ClaimPending:
				result.PendingPVCs++
			}
		}
	}
	if classes, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{}); err == nil {
		result.StorageClasses = len(classes.Items)
		for _, sc := range classes.Items {
			if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
				result.DefaultSC = sc.Name
				break
			}
		}
	}

	status := model.CheckStatusSuccess
	if result.FailedPVs > 0 || result.PendingPVCs > 0 {
		status = model.CheckStatusError
	} else if result.ReleasedPVs > 0 || result.DefaultSC == "" {
		status = model.CheckStatusWarning
	}
	result.Items = append(result.Items, model.CheckItem{
		Category:   "存储",
		Name:       "PV/PVC 状态",
		Status:     status,
		Value:      fmt.Sprintf("PVC Pending: %d, PV Failed: %d", result.PendingPVCs, result.FailedPVs),
		Detail:     fmt.Sprintf("PV: %d, PVC: %d, StorageClass: %d, 默认 StorageClass: %s", result.TotalPVs, result.TotalPVCs, result.StorageClasses, valueOrNone(result.DefaultSC)),
		Suggestion: storageSuggestion(status),
	})
	return result
}

func (s *InspectionService) checkSecurity(ctx context.Context, clientset *kubernetes.Clientset) model.SecurityResult {
	result := model.SecurityResult{Items: []model.CheckItem{}}
	if serviceAccounts, err := clientset.CoreV1().ServiceAccounts("").List(ctx, metav1.ListOptions{}); err == nil {
		result.ServiceAccounts = len(serviceAccounts.Items)
	}
	if roles, err := clientset.RbacV1().Roles("").List(ctx, metav1.ListOptions{}); err == nil {
		result.Roles = len(roles.Items)
	}
	if clusterRoles, err := clientset.RbacV1().ClusterRoles().List(ctx, metav1.ListOptions{}); err == nil {
		result.ClusterRoles = len(clusterRoles.Items)
	}
	if bindings, err := clientset.RbacV1().RoleBindings("").List(ctx, metav1.ListOptions{}); err == nil {
		result.RoleBindings = len(bindings.Items)
		result.ClusterAdminBindings += countClusterAdminRoleBindings(bindings.Items)
	}
	if bindings, err := clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{}); err == nil {
		result.ClusterRoleBindings = len(bindings.Items)
		result.ClusterAdminBindings += countClusterAdminClusterRoleBindings(bindings.Items)
	}
	if pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
		for _, pod := range pods.Items {
			if pod.Spec.HostNetwork {
				result.HostNetworkPods++
			}
			if podHasPrivilegedContainer(pod) {
				result.PrivilegedPods++
			}
			result.RootUserContainers += countRootUserContainers(pod)
		}
	}

	result.Items = append(result.Items, model.CheckItem{
		Category: "安全",
		Name:     "RBAC 资源统计",
		Status:   model.CheckStatusSuccess,
		Value:    fmt.Sprintf("SA: %d, Role: %d, CRB: %d", result.ServiceAccounts, result.Roles, result.ClusterRoleBindings),
		Detail:   "RBAC 资源数量统计",
	})
	if result.ClusterAdminBindings > 3 {
		result.Items = append(result.Items, model.CheckItem{
			Category:   "安全",
			Name:       "cluster-admin 绑定",
			Status:     model.CheckStatusWarning,
			Value:      fmt.Sprintf("%d 个", result.ClusterAdminBindings),
			Detail:     "cluster-admin 绑定数量较多",
			Suggestion: "按最小权限原则收敛 cluster-admin 使用范围",
		})
	}
	if result.PrivilegedPods > 0 || result.HostNetworkPods > 0 || result.RootUserContainers > 0 {
		result.Items = append(result.Items, model.CheckItem{
			Category:   "安全",
			Name:       "高风险 Pod 安全上下文",
			Status:     model.CheckStatusWarning,
			Value:      fmt.Sprintf("privileged=%d, hostNetwork=%d, root=%d", result.PrivilegedPods, result.HostNetworkPods, result.RootUserContainers),
			Detail:     "存在特权、hostNetwork 或 root 用户运行的容器",
			Suggestion: "逐项确认必要性，优先使用非特权和非 root 配置",
		})
	}
	return result
}

func (s *InspectionService) checkConfig(ctx context.Context, clientset *kubernetes.Clientset) model.ConfigResult {
	result := model.ConfigResult{Items: []model.CheckItem{}}
	if configMaps, err := clientset.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalConfigMaps = len(configMaps.Items)
	}
	if secrets, err := clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{}); err == nil {
		result.TotalSecrets = len(secrets.Items)
	}
	if namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); err == nil {
		result.NamespaceCount = len(namespaces.Items)
	}
	if quotas, err := clientset.CoreV1().ResourceQuotas("").List(ctx, metav1.ListOptions{}); err == nil {
		result.ResourceQuotaCount = len(quotas.Items)
	}
	if limits, err := clientset.CoreV1().LimitRanges("").List(ctx, metav1.ListOptions{}); err == nil {
		result.LimitRangeCount = len(limits.Items)
	}
	totalPods := 0
	if pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
		totalPods = len(pods.Items)
		for _, pod := range pods.Items {
			if podMissingRequestsOrLimits(pod) {
				result.NoRequestLimitPods++
			}
		}
	}

	result.Items = append(result.Items, model.CheckItem{
		Category: "配置",
		Name:     "配置资源统计",
		Status:   model.CheckStatusSuccess,
		Value:    fmt.Sprintf("ConfigMap: %d, Secret: %d", result.TotalConfigMaps, result.TotalSecrets),
		Detail:   fmt.Sprintf("Namespace: %d, ResourceQuota: %d, LimitRange: %d", result.NamespaceCount, result.ResourceQuotaCount, result.LimitRangeCount),
	})
	if result.NoRequestLimitPods > 0 {
		percent := percent(result.NoRequestLimitPods, totalPods)
		status := model.CheckStatusWarning
		if percent > 50 {
			status = model.CheckStatusError
		}
		result.Items = append(result.Items, model.CheckItem{
			Category:   "配置",
			Name:       "资源请求与限制",
			Status:     status,
			Value:      fmt.Sprintf("%.1f%% 未完整设置", percent),
			Detail:     fmt.Sprintf("%d/%d 个 Pod 未完整设置 requests/limits", result.NoRequestLimitPods, totalPods),
			Suggestion: "为业务容器设置合理的 CPU/内存 requests 和 limits",
		})
	}
	return result
}

func (s *InspectionService) checkCapacity(ctx context.Context, clientset *kubernetes.Clientset) model.CapacityResult {
	result := model.CapacityResult{Items: []model.CheckItem{}}
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Items = append(result.Items, model.CheckItem{
			Category: "容量",
			Name:     "节点容量获取",
			Status:   model.CheckStatusError,
			Detail:   "获取节点容量失败: " + err.Error(),
		})
		return result
	}

	var totalCPU, allocatedCPU int64
	var totalMemory, allocatedMemory int64
	var totalPodCapacity int
	for _, node := range nodes.Items {
		totalCPU += node.Status.Capacity.Cpu().MilliValue()
		totalMemory += node.Status.Capacity.Memory().Value()
		totalPodCapacity += int(node.Status.Capacity.Pods().Value())
	}
	if pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil {
		result.CurrentPodCount = len(pods.Items)
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
				continue
			}
			for _, container := range pod.Spec.Containers {
				allocatedCPU += container.Resources.Requests.Cpu().MilliValue()
				allocatedMemory += container.Resources.Requests.Memory().Value()
			}
		}
	}

	result.TotalCPU = fmt.Sprintf("%.2f cores", float64(totalCPU)/1000)
	result.AllocatedCPU = fmt.Sprintf("%.2f cores", float64(allocatedCPU)/1000)
	result.CPUAllocatePercent = percent64(allocatedCPU, totalCPU)
	result.TotalMemory = formatInspectionMemory(totalMemory)
	result.AllocatedMemory = formatInspectionMemory(allocatedMemory)
	result.MemoryAllocatePercent = percent64(allocatedMemory, totalMemory)
	result.TotalPodCapacity = totalPodCapacity
	result.PodDensityPercent = percent(result.CurrentPodCount, totalPodCapacity)

	result.Items = append(result.Items,
		capacityItem("CPU 容量", result.CPUAllocatePercent, fmt.Sprintf("已分配: %s / 总量: %s", result.AllocatedCPU, result.TotalCPU), getCapacitySuggestion("CPU", result.CPUAllocatePercent)),
		capacityItem("内存容量", result.MemoryAllocatePercent, fmt.Sprintf("已分配: %s / 总量: %s", result.AllocatedMemory, result.TotalMemory), getCapacitySuggestion("内存", result.MemoryAllocatePercent)),
		capacityItem("Pod 密度", result.PodDensityPercent, fmt.Sprintf("当前: %d / 最大: %d", result.CurrentPodCount, result.TotalPodCapacity), getCapacitySuggestion("Pod", result.PodDensityPercent)),
	)
	return result
}

func (s *InspectionService) checkEvents(ctx context.Context, clientset *kubernetes.Clientset) model.EventsResult {
	result := model.EventsResult{
		Items:          []model.CheckItem{},
		RecentEvents:   []model.InspectionEventInfo{},
		HighFreqEvents: []model.InspectionEventInfo{},
	}
	events, err := clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		result.Items = append(result.Items, model.CheckItem{
			Category: "事件",
			Name:     "事件获取",
			Status:   model.CheckStatusError,
			Detail:   "获取事件失败: " + err.Error(),
		})
		return result
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	eventCounts := make(map[string]int)
	type recentEventCandidate struct {
		last time.Time
		info model.InspectionEventInfo
	}
	recentEventCandidates := make([]recentEventCandidate, 0)
	for _, event := range events.Items {
		last := eventLastSeenTime(event)
		if last.After(oneHourAgo) {
			if event.Type == corev1.EventTypeWarning {
				result.WarningEvents++
			}
			if strings.Contains(strings.ToLower(event.Reason), "error") {
				result.ErrorEvents++
			}
			key := fmt.Sprintf("%s/%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Reason)
			eventCounts[key] += int(event.Count)
		}
		if event.Type == corev1.EventTypeWarning {
			recentEventCandidates = append(recentEventCandidates, recentEventCandidate{
				last: last,
				info: model.InspectionEventInfo{
					Type:      event.Type,
					Reason:    event.Reason,
					Message:   event.Message,
					Object:    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
					Namespace: event.Namespace,
					Count:     int(event.Count),
					LastSeen:  last.Format("2006-01-02 15:04:05"),
				},
			})
		}
	}
	sort.Slice(recentEventCandidates, func(i, j int) bool {
		return recentEventCandidates[i].last.After(recentEventCandidates[j].last)
	})
	recentLimit := len(recentEventCandidates)
	if recentLimit > 10 {
		recentLimit = 10
	}
	for i := 0; i < recentLimit; i++ {
		result.RecentEvents = append(result.RecentEvents, recentEventCandidates[i].info)
	}
	result.HighFreqEvents = highFrequencyEvents(eventCounts)

	status := model.CheckStatusSuccess
	if result.ErrorEvents > 5 || result.WarningEvents > 50 {
		status = model.CheckStatusError
	} else if result.ErrorEvents > 0 || result.WarningEvents > 10 {
		status = model.CheckStatusWarning
	}
	result.Items = append(result.Items, model.CheckItem{
		Category:   "事件",
		Name:       "最近 1 小时事件",
		Status:     status,
		Value:      fmt.Sprintf("Warning: %d, Error: %d", result.WarningEvents, result.ErrorEvents),
		Detail:     "最近 1 小时内的告警和错误事件统计",
		Suggestion: eventSuggestion(result.WarningEvents, result.ErrorEvents),
	})
	if len(result.HighFreqEvents) > 0 {
		result.Items = append(result.Items, model.CheckItem{
			Category:   "事件",
			Name:       "高频事件",
			Status:     model.CheckStatusWarning,
			Value:      fmt.Sprintf("%d 个", len(result.HighFreqEvents)),
			Detail:     "存在重复次数超过 10 次的事件",
			Suggestion: "优先排查高频事件对应对象的状态和配置",
		})
	}
	return result
}

func eventLastSeenTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.Time.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.Time.IsZero() {
		return event.EventTime.Time
	}
	if !event.FirstTimestamp.Time.IsZero() {
		return event.FirstTimestamp.Time
	}
	return event.CreationTimestamp.Time
}

func (s *InspectionService) summarizeResults(result *model.InspectionResult) model.InspectionSummary {
	items := collectInspectionItems(result)
	summary := model.InspectionSummary{TotalChecks: len(items)}
	for _, item := range items {
		switch item.Status {
		case model.CheckStatusSuccess:
			summary.PassedChecks++
		case model.CheckStatusWarning:
			summary.WarningChecks++
		case model.CheckStatusError:
			summary.FailedChecks++
		}
	}
	return summary
}

func (s *InspectionService) calculateScore(result *model.InspectionResult) int {
	if result.Summary.TotalChecks == 0 {
		return 100
	}

	// Score by weighted health ratio instead of fixed deductions, so a report
	// with many warning items does not collapse to 0 unless most checks failed.
	score := (result.Summary.PassedChecks*100 + result.Summary.WarningChecks*60) / result.Summary.TotalChecks
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func (s *InspectionService) saveInspectionResult(inspectionID uint, result *model.InspectionResult, startTime time.Time) {
	inspection, err := s.inspectionDao.GetByID(inspectionID)
	if err != nil {
		return
	}
	endTime := time.Now()
	reportData, _ := json.Marshal(result)
	inspection.Status = model.InspectionStatusCompleted
	inspection.Progress = 100
	inspection.CurrentStep = "巡检完成"
	inspection.CompletedClusters = inspection.TotalClusters
	inspection.Score = result.Score
	inspection.CheckCount = result.Summary.TotalChecks
	inspection.PassCount = result.Summary.PassedChecks
	inspection.WarningCount = result.Summary.WarningChecks
	inspection.FailCount = result.Summary.FailedChecks
	inspection.Duration = int(endTime.Sub(startTime).Seconds())
	inspection.ReportData = string(reportData)
	inspection.EndTime = &endTime
	_ = s.inspectionDao.Save(inspection)
}

func (s *InspectionService) updateInspectionFailed(inspection *model.ClusterInspection, runErr error) {
	endTime := time.Now()
	inspection.Status = model.InspectionStatusFailed
	inspection.Progress = 0
	inspection.CurrentStep = "巡检失败"
	inspection.EndTime = &endTime
	inspection.ReportData = fmt.Sprintf(`{"error":%q}`, runErr.Error())
	_ = s.inspectionDao.Save(inspection)
}

func (s *InspectionService) GetInspectionProgress(ctx context.Context, inspectionID uint) (*model.InspectionProgress, error) {
	inspection, err := s.inspectionDao.GetByID(inspectionID)
	if err != nil {
		return nil, err
	}
	progress := &model.InspectionProgress{
		InspectionID:      inspection.ID,
		Status:            inspection.Status,
		Progress:          inspection.Progress,
		CurrentStep:       inspection.CurrentStep,
		CompletedClusters: inspection.CompletedClusters,
		TotalClusters:     inspection.TotalClusters,
	}
	if progress.CurrentStep == "" {
		switch inspection.Status {
		case model.InspectionStatusCompleted:
			progress.CurrentStep = "巡检完成"
			progress.Progress = 100
		case model.InspectionStatusFailed:
			progress.CurrentStep = "巡检失败"
		default:
			progress.CurrentStep = "正在执行巡检..."
			if progress.Progress == 0 {
				progress.Progress = 50
			}
		}
	}
	return progress, nil
}

func (s *InspectionService) GetInspectionResult(ctx context.Context, inspectionID uint) (*model.ClusterInspection, *model.InspectionResult, error) {
	inspection, err := s.inspectionDao.GetByID(inspectionID)
	if err != nil {
		return nil, nil, err
	}
	var result model.InspectionResult
	if strings.TrimSpace(inspection.ReportData) != "" {
		if err := json.Unmarshal([]byte(inspection.ReportData), &result); err != nil {
			return inspection, nil, nil
		}
	}
	return inspection, &result, nil
}

func (s *InspectionService) GetInspectionHistory(ctx context.Context, clusterID uint, page, pageSize int) ([]model.InspectionHistoryItem, int64, error) {
	inspections, total, err := s.inspectionDao.List(clusterID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	items := make([]model.InspectionHistoryItem, 0, len(inspections))
	for _, inspection := range inspections {
		items = append(items, model.InspectionHistoryItem{
			ID:           inspection.ID,
			ClusterID:    inspection.ClusterID,
			ClusterName:  inspection.ClusterName,
			Score:        inspection.Score,
			Status:       inspection.Status,
			CheckCount:   inspection.CheckCount,
			PassCount:    inspection.PassCount,
			WarningCount: inspection.WarningCount,
			FailCount:    inspection.FailCount,
			Duration:     inspection.Duration,
			CreatedAt:    inspection.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, total, nil
}

func (s *InspectionService) DeleteInspection(ctx context.Context, inspectionID uint) error {
	return s.inspectionDao.Delete(inspectionID)
}

func hasAnyInspectionOption(options model.InspectionOptions) bool {
	return options.CheckCluster || options.CheckNodes || options.CheckWorkloads ||
		options.CheckNetwork || options.CheckStorage || options.CheckSecurity ||
		options.CheckConfig || options.CheckCapacity || options.CheckEvents
}

func defaultInspectionOptions() model.InspectionOptions {
	return model.InspectionOptions{
		CheckCluster:   true,
		CheckNodes:     true,
		CheckWorkloads: true,
		CheckNetwork:   true,
		CheckStorage:   true,
		CheckSecurity:  true,
		CheckConfig:    true,
		CheckCapacity:  true,
		CheckEvents:    true,
	}
}

func collectInspectionItems(result *model.InspectionResult) []model.CheckItem {
	items := []model.CheckItem{}
	items = append(items, result.ClusterInfo.Items...)
	items = append(items, result.NodeHealth.Items...)
	items = append(items, result.Components.Items...)
	items = append(items, result.Workloads.Items...)
	items = append(items, result.Network.Items...)
	items = append(items, result.Storage.Items...)
	items = append(items, result.Security.Items...)
	items = append(items, result.Config.Items...)
	items = append(items, result.Capacity.Items...)
	items = append(items, result.Events.Items...)
	return items
}

func isNodePressureCondition(cond corev1.NodeCondition) bool {
	if cond.Status != corev1.ConditionTrue {
		return false
	}
	return cond.Type == corev1.NodeMemoryPressure ||
		cond.Type == corev1.NodeDiskPressure ||
		cond.Type == corev1.NodePIDPressure
}

func matchComponentName(podName string, controlPlane, addons []string) (string, bool, bool) {
	for _, name := range controlPlane {
		if strings.Contains(podName, name) {
			return name, true, false
		}
	}
	for _, name := range addons {
		if strings.Contains(podName, name) {
			return name, false, true
		}
	}
	return "", false, false
}

func containerReadyStatus(pod corev1.Pod) (int, string) {
	restarts := 0
	readyCount := 0
	for _, status := range pod.Status.ContainerStatuses {
		restarts += int(status.RestartCount)
		if status.Ready {
			readyCount++
		}
	}
	return restarts, fmt.Sprintf("%d/%d", readyCount, len(pod.Status.ContainerStatuses))
}

func appendComponentItems(result *model.ComponentsResult, components []model.ComponentStatus, category string) {
	unhealthy := 0
	for _, component := range components {
		if component.Status != "Healthy" {
			unhealthy++
			result.Items = append(result.Items, model.CheckItem{
				Category:   category,
				Name:       component.Name,
				Status:     model.CheckStatusError,
				Detail:     "组件状态异常: " + component.Status,
				Suggestion: "检查组件 Pod 事件和日志，确认镜像、资源与节点状态",
			})
			continue
		}
		if component.Restarts > 5 {
			result.Items = append(result.Items, model.CheckItem{
				Category:   category,
				Name:       component.Name,
				Status:     model.CheckStatusWarning,
				Value:      fmt.Sprintf("重启 %d 次", component.Restarts),
				Detail:     "组件重启次数较多",
				Suggestion: "检查组件日志，排查重启原因",
			})
		}
	}
	if unhealthy == 0 && len(components) > 0 {
		result.Items = append(result.Items, model.CheckItem{
			Category: category,
			Name:     category + "健康检查",
			Status:   model.CheckStatusSuccess,
			Value:    fmt.Sprintf("%d 个组件正常", len(components)),
			Detail:   "组件运行正常",
		})
	}
}

func workloadInfo(kind, namespace, name string, ready, desired int32, reason string) model.WorkloadInfo {
	return model.WorkloadInfo{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Ready:     fmt.Sprintf("%d/%d", ready, desired),
		Status:    "NotReady",
		Reason:    reason,
	}
}

func desiredStatefulSetReplicas(sts appsv1.StatefulSet) int32 {
	if sts.Spec.Replicas == nil {
		return 1
	}
	return *sts.Spec.Replicas
}

func podHasHighRestarts(pod corev1.Pod, threshold int32) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount > threshold {
			return true
		}
	}
	return false
}

func podHasImagePullError(pod corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting == nil {
			continue
		}
		reason := status.State.Waiting.Reason
		if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
			return true
		}
	}
	return false
}

func workloadSuggestion(status string) string {
	if status == model.CheckStatusSuccess {
		return ""
	}
	return "检查异常 Pod 的事件、镜像拉取状态、调度状态和控制器副本配置"
}

func serviceNeedsEndpoint(svc corev1.Service) bool {
	return svc.Spec.Type != corev1.ServiceTypeExternalName && svc.Spec.ClusterIP != corev1.ClusterIPNone
}

func networkSuggestion(noEndpointServices int) string {
	if noEndpointServices == 0 {
		return ""
	}
	return "检查 Service selector 与后端 Pod 标签是否匹配，以及 Pod 是否 Ready"
}

func storageSuggestion(status string) string {
	if status == model.CheckStatusSuccess {
		return ""
	}
	return "检查 PVC 绑定状态、StorageClass 配置、底层存储插件和 PV 生命周期"
}

func countClusterAdminRoleBindings(bindings []rbacv1.RoleBinding) int {
	count := 0
	for _, binding := range bindings {
		if binding.RoleRef.Kind == "ClusterRole" && binding.RoleRef.Name == "cluster-admin" {
			count++
		}
	}
	return count
}

func countClusterAdminClusterRoleBindings(bindings []rbacv1.ClusterRoleBinding) int {
	count := 0
	for _, binding := range bindings {
		if binding.RoleRef.Kind == "ClusterRole" && binding.RoleRef.Name == "cluster-admin" {
			count++
		}
	}
	return count
}

func podHasPrivilegedContainer(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil &&
			container.SecurityContext.Privileged != nil &&
			*container.SecurityContext.Privileged {
			return true
		}
	}
	return false
}

func countRootUserContainers(pod corev1.Pod) int {
	count := 0
	for _, container := range pod.Spec.Containers {
		if container.SecurityContext != nil &&
			container.SecurityContext.RunAsUser != nil &&
			*container.SecurityContext.RunAsUser == 0 {
			count++
		}
	}
	return count
}

func podMissingRequestsOrLimits(pod corev1.Pod) bool {
	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests.Cpu().IsZero() ||
			container.Resources.Requests.Memory().IsZero() ||
			container.Resources.Limits.Cpu().IsZero() ||
			container.Resources.Limits.Memory().IsZero() {
			return true
		}
	}
	return false
}

func capacityItem(name string, value float64, detail, suggestion string) model.CheckItem {
	status := model.CheckStatusSuccess
	if value > 85 {
		status = model.CheckStatusError
	} else if value > 70 {
		status = model.CheckStatusWarning
	}
	return model.CheckItem{
		Category:   "容量",
		Name:       name,
		Status:     status,
		Value:      fmt.Sprintf("%.1f%%", value),
		Detail:     detail,
		Suggestion: suggestion,
	}
}

func getCapacitySuggestion(name string, percent float64) string {
	if percent > 85 {
		return name + " 资源水位很高，建议立即扩容或治理资源占用"
	}
	if percent > 70 {
		return name + " 资源水位偏高，建议规划扩容"
	}
	return name + " 容量充足"
}

func highFrequencyEvents(eventCounts map[string]int) []model.InspectionEventInfo {
	type eventCount struct {
		key   string
		count int
	}
	items := make([]eventCount, 0)
	for key, count := range eventCounts {
		if count > 10 {
			items = append(items, eventCount{key: key, count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
	})
	limit := len(items)
	if limit > 5 {
		limit = 5
	}
	result := make([]model.InspectionEventInfo, 0, limit)
	for i := 0; i < limit; i++ {
		parts := strings.Split(items[i].key, "/")
		reason := parts[len(parts)-1]
		result = append(result, model.InspectionEventInfo{
			Object: items[i].key,
			Reason: reason,
			Count:  items[i].count,
		})
	}
	return result
}

func eventSuggestion(warnings, errors int) string {
	if errors > 5 || warnings > 50 {
		return "存在大量告警或错误事件，需要立即排查"
	}
	if errors > 0 || warnings > 10 {
		return "存在告警事件，建议关注并定位高频对象"
	}
	return "事件状态正常"
}

func percent(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func percent64(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func valueOrNone(value string) string {
	if value == "" {
		return "无"
	}
	return value
}

func formatInspectionAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24 {
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dm", int(duration.Minutes()))
}

func formatInspectionMemory(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.2fTi", float64(bytes)/tb)
	case bytes >= gb:
		return fmt.Sprintf("%.2fGi", float64(bytes)/gb)
	case bytes >= mb:
		return fmt.Sprintf("%.2fMi", float64(bytes)/mb)
	case bytes >= kb:
		return fmt.Sprintf("%.2fKi", float64(bytes)/kb)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
