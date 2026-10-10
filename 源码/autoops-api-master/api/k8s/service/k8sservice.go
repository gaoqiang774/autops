package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dodevops-api/api/k8s/dao"
	"dodevops-api/api/k8s/model"
	"dodevops-api/common/cache"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

// IK8sServiceService K8s Service服务接口
type IK8sServiceService interface {
	// Service管理
	GetServices(c *gin.Context, clusterId uint, namespaceName string, keyword string, page, pageSize int)
	GetServiceDetail(c *gin.Context, clusterId uint, namespaceName string, serviceName string)
	CreateService(c *gin.Context, clusterId uint, namespaceName string, req *model.CreateServiceRequest)
	UpdateService(c *gin.Context, clusterId uint, namespaceName string, serviceName string, req *model.UpdateServiceRequest)
	DeleteService(c *gin.Context, clusterId uint, namespaceName string, serviceName string)
	GetServiceYaml(c *gin.Context, clusterId uint, namespaceName string, serviceName string)
	UpdateServiceYaml(c *gin.Context, clusterId uint, namespaceName string, serviceName string, yamlData map[string]interface{})
	GetServiceEvents(c *gin.Context, clusterId uint, namespaceName string, serviceName string)
}

// K8sServiceServiceImpl K8s Service服务实现
type K8sServiceServiceImpl struct {
	clusterDao   *dao.KubeClusterDao
	cacheService cache.ICacheService
}

// NewK8sServiceService 创建K8s Service服务实例
func NewK8sServiceService(db *gorm.DB) IK8sServiceService {
	return &K8sServiceServiceImpl{
		clusterDao:   dao.NewKubeClusterDao(db),
		cacheService: newK8sCacheService(),
	}
}

// getK8sClient 获取K8s客户端
func (s *K8sServiceServiceImpl) getK8sClient(clusterId uint) (*kubernetes.Clientset, error) {
	cluster, err := s.clusterDao.GetByID(clusterId)
	if err != nil {
		return nil, fmt.Errorf("获取集群信息失败: %v", err)
	}

	if cluster.Credential == "" {
		return nil, fmt.Errorf("集群凭证为空")
	}

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		return nil, fmt.Errorf("解析集群凭证失败: %v", err)
	}

	if config.Timeout == 0 {
		config.Timeout = 12 * time.Second
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("创建K8s客户端失败: %v", err)
	}

	return clientset, nil
}

// GetServices 获取Service列表
func (s *K8sServiceServiceImpl) GetServices(c *gin.Context, clusterId uint, namespaceName string, keyword string, page, pageSize int) {
	ctx := c.Request.Context()
	keyword = strings.TrimSpace(keyword)

	cacheKey := servicesListCacheKey(clusterId, namespaceName)
	var cachedResponse model.ServiceListResponse
	if !getCachedJSON(ctx, s.cacheService, cacheKey, &cachedResponse) {
		response, err := s.loadServiceList(ctx, clusterId, namespaceName)
		if err != nil {
			result.Failed(c, http.StatusInternalServerError, "获取Service列表失败: "+err.Error())
			return
		}

		setCachedJSON(ctx, s.cacheService, cacheKey, response, servicesListCacheTTL)
		cachedResponse = response
	}

	if keyword != "" {
		cachedResponse.Services = filterServicesByName(cachedResponse.Services, keyword)
		cachedResponse.Total = len(cachedResponse.Services)
	}

	if pageSize > 0 {
		cachedResponse = paginateServiceListResponse(cachedResponse, page, pageSize)
	}

	result.Success(c, cachedResponse)
}

func (s *K8sServiceServiceImpl) loadServiceList(ctx context.Context, clusterId uint, namespaceName string) (model.ServiceListResponse, error) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		return model.ServiceListResponse{}, fmt.Errorf("连接K8s集群失败: %w", err)
	}

	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	type listResult struct {
		services  *corev1.ServiceList
		endpoints map[string][]model.ServiceEndpoint
		err       error
		kind      string
	}
	ch := make(chan listResult, 2)

	go func() {
		services, err := clientset.CoreV1().Services(namespaceName).List(listCtx, metav1.ListOptions{})
		ch <- listResult{services: services, err: err, kind: "services"}
	}()

	go func() {
		endpoints, err := listAllServiceEndpointsCompat(listCtx, clientset, namespaceName)
		ch <- listResult{endpoints: endpoints, err: err, kind: "endpoints"}
	}()

	var (
		services  *corev1.ServiceList
		endpoints map[string][]model.ServiceEndpoint
	)

	for i := 0; i < 2; i++ {
		item := <-ch
		if item.err != nil {
			if item.kind == "services" {
				return model.ServiceListResponse{}, item.err
			}
			continue
		}

		if item.kind == "services" {
			services = item.services
			continue
		}

		endpoints = item.endpoints
	}

	var serviceList []model.K8sService
	for _, svc := range services.Items {
		k8sService := s.convertToK8sService(&svc, endpoints[svc.Name])
		serviceList = append(serviceList, k8sService)
	}

	return model.ServiceListResponse{
		Services: serviceList,
		Total:    len(serviceList),
	}, nil
}

func (s *K8sServiceServiceImpl) loadServiceListPage(ctx context.Context, clusterId uint, namespaceName string, page, pageSize int) (model.ServiceListResponse, error) {
	if page <= 0 {
		page = 1
	}

	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		return model.ServiceListResponse{}, fmt.Errorf("连接K8s集群失败: %w", err)
	}

	listCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	services, total, err := listServicePage(listCtx, clientset, namespaceName, page, pageSize)
	if err != nil {
		return model.ServiceListResponse{}, err
	}

	serviceNames := make([]string, 0, len(services))
	for i := range services {
		serviceNames = append(serviceNames, services[i].Name)
	}

	endpointIndex := map[string][]model.ServiceEndpoint{}
	if len(serviceNames) > 0 {
		endpointIndex = s.loadEndpointIndexForServices(listCtx, clientset, namespaceName, serviceNames)
	}

	serviceList := make([]model.K8sService, 0, len(services))
	for i := range services {
		svc := services[i]
		serviceList = append(serviceList, s.convertToK8sService(&svc, endpointIndex[svc.Name]))
	}

	return model.ServiceListResponse{
		Services:   serviceList,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: calculateTotalPages(total, pageSize),
	}, nil
}

func listServicePage(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string, page, pageSize int) ([]corev1.Service, int, error) {
	targetStart := (page - 1) * pageSize
	targetEnd := targetStart + pageSize
	var (
		continueToken string
		seen          int
		total         int
		pageItems     []corev1.Service
	)

	for {
		serviceList, err := clientset.CoreV1().Services(namespaceName).List(ctx, metav1.ListOptions{
			Limit:    int64(pageSize),
			Continue: continueToken,
		})
		if err != nil {
			return nil, 0, err
		}

		for _, item := range serviceList.Items {
			if seen >= targetStart && seen < targetEnd {
				pageItems = append(pageItems, item)
			}
			seen++
		}

		continueToken = serviceList.Continue
		if continueToken == "" {
			total = seen
			break
		}
		if seen >= targetEnd && serviceList.RemainingItemCount != nil {
			total = seen + int(*serviceList.RemainingItemCount)
			break
		}
	}

	if pageItems == nil {
		pageItems = []corev1.Service{}
	}

	return pageItems, total, nil
}

func filterServicesByName(services []model.K8sService, keyword string) []model.K8sService {
	lowerKeyword := strings.ToLower(strings.TrimSpace(keyword))
	if lowerKeyword == "" {
		return services
	}

	filtered := make([]model.K8sService, 0, len(services))
	for _, service := range services {
		if strings.Contains(strings.ToLower(service.Name), lowerKeyword) {
			filtered = append(filtered, service)
		}
	}

	return filtered
}

func paginateServiceListResponse(response model.ServiceListResponse, page, pageSize int) model.ServiceListResponse {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = len(response.Services)
	}

	total := len(response.Services)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	response.Services = response.Services[start:end]
	response.Total = total
	response.Page = page
	response.PageSize = pageSize
	response.TotalPages = calculateTotalPages(total, pageSize)
	return response
}

func (s *K8sServiceServiceImpl) loadEndpointIndexForServices(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string, serviceNames []string) map[string][]model.ServiceEndpoint {
	endpointIndex, err := listServiceEndpointsCompat(ctx, clientset, namespaceName, serviceNames)
	if err != nil {
		return map[string][]model.ServiceEndpoint{}
	}
	return endpointIndex
}

// GetServiceDetail 获取Service详情
func (s *K8sServiceServiceImpl) GetServiceDetail(c *gin.Context, clusterId uint, namespaceName string, serviceName string) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	svc, err := clientset.CoreV1().Services(namespaceName).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Service不存在: "+err.Error())
		return
	}

	// 获取Service详情
	endpoints := s.getServiceEndpoints(ctx, clientset, namespaceName, serviceName)
	k8sService := s.convertToK8sService(svc, endpoints)

	// 获取相关事件
	events, _ := s.getServiceEvents(ctx, clientset, namespaceName, serviceName)

	// 获取关联的Pod列表
	pods := s.getServicePods(ctx, clientset, namespaceName, svc.Spec.Selector)

	serviceDetail := model.ServiceDetail{
		K8sService: k8sService,
		Events:     events,
		Pods:       pods,
		Spec:       svc.Spec,
	}

	result.Success(c, serviceDetail)
}

// CreateService 创建Service
func (s *K8sServiceServiceImpl) CreateService(c *gin.Context, clusterId uint, namespaceName string, req *model.CreateServiceRequest) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	// 构建Service对象
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: namespaceName,
			Labels:    req.Labels,
		},
		Spec: corev1.ServiceSpec{
			Type:        corev1.ServiceType(req.Type),
			Selector:    req.Selector,
			ExternalIPs: req.ExternalIPs,
		},
	}

	// 设置端口配置
	var ports []corev1.ServicePort
	for _, portSpec := range req.Ports {
		port := corev1.ServicePort{
			Name:     portSpec.Name,
			Protocol: corev1.Protocol(portSpec.Protocol),
			Port:     portSpec.Port,
		}

		if portSpec.TargetPort != "" {
			port.TargetPort = intstr.FromString(portSpec.TargetPort)
		} else {
			port.TargetPort = intstr.FromInt(int(portSpec.Port))
		}

		if portSpec.NodePort > 0 && req.Type == "NodePort" {
			port.NodePort = portSpec.NodePort
		}

		ports = append(ports, port)
	}
	service.Spec.Ports = ports

	// 创建Service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	createdService, err := clientset.CoreV1().Services(namespaceName).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "创建Service失败: "+err.Error())
		return
	}

	invalidateCachedJSON(ctx, s.cacheService, servicesListCacheKey(clusterId, namespaceName))

	k8sService := s.convertToK8sService(createdService, s.getServiceEndpoints(ctx, clientset, namespaceName, createdService.Name))
	result.Success(c, k8sService)
}

// UpdateService 更新Service
func (s *K8sServiceServiceImpl) UpdateService(c *gin.Context, clusterId uint, namespaceName string, serviceName string, req *model.UpdateServiceRequest) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	// 获取现有Service
	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	existingService, err := clientset.CoreV1().Services(namespaceName).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Service不存在: "+err.Error())
		return
	}

	// 更新Service属性
	if req.Labels != nil {
		existingService.Labels = req.Labels
	}
	if req.Type != "" {
		existingService.Spec.Type = corev1.ServiceType(req.Type)
	}
	if req.Selector != nil {
		existingService.Spec.Selector = req.Selector
	}
	if req.ExternalIPs != nil {
		existingService.Spec.ExternalIPs = req.ExternalIPs
	}

	// 更新端口配置
	if req.Ports != nil {
		var ports []corev1.ServicePort
		for _, portSpec := range req.Ports {
			port := corev1.ServicePort{
				Name:     portSpec.Name,
				Protocol: corev1.Protocol(portSpec.Protocol),
				Port:     portSpec.Port,
			}

			if portSpec.TargetPort != "" {
				port.TargetPort = intstr.FromString(portSpec.TargetPort)
			} else {
				port.TargetPort = intstr.FromInt(int(portSpec.Port))
			}

			if portSpec.NodePort > 0 && req.Type == "NodePort" {
				port.NodePort = portSpec.NodePort
			}

			ports = append(ports, port)
		}
		existingService.Spec.Ports = ports
	}

	// 更新Service
	updatedService, err := clientset.CoreV1().Services(namespaceName).Update(ctx, existingService, metav1.UpdateOptions{})
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "更新Service失败: "+err.Error())
		return
	}

	invalidateCachedJSON(ctx, s.cacheService, servicesListCacheKey(clusterId, namespaceName))

	k8sService := s.convertToK8sService(updatedService, s.getServiceEndpoints(ctx, clientset, namespaceName, updatedService.Name))
	result.Success(c, k8sService)
}

// DeleteService 删除Service
func (s *K8sServiceServiceImpl) DeleteService(c *gin.Context, clusterId uint, namespaceName string, serviceName string) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	err = clientset.CoreV1().Services(namespaceName).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "删除Service失败: "+err.Error())
		return
	}

	invalidateCachedJSON(ctx, s.cacheService, servicesListCacheKey(clusterId, namespaceName))
	result.Success(c, "Service删除成功")
}

// GetServiceYaml 获取Service的YAML配置
func (s *K8sServiceServiceImpl) GetServiceYaml(c *gin.Context, clusterId uint, namespaceName string, serviceName string) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	svc, err := clientset.CoreV1().Services(namespaceName).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Service不存在: "+err.Error())
		return
	}

	// 清理不需要的字段，但保留 status
	svc.ManagedFields = nil

	// 确保 apiVersion 和 kind 字段存在
	if svc.APIVersion == "" {
		svc.APIVersion = "v1"
	}
	if svc.Kind == "" {
		svc.Kind = "Service"
	}

	// 清理元数据字段
	svc.ResourceVersion = ""
	svc.UID = ""
	svc.Generation = 0
	svc.CreationTimestamp = metav1.Time{}
	svc.SelfLink = ""

	yamlData, err := yaml.Marshal(svc)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "转换YAML失败: "+err.Error())
		return
	}

	result.Success(c, map[string]interface{}{
		"yaml": string(yamlData),
	})
}

// UpdateServiceYaml 通过YAML更新Service
func (s *K8sServiceServiceImpl) UpdateServiceYaml(c *gin.Context, clusterId uint, namespaceName string, serviceName string, yamlData map[string]interface{}) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	// 首先获取当前的 Service 以获取 resourceVersion 和 clusterIP
	existingService, err := clientset.CoreV1().Services(namespaceName).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Service不存在: "+err.Error())
		return
	}

	yamlBytes, err := yamlPayloadBytes(yamlData)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "YAML数据格式错误: "+err.Error())
		return
	}

	// 解析 YAML 到 Service 对象
	var service corev1.Service
	err = yaml.Unmarshal(yamlBytes, &service)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, "YAML解析失败: "+err.Error())
		return
	}

	// 确保名称和命名空间正确
	service.Name = serviceName
	service.Namespace = namespaceName

	// 保留 resourceVersion 和 clusterIP（Service 的 clusterIP 是不可变的）
	service.ResourceVersion = existingService.ResourceVersion
	service.Spec.ClusterIP = existingService.Spec.ClusterIP
	service.Spec.ClusterIPs = existingService.Spec.ClusterIPs

	// 更新Service
	_, err = clientset.CoreV1().Services(namespaceName).Update(ctx, &service, metav1.UpdateOptions{})
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "更新Service失败: "+err.Error())
		return
	}

	invalidateCachedJSON(ctx, s.cacheService, servicesListCacheKey(clusterId, namespaceName))
	result.Success(c, "Service YAML更新成功")
}

// GetServiceEvents 获取Service事件
func (s *K8sServiceServiceImpl) GetServiceEvents(c *gin.Context, clusterId uint, namespaceName string, serviceName string) {
	clientset, err := s.getK8sClient(clusterId)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	events, err := s.getServiceEvents(ctx, clientset, namespaceName, serviceName)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "获取Service事件失败: "+err.Error())
		return
	}

	result.Success(c, events)
}

// convertToK8sService 转换Service对象
func (s *K8sServiceServiceImpl) convertToK8sService(svc *corev1.Service, endpoints []model.ServiceEndpoint) model.K8sService {
	// 转换端口配置
	var ports []model.ServicePort
	for _, port := range svc.Spec.Ports {
		servicePort := model.ServicePort{
			Name:       port.Name,
			Protocol:   string(port.Protocol),
			Port:       port.Port,
			TargetPort: port.TargetPort.String(),
			NodePort:   port.NodePort,
		}
		ports = append(ports, servicePort)
	}

	// 确定状态
	status := "Active"
	if len(endpoints) == 0 {
		status = "No Endpoints"
	}

	return model.K8sService{
		Name:        svc.Name,
		Namespace:   svc.Namespace,
		Labels:      svc.Labels,
		Type:        string(svc.Spec.Type),
		Selector:    svc.Spec.Selector,
		ClusterIP:   svc.Spec.ClusterIP,
		ExternalIPs: svc.Spec.ExternalIPs,
		Ports:       ports,
		Endpoints:   endpoints,
		CreatedAt:   svc.CreationTimestamp.Format(time.RFC3339),
		Status:      status,
	}
}

// getServiceEndpoints 获取Service端点信息
func (s *K8sServiceServiceImpl) getServiceEndpoints(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string, serviceName string) []model.ServiceEndpoint {
	endpointIndex, err := listServiceEndpointsCompat(ctx, clientset, namespaceName, []string{serviceName})
	if err != nil {
		return nil
	}

	return endpointIndex[serviceName]
}

// getServiceEvents 获取Service相关事件
func (s *K8sServiceServiceImpl) getServiceEvents(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string, serviceName string) ([]model.K8sEvent, error) {
	events, err := clientset.CoreV1().Events(namespaceName).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Service", serviceName),
	})
	if err != nil {
		return nil, err
	}

	var k8sEvents []model.K8sEvent
	for _, event := range events.Items {
		k8sEvent := model.K8sEvent{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Source:    event.Source.Component,
			Count:     event.Count,
			FirstTime: event.FirstTimestamp.Format(time.RFC3339),
			LastTime:  event.LastTimestamp.Format(time.RFC3339),
		}
		k8sEvents = append(k8sEvents, k8sEvent)
	}

	return k8sEvents, nil
}

// getServicePods 获取Service关联的Pod列表
func (s *K8sServiceServiceImpl) getServicePods(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string, selector map[string]string) []model.K8sPodInfo {
	var pods []model.K8sPodInfo

	if len(selector) == 0 {
		return pods
	}

	// 构建标签选择器
	var selectorParts []string
	for key, value := range selector {
		selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", key, value))
	}
	labelSelector := fmt.Sprintf("%s", selectorParts[0])
	for i := 1; i < len(selectorParts); i++ {
		labelSelector += "," + selectorParts[i]
	}

	podList, err := clientset.CoreV1().Pods(namespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return pods
	}

	for _, pod := range podList.Items {
		podInfo := model.K8sPodInfo{
			Name:         pod.Name,
			Status:       string(pod.Status.Phase),
			Phase:        string(pod.Status.Phase),
			RestartCount: 0,
			NodeName:     pod.Spec.NodeName,
			PodIP:        pod.Status.PodIP,
			HostIP:       pod.Status.HostIP,
			CreatedAt:    pod.CreationTimestamp.Format(time.RFC3339),
			Labels:       pod.Labels,
		}

		// 计算重启次数
		for _, containerStatus := range pod.Status.ContainerStatuses {
			podInfo.RestartCount += containerStatus.RestartCount
		}

		// 计算运行时间
		if pod.Status.StartTime != nil {
			duration := time.Since(pod.Status.StartTime.Time)
			podInfo.RunningTime = duration.String()
		}

		pods = append(pods, podInfo)
	}

	return pods
}
