package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	aiopsservice "dodevops-api/api/aiops/service"
	cmdbmodel "dodevops-api/api/cmdb/model"
	accountdao "dodevops-api/api/configcenter/dao"
	dbdao "dodevops-api/api/db/dao"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common/util"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

const maxCSVExportRows = 10000

const sqlAssistantSystemPrompt = "You are an expert database engineer and SQL assistant. Answer in Chinese. Return only valid JSON."

type DatabaseService struct {
	assetDao       *dbdao.AssetDao
	accountDao     *accountdao.AccountAuthDao
	auditDao       *dbdao.AuditDao
	slowLogTaskDao *dbdao.SlowLogTaskDao
}

func NewDatabaseService() *DatabaseService {
	return &DatabaseService{
		assetDao:       dbdao.NewAssetDao(),
		accountDao:     accountdao.NewAccountAuthDao(),
		auditDao:       dbdao.NewAuditDao(),
		slowLogTaskDao: dbdao.NewSlowLogTaskDao(),
	}
}

func (s *DatabaseService) ListAssets(query dbmodel.AssetQuery) ([]dbmodel.AssetVO, int64, error) {
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	assets, total, err := s.assetDao.List(query)
	if err != nil {
		return nil, 0, err
	}
	s.fillAssetPrometheusInfo(assets)
	return buildAssetVOs(assets), total, nil
}

func (s *DatabaseService) Ping(ctx *gin.Context, assetID uint) (*dbmodel.PingResult, error) {
	asset, cfg, err := s.BuildConfig(assetID, "")
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionPing, map[string]interface{}{"assetId": assetID})
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	err = driver.Test(contextFromGin(ctx), cfg)
	if err != nil {
		return &dbmodel.PingResult{AssetID: assetID, Success: false, Message: err.Error()}, nil
	}
	return &dbmodel.PingResult{AssetID: assetID, Success: true, Message: "ping success"}, nil
}

func (s *DatabaseService) Tree(ctx *gin.Context, req dbmodel.TreeQuery) ([]dbmodel.TreeNode, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionMetadata, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	return driver.Tree(contextFromGin(ctx), cfg, req)
}

func (s *DatabaseService) Columns(ctx *gin.Context, req dbmodel.ObjectRequest) ([]dbmodel.ColumnInfo, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionMetadata, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	return driver.Columns(contextFromGin(ctx), cfg, req)
}

func (s *DatabaseService) Indexes(ctx *gin.Context, req dbmodel.ObjectRequest) ([]dbmodel.IndexInfo, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionMetadata, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	return driver.Indexes(contextFromGin(ctx), cfg, req)
}

func (s *DatabaseService) DDL(ctx *gin.Context, req dbmodel.ObjectRequest) (string, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return "", err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionMetadata, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return "", err
	}
	return driver.DDL(contextFromGin(ctx), cfg, req)
}

func (s *DatabaseService) Query(ctx *gin.Context, req dbmodel.DataQueryRequest) (*dbmodel.DataResult, error) {
	req.Page, req.PageSize = dbmodel.NormalizePage(req.Page, req.PageSize)
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionQuery, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	recorder.SetWhere(req.Filters)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	data, err := driver.Query(contextFromGin(ctx), cfg, req)
	if data != nil {
		recorder.SetRowsAffected(int64(len(data.Rows)))
	}
	return data, err
}

func (s *DatabaseService) ExportCSV(ctx *gin.Context, req dbmodel.DataQueryRequest, writer io.Writer) (int64, error) {
	if req.PageSize <= 0 || req.PageSize > dbmodel.MaxPageSize {
		req.PageSize = dbmodel.MaxPageSize
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return 0, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionExport, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	recorder.SetWhere(req.Filters)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return 0, err
	}

	csvWriter := csv.NewWriter(writer)
	var exported int64
	headerWritten := false
	for exported < maxCSVExportRows {
		data, queryErr := driver.Query(contextFromGin(ctx), cfg, req)
		if queryErr != nil {
			err = queryErr
			return exported, err
		}
		if data == nil || len(data.Rows) == 0 {
			break
		}
		if !headerWritten {
			header := make([]string, 0, len(data.Columns))
			for _, column := range data.Columns {
				header = append(header, column.Name)
			}
			if writeErr := csvWriter.Write(header); writeErr != nil {
				err = writeErr
				return exported, err
			}
			headerWritten = true
		}
		for _, row := range data.Rows {
			record := make([]string, 0, len(data.Columns))
			for _, column := range data.Columns {
				record = append(record, fmt.Sprint(row[column.Name]))
			}
			if writeErr := csvWriter.Write(record); writeErr != nil {
				err = writeErr
				return exported, err
			}
			exported++
			if exported >= maxCSVExportRows {
				break
			}
		}
		if !req.ExportAll {
			break
		}
		if int64(req.Page*req.PageSize) >= data.Total || len(data.Rows) < req.PageSize {
			break
		}
		req.Page++
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		return exported, err
	}
	recorder.SetRowsAffected(exported)
	return exported, nil
}

func (s *DatabaseService) Create(ctx *gin.Context, req dbmodel.DataCreateRequest) (*dbmodel.MutationResult, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionCreate, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	data, err := driver.Create(contextFromGin(ctx), cfg, req)
	if data != nil {
		recorder.SetRowsAffected(data.RowsAffected)
	}
	return data, err
}

func (s *DatabaseService) Update(ctx *gin.Context, req dbmodel.DataUpdateRequest) (*dbmodel.MutationResult, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionUpdate, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	recorder.SetWhere(req.Keys)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	data, err := driver.Update(contextFromGin(ctx), cfg, req)
	if data != nil {
		recorder.SetRowsAffected(data.RowsAffected)
	}
	return data, err
}

func (s *DatabaseService) Delete(ctx *gin.Context, req dbmodel.DataDeleteRequest) (*dbmodel.MutationResult, error) {
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionDelete, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, req.ObjectType)
	recorder.SetWhere(req.Keys)
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	data, err := driver.Delete(contextFromGin(ctx), cfg, req)
	if data != nil {
		recorder.SetRowsAffected(data.RowsAffected)
	}
	return data, err
}

func (s *DatabaseService) ExecuteSQL(ctx *gin.Context, req dbmodel.SQLExecuteRequest) (*dbmodel.SQLExecuteResult, error) {
	req.Page, req.PageSize = dbmodel.NormalizePage(req.Page, req.PageSize)
	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionSQLExec, req)
	recorder.SetObject(req.Database, req.Schema, "", "sql")
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	data, err := driver.ExecuteSQL(contextFromGin(ctx), cfg, req)
	if data != nil {
		recorder.SetRowsAffected(int64(len(data.Rows)))
	}
	return data, err
}

func (s *DatabaseService) Completion(ctx *gin.Context, query dbmodel.SQLCompletionQuery) (*dbmodel.SQLCompletionResponse, error) {
	asset, cfg, err := s.BuildConfig(query.AssetID, query.Database)
	if err != nil {
		return nil, err
	}
	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	return driver.Completion(contextFromGin(ctx), cfg, query)
}

func (s *DatabaseService) SQLAssistant(ctx *gin.Context, req dbmodel.SQLAssistantRequest) (*dbmodel.SQLAssistantResult, error) {
	req.Mode = normalizeSQLAssistantMode(req.Mode)
	if req.AssetID == 0 {
		return nil, fmt.Errorf("assetId is required")
	}
	if strings.TrimSpace(req.Input) == "" && strings.TrimSpace(req.SQL) == "" {
		return nil, fmt.Errorf("input or sql is required")
	}
	if (req.Mode == dbmodel.SQLAssistantModeOptimize || req.Mode == dbmodel.SQLAssistantModeExplain) && strings.TrimSpace(req.SQL) == "" {
		return nil, fmt.Errorf("sql is required")
	}

	asset, cfg, err := s.BuildConfig(req.AssetID, req.Database)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionAI, req)
	recorder.SetObject(req.Database, req.Schema, req.ObjectName, firstNonEmpty(req.ObjectType, "sql"))
	defer func() { recorder.Finish(err) }()

	driver, err := getDriver(asset.Type)
	if err != nil {
		return nil, err
	}
	completion, metaErr := driver.Completion(contextFromGin(ctx), cfg, dbmodel.SQLCompletionQuery{
		AssetID:    req.AssetID,
		Database:   req.Database,
		Schema:     req.Schema,
		ObjectName: req.ObjectName,
	})
	if metaErr != nil {
		return nil, fmt.Errorf("build sql assistant metadata failed: %w", metaErr)
	}

	provider, apiKey, err := aiopsservice.ResolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}

	prompt := buildSQLAssistantPrompt(req, asset.Type, completion)
	llmCtx, cancel := context.WithTimeout(contextFromGin(ctx), llmTimeout(provider.TimeoutSeconds))
	defer cancel()

	answer, err := aiopsservice.CallLLM(llmCtx, provider, apiKey, sqlAssistantSystemPrompt, prompt)
	if err != nil {
		return nil, err
	}

	data := parseSQLAssistantAnswer(answer)
	data.Mode = req.Mode
	data.RawAnswer = answer
	data.UsedLLM = true
	data.ProviderID = provider.ID
	data.ProviderName = provider.Name
	data.ModelCode = provider.DefaultModel
	return data, nil
}

func (s *DatabaseService) SQLHistory(query dbmodel.AuditQuery) ([]dbmodel.OperationAudit, int64, error) {
	query.Action = dbmodel.ActionSQLExec
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	return s.auditDao.List(query)
}

func (s *DatabaseService) ListAudits(query dbmodel.AuditQuery) ([]dbmodel.OperationAudit, int64, error) {
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	return s.auditDao.List(query)
}

func (s *DatabaseService) GetAudit(id uint) (*dbmodel.OperationAudit, error) {
	return s.auditDao.GetByID(id)
}

func (s *DatabaseService) BuildConfig(assetID uint, databaseName string) (*cmdbmodel.CmdbSQL, ConnConfig, error) {
	if assetID == 0 {
		return nil, ConnConfig{}, fmt.Errorf("assetId is required")
	}

	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, ConnConfig{}, err
	}
	if asset.AuthStatus != cmdbmodel.DatabaseAuthStatusSuccess {
		return nil, ConnConfig{}, fmt.Errorf("database asset auth status is not success")
	}
	if asset.AccountID == 0 {
		return nil, ConnConfig{}, fmt.Errorf("database asset has no accountId")
	}

	account, err := s.accountDao.GetByID(asset.AccountID)
	if err != nil {
		return nil, ConnConfig{}, fmt.Errorf("get account auth failed: %w", err)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		return nil, ConnConfig{}, fmt.Errorf("decrypt account password failed: %w", err)
	}

	host := firstNonEmpty(asset.HostIP, account.Host, asset.Endpoint)
	port := asset.Port
	if port == 0 {
		port = account.Port
	}
	if port == 0 {
		port = defaultPort(asset.Type)
	}
	dbName := strings.TrimSpace(databaseName)
	if dbName == "" {
		dbName = firstNonEmpty(asset.Name, asset.InstanceName)
	}

	cfg := ConnConfig{
		AssetID:      asset.ID,
		Engine:       asset.Type,
		Name:         asset.Name,
		Host:         host,
		Port:         port,
		Endpoint:     asset.Endpoint,
		Username:     account.Name,
		Password:     password,
		DatabaseName: dbName,
		SSLEnabled:   asset.SSLEnabled,
	}
	return asset, cfg, nil
}

type AuditRecorder struct {
	dao       *dbdao.AuditDao
	ctx       *gin.Context
	audit     dbmodel.OperationAudit
	startTime time.Time
}

func NewAuditRecorder(ctx *gin.Context, assetID uint, engine int, action string, req interface{}) *AuditRecorder {
	recorder := &AuditRecorder{
		dao:       dbdao.NewAuditDao(),
		ctx:       ctx,
		startTime: time.Now(),
		audit: dbmodel.OperationAudit{
			AssetID:   assetID,
			Engine:    engine,
			Action:    action,
			CreatedAt: util.HTime{Time: time.Now()},
		},
	}
	recorder.audit.RequestBody = encodeAuditJSON(req)
	if ctx != nil {
		recorder.audit.ClientIP = ctx.ClientIP()
		if admin, err := jwt.GetAdmin(ctx); err == nil && admin != nil {
			recorder.audit.OperatorID = admin.ID
			recorder.audit.OperatorName = admin.Username
		}
	}
	return recorder
}

func (r *AuditRecorder) SetObject(database, schema, objectName, objectType string) {
	if r == nil {
		return
	}
	r.audit.DatabaseName = database
	r.audit.SchemaName = schema
	r.audit.ObjectName = objectName
	r.audit.ObjectType = objectType
}

func (r *AuditRecorder) SetWhere(where interface{}) {
	if r == nil {
		return
	}
	r.audit.WhereJSON = encodeAuditJSON(where)
}

func (r *AuditRecorder) SetRowsAffected(rows int64) {
	if r == nil {
		return
	}
	r.audit.RowsAffected = rows
}

func (r *AuditRecorder) Finish(err error) {
	if r == nil {
		return
	}
	r.audit.DurationMS = time.Since(r.startTime).Milliseconds()
	r.audit.Success = err == nil
	if err != nil {
		r.audit.ErrorMessage = err.Error()
	}
	_ = r.dao.Create(&r.audit)
}

func buildAssetVOs(assets []cmdbmodel.CmdbSQL) []dbmodel.AssetVO {
	result := make([]dbmodel.AssetVO, 0, len(assets))
	for _, asset := range assets {
		result = append(result, buildAssetVO(asset))
	}
	return result
}

func buildAssetVO(asset cmdbmodel.CmdbSQL) dbmodel.AssetVO {
	return dbmodel.AssetVO{
		ID:                      asset.ID,
		Name:                    asset.Name,
		InstanceName:            asset.InstanceName,
		Type:                    asset.Type,
		TypeName:                databaseTypeName(asset.Type),
		Version:                 asset.Version,
		Environment:             asset.Environment,
		Status:                  asset.Status,
		AuthStatus:              asset.AuthStatus,
		AuthStatusName:          databaseAuthStatusName(asset.AuthStatus),
		HostIP:                  asset.HostIP,
		Endpoint:                asset.Endpoint,
		Port:                    asset.Port,
		BusinessSystem:          asset.BusinessSystem,
		Owner:                   asset.Owner,
		DBAOwner:                asset.DBAOwner,
		OpsOwner:                asset.OpsOwner,
		SlowLogPath:             asset.SlowLogPath,
		PrometheusAccountID:     asset.PrometheusAccountID,
		PrometheusAccountAlias:  asset.PrometheusAccountAlias,
		PrometheusEndpoint:      asset.PrometheusEndpoint,
		PrometheusLabelSelector: asset.PrometheusLabelSelector,
	}
}

func (s *DatabaseService) fillAssetPrometheusInfo(assets []cmdbmodel.CmdbSQL) {
	for i := range assets {
		accountID := assets[i].PrometheusAccountID
		if accountID == 0 {
			continue
		}
		account, err := s.accountDao.GetByID(accountID)
		if err != nil || account == nil {
			continue
		}
		assets[i].PrometheusAccountAlias = account.Alias
		assets[i].PrometheusEndpoint = buildPrometheusEndpoint(account.Host, account.Port)
	}
}

func databaseTypeName(dbType int) string {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return "MySQL"
	case cmdbmodel.DatabaseTypePostgreSQL:
		return "PostgreSQL"
	case cmdbmodel.DatabaseTypeOracle:
		return "Oracle"
	case cmdbmodel.DatabaseTypeRedis:
		return "Redis"
	case cmdbmodel.DatabaseTypeMongoDB:
		return "MongoDB"
	case cmdbmodel.DatabaseTypeSQLServer:
		return "SQL Server"
	case cmdbmodel.DatabaseTypeElasticsearch:
		return "Elasticsearch"
	default:
		return ""
	}
}

func databaseAuthStatusName(status int) string {
	switch status {
	case cmdbmodel.DatabaseAuthStatusUnauthenticated:
		return "未认证"
	case cmdbmodel.DatabaseAuthStatusFailed:
		return "认证失败"
	case cmdbmodel.DatabaseAuthStatusSuccess:
		return "认证成功"
	default:
		return ""
	}
}

func normalizeSQLAssistantMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case dbmodel.SQLAssistantModeOptimize:
		return dbmodel.SQLAssistantModeOptimize
	case dbmodel.SQLAssistantModeExplain:
		return dbmodel.SQLAssistantModeExplain
	default:
		return dbmodel.SQLAssistantModeTextToSQL
	}
}

func buildSQLAssistantPrompt(req dbmodel.SQLAssistantRequest, dbType int, completion *dbmodel.SQLCompletionResponse) string {
	metaJSON, _ := json.Marshal(buildSQLAssistantMetadata(completion))
	var builder strings.Builder
	builder.WriteString("请基于给定数据库上下文完成 SQL 助手任务。\n")
	builder.WriteString("数据库类型: ")
	builder.WriteString(databaseTypeName(dbType))
	builder.WriteString("\n当前库: ")
	builder.WriteString(req.Database)
	builder.WriteString("\n当前 schema: ")
	builder.WriteString(req.Schema)
	builder.WriteString("\n当前对象: ")
	builder.WriteString(req.ObjectName)
	builder.WriteString("\n任务模式: ")
	builder.WriteString(req.Mode)
	builder.WriteString("\n\n元数据 JSON:\n")
	builder.Write(metaJSON)
	builder.WriteString("\n\n规则:\n")
	builder.WriteString("1. 只使用元数据里存在的库、表、字段，不要臆造字段。\n")
	builder.WriteString("2. text_to_sql 模式默认生成只读 SELECT/WITH SQL，并尽量补充 LIMIT。\n")
	builder.WriteString("3. optimize 模式输出优化后的 SQL、原因、风险和索引建议。\n")
	builder.WriteString("4. explain 模式解释 SQL 语义、潜在风险、性能瓶颈和优化方向。\n")
	builder.WriteString("5. 如果用户要求危险 DDL/DML，请在 warnings 中说明需要走 SQL 审核，不要直接给 destructive SQL。\n")
	builder.WriteString("6. 返回必须是 JSON，不要包裹 Markdown 代码块。\n")
	builder.WriteString("JSON 格式: {\"sql\":\"\",\"explanation\":\"\",\"warnings\":[],\"indexes\":[],\"confidence\":\"high|medium|low\"}\n")
	if strings.TrimSpace(req.CustomPrompt) != "" {
		builder.WriteString("\n自定义要求:\n")
		builder.WriteString(req.CustomPrompt)
		builder.WriteString("\n")
	}
	if strings.TrimSpace(req.SQL) != "" {
		builder.WriteString("\n待处理 SQL:\n")
		builder.WriteString(req.SQL)
		builder.WriteString("\n")
	}
	if strings.TrimSpace(req.Input) != "" {
		builder.WriteString("\n用户输入:\n")
		builder.WriteString(req.Input)
		builder.WriteString("\n")
	}
	return builder.String()
}

func buildSQLAssistantMetadata(completion *dbmodel.SQLCompletionResponse) map[string]interface{} {
	if completion == nil {
		return map[string]interface{}{}
	}
	tables := make([]map[string]interface{}, 0, len(completion.Tables))
	for _, table := range completion.Tables {
		tables = append(tables, map[string]interface{}{
			"name":     table.Value,
			"type":     table.Type,
			"database": table.Database,
			"schema":   table.Schema,
		})
		if len(tables) >= 80 {
			break
		}
	}
	columns := make([]map[string]interface{}, 0, len(completion.Columns))
	for _, column := range completion.Columns {
		columns = append(columns, map[string]interface{}{
			"name":       column.Value,
			"dataType":   column.DataType,
			"table":      column.ObjectName,
			"database":   column.Database,
			"schema":     column.Schema,
			"primaryKey": column.PrimaryKey,
		})
		if len(columns) >= 200 {
			break
		}
	}
	return map[string]interface{}{
		"databases": completion.Databases,
		"tables":    tables,
		"columns":   columns,
		"keywords":  completion.Keywords,
	}
}

func parseSQLAssistantAnswer(answer string) *dbmodel.SQLAssistantResult {
	result := &dbmodel.SQLAssistantResult{
		RawAnswer:   strings.TrimSpace(answer),
		Warnings:    []string{},
		Indexes:     []string{},
		Confidence:  "medium",
		Explanation: strings.TrimSpace(answer),
	}
	raw := strings.TrimSpace(answer)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	var parsed struct {
		SQL         string   `json:"sql"`
		Explanation string   `json:"explanation"`
		Warnings    []string `json:"warnings"`
		Indexes     []string `json:"indexes"`
		Confidence  string   `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return result
	}
	result.SQL = strings.TrimSpace(parsed.SQL)
	result.Explanation = strings.TrimSpace(parsed.Explanation)
	if result.Explanation == "" {
		result.Explanation = result.RawAnswer
	}
	result.Warnings = parsed.Warnings
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	result.Indexes = parsed.Indexes
	if result.Indexes == nil {
		result.Indexes = []string{}
	}
	if strings.TrimSpace(parsed.Confidence) != "" {
		result.Confidence = strings.TrimSpace(parsed.Confidence)
	}
	return result
}

func llmTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	if seconds > 180 {
		seconds = 180
	}
	return time.Duration(seconds) * time.Second
}

func defaultPort(dbType int) int {
	switch dbType {
	case cmdbmodel.DatabaseTypeMySQL:
		return 3306
	case cmdbmodel.DatabaseTypePostgreSQL:
		return 5432
	case cmdbmodel.DatabaseTypeOracle:
		return 1521
	case cmdbmodel.DatabaseTypeRedis:
		return 6379
	case cmdbmodel.DatabaseTypeMongoDB:
		return 27017
	case cmdbmodel.DatabaseTypeSQLServer:
		return 1433
	case cmdbmodel.DatabaseTypeElasticsearch:
		return 9200
	default:
		return 0
	}
}

func buildPrometheusEndpoint(host string, port int) string {
	value := strings.TrimSpace(strings.TrimRight(host, "/"))
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if _, _, err := net.SplitHostPort(value); err == nil {
		return "http://" + value
	}
	if port == 443 {
		return "https://" + value
	}
	if port > 0 {
		return fmt.Sprintf("http://%s:%d", value, port)
	}
	return fmt.Sprintf("http://%s:%d", value, 9090)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func contextFromGin(ctx *gin.Context) context.Context {
	if ctx == nil || ctx.Request == nil {
		return context.Background()
	}
	return ctx.Request.Context()
}

func encodeAuditJSON(value interface{}) string {
	if value == nil {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(data)
}
