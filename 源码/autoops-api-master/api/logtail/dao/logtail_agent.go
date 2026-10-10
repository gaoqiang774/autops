package dao

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type LogAgentDAO struct{}
type LogAgentOperationDAO struct{}
type LogK8SCollectorOperationDAO struct{}
type LogK8SClusterCollectorOperationDAO struct{}

var (
	logAgentSchemaOnce sync.Once
	logAgentSchemaErr  error
)

func NewLogAgentDAO() *LogAgentDAO {
	return &LogAgentDAO{}
}

func NewLogAgentOperationDAO() *LogAgentOperationDAO {
	return &LogAgentOperationDAO{}
}

func NewLogK8SCollectorOperationDAO() *LogK8SCollectorOperationDAO {
	return &LogK8SCollectorOperationDAO{}
}

func NewLogK8SClusterCollectorOperationDAO() *LogK8SClusterCollectorOperationDAO {
	return &LogK8SClusterCollectorOperationDAO{}
}

func (d *LogAgentDAO) List(query logtailmodel.LogAgentQuery, heartbeatThreshold time.Time) ([]logtailmodel.LogAgentVO, int64, error) {
	var items []logtailmodel.LogAgentVO
	var total int64

	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, 0, err
	}

	db := applyLogAgentFilters(d.baseAgentQuery(), query, heartbeatThreshold)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := applyLogAgentSelect(db, heartbeatThreshold).
		Order("ma.id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Scan(&items).Error
	return items, total, err
}

func (d *LogAgentDAO) GetByAgentID(agentID uint, heartbeatThreshold time.Time) (*logtailmodel.LogAgentVO, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}

	var item logtailmodel.LogAgentVO
	err := applyLogAgentSelect(d.baseAgentQuery().Where("ma.id = ?", agentID), heartbeatThreshold).
		Limit(1).
		Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.AgentID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (d *LogAgentDAO) GetByHostID(hostID uint, heartbeatThreshold time.Time) (*logtailmodel.LogAgentVO, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}

	var item logtailmodel.LogAgentVO
	err := applyLogAgentSelect(d.baseAgentQuery().Where("ma.host_id = ?", hostID), heartbeatThreshold).
		Limit(1).
		Scan(&item).Error
	if err != nil {
		return nil, err
	}
	if item.AgentID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func (d *LogAgentDAO) UpsertStatus(item *logtailmodel.LogAgentStatus) error {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}

	now := time.Now()
	updates := map[string]interface{}{
		"host_id":                item.HostID,
		"runtime_type":           item.RuntimeType,
		"cluster_id":             item.ClusterID,
		"node_name":              item.NodeName,
		"vector_version":         item.VectorVersion,
		"vector_status":          item.VectorStatus,
		"vector_pid":             item.VectorPID,
		"install_path":           item.InstallPath,
		"config_path":            item.ConfigPath,
		"desired_config_version": item.DesiredConfigVersion,
		"applied_config_version": item.AppliedConfigVersion,
		"desired_config_hash":    item.DesiredConfigHash,
		"applied_config_hash":    item.AppliedConfigHash,
		"config_status":          item.ConfigStatus,
		"last_heartbeat_at":      item.LastHeartbeatAt,
		"last_error":             item.LastError,
		"last_error_at":          item.LastErrorAt,
		"metrics_json":           item.MetricsJSON,
		"updated_at":             now,
	}
	if item.LastReloadAt != nil {
		updates["last_reload_at"] = item.LastReloadAt
	}
	db := common.GetDB().Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	result := db.Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", item.AgentID).
		Updates(updates)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	return db.Create(item).Error
}

func (d *LogAgentDAO) ClearLastError(agentID uint) error {
	if agentID == 0 {
		return nil
	}
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	return common.GetDB().Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", agentID).
		Updates(map[string]interface{}{
			"last_error":    "",
			"last_error_at": nil,
			"updated_at":    time.Now(),
		}).Error
}

func (d *LogAgentDAO) MarkConfigApplied(agentID, hostID uint, configVersion int) error {
	if agentID == 0 {
		return nil
	}
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"config_status":  logtailmodel.LogAgentConfigStatusApplied,
		"last_reload_at": &now,
		"last_error":     "",
		"last_error_at":  nil,
		"updated_at":     now,
	}
	if configVersion > 0 {
		updates["desired_config_version"] = configVersion
		updates["applied_config_version"] = configVersion
	}
	result := common.GetDB().Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", agentID).
		Updates(updates)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	return common.GetDB().Create(&logtailmodel.LogAgentStatus{
		AgentID:              agentID,
		HostID:               hostID,
		RuntimeType:          logtailmodel.LogAgentRuntimeHost,
		DesiredConfigVersion: configVersion,
		AppliedConfigVersion: configVersion,
		ConfigStatus:         logtailmodel.LogAgentConfigStatusApplied,
		LastReloadAt:         &now,
		LastError:            "",
		LastErrorAt:          nil,
	}).Error
}

func (d *LogAgentDAO) SetLastError(agentID, hostID uint, errorMsg string) error {
	if agentID == 0 {
		return nil
	}
	errorMsg = strings.TrimSpace(errorMsg)
	if errorMsg == "" {
		return d.ClearLastError(agentID)
	}
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"last_error":    errorMsg,
		"last_error_at": &now,
		"updated_at":    now,
	}
	result := common.GetDB().Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", agentID).
		Updates(updates)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	return common.GetDB().Create(&logtailmodel.LogAgentStatus{
		AgentID:      agentID,
		HostID:       hostID,
		RuntimeType:  logtailmodel.LogAgentRuntimeHost,
		VectorStatus: logtailmodel.VectorStatusError,
		ConfigStatus: logtailmodel.LogAgentConfigStatusFailed,
		LastError:    errorMsg,
		LastErrorAt:  &now,
	}).Error
}

func (d *LogAgentDAO) SetConfigError(agentID, hostID uint, errorMsg string, desiredConfigVersion int) error {
	if agentID == 0 {
		return nil
	}
	errorMsg = strings.TrimSpace(errorMsg)
	if errorMsg == "" {
		return d.ClearLastError(agentID)
	}
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"config_status": logtailmodel.LogAgentConfigStatusFailed,
		"last_error":    errorMsg,
		"last_error_at": &now,
		"updated_at":    now,
	}
	if desiredConfigVersion > 0 {
		updates["desired_config_version"] = desiredConfigVersion
	}
	result := common.GetDB().Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", agentID).
		Updates(updates)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	return common.GetDB().Create(&logtailmodel.LogAgentStatus{
		AgentID:              agentID,
		HostID:               hostID,
		RuntimeType:          logtailmodel.LogAgentRuntimeHost,
		VectorStatus:         logtailmodel.VectorStatusError,
		DesiredConfigVersion: desiredConfigVersion,
		ConfigStatus:         logtailmodel.LogAgentConfigStatusFailed,
		LastError:            errorMsg,
		LastErrorAt:          &now,
	}).Error
}

func (d *LogAgentDAO) SetConfigPending(agentID, hostID uint, desiredConfigVersion int) error {
	if agentID == 0 {
		return nil
	}
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"config_status": logtailmodel.LogAgentConfigStatusPending,
		"last_error":    "",
		"last_error_at": nil,
		"updated_at":    now,
	}
	if desiredConfigVersion > 0 {
		updates["desired_config_version"] = desiredConfigVersion
	}
	result := common.GetDB().Model(&logtailmodel.LogAgentStatus{}).
		Where("agent_id = ?", agentID).
		Updates(updates)
	if result.Error != nil || result.RowsAffected > 0 {
		return result.Error
	}
	return common.GetDB().Create(&logtailmodel.LogAgentStatus{
		AgentID:              agentID,
		HostID:               hostID,
		RuntimeType:          logtailmodel.LogAgentRuntimeHost,
		DesiredConfigVersion: desiredConfigVersion,
		ConfigStatus:         logtailmodel.LogAgentConfigStatusPending,
		LastError:            "",
		LastErrorAt:          nil,
	}).Error
}

func ensureLogAgentRuntimeSchema() error {
	logAgentSchemaOnce.Do(func() {
		db := common.GetDB()
		if db == nil {
			logAgentSchemaErr = fmt.Errorf("database not initialized")
			return
		}
		if !db.Migrator().HasTable("log_agent_status") {
			if err := db.AutoMigrate(&logtailmodel.LogAgentStatus{}); err != nil {
				logAgentSchemaErr = err
				return
			}
		}
		if !db.Migrator().HasTable("log_agent_operation") {
			if err := db.AutoMigrate(&logtailmodel.LogAgentOperation{}); err != nil {
				logAgentSchemaErr = err
				return
			}
		}
		if !db.Migrator().HasTable("log_k8s_collector_operation") {
			if err := db.AutoMigrate(&logtailmodel.LogK8SCollectorOperation{}); err != nil {
				logAgentSchemaErr = err
				return
			}
		}
		if !db.Migrator().HasTable("log_k8s_cluster_collector_operation") {
			if err := db.AutoMigrate(&logtailmodel.LogK8SClusterCollectorOperation{}); err != nil {
				logAgentSchemaErr = err
				return
			}
		}

		statusColumns := []struct {
			name string
			sql  string
		}{
			{"runtime_type", "ALTER TABLE log_agent_status ADD COLUMN runtime_type varchar(32) NOT NULL DEFAULT 'host'"},
			{"cluster_id", "ALTER TABLE log_agent_status ADD COLUMN cluster_id bigint unsigned DEFAULT 0"},
			{"node_name", "ALTER TABLE log_agent_status ADD COLUMN node_name varchar(128) DEFAULT ''"},
			{"vector_version", "ALTER TABLE log_agent_status ADD COLUMN vector_version varchar(64) DEFAULT ''"},
			{"vector_status", "ALTER TABLE log_agent_status ADD COLUMN vector_status varchar(32) NOT NULL DEFAULT 'not_installed'"},
			{"vector_pid", "ALTER TABLE log_agent_status ADD COLUMN vector_pid bigint DEFAULT 0"},
			{"install_path", "ALTER TABLE log_agent_status ADD COLUMN install_path varchar(256) DEFAULT ''"},
			{"config_path", "ALTER TABLE log_agent_status ADD COLUMN config_path varchar(256) DEFAULT ''"},
			{"desired_config_version", "ALTER TABLE log_agent_status ADD COLUMN desired_config_version bigint DEFAULT 0"},
			{"applied_config_version", "ALTER TABLE log_agent_status ADD COLUMN applied_config_version bigint DEFAULT 0"},
			{"desired_config_hash", "ALTER TABLE log_agent_status ADD COLUMN desired_config_hash varchar(128) DEFAULT ''"},
			{"applied_config_hash", "ALTER TABLE log_agent_status ADD COLUMN applied_config_hash varchar(128) DEFAULT ''"},
			{"config_status", "ALTER TABLE log_agent_status ADD COLUMN config_status varchar(32) NOT NULL DEFAULT 'pending'"},
			{"last_heartbeat_at", "ALTER TABLE log_agent_status ADD COLUMN last_heartbeat_at datetime NULL"},
			{"last_reload_at", "ALTER TABLE log_agent_status ADD COLUMN last_reload_at datetime NULL"},
			{"last_error", "ALTER TABLE log_agent_status ADD COLUMN last_error text NULL"},
			{"last_error_at", "ALTER TABLE log_agent_status ADD COLUMN last_error_at datetime NULL"},
			{"metrics_json", "ALTER TABLE log_agent_status ADD COLUMN metrics_json text NULL"},
			{"created_at", "ALTER TABLE log_agent_status ADD COLUMN created_at datetime NULL"},
			{"updated_at", "ALTER TABLE log_agent_status ADD COLUMN updated_at datetime NULL"},
		}
		for _, column := range statusColumns {
			if err := ensureLogAgentColumn(db, "log_agent_status", column.name, column.sql); err != nil {
				logAgentSchemaErr = err
				return
			}
		}

		operationColumns := []struct {
			name string
			sql  string
		}{
			{"agent_id", "ALTER TABLE log_agent_operation ADD COLUMN agent_id bigint unsigned NOT NULL DEFAULT 0"},
			{"host_id", "ALTER TABLE log_agent_operation ADD COLUMN host_id bigint unsigned NOT NULL DEFAULT 0"},
			{"op_type", "ALTER TABLE log_agent_operation ADD COLUMN op_type varchar(64) NOT NULL DEFAULT ''"},
			{"op_status", "ALTER TABLE log_agent_operation ADD COLUMN op_status varchar(32) NOT NULL DEFAULT 'pending'"},
			{"progress", "ALTER TABLE log_agent_operation ADD COLUMN progress bigint NOT NULL DEFAULT 0"},
			{"progress_text", "ALTER TABLE log_agent_operation ADD COLUMN progress_text varchar(128) DEFAULT ''"},
			{"target_version", "ALTER TABLE log_agent_operation ADD COLUMN target_version varchar(64) DEFAULT ''"},
			{"config_version", "ALTER TABLE log_agent_operation ADD COLUMN config_version bigint DEFAULT 0"},
			{"request_json", "ALTER TABLE log_agent_operation ADD COLUMN request_json text NULL"},
			{"result_json", "ALTER TABLE log_agent_operation ADD COLUMN result_json text NULL"},
			{"error_msg", "ALTER TABLE log_agent_operation ADD COLUMN error_msg text NULL"},
			{"operator", "ALTER TABLE log_agent_operation ADD COLUMN operator varchar(64) DEFAULT ''"},
			{"started_at", "ALTER TABLE log_agent_operation ADD COLUMN started_at datetime NULL"},
			{"finished_at", "ALTER TABLE log_agent_operation ADD COLUMN finished_at datetime NULL"},
			{"created_at", "ALTER TABLE log_agent_operation ADD COLUMN created_at datetime NULL"},
			{"updated_at", "ALTER TABLE log_agent_operation ADD COLUMN updated_at datetime NULL"},
		}
		for _, column := range operationColumns {
			if err := ensureLogAgentColumn(db, "log_agent_operation", column.name, column.sql); err != nil {
				logAgentSchemaErr = err
				return
			}
		}
	})
	return logAgentSchemaErr
}

func ensureLogAgentColumn(db *gorm.DB, tableName, columnName, alterSQL string) error {
	var count int64
	err := db.Raw(`
SELECT COUNT(1)
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND column_name = ?`, tableName, columnName).Scan(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	log.Printf("Logtail runtime schema adding missing column: %s.%s", tableName, columnName)
	return db.Exec(alterSQL).Error
}

func (d *LogAgentDAO) CountCollectTasksForAgent(agentID, hostID uint) (int64, error) {
	var total int64
	db := applyAgentCollectTaskFilter(common.GetDB().Model(&logtailmodel.LogCollectTask{}), agentID, hostID)
	err := db.Count(&total).Error
	return total, err
}

func (d *LogAgentDAO) ListCollectTasksForAgent(agentID, hostID uint, query logtailmodel.LogCollectTaskQuery) ([]logtailmodel.LogCollectTask, int64, error) {
	var items []logtailmodel.LogCollectTask
	var total int64
	db := applyAgentCollectTaskFilter(common.GetDB().Model(&logtailmodel.LogCollectTask{}), agentID, hostID)
	if query.ProjectID > 0 {
		db = db.Where("project_id = ?", query.ProjectID)
	}
	if query.StoreID > 0 {
		db = db.Where("store_id = ?", query.StoreID)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR code LIKE ? OR path_pattern LIKE ?", like, like, like)
	}
	if collectType := strings.TrimSpace(query.CollectType); collectType != "" {
		db = db.Where("collect_type = ?", collectType)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogAgentDAO) ListEnabledCollectTasksForAgent(agentID, hostID uint) ([]logtailmodel.LogCollectTask, error) {
	var items []logtailmodel.LogCollectTask
	err := applyAgentCollectTaskFilter(common.GetDB().Model(&logtailmodel.LogCollectTask{}), agentID, hostID).
		Where("status = ?", logtailmodel.LogStatusEnabled).
		Order("id ASC").
		Find(&items).Error
	return items, err
}

func (d *LogAgentDAO) ListAgentsForCollectTask(task logtailmodel.LogCollectTask, agentIDs, hostIDs []uint, query logtailmodel.LogAgentQuery, heartbeatThreshold time.Time) ([]logtailmodel.LogAgentVO, int64, error) {
	var items []logtailmodel.LogAgentVO
	var total int64

	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, 0, err
	}

	db := applyCollectTaskAgentFilter(d.baseAgentQuery(), task, agentIDs, hostIDs)
	db = applyLogAgentFilters(db, query, heartbeatThreshold)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	db = applyLogAgentSelect(db, heartbeatThreshold).Order("ma.id DESC")
	if query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	err := db.Scan(&items).Error
	return items, total, err
}

func (d *LogAgentDAO) baseAgentQuery() *gorm.DB {
	return common.GetDB().
		Table("monitor_agent ma").
		Joins("LEFT JOIN cmdb_host ch ON ch.id = ma.host_id").
		Joins("LEFT JOIN log_agent_status las ON las.agent_id = ma.id").
		Joins(`LEFT JOIN (
			SELECT lao.*
			FROM log_agent_operation lao
			INNER JOIN (
				SELECT agent_id, MAX(id) AS id
				FROM log_agent_operation
				WHERE op_status IN ('pending', 'running')
				GROUP BY agent_id
			) latest_lao ON latest_lao.id = lao.id
		) lao_current ON lao_current.agent_id = ma.id`)
}

func applyLogAgentSelect(db *gorm.DB, heartbeatThreshold time.Time) *gorm.DB {
	return db.Select(`
		COALESCE(las.id, 0) AS id,
		ma.id AS agent_id,
		ma.host_id AS host_id,
		ma.host_name AS host_name,
		ch.name AS name,
		ch.private_ip AS private_ip,
		ch.public_ip AS public_ip,
		ch.ssh_ip AS ssh_ip,
		ch.os AS os,
		ma.version AS agent_version,
		ma.status AS agent_status,
		CASE WHEN ma.status = ? AND ma.last_heartbeat >= ? THEN ? ELSE ? END AS agent_online_status,
		ma.last_heartbeat AS agent_last_heartbeat,
		COALESCE(las.vector_version, '') AS vector_version,
		CASE
			WHEN las.vector_status = ? THEN ?
			WHEN COALESCE(las.vector_status, '') = '' THEN ?
			ELSE las.vector_status
		END AS vector_status,
		COALESCE(las.vector_pid, 0) AS vector_pid,
		COALESCE(las.install_path, '') AS install_path,
		COALESCE(las.config_path, '') AS config_path,
		COALESCE(las.desired_config_version, 0) AS desired_config_version,
		COALESCE(las.applied_config_version, 0) AS applied_config_version,
		COALESCE(las.desired_config_hash, '') AS desired_config_hash,
		COALESCE(las.applied_config_hash, '') AS applied_config_hash,
		COALESCE(las.config_status, ?) AS config_status,
		las.last_heartbeat_at AS last_vector_heartbeat,
		las.last_reload_at AS last_reload_at,
		COALESCE(las.last_error, '') AS last_error,
		las.last_error_at AS last_error_at,
		COALESCE(lao_current.id, 0) AS current_operation_id,
		COALESCE(lao_current.op_type, '') AS current_operation_type,
		COALESCE(lao_current.op_status, '') AS current_operation_status,
		COALESCE(lao_current.progress, 0) AS current_operation_progress,
		COALESCE(lao_current.progress_text, '') AS current_operation_progress_text
	`, cmdbmodel.AgentStatusRunning, heartbeatThreshold, cmdbmodel.AgentOnlineStatusOnline, cmdbmodel.AgentOnlineStatusOffline, "stopped", logtailmodel.VectorStatusError, logtailmodel.VectorStatusNotInstalled, logtailmodel.LogAgentConfigStatusPending)
}

func applyLogAgentFilters(db *gorm.DB, query logtailmodel.LogAgentQuery, heartbeatThreshold time.Time) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("(ma.host_name LIKE ? OR ch.name LIKE ? OR ch.private_ip LIKE ? OR ch.public_ip LIKE ? OR ch.ssh_ip LIKE ?)", like, like, like, like, like)
	}
	if query.OnlineStatus > 0 {
		switch query.OnlineStatus {
		case cmdbmodel.AgentOnlineStatusOnline:
			db = db.Where("ma.status = ? AND ma.last_heartbeat >= ?", cmdbmodel.AgentStatusRunning, heartbeatThreshold)
		case cmdbmodel.AgentOnlineStatusOffline:
			db = db.Where("(ma.status <> ? OR ma.last_heartbeat IS NULL OR ma.last_heartbeat < ?)", cmdbmodel.AgentStatusRunning, heartbeatThreshold)
		}
	}
	if vectorStatus := strings.TrimSpace(query.VectorStatus); vectorStatus != "" {
		if vectorStatus == "stopped" {
			vectorStatus = logtailmodel.VectorStatusError
		}
		if vectorStatus == logtailmodel.VectorStatusNotInstalled {
			db = db.Where("(las.vector_status = ? OR las.vector_status IS NULL OR las.id IS NULL)", vectorStatus)
		} else if vectorStatus == logtailmodel.VectorStatusError {
			db = db.Where("(las.vector_status = ? OR las.vector_status = ?)", vectorStatus, "stopped")
		} else {
			db = db.Where("las.vector_status = ?", vectorStatus)
		}
	}
	if configStatus := strings.TrimSpace(query.ConfigStatus); configStatus != "" {
		db = db.Where("las.config_status = ?", configStatus)
	}
	return db
}

func applyAgentCollectTaskFilter(db *gorm.DB, agentID, hostID uint) *gorm.DB {
	agentLike := fmt.Sprintf("%%\"agentId\":%d%%", agentID)
	agentLikeWithSpace := fmt.Sprintf("%%\"agentId\": %d%%", agentID)
	hostLike := fmt.Sprintf("%%\"hostId\":%d%%", hostID)
	hostLikeWithSpace := fmt.Sprintf("%%\"hostId\": %d%%", hostID)
	return db.Where(
		"collect_type IN ? AND (execute_targets_json LIKE ? OR execute_targets_json LIKE ? OR execute_targets_json LIKE ? OR execute_targets_json LIKE ?)",
		[]string{"host", "logtail_host"},
		agentLike,
		agentLikeWithSpace,
		hostLike,
		hostLikeWithSpace,
	)
}

func applyCollectTaskAgentFilter(db *gorm.DB, task logtailmodel.LogCollectTask, agentIDs, hostIDs []uint) *gorm.DB {
	collectType := strings.TrimSpace(task.CollectType)
	if collectType != "host" && collectType != "logtail_host" {
		return db.Where("1 = 0")
	}

	switch {
	case len(agentIDs) > 0 && len(hostIDs) > 0:
		return db.Where("(ma.id IN ? OR ma.host_id IN ?)", agentIDs, hostIDs)
	case len(agentIDs) > 0:
		return db.Where("ma.id IN ?", agentIDs)
	case len(hostIDs) > 0:
		return db.Where("ma.host_id IN ?", hostIDs)
	default:
		return db.Where("1 = 0")
	}
}

func (d *LogAgentOperationDAO) Create(item *logtailmodel.LogAgentOperation) error {
	if item.OpStatus == "" {
		item.OpStatus = logtailmodel.LogAgentOperationStatusPending
	}
	if item.ProgressText == "" {
		item.ProgressText = "等待执行"
	}
	return common.GetDB().Create(item).Error
}

func (d *LogAgentOperationDAO) HasActive(agentID uint, opType string) (bool, error) {
	var total int64
	err := common.GetDB().Model(&logtailmodel.LogAgentOperation{}).
		Where("agent_id = ? AND op_type = ? AND op_status IN ?", agentID, opType, []string{
			logtailmodel.LogAgentOperationStatusPending,
			logtailmodel.LogAgentOperationStatusRunning,
		}).
		Count(&total).Error
	return total > 0, err
}

func (d *LogAgentOperationDAO) GetActive(agentID uint, opType string) (*logtailmodel.LogAgentOperation, error) {
	var item logtailmodel.LogAgentOperation
	err := common.GetDB().Where("agent_id = ? AND op_type = ? AND op_status IN ?", agentID, opType, []string{
		logtailmodel.LogAgentOperationStatusPending,
		logtailmodel.LogAgentOperationStatusRunning,
	}).Order("id DESC").First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (d *LogAgentOperationDAO) CancelPending(id, agentID uint, reason string) error {
	if id == 0 || agentID == 0 {
		return nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "操作已被新的请求替换"
	}
	now := time.Now()
	return common.GetDB().Model(&logtailmodel.LogAgentOperation{}).
		Where("id = ? AND agent_id = ? AND op_status = ?", id, agentID, logtailmodel.LogAgentOperationStatusPending).
		Updates(map[string]interface{}{
			"op_status":     logtailmodel.LogAgentOperationStatusCanceled,
			"progress_text": reason,
			"error_msg":     reason,
			"finished_at":   now,
			"updated_at":    now,
		}).Error
}

func (d *LogAgentOperationDAO) ClaimPending(agentID uint, limit int) ([]logtailmodel.LogAgentOperation, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	var items []logtailmodel.LogAgentOperation
	now := time.Now()
	err := common.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ? AND op_status = ?", agentID, logtailmodel.LogAgentOperationStatusPending).
			Order("id ASC").
			Limit(limit).
			Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		ids := make([]uint, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		if err := tx.Model(&logtailmodel.LogAgentOperation{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"op_status":     logtailmodel.LogAgentOperationStatusRunning,
				"progress":      0,
				"progress_text": "开始执行",
				"started_at":    now,
				"updated_at":    now,
			}).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OpStatus = logtailmodel.LogAgentOperationStatusRunning
			items[i].Progress = 0
			items[i].ProgressText = "开始执行"
			items[i].StartedAt = &now
		}
		return nil
	})
	return items, err
}

func (d *LogAgentOperationDAO) Report(id, agentID uint, status string, progress int, progressText, resultJSON, errorMsg string) (*logtailmodel.LogAgentOperation, error) {
	now := time.Now()
	updates := map[string]interface{}{
		"op_status":     status,
		"progress":      progress,
		"progress_text": progressText,
		"result_json":   resultJSON,
		"error_msg":     errorMsg,
		"finished_at":   now,
		"updated_at":    now,
	}
	if status == logtailmodel.LogAgentOperationStatusRunning {
		updates["finished_at"] = nil
	}
	if err := common.GetDB().Model(&logtailmodel.LogAgentOperation{}).
		Where("id = ? AND agent_id = ?", id, agentID).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	var item logtailmodel.LogAgentOperation
	err := common.GetDB().Where("id = ? AND agent_id = ?", id, agentID).First(&item).Error
	return &item, err
}

func (d *LogAgentOperationDAO) PruneCompleted(agentID, keepID uint) error {
	if agentID == 0 || keepID == 0 {
		return nil
	}
	return common.GetDB().Where(
		"agent_id = ? AND id <> ? AND op_status NOT IN ?",
		agentID,
		keepID,
		[]string{
			logtailmodel.LogAgentOperationStatusPending,
			logtailmodel.LogAgentOperationStatusRunning,
		},
	).Delete(&logtailmodel.LogAgentOperation{}).Error
}

func (d *LogAgentOperationDAO) List(agentID uint, query logtailmodel.LogAgentOperationQuery) ([]logtailmodel.LogAgentOperation, int64, error) {
	var items []logtailmodel.LogAgentOperation
	db := common.GetDB().Model(&logtailmodel.LogAgentOperation{}).Where("agent_id = ?", agentID)
	if opType := strings.TrimSpace(query.OpType); opType != "" {
		db = db.Where("op_type = ?", opType)
	}
	if opStatus := strings.TrimSpace(query.OpStatus); opStatus != "" {
		db = db.Where("op_status = ?", opStatus)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogK8SCollectorOperationDAO) Create(item *logtailmodel.LogK8SCollectorOperation) error {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	if item.OpStatus == "" {
		item.OpStatus = logtailmodel.LogAgentOperationStatusPending
	}
	if item.ProgressText == "" {
		item.ProgressText = "等待执行"
	}
	return common.GetDB().Create(item).Error
}

func (d *LogK8SCollectorOperationDAO) UpdateProgress(id uint, status string, progress int, progressText, resultJSON, errorMsg string) (*logtailmodel.LogK8SCollectorOperation, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"op_status":     status,
		"progress":      progress,
		"progress_text": progressText,
		"result_json":   resultJSON,
		"error_msg":     errorMsg,
		"updated_at":    now,
	}
	if status == logtailmodel.LogAgentOperationStatusRunning {
		updates["started_at"] = now
		updates["finished_at"] = nil
	} else {
		updates["finished_at"] = now
	}
	if err := common.GetDB().Model(&logtailmodel.LogK8SCollectorOperation{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	var item logtailmodel.LogK8SCollectorOperation
	err := common.GetDB().Where("id = ?", id).First(&item).Error
	return &item, err
}

func (d *LogK8SCollectorOperationDAO) List(query logtailmodel.LogK8SCollectorOperationQuery) ([]logtailmodel.LogK8SCollectorOperation, int64, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, 0, err
	}
	var items []logtailmodel.LogK8SCollectorOperation
	var total int64
	db := common.GetDB().Model(&logtailmodel.LogK8SCollectorOperation{})
	if query.TaskID > 0 {
		db = db.Where("task_id = ?", query.TaskID)
	}
	if query.TargetIndex != nil {
		db = db.Where("target_index = ?", *query.TargetIndex)
	}
	if query.ClusterID > 0 {
		db = db.Where("cluster_id = ?", query.ClusterID)
	}
	if opType := strings.TrimSpace(query.OpType); opType != "" {
		db = db.Where("op_type = ?", opType)
	}
	if opStatus := strings.TrimSpace(query.OpStatus); opStatus != "" {
		db = db.Where("op_status = ?", opStatus)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogK8SCollectorOperationDAO) CountActiveByCluster(clusterID uint, since time.Time) (int64, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return 0, err
	}
	if clusterID == 0 {
		return 0, nil
	}
	db := common.GetDB().Model(&logtailmodel.LogK8SCollectorOperation{}).
		Where("cluster_id = ?", clusterID).
		Where("op_status IN ?", []string{
			logtailmodel.LogAgentOperationStatusPending,
			logtailmodel.LogAgentOperationStatusRunning,
		})
	if !since.IsZero() {
		db = db.Where("updated_at >= ?", since)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func (d *LogK8SCollectorOperationDAO) LatestByTaskTargets(taskID uint) (map[int]logtailmodel.LogK8SCollectorOperation, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}
	var items []logtailmodel.LogK8SCollectorOperation
	err := common.GetDB().Where("task_id = ?", taskID).
		Order("target_index ASC, updated_at DESC, id DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	result := map[int]logtailmodel.LogK8SCollectorOperation{}
	for _, item := range items {
		if _, ok := result[item.TargetIndex]; ok {
			continue
		}
		result[item.TargetIndex] = item
	}
	return result, nil
}

func (d *LogK8SClusterCollectorOperationDAO) Create(item *logtailmodel.LogK8SClusterCollectorOperation) error {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return err
	}
	if item.OpStatus == "" {
		item.OpStatus = logtailmodel.LogAgentOperationStatusPending
	}
	if item.ProgressText == "" {
		item.ProgressText = "等待执行"
	}
	return common.GetDB().Create(item).Error
}

func (d *LogK8SClusterCollectorOperationDAO) UpdateProgress(id uint, status string, progress int, progressText, resultJSON, errorMsg string) (*logtailmodel.LogK8SClusterCollectorOperation, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}
	now := time.Now()
	updates := map[string]interface{}{
		"op_status":     status,
		"progress":      progress,
		"progress_text": progressText,
		"result_json":   resultJSON,
		"error_msg":     errorMsg,
		"updated_at":    now,
	}
	if status == logtailmodel.LogAgentOperationStatusRunning {
		updates["started_at"] = now
		updates["finished_at"] = nil
	} else {
		updates["finished_at"] = now
	}
	if err := common.GetDB().Model(&logtailmodel.LogK8SClusterCollectorOperation{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return nil, err
	}
	var item logtailmodel.LogK8SClusterCollectorOperation
	err := common.GetDB().Where("id = ?", id).First(&item).Error
	return &item, err
}

func (d *LogK8SClusterCollectorOperationDAO) List(query logtailmodel.LogK8SClusterCollectorOperationQuery) ([]logtailmodel.LogK8SClusterCollectorOperation, int64, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, 0, err
	}
	var items []logtailmodel.LogK8SClusterCollectorOperation
	var total int64
	db := common.GetDB().Model(&logtailmodel.LogK8SClusterCollectorOperation{})
	if query.ClusterID > 0 {
		db = db.Where("cluster_id = ?", query.ClusterID)
	}
	if opType := strings.TrimSpace(query.OpType); opType != "" {
		db = db.Where("op_type = ?", opType)
	}
	if opStatus := strings.TrimSpace(query.OpStatus); opStatus != "" {
		db = db.Where("op_status = ?", opStatus)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&items).Error
	return items, total, err
}

func (d *LogK8SClusterCollectorOperationDAO) CountActiveByCluster(clusterID uint, since time.Time) (int64, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return 0, err
	}
	if clusterID == 0 {
		return 0, nil
	}
	db := common.GetDB().Model(&logtailmodel.LogK8SClusterCollectorOperation{}).
		Where("cluster_id = ?", clusterID).
		Where("op_status IN ?", []string{
			logtailmodel.LogAgentOperationStatusPending,
			logtailmodel.LogAgentOperationStatusRunning,
		})
	if !since.IsZero() {
		db = db.Where("updated_at >= ?", since)
	}
	var count int64
	err := db.Count(&count).Error
	return count, err
}

func (d *LogK8SClusterCollectorOperationDAO) LatestByClusters(clusterIDs []uint) (map[uint]logtailmodel.LogK8SClusterCollectorOperation, error) {
	if err := ensureLogAgentRuntimeSchema(); err != nil {
		return nil, err
	}
	result := map[uint]logtailmodel.LogK8SClusterCollectorOperation{}
	if len(clusterIDs) == 0 {
		return result, nil
	}
	var items []logtailmodel.LogK8SClusterCollectorOperation
	err := common.GetDB().Where("cluster_id IN ?", clusterIDs).
		Order("cluster_id ASC, updated_at DESC, id DESC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if _, ok := result[item.ClusterID]; ok {
			continue
		}
		result[item.ClusterID] = item
	}
	return result, nil
}
