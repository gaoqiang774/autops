package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common/util"
)

func TestExtractObjectNameQuotedStatements(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"insert", "INSERT INTO `users` (`username`) VALUES ('zhangsan')", "users"},
		{"replace", "REPLACE INTO `test1`.`users` (`id`) VALUES (1)", "test1.users"},
		{"update", "UPDATE `users` SET `status` = 1 WHERE `id` = 1", "users"},
		{"delete", "DELETE FROM `users` WHERE `id` = 1", "users"},
		{"create_database", "CREATE DATABASE `test1` DEFAULT CHARACTER SET utf8mb4", "test1"},
		{"create_table", "CREATE TABLE `users` (`id` INT)", "users"},
		{"alter", "ALTER TABLE `users` ADD COLUMN `age` INT", "users"},
		{"drop", "DROP TABLE IF EXISTS `users`", "users"},
		{"truncate", "TRUNCATE TABLE `users`", "users"},
		{"create_index", "CREATE INDEX `idx_username` ON `users` (`username`)", "users"},
		{"select", "SELECT * FROM `users` WHERE `id` = 1", "users"},
		{"with", "WITH t AS (SELECT * FROM `users`) SELECT * FROM t", "users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractObjectName(tt.sql); got != tt.want {
				t.Fatalf("extractObjectName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFillTicketFlowRejectedEndsProcess(t *testing.T) {
	now := util.HTime{Time: time.Now()}
	detail := &dbmodel.SQLTicketDetail{
		Ticket: dbmodel.SQLTicket{
			ID:            1,
			Status:        dbmodel.SQLTicketStatusRejected,
			SubmitterID:   89,
			SubmitterName: "admin",
			SubmittedAt:   now,
			ApproverID:    109,
			ApproverName:  "zhangfan",
			ReviewedAt:    now,
			ExecutorID:    89,
			ExecutorName:  "admin",
		},
		ApprovalRecords: []dbmodel.SQLApprovalRecord{
			{
				TicketID:     1,
				OperatorID:   109,
				OperatorName: "zhangfan",
				Action:       dbmodel.SQLApprovalActionReject,
				Comment:      "不同意执行",
				CreatedAt:    now,
			},
		},
	}

	fillTicketFlow(detail)

	if !detail.FlowEnded {
		t.Fatalf("FlowEnded = false, want true")
	}
	if detail.FlowEndReason == "" {
		t.Fatalf("FlowEndReason should not be empty")
	}
	if len(detail.FlowSteps) != 3 {
		t.Fatalf("FlowSteps len = %d, want 3", len(detail.FlowSteps))
	}
	if detail.FlowSteps[1].Status != "rejected" {
		t.Fatalf("review step status = %q, want rejected", detail.FlowSteps[1].Status)
	}
	if detail.FlowSteps[2].Status != "skipped" {
		t.Fatalf("execute step status = %q, want skipped", detail.FlowSteps[2].Status)
	}
}

func TestExtractCreateTableNameQuoted(t *testing.T) {
	sqlText := "CREATE TABLE `users` (\n  `id` INT NOT NULL\n) ENGINE=InnoDB"

	got := extractCreateTableName(sqlText)
	if got != "`users`" {
		t.Fatalf("extractCreateTableName() = %q, want %q", got, "`users`")
	}
}

func TestExtractCreateDatabaseNameQuoted(t *testing.T) {
	sqlText := "CREATE DATABASE `test1` DEFAULT CHARACTER SET utf8mb4"

	got := extractCreateDatabaseName(sqlText)
	if got != "`test1`" {
		t.Fatalf("extractCreateDatabaseName() = %q, want %q", got, "`test1`")
	}
}

func TestParseMySQLInsertStatementQuotedTable(t *testing.T) {
	sqlText := "INSERT INTO `users` (`username`, `email`) VALUES ('zhangsan', 'zhangsan@test.com')"

	got, err := parseMySQLInsertStatement(sqlText, "test1", "insert")
	if err != nil {
		t.Fatalf("parseMySQLInsertStatement() error = %v", err)
	}
	if got.DatabaseName != "test1" || got.TableName != "users" {
		t.Fatalf("parseMySQLInsertStatement() database/table = %s/%s, want test1/users", got.DatabaseName, got.TableName)
	}
}

func TestParseMySQLDMLStatementQuotedTables(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"insert", "INSERT INTO `users` (`username`) VALUES ('zhangsan')", "users"},
		{"replace", "REPLACE INTO `users` (`id`, `username`) VALUES (1, 'zhangsan')", "users"},
		{"update", "UPDATE `users` SET `status` = 1 WHERE `id` = 1", "users"},
		{"delete", "DELETE FROM `users` WHERE `id` = 1", "users"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseMySQLDMLStatement(tt.sql, "test1")
			if err != nil {
				t.Fatalf("parseMySQLDMLStatement() error = %v", err)
			}
			if got.DatabaseName != "test1" || got.TableName != tt.want {
				t.Fatalf("parseMySQLDMLStatement() database/table = %s/%s, want test1/%s", got.DatabaseName, got.TableName, tt.want)
			}
		})
	}
}

func TestWhereContainsAnyKeyGroup(t *testing.T) {
	keyGroups := [][]string{{"id"}, {"tenant_id", "username"}}
	tests := []struct {
		name  string
		where string
		want  bool
	}{
		{"primary_key", "`id` = 1", true},
		{"unique_key", "`tenant_id` = 1 AND `username` = 'zhangsan'", true},
		{"missing_partial_unique", "`tenant_id` = 1", false},
		{"no_key", "`status` = 1", false},
		{"in_condition", "id IN (1,2)", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := whereContainsAnyKeyGroup(tt.where, keyGroups); got != tt.want {
				t.Fatalf("whereContainsAnyKeyGroup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRollbackTicketType(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"dml_delete", "DELETE FROM `users` WHERE id BETWEEN 1 AND 2", "dml"},
		{"dml_insert", "INSERT INTO `users` SELECT * FROM `_backup_users`", "dml"},
		{"ddl_drop_table", "DROP TABLE `users`", "ddl"},
		{"mixed_reject", "DELETE FROM `users` WHERE id = 1; DROP TABLE `users`", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rollbackTicketType(tt.sql); got != tt.want {
				t.Fatalf("rollbackTicketType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectBackupTables(t *testing.T) {
	records := []dbmodel.SQLRollbackRecord{
		{BackupDatabase: "test1", BackupTable: "_aiops_backup_db_dml_1"},
		{BackupDatabase: "test1", BackupTable: "_aiops_backup_db_dml_1"},
		{BackupDatabase: "test1", BackupTable: "users"},
		{BackupDatabase: "", BackupTable: "_aiops_backup_empty_db"},
	}

	got := collectBackupTables(records)
	if len(got) != 1 {
		t.Fatalf("collectBackupTables() len = %d, want 1", len(got))
	}
	if got[0].String() != "test1._aiops_backup_db_dml_1" {
		t.Fatalf("collectBackupTables()[0] = %q", got[0].String())
	}
}

func TestBuildDDLReverseSQLAlterAddColumnQuoted(t *testing.T) {
	sqlText := "ALTER TABLE `users` ADD COLUMN `user_avatar` text NULL COMMENT '用户头像链接' AFTER `email`"

	got, ok := buildDDLReverseSQL(sqlText)
	if !ok {
		t.Fatalf("buildDDLReverseSQL() ok = false")
	}
	want := "ALTER TABLE `users` DROP COLUMN `user_avatar`;"
	if got != want {
		t.Fatalf("buildDDLReverseSQL() = %q, want %q", got, want)
	}
	if strings.Contains(got, "`COLUMN`") {
		t.Fatalf("rollback SQL should not use COLUMN as column name: %s", got)
	}
}

func TestBuildInitialRollbackRecordsForMultipleDDLAddColumns(t *testing.T) {
	now := util.HTime{Time: time.Now()}
	statements := []dbmodel.SQLStatement{
		{
			StatementIndex: 1,
			StatementType:  "alter",
			ObjectName:     "users",
			SQLText:        "ALTER TABLE `users` ADD COLUMN `user_avatar` text NULL COMMENT '用户头像链接' AFTER `email`",
		},
		{
			StatementIndex: 2,
			StatementType:  "alter",
			ObjectName:     "users",
			SQLText:        "ALTER TABLE `users` ADD COLUMN `user_bio` text NULL COMMENT '用户个人简介' AFTER `user_avatar`",
		},
	}

	got := buildInitialRollbackRecords(dbmodel.SQLTicketTypeDDL, statements, "", now)
	if len(got) != 2 {
		t.Fatalf("rollback records len = %d, want 2", len(got))
	}
	if got[0].RollbackSQL != "ALTER TABLE `users` DROP COLUMN `user_avatar`;" {
		t.Fatalf("rollback[0] = %q", got[0].RollbackSQL)
	}
	if got[1].RollbackSQL != "ALTER TABLE `users` DROP COLUMN `user_bio`;" {
		t.Fatalf("rollback[1] = %q", got[1].RollbackSQL)
	}
}

func TestParseGhOstAlterPlansMergeSameTable(t *testing.T) {
	sqlText := "ALTER TABLE `jjsfang`.`fang_entrust_wq` ADD COLUMN `WM_URL` text NULL COMMENT '委卖链接' AFTER `TENANT_ID`;\n" +
		"ALTER TABLE `jjsfang`.`fang_entrust_wq` ADD COLUMN `WM_URL_H5` text NULL COMMENT '委卖链接(房协接口返回的链接)' AFTER `WM_URL`;"

	plans, err := parseGhOstAlterPlans("jjsfang", sqlText)
	if err != nil {
		t.Fatalf("parseGhOstAlterPlans() error = %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("parseGhOstAlterPlans() len = %d, want 1", len(plans))
	}
	if plans[0].Database != "jjsfang" || plans[0].Table != "fang_entrust_wq" {
		t.Fatalf("plan target = %s.%s", plans[0].Database, plans[0].Table)
	}
	if !strings.Contains(plans[0].Alter, "WM_URL") || !strings.Contains(plans[0].Alter, "WM_URL_H5") {
		t.Fatalf("merged alter missing columns: %s", plans[0].Alter)
	}
}

func TestGhOstLogSucceeded(t *testing.T) {
	if !isGhOstLogSucceeded("copy rows\nDone\n") {
		t.Fatalf("isGhOstLogSucceeded() should accept final Done")
	}
	successWithCleanupError := "2026-05-21 19:16:06 INFO Done migrating `test1`.`users`\n" +
		"2026-05-21 19:16:06 ERROR Error 1146: Table 'test1._users_ghc' doesn't exist\n" +
		"# Done\n"
	if !isGhOstLogSucceeded(successWithCleanupError) {
		t.Fatalf("isGhOstLogSucceeded() should accept final Done with benign cleanup error")
	}
	if isGhOstLogSucceeded("copy rows\nFATAL Error 1060: Duplicate column name 'user_avatar'\n") {
		t.Fatalf("isGhOstLogSucceeded() should reject fatal gh-ost failure")
	}
	if isGhOstLogSucceeded("copy rows\nERROR Unable to ALTER ghost table, see further error details. Bailing out\n# Done\n") {
		t.Fatalf("isGhOstLogSucceeded() should reject failed migration even with final Done")
	}
	if isGhOstLogSucceeded("Done\ncleanup\n") {
		t.Fatalf("isGhOstLogSucceeded() should require last effective line contains Done")
	}
}

func TestBuildGhOstScriptUsesConfigCredential(t *testing.T) {
	script, err := buildGhOstScript(ConnConfig{
		Engine:       1,
		Host:         "127.0.0.1",
		Port:         3306,
		Username:     "root",
		Password:     "jjshome0822^&*10'xx",
		DatabaseName: "jjsfang",
	}, "ALTER TABLE `jjsfang`.`fang_entrust_wq` ADD COLUMN `WM_URL` text NULL")
	if err != nil {
		t.Fatalf("buildGhOstScript() error = %v", err)
	}
	if !strings.Contains(script, "/opt/db/gh-ost  \\") {
		t.Fatalf("script missing binary path: %s", script)
	}
	if !strings.Contains(script, "--user='root'") {
		t.Fatalf("script missing user: %s", script)
	}
	if !strings.Contains(script, "--password='jjshome0822^&*10'\"'\"'xx'") {
		t.Fatalf("script password not shell-escaped: %s", script)
	}
}

func TestBuildGhOstScriptRejectsCreateTable(t *testing.T) {
	_, err := buildGhOstScript(ConnConfig{
		Engine:       1,
		Host:         "127.0.0.1",
		Port:         3306,
		Username:     "root",
		Password:     "secret",
		DatabaseName: "test1",
	}, "CREATE TABLE `users` (`id` BIGINT PRIMARY KEY)")
	if err == nil {
		t.Fatalf("buildGhOstScript() should reject non ALTER TABLE SQL")
	}
}

func TestExtractMy2SQLRollbackSQL(t *testing.T) {
	output := "header\n" +
		"DELETE FROM `test1`.`users` WHERE `id`=1\n" +
		"noise\n" +
		"UPDATE `test1`.`users` SET `name`='old' WHERE `id`=2;\n"

	got := extractMy2SQLRollbackSQL(output)
	want := "DELETE FROM `test1`.`users` WHERE `id`=1;\nUPDATE `test1`.`users` SET `name`='old' WHERE `id`=2;"
	if got != want {
		t.Fatalf("extractMy2SQLRollbackSQL() = %q, want %q", got, want)
	}
}

func TestSelectFlashbackRollbackRecord(t *testing.T) {
	records := []dbmodel.SQLRollbackRecord{
		{ID: 1, RollbackType: "manual"},
		{ID: 2, RollbackType: "binlog_flashback", BinlogStartFile: "mysql-bin.000001"},
	}
	got, err := selectFlashbackRollbackRecord(records, 0)
	if err != nil {
		t.Fatalf("selectFlashbackRollbackRecord() error = %v", err)
	}
	if got.ID != 2 {
		t.Fatalf("selectFlashbackRollbackRecord() ID = %d, want 2", got.ID)
	}
	if _, err := selectFlashbackRollbackRecord(records, 1); err == nil {
		t.Fatalf("selectFlashbackRollbackRecord() should reject non-flashback record")
	}
}

func TestResolveMy2SQLBinaryPathFromEnv(t *testing.T) {
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "my2sql")
	if err := os.WriteFile(binary, []byte("test"), 0o755); err != nil {
		t.Fatalf("write temp my2sql failed: %v", err)
	}
	t.Setenv("AIOPS_MY2SQL_PATH", binary)
	got, err := resolveMy2SQLBinaryPath()
	if err != nil {
		t.Fatalf("resolveMy2SQLBinaryPath() error = %v", err)
	}
	if got != binary {
		t.Fatalf("resolveMy2SQLBinaryPath() = %q, want %q", got, binary)
	}
}

func TestReviewPostgreSQLRejectsMySQLOnlySyntax(t *testing.T) {
	sqlText := "CREATE TABLE `users` (`id` INT NOT NULL AUTO_INCREMENT) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"

	got := reviewSQL(dbmodel.SQLTicketTypeDDL, "test", "", sqlText, false, cmdbmodel.DatabaseTypePostgreSQL)

	if got.ReviewPassed {
		t.Fatalf("ReviewPassed = true, want false")
	}
	if !hasReviewCode(got.ReviewResults, "pgsql_backtick_identifier") {
		t.Fatalf("missing pgsql_backtick_identifier finding: %#v", got.ReviewResults)
	}
	if !hasReviewCode(got.ReviewResults, "pgsql_auto_increment_unsupported") {
		t.Fatalf("missing pgsql_auto_increment_unsupported finding: %#v", got.ReviewResults)
	}
	if !hasReviewCode(got.ReviewResults, "pgsql_engine_option_unsupported") {
		t.Fatalf("missing pgsql_engine_option_unsupported finding: %#v", got.ReviewResults)
	}
}

func TestReviewPostgreSQLRejectsReplace(t *testing.T) {
	sqlText := "REPLACE INTO users (id, name) VALUES (1, 'zhangsan')"

	got := reviewSQL(dbmodel.SQLTicketTypeDML, "test", "", sqlText, true, cmdbmodel.DatabaseTypePostgreSQL)

	if got.ReviewPassed {
		t.Fatalf("ReviewPassed = true, want false")
	}
	if !hasReviewCode(got.ReviewResults, "pgsql_replace_unsupported") {
		t.Fatalf("missing pgsql_replace_unsupported finding: %#v", got.ReviewResults)
	}
}

func TestReviewPostgreSQLUpdateUsesBatchAdvice(t *testing.T) {
	sqlText := "UPDATE users SET status = 1 WHERE id = 1"

	got := reviewSQL(dbmodel.SQLTicketTypeDML, "test", "", sqlText, true, cmdbmodel.DatabaseTypePostgreSQL)

	if !hasReviewCode(got.ReviewResults, "pgsql_dml_batch_advice") {
		t.Fatalf("missing pgsql_dml_batch_advice finding: %#v", got.ReviewResults)
	}
	if hasReviewCode(got.ReviewResults, "dml_warn_no_limit") {
		t.Fatalf("PostgreSQL review should not require MySQL LIMIT syntax: %#v", got.ReviewResults)
	}
}

func TestReviewDMLRejectsAlwaysTrueWhere(t *testing.T) {
	sqlText := "DELETE FROM users WHERE 1 = 1 LIMIT 100"

	got := reviewSQL(dbmodel.SQLTicketTypeDML, "test", "", sqlText, true, cmdbmodel.DatabaseTypeMySQL)

	if got.ReviewPassed {
		t.Fatalf("ReviewPassed = true, want false")
	}
	if !hasReviewCode(got.ReviewResults, "dml_forbid_always_true_where") {
		t.Fatalf("missing dml_forbid_always_true_where finding: %#v", got.ReviewResults)
	}
}

func TestReviewQueryRejectsLockingRead(t *testing.T) {
	sqlText := "SELECT id FROM users WHERE id = 1 FOR UPDATE"

	got := reviewSQL(dbmodel.SQLTicketTypeQuery, "test", "", sqlText, false, cmdbmodel.DatabaseTypePostgreSQL)

	if got.ReviewPassed {
		t.Fatalf("ReviewPassed = true, want false")
	}
	if !hasReviewCode(got.ReviewResults, "query_forbid_locking_read") {
		t.Fatalf("missing query_forbid_locking_read finding: %#v", got.ReviewResults)
	}
}

func TestReviewCreateTableRequiresPrimaryKey(t *testing.T) {
	sqlText := "CREATE TABLE users (id BIGINT NOT NULL, name VARCHAR(64)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"

	got := reviewSQL(dbmodel.SQLTicketTypeDDL, "test", "", sqlText, false, cmdbmodel.DatabaseTypeMySQL)

	if got.ReviewPassed {
		t.Fatalf("ReviewPassed = true, want false")
	}
	if !hasReviewCode(got.ReviewResults, "ddl_create_table_require_primary_key") {
		t.Fatalf("missing ddl_create_table_require_primary_key finding: %#v", got.ReviewResults)
	}
}

func TestReviewPostgreSQLCreateIndexWarnsConcurrently(t *testing.T) {
	sqlText := "CREATE INDEX idx_users_name ON users (name)"

	got := reviewSQL(dbmodel.SQLTicketTypeDDL, "test", "", sqlText, false, cmdbmodel.DatabaseTypePostgreSQL)

	if !hasReviewCode(got.ReviewResults, "pgsql_ddl_create_index_concurrently") {
		t.Fatalf("missing pgsql_ddl_create_index_concurrently finding: %#v", got.ReviewResults)
	}
}

func TestReviewQueryWarnsExpensivePredicates(t *testing.T) {
	sqlText := "SELECT * FROM users WHERE DATE(created_at) = '2026-05-25' OR name LIKE '%abc%' ORDER BY RAND() LIMIT 20"

	got := reviewSQL(dbmodel.SQLTicketTypeQuery, "test", "", sqlText, false, cmdbmodel.DatabaseTypeMySQL)

	for _, code := range []string{
		"query_warn_select_star",
		"query_warn_leading_wildcard_like",
		"query_warn_or_condition",
		"query_warn_function_in_where",
		"query_warn_order_by_random",
	} {
		if !hasReviewCode(got.ReviewResults, code) {
			t.Fatalf("missing %s finding: %#v", code, got.ReviewResults)
		}
	}
}

func TestExecutePostgreSQLDDLAllowsCommonStatements(t *testing.T) {
	ticket := &dbmodel.SQLTicket{TicketType: dbmodel.SQLTicketTypeDDL}
	tests := []struct {
		name      string
		statement dbmodel.SQLStatement
	}{
		{
			name:      "alter_table",
			statement: dbmodel.SQLStatement{StatementType: "alter", SQLText: "ALTER TABLE departments ADD COLUMN status SMALLINT DEFAULT 1"},
		},
		{
			name:      "create_index",
			statement: dbmodel.SQLStatement{StatementType: "create_index", SQLText: "CREATE INDEX idx_departments_name ON departments (dept_name)"},
		},
		{
			name:      "drop_index",
			statement: dbmodel.SQLStatement{StatementType: "drop_index", SQLText: "DROP INDEX idx_departments_name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executePostgreSQLDDLReviewStatement(context.Background(), ConnConfig{}, ticket, tt.statement)
			if err == nil {
				t.Fatalf("executePostgreSQLDDLReviewStatement() should fail opening database, proving statement passed policy gate")
			}
			if strings.Contains(err.Error(), "当前仅支持执行PostgreSQL") || strings.Contains(err.Error(), "仅支持标准PostgreSQL") {
				t.Fatalf("statement was rejected by policy gate: %v", err)
			}
		})
	}
}

func hasReviewCode(results []dbmodel.SQLReviewResult, code string) bool {
	for _, item := range results {
		if item.RuleCode == code {
			return true
		}
	}
	return false
}
