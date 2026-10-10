package dao

import (
	"encoding/json"
	"strings"
	"time"

	"dodevops-api/api/aiops/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

type AgentDao struct {
	db *gorm.DB
}

func NewAgentDao() *AgentDao {
	return &AgentDao{db: common.GetDB()}
}

func (d *AgentDao) UpsertSession(session *model.AgentSession) error {
	if session == nil {
		return nil
	}
	now := time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	if session.Status == "" {
		session.Status = "active"
	}

	var existing model.AgentSession
	err := d.db.Where("session_id = ?", session.SessionID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return d.db.Create(session).Error
	}
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"updated_at": now,
	}
	if session.Title != "" {
		updates["title"] = session.Title
	}
	if session.Channel != "" {
		updates["channel"] = session.Channel
	}
	if session.UserID > 0 {
		updates["user_id"] = session.UserID
	}
	if session.Username != "" {
		updates["username"] = session.Username
	}
	if session.Status != "" {
		updates["status"] = session.Status
	}
	if session.LastRunID > 0 {
		updates["last_run_id"] = session.LastRunID
	}
	return d.db.Model(&model.AgentSession{}).Where("session_id = ?", session.SessionID).Updates(updates).Error
}

func (d *AgentDao) CreateMessage(message *model.AgentMessage) error {
	if message == nil {
		return nil
	}
	now := time.Now()
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}
	if message.UpdatedAt.IsZero() {
		message.UpdatedAt = now
	}
	return d.db.Create(message).Error
}

func (d *AgentDao) ListRecentMessages(sessionID string, limit int) ([]model.AgentMessage, error) {
	if sessionID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}
	var messages []model.AgentMessage
	err := d.db.Where("session_id = ?", sessionID).
		Order("id DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (d *AgentDao) CreateRun(run *model.AgentRun) error {
	now := time.Now()
	if run.StartedAt.IsZero() {
		run.StartedAt = now
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = now
	}
	return d.db.Create(run).Error
}

func (d *AgentDao) FinishRun(id uint64, status, message string, finishedAt time.Time) error {
	updates := map[string]interface{}{
		"status":      status,
		"finished_at": finishedAt,
		"updated_at":  finishedAt,
	}
	if message != "" {
		updates["error"] = message
	}

	var run model.AgentRun
	if err := d.db.First(&run, id).Error; err != nil {
		return err
	}
	if !run.StartedAt.IsZero() {
		updates["duration_ms"] = finishedAt.Sub(run.StartedAt).Milliseconds()
	}
	return d.db.Model(&model.AgentRun{}).Where("id = ?", id).Updates(updates).Error
}

func (d *AgentDao) AppendRunStep(runID uint64, step model.AgentStep) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var run model.AgentRun
		if err := tx.First(&run, runID).Error; err != nil {
			return err
		}

		var steps []model.AgentStep
		if run.StepsJSON != "" {
			_ = json.Unmarshal([]byte(run.StepsJSON), &steps)
		}
		steps = append(steps, step)

		data, err := json.Marshal(steps)
		if err != nil {
			return err
		}
		return tx.Model(&model.AgentRun{}).
			Where("id = ?", runID).
			Updates(map[string]interface{}{
				"steps_json": string(data),
				"updated_at": time.Now(),
			}).Error
	})
}

func (d *AgentDao) CreateEvidence(evidence *model.AgentEvidence) error {
	now := time.Now()
	if evidence.CreatedAt.IsZero() {
		evidence.CreatedAt = now
	}
	if evidence.UpdatedAt.IsZero() {
		evidence.UpdatedAt = now
	}
	return d.db.Create(evidence).Error
}

func (d *AgentDao) CreateFeedback(feedback *model.AgentFeedback) error {
	if feedback == nil {
		return nil
	}
	now := time.Now()
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = now
	}
	if feedback.UpdatedAt.IsZero() {
		feedback.UpdatedAt = now
	}
	return d.db.Create(feedback).Error
}

func (d *AgentDao) BatchCreateEvidence(items []model.AgentEvidence) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now()
	for i := range items {
		if items[i].CreatedAt.IsZero() {
			items[i].CreatedAt = now
		}
		if items[i].UpdatedAt.IsZero() {
			items[i].UpdatedAt = now
		}
	}
	return d.db.CreateInBatches(items, 100).Error
}

func (d *AgentDao) UpdateRunResult(runID uint64, result *model.AlertDiagnosisResult, resultJSON string, generatedAt time.Time) error {
	return d.db.Model(&model.AgentRun{}).
		Where("id = ?", runID).
		Updates(map[string]interface{}{
			"provider_id":   result.ProviderID,
			"provider_name": result.ProviderName,
			"provider_type": result.ProviderType,
			"model_code":    result.ModelCode,
			"summary":       result.Summary,
			"severity":      result.Severity,
			"result_json":   resultJSON,
			"used_llm":      result.UsedLLM,
			"generated_at":  generatedAt,
			"updated_at":    time.Now(),
		}).Error
}

func (d *AgentDao) GetRun(id uint64) (*model.AgentRun, error) {
	var run model.AgentRun
	if err := d.db.First(&run, id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (d *AgentDao) ListSteps(runID uint64) ([]model.AgentStep, error) {
	run, err := d.GetRun(runID)
	if err != nil {
		return nil, err
	}
	var steps []model.AgentStep
	if run.StepsJSON == "" {
		return steps, nil
	}
	if err := json.Unmarshal([]byte(run.StepsJSON), &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

func (d *AgentDao) ListEvidence(runID uint64) ([]model.AgentEvidence, error) {
	var evidence []model.AgentEvidence
	err := d.db.Where("run_id = ?", runID).Order("id ASC").Find(&evidence).Error
	return evidence, err
}

func (d *AgentDao) ClearAssistantChatHistory(sessionID string) (*model.AssistantChatHistoryClearResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	cleared := &model.AssistantChatHistoryClearResult{
		SessionID:  sessionID,
		ClearedAll: sessionID == "",
	}
	err := d.db.Transaction(func(tx *gorm.DB) error {
		runQuery := tx.Model(&model.AgentRun{}).Where("run_type = ?", model.AgentRunTypeAssistantChat)
		if sessionID != "" {
			runQuery = runQuery.Where(
				"id IN (?)",
				tx.Model(&model.AgentMessage{}).
					Select("run_id").
					Where("session_id = ? AND run_id > 0", sessionID),
			)
		}

		var runIDs []uint64
		if err := runQuery.Pluck("id", &runIDs).Error; err != nil {
			return err
		}

		if len(runIDs) > 0 {
			evidenceResult := tx.Where("run_id IN ?", runIDs).Delete(&model.AgentEvidence{})
			if evidenceResult.Error != nil {
				return evidenceResult.Error
			}
			cleared.DeletedEvidence = evidenceResult.RowsAffected

			runResult := tx.Where("id IN ?", runIDs).Delete(&model.AgentRun{})
			if runResult.Error != nil {
				return runResult.Error
			}
			cleared.DeletedRuns = runResult.RowsAffected
		}

		messageQuery := tx.Where("1 = 1")
		if sessionID != "" {
			messageQuery = messageQuery.Where("session_id = ?", sessionID)
		}
		messageResult := messageQuery.Delete(&model.AgentMessage{})
		if messageResult.Error != nil {
			return messageResult.Error
		}
		cleared.DeletedMessages = messageResult.RowsAffected

		sessionQuery := tx.Where("1 = 1")
		if sessionID != "" {
			sessionQuery = sessionQuery.Where("session_id = ?", sessionID)
		}
		sessionResult := sessionQuery.Delete(&model.AgentSession{})
		if sessionResult.Error != nil {
			return sessionResult.Error
		}
		cleared.DeletedSessions = sessionResult.RowsAffected
		return nil
	})
	return cleared, err
}
