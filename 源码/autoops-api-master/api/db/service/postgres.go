package service

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	postgresPoolMaxOpenConns    = 3
	postgresPoolMaxIdleConns    = 1
	postgresPoolConnMaxLifetime = 30 * time.Minute
	postgresPoolIdleTTL         = 10 * time.Minute
	postgresMaintenanceDatabase = "postgres"
)

var postgresPools = struct {
	sync.Mutex
	items map[string]*mysqlPoolEntry
}{
	items: map[string]*mysqlPoolEntry{},
}

func init() {
	go cleanupIdlePostgresPools()
}

type PostgreSQLDriver struct{}

func NewPostgreSQLDriver() *PostgreSQLDriver {
	return &PostgreSQLDriver{}
}

func (d *PostgreSQLDriver) Type() int {
	return cmdbmodel.DatabaseTypePostgreSQL
}

func (d *PostgreSQLDriver) Test(ctx context.Context, cfg ConnConfig) error {
	db, err := openPostgres(cfg, postgresMaintenanceDatabase)
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}

func (d *PostgreSQLDriver) Tree(ctx context.Context, cfg ConnConfig, req dbmodel.TreeQuery) ([]dbmodel.TreeNode, error) {
	db, err := openPostgres(cfg, postgresMaintenanceDatabase)
	if err != nil {
		return nil, err
	}
	level := strings.TrimSpace(req.Level)
	if level == "" || level == "root" {
		return d.databaseNodes(ctx, db, cfg, req.Keyword)
	}
	if level == dbmodel.ObjectTypeDatabase {
		return d.schemaNodes(ctx, db, cfg, req.Database, req.Keyword)
	}
	if level == dbmodel.ObjectTypeSchema {
		return d.tableNodes(ctx, db, cfg, req.Database, req.Schema, req.Keyword)
	}
	if level == dbmodel.ObjectTypeTable || level == dbmodel.ObjectTypeView {
		columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{
			AssetID:    req.AssetID,
			Database:   req.Database,
			Schema:     req.Schema,
			ObjectName: req.ObjectName,
			ObjectType: req.ObjectType,
		})
		if err != nil {
			return nil, err
		}
		nodes := make([]dbmodel.TreeNode, 0, len(columns))
		for _, column := range columns {
			nodes = append(nodes, dbmodel.TreeNode{
				Key:        fmt.Sprintf("postgres:%d:%s:%s:%s:%s", cfg.AssetID, req.Database, req.Schema, req.ObjectName, column.Name),
				Label:      column.Name,
				Type:       dbmodel.ObjectTypeColumn,
				AssetID:    cfg.AssetID,
				Engine:     cfg.Engine,
				Database:   req.Database,
				Schema:     req.Schema,
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

func (d *PostgreSQLDriver) Columns(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error) {
	if err := validatePostgresObjectRequest(req); err != nil {
		return nil, err
	}
	schema := firstNonEmpty(req.Schema, "public")
	db, err := openPostgres(cfg, req.Database)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
SELECT
  c.column_name,
  CASE
    WHEN c.data_type = 'USER-DEFINED' THEN c.udt_name
    WHEN c.character_maximum_length IS NOT NULL THEN c.data_type || '(' || c.character_maximum_length || ')'
    WHEN c.numeric_precision IS NOT NULL AND c.numeric_scale IS NOT NULL THEN c.data_type || '(' || c.numeric_precision || ',' || c.numeric_scale || ')'
    ELSE c.data_type
  END AS column_type,
  c.is_nullable,
  c.column_default,
  COALESCE(pg_catalog.col_description(format('%I.%I', c.table_schema, c.table_name)::regclass::oid, c.ordinal_position), '') AS comment,
  EXISTS (
    SELECT 1
    FROM pg_index i
    JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
    WHERE i.indrelid = format('%I.%I', c.table_schema, c.table_name)::regclass
      AND i.indisprimary
      AND a.attname = c.column_name
  ) AS primary_key
FROM information_schema.columns c
WHERE c.table_catalog = $1 AND c.table_schema = $2 AND c.table_name = $3
ORDER BY c.ordinal_position`, req.Database, schema, req.ObjectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]dbmodel.ColumnInfo, 0)
	for rows.Next() {
		var name, columnType, nullable string
		var defaultValue, comment sql.NullString
		var primaryKey bool
		if err := rows.Scan(&name, &columnType, &nullable, &defaultValue, &comment, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, dbmodel.ColumnInfo{
			Name:          name,
			Type:          columnType,
			Nullable:      strings.EqualFold(nullable, "YES"),
			DefaultValue:  defaultValue.String,
			Comment:       comment.String,
			PrimaryKey:    primaryKey,
			AutoIncrement: strings.Contains(strings.ToLower(defaultValue.String), "nextval("),
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

func (d *PostgreSQLDriver) Indexes(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error) {
	if err := validatePostgresObjectRequest(req); err != nil {
		return nil, err
	}
	schema := firstNonEmpty(req.Schema, "public")
	db, err := openPostgres(cfg, req.Database)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
SELECT
  i.relname AS index_name,
  ix.indisunique,
  ix.indisprimary,
  am.amname AS index_type,
  COALESCE(obj_description(i.oid, 'pg_class'), '') AS comment,
  ARRAY_AGG(a.attname ORDER BY array_position(ix.indkey, a.attnum)) AS columns
FROM pg_class t
JOIN pg_namespace n ON n.oid = t.relnamespace
JOIN pg_index ix ON ix.indrelid = t.oid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_am am ON am.oid = i.relam
JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ordinality) ON true
JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
WHERE t.relname = $1 AND n.nspname = $2
GROUP BY i.relname, ix.indisunique, ix.indisprimary, am.amname, i.oid
ORDER BY ix.indisprimary DESC, i.relname`, req.ObjectName, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := make([]dbmodel.IndexInfo, 0)
	for rows.Next() {
		var item dbmodel.IndexInfo
		var columns string
		if err := rows.Scan(&item.Name, &item.Unique, &item.Primary, &item.IndexType, &item.Comment, &columns); err != nil {
			return nil, err
		}
		item.Columns = parsePostgresArrayText(columns)
		indexes = append(indexes, item)
	}
	return indexes, rows.Err()
}

func (d *PostgreSQLDriver) DDL(ctx context.Context, cfg ConnConfig, req dbmodel.ObjectRequest) (string, error) {
	if err := validatePostgresObjectRequest(req); err != nil {
		return "", err
	}
	schema := firstNonEmpty(req.Schema, "public")
	if req.ObjectType == dbmodel.ObjectTypeView {
		db, err := openPostgres(cfg, req.Database)
		if err != nil {
			return "", err
		}
		var definition string
		if err := db.QueryRowContext(ctx, `
SELECT pg_get_viewdef(format('%I.%I', $1, $2)::regclass, true)`, schema, req.ObjectName).Scan(&definition); err != nil {
			return "", err
		}
		return fmt.Sprintf("CREATE OR REPLACE VIEW %s.%s AS\n%s;", quotePostgresIdentifier(schema), quotePostgresIdentifier(req.ObjectName), strings.TrimSpace(definition)), nil
	}
	columns, err := d.Columns(ctx, cfg, req)
	if err != nil {
		return "", err
	}
	indexes, _ := d.Indexes(ctx, cfg, req)
	lines := make([]string, 0, len(columns)+4)
	for _, column := range columns {
		line := fmt.Sprintf("  %s %s", quotePostgresIdentifier(column.Name), column.Type)
		if !column.Nullable {
			line += " NOT NULL"
		}
		if column.DefaultValue != "" {
			line += " DEFAULT " + column.DefaultValue
		}
		lines = append(lines, line)
	}
	for _, index := range indexes {
		if !index.Primary {
			continue
		}
		quoted := quotePostgresIdentifiers(index.Columns)
		lines = append(lines, fmt.Sprintf("  CONSTRAINT %s PRIMARY KEY (%s)", quotePostgresIdentifier(index.Name), strings.Join(quoted, ", ")))
	}
	return fmt.Sprintf("CREATE TABLE %s.%s (\n%s\n);", quotePostgresIdentifier(schema), quotePostgresIdentifier(req.ObjectName), strings.Join(lines, ",\n")), nil
}

func (d *PostgreSQLDriver) Query(ctx context.Context, cfg ConnConfig, req dbmodel.DataQueryRequest) (*dbmodel.DataResult, error) {
	if req.ObjectType == "" {
		req.ObjectType = dbmodel.ObjectTypeTable
	}
	if req.ObjectType != dbmodel.ObjectTypeTable && req.ObjectType != dbmodel.ObjectTypeView {
		return nil, fmt.Errorf("unsupported objectType for PostgreSQL query: %s", req.ObjectType)
	}
	objectReq := dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, Schema: req.Schema, ObjectName: req.ObjectName, ObjectType: req.ObjectType}
	columns, err := d.Columns(ctx, cfg, objectReq)
	if err != nil {
		return nil, err
	}
	schema := firstNonEmpty(req.Schema, "public")
	columnMap := buildColumnMap(columns)
	selected, err := selectedColumns(req.Columns, columns, columnMap)
	if err != nil {
		return nil, err
	}
	whereSQL, args, err := buildPostgresWhere(req.Filters, columnMap)
	if err != nil {
		return nil, err
	}
	orderSQL, err := buildPostgresOrder(req.Sorts, columnMap)
	if err != nil {
		return nil, err
	}
	db, err := openPostgres(cfg, req.Database)
	if err != nil {
		return nil, err
	}
	tableName := qualifyPostgresTable(schema, req.ObjectName)
	countSQL := "SELECT COUNT(1) FROM " + tableName + whereSQL
	var total int64
	if err := db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	selectSQL := "SELECT " + strings.Join(quotePostgresIdentifiers(selected), ", ") + " FROM " + tableName + whereSQL + orderSQL + fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
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
	pks := primaryKeys(columns)
	return &dbmodel.DataResult{
		Columns:     filterColumnInfos(columns, selected),
		Rows:        dataRows,
		PrimaryKeys: pks,
		Page:        page,
		PageSize:    pageSize,
		Total:       total,
		Editable:    false,
	}, nil
}

func (d *PostgreSQLDriver) Create(context.Context, ConnConfig, dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("PostgreSQL工作台暂不支持新增数据")
}

func (d *PostgreSQLDriver) Update(context.Context, ConnConfig, dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("PostgreSQL工作台暂不支持更新数据")
}

func (d *PostgreSQLDriver) Delete(context.Context, ConnConfig, dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error) {
	return nil, fmt.Errorf("PostgreSQL工作台暂不支持删除数据")
}

func (d *PostgreSQLDriver) ExecuteSQL(ctx context.Context, cfg ConnConfig, req dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error) {
	sqlText := normalizeSQLText(req.SQL)
	if sqlText == "" {
		return nil, fmt.Errorf("sql is required")
	}
	if !isReadOnlySQL(sqlText) {
		return nil, fmt.Errorf("only SELECT/WITH read-only SQL is allowed")
	}
	dbName := firstNonEmpty(req.Database, cfg.DatabaseName)
	if err := validatePostgresIdentifier(dbName); err != nil {
		return nil, fmt.Errorf("invalid database: %w", err)
	}
	db, err := openPostgres(cfg, dbName)
	if err != nil {
		return nil, err
	}
	estimate := estimatePostgresReadOnlySQL(ctx, db, sqlText)
	if estimate.EstimatedSeconds >= sqlConfirmThresholdSeconds && !req.Confirmed.Bool() {
		return &dbmodel.SQLExecuteResult{
			SQL:            sqlText,
			ReadOnly:       true,
			RequireConfirm: true,
			Estimate:       estimate,
		}, nil
	}
	page, pageSize := dbmodel.NormalizePage(req.Page, req.PageSize)
	pagedSQL := fmt.Sprintf("SELECT * FROM (%s) AS db_query_result LIMIT $1 OFFSET $2", sqlText)
	start := time.Now()
	rows, err := db.QueryContext(ctx, pagedSQL, pageSize, (page-1)*pageSize)
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
	if err := db.QueryRowContext(ctx, countSQL).Scan(&total); err != nil {
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

func (d *PostgreSQLDriver) Completion(ctx context.Context, cfg ConnConfig, req dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error) {
	response := &dbmodel.SQLCompletionResponse{Keywords: append([]string{}, sqlKeywords...)}
	db, err := openPostgres(cfg, postgresMaintenanceDatabase)
	if err != nil {
		return nil, err
	}
	databases, err := d.databaseNodes(ctx, db, cfg, req.Keyword)
	if err != nil {
		return nil, err
	}
	for _, node := range databases {
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
	db, err = openPostgres(cfg, req.Database)
	if err != nil {
		return nil, err
	}
	schema := firstNonEmpty(req.Schema, "public")
	tables, err := d.tableNodes(ctx, db, cfg, req.Database, schema, req.Keyword)
	if err != nil {
		return nil, err
	}
	for _, node := range tables {
		response.Tables = append(response.Tables, dbmodel.SQLCompletionItem{
			Label:      node.Label,
			Value:      node.Label,
			Type:       node.Type,
			Database:   req.Database,
			Schema:     schema,
			ObjectName: node.ObjectName,
		})
	}
	if strings.TrimSpace(req.ObjectName) == "" {
		return response, nil
	}
	columns, err := d.Columns(ctx, cfg, dbmodel.ObjectRequest{AssetID: req.AssetID, Database: req.Database, Schema: schema, ObjectName: req.ObjectName, ObjectType: dbmodel.ObjectTypeTable})
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
			Schema:     schema,
			ObjectName: req.ObjectName,
			PrimaryKey: column.PrimaryKey,
		})
	}
	return response, nil
}

func (d *PostgreSQLDriver) databaseNodes(ctx context.Context, db *sql.DB, cfg ConnConfig, keyword string) ([]dbmodel.TreeNode, error) {
	rows, err := db.QueryContext(ctx, `
SELECT datname
FROM pg_database
WHERE datallowconn = true AND datistemplate = false
ORDER BY datname`)
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
		if isPostgresSystemDatabase(name) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:      fmt.Sprintf("postgres:%d:%s", cfg.AssetID, name),
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

func (d *PostgreSQLDriver) schemaNodes(ctx context.Context, db *sql.DB, cfg ConnConfig, database, keyword string) ([]dbmodel.TreeNode, error) {
	db, err := openPostgres(cfg, database)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
SELECT schema_name
FROM information_schema.schemata
WHERE catalog_name = $1
ORDER BY schema_name`, database)
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
		if isPostgresSystemSchema(name) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:      fmt.Sprintf("postgres:%d:%s:%s", cfg.AssetID, database, name),
			Label:    name,
			Type:     dbmodel.ObjectTypeSchema,
			AssetID:  cfg.AssetID,
			Engine:   cfg.Engine,
			Database: database,
			Schema:   name,
			Leaf:     false,
		})
	}
	return nodes, rows.Err()
}

func (d *PostgreSQLDriver) tableNodes(ctx context.Context, db *sql.DB, cfg ConnConfig, database, schema, keyword string) ([]dbmodel.TreeNode, error) {
	db, err := openPostgres(cfg, database)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
SELECT c.relname,
       c.relkind,
       pg_total_relation_size(c.oid) AS size_bytes
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1 AND c.relkind IN ('r','p','v','m')
ORDER BY c.relname`, schema)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	nodes := make([]dbmodel.TreeNode, 0)
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	for rows.Next() {
		var name, kind string
		var tableSize int64
		if err := rows.Scan(&name, &kind, &tableSize); err != nil {
			return nil, err
		}
		if keyword != "" && !strings.Contains(strings.ToLower(name), keyword) {
			continue
		}
		nodeType := dbmodel.ObjectTypeTable
		if kind == "v" || kind == "m" {
			nodeType = dbmodel.ObjectTypeView
		}
		nodes = append(nodes, dbmodel.TreeNode{
			Key:        fmt.Sprintf("postgres:%d:%s:%s:%s", cfg.AssetID, database, schema, name),
			Label:      name,
			Type:       nodeType,
			AssetID:    cfg.AssetID,
			Engine:     cfg.Engine,
			Database:   database,
			Schema:     schema,
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

func openPostgres(cfg ConnConfig, database string) (*sql.DB, error) {
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port == 0 {
		return nil, fmt.Errorf("host and port are required")
	}
	dbName := firstNonEmpty(database, cfg.DatabaseName, "postgres")
	key := postgresPoolKey(cfg, dbName)
	postgresPools.Lock()
	if entry, ok := postgresPools.items[key]; ok {
		entry.lastActive = time.Now()
		db := entry.db
		postgresPools.Unlock()
		return db, nil
	}
	sslMode := "disable"
	if cfg.SSLEnabled {
		sslMode = "require"
	}
	connURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port)),
		Path:   dbName,
	}
	values := connURL.Query()
	values.Set("sslmode", sslMode)
	values.Set("connect_timeout", "10")
	connURL.RawQuery = values.Encode()
	db, err := sql.Open("pgx", connURL.String())
	if err != nil {
		postgresPools.Unlock()
		return nil, err
	}
	db.SetMaxOpenConns(postgresPoolMaxOpenConns)
	db.SetMaxIdleConns(postgresPoolMaxIdleConns)
	db.SetConnMaxLifetime(postgresPoolConnMaxLifetime)
	db.SetConnMaxIdleTime(postgresPoolIdleTTL)
	postgresPools.items[key] = &mysqlPoolEntry{db: db, lastActive: time.Now()}
	postgresPools.Unlock()
	return db, nil
}

func postgresPoolKey(cfg ConnConfig, database string) string {
	return strings.Join([]string{
		fmt.Sprint(cfg.AssetID),
		cfg.Host,
		fmt.Sprint(cfg.Port),
		cfg.Username,
		database,
	}, "|")
}

func cleanupIdlePostgresPools() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		var expired []*sql.DB
		postgresPools.Lock()
		for key, entry := range postgresPools.items {
			if now.Sub(entry.lastActive) < postgresPoolIdleTTL {
				continue
			}
			expired = append(expired, entry.db)
			delete(postgresPools.items, key)
		}
		postgresPools.Unlock()
		for _, db := range expired {
			_ = db.Close()
		}
	}
}

var postgresIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

func validatePostgresIdentifier(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("identifier is required")
	}
	if !postgresIdentifierPattern.MatchString(value) {
		return fmt.Errorf("invalid identifier: %s", value)
	}
	return nil
}

func validatePostgresObjectRequest(req dbmodel.ObjectRequest) error {
	if req.AssetID == 0 {
		return fmt.Errorf("assetId is required")
	}
	if err := validatePostgresIdentifier(req.Database); err != nil {
		return fmt.Errorf("invalid database: %w", err)
	}
	if err := validatePostgresIdentifier(firstNonEmpty(req.Schema, "public")); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}
	if err := validatePostgresIdentifier(req.ObjectName); err != nil {
		return fmt.Errorf("invalid objectName: %w", err)
	}
	return nil
}

func quotePostgresIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quotePostgresIdentifiers(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, quotePostgresIdentifier(value))
	}
	return result
}

func qualifyPostgresTable(schema, table string) string {
	return quotePostgresIdentifier(firstNonEmpty(schema, "public")) + "." + quotePostgresIdentifier(table)
}

func buildPostgresWhere(filters []dbmodel.DataFilter, columnMap map[string]dbmodel.ColumnInfo) (string, []interface{}, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	clauses := make([]string, 0, len(filters))
	args := make([]interface{}, 0, len(filters))
	for _, filter := range filters {
		column := strings.TrimSpace(filter.Column)
		columnInfo, ok := columnMap[column]
		if !ok {
			return "", nil, fmt.Errorf("unknown filter column: %s", column)
		}
		operator := strings.ToLower(strings.TrimSpace(filter.Operator))
		switch operator {
		case "=", "!=", ">", ">=", "<", "<=":
			expr, value, impossible := normalizePostgresFilterExpr(column, columnInfo, operator, filter.Value)
			if impossible {
				clauses = append(clauses, "1 = 0")
				continue
			}
			args = append(args, value)
			clauses = append(clauses, fmt.Sprintf("%s %s $%d", expr, operator, len(args)))
		case "like":
			args = append(args, "%"+fmt.Sprint(filter.Value)+"%")
			clauses = append(clauses, fmt.Sprintf("%s::text ILIKE $%d", quotePostgresIdentifier(column), len(args)))
		case "in":
			values, err := interfaceSlice(filter.Value)
			if err != nil {
				return "", nil, err
			}
			if len(values) == 0 {
				return "", nil, fmt.Errorf("in filter value cannot be empty")
			}
			placeholders := make([]string, 0, len(values))
			expr := quotePostgresIdentifier(column)
			if shouldComparePostgresValuesAsText(columnInfo, values) {
				expr += "::text"
			}
			for _, value := range values {
				if strings.HasSuffix(expr, "::text") {
					value = fmt.Sprint(value)
				}
				args = append(args, value)
				placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
			}
			clauses = append(clauses, expr+" IN ("+strings.Join(placeholders, ", ")+")")
		case "is_null":
			clauses = append(clauses, quotePostgresIdentifier(column)+" IS NULL")
		case "is_not_null":
			clauses = append(clauses, quotePostgresIdentifier(column)+" IS NOT NULL")
		default:
			return "", nil, fmt.Errorf("unsupported filter operator: %s", filter.Operator)
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func buildPostgresOrder(sorts []dbmodel.DataSort, columnMap map[string]dbmodel.ColumnInfo) (string, error) {
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
		clauses = append(clauses, quotePostgresIdentifier(column)+" "+strings.ToUpper(order))
	}
	return " ORDER BY " + strings.Join(clauses, ", "), nil
}

func normalizePostgresFilterExpr(column string, columnInfo dbmodel.ColumnInfo, operator string, value interface{}) (string, interface{}, bool) {
	expr := quotePostgresIdentifier(column)
	if isPostgresTextType(columnInfo.Type) {
		return expr, value, false
	}
	textValue, isString := value.(string)
	if !isString {
		return expr, value, false
	}
	textValue = strings.TrimSpace(textValue)
	if canCoercePostgresValue(columnInfo.Type, textValue) {
		return expr, value, false
	}
	if operator == "=" || operator == "!=" {
		return expr + "::text", textValue, false
	}
	return "", nil, true
}

func shouldComparePostgresValuesAsText(columnInfo dbmodel.ColumnInfo, values []interface{}) bool {
	if isPostgresTextType(columnInfo.Type) {
		return false
	}
	for _, value := range values {
		textValue, ok := value.(string)
		if !ok {
			continue
		}
		if !canCoercePostgresValue(columnInfo.Type, strings.TrimSpace(textValue)) {
			return true
		}
	}
	return false
}

func canCoercePostgresValue(dataType, value string) bool {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	switch {
	case isPostgresNumericType(dataType):
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	case isPostgresBoolType(dataType):
		_, err := strconv.ParseBool(value)
		if err == nil {
			return true
		}
		switch strings.ToLower(value) {
		case "t", "f", "yes", "no", "on", "off", "1", "0":
			return true
		default:
			return false
		}
	default:
		return true
	}
}

func isPostgresTextType(dataType string) bool {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	if dataType == "" {
		return true
	}
	return strings.Contains(dataType, "char") ||
		strings.Contains(dataType, "text") ||
		strings.Contains(dataType, "uuid") ||
		strings.Contains(dataType, "json") ||
		strings.Contains(dataType, "xml") ||
		strings.Contains(dataType, "inet") ||
		strings.Contains(dataType, "cidr") ||
		strings.Contains(dataType, "macaddr")
}

func isPostgresNumericType(dataType string) bool {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	return strings.Contains(dataType, "int") ||
		strings.Contains(dataType, "serial") ||
		strings.Contains(dataType, "numeric") ||
		strings.Contains(dataType, "decimal") ||
		strings.Contains(dataType, "real") ||
		strings.Contains(dataType, "double") ||
		strings.Contains(dataType, "money")
}

func isPostgresBoolType(dataType string) bool {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	return dataType == "boolean" || dataType == "bool"
}

func estimatePostgresReadOnlySQL(ctx context.Context, db sqlQueryer, sqlText string) dbmodel.SQLExecutionEstimate {
	estimate := dbmodel.SQLExecutionEstimate{RiskLevel: "low", Message: "本条 SQL 预计执行较快"}
	rows, err := db.QueryContext(ctx, "EXPLAIN (FORMAT JSON) "+sqlText)
	if err != nil {
		estimate.EstimatedSeconds = sqlConfirmThresholdSeconds
		estimate.RiskLevel = "unknown"
		estimate.Message = "无法获取执行计划，建议确认后再执行"
		return estimate
	}
	defer rows.Close()
	estimate.EstimatedSeconds = 1
	estimate.RiskLevel = "low"
	estimate.Message = "本条 SQL 预计执行较快"
	return estimate
}

func parsePostgresArrayText(value string) []string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "{")
	value = strings.TrimSuffix(value, "}")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `"`)
		if part != "" {
			result = append(result, part)
		}
	}
	sort.Strings(result)
	return result
}

func isPostgresSystemDatabase(name string) bool {
	switch strings.ToLower(name) {
	case "template0", "template1":
		return true
	default:
		return false
	}
}

func isPostgresSystemSchema(name string) bool {
	if strings.HasPrefix(strings.ToLower(name), "pg_") {
		return true
	}
	return strings.EqualFold(name, "information_schema")
}
