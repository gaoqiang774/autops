package dao

import (
	"strings"

	"dodevops-api/api/ticket/model"

	"gorm.io/gorm"
)

type TicketDao struct{ db *gorm.DB }

func NewTicketDao(db *gorm.DB) *TicketDao { return &TicketDao{db: db} }
func (d *TicketDao) DB() *gorm.DB         { return d.db }

func (d *TicketDao) ListWorkflowDefinitions(query model.PageQuery, enabled *int) ([]model.WorkflowDefinition, int64, error) {
	var list []model.WorkflowDefinition
	var total int64
	db := d.db.Model(&model.WorkflowDefinition{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		db = db.Where("enabled = ?", *enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *TicketDao) GetWorkflowDefinition(id uint) (*model.WorkflowDefinition, error) {
	var item model.WorkflowDefinition
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *TicketDao) GetWorkflowInstance(id uint) (*model.WorkflowInstance, error) {
	var item model.WorkflowInstance
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *TicketDao) WorkflowInstanceDetail(id uint) (*model.WorkflowInstance, []model.ApprovalTask, error) {
	instance, err := d.GetWorkflowInstance(id)
	if err != nil {
		return nil, nil, err
	}
	var tasks []model.ApprovalTask
	if err := d.db.Where("instance_id = ?", id).Order("node_order ASC, id ASC").Find(&tasks).Error; err != nil {
		return nil, nil, err
	}
	return instance, tasks, nil
}

func (d *TicketDao) ListMyApprovalTasks(userID uint, query model.PageQuery, status, businessType string) ([]model.ApprovalTask, int64, error) {
	var list []model.ApprovalTask
	var total int64
	db := d.db.Model(&model.ApprovalTask{}).
		Select("ticket_approval_tasks.*, ticket_workflow_instances.business_type, ticket_workflow_instances.business_id, ticket_workflow_instances.business_no, ticket_workflow_instances.definition_name").
		Joins("JOIN ticket_workflow_instances ON ticket_workflow_instances.id = ticket_approval_tasks.instance_id").
		Where("ticket_approval_tasks.assignee_id = ?", userID)
	if status != "" {
		db = db.Where("ticket_approval_tasks.status = ?", status)
	}
	if businessType != "" || strings.TrimSpace(query.Keyword) != "" {
		if businessType != "" {
			db = db.Where("ticket_workflow_instances.business_type = ?", businessType)
		}
		if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
			db = db.Where("ticket_workflow_instances.business_no LIKE ? OR ticket_workflow_instances.definition_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("ticket_approval_tasks.created_at DESC, ticket_approval_tasks.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

type DailyQuery struct {
	model.PageQuery
	Category, Environment, Priority, Status, Scope string
	CurrentUserID                                  uint
}

func (d *TicketDao) ListDailyTickets(query DailyQuery) ([]model.DailyTicket, int64, error) {
	var list []model.DailyTicket
	var total int64
	db := d.db.Model(&model.DailyTicket{}).Where("business_type = ?", model.BusinessTypeDaily)
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("ticket_no LIKE ? OR title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	for column, value := range map[string]string{"category": query.Category, "environment": query.Environment, "priority": query.Priority, "status": query.Status} {
		if value != "" {
			db = db.Where(column+" = ?", value)
		}
	}
	switch query.Scope {
	case "created":
		db = db.Where("applicant_id = ?", query.CurrentUserID)
	case "handled":
		db = db.Where("handler_id = ?", query.CurrentUserID)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *TicketDao) GetDailyTicket(id uint) (*model.DailyTicket, []model.DailyAction, error) {
	var item model.DailyTicket
	if err := d.db.Where("business_type = ?", model.BusinessTypeDaily).First(&item, id).Error; err != nil {
		return nil, nil, err
	}
	var actions []model.DailyAction
	err := d.db.Where("ticket_id = ?", id).Order("id ASC").Find(&actions).Error
	return &item, actions, err
}

type ReleaseQuery struct {
	model.PageQuery
	Environment, Status, Scope string
	CurrentUserID              uint
}

func (d *TicketDao) ListReleaseTickets(query ReleaseQuery) ([]model.ReleaseTicket, int64, error) {
	var list []model.ReleaseTicket
	var total int64
	db := d.db.Model(&model.ReleaseTicket{}).Where("business_type = ?", model.BusinessTypeRelease)
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("ticket_no LIKE ? OR title LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if query.Environment != "" {
		db = db.Where("environment = ?", query.Environment)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	switch query.Scope {
	case "created":
		db = db.Where("applicant_id = ?", query.CurrentUserID)
	case "executed":
		db = db.Where("executor_id = ?", query.CurrentUserID)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Preload("Items").Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *TicketDao) GetReleaseTicket(id uint) (*model.ReleaseTicket, error) {
	var item model.ReleaseTicket
	err := d.db.Preload("Items", func(tx *gorm.DB) *gorm.DB { return tx.Order("execute_order ASC, id ASC") }).Where("business_type = ?", model.BusinessTypeRelease).First(&item, id).Error
	return &item, err
}

func (d *TicketDao) ListApplications(query model.PageQuery, environment string, enabled *int) ([]model.ReleaseApplication, int64, error) {
	var list []model.ReleaseApplication
	var total int64
	db := d.db.Model(&model.ReleaseApplication{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("name LIKE ? OR code LIKE ? OR system_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if environment != "" {
		db = db.Where("environment = ?", environment)
	}
	if enabled != nil {
		db = db.Where("enabled = ?", *enabled)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	err := db.Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
