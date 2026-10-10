package service

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/configcenter/dao"
	"dodevops-api/api/configcenter/model"
)

const (
	accountTypePrometheus        = 1
	legacyAccountTypeJenkins     = 4
	legacyAccountTypeZabbix      = 5
	accountAuthImportDefaultType = legacyAccountTypeZabbix
)

type AccountAuthService struct {
	dao *dao.AccountAuthDao
}

type accountAuthImportRow struct {
	Line     int
	Alias    string
	Host     string
	Username string
	Password string
}

func NewAccountAuthService() *AccountAuthService {
	return &AccountAuthService{
		dao: dao.NewAccountAuthDao(),
	}
}

// Create 创建账号
func (s *AccountAuthService) Create(account *model.AccountAuth) error {
	existing, err := s.GetByAlias(account.Alias)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("alias '%s' 已存在", account.Alias)
	}
	return s.dao.Create(account)
}

// Update 更新账号
func (s *AccountAuthService) Update(account *model.AccountAuth) error {
	return s.dao.Update(account)
}

// Delete 删除账号
func (s *AccountAuthService) Delete(id uint) error {
	return s.dao.Delete(id)
}

// GetByID 根据 ID 查询账号
func (s *AccountAuthService) GetByID(id uint) (*model.AccountAuth, error) {
	return s.dao.GetByID(id)
}

// List 获取账号列表
func (s *AccountAuthService) List() ([]model.AccountAuth, error) {
	return s.dao.List()
}

// ListWithPage 获取账号列表（分页）
func (s *AccountAuthService) ListWithPage(page, pageSize int) ([]model.AccountAuth, int64, error) {
	return s.dao.ListWithPage(page, pageSize)
}

func (s *AccountAuthService) QueryWithPage(page, pageSize int, alias string, accountType int) ([]model.AccountAuth, int64, error) {
	return s.dao.QueryWithPage(page, pageSize, alias, accountType)
}

// DecryptPassword 解密密码
func (s *AccountAuthService) DecryptPassword(id uint) (string, error) {
	account, err := s.dao.GetByID(id)
	if err != nil {
		return "", err
	}
	return account.DecryptPassword()
}

// GetByType 根据类型查询账号
func (s *AccountAuthService) GetByType(accountType string) ([]model.AccountAuth, error) {
	return s.dao.GetByType(accountType)
}

// GetByAlias 根据别名查询账号
func (s *AccountAuthService) GetByAlias(alias string) (*model.AccountAuth, error) {
	return s.dao.GetByAlias(alias)
}

func (s *AccountAuthService) Import(dto *model.ImportAccountAuthDto) (model.ImportAccountAuthResult, error) {
	rows, failedRows, total := parseAccountAuthImportRows(dto.TableData)
	if total == 0 {
		return model.ImportAccountAuthResult{}, fmt.Errorf("表格中没有可导入的数据")
	}

	accountType := dto.Type
	if accountType <= 0 {
		accountType = accountAuthImportDefaultType
	}

	seenAliases := make(map[string]struct{}, len(rows))
	successCount := 0

	for _, row := range rows {
		aliasKey := strings.ToLower(row.Alias)
		if _, exists := seenAliases[aliasKey]; exists {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   row.Line,
				Alias:  row.Alias,
				Reason: "导入数据中存在重复的账号别名",
			})
			continue
		}
		seenAliases[aliasKey] = struct{}{}

		existing, err := s.GetByAlias(row.Alias)
		if err != nil {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   row.Line,
				Alias:  row.Alias,
				Reason: err.Error(),
			})
			continue
		}
		if existing != nil {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   row.Line,
				Alias:  row.Alias,
				Reason: "账号别名已存在",
			})
			continue
		}

		account := model.AccountAuth{
			Alias:    row.Alias,
			Host:     row.Host,
			Port:     inferAccountAuthPort(row.Host),
			Name:     row.Username,
			Password: row.Password,
			Type:     accountType,
			Remark:   strings.TrimSpace(dto.Remark),
		}
		if err := s.dao.Create(&account); err != nil {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   row.Line,
				Alias:  row.Alias,
				Reason: err.Error(),
			})
			continue
		}
		successCount++
	}

	return model.ImportAccountAuthResult{
		Total:      total,
		Success:    successCount,
		Fail:       len(failedRows),
		FailedRows: failedRows,
	}, nil
}

// TestConnection 测试账号连接
func (s *AccountAuthService) TestConnection(id uint) (bool, string, error) {
	account, err := s.dao.GetByID(id)
	if err != nil {
		return false, "", fmt.Errorf("获取账号失败: %w", err)
	}

	password, err := account.DecryptPassword()
	if err != nil {
		return false, "", fmt.Errorf("解密密码失败: %w", err)
	}

	switch account.Type {
	case accountTypePrometheus:
		return testPrometheusConnection(account.Host, account.Port, account.Name, password)
	case legacyAccountTypeJenkins:
		return testJenkinsConnection(account.Host, account.Port, account.Name, password)
	case legacyAccountTypeZabbix:
		return testZabbixConnection(account.Host, account.Port, account.Name, password)
	default:
		return false, "不支持的账号类型", nil
	}
}

func parseAccountAuthImportRows(tableData string) ([]accountAuthImportRow, []model.ImportAccountAuthFailedRow, int) {
	normalized := strings.ReplaceAll(tableData, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")

	rows := make([]accountAuthImportRow, 0, len(lines))
	failedRows := make([]model.ImportAccountAuthFailedRow, 0)
	total := 0

	for idx, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		fields := splitAccountAuthImportFields(line)
		if len(fields) == 0 {
			continue
		}
		if isAccountAuthImportHeader(fields) {
			continue
		}

		total++
		lineNo := idx + 1
		if len(fields) < 4 {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   lineNo,
				Alias:  "",
				Reason: "字段数量不足，需要 账号别名 账号地址 用户名 密码",
			})
			continue
		}

		alias := strings.TrimSpace(fields[0])
		host := strings.TrimSpace(fields[1])
		username := strings.TrimSpace(fields[2])
		password := strings.TrimSpace(strings.Join(fields[3:], " "))

		if alias == "" {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   lineNo,
				Alias:  "",
				Reason: "账号别名不能为空",
			})
			continue
		}
		if host == "" {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   lineNo,
				Alias:  alias,
				Reason: "账号地址不能为空",
			})
			continue
		}
		if username == "" {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   lineNo,
				Alias:  alias,
				Reason: "用户名不能为空",
			})
			continue
		}
		if password == "" {
			failedRows = append(failedRows, model.ImportAccountAuthFailedRow{
				Line:   lineNo,
				Alias:  alias,
				Reason: "密码不能为空",
			})
			continue
		}

		rows = append(rows, accountAuthImportRow{
			Line:     lineNo,
			Alias:    alias,
			Host:     host,
			Username: username,
			Password: password,
		})
	}

	return rows, failedRows, total
}

func splitAccountAuthImportFields(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	if strings.Contains(line, "\t") {
		return trimAccountAuthImportFields(strings.Split(line, "\t"))
	}

	if strings.Contains(line, ",") || strings.Contains(line, "，") {
		replaced := strings.ReplaceAll(line, "，", ",")
		reader := csv.NewReader(strings.NewReader(replaced))
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		fields, err := reader.Read()
		if err == nil || err == io.EOF {
			return trimAccountAuthImportFields(fields)
		}
		return trimAccountAuthImportFields(strings.Split(replaced, ","))
	}

	return trimAccountAuthImportFields(strings.Fields(line))
}

func trimAccountAuthImportFields(fields []string) []string {
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}
	return fields
}

func isAccountAuthImportHeader(fields []string) bool {
	if len(fields) < 4 {
		return false
	}

	field0 := normalizeAccountAuthImportHeader(fields[0])
	field1 := normalizeAccountAuthImportHeader(fields[1])
	field2 := normalizeAccountAuthImportHeader(fields[2])
	field3 := normalizeAccountAuthImportHeader(fields[3])

	isAlias := field0 == "账号别名" || field0 == "别名" || field0 == "alias" || field0 == "accountalias"
	isHost := field1 == "账号地址" || field1 == "地址" || field1 == "host" || field1 == "url" || field1 == "accounturl" || field1 == "accountaddress"
	isUser := field2 == "用户名" || field2 == "用户" || field2 == "username" || field2 == "user" || field2 == "name"
	isPassword := field3 == "密码" || field3 == "password" || field3 == "passwd" || field3 == "pwd"

	return isAlias && isHost && isUser && isPassword
}

func normalizeAccountAuthImportHeader(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer("：", "", ":", "", " ", "", "\t", "", "_", "", "-", "")
	return replacer.Replace(v)
}

func inferAccountAuthPort(host string) int {
	value := strings.TrimSpace(host)
	if value == "" {
		return 0
	}

	lowerValue := strings.ToLower(value)
	if strings.HasPrefix(lowerValue, "http://") || strings.HasPrefix(lowerValue, "https://") {
		parsed, err := url.Parse(value)
		if err == nil {
			if portText := parsed.Port(); portText != "" {
				if port, err := strconv.Atoi(portText); err == nil && port > 0 && port <= 65535 {
					return port
				}
			}
			switch parsed.Scheme {
			case "https":
				return 443
			case "http":
				return 80
			}
		}
	}

	_, portText, err := net.SplitHostPort(value)
	if err == nil {
		if port, err := strconv.Atoi(portText); err == nil && port > 0 && port <= 65535 {
			return port
		}
	}

	return 0
}

func buildBaseURL(host string, port, defaultPort int) string {
	host = strings.TrimSpace(strings.TrimRight(host, "/"))
	if host == "" {
		return ""
	}

	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}

	if strings.Contains(host, "/") {
		return "http://" + host
	}

	if _, _, err := net.SplitHostPort(host); err == nil {
		return "http://" + host
	}

	if port == 443 {
		return fmt.Sprintf("https://%s", host)
	}
	if port > 0 {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	return fmt.Sprintf("http://%s:%d", host, defaultPort)
}

// testPrometheusConnection 测试 Prometheus 连接
func testPrometheusConnection(host string, port int, username, password string) (bool, string, error) {
	baseURL := buildBaseURL(host, port, 9090)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", baseURL+"/api/v1/query", nil)
	if err != nil {
		return false, "创建请求失败", err
	}

	query := req.URL.Query()
	query.Set("query", "vector(1)")
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Accept", "application/json")

	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, "连接失败: " + err.Error(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "认证失败: 用户名或密码错误", nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Sprintf("连接失败: HTTP状态码 %d", resp.StatusCode), nil
	}

	var result struct {
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "解析 Prometheus 响应失败", err
	}

	if result.Status == "success" {
		return true, "连接成功", nil
	}
	if result.ErrorType != "" && result.Error != "" {
		return false, "Prometheus 查询失败: " + result.ErrorType + ": " + result.Error, nil
	}
	if result.Error != "" {
		return false, "Prometheus 查询失败: " + result.Error, nil
	}
	if result.ErrorType != "" {
		return false, "Prometheus 查询失败: " + result.ErrorType, nil
	}

	return false, "Prometheus 返回异常响应", nil
}

// testJenkinsConnection 测试 Jenkins 连接
func testJenkinsConnection(host string, port int, username, password string) (bool, string, error) {
	baseURL := buildBaseURL(host, port, 8080)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", baseURL+"/api/json", nil)
	if err != nil {
		return false, "创建请求失败", err
	}

	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return false, "连接失败: " + err.Error(), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, "连接成功", nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "认证失败: 用户名或密码错误", nil
	}
	return false, fmt.Sprintf("连接失败: HTTP状态码 %d", resp.StatusCode), nil
}

// testZabbixConnection 测试 Zabbix 连接
func testZabbixConnection(host string, port int, username, password string) (bool, string, error) {
	baseURL := buildBaseURL(host, port, 80)
	baseURL = strings.TrimSuffix(baseURL, "/zabbix.php")

	loginData := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "user.login",
		"params": map[string]string{
			"user":     username,
			"password": password,
		},
		"id":   1,
		"auth": nil,
	}

	jsonData, err := json.Marshal(loginData)
	if err != nil {
		return false, "构建请求数据失败", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", baseURL+"/api_jsonrpc.php", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, "创建请求失败", err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false, "连接失败: " + err.Error(), nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "解析响应失败", err
	}

	if authToken, ok := result["result"].(string); ok && authToken != "" {
		return true, "连接成功", nil
	}
	if errData, ok := result["error"].(map[string]interface{}); ok {
		if message, ok := errData["data"].(string); ok {
			return false, "认证失败: " + message, nil
		}
		return false, "认证失败: 用户名或密码错误", nil
	}

	return false, "未知错误", nil
}
