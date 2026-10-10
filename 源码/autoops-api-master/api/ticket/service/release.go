package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	configdao "dodevops-api/api/configcenter/dao"
	configmodel "dodevops-api/api/configcenter/model"
	"dodevops-api/api/ticket/dao"
	"dodevops-api/api/ticket/model"
	"dodevops-api/common/util"
	"dodevops-api/pkg/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const jenkinsAccountType = 4

var jenkinsQueuePattern = regexp.MustCompile(`/queue/item/(\d+)/?`)

type ReleaseService struct {
	dao      *dao.TicketDao
	workflow *WorkflowService
}

type ApplicationInput struct {
	Name              string        `json:"name"`
	Code              string        `json:"code"`
	SystemName        string        `json:"systemName"`
	Environment       string        `json:"environment"`
	JenkinsAccountID  uint          `json:"jenkinsAccountId"`
	JenkinsJobName    string        `json:"jenkinsJobName"`
	DefaultParameters model.JSONMap `json:"defaultParameters"`
	Description       string        `json:"description"`
	Enabled           *int          `json:"enabled"`
}

type ReleaseCreateInput struct {
	Title        string                   `json:"title"`
	Environment  string                   `json:"environment"`
	Strategy     string                   `json:"strategy"`
	Description  string                   `json:"description"`
	RollbackPlan string                   `json:"rollbackPlan"`
	WorkflowID   uint                     `json:"workflowId"`
	ExecutorID   uint                     `json:"executorId"`
	Items        []ReleaseCreateItemInput `json:"items"`
}

type ReleaseCreateItemInput struct {
	ApplicationID uint          `json:"applicationId"`
	Version       string        `json:"version"`
	Target        string        `json:"target"`
	Parameters    model.JSONMap `json:"parameters"`
	ExecuteOrder  int           `json:"executeOrder"`
}

type JenkinsLog struct {
	ItemID      uint   `json:"itemId"`
	BuildNumber int64  `json:"buildNumber"`
	BuildURL    string `json:"buildUrl"`
	Content     string `json:"content"`
}

func NewReleaseService(db *gorm.DB) *ReleaseService {
	return &ReleaseService{dao: dao.NewTicketDao(db), workflow: NewWorkflowService(db)}
}

func (s *ReleaseService) ListApplications(query model.PageQuery, environment string, enabled *int) ([]model.ReleaseApplication, int64, error) {
	return s.dao.ListApplications(query, environment, enabled)
}

func (s *ReleaseService) SaveApplication(id uint, input ApplicationInput) (*model.ReleaseApplication, error) {
	item, err := normalizeApplicationInput(input)
	if err != nil {
		return nil, err
	}
	if _, err := loadJenkinsAccount(item.JenkinsAccountID); err != nil {
		return nil, err
	}
	now := util.HTime{Time: time.Now()}
	if id == 0 {
		item.CreatedAt, item.UpdatedAt = now, now
		if err := s.dao.DB().Create(&item).Error; err != nil {
			return nil, err
		}
	} else {
		item.ID, item.UpdatedAt = id, now
		if err := s.dao.DB().Model(&model.ReleaseApplication{}).Where("id = ?", id).Select("name", "code", "system_name", "environment", "jenkins_account_id", "jenkins_job_name", "default_parameters", "description", "enabled", "updated_at").Updates(&item).Error; err != nil {
			return nil, err
		}
	}
	return &item, nil
}

func (s *ReleaseService) DeleteApplication(id uint) error {
	var count int64
	if err := s.dao.DB().Model(&model.ReleaseItem{}).Where("application_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("应用已被发布工单使用，不能删除")
	}
	return s.dao.DB().Delete(&model.ReleaseApplication{}, id).Error
}

func (s *ReleaseService) Create(input ReleaseCreateInput, applicantID uint, applicantName string) (*model.ReleaseTicket, error) {
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("发布标题不能为空")
	}
	if input.WorkflowID == 0 {
		return nil, fmt.Errorf("发布工单必须选择审批流")
	}
	if input.ExecutorID == 0 {
		return nil, fmt.Errorf("请选择发布执行人")
	}
	if len(input.Items) == 0 {
		return nil, fmt.Errorf("至少选择一个发布应用")
	}
	executorID, executorName, err := resolveSystemUser(s.dao.DB(), input.ExecutorID)
	if err != nil {
		return nil, err
	}
	environment := strings.ToLower(strings.TrimSpace(input.Environment))
	if environment == "" {
		return nil, fmt.Errorf("发布环境不能为空")
	}
	strategy := strings.ToLower(strings.TrimSpace(input.Strategy))
	if strategy == "" {
		strategy = "serial"
	}
	if strategy != "serial" && strategy != "parallel" {
		return nil, fmt.Errorf("发布策略仅支持 serial 或 parallel")
	}
	items, err := s.buildReleaseItems(input.Items, environment)
	if err != nil {
		return nil, err
	}
	now := util.HTime{Time: time.Now()}
	item := model.ReleaseTicket{TicketNo: fmt.Sprintf("TMP-REL-%d", time.Now().UnixNano()), Title: strings.TrimSpace(input.Title), BusinessType: model.BusinessTypeRelease, Environment: environment, Strategy: strategy, Description: strings.TrimSpace(input.Description), RollbackPlan: strings.TrimSpace(input.RollbackPlan), Status: model.ReleaseStatusPending, ApplicantID: applicantID, ApplicantName: applicantName, ExecutorID: executorID, ExecutorName: executorName, CreatedAt: now, UpdatedAt: now}
	err = s.dao.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		item.TicketNo = fmt.Sprintf("REL-%s-%06d", now.Format("20060102"), item.ID)
		if err := tx.Model(&item).Update("ticket_no", item.TicketNo).Error; err != nil {
			return err
		}
		for index := range items {
			items[index].ReleaseID = item.ID
			items[index].CreatedAt, items[index].UpdatedAt = now, now
		}
		if err := tx.Create(&items).Error; err != nil {
			return err
		}
		instance, err := s.workflow.StartWithTx(tx, WorkflowStartInput{DefinitionID: input.WorkflowID, BusinessType: model.BusinessTypeRelease, BusinessID: item.ID, BusinessNo: item.TicketNo, Environment: item.Environment, ApplicantID: applicantID, ApplicantName: applicantName})
		if err != nil {
			return err
		}
		item.WorkflowInstanceID = instance.ID
		return tx.Model(&item).Update("workflow_instance_id", instance.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return s.dao.GetReleaseTicket(item.ID)
}

func (s *ReleaseService) List(query dao.ReleaseQuery) ([]model.ReleaseTicket, int64, error) {
	return s.dao.ListReleaseTickets(query)
}

func (s *ReleaseService) Detail(id uint) (*model.ReleaseTicket, *WorkflowDetail, error) {
	item, err := s.dao.GetReleaseTicket(id)
	if err != nil {
		return nil, nil, err
	}
	var workflow *WorkflowDetail
	if item.WorkflowInstanceID > 0 {
		workflow, _ = s.workflow.Detail(item.WorkflowInstanceID)
	}
	return item, workflow, nil
}

func (s *ReleaseService) Execute(ctx context.Context, id, operatorID uint) (*model.ReleaseTicket, error) {
	var release model.ReleaseTicket
	err := s.dao.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("business_type = ?", model.BusinessTypeRelease).First(&release, id).Error; err != nil {
			return err
		}
		if release.Status != model.ReleaseStatusApproved && release.Status != model.ReleaseStatusFailed {
			return fmt.Errorf("发布工单尚未审批通过或正在执行")
		}
		if release.ExecutorID != operatorID {
			return fmt.Errorf("只有指定执行人可以执行发布")
		}
		now := util.HTime{Time: time.Now()}
		if err := tx.Model(&release).Updates(map[string]interface{}{"status": model.ReleaseStatusRunning, "started_at": now, "finished_at": nil, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Where("release_id = ? AND status IN ?", release.ID, []string{model.ReleaseItemFailed, model.ReleaseItemCanceled}).Model(&model.ReleaseItem{}).Updates(map[string]interface{}{"status": model.ReleaseItemPending, "queue_id": 0, "build_number": 0, "build_url": "", "error_message": "", "started_at": nil, "finished_at": nil, "updated_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	items, err := s.pendingReleaseItems(release.ID)
	if err != nil {
		return nil, err
	}
	if release.Strategy == "parallel" {
		for index := range items {
			if err := s.triggerItem(ctx, &items[index]); err != nil {
				s.failReleaseItem(items[index].ID, err)
			}
		}
	} else if len(items) > 0 {
		if err := s.triggerItem(ctx, &items[0]); err != nil {
			s.failReleaseItem(items[0].ID, err)
		}
	}
	s.refreshReleaseStatus(release.ID)
	return s.dao.GetReleaseTicket(release.ID)
}

func (s *ReleaseService) RetryItem(ctx context.Context, releaseID, itemID, operatorID uint) error {
	release, err := s.dao.GetReleaseTicket(releaseID)
	if err != nil {
		return err
	}
	if release.ExecutorID != operatorID {
		return fmt.Errorf("只有指定执行人可以重试发布")
	}
	var item model.ReleaseItem
	if err := s.dao.DB().Where("id = ? AND release_id = ?", itemID, releaseID).First(&item).Error; err != nil {
		return err
	}
	if item.Status != model.ReleaseItemFailed && item.Status != model.ReleaseItemCanceled {
		return fmt.Errorf("当前发布项不能重试")
	}
	now := util.HTime{Time: time.Now()}
	if err := s.dao.DB().Model(&model.ReleaseTicket{}).Where("id = ? AND business_type = ?", releaseID, model.BusinessTypeRelease).Updates(map[string]interface{}{"status": model.ReleaseStatusRunning, "finished_at": nil, "updated_at": now}).Error; err != nil {
		return err
	}
	if err := s.dao.DB().Model(&item).Updates(map[string]interface{}{"status": model.ReleaseItemPending, "queue_id": 0, "build_number": 0, "build_url": "", "error_message": "", "started_at": nil, "finished_at": nil, "updated_at": now}).Error; err != nil {
		return err
	}
	return s.triggerItem(ctx, &item)
}

func (s *ReleaseService) CancelItem(ctx context.Context, releaseID, itemID, operatorID uint) error {
	release, err := s.dao.GetReleaseTicket(releaseID)
	if err != nil {
		return err
	}
	if release.ExecutorID != operatorID {
		return fmt.Errorf("只有指定执行人可以中止发布")
	}
	var item model.ReleaseItem
	if err := s.dao.DB().Where("id = ? AND release_id = ?", itemID, releaseID).First(&item).Error; err != nil {
		return err
	}
	client, err := newJenkinsClient(item.JenkinsAccountID)
	if err != nil {
		return err
	}
	if item.BuildNumber > 0 {
		path := jenkinsJobPath(item.JenkinsJobName) + "/" + strconv.FormatInt(item.BuildNumber, 10) + "/stop"
		_, _ = client.request(ctx, http.MethodPost, path, nil)
	} else if item.QueueID > 0 {
		_, _ = client.request(ctx, http.MethodPost, "/queue/cancelItem?id="+strconv.FormatInt(item.QueueID, 10), nil)
	}
	now := util.HTime{Time: time.Now()}
	if err := s.dao.DB().Model(&item).Updates(map[string]interface{}{"status": model.ReleaseItemCanceled, "finished_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	s.refreshReleaseStatus(releaseID)
	return nil
}

func (s *ReleaseService) Log(ctx context.Context, releaseID, itemID uint) (*JenkinsLog, error) {
	var item model.ReleaseItem
	if err := s.dao.DB().Where("id = ? AND release_id = ?", itemID, releaseID).First(&item).Error; err != nil {
		return nil, err
	}
	if item.BuildNumber == 0 {
		return nil, fmt.Errorf("Jenkins 构建尚未开始")
	}
	client, err := newJenkinsClient(item.JenkinsAccountID)
	if err != nil {
		return nil, err
	}
	response, err := client.request(ctx, http.MethodGet, jenkinsJobPath(item.JenkinsJobName)+"/"+strconv.FormatInt(item.BuildNumber, 10)+"/consoleText", nil)
	if err != nil {
		return nil, err
	}
	return &JenkinsLog{ItemID: item.ID, BuildNumber: item.BuildNumber, BuildURL: item.BuildURL, Content: string(response)}, nil
}

func StartReleaseSynchronizer(ctx context.Context, db *gorm.DB) {
	service := NewReleaseService(db)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := service.syncRunning(ctx); err != nil {
				log.Log().Warnf("release status sync failed: %v", err)
			}
		}
	}
}

func (s *ReleaseService) syncRunning(ctx context.Context) error {
	var items []model.ReleaseItem
	if err := s.dao.DB().Where("status IN ?", []string{model.ReleaseItemQueued, model.ReleaseItemRunning}).Order("id ASC").Limit(100).Find(&items).Error; err != nil {
		return err
	}
	affected := map[uint]struct{}{}
	for index := range items {
		if err := s.syncItem(ctx, &items[index]); err != nil {
			log.Log().Warnf("sync Jenkins item %d failed: %v", items[index].ID, err)
			continue
		}
		affected[items[index].ReleaseID] = struct{}{}
	}
	for releaseID := range affected {
		s.refreshReleaseStatus(releaseID)
		s.triggerNextSerial(ctx, releaseID)
	}
	return nil
}

func (s *ReleaseService) syncItem(ctx context.Context, item *model.ReleaseItem) error {
	client, err := newJenkinsClient(item.JenkinsAccountID)
	if err != nil {
		return err
	}
	now := util.HTime{Time: time.Now()}
	if item.BuildNumber == 0 {
		data, err := client.request(ctx, http.MethodGet, "/queue/item/"+strconv.FormatInt(item.QueueID, 10)+"/api/json", nil)
		if err != nil {
			return err
		}
		var queue struct {
			Cancelled  bool `json:"cancelled"`
			Executable *struct {
				Number int64  `json:"number"`
				URL    string `json:"url"`
			} `json:"executable"`
		}
		if err := json.Unmarshal(data, &queue); err != nil {
			return err
		}
		if queue.Cancelled {
			return s.dao.DB().Model(item).Updates(map[string]interface{}{"status": model.ReleaseItemCanceled, "finished_at": now, "updated_at": now}).Error
		}
		if queue.Executable == nil {
			return nil
		}
		item.BuildNumber, item.BuildURL, item.Status = queue.Executable.Number, queue.Executable.URL, model.ReleaseItemRunning
		return s.dao.DB().Model(item).Updates(map[string]interface{}{"build_number": item.BuildNumber, "build_url": item.BuildURL, "status": item.Status, "updated_at": now}).Error
	}
	data, err := client.request(ctx, http.MethodGet, jenkinsJobPath(item.JenkinsJobName)+"/"+strconv.FormatInt(item.BuildNumber, 10)+"/api/json", nil)
	if err != nil {
		return err
	}
	var build struct {
		Building bool   `json:"building"`
		Result   string `json:"result"`
		URL      string `json:"url"`
	}
	if err := json.Unmarshal(data, &build); err != nil {
		return err
	}
	if build.Building || build.Result == "" {
		return nil
	}
	status := model.ReleaseItemFailed
	errorMessage := "Jenkins 构建结果: " + build.Result
	if strings.EqualFold(build.Result, "SUCCESS") {
		status, errorMessage = model.ReleaseItemSucceeded, ""
	}
	if strings.EqualFold(build.Result, "ABORTED") {
		status = model.ReleaseItemCanceled
	}
	item.Status = status
	return s.dao.DB().Model(item).Updates(map[string]interface{}{"status": status, "build_url": build.URL, "error_message": errorMessage, "finished_at": now, "updated_at": now}).Error
}

func (s *ReleaseService) triggerNextSerial(ctx context.Context, releaseID uint) {
	release, err := s.dao.GetReleaseTicket(releaseID)
	if err != nil || release.Strategy != "serial" || release.Status != model.ReleaseStatusRunning {
		return
	}
	for _, item := range release.Items {
		if item.Status == model.ReleaseItemQueued || item.Status == model.ReleaseItemRunning || item.Status == model.ReleaseItemFailed || item.Status == model.ReleaseItemCanceled {
			return
		}
	}
	for index := range release.Items {
		if release.Items[index].Status == model.ReleaseItemPending {
			if err := s.triggerItem(ctx, &release.Items[index]); err != nil {
				s.failReleaseItem(release.Items[index].ID, err)
			}
			return
		}
	}
}

func (s *ReleaseService) triggerItem(ctx context.Context, item *model.ReleaseItem) error {
	client, err := newJenkinsClient(item.JenkinsAccountID)
	if err != nil {
		return err
	}
	parameters := url.Values{}
	for key, value := range item.Parameters {
		parameters.Set(key, fmt.Sprint(value))
	}
	path := jenkinsJobPath(item.JenkinsJobName) + "/build"
	if len(parameters) > 0 {
		path += "WithParameters"
	}
	_, location, err := client.trigger(ctx, path, parameters)
	if err != nil {
		return err
	}
	matches := jenkinsQueuePattern.FindStringSubmatch(location)
	if len(matches) != 2 {
		return fmt.Errorf("Jenkins 未返回有效队列地址")
	}
	queueID, _ := strconv.ParseInt(matches[1], 10, 64)
	now := util.HTime{Time: time.Now()}
	item.QueueID, item.Status, item.StartedAt = queueID, model.ReleaseItemQueued, &now
	return s.dao.DB().Model(item).Updates(map[string]interface{}{"queue_id": queueID, "status": model.ReleaseItemQueued, "started_at": now, "updated_at": now}).Error
}

func (s *ReleaseService) failReleaseItem(itemID uint, cause error) {
	now := util.HTime{Time: time.Now()}
	_ = s.dao.DB().Model(&model.ReleaseItem{}).Where("id = ?", itemID).Updates(map[string]interface{}{"status": model.ReleaseItemFailed, "error_message": cause.Error(), "finished_at": now, "updated_at": now}).Error
}

func (s *ReleaseService) refreshReleaseStatus(releaseID uint) {
	var items []model.ReleaseItem
	if s.dao.DB().Where("release_id = ?", releaseID).Find(&items).Error != nil || len(items) == 0 {
		return
	}
	var release model.ReleaseTicket
	if s.dao.DB().Select("id", "strategy").First(&release, releaseID).Error != nil {
		return
	}
	status, finished := model.ReleaseStatusRunning, true
	allSuccess, hasTerminalFailure := true, false
	for _, item := range items {
		if item.Status == model.ReleaseItemPending || item.Status == model.ReleaseItemQueued || item.Status == model.ReleaseItemRunning {
			finished = false
		}
		if item.Status != model.ReleaseItemSucceeded {
			allSuccess = false
		}
		if item.Status == model.ReleaseItemFailed || item.Status == model.ReleaseItemCanceled {
			hasTerminalFailure = true
		}
	}
	if release.Strategy == "serial" && hasTerminalFailure {
		finished = true
	}
	updates := map[string]interface{}{"status": status, "updated_at": util.HTime{Time: time.Now()}}
	if finished {
		if allSuccess {
			status = model.ReleaseStatusSucceeded
		} else {
			status = model.ReleaseStatusFailed
		}
		now := util.HTime{Time: time.Now()}
		updates["status"], updates["finished_at"] = status, now
	}
	_ = s.dao.DB().Model(&model.ReleaseTicket{}).Where("id = ? AND business_type = ?", releaseID, model.BusinessTypeRelease).Updates(updates).Error
}

func (s *ReleaseService) pendingReleaseItems(releaseID uint) ([]model.ReleaseItem, error) {
	var items []model.ReleaseItem
	err := s.dao.DB().Where("release_id = ? AND status = ?", releaseID, model.ReleaseItemPending).Order("execute_order ASC, id ASC").Find(&items).Error
	return items, err
}

func (s *ReleaseService) buildReleaseItems(inputs []ReleaseCreateItemInput, environment string) ([]model.ReleaseItem, error) {
	ids := make([]uint, 0, len(inputs))
	for _, input := range inputs {
		if input.ApplicationID > 0 {
			ids = append(ids, input.ApplicationID)
		}
	}
	ids = uniqueUint(ids)
	if len(ids) != len(inputs) {
		return nil, fmt.Errorf("发布应用不能重复")
	}
	var applications []model.ReleaseApplication
	if err := s.dao.DB().Where("id IN ? AND enabled = ?", ids, 1).Find(&applications).Error; err != nil {
		return nil, err
	}
	appMap := map[uint]model.ReleaseApplication{}
	for _, app := range applications {
		appMap[app.ID] = app
	}
	items := make([]model.ReleaseItem, 0, len(inputs))
	for index, input := range inputs {
		app, ok := appMap[input.ApplicationID]
		if !ok {
			return nil, fmt.Errorf("发布应用 %d 不存在或已停用", input.ApplicationID)
		}
		if app.Environment != environment {
			return nil, fmt.Errorf("应用 %s 不属于 %s 环境", app.Name, environment)
		}
		version := strings.TrimSpace(input.Version)
		if version == "" {
			return nil, fmt.Errorf("应用 %s 的版本不能为空", app.Name)
		}
		parameters := model.JSONMap{}
		for key, value := range app.DefaultParameters {
			parameters[key] = value
		}
		for key, value := range input.Parameters {
			parameters[key] = value
		}
		order := input.ExecuteOrder
		if order <= 0 {
			order = index + 1
		}
		items = append(items, model.ReleaseItem{ApplicationID: app.ID, ApplicationName: app.Name, Version: version, Target: strings.TrimSpace(input.Target), JenkinsAccountID: app.JenkinsAccountID, JenkinsJobName: app.JenkinsJobName, Parameters: parameters, ExecuteOrder: order, Status: model.ReleaseItemPending})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].ExecuteOrder < items[j].ExecuteOrder })
	return items, nil
}

func normalizeApplicationInput(input ApplicationInput) (model.ReleaseApplication, error) {
	item := model.ReleaseApplication{Name: strings.TrimSpace(input.Name), Code: strings.ToLower(strings.TrimSpace(input.Code)), SystemName: strings.TrimSpace(input.SystemName), Environment: strings.ToLower(strings.TrimSpace(input.Environment)), JenkinsAccountID: input.JenkinsAccountID, JenkinsJobName: strings.TrimSpace(input.JenkinsJobName), DefaultParameters: input.DefaultParameters, Description: strings.TrimSpace(input.Description), Enabled: 1}
	if input.Enabled != nil && *input.Enabled != 1 {
		item.Enabled = 0
	}
	if item.Name == "" || item.Code == "" || item.Environment == "" || item.JenkinsAccountID == 0 || item.JenkinsJobName == "" {
		return item, fmt.Errorf("应用名称、编码、环境、Jenkins 账号和 Job 不能为空")
	}
	return item, nil
}

type jenkinsClient struct {
	baseURL, username, password string
	client                      *http.Client
}

func newJenkinsClient(accountID uint) (*jenkinsClient, error) {
	account, err := loadJenkinsAccount(accountID)
	if err != nil {
		return nil, err
	}
	password, err := account.DecryptPassword()
	if err != nil {
		return nil, fmt.Errorf("解密 Jenkins 凭据失败: %w", err)
	}
	return &jenkinsClient{baseURL: jenkinsBaseURL(account), username: account.Name, password: password, client: &http.Client{Timeout: 20 * time.Second}}, nil
}

func loadJenkinsAccount(accountID uint) (*configmodel.AccountAuth, error) {
	account, err := configdao.NewAccountAuthDao().GetByID(accountID)
	if err != nil {
		return nil, fmt.Errorf("Jenkins 账号不存在")
	}
	if account.Type != jenkinsAccountType {
		return nil, fmt.Errorf("所选账号不是 Jenkins 类型")
	}
	return account, nil
}

func jenkinsBaseURL(account *configmodel.AccountAuth) string {
	host := strings.TrimSpace(account.Host)
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return strings.TrimRight(host, "/")
	}
	if account.Port > 0 && parsed.Port() == "" {
		parsed.Host = net.JoinHostPort(parsed.Hostname(), strconv.Itoa(account.Port))
	}
	return strings.TrimRight(parsed.String(), "/")
}

func jenkinsJobPath(jobName string) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(jobName), "/"), "/")
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "job" {
			encoded = append(encoded, "job", url.PathEscape(part))
		}
	}
	return "/" + strings.Join(encoded, "/")
}

func (c *jenkinsClient) trigger(ctx context.Context, path string, params url.Values) ([]byte, string, error) {
	var body *strings.Reader
	if len(params) > 0 {
		body = strings.NewReader(params.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, "", err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.addCrumb(ctx, req)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, "", fmt.Errorf("Jenkins 触发失败，状态码 %d", resp.StatusCode)
	}
	return nil, resp.Header.Get("Location"), nil
}

func (c *jenkinsClient) request(ctx context.Context, method, path string, params url.Values) ([]byte, error) {
	var body *strings.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	if method == http.MethodPost {
		c.addCrumb(ctx, req)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Jenkins 请求失败，状态码 %d", resp.StatusCode)
	}
	var data json.RawMessage
	if strings.HasSuffix(path, "consoleText") {
		return io.ReadAll(resp.Body)
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	return []byte(data), err
}

func (c *jenkinsClient) addCrumb(ctx context.Context, req *http.Request) {
	crumbReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/crumbIssuer/api/json", nil)
	if err != nil {
		return
	}
	crumbReq.SetBasicAuth(c.username, c.password)
	resp, err := c.client.Do(crumbReq)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var crumb struct {
		Field string `json:"crumbRequestField"`
		Value string `json:"crumb"`
	}
	if json.NewDecoder(resp.Body).Decode(&crumb) == nil && crumb.Field != "" {
		req.Header.Set(crumb.Field, crumb.Value)
	}
}
