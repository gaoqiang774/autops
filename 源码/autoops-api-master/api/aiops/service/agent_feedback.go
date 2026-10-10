package service

import (
	"errors"
	"strings"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type AgentFeedbackService struct {
	agentDao *aiopsdao.AgentDao
}

func NewAgentFeedbackService() *AgentFeedbackService {
	return &AgentFeedbackService{agentDao: aiopsdao.NewAgentDao()}
}

func (s *AgentFeedbackService) Create(c *gin.Context, req *aiopsmodel.AgentFeedbackRequest) (*aiopsmodel.AgentFeedback, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.RunID == 0 {
		return nil, errors.New("runId is required")
	}
	rating := strings.TrimSpace(req.Rating)
	if !validAgentFeedbackRating(rating) {
		return nil, errors.New("invalid rating")
	}
	userID, username := agentFeedbackUser(c)
	now := time.Now()
	feedback := &aiopsmodel.AgentFeedback{
		RunID:     req.RunID,
		SessionID: strings.TrimSpace(req.SessionID),
		Rating:    rating,
		Reason:    strings.TrimSpace(req.Reason),
		Comment:   strings.TrimSpace(req.Comment),
		Intent:    strings.TrimSpace(req.Intent),
		UserID:    userID,
		Username:  username,
		RawJSON:   compactToolJSON(req.Context),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.agentDao.CreateFeedback(feedback); err != nil {
		return nil, err
	}
	return feedback, nil
}

func validAgentFeedbackRating(rating string) bool {
	switch rating {
	case aiopsmodel.AgentFeedbackUseful,
		aiopsmodel.AgentFeedbackNotUseful,
		aiopsmodel.AgentFeedbackWrong,
		aiopsmodel.AgentFeedbackInsufficient,
		aiopsmodel.AgentFeedbackToolError:
		return true
	default:
		return false
	}
}

func agentFeedbackUser(c *gin.Context) (uint, string) {
	if c == nil {
		return 0, ""
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return 0, ""
	}
	return admin.ID, admin.Username
}
