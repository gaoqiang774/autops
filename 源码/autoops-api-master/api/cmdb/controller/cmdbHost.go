package controller

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

var hostTemplateHeaders = []string{"name", "内网ip", "ssh_ip", "端口", "cpu", "内存", "磁盘", "备注", "env1", "env2"}

var hostTemplateSampleRows = [][]string{
	{"softwarePool", "172.16.3.10", "172.16.3.10", "22", "4", "4", "150 + 1000", "人事共享文件", "company", "贺凌莉"},
	{"openvpn-changsha-3-12", "172.16.3.12", "172.16.3.12", "22", "4", "2", "500", "打通【长沙分公司】环境的openvpn", "company", "运维部"},
	{"ceshi-DB-4-32", "172.16.3.32", "172.16.3.32", "22", "4", "12", "150 + 500", "mysql-33098", "dev", "欧阳文斌"},
	{"old-system-db-4-86", "172.16.3.86", "172.16.3.86", "22", "4", "10", "200", "", "test", ""},
	{"dev-gateway-gray-3-98", "172.16.3.98", "172.16.3.98", "22", "4", "4", "150", "workflow-boot-1.0.0.jar、purview-web-1.0.0.jar", "dev", "曾安"},
	{"dev-document-3-99", "172.16.3.99", "172.16.3.99", "22", "4", "4", "150", "node-9001", "dev", "郑泽正"},
	{"nginx-3-100", "172.16.3.100", "172.16.3.100", "22", "4", "4", "150", "nginx", "dev", ""},
	{"ceshi-yunying-houtai-3-101", "172.16.3.101", "172.16.3.101", "22", "4", "4", "150", "人事 elasticsearch", "itest", ""},
	{"prod-api-3-102", "172.16.3.102", "172.16.3.102", "22", "8", "16", "200 + 500", "订单服务", "prod", "张三"},
	{"prod-cache-3-103", "172.16.3.103", "172.16.3.103", "22", "8", "8", "300", "redis 缓存节点", "prod", "李四"},
}

func buildHostTemplateFile() (*excelize.File, error) {
	f := excelize.NewFile()
	defaultSheet := f.GetSheetName(0)
	if defaultSheet != "Sheet1" {
		f.SetSheetName(defaultSheet, "Sheet1")
	}

	for i, header := range hostTemplateHeaders {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue("Sheet1", cell, header); err != nil {
			return nil, err
		}
	}

	for rowIndex, row := range hostTemplateSampleRows {
		for colIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(colIndex+1, rowIndex+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue("Sheet1", cell, value); err != nil {
				return nil, err
			}
		}
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D9EAF7"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle("Sheet1", "A1", "J1", headerStyle); err != nil {
		return nil, err
	}

	if err := f.SetPanes("Sheet1", &excelize.Panes{Freeze: true, Split: true, XSplit: 0, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	if err := f.SetColWidth("Sheet1", "A", "J", 18); err != nil {
		return nil, err
	}
	if err := f.SetColWidth("Sheet1", "H", "H", 40); err != nil {
		return nil, err
	}

	return f, nil
}

func isHostTemplateRowEmpty(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func getHostTemplateValue(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func parseHostTemplateRow(row []string) model.ExcelHostTemplate {
	sshPort := util.StringToInt(getHostTemplateValue(row, 3))
	if sshPort == 0 {
		sshPort = 22
	}

	return model.ExcelHostTemplate{
		Name:      getHostTemplateValue(row, 0),
		PrivateIP: getHostTemplateValue(row, 1),
		SSHIP:     getHostTemplateValue(row, 2),
		SSHPort:   sshPort,
		SSHName:   "root",
		CPU:       getHostTemplateValue(row, 4),
		Memory:    getHostTemplateValue(row, 5),
		Disk:      getHostTemplateValue(row, 6),
		Remark:    getHostTemplateValue(row, 7),
		Env1:      getHostTemplateValue(row, 8),
		Env2:      getHostTemplateValue(row, 9),
	}
}

func responseTemplateFileError(ctx *gin.Context, err error) {
	result.Failed(ctx, constant.FILE_OPERATION_ERROR, "生成Excel模板失败: "+err.Error())
}

func writeHostTemplateToContext(ctx *gin.Context) {
	f, err := buildHostTemplateFile()
	if err != nil {
		responseTemplateFileError(ctx, err)
		return
	}
	defer f.Close()

	ctx.Header("Content-Description", "File Transfer")
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.Header("Content-Disposition", "attachment; filename=host_template.xlsx")
	ctx.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	if err := f.Write(ctx.Writer); err != nil {
		responseTemplateFileError(ctx, err)
	}
}

type CmdbHostController struct {
	service service.CmdbHostServiceInterface
}

func NewCmdbHostController() *CmdbHostController {
	return &CmdbHostController{
		service: service.GetCmdbHostService(),
	}
}

// 分页参数
type PageParams struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"pageSize" binding:"required,min=1,max=100"`
	SortBy    string `form:"sortBy"`    // 排序字段(id/update_time/create_time等)
	SortOrder string `form:"sortOrder"` // 排序方向(asc/desc)
}

// 下载主机导入模板
// @Summary 下载主机导入模板
// @Description 动态生成并下载主机导入Excel模板
// @Tags CMDB资产管理
// @Produce octet-stream
// @Success 200 {file} file
// @Router /api/v1/cmdb/hosttemplate [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) DownloadHostTemplate(ctx *gin.Context) {
	writeHostTemplateToContext(ctx)
}

// 获取主机列表(分页)
// @Summary 获取主机列表(分页)
// @Description 获取主机列表(分页)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Param sortBy query string false "排序字段(id/update_time/create_time等，默认id)"
// @Param sortOrder query string false "排序方向(asc/desc，默认desc)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostlist [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostListWithPage(ctx *gin.Context) {
	var params PageParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "分页参数错误")
		return
	}

	// 设置默认排序
	if params.SortBy == "" {
		params.SortBy = "update_time"
	}
	if params.SortOrder == "" {
		params.SortOrder = "desc"
	}

	c.service.GetCmdbHostListWithPage(ctx, params.Page, params.PageSize, params.SortBy, params.SortOrder)
}

// ExportCmdbHostsCSV 导出主机列表CSV
// @Summary 导出主机列表CSV
// @Description 按当前筛选条件导出主机列表，支持认证状态筛选
// @Tags CMDB资产管理
// @Accept json
// @Produce text/csv
// @Param keyword query string false "主机名称/IP关键字"
// @Param name query string false "兼容旧字段：主机名称关键字"
// @Param ip query string false "兼容旧字段：IP关键字"
// @Param groupId query int false "分组ID"
// @Param status query string false "认证状态，支持单值或逗号分隔多值，如 1 或 2,3"
// @Param sortBy query string false "排序字段"
// @Param sortOrder query string false "排序方向(asc/desc)"
// @Success 200 {file} file
// @Router /api/v1/cmdb/hostexport [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) ExportCmdbHostsCSV(ctx *gin.Context) {
	var query model.CmdbHostExportQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "查询参数错误")
		return
	}

	fileName := fmt.Sprintf("cmdb_hosts_%s.csv", time.Now().Format("20060102150405"))
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	ctx.Status(http.StatusOK)
	if _, err := ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}

	if _, err := c.service.ExportCmdbHostsCSV(ctx, query, ctx.Writer); err != nil {
		ctx.Header("Content-Disposition", "")
		result.Failed(ctx, constant.INVALID_PARAMS, "导出主机CSV失败: "+err.Error())
		return
	}
}

// 创建主机
// @Summary 创建主机
// @Description 创建主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CreateCmdbHostDto true "主机信息"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcreate [post]
// @Security ApiKeyAuth
func (c *CmdbHostController) CreateCmdbHost(ctx *gin.Context) {
	var dto model.CreateCmdbHostDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.CreateCmdbHost(ctx, &dto)
}

// 从Excel导入主机
// @Summary 从Excel导入主机
// @Description 通过上传Excel模板批量导入主机（Excel列顺序：name、内网ip、ssh_ip、端口、cpu、内存、磁盘、备注、env1、env2）
// @Tags CMDB资产管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Excel文件"
// @Param groupId formData int true "分组ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostimport [post]
// @Security ApiKeyAuth
func (c *CmdbHostController) ImportHostsFromExcel(ctx *gin.Context) {
	// 获取上传的文件
	file, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "请上传文件")
		return
	}

	// 获取分组ID
	groupId := util.StringToUint(ctx.PostForm("groupId"))
	if groupId == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "分组ID不能为空")
		return
	}

	// 使用项目upload目录作为临时存储，避免系统临时目录的权限问题
	tempDir := "./upload/temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result.Failed(ctx, constant.FILE_OPERATION_ERROR, "创建临时目录失败: "+err.Error())
		return
	}

	// 生成唯一的临时文件名
	timestamp := time.Now().UnixNano()
	ext := filepath.Ext(file.Filename)
	tempFileName := fmt.Sprintf("host_import_%d%s", timestamp, ext)
	tempFilePath := filepath.Join(tempDir, tempFileName)

	// 保存上传的文件
	if err := ctx.SaveUploadedFile(file, tempFilePath); err != nil {
		result.Failed(ctx, constant.FILE_OPERATION_ERROR, "保存上传文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFilePath)

	// 打开Excel文件
	f, err := excelize.OpenFile(tempFilePath)
	if err != nil {
		result.Failed(ctx, constant.FILE_OPERATION_ERROR, "打开Excel文件失败")
		return
	}
	defer f.Close()

	// 读取Excel数据
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		result.Failed(ctx, constant.FILE_OPERATION_ERROR, "读取Excel数据失败")
		return
	}

	// 解析Excel数据
	var hosts []model.ExcelHostTemplate
	for i, row := range rows {
		if i == 0 { // 跳过表头
			continue
		}
		if isHostTemplateRowEmpty(row) {
			continue
		}
		if len(row) < 3 {
			continue
		}

		hosts = append(hosts, parseHostTemplateRow(row))
	}

	// 调用服务层批量导入
	dto := model.ImportHostsFromExcelDto{
		GroupID: groupId,
		File:    file.Filename,
	}
	c.service.ImportHostsFromExcel(ctx, &dto, hosts)
}

// 更新主机
// @Summary 更新主机
// @Description 更新主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.UpdateCmdbHostDto true "主机信息(包含ID)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostupdate [put]
// @Security ApiKeyAuth
func (c *CmdbHostController) UpdateCmdbHost(ctx *gin.Context) {
	var dto model.UpdateCmdbHostDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.UpdateCmdbHost(ctx, dto.ID, &dto)
}

// 删除主机
// @Summary 删除主机
// @Description 删除主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CmdbHostIdDto true "主机ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostdelete [delete]
// @Security ApiKeyAuth
func (c *CmdbHostController) DeleteCmdbHost(ctx *gin.Context) {
	var dto model.CmdbHostIdDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.DeleteCmdbHost(ctx, dto.ID)
}

// 根据ID获取主机
// @Summary 根据ID获取主机
// @Description 根据ID获取主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param id query int true "主机ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostinfo [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostById(ctx *gin.Context) {
	id := util.StringToUint(ctx.Query("id"))
	c.service.GetCmdbHostById(ctx, id)
}

// 根据分组ID获取主机列表
// @Summary 根据分组ID获取主机列表
// @Description 根据分组ID获取主机列表（包括所有子分组的主机，支持分页）
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param groupId query int true "分组ID"
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostgroup [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostsByGroupId(ctx *gin.Context) {
	groupId := util.StringToUint(ctx.Query("groupId"))

	// 获取分页参数
	var params PageParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "分页参数错误")
		return
	}

	c.service.GetCmdbHostsByGroupId(ctx, groupId, params.Page, params.PageSize)
}

// 根据主机名称模糊查询
// @Summary 根据主机名称、内网IP、外网IP模糊查询
// @Description 根据主机名称、内网IP、外网IP模糊查询(支持同时匹配名称、内网IP、外网IP)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param name query string true "主机名称/内网IP/外网IP(模糊匹配)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbyname [get]
// @Security ApiKeyAuth
// SearchCmdbHosts 模糊查询主机(主机名/IP/SSH IP)。
// @Summary 模糊查询主机
// @Description 根据主机名称、name、内网IP、外网IP、SSH IP 模糊查询主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param keyword query string true "主机名称/IP关键字(模糊匹配)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostsearch [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) SearchCmdbHosts(ctx *gin.Context) {
	keyword := ctx.Query("keyword")
	if keyword == "" {
		keyword = ctx.Query("name")
	}
	if keyword == "" {
		keyword = ctx.Query("ip")
	}
	c.service.GetCmdbHostsByHostNameLike(ctx, keyword)
}

func (c *CmdbHostController) GetCmdbHostsByHostNameLike(ctx *gin.Context) {
	name := ctx.Query("name")
	c.service.GetCmdbHostsByHostNameLike(ctx, name)
}

// 根据IP查询主机
// @Summary 根据IP查询主机
// @Description 根据IP查询主机(匹配内网IP、公网IP或SSH IP)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param ip query string true "IP地址"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbyip [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostsByIP(ctx *gin.Context) {
	ip := ctx.Query("ip")
	c.service.GetCmdbHostsByHostNameLike(ctx, ip)
}

// 根据状态查询主机
// @Summary 根据状态查询主机
// @Description 根据状态查询主机(1->认证成功,2->未认证,3->认证失败)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param status query int true "状态(1/2/3)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbystatus [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostsByStatus(ctx *gin.Context) {
	status := int(util.StringToUint(ctx.Query("status")))
	c.service.GetCmdbHostsByStatus(ctx, status)
}

// 根据主机系统类型查询主机
// @Summary 根据主机系统类型查询主机
// @Description 根据主机系统类型查询主机(1->Linux,2->Windows)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param hostType query int true "主机系统类型(1->Linux,2->Windows)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbyhosttype [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostsByHostType(ctx *gin.Context) {
	hostType := int(util.StringToUint(ctx.Query("hostType")))
	page := util.StringToInt(ctx.DefaultQuery("page", "1"))
	pageSize := util.StringToInt(ctx.DefaultQuery("pageSize", "10"))
	sortBy := strings.TrimSpace(ctx.DefaultQuery("sortBy", "id"))
	sortOrder := strings.TrimSpace(ctx.DefaultQuery("sortOrder", "desc"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	c.service.GetCmdbHostsByHostType(ctx, hostType, page, pageSize, sortBy, sortOrder)
}

func parseHostVendorQuery(ctx *gin.Context) int {
	vendor := strings.TrimSpace(ctx.Query("vendor"))
	if vendor == "" {
		vendor = strings.TrimSpace(ctx.Query("type"))
	}
	if vendor == "" {
		vendor = strings.TrimSpace(ctx.Query("vendorName"))
	}

	vendorNames := map[string]int{
		"自建":  1,
		"阿里云": 2,
		"腾讯云": 3,
		"百度云": 4,
		"华为云": 5,
		"AWS": 6,
		"aws": 6,
		"天翼云": 7,
	}
	if value, ok := vendorNames[vendor]; ok {
		return value
	}
	return int(util.StringToUint(vendor))
}

// 根据主机类型/厂商查询主机
// @Summary 根据主机类型/厂商查询主机
// @Description 根据主机类型/厂商查询主机(1->自建,2->阿里云,3->腾讯云,4->百度云,5->华为云,6->AWS,7->天翼云)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param vendor query int true "主机类型/厂商(1->自建,2->阿里云,3->腾讯云,4->百度云,5->华为云,6->AWS,7->天翼云)"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbytype [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCmdbHostsByVendor(ctx *gin.Context) {
	vendor := parseHostVendorQuery(ctx)
	c.service.GetCmdbHostsByVendor(ctx, vendor)
}

// 同步主机基本信息
// @Summary 同步主机基本信息
// @Description 根据主机ID自动同步获取目标主机的基本信息(主机名称、操作系统、CPU、内存、磁盘、内网IP、公网IP)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.CmdbHostIdDto true "主机ID"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostsync [post]
// @Security ApiKeyAuth
func (c *CmdbHostController) SyncHostInfo(ctx *gin.Context) {
	var dto model.CmdbHostIdDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.SyncHostInfo(ctx, dto.ID)
}

// 获取云主机统计信息
// @Summary 获取云主机统计信息
// @Description 按类型统计主机数量(自建、阿里云、腾讯云、百度云、华为云、AWS)
// @Tags CMDB资产管理
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostcloudstatistics [get]
// @Security ApiKeyAuth
func (c *CmdbHostController) GetCloudHostStatistics(ctx *gin.Context) {
	c.service.GetCloudHostStatistics(ctx)
}

// 批量更新主机分组
// @Summary 批量更新主机分组
// @Description 批量更新多台主机的分组,目标分组必须是二级分组(子分组)
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.BatchUpdateHostGroupDto true "批量更新分组请求参数"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbatchupdategroup [put]
// @Security ApiKeyAuth
func (c *CmdbHostController) BatchUpdateHostGroup(ctx *gin.Context) {
	var dto model.BatchUpdateHostGroupDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.BatchUpdateHostGroup(ctx, &dto)
}

// 批量删除主机
// @Summary 批量删除主机
// @Description 批量删除多台主机
// @Tags CMDB资产管理
// @Accept json
// @Produce json
// @Param data body model.BatchDeleteHostDto true "批量删除请求参数"
// @Success 200 {object} result.Result
// @Router /api/v1/cmdb/hostbatchdelete [delete]
// @Security ApiKeyAuth
func (c *CmdbHostController) BatchDeleteHost(ctx *gin.Context) {
	var dto model.BatchDeleteHostDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "参数错误")
		return
	}
	c.service.BatchDeleteHost(ctx, &dto)
}
