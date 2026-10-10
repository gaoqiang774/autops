package dao

import (
	"strings"
	"time"

	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/config"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// AgentDao Agent 数据访问接口。
type AgentDao interface {
	Create(agent *model.Agent) error
	GetByID(id uint) (*model.Agent, error)
	GetByHostID(hostID uint) (*model.Agent, error)
	Update(agent *model.Agent) error
	UpdateStatus(id uint, status int, errorMsg string) error
	UpdateHeartbeat(hostID uint, heartbeat *model.AgentHeartbeatDto) error
	UpdateHeartbeatByIP(hostID uint, heartbeat *model.AgentHeartbeatDto) error
	UpdateByHostID(hostID uint, updates map[string]interface{}) error
	UpdateInstallProgress(hostID uint, progress int) error
	Delete(id uint) error
	DeleteByHostID(hostID uint) error
	GetList(dto *model.AgentListDto) ([]model.Agent, int64, error)
	GetAll() ([]model.Agent, error)
	GetVersionStats() ([]model.AgentVersionStat, error)
	GetAllRunning() ([]model.Agent, error)
	GetOfflineAgents(duration time.Duration) ([]model.Agent, error)
	CreateOrUpdate(agent *model.Agent) error
	GetAgentStatistics() (map[string]int64, error)
}

// AgentDaoImpl Agent 数据访问实现。
type AgentDaoImpl struct {
	db *gorm.DB
}

func agentOnlineFilterTimeout() time.Duration {
	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatTimeout > 0 {
		return time.Duration(config.Config.Monitor.Agent.HeartbeatTimeout) * time.Second
	}
	return 5 * time.Minute
}

func (d *AgentDaoImpl) silentDB() *gorm.DB {
	return d.db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

// NewAgentDao 创建 Agent DAO 实例。
func NewAgentDao() AgentDao {
	return &AgentDaoImpl{
		db: common.GetDB(),
	}
}

// Create 创建 Agent 记录。
func (d *AgentDaoImpl) Create(agent *model.Agent) error {
	agent.CreateTime.Time = time.Now()
	agent.UpdateTime.Time = time.Now()
	return d.db.Create(agent).Error
}

// GetByID 根据 ID 获取 Agent。
func (d *AgentDaoImpl) GetByID(id uint) (*model.Agent, error) {
	var agent model.Agent
	err := d.db.Where("id = ?", id).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetByHostID 根据主机 ID 获取 Agent。
func (d *AgentDaoImpl) GetByHostID(hostID uint) (*model.Agent, error) {
	var agent model.Agent
	err := d.db.Where("host_id = ?", hostID).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

// Update 更新 Agent。
func (d *AgentDaoImpl) Update(agent *model.Agent) error {
	agent.UpdateTime.Time = time.Now()
	return d.db.Save(agent).Error
}

// UpdateStatus 更新 Agent 状态。
func (d *AgentDaoImpl) UpdateStatus(id uint, status int, errorMsg string) error {
	updates := map[string]interface{}{
		"status":      status,
		"error_msg":   errorMsg,
		"update_time": time.Now(),
	}
	return d.db.Model(&model.Agent{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateHeartbeat 更新 Agent 心跳信息。
func (d *AgentDaoImpl) UpdateHeartbeat(hostID uint, heartbeat *model.AgentHeartbeatDto) error {
	updates := map[string]interface{}{
		"pid":              heartbeat.PID,
		"last_heartbeat":   time.Now(),
		"update_time":      time.Now(),
		"status":           model.AgentStatusRunning,
		"install_progress": model.InstallProgressStarted,
		"error_msg":        "",
	}
	if version := strings.TrimSpace(heartbeat.Version); version != "" {
		updates["version"] = version
	}
	return d.silentDB().Model(&model.Agent{}).Where("host_id = ?", hostID).Updates(updates).Error
}

// UpdateHeartbeatByIP 通过 IP 更新 Agent 心跳信息。
func (d *AgentDaoImpl) UpdateHeartbeatByIP(hostID uint, heartbeat *model.AgentHeartbeatDto) error {
	updates := map[string]interface{}{
		"pid":              heartbeat.PID,
		"port":             heartbeat.Port,
		"last_heartbeat":   time.Now(),
		"update_time":      time.Now(),
		"status":           model.AgentStatusRunning,
		"install_progress": model.InstallProgressStarted,
		"error_msg":        "",
	}
	if version := strings.TrimSpace(heartbeat.Version); version != "" {
		updates["version"] = version
	}
	return d.silentDB().Model(&model.Agent{}).Where("host_id = ?", hostID).Updates(updates).Error
}

// UpdateInstallProgress 更新 Agent 安装进度。
func (d *AgentDaoImpl) UpdateInstallProgress(hostID uint, progress int) error {
	updates := map[string]interface{}{
		"install_progress": progress,
		"update_time":      time.Now(),
	}
	return d.db.Model(&model.Agent{}).Where("host_id = ?", hostID).Updates(updates).Error
}

// Delete 删除 Agent。
func (d *AgentDaoImpl) Delete(id uint) error {
	return d.db.Delete(&model.Agent{}, id).Error
}

// DeleteByHostID 通过主机 ID 删除 Agent 记录。
func (d *AgentDaoImpl) DeleteByHostID(hostID uint) error {
	return d.db.Where("host_id = ?", hostID).Delete(&model.Agent{}).Error
}

// GetList 获取 Agent 列表。
func (d *AgentDaoImpl) GetList(dto *model.AgentListDto) ([]model.Agent, int64, error) {
	var agents []model.Agent
	var total int64

	query := d.db.Model(&model.Agent{})

	if dto.SSHIP != "" {
		query = query.Joins("LEFT JOIN cmdb_host ON cmdb_host.id = monitor_agent.host_id")
	}

	if dto.HostID > 0 {
		query = query.Where("monitor_agent.host_id = ?", dto.HostID)
	}
	if dto.HostName != "" {
		query = query.Where("monitor_agent.host_name LIKE ?", "%"+dto.HostName+"%")
	}
	if dto.SSHIP != "" {
		ipLike := "%" + dto.SSHIP + "%"
		query = query.Where("(cmdb_host.ssh_ip LIKE ? OR cmdb_host.private_ip LIKE ? OR cmdb_host.public_ip LIKE ?)", ipLike, ipLike, ipLike)
	}
	if dto.Version != "" {
		query = query.Where("monitor_agent.version = ?", dto.Version)
	}
	if dto.Status > 0 {
		query = query.Where("monitor_agent.status = ?", dto.Status)
	}
	if dto.OnlineStatus > 0 {
		threshold := time.Now().Add(-agentOnlineFilterTimeout())
		switch dto.OnlineStatus {
		case model.AgentOnlineStatusOnline:
			query = query.Where("monitor_agent.status = ? AND monitor_agent.last_heartbeat >= ?", model.AgentStatusRunning, threshold)
		case model.AgentOnlineStatusOffline:
			query = query.Where("(monitor_agent.status <> ? OR monitor_agent.last_heartbeat IS NULL OR monitor_agent.last_heartbeat < ?)", model.AgentStatusRunning, threshold)
		}
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := dto.Page
	if page <= 0 {
		page = 1
	}
	pageSize := dto.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	err := query.Select("monitor_agent.*").Order("monitor_agent.id DESC").Offset(offset).Limit(pageSize).Find(&agents).Error

	return agents, total, err
}

// GetAll 获取全部 Agent 记录。
func (d *AgentDaoImpl) GetAll() ([]model.Agent, error) {
	var agents []model.Agent
	err := d.db.Find(&agents).Error
	return agents, err
}

func (d *AgentDaoImpl) GetVersionStats() ([]model.AgentVersionStat, error) {
	var stats []model.AgentVersionStat
	err := d.db.Model(&model.Agent{}).
		Select("COALESCE(NULLIF(version, ''), 'unknown') AS version, COUNT(*) AS count").
		Group("COALESCE(NULLIF(version, ''), 'unknown')").
		Order("version ASC").
		Scan(&stats).Error
	return stats, err
}

// GetAllRunning 获取所有运行中的 Agent。
func (d *AgentDaoImpl) GetAllRunning() ([]model.Agent, error) {
	var agents []model.Agent
	err := d.db.Where("status = ?", model.AgentStatusRunning).Find(&agents).Error
	return agents, err
}

// GetOfflineAgents 获取超过指定时间未上报心跳的 Agent。
func (d *AgentDaoImpl) GetOfflineAgents(duration time.Duration) ([]model.Agent, error) {
	var agents []model.Agent
	threshold := time.Now().Add(-duration)
	err := d.db.Where("status = ? AND last_heartbeat < ?", model.AgentStatusRunning, threshold).Find(&agents).Error
	return agents, err
}

// CreateOrUpdate 根据 hostID 创建或更新 Agent。
func (d *AgentDaoImpl) CreateOrUpdate(agent *model.Agent) error {
	var existing model.Agent
	err := d.db.Where("host_id = ?", agent.HostID).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		return d.Create(agent)
	}
	if err != nil {
		return err
	}

	agent.ID = existing.ID
	agent.CreateTime = existing.CreateTime
	return d.Update(agent)
}

// UpdateByHostID 根据主机 ID 更新 Agent。
func (d *AgentDaoImpl) UpdateByHostID(hostID uint, updates map[string]interface{}) error {
	updates["update_time"] = time.Now()
	return d.db.Model(&model.Agent{}).Where("host_id = ?", hostID).Updates(updates).Error
}

// GetAgentStatistics 获取 Agent 统计信息。
func (d *AgentDaoImpl) GetAgentStatistics() (map[string]int64, error) {
	stats := make(map[string]int64)

	var total int64
	d.db.Model(&model.Agent{}).Count(&total)
	stats["total"] = total

	var running, startError, failed, deploying int64
	d.db.Model(&model.Agent{}).Where("status = ?", model.AgentStatusRunning).Count(&running)
	d.db.Model(&model.Agent{}).Where("status = ?", model.AgentStatusStartError).Count(&startError)
	d.db.Model(&model.Agent{}).Where("status = ?", model.AgentStatusDeployFailed).Count(&failed)
	d.db.Model(&model.Agent{}).Where("status = ?", model.AgentStatusDeploying).Count(&deploying)

	stats["running"] = running
	stats["startError"] = startError
	stats["failed"] = failed
	stats["deploying"] = deploying
	stats["linux"] = total
	stats["windows"] = 0
	stats["darwin"] = 0

	return stats, nil
}
