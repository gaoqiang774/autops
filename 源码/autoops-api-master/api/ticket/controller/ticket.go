package controller

import (
	"net/http"
	"strconv"

	"dodevops-api/api/ticket/dao"
	"dodevops-api/api/ticket/model"
	"dodevops-api/api/ticket/service"
	"dodevops-api/common"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TicketController struct {
	db       *gorm.DB
	workflow *service.WorkflowService
	daily    *service.DailyService
	release  *service.ReleaseService
}

func NewTicketController() *TicketController {
	db := common.GetDB()
	return &TicketController{db: db, workflow: service.NewWorkflowService(db), daily: service.NewDailyService(db), release: service.NewReleaseService(db)}
}

func (c *TicketController) Overview(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	var pending, dailyOpen, releaseRunning, sqlPending int64
	c.db.Model(&model.ApprovalTask{}).Where("assignee_id = ? AND status = ?", userID, model.ApprovalTaskPending).Count(&pending)
	c.db.Model(&model.Ticket{}).Where("business_type = ? AND status NOT IN ?", model.BusinessTypeDaily, []string{model.DailyStatusClosed, model.DailyStatusCanceled, model.DailyStatusRejected}).Count(&dailyOpen)
	c.db.Model(&model.Ticket{}).Where("business_type = ? AND status IN ?", model.BusinessTypeRelease, []string{model.ReleaseStatusPending, model.ReleaseStatusApproved, model.ReleaseStatusRunning}).Count(&releaseRunning)
	c.db.Table("db_sql_ticket").Where("status = ?", "pending_review").Count(&sqlPending)
	result.Success(ctx, gin.H{"pendingApprovals": pending, "dailyOpen": dailyOpen, "releaseActive": releaseRunning, "sqlPending": sqlPending})
}

func (c *TicketController) ListWorkflows(ctx *gin.Context) {
	query := pageQuery(ctx)
	enabled := optionalInt(ctx.Query("enabled"))
	list, total, err := c.workflow.List(query, enabled)
	if err != nil {
		fail(ctx, err)
		return
	}
	successPage(ctx, list, total, query)
}

func (c *TicketController) GetWorkflow(ctx *gin.Context) {
	item, err := c.workflow.Get(pathID(ctx))
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) SaveWorkflow(ctx *gin.Context) {
	var input service.WorkflowDefinitionInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	item, err := c.workflow.Save(pathIDOptional(ctx), input, userID, userName)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) DeleteWorkflow(ctx *gin.Context) {
	if err := c.workflow.Delete(pathID(ctx)); err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, true)
}

func (c *TicketController) ChangeWorkflowStatus(ctx *gin.Context) {
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	enabled, _ := strconv.Atoi(ctx.DefaultQuery("enabled", "1"))
	if err := c.workflow.ChangeEnabled(pathID(ctx), enabled, userID, userName); err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, true)
}

func (c *TicketController) WorkflowDetail(ctx *gin.Context) {
	item, err := c.workflow.Detail(pathID(ctx))
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) MyApprovalTasks(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	query := pageQuery(ctx)
	list, total, err := c.workflow.MyTasks(userID, query, ctx.Query("status"), ctx.Query("businessType"))
	if err != nil {
		fail(ctx, err)
		return
	}
	successPage(ctx, list, total, query)
}

func (c *TicketController) ApproveTask(ctx *gin.Context) { c.approvalAction(ctx, true) }
func (c *TicketController) RejectTask(ctx *gin.Context)  { c.approvalAction(ctx, false) }

func (c *TicketController) approvalAction(ctx *gin.Context, approve bool) {
	var input service.WorkflowActionInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	var item *model.WorkflowInstance
	var err error
	if approve {
		item, err = c.workflow.Approve(pathID(ctx), userID, userName, input.Comment)
	} else {
		item, err = c.workflow.Reject(pathID(ctx), userID, userName, input.Comment)
	}
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) CreateDaily(ctx *gin.Context) {
	var input service.DailyCreateInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	item, err := c.daily.Create(input, userID, userName)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) ListDaily(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	base := pageQuery(ctx)
	query := dao.DailyQuery{PageQuery: base, Category: ctx.Query("category"), Environment: ctx.Query("environment"), Priority: ctx.Query("priority"), Status: ctx.Query("status"), Scope: ctx.Query("scope"), CurrentUserID: userID}
	list, total, err := c.daily.List(query)
	if err != nil {
		fail(ctx, err)
		return
	}
	successPage(ctx, list, total, base)
}

func (c *TicketController) DailyDetail(ctx *gin.Context) {
	item, err := c.daily.Detail(pathID(ctx))
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) AssignDaily(ctx *gin.Context) {
	var input service.DailyAssignInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	item, err := c.daily.Assign(pathID(ctx), input, userID, userName)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) StartDaily(ctx *gin.Context)   { c.dailyAction(ctx, "start") }
func (c *TicketController) ResolveDaily(ctx *gin.Context) { c.dailyAction(ctx, "resolve") }
func (c *TicketController) ConfirmDaily(ctx *gin.Context) { c.dailyAction(ctx, "confirm") }
func (c *TicketController) CloseDaily(ctx *gin.Context)   { c.dailyAction(ctx, "close") }
func (c *TicketController) CancelDaily(ctx *gin.Context)  { c.dailyAction(ctx, "cancel") }

func (c *TicketController) dailyAction(ctx *gin.Context, action string) {
	var input service.DailyActionInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	var item *model.DailyTicket
	var err error
	id := pathID(ctx)
	switch action {
	case "start":
		item, err = c.daily.Start(id, userID, userName, input.Comment)
	case "resolve":
		item, err = c.daily.Resolve(id, userID, userName, input.Comment)
	case "confirm":
		item, err = c.daily.Confirm(id, userID, userName, input.Comment)
	case "close":
		item, err = c.daily.Close(id, userID, userName, input.Comment)
	case "cancel":
		item, err = c.daily.Cancel(id, userID, userName, input.Comment)
	}
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) ListApplications(ctx *gin.Context) {
	query := pageQuery(ctx)
	list, total, err := c.release.ListApplications(query, ctx.Query("environment"), optionalInt(ctx.Query("enabled")))
	if err != nil {
		fail(ctx, err)
		return
	}
	successPage(ctx, list, total, query)
}

func (c *TicketController) SaveApplication(ctx *gin.Context) {
	var input service.ApplicationInput
	if !bind(ctx, &input) {
		return
	}
	item, err := c.release.SaveApplication(pathIDOptional(ctx), input)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) DeleteApplication(ctx *gin.Context) {
	if err := c.release.DeleteApplication(pathID(ctx)); err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, true)
}

func (c *TicketController) CreateRelease(ctx *gin.Context) {
	var input service.ReleaseCreateInput
	if !bind(ctx, &input) {
		return
	}
	userID, userName, ok := currentUser(ctx)
	if !ok {
		return
	}
	item, err := c.release.Create(input, userID, userName)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func (c *TicketController) ListReleases(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	base := pageQuery(ctx)
	query := dao.ReleaseQuery{PageQuery: base, Environment: ctx.Query("environment"), Status: ctx.Query("status"), Scope: ctx.Query("scope"), CurrentUserID: userID}
	list, total, err := c.release.List(query)
	if err != nil {
		fail(ctx, err)
		return
	}
	successPage(ctx, list, total, base)
}

func (c *TicketController) ReleaseDetail(ctx *gin.Context) {
	item, workflow, err := c.release.Detail(pathID(ctx))
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, gin.H{"ticket": item, "workflow": workflow})
}

func (c *TicketController) ExecuteRelease(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	item, err := c.release.Execute(ctx.Request.Context(), pathID(ctx), userID)
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}
func (c *TicketController) RetryReleaseItem(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	if err := c.release.RetryItem(ctx.Request.Context(), pathID(ctx), pathSubID(ctx, "itemId"), userID); err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, true)
}
func (c *TicketController) CancelReleaseItem(ctx *gin.Context) {
	userID, _, ok := currentUser(ctx)
	if !ok {
		return
	}
	if err := c.release.CancelItem(ctx.Request.Context(), pathID(ctx), pathSubID(ctx, "itemId"), userID); err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, true)
}
func (c *TicketController) ReleaseItemLog(ctx *gin.Context) {
	item, err := c.release.Log(ctx.Request.Context(), pathID(ctx), pathSubID(ctx, "itemId"))
	if err != nil {
		fail(ctx, err)
		return
	}
	result.Success(ctx, item)
}

func currentUser(ctx *gin.Context) (uint, string, bool) {
	admin, err := jwt.GetAdmin(ctx)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return 0, "", false
	}
	id, name, err := service.CurrentUser(admin)
	if err != nil {
		result.Failed(ctx, http.StatusUnauthorized, err.Error())
		return 0, "", false
	}
	return id, name, true
}

func bind(ctx *gin.Context, input interface{}) bool {
	if err := ctx.ShouldBindJSON(input); err != nil {
		result.Failed(ctx, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}
func fail(ctx *gin.Context, err error) {
	status := http.StatusBadRequest
	if err == gorm.ErrRecordNotFound {
		status = http.StatusNotFound
	}
	result.Failed(ctx, status, err.Error())
}
func pathID(ctx *gin.Context) uint {
	id, _ := strconv.ParseUint(ctx.Param("id"), 10, 32)
	return uint(id)
}
func pathSubID(ctx *gin.Context, key string) uint {
	id, _ := strconv.ParseUint(ctx.Param(key), 10, 32)
	return uint(id)
}
func pathIDOptional(ctx *gin.Context) uint {
	if ctx.Param("id") == "" {
		return 0
	}
	return pathID(ctx)
}
func pageQuery(ctx *gin.Context) model.PageQuery {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("pageSize", "10"))
	page, size = model.NormalizePage(page, size)
	return model.PageQuery{Page: page, PageSize: size, Keyword: ctx.Query("keyword")}
}
func normalizedPage(query model.PageQuery) (int, int) {
	return model.NormalizePage(query.Page, query.PageSize)
}
func optionalInt(value string) *int {
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &parsed
}
func successPage(ctx *gin.Context, list interface{}, total int64, query model.PageQuery) {
	page, size := normalizedPage(query)
	result.SuccessWithPage(ctx, list, total, page, size)
}
