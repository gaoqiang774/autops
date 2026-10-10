package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/api/ticket/dao"
	"dodevops-api/api/ticket/model"
	"dodevops-api/common/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WorkflowService struct{ dao *dao.TicketDao }

type WorkflowDefinitionInput struct {
	Name                string              `json:"name"`
	BusinessTypes       []string            `json:"businessTypes"`
	Environments        []string            `json:"environments"`
	Description         string              `json:"description"`
	Enabled             *int                `json:"enabled"`
	PreventSelfApproval *int                `json:"preventSelfApproval"`
	Nodes               []WorkflowNodeInput `json:"nodes"`
}

type WorkflowNodeInput struct {
	Name         string `json:"name"`
	ApproverType string `json:"approverType"`
	ApproverIDs  []uint `json:"approverIds"`
	ApprovalMode string `json:"approvalMode"`
	Description  string `json:"description"`
}

type WorkflowStartInput struct {
	DefinitionID  uint
	BusinessType  string
	BusinessID    uint
	BusinessNo    string
	Environment   string
	ApplicantID   uint
	ApplicantName string
}

type WorkflowActionInput struct {
	Comment string `json:"comment"`
}

type WorkflowDetail struct {
	Instance *model.WorkflowInstance `json:"instance"`
	Tasks    []model.ApprovalTask    `json:"tasks"`
}

type workflowSnapshotNode struct {
	NodeID       uint               `json:"nodeId"`
	NodeOrder    int                `json:"nodeOrder"`
	Name         string             `json:"name"`
	ApprovalMode string             `json:"approvalMode"`
	Approvers    []workflowApprover `json:"approvers"`
}

type workflowApprover struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func NewWorkflowService(db *gorm.DB) *WorkflowService {
	return &WorkflowService{dao: dao.NewTicketDao(db)}
}

func (s *WorkflowService) List(query model.PageQuery, enabled *int) ([]model.WorkflowDefinition, int64, error) {
	return s.dao.ListWorkflowDefinitions(query, enabled)
}

func (s *WorkflowService) Get(id uint) (*model.WorkflowDefinition, error) {
	return s.dao.GetWorkflowDefinition(id)
}

func (s *WorkflowService) Save(id uint, input WorkflowDefinitionInput, operatorID uint, operatorName string) (*model.WorkflowDefinition, error) {
	item, nodes, err := normalizeWorkflowInput(input)
	if err != nil {
		return nil, err
	}
	item.Nodes = model.JSONWorkflowNodes(nodes)
	now := util.HTime{Time: time.Now()}
	returnItem := &model.WorkflowDefinition{}
	err = s.dao.DB().Transaction(func(tx *gorm.DB) error {
		if id == 0 {
			if item.Code == "" {
				item.Code = fmt.Sprintf("workflow_%d", now.UnixNano())
			}
			item.MaintainerID, item.MaintainerName = operatorID, operatorName
			item.Version = 1
			item.CreatedAt, item.UpdatedAt = now, now
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		} else {
			var existing model.WorkflowDefinition
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, id).Error; err != nil {
				return err
			}
			item.ID = id
			item.Code = existing.Code
			item.Version = existing.Version + 1
			item.MaintainerID, item.MaintainerName = operatorID, operatorName
			item.CreatedAt, item.UpdatedAt = existing.CreatedAt, now
			if err := tx.Model(&existing).Select("name", "business_types", "environments", "description", "enabled", "version", "prevent_self_approval", "maintainer_id", "maintainer_name", "updated_at", "nodes").Updates(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	returnItem, err = s.dao.GetWorkflowDefinition(item.ID)
	return returnItem, err
}

func (s *WorkflowService) Delete(id uint) error {
	return s.dao.DB().Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.WorkflowInstance{}).Where("definition_id = ? AND status = ?", id, model.WorkflowStatusRunning).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("流程存在运行中的审批实例，不能删除")
		}
		return tx.Delete(&model.WorkflowDefinition{}, id).Error
	})
}

func (s *WorkflowService) ChangeEnabled(id uint, enabled int, operatorID uint, operatorName string) error {
	if enabled != 1 {
		enabled = 0
	}
	return s.dao.DB().Model(&model.WorkflowDefinition{}).Where("id = ?", id).Updates(map[string]interface{}{
		"enabled": enabled, "maintainer_id": operatorID, "maintainer_name": operatorName,
		"updated_at": util.HTime{Time: time.Now()},
	}).Error
}

func (s *WorkflowService) Start(input WorkflowStartInput) (*model.WorkflowInstance, error) {
	var instance *model.WorkflowInstance
	err := s.dao.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		instance, err = s.StartWithTx(tx, input)
		return err
	})
	return instance, err
}

func (s *WorkflowService) StartWithTx(tx *gorm.DB, input WorkflowStartInput) (*model.WorkflowInstance, error) {
	if input.DefinitionID == 0 || input.BusinessID == 0 || input.ApplicantID == 0 {
		return nil, fmt.Errorf("启动审批流参数不完整")
	}
	var definition model.WorkflowDefinition
	if err := tx.Where("enabled = ?", 1).First(&definition, input.DefinitionID).Error; err != nil {
		return nil, fmt.Errorf("审批流不存在或未启用: %w", err)
	}
	if !containsString(definition.BusinessTypes, input.BusinessType) {
		return nil, fmt.Errorf("审批流不适用于当前工单类型")
	}
	if len(definition.Environments) > 0 && !containsString(definition.Environments, strings.ToLower(strings.TrimSpace(input.Environment))) {
		return nil, fmt.Errorf("审批流不适用于当前运行环境")
	}
	if len(definition.Nodes) == 0 {
		return nil, fmt.Errorf("审批流没有审批节点")
	}
	snapshot := make([]workflowSnapshotNode, 0, len(definition.Nodes))
	for _, node := range definition.Nodes {
		approvers, err := resolveApprovers(tx, node)
		if err != nil {
			return nil, fmt.Errorf("审批节点 %s: %w", node.Name, err)
		}
		if len(approvers) == 0 {
			return nil, fmt.Errorf("审批节点 %s 没有启用中的审批人，请检查审批流配置", node.Name)
		}
		if definition.PreventSelfApproval == 1 {
			filtered := approvers[:0]
			for _, approver := range approvers {
				if approver.ID != input.ApplicantID {
					filtered = append(filtered, approver)
				}
			}
			approvers = filtered
			if len(approvers) == 0 {
				return nil, fmt.Errorf("审批节点 %s 的审批人只有申请人本人，当前流程禁止自审，请更换审批人或关闭禁止自审", node.Name)
			}
		}
		snapshot = append(snapshot, workflowSnapshotNode{NodeID: node.ID, NodeOrder: node.NodeOrder, Name: node.Name, ApprovalMode: node.ApprovalMode, Approvers: approvers})
	}
	snapshotJSON, _ := json.Marshal(snapshot)
	now := util.HTime{Time: time.Now()}
	instance := &model.WorkflowInstance{
		DefinitionID: definition.ID, DefinitionName: definition.Name, DefinitionVersion: definition.Version,
		BusinessType: input.BusinessType, BusinessID: input.BusinessID, BusinessNo: input.BusinessNo,
		ApplicantID: input.ApplicantID, ApplicantName: input.ApplicantName,
		Status: model.WorkflowStatusRunning, CurrentNodeOrder: snapshot[0].NodeOrder, CurrentNodeName: snapshot[0].Name,
		NodeSnapshot: string(snapshotJSON), StartedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(instance).Error; err != nil {
		return nil, err
	}
	if err := createApprovalTasks(tx, instance.ID, snapshot[0], now); err != nil {
		return nil, err
	}
	return instance, nil
}

func (s *WorkflowService) Detail(id uint) (*WorkflowDetail, error) {
	instance, tasks, err := s.dao.WorkflowInstanceDetail(id)
	if err != nil {
		return nil, err
	}
	return &WorkflowDetail{Instance: instance, Tasks: tasks}, nil
}

func (s *WorkflowService) MyTasks(userID uint, query model.PageQuery, status, businessType string) ([]model.ApprovalTask, int64, error) {
	return s.dao.ListMyApprovalTasks(userID, query, status, businessType)
}

func (s *WorkflowService) Approve(taskID, operatorID uint, operatorName, comment string) (*model.WorkflowInstance, error) {
	return s.act(taskID, operatorID, operatorName, model.ApprovalTaskApproved, strings.TrimSpace(comment))
}

func (s *WorkflowService) Reject(taskID, operatorID uint, operatorName, comment string) (*model.WorkflowInstance, error) {
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return nil, fmt.Errorf("驳回原因不能为空")
	}
	return s.act(taskID, operatorID, operatorName, model.ApprovalTaskRejected, comment)
}

func (s *WorkflowService) CancelWithTx(tx *gorm.DB, instanceID, operatorID uint, operatorName, comment string) error {
	if instanceID == 0 {
		return nil
	}
	var instance model.WorkflowInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&instance, instanceID).Error; err != nil {
		return err
	}
	if instance.Status != model.WorkflowStatusRunning {
		return nil
	}
	now := util.HTime{Time: time.Now()}
	if err := tx.Model(&model.ApprovalTask{}).Where("instance_id = ? AND status = ?", instance.ID, model.ApprovalTaskPending).Updates(map[string]interface{}{"status": model.ApprovalTaskCanceled, "comment": strings.TrimSpace(comment), "operator_id": operatorID, "operator_name": operatorName, "acted_at": now}).Error; err != nil {
		return err
	}
	return tx.Model(&instance).Updates(map[string]interface{}{"status": model.WorkflowStatusCanceled, "current_node_name": "", "finished_at": now, "updated_at": now}).Error
}

func (s *WorkflowService) act(taskID, operatorID uint, operatorName, action, comment string) (*model.WorkflowInstance, error) {
	var result model.WorkflowInstance
	err := s.dao.DB().Transaction(func(tx *gorm.DB) error {
		now := util.HTime{Time: time.Now()}
		var task model.ApprovalTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, taskID).Error; err != nil {
			return err
		}
		if task.AssigneeID != operatorID {
			return fmt.Errorf("当前用户不是该节点审批人")
		}
		if task.Status != model.ApprovalTaskPending {
			return fmt.Errorf("审批任务已处理")
		}
		var instance model.WorkflowInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&instance, task.InstanceID).Error; err != nil {
			return err
		}
		if instance.Status != model.WorkflowStatusRunning || instance.CurrentNodeOrder != task.NodeOrder {
			return fmt.Errorf("审批流程已进入其他节点")
		}
		if err := tx.Model(&task).Updates(map[string]interface{}{"status": action, "comment": comment, "operator_id": operatorID, "operator_name": operatorName, "acted_at": now}).Error; err != nil {
			return err
		}
		if action == model.ApprovalTaskRejected {
			if err := tx.Model(&model.ApprovalTask{}).Where("instance_id = ? AND status = ?", instance.ID, model.ApprovalTaskPending).Update("status", model.ApprovalTaskCanceled).Error; err != nil {
				return err
			}
			if err := finishWorkflow(tx, &instance, model.WorkflowStatusRejected, now); err != nil {
				return err
			}
			result = instance
			return nil
		}
		var snapshot []workflowSnapshotNode
		if err := json.Unmarshal([]byte(instance.NodeSnapshot), &snapshot); err != nil {
			return err
		}
		current, ok := snapshotNode(snapshot, task.NodeOrder)
		if !ok {
			return fmt.Errorf("审批流程快照缺少当前节点")
		}
		nodeDone := current.ApprovalMode == model.ApprovalModeAny
		if current.ApprovalMode == model.ApprovalModeAll {
			var pending int64
			if err := tx.Model(&model.ApprovalTask{}).Where("instance_id = ? AND node_order = ? AND status = ?", instance.ID, task.NodeOrder, model.ApprovalTaskPending).Count(&pending).Error; err != nil {
				return err
			}
			nodeDone = pending == 0
		}
		if !nodeDone {
			result = instance
			return nil
		}
		if current.ApprovalMode == model.ApprovalModeAny {
			if err := tx.Model(&model.ApprovalTask{}).Where("instance_id = ? AND node_order = ? AND status = ?", instance.ID, task.NodeOrder, model.ApprovalTaskPending).Update("status", model.ApprovalTaskCanceled).Error; err != nil {
				return err
			}
		}
		next, hasNext := snapshotNode(snapshot, task.NodeOrder+1)
		if !hasNext {
			if err := finishWorkflow(tx, &instance, model.WorkflowStatusApproved, now); err != nil {
				return err
			}
			result = instance
			return nil
		}
		if err := createApprovalTasks(tx, instance.ID, next, now); err != nil {
			return err
		}
		if err := tx.Model(&instance).Updates(map[string]interface{}{"current_node_order": next.NodeOrder, "current_node_name": next.Name, "updated_at": now}).Error; err != nil {
			return err
		}
		instance.CurrentNodeOrder, instance.CurrentNodeName, instance.UpdatedAt = next.NodeOrder, next.Name, now
		result = instance
		return nil
	})
	return &result, err
}

func finishWorkflow(tx *gorm.DB, instance *model.WorkflowInstance, status string, now util.HTime) error {
	if err := applyWorkflowBusinessResult(tx, instance.BusinessType, instance.BusinessID, status, now); err != nil {
		return err
	}
	if err := tx.Model(instance).Updates(map[string]interface{}{"status": status, "current_node_name": "", "finished_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	instance.Status, instance.CurrentNodeName, instance.FinishedAt, instance.UpdatedAt = status, "", &now, now
	return nil
}

func applyWorkflowBusinessResult(tx *gorm.DB, businessType string, businessID uint, status string, now util.HTime) error {
	var table, nextStatus string
	switch businessType {
	case model.BusinessTypeDaily:
		table = model.Ticket{}.TableName()
		if status == model.WorkflowStatusApproved {
			nextStatus = model.DailyStatusApproved
		} else {
			nextStatus = model.DailyStatusRejected
		}
	case model.BusinessTypeRelease:
		table = model.Ticket{}.TableName()
		if status == model.WorkflowStatusApproved {
			nextStatus = model.ReleaseStatusApproved
		} else {
			nextStatus = model.ReleaseStatusRejected
		}
	case model.BusinessTypeSQLReview:
		table = "db_sql_ticket"
		if status == model.WorkflowStatusApproved {
			nextStatus = "approved"
		} else {
			nextStatus = "rejected"
		}
	default:
		return fmt.Errorf("不支持的审批业务类型 %s", businessType)
	}
	updates := map[string]interface{}{"status": nextStatus, "updated_at": now}
	if businessType == model.BusinessTypeSQLReview {
		updates["reviewed_at"] = now
	}
	db := tx.Table(table).Where("id = ?", businessID)
	if businessType != model.BusinessTypeSQLReview {
		db = db.Where("business_type = ?", businessType)
	}
	return db.Updates(updates).Error
}

func normalizeWorkflowInput(input WorkflowDefinitionInput) (model.WorkflowDefinition, []model.WorkflowNode, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return model.WorkflowDefinition{}, nil, fmt.Errorf("流程名称不能为空")
	}
	if len(input.BusinessTypes) == 0 {
		return model.WorkflowDefinition{}, nil, fmt.Errorf("至少选择一种适用工单")
	}
	if len(input.Nodes) == 0 {
		return model.WorkflowDefinition{}, nil, fmt.Errorf("至少配置一个审批节点")
	}
	enabled, preventSelf := 1, 1
	if input.Enabled != nil && *input.Enabled != 1 {
		enabled = 0
	}
	if input.PreventSelfApproval != nil && *input.PreventSelfApproval != 1 {
		preventSelf = 0
	}
	item := model.WorkflowDefinition{Name: name, BusinessTypes: uniqueStrings(input.BusinessTypes), Environments: uniqueStrings(input.Environments), Description: strings.TrimSpace(input.Description), Enabled: enabled, PreventSelfApproval: preventSelf}
	nodes := make([]model.WorkflowNode, 0, len(input.Nodes))
	for index, raw := range input.Nodes {
		node := model.WorkflowNode{ID: uint(index + 1), NodeOrder: index + 1, Name: strings.TrimSpace(raw.Name), ApproverType: strings.ToLower(strings.TrimSpace(raw.ApproverType)), ApproverIDs: model.JSONUintArray(uniqueUint(raw.ApproverIDs)), ApprovalMode: strings.ToLower(strings.TrimSpace(raw.ApprovalMode)), Description: strings.TrimSpace(raw.Description)}
		if node.Name == "" || len(node.ApproverIDs) == 0 {
			return item, nil, fmt.Errorf("审批节点名称和审批对象不能为空")
		}
		if node.ApproverType != model.ApproverTypeUser && node.ApproverType != model.ApproverTypeRole {
			return item, nil, fmt.Errorf("审批对象类型仅支持 user 或 role")
		}
		if node.ApprovalMode == "" {
			node.ApprovalMode = model.ApprovalModeAny
		}
		if node.ApprovalMode != model.ApprovalModeAny && node.ApprovalMode != model.ApprovalModeAll {
			return item, nil, fmt.Errorf("审批方式仅支持 any 或 all")
		}
		nodes = append(nodes, node)
	}
	return item, nodes, nil
}

func resolveApprovers(tx *gorm.DB, node model.WorkflowNode) ([]workflowApprover, error) {
	type userRow struct {
		ID                 uint
		Username, Nickname string
	}
	var users []userRow
	db := tx.Table("sys_admin").Select("DISTINCT sys_admin.id, sys_admin.username, sys_admin.nickname").Where("sys_admin.status = ?", 1)
	if node.ApproverType == model.ApproverTypeRole {
		db = db.Joins("JOIN sys_admin_role ON sys_admin_role.admin_id = sys_admin.id").Where("sys_admin_role.role_id IN ?", []uint(node.ApproverIDs))
	} else {
		db = db.Where("sys_admin.id IN ?", []uint(node.ApproverIDs))
	}
	if err := db.Order("sys_admin.id ASC").Scan(&users).Error; err != nil {
		return nil, err
	}
	result := make([]workflowApprover, 0, len(users))
	for _, user := range users {
		name := strings.TrimSpace(user.Nickname)
		if name == "" {
			name = user.Username
		}
		result = append(result, workflowApprover{ID: user.ID, Name: name})
	}
	return result, nil
}

func createApprovalTasks(tx *gorm.DB, instanceID uint, node workflowSnapshotNode, now util.HTime) error {
	tasks := make([]model.ApprovalTask, 0, len(node.Approvers))
	for _, approver := range node.Approvers {
		tasks = append(tasks, model.ApprovalTask{InstanceID: instanceID, NodeID: node.NodeID, NodeOrder: node.NodeOrder, NodeName: node.Name, AssigneeID: approver.ID, AssigneeName: approver.Name, Status: model.ApprovalTaskPending, CreatedAt: now})
	}
	return tx.Create(&tasks).Error
}

func snapshotNode(nodes []workflowSnapshotNode, order int) (workflowSnapshotNode, bool) {
	for _, node := range nodes {
		if node.NodeOrder == order {
			return node, true
		}
	}
	return workflowSnapshotNode{}, false
}

func uniqueStrings(values []string) model.JSONStringArray {
	seen := map[string]struct{}{}
	result := make(model.JSONStringArray, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func uniqueUint(values []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func CurrentUser(admin *systemmodel.JwtAdmin) (uint, string, error) {
	if admin == nil || admin.ID == 0 {
		return 0, "", errors.New("无法获取当前用户")
	}
	name := strings.TrimSpace(admin.Nickname)
	if name == "" {
		name = admin.Username
	}
	return admin.ID, name, nil
}
