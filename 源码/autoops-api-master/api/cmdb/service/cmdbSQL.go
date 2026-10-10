package service

import (
	"context"
	"database/sql"
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	configDao "dodevops-api/api/configcenter/dao"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	mysqlDriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var databaseTypeNames = map[int]string{
	model.DatabaseTypeMySQL:         "MySQL",
	model.DatabaseTypePostgreSQL:    "PostgreSQL",
	model.DatabaseTypeOracle:        "Oracle",
	model.DatabaseTypeRedis:         "Redis",
	model.DatabaseTypeMongoDB:       "MongoDB",
	model.DatabaseTypeSQLServer:     "SQL Server",
	model.DatabaseTypeElasticsearch: "Elasticsearch",
}

var databaseAuthStatusNames = map[int]string{
	model.DatabaseAuthStatusUnauthenticated: "未认证",
	model.DatabaseAuthStatusFailed:          "认证失败",
	model.DatabaseAuthStatusSuccess:         "认证成功",
}

const accountTypePrometheus = 1

type CmdbSQLService struct {
	dao *dao.CmdbSQLDao
}

func NewCmdbSQLService(dao *dao.CmdbSQLDao) *CmdbSQLService {
	return &CmdbSQLService{dao: dao}
}

func (s *CmdbSQLService) CreateDatabase(database *model.CmdbSQL) error {
	if err := s.normalizeAndValidate(database, 0); err != nil {
		return err
	}
	if s.dao.CheckNameExists(database.Name, 0) {
		return errors.New("database asset name already exists")
	}
	return s.dao.Create(database)
}

func (s *CmdbSQLService) UpdateDatabase(database *model.CmdbSQL) error {
	if database.ID == 0 {
		return errors.New("id is required")
	}
	existing, err := s.dao.GetByID(database.ID)
	if err != nil {
		return err
	}
	database.CreatedAt = existing.CreatedAt
	if err := s.normalizeAndValidate(database, database.ID); err != nil {
		return err
	}
	if s.dao.CheckNameExists(database.Name, database.ID) {
		return errors.New("database asset name already exists")
	}
	return s.dao.Update(database)
}

func (s *CmdbSQLService) DeleteDatabase(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.dao.Delete(id)
}

func (s *CmdbSQLService) GetDatabase(id uint) (*model.CmdbSQL, error) {
	database, err := s.dao.GetByID(id)
	if err != nil {
		return nil, err
	}
	s.fillDisplayNames(database)
	return database, nil
}

func (s *CmdbSQLService) ListDatabases(page, pageSize int) ([]model.CmdbSQL, int64, error) {
	list, total, err := s.dao.List(page, pageSize)
	s.fillTypeNames(list)
	return list, total, err
}

func (s *CmdbSQLService) QueryDatabases(query model.CmdbSQLQuery) ([]model.CmdbSQL, int64, error) {
	if query.Type > 0 && !IsValidDatabaseType(query.Type) {
		return nil, 0, errors.New("invalid database type")
	}
	list, total, err := s.dao.Query(query)
	s.fillTypeNames(list)
	return list, total, err
}

func (s *CmdbSQLService) TestDatabaseAuth(c *gin.Context, id uint) (map[string]interface{}, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}

	database, err := s.dao.GetByID(id)
	if err != nil {
		return nil, err
	}
	if database.AccountID == 0 {
		_ = s.dao.UpdateAuthStatus(database.ID, model.DatabaseAuthStatusFailed)
		return map[string]interface{}{
			"id":         database.ID,
			"success":    false,
			"authStatus": model.DatabaseAuthStatusFailed,
			"message":    "database asset has no accountId",
		}, nil
	}

	accountDao := configDao.NewAccountAuthDao()
	account, err := accountDao.GetByID(database.AccountID)
	if err != nil {
		_ = s.dao.UpdateAuthStatus(database.ID, model.DatabaseAuthStatusFailed)
		return nil, fmt.Errorf("get account auth failed: %w", err)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		_ = s.dao.UpdateAuthStatus(database.ID, model.DatabaseAuthStatusFailed)
		return nil, fmt.Errorf("decrypt account password failed: %w", err)
	}

	host := firstNonEmpty(database.HostIP, account.Host)
	port := database.Port
	if port == 0 {
		port = account.Port
	}
	if port == 0 {
		port = DefaultDatabasePort(database.Type)
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	success, message := testDatabaseCredential(ctx, database.Type, host, port, account.Name, password)
	authStatus := model.DatabaseAuthStatusFailed
	if success {
		authStatus = model.DatabaseAuthStatusSuccess
	}
	if err := s.dao.UpdateAuthStatus(database.ID, authStatus); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"id":             database.ID,
		"type":           database.Type,
		"typeName":       databaseTypeNames[database.Type],
		"host":           host,
		"port":           port,
		"accountId":      database.AccountID,
		"username":       account.Name,
		"success":        success,
		"authStatus":     authStatus,
		"authStatusName": databaseAuthStatusNames[authStatus],
		"message":        message,
	}, nil
}

func (s *CmdbSQLService) GetDatabaseStatistics() (map[string]interface{}, error) {
	counts, err := s.dao.CountByType()
	if err != nil {
		return nil, err
	}

	var total int64
	details := make([]map[string]interface{}, 0, len(databaseTypeNames))
	for databaseType := model.DatabaseTypeMySQL; databaseType <= model.DatabaseTypeElasticsearch; databaseType++ {
		count := counts[databaseType]
		total += count
		details = append(details, map[string]interface{}{
			"type":     databaseType,
			"typeName": databaseTypeNames[databaseType],
			"count":    count,
		})
	}

	return map[string]interface{}{
		"total":   total,
		"details": details,
		"summary": map[string]int64{
			"mysql":         counts[model.DatabaseTypeMySQL],
			"postgresql":    counts[model.DatabaseTypePostgreSQL],
			"oracle":        counts[model.DatabaseTypeOracle],
			"redis":         counts[model.DatabaseTypeRedis],
			"mongodb":       counts[model.DatabaseTypeMongoDB],
			"sqlserver":     counts[model.DatabaseTypeSQLServer],
			"elasticsearch": counts[model.DatabaseTypeElasticsearch],
		},
	}, nil
}

func (s *CmdbSQLService) GetDatabasesByAccount(accountID uint) ([]model.CmdbSQL, error) {
	list, err := s.dao.GetByAccountID(accountID)
	s.fillTypeNames(list)
	return list, err
}

func (s *CmdbSQLService) GetDatabasesByGroup(groupID uint) ([]model.CmdbSQL, error) {
	list, err := s.dao.GetByGroupID(groupID)
	s.fillTypeNames(list)
	return list, err
}

func (s *CmdbSQLService) GetDatabasesByName(name string) ([]model.CmdbSQL, error) {
	list, err := s.dao.GetByName(name)
	s.fillTypeNames(list)
	return list, err
}

func (s *CmdbSQLService) GetDatabasesByType(dbType int) ([]model.CmdbSQL, error) {
	if !IsValidDatabaseType(dbType) {
		return nil, errors.New("invalid database type")
	}
	list, err := s.dao.GetByType(dbType)
	s.fillTypeNames(list)
	return list, err
}

func (s *CmdbSQLService) normalizeAndValidate(database *model.CmdbSQL, excludeID uint) error {
	database.Name = strings.TrimSpace(database.Name)
	database.InstanceName = strings.TrimSpace(database.InstanceName)
	database.HostIP = strings.TrimSpace(database.HostIP)
	database.Endpoint = strings.TrimSpace(database.Endpoint)
	database.Environment = strings.TrimSpace(database.Environment)
	database.BusinessSystem = strings.TrimSpace(database.BusinessSystem)
	database.Owner = strings.TrimSpace(database.Owner)
	database.DBAOwner = strings.TrimSpace(database.DBAOwner)
	database.OpsOwner = strings.TrimSpace(database.OpsOwner)
	database.ClusterName = strings.TrimSpace(database.ClusterName)
	database.SlowLogPath = strings.TrimSpace(database.SlowLogPath)
	database.PrometheusLabelSelector = strings.TrimSpace(database.PrometheusLabelSelector)

	if database.Name == "" {
		return errors.New("database asset name is required")
	}
	if !IsValidDatabaseType(database.Type) {
		return errors.New("invalid database type")
	}
	if database.Status == 0 {
		database.Status = model.DatabaseStatusRunning
	}
	if !IsValidDatabaseStatus(database.Status) {
		return errors.New("invalid database status")
	}
	if _, ok := databaseAuthStatusNames[database.AuthStatus]; !ok {
		database.AuthStatus = model.DatabaseAuthStatusUnauthenticated
	}
	if database.InstanceName == "" {
		database.InstanceName = database.Name
	}
	if database.Port == 0 {
		database.Port = DefaultDatabasePort(database.Type)
	}
	if database.NodeCount == 0 {
		database.NodeCount = 1
	}
	if err := s.validatePrometheusAccount(database.PrometheusAccountID); err != nil {
		return err
	}
	s.fillDisplayNames(database)
	return nil
}

func (s *CmdbSQLService) validatePrometheusAccount(accountID uint) error {
	if accountID == 0 {
		return nil
	}
	account, err := configDao.NewAccountAuthDao().GetByID(accountID)
	if err != nil {
		return fmt.Errorf("get prometheus account failed: %w", err)
	}
	if account.Type != accountTypePrometheus {
		return errors.New("prometheusAccountId must reference a prometheus account")
	}
	return nil
}

func (s *CmdbSQLService) fillTypeNames(databases []model.CmdbSQL) {
	for i := range databases {
		s.fillDisplayNames(&databases[i])
	}
}

func (s *CmdbSQLService) fillDisplayNames(database *model.CmdbSQL) {
	if database == nil {
		return
	}
	database.TypeName = databaseTypeNames[database.Type]
	database.AuthStatusName = databaseAuthStatusNames[database.AuthStatus]
	s.fillPrometheusAccount(database)
}

func (s *CmdbSQLService) fillPrometheusAccount(database *model.CmdbSQL) {
	if database.PrometheusAccountID == 0 {
		return
	}
	account, err := configDao.NewAccountAuthDao().GetByID(database.PrometheusAccountID)
	if err != nil || account == nil {
		return
	}
	database.PrometheusAccountAlias = account.Alias
	database.PrometheusEndpoint = buildPrometheusEndpoint(account.Host, account.Port)
}

func IsValidDatabaseType(dbType int) bool {
	_, ok := databaseTypeNames[dbType]
	return ok
}

func IsValidDatabaseStatus(status int) bool {
	return status >= model.DatabaseStatusRunning && status <= model.DatabaseStatusOffline
}

func DefaultDatabasePort(dbType int) int {
	switch dbType {
	case model.DatabaseTypeMySQL:
		return 3306
	case model.DatabaseTypePostgreSQL:
		return 5432
	case model.DatabaseTypeOracle:
		return 1521
	case model.DatabaseTypeRedis:
		return 6379
	case model.DatabaseTypeMongoDB:
		return 27017
	case model.DatabaseTypeSQLServer:
		return 1433
	case model.DatabaseTypeElasticsearch:
		return 9200
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func testDatabaseCredential(ctx context.Context, dbType int, host string, port int, username, password string) (bool, string) {
	if host == "" || port == 0 {
		return false, "host and port are required"
	}

	switch dbType {
	case model.DatabaseTypeMySQL:
		return testMySQLCredential(ctx, host, port, username, password)
	case model.DatabaseTypePostgreSQL:
		return testPostgreSQLCredential(ctx, host, port, username, password)
	case model.DatabaseTypeRedis:
		return testRedisCredential(ctx, host, port, username, password)
	case model.DatabaseTypeMongoDB:
		return false, "MongoDB auth test driver is not enabled"
	case model.DatabaseTypeElasticsearch:
		return testElasticsearchCredential(ctx, host, port, username, password)
	case model.DatabaseTypeOracle:
		return false, "Oracle auth test driver is not enabled"
	case model.DatabaseTypeSQLServer:
		return false, "SQL Server auth test driver is not enabled"
	default:
		return false, "unsupported database type"
	}
}

func testMySQLCredential(ctx context.Context, host string, port int, username, password string) (bool, string) {
	cfg := mysqlDriver.NewConfig()
	cfg.User = username
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", host, port)
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return false, "connect init failed: " + err.Error()
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return false, "auth failed: " + err.Error()
	}
	return true, "auth success"
}

func testPostgreSQLCredential(ctx context.Context, host string, port int, username, password string) (bool, string) {
	connURL := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(username, password),
		Host:   net.JoinHostPort(host, fmt.Sprint(port)),
		Path:   "postgres",
	}
	values := connURL.Query()
	values.Set("sslmode", "disable")
	values.Set("connect_timeout", "5")
	connURL.RawQuery = values.Encode()

	db, err := sql.Open("pgx", connURL.String())
	if err != nil {
		return false, "connect init failed: " + err.Error()
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Minute)

	if err := db.PingContext(ctx); err != nil {
		return false, "auth failed: " + err.Error()
	}
	return true, "auth success"
}

func testRedisCredential(ctx context.Context, host string, port int, username, password string) (bool, string) {
	redisUsername := ""
	if shouldUseRedisACLUsername(username) {
		redisUsername = username
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Username: redisUsername,
		Password: password,
		DB:       0,
	})
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		return false, "auth failed: " + err.Error()
	}
	return true, "auth success"
}

func shouldUseRedisACLUsername(username string) bool {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return false
	}
	switch username {
	case "default", "redis", "root", "admin":
		return false
	default:
		return true
	}
}

func testElasticsearchCredential(ctx context.Context, host string, port int, username, password string) (bool, string) {
	endpoint := buildElasticsearchURL(host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false, "create request failed: " + err.Error()
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "connect failed: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "auth failed: username or password is invalid"
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, "auth success"
	}
	return false, fmt.Sprintf("auth failed: HTTP status %d", resp.StatusCode)
}

func buildElasticsearchURL(host string, port int) string {
	value := strings.TrimSpace(strings.TrimRight(host, "/"))
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return value
	}
	return fmt.Sprintf("http://%s:%d", value, port)
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
