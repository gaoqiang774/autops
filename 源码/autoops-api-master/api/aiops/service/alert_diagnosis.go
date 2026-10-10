package service

import (
	"encoding/json"
	"errors"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"

	"github.com/gin-gonic/gin"
)

type AlertDiagnosisService struct {
	agentDao *aiopsdao.AgentDao
}

func NewAlertDiagnosisService() *AlertDiagnosisService {
	return &AlertDiagnosisService{
		agentDao: aiopsdao.NewAgentDao(),
	}
}

func (s *AlertDiagnosisService) AnalyzeAlert(c *gin.Context, req *aiopsmodel.AlertAnalyzeRequest) (*aiopsmodel.AlertDiagnosisResult, error) {
	return nil, errors.New("旧 api/monitor 告警诊断已移除，请使用 alertmanager 告警能力")
}

func (s *AlertDiagnosisService) GetRunDetail(runID uint64) (*aiopsmodel.AgentRunDetail, error) {
	run, err := s.agentDao.GetRun(runID)
	if err != nil {
		return nil, err
	}
	steps, err := s.agentDao.ListSteps(runID)
	if err != nil {
		return nil, err
	}
	evidence, err := s.agentDao.ListEvidence(runID)
	if err != nil {
		return nil, err
	}

	detail := &aiopsmodel.AgentRunDetail{
		Run:      *run,
		Steps:    steps,
		Evidence: compactDiagnosisEvidence(toDiagnosisEvidence(evidence), false),
	}
	if run.ResultJSON == "" {
		return detail, nil
	}

	if run.RunType == aiopsmodel.AgentRunTypeAssistantChat {
		var parsed aiopsmodel.AssistantChatResult
		if json.Unmarshal([]byte(run.ResultJSON), &parsed) == nil {
			detail.AssistantResult = &parsed
		}
		return detail, nil
	}

	var parsed aiopsmodel.AlertDiagnosisResult
	if json.Unmarshal([]byte(run.ResultJSON), &parsed) == nil {
		detail.Result = &parsed
	}
	return detail, nil
}

func (s *AlertDiagnosisService) BuildRunStreamEvents(runID uint64) ([]aiopsmodel.AgentRunStreamEvent, error) {
	detail, err := s.GetRunDetail(runID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	events := make([]aiopsmodel.AgentRunStreamEvent, 0, len(detail.Steps)+3)
	events = append(events, aiopsmodel.AgentRunStreamEvent{
		Type:    "run",
		RunID:   runID,
		Status:  detail.Run.Status,
		Message: "Agent run loaded",
		Time:    now,
	})
	for i := range detail.Steps {
		step := detail.Steps[i]
		events = append(events, aiopsmodel.AgentRunStreamEvent{
			Type:      "step",
			RunID:     runID,
			Status:    step.Status,
			StepIndex: i + 1,
			Step:      &step,
			Error:     step.Error,
			Time:      time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	if detail.AssistantResult != nil {
		events = append(events, aiopsmodel.AgentRunStreamEvent{
			Type:    "result",
			RunID:   runID,
			Status:  detail.Run.Status,
			Result:  detail.AssistantResult,
			Message: "assistant result",
			Time:    time.Now().Format("2006-01-02 15:04:05"),
		})
	} else if detail.Result != nil {
		events = append(events, aiopsmodel.AgentRunStreamEvent{
			Type:    "result",
			RunID:   runID,
			Status:  detail.Run.Status,
			Result:  detail.Result,
			Message: "diagnosis result",
			Time:    time.Now().Format("2006-01-02 15:04:05"),
		})
	}
	events = append(events, aiopsmodel.AgentRunStreamEvent{
		Type:    "done",
		RunID:   runID,
		Status:  detail.Run.Status,
		Message: "stream completed",
		Time:    time.Now().Format("2006-01-02 15:04:05"),
	})
	return events, nil
}

func toDiagnosisEvidence(items []aiopsmodel.AgentEvidence) []aiopsmodel.AgentDiagnosisEvidence {
	result := make([]aiopsmodel.AgentDiagnosisEvidence, 0, len(items))
	for _, item := range items {
		result = append(result, aiopsmodel.AgentDiagnosisEvidence{
			Type:      item.Type,
			Title:     item.Title,
			Reference: item.Reference,
			Content:   item.Content,
		})
	}
	return result
}

func compactDiagnosisEvidence(evidence []aiopsmodel.AgentDiagnosisEvidence, includeRaw bool) []aiopsmodel.AgentDiagnosisEvidence {
	result := make([]aiopsmodel.AgentDiagnosisEvidence, 0, len(evidence))
	for _, item := range evidence {
		if !includeRaw {
			item.Raw = nil
		}
		result = append(result, item)
	}
	return result
}
