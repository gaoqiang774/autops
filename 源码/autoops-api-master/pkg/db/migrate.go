package db

import (
	"crypto/sha256"
	aiopsmodel "dodevops-api/api/aiops/model"
	alertmanagermodel "dodevops-api/api/alertmanager/model"
	cmdbmodel "dodevops-api/api/cmdb/model"
	ccmodel "dodevops-api/api/configcenter/model"
	dbmodel "dodevops-api/api/db/model"
	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"
	systemmodel "dodevops-api/api/system/model"
	taskmodel "dodevops-api/api/task/model"
	ticketmodel "dodevops-api/api/ticket/model"
	commonconfig "dodevops-api/common/config"
	"dodevops-api/common/util"

	costcentermodel "dodevops-api/api/costcenter/model"
	"encoding/hex"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var models = []interface{}{
	&cmdbmodel.CmdbGroup{},
	&cmdbmodel.CmdbIdcRoom{},
	&cmdbmodel.CmdbIdcCabinet{},
	&cmdbmodel.CmdbIdcAsset{},
	&cmdbmodel.CmdbIdcVm{},
	&cmdbmodel.CmdbHost{},
	&cmdbmodel.CmdbHostAuth{},
	&cmdbmodel.CmdbWindowsExporterStatus{},
	&cmdbmodel.CmdbSQL{},
	&cmdbmodel.CmdbSnmpDevice{},
	&cmdbmodel.BastionUserGroup{},
	&cmdbmodel.BastionHostAuthorization{},
	&cmdbmodel.BastionAssetAuthorization{},
	&cmdbmodel.HostSSHUploadTask{},
	&cmdbmodel.Agent{},
	&dbmodel.OperationAudit{},
	&dbmodel.SlowLogAnalyzeTask{},
	&dbmodel.SQLTicket{},
	&dbmodel.SQLStatement{},
	&dbmodel.SQLReviewResult{},
	&dbmodel.SQLApprovalRecord{},
	&dbmodel.SQLExecutionRecord{},
	&dbmodel.SQLRollbackRecord{},
	&dbmodel.SQLRuleGroup{},
	&dbmodel.AssetReviewPolicy{},
	// 业务模型
	&cmdbmodel.CmdbBusinessModel{},
	&cmdbmodel.CmdbBusinessModelBinding{},
	&cmdbmodel.CmdbBusinessModelRelation{},
	&cmdbmodel.CmdbBusinessModelHealthSnapshot{},

	&alertmanagermodel.AlertDatasource{},
	&alertmanagermodel.AlertRuleGroup{},
	&alertmanagermodel.AlertRule{},
	&alertmanagermodel.AlertBuiltinRuleTemplate{},
	&alertmanagermodel.AlertBuiltinRuleTemplateGroup{},
	&alertmanagermodel.AlertActiveEvent{},
	&alertmanagermodel.AlertHistoryEvent{},
	&alertmanagermodel.AlertEventTimeline{},
	&alertmanagermodel.AlertSilenceRule{},
	&alertmanagermodel.AlertNotifyChannel{},
	&alertmanagermodel.AlertNotifyGroup{},
	&alertmanagermodel.AlertNotifyRouteRule{},
	&alertmanagermodel.AlertNotifyTemplate{},
	&alertmanagermodel.AlertNotifyRule{},
	&alertmanagermodel.AlertNotifyRecord{},
	&alertmanagermodel.AlertDomain{},
	&alertmanagermodel.SSLCertificate{},
	&alertmanagermodel.SSLCertificateDeployLog{},
	&ticketmodel.WorkflowDefinition{},
	&ticketmodel.WorkflowInstance{},
	&ticketmodel.ApprovalTask{},
	&ticketmodel.Ticket{},
	&ticketmodel.TicketAction{},
	&ticketmodel.ReleaseApplication{},
	&ticketmodel.ReleaseItem{},
	&taskmodel.TaskTemplate{},
	&taskmodel.Task{},
	&taskmodel.TaskWork{},
	&taskmodel.TaskAnsible{},
	&taskmodel.TaskAnsibleWork{},

	&ccmodel.EcsAuth{},
	&ccmodel.KeyManage{},
	&ccmodel.AccountAuth{},
	&aiopsmodel.LLMProvider{},
	&aiopsmodel.AgentSession{},
	&aiopsmodel.AgentMessage{},
	&aiopsmodel.AgentRun{},
	&aiopsmodel.AgentEvidence{},
	&aiopsmodel.AgentFeedback{},
	&aiopsmodel.MCPServerConfig{},
	&aiopsmodel.MCPToolConfig{},
	&aiopsmodel.KnowledgeGroup{},
	&aiopsmodel.KnowledgeDocument{},
	&aiopsmodel.KnowledgeChunk{},
	&logtailmodel.LogProject{},
	&logtailmodel.LogStore{},
	&logtailmodel.LogParseTemplate{},
	&logtailmodel.LogCollectTask{},
	&logtailmodel.LogAgentStatus{},
	&logtailmodel.LogAgentOperation{},
	&logtailmodel.LogK8SCollectorOperation{},
	&logtailmodel.LogK8SClusterCollectorOperation{},
	&logtailmodel.LogAlertRule{},
	&logtailmodel.LogAlertEvent{},
	&k8smodel.KubeCluster{},
	&k8smodel.ClusterInspection{},
	&systemmodel.SysOperationLog{},
	&systemmodel.SysActivityLog{},
	&systemmodel.SysConfig{},
	&systemmodel.SysNavCategory{},
	&systemmodel.SysNavItem{},
	&systemmodel.SysSessionRecording{},
	&systemmodel.SysCommandAudit{},
	&systemmodel.SysRoleMenu{},
	&systemmodel.SysRole{},
	&systemmodel.SysPost{},
	&systemmodel.SysMenu{},
	&systemmodel.SysLoginInfo{},
	&systemmodel.SysDept{},
	&systemmodel.SysAdminRole{},
	&systemmodel.SysAdmin{},

	&costcentermodel.BillDetail{},
	&costcentermodel.BillCloudAccount{},
	&costcentermodel.BillMonthlySummary{},
}

const autoMigrateSignatureKey = "auto_migrate_models_v1"
const autoMigrateBaselineTableThreshold = 10

func AutoMigrate(db *gorm.DB) error {
	signature, err := calculateModelSignature()
	if err != nil {
		return err
	}

	if err := ensureMigrationMetaTable(db); err != nil {
		return err
	}

	storedSignature, err := getStoredMigrationSignature(db, autoMigrateSignatureKey)
	if err != nil {
		return err
	}

	missingModels, missingTables, err := collectMissingModels(db)
	if err != nil {
		return err
	}

	if len(missingModels) > 0 {
		log.Printf("AutoMigrate found %d missing tables: %s",
			len(missingTables), strings.Join(missingTables, ", "))
		if err := migrateModels(db, missingModels, missingTables); err != nil {
			return err
		}
	}

	if err := ensureColumnMigrations(db); err != nil {
		return err
	}
	if err := ensureCostCenterBillDetailIndexes(db); err != nil {
		return err
	}
	if err := ensureKnowledgeGroupMigration(db); err != nil {
		return err
	}
	if err := ensureDefaultKnowledgeGroups(db); err != nil {
		return err
	}
	if err := ensureLogCollectTaskDedupIndexes(db); err != nil {
		return err
	}
	if err := ensureTechnicalManagerPost(db); err != nil {
		return err
	}
	if err := renameTestDirectorPost(db); err != nil {
		return err
	}
	if err := ensureTestGroupRole(db); err != nil {
		return err
	}
	if err := ensureDefaultAlertDatasources(db); err != nil {
		return err
	}
	if err := ensureLogAlertRuleAssociation(db); err != nil {
		return err
	}
	if err := ensureDefaultAlertBuiltinRuleTemplates(db); err != nil {
		return err
	}
	if err := ensureDefaultAlertBuiltinRuleTemplateGroups(db); err != nil {
		return err
	}
	if err := ensureDefaultAlertNotifyTemplates(db); err != nil {
		return err
	}
	if err := removeLegacyAlertNotifyRouteColumns(db); err != nil {
		return err
	}
	if err := ensureBastionAuthorizationLifecycleMigration(db); err != nil {
		return err
	}
	if err := ensureCmdbGroupLifecycleMigration(db); err != nil {
		return err
	}
	if err := ensureDefaultCmdbGroup(db); err != nil {
		return err
	}
	if err := ensureSecretMigrations(db); err != nil {
		return err
	}
	if err := ensureK8sEventsMenu(db); err != nil {
		return err
	}
	if err := ensureK8sMenuIcons(db); err != nil {
		return err
	}
	if err := ensureDomainMonitorMenu(db); err != nil {
		return err
	}
	if err := ensureMCPManagementMenu(db); err != nil {
		return err
	}
	if err := ensureLicenseManagementMenu(db); err != nil {
		return err
	}
	if err := ensureDefaultNavCategories(db); err != nil {
		return err
	}

	if storedSignature == "" {
		existingTableCount, err := countUserTables(db)
		if err != nil {
			return err
		}

		if existingTableCount >= autoMigrateBaselineTableThreshold {
			log.Printf("AutoMigrate baseline initialized: detected %d existing tables, storing signature %s",
				existingTableCount, shortSignature(signature))
			return setStoredMigrationSignature(db, autoMigrateSignatureKey, signature)
		}
	}

	if storedSignature == signature {
		if len(missingModels) == 0 {
			log.Printf("AutoMigrate skipped: model signature unchanged (%s)", shortSignature(signature))
		} else {
			log.Printf("AutoMigrate completed: restored %d missing tables with unchanged signature (%s)",
				len(missingModels), shortSignature(signature))
		}
		return nil
	}

	log.Printf("AutoMigrate running: model signature changed old=%s new=%s",
		shortSignature(storedSignature), shortSignature(signature))

	log.Printf("AutoMigrate note: skipping full schema alter for existing tables; only missing tables are created automatically")
	return setStoredMigrationSignature(db, autoMigrateSignatureKey, signature)
}

func ensureLogAlertRuleAssociation(db *gorm.DB) error {
	if !db.Migrator().HasTable("alert_rules") || !db.Migrator().HasTable("alert_rule_groups") || !db.Migrator().HasTable("alert_datasources") {
		return nil
	}
	var group alertmanagermodel.AlertRuleGroup
	err := db.Where("name = ? AND parent_id = ?", "日志告警", 0).First(&group).Error
	if err == gorm.ErrRecordNotFound {
		group = alertmanagermodel.AlertRuleGroup{Name: "日志告警", Description: "VictoriaLogs LogsQL 日志告警规则", Enabled: 1, CreatedBy: "system", UpdatedBy: "system"}
		if err := db.Create(&group).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	var datasource alertmanagermodel.AlertDatasource
	if err := db.Where("datasource_type = ? AND enabled = ?", alertmanagermodel.AlertDatasourceVictoriaLogs, 1).
		Order("is_default DESC, builtin DESC, id ASC").First(&datasource).Error; err != nil {
		return nil
	}
	updates := map[string]interface{}{"group_id": group.ID}
	if datasource.ID > 0 {
		updates["datasource_id"] = datasource.ID
		updates["datasource_type"] = alertmanagermodel.AlertDatasourceVictoriaLogs
	}
	return db.Model(&alertmanagermodel.AlertRule{}).
		Where("datasource_type = ? AND (group_id = 0 OR datasource_id = 0)", alertmanagermodel.AlertDatasourceVictoriaLogs).
		Updates(updates).Error
}

func collectMissingModels(db *gorm.DB) ([]interface{}, []string, error) {
	missingModels := make([]interface{}, 0)
	missingTables := make([]string, 0)

	for _, model := range models {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(model); err != nil {
			return nil, nil, err
		}

		if db.Migrator().HasTable(stmt.Schema.Table) {
			continue
		}

		missingModels = append(missingModels, model)
		missingTables = append(missingTables, stmt.Schema.Table)
	}

	return missingModels, missingTables, nil
}

func migrateModels(db *gorm.DB, targetModels []interface{}, targetTables []string) error {
	for i, model := range targetModels {
		log.Printf("AutoMigrate creating missing table: %s", targetTables[i])
		if err := db.AutoMigrate(model); err != nil {
			return err
		}
	}

	return nil
}

func ensureColumnMigrations(db *gorm.DB) error {
	type columnMigration struct {
		model interface{}
		field string
		table string
	}

	migrations := make([]columnMigration, 0, 32)
	appendModelColumns := func(model interface{}, table string, fields ...string) {
		for _, field := range fields {
			migrations = append(migrations, columnMigration{model: model, field: field, table: table})
		}
	}

	// The redesigned cost center reuses the legacy bill_detail table. Keep the
	// legacy columns and add every column required by the provider-neutral model.
	appendModelColumns(
		&costcentermodel.BillDetail{},
		"bill_detail",
		"CloudAccountID",
		"ProductCode",
		"BillingItemCode",
		"BillingItemName",
		"UsageQuantity",
		"OriginalAmount",
		"DiscountAmount",
		"PayableAmount",
		"Currency",
		"ChargeType",
		"TransactionType",
		"TransactionID",
		"DeductedAt",
		"BillingStartAt",
		"BillingEndAt",
	)
	appendModelColumns(
		&costcentermodel.BillMonthlySummary{},
		"bill_monthly_summary",
		"CloudAccountID",
		"Operator",
		"Month",
		"TotalCost",
		"Currency",
		"BillStatus",
		"CreatedAt",
		"UpdatedAt",
	)

	migrations = append(migrations, []columnMigration{
		{
			model: &cmdbmodel.CmdbIdcCabinet{},
			field: "TotalU",
			table: "cmdb_idc_cabinet",
		},
		{
			model: &cmdbmodel.CmdbIdcAsset{},
			field: "CabinetPosition",
			table: "cmdb_idc_asset",
		},
		{
			model: &cmdbmodel.CmdbIdcAsset{},
			field: "CabinetUSize",
			table: "cmdb_idc_asset",
		},
		{
			model: &cmdbmodel.CmdbHost{},
			field: "HostType",
			table: "cmdb_host",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "InstanceName",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Version",
			table: "cmdb_db_management",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "ResourceType",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Name",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "DatasourceType",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "URL",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "AuthType",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Username",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Password",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Token",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Config",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Labels",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Description",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Builtin",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Default",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertDatasource{},
			field: "Enabled",
			table: "alert_datasources",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "Note",
			table: "alert_rules",
		},
		{
			model: &dbmodel.SQLTicket{},
			field: "WorkflowInstanceID",
			table: "db_sql_ticket",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "Internal",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "DatasourceQueries",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "RuleConfig",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "AppendTags",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "CronPattern",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "RecoverDurationSeconds",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "EnableInBG",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "TimeZone",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "NotifyRuleIDs",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "NotifyRecovered",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "NotifyRepeatStepMinutes",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "NotifyMaxNumber",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "PipelineConfigs",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "RunbookURL",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "Callbacks",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertRule{},
			field: "SelfHealConfig",
			table: "alert_rules",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "Component",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "Collector",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "Source",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "SourceUUID",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "DefaultEnabled",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "Tags",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "RuleConfig",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertBuiltinRuleTemplate{},
			field: "Sort",
			table: "alert_builtin_rule_templates",
		},
		{
			model: &alertmanagermodel.AlertActiveEvent{},
			field: "ClaimUser",
			table: "alert_active_events",
		},
		{
			model: &alertmanagermodel.AlertActiveEvent{},
			field: "ClaimedAt",
			table: "alert_active_events",
		},
		{
			model: &alertmanagermodel.AlertActiveEvent{},
			field: "TriggerQuery",
			table: "alert_active_events",
		},
		{
			model: &alertmanagermodel.AlertActiveEvent{},
			field: "TriggerCondition",
			table: "alert_active_events",
		},
		{
			model: &alertmanagermodel.AlertActiveEvent{},
			field: "EventDetail",
			table: "alert_active_events",
		},
		{
			model: &alertmanagermodel.AlertNotifyChannel{},
			field: "Description",
			table: "alert_notify_channels",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Ident",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Name",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "ChannelType",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "TemplateType",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Description",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Content",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Builtin",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyTemplate{},
			field: "Enabled",
			table: "alert_notify_templates",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "RepeatNotify",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "RepeatMode",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "RepeatIntervalSeconds",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "RepeatIntervals",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "MaxRepeatTimes",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "RecoveryNotify",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "AggregationEnabled",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "GroupWaitSeconds",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "GroupIntervalSeconds",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRule{},
			field: "AggregationWindowSeconds",
			table: "alert_notify_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRouteRule{},
			field: "NotifyRuleID",
			table: "alert_notify_route_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRouteRule{},
			field: "Conditions",
			table: "alert_notify_route_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRouteRule{},
			field: "OverrideGroup",
			table: "alert_notify_route_rules",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "EventID",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "EventType",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "RuleID",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "RuleName",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "NotifyGroupID",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "NotifyGroupName",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "Request",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "Response",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertNotifyRecord{},
			field: "Operator",
			table: "alert_notify_records",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "DatasourceID",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "DatasourceType",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "FirstTriggeredAt",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "RecoveredAt",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "ClaimUser",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "TriggerQuery",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "TriggerCondition",
			table: "alert_history_events",
		},
		{
			model: &alertmanagermodel.AlertHistoryEvent{},
			field: "EventDetail",
			table: "alert_history_events",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Environment",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Status",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "AuthStatus",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "HostID",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "HostIP",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Port",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Endpoint",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Region",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "IdcRoomID",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "NetworkArea",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "PublicAccess",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "BusinessSystem",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Project",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Owner",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "DBAOwner",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "OpsOwner",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Department",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "CPU",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Memory",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "StorageSizeGB",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "UsedStorageGB",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "StorageType",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "DeployMode",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "NodeCount",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "ClusterName",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "Role",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "PrimaryID",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "AuthMethod",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "SSLEnabled",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "AuditEnabled",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "SecurityLevel",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "RiskTags",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "BackupPolicy",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "LastBackupTime",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "MonitorStatus",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "AlertStatus",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "SlowLogPath",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "PrometheusAccountID",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "PrometheusLabelSelector",
			table: "cmdb_db_management",
		},
		{
			model: &k8smodel.KubeCluster{},
			field: "MonitorDatasourceID",
			table: "k8s_cluster",
		},
		{
			model: &k8smodel.KubeCluster{},
			field: "MonitorDatasourceType",
			table: "k8s_cluster",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "LastInspectionTime",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSQL{},
			field: "ExpireTime",
			table: "cmdb_db_management",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "DeviceVendor",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "DeviceModel",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "SnmpURL",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "SnmpVersion",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "SnmpCommunity",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "SnmpPort",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "RemotePort",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "RemoteUsername",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "RemotePassword",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "RemoteDomain",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "ExporterInstalled",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "ExporterPort",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "ExporterVersion",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "MonitorEnabled",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "Location",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "Owner",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "Remark",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "Status",
			table: "cmdb_snmp_devices",
		},
		{
			model: &cmdbmodel.CmdbSnmpDevice{},
			field: "LastSeen",
			table: "cmdb_snmp_devices",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "StatementType",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "OriginalDatabase",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "OriginalTable",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BackupDatabase",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BackupTable",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BackupRows",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BinlogStartFile",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BinlogStartPos",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BinlogEndFile",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "BinlogEndPos",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "FlashbackTool",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLRollbackRecord{},
			field: "GeneratedAt",
			table: "db_sql_rollback_record",
		},
		{
			model: &dbmodel.SQLExecutionRecord{},
			field: "ResultColumns",
			table: "db_sql_execution_record",
		},
		{
			model: &dbmodel.SQLExecutionRecord{},
			field: "ResultRows",
			table: "db_sql_execution_record",
		},
		{
			model: &dbmodel.SQLTicket{},
			field: "UseGhOst",
			table: "db_sql_ticket",
		},
		{
			model: &dbmodel.SQLTicket{},
			field: "GhOstScript",
			table: "db_sql_ticket",
		},
		{
			model: &dbmodel.SQLExecutionRecord{},
			field: "ExecLog",
			table: "db_sql_execution_record",
		},
		{
			model: &ccmodel.EcsAuth{},
			field: "UpdateTime",
			table: "config_ecsauth",
		},
		{
			model: &cmdbmodel.CmdbGroup{},
			field: "UpdateTime",
			table: "cmdb_group",
		},
		{
			model: &cmdbmodel.BastionHostAuthorization{},
			field: "EffectiveTime",
			table: "bastion_host_authorization",
		},
		{
			model: &cmdbmodel.BastionHostAuthorization{},
			field: "ExpireTime",
			table: "bastion_host_authorization",
		},
		{
			model: &cmdbmodel.BastionHostAuthorization{},
			field: "UpdateTime",
			table: "bastion_host_authorization",
		},
		{
			model: &aiopsmodel.KnowledgeDocument{},
			field: "GroupName",
			table: "aiops_knowledge_document",
		},
		{
			model: &aiopsmodel.KnowledgeDocument{},
			field: "Category",
			table: "aiops_knowledge_document",
		},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "GroupID", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "SummaryStatus", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "SummaryError", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "SearchStatus", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "SearchError", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "SearchVersion", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeDocument{}, field: "ChunkCount", table: "aiops_knowledge_document"},
		{model: &aiopsmodel.KnowledgeChunk{}, field: "SectionPath", table: "aiops_knowledge_chunk"},
		{
			model: &logtailmodel.LogCollectTask{},
			field: "PathPatternsJSON",
			table: "log_collect_task",
		},
		{
			model: &logtailmodel.LogCollectTask{},
			field: "SampleLog",
			table: "log_collect_task",
		},
		{
			model: &logtailmodel.LogCollectTask{},
			field: "RegexPattern",
			table: "log_collect_task",
		},
		{model: &logtailmodel.LogCollectTask{}, field: "MultilineEnabledSnapshot", table: "log_collect_task"},
		{model: &logtailmodel.LogCollectTask{}, field: "NameKey", table: "log_collect_task"},
		{model: &logtailmodel.LogCollectTask{}, field: "ConfigKey", table: "log_collect_task"},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "RuntimeType",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "ClusterID",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "NodeName",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "VectorVersion",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "VectorStatus",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "VectorPID",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "InstallPath",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "ConfigPath",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "DesiredConfigVersion",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "AppliedConfigVersion",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "DesiredConfigHash",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "AppliedConfigHash",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "ConfigStatus",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "LastHeartbeatAt",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "LastReloadAt",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "LastError",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "LastErrorAt",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "MetricsJSON",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "CreatedAt",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentStatus{},
			field: "UpdatedAt",
			table: "log_agent_status",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "AgentID",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "HostID",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "OpType",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "OpStatus",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "TargetVersion",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "ConfigVersion",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "RequestJSON",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "ResultJSON",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "ErrorMsg",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "Operator",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "StartedAt",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "FinishedAt",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "CreatedAt",
			table: "log_agent_operation",
		},
		{
			model: &logtailmodel.LogAgentOperation{},
			field: "UpdatedAt",
			table: "log_agent_operation",
		},
	}...)

	for _, migration := range migrations {
		if !db.Migrator().HasTable(migration.table) {
			continue
		}
		if db.Migrator().HasColumn(migration.model, migration.field) {
			continue
		}

		log.Printf("AutoMigrate adding missing column: %s.%s", migration.table, migration.field)
		if err := db.Migrator().AddColumn(migration.model, migration.field); err != nil {
			return err
		}
	}

	return nil
}

func ensureKnowledgeGroupMigration(db *gorm.DB) error {
	if !db.Migrator().HasTable("aiops_knowledge_group") || !db.Migrator().HasTable("aiops_knowledge_document") ||
		!db.Migrator().HasColumn(&aiopsmodel.KnowledgeDocument{}, "GroupID") {
		return nil
	}
	type legacyKnowledgeDocument struct {
		ID        uint
		GroupID   uint
		GroupName string
		Category  string
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var documents []legacyKnowledgeDocument
		if err := tx.Model(&aiopsmodel.KnowledgeDocument{}).
			Select("id, group_id, group_name, category").
			Where("group_id = 0").Find(&documents).Error; err != nil {
			return err
		}
		for _, document := range documents {
			groupName := strings.TrimSpace(document.GroupName)
			if groupName == "" {
				groupName = "未分组"
			}
			root := aiopsmodel.KnowledgeGroup{Name: groupName}
			if err := tx.Where("parent_id = ? AND name = ?", 0, groupName).FirstOrCreate(&root).Error; err != nil {
				return err
			}
			leafID := root.ID
			category := strings.TrimSpace(document.Category)
			if category != "" && category != groupName {
				child := aiopsmodel.KnowledgeGroup{ParentID: root.ID, Name: category}
				if err := tx.Where("parent_id = ? AND name = ?", root.ID, category).FirstOrCreate(&child).Error; err != nil {
					return err
				}
				leafID = child.ID
			}
			if err := tx.Model(&aiopsmodel.KnowledgeDocument{}).Where("id = ?", document.ID).
				Update("group_id", leafID).Error; err != nil {
				return err
			}
		}
		if err := tx.Exec(`UPDATE aiops_knowledge_document d
			SET d.chunk_count = (SELECT COUNT(*) FROM aiops_knowledge_chunk c WHERE c.document_id = d.id)`).Error; err != nil {
			return err
		}
		if err := tx.Model(&aiopsmodel.KnowledgeDocument{}).
			Where("summary <> '' AND summary_status = ?", aiopsmodel.KnowledgeSummaryPending).
			Update("summary_status", aiopsmodel.KnowledgeSummaryCompleted).Error; err != nil {
			return err
		}
		return nil
	})
}

func ensureLogCollectTaskDedupIndexes(db *gorm.DB) error {
	if !db.Migrator().HasTable("log_collect_task") ||
		!db.Migrator().HasColumn(&logtailmodel.LogCollectTask{}, "NameKey") ||
		!db.Migrator().HasColumn(&logtailmodel.LogCollectTask{}, "ConfigKey") {
		return nil
	}
	for _, indexName := range []string{"uk_log_collect_task_name_key", "uk_log_collect_task_config_key"} {
		if db.Migrator().HasIndex(&logtailmodel.LogCollectTask{}, indexName) {
			continue
		}
		if err := db.Migrator().CreateIndex(&logtailmodel.LogCollectTask{}, indexName); err != nil {
			return fmt.Errorf("create log collect task index %s: %w", indexName, err)
		}
	}
	return nil
}

func ensureTechnicalManagerPost(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_post") {
		return nil
	}

	var count int64
	if err := db.Table("sys_post").
		Where("post_name = ?", "技术经理").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	post := systemmodel.SysPost{
		PostCode:   "technical_manager",
		PostName:   "技术经理",
		PostStatus: 1,
		CreateTime: util.HTime{Time: time.Now()},
		Remark:     "技术管理岗位",
	}
	if err := db.Create(&post).Error; err != nil {
		return err
	}

	log.Printf("AutoMigrate created system post: %s", post.PostName)
	return nil
}

func renameTestDirectorPost(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_post") {
		return nil
	}

	result := db.Table("sys_post").
		Where("post_name = ?", "测试总监").
		Update("post_name", "测试经理")
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("AutoMigrate renamed system post: 测试总监 -> 测试经理")
	}

	return nil
}

func ensureTestGroupRole(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_role") {
		return nil
	}

	var count int64
	if err := db.Table("sys_role").
		Where("role_name = ?", "测试组").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	role := systemmodel.SysRole{
		RoleName:    "测试组",
		RoleKey:     "test_group",
		Status:      1,
		Description: "测试组角色",
		CreateTime:  util.HTime{Time: time.Now()},
	}
	if err := db.Create(&role).Error; err != nil {
		return err
	}

	log.Printf("AutoMigrate created system role: %s", role.RoleName)
	return nil
}

func ensureDefaultAlertDatasources(db *gorm.DB) error {
	if !db.Migrator().HasTable("alert_datasources") {
		return nil
	}

	vmURL := defaultAlertDatasourceURL()
	vlURL := defaultAlertLogDatasourceURL()
	now := util.HTime{Time: time.Now()}
	defaults := []alertmanagermodel.AlertDatasource{
		{
			Name:           "平台内置告警源",
			DatasourceType: alertmanagermodel.AlertDatasourceBuiltin,
			URL:            vmURL,
			AuthType:       "none",
			Config: alertmanagermodel.JSONMap{
				"backend":   alertmanagermodel.AlertDatasourceVictoriaMetrics,
				"queryType": "promql",
			},
			Labels: alertmanagermodel.JSONMap{
				"source":  "platform",
				"backend": alertmanagermodel.AlertDatasourceVictoriaMetrics,
			},
			Description: "平台内置 VictoriaMetrics 告警查询数据源",
			Builtin:     1,
			Default:     1,
			Enabled:     1,
			CreatedBy:   "system",
			UpdatedBy:   "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}
	if vlURL != "" {
		defaults = append(defaults, alertmanagermodel.AlertDatasource{
			Name:           "平台内置日志源",
			DatasourceType: alertmanagermodel.AlertDatasourceVictoriaLogs,
			URL:            vlURL,
			AuthType:       "none",
			Config: alertmanagermodel.JSONMap{
				"backend":   alertmanagermodel.AlertDatasourceVictoriaLogs,
				"queryType": "logsql",
			},
			Labels: alertmanagermodel.JSONMap{
				"source":  "platform",
				"backend": alertmanagermodel.AlertDatasourceVictoriaLogs,
			},
			Description: "平台内置 VictoriaLogs 日志告警查询数据源",
			Builtin:     1,
			Default:     0,
			Enabled:     1,
			CreatedBy:   "system",
			UpdatedBy:   "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	if err := removeLegacyDefaultPrometheusDatasource(session); err != nil {
		return err
	}
	for _, item := range defaults {
		if err := upsertDefaultAlertDatasource(session, item); err != nil {
			return err
		}
	}

	return nil
}

func upsertDefaultAlertDatasource(db *gorm.DB, item alertmanagermodel.AlertDatasource) error {
	var existing alertmanagermodel.AlertDatasource
	err := db.Where("builtin = ? AND datasource_type = ?", 1, item.DatasourceType).
		Take(&existing).Error
	if err == gorm.ErrRecordNotFound {
		err = db.Where("builtin = ? AND name = ?", 1, item.Name).Take(&existing).Error
	}
	if err == gorm.ErrRecordNotFound {
		if err := db.Create(&item).Error; err != nil {
			return err
		}
		log.Printf("AutoMigrate created alert datasource: %s", item.Name)
		return nil
	}
	if err != nil {
		return err
	}

	if existing.Name == item.Name &&
		existing.DatasourceType == item.DatasourceType &&
		existing.URL == item.URL &&
		existing.AuthType == item.AuthType &&
		existing.Description == item.Description &&
		reflect.DeepEqual(existing.Config, item.Config) &&
		reflect.DeepEqual(existing.Labels, item.Labels) {
		return nil
	}

	updates := map[string]interface{}{
		"name":            item.Name,
		"datasource_type": item.DatasourceType,
		"url":             item.URL,
		"auth_type":       item.AuthType,
		"config":          item.Config,
		"labels":          item.Labels,
		"description":     item.Description,
		"builtin":         item.Builtin,
		"updated_by":      item.UpdatedBy,
		"updated_at":      item.UpdatedAt,
	}
	if err := db.Model(&alertmanagermodel.AlertDatasource{}).
		Where("id = ?", existing.ID).
		Updates(updates).Error; err != nil {
		return err
	}
	return nil
}

func defaultAlertDatasourceURL() string {
	if commonconfig.Config == nil {
		return ""
	}
	vmURL := strings.TrimSpace(commonconfig.Config.Monitor.VictoriaMetrics.URL)
	prometheusURL := strings.TrimSpace(commonconfig.Config.Monitor.Prometheus.URL)
	if vmURL == "" {
		vmURL = prometheusURL
	}
	return vmURL
}

func defaultAlertLogDatasourceURL() string {
	if commonconfig.Config == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(commonconfig.Config.Logtail.VictoriaLogs.URL), "/")
}

func removeLegacyDefaultPrometheusDatasource(db *gorm.DB) error {
	var datasource alertmanagermodel.AlertDatasource
	err := db.Where("name = ? AND datasource_type = ? AND builtin = ? AND created_by = ?",
		"默认 Prometheus", alertmanagermodel.AlertDatasourcePrometheus, 1, "system").
		Take(&datasource).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	var count int64
	if err := db.Model(&alertmanagermodel.AlertRule{}).
		Where("datasource_id = ?", datasource.ID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		log.Printf("AutoMigrate kept legacy default Prometheus datasource because it is referenced by %d alert rules", count)
		return nil
	}

	if err := db.Delete(&alertmanagermodel.AlertDatasource{}, datasource.ID).Error; err != nil {
		return err
	}
	log.Printf("AutoMigrate removed legacy default Prometheus datasource: id=%d", datasource.ID)
	return nil
}

func ensureDefaultAlertBuiltinRuleTemplates(db *gorm.DB) error {
	if !db.Migrator().HasTable("alert_builtin_rule_templates") {
		return nil
	}

	now := util.HTime{Time: time.Now()}
	defaults := []alertmanagermodel.AlertBuiltinRuleTemplate{
		{
			ResourceType:               "host",
			Component:                  "Linux",
			Collector:                  "node_exporter",
			Category:                   "cpu",
			Name:                       "主机 CPU 使用率过高",
			Description:                "参考 n9e Linux 内置规则，检测 CPU 使用率持续过高",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.host.cpu.high",
			QueryTemplate:              "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100) > {{threshold}}",
			DefaultConditionOperator:   ">",
			DefaultConditionValue:      85,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  300,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("host", "Linux", "cpu"),
			Annotations:                alertBuiltinAnnotations("主机 CPU 使用率过高", "实例 {{ $labels.instance }} CPU 使用率持续超过 {{threshold}}%。"),
			Variables:                  alertThresholdVariable(85, "%"),
			RuleConfig: alertRuleConfig([]map[string]interface{}{
				{"prom_ql": "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100) > 75", "severity": "info"},
				{"prom_ql": "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100) > 85", "severity": "warning"},
				{"prom_ql": "100 - (avg by(instance) (irate(node_cpu_seconds_total{mode=\"idle\"}[5m])) * 100) > 95", "severity": "critical"},
			}),
			Sort:      100,
			Enabled:   1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ResourceType:               "host",
			Component:                  "Linux",
			Collector:                  "node_exporter",
			Category:                   "memory",
			Name:                       "主机内存可用率过低",
			Description:                "检测 Linux 主机可用内存比例过低",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.host.memory.available.low",
			QueryTemplate:              "node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes * 100 < {{threshold}}",
			DefaultConditionOperator:   "<",
			DefaultConditionValue:      10,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  300,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("host", "Linux", "memory"),
			Annotations:                alertBuiltinAnnotations("主机内存可用率过低", "实例 {{ $labels.instance }} 可用内存低于 {{threshold}}%。"),
			Variables:                  alertThresholdVariable(10, "%"),
			Sort:                       95,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "host",
			Component:                  "Linux",
			Collector:                  "node_exporter",
			Category:                   "disk",
			Name:                       "主机磁盘使用率过高",
			Description:                "参考 n9e Linux 内置规则，检测文件系统使用率过高",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.host.disk.usage.high",
			QueryTemplate:              "(100 - ((node_filesystem_avail_bytes{fstype!~\"tmpfs|overlay\"} * 100) / node_filesystem_size_bytes{fstype!~\"tmpfs|overlay\"})) > {{threshold}}",
			DefaultConditionOperator:   ">",
			DefaultConditionValue:      92,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  300,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("host", "Linux", "disk"),
			Annotations:                alertBuiltinAnnotations("主机磁盘使用率过高", "实例 {{ $labels.instance }} 挂载点 {{ $labels.mountpoint }} 使用率超过 {{threshold}}%。"),
			Variables:                  alertThresholdVariable(92, "%"),
			Sort:                       90,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "host",
			Component:                  "Linux",
			Collector:                  "node_exporter",
			Category:                   "availability",
			Name:                       "主机 Exporter 离线",
			Description:                "检测 Prometheus 抓取目标不可用",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.host.exporter.down",
			QueryTemplate:              "up{job=~\"node.*|node_exporter\"} == 0",
			DefaultConditionOperator:   "==",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  180,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("host", "Linux", "availability"),
			Annotations:                alertBuiltinAnnotations("主机 Exporter 离线", "实例 {{ $labels.instance }} Exporter 无法抓取。"),
			Sort:                       85,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "k8s",
			Component:                  "Kubernetes",
			Collector:                  "kube-state-metrics",
			Category:                   "node",
			Name:                       "K8S Node NotReady",
			Description:                "参考 n9e Kubernetes 内置规则，检测节点 Ready 状态异常",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.k8s.node.notready",
			QueryTemplate:              "kube_node_status_condition{job=\"kube-state-metrics\",condition=\"Ready\",status=\"true\"} == 0",
			DefaultConditionOperator:   "==",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  300,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("k8s", "Kubernetes", "node"),
			Annotations:                alertBuiltinAnnotations("K8S Node NotReady", "节点 {{ $labels.node }} 处于 NotReady 状态。"),
			Sort:                       100,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "k8s",
			Component:                  "Kubernetes",
			Collector:                  "kube-state-metrics",
			Category:                   "deployment",
			Name:                       "Deployment 副本异常",
			Description:                "参考 n9e Kubernetes k8s-workloads 规则，检测 Deployment 可用副本不足",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.k8s.deployment.replicas.mismatch",
			QueryTemplate:              "kube_deployment_spec_replicas{job=\"kube-state-metrics\"} != kube_deployment_status_replicas_available{job=\"kube-state-metrics\"}",
			DefaultConditionOperator:   "!=",
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  600,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("k8s", "Kubernetes", "deployment"),
			Annotations:                alertBuiltinAnnotations("Deployment 副本异常", "命名空间 {{ $labels.namespace }} 的 Deployment {{ $labels.deployment }} 可用副本与期望副本不一致。"),
			Sort:                       95,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "k8s",
			Component:                  "Kubernetes",
			Collector:                  "kube-state-metrics",
			Category:                   "pod",
			Name:                       "Pod 失败",
			Description:                "检测 Pod phase 为 Failed",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.k8s.pod.failed",
			QueryTemplate:              "kube_pod_status_phase{job=\"kube-state-metrics\",phase=\"Failed\"} > 0",
			DefaultConditionOperator:   ">",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  300,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("k8s", "Kubernetes", "pod"),
			Annotations:                alertBuiltinAnnotations("Pod 失败", "命名空间 {{ $labels.namespace }} 的 Pod {{ $labels.pod }} 处于 Failed 状态。"),
			Sort:                       90,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "mysql",
			Component:                  "MySQL",
			Collector:                  "mysqld_exporter",
			Category:                   "connection",
			Name:                       "MySQL 运行连接占比过高",
			Description:                "参考 n9e MySQL exporter 规则，检测 Threads_running 占最大连接数比例",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.mysql.threads.running.high",
			QueryTemplate:              "avg by (instance) (mysql_global_status_threads_running) / avg by (instance) (mysql_global_variables_max_connections) * 100 > {{threshold}}",
			DefaultConditionOperator:   ">",
			DefaultConditionValue:      60,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  120,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("mysql", "MySQL", "connection"),
			Annotations:                alertBuiltinAnnotations("MySQL 运行连接占比过高", "实例 {{ $labels.instance }} 运行中连接占比超过 {{threshold}}%。"),
			Variables:                  alertThresholdVariable(60, "%"),
			Sort:                       100,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "mysql",
			Component:                  "MySQL",
			Collector:                  "mysqld_exporter",
			Category:                   "file",
			Name:                       "MySQL 打开文件数占比过高",
			Description:                "参考 n9e MySQL exporter 规则，检测打开文件数接近限制",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.mysql.open.files.high",
			QueryTemplate:              "avg by (instance) (mysql_global_status_innodb_num_open_files) / avg by (instance)(mysql_global_variables_open_files_limit) * 100 > {{threshold}}",
			DefaultConditionOperator:   ">",
			DefaultConditionValue:      80,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  120,
			DefaultSeverity:            "warning",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("mysql", "MySQL", "file"),
			Annotations:                alertBuiltinAnnotations("MySQL 打开文件数占比过高", "实例 {{ $labels.instance }} 打开文件数占比超过 {{threshold}}%。"),
			Variables:                  alertThresholdVariable(80, "%"),
			Sort:                       95,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "mysql",
			Component:                  "MySQL",
			Collector:                  "mysqld_exporter",
			Category:                   "replication",
			Name:                       "MySQL 主从复制中断",
			Description:                "检测 MySQL Slave IO 或 SQL 线程异常",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.mysql.replication.down",
			QueryTemplate:              "mysql_slave_status_slave_io_running == 0 or mysql_slave_status_slave_sql_running == 0",
			DefaultConditionOperator:   "==",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  120,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("mysql", "MySQL", "replication"),
			Annotations:                alertBuiltinAnnotations("MySQL 主从复制中断", "实例 {{ $labels.instance }} 主从复制线程异常。"),
			Sort:                       90,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "prometheus",
			Component:                  "Prometheus",
			Collector:                  "prometheus",
			Category:                   "target",
			Name:                       "Prometheus Target Down",
			Description:                "检测 Prometheus 抓取目标不可用",
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.prometheus.target.down",
			QueryTemplate:              "up == 0",
			DefaultConditionOperator:   "==",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  180,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("prometheus", "Prometheus", "target"),
			Annotations:                alertBuiltinAnnotations("Prometheus Target Down", "抓取目标 {{ $labels.job }}/{{ $labels.instance }} 不可用。"),
			Sort:                       100,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
		{
			ResourceType:               "victoriametrics",
			Component:                  "VictoriaMetrics",
			Collector:                  "victoriametrics",
			Category:                   "availability",
			Name:                       "VictoriaMetrics 实例不可用",
			Description:                "检测 VictoriaMetrics 自身采集目标不可用",
			DatasourceType:             "victoriametrics",
			Source:                     "platform",
			SourceUUID:                 "platform.victoriametrics.down",
			QueryTemplate:              "up{job=~\"victoriametrics|vm.*\"} == 0",
			DefaultConditionOperator:   "==",
			DefaultConditionValue:      0,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  180,
			DefaultSeverity:            "critical",
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("victoriametrics", "VictoriaMetrics", "availability"),
			Annotations:                alertBuiltinAnnotations("VictoriaMetrics 实例不可用", "VictoriaMetrics 目标 {{ $labels.instance }} 不可用。"),
			Sort:                       100,
			Enabled:                    1,
			CreatedAt:                  now,
			UpdatedAt:                  now,
		},
	}
	defaults = append(defaults, defaultAgentAlertBuiltinRuleTemplates(now)...)

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	for _, item := range defaults {
		var count int64
		if err := session.Model(&alertmanagermodel.AlertBuiltinRuleTemplate{}).
			Where("source = ? AND source_uuid = ?", item.Source, item.SourceUUID).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := session.Create(&item).Error; err != nil {
			return err
		}
		log.Printf("AutoMigrate created alert builtin rule template: %s", item.Name)
	}

	return nil
}

func ensureDefaultAlertBuiltinRuleTemplateGroups(db *gorm.DB) error {
	if !db.Migrator().HasTable("alert_builtin_rule_template_groups") {
		return nil
	}
	if !db.Migrator().HasTable("alert_builtin_rule_templates") {
		return nil
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	var templates []alertmanagermodel.AlertBuiltinRuleTemplate
	if err := session.
		Select("component, collector, category, resource_type").
		Where("component <> '' AND collector <> '' AND category <> ''").
		Find(&templates).Error; err != nil {
		return err
	}

	now := util.HTime{Time: time.Now()}
	for _, template := range templates {
		component := strings.TrimSpace(template.Component)
		collector := strings.TrimSpace(template.Collector)
		category := strings.TrimSpace(template.Category)
		if component == "" || collector == "" || category == "" {
			continue
		}

		componentID, err := ensureAlertBuiltinRuleTemplateGroup(session, alertmanagermodel.AlertBuiltinRuleTemplateGroup{
			ParentID:     0,
			Level:        alertmanagermodel.TemplateGroupLevelComponent,
			Name:         component,
			Value:        component,
			ResourceType: template.ResourceType,
			Builtin:      1,
			Enabled:      1,
			CreatedBy:    "system",
			UpdatedBy:    "system",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if err != nil {
			return err
		}

		collectorID, err := ensureAlertBuiltinRuleTemplateGroup(session, alertmanagermodel.AlertBuiltinRuleTemplateGroup{
			ParentID:     componentID,
			Level:        alertmanagermodel.TemplateGroupLevelCollector,
			Name:         alertBuiltinCollectorDisplayName(collector),
			Value:        collector,
			ResourceType: template.ResourceType,
			Builtin:      1,
			Enabled:      1,
			CreatedBy:    "system",
			UpdatedBy:    "system",
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if err != nil {
			return err
		}

		if _, err := ensureAlertBuiltinRuleTemplateGroup(session, alertmanagermodel.AlertBuiltinRuleTemplateGroup{
			ParentID:     collectorID,
			Level:        alertmanagermodel.TemplateGroupLevelCategory,
			Name:         alertBuiltinCategoryDisplayName(collector, category),
			Value:        category,
			ResourceType: template.ResourceType,
			Builtin:      1,
			Enabled:      1,
			CreatedBy:    "system",
			UpdatedBy:    "system",
			CreatedAt:    now,
			UpdatedAt:    now,
		}); err != nil {
			return err
		}
	}

	return nil
}

func ensureDefaultAlertNotifyTemplates(db *gorm.DB) error {
	if !db.Migrator().HasTable("alert_notify_templates") {
		return nil
	}

	now := util.HTime{Time: time.Now()}
	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	for _, item := range alertmanagermodel.DefaultAlertNotifyTemplates() {
		item.CreatedAt = now
		item.UpdatedAt = now
		var existing alertmanagermodel.AlertNotifyTemplate
		err := session.Model(&alertmanagermodel.AlertNotifyTemplate{}).
			Where("ident = ?", item.Ident).
			First(&existing).Error
		if err == nil {
			updates := map[string]interface{}{
				"name":          item.Name,
				"channel_type":  item.ChannelType,
				"template_type": item.TemplateType,
				"description":   item.Description,
				"content":       item.Content,
				"builtin":       1,
				"enabled":       item.Enabled,
				"updated_by":    "system",
				"updated_at":    now,
			}
			if err := session.Model(&alertmanagermodel.AlertNotifyTemplate{}).
				Where("id = ?", existing.ID).
				Updates(updates).Error; err != nil {
				return err
			}
			continue
		}
		if err != gorm.ErrRecordNotFound {
			return err
		}
		if err := session.Create(&item).Error; err != nil {
			return err
		}
		log.Printf("AutoMigrate created alert notify template: %s", item.Name)
	}

	return nil
}

func removeLegacyAlertNotifyRouteColumns(db *gorm.DB) error {
	const table = "alert_notify_route_rules"
	if !db.Migrator().HasTable(table) {
		return nil
	}
	for _, column := range []string{"label_key", "label_keys", "match_mode", "match_values"} {
		if !db.Migrator().HasColumn(table, column) {
			continue
		}
		log.Printf("AutoMigrate dropping obsolete column: %s.%s", table, column)
		if err := db.Migrator().DropColumn(table, column); err != nil {
			return err
		}
	}
	return nil
}

func ensureAlertBuiltinRuleTemplateGroup(db *gorm.DB, group alertmanagermodel.AlertBuiltinRuleTemplateGroup) (uint, error) {
	var existing alertmanagermodel.AlertBuiltinRuleTemplateGroup
	err := db.
		Where("parent_id = ? AND level = ? AND value = ?", group.ParentID, group.Level, group.Value).
		Take(&existing).Error
	if err == nil {
		updates := map[string]interface{}{}
		if group.Name != "" && existing.Name != group.Name {
			updates["name"] = group.Name
		}
		if existing.Builtin == 0 {
			updates["builtin"] = 1
		}
		if existing.ResourceType == "" && group.ResourceType != "" {
			updates["resource_type"] = group.ResourceType
		}
		if existing.Enabled == 0 {
			updates["enabled"] = 1
		}
		if len(updates) > 0 {
			updates["updated_at"] = group.UpdatedAt
			if updateErr := db.Model(&existing).Updates(updates).Error; updateErr != nil {
				return 0, updateErr
			}
		}
		return existing.ID, nil
	}
	if err != gorm.ErrRecordNotFound {
		return 0, err
	}
	if err := db.Create(&group).Error; err != nil {
		return 0, err
	}
	log.Printf("AutoMigrate created alert builtin rule template group: %s/%s", group.Level, group.Value)
	return group.ID, nil
}

func alertBuiltinLabels(resourceType, component, category string) alertmanagermodel.JSONMap {
	return alertmanagermodel.JSONMap{
		"resourceType": resourceType,
		"component":    component,
		"category":     category,
	}
}

func alertBuiltinAnnotations(summary, description string) alertmanagermodel.JSONMap {
	return alertmanagermodel.JSONMap{
		"summary":     summary,
		"description": description,
	}
}

func alertThresholdVariable(defaultValue float64, unit string) alertmanagermodel.JSONMap {
	return alertmanagermodel.JSONMap{
		"threshold": map[string]interface{}{
			"type":    "number",
			"default": defaultValue,
			"unit":    unit,
		},
	}
}

func alertRuleConfig(queries []map[string]interface{}) alertmanagermodel.JSONMap {
	return alertmanagermodel.JSONMap{
		"inhibit": false,
		"queries": queries,
	}
}

func ensureMySQLIndex(db *gorm.DB, tableName, indexName, columns string) error {
	var count int64
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Raw(`
SELECT COUNT(1)
FROM information_schema.statistics
WHERE table_schema = DATABASE()
  AND table_name = ?
  AND index_name = ?`, tableName, indexName).
		Scan(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	log.Printf("AutoMigrate creating index: %s.%s", tableName, indexName)
	sql := fmt.Sprintf("CREATE INDEX `%s` ON `%s` (%s)", indexName, tableName, columns)
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(sql).Error
}

func ensureCostCenterBillDetailIndexes(db *gorm.DB) error {
	if !db.Migrator().HasTable("bill_detail") ||
		!db.Migrator().HasColumn(&costcentermodel.BillDetail{}, "CloudAccountID") ||
		!db.Migrator().HasColumn(&costcentermodel.BillDetail{}, "Month") ||
		!db.Migrator().HasColumn(&costcentermodel.BillDetail{}, "RecordID") {
		return nil
	}

	const desiredIndex = "uk_bill_detail_account_month_record"
	if !db.Migrator().HasIndex(&costcentermodel.BillDetail{}, desiredIndex) {
		if err := db.Migrator().CreateIndex(&costcentermodel.BillDetail{}, desiredIndex); err != nil {
			return fmt.Errorf("create cost center bill detail index %s: %w", desiredIndex, err)
		}
	}

	// These indexes came from the legacy billing schema and make record IDs
	// collide across months or accounts. Keep only the provider-neutral key.
	for _, indexName := range []string{"uk_bill_detail_record", "uk_operator_record", "uk_operator_record_name"} {
		if !db.Migrator().HasIndex(&costcentermodel.BillDetail{}, indexName) {
			continue
		}
		if err := db.Migrator().DropIndex(&costcentermodel.BillDetail{}, indexName); err != nil {
			return fmt.Errorf("drop legacy cost center bill detail index %s: %w", indexName, err)
		}
	}
	return nil
}

func ensureBastionAuthorizationLifecycleMigration(db *gorm.DB) error {
	if !db.Migrator().HasTable("bastion_host_authorization") {
		return nil
	}
	if !db.Migrator().HasColumn(&cmdbmodel.BastionHostAuthorization{}, "EffectiveTime") {
		return nil
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})

	if err := session.Exec(`
UPDATE bastion_host_authorization
SET effective_time = create_time
WHERE effective_time IS NULL`).Error; err != nil {
		return err
	}

	if db.Migrator().HasColumn(&cmdbmodel.BastionHostAuthorization{}, "UpdateTime") {
		if err := session.Exec(`
UPDATE bastion_host_authorization
SET update_time = create_time
WHERE update_time IS NULL`).Error; err != nil {
			return err
		}
	}

	return nil
}

func ensureCmdbGroupLifecycleMigration(db *gorm.DB) error {
	if !db.Migrator().HasTable("cmdb_group") {
		return nil
	}
	if !db.Migrator().HasColumn(&cmdbmodel.CmdbGroup{}, "UpdateTime") {
		return nil
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	return session.Exec(`
UPDATE cmdb_group
SET update_time = create_time
WHERE update_time IS NULL`).Error
}

func ensureSecretMigrations(db *gorm.DB) error {
	if !db.Migrator().HasTable("cmdb_snmp_devices") {
		return nil
	}
	if !db.Migrator().HasColumn(&cmdbmodel.CmdbSnmpDevice{}, "RemotePassword") {
		return nil
	}
	if err := ensureSnmpRemotePasswordColumnType(db); err != nil {
		return err
	}

	type snmpDeviceSecret struct {
		ID             uint   `gorm:"column:id"`
		RemotePassword string `gorm:"column:remote_password"`
	}

	var devices []snmpDeviceSecret
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Table("cmdb_snmp_devices").
		Select("id, remote_password").
		Where("remote_password IS NOT NULL AND remote_password <> ''").
		Find(&devices).Error
	if err != nil {
		return err
	}

	migrated := 0
	for _, device := range devices {
		encrypted, err := cmdbmodel.EncryptSnmpSecretIfNeeded(device.RemotePassword)
		if err != nil {
			return err
		}
		if encrypted == device.RemotePassword {
			continue
		}
		if err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
			Table("cmdb_snmp_devices").
			Where("id = ?", device.ID).
			Update("remote_password", encrypted).Error; err != nil {
			return err
		}
		migrated++
	}

	if migrated > 0 {
		log.Printf("AutoMigrate encrypted %d cmdb_snmp_devices remote passwords", migrated)
	}
	return nil
}

func ensureSnmpRemotePasswordColumnType(db *gorm.DB) error {
	var dataType string
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Raw(`
SELECT data_type
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = 'cmdb_snmp_devices'
  AND column_name = 'remote_password'`).
		Row().
		Scan(&dataType)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(dataType), "text") {
		return nil
	}
	return db.Migrator().AlterColumn(&cmdbmodel.CmdbSnmpDevice{}, "RemotePassword")
}

func ensureK8sEventsMenu(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_menu") || !db.Migrator().HasTable("sys_role_menu") {
		return nil
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})

	type menuRow struct {
		ID       uint `gorm:"column:id"`
		ParentID uint `gorm:"column:parent_id"`
		Sort     uint `gorm:"column:sort"`
	}

	var existing menuRow
	err := session.Table("sys_menu").
		Select("id").
		Where("menu_name = ? OR url = ?", "事件管理", "k8s/events").
		Take(&existing).Error
	if err == nil && existing.ID > 0 {
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	var containerMenu menuRow
	err = session.Table("sys_menu").
		Select("id").
		Where("menu_name = ?", "容器管理").
		Take(&containerMenu).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	var maxSort uint
	if err := session.Table("sys_menu").
		Select("COALESCE(MAX(sort), 0)").
		Where("parent_id = ?", containerMenu.ID).
		Scan(&maxSort).Error; err != nil {
		return err
	}

	eventMenu := systemmodel.SysMenu{
		ParentId:   containerMenu.ID,
		MenuName:   "事件管理",
		Icon:       "Bell",
		Value:      "k8s:events:list",
		MenuType:   2,
		Url:        "k8s/events",
		MenuStatus: 2,
		Sort:       maxSort + 1,
		CreateTime: util.HTime{Time: time.Now()},
	}

	if err := session.Create(&eventMenu).Error; err != nil {
		return err
	}

	if eventMenu.ID == 0 {
		return nil
	}

	type roleIDRow struct {
		RoleID uint `gorm:"column:role_id"`
	}

	var roleRows []roleIDRow
	if err := session.Table("sys_role_menu").
		Select("DISTINCT role_id").
		Where("menu_id = ?", containerMenu.ID).
		Find(&roleRows).Error; err != nil {
		return err
	}

	if len(roleRows) == 0 {
		if err := session.Table("sys_role").
			Select("id AS role_id").
			Where("role_key IN ? OR role_name IN ?", []string{"admin", "administrator", "superadmin"}, []string{"管理员", "超级管理员"}).
			Find(&roleRows).Error; err != nil {
			return err
		}
	}

	if len(roleRows) == 0 {
		log.Printf("AutoMigrate created K8s events menu without role binding: menu_id=%d", eventMenu.ID)
		return nil
	}

	roleMenus := make([]systemmodel.SysRoleMenu, 0, len(roleRows))
	for _, role := range roleRows {
		if role.RoleID == 0 {
			continue
		}
		roleMenus = append(roleMenus, systemmodel.SysRoleMenu{
			RoleId: role.RoleID,
			MenuId: eventMenu.ID,
		})
	}

	if len(roleMenus) == 0 {
		return nil
	}

	if err := session.CreateInBatches(roleMenus, 100).Error; err != nil {
		return err
	}

	log.Printf("AutoMigrate created K8s events menu: menu_id=%d roles=%d", eventMenu.ID, len(roleMenus))
	return nil
}

func ensureK8sMenuIcons(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_menu") {
		return nil
	}

	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})

	type menuRow struct {
		ID uint `gorm:"column:id"`
	}

	var containerMenu menuRow
	err := session.Table("sys_menu").
		Select("id").
		Where("menu_name = ?", "容器管理").
		Take(&containerMenu).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	menuIcons := []struct {
		name string
		url  string
		icon string
	}{
		{name: "集群管理", url: "k8s/list", icon: "Platform"},
		{name: "节点管理", url: "k8s/node", icon: "Monitor"},
		{name: "命名空间", url: "k8s/namespace", icon: "Collection"},
		{name: "工作负载", url: "k8s/workload", icon: "Operation"},
		{name: "事件管理", url: "k8s/events", icon: "Bell"},
		{name: "网络管理", url: "k8s/network", icon: "Connection"},
		{name: "配置管理", url: "k8s/config", icon: "Setting"},
		{name: "存储管理", url: "k8s/storage", icon: "FolderOpened"},
		{name: "监控仪表板", url: "k8s/monitoring", icon: "DataAnalysis"},
		{name: "应用诊断", url: "k8s/arthas", icon: "FirstAidKit"},
		{name: "集群巡检", url: "k8s/inspection", icon: "Finished"},
	}

	updated := int64(0)
	for _, item := range menuIcons {
		tx := session.Table("sys_menu").
			Where("parent_id = ?", containerMenu.ID).
			Where("menu_name = ? OR url = ?", item.name, item.url).
			Update("icon", item.icon)
		if tx.Error != nil {
			return tx.Error
		}
		updated += tx.RowsAffected
	}

	if updated > 0 {
		log.Printf("AutoMigrate updated K8s menu icons: rows=%d", updated)
	}
	return nil
}

func ensureDomainMonitorMenu(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_menu") || !db.Migrator().HasTable("sys_role_menu") {
		return nil
	}
	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	var existing int64
	if err := session.Table("sys_menu").Where("url = ?", "alert/domain").Count(&existing).Error; err != nil || existing > 0 {
		return err
	}
	type menuRow struct {
		ID uint `gorm:"column:id"`
	}
	var parent menuRow
	if err := session.Table("sys_menu").Select("id").Where("menu_name = ?", "告警中心").Take(&parent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	var maxSort uint
	if err := session.Table("sys_menu").Select("COALESCE(MAX(sort), 0)").Where("parent_id = ?", parent.ID).Scan(&maxSort).Error; err != nil {
		return err
	}
	item := systemmodel.SysMenu{
		ParentId: parent.ID, MenuName: "域名告警", Icon: "Link", Value: "alert:domain:list",
		MenuType: 2, Url: "alert/domain", MenuStatus: 2, Sort: maxSort + 1,
		CreateTime: util.HTime{Time: time.Now()},
	}
	var roleMenus []systemmodel.SysRoleMenu
	if err := session.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		var roleIDs []uint
		if err := tx.Table("sys_role_menu").Distinct("role_id").Where("menu_id = ?", parent.ID).Pluck("role_id", &roleIDs).Error; err != nil {
			return err
		}
		roleMenus = make([]systemmodel.SysRoleMenu, 0, len(roleIDs))
		for _, roleID := range roleIDs {
			roleMenus = append(roleMenus, systemmodel.SysRoleMenu{RoleId: roleID, MenuId: item.ID})
		}
		if len(roleMenus) > 0 {
			return tx.CreateInBatches(roleMenus, 100).Error
		}
		return nil
	}); err != nil {
		return err
	}
	log.Printf("AutoMigrate created domain alert menu: menu_id=%d roles=%d", item.ID, len(roleMenus))
	return nil
}

func ensureLicenseManagementMenu(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_menu") || !db.Migrator().HasTable("sys_role_menu") {
		return nil
	}
	session := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	type menuRow struct {
		ID uint `gorm:"column:id"`
	}
	var parent menuRow
	if err := session.Table("sys_menu").Select("id").Where("menu_name = ?", "系统管理").Take(&parent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	var item systemmodel.SysMenu
	err := session.Where("url = ?", "system/license").First(&item).Error
	created := false
	if err == gorm.ErrRecordNotFound {
		var maxSort uint
		if err := session.Table("sys_menu").Select("COALESCE(MAX(sort), 0)").Where("parent_id = ?", parent.ID).Scan(&maxSort).Error; err != nil {
			return err
		}
		item = systemmodel.SysMenu{
			ParentId: parent.ID, MenuName: "授权管理", Icon: "Key", Value: "system:license:view",
			MenuType: 2, Url: "system/license", MenuStatus: 2, Sort: maxSort + 1,
			CreateTime: util.HTime{Time: time.Now()},
		}
		if err := session.Create(&item).Error; err != nil {
			return err
		}
		created = true
	} else if err != nil {
		return err
	}

	var roleIDs []uint
	if err := session.Table("sys_role_menu").Distinct("role_id").Where("menu_id = ?", parent.ID).Pluck("role_id", &roleIDs).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		if err := session.Table("sys_role").Where("role_key IN ? OR role_name IN ?", []string{"admin", "administrator", "superadmin"}, []string{"管理员", "超级管理员"}).Pluck("id", &roleIDs).Error; err != nil {
			return err
		}
	}
	bound := 0
	for _, roleID := range roleIDs {
		if roleID == 0 {
			continue
		}
		var count int64
		if err := session.Table("sys_role_menu").Where("role_id = ? AND menu_id = ?", roleID, item.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			if err := session.Create(&systemmodel.SysRoleMenu{RoleId: roleID, MenuId: item.ID}).Error; err != nil {
				return err
			}
			bound++
		}
	}
	if created || bound > 0 {
		log.Printf("AutoMigrate ensured license management menu: menu_id=%d new_bindings=%d", item.ID, bound)
	}
	return nil
}

func ensureMCPManagementMenu(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_menu") || !db.Migrator().HasTable("sys_role_menu") || !db.Migrator().HasTable("sys_role") {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		session := tx.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
		var root systemmodel.SysMenu
		err := session.Where("menu_type = ? AND menu_name IN ?", 1, []string{"智能运维", "AIOps", "AI运维"}).
			Order("id ASC").Take(&root).Error
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		if err != nil {
			return err
		}

		var menu systemmodel.SysMenu
		err = session.Where("url = ? OR value = ?", "/ai/mcp-management", "aiops:mcp:view").Order("id ASC").Take(&menu).Error
		if err == gorm.ErrRecordNotFound {
			menu = systemmodel.SysMenu{
				ParentId: root.ID, MenuName: "MCP 管理", Icon: "Tools",
				Value: "aiops:mcp:view", MenuType: 2, Url: "/ai/mcp-management",
				MenuStatus: 2, Sort: 90, CreateTime: util.HTime{Time: time.Now()},
			}
			if err := session.Create(&menu).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := session.Model(&systemmodel.SysMenu{}).Where("id = ?", menu.ID).Updates(map[string]interface{}{
			"parent_id": root.ID, "menu_name": "MCP 管理", "icon": "Tools",
			"value": "aiops:mcp:view", "menu_type": 2, "url": "/ai/mcp-management",
			"menu_status": 2, "sort": 90,
		}).Error; err != nil {
			return err
		}

		var permission systemmodel.SysMenu
		err = session.Where("value = ?", "aiops:mcp:manage").Order("id ASC").Take(&permission).Error
		if err == gorm.ErrRecordNotFound {
			permission = systemmodel.SysMenu{
				ParentId: menu.ID, MenuName: "管理 MCP 配置", Value: "aiops:mcp:manage",
				MenuType: 3, MenuStatus: 2, Sort: 1, CreateTime: util.HTime{Time: time.Now()},
			}
			if err := session.Create(&permission).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if err := session.Model(&systemmodel.SysMenu{}).Where("id = ?", permission.ID).Updates(map[string]interface{}{
			"parent_id": menu.ID, "menu_name": "管理 MCP 配置", "value": "aiops:mcp:manage",
			"menu_type": 3, "menu_status": 2, "sort": 1,
		}).Error; err != nil {
			return err
		}

		var roleIDs []uint
		if err := session.Table("sys_role").
			Where("status = ? AND (role_key IN ? OR role_name IN ?)", 1, []string{"admin", "administrator", "superadmin"}, []string{"管理员", "超级管理员"}).
			Pluck("id", &roleIDs).Error; err != nil {
			return err
		}
		bound := 0
		for _, roleID := range roleIDs {
			if roleID == 0 {
				continue
			}
			for _, menuID := range []uint{menu.ID, permission.ID} {
				var count int64
				if err := session.Table("sys_role_menu").Where("role_id = ? AND menu_id = ?", roleID, menuID).Count(&count).Error; err != nil {
					return err
				}
				if count == 0 {
					if err := session.Create(&systemmodel.SysRoleMenu{RoleId: roleID, MenuId: menuID}).Error; err != nil {
						return err
					}
					bound++
				}
			}
		}
		log.Printf("AutoMigrate ensured AIOps MCP management menu: menu_id=%d permission_id=%d new_bindings=%d", menu.ID, permission.ID, bound)
		return nil
	})
}

func ensureDefaultNavCategories(db *gorm.DB) error {
	if !db.Migrator().HasTable("sys_nav_category") {
		return nil
	}

	defaultCategories := []systemmodel.SysNavCategory{
		{
			Name:        "公共服务",
			Code:        "public_service",
			Icon:        "Grid",
			Description: "公司内部公共系统、项目系统和运维平台入口",
			Sort:        10,
			Status:      1,
			CreateTime:  util.HTime{Time: time.Now()},
			UpdateTime:  util.HTime{Time: time.Now()},
		},
		{
			Name:        "个人工具",
			Code:        "personal_tools",
			Icon:        "Star",
			Description: "个人常用工具网站和个人链接",
			Sort:        20,
			Status:      1,
			CreateTime:  util.HTime{Time: time.Now()},
			UpdateTime:  util.HTime{Time: time.Now()},
		},
	}

	for _, category := range defaultCategories {
		var count int64
		if err := db.Table("sys_nav_category").
			Where("code = ?", category.Code).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Create(&category).Error; err != nil {
			return err
		}
		log.Printf("AutoMigrate created default nav category: %s", category.Name)
	}

	return nil
}

func calculateModelSignature() (string, error) {
	parts := make([]string, 0, len(models))

	for _, m := range models {
		t := reflect.TypeOf(m)
		if t == nil {
			continue
		}
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return "", fmt.Errorf("unsupported model type: %T", m)
		}

		fieldParts := make([]string, 0, t.NumField())
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fieldParts = append(fieldParts, strings.Join([]string{
				f.Name,
				f.Type.String(),
				string(f.Tag),
			}, "|"))
		}

		tableName := t.PkgPath() + "." + t.Name()
		if namer, ok := reflect.New(t).Interface().(interface{ TableName() string }); ok {
			tableName = namer.TableName()
		}

		parts = append(parts, tableName+"::"+strings.Join(fieldParts, "||"))
	}

	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func ensureMigrationMetaTable(db *gorm.DB) error {
	sql := `
CREATE TABLE IF NOT EXISTS system_migration_meta (
  meta_key VARCHAR(128) PRIMARY KEY,
  meta_value VARCHAR(128) NOT NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
)`
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(sql).Error
}

func getStoredMigrationSignature(db *gorm.DB, key string) (string, error) {
	type migrationMeta struct {
		MetaValue string `gorm:"column:meta_value"`
	}

	var meta migrationMeta
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Table("system_migration_meta").
		Select("meta_value").
		Where("meta_key = ?", key).
		Take(&meta).Error
	if err == gorm.ErrRecordNotFound {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return meta.MetaValue, nil
}

func setStoredMigrationSignature(db *gorm.DB, key, value string) error {
	sql := `
INSERT INTO system_migration_meta (meta_key, meta_value)
VALUES (?, ?)
ON DUPLICATE KEY UPDATE
  meta_value = VALUES(meta_value),
  updated_at = CURRENT_TIMESTAMP`

	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Exec(sql, key, value).Error
}

func countUserTables(db *gorm.DB) (int64, error) {
	type tableCount struct {
		Count int64 `gorm:"column:cnt"`
	}

	var result tableCount
	err := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).
		Raw(`
SELECT COUNT(*) AS cnt
FROM information_schema.tables
WHERE table_schema = DATABASE()
  AND table_type = 'BASE TABLE'
  AND table_name <> 'system_migration_meta'`).
		Scan(&result).Error
	if err != nil {
		return 0, err
	}
	return result.Count, nil
}

func shortSignature(signature string) string {
	if signature == "" {
		return "none"
	}
	if len(signature) <= 12 {
		return signature
	}
	return signature[:12]
}
