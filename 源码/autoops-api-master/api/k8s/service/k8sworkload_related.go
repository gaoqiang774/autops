package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"dodevops-api/api/k8s/model"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type podRelatedLists struct {
	services       *corev1.ServiceList
	endpoints      map[string][]model.ServiceEndpoint
	ingresses      *networkingv1.IngressList
	pvcs           *corev1.PersistentVolumeClaimList
	pvs            *corev1.PersistentVolumeList
	storageClasses *storagev1.StorageClassList
	configMaps     *corev1.ConfigMapList
	secrets        *corev1.SecretList
	warnings       []string
}

// GetPodRelatedResources 获取Pod关联的网络、存储和配置资源
func (s *K8sWorkloadServiceImpl) GetPodRelatedResources(c *gin.Context, clusterId uint, namespaceName string, podName string) {
	cluster, err := s.clusterDao.GetByID(clusterId)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, "获取集群信息失败: "+err.Error())
		return
	}

	clientset, err := s.createK8sClient(cluster.Credential)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "连接K8s集群失败: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 12*time.Second)
	defer cancel()

	pod, err := clientset.CoreV1().Pods(namespaceName).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		result.Failed(c, http.StatusNotFound, "Pod不存在: "+err.Error())
		return
	}

	lists := s.loadPodRelatedLists(ctx, clientset, namespaceName)
	response := model.PodRelatedResourcesResponse{
		Pod: model.PodRelatedPodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		},
		Network:  s.buildPodRelatedNetwork(pod, lists),
		Storage:  s.buildPodRelatedStorage(pod, lists),
		Config:   s.buildPodRelatedConfig(pod, lists),
		Warnings: lists.warnings,
	}

	result.Success(c, response)
}

func (s *K8sWorkloadServiceImpl) loadPodRelatedLists(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string) podRelatedLists {
	type listResult struct {
		kind string
		data interface{}
		err  error
	}

	ch := make(chan listResult, 8)

	go func() {
		items, err := clientset.CoreV1().Services(namespaceName).List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "services", data: items, err: err}
	}()
	go func() {
		items, err := listAllServiceEndpointsCompat(ctx, clientset, namespaceName)
		ch <- listResult{kind: "endpoints", data: items, err: err}
	}()
	go func() {
		items, err := clientset.NetworkingV1().Ingresses(namespaceName).List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "ingresses", data: items, err: err}
	}()
	go func() {
		items, err := clientset.CoreV1().PersistentVolumeClaims(namespaceName).List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "pvcs", data: items, err: err}
	}()
	go func() {
		items, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "pvs", data: items, err: err}
	}()
	go func() {
		items, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "storageClasses", data: items, err: err}
	}()
	go func() {
		items, err := clientset.CoreV1().ConfigMaps(namespaceName).List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "configMaps", data: items, err: err}
	}()
	go func() {
		items, err := clientset.CoreV1().Secrets(namespaceName).List(ctx, metav1.ListOptions{})
		ch <- listResult{kind: "secrets", data: items, err: err}
	}()

	lists := podRelatedLists{}
	for i := 0; i < 8; i++ {
		item := <-ch
		if item.err != nil {
			lists.warnings = append(lists.warnings, fmt.Sprintf("获取%s失败: %v", item.kind, item.err))
			continue
		}

		switch item.kind {
		case "services":
			lists.services = item.data.(*corev1.ServiceList)
		case "endpoints":
			lists.endpoints = item.data.(map[string][]model.ServiceEndpoint)
		case "ingresses":
			lists.ingresses = item.data.(*networkingv1.IngressList)
		case "pvcs":
			lists.pvcs = item.data.(*corev1.PersistentVolumeClaimList)
		case "pvs":
			lists.pvs = item.data.(*corev1.PersistentVolumeList)
		case "storageClasses":
			lists.storageClasses = item.data.(*storagev1.StorageClassList)
		case "configMaps":
			lists.configMaps = item.data.(*corev1.ConfigMapList)
		case "secrets":
			lists.secrets = item.data.(*corev1.SecretList)
		}
	}

	return lists
}

func (s *K8sWorkloadServiceImpl) buildPodRelatedNetwork(pod *corev1.Pod, lists podRelatedLists) model.PodRelatedNetwork {
	network := model.PodRelatedNetwork{
		Services:  []model.PodRelatedService{},
		Ingresses: []model.PodRelatedIngress{},
	}
	serviceSet := map[string]model.PodRelatedService{}
	if lists.services != nil {
		for i := range lists.services.Items {
			svc := &lists.services.Items[i]
			if !selectorMatchesPod(svc.Spec.Selector, pod.Labels) {
				continue
			}
			serviceSet[svc.Name] = model.PodRelatedService{
				K8sService:  convertCoreServiceToModel(svc, lists.endpoints[svc.Name]),
				MatchReason: "selector",
			}
		}
	}

	for _, svc := range serviceSet {
		network.Services = append(network.Services, svc)
	}

	if lists.ingresses != nil {
		for i := range lists.ingresses.Items {
			ing := &lists.ingresses.Items[i]
			if ingressRefsService(ing, serviceSet) {
				network.Ingresses = append(network.Ingresses, model.PodRelatedIngress{
					K8sIngress:  convertIngressToRelatedModel(ing),
					MatchReason: "backendService",
				})
			}
		}
	}

	return network
}

func (s *K8sWorkloadServiceImpl) buildPodRelatedStorage(pod *corev1.Pod, lists podRelatedLists) model.PodRelatedStorage {
	storage := model.PodRelatedStorage{
		PVCs:           []model.PodRelatedPVC{},
		PVs:            []model.K8sPersistentVolume{},
		StorageClasses: []model.K8sStorageClass{},
	}

	pvcUsage := collectPVCUsage(pod)
	if len(pvcUsage) == 0 {
		return storage
	}

	pvcNameSet := map[string]struct{}{}
	for name := range pvcUsage {
		pvcNameSet[name] = struct{}{}
	}

	pvNameSet := map[string]struct{}{}
	storageClassSet := map[string]struct{}{}
	if lists.pvcs != nil {
		for i := range lists.pvcs.Items {
			pvc := &lists.pvcs.Items[i]
			if _, ok := pvcNameSet[pvc.Name]; !ok {
				continue
			}
			storage.PVCs = append(storage.PVCs, model.PodRelatedPVC{
				K8sPersistentVolumeClaim: convertPVCToRelatedModel(pvc),
				Usage:                    pvcUsage[pvc.Name],
			})
			if pvc.Spec.VolumeName != "" {
				pvNameSet[pvc.Spec.VolumeName] = struct{}{}
			}
			if pvc.Spec.StorageClassName != nil && *pvc.Spec.StorageClassName != "" {
				storageClassSet[*pvc.Spec.StorageClassName] = struct{}{}
			}
		}
	}

	if lists.pvs != nil {
		for i := range lists.pvs.Items {
			pv := &lists.pvs.Items[i]
			if _, ok := pvNameSet[pv.Name]; !ok {
				if !pvClaimRefsAnyPVC(pv, pvcNameSet, pod.Namespace) {
					continue
				}
			}
			storage.PVs = append(storage.PVs, convertPVToRelatedModel(pv))
			if pv.Spec.StorageClassName != "" {
				storageClassSet[pv.Spec.StorageClassName] = struct{}{}
			}
		}
	}

	if lists.storageClasses != nil {
		for i := range lists.storageClasses.Items {
			sc := &lists.storageClasses.Items[i]
			if _, ok := storageClassSet[sc.Name]; ok {
				storage.StorageClasses = append(storage.StorageClasses, convertStorageClassToRelatedModel(sc))
			}
		}
	}

	return storage
}

func (s *K8sWorkloadServiceImpl) buildPodRelatedConfig(pod *corev1.Pod, lists podRelatedLists) model.PodRelatedConfig {
	config := model.PodRelatedConfig{
		ConfigMaps: []model.PodRelatedConfigMap{},
		Secrets:    []model.PodRelatedSecret{},
	}

	configMapUsage := collectConfigMapUsage(pod)
	secretUsage := collectSecretUsage(pod)

	if lists.configMaps != nil {
		for i := range lists.configMaps.Items {
			cm := &lists.configMaps.Items[i]
			usage, ok := configMapUsage[cm.Name]
			if !ok {
				continue
			}
			config.ConfigMaps = append(config.ConfigMaps, model.PodRelatedConfigMap{
				K8sConfigMap: convertConfigMapToRelatedModel(cm),
				Usage:        usage,
			})
		}
	}

	if lists.secrets != nil {
		for i := range lists.secrets.Items {
			secret := &lists.secrets.Items[i]
			usage, ok := secretUsage[secret.Name]
			if !ok {
				continue
			}
			config.Secrets = append(config.Secrets, model.PodRelatedSecret{
				K8sSecret: convertSecretToRelatedModel(secret),
				Usage:     usage,
			})
		}
	}

	return config
}

func selectorMatchesPod(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func convertCoreServiceToModel(svc *corev1.Service, endpoints []model.ServiceEndpoint) model.K8sService {
	ports := make([]model.ServicePort, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, model.ServicePort{
			Name:       port.Name,
			Protocol:   string(port.Protocol),
			Port:       port.Port,
			TargetPort: port.TargetPort.String(),
			NodePort:   port.NodePort,
		})
	}

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

func ingressRefsService(ing *networkingv1.Ingress, serviceSet map[string]model.PodRelatedService) bool {
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		if _, ok := serviceSet[ing.Spec.DefaultBackend.Service.Name]; ok {
			return true
		}
	}

	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service == nil {
				continue
			}
			if _, ok := serviceSet[path.Backend.Service.Name]; ok {
				return true
			}
		}
	}

	return false
}

func convertIngressToRelatedModel(ing *networkingv1.Ingress) model.K8sIngress {
	rules := make([]model.IngressRule, 0, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		paths := []model.IngressPath{}
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				relatedPath := model.IngressPath{
					Path: path.Path,
				}
				if path.PathType != nil {
					relatedPath.PathType = string(*path.PathType)
				}
				if path.Backend.Service != nil {
					relatedPath.Backend.Service.Name = path.Backend.Service.Name
					relatedPath.Backend.Service.Port.Number = path.Backend.Service.Port.Number
					relatedPath.Backend.Service.Port.Name = path.Backend.Service.Port.Name
				}
				paths = append(paths, relatedPath)
			}
		}
		rules = append(rules, model.IngressRule{
			Host: rule.Host,
			HTTP: model.IngressRuleValue{
				Paths: paths,
			},
		})
	}

	tlsItems := make([]model.IngressTLS, 0, len(ing.Spec.TLS))
	for _, tls := range ing.Spec.TLS {
		tlsItems = append(tlsItems, model.IngressTLS{
			Hosts:      tls.Hosts,
			SecretName: tls.SecretName,
		})
	}

	lb := model.IngressLoadBalancer{}
	for _, item := range ing.Status.LoadBalancer.Ingress {
		ports := make([]model.IngressLoadBalancerPort, 0, len(item.Ports))
		for _, port := range item.Ports {
			ports = append(ports, model.IngressLoadBalancerPort{
				Port:     port.Port,
				Protocol: string(port.Protocol),
			})
		}
		lb.Ingress = append(lb.Ingress, model.IngressLoadBalancerIngress{
			IP:       item.IP,
			Hostname: item.Hostname,
			Ports:    ports,
		})
	}

	className := ""
	if ing.Spec.IngressClassName != nil {
		className = *ing.Spec.IngressClassName
	}

	return model.K8sIngress{
		Name:         ing.Name,
		Namespace:    ing.Namespace,
		Labels:       ing.Labels,
		Class:        className,
		Rules:        rules,
		TLS:          tlsItems,
		LoadBalancer: lb,
		CreatedAt:    ing.CreationTimestamp.Format(time.RFC3339),
		Status:       ingressStatus(ing),
	}
}

func ingressStatus(ing *networkingv1.Ingress) string {
	if len(ing.Status.LoadBalancer.Ingress) > 0 {
		return "Active"
	}
	return "Pending"
}

func collectPVCUsage(pod *corev1.Pod) map[string][]model.PodResourceUsage {
	usage := map[string][]model.PodResourceUsage{}
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := volume.PersistentVolumeClaim.ClaimName
		usage[pvcName] = append(usage[pvcName], model.PodResourceUsage{
			Type:       "volume",
			VolumeName: volume.Name,
			ReadOnly:   volume.PersistentVolumeClaim.ReadOnly,
		})
		appendMountUsage(usage, pvcName, volume.Name, pod)
	}
	return usage
}

func collectConfigMapUsage(pod *corev1.Pod) map[string][]model.PodResourceUsage {
	usage := map[string][]model.PodResourceUsage{}
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			name := volume.ConfigMap.Name
			usage[name] = append(usage[name], model.PodResourceUsage{Type: "volume", VolumeName: volume.Name})
			appendMountUsage(usage, name, volume.Name, pod)
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.ConfigMap != nil {
					name := source.ConfigMap.Name
					usage[name] = append(usage[name], model.PodResourceUsage{Type: "projected", VolumeName: volume.Name})
					appendMountUsage(usage, name, volume.Name, pod)
				}
			}
		}
	}

	for _, container := range allPodContainers(pod) {
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				name := env.ValueFrom.ConfigMapKeyRef.Name
				usage[name] = append(usage[name], model.PodResourceUsage{
					Type:          "env",
					ContainerName: container.Name,
					EnvName:       env.Name,
					Key:           env.ValueFrom.ConfigMapKeyRef.Key,
				})
			}
		}
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				name := envFrom.ConfigMapRef.Name
				usage[name] = append(usage[name], model.PodResourceUsage{
					Type:          "envFrom",
					ContainerName: container.Name,
				})
			}
		}
	}

	return usage
}

func collectSecretUsage(pod *corev1.Pod) map[string][]model.PodResourceUsage {
	usage := map[string][]model.PodResourceUsage{}
	for _, volume := range pod.Spec.Volumes {
		if volume.Secret != nil {
			name := volume.Secret.SecretName
			usage[name] = append(usage[name], model.PodResourceUsage{Type: "volume", VolumeName: volume.Name})
			appendMountUsage(usage, name, volume.Name, pod)
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.Secret != nil {
					name := source.Secret.Name
					usage[name] = append(usage[name], model.PodResourceUsage{Type: "projected", VolumeName: volume.Name})
					appendMountUsage(usage, name, volume.Name, pod)
				}
			}
		}
	}

	for _, container := range allPodContainers(pod) {
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				name := env.ValueFrom.SecretKeyRef.Name
				usage[name] = append(usage[name], model.PodResourceUsage{
					Type:          "env",
					ContainerName: container.Name,
					EnvName:       env.Name,
					Key:           env.ValueFrom.SecretKeyRef.Key,
				})
			}
		}
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				name := envFrom.SecretRef.Name
				usage[name] = append(usage[name], model.PodResourceUsage{
					Type:          "envFrom",
					ContainerName: container.Name,
				})
			}
		}
	}

	for _, imagePullSecret := range pod.Spec.ImagePullSecrets {
		usage[imagePullSecret.Name] = append(usage[imagePullSecret.Name], model.PodResourceUsage{Type: "imagePullSecret"})
	}

	return usage
}

func appendMountUsage(usage map[string][]model.PodResourceUsage, resourceName, volumeName string, pod *corev1.Pod) {
	for _, container := range allPodContainers(pod) {
		for _, mount := range container.VolumeMounts {
			if mount.Name != volumeName {
				continue
			}
			usage[resourceName] = append(usage[resourceName], model.PodResourceUsage{
				Type:          "mount",
				ContainerName: container.Name,
				VolumeName:    volumeName,
				MountPath:     mount.MountPath,
				ReadOnly:      mount.ReadOnly,
			})
		}
	}
}

func allPodContainers(pod *corev1.Pod) []corev1.Container {
	containers := make([]corev1.Container, 0, len(pod.Spec.InitContainers)+len(pod.Spec.Containers)+len(pod.Spec.EphemeralContainers))
	containers = append(containers, pod.Spec.InitContainers...)
	containers = append(containers, pod.Spec.Containers...)
	for _, container := range pod.Spec.EphemeralContainers {
		containers = append(containers, corev1.Container{
			Name:         container.Name,
			Env:          container.Env,
			EnvFrom:      container.EnvFrom,
			VolumeMounts: container.VolumeMounts,
		})
	}
	return containers
}

func pvClaimRefsAnyPVC(pv *corev1.PersistentVolume, pvcNameSet map[string]struct{}, namespace string) bool {
	if pv.Spec.ClaimRef == nil || pv.Spec.ClaimRef.Namespace != namespace {
		return false
	}
	_, ok := pvcNameSet[pv.Spec.ClaimRef.Name]
	return ok
}

func convertPVCToRelatedModel(pvc *corev1.PersistentVolumeClaim) model.K8sPersistentVolumeClaim {
	capacity := ""
	if storage, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		capacity = storage.String()
	}
	accessModes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}
	storageClass := ""
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}
	volumeMode := ""
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}
	return model.K8sPersistentVolumeClaim{
		Name:         pvc.Name,
		Namespace:    pvc.Namespace,
		Labels:       pvc.Labels,
		Capacity:     capacity,
		AccessModes:  accessModes,
		Status:       string(pvc.Status.Phase),
		StorageClass: storageClass,
		VolumeMode:   volumeMode,
		VolumeName:   pvc.Spec.VolumeName,
		CreatedAt:    pvc.CreationTimestamp.Format(time.RFC3339),
	}
}

func convertPVToRelatedModel(pv *corev1.PersistentVolume) model.K8sPersistentVolume {
	capacity := ""
	if storage, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = storage.String()
	}
	accessModes := make([]string, 0, len(pv.Spec.AccessModes))
	for _, mode := range pv.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}
	volumeMode := ""
	if pv.Spec.VolumeMode != nil {
		volumeMode = string(*pv.Spec.VolumeMode)
	}
	var claimRef *model.PVClaimRef
	if pv.Spec.ClaimRef != nil {
		claimRef = &model.PVClaimRef{
			Kind:       pv.Spec.ClaimRef.Kind,
			Namespace:  pv.Spec.ClaimRef.Namespace,
			Name:       pv.Spec.ClaimRef.Name,
			UID:        string(pv.Spec.ClaimRef.UID),
			APIVersion: pv.Spec.ClaimRef.APIVersion,
		}
	}
	return model.K8sPersistentVolume{
		Name:          pv.Name,
		Labels:        pv.Labels,
		Capacity:      capacity,
		AccessModes:   accessModes,
		ReclaimPolicy: string(pv.Spec.PersistentVolumeReclaimPolicy),
		Status:        string(pv.Status.Phase),
		StorageClass:  pv.Spec.StorageClassName,
		VolumeMode:    volumeMode,
		ClaimRef:      claimRef,
		MountOptions:  pv.Spec.MountOptions,
		CreatedAt:     pv.CreationTimestamp.Format(time.RFC3339),
	}
}

func convertStorageClassToRelatedModel(sc *storagev1.StorageClass) model.K8sStorageClass {
	reclaimPolicy := ""
	if sc.ReclaimPolicy != nil {
		reclaimPolicy = string(*sc.ReclaimPolicy)
	}
	volumeBindingMode := ""
	if sc.VolumeBindingMode != nil {
		volumeBindingMode = string(*sc.VolumeBindingMode)
	}
	allowVolumeExpansion := false
	if sc.AllowVolumeExpansion != nil {
		allowVolumeExpansion = *sc.AllowVolumeExpansion
	}
	return model.K8sStorageClass{
		Name:                 sc.Name,
		Labels:               sc.Labels,
		Provisioner:          sc.Provisioner,
		Parameters:           sc.Parameters,
		ReclaimPolicy:        reclaimPolicy,
		VolumeBindingMode:    volumeBindingMode,
		AllowVolumeExpansion: allowVolumeExpansion,
		MountOptions:         sc.MountOptions,
		CreatedAt:            sc.CreationTimestamp.Format(time.RFC3339),
	}
}

func convertConfigMapToRelatedModel(cm *corev1.ConfigMap) model.K8sConfigMap {
	return model.K8sConfigMap{
		Name:        cm.Name,
		Namespace:   cm.Namespace,
		Labels:      cm.Labels,
		Data:        cm.Data,
		BinaryData:  cm.BinaryData,
		Immutable:   cm.Immutable != nil && *cm.Immutable,
		CreatedTime: cm.CreationTimestamp.Format(time.RFC3339),
	}
}

func convertSecretToRelatedModel(secret *corev1.Secret) model.K8sSecret {
	data := make(map[string][]byte, len(secret.Data))
	stringData := make(map[string]string, len(secret.Data))
	for key := range secret.Data {
		data[key] = []byte("***")
		stringData[key] = "***"
	}
	return model.K8sSecret{
		Name:        secret.Name,
		Namespace:   secret.Namespace,
		Labels:      secret.Labels,
		Type:        string(secret.Type),
		Data:        data,
		StringData:  stringData,
		Immutable:   secret.Immutable != nil && *secret.Immutable,
		CreatedTime: secret.CreationTimestamp.Format(time.RFC3339),
	}
}
