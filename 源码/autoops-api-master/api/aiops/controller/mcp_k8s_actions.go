package controller

import (
	"net/http"
	"strconv"
	"strings"

	"dodevops-api/api/aiops/service"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

type mcpK8sActionConfirmRequest struct {
	ConfirmationID string `json:"confirmationId" binding:"required"`
}

// GetMCPK8sRolloutStatus returns rollout progress for an action card to poll.
func GetMCPK8sRolloutStatus(c *gin.Context) {
	clusterID, err := strconv.ParseUint(strings.TrimSpace(c.Query("clusterId")), 10, 64)
	if err != nil || clusterID == 0 {
		result.Failed(c, http.StatusBadRequest, "clusterId 无效")
		return
	}
	generation, generationErr := strconv.ParseInt(strings.TrimSpace(c.Query("generation")), 10, 64)
	if strings.TrimSpace(c.Query("generation")) != "" && (generationErr != nil || generation <= 0) {
		result.Failed(c, http.StatusBadRequest, "generation 无效")
		return
	}
	targetReplicas := int64(-1)
	if raw := strings.TrimSpace(c.Query("targetReplicas")); raw != "" {
		var targetErr error
		targetReplicas, targetErr = strconv.ParseInt(raw, 10, 32)
		if targetErr != nil || targetReplicas < 0 {
			result.Failed(c, http.StatusBadRequest, "targetReplicas 无效")
			return
		}
	}
	data, err := service.NewMCPK8sActionService().RolloutStatus(
		c,
		c.Query("action"),
		uint(clusterID),
		c.Query("namespace"),
		c.Query("workloadType"),
		c.Query("workloadName"),
		generation,
		int32(targetReplicas),
	)
	if err != nil {
		result.Failed(c, http.StatusBadRequest, err.Error())
		return
	}
	result.Success(c, data)
}

// ConfirmMCPK8sAction executes a signed K8s workload preview using the current JWT user.
func ConfirmMCPK8sAction(c *gin.Context) {
	var req mcpK8sActionConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ConfirmationID) == "" {
		result.Failed(c, http.StatusBadRequest, "confirmationId 不能为空")
		return
	}
	data, err := service.NewMCPK8sActionService().Confirm(c, strings.TrimSpace(req.ConfirmationID))
	if err != nil {
		result.Failed(c, http.StatusForbidden, err.Error())
		return
	}
	result.Success(c, data)
}
