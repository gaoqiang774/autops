package service

import (
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbdao "dodevops-api/api/db/dao"
	dbmodel "dodevops-api/api/db/model"
	systemmodel "dodevops-api/api/system/model"
	ticketmodel "dodevops-api/api/ticket/model"
	ticketservice "dodevops-api/api/ticket/service"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	queryReviewDefaultLimit = 1000
	queryResultPreviewRows  = 100
	queryResultDownloadRows = 1000
	ghOstBinaryPath         = "/opt/db/gh-ost"
	defaultMy2SQLBinaryPath = "/opt/db/my2sql"
	pgRollBinaryPath        = "/opt/db/pgroll"
	walMinerBinaryPath      = "/opt/db/walminer"
	my2SQLDownloadURL       = "https://github.com/liuhr/my2sql/releases"
	ghOstDownloadURL        = "https://github.com/github/gh-ost/releases"
	pgRollDownloadURL       = "https://github.com/xataio/pgroll/releases"
	walMinerDownloadURL     = "https://github.com/HighgoSoftware/WalMiner"
)

type ReviewService struct {
	dbService *DatabaseService
	reviewDao *dbdao.ReviewDao
}

func NewReviewService() *ReviewService {
	return &ReviewService{
		dbService: NewDatabaseService(),
		reviewDao: dbdao.NewReviewDao(),
	}
}

func (s *ReviewService) Check(ctx *gin.Context, ticketType string, req dbmodel.SQLReviewCheckRequest) (*dbmodel.SQLReviewCheckResult, error) {
	ticketType = normalizeTicketType(ticketType)
	if ticketType == "" {
		return nil, fmt.Errorf("invalid ticket type")
	}
	if req.AssetID == 0 {
		return nil, fmt.Errorf("assetId is required")
	}
	if strings.TrimSpace(req.SQL) == "" {
		return nil, fmt.Errorf("sql is required")
	}
	asset, cfg, err := s.dbService.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	check := reviewSQL(ticketType, req.Database, req.Schema, req.SQL, req.BackupRequired, asset.Type)
	if ticketType == dbmodel.SQLTicketTypeDML {
		s.fillDMLCheckExtras(ctx, req, check, asset.Type, cfg)
	}
	return check, nil
}

func (s *ReviewService) Submit(ctx *gin.Context, ticketType string, req dbmodel.SQLReviewSubmitRequest) (*dbmodel.SQLTicketDetail, error) {
	ticketType = normalizeTicketType(ticketType)
	if ticketType == "" {
		return nil, fmt.Errorf("invalid ticket type")
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("title is required")
	}
	if req.WorkflowID == 0 && req.ApproverID == 0 {
		return nil, fmt.Errorf("approverId is required")
	}
	if req.ExecutorID == 0 {
		return nil, fmt.Errorf("executorId is required")
	}
	var approverID uint
	var approverName string
	var err error
	if req.WorkflowID == 0 {
		approverID, approverName, err = resolveReviewUser(req.ApproverID, "approver")
		if err != nil {
			return nil, err
		}
	}
	executorID, executorName, err := resolveReviewUser(req.ExecutorID, "executor")
	if err != nil {
		return nil, err
	}

	asset, cfg, err := s.dbService.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	check := reviewSQL(ticketType, req.Database, req.Schema, req.SQL, req.BackupRequired, asset.Type)
	if !check.ReviewPassed {
		return nil, fmt.Errorf("sql review has error-level findings")
	}
	ghOstScript := ""
	if req.UseGhOst {
		if ticketType != dbmodel.SQLTicketTypeDDL {
			return nil, fmt.Errorf("gh-ost仅支持DDL工单")
		}
		if asset.Type != cmdbmodel.DatabaseTypeMySQL {
			return nil, fmt.Errorf("gh-ost仅支持MySQL DDL工单；PostgreSQL在线DDL请使用pgroll方案")
		}
		ghOstScript, err = buildGhOstScript(cfg, req.SQL)
		if err != nil {
			return nil, err
		}
	}
	sqlContent := strings.TrimSpace(req.SQL)
	if ticketType == dbmodel.SQLTicketTypeQuery {
		sqlContent = joinSQLStatements(check.Statements)
	}

	now := util.HTime{Time: time.Now()}
	submitterID, submitterName := currentAdmin(ctx)
	ticket := &dbmodel.SQLTicket{
		TicketNo:         buildTicketNo(ticketType),
		Title:            strings.TrimSpace(req.Title),
		TicketType:       ticketType,
		AssetID:          asset.ID,
		UseGhOst:         req.UseGhOst,
		GhOstScript:      ghOstScript,
		Engine:           asset.Type,
		DatabaseName:     strings.TrimSpace(req.Database),
		SchemaName:       strings.TrimSpace(req.Schema),
		Environment:      asset.Environment,
		SQLContent:       sqlContent,
		SQLFingerprint:   sqlFingerprint(sqlContent),
		Status:           dbmodel.SQLTicketStatusPendingReview,
		RiskLevel:        check.RiskLevel,
		BackupRequired:   req.BackupRequired,
		RollbackRequired: ticketType != dbmodel.SQLTicketTypeQuery,
		ReviewPassed:     check.ReviewPassed,
		Remark:           strings.TrimSpace(req.Remark),
		SubmitterID:      submitterID,
		SubmitterName:    submitterName,
		SubmittedAt:      now,
		ApproverID:       approverID,
		ApproverName:     approverName,
		ExecutorID:       executorID,
		ExecutorName:     executorName,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	rollbacks := buildInitialRollbackRecords(ticketType, check.Statements, req.RollbackSQL, now)
	if err := s.reviewDao.CreateTicket(ticket, check.Statements, check.ReviewResults, rollbacks); err != nil {
		return nil, err
	}
	if req.WorkflowID > 0 {
		instance, flowErr := ticketservice.NewWorkflowService(common.GetDB()).Start(ticketservice.WorkflowStartInput{
			DefinitionID: req.WorkflowID, BusinessType: ticketmodel.BusinessTypeSQLReview,
			BusinessID: ticket.ID, BusinessNo: ticket.TicketNo, Environment: ticket.Environment,
			ApplicantID: submitterID, ApplicantName: submitterName,
		})
		if flowErr != nil {
			s.cleanupFailedWorkflowTicket(ticket.ID)
			return nil, flowErr
		}
		ticket.WorkflowInstanceID = instance.ID
		if err := s.reviewDao.UpdateTicket(ticket.ID, map[string]interface{}{"workflow_instance_id": instance.ID}); err != nil {
			return nil, err
		}
	}
	return s.Detail(ticket.ID)
}

func (s *ReviewService) List(query dbmodel.SQLTicketQuery) ([]dbmodel.SQLTicket, int64, error) {
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	query.TicketType = normalizeTicketType(query.TicketType)
	list, total, err := s.reviewDao.ListTickets(query)
	if err != nil {
		return nil, 0, err
	}
	s.decorateTickets(list)
	return list, total, nil
}

func (s *ReviewService) cleanupFailedWorkflowTicket(ticketID uint) {
	_ = common.GetDB().Transaction(func(tx *gorm.DB) error {
		for _, record := range []interface{}{&dbmodel.SQLReviewResult{}, &dbmodel.SQLRollbackRecord{}, &dbmodel.SQLStatement{}, &dbmodel.SQLApprovalRecord{}, &dbmodel.SQLExecutionRecord{}} {
			if err := tx.Where("ticket_id = ?", ticketID).Delete(record).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&dbmodel.SQLTicket{}, ticketID).Error
	})
}

func (s *ReviewService) Detail(id uint) (*dbmodel.SQLTicketDetail, error) {
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	tickets := []dbmodel.SQLTicket{detail.Ticket}
	s.decorateTickets(tickets)
	detail.Ticket = tickets[0]
	fillTicketFlow(detail)
	return detail, nil
}

func (s *ReviewService) My2SQLStatus(ctx context.Context) dbmodel.DBToolStatus {
	return checkExternalDBTool(ctx, externalDBToolConfig{
		ToolName:           "my2sql",
		DatabaseType:       cmdbmodel.DatabaseTypeMySQL,
		DatabaseTypeName:   "MySQL",
		Capability:         "flashback_rollback",
		EnvName:            "AIOPS_MY2SQL_PATH",
		InstallPath:        defaultMy2SQLBinaryPath,
		UsageScenario:      "DML Binlog 闪回回滚 SQL 生成",
		DownloadURL:        my2SQLDownloadURL,
		InstallInstruction: buildMy2SQLInstallInstruction(),
		HelpArgs:           []string{"-help"},
		CapabilityOnOK:     true,
	})
}

func (s *ReviewService) GhOstStatus(ctx context.Context) dbmodel.DBToolStatus {
	return checkExternalDBTool(ctx, externalDBToolConfig{
		ToolName:           "gh-ost",
		DatabaseType:       cmdbmodel.DatabaseTypeMySQL,
		DatabaseTypeName:   "MySQL",
		Capability:         "online_ddl",
		EnvName:            "AIOPS_GHOST_PATH",
		InstallPath:        ghOstBinaryPath,
		UsageScenario:      "MySQL 在线 DDL 无锁变更执行",
		DownloadURL:        ghOstDownloadURL,
		InstallInstruction: buildGhOstInstallInstruction(),
		HelpArgs:           []string{"--help"},
		CapabilityOnOK:     true,
	})
}

func (s *ReviewService) PgRollStatus(ctx context.Context) dbmodel.DBToolStatus {
	return checkExternalDBTool(ctx, externalDBToolConfig{
		ToolName:           "pgroll",
		DatabaseType:       cmdbmodel.DatabaseTypePostgreSQL,
		DatabaseTypeName:   "PostgreSQL",
		Capability:         "online_ddl",
		EnvName:            "AIOPS_PGROLL_PATH",
		InstallPath:        pgRollBinaryPath,
		UsageScenario:      "PostgreSQL 在线 DDL 零停机迁移",
		DownloadURL:        pgRollDownloadURL,
		InstallInstruction: buildPgRollInstallInstruction(),
		HelpArgs:           []string{"--help"},
		CapabilityOnOK:     true,
	})
}

func (s *ReviewService) WalMinerStatus(ctx context.Context) dbmodel.DBToolStatus {
	return checkExternalDBTool(ctx, externalDBToolConfig{
		ToolName:           "WalMiner",
		DatabaseType:       cmdbmodel.DatabaseTypePostgreSQL,
		DatabaseTypeName:   "PostgreSQL",
		Capability:         "flashback_rollback",
		EnvName:            "AIOPS_WALMINER_PATH",
		InstallPath:        walMinerBinaryPath,
		UsageScenario:      "PostgreSQL WAL 解析生成 Undo SQL",
		DownloadURL:        walMinerDownloadURL,
		InstallInstruction: buildWalMinerInstallInstruction(),
		HelpArgs:           []string{"--help"},
		CapabilityOnOK:     true,
	})
}

func (s *ReviewService) ToolboxStatus(ctx context.Context, assetID uint, dbType int) (*dbmodel.DBToolboxStatus, error) {
	if dbType == 0 && assetID > 0 {
		asset, err := s.dbService.assetDao.GetByID(assetID)
		if err != nil {
			return nil, err
		}
		dbType = asset.Type
	}
	status := &dbmodel.DBToolboxStatus{
		DatabaseType:     dbType,
		DatabaseTypeName: databaseTypeName(dbType),
		Tools:            []dbmodel.DBToolStatus{},
	}
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		status.Tools = append(status.Tools, s.My2SQLStatus(ctx), s.GhOstStatus(ctx))
	case cmdbmodel.DatabaseTypePostgreSQL:
		status.Tools = append(status.Tools, s.WalMinerStatus(ctx), s.PgRollStatus(ctx))
	case 0:
		status.DatabaseTypeName = "全部"
		status.Tools = append(status.Tools,
			s.My2SQLStatus(ctx),
			s.GhOstStatus(ctx),
			s.WalMinerStatus(ctx),
			s.PgRollStatus(ctx),
		)
	default:
		return nil, fmt.Errorf("database type %d has no supported toolbox mapping", dbType)
	}
	return status, nil
}

func fillTicketFlow(detail *dbmodel.SQLTicketDetail) {
	if detail == nil {
		return
	}
	ticket := detail.Ticket
	submitStatus := "done"
	reviewStatus := "pending"
	executeStatus := "pending"
	reviewComment := ticket.ReviewComment
	executeComment := ticket.ExecuteComment
	reviewedAt := ticket.ReviewedAt
	executedAt := ticket.ExecutedAt

	if record, ok := latestApprovalRecord(detail.ApprovalRecords); ok {
		reviewComment = record.Comment
		reviewedAt = record.CreatedAt
		switch record.Action {
		case dbmodel.SQLApprovalActionApprove:
			reviewStatus = "done"
		case dbmodel.SQLApprovalActionReject:
			reviewStatus = "rejected"
		case dbmodel.SQLApprovalActionCancel:
			reviewStatus = "canceled"
		}
	}

	switch ticket.Status {
	case dbmodel.SQLTicketStatusPendingReview:
		reviewStatus = "current"
	case dbmodel.SQLTicketStatusRejected:
		detail.FlowEnded = true
		detail.FlowEndReason = "审批已驳回，流程结束"
		reviewStatus = "rejected"
		executeStatus = "skipped"
	case dbmodel.SQLTicketStatusCanceled:
		detail.FlowEnded = true
		detail.FlowEndReason = "工单已撤销，流程结束"
		if reviewStatus == "pending" || reviewStatus == "current" {
			reviewStatus = "canceled"
		}
		executeStatus = "skipped"
	case dbmodel.SQLTicketStatusApproved:
		reviewStatus = "done"
		executeStatus = "current"
	case dbmodel.SQLTicketStatusRunning:
		reviewStatus = "done"
		executeStatus = "current"
	case dbmodel.SQLTicketStatusSucceeded:
		detail.FlowEnded = true
		detail.FlowEndReason = "閹笛嗩攽閹存劕濮涢敍灞剧ウ缁嬪鍑＄紒鎾存将"
		reviewStatus = "done"
		executeStatus = "done"
	case dbmodel.SQLTicketStatusFailed:
		detail.FlowEnded = true
		detail.FlowEndReason = "閹笛嗩攽婢惰精瑙﹂敍灞剧ウ缁嬪鍑＄紒鎾存将"
		reviewStatus = "done"
		executeStatus = "failed"
	}

	if record, ok := latestExecutionRecord(detail.ExecutionRecords); ok {
		executeComment = record.ErrorMessage
		if record.ErrorMessage == "" {
			executeComment = ticket.ExecuteComment
		}
		executedAt = record.FinishedAt
		if record.Status == dbmodel.SQLExecutionStatusSucceeded && executeStatus != "failed" {
			executeStatus = "done"
		}
		if record.Status == dbmodel.SQLExecutionStatusFailed {
			executeStatus = "failed"
		}
	}

	detail.FlowSteps = []dbmodel.SQLTicketFlowStep{
		{
			Key:          "submit",
			Name:         "提交",
			Status:       submitStatus,
			OperatorID:   ticket.SubmitterID,
			OperatorName: ticket.SubmitterName,
			Comment:      ticket.Remark,
			FinishedAt:   ticket.SubmittedAt,
		},
		{
			Key:          "review",
			Name:         "审批",
			Status:       reviewStatus,
			OperatorID:   ticket.ApproverID,
			OperatorName: ticket.ApproverName,
			Comment:      reviewComment,
			FinishedAt:   reviewedAt,
		},
		{
			Key:          "execute",
			Name:         "閹笛嗩攽",
			Status:       executeStatus,
			OperatorID:   ticket.ExecutorID,
			OperatorName: ticket.ExecutorName,
			Comment:      executeComment,
			FinishedAt:   executedAt,
		},
	}
}

func latestApprovalRecord(records []dbmodel.SQLApprovalRecord) (dbmodel.SQLApprovalRecord, bool) {
	for i := len(records) - 1; i >= 0; i-- {
		switch records[i].Action {
		case dbmodel.SQLApprovalActionApprove, dbmodel.SQLApprovalActionReject, dbmodel.SQLApprovalActionCancel:
			return records[i], true
		}
	}
	return dbmodel.SQLApprovalRecord{}, false
}

func latestExecutionRecord(records []dbmodel.SQLExecutionRecord) (dbmodel.SQLExecutionRecord, bool) {
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Status == dbmodel.SQLExecutionStatusSucceeded || records[i].Status == dbmodel.SQLExecutionStatusFailed {
			return records[i], true
		}
	}
	return dbmodel.SQLExecutionRecord{}, false
}

func (s *ReviewService) ExportResultCSV(ctx *gin.Context, id uint, writer io.Writer) (int64, error) {
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return 0, err
	}
	target, columns, rows, err := parseLatestQueryResult(detail.ExecutionRecords)
	if err != nil {
		return 0, err
	}
	_ = target
	exportRows := limitRows(rows, queryResultDownloadRows)
	return writeResultRowsCSV(writer, columns, exportRows)
}

func (s *ReviewService) QueryResult(ctx *gin.Context, id uint) (*dbmodel.SQLTicketQueryResult, error) {
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	if detail.Ticket.TicketType != dbmodel.SQLTicketTypeQuery {
		return nil, fmt.Errorf("只有查询工单支持查看查询结果")
	}
	target, columns, rows, err := parseLatestQueryResult(detail.ExecutionRecords)
	if err != nil {
		target, columns, rows, err = s.rebuildQueryResult(ctx, detail)
		if err != nil {
			return nil, err
		}
	}
	previewRows := limitRows(rows, queryResultPreviewRows)
	return &dbmodel.SQLTicketQueryResult{
		TicketID:          id,
		ExecutionRecordID: target.ID,
		Columns:           columns,
		Rows:              previewRows,
		Total:             len(rows),
		DurationMS:        target.DurationMS,
		SQL:               target.SQLText,
	}, nil
}

func (s *ReviewService) CreateRollbackTicket(ctx *gin.Context, id uint, req dbmodel.SQLRollbackTicketRequest) (*dbmodel.SQLTicketDetail, error) {
	source, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	if source.Ticket.Status != dbmodel.SQLTicketStatusSucceeded && source.Ticket.Status != dbmodel.SQLTicketStatusFailed {
		return nil, fmt.Errorf("閸樼喎浼愰崡鏇炵毣閺堫亝澧界悰宀嬬礉娑撳秷鍏橀悽鐔稿灇閸ョ偞绮村銉ュ礋")
	}
	rollbackSQL, err := selectRollbackSQL(source.RollbackRecords, req.RollbackRecordID)
	if err != nil {
		return nil, err
	}
	ticketType := rollbackTicketType(rollbackSQL)
	if ticketType == "" {
		return nil, fmt.Errorf("无法识别回滚SQL类型")
	}
	approverID, approverName, err := resolveReviewUser(req.ApproverID, "approver")
	if err != nil {
		return nil, err
	}
	executorID, executorName, err := resolveReviewUser(req.ExecutorID, "executor")
	if err != nil {
		return nil, err
	}
	check := reviewSQL(ticketType, source.Ticket.DatabaseName, source.Ticket.SchemaName, rollbackSQL, false, source.Ticket.Engine)
	if !check.ReviewPassed {
		return nil, fmt.Errorf("回滚SQL检测未通过，请先处理审核错误")
	}

	now := util.HTime{Time: time.Now()}
	submitterID, submitterName := currentAdmin(ctx)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = fmt.Sprintf("閸ョ偞绮村銉ュ礋-%s", source.Ticket.TicketNo)
	}
	remark := strings.TrimSpace(req.Remark)
	if remark == "" {
		remark = fmt.Sprintf("由原工单 %s 生成的回滚变更", source.Ticket.TicketNo)
	}
	ticket := &dbmodel.SQLTicket{
		TicketNo:         buildTicketNo(ticketType),
		Title:            title,
		TicketType:       ticketType,
		AssetID:          source.Ticket.AssetID,
		Engine:           source.Ticket.Engine,
		DatabaseName:     source.Ticket.DatabaseName,
		SchemaName:       source.Ticket.SchemaName,
		Environment:      source.Ticket.Environment,
		SQLContent:       rollbackSQL,
		SQLFingerprint:   sqlFingerprint(rollbackSQL),
		Status:           dbmodel.SQLTicketStatusPendingReview,
		RiskLevel:        check.RiskLevel,
		BackupRequired:   false,
		RollbackRequired: false,
		ReviewPassed:     check.ReviewPassed,
		Remark:           remark,
		SubmitterID:      submitterID,
		SubmitterName:    submitterName,
		SubmittedAt:      now,
		ApproverID:       approverID,
		ApproverName:     approverName,
		ExecutorID:       executorID,
		ExecutorName:     executorName,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	initialRollback := []dbmodel.SQLRollbackRecord{{
		StatementID:  1,
		RollbackSQL:  source.Ticket.SQLContent,
		RollbackType: "rollback_reverse",
		Available:    true,
		Message:      fmt.Sprintf("閸ョ偞绮村銉ュ礋閻ㄥ嫬寮介崥鎱L閿涘本娼靛┃鎰斧瀹搞儱宕?%s", source.Ticket.TicketNo),
		CreatedAt:    now,
	}}
	if err := s.reviewDao.CreateTicket(ticket, check.Statements, check.ReviewResults, initialRollback); err != nil {
		return nil, err
	}
	return s.Detail(ticket.ID)
}

func (s *ReviewService) CleanupBackupTables(ctx *gin.Context, id uint, req dbmodel.SQLBackupCleanupRequest) (*dbmodel.SQLBackupCleanupResult, error) {
	if !req.Confirmed {
		return nil, fmt.Errorf("请确认后再清理备份表")
	}
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	if detail.Ticket.Engine != cmdbmodel.DatabaseTypeMySQL {
		return nil, fmt.Errorf("当前仅支持清理MySQL备份表")
	}
	backupTables := collectBackupTables(detail.RollbackRecords)
	if len(backupTables) == 0 {
		return &dbmodel.SQLBackupCleanupResult{
			DeletedTables: []string{},
			SkippedTables: []string{},
		}, nil
	}
	_, cfg, err := s.dbService.BuildConfig(detail.Ticket.AssetID, detail.Ticket.DatabaseName)
	if err != nil {
		return nil, err
	}
	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(contextFromGin(ctx))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	result := &dbmodel.SQLBackupCleanupResult{
		DeletedTables: make([]string, 0, len(backupTables)),
		SkippedTables: make([]string, 0),
	}
	for _, table := range backupTables {
		if err := validateIdentifier(table.Database); err != nil {
			result.SkippedTables = append(result.SkippedTables, table.String())
			continue
		}
		if err := validateIdentifier(table.Table); err != nil {
			result.SkippedTables = append(result.SkippedTables, table.String())
			continue
		}
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS %s.%s", quoteMySQLIdentifier(table.Database), quoteMySQLIdentifier(table.Table))
		if _, err := conn.ExecContext(contextFromGin(ctx), dropSQL); err != nil {
			result.SkippedTables = append(result.SkippedTables, table.String())
			continue
		}
		result.DeletedTables = append(result.DeletedTables, table.String())
	}
	result.DeletedCount = len(result.DeletedTables)
	return result, nil
}

func (s *ReviewService) GenerateFlashbackRollback(ctx *gin.Context, id uint, req dbmodel.SQLFlashbackGenerateRequest) (*dbmodel.SQLRollbackRecord, error) {
	if !req.Confirmed {
		return nil, fmt.Errorf("生成闪回回滚SQL前请先确认")
	}
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	if detail.Ticket.Engine != cmdbmodel.DatabaseTypeMySQL {
		return nil, fmt.Errorf("当前仅支持MySQL生成binlog闪回SQL")
	}
	record, err := selectFlashbackRollbackRecord(detail.RollbackRecords, req.RollbackRecordID)
	if err != nil {
		return nil, err
	}
	if record.BinlogStartFile == "" || record.BinlogStartPos == 0 || record.BinlogEndFile == "" || record.BinlogEndPos == 0 {
		return nil, fmt.Errorf("回滚记录缺少binlog起止位点，无法生成闪回SQL")
	}
	_, cfg, err := s.dbService.BuildConfig(detail.Ticket.AssetID, detail.Ticket.DatabaseName)
	if err != nil {
		return nil, err
	}
	rollbackSQL, err := runMy2SQLFlashback(ctx, cfg, record)
	now := util.HTime{Time: time.Now()}
	if err != nil {
		_ = s.reviewDao.UpdateRollbackRecord(record.ID, map[string]interface{}{
			"available":      false,
			"message":        "生成闪回回滚SQL失败: " + err.Error(),
			"flashback_tool": "my2sql",
		})
		return nil, err
	}
	updates := map[string]interface{}{
		"rollback_sql":   rollbackSQL,
		"rollback_type":  "binlog_flashback",
		"flashback_tool": "my2sql",
		"generated_at":   now,
		"available":      true,
		"message":        "已通过my2sql生成binlog闪回回滚SQL",
	}
	if err := s.reviewDao.UpdateRollbackRecord(record.ID, updates); err != nil {
		return nil, err
	}
	record.RollbackSQL = rollbackSQL
	record.RollbackType = "binlog_flashback"
	record.FlashbackTool = "my2sql"
	record.GeneratedAt = now
	record.Available = true
	record.Message = "已通过my2sql生成binlog闪回回滚SQL"
	return &record, nil
}

func (s *ReviewService) Approve(ctx *gin.Context, id uint, req dbmodel.SQLTicketActionRequest) (*dbmodel.SQLTicket, error) {
	ticket, err := s.reviewDao.GetTicket(id)
	if err != nil {
		return nil, err
	}
	if ticket.Status != dbmodel.SQLTicketStatusPendingReview {
		return nil, fmt.Errorf("工单状态不是待审批，当前不能审批")
	}
	operatorID, operatorName := currentAdmin(ctx)
	if ticket.WorkflowInstanceID > 0 {
		var task ticketmodel.ApprovalTask
		if err := common.GetDB().Where("instance_id = ? AND assignee_id = ? AND status = ?", ticket.WorkflowInstanceID, operatorID, ticketmodel.ApprovalTaskPending).Order("node_order ASC, id ASC").First(&task).Error; err != nil {
			return nil, fmt.Errorf("当前用户没有待处理的审批任务")
		}
		if _, err := ticketservice.NewWorkflowService(common.GetDB()).Approve(task.ID, operatorID, operatorName, req.Comment); err != nil {
			return nil, err
		}
		return s.decoratedTicket(id)
	}
	if ticket.ApproverID > 0 && operatorID > 0 && ticket.ApproverID != operatorID {
		return nil, fmt.Errorf("无审批权限，请联系审批人 %s", reviewContactName(ticket.ApproverName, ticket.ApproverID))
	}
	now := util.HTime{Time: time.Now()}
	if err := s.reviewDao.UpdateTicket(id, map[string]interface{}{
		"status":         dbmodel.SQLTicketStatusApproved,
		"reviewed_at":    now,
		"review_comment": strings.TrimSpace(req.Comment),
	}); err != nil {
		return nil, err
	}
	_ = s.reviewDao.AddApprovalRecord(&dbmodel.SQLApprovalRecord{
		TicketID:     id,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		Action:       dbmodel.SQLApprovalActionApprove,
		Comment:      strings.TrimSpace(req.Comment),
		CreatedAt:    now,
	})
	return s.decoratedTicket(id)
}

func (s *ReviewService) Reject(ctx *gin.Context, id uint, req dbmodel.SQLTicketActionRequest) (*dbmodel.SQLTicket, error) {
	ticket, err := s.reviewDao.GetTicket(id)
	if err != nil {
		return nil, err
	}
	if ticket.Status != dbmodel.SQLTicketStatusPendingReview {
		return nil, fmt.Errorf("工单状态不是待审批，当前不能驳回")
	}
	operatorID, operatorName := currentAdmin(ctx)
	if ticket.WorkflowInstanceID > 0 {
		var task ticketmodel.ApprovalTask
		if err := common.GetDB().Where("instance_id = ? AND assignee_id = ? AND status = ?", ticket.WorkflowInstanceID, operatorID, ticketmodel.ApprovalTaskPending).Order("node_order ASC, id ASC").First(&task).Error; err != nil {
			return nil, fmt.Errorf("当前用户没有待处理的审批任务")
		}
		if _, err := ticketservice.NewWorkflowService(common.GetDB()).Reject(task.ID, operatorID, operatorName, req.Comment); err != nil {
			return nil, err
		}
		return s.decoratedTicket(id)
	}
	if ticket.ApproverID > 0 && operatorID > 0 && ticket.ApproverID != operatorID {
		return nil, fmt.Errorf("无审批权限，请联系审批人 %s", reviewContactName(ticket.ApproverName, ticket.ApproverID))
	}
	now := util.HTime{Time: time.Now()}
	if err := s.reviewDao.UpdateTicket(id, map[string]interface{}{
		"status":         dbmodel.SQLTicketStatusRejected,
		"reviewed_at":    now,
		"review_comment": strings.TrimSpace(req.Comment),
	}); err != nil {
		return nil, err
	}
	_ = s.reviewDao.AddApprovalRecord(&dbmodel.SQLApprovalRecord{
		TicketID:     id,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		Action:       dbmodel.SQLApprovalActionReject,
		Comment:      strings.TrimSpace(req.Comment),
		CreatedAt:    now,
	})
	return s.decoratedTicket(id)
}

func (s *ReviewService) Cancel(ctx *gin.Context, id uint, req dbmodel.SQLTicketActionRequest) (*dbmodel.SQLTicket, error) {
	ticket, err := s.reviewDao.GetTicket(id)
	if err != nil {
		return nil, err
	}
	if ticket.Status == dbmodel.SQLTicketStatusRunning || ticket.Status == dbmodel.SQLTicketStatusSucceeded {
		return nil, fmt.Errorf("工单正在执行或已执行成功，不能撤销")
	}
	operatorID, operatorName := currentAdmin(ctx)
	if ticket.SubmitterID > 0 && operatorID > 0 && ticket.SubmitterID != operatorID {
		return nil, fmt.Errorf("无撤销权限，请联系提交人 %s", reviewContactName(ticket.SubmitterName, ticket.SubmitterID))
	}
	now := util.HTime{Time: time.Now()}
	err = common.GetDB().Transaction(func(tx *gorm.DB) error {
		if ticket.WorkflowInstanceID > 0 {
			if err := ticketservice.NewWorkflowService(tx).CancelWithTx(tx, ticket.WorkflowInstanceID, operatorID, operatorName, req.Comment); err != nil {
				return err
			}
		}
		if err := tx.Model(&dbmodel.SQLTicket{}).Where("id = ?", id).Updates(map[string]interface{}{"status": dbmodel.SQLTicketStatusCanceled, "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Create(&dbmodel.SQLApprovalRecord{
			TicketID: id, OperatorID: operatorID, OperatorName: operatorName,
			Action: dbmodel.SQLApprovalActionCancel, Comment: strings.TrimSpace(req.Comment), CreatedAt: now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.decoratedTicket(id)
}

func (s *ReviewService) Execute(ctx *gin.Context, id uint, req dbmodel.SQLTicketActionRequest) (*dbmodel.SQLTicket, error) {
	ticket, err := s.reviewDao.GetTicket(id)
	if err != nil {
		return nil, err
	}
	if ticket.Status != dbmodel.SQLTicketStatusApproved {
		return nil, fmt.Errorf("工单未审批通过，当前不能执行")
	}
	if !req.Confirmed.Bool() {
		return nil, fmt.Errorf("执行前请先确认")
	}
	operatorID, operatorName := currentAdmin(ctx)
	if ticket.ExecutorID > 0 && operatorID > 0 && ticket.ExecutorID != operatorID {
		return nil, fmt.Errorf("无执行权限，请联系执行人 %s", reviewContactName(ticket.ExecutorName, ticket.ExecutorID))
	}
	detail, err := s.reviewDao.GetTicketDetail(id)
	if err != nil {
		return nil, err
	}
	now := util.HTime{Time: time.Now()}
	if len(detail.Statements) == 0 {
		return nil, fmt.Errorf("工单没有可执行SQL")
	}

	if err := s.reviewDao.UpdateTicket(id, map[string]interface{}{
		"status":          dbmodel.SQLTicketStatusRunning,
		"execute_comment": strings.TrimSpace(req.Comment),
	}); err != nil {
		return nil, err
	}

	_, cfg, err := s.dbService.BuildConfig(ticket.AssetID, ticket.DatabaseName)
	if err != nil {
		_ = s.reviewDao.UpdateTicket(id, map[string]interface{}{
			"status": dbmodel.SQLTicketStatusFailed,
		})
		return nil, err
	}

	finalStatus := dbmodel.SQLTicketStatusSucceeded
	statements := detail.Statements
	if ticket.TicketType == dbmodel.SQLTicketTypeDDL && ticket.UseGhOst {
		statements = []dbmodel.SQLStatement{{
			ID:             firstStatementID(detail.Statements),
			TicketID:       id,
			StatementIndex: 1,
			SQLText:        ticket.SQLContent,
			StatementType:  "online_ddl",
		}}
	}
	for _, statement := range statements {
		startedAt := util.HTime{Time: time.Now()}
		record := &dbmodel.SQLExecutionRecord{
			TicketID:     id,
			StatementID:  statement.ID,
			SQLText:      statement.SQLText,
			Status:       dbmodel.SQLExecutionStatusRunning,
			StartedAt:    startedAt,
			ExecutorID:   operatorID,
			ExecutorName: operatorName,
			CreatedAt:    now,
		}
		execResult, execErr := s.executeTicketStatement(ctx, cfg, ticket, statement)
		record.FinishedAt = util.HTime{Time: time.Now()}
		if execResult != nil {
			record.RowsAffected = execResult.RowsAffected
			record.DurationMS = execResult.DurationMS
			if execResult.Rollback != nil {
				if err := s.reviewDao.AddRollbackRecord(execResult.Rollback); err != nil {
					return nil, err
				}
			}
			if execResult.Result != nil {
				record.ResultColumns = encodeJSON(execResult.Result.Columns)
				record.ResultRows = encodeJSON(execResult.Result.Rows)
			}
			record.ExecLog = execResult.ExecLog
		} else {
			record.DurationMS = time.Since(startedAt.Time).Milliseconds()
		}
		if execErr != nil {
			record.Status = dbmodel.SQLExecutionStatusFailed
			record.ErrorMessage = execErr.Error()
			finalStatus = dbmodel.SQLTicketStatusFailed
		} else {
			record.Status = dbmodel.SQLExecutionStatusSucceeded
		}
		if err := s.reviewDao.AddExecutionRecord(record); err != nil {
			return nil, err
		}
		if execErr != nil {
			break
		}
	}

	if err := s.reviewDao.UpdateTicket(id, map[string]interface{}{
		"status":          finalStatus,
		"executed_at":     util.HTime{Time: time.Now()},
		"execute_comment": strings.TrimSpace(req.Comment),
	}); err != nil {
		return nil, err
	}
	return s.decoratedTicket(id)
}

func (s *ReviewService) executeTicketStatement(ctx *gin.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	if ticket.TicketType == dbmodel.SQLTicketTypeDDL && ticket.UseGhOst {
		return executeGhOstTicketStatement(ctx, ticket, statement)
	}
	if ticket.TicketType == dbmodel.SQLTicketTypeQuery {
		start := time.Now()
		data, err := s.dbService.ExecuteSQL(ctx, dbmodel.SQLExecuteRequest{
			AssetID:   ticket.AssetID,
			Database:  ticket.DatabaseName,
			Schema:    ticket.SchemaName,
			SQL:       statement.SQLText,
			Page:      1,
			PageSize:  queryResultDownloadRows,
			Confirmed: dbmodel.SQLConfirm(true),
		})
		durationMS := time.Since(start).Milliseconds()
		if err != nil {
			return &reviewExecuteResult{DurationMS: durationMS}, err
		}
		rowsAffected := int64(0)
		if data != nil {
			rowsAffected = int64(len(data.Rows))
			if data.DurationMS > 0 {
				durationMS = data.DurationMS
			}
		}
		return &reviewExecuteResult{
			RowsAffected: rowsAffected,
			DurationMS:   durationMS,
			Result:       data,
		}, nil
	}
	return executeReviewStatement(contextFromGin(ctx), cfg, ticket, statement)
}

func (s *ReviewService) decoratedTicket(id uint) (*dbmodel.SQLTicket, error) {
	ticket, err := s.reviewDao.GetTicket(id)
	if err != nil {
		return nil, err
	}
	tickets := []dbmodel.SQLTicket{*ticket}
	s.decorateTickets(tickets)
	return &tickets[0], nil
}

func (s *ReviewService) decorateTickets(tickets []dbmodel.SQLTicket) {
	s.fillTicketAssetInfo(tickets)
	s.fillTicketUserNames(tickets)
	s.fillTicketLastErrors(tickets)
}

func (s *ReviewService) fillDMLCheckExtras(ctx *gin.Context, req dbmodel.SQLReviewCheckRequest, check *dbmodel.SQLReviewCheckResult, dbType int, cfg ConnConfig) {
	if check == nil || len(check.Statements) == 0 {
		return
	}
	var engineFindings []dbmodel.SQLReviewResult
	if dbType == cmdbmodel.DatabaseTypeMySQL {
		engineFindings = checkMySQLBinlogReviewReadiness(contextFromGin(ctx), cfg)
	}
	if dbType == cmdbmodel.DatabaseTypePostgreSQL {
		engineFindings = checkPostgreSQLWalReviewReadiness()
	}
	for i := range check.Statements {
		statement := &check.Statements[i]
		if !isDMLType(statement.StatementType) {
			continue
		}
		statementFindings := make([]dbmodel.SQLReviewResult, 0)
		if statement.StatementType == "update" || statement.StatementType == "delete" {
			if dbType == cmdbmodel.DatabaseTypeMySQL {
				estimatedRows := estimateMySQLDMLRows(contextFromGin(ctx), cfg, statement.SQLText, req.Database)
				statement.EstimatedRows = estimatedRows
				check.EstimatedRows += estimatedRows
				statementFindings = append(statementFindings, s.checkDMLKeyCondition(contextFromGin(ctx), cfg, req.Database, statement.SQLText)...)
			}
			statementFindings = append(statementFindings, engineFindings...)
		}
		if statement.StatementType == "insert" || statement.StatementType == "replace" {
			statementFindings = append(statementFindings, reviewFinding(0, "dml_insert_rollback_unique_condition", "INSERT回滚需要可靠唯一条件", dbmodel.SQLReviewLevelWarn, "如果手工编写DELETE回滚SQL，必须带主键或唯一键条件，避免误删相似数据", ""))
		}
		if dbType == cmdbmodel.DatabaseTypeMySQL {
			rollbackSQL := buildDMLCheckRollbackSQL(req.Database, statement.SQLText)
			if rollbackSQL != "" {
				if check.RollbackSQL != "" {
					check.RollbackSQL += "\n"
				}
				check.RollbackSQL += rollbackSQL
			}
		}
		for j := range statementFindings {
			statementFindings[j].StatementID = uint(statement.StatementIndex)
		}
		check.ReviewResults = append(check.ReviewResults, statementFindings...)
	}
}

func checkPostgreSQLWalReviewReadiness() []dbmodel.SQLReviewResult {
	return []dbmodel.SQLReviewResult{
		reviewFinding(0, "pgsql_walminer_required", "PostgreSQL自动回滚依赖WalMiner/WAL解析能力", dbmodel.SQLReviewLevelWarn, "如需生成Undo SQL，请在数据库工具箱中部署WalMiner并确认目标库WAL保留策略；未部署时请提交手工回滚SQL", ""),
		reviewFinding(0, "pgsql_dml_key_condition", "PostgreSQL回滚建议使用主键或唯一键限定范围", dbmodel.SQLReviewLevelWarn, "UPDATE/DELETE建议WHERE条件包含主键或唯一键，避免Undo SQL条件过宽", ""),
	}
}

func checkMySQLBinlogReviewReadiness(ctx context.Context, cfg ConnConfig) []dbmodel.SQLReviewResult {
	db, err := openMySQL(cfg, "")
	if err != nil {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_binlog_check_failed", "无法检查MySQL Binlog配置", dbmodel.SQLReviewLevelWarn, "执行前请确认目标库已开启 Binlog、binlog_format=ROW、binlog_row_image=FULL", "")}
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_binlog_check_failed", "无法连接数据库检查Binlog配置", dbmodel.SQLReviewLevelWarn, "执行前请确认目标库已开启 Binlog、binlog_format=ROW、binlog_row_image=FULL", "")}
	}
	defer conn.Close()
	logBin, err := getMySQLServerVariable(ctx, conn, "log_bin")
	if err != nil || !isMySQLOn(logBin) {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_binlog_not_enabled", "目标库未开启Binlog", dbmodel.SQLReviewLevelWarn, "DML闪回回滚依赖Binlog，请开启 log_bin", "")}
	}
	binlogFormat, err := getMySQLServerVariable(ctx, conn, "binlog_format")
	if err != nil || !strings.EqualFold(strings.TrimSpace(binlogFormat), "ROW") {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_binlog_row_required", "目标库binlog_format不是ROW", dbmodel.SQLReviewLevelWarn, "DML闪回回滚依赖ROW格式Binlog，请设置 binlog_format=ROW", "")}
	}
	rowImage, err := getMySQLServerVariable(ctx, conn, "binlog_row_image")
	if err != nil || !strings.EqualFold(strings.TrimSpace(rowImage), "FULL") {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_binlog_row_image_full_required", "目标库binlog_row_image不是FULL", dbmodel.SQLReviewLevelWarn, "DML闪回回滚需要完整行镜像，请设置 binlog_row_image=FULL", "")}
	}
	return nil
}

func (s *ReviewService) checkDMLKeyCondition(ctx context.Context, cfg ConnConfig, defaultDatabase, sqlText string) []dbmodel.SQLReviewResult {
	parsed, err := parseMySQLDMLStatement(sqlText, defaultDatabase)
	if err != nil || (parsed.StatementType != "update" && parsed.StatementType != "delete") {
		return nil
	}
	columns, indexes, err := loadMySQLTableKeyMetadata(ctx, cfg, parsed.DatabaseName, parsed.TableName)
	if err != nil {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_key_metadata_unavailable", "无法读取表主键/唯一键信息", dbmodel.SQLReviewLevelWarn, "建议确认WHERE条件包含主键或唯一键，避免回滚SQL条件过长或误回滚", "")}
	}
	keyGroups := buildUniqueKeyGroups(columns, indexes)
	if len(keyGroups) == 0 {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_table_no_primary_or_unique_key", "目标表未发现主键或唯一键", dbmodel.SQLReviewLevelWarn, "建议先为表补充主键或唯一键；否则回滚SQL可能需要包含所有字段，效率低且并发场景容易失败", "")}
	}
	if !whereContainsAnyKeyGroup(parsed.WhereClause, keyGroups) {
		return []dbmodel.SQLReviewResult{reviewFinding(0, "dml_where_missing_primary_or_unique_key", "WHERE条件未明显包含主键或唯一键", dbmodel.SQLReviewLevelWarn, "建议WHERE条件带上主键或唯一键，例如 WHERE id = 1，提升执行和回滚可靠性", "")}
	}
	return nil
}

func (s *ReviewService) fillTicketAssetInfo(tickets []dbmodel.SQLTicket) {
	if len(tickets) == 0 {
		return
	}
	assetIDs := make([]uint, 0, len(tickets))
	seen := map[uint]struct{}{}
	for _, ticket := range tickets {
		if ticket.AssetID == 0 {
			continue
		}
		if _, ok := seen[ticket.AssetID]; ok {
			continue
		}
		seen[ticket.AssetID] = struct{}{}
		assetIDs = append(assetIDs, ticket.AssetID)
	}
	if len(assetIDs) == 0 {
		return
	}

	var assets []struct {
		ID           uint
		Name         string
		InstanceName string
		Type         int
	}
	if err := common.GetDB().
		Table("cmdb_db_management").
		Select("id, name, instance_name, type").
		Where("id IN ?", assetIDs).
		Find(&assets).Error; err != nil {
		return
	}
	assetMap := make(map[uint]struct {
		Name         string
		InstanceName string
		TypeName     string
	}, len(assets))
	for _, asset := range assets {
		assetMap[asset.ID] = struct {
			Name         string
			InstanceName string
			TypeName     string
		}{
			Name:         asset.Name,
			InstanceName: asset.InstanceName,
			TypeName:     databaseTypeName(asset.Type),
		}
	}
	for i := range tickets {
		asset, ok := assetMap[tickets[i].AssetID]
		if !ok {
			continue
		}
		tickets[i].AssetName = asset.Name
		tickets[i].AssetInstance = asset.InstanceName
		tickets[i].AssetTypeName = asset.TypeName
	}
}

func (s *ReviewService) fillTicketUserNames(tickets []dbmodel.SQLTicket) {
	if len(tickets) == 0 {
		return
	}
	userIDs := make([]uint, 0, len(tickets)*3)
	seen := map[uint]struct{}{}
	for _, ticket := range tickets {
		for _, userID := range []uint{ticket.SubmitterID, ticket.ApproverID, ticket.ExecutorID} {
			if userID == 0 {
				continue
			}
			if _, ok := seen[userID]; ok {
				continue
			}
			seen[userID] = struct{}{}
			userIDs = append(userIDs, userID)
		}
	}
	if len(userIDs) == 0 {
		return
	}

	var users []systemmodel.SysAdmin
	if err := common.GetDB().Select("id, username").Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return
	}
	userMap := make(map[uint]string, len(users))
	for _, user := range users {
		if username := strings.TrimSpace(user.Username); username != "" {
			userMap[user.ID] = username
		}
	}
	for i := range tickets {
		if username := userMap[tickets[i].SubmitterID]; username != "" {
			tickets[i].SubmitterName = username
		}
		if username := userMap[tickets[i].ApproverID]; username != "" {
			tickets[i].ApproverName = username
		}
		if username := userMap[tickets[i].ExecutorID]; username != "" {
			tickets[i].ExecutorName = username
		}
	}
}

func (s *ReviewService) fillTicketLastErrors(tickets []dbmodel.SQLTicket) {
	if len(tickets) == 0 {
		return
	}
	ticketIDs := make([]uint, 0, len(tickets))
	seen := map[uint]struct{}{}
	for _, ticket := range tickets {
		if ticket.ID == 0 {
			continue
		}
		if _, ok := seen[ticket.ID]; ok {
			continue
		}
		seen[ticket.ID] = struct{}{}
		ticketIDs = append(ticketIDs, ticket.ID)
	}
	if len(ticketIDs) == 0 {
		return
	}

	var records []dbmodel.SQLExecutionRecord
	if err := common.GetDB().
		Select("ticket_id, error_message").
		Where("ticket_id IN ? AND status = ? AND error_message <> ''", ticketIDs, dbmodel.SQLExecutionStatusFailed).
		Order("id DESC").
		Find(&records).Error; err != nil {
		return
	}
	errorMap := make(map[uint]string, len(records))
	for _, record := range records {
		if _, ok := errorMap[record.TicketID]; ok {
			continue
		}
		errorMap[record.TicketID] = record.ErrorMessage
	}
	for i := range tickets {
		tickets[i].LastError = errorMap[tickets[i].ID]
	}
}

func reviewSQL(ticketType, databaseName, schemaName, rawSQL string, backupRequired bool, dbType int) *dbmodel.SQLReviewCheckResult {
	statements := splitSQLStatements(rawSQL)
	result := &dbmodel.SQLReviewCheckResult{
		TicketType:    ticketType,
		ReviewPassed:  true,
		RiskLevel:     dbmodel.SQLRiskLow,
		Statements:    make([]dbmodel.SQLStatement, 0, len(statements)),
		ReviewResults: []dbmodel.SQLReviewResult{},
	}
	if len(statements) == 0 {
		result.ReviewPassed = false
		result.RiskLevel = dbmodel.SQLRiskCritical
		result.ReviewResults = append(result.ReviewResults, reviewFinding(0, "sql_empty", "SQL不能为空", dbmodel.SQLReviewLevelError, "请填写需要审核的SQL", ""))
		return result
	}
	for i, sqlText := range statements {
		stmtType := detectSQLType(sqlText)
		originalSQLText := sqlText
		if ticketType == dbmodel.SQLTicketTypeQuery && isQueryType(stmtType) {
			sqlText = ensureQueryLimit(sqlText, queryReviewDefaultLimit)
		}
		stmtRisk := riskForSQLType(ticketType, stmtType)
		status := dbmodel.SQLReviewStatusPass
		message := "pass"
		statementIndex := i + 1
		statement := dbmodel.SQLStatement{
			StatementIndex: statementIndex,
			SQLText:        sqlText,
			StatementType:  stmtType,
			ObjectName:     extractObjectName(sqlText),
			DatabaseName:   databaseName,
			SchemaName:     schemaName,
			RiskLevel:      stmtRisk,
			ReviewStatus:   status,
			Message:        message,
			CreatedAt:      util.HTime{Time: time.Now()},
		}
		findings := reviewStatement(ticketType, stmtType, originalSQLText, backupRequired, dbType)
		for j := range findings {
			findings[j].StatementID = uint(statementIndex)
			if findings[j].Level == dbmodel.SQLReviewLevelError {
				result.ReviewPassed = false
				status = dbmodel.SQLReviewStatusError
				message = findings[j].Message
			} else if findings[j].Level == dbmodel.SQLReviewLevelWarn && status != dbmodel.SQLReviewStatusError {
				status = dbmodel.SQLReviewStatusWarn
				message = findings[j].Message
			}
			result.RiskLevel = maxRisk(result.RiskLevel, riskFromLevel(findings[j].Level, stmtRisk))
		}
		statement.ReviewStatus = status
		statement.Message = message
		result.RiskLevel = maxRisk(result.RiskLevel, stmtRisk)
		result.Statements = append(result.Statements, statement)
		result.ReviewResults = append(result.ReviewResults, findings...)
	}
	return result
}

func reviewStatement(ticketType, stmtType, sqlText string, backupRequired bool, dbType int) []dbmodel.SQLReviewResult {
	findings := []dbmodel.SQLReviewResult{}
	findings = append(findings, reviewSQLSyntax(stmtType, sqlText)...)
	findings = append(findings, reviewEngineCompatibility(dbType, ticketType, stmtType, sqlText)...)

	if ticketType == dbmodel.SQLTicketTypeDML && !isDMLType(stmtType) {
		message := "DML工单只能提交DML语句"
		suggestion := "请切换到DDL工单或修改SQL"
		if isDDLType(stmtType) {
			message = "当前SQL是DDL语句，请提交DDL工单"
			suggestion = "请使用 /api/v1/db/review/ddl/check 和 /api/v1/db/review/ddl/tickets"
		} else if isQueryType(stmtType) {
			message = "当前SQL是查询语句，请提交查询工单"
			suggestion = "请使用 /api/v1/db/review/query/check 和 /api/v1/db/review/query/tickets"
		} else if stmtType == "unknown" {
			message = "无法识别SQL语句类型"
			suggestion = buildUnknownSQLSuggestion(sqlText)
		}
		findings = append(findings, reviewFinding(0, "type_mismatch", message, dbmodel.SQLReviewLevelError, suggestion, ""))
	}

	if ticketType == dbmodel.SQLTicketTypeDDL && !isDDLType(stmtType) {
		message := "DDL工单只能提交DDL语句"
		suggestion := "请切换到DML工单或修改SQL"
		if isDMLType(stmtType) {
			message = "当前SQL是DML语句，请提交DML工单"
			suggestion = "请使用 /api/v1/db/review/dml/check 和 /api/v1/db/review/dml/tickets"
		} else if isQueryType(stmtType) {
			message = "当前SQL是查询语句，请提交查询工单"
			suggestion = "请使用 /api/v1/db/review/query/check 和 /api/v1/db/review/query/tickets"
		} else if stmtType == "unknown" {
			message = "无法识别SQL语句类型"
			suggestion = buildUnknownSQLSuggestion(sqlText)
		}
		findings = append(findings, reviewFinding(0, "type_mismatch", message, dbmodel.SQLReviewLevelError, suggestion, ""))
	}

	if ticketType == dbmodel.SQLTicketTypeQuery && !isQueryType(stmtType) {
		message := "查询工单只能提交SELECT/WITH语句"
		suggestion := "请填写只读查询SQL"
		if isDDLType(stmtType) {
			message = "当前SQL是DDL语句，请提交DDL工单"
			suggestion = "请使用 /api/v1/db/review/ddl/check 和 /api/v1/db/review/ddl/tickets"
		} else if isDMLType(stmtType) {
			message = "当前SQL是DML语句，请提交DML工单"
			suggestion = "请使用 /api/v1/db/review/dml/check 和 /api/v1/db/review/dml/tickets"
		}
		findings = append(findings, reviewFinding(0, "type_mismatch", message, dbmodel.SQLReviewLevelError, suggestion, ""))
	}

	findings = append(findings, reviewBuiltinSafetyRules(ticketType, stmtType, sqlText, dbType)...)

	if isDMLType(stmtType) {
		if (stmtType == "update" || stmtType == "delete") && !containsWord(sqlText, "where") {
			findings = append(findings, reviewFinding(0, "dml_require_where", "UPDATE/DELETE必须带WHERE条件", dbmodel.SQLReviewLevelError, "补充明确的WHERE条件", ""))
		}
		if dbType == cmdbmodel.DatabaseTypePostgreSQL && (stmtType == "update" || stmtType == "delete") {
			findings = append(findings, reviewFinding(0, "pgsql_dml_batch_advice", "PostgreSQL UPDATE/DELETE不支持直接LIMIT", dbmodel.SQLReviewLevelWarn, "请先用WHERE精确限定范围；大批量变更建议使用CTE或主键分批执行", ""))
		}
		if dbType != cmdbmodel.DatabaseTypePostgreSQL && (stmtType == "update" || stmtType == "delete") && !containsWord(sqlText, "limit") {
			findings = append(findings, reviewFinding(0, "dml_warn_no_limit", "UPDATE/DELETE建议带LIMIT限制影响范围", dbmodel.SQLReviewLevelWarn, "评估影响行数后增加LIMIT", ""))
		}
		if !backupRequired {
			findings = append(findings, reviewFinding(0, "dml_require_backup", "DML建议开启自动回滚能力或填写回滚SQL", dbmodel.SQLReviewLevelWarn, "开启自动回滚或补充回滚SQL", ""))
		}
	}

	if isDDLType(stmtType) {
		if stmtType == "drop_database" {
			findings = append(findings, reviewFinding(0, "ddl_forbid_drop_database", "禁止DROP DATABASE", dbmodel.SQLReviewLevelError, "请人工拆解变更方案", ""))
		}
		if stmtType == "drop_table" {
			findings = append(findings, reviewFinding(0, "ddl_forbid_drop_table_prod", "DROP TABLE属于高危DDL", dbmodel.SQLReviewLevelError, "生产环境禁止直接DROP TABLE", ""))
		}
		if stmtType == "truncate" {
			findings = append(findings, reviewFinding(0, "ddl_forbid_truncate_prod", "TRUNCATE属于高危DDL", dbmodel.SQLReviewLevelError, "请改用可回滚方案", ""))
		}
		if stmtType == "alter" {
			findings = append(findings, reviewFinding(0, "ddl_warn_alter_table", "ALTER TABLE需要评估表大小和锁表风险", dbmodel.SQLReviewLevelWarn, "大表建议走在线DDL", ""))
		}
	}

	if isQueryType(stmtType) && !containsWord(sqlText, "limit") {
		findings = append(findings, reviewFinding(0, "query_warn_no_limit", "查询SQL建议增加LIMIT", dbmodel.SQLReviewLevelWarn, "后台会默认补充LIMIT 1000", ""))
	}
	if len(findings) == 0 {
		findings = append(findings, reviewFinding(0, "review_pass", "基础审核通过", dbmodel.SQLReviewLevelInfo, "", ""))
	}
	return findings
}

func reviewBuiltinSafetyRules(ticketType, stmtType, sqlText string, dbType int) []dbmodel.SQLReviewResult {
	findings := make([]dbmodel.SQLReviewResult, 0)
	switch {
	case isDMLType(stmtType):
		findings = append(findings, reviewBuiltinDMLSafetyRules(stmtType, sqlText, dbType)...)
	case isDDLType(stmtType):
		findings = append(findings, reviewBuiltinDDLSafetyRules(stmtType, sqlText, dbType)...)
	case ticketType == dbmodel.SQLTicketTypeQuery && isQueryType(stmtType):
		findings = append(findings, reviewBuiltinQuerySafetyRules(sqlText, dbType)...)
	}
	return findings
}

func reviewBuiltinDMLSafetyRules(stmtType, sqlText string, dbType int) []dbmodel.SQLReviewResult {
	findings := make([]dbmodel.SQLReviewResult, 0)
	if stmtType == "replace" && dbType == cmdbmodel.DatabaseTypeMySQL {
		findings = append(findings, reviewFinding(0, "mysql_dml_replace_high_risk", "REPLACE可能触发DELETE+INSERT", dbmodel.SQLReviewLevelWarn, "建议改用 INSERT ... ON DUPLICATE KEY UPDATE，并确认不会触发级联删除或触发器副作用", ""))
	}
	if stmtType == "insert" || stmtType == "replace" {
		if !insertHasColumnList(sqlText) {
			findings = append(findings, reviewFinding(0, "dml_insert_require_columns", "INSERT/REPLACE建议显式指定字段列表", dbmodel.SQLReviewLevelWarn, "请写成 INSERT INTO table_name (col1, col2) VALUES (...)，避免表结构变更导致写错字段", ""))
		}
		if regexp.MustCompile(`(?is)\binsert\s+into\b.*\bselect\b`).MatchString(sqlText) {
			findings = append(findings, reviewFinding(0, "dml_insert_select_high_risk", "INSERT INTO ... SELECT 可能产生大量写入", dbmodel.SQLReviewLevelWarn, "请确认SELECT范围和目标行数，必要时分批写入", ""))
		}
	}
	if stmtType == "update" || stmtType == "delete" {
		if hasAlwaysTrueWhere(sqlText) {
			findings = append(findings, reviewFinding(0, "dml_forbid_always_true_where", "UPDATE/DELETE禁止使用恒真WHERE条件", dbmodel.SQLReviewLevelError, "请改为明确的主键、唯一键或业务条件，禁止 WHERE 1=1 / OR 1=1 / WHERE TRUE", ""))
		}
		if hasWhereOR(sqlText) {
			findings = append(findings, reviewFinding(0, "dml_warn_or_condition", "UPDATE/DELETE的WHERE包含OR条件", dbmodel.SQLReviewLevelWarn, "OR条件容易扩大影响范围，请确认命中行数并优先使用主键/唯一键条件", ""))
		}
		if hasJoinInDML(sqlText) {
			findings = append(findings, reviewFinding(0, "dml_warn_join_update_delete", "UPDATE/DELETE JOIN属于高风险变更", dbmodel.SQLReviewLevelWarn, "请先用SELECT确认JOIN结果集，并尽量拆成主键列表分批执行", ""))
		}
		if dbType == cmdbmodel.DatabaseTypeMySQL && containsWord(sqlText, "limit") && !containsWord(sqlText, "order") {
			findings = append(findings, reviewFinding(0, "mysql_dml_limit_require_order", "MySQL UPDATE/DELETE使用LIMIT时建议配合ORDER BY", dbmodel.SQLReviewLevelWarn, "没有ORDER BY时LIMIT命中顺序不稳定，请指定主键或时间字段排序", ""))
		}
	}
	return findings
}

func reviewBuiltinDDLSafetyRules(stmtType, sqlText string, dbType int) []dbmodel.SQLReviewResult {
	findings := make([]dbmodel.SQLReviewResult, 0)
	if stmtType == "create_table" && !containsWord(normalizedReviewSQL(sqlText), "primary key") {
		findings = append(findings, reviewFinding(0, "ddl_create_table_require_primary_key", "CREATE TABLE必须包含PRIMARY KEY", dbmodel.SQLReviewLevelError, "请为新表设计主键，避免复制、回滚、分页和清理任务退化为全表扫描", ""))
	}
	if stmtType == "alter" {
		if regexp.MustCompile(`(?is)\balter\s+table\b.*\bdrop\s+(?:column\s+)?`).MatchString(normalizedReviewSQL(sqlText)) {
			findings = append(findings, reviewFinding(0, "ddl_forbid_drop_column", "禁止直接DROP COLUMN", dbmodel.SQLReviewLevelError, "请先确认字段无人使用并走可回滚变更方案，必要时先改名观察再删除", ""))
		}
		if regexp.MustCompile(`(?is)\badd\s+(?:column\s+)?[^\n;]+not\s+null\b`).MatchString(normalizedReviewSQL(sqlText)) && !containsWord(sqlText, "default") {
			findings = append(findings, reviewFinding(0, "ddl_add_not_null_require_default", "新增NOT NULL字段必须提供DEFAULT", dbmodel.SQLReviewLevelWarn, "大表新增非空字段可能锁表或回填失败，请提供默认值或拆分为可灰度步骤", ""))
		}
		if regexp.MustCompile(`(?is)\b(modify|change)\s+(?:column\s+)?`).MatchString(normalizedReviewSQL(sqlText)) {
			findings = append(findings, reviewFinding(0, "ddl_warn_modify_column", "修改字段类型/定义需要评估兼容性和锁表风险", dbmodel.SQLReviewLevelWarn, "请确认数据可转换、索引影响和业务兼容性，大表建议使用在线DDL工具", ""))
		}
	}
	if dbType == cmdbmodel.DatabaseTypeMySQL && stmtType == "create_table" {
		normalized := normalizedReviewSQL(sqlText)
		if !regexp.MustCompile(`(?is)\bengine\s*=\s*innodb\b`).MatchString(normalized) {
			findings = append(findings, reviewFinding(0, "mysql_ddl_create_table_engine_innodb", "MySQL建表建议显式指定ENGINE=InnoDB", dbmodel.SQLReviewLevelWarn, "请确认表使用InnoDB，避免误用非事务存储引擎", ""))
		}
		if !regexp.MustCompile(`(?is)\b(?:default\s+)?charset\s*=?\s*utf8mb4\b|\bcharacter\s+set\s*=?\s*utf8mb4\b`).MatchString(normalized) {
			findings = append(findings, reviewFinding(0, "mysql_ddl_create_table_charset_utf8mb4", "MySQL建表建议显式指定utf8mb4字符集", dbmodel.SQLReviewLevelWarn, "请使用 utf8mb4，避免中文或emoji写入异常", ""))
		}
	}
	if dbType == cmdbmodel.DatabaseTypePostgreSQL && stmtType == "create_index" && !containsWord(sqlText, "concurrently") {
		findings = append(findings, reviewFinding(0, "pgsql_ddl_create_index_concurrently", "PostgreSQL生产建索引建议使用CONCURRENTLY", dbmodel.SQLReviewLevelWarn, "大表请使用 CREATE INDEX CONCURRENTLY，降低写入阻塞风险", ""))
	}
	if dbType == cmdbmodel.DatabaseTypeMySQL && stmtType == "create_index" {
		findings = append(findings, reviewFinding(0, "mysql_ddl_create_index_online_advice", "MySQL大表建索引需要评估锁表风险", dbmodel.SQLReviewLevelWarn, "大表建议走gh-ost/online DDL或低峰执行", ""))
	}
	return findings
}

func reviewBuiltinQuerySafetyRules(sqlText string, dbType int) []dbmodel.SQLReviewResult {
	findings := make([]dbmodel.SQLReviewResult, 0)
	if hasSelectStar(sqlText) {
		findings = append(findings, reviewFinding(0, "query_warn_select_star", "查询不建议使用SELECT *", dbmodel.SQLReviewLevelWarn, "请只选择需要的字段，减少网络传输和回表开销", ""))
	}
	if !containsWord(sqlText, "where") {
		findings = append(findings, reviewFinding(0, "query_warn_no_where", "查询SQL建议包含WHERE条件", dbmodel.SQLReviewLevelWarn, "请增加时间、主键或业务条件，避免全表扫描", ""))
	}
	if hasFullWildcardLike(sqlText) {
		findings = append(findings, reviewFinding(0, "query_warn_full_wildcard_like", "LIKE '%%' 会匹配全部数据", dbmodel.SQLReviewLevelWarn, "请移除无意义LIKE条件或改为精确条件", ""))
	} else if hasLeadingWildcardLike(sqlText) {
		findings = append(findings, reviewFinding(0, "query_warn_leading_wildcard_like", "LIKE前缀通配符可能导致索引失效", dbmodel.SQLReviewLevelWarn, "避免 LIKE '%abc' 或 LIKE '%abc%'，优先使用前缀匹配或搜索索引", ""))
	}
	if hasWhereOR(sqlText) {
		findings = append(findings, reviewFinding(0, "query_warn_or_condition", "WHERE包含OR条件，可能导致索引利用率下降", dbmodel.SQLReviewLevelWarn, "请确认执行计划，必要时拆分为UNION ALL或使用更精确条件", ""))
	}
	if hasFunctionInWhere(sqlText) {
		findings = append(findings, reviewFinding(0, "query_warn_function_in_where", "WHERE中对字段使用函数可能导致索引失效", dbmodel.SQLReviewLevelWarn, "请改写为范围条件，例如避免 DATE(created_at)=...，改用 created_at >= ... AND created_at < ...", ""))
	}
	if hasOrderByRandom(sqlText, dbType) {
		findings = append(findings, reviewFinding(0, "query_warn_order_by_random", "ORDER BY RAND()/RANDOM()可能拖垮大表", dbmodel.SQLReviewLevelWarn, "请使用采样表、预生成随机值或按主键范围随机", ""))
	}
	if hasLockingRead(sqlText, dbType) {
		findings = append(findings, reviewFinding(0, "query_forbid_locking_read", "查询工单禁止提交锁定读SQL", dbmodel.SQLReviewLevelError, "请移除 FOR UPDATE / FOR SHARE / LOCK IN SHARE MODE，查询工单必须保持只读无锁", ""))
	}
	return findings
}

func reviewEngineCompatibility(dbType int, ticketType, stmtType, sqlText string) []dbmodel.SQLReviewResult {
	switch dbType {
	case cmdbmodel.DatabaseTypePostgreSQL:
		return reviewPostgreSQLCompatibility(ticketType, stmtType, sqlText)
	default:
		return nil
	}
}

func reviewPostgreSQLCompatibility(ticketType, stmtType, sqlText string) []dbmodel.SQLReviewResult {
	findings := make([]dbmodel.SQLReviewResult, 0)
	if stmtType == "replace" {
		findings = append(findings, reviewFinding(0, "pgsql_replace_unsupported", "PostgreSQL不支持REPLACE语句", dbmodel.SQLReviewLevelError, "请改用 INSERT ... ON CONFLICT ... DO UPDATE/DO NOTHING", ""))
	}
	if hasBacktickOutsideString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_backtick_identifier", "PostgreSQL不支持MySQL反引号标识符", dbmodel.SQLReviewLevelError, "请使用双引号标识符或普通小写对象名，例如 \"users\" 或 users", ""))
	}
	if regexp.MustCompile(`(?i)\bauto_increment\b`).MatchString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_auto_increment_unsupported", "PostgreSQL不支持AUTO_INCREMENT", dbmodel.SQLReviewLevelError, "请改用 GENERATED BY DEFAULT AS IDENTITY 或 serial/bigserial", ""))
	}
	if regexp.MustCompile(`(?i)\bengine\s*=`).MatchString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_engine_option_unsupported", "PostgreSQL不支持ENGINE表选项", dbmodel.SQLReviewLevelError, "请移除 ENGINE=InnoDB 等MySQL表选项", ""))
	}
	if regexp.MustCompile(`(?i)\bdefault\s+charset\b|\bcharset\s*=`).MatchString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_charset_option_unsupported", "PostgreSQL不支持MySQL CHARSET表选项", dbmodel.SQLReviewLevelError, "请移除 DEFAULT CHARSET/CHARSET= 选项，字符集由数据库编码控制", ""))
	}
	if regexp.MustCompile(`(?i)\b(first|after)\s+["\w` + "`" + `]+`).MatchString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_column_position_unsupported", "PostgreSQL不支持FIRST/AFTER调整字段位置", dbmodel.SQLReviewLevelWarn, "请移除字段位置语法；PostgreSQL通常不依赖物理字段顺序", ""))
	}
	if regexp.MustCompile(`(?i)\b(tinyint|mediumint|unsigned)\b`).MatchString(sqlText) {
		findings = append(findings, reviewFinding(0, "pgsql_mysql_type_hint", "SQL包含MySQL专属或差异较大的字段类型", dbmodel.SQLReviewLevelWarn, "请确认类型映射，例如 tinyint改smallint，unsigned需用约束或更大整数类型替代", ""))
	}
	if ticketType == dbmodel.SQLTicketTypeDDL && stmtType == "alter" {
		findings = append(findings, reviewFinding(0, "pgsql_online_ddl_tool", "PostgreSQL大表DDL建议使用pgroll在线迁移", dbmodel.SQLReviewLevelWarn, "请在数据库工具箱检查pgroll状态；普通小表DDL可按审批流程执行", ""))
	}
	return findings
}

func hasBacktickOutsideString(sqlText string) bool {
	var quote rune
	escaped := false
	for _, r := range sqlText {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '`' {
			return true
		}
	}
	return false
}

func normalizedReviewSQL(sqlText string) string {
	return strings.ToLower(stripSQLCommentsAndStrings(sqlText))
}

func stripSQLCommentsAndStrings(sqlText string) string {
	var builder strings.Builder
	var quote rune
	escaped := false
	for i := 0; i < len(sqlText); i++ {
		ch := rune(sqlText[i])
		if quote != 0 {
			if escaped {
				escaped = false
				builder.WriteRune(' ')
				continue
			}
			if ch == '\\' {
				escaped = true
				builder.WriteRune(' ')
				continue
			}
			if ch == quote {
				quote = 0
			}
			builder.WriteRune(' ')
			continue
		}
		if ch == '\'' || ch == '"' || ch == '`' {
			quote = ch
			builder.WriteRune(' ')
			continue
		}
		if ch == '-' && i+1 < len(sqlText) && sqlText[i+1] == '-' {
			for i < len(sqlText) && sqlText[i] != '\n' {
				builder.WriteRune(' ')
				i++
			}
			if i < len(sqlText) {
				builder.WriteByte(sqlText[i])
			}
			continue
		}
		if ch == '#' {
			for i < len(sqlText) && sqlText[i] != '\n' {
				builder.WriteRune(' ')
				i++
			}
			if i < len(sqlText) {
				builder.WriteByte(sqlText[i])
			}
			continue
		}
		if ch == '/' && i+1 < len(sqlText) && sqlText[i+1] == '*' {
			builder.WriteString("  ")
			i += 2
			for i+1 < len(sqlText) && !(sqlText[i] == '*' && sqlText[i+1] == '/') {
				builder.WriteRune(' ')
				i++
			}
			if i+1 < len(sqlText) {
				builder.WriteString("  ")
				i++
			}
			continue
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}

func insertHasColumnList(sqlText string) bool {
	normalized := normalizedReviewSQL(sqlText)
	intoIndex := regexp.MustCompile(`(?is)\binsert\s+into\b|\breplace\s+into\b`).FindStringIndex(normalized)
	if intoIndex == nil {
		return true
	}
	rest := strings.TrimSpace(normalized[intoIndex[1]:])
	valuesIndex := regexp.MustCompile(`(?is)\b(values|select)\b`).FindStringIndex(rest)
	if valuesIndex == nil {
		return true
	}
	targetPart := rest[:valuesIndex[0]]
	return strings.Contains(targetPart, "(")
}

func hasAlwaysTrueWhere(sqlText string) bool {
	normalized := normalizedReviewSQL(sqlText)
	patterns := []string{
		`(?is)\bwhere\b\s*\(?\s*(?:1\s*=\s*1|true)\b`,
		`(?is)\bor\s+\(?\s*(?:1\s*=\s*1|true)\b`,
	}
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(normalized) {
			return true
		}
	}
	return false
}

func hasWhereOR(sqlText string) bool {
	whereClause := reviewWhereClause(sqlText)
	return containsWord(whereClause, "or")
}

func hasJoinInDML(sqlText string) bool {
	normalized := normalizedReviewSQL(sqlText)
	return regexp.MustCompile(`(?is)\b(update|delete)\b.*\bjoin\b`).MatchString(normalized)
}

func hasSelectStar(sqlText string) bool {
	normalized := normalizedReviewSQL(sqlText)
	return regexp.MustCompile(`(?is)\bselect\s+(?:distinct\s+)?(?:\*|[a-zA-Z0-9_$]+\.\*)\s+\bfrom\b`).MatchString(normalized)
}

func hasLeadingWildcardLike(sqlText string) bool {
	return regexp.MustCompile(`(?is)\blike\s+(['"])\s*%`).MatchString(sqlText)
}

func hasFullWildcardLike(sqlText string) bool {
	return regexp.MustCompile(`(?is)\blike\s+'\s*%\s*'`).MatchString(sqlText) ||
		regexp.MustCompile(`(?is)\blike\s+"\s*%\s*"`).MatchString(sqlText)
}

func hasFunctionInWhere(sqlText string) bool {
	whereClause := reviewWhereClause(sqlText)
	if whereClause == "" {
		return false
	}
	return regexp.MustCompile(`(?is)\b(lower|upper|date|year|month|day|substr|substring|coalesce|ifnull|cast|convert)\s*\(`).MatchString(whereClause)
}

func hasOrderByRandom(sqlText string, dbType int) bool {
	normalized := normalizedReviewSQL(sqlText)
	if dbType == cmdbmodel.DatabaseTypePostgreSQL {
		return regexp.MustCompile(`(?is)\border\s+by\b.*\brandom\s*\(`).MatchString(normalized)
	}
	return regexp.MustCompile(`(?is)\border\s+by\b.*\brand\s*\(`).MatchString(normalized)
}

func hasLockingRead(sqlText string, dbType int) bool {
	normalized := normalizedReviewSQL(sqlText)
	if regexp.MustCompile(`(?is)\bfor\s+update\b|\bfor\s+share\b`).MatchString(normalized) {
		return true
	}
	if dbType == cmdbmodel.DatabaseTypeMySQL && regexp.MustCompile(`(?is)\block\s+in\s+share\s+mode\b`).MatchString(normalized) {
		return true
	}
	return false
}

func reviewWhereClause(sqlText string) string {
	normalized := normalizedReviewSQL(sqlText)
	whereIndex := regexp.MustCompile(`(?is)\bwhere\b`).FindStringIndex(normalized)
	if whereIndex == nil {
		return ""
	}
	whereClause := normalized[whereIndex[1]:]
	if endIndex := regexp.MustCompile(`(?is)\b(group\s+by|order\s+by|having|limit|offset|returning)\b`).FindStringIndex(whereClause); endIndex != nil {
		whereClause = whereClause[:endIndex[0]]
	}
	return whereClause
}

func reviewFinding(statementID uint, code, message, level, suggestion, ruleName string) dbmodel.SQLReviewResult {
	if ruleName == "" {
		ruleName = code
	}
	return dbmodel.SQLReviewResult{
		StatementID: statementID,
		RuleCode:    code,
		RuleName:    ruleName,
		Level:       level,
		Message:     message,
		Suggestion:  suggestion,
		CreatedAt:   util.HTime{Time: time.Now()},
	}
}

func reviewSQLSyntax(stmtType, sqlText string) []dbmodel.SQLReviewResult {
	sqlText = normalizeSQLText(sqlText)
	findings := make([]dbmodel.SQLReviewResult, 0)
	if sqlText == "" {
		return append(findings, reviewFinding(0, "sql_empty", "SQL不能为空", dbmodel.SQLReviewLevelError, "请填写需要审核的SQL", ""))
	}
	if message, suggestion := detectFullWidthSQLPunctuation(sqlText); message != "" {
		findings = append(findings, reviewFinding(0, "syntax_fullwidth_punctuation", message, dbmodel.SQLReviewLevelError, suggestion, ""))
	}
	if err := validateSQLBalance(sqlText); err != nil {
		findings = append(findings, reviewFinding(0, "syntax_invalid", "SQL语法格式不完整: "+err.Error(), dbmodel.SQLReviewLevelError, "请补齐括号或引号后重新检测", ""))
	}
	if stmtType == "unknown" {
		findings = append(findings, reviewFinding(0, "syntax_unknown_statement", "无法识别SQL语句类型", dbmodel.SQLReviewLevelError, buildUnknownSQLSuggestion(sqlText), ""))
		return findings
	}
	if msg, suggestion := validateStatementShape(stmtType, sqlText); msg != "" {
		findings = append(findings, reviewFinding(0, "syntax_invalid", msg, dbmodel.SQLReviewLevelError, suggestion, ""))
	}
	return findings
}

func detectFullWidthSQLPunctuation(sqlText string) (string, string) {
	replacements := map[rune]string{
		'\uFF0C': ",",
		'\uFF1B': ";",
		'\uFF1A': ":",
		'\uFF08': "(",
		'\uFF09': ")",
		'\u201C': "\"",
		'\u201D': "\"",
		'\u2018': "'",
		'\u2019': "'",
	}
	seen := make([]string, 0)
	used := map[rune]struct{}{}
	var quote rune
	escaped := false
	for _, r := range sqlText {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		replacement, ok := replacements[r]
		if !ok {
			continue
		}
		if _, exists := used[r]; exists {
			continue
		}
		used[r] = struct{}{}
		seen = append(seen, fmt.Sprintf("%c -> %s", r, replacement))
	}
	if len(seen) == 0 {
		return "", ""
	}
	return "SQL包含中文/全角符号，数据库可能无法正确解析", "请替换为英文半角符号: " + strings.Join(seen, ", ")
}

func validateSQLBalance(sqlText string) error {
	var quote rune
	escaped := false
	parentheses := 0
	for _, r := range sqlText {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		switch r {
		case '\'', '"', '`':
			quote = r
		case '(':
			parentheses++
		case ')':
			parentheses--
			if parentheses < 0 {
				return fmt.Errorf("右括号多于左括号")
			}
		}
	}
	if quote != 0 {
		return fmt.Errorf("引号未闭合")
	}
	if parentheses > 0 {
		return fmt.Errorf("括号未闭合")
	}
	return nil
}

func validateStatementShape(stmtType, sqlText string) (string, string) {
	switch stmtType {
	case "insert", "replace":
		if !containsWord(sqlText, "into") {
			return "INSERT/REPLACE语句缺少INTO关键字", "示例: INSERT INTO table_name (col) VALUES (value)"
		}
		if !containsWord(sqlText, "values") && !containsWord(sqlText, "select") {
			return "INSERT/REPLACE语句缺少VALUES或SELECT数据来源", "请使用 VALUES (...) 或 INSERT INTO ... SELECT ..."
		}
	case "update":
		if !containsWord(sqlText, "set") {
			return "UPDATE语句缺少SET关键字", "示例: UPDATE table_name SET col = value WHERE id = 1"
		}
	case "delete":
		if !containsWord(sqlText, "from") {
			return "DELETE语句缺少FROM关键字", "示例: DELETE FROM table_name WHERE id = 1"
		}
	case "select":
		if !containsWord(sqlText, "from") {
			return "SELECT语句缺少FROM关键字", "示例: SELECT col FROM table_name LIMIT 100"
		}
	case "create_database":
		if extractCreateDatabaseName(sqlText) == "" {
			return "CREATE DATABASE语句缺少数据库名", "示例: CREATE DATABASE db_name"
		}
	case "create_table":
		if extractObjectName(sqlText) == "" {
			return "CREATE TABLE语句缺少表名", "示例: CREATE TABLE table_name (...)"
		}
		if !strings.Contains(sqlText, "(") || !strings.Contains(sqlText, ")") {
			return "CREATE TABLE语句缺少字段定义括号", "请补充字段定义，例如 CREATE TABLE table_name (id BIGINT PRIMARY KEY)"
		}
	case "alter":
		if !containsWord(sqlText, "table") {
			return "ALTER语句缺少TABLE关键字", "示例: ALTER TABLE table_name ADD COLUMN col INT"
		}
	}
	return "", ""
}

func buildUnknownSQLSuggestion(sqlText string) string {
	first := firstSQLToken(sqlText)
	switch strings.ToLower(first) {
	case "cretate", "craete", "ceate":
		return "可能想输入 CREATE，示例: CREATE DATABASE db_name"
	case "udpate", "upate":
		return "可能想输入 UPDATE，示例: UPDATE table_name SET col = value WHERE id = 1"
	case "delect", "detete":
		return "可能想输入 DELETE，示例: DELETE FROM table_name WHERE id = 1"
	case "instert", "isnert":
		return "可能想输入 INSERT，示例: INSERT INTO table_name (col) VALUES (value)"
	case "selet", "sleect":
		return "可能想输入 SELECT，示例: SELECT col FROM table_name LIMIT 100"
	default:
		return "请检查SQL关键字拼写，或确认当前数据库类型是否支持该语句"
	}
}

func firstSQLToken(sqlText string) string {
	fields := strings.Fields(strings.TrimSpace(sqlText))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "`'\"")
}

func estimateMySQLDMLRows(ctx context.Context, cfg ConnConfig, sqlText, defaultDatabase string) int64 {
	parsed, err := parseMySQLDMLStatement(sqlText, defaultDatabase)
	if err != nil || !parsed.Backupable {
		return 0
	}
	db, err := openMySQL(cfg, "")
	if err != nil {
		return 0
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0
	}
	defer conn.Close()
	if parsed.DatabaseName != "" {
		if err := validateIdentifier(parsed.DatabaseName); err != nil {
			return 0
		}
		if _, err := conn.ExecContext(ctx, "USE "+quoteMySQLIdentifier(parsed.DatabaseName)); err != nil {
			return 0
		}
	}
	tableName := qualifyMySQLTable(parsed.DatabaseName, parsed.TableName)
	query := fmt.Sprintf("EXPLAIN SELECT * FROM %s WHERE %s%s", tableName, parsed.WhereClause, parsed.TailClause)
	return estimateRowsFromExplain(ctx, conn, query)
}

func estimateRowsFromExplain(ctx context.Context, queryer sqlQueryer, query string) int64 {
	rows, err := queryer.QueryContext(ctx, query)
	if err != nil {
		return 0
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return 0
	}
	rowsIndex := -1
	for i, name := range columns {
		if strings.EqualFold(name, "rows") {
			rowsIndex = i
			break
		}
	}
	if rowsIndex < 0 {
		return 0
	}
	var estimatedRows int64
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}
		estimatedRows += toInt64(values[rowsIndex])
	}
	return estimatedRows
}

func buildDMLCheckRollbackSQL(defaultDatabase, sqlText string) string {
	parsed, err := parseMySQLDMLStatement(sqlText, defaultDatabase)
	if err != nil {
		return ""
	}
	tableName := qualifyMySQLTable(parsed.DatabaseName, parsed.TableName)
	switch parsed.StatementType {
	case "update":
		return fmt.Sprintf("-- UPDATE 回滚建议: 使用 my2sql 基于 ROW Binlog 生成修改前镜像的反向 SQL。\n-- 如果未启用 Binlog 闪回，请在执行前保留受影响行快照。\n-- 备份示例: CREATE TABLE _aiops_backup_xxx LIKE %s; INSERT INTO _aiops_backup_xxx SELECT * FROM %s WHERE %s%s;", tableName, tableName, parsed.WhereClause, parsed.TailClause)
	case "delete":
		return fmt.Sprintf("-- DELETE 回滚建议: 使用 my2sql 基于 ROW Binlog 生成 INSERT 反向 SQL。\n-- 如果未启用 Binlog 闪回，请在执行前保留受影响行快照。\n-- 恢复示例: INSERT INTO %s SELECT * FROM _aiops_backup_xxx;", tableName)
	case "insert":
		return fmt.Sprintf("-- INSERT 回滚建议: 生成 DELETE 反向 SQL 时必须使用主键或唯一键限定范围，避免误删相似数据。\n-- 示例: DELETE FROM %s WHERE id BETWEEN <first_insert_id> AND <last_insert_id>;", tableName)
	case "replace":
		return "-- REPLACE 可能覆盖已有数据，必须人工确认回滚方案，并使用主键或唯一键条件限定回滚范围。"
	default:
		return ""
	}
}

type ghOstAlterPlan struct {
	Database string
	Table    string
	Alter    string
}

func firstStatementID(statements []dbmodel.SQLStatement) uint {
	if len(statements) == 0 {
		return 0
	}
	return statements[0].ID
}

func buildGhOstScript(cfg ConnConfig, rawSQL string) (string, error) {
	plans, err := parseGhOstAlterPlans(cfg.DatabaseName, rawSQL)
	if err != nil {
		return "", err
	}
	binaryPath := resolveConfiguredToolPath("AIOPS_GHOST_PATH", ghOstBinaryPath)
	host := firstNonEmpty(cfg.Host, cfg.Endpoint, "127.0.0.1")
	port := cfg.Port
	lines := make([]string, 0, len(plans)*16)
	for i, plan := range plans {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines,
			binaryPath+"  \\",
			"--initially-drop-socket-file \\",
			fmt.Sprintf("--host=%s \\", shellQuote(host)),
			fmt.Sprintf("--port=%d \\", port),
			fmt.Sprintf("--user=%s \\", shellQuote(cfg.Username)),
			fmt.Sprintf("--password=%s \\", shellQuote(cfg.Password)),
			fmt.Sprintf("--database=%s \\", shellQuote(plan.Database)),
			fmt.Sprintf("--table=%s \\", shellQuote(plan.Table)),
			"--verbose \\",
			"--chunk-size=80000 \\",
			"--max-load=Threads_running=64 \\",
			fmt.Sprintf("--alter=%s \\", shellQuote(plan.Alter)),
			"--panic-flag-file=/tmp/ghost.panic.flag \\",
			"--allow-on-master \\",
			"--timestamp-old-table \\",
			"--execute \\",
			"--initially-drop-ghost-table",
		)
	}
	return strings.Join(lines, "\n"), nil
}

func parseGhOstAlterPlans(defaultDatabase, rawSQL string) ([]ghOstAlterPlan, error) {
	statements := splitSQLStatements(rawSQL)
	if len(statements) == 0 {
		return nil, fmt.Errorf("gh-ost SQL不能为空")
	}
	grouped := map[string]*ghOstAlterPlan{}
	order := make([]string, 0)
	for _, statement := range statements {
		plan, err := parseGhOstAlterStatement(defaultDatabase, statement)
		if err != nil {
			return nil, err
		}
		key := plan.Database + "." + plan.Table
		if existing, ok := grouped[key]; ok {
			existing.Alter += ",\n" + plan.Alter
			continue
		}
		grouped[key] = &plan
		order = append(order, key)
	}
	plans := make([]ghOstAlterPlan, 0, len(order))
	for _, key := range order {
		plans = append(plans, *grouped[key])
	}
	return plans, nil
}

func parseGhOstAlterStatement(defaultDatabase, sqlText string) (ghOstAlterPlan, error) {
	sqlText = normalizeSQLText(sqlText)
	tableName := extractAlterTableName(sqlText)
	if tableName == "" {
		return ghOstAlterPlan{}, fmt.Errorf("gh-ost仅支持ALTER TABLE语句")
	}
	databaseName, tableNameOnly, err := splitMySQLTableName(tableName, defaultDatabase)
	if err != nil {
		return ghOstAlterPlan{}, err
	}
	re := regexp.MustCompile(`(?is)^\s*alter\s+table\s+` + regexp.QuoteMeta(tableName) + `\s+(.+)$`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		return ghOstAlterPlan{}, fmt.Errorf("ALTER TABLE缺少变更表达式")
	}
	return ghOstAlterPlan{
		Database: databaseName,
		Table:    tableNameOnly,
		Alter:    strings.TrimSpace(matches[1]),
	}, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func executeGhOstTicketStatement(ctx *gin.Context, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	script := strings.TrimSpace(ticket.GhOstScript)
	if script == "" {
		return nil, fmt.Errorf("gh-ost脚本为空")
	}
	start := time.Now()
	cmd := shellCommandContext(contextFromGin(ctx), script)
	output, err := cmd.CombinedOutput()
	execLog := string(output)
	durationMS := time.Since(start).Milliseconds()
	if err != nil {
		return &reviewExecuteResult{DurationMS: durationMS, ExecLog: execLog}, fmt.Errorf("gh-ost执行失败: %w", err)
	}
	if !isGhOstLogSucceeded(execLog) {
		return &reviewExecuteResult{DurationMS: durationMS, ExecLog: execLog}, fmt.Errorf("gh-ost执行未成功，需满足退出码为0、最后有效行包含Done且日志无关键错误")
	}
	return &reviewExecuteResult{DurationMS: durationMS, ExecLog: execLog}, nil
}

func shellCommandContext(ctx context.Context, script string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	}
	return exec.CommandContext(ctx, "sh", "-c", script)
}

func runMy2SQLFlashback(ctx *gin.Context, cfg ConnConfig, record dbmodel.SQLRollbackRecord) (string, error) {
	if cfg.Host == "" || cfg.Username == "" {
		return "", fmt.Errorf("数据库连接信息不完整，无法调用my2sql")
	}
	binaryPath, err := resolveMy2SQLBinaryPath()
	if err != nil {
		return "", err
	}
	args := []string{
		"-host", cfg.Host,
		"-port", fmt.Sprint(cfg.Port),
		"-user", cfg.Username,
		"-password", cfg.Password,
		"-work-type", "rollback",
		"-start-file", record.BinlogStartFile,
		"-start-pos", fmt.Sprint(record.BinlogStartPos),
		"-stop-file", record.BinlogEndFile,
		"-stop-pos", fmt.Sprint(record.BinlogEndPos),
		"-databases", record.OriginalDatabase,
		"-tables", record.OriginalTable,
		"-sql", strings.ToUpper(record.StatementType),
		"-full-columns",
		"-output-toScreen",
	}
	cmd := exec.CommandContext(contextFromGin(ctx), binaryPath, args...)
	output, err := cmd.CombinedOutput()
	outputText := string(output)
	if err != nil {
		return "", fmt.Errorf("调用my2sql失败: %w; %s", err, strings.TrimSpace(outputText))
	}
	rollbackSQL := extractMy2SQLRollbackSQL(outputText)
	if rollbackSQL == "" {
		return "", fmt.Errorf("my2sql未输出回滚SQL，请确认binlog未被清理且位点范围正确")
	}
	return rollbackSQL, nil
}

func buildMy2SQLInstallInstruction() string {
	return strings.Join([]string{
		"mkdir -p /opt/db",
		"cd /opt/db",
		"wget -O my2sql <请替换为生产确认的my2sql Linux amd64下载地址>",
		"chmod +x /opt/db/my2sql",
		"/opt/db/my2sql -help",
	}, "\n")
}

func buildGhOstInstallInstruction() string {
	return strings.Join([]string{
		"mkdir -p /opt/db",
		"cd /opt/db",
		"wget -O gh-ost.tar.gz <请替换为生产确认的gh-ost Linux amd64下载地址>",
		"tar -zxf gh-ost.tar.gz",
		"mv gh-ost /opt/db/gh-ost",
		"chmod +x /opt/db/gh-ost",
		"/opt/db/gh-ost --help",
	}, "\n")
}

func buildPgRollInstallInstruction() string {
	return strings.Join([]string{
		"mkdir -p /opt/db",
		"cd /opt/db",
		"wget -O pgroll <请替换为生产确认的pgroll Linux amd64下载地址>",
		"chmod +x /opt/db/pgroll",
		"/opt/db/pgroll --help",
	}, "\n")
}

func buildWalMinerInstallInstruction() string {
	return strings.Join([]string{
		"mkdir -p /opt/db",
		"cd /opt/db",
		"wget -O walminer <请替换为生产确认的WalMiner命令行工具或安装包地址>",
		"chmod +x /opt/db/walminer",
		"/opt/db/walminer --help",
		"# 注意：WalMiner通常还需要在PostgreSQL侧安装扩展并按版本准备WAL解析环境",
	}, "\n")
}

func resolveConfiguredToolPath(envName, defaultPath string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	return defaultPath
}

type externalDBToolConfig struct {
	ToolName           string
	DatabaseType       int
	DatabaseTypeName   string
	Capability         string
	EnvName            string
	InstallPath        string
	UsageScenario      string
	DownloadURL        string
	InstallInstruction string
	HelpArgs           []string
	CapabilityOnOK     bool
}

func checkExternalDBTool(ctx context.Context, cfg externalDBToolConfig) dbmodel.DBToolStatus {
	envPath := strings.TrimSpace(os.Getenv(cfg.EnvName))
	resolvedPath := cfg.InstallPath
	if envPath != "" {
		resolvedPath = envPath
	}
	checkArgs := cfg.HelpArgs
	if len(checkArgs) == 0 {
		checkArgs = []string{"--help"}
	}
	status := dbmodel.DBToolStatus{
		ToolName:           cfg.ToolName,
		DatabaseType:       cfg.DatabaseType,
		DatabaseTypeName:   cfg.DatabaseTypeName,
		Capability:         cfg.Capability,
		Status:             "not_installed",
		StatusText:         "未安装",
		InstallPath:        cfg.InstallPath,
		ResolvedPath:       resolvedPath,
		EnvPath:            envPath,
		UsageScenario:      cfg.UsageScenario,
		CheckButtonText:    "重新检测",
		InstallCommand:     cfg.InstallInstruction,
		URL:                cfg.DownloadURL,
		DownloadURL:        cfg.DownloadURL,
		CheckCommand:       strings.TrimSpace(resolvedPath + " " + strings.Join(checkArgs, " ")),
		InstallInstruction: cfg.InstallInstruction,
		LastCheckMessage:   fmt.Sprintf("未检测到%s，请先在执行节点安装 %s 后重试", cfg.ToolName, cfg.InstallPath),
	}
	if _, err := os.Stat(resolvedPath); err != nil {
		if envPath != "" {
			status.LastCheckMessage = fmt.Sprintf("未检测到%s，请确认 %s 指向的文件存在", cfg.ToolName, cfg.EnvName)
		}
		return status
	}

	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, resolvedPath, checkArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		status.Status = "abnormal"
		status.StatusText = "异常"
		status.LastCheckMessage = fmt.Sprintf("%s文件存在但执行校验失败: %s", cfg.ToolName, strings.TrimSpace(string(output)))
		if strings.HasSuffix(status.LastCheckMessage, ": ") {
			status.LastCheckMessage = fmt.Sprintf("%s文件存在但执行校验失败: %s", cfg.ToolName, err.Error())
		}
		return status
	}
	status.Status = "installed"
	status.StatusText = "已安装"
	status.LastCheckMessage = cfg.ToolName + "已安装并可执行"
	status.CanGenerateRollback = cfg.CapabilityOnOK
	return status
}

func resolveMy2SQLBinaryPath() (string, error) {
	candidates := []string{}
	if value := strings.TrimSpace(os.Getenv("AIOPS_MY2SQL_PATH")); value != "" {
		candidates = append(candidates, value)
	}
	candidates = append(candidates,
		defaultMy2SQLBinaryPath,
	)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未检测到my2sql，请先在执行节点安装 /opt/db/my2sql 后重试")
}

func extractMy2SQLRollbackSQL(output string) string {
	lines := strings.Split(output, "\n")
	sqlLines := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "INSERT") || strings.HasPrefix(upper, "UPDATE") || strings.HasPrefix(upper, "DELETE") || strings.HasPrefix(upper, "REPLACE") {
			if !strings.HasSuffix(line, ";") {
				line += ";"
			}
			sqlLines = append(sqlLines, line)
		}
	}
	return strings.Join(sqlLines, "\n")
}

func isGhOstLogSucceeded(logText string) bool {
	lines := strings.Split(strings.TrimSpace(logText), "\n")
	last := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			last = line
			break
		}
	}
	if last == "" || !strings.Contains(last, "Done") {
		return false
	}
	return !regexp.MustCompile(`(?i)\b(fatal|panic|cannot migrate|failed|bailing out)\b|unable to alter ghost table`).MatchString(logText)
}

func selectRollbackSQL(records []dbmodel.SQLRollbackRecord, rollbackRecordID uint) (string, error) {
	for _, record := range records {
		if rollbackRecordID > 0 && record.ID != rollbackRecordID {
			continue
		}
		if !record.Available {
			continue
		}
		rollbackSQL := strings.TrimSpace(record.RollbackSQL)
		if rollbackSQL == "" {
			continue
		}
		return rollbackSQL, nil
	}
	if rollbackRecordID > 0 {
		return "", fmt.Errorf("未找到指定的回滚SQL记录")
	}
	return "", fmt.Errorf("未找到可用的回滚SQL")
}

func selectFlashbackRollbackRecord(records []dbmodel.SQLRollbackRecord, rollbackRecordID uint) (dbmodel.SQLRollbackRecord, error) {
	for _, record := range records {
		if rollbackRecordID > 0 && record.ID != rollbackRecordID {
			continue
		}
		if record.RollbackType != "binlog_flashback" {
			continue
		}
		return record, nil
	}
	if rollbackRecordID > 0 {
		return dbmodel.SQLRollbackRecord{}, fmt.Errorf("未找到指定的binlog闪回记录")
	}
	return dbmodel.SQLRollbackRecord{}, fmt.Errorf("未找到可生成的binlog闪回记录")
}

func rollbackTicketType(sqlText string) string {
	statements := splitSQLStatements(sqlText)
	if len(statements) == 0 {
		return ""
	}
	hasDDL := false
	hasDML := false
	for _, statement := range statements {
		stmtType := detectSQLType(statement)
		if isDDLType(stmtType) {
			hasDDL = true
		} else if isDMLType(stmtType) {
			hasDML = true
		} else {
			return ""
		}
	}
	if hasDDL && hasDML {
		return ""
	}
	if hasDDL {
		return dbmodel.SQLTicketTypeDDL
	}
	if hasDML {
		return dbmodel.SQLTicketTypeDML
	}
	return ""
}

type mysqlBackupTableRef struct {
	Database string
	Table    string
}

func (r mysqlBackupTableRef) String() string {
	return fmt.Sprintf("%s.%s", r.Database, r.Table)
}

func collectBackupTables(records []dbmodel.SQLRollbackRecord) []mysqlBackupTableRef {
	refs := make([]mysqlBackupTableRef, 0)
	seen := map[string]struct{}{}
	for _, record := range records {
		databaseName := strings.TrimSpace(record.BackupDatabase)
		tableName := strings.TrimSpace(record.BackupTable)
		if databaseName == "" || tableName == "" {
			continue
		}
		if !strings.HasPrefix(tableName, "_aiops_backup_") {
			continue
		}
		ref := mysqlBackupTableRef{Database: databaseName, Table: tableName}
		key := ref.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, ref)
	}
	return refs
}

func loadMySQLTableKeyMetadata(ctx context.Context, cfg ConnConfig, databaseName, tableName string) ([]dbmodel.ColumnInfo, []dbmodel.IndexInfo, error) {
	driver := NewMySQLDriver()
	req := dbmodel.ObjectRequest{
		AssetID:    cfg.AssetID,
		Database:   databaseName,
		ObjectName: tableName,
		ObjectType: dbmodel.ObjectTypeTable,
	}
	columns, err := driver.Columns(ctx, cfg, req)
	if err != nil {
		return nil, nil, err
	}
	indexes, err := driver.Indexes(ctx, cfg, req)
	if err != nil {
		return columns, nil, err
	}
	return columns, indexes, nil
}

func buildUniqueKeyGroups(columns []dbmodel.ColumnInfo, indexes []dbmodel.IndexInfo) [][]string {
	groups := make([][]string, 0)
	primaryColumns := make([]string, 0)
	for _, column := range columns {
		if column.PrimaryKey {
			primaryColumns = append(primaryColumns, column.Name)
		}
	}
	if len(primaryColumns) > 0 {
		groups = append(groups, primaryColumns)
	}
	for _, index := range indexes {
		if !index.Unique || len(index.Columns) == 0 {
			continue
		}
		groups = append(groups, index.Columns)
	}
	return groups
}

func whereContainsAnyKeyGroup(whereClause string, keyGroups [][]string) bool {
	for _, group := range keyGroups {
		if len(group) == 0 {
			continue
		}
		matched := true
		for _, column := range group {
			if !whereContainsColumnPredicate(whereClause, column) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func whereContainsColumnPredicate(whereClause, column string) bool {
	column = strings.TrimSpace(column)
	if column == "" {
		return false
	}
	pattern := `(?is)(?:^|[^A-Za-z0-9_@$])(?:` + "`" + regexp.QuoteMeta(column) + "`" + `|` + regexp.QuoteMeta(column) + `)\s*(?:=|in\s*\(|<=>)`
	return regexp.MustCompile(pattern).MatchString(whereClause)
}

func buildInitialRollbackRecords(ticketType string, statements []dbmodel.SQLStatement, manualRollbackSQL string, now util.HTime) []dbmodel.SQLRollbackRecord {
	records := make([]dbmodel.SQLRollbackRecord, 0)
	manualRollbackSQL = strings.TrimSpace(manualRollbackSQL)
	if manualRollbackSQL != "" {
		records = append(records, dbmodel.SQLRollbackRecord{
			StatementID:  1,
			RollbackSQL:  manualRollbackSQL,
			RollbackType: "manual",
			Available:    true,
			Message:      "manual rollback sql",
			CreatedAt:    now,
		})
	}
	if ticketType != dbmodel.SQLTicketTypeDDL {
		return records
	}
	for _, statement := range statements {
		rollbackSQL, ok := buildDDLReverseSQL(statement.SQLText)
		if !ok {
			continue
		}
		records = append(records, dbmodel.SQLRollbackRecord{
			StatementID:   uint(statement.StatementIndex),
			StatementType: statement.StatementType,
			OriginalTable: statement.ObjectName,
			RollbackSQL:   rollbackSQL,
			RollbackType:  "ddl_reverse",
			Available:     true,
			Message:       "auto generated reverse DDL",
			CreatedAt:     now,
		})
	}
	return records
}

func resolveReviewUser(userID uint, roleName string) (uint, string, error) {
	var user systemmodel.SysAdmin
	if err := common.GetDB().First(&user, userID).Error; err != nil {
		return 0, "", fmt.Errorf("%s user not found", roleName)
	}
	if user.Status != 1 {
		return 0, "", fmt.Errorf("%s user is disabled", roleName)
	}
	displayName := strings.TrimSpace(user.Username)
	if displayName == "" {
		displayName = strings.TrimSpace(user.Nickname)
	}
	return user.ID, displayName, nil
}

func reviewContactName(name string, userID uint) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return fmt.Sprintf("%s(ID:%d)", name, userID)
	}
	if userID > 0 {
		return fmt.Sprintf("用户ID:%d", userID)
	}
	return "工单指定联系人"
}

func encodeJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseLatestQueryResult(records []dbmodel.SQLExecutionRecord) (*dbmodel.SQLExecutionRecord, []dbmodel.ColumnInfo, []map[string]interface{}, error) {
	var target *dbmodel.SQLExecutionRecord
	for i := len(records) - 1; i >= 0; i-- {
		record := &records[i]
		if record.Status == dbmodel.SQLExecutionStatusSucceeded && strings.TrimSpace(record.ResultRows) != "" {
			target = record
			break
		}
	}
	if target == nil {
		return nil, nil, nil, fmt.Errorf("未找到可查看的查询结果")
	}

	var columns []dbmodel.ColumnInfo
	if err := json.Unmarshal([]byte(target.ResultColumns), &columns); err != nil {
		return nil, nil, nil, fmt.Errorf("解析查询结果字段失败: %w", err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(target.ResultRows), &rows); err != nil {
		return nil, nil, nil, fmt.Errorf("解析查询结果数据失败: %w", err)
	}
	return target, columns, rows, nil
}

func limitRows(rows []map[string]interface{}, limit int) []map[string]interface{} {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func (s *ReviewService) rebuildQueryResult(ctx *gin.Context, detail *dbmodel.SQLTicketDetail) (*dbmodel.SQLExecutionRecord, []dbmodel.ColumnInfo, []map[string]interface{}, error) {
	if detail == nil {
		return nil, nil, nil, fmt.Errorf("工单详情为空")
	}
	if detail.Ticket.Status != dbmodel.SQLTicketStatusSucceeded {
		return nil, nil, nil, fmt.Errorf("工单尚未执行成功，暂无查询结果")
	}
	var target *dbmodel.SQLExecutionRecord
	for i := len(detail.ExecutionRecords) - 1; i >= 0; i-- {
		record := &detail.ExecutionRecords[i]
		if record.Status == dbmodel.SQLExecutionStatusSucceeded {
			target = record
			break
		}
	}
	if target == nil {
		return nil, nil, nil, fmt.Errorf("未找到成功的执行记录")
	}

	sqlText := strings.TrimSpace(target.SQLText)
	if sqlText == "" {
		sqlText = strings.TrimSpace(detail.Ticket.SQLContent)
	}
	if sqlText == "" {
		return nil, nil, nil, fmt.Errorf("执行记录缺少可查询的SQL")
	}

	data, err := s.dbService.ExecuteSQL(ctx, dbmodel.SQLExecuteRequest{
		AssetID:   detail.Ticket.AssetID,
		Database:  detail.Ticket.DatabaseName,
		Schema:    detail.Ticket.SchemaName,
		SQL:       sqlText,
		Page:      1,
		PageSize:  queryResultDownloadRows,
		Confirmed: dbmodel.SQLConfirm(true),
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("重新查询结果失败: %w", err)
	}
	if data == nil {
		return nil, nil, nil, fmt.Errorf("重新查询结果为空")
	}
	target.ResultColumns = encodeJSON(data.Columns)
	target.ResultRows = encodeJSON(data.Rows)
	if data.DurationMS > 0 {
		target.DurationMS = data.DurationMS
	}
	target.RowsAffected = int64(len(data.Rows))
	_ = s.reviewDao.UpdateExecutionRecord(target.ID, map[string]interface{}{
		"result_columns": target.ResultColumns,
		"result_rows":    target.ResultRows,
		"duration_ms":    target.DurationMS,
		"rows_affected":  target.RowsAffected,
	})
	return target, data.Columns, data.Rows, nil
}

func writeResultRowsCSV(writer io.Writer, columns []dbmodel.ColumnInfo, rows []map[string]interface{}) (int64, error) {
	csvWriter := csv.NewWriter(writer)
	columnNames := make([]string, 0, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if name != "" {
			columnNames = append(columnNames, name)
		}
	}
	if len(columnNames) == 0 && len(rows) > 0 {
		for name := range rows[0] {
			columnNames = append(columnNames, name)
		}
		sort.Strings(columnNames)
	}
	if len(columnNames) > 0 {
		if err := csvWriter.Write(columnNames); err != nil {
			return 0, err
		}
	}
	for _, row := range rows {
		record := make([]string, 0, len(columnNames))
		for _, column := range columnNames {
			record = append(record, fmt.Sprint(row[column]))
		}
		if err := csvWriter.Write(record); err != nil {
			return 0, err
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func buildDDLReverseSQL(sqlText string) (string, bool) {
	sqlText = normalizeSQLText(sqlText)
	if rollbackSQL, ok := reverseCreateDatabase(sqlText); ok {
		return rollbackSQL, true
	}
	if rollbackSQL, ok := reverseCreateTable(sqlText); ok {
		return rollbackSQL, true
	}
	if rollbackSQL, ok := reverseAlterAddColumn(sqlText); ok {
		return rollbackSQL, true
	}
	if rollbackSQL, ok := reverseAlterDropColumn(sqlText); ok {
		return rollbackSQL, true
	}
	if rollbackSQL, ok := reverseCreateIndex(sqlText); ok {
		return rollbackSQL, true
	}
	return "", false
}

func reverseCreateDatabase(sqlText string) (string, bool) {
	databaseName := extractCreateDatabaseName(sqlText)
	if databaseName == "" {
		return "", false
	}
	return fmt.Sprintf("DROP DATABASE %s;", normalizeMySQLObjectName(databaseName)), true
}

func reverseCreateTable(sqlText string) (string, bool) {
	tableName := extractCreateTableName(sqlText)
	if tableName == "" {
		return "", false
	}
	return fmt.Sprintf("DROP TABLE %s;", normalizeMySQLObjectName(tableName)), true
}

func reverseAlterAddColumn(sqlText string) (string, bool) {
	tableName := extractAlterTableName(sqlText)
	columnName := extractAlterColumnName(sqlText, "add")
	if tableName == "" || columnName == "" {
		return "", false
	}
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", normalizeMySQLObjectName(tableName), normalizeMySQLObjectName(columnName)), true
}

func reverseAlterDropColumn(sqlText string) (string, bool) {
	tableName := extractAlterTableName(sqlText)
	columnName := extractAlterColumnName(sqlText, "drop")
	if tableName == "" || columnName == "" {
		return "", false
	}
	return fmt.Sprintf("-- ALTER TABLE %s ADD COLUMN %s <column_definition>;", normalizeMySQLObjectName(tableName), normalizeMySQLObjectName(columnName)), true
}

func reverseCreateIndex(sqlText string) (string, bool) {
	re := regexp.MustCompile(`(?is)^\s*create\s+(?:unique\s+)?index\s+(` + "`" + `[^` + "`" + `]+` + "`" + `|[A-Za-z0-9_@$-]+)\s+on\s+((?:` + "`" + `[^` + "`" + `]+` + "`" + `|[A-Za-z0-9_@$-]+)(?:\s*\.\s*(?:` + "`" + `[^` + "`" + `]+` + "`" + `|[A-Za-z0-9_@$-]+))?)\s*\(`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) != 3 {
		return "", false
	}
	return fmt.Sprintf("DROP INDEX %s ON %s;", normalizeMySQLObjectName(matches[1]), normalizeMySQLObjectName(matches[2])), true
}

func normalizeMySQLObjectName(rawName string) string {
	parts := strings.Split(rawName, ".")
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "`")
		if part == "" {
			continue
		}
		quoted = append(quoted, quoteMySQLIdentifier(part))
	}
	return strings.Join(quoted, ".")
}

func splitSQLStatements(rawSQL string) []string {
	parts := strings.Split(rawSQL, ";")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func joinSQLStatements(statements []dbmodel.SQLStatement) string {
	parts := make([]string, 0, len(statements))
	for _, statement := range statements {
		sqlText := strings.TrimSpace(statement.SQLText)
		if sqlText == "" {
			continue
		}
		parts = append(parts, sqlText)
	}
	return strings.Join(parts, ";\n")
}

func ensureQueryLimit(sqlText string, limit int) string {
	sqlText = normalizeSQLText(sqlText)
	if limit <= 0 {
		limit = queryReviewDefaultLimit
	}
	if containsWord(sqlText, "limit") {
		return sqlText
	}
	return fmt.Sprintf("%s LIMIT %d", sqlText, limit)
}

func detectSQLType(sqlText string) string {
	lower := strings.ToLower(strings.TrimSpace(sqlText))
	switch {
	case strings.HasPrefix(lower, "insert"):
		return "insert"
	case strings.HasPrefix(lower, "update"):
		return "update"
	case strings.HasPrefix(lower, "delete"):
		return "delete"
	case strings.HasPrefix(lower, "replace"):
		return "replace"
	case strings.HasPrefix(lower, "select"):
		return "select"
	case strings.HasPrefix(lower, "with"):
		return "with"
	case strings.HasPrefix(lower, "create database"), strings.HasPrefix(lower, "create schema"):
		return "create_database"
	case strings.HasPrefix(lower, "create index"):
		return "create_index"
	case strings.HasPrefix(lower, "create table"):
		return "create_table"
	case strings.HasPrefix(lower, "drop index"):
		return "drop_index"
	case strings.HasPrefix(lower, "create"):
		return "create"
	case strings.HasPrefix(lower, "alter"):
		return "alter"
	case strings.HasPrefix(lower, "drop database"):
		return "drop_database"
	case strings.HasPrefix(lower, "drop table"):
		return "drop_table"
	case strings.HasPrefix(lower, "drop"):
		return "drop"
	case strings.HasPrefix(lower, "truncate"):
		return "truncate"
	case strings.HasPrefix(lower, "rename"):
		return "rename"
	default:
		return "unknown"
	}
}

func isDMLType(stmtType string) bool {
	switch stmtType {
	case "insert", "update", "delete", "replace":
		return true
	default:
		return false
	}
}

func isQueryType(stmtType string) bool {
	switch stmtType {
	case "select", "with":
		return true
	default:
		return false
	}
}

func isDDLType(stmtType string) bool {
	switch stmtType {
	case "create", "create_database", "create_table", "alter", "drop", "drop_database", "drop_table", "truncate", "rename", "create_index", "drop_index":
		return true
	default:
		return false
	}
}

func riskForSQLType(ticketType, stmtType string) string {
	switch stmtType {
	case "drop_database", "drop_table", "truncate":
		return dbmodel.SQLRiskCritical
	case "alter", "drop", "rename":
		return dbmodel.SQLRiskHigh
	case "update", "delete", "create_database":
		return dbmodel.SQLRiskMedium
	case "select", "with":
		return dbmodel.SQLRiskLow
	default:
		if ticketType == dbmodel.SQLTicketTypeDDL {
			return dbmodel.SQLRiskMedium
		}
		return dbmodel.SQLRiskLow
	}
}

func riskFromLevel(level, fallback string) string {
	switch level {
	case dbmodel.SQLReviewLevelError:
		return dbmodel.SQLRiskCritical
	case dbmodel.SQLReviewLevelWarn:
		return maxRisk(dbmodel.SQLRiskMedium, fallback)
	default:
		return fallback
	}
}

func maxRisk(a, b string) string {
	order := map[string]int{
		dbmodel.SQLRiskLow:      1,
		dbmodel.SQLRiskMedium:   2,
		dbmodel.SQLRiskHigh:     3,
		dbmodel.SQLRiskCritical: 4,
	}
	if order[b] > order[a] {
		return b
	}
	return a
}

func containsWord(value, word string) bool {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(word) + `\b`)
	return re.MatchString(value)
}

const mysqlIdentifierPatternText = "`[^`]+`|[A-Za-z0-9_@$-]+"

var mysqlObjectNamePatternText = `(?:` + mysqlIdentifierPatternText + `)(?:\s*\.\s*(?:` + mysqlIdentifierPatternText + `))?`

func extractObjectName(sqlText string) string {
	if databaseName := extractCreateDatabaseName(sqlText); databaseName != "" {
		return normalizeSQLObjectName(databaseName)
	}
	if tableName := extractCreateTableName(sqlText); tableName != "" {
		return normalizeSQLObjectName(tableName)
	}
	for _, extractor := range []func(string) string{
		extractAlterTableName,
		extractDropTableName,
		extractTruncateTableName,
		extractCreateIndexTableName,
		extractInsertTableName,
		extractUpdateTableName,
		extractDeleteTableName,
		extractSelectTableName,
	} {
		if objectName := extractor(sqlText); objectName != "" {
			return normalizeSQLObjectName(objectName)
		}
	}
	return ""
}

func normalizeSQLObjectName(rawName string) string {
	parts := strings.Split(rawName, ".")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "`")
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, ".")
}

func extractObjectNameWithPattern(sqlText, prefix, suffix string) string {
	re := regexp.MustCompile(`(?is)^\s*` + prefix + `(` + mysqlObjectNamePatternText + `)` + suffix)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) > 1 {
		return strings.ReplaceAll(strings.TrimSpace(matches[1]), " ", "")
	}
	return ""
}

func extractInsertTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `(?:insert|replace)\s+into\s+`, `\s*(?:\(|values\b|value\b|select\b|set\b|$)`)
}

func extractUpdateTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `update\s+`, `\s+set\b`)
}

func extractDeleteTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `delete\s+from\s+`, `\s+where\b`)
}

func extractAlterTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `alter\s+table\s+`, `(?:\s|$)`)
}

func extractAlterColumnName(sqlText, action string) string {
	tableName := extractAlterTableName(sqlText)
	if tableName == "" {
		return ""
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "add" && action != "drop" {
		return ""
	}
	re := regexp.MustCompile(`(?is)^\s*alter\s+table\s+` + regexp.QuoteMeta(tableName) + `\s+` + action + `\s+(?:column\s+)?(` + mysqlIdentifierPatternText + `)(?:\s|$|,|\))`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractDropTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `drop\s+table\s+(?:if\s+exists\s+)?`, `(?:\s|$)`)
}

func extractTruncateTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `truncate\s+(?:table\s+)?`, `(?:\s|$)`)
}

func extractCreateIndexTableName(sqlText string) string {
	re := regexp.MustCompile(`(?is)^\s*create\s+(?:unique\s+)?index\s+(?:` + mysqlIdentifierPatternText + `)\s+on\s+(` + mysqlObjectNamePatternText + `)\s*\(`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) > 1 {
		return strings.ReplaceAll(strings.TrimSpace(matches[1]), " ", "")
	}
	return ""
}

func extractSelectTableName(sqlText string) string {
	re := regexp.MustCompile(`(?is)\bfrom\s+(` + mysqlObjectNamePatternText + `)(?:\s|$|\)|,)`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) > 1 {
		return strings.ReplaceAll(strings.TrimSpace(matches[1]), " ", "")
	}
	return ""
}

func extractCreateTableName(sqlText string) string {
	return extractObjectNameWithPattern(sqlText, `create\s+(?:temporary\s+)?table\s+(?:if\s+not\s+exists\s+)?`, `\s*(?:\(|$)`)
}

func extractCreateDatabaseName(sqlText string) string {
	re := regexp.MustCompile(`(?is)^\s*create\s+(?:database|schema)\s+(?:if\s+not\s+exists\s+)?(` + mysqlIdentifierPatternText + `)(?:\s|$)`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func normalizeTicketType(ticketType string) string {
	switch strings.ToLower(strings.TrimSpace(ticketType)) {
	case dbmodel.SQLTicketTypeDML:
		return dbmodel.SQLTicketTypeDML
	case dbmodel.SQLTicketTypeDDL:
		return dbmodel.SQLTicketTypeDDL
	case dbmodel.SQLTicketTypeQuery:
		return dbmodel.SQLTicketTypeQuery
	default:
		return ""
	}
}

func buildTicketNo(ticketType string) string {
	return fmt.Sprintf("DB-%s-%s", strings.ToUpper(ticketType), time.Now().Format("20060102150405"))
}

func sqlFingerprint(sqlText string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.Join(strings.Fields(sqlText), " "))))
	return hex.EncodeToString(sum[:])
}

func currentAdmin(ctx *gin.Context) (uint, string) {
	admin, err := jwt.GetAdmin(ctx)
	if err != nil || admin == nil {
		return 0, ""
	}
	return admin.ID, admin.Username
}
