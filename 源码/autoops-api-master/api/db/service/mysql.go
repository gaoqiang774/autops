package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN",
	"GROUP BY", "ORDER BY", "HAVING", "LIMIT", "OFFSET", "COUNT", "SUM", "AVG",
	"MIN", "MAX", "DISTINCT", "AND", "OR", "IN", "LIKE", "IS NULL", "IS NOT NULL",
	"CREATE", "CREATE DATABASE", "CREATE SCHEMA", "CREATE TABLE", "CREATE INDEX",
	"ALTER", "ALTER TABLE", "ADD COLUMN", "DROP COLUMN", "MODIFY COLUMN", "CHANGE COLUMN",
	"DROP", "DROP DATABASE", "DROP TABLE", "DROP INDEX", "TRUNCATE", "RENAME TABLE",
	"DATABASE", "SCHEMA", "TABLE", "INDEX", "COLUMN", "PRIMARY KEY", "UNIQUE",
	"NOT NULL", "DEFAULT", "COMMENT", "CHARACTER SET", "COLLATE", "IF NOT EXISTS",
	"IF EXISTS", "ENGINE", "AUTO_INCREMENT",
}

var readOnlySQLDeniedKeywordPattern = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|TRUNCATE|CREATE|REPLACE|MERGE|CALL|EXEC|GRANT|REVOKE|ANALYZE|VACUUM|COPY|LOAD|LOCK|UNLOCK|SET)\b`)

const sqlConfirmThresholdSeconds = 10

const (
	mysqlPoolMaxOpenConns    = 3
	mysqlPoolMaxIdleConns    = 1
	mysqlPoolConnMaxLifetime = 30 * time.Minute
	mysqlPoolIdleTTL         = 10 * time.Minute
)

type mysqlPoolEntry struct {
	db         *sql.DB
	lastActive time.Time
}

type sqlQueryer interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

var mysqlPools = struct {
	sync.Mutex
	items map[string]*mysqlPoolEntry
}{
	items: map[string]*mysqlPoolEntry{},
}

func init() {
	go cleanupIdleMySQLPools()
}

type MySQLDriver struct{}

func NewMySQLDriver() *MySQLDriver {
	return &MySQLDriver{}
}

func (d *MySQLDriver) Type() int {
	return cmdbmodel.DatabaseTypeMySQL
}

func (d *MySQLDriver) Test(ctx context.Context, cfg ConnConfig) error {
	db, err := openMySQL(cfg, "")
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

func (d *MySQLDriver) Tree(ctx context.Context, cfg ConnConfig, req dbmodel.TreeQuery) ([]dbmodel.TreeNode, error) {
	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	level := strings.TrimSpace(req.Level)
	if level == "" || level == "root" {
		return d.databaseNodes(ctx, db, cfg, req.Keyword)
	}
	if level == dbmodel.ObjectTypeDatabase {
		if err := validateIdentifier(req.Database); err != nil {
			return nil, err
		}
		return d.tableNodes(ctx, db, cfg, req.Database, req.Keyword)
	}
	if level == dbmodel.ObjectTypeTable || level == dbmodel.ObjectTypeView {
		objectReq := dbmodel.ObjectRequest{
			AssetID:    req.AssetID,
			Database:   req.Database,
			ObjectName: req.ObjectName,
			ObjectType: req.ObjectType,
		}
		columns, err := d.Columns(ctx, cfg, objectReq)
		if err != nil {
			return nil, err
		}
		nodes := make([]dbmodel.TreeNode, 0, len(columns))
		for _, column := range columns {
			nodes = append(nodes, dbmodel.TreeNode{
				Key:        fmt.Sprintf("mysql:%d:%s:%s:%s", cfg.AssetID, req.Database, req.ObjectName, column.Name),
				Label:      column.Name,
				Type:       dbmodel.ObjectTypeColumn,
				AssetID:    cfg.AssetID,
				Engine:     cfg.Engine,
				Database:   req.Database,
				ObjectName: column.Name,
				Leaf:       true,
				Extra: map[string]interface{}{
					"dataType":   column.Type,
					"primaryKey": column.PrimaryKey,
					"nullable":   column.Nullable,
				},
			})
		}
		return nodes, nil
	}
	return nil, fmt.Errorf("unsupported tree level: %s", level)
}

func (d *MySQLDriver) Columns(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error) {
	if err := validateObjectRequest(req); err != nil {
		return nil, err
	}
	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
SELECT column_name, column_type, is_nullable, column_default, column_comment, column_key, extra
FROM information_schema.columns
WHERE table_schema = ? AND table_name = ?
ORDER BY ordinal_position`, req.Database, req.ObjectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]dbmodel.ColumnInfo, 0)
	for rows.Next() {
		var name, columnType, nullable, columnKey, extra string
		var defaultValue, comment sql.NullString
		if err := rows.Scan(&name, &columnType, &nullable, &defaultValue, &comment, &columnKey, &extra); err != nil {
			return nil, err
		}
		columns = append(columns, dbmodel.ColumnInfo{
			Name:          name,
			Type:          columnType,
			Nullable:      strings.EqualFold(nullable, "YES"),
			DefaultValue:  defaultValue.String,
			Comment:       comment.String,
			PrimaryKey:    strings.EqualFold(columnKey, "PRI"),
			AutoIncrement: strings.Contains(strings.ToLower(extra), "auto_increment"),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table not found or has no columns")
	}
	return columns, nil
}

func (d *MySQLDriver) Indexes(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error) {
	if err := validateObjectRequest(req); err != nil {
		return nil, err
	}
	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `
SELECT index_name, non_unique, seq_in_index, column_name, index_type, index_comment
FROM information_schema.statistics
WHERE table_schema = ? AND table_name = ?
ORDER BY index_name, seq_in_index`, req.Database, req.ObjectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type indexRow struct {
		name      string
		nonUnique int
		seq       int
		column    string
		indexType string
		comment   string
	}
	grouped := map[string][]indexRow{}
	for rows.Next() {
		var item indexRow
		var comment sql.NullString
		if err := rows.Scan(&item.name, &item.nonUnique, &item.seq, &item.column, &item.indexType, &comment); err != nil {
			return nil, err
		}
		item.comment = comment.String
		grouped[item.name] = append(grouped[item.name], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(grouped))
	for name := range grouped {
		names = append(names, name)
	}
	sort.Strings(names)

	indexes := make([]dbmodel.IndexInfo, 0, len(names))
	for _, name := range names {
		items := grouped[name]
		sort.Slice(items, func(i, j int) bool { return items[i].seq < items[j].seq })
		columns := make([]string, 0, len(items))
		for _, item := range items {
			columns = append(columns, item.column)
		}
		first := items[0]
		indexes = append(indexes, dbmodel.IndexInfo{
			Name:      name,
			Columns:   columns,
			Unique:    first.nonUnique == 0,
			Primary:   name == "PRIMARY",
			IndexType: first.indexType,
			Comment:   first.comment,
		})
	}
	return indexes, nil
}

func (d *MySQLDriver) DDL(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) (string, error) {
	if err := validateObjectRequest(req); err != nil {
		return "", err
	}
	db, err := openMySQL(cfg, "")
	if err != nil {
		return "", err
	}

	query := fmt.Sprintf("SHOW CREATE TABLE %s.%s", quoteMySQLIdentifier(req.Database), quoteMySQLIdentifier(req.ObjectName))
	var name string
	var ddl string
	if err := db.QueryRowContext(ctx, query).Scan(&name, &ddl); err != nil {
		return "", err
	}
	return ddl, nil
}

func (d *MySQLDriver) Query(ctx context.Context, cfg ConnConfig, req dbmodel.DataQueryRequest) (*dbmodel.DataResult, error) {
	if req.ObjectType == "" {
		req.ObjectType = dbmodel.ObjectTypeTable
	}
	if req.ObjectType != dbmodel.ObjectTypeTable && req.ObjectType != dbmodel.ObjectTypeView {
		return nil, fmt.Errorf("unsupported objectType for MySQL query: %s", req.ObjectType)
	}
	objectReq := dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, Schema: req.Schema, ObjectName: req.ObjectName, ObjectType: req.ObjectType}
	columns, err := d.Columns(ctx, cfg, objectReq)
	if err != nil {
		return nil, err
	}
	columnMap := buildColumnMap(columns)
	selected, err := selectedColumns(req.Columns, columns, columnMap)
	if err != nil {
		return nil, err
	}

	whereSQL, args, err := buildMySQLWhere(req.Filters, columnMap)
	if err != nil {
		return nil, err
	}
	orderSQL, err := buildMySQLOrder(req.Sorts, columnMap)
	if err != nil {
		return nil, err
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	tableName := fmt.Sprintf("%s.%s", quoteMySQLIdentifier(req.Database), quoteMySQLIdentifier(req.ObjectName))
	countSQL := "SELECT COUNT(1) FROM " + tableName + whereSQL
	var total int64
	if err := db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	selectSQL := "SELECT " + strings.Join(quoteMySQLIdentifiers(selected), ", ") + " FROM " + tableName + whereSQL + orderSQL + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := db.QueryContext(ctx, selectSQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dataRows, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	return &dbmodel.DataResult{
		Columns:     filterColumnInfos(columns, selected),
		Rows:        dataRows,
		PrimaryKeys: primaryKeys(columns),
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		Editable:    len(primaryKeys(columns)) > 0 && req.ObjectType == dbmodel.ObjectTypeTable,
	}, nil
}

func (d *MySQLDriver) Create(ctx context.Context, cfg ConnConfig, req dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error) {
	if err := validateMutationObject(req.Database, req.ObjectName); err != nil {
		return nil, err
	}
	if len(req.Values) == 0 {
		return nil, fmt.Errorf("values is required")
	}
	columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, ObjectName: req.ObjectName, ObjectType: dbmodel.ObjectTypeTable})
	if err != nil {
		return nil, err
	}
	columnMap := buildColumnMap(columns)
	names, values, err := validateValueMap(req.Values, columnMap, nil)
	if err != nil {
		return nil, err
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(names))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	query := fmt.Sprintf("INSERT INTO %s.%s (%s) VALUES (%s)",
		quoteMySQLIdentifier(req.Database),
		quoteMySQLIdentifier(req.ObjectName),
		strings.Join(quoteMySQLIdentifiers(names), ", "),
		strings.Join(placeholders, ", "),
	)
	result, err := db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	lastInsertID, _ := result.LastInsertId()
	return &dbmodel.MutationResult{RowsAffected: rowsAffected, LastInsertID: lastInsertID}, nil
}

func (d *MySQLDriver) Update(ctx context.Context, cfg ConnConfig, req dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error) {
	if err := validateMutationObject(req.Database, req.ObjectName); err != nil {
		return nil, err
	}
	if len(req.Values) == 0 {
		return nil, fmt.Errorf("values is required")
	}
	columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, ObjectName: req.ObjectName, ObjectType: dbmodel.ObjectTypeTable})
	if err != nil {
		return nil, err
	}
	columnMap := buildColumnMap(columns)
	pks := primaryKeys(columns)
	if err := validateKeys(req.Keys, pks, columnMap); err != nil {
		return nil, err
	}
	updateNames, updateValues, err := validateValueMap(req.Values, columnMap, stringSet(pks))
	if err != nil {
		return nil, err
	}

	setClauses := make([]string, 0, len(updateNames))
	for _, name := range updateNames {
		setClauses = append(setClauses, quoteMySQLIdentifier(name)+" = ?")
	}
	whereSQL, whereValues := buildPrimaryKeyWhere(req.Keys, pks)
	args := append(updateValues, whereValues...)

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("UPDATE %s.%s SET %s WHERE %s",
		quoteMySQLIdentifier(req.Database),
		quoteMySQLIdentifier(req.ObjectName),
		strings.Join(setClauses, ", "),
		whereSQL,
	)
	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	return &dbmodel.MutationResult{RowsAffected: rowsAffected, Keys: req.Keys}, nil
}

func (d *MySQLDriver) Delete(ctx context.Context, cfg ConnConfig, req dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error) {
	if err := validateMutationObject(req.Database, req.ObjectName); err != nil {
		return nil, err
	}
	columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, ObjectName: req.ObjectName, ObjectType: dbmodel.ObjectTypeTable})
	if err != nil {
		return nil, err
	}
	columnMap := buildColumnMap(columns)
	pks := primaryKeys(columns)
	if err := validateKeys(req.Keys, pks, columnMap); err != nil {
		return nil, err
	}
	whereSQL, whereValues := buildPrimaryKeyWhere(req.Keys, pks)

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("DELETE FROM %s.%s WHERE %s",
		quoteMySQLIdentifier(req.Database),
		quoteMySQLIdentifier(req.ObjectName),
		whereSQL,
	)
	result, err := db.ExecContext(ctx, query, whereValues...)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	return &dbmodel.MutationResult{RowsAffected: rowsAffected, Keys: req.Keys}, nil
}

func (d *MySQLDriver) ExecuteSQL(ctx context.Context, cfg ConnConfig, req dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error) {
	sqlText := normalizeSQLText(req.SQL)
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	if !isReadOnlySQL(sqlText) {
		return nil, fmt.Errorf("only SELECT/WITH read-only SQL is allowed")
	}
	if strings.TrimSpace(req.Database) != "" {
		if err := validateIdentifier(req.Database); err != nil {
			return nil, fmt.Errorf("invalid database: %w", err)
		}
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
	if strings.TrimSpace(req.Database) != "" {
		if _, err := conn.ExecContext(ctx, "USE "+quoteMySQLIdentifier(req.Database)); err != nil {
			return nil, err
		}
	}

	estimate := estimateMySQLReadOnlySQL(ctx, conn, sqlText)
	if estimate.EstimatedSeconds >= sqlConfirmThresholdSeconds && !req.Confirmed.Bool() {
		return &dbmodel.SQLExecuteResult{
			SQL:            sqlText,
			ReadOnly:       true,
			RequireConfirm: true,
			Estimate:       estimate,
		}, nil
	}

	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	pagedSQL := fmt.Sprintf("SELECT * FROM (%s) AS db_query_result LIMIT ? OFFSET ?", sqlText)
	args := []interface{}{pageSize, (page - 1) * pageSize}

	start := time.Now()
	rows, err := conn.QueryContext(ctx, pagedSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	dataRows, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	columns := make([]dbmodel.ColumnInfo, 0, len(columnNames))
	for _, name := range columnNames {
		columns = append(columns, dbmodel.ColumnInfo{Name: name})
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(1) FROM (%s) AS db_query_count", sqlText)
	if err := conn.QueryRowContext(ctx, countSQL).Scan(&total); err != nil {
		total = int64(len(dataRows))
	}

	return &dbmodel.SQLExecuteResult{
		Columns:    columns,
		Rows:       dataRows,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		DurationMS: time.Since(start).Milliseconds(),
		SQL:        sqlText,
		ReadOnly:   true,
		Estimate:   estimate,
	}, nil
}

func (d *MySQLDriver) Completion(ctx context.Context, cfg ConnConfig, req dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error) {
	response := &dbmodel.SQLCompletionResponse{
		Keywords: append([]string{}, sqlKeywords...),
	}

	db, err := openMySQL(cfg, "")
	if err != nil {
		return nil, err
	}

	databaseNodes, err := d.databaseNodes(ctx, db, cfg, req.Keyword)
	if err != nil {
		return nil, err
	}
	for _, node := range databaseNodes {
		response.Databases = append(response.Databases, dbmodel.SQLCompletionItem{
			Label:    node.Label,
			Value:    node.Label,
			Type:     dbmodel.ObjectTypeDatabase,
			Database: node.Database,
		})
	}

	if strings.TrimSpace(req.Database) == "" {
		return response, nil
	}
	if err := validateIdentifier(req.Database); err != nil {
		return nil, fmt.Errorf("invalid database: %w", err)
	}

	tableNodes, err := d.tableNodes(ctx, db, cfg, req.Database, req.Keyword)
	if err != nil {
		return nil, err
	}
	for _, node := range tableNodes {
		response.Tables = append(response.Tables, dbmodel.SQLCompletionItem{
			Label:      node.Label,
			Value:      node.Label,
			Type:       node.Type,
			Database:   req.Database,
			ObjectName: node.ObjectName,
		})
	}

	if strings.TrimSpace(req.ObjectName) == "" {
		return response, nil
	}
	columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{
		AssetID:    req.AssetID,
		Database:   req.Database,
		Schema:     req.Schema,
		ObjectName: req.ObjectName,
		ObjectType: dbmodel.ObjectTypeTable,
	})
	if err != nil {
		return nil, err
	}
	for _, column := range columns {
		response.Columns = append(response.Columns, dbmodel.SQLCompletionColumn{
			Label:      column.Name,
			Value:      column.Name,
			Type:       dbmodel.ObjectTypeColumn,
			DataType:   column.Type,
			Database:   req.Database,
			Schema:     req.Schema,
			ObjectName: req.ObjectName,
			PrimaryKey: column.PrimaryKey,
		})
	}
	return response, nil
}

func (d *MySQLDriver) databaseNodes(ctx context.Context, db *sql.DB, cfg ConnConfig, keyword string) ([]dbmodel.TreeNode, error) {
	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]dbmodel.TreeNode, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if isMySQLSystemDatabase(name) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:      fmt.Sprintf("mysql:%d:%s", cfg.AssetID, name),
			Label:    name,
			Type:     dbmodel.ObjectTypeDatabase,
			AssetID:  cfg.AssetID,
			Engine:   cfg.Engine,
			Database: name,
			Leaf:     false,
		})
	}
	return nodes, rows.Err()
}

func (d *MySQLDriver) tableNodes(ctx context.Context, db *sql.DB, cfg ConnConfig, database, keyword string) ([]dbmodel.TreeNode, error) {
	rows, err := db.QueryContext(ctx, `
SELECT table_name, table_type, COALESCE(data_length, 0) + COALESCE(index_length, 0) AS table_size
FROM information_schema.tables
WHERE table_schema = ?
ORDER BY table_name`, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]dbmodel.TreeNode, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for rows.Next() {
		var name, tableType string
		var tableSize int64
		if err := rows.Scan(&name, &tableType, &tableSize); err != nil {
			return nil, err
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		nodeType := dbmodel.ObjectTypeTable
		if strings.Contains(strings.ToUpper(tableType), "VIEW") {
			nodeType = dbmodel.ObjectTypeView
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:        fmt.Sprintf("mysql:%d:%s:%s", cfg.AssetID, database, name),
			Label:      name,
			Type:       nodeType,
			AssetID:    cfg.AssetID,
			Engine:     cfg.Engine,
			Database:   database,
			ObjectName: name,
			Leaf:       false,
			Extra: map[string]interface{}{
				"sizeBytes": tableSize,
				"sizeText":  formatBytes(tableSize),
			},
		})
	}
	return nodes, rows.Err()
}

func openMySQL(cfg ConnConfig, database string) (*sql.DB, error) {
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port == 0 {
		return nil, fmt.Errorf("host and port are required")
	}
	key := mysqlPoolKey(cfg, database)
	mysqlPools.Lock()
	if entry, ok := mysqlPools.items[key]; ok {
		entry.lastActive = time.Now()
		db := entry.db
		mysqlPools.Unlock()
		return db, nil
	}

	mysqlCfg := mysqlDriver.NewConfig()
	mysqlCfg.User = cfg.Username
	mysqlCfg.Passwd = cfg.Password
	mysqlCfg.Net = "tcp"
	mysqlCfg.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	mysqlCfg.Timeout = 10 * time.Second
	mysqlCfg.ReadTimeout = 30 * time.Second
	mysqlCfg.WriteTimeout = 30 * time.Second
	mysqlCfg.ParseTime = true
	mysqlCfg.Params = map[string]string{"charset": "utf8mb4"}
	db, err := sql.Open("mysql", mysqlCfg.FormatDSN())
	if err != nil {
		mysqlPools.Unlock()
		return nil, err
	}
	db.SetMaxOpenConns(mysqlPoolMaxOpenConns)
	db.SetMaxIdleConns(mysqlPoolMaxIdleConns)
	db.SetConnMaxLifetime(mysqlPoolConnMaxLifetime)
	db.SetConnMaxIdleTime(mysqlPoolIdleTTL)
	mysqlPools.items[key] = &mysqlPoolEntry{
		db:         db,
		lastActive: time.Now(),
	}
	mysqlPools.Unlock()
	return db, nil
}

func mysqlPoolKey(cfg ConnConfig, database string) string {
	return strings.Join([]string{
		fmt.Sprint(cfg.AssetID),
		cfg.Host,
		fmt.Sprint(cfg.Port),
		cfg.Username,
	}, "|")
}

func cleanupIdleMySQLPools() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var expired []*sql.DB
		mysqlPools.Lock()
		for key, entry := range mysqlPools.items {
			if now.Sub(entry.lastActive) < mysqlPoolIdleTTL {
				continue
			}
			expired = append(expired, entry.db)
			delete(mysqlPools.items, key)
		}
		mysqlPools.Unlock()
		for _, db := range expired {
			_ = db.Close()
		}
	}
}

var mysqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9_@$-]+$`)

func validateIdentifier(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("identifier is required")
	}
	if !mysqlIdentifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier: %s", value)
	}
	return nil
}

func validateObjectRequest(req dbmodel.ObjectRequest) error {
	if req.AssetID == 0 {
		return fmt.Errorf("assetId is required")
	}
	return validateMutationObject(req.Database, req.ObjectName)
}

func validateMutationObject(database, objectName string) error {
	if err := validateIdentifier(database); err != nil {
		return fmt.Errorf("invalid database: %w", err)
	}
	if err := validateIdentifier(objectName); err != nil {
		return fmt.Errorf("invalid objectName: %w", err)
	}
	return nil
}

func quoteMySQLIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteMySQLIdentifiers(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, quoteMySQLIdentifier(value))
	}
	return result
}

func isMySQLSystemDatabase(name string) bool {
	switch strings.ToLower(name) {
	case "information_schema", "performance_schema", "mysql", "sys":
		return true
	default:
		return false
	}
}

func buildColumnMap(columns []dbmodel.ColumnInfo) map[string]dbmodel.ColumnInfo {
	result := make(map[string]dbmodel.ColumnInfo, len(columns))
	for _, column := range columns {
		result[column.Name] = column
	}
	return result
}

func selectedColumns(requested []string, columns []dbmodel.ColumnInfo, columnMap map[string]dbmodel.ColumnInfo) ([]string, error) {
	if len(requested) == 0 {
		result := make([]string, 0, len(columns))
		for _, column := range columns {
			result = append(result, column.Name)
		}
		return result, nil
	}
	result := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if _, ok := columnMap[name]; !ok {
			return nil, fmt.Errorf("unknown column: %s", name)
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("columns is empty")
	}
	return result, nil
}

func buildMySQLWhere(filters []dbmodel.DataFilter, columnMap map[string]dbmodel.ColumnInfo) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	clauses := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))
	for _, filter := range filters {
		column := strings.TrimSpace(filter.Column)
		if _, ok := columnMap[column]; !ok {
			return "", nil, fmt.Errorf("unknown filter column: %s", column)
		}
		operator := strings.ToLower(strings.TrimSpace(filter.Operator))
		switch operator {
		case "=", "!=", ">", ">=", "<", "<=":
			clauses = append(clauses, quoteMySQLIdentifier(column)+" "+operator+" ?")
			args = append(args, filter.Value)
		case "like":
			clauses = append(clauses, quoteMySQLIdentifier(column)+" LIKE ?")
			args = append(args, "%"+fmt.Sprint(filter.Value)+"%")
		case "in":
			values, err := interfaceSlice(filter.Value)
			if err != nil {
				return "", nil, err
			}
			if len(values) == 0 {
				return "", nil, fmt.Errorf("in filter value cannot be empty")
			}
			placeholders := make([]string, len(values))
			for i := range values {
				placeholders[i] = "?"
			}
			clauses = append(clauses, quoteMySQLIdentifier(column)+" IN ("+strings.Join(placeholders, ", ")+")")
			args = append(args, values...)
		case "is_null":
			clauses = append(clauses, quoteMySQLIdentifier(column)+" IS NULL")
		case "is_not_null":
			clauses = append(clauses, quoteMySQLIdentifier(column)+" IS NOT NULL")
		default:
			return "", nil, fmt.Errorf("unsupported filter operator: %s", filter.Operator)
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func buildMySQLOrder(sorts []dbmodel.DataSort, columnMap map[string]dbmodel.ColumnInfo) (string, error) {
	if len(sorts) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(sorts))
	for _, sortItem := range sorts {
		column := strings.TrimSpace(sortItem.Column)
		if _, ok := columnMap[column]; !ok {
			return "", fmt.Errorf("unknown sort column: %s", column)
		}
		order := strings.ToLower(strings.TrimSpace(sortItem.Order))
		if order != "asc" && order != "desc" {
			return "", fmt.Errorf("unsupported sort order: %s", sortItem.Order)
		}
		clauses = append(clauses, quoteMySQLIdentifier(column)+" "+strings.ToUpper(order))
	}
	return " ORDER BY " + strings.Join(clauses, ", "), nil
}

func interfaceSlice(value interface{}) ([]interface{}, error) {
	switch values := value.(type) {
	case []interface{}:
		return values, nil
	case []string:
		result := make([]interface{}, 0, len(values))
		for _, item := range values {
			result = append(result, item)
		}
		return result, nil
	case []int:
		result := make([]interface{}, 0, len(values))
		for _, item := range values {
			result = append(result, item)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("in filter value must be array")
	}
}

func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		item := make(map[string]interface{}, len(columns))
		for i, column := range columns {
			item[column] = normalizeSQLValue(values[i])
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func normalizeSQLValue(value interface{}) interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05")
	default:
		return v
	}
}

func normalizeSQLText(sqlText string) string {
	sqlText = strings.TrimSpace(sqlText)
	sqlText = strings.TrimRight(sqlText, ";")
	return strings.TrimSpace(sqlText)
}

func isReadOnlySQL(sqlText string) bool {
	trimmed := strings.TrimSpace(sqlText)
	if trimmed == "" {
		return false
	}
	upper := strings.ToUpper(trimmed)
	if strings.Contains(upper, ";") {
		return false
	}
	if readOnlySQLDeniedKeywordPattern.MatchString(trimmed) {
		return false
	}
	if strings.HasPrefix(upper, "SELECT ") || upper == "SELECT" {
		return true
	}
	return strings.HasPrefix(upper, "WITH ")
}

func estimateMySQLReadOnlySQL(ctx context.Context, db sqlQueryer, sqlText string) dbmodel.SQLExecutionEstimate {
	estimate := dbmodel.SQLExecutionEstimate{
		RiskLevel: "low",
		Message:   "本条 SQL 预计执行较快",
	}
	rows, err := db.QueryContext(ctx, "EXPLAIN "+sqlText)
	if err != nil {
		estimate.EstimatedSeconds = sqlConfirmThresholdSeconds
		estimate.RiskLevel = "unknown"
		estimate.Message = "无法获取执行计划，建议确认后再执行"
		return estimate
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return estimate
	}
	rowsIndex := -1
	for i, name := range columns {
		if strings.EqualFold(name, "rows") {
			rowsIndex = i
			break
		}
	}
	if rowsIndex < 0 {
		return estimate
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

	estimate.EstimatedRows = estimatedRows
	estimate.EstimatedSeconds = estimateSecondsByRows(estimatedRows)
	estimate.RiskLevel = estimateRiskLevel(estimate.EstimatedSeconds)
	if estimate.EstimatedSeconds >= sqlConfirmThresholdSeconds {
		estimate.Message = fmt.Sprintf("本条 SQL 执行预计耗时 %d 秒，是否要执行？", estimate.EstimatedSeconds)
	} else {
		estimate.Message = fmt.Sprintf("本条 SQL 执行预计耗时 %d 秒", estimate.EstimatedSeconds)
	}
	return estimate
}

func estimateSecondsByRows(rows int64) int {
	if rows <= 0 {
		return 1
	}
	seconds := int(rows / 100000)
	if rows%100000 != 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	return seconds
}

func estimateRiskLevel(seconds int) string {
	switch {
	case seconds >= 30:
		return "high"
	case seconds >= sqlConfirmThresholdSeconds:
		return "medium"
	default:
		return "low"
	}
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case nil:
		return 0
	case int64:
		return v
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return int64(^uint64(0) >> 1)
		}
		return int64(v)
	case []byte:
		var result int64
		_, _ = fmt.Sscan(string(v), &result)
		return result
	case string:
		var result int64
		_, _ = fmt.Sscan(v, &result)
		return result
	default:
		return 0
	}
}

func formatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(size)
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value = value / 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", size, units[unit])
	}
	if value >= 10 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}

func filterColumnInfos(columns []dbmodel.ColumnInfo, selected []string) []dbmodel.ColumnInfo {
	selectedSet := stringSet(selected)
	result := make([]dbmodel.ColumnInfo, 0, len(selected))
	for _, column := range columns {
		if _, ok := selectedSet[column.Name]; ok {
			result = append(result, column)
		}
	}
	return result
}

func primaryKeys(columns []dbmodel.ColumnInfo) []string {
	result := make([]string, 0)
	for _, column := range columns {
		if column.PrimaryKey {
			result = append(result, column.Name)
		}
	}
	return result
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validateValueMap(values map[string]interface{}, columnMap map[string]dbmodel.ColumnInfo, forbidden map[string]struct{}) ([]string, []interface{}, error) {
	names := make([]string, 0, len(values))
	for name := range values {
		name = strings.TrimSpace(name)
		if _, ok := columnMap[name]; !ok {
			return nil, nil, fmt.Errorf("unknown column: %s", name)
		}
		if _, ok := forbidden[name]; ok {
			return nil, nil, fmt.Errorf("column cannot be updated: %s", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	args := make([]interface{}, 0, len(names))
	for _, name := range names {
		args = append(args, values[name])
	}
	return names, args, nil
}

func validateKeys(keys map[string]interface{}, primaryKeys []string, columnMap map[string]dbmodel.ColumnInfo) error {
	if len(primaryKeys) == 0 {
		return fmt.Errorf("table has no primary key; update/delete is not allowed")
	}
	if len(keys) == 0 {
		return fmt.Errorf("primary key values are required")
	}
	for _, key := range primaryKeys {
		if _, ok := keys[key]; !ok {
			return fmt.Errorf("primary key is required: %s", key)
		}
		if _, ok := columnMap[key]; !ok {
			return fmt.Errorf("unknown primary key: %s", key)
		}
	}
	for key := range keys {
		if _, ok := columnMap[key]; !ok {
			return fmt.Errorf("unknown key column: %s", key)
		}
		if _, ok := stringSet(primaryKeys)[key]; !ok {
			return fmt.Errorf("only primary key conditions are allowed: %s", key)
		}
	}
	return nil
}

func buildPrimaryKeyWhere(keys map[string]interface{}, primaryKeys []string) (string, []interface{}) {
	clauses := make([]string, 0, len(primaryKeys))
	values := make([]interface{}, 0, len(primaryKeys))
	for _, key := range primaryKeys {
		clauses = append(clauses, quoteMySQLIdentifier(key)+" = ?")
		values = append(values, keys[key])
	}
	return strings.Join(clauses, " AND "), values
}
