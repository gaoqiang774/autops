package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	alertmodel "dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	"dodevops-api/api/k8s/dao"
	"dodevops-api/api/k8s/model"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	clusterDetailTimeout           = 4 * time.Second
	clusterDetailSectionTimeout    = 2 * time.Second
	clusterDetailMonitoringTimeout = 3500 * time.Millisecond
	clusterDetailHistoryTimeout    = 800 * time.Millisecond

	metricsServerInstallURL     = "https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"
	metricsServerInstallCommand = "kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml"

	metricsScraperInstallURL     = "https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/"
	metricsScraperInstallCommand = "helm repo add kubernetes-dashboard https://kubernetes.github.io/dashboard/\nhelm repo update\nhelm upgrade --install kubernetes-dashboard kubernetes-dashboard/kubernetes-dashboard --create-namespace --namespace kubernetes-dashboard --set metricsScraper.enabled=true"
)

// IKubeClusterService K8s集群服务接口
type IKubeClusterService interface {
	CreateCluster(c *gin.Context, req *model.CreateKubeClusterRequest)
	GetCluster(c *gin.Context, id uint)
	GetClusterList(c *gin.Context, page, size int)
	UpdateCluster(c *gin.Context, id uint, req *model.UpdateKubeClusterRequest)
	DeleteCluster(c *gin.Context, id uint)
	GetClusterStatus(c *gin.Context, id uint)
	SyncCluster(c *gin.Context, id uint)
	GetClusterDetail(c *gin.Context, id uint)
	CheckMetricsComponents(c *gin.Context, id uint)
}

// KubeClusterServiceImpl K8s集群服务实现
type KubeClusterServiceImpl struct {
	dao *dao.KubeClusterDao
}

type logCollectTaskClusterReference struct {
	ID                 uint   `gorm:"column:id"`
	Name               string `gorm:"column:name"`
	CollectType        string `gorm:"column:collect_type"`
	K8SLogSource       string `gorm:"column:k8s_log_source"`
	ExecuteTargetsJSON string `gorm:"column:execute_targets_json"`
}

func NewKubeClusterService(db *gorm.DB) IKubeClusterService {
	return &KubeClusterServiceImpl{
		dao: dao.NewKubeClusterDao(db),
	}
}

func isK8SRootAdminFromContext(c *gin.Context) bool {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return false
	}
	return admin.ID == 1 || admin.Username == "admin" || admin.Username == "root"
}

func (s *KubeClusterServiceImpl) normalizeCreateMonitorDatasource(req *model.CreateKubeClusterRequest) error {
	if req.MonitorDatasourceID == 0 {
		req.MonitorDatasourceType = ""
		return nil
	}
	ds, err := s.resolveMonitorDatasource(req.MonitorDatasourceID, req.MonitorDatasourceType)
	if err != nil {
		return err
	}
	req.MonitorDatasourceID = ds.ID
	req.MonitorDatasourceType = ds.DatasourceType
	return nil
}

func (s *KubeClusterServiceImpl) resolveMonitorDatasource(id uint, datasourceType string) (*alertmodel.AlertDatasource, error) {
	if id == 0 {
		return nil, nil
	}

	var ds alertmodel.AlertDatasource
	if err := s.dao.DB.First(&ds, id).Error; err != nil {
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
	if strings.TrimSpace(datasourceType) != "" && datasourceType != ds.DatasourceType {
		return nil, fmt.Errorf("监控数据源类型不匹配")
	}
	if strings.TrimSpace(ds.URL) == "" {
		return nil, fmt.Errorf("监控数据源 URL 为空")
	}
	return &ds, nil
}

func isK8SPromQLDatasource(datasourceType string) bool {
	switch datasourceType {
	case alertmodel.AlertDatasourcePrometheus, alertmodel.AlertDatasourceVictoriaMetrics, alertmodel.AlertDatasourceBuiltin:
		return true
	default:
		return false
	}
}

func (s *KubeClusterServiceImpl) fillMonitorDatasourceName(cluster *model.KubeCluster) {
	if cluster == nil || cluster.MonitorDatasourceID == 0 {
		return
	}

	var ds alertmodel.AlertDatasource
	if err := s.dao.DB.Select("id", "name", "datasource_type").First(&ds, cluster.MonitorDatasourceID).Error; err != nil {
		return
	}
	cluster.MonitorDatasourceName = ds.Name
	if cluster.MonitorDatasourceType == "" {
		cluster.MonitorDatasourceType = ds.DatasourceType
	}
}

func (s *KubeClusterServiceImpl) fillMonitorDatasourceNames(clusters []model.KubeCluster) {
	ids := make([]uint, 0)
	seen := make(map[uint]struct{})
	for i := range clusters {
		id := clusters[i].MonitorDatasourceID
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}

	var datasources []alertmodel.AlertDatasource
	if err := s.dao.DB.Select("id", "name", "datasource_type").Where("id IN ?", ids).Find(&datasources).Error; err != nil {
		return
	}
	byID := make(map[uint]alertmodel.AlertDatasource, len(datasources))
	for _, ds := range datasources {
		byID[ds.ID] = ds
	}
	for i := range clusters {
		if ds, ok := byID[clusters[i].MonitorDatasourceID]; ok {
			clusters[i].MonitorDatasourceName = ds.Name
			if clusters[i].MonitorDatasourceType == "" {
				clusters[i].MonitorDatasourceType = ds.DatasourceType
			}
		}
	}
}

// CreateCluster 创建K8s集群
func (s *KubeClusterServiceImpl) CreateCluster(c *gin.Context, req *model.CreateKubeClusterRequest) {
	if err := s.normalizeCreateMonitorDatasource(req); err != nil {
		result.Failed(c, http.StatusBadRequest, err.Error())
		return
	}

	// 检查集群名称是否已存在
	exists, err := s.dao.IsClusterNameExists(req.Name)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("检查集群名称失败: %v", err))
		return
	}
	if exists {
		result.Failed(c, http.StatusBadRequest, "集群名称已存在")
		return
	}

	// 根据集群类型处理不同逻辑
	// 默认为自建集群
	if req.ClusterType == 0 {
		req.ClusterType = model.ClusterTypeSelfBuilt
	}

	switch req.ClusterType {
	case model.ClusterTypeSelfBuilt:
		s.createSelfBuiltCluster(c, req)
	case model.ClusterTypeImported:
		s.importCluster(c, req)
	default:
		result.Failed(c, http.StatusBadRequest, "无效的集群类型")
	}
}

// GetCluster 获取集群详情
func (s *KubeClusterServiceImpl) GetCluster(c *gin.Context, id uint) {
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}
	s.fillMonitorDatasourceName(cluster)

	// 获取集群节点信息（通过K8s API）
	nodes, nodeErr := s.getClusterNodes(cluster)
	if nodeErr != nil {
		// 如果获取节点失败，仍然返回集群基本信息，但标记节点获取失败
		nodes = []model.NodeInfo{}
	}

	// 计算集群概要信息
	summary := s.calculateClusterSummary(nodes)

	response := model.ClusterDetailResponse{
		Cluster: *cluster,
		Nodes:   nodes,
		Summary: summary,
	}

	result.Success(c, response)
}

// GetClusterList 获取集群列表
func (s *KubeClusterServiceImpl) GetClusterList(c *gin.Context, page, size int) {
	var clusters []model.KubeCluster
	var total int64
	var err error

	if isK8SRootAdminFromContext(c) {
		clusters, total, err = s.dao.List(page, size)
	} else {
		admin, adminErr := jwt.GetAdmin(c)
		if adminErr != nil || admin == nil {
			result.Failed(c, http.StatusUnauthorized, "用户未登录")
			return
		}
		authDao := cmdbdao.NewBastionAssetAuthorizationDao()
		authorizedIDs, authErr := authDao.GetUserAuthorizedAssetIDs(admin.ID, cmdbmodel.AssetTypeK8SCluster)
		if authErr != nil {
			result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取K8S授权集群失败: %v", authErr))
			return
		}
		clusters, total, err = s.dao.ListByIDs(authorizedIDs, page, size)
	}
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群列表失败: %v", err))
		return
	}
	s.fillMonitorDatasourceNames(clusters)

	response := model.KubeClusterListResponse{
		List:  clusters,
		Total: total,
	}

	result.Success(c, response)
}

// UpdateCluster 更新集群信息
func (s *KubeClusterServiceImpl) UpdateCluster(c *gin.Context, id uint, req *model.UpdateKubeClusterRequest) {
	// 检查集群是否存在
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}

	// 检查名称是否被其他集群使用
	if req.Name != "" {
		existing, err := s.dao.GetByName(req.Name)
		if err == nil && existing.ID != id {
			result.Failed(c, http.StatusBadRequest, "集群名称已存在")
			return
		}
	}

	// 构建更新字段
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Credential != "" {
		updates["credential"] = req.Credential
	}
	if req.Version != "" {
		updates["version"] = req.Version
	}
	if req.MonitorDatasourceID != nil {
		if *req.MonitorDatasourceID == 0 {
			updates["monitor_datasource_id"] = 0
			updates["monitor_datasource_type"] = ""
		} else {
			ds, err := s.resolveMonitorDatasource(*req.MonitorDatasourceID, req.MonitorDatasourceType)
			if err != nil {
				result.Failed(c, http.StatusBadRequest, err.Error())
				return
			}
			updates["monitor_datasource_id"] = ds.ID
			updates["monitor_datasource_type"] = ds.DatasourceType
		}
	} else if req.MonitorDatasourceType != "" && cluster.MonitorDatasourceID > 0 {
		ds, err := s.resolveMonitorDatasource(cluster.MonitorDatasourceID, req.MonitorDatasourceType)
		if err != nil {
			result.Failed(c, http.StatusBadRequest, err.Error())
			return
		}
		updates["monitor_datasource_type"] = ds.DatasourceType
	}

	// 如果有更新字段，执行更新
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		if err := s.dao.Update(id, updates); err != nil {
			result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("更新集群失败: %v", err))
			return
		}
	}

	// 返回更新后的集群信息
	updatedCluster, err := s.dao.GetByID(id)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取更新后集群信息失败: %v", err))
		return
	}
	s.fillMonitorDatasourceName(updatedCluster)

	result.Success(c, updatedCluster)
}

// DeleteCluster 删除集群
func (s *KubeClusterServiceImpl) DeleteCluster(c *gin.Context, id uint) {
	// 检查集群是否存在
	_, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}

	references, err := s.findLogCollectTaskClusterReferences(id)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("检查日志采集任务引用失败: %v", err))
		return
	}
	if len(references) > 0 {
		result.Failed(c, http.StatusConflict, formatLogCollectTaskClusterReferenceMessage(references))
		return
	}

	if err := s.dao.Delete(id); err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("删除集群失败: %v", err))
		return
	}

	result.Success(c, "删除成功")
}

func (s *KubeClusterServiceImpl) findLogCollectTaskClusterReferences(clusterID uint) ([]logCollectTaskClusterReference, error) {
	var tasks []logCollectTaskClusterReference
	err := s.dao.DB.Table("log_collect_task").
		Select("id, name, collect_type, k8s_log_source, execute_targets_json").
		Where("execute_targets_json IS NOT NULL AND execute_targets_json <> ''").
		Scan(&tasks).Error
	if err != nil {
		return nil, err
	}

	references := make([]logCollectTaskClusterReference, 0)
	for _, task := range tasks {
		referenced, parseErr := logCollectTaskReferencesCluster(task.ExecuteTargetsJSON, clusterID)
		if parseErr != nil {
			if isKubernetesLogCollectTask(task) {
				return nil, fmt.Errorf("日志采集任务 %s(ID:%d) 的目标配置格式错误: %w", task.Name, task.ID, parseErr)
			}
			continue
		}
		if referenced {
			references = append(references, task)
		}
	}
	return references, nil
}

func logCollectTaskReferencesCluster(raw string, clusterID uint) (bool, error) {
	var targets []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &targets); err != nil {
		return false, err
	}
	for _, target := range targets {
		for _, key := range []string{"clusterId", "clusterID"} {
			value, exists := target[key]
			if !exists {
				continue
			}
			switch typed := value.(type) {
			case float64:
				if typed > 0 && typed == math.Trunc(typed) && uint64(typed) == uint64(clusterID) {
					return true, nil
				}
			case string:
				var parsed uint64
				if _, err := fmt.Sscan(strings.TrimSpace(typed), &parsed); err == nil && parsed == uint64(clusterID) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func isKubernetesLogCollectTask(task logCollectTaskClusterReference) bool {
	if strings.TrimSpace(task.K8SLogSource) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(task.CollectType)) {
	case "kubernetes", "k8s", "kubernetes_workload", "k8s_workload":
		return true
	default:
		return false
	}
}

func formatLogCollectTaskClusterReferenceMessage(references []logCollectTaskClusterReference) string {
	const displayLimit = 5
	items := make([]string, 0, displayLimit)
	for index, reference := range references {
		if index >= displayLimit {
			break
		}
		items = append(items, fmt.Sprintf("%s(ID:%d)", reference.Name, reference.ID))
	}
	message := "该K8S集群正在被日志采集任务引用，禁止删除。请先解除或迁移任务: " + strings.Join(items, "、")
	if len(references) > displayLimit {
		message += fmt.Sprintf("，另有%d个任务", len(references)-displayLimit)
	}
	return message
}

// GetClusterStatus 获取集群状态
func (s *KubeClusterServiceImpl) GetClusterStatus(c *gin.Context, id uint) {
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}

	// 获取节点信息
	nodes, _ := s.getClusterNodes(cluster)
	summary := s.calculateClusterSummary(nodes)

	statusInfo := map[string]interface{}{
		"id":          cluster.ID,
		"name":        cluster.Name,
		"status":      cluster.Status,
		"statusText":  cluster.GetStatusText(),
		"clusterType": cluster.ClusterType,
		"summary":     summary,
		"nodes":       nodes,
	}

	result.Success(c, statusInfo)
}

// createSelfBuiltCluster 创建自建集群
func (s *KubeClusterServiceImpl) createSelfBuiltCluster(c *gin.Context, req *model.CreateKubeClusterRequest) {
	// 验证必要参数
	if req.NodeConfig == nil {
		result.Failed(c, http.StatusBadRequest, "节点配置不能为空")
		return
	}
	if req.Version == "" {
		result.Failed(c, http.StatusBadRequest, "集群版本不能为空")
		return
	}

	// 验证主机ID并获取主机信息
	_, err := s.getHostInfos(*req.NodeConfig)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, fmt.Sprintf("获取主机信息失败: %v", err))
		return
	}

	// 创建集群记录
	cluster := &model.KubeCluster{
		Name:                  req.Name,
		Version:               req.Version,
		Status:                model.ClusterStatusCreating,
		Description:           req.Description,
		ClusterType:           model.ClusterTypeSelfBuilt,
		MonitorDatasourceID:   req.MonitorDatasourceID,
		MonitorDatasourceType: req.MonitorDatasourceType,
	}

	if err := s.dao.Create(cluster); err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("创建集群失败: %v", err))
		return
	}

	result.Success(c, cluster)
}

// importCluster 导入集群
func (s *KubeClusterServiceImpl) importCluster(c *gin.Context, req *model.CreateKubeClusterRequest) {
	// 验证必要参数
	if req.Kubeconfig == "" {
		result.Failed(c, http.StatusBadRequest, "kubeconfig不能为空")
		return
	}

	// 验证kubeconfig有效性
	if err := s.validateKubeconfig(req.Kubeconfig); err != nil {
		result.Failed(c, http.StatusBadRequest, fmt.Sprintf("kubeconfig验证失败: %v", err))
		return
	}

	// 从kubeconfig获取集群版本信息
	version := req.Version
	if version == "" {
		version = "unknown" // 如果没有提供版本，从kubeconfig中解析或标记为未知
	}

	// 创建集群记录
	cluster := &model.KubeCluster{
		Name:                  req.Name,
		Version:               version,
		Status:                model.ClusterStatusRunning, // 导入的集群默认为运行中
		Description:           req.Description,
		ClusterType:           model.ClusterTypeImported,
		Credential:            req.Kubeconfig,
		MonitorDatasourceID:   req.MonitorDatasourceID,
		MonitorDatasourceType: req.MonitorDatasourceType,
	}

	if err := s.dao.Create(cluster); err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("导入集群失败: %v", err))
		return
	}

	// 自动同步集群信息
	go func() {
		// 在goroutine中进行同步，避免阻塞响应
		if err := s.syncClusterInternal(cluster.ID); err != nil {
			// 记录同步失败日志，但不影响导入成功的响应
			fmt.Printf("集群[%d]自动同步失败: %v\n", cluster.ID, err)
		} else {
			fmt.Printf("集群[%d]自动同步成功\n", cluster.ID)
		}
	}()

	result.Success(c, cluster)
}

// getHostInfos 获取主机信息
func (s *KubeClusterServiceImpl) getHostInfos(nodeConfig model.NodeConfig) (map[uint]cmdbmodel.CmdbHost, error) {
	// 收集所有主机ID
	allHostIDs := make(map[uint]bool)
	for _, id := range nodeConfig.MasterHostIDs {
		allHostIDs[id] = true
	}
	for _, id := range nodeConfig.WorkerHostIDs {
		allHostIDs[id] = true
	}
	for _, id := range nodeConfig.EtcdHostIDs {
		allHostIDs[id] = true
	}

	// 转换为切片
	var hostIDs []uint
	for id := range allHostIDs {
		hostIDs = append(hostIDs, id)
	}

	// 查询主机信息
	var hosts []cmdbmodel.CmdbHost
	if err := s.dao.DB.Where("id IN ?", hostIDs).Find(&hosts).Error; err != nil {
		return nil, fmt.Errorf("查询主机信息失败: %v", err)
	}

	// 构建主机信息映射
	hostMap := make(map[uint]cmdbmodel.CmdbHost)
	for _, host := range hosts {
		hostMap[host.ID] = host
	}

	// 验证所有主机是否都存在
	for _, id := range hostIDs {
		if _, exists := hostMap[id]; !exists {
			return nil, fmt.Errorf("主机ID %d 不存在", id)
		}
	}

	return hostMap, nil
}

// getClusterNodes 通过K8s API获取集群节点信息
func (s *KubeClusterServiceImpl) getClusterNodes(cluster *model.KubeCluster) ([]model.NodeInfo, error) {
	// TODO: 实现K8s API调用获取节点信息
	// 这里暂时返回空切片，实际应该调用K8s API

	// 示例代码：
	// k8sClient := NewK8sClient(cluster.Credential)
	// nodes, err := k8sClient.GetNodes()
	// if err != nil {
	//     return nil, err
	// }
	// return convertToNodeInfo(nodes), nil

	return []model.NodeInfo{}, nil
}

// calculateClusterSummary 计算集群概要信息
func (s *KubeClusterServiceImpl) calculateClusterSummary(nodes []model.NodeInfo) model.ClusterSummary {
	summary := model.ClusterSummary{
		TotalNodes:  len(nodes),
		ReadyNodes:  0,
		MasterNodes: 0,
		WorkerNodes: 0,
	}

	for _, node := range nodes {
		if node.Status == "Ready" {
			summary.ReadyNodes++
		}
		if node.Role == "master" {
			summary.MasterNodes++
		} else if node.Role == "worker" {
			summary.WorkerNodes++
		}
	}

	return summary
}

// SyncCluster 同步集群信息
func (s *KubeClusterServiceImpl) SyncCluster(c *gin.Context, id uint) {
	// 获取集群信息
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
		} else {
			result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("查询集群失败: %v", err))
		}
		return
	}

	// 检查集群是否有kubeconfig凭证，有凭证的集群都可以同步
	if cluster.Credential == "" {
		result.Failed(c, http.StatusBadRequest, "集群缺少kubeconfig配置，无法同步")
		return
	}

	// 通过K8s API同步集群信息
	syncData, err := s.syncFromK8sAPI(cluster.Credential)
	if err != nil {
		// 同步失败时，判断是否为超时错误，将集群状态设为离线
		now := time.Now()
		updates := map[string]interface{}{
			"status":       model.ClusterStatusStopped, // 3-离线
			"last_sync_at": &now,
			"updated_at":   now,
		}

		// 更新集群状态为离线
		if updateErr := s.dao.Update(id, updates); updateErr != nil {
			result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("同步失败且更新状态失败: %v, %v", err, updateErr))
			return
		}

		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("同步集群信息失败，已将集群状态设为离线: %v", err))
		return
	}

	// 更新集群信息到数据库
	now := time.Now()
	updates := map[string]interface{}{
		"version":      syncData.Version,
		"status":       syncData.Status,
		"node_count":   syncData.NodeCount,
		"ready_nodes":  syncData.ReadyNodes,
		"master_nodes": syncData.MasterNodes,
		"worker_nodes": syncData.WorkerNodes,
		"last_sync_at": &now,
		"updated_at":   now,
	}

	if err := s.dao.Update(id, updates); err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("更新集群信息失败: %v", err))
		return
	}

	// 重新查询更新后的集群信息
	updatedCluster, err := s.dao.GetByID(id)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("查询更新后的集群失败: %v", err))
		return
	}

	result.Success(c, updatedCluster)
}

// K8sSyncData K8s同步数据结构
type K8sSyncData struct {
	Version     string
	Status      int
	NodeCount   int
	ReadyNodes  int
	MasterNodes int
	WorkerNodes int
}

// syncFromK8sAPI 从K8s API同步集群信息
func (s *KubeClusterServiceImpl) syncFromK8sAPI(kubeconfig string) (*K8sSyncData, error) {
	// 解析kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("解析kubeconfig失败: %v", err)
	}

	// 设置超时配置
	config.Timeout = 5 * time.Second

	// 创建K8s客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("创建K8s客户端失败: %v", err)
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 获取集群版本信息
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("获取集群版本失败: %v", err)
	}

	// 获取节点列表
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取节点列表失败: %v", err)
	}

	// 统计节点信息
	var readyNodes, masterNodes, workerNodes int
	for _, node := range nodes.Items {
		// 统计就绪节点
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				readyNodes++
				break
			}
		}

		// 统计节点类型
		if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
			masterNodes++
		} else if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
			masterNodes++
		} else {
			workerNodes++
		}
	}

	// 确定集群状态
	status := model.ClusterStatusRunning
	if readyNodes == 0 {
		status = model.ClusterStatusStopped // 离线
	} else if readyNodes < len(nodes.Items) {
		status = model.ClusterStatusRunning // 部分节点不可用但仍保持运行状态
	}

	return &K8sSyncData{
		Version:     versionInfo.GitVersion,
		Status:      status,
		NodeCount:   len(nodes.Items),
		ReadyNodes:  readyNodes,
		MasterNodes: masterNodes,
		WorkerNodes: workerNodes,
	}, nil
}

// syncClusterInternal 内部同步集群信息（不依赖gin.Context）
func (s *KubeClusterServiceImpl) syncClusterInternal(clusterID uint) error {
	// 获取集群信息
	cluster, err := s.dao.GetByID(clusterID)
	if err != nil {
		return fmt.Errorf("查询集群失败: %v", err)
	}

	// 检查集群是否有kubeconfig凭证
	if cluster.Credential == "" {
		return fmt.Errorf("集群缺少kubeconfig配置，无法同步")
	}

	// 通过K8s API同步集群信息
	syncData, err := s.syncFromK8sAPI(cluster.Credential)
	if err != nil {
		// 同步失败时，将集群状态设为离线
		now := time.Now()
		updates := map[string]interface{}{
			"status":       model.ClusterStatusStopped, // 3-离线
			"last_sync_at": &now,
		}

		// 更新集群状态为离线
		if updateErr := s.dao.Update(clusterID, updates); updateErr != nil {
			return fmt.Errorf("同步失败且更新状态失败: %v, %v", err, updateErr)
		}

		return fmt.Errorf("同步集群信息失败，已将集群状态设为离线: %v", err)
	}

	// 更新集群信息到数据库
	now := time.Now()
	updates := map[string]interface{}{
		"version":      syncData.Version,
		"node_count":   syncData.NodeCount,
		"ready_nodes":  syncData.ReadyNodes,
		"master_nodes": syncData.MasterNodes,
		"worker_nodes": syncData.WorkerNodes,
		"status":       syncData.Status,
		"last_sync_at": &now,
	}

	return s.dao.Update(clusterID, updates)
}

// validateKubeconfig 验证kubeconfig有效性
func (s *KubeClusterServiceImpl) validateKubeconfig(kubeconfig string) error {
	// 解析kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return fmt.Errorf("kubeconfig格式错误: %v", err)
	}

	// 创建K8s客户端测试连接
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("创建K8s客户端失败: %v", err)
	}

	// 测试连接 - 尝试获取集群版本信息
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("连接K8s集群失败: %v", err)
	}

	// 测试节点访问权限
	_, err = clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("缺少节点访问权限: %v", err)
	}

	return nil
}

// GetClusterDetail 获取集群详细信息
func (s *KubeClusterServiceImpl) GetClusterDetail(c *gin.Context, id uint) {
	// 获取基本集群信息
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}

	// 检查集群是否有kubeconfig凭证
	if cluster.Credential == "" {
		result.Failed(c, http.StatusBadRequest, "集群缺少kubeconfig配置，无法获取详细信息")
		return
	}

	// 获取详细信息
	detail, err := s.getClusterDetailInfo(c.Request.Context(), cluster)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群详细信息失败: %v", err))
		return
	}

	result.Success(c, detail)
}

// CheckMetricsComponents checks metrics-server and metrics-scraper installation status.
func (s *KubeClusterServiceImpl) CheckMetricsComponents(c *gin.Context, id uint) {
	cluster, err := s.dao.GetByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			result.Failed(c, http.StatusNotFound, "集群不存在")
			return
		}
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("获取集群失败: %v", err))
		return
	}
	if cluster.Credential == "" {
		result.Failed(c, http.StatusBadRequest, "集群缺少 kubeconfig 配置，无法检查组件")
		return
	}

	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("解析 kubeconfig 失败: %v", err))
		return
	}
	config.Timeout = 4 * time.Second

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, fmt.Sprintf("创建 K8s 客户端失败: %v", err))
		return
	}
	metricsClient, _ := metricsclientset.NewForConfig(config)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var metricsServerStatus, metricsScraperStatus model.MetricsComponentStatus
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		metricsServerStatus = s.checkMetricsServer(ctx, clientset, metricsClient)
	}()
	go func() {
		defer wg.Done()
		metricsScraperStatus = s.checkMetricsScraper(ctx, clientset)
	}()
	wg.Wait()

	response := model.MetricsComponentCheckResponse{
		ClusterID:    cluster.ID,
		AllInstalled: metricsServerStatus.Installed && metricsScraperStatus.Installed,
		Components: []model.MetricsComponentStatus{
			metricsServerStatus,
			metricsScraperStatus,
		},
	}
	result.Success(c, response)
}

func (s *KubeClusterServiceImpl) checkMetricsServer(ctx context.Context, clientset *kubernetes.Clientset, metricsClient *metricsclientset.Clientset) model.MetricsComponentStatus {
	status := model.MetricsComponentStatus{
		Name:           "metrics-server",
		DisplayName:    "metrics-server",
		Description:    "用于提供 Kubernetes CPU/内存实时度量信息",
		InstallURL:     metricsServerInstallURL,
		InstallCommand: metricsServerInstallCommand,
		Message:        "未检测到 metrics-server，请安装后再查看实时 CPU/内存指标",
	}

	if deploy, err := clientset.AppsV1().Deployments("kube-system").Get(ctx, "metrics-server", metav1.GetOptions{}); err == nil {
		status.Installed = true
		status.Namespace = deploy.Namespace
		status.ResourceName = deploy.Name
		if deploy.Status.AvailableReplicas > 0 {
			status.Available = true
			status.Message = "metrics-server Deployment 已安装且可用"
		} else {
			status.Message = "metrics-server Deployment 已安装，但暂无可用副本"
		}
	}

	if metricsClient != nil {
		if _, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
			status.Installed = true
			status.Available = true
			if status.Namespace == "" {
				status.Namespace = "kube-system"
			}
			if status.ResourceName == "" {
				status.ResourceName = "metrics-server"
			}
			status.Message = "metrics-server Metrics API 可用"
		} else if status.Installed {
			status.Message = fmt.Sprintf("metrics-server 已安装，但 Metrics API 暂不可用: %v", err)
		}
	}

	return status
}

func (s *KubeClusterServiceImpl) checkMetricsScraper(ctx context.Context, clientset *kubernetes.Clientset) model.MetricsComponentStatus {
	status := model.MetricsComponentStatus{
		Name:           "metrics-scraper",
		DisplayName:    "metrics-scraper",
		Description:    "用于存储最近一段时间内的 CPU/内存度量信息，以便展示 CPU/内存历史曲线",
		InstallURL:     metricsScraperInstallURL,
		InstallCommand: metricsScraperInstallCommand,
		Message:        "未检测到 metrics-scraper，请安装 Kubernetes Dashboard/Kuboard 对应的 metrics-scraper 组件",
	}

	for _, svc := range getMetricsScraperServiceCandidates(ctx, clientset) {
		service, err := clientset.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		status.Installed = true
		status.Available = true
		status.Namespace = service.Namespace
		status.ResourceName = service.Name
		status.Message = "metrics-scraper Service 已安装"
		return status
	}

	return status
}

// getClusterDetailInfo 获取集群详细信息
func (s *KubeClusterServiceImpl) getClusterDetailInfo(parentCtx context.Context, cluster *model.KubeCluster) (*model.ClusterDetailResponse, error) {
	// 解析kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(cluster.Credential))
	if err != nil {
		return nil, fmt.Errorf("解析kubeconfig失败: %v", err)
	}

	// 设置超时
	config.Timeout = clusterDetailTimeout

	// 创建K8s客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("创建K8s客户端失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(parentCtx, clusterDetailTimeout)
	defer cancel()

	// 并行获取各种信息
	metricsClient, _ := metricsclientset.NewForConfig(config)

	var nodes []model.NodeInfo
	var components []model.ComponentInfo
	var workloads model.WorkloadSummary
	var network model.NetworkInfo
	var events []model.ClusterEvent
	var monitoring model.MonitoringInfo
	var runtime model.RuntimeSummary

	var wg sync.WaitGroup
	wg.Add(7)

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		nodes, _ = s.getDetailedNodes(sectionCtx, clientset)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		components, _ = s.getClusterComponents(sectionCtx, clientset)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		workloads, _ = s.getWorkloadSummary(sectionCtx, clientset)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		network, _ = s.getNetworkInfo(sectionCtx, clientset)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		events, _ = s.getClusterEvents(sectionCtx, clientset)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailMonitoringTimeout)
		defer sectionCancel()
		monitoring, _ = s.getMonitoringInfo(sectionCtx, clientset, metricsClient)
	}()

	go func() {
		defer finishClusterDetailSection(&wg)
		sectionCtx, sectionCancel := context.WithTimeout(ctx, clusterDetailSectionTimeout)
		defer sectionCancel()
		runtime, _ = s.getRuntimeInfo(sectionCtx, clientset)
	}()

	wg.Wait()

	summary := s.calculateClusterSummary(nodes)

	return &model.ClusterDetailResponse{
		Cluster:    *cluster,
		Nodes:      nodes,
		Summary:    summary,
		Components: components,
		Workloads:  workloads,
		Network:    network,
		Events:     events,
		Monitoring: monitoring,
		Runtime:    runtime,
	}, nil
}

func finishClusterDetailSection(wg *sync.WaitGroup) {
	if r := recover(); r != nil {
		// 单个详情区块失败不应导致整个集群详情接口不可用。
	}
	wg.Done()
}

// getDetailedNodes 获取详细的节点信息
func (s *KubeClusterServiceImpl) getDetailedNodes(ctx context.Context, clientset *kubernetes.Clientset) ([]model.NodeInfo, error) {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodeInfos []model.NodeInfo
	for _, node := range nodes.Items {
		nodeInfo := model.NodeInfo{
			Name:        node.Name,
			InternalIP:  "",
			ExternalIP:  "",
			Status:      "NotReady",
			Version:     node.Status.NodeInfo.KubeletVersion,
			OS:          node.Status.NodeInfo.OSImage,
			CreatedAt:   node.CreationTimestamp.Format(time.RFC3339),
			Capacity:    make(map[string]string),
			Allocatable: make(map[string]string),
			Conditions:  make([]model.NodeCondition, 0),
		}

		// 获取节点IP地址
		for _, addr := range node.Status.Addresses {
			if addr.Type == "InternalIP" {
				nodeInfo.InternalIP = addr.Address
			} else if addr.Type == "ExternalIP" {
				nodeInfo.ExternalIP = addr.Address
			}
		}

		// 获取节点状态
		for _, condition := range node.Status.Conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				nodeInfo.Status = "Ready"
			}
			nodeInfo.Conditions = append(nodeInfo.Conditions, model.NodeCondition{
				Type:   string(condition.Type),
				Status: string(condition.Status),
				Reason: condition.Reason,
			})
		}

		// 获取节点角色
		if _, exists := node.Labels["node-role.kubernetes.io/master"]; exists {
			nodeInfo.Role = "master"
		} else if _, exists := node.Labels["node-role.kubernetes.io/control-plane"]; exists {
			nodeInfo.Role = "master"
		} else {
			nodeInfo.Role = "worker"
		}

		// 获取资源信息
		for resourceName, quantity := range node.Status.Capacity {
			nodeInfo.Capacity[string(resourceName)] = quantity.String()
		}
		for resourceName, quantity := range node.Status.Allocatable {
			nodeInfo.Allocatable[string(resourceName)] = quantity.String()
		}

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos, nil
}

// getClusterComponents 获取集群组件信息
func (s *KubeClusterServiceImpl) getClusterComponents(ctx context.Context, clientset *kubernetes.Clientset) ([]model.ComponentInfo, error) {
	var components []model.ComponentInfo

	// 获取系统命名空间的Pod作为组件
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}

	for _, namespace := range systemNamespaces {
		pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		// 按组件名分组
		componentMap := make(map[string]*model.ComponentInfo)

		for _, pod := range pods.Items {
			componentName := getComponentNameFromPod(pod)
			if componentName == "" {
				continue
			}

			version := getVersionFromPod(pod)

			if comp, exists := componentMap[componentName]; exists {
				// 更新状态 - 如果有任何Pod运行，组件状态为Running
				if pod.Status.Phase == "Running" && comp.Status != "Running" {
					comp.Status = "Running"
				}
				if comp.Version == "" || comp.Version == "unknown" {
					comp.Version = version
				}
			} else {
				status := "NotReady"
				if pod.Status.Phase == "Running" {
					status = "Running"
				}

				componentMap[componentName] = &model.ComponentInfo{
					Name:      componentName,
					Namespace: namespace,
					Status:    status,
					Version:   version,
					Type:      "system",
				}
			}
		}

		// 将组件添加到结果中
		for _, comp := range componentMap {
			components = append(components, *comp)
		}
	}

	return components, nil
}

func getComponentNameFromPod(pod corev1.Pod) string {
	name := strings.ToLower(pod.Name)

	knownComponents := []string{
		"kube-apiserver",
		"etcd",
		"kube-controller-manager",
		"kube-scheduler",
		"kube-proxy",
		"coredns",
		"kube-dns",
		"metrics-server",
		"calico",
		"flannel",
		"cilium",
		"traefik",
		"ingress-nginx",
	}

	for _, component := range knownComponents {
		if strings.Contains(name, component) {
			if component == "kube-dns" {
				return "coredns"
			}
			return component
		}
	}

	for _, labels := range []map[string]string{pod.Labels, pod.Annotations} {
		for _, key := range []string{
			"app.kubernetes.io/name",
			"app.kubernetes.io/component",
			"k8s-app",
			"component",
			"app",
		} {
			value := strings.ToLower(labels[key])
			for _, component := range knownComponents {
				if strings.Contains(value, component) {
					if component == "kube-dns" {
						return "coredns"
					}
					return component
				}
			}
		}
	}

	return ""
}

func getVersionFromPod(pod corev1.Pod) string {
	for _, container := range pod.Spec.Containers {
		version := extractImageVersion(container.Image)
		if version != "" {
			return version
		}
	}

	for _, container := range pod.Spec.InitContainers {
		version := extractImageVersion(container.Image)
		if version != "" {
			return version
		}
	}

	return "unknown"
}

func extractImageVersion(image string) string {
	if image == "" {
		return ""
	}

	if idx := strings.Index(image, "@"); idx > -1 {
		image = image[:idx]
	}

	idx := strings.LastIndex(image, ":")
	lastSlash := strings.LastIndex(image, "/")
	if idx > lastSlash && idx < len(image)-1 {
		return image[idx+1:]
	}

	return ""
}

// getWorkloadSummary 获取工作负载统计
func (s *KubeClusterServiceImpl) getWorkloadSummary(ctx context.Context, clientset *kubernetes.Clientset) (model.WorkloadSummary, error) {
	summary := model.WorkloadSummary{}

	// 获取Deployments
	if deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{}); err == nil && deployments != nil {
		summary.TotalDeployments = len(deployments.Items)
	}

	// 获取StatefulSets
	if statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{}); err == nil && statefulSets != nil {
		summary.TotalStatefulSets = len(statefulSets.Items)
	}

	// 获取DaemonSets
	if daemonSets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{}); err == nil && daemonSets != nil {
		summary.TotalDaemonSets = len(daemonSets.Items)
	}

	// 获取Jobs
	if jobs, err := clientset.BatchV1().Jobs("").List(ctx, metav1.ListOptions{}); err == nil && jobs != nil {
		summary.TotalJobs = len(jobs.Items)
	}

	// 获取CronJobs
	if cronJobs, err := clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{}); err == nil && cronJobs != nil {
		summary.TotalCronJobs = len(cronJobs.Items)
	}

	// 获取Pods
	if pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); err == nil && pods != nil {
		summary.TotalPods = len(pods.Items)
		for _, pod := range pods.Items {
			if pod.Status.Phase == "Running" {
				summary.RunningPods++
			}
		}
	}

	return summary, nil
}

// getNetworkInfo 获取网络配置信息
func (s *KubeClusterServiceImpl) getNetworkInfo(ctx context.Context, clientset *kubernetes.Clientset) (model.NetworkInfo, error) {
	network := model.NetworkInfo{}

	// 获取kube-system命名空间中的ConfigMap来获取网络配置
	configMaps, err := clientset.CoreV1().ConfigMaps("kube-system").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, cm := range configMaps.Items {
			if cm.Name == "kubeadm-config" {
				// 从kubeadm-config中解析网络配置
				if clusterConfig, exists := cm.Data["ClusterConfiguration"]; exists {
					if strings.Contains(clusterConfig, "serviceSubnet") {
						// 简单解析ServiceCIDR（实际应该用yaml解析）
						lines := strings.Split(clusterConfig, "\n")
						for _, line := range lines {
							if strings.Contains(line, "serviceSubnet") {
								parts := strings.Split(line, ":")
								if len(parts) > 1 {
									network.ServiceCIDR = strings.TrimSpace(parts[1])
								}
							}
						}
					}
				}
			}
		}
	}

	// 获取API Server端点，优先使用 EndpointSlice 以兼容 v1.33+ 集群。
	endpointIndex, err := listServiceEndpointsCompat(ctx, clientset, "default", []string{"kubernetes"})
	if err == nil {
		for _, endpoint := range endpointIndex["kubernetes"] {
			if endpoint.IP == "" {
				continue
			}
			network.APIServerEndpoint = endpoint.IP
			if len(endpoint.Ports) > 0 && endpoint.Ports[0].Port > 0 {
				network.APIServerEndpoint += fmt.Sprintf(":%d", endpoint.Ports[0].Port)
			}
			break
		}
	}

	// 默认值设置
	if network.ServiceCIDR == "" {
		network.ServiceCIDR = "10.96.0.0/12" // Kubernetes默认
	}
	network.PodCIDR = "10.244.0.0/16" // Flannel默认
	network.DNSService = "CoreDNS"
	network.NetworkPlugin = "Flannel" // 需要实际检测
	network.ProxyMode = "iptables"

	return network, nil
}

// getClusterEvents 获取集群事件
func (s *KubeClusterServiceImpl) getClusterEvents(ctx context.Context, clientset *kubernetes.Clientset) ([]model.ClusterEvent, error) {
	events, err := clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{
		Limit: 50, // 限制最近50个事件
	})
	if err != nil {
		return nil, err
	}

	var clusterEvents []model.ClusterEvent
	for _, event := range events.Items {
		clusterEvent := model.ClusterEvent{
			Type:           event.Type,
			Reason:         event.Reason,
			Message:        event.Message,
			Source:         event.Source.Component,
			FirstTime:      event.FirstTimestamp.Format(time.RFC3339),
			LastTime:       event.LastTimestamp.Format(time.RFC3339),
			Count:          event.Count,
			InvolvedObject: fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
		}
		clusterEvents = append(clusterEvents, clusterEvent)
	}

	return clusterEvents, nil
}

// getMonitoringInfo 获取监控信息
func (s *KubeClusterServiceImpl) getMonitoringInfo(ctx context.Context, clientset *kubernetes.Clientset, metricsClient *metricsclientset.Clientset) (model.MonitoringInfo, error) {
	monitoring := model.MonitoringInfo{}

	// 获取节点资源统计
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return monitoring, err
	}

	var totalCPU, totalMemory int64
	var allocatableCPU, allocatableMemory int64
	var usedCPU, usedMemory int64

	for _, node := range nodes.Items {
		if cpu, exists := node.Status.Capacity["cpu"]; exists {
			// 使用MilliValue获取毫核数
			totalCPU += cpu.MilliValue()
		}
		if memory, exists := node.Status.Capacity["memory"]; exists {
			totalMemory += memory.Value()
		}
		if cpu, exists := node.Status.Allocatable["cpu"]; exists {
			allocatableCPU += cpu.MilliValue()
		}
		if memory, exists := node.Status.Allocatable["memory"]; exists {
			allocatableMemory += memory.Value()
		}
	}

	// CPU监控信息
	if metricsClient != nil {
		if nodeMetrics, metricErr := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{}); metricErr == nil {
			for _, item := range nodeMetrics.Items {
				usedCPU += item.Usage.Cpu().MilliValue()
				usedMemory += item.Usage.Memory().Value()
			}
		}
	}

	monitoring.CPU = model.ClusterResourceMetrics{
		Total:       fmt.Sprintf("%.1f cores", float64(totalCPU)/1000), // 毫核转换为核
		Used:        fmt.Sprintf("%.1f cores", float64(usedCPU)/1000),
		Available:   fmt.Sprintf("%.1f cores", float64(allocatableCPU)/1000), // 毫核转换为核
		UsageRate:   percentOf(usedCPU, allocatableCPU),
		RequestRate: 0,
	}

	// 内存监控信息
	monitoring.Memory = model.ClusterResourceMetrics{
		Total:       fmt.Sprintf("%d Mi", totalMemory/(1024*1024)),
		Used:        fmt.Sprintf("%d Mi", usedMemory/(1024*1024)),
		Available:   fmt.Sprintf("%d Mi", allocatableMemory/(1024*1024)),
		UsageRate:   percentOf(usedMemory, allocatableMemory),
		RequestRate: 0,
	}

	// CPU/内存历史曲线是增强信息，使用更短超时避免拖慢集群详情首屏。
	historyCtx, historyCancel := context.WithTimeout(ctx, clusterDetailHistoryTimeout)
	defer historyCancel()
	monitoring.History = s.getClusterMetricsHistory(historyCtx, clientset, nodes.Items, allocatableCPU, allocatableMemory)

	// 网络监控（示例数据）
	monitoring.Network = model.NetworkMetrics{
		InboundTraffic:  "0 MB/s",
		OutboundTraffic: "0 MB/s",
		PacketsIn:       0,
		PacketsOut:      0,
	}

	// 存储监控
	boundPVs := 0
	totalPVs := 0
	if pvs, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{}); err == nil && pvs != nil {
		totalPVs = len(pvs.Items)
		for _, pv := range pvs.Items {
			if pv.Status.Phase == "Bound" {
				boundPVs++
			}
		}
	}

	totalPVCs := 0
	if pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{}); err == nil && pvcs != nil {
		totalPVCs = len(pvcs.Items)
	}

	var scNames []string
	if storageClasses, err := clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{}); err == nil && storageClasses != nil {
		for _, sc := range storageClasses.Items {
			scNames = append(scNames, sc.Name)
		}
	}

	monitoring.Storage = model.StorageMetrics{
		TotalPVs:       totalPVs,
		BoundPVs:       boundPVs,
		TotalPVCs:      totalPVCs,
		StorageClasses: scNames,
	}

	return monitoring, nil
}

// metrics-scraper returns Dashboard-compatible history data for node and pod charts.
type scraperMetricResultList struct {
	Items []scraperMetric `json:"items"`
}

type scraperMetric struct {
	MetricName   string               `json:"metricName"`
	MetricPoints []scraperMetricPoint `json:"metricPoints"`
	UIDs         []string             `json:"uids"`
}

type scraperMetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     uint64    `json:"value"`
}

func (s *KubeClusterServiceImpl) getClusterMetricsHistory(ctx context.Context, clientset *kubernetes.Clientset, nodes []corev1.Node, allocatableCPU, allocatableMemory int64) model.ClusterMetricsHistory {
	history := model.ClusterMetricsHistory{
		Available: false,
		Source:    "metrics-scraper",
		Message:   "metrics-scraper has no historical CPU/memory data",
		Charts: []model.MetricsChart{
			newMetricsChart("cpuUsageRate", "CPU 利用率", "%", 2),
			newMetricsChart("memoryUsageRate", "内存利用率", "%", 2),
			newMetricsChart("cpuUsageCores", "CPU 使用量", "cores", 3),
			newMetricsChart("memoryUsageGB", "内存使用量", "GB", 3),
		},
	}

	nodeNames := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	sort.Strings(nodeNames)

	candidates := getMetricsScraperServiceCandidates(ctx, clientset)
	var nodeCPU, nodeMemory map[string][]scraperMetricPoint
	var errCPU, errMemory error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		nodeCPU, errCPU = getScraperResourceMetricsWithCandidates(ctx, clientset, candidates, "node", "", nodeNames, "cpu")
	}()
	go func() {
		defer wg.Done()
		nodeMemory, errMemory = getScraperResourceMetricsWithCandidates(ctx, clientset, candidates, "node", "", nodeNames, "memory")
	}()
	wg.Wait()

	if errCPU != nil && errMemory != nil {
		history.Message = fmt.Sprintf("metrics-scraper query failed: %v; %v", errCPU, errMemory)
		return history
	}

	nodeHistories := make([]model.ResourceMetricHistory, 0, len(nodeNames))
	for _, name := range nodeNames {
		cpuPoints := convertRawPoints(nodeCPU[name], 0.001)
		memoryPoints := convertRawPoints(nodeMemory[name], 1.0/(1024*1024*1024))
		if len(cpuPoints) == 0 && len(memoryPoints) == 0 {
			continue
		}
		nodeHistories = append(nodeHistories, model.ResourceMetricHistory{
			Name:   name,
			Kind:   "node",
			CPU:    cpuPoints,
			Memory: memoryPoints,
		})
	}

	allCPU := sumMetricSeries(nodeCPU)
	allMemory := sumMetricSeries(nodeMemory)
	cpuCores := convertRawPoints(allCPU, 0.001)
	memoryGB := convertRawPoints(allMemory, 1.0/(1024*1024*1024))
	cpuRate := ratePoints(allCPU, allocatableCPU)
	memoryRate := ratePoints(allMemory, allocatableMemory)

	history.Charts = []model.MetricsChart{
		{Key: "cpuUsageRate", Title: "CPU 利用率", Unit: "%", Precision: 2, Points: cpuRate},
		{Key: "memoryUsageRate", Title: "内存利用率", Unit: "%", Precision: 2, Points: memoryRate},
		{Key: "cpuUsageCores", Title: "CPU 使用量", Unit: "cores", Precision: 3, Points: cpuCores},
		{Key: "memoryUsageGB", Title: "内存使用量", Unit: "GB", Precision: 3, Points: memoryGB},
	}
	history.Nodes = nodeHistories
	history.Available = len(cpuCores) > 0 || len(memoryGB) > 0
	if history.Available {
		history.Message = ""
		history.TimeRange, history.StepSeconds = describeMetricRange(cpuCores, memoryGB)
	}

	history.Pods = s.getClusterPodMetricsHistory(ctx, clientset)

	return history
}

func (s *KubeClusterServiceImpl) getClusterPodMetricsHistory(ctx context.Context, clientset *kubernetes.Clientset) []model.ResourceMetricHistory {
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	namespacePods := make(map[string][]string)
	podResourceTotals := make(map[string]podResourceTotal)
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		namespacePods[pod.Namespace] = append(namespacePods[pod.Namespace], pod.Name)
		podResourceTotals[podMetricKey(pod.Namespace, pod.Name)] = getPodResourceTotal(pod)
	}

	var histories []model.ResourceMetricHistory
	for namespace, names := range namespacePods {
		sort.Strings(names)
		if len(names) > 30 {
			names = names[:30]
		}

		cpuSeries, _ := getScraperResourceMetrics(ctx, clientset, "pod", namespace, names, "cpu")
		memorySeries, _ := getScraperResourceMetrics(ctx, clientset, "pod", namespace, names, "memory")
		for _, name := range names {
			cpuPoints := convertRawPoints(cpuSeries[name], 0.001)
			memoryPoints := convertRawPoints(memorySeries[name], 1.0/(1024*1024*1024))
			if len(cpuPoints) == 0 && len(memoryPoints) == 0 {
				continue
			}
			resourceTotal := podResourceTotals[podMetricKey(namespace, name)]
			histories = append(histories, model.ResourceMetricHistory{
				Name:            name,
				Namespace:       namespace,
				Kind:            "pod",
				CPU:             cpuPoints,
				Memory:          memoryPoints,
				CPUUsageRate:    ratePoints(cpuSeries[name], resourceTotal.CPUMilli),
				MemoryUsageRate: ratePoints(memorySeries[name], resourceTotal.MemoryBytes),
			})
			if len(histories) >= 100 {
				return histories
			}
		}
	}

	return histories
}

type podResourceTotal struct {
	CPUMilli    int64
	MemoryBytes int64
}

func getPodResourceTotal(pod corev1.Pod) podResourceTotal {
	requests := podResourceTotal{}
	limits := podResourceTotal{}

	for _, container := range pod.Spec.Containers {
		if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			requests.CPUMilli += cpu.MilliValue()
		}
		if memory, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			requests.MemoryBytes += memory.Value()
		}
		if cpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
			limits.CPUMilli += cpu.MilliValue()
		}
		if memory, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			limits.MemoryBytes += memory.Value()
		}
	}

	if requests.CPUMilli == 0 {
		requests.CPUMilli = limits.CPUMilli
	}
	if requests.MemoryBytes == 0 {
		requests.MemoryBytes = limits.MemoryBytes
	}

	return requests
}

func podMetricKey(namespace, name string) string {
	return namespace + "/" + name
}

func getScraperResourceMetrics(ctx context.Context, clientset *kubernetes.Clientset, kind, namespace string, names []string, metricName string) (map[string][]scraperMetricPoint, error) {
	return getScraperResourceMetricsWithCandidates(ctx, clientset, getMetricsScraperServiceCandidates(ctx, clientset), kind, namespace, names, metricName)
}

func getScraperResourceMetricsWithCandidates(ctx context.Context, clientset *kubernetes.Clientset, candidates []metricsScraperService, kind, namespace string, names []string, metricName string) (map[string][]scraperMetricPoint, error) {
	result := make(map[string][]scraperMetricPoint)
	if len(names) == 0 {
		return result, nil
	}

	pathName := strings.Join(names, ",")
	path := fmt.Sprintf("api/v1/dashboard/nodes/%s/metrics/%s/history", pathName, metricName)
	if kind == "pod" {
		path = fmt.Sprintf("api/v1/dashboard/namespaces/%s/pod-list/%s/metrics/%s/history", namespace, pathName, metricName)
	}

	var data []byte
	var err error
	for _, svc := range candidates {
		data, err = clientset.CoreV1().Services(svc.Namespace).ProxyGet("http", svc.Name, "", path, nil).DoRaw(ctx)
		if err == nil {
			break
		}
	}
	if err != nil {
		return result, err
	}

	var response scraperMetricResultList
	if err := json.Unmarshal(data, &response); err != nil {
		return result, err
	}

	for _, item := range response.Items {
		name := ""
		if len(item.UIDs) > 0 {
			name = item.UIDs[0]
		}
		if name == "" {
			continue
		}
		sort.Slice(item.MetricPoints, func(i, j int) bool {
			return item.MetricPoints[i].Timestamp.Before(item.MetricPoints[j].Timestamp)
		})
		result[name] = item.MetricPoints
	}

	return result, nil
}

func getMetricsScraperServiceCandidates(ctx context.Context, clientset *kubernetes.Clientset) []metricsScraperService {
	candidates := []metricsScraperService{
		{Namespace: "kube-system", Name: "metrics-scraper"},
		{Namespace: "kubernetes-dashboard", Name: "metrics-scraper"},
		{Namespace: "kubernetes-dashboard", Name: "dashboard-metrics-scraper"},
		{Namespace: "kubernetes-dashboard", Name: "kubernetes-dashboard-metrics-scraper"},
		{Namespace: "kuboard", Name: "metrics-scraper"},
		{Namespace: "kuboard", Name: "dashboard-metrics-scraper"},
		{Namespace: "kuboard", Name: "kubernetes-dashboard-metrics-scraper"},
	}

	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return candidates
	}

	seen := make(map[string]struct{}, len(candidates))
	for _, item := range candidates {
		seen[item.Namespace+"/"+item.Name] = struct{}{}
	}
	for _, svc := range services.Items {
		name := strings.ToLower(svc.Name)
		if !strings.Contains(name, "metrics-scraper") {
			continue
		}
		key := svc.Namespace + "/" + svc.Name
		if _, exists := seen[key]; exists {
			continue
		}
		candidates = append(candidates, metricsScraperService{
			Namespace: svc.Namespace,
			Name:      svc.Name,
		})
		seen[key] = struct{}{}
	}

	return candidates
}

type metricsScraperService struct {
	Namespace string
	Name      string
}

func newMetricsChart(key, title, unit string, precision int) model.MetricsChart {
	return model.MetricsChart{
		Key:       key,
		Title:     title,
		Unit:      unit,
		Precision: precision,
		Points:    []model.MetricsPoint{},
	}
}

func convertRawPoints(points []scraperMetricPoint, multiplier float64) []model.MetricsPoint {
	result := make([]model.MetricsPoint, 0, len(points))
	for _, point := range points {
		result = append(result, model.MetricsPoint{
			Timestamp: point.Timestamp.Format(time.RFC3339),
			Value:     roundMetric(float64(point.Value)*multiplier, 3),
		})
	}
	return result
}

func ratePoints(points []scraperMetricPoint, total int64) []model.MetricsPoint {
	result := make([]model.MetricsPoint, 0, len(points))
	for _, point := range points {
		result = append(result, model.MetricsPoint{
			Timestamp: point.Timestamp.Format(time.RFC3339),
			Value:     percentOf(int64(point.Value), total),
		})
	}
	return result
}

func sumMetricSeries(series map[string][]scraperMetricPoint) []scraperMetricPoint {
	bucket := make(map[string]uint64)
	times := make(map[string]time.Time)
	for _, points := range series {
		for _, point := range points {
			key := point.Timestamp.UTC().Format(time.RFC3339)
			bucket[key] += point.Value
			times[key] = point.Timestamp
		}
	}

	keys := make([]string, 0, len(bucket))
	for key := range bucket {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]scraperMetricPoint, 0, len(keys))
	for _, key := range keys {
		result = append(result, scraperMetricPoint{
			Timestamp: times[key],
			Value:     bucket[key],
		})
	}
	return result
}

func describeMetricRange(series ...[]model.MetricsPoint) (string, int64) {
	var first, last time.Time
	var previous time.Time
	var step int64
	for _, points := range series {
		for _, point := range points {
			current, err := time.Parse(time.RFC3339, point.Timestamp)
			if err != nil {
				continue
			}
			if first.IsZero() || current.Before(first) {
				first = current
			}
			if last.IsZero() || current.After(last) {
				last = current
			}
			if !previous.IsZero() {
				diff := int64(current.Sub(previous).Seconds())
				if diff > 0 && (step == 0 || diff < step) {
					step = diff
				}
			}
			previous = current
		}
	}

	if first.IsZero() || last.IsZero() {
		return "", step
	}
	return fmt.Sprintf("%s - %s", first.Format(time.RFC3339), last.Format(time.RFC3339)), step
}

func percentOf(value, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return roundMetric(float64(value)/float64(total)*100, 2)
}

func roundMetric(value float64, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	return math.Round(value*factor) / factor
}

func (s *KubeClusterServiceImpl) getRuntimeInfo(ctx context.Context, clientset *kubernetes.Clientset) (model.RuntimeSummary, error) {
	runtime := model.RuntimeSummary{}

	// 获取集群版本
	version, err := clientset.Discovery().ServerVersion()
	if err == nil {
		runtime.KubernetesVersion = version.GitVersion
		runtime.APIServerVersion = version.GitVersion
	}

	// 获取第一个节点的运行时信息作为示例
	var nodeItems []corev1.Node
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err == nil && nodes != nil {
		nodeItems = nodes.Items
	}
	if len(nodeItems) > 0 {
		node := nodeItems[0]
		runtime.ContainerRuntime = node.Status.NodeInfo.ContainerRuntimeVersion
		runtime.KubeProxyVersion = node.Status.NodeInfo.KubeProxyVersion
	}

	// 从kube-system命名空间获取etcd和CoreDNS版本
	if pods, err := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{}); err == nil && pods != nil {
		for _, pod := range pods.Items {
			componentName := getComponentNameFromPod(pod)
			version := getVersionFromPod(pod)
			if version == "unknown" {
				continue
			}

			switch componentName {
			case "etcd":
				runtime.EtcdVersion = version
			case "coredns":
				runtime.CoreDNSVersion = version
			case "kube-proxy":
				if runtime.KubeProxyVersion == "" {
					runtime.KubeProxyVersion = version
				}
			}
		}
	}

	// 计算集群运行时间（从最老的节点创建时间算起）
	if len(nodeItems) > 0 {
		oldestTime := nodeItems[0].CreationTimestamp.Time
		for _, node := range nodeItems {
			if node.CreationTimestamp.Time.Before(oldestTime) {
				oldestTime = node.CreationTimestamp.Time
			}
		}
		upTime := time.Since(oldestTime)
		runtime.UpTime = fmt.Sprintf("%.0f days", upTime.Hours()/24)
	}

	return runtime, nil
}
