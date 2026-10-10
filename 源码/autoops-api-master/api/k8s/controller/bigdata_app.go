package controller

import (
	"strconv"

	"dodevops-api/api/k8s/model"
	"dodevops-api/api/k8s/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BigDataAppController struct {
	service service.IBigDataAppService
}

func NewBigDataAppController(db *gorm.DB) *BigDataAppController {
	return &BigDataAppController{
		service: service.NewBigDataAppService(db),
	}
}

func parseBigDataClusterID(c *gin.Context) (uint, bool) {
	clusterID, err := strconv.Atoi(c.Param("id"))
	if err != nil || clusterID <= 0 {
		result.Failed(c, 400, "无效的集群ID")
		return 0, false
	}
	return uint(clusterID), true
}

func requireBigDataNamespace(c *gin.Context) (string, bool) {
	namespace := c.Param("namespaceName")
	if namespace == "" {
		result.Failed(c, 400, "命名空间名称不能为空")
		return "", false
	}
	return namespace, true
}

// GetComponentTree 获取大数据应用组件树
func (ctrl *BigDataAppController) GetComponentTree(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	ctrl.service.GetComponentTree(c, clusterID, c.Query("namespace"))
}

// GetComponentDetail 获取组件详情、Pod列表和容器监控
func (ctrl *BigDataAppController) GetComponentDetail(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	component := c.Param("componentName")
	if component == "" {
		result.Failed(c, 400, "组件名称不能为空")
		return
	}
	ctrl.service.GetComponentDetail(c, clusterID, c.Query("namespace"), component)
}

// GetComponentMetrics 获取组件Pod和容器监控
func (ctrl *BigDataAppController) GetComponentMetrics(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	component := c.Param("componentName")
	if component == "" {
		result.Failed(c, 400, "组件名称不能为空")
		return
	}
	ctrl.service.GetComponentMetrics(c, clusterID, c.Query("namespace"), component)
}

// OperateComponent 对组件执行启动、停止、重启、快速重启等操作
func (ctrl *BigDataAppController) OperateComponent(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	component := c.Param("componentName")
	if component == "" {
		result.Failed(c, 400, "组件名称不能为空")
		return
	}
	var req model.BigDataActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "参数验证失败: "+err.Error())
		return
	}
	ctrl.service.OperateComponent(c, clusterID, c.Query("namespace"), component, &req)
}

// OperateWorkload 对单个工作负载执行操作
func (ctrl *BigDataAppController) OperateWorkload(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	namespace, ok := requireBigDataNamespace(c)
	if !ok {
		return
	}
	workloadType := c.Param("workloadType")
	workloadName := c.Param("workloadName")
	if workloadType == "" || workloadName == "" {
		result.Failed(c, 400, "工作负载类型和名称不能为空")
		return
	}
	var req model.BigDataActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "参数验证失败: "+err.Error())
		return
	}
	ctrl.service.OperateWorkload(c, clusterID, namespace, workloadType, workloadName, &req)
}

// OperatePod 对单个Pod执行操作
func (ctrl *BigDataAppController) OperatePod(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	namespace, ok := requireBigDataNamespace(c)
	if !ok {
		return
	}
	podName := c.Param("podName")
	if podName == "" {
		result.Failed(c, 400, "Pod名称不能为空")
		return
	}
	var req model.BigDataActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "参数验证失败: "+err.Error())
		return
	}
	ctrl.service.OperatePod(c, clusterID, namespace, podName, &req)
}

// GetWorkloadYAML 获取工作负载配置
func (ctrl *BigDataAppController) GetWorkloadYAML(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	namespace, ok := requireBigDataNamespace(c)
	if !ok {
		return
	}
	ctrl.service.GetWorkloadYAML(c, clusterID, namespace, c.Param("workloadType"), c.Param("workloadName"))
}

// ApplyWorkloadYAML 应用工作负载配置
func (ctrl *BigDataAppController) ApplyWorkloadYAML(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	namespace, ok := requireBigDataNamespace(c)
	if !ok {
		return
	}
	var req model.UpdateWorkloadYAMLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "请求参数错误: "+err.Error())
		return
	}
	ctrl.service.ApplyWorkloadYAML(c, clusterID, namespace, &req)
}

// CreatePodFromYAML 新增Pod
func (ctrl *BigDataAppController) CreatePodFromYAML(c *gin.Context) {
	clusterID, ok := parseBigDataClusterID(c)
	if !ok {
		return
	}
	namespace, ok := requireBigDataNamespace(c)
	if !ok {
		return
	}
	var req model.CreatePodFromYAMLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		result.Failed(c, 400, "请求参数错误: "+err.Error())
		return
	}
	ctrl.service.CreatePodFromYAML(c, clusterID, namespace, &req)
}
