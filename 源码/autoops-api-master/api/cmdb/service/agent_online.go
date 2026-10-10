package service

import (
	"time"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/config"
)

const defaultAgentHeartbeatTimeout = 5 * time.Minute

func agentHeartbeatTimeout() time.Duration {
	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatTimeout > 0 {
		return time.Duration(config.Config.Monitor.Agent.HeartbeatTimeout) * time.Second
	}
	return defaultAgentHeartbeatTimeout
}

func isAgentOnline(agent *model.Agent) bool {
	if agent == nil {
		return false
	}
	if agent.Status != model.AgentStatusRunning {
		return false
	}
	if agent.LastHeartbeat.Time.IsZero() {
		return false
	}
	return time.Since(agent.LastHeartbeat.Time) <= agentHeartbeatTimeout()
}

func fillAgentOnlineStatus(agent *model.Agent, agentVO *model.AgentVO) {
	if agent == nil || agentVO == nil {
		return
	}

	if isAgentOnline(agent) {
		agentVO.OnlineStatus = model.AgentOnlineStatusOnline
		agentVO.IsHealthy = true
		if agent.Status == model.AgentStatusRunning {
			agentVO.StatusText = "在线"
		}
		return
	}

	agentVO.OnlineStatus = model.AgentOnlineStatusOffline
	agentVO.IsHealthy = false
	if agent.Status == model.AgentStatusRunning {
		agentVO.StatusText = "离线"
	}
}
