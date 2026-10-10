package service

import (
	"fmt"
	"strings"
	"time"

	"dodevops-api/api/ticket/dao"
	"dodevops-api/api/ticket/model"
	"dodevops-api/common/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var dailyCategories = map[string]struct{}{
	"ops_issue": {}, "permission": {}, "config_change": {}, "resource_request": {},
}

type DailyService struct {
	dao      *dao.TicketDao
	workflow *WorkflowService
}

type DailyCreateInput struct {
	Title       string        `json:"title"`
	Category    string        `json:"category"`
	Environment string        `json:"environment"`
	Priority    string        `json:"priority"`
	ImpactScope string        `json:"impactScope"`
	Description string        `json:"description"`
	FormData    model.JSONMap `json:"formData"`
	Attachments []string      `json:"attachments"`
	WorkflowID  uint          `json:"workflowId"`
	ExpectedAt  string        `json:"expectedAt"`
}

type DailyAssignInput struct {
	HandlerID uint   `json:"handlerId"`
	Comment   string `json:"comment"`
}

type DailyActionInput struct {
	Comment string `json:"comment"`
}

type DailyDetail struct {
	Ticket   *model.DailyTicket  `json:"ticket"`
	Actions  []model.DailyAction `json:"actions"`
	Workflow *WorkflowDetail     `json:"workflow,omitempty"`
}

func NewDailyService(db *gorm.DB) *DailyService {
	return &DailyService{dao: dao.NewTicketDao(db), workflow: NewWorkflowService(db)}
}

func (s *DailyService) Create(input DailyCreateInput, applicantID uint, applicantName string) (*model.DailyTicket, error) {
	item, err := normalizeDailyInput(input)
	if err != nil {
		return nil, err
	}
	item.ApplicantID, item.ApplicantName = applicantID, applicantName
	item.TicketNo = fmt.Sprintf("TMP-OPS-%d", time.Now().UnixNano())
	item.Status = model.DailyStatusSubmitted
	if input.WorkflowID > 0 {
		item.Status = model.DailyStatusPending
	}
	now := util.HTime{Time: time.Now()}
	item.CreatedAt, item.UpdatedAt = now, now
	err = s.dao.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		item.TicketNo = fmt.Sprintf("OPS-%s-%06d", now.Format("20060102"), item.ID)
		if err := tx.Model(&item).Update("ticket_no", item.TicketNo).Error; err != nil {
			return err
		}
		if input.WorkflowID > 0 {
			instance, err := s.workflow.StartWithTx(tx, WorkflowStartInput{DefinitionID: input.WorkflowID, BusinessType: model.BusinessTypeDaily, BusinessID: item.ID, BusinessNo: item.TicketNo, Environment: item.Environment, ApplicantID: applicantID, ApplicantName: applicantName})
			if err != nil {
				return err
			}
			item.WorkflowInstanceID = instance.ID
			if err := tx.Model(&item).Update("workflow_instance_id", instance.ID).Error; err != nil {
				return err
			}
		}
		return addDailyAction(tx, item.ID, applicantID, applicantName, "create", "", item.Status, "创建工单", now)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *DailyService) List(query dao.DailyQuery) ([]model.DailyTicket, int64, error) {
	return s.dao.ListDailyTickets(query)
}

func (s *DailyService) Detail(id uint) (*DailyDetail, error) {
	item, actions, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	detail := &DailyDetail{Ticket: item, Actions: actions}
	if item.WorkflowInstanceID > 0 {
		detail.Workflow, _ = s.workflow.Detail(item.WorkflowInstanceID)
	}
	return detail, nil
}

func (s *DailyService) Assign(id uint, input DailyAssignInput, operatorID uint, operatorName string) (*model.DailyTicket, error) {
	if input.HandlerID == 0 {
		return nil, fmt.Errorf("请选择处理人")
	}
	handlerID, handlerName, err := resolveSystemUser(s.dao.DB(), input.HandlerID)
	if err != nil {
		return nil, err
	}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusSubmitted, model.DailyStatusApproved, model.DailyStatusAssigned}, model.DailyStatusAssigned, strings.TrimSpace(input.Comment), map[string]interface{}{"handler_id": handlerID, "handler_name": handlerName})
}

func (s *DailyService) Start(id, operatorID uint, operatorName, comment string) (*model.DailyTicket, error) {
	item, _, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	if item.HandlerID != 0 && item.HandlerID != operatorID {
		return nil, fmt.Errorf("只有指定处理人可以开始处理")
	}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusAssigned, model.DailyStatusApproved}, model.DailyStatusProcessing, comment, nil)
}

func (s *DailyService) Resolve(id, operatorID uint, operatorName, comment string) (*model.DailyTicket, error) {
	item, _, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	if item.HandlerID != 0 && item.HandlerID != operatorID {
		return nil, fmt.Errorf("只有指定处理人可以提交解决结果")
	}
	now := util.HTime{Time: time.Now()}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusProcessing}, model.DailyStatusResolved, comment, map[string]interface{}{"resolved_at": now})
}

func (s *DailyService) Confirm(id, operatorID uint, operatorName, comment string) (*model.DailyTicket, error) {
	item, _, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	if item.ApplicantID != operatorID {
		return nil, fmt.Errorf("只有申请人可以确认处理结果")
	}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusResolved}, model.DailyStatusConfirmed, comment, nil)
}

func (s *DailyService) Close(id, operatorID uint, operatorName, comment string) (*model.DailyTicket, error) {
	item, _, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	if item.ApplicantID != operatorID && item.HandlerID != operatorID {
		return nil, fmt.Errorf("只有申请人或处理人可以关闭工单")
	}
	now := util.HTime{Time: time.Now()}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusConfirmed}, model.DailyStatusClosed, comment, map[string]interface{}{"closed_at": now})
}

func (s *DailyService) Cancel(id, operatorID uint, operatorName, comment string) (*model.DailyTicket, error) {
	item, _, err := s.dao.GetDailyTicket(id)
	if err != nil {
		return nil, err
	}
	if item.ApplicantID != operatorID {
		return nil, fmt.Errorf("只有申请人可以撤销工单")
	}
	return s.transition(id, operatorID, operatorName, []string{model.DailyStatusSubmitted, model.DailyStatusPending, model.DailyStatusApproved, model.DailyStatusAssigned}, model.DailyStatusCanceled, comment, nil)
}

func (s *DailyService) transition(id, operatorID uint, operatorName string, allowed []string, target, comment string, extras map[string]interface{}) (*model.DailyTicket, error) {
	var item model.DailyTicket
	err := s.dao.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("business_type = ?", model.BusinessTypeDaily).First(&item, id).Error; err != nil {
			return err
		}
		if len(allowed) > 0 && !containsString(allowed, item.Status) {
			return fmt.Errorf("工单当前状态 %s 不允许此操作", item.Status)
		}
		if target == model.DailyStatusCanceled && item.WorkflowInstanceID > 0 {
			if err := s.workflow.CancelWithTx(tx, item.WorkflowInstanceID, operatorID, operatorName, comment); err != nil {
				return err
			}
		}
		now := util.HTime{Time: time.Now()}
		updates := map[string]interface{}{"status": target, "updated_at": now}
		for key, value := range extras {
			updates[key] = value
		}
		from := item.Status
		if err := tx.Model(&item).Updates(updates).Error; err != nil {
			return err
		}
		if err := addDailyAction(tx, item.ID, operatorID, operatorName, target, from, target, strings.TrimSpace(comment), now); err != nil {
			return err
		}
		return tx.Where("business_type = ?", model.BusinessTypeDaily).First(&item, id).Error
	})
	return &item, err
}

func normalizeDailyInput(input DailyCreateInput) (model.DailyTicket, error) {
	title, category := strings.TrimSpace(input.Title), strings.ToLower(strings.TrimSpace(input.Category))
	if title == "" {
		return model.DailyTicket{}, fmt.Errorf("工单标题不能为空")
	}
	if _, ok := dailyCategories[category]; !ok {
		return model.DailyTicket{}, fmt.Errorf("不支持的日常工单类型")
	}
	environment := strings.ToLower(strings.TrimSpace(input.Environment))
	if environment == "" {
		environment = "test"
	}
	priority := strings.ToLower(strings.TrimSpace(input.Priority))
	if priority == "" {
		priority = "medium"
	}
	if !containsString([]string{"low", "medium", "high", "urgent"}, priority) {
		return model.DailyTicket{}, fmt.Errorf("优先级无效")
	}
	if category != "ops_issue" && input.WorkflowID == 0 {
		return model.DailyTicket{}, fmt.Errorf("申请和变更类工单必须选择审批流")
	}
	var expected *util.HTime
	if strings.TrimSpace(input.ExpectedAt) != "" {
		parsed, err := parseTicketTime(input.ExpectedAt)
		if err != nil {
			return model.DailyTicket{}, fmt.Errorf("期望完成时间格式无效")
		}
		expected = &util.HTime{Time: parsed}
	}
	return model.DailyTicket{BusinessType: model.BusinessTypeDaily, Title: title, Category: category, Environment: environment, Priority: priority, ImpactScope: strings.TrimSpace(input.ImpactScope), Description: strings.TrimSpace(input.Description), FormData: input.FormData, Attachments: model.JSONStringArray(uniqueText(input.Attachments)), ExpectedAt: expected}, nil
}

func addDailyAction(tx *gorm.DB, ticketID, operatorID uint, operatorName, action, from, to, comment string, now util.HTime) error {
	return tx.Create(&model.DailyAction{TicketID: ticketID, OperatorID: operatorID, OperatorName: operatorName, Action: action, FromStatus: from, ToStatus: to, Comment: comment, CreatedAt: now}).Error
}

func resolveSystemUser(db *gorm.DB, id uint) (uint, string, error) {
	var row struct {
		ID                 uint
		Username, Nickname string
	}
	if err := db.Table("sys_admin").Select("id, username, nickname").Where("id = ? AND status = ?", id, 1).Take(&row).Error; err != nil {
		return 0, "", fmt.Errorf("用户不存在或已停用")
	}
	name := strings.TrimSpace(row.Nickname)
	if name == "" {
		name = row.Username
	}
	return row.ID, name, nil
}

func parseTicketTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time")
}

func uniqueText(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
