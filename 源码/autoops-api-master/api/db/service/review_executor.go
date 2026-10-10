package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common/util"
)

type reviewExecuteResult struct {
	RowsAffected int64
	DurationMS   int64
	Rollback     *dbmodel.SQLRollbackRecord
	Result       *dbmodel.SQLExecuteResult
	ExecLog      string
}

type parsedDMLStatement struct {
	StatementType string
	DatabaseName  string
	TableName     string
	WhereClause   string
	TailClause    string
	Backupable    bool
}

type mysqlBinlogPosition struct {
	File string
	Pos  uint64
}

func executeReviewStatement(ctx context.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	switch cfg.Engine {
	case cmdbmodel.DatabaseTypeMySQL:
		return executeMySQLReviewStatement(ctx, cfg, ticket, statement)
	case cmdbmodel.DatabaseTypePostgreSQL:
		return executePostgreSQLReviewStatement(ctx, cfg, ticket, statement)
	default:
		return nil, fmt.Errorf("database type %d review execution is not implemented yet", cfg.Engine)
	}
}

func executePostgreSQLReviewStatement(ctx context.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	if ticket.TicketType == dbmodel.SQLTicketTypeDDL {
		return executePostgreSQLDDLReviewStatement(ctx, cfg, ticket, statement)
	}
	if ticket.TicketType != dbmodel.SQLTicketTypeDML {
		return nil, fmt.Errorf("only DML and supported DDL tickets can be executed by PostgreSQL review executor")
	}
	if !isDMLType(statement.StatementType) {
		return nil, fmt.Errorf("only DML statements can be executed by PostgreSQL review executor")
	}
	db, err := openPostgres(cfg, ticket.DatabaseName)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	result, err := db.ExecContext(ctx, normalizeSQLText(statement.SQLText))
	durationMS := time.Since(start).Milliseconds()
	if err != nil {
		return &reviewExecuteResult{DurationMS: durationMS}, err
	}
	rowsAffected, _ := result.RowsAffected()
	rollback := buildPostgreSQLManualRollbackHint(ticket, statement, rowsAffected)
	return &reviewExecuteResult{RowsAffected: rowsAffected, DurationMS: durationMS, Rollback: rollback}, nil
}

func executePostgreSQLDDLReviewStatement(ctx context.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	switch statement.StatementType {
	case "create_database":
		return executePostgreSQLCreateDatabaseStatement(ctx, cfg, statement)
	case "create_table":
		return executePostgreSQLDatabaseDDLStatement(ctx, cfg, ticket, statement, `(?is)^\s*create\s+table\s+`, "仅支持标准PostgreSQL CREATE TABLE语句")
	case "alter":
		return executePostgreSQLDatabaseDDLStatement(ctx, cfg, ticket, statement, `(?is)^\s*alter\s+table\s+`, "仅支持标准PostgreSQL ALTER TABLE语句")
	case "create_index":
		return executePostgreSQLDatabaseDDLStatement(ctx, cfg, ticket, statement, `(?is)^\s*create\s+(unique\s+)?index\s+`, "仅支持标准PostgreSQL CREATE INDEX语句")
	case "drop_index":
		return executePostgreSQLDatabaseDDLStatement(ctx, cfg, ticket, statement, `(?is)^\s*drop\s+index\s+`, "仅支持标准PostgreSQL DROP INDEX语句")
	default:
		return nil, fmt.Errorf("当前仅支持执行PostgreSQL CREATE DATABASE、CREATE TABLE、ALTER TABLE、CREATE/DROP INDEX DDL；高危DDL请审核后手工执行或使用pgroll方案")
	}
}

func executePostgreSQLCreateDatabaseStatement(ctx context.Context, cfg ConnConfig, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	sqlText := normalizeSQLText(statement.SQLText)
	if !regexp.MustCompile(`(?is)^\s*create\s+database\s+`).MatchString(sqlText) {
		return nil, fmt.Errorf("仅支持标准PostgreSQL CREATE DATABASE语句")
	}
	db, err := openPostgres(cfg, postgresMaintenanceDatabase)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if _, err := db.ExecContext(ctx, sqlText); err != nil {
		return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, err
	}
	return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, nil
}

func executePostgreSQLDatabaseDDLStatement(ctx context.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement, pattern, invalidMessage string) (*reviewExecuteResult, error) {
	sqlText := normalizeSQLText(statement.SQLText)
	if !regexp.MustCompile(pattern).MatchString(sqlText) {
		return nil, fmt.Errorf("%s", invalidMessage)
	}
	db, err := openPostgres(cfg, ticket.DatabaseName)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	if _, err := db.ExecContext(ctx, sqlText); err != nil {
		return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, err
	}
	return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, nil
}

func executeMySQLReviewStatement(ctx context.Context, cfg ConnConfig, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	if ticket.TicketType == dbmodel.SQLTicketTypeDDL {
		return executeMySQLDDLReviewStatement(ctx, cfg, statement)
	}
	if ticket.TicketType != dbmodel.SQLTicketTypeDML {
		return nil, fmt.Errorf("only DML and supported DDL tickets can be executed by review executor")
	}
	if !isDMLType(statement.StatementType) {
		return nil, fmt.Errorf("only DML statements can be executed by review executor")
	}

	sqlText := normalizeSQLText(statement.SQLText)
	parsed, err := parseMySQLDMLStatement(sqlText, ticket.DatabaseName)
	if err != nil {
		return nil, err
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if parsed.DatabaseName != "" {
		if err := validateIdentifier(parsed.DatabaseName); err != nil {
			return nil, fmt.Errorf("invalid database: %w", err)
		}
		if _, err := conn.ExecContext(ctx, "USE "+quoteMySQLIdentifier(parsed.DatabaseName)); err != nil {
			return nil, err
		}
	}
	var rollback *dbmodel.SQLRollbackRecord
	var startPosition mysqlBinlogPosition
	if ticket.BackupRequired {
		if err := ensureMySQLBinlogFlashbackReady(ctx, conn); err != nil {
			return nil, err
		}
		startPosition, err = getMySQLCurrentBinlogPosition(ctx, conn)
		if err != nil {
			return nil, fmt.Errorf("获取执行前binlog位点失败: %w", err)
		}
	}

	start := time.Now()
	result, err := conn.ExecContext(ctx, sqlText)
	durationMS := time.Since(start).Milliseconds()
	if err != nil {
		return &reviewExecuteResult{
			DurationMS: durationMS,
			Rollback:   rollback,
		}, err
	}
	rowsAffected, _ := result.RowsAffected()
	if ticket.BackupRequired {
		endPosition, posErr := getMySQLCurrentBinlogPosition(ctx, conn)
		if posErr != nil {
			rollback = buildMySQLBinlogFlashbackRecord(ticket, statement, parsed, rowsAffected, startPosition, mysqlBinlogPosition{}, fmt.Sprintf("SQL已执行成功，但获取执行后binlog位点失败: %v", posErr), false)
		} else {
			rollback = buildMySQLBinlogFlashbackRecord(ticket, statement, parsed, rowsAffected, startPosition, endPosition, "已记录执行前后binlog位点，可生成闪回回滚SQL", false)
		}
	} else if rollback == nil && (parsed.StatementType == "insert" || parsed.StatementType == "replace") {
		rollback = buildMySQLPostExecuteRollback(ticket, statement, parsed, result, rowsAffected)
	}

	return &reviewExecuteResult{
		RowsAffected: rowsAffected,
		DurationMS:   durationMS,
		Rollback:     rollback,
	}, nil
}

func executeMySQLDDLReviewStatement(ctx context.Context, cfg ConnConfig, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	switch statement.StatementType {
	case "create_database":
		return executeMySQLCreateDatabaseStatement(ctx, cfg, statement)
	case "create_table":
		return executeMySQLCreateTableStatement(ctx, cfg, statement)
	default:
		return nil, fmt.Errorf("当前仅支持执行CREATE DATABASE和CREATE TABLE DDL，其它DDL请审核后手工执行")
	}
}

func executeMySQLCreateDatabaseStatement(ctx context.Context, cfg ConnConfig, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	databaseName := extractCreateDatabaseName(statement.SQLText)
	databaseName = strings.Trim(databaseName, "`")
	if err := validateIdentifier(databaseName); err != nil {
		return nil, fmt.Errorf("数据库名不合法: %w", err)
	}
	sqlText := normalizeSQLText(statement.SQLText)
	if !isCreateDatabaseSQL(sqlText) {
		return nil, fmt.Errorf("仅支持标准CREATE DATABASE语句")
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	start := time.Now()
	if _, err := conn.ExecContext(ctx, sqlText); err != nil {
		return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, err
	}
	return &reviewExecuteResult{
		RowsAffected: 0,
		DurationMS:   time.Since(start).Milliseconds(),
	}, nil
}

func executeMySQLCreateTableStatement(ctx context.Context, cfg ConnConfig, statement dbmodel.SQLStatement) (*reviewExecuteResult, error) {
	sqlText := normalizeSQLText(statement.SQLText)
	tableName := extractCreateTableName(sqlText)
	if tableName == "" {
		return nil, fmt.Errorf("CREATE TABLE语句缺少表名")
	}
	if _, _, err := splitMySQLTableName(tableName, cfg.DatabaseName); err != nil {
		return nil, fmt.Errorf("表名不合法: %w", err)
	}
	if !isCreateTableSQL(sqlText) {
		return nil, fmt.Errorf("仅支持标准CREATE TABLE语句")
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if strings.TrimSpace(cfg.DatabaseName) != "" && !strings.Contains(tableName, ".") {
		if err := validateIdentifier(cfg.DatabaseName); err != nil {
			return nil, fmt.Errorf("数据库名不合法: %w", err)
		}
		if _, err := conn.ExecContext(ctx, "USE "+quoteMySQLIdentifier(cfg.DatabaseName)); err != nil {
			return nil, err
		}
	}

	start := time.Now()
	if _, err := conn.ExecContext(ctx, sqlText); err != nil {
		return &reviewExecuteResult{DurationMS: time.Since(start).Milliseconds()}, err
	}
	return &reviewExecuteResult{
		RowsAffected: 0,
		DurationMS:   time.Since(start).Milliseconds(),
	}, nil
}

func isCreateDatabaseSQL(sqlText string) bool {
	re := regexp.MustCompile(`(?is)^\s*create\s+(?:database|schema)\s+(?:if\s+not\s+exists\s+)?(?:` + "`" + `[^` + "`" + `]+` + "`" + `|[A-Za-z0-9_@$-]+)(?:\s+(?:default\s+)?(?:character\s+set|charset)\s+['"` + "`" + `]?[A-Za-z0-9_]+['"` + "`" + `]?|\s+collate\s+['"` + "`" + `]?[A-Za-z0-9_]+['"` + "`" + `]?)*\s*$`)
	return re.MatchString(sqlText)
}

func isCreateTableSQL(sqlText string) bool {
	return regexp.MustCompile(`(?is)^\s*create\s+(?:temporary\s+)?table\s+`).MatchString(sqlText) &&
		extractCreateTableName(sqlText) != "" &&
		strings.Contains(sqlText, "(") &&
		strings.Contains(sqlText, ")")
}

func createMySQLRollbackBackup(ctx context.Context, conn *sql.Conn, ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement, parsed parsedDMLStatement) (*dbmodel.SQLRollbackRecord, error) {
	now := util.HTime{Time: time.Now()}
	record := &dbmodel.SQLRollbackRecord{
		TicketID:         ticket.ID,
		StatementID:      statement.ID,
		StatementType:    parsed.StatementType,
		OriginalDatabase: parsed.DatabaseName,
		OriginalTable:    parsed.TableName,
		RollbackType:     "before_image_backup",
		Available:        false,
		CreatedAt:        now,
	}

	if !parsed.Backupable {
		record.Message = "当前语句无法通过执行前备份自动生成回滚SQL，请使用手工回滚SQL"
		return record, nil
	}

	backupTable := buildMySQLBackupTableName(ticket, statement)
	originalTable := qualifyMySQLTable(parsed.DatabaseName, parsed.TableName)
	backupTableName := qualifyMySQLTable(parsed.DatabaseName, backupTable)
	createSQL := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s LIKE %s", backupTableName, originalTable)
	if _, err := conn.ExecContext(ctx, createSQL); err != nil {
		return nil, fmt.Errorf("create backup table failed: %w", err)
	}

	selectSQL := fmt.Sprintf("SELECT * FROM %s WHERE %s%s", originalTable, parsed.WhereClause, parsed.TailClause)
	insertSQL := fmt.Sprintf("INSERT INTO %s %s", backupTableName, selectSQL)
	insertResult, err := conn.ExecContext(ctx, insertSQL)
	if err != nil {
		return nil, fmt.Errorf("backup affected rows failed: %w", err)
	}
	backupRows, _ := insertResult.RowsAffected()

	record.BackupDatabase = parsed.DatabaseName
	record.BackupTable = backupTable
	record.BackupRows = backupRows
	record.RollbackSQL = buildMySQLRollbackSQL(parsed, backupTable)
	record.Available = true
	record.Message = fmt.Sprintf("已备份 %d 行到 %s", backupRows, backupTableName)
	return record, nil
}

func ensureMySQLBinlogFlashbackReady(ctx context.Context, conn *sql.Conn) error {
	logBin, err := getMySQLServerVariable(ctx, conn, "log_bin")
	if err != nil {
		return fmt.Errorf("check log_bin failed: %w", err)
	}
	if !isMySQLOn(logBin) {
		return fmt.Errorf("MySQL binlog must be enabled before executing auditable DML")
	}
	binlogFormat, err := getMySQLServerVariable(ctx, conn, "binlog_format")
	if err != nil {
		return fmt.Errorf("check binlog_format failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(binlogFormat), "ROW") {
		return fmt.Errorf("MySQL binlog_format must be ROW before executing auditable DML")
	}
	rowImage, err := getMySQLServerVariable(ctx, conn, "binlog_row_image")
	if err != nil {
		return fmt.Errorf("check binlog_row_image failed: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(rowImage), "FULL") {
		return fmt.Errorf("MySQL binlog_row_image must be FULL before generating reliable flashback SQL")
	}
	return nil
}

func getMySQLServerVariable(ctx context.Context, conn *sql.Conn, name string) (string, error) {
	switch name {
	case "log_bin", "binlog_format", "binlog_row_image":
	default:
		return "", fmt.Errorf("unsupported MySQL variable: %s", name)
	}
	var variableName string
	var value string
	err := conn.QueryRowContext(ctx, fmt.Sprintf("SHOW VARIABLES LIKE '%s'", name)).Scan(&variableName, &value)
	return value, err
}

func getMySQLCurrentBinlogPosition(ctx context.Context, conn *sql.Conn) (mysqlBinlogPosition, error) {
	position, err := queryMySQLBinlogPosition(ctx, conn, "SHOW BINARY LOG STATUS")
	if err == nil {
		return position, nil
	}
	position, fallbackErr := queryMySQLBinlogPosition(ctx, conn, "SHOW MASTER STATUS")
	if fallbackErr == nil {
		return position, nil
	}
	return mysqlBinlogPosition{}, err
}

func queryMySQLBinlogPosition(ctx context.Context, conn *sql.Conn, query string) (mysqlBinlogPosition, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return mysqlBinlogPosition{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return mysqlBinlogPosition{}, err
	}
	values := make([]sql.NullString, len(columns))
	scanArgs := make([]interface{}, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	if !rows.Next() {
		return mysqlBinlogPosition{}, sql.ErrNoRows
	}
	if err := rows.Scan(scanArgs...); err != nil {
		return mysqlBinlogPosition{}, err
	}
	var position mysqlBinlogPosition
	for i, column := range columns {
		switch strings.ToLower(column) {
		case "file":
			position.File = values[i].String
		case "position":
			_, _ = fmt.Sscan(values[i].String, &position.Pos)
		}
	}
	if position.File == "" || position.Pos == 0 {
		return mysqlBinlogPosition{}, fmt.Errorf("binlog position is empty")
	}
	return position, nil
}

func isMySQLOn(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "ON", "1", "TRUE":
		return true
	default:
		return false
	}
}

func parseMySQLDMLStatement(sqlText, defaultDatabase string) (parsedDMLStatement, error) {
	stmtType := detectSQLType(sqlText)
	switch stmtType {
	case "update":
		return parseMySQLUpdateStatement(sqlText, defaultDatabase)
	case "delete":
		return parseMySQLDeleteStatement(sqlText, defaultDatabase)
	case "insert", "replace":
		return parseMySQLInsertStatement(sqlText, defaultDatabase, stmtType)
	default:
		return parsedDMLStatement{}, fmt.Errorf("unsupported DML statement type: %s", stmtType)
	}
}

func parseMySQLUpdateStatement(sqlText, defaultDatabase string) (parsedDMLStatement, error) {
	tableName := extractUpdateTableName(sqlText)
	if tableName == "" {
		return parsedDMLStatement{}, fmt.Errorf("UPDATE statement must include table name and SET clause")
	}
	re := regexp.MustCompile(`(?is)^\s*update\s+` + regexp.QuoteMeta(tableName) + `\s+set\s+.+?\s+where\s+(.+)$`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) != 2 {
		return parsedDMLStatement{}, fmt.Errorf("UPDATE statement must be a simple single-table statement with WHERE")
	}
	databaseName, tableName, err := splitMySQLTableName(tableName, defaultDatabase)
	if err != nil {
		return parsedDMLStatement{}, err
	}
	whereClause, tailClause := splitMySQLWhereTail(matches[1])
	if strings.TrimSpace(whereClause) == "" {
		return parsedDMLStatement{}, fmt.Errorf("UPDATE statement must have WHERE condition")
	}
	return parsedDMLStatement{
		StatementType: "update",
		DatabaseName:  databaseName,
		TableName:     tableName,
		WhereClause:   whereClause,
		TailClause:    tailClause,
		Backupable:    true,
	}, nil
}

func parseMySQLDeleteStatement(sqlText, defaultDatabase string) (parsedDMLStatement, error) {
	tableName := extractDeleteTableName(sqlText)
	if tableName == "" {
		return parsedDMLStatement{}, fmt.Errorf("DELETE statement must include FROM table name and WHERE")
	}
	re := regexp.MustCompile(`(?is)^\s*delete\s+from\s+` + regexp.QuoteMeta(tableName) + `\s+where\s+(.+)$`)
	matches := re.FindStringSubmatch(sqlText)
	if len(matches) != 2 {
		return parsedDMLStatement{}, fmt.Errorf("DELETE statement must be a simple single-table statement with WHERE")
	}
	databaseName, tableName, err := splitMySQLTableName(tableName, defaultDatabase)
	if err != nil {
		return parsedDMLStatement{}, err
	}
	whereClause, tailClause := splitMySQLWhereTail(matches[1])
	if strings.TrimSpace(whereClause) == "" {
		return parsedDMLStatement{}, fmt.Errorf("DELETE statement must have WHERE condition")
	}
	return parsedDMLStatement{
		StatementType: "delete",
		DatabaseName:  databaseName,
		TableName:     tableName,
		WhereClause:   whereClause,
		TailClause:    tailClause,
		Backupable:    true,
	}, nil
}

func parseMySQLInsertStatement(sqlText, defaultDatabase, stmtType string) (parsedDMLStatement, error) {
	tableName := extractInsertTableName(sqlText)
	if tableName == "" {
		return parsedDMLStatement{}, fmt.Errorf("%s statement must include INTO table name", strings.ToUpper(stmtType))
	}
	databaseName, tableName, err := splitMySQLTableName(tableName, defaultDatabase)
	if err != nil {
		return parsedDMLStatement{}, err
	}
	return parsedDMLStatement{
		StatementType: stmtType,
		DatabaseName:  databaseName,
		TableName:     tableName,
		Backupable:    false,
	}, nil
}

func splitMySQLTableName(rawName, defaultDatabase string) (string, string, error) {
	parts := strings.Split(rawName, ".")
	for i := range parts {
		parts[i] = strings.Trim(strings.TrimSpace(parts[i]), "`")
	}
	databaseName := strings.TrimSpace(defaultDatabase)
	tableName := ""
	if len(parts) == 1 {
		tableName = parts[0]
	} else if len(parts) == 2 {
		databaseName = parts[0]
		tableName = parts[1]
	} else {
		return "", "", fmt.Errorf("invalid table name: %s", rawName)
	}
	if err := validateIdentifier(databaseName); err != nil {
		return "", "", fmt.Errorf("invalid database: %w", err)
	}
	if err := validateIdentifier(tableName); err != nil {
		return "", "", fmt.Errorf("invalid table: %w", err)
	}
	return databaseName, tableName, nil
}

func splitMySQLWhereTail(whereAndTail string) (string, string) {
	value := strings.TrimSpace(whereAndTail)
	re := regexp.MustCompile(`(?is)\s+(order\s+by|limit)\s+`)
	match := re.FindStringIndex(value)
	if len(match) == 0 {
		return value, ""
	}
	whereClause := strings.TrimSpace(value[:match[0]])
	tailClause := " " + strings.TrimSpace(value[match[0]:])
	return whereClause, tailClause
}

func buildMySQLBackupTableName(ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement) string {
	ticketNo := regexp.MustCompile(`[^A-Za-z0-9_]+`).ReplaceAllString(ticket.TicketNo, "_")
	return fmt.Sprintf("_aiops_backup_%s_%d", strings.ToLower(ticketNo), statement.StatementIndex)
}

func buildMySQLPostExecuteRollback(ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement, parsed parsedDMLStatement, result sql.Result, rowsAffected int64) *dbmodel.SQLRollbackRecord {
	record := &dbmodel.SQLRollbackRecord{
		TicketID:         ticket.ID,
		StatementID:      statement.ID,
		StatementType:    parsed.StatementType,
		OriginalDatabase: parsed.DatabaseName,
		OriginalTable:    parsed.TableName,
		RollbackType:     "post_execute",
		Available:        false,
		CreatedAt:        util.HTime{Time: time.Now()},
	}
	if parsed.StatementType != "insert" || rowsAffected <= 0 {
		record.Message = "当前语句需要手工回滚"
		return record
	}
	lastInsertID, err := result.LastInsertId()
	if err != nil || lastInsertID <= 0 {
		record.Message = "INSERT已执行，但数据库未返回自增ID，请手工确认回滚条件"
		return record
	}
	firstInsertID := lastInsertID
	if rowsAffected > 1 {
		firstInsertID = lastInsertID - rowsAffected + 1
	}
	tableName := qualifyMySQLTable(parsed.DatabaseName, parsed.TableName)
	record.RollbackSQL = fmt.Sprintf("DELETE FROM %s WHERE id BETWEEN %d AND %d;", tableName, firstInsertID, lastInsertID)
	record.Available = true
	record.Message = fmt.Sprintf("基于自增ID生成INSERT回滚SQL，影响ID范围 %d-%d", firstInsertID, lastInsertID)
	return record
}

func buildMySQLBinlogFlashbackRecord(ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement, parsed parsedDMLStatement, rowsAffected int64, startPosition, endPosition mysqlBinlogPosition, message string, available bool) *dbmodel.SQLRollbackRecord {
	return &dbmodel.SQLRollbackRecord{
		TicketID:         ticket.ID,
		StatementID:      statement.ID,
		StatementType:    parsed.StatementType,
		OriginalDatabase: parsed.DatabaseName,
		OriginalTable:    parsed.TableName,
		BackupRows:       rowsAffected,
		RollbackType:     "binlog_flashback",
		BinlogStartFile:  startPosition.File,
		BinlogStartPos:   startPosition.Pos,
		BinlogEndFile:    endPosition.File,
		BinlogEndPos:     endPosition.Pos,
		FlashbackTool:    "my2sql",
		Available:        available,
		Message:          message,
		CreatedAt:        util.HTime{Time: time.Now()},
	}
}

func buildPostgreSQLManualRollbackHint(ticket *dbmodel.SQLTicket, statement dbmodel.SQLStatement, rowsAffected int64) *dbmodel.SQLRollbackRecord {
	if rowsAffected <= 0 {
		return nil
	}
	return &dbmodel.SQLRollbackRecord{
		TicketID:      ticket.ID,
		StatementID:   statement.ID,
		StatementType: statement.StatementType,
		RollbackType:  "pgsql_walminer_hint",
		BackupRows:    rowsAffected,
		Available:     false,
		FlashbackTool: "WalMiner",
		Message:       "PostgreSQL工单已执行；如需自动生成Undo SQL，请结合WalMiner解析WAL，当前版本请使用手工回滚SQL或生成回滚工单",
		CreatedAt:     util.HTime{Time: time.Now()},
	}
}

func buildMySQLRollbackSQL(parsed parsedDMLStatement, backupTable string) string {
	originalTable := qualifyMySQLTable(parsed.DatabaseName, parsed.TableName)
	backupTableName := qualifyMySQLTable(parsed.DatabaseName, backupTable)
	switch parsed.StatementType {
	case "delete":
		return fmt.Sprintf("INSERT INTO %s SELECT * FROM %s;", originalTable, backupTableName)
	case "update":
		return fmt.Sprintf("REPLACE INTO %s SELECT * FROM %s;", originalTable, backupTableName)
	default:
		return ""
	}
}

func qualifyMySQLTable(databaseName, tableName string) string {
	return quoteMySQLIdentifier(databaseName) + "." + quoteMySQLIdentifier(tableName)
}
