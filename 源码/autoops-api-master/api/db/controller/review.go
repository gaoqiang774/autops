package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/api/db/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type ReviewController struct {
	service *service.ReviewService
}

func NewReviewController() *ReviewController {
	return &ReviewController{service: service.NewReviewService()}
}

// My2SQLStatus 获取my2sql工具状态
// @Summary 获取my2sql工具状态
// @Description 检测执行节点 /opt/db/my2sql 或 AIOPS_MY2SQL_PATH 指定的my2sql是否已安装且可执行
// @Tags 数据库工具
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/tools/my2sql/status [get]
// @Security ApiKeyAuth
func (c *ReviewController) My2SQLStatus(ctx *gin.Context) {
	result.Success(ctx, c.service.My2SQLStatus(ctx.Request.Context()))
}

// GhOstStatus 获取gh-ost工具状态
// @Summary 获取gh-ost工具状态
// @Description 检测执行节点 /opt/db/gh-ost 或 AIOPS_GHOST_PATH 指定的gh-ost是否已安装且可执行
// @Tags 数据库工具
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/tools/gh-ost/status [get]
// @Security ApiKeyAuth
func (c *ReviewController) GhOstStatus(ctx *gin.Context) {
	result.Success(ctx, c.service.GhOstStatus(ctx.Request.Context()))
}

// PgRollStatus 获取pgroll工具状态
// @Summary 获取pgroll工具状态
// @Description 检测执行节点 /opt/db/pgroll 或 AIOPS_PGROLL_PATH 指定的pgroll是否已安装且可执行
// @Tags 数据库工具
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/tools/pgroll/status [get]
// @Security ApiKeyAuth
func (c *ReviewController) PgRollStatus(ctx *gin.Context) {
	result.Success(ctx, c.service.PgRollStatus(ctx.Request.Context()))
}

// WalMinerStatus 获取WalMiner工具状态
// @Summary 获取WalMiner工具状态
// @Description 检测执行节点 /opt/db/walminer 或 AIOPS_WALMINER_PATH 指定的WalMiner是否已安装且可执行
// @Tags 数据库工具
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/tools/walminer/status [get]
// @Security ApiKeyAuth
func (c *ReviewController) WalMinerStatus(ctx *gin.Context) {
	result.Success(ctx, c.service.WalMinerStatus(ctx.Request.Context()))
}

// ToolboxStatus 按数据库类型获取工具箱状态
// @Summary 按数据库类型获取工具箱状态
// @Description 根据 assetId 或 dbType 返回对应数据库类型可用工具状态
// @Tags 数据库工具
// @Produce json
// @Param assetId query int false "数据库资产ID"
// @Param dbType query int false "数据库类型；1=MySQL，2=PostgreSQL；不传则返回全部"
// @Success 200 {object} result.Result
// @Router /api/v1/db/tools/status [get]
// @Security ApiKeyAuth
func (c *ReviewController) ToolboxStatus(ctx *gin.Context) {
	assetID, _ := strconv.Atoi(ctx.DefaultQuery("assetId", "0"))
	dbType, _ := strconv.Atoi(ctx.DefaultQuery("dbType", "0"))
	data, err := c.service.ToolboxStatus(ctx.Request.Context(), uint(assetID), dbType)
	if err != nil {
		result.Failed(ctx, 500, "获取数据库工具箱状态失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// CheckDML 自动检测DML SQL
// @Summary 自动检测DML SQL
// @Description 对DML工单SQL执行基础规则检测，返回语句拆分、风险等级和审核结果
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewCheckRequest true "检测参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/dml/check [post]
// @Security ApiKeyAuth
func (c *ReviewController) CheckDML(ctx *gin.Context) {
	c.check(ctx, dbmodel.SQLTicketTypeDML)
}

// CheckDDL 自动检测DDL SQL
// @Summary 自动检测DDL SQL
// @Description 对DDL工单SQL执行基础规则检测，返回语句拆分、风险等级和审核结果
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewCheckRequest true "检测参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/ddl/check [post]
// @Security ApiKeyAuth
func (c *ReviewController) CheckDDL(ctx *gin.Context) {
	c.check(ctx, dbmodel.SQLTicketTypeDDL)
}

// CheckQuery 自动检测查询SQL
// @Summary 自动检测查询SQL
// @Description 对查询工单SQL执行基础规则检测，只允许SELECT/WITH
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewCheckRequest true "检测参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/query/check [post]
// @Security ApiKeyAuth
func (c *ReviewController) CheckQuery(ctx *gin.Context) {
	c.check(ctx, dbmodel.SQLTicketTypeQuery)
}

// SubmitDML 提交DML工单
// @Summary 提交DML工单
// @Description 提交DML SQL审核工单，流程为发起人提交、审批人审批、执行人执行
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewSubmitRequest true "提交参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/dml/tickets [post]
// @Security ApiKeyAuth
func (c *ReviewController) SubmitDML(ctx *gin.Context) {
	c.submit(ctx, dbmodel.SQLTicketTypeDML)
}

// SubmitDDL 提交DDL工单
// @Summary 提交DDL工单
// @Description 提交DDL SQL审核工单，流程为发起人提交、审批人审批、执行人执行
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewSubmitRequest true "提交参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/ddl/tickets [post]
// @Security ApiKeyAuth
func (c *ReviewController) SubmitDDL(ctx *gin.Context) {
	c.submit(ctx, dbmodel.SQLTicketTypeDDL)
}

// SubmitQuery 提交查询工单
// @Summary 提交查询工单
// @Description 提交 SELECT/WITH 查询审核工单，审批通过后由执行人执行
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLReviewSubmitRequest true "提交参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/query/tickets [post]
// @Security ApiKeyAuth
func (c *ReviewController) SubmitQuery(ctx *gin.Context) {
	c.submit(ctx, dbmodel.SQLTicketTypeQuery)
}

// ListDML DML工单列表
// @Summary DML工单列表
// @Description 分页查询DML SQL审核工单
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/dml/tickets [get]
// @Security ApiKeyAuth
func (c *ReviewController) ListDML(ctx *gin.Context) {
	c.list(ctx, dbmodel.SQLTicketTypeDML)
}

// ListDDL DDL工单列表
// @Summary DDL工单列表
// @Description 分页查询DDL SQL审核工单
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/ddl/tickets [get]
// @Security ApiKeyAuth
func (c *ReviewController) ListDDL(ctx *gin.Context) {
	c.list(ctx, dbmodel.SQLTicketTypeDDL)
}

// ListQuery 查询工单列表
// @Summary 查询工单列表
// @Description 分页查询 SELECT/WITH 查询审核工单
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/query/tickets [get]
// @Security ApiKeyAuth
func (c *ReviewController) ListQuery(ctx *gin.Context) {
	c.list(ctx, dbmodel.SQLTicketTypeQuery)
}

func (c *ReviewController) ListTickets(ctx *gin.Context) {
	var query dbmodel.SQLTicketQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	list, total, err := c.service.List(query)
	if err != nil {
		result.Failed(ctx, 500, "query sql tickets failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

func (c *ReviewController) Detail(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	data, err := c.service.Detail(id)
	if err != nil {
		result.Failed(ctx, 500, "query sql ticket detail failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *ReviewController) Approve(ctx *gin.Context) {
	c.action(ctx, "approve")
}

func (c *ReviewController) Reject(ctx *gin.Context) {
	c.action(ctx, "reject")
}

func (c *ReviewController) Cancel(ctx *gin.Context) {
	c.action(ctx, "cancel")
}

func (c *ReviewController) Execute(ctx *gin.Context) {
	c.action(ctx, "execute")
}

func (c *ReviewController) Logs(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	data, err := c.service.Detail(id)
	if err != nil {
		result.Failed(ctx, 500, "query sql ticket logs failed: "+err.Error())
		return
	}
	result.Success(ctx, data.ExecutionRecords)
}

func (c *ReviewController) Rollback(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	data, err := c.service.Detail(id)
	if err != nil {
		result.Failed(ctx, 500, "query rollback sql failed: "+err.Error())
		return
	}
	result.Success(ctx, data.RollbackRecords)
}

// CreateRollbackTicket 基于回滚SQL生成回滚工单
// @Summary 基于回滚SQL生成回滚工单
// @Description 将原工单生成的可用回滚SQL转化为新的SQL审核工单，用于故障回滚变更
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLRollbackTicketRequest true "回滚工单参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/tickets/{id}/rollback/tickets [post]
// @Security ApiKeyAuth
func (c *ReviewController) CreateRollbackTicket(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	var req dbmodel.SQLRollbackTicketRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.CreateRollbackTicket(ctx, id, req)
	if err != nil {
		result.Failed(ctx, 500, "生成回滚工单失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// CleanupBackupTables 一键清理工单备份表
// @Summary 一键清理工单备份表
// @Description 删除该工单回滚记录中登记的 _aiops_backup_ 备份表
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLBackupCleanupRequest true "清理参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/tickets/{id}/backup/cleanup [post]
// @Security ApiKeyAuth
func (c *ReviewController) CleanupBackupTables(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	var req dbmodel.SQLBackupCleanupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.CleanupBackupTables(ctx, id, req)
	if err != nil {
		result.Failed(ctx, 500, "清理备份表失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// GenerateFlashbackRollback 生成binlog闪回回滚SQL
// @Summary 生成binlog闪回回滚SQL
// @Description 根据DML工单执行时记录的binlog起止位点调用my2sql生成反向回滚SQL
// @Tags 数据库SQL审核
// @Accept json
// @Produce json
// @Param request body model.SQLFlashbackGenerateRequest true "生成参数"
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/tickets/{id}/rollback/flashback [post]
// @Security ApiKeyAuth
func (c *ReviewController) GenerateFlashbackRollback(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	var req dbmodel.SQLFlashbackGenerateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.GenerateFlashbackRollback(ctx, id, req)
	if err != nil {
		result.Failed(ctx, 500, "生成binlog闪回回滚SQL失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// QueryResult 获取查询工单结果
// @Summary 获取查询工单结果
// @Description 获取查询工单执行时保存的结构化查询结果
// @Tags 数据库SQL审核
// @Produce json
// @Success 200 {object} result.Result
// @Router /api/v1/db/review/tickets/{id}/result [get]
// @Security ApiKeyAuth
func (c *ReviewController) QueryResult(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	data, err := c.service.QueryResult(ctx, id)
	if err != nil {
		result.Failed(ctx, 500, "查询结果获取失败: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

// ExportResult 导出查询工单结果 CSV
// @Summary 导出查询工单结果 CSV
// @Description 导出查询工单执行时保存的查询结果，不会重新执行SQL
// @Tags 数据库SQL审核
// @Produce text/csv
// @Success 200 {file} file
// @Router /api/v1/db/review/tickets/{id}/result/export [get]
// @Security ApiKeyAuth
func (c *ReviewController) ExportResult(ctx *gin.Context) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	fileName := buildReviewCSVFileName(fmt.Sprintf("ticket_%d_result", id))
	ctx.Header("Content-Type", "text/csv; charset=utf-8")
	ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	ctx.Status(http.StatusOK)
	if _, err := ctx.Writer.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	if _, err := c.service.ExportResultCSV(ctx, id, ctx.Writer); err != nil {
		ctx.Header("Content-Disposition", "")
		result.Failed(ctx, 500, "导出查询结果失败: "+err.Error())
		return
	}
}

func (c *ReviewController) check(ctx *gin.Context, ticketType string) {
	var req dbmodel.SQLReviewCheckRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Check(ctx, ticketType, req)
	if err != nil {
		result.Failed(ctx, 500, "check sql failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *ReviewController) submit(ctx *gin.Context, ticketType string) {
	var req dbmodel.SQLReviewSubmitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid request body")
		return
	}
	data, err := c.service.Submit(ctx, ticketType, req)
	if err != nil {
		result.Failed(ctx, 500, "submit sql ticket failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func (c *ReviewController) list(ctx *gin.Context, ticketType string) {
	var query dbmodel.SQLTicketQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid query params")
		return
	}
	query.TicketType = ticketType
	list, total, err := c.service.List(query)
	if err != nil {
		result.Failed(ctx, 500, "query sql tickets failed: "+err.Error())
		return
	}
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	result.SuccessWithPage(ctx, list, total, query.Page, query.PageSize)
}

func (c *ReviewController) action(ctx *gin.Context, action string) {
	id, ok := parseReviewID(ctx)
	if !ok {
		return
	}
	var req dbmodel.SQLTicketActionRequest
	_ = ctx.ShouldBindJSON(&req)
	var (
		data interface{}
		err  error
	)
	switch action {
	case "approve":
		data, err = c.service.Approve(ctx, id, req)
	case "reject":
		data, err = c.service.Reject(ctx, id, req)
	case "cancel":
		data, err = c.service.Cancel(ctx, id, req)
	case "execute":
		data, err = c.service.Execute(ctx, id, req)
	}
	if err != nil {
		result.Failed(ctx, 500, action+" sql ticket failed: "+err.Error())
		return
	}
	result.Success(ctx, data)
}

func parseReviewID(ctx *gin.Context) (uint, bool) {
	value := ctx.Param("id")
	id, err := strconv.ParseUint(value, 10, 32)
	if err != nil || id == 0 {
		result.Failed(ctx, constant.INVALID_PARAMS, "invalid id")
		return 0, false
	}
	return uint(id), true
}

func buildReviewCSVFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "db_review_result"
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	return fmt.Sprintf("%s_%s.csv", name, time.Now().Format("20060102150405"))
}
