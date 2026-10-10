package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"dodevops-api/api/aiops/model"
)

const defaultTestMessage = "请只回复 OK，用于测试模型连通性。"

func (s *LLMProviderService) TestConnection(dto *model.TestLLMProviderDto) (*model.TestLLMProviderResult, error) {
	provider, apiKey, err := s.buildTestProvider(dto)
	if err != nil {
		return nil, err
	}

	result := &model.TestLLMProviderResult{
		ModelName: provider.Name,
		ModelType: provider.ProviderType,
		ModelCode: provider.DefaultModel,
		APIURL:    provider.BaseURL,
	}

	if provider.ProviderType == model.LLMProviderTypeCodex && strings.TrimSpace(provider.BaseURL) == "" {
		result.Success = false
		result.Message = "codex 类型未配置 apiUrl，暂不支持 HTTP API 连通性测试"
		result.Error = "codex_cli_test_not_supported"
		return result, nil
	}

	message := firstNonEmpty(dto.Message, defaultTestMessage)
	timeout := normalizeTestTimeout(dto.TimeoutSeconds, provider.TimeoutSeconds)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch provider.ProviderType {
	case model.LLMProviderTypeAnthropic:
		return testAnthropicProvider(ctx, provider, apiKey, message, result)
	case model.LLMProviderTypeCodex:
		normalizeCodexProvider(provider)
		result.APIURL = provider.BaseURL
		return testResponsesProvider(ctx, provider, apiKey, message, result)
	default:
		return testOpenAICompatibleProvider(ctx, provider, apiKey, message, result)
	}
}

func (s *LLMProviderService) buildTestProvider(dto *model.TestLLMProviderDto) (*model.LLMProvider, string, error) {
	if dto.ID > 0 {
		provider, err := s.dao.GetByID(dto.ID)
		if err != nil {
			return nil, "", err
		}

		apiKey, err := provider.DecryptAPIKey()
		if err != nil {
			return nil, "", err
		}

		applyTestOverrides(provider, dto)
		return provider, firstNonEmpty(dto.APIKey, dto.APIKeySnake, apiKey), nil
	}

	createDTO := dto.CreateLLMProviderDto
	if firstNonEmpty(createDTO.ModelName, createDTO.Name) == "" {
		createDTO.ModelName = "临时测试模型"
	}
	provider, err := buildProviderFromCreateDTO(&createDTO)
	if err != nil {
		return nil, "", err
	}
	return provider, firstNonEmpty(createDTO.APIKey, createDTO.APIKeySnake), nil
}

func applyTestOverrides(provider *model.LLMProvider, dto *model.TestLLMProviderDto) {
	if value := firstNonEmpty(dto.ModelName, dto.Name); value != "" {
		provider.Name = value
	}
	if value := model.NormalizeProviderType(firstNonEmpty(dto.ModelType, dto.ProviderType)); value != "" {
		provider.ProviderType = value
		defaults := model.DefaultsForProviderType(value)
		if provider.Protocol == "" {
			provider.Protocol = defaults.Protocol
		}
		if provider.ChatPath == "" {
			provider.ChatPath = defaults.ChatPath
		}
		if provider.AuthType == "" {
			provider.AuthType = defaults.AuthType
		}
	}
	if value := firstNonEmpty(dto.ModelCode, dto.DefaultModel); value != "" {
		provider.DefaultModel = value
	}
	if value := firstNonEmpty(dto.APIURL, dto.APIURLSnake, dto.BaseURL); value != "" {
		provider.BaseURL = value
	}
	if value := firstNonEmpty(dto.ChatPath); value != "" {
		provider.ChatPath = value
	}
}

func testOpenAICompatibleProvider(ctx context.Context, provider *model.LLMProvider, apiKey, message string, result *model.TestLLMProviderResult) (*model.TestLLMProviderResult, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return nil, err
	}

	payload := buildOpenAICompatibleTestPayload(provider, message)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)

	return doLLMTestRequest(req, result, parseOpenAICompatibleResponse)
}

func testAnthropicProvider(ctx context.Context, provider *model.LLMProvider, apiKey, message string, result *model.TestLLMProviderResult) (*model.TestLLMProviderResult, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"model":       provider.DefaultModel,
		"max_tokens":  16,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)

	return doLLMTestRequest(req, result, parseAnthropicResponse)
}

func testResponsesProvider(ctx context.Context, provider *model.LLMProvider, apiKey, message string, result *model.TestLLMProviderResult) (*model.TestLLMProviderResult, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return nil, err
	}

	payload := buildResponsesTestPayload(provider, message)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)

	return doLLMTestRequest(req, result, parseResponsesAPIResponse)
}

func normalizeCodexProvider(provider *model.LLMProvider) {
	if provider == nil {
		return
	}
	provider.Protocol = "responses"
	provider.BaseURL = normalizeCodexBaseURL(provider.BaseURL)
	if strings.TrimSpace(provider.ChatPath) == "" || strings.EqualFold(strings.TrimSpace(provider.ChatPath), "/chat/completions") {
		provider.ChatPath = "/responses"
	}
	authType := strings.ToLower(strings.TrimSpace(provider.AuthType))
	if authType == "" || authType == "none" {
		provider.AuthType = "bearer"
	}
}

func normalizeCodexBaseURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(strings.ToLower(baseURL), "/v1") {
		return baseURL
	}
	return baseURL + "/v1"
}

func buildOpenAICompatibleTestPayload(provider *model.LLMProvider, message string) map[string]interface{} {
	extra := model.DecodeAnyMap(provider.ExtraJSON)
	maxTokensField := openAITestMaxTokensField(provider, extra)
	maxTokens := openAITestMaxTokens(provider, extra)
	payload := map[string]interface{}{
		"model": provider.DefaultModel,
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
		"temperature": 0,
	}
	if maxTokensField != "" && maxTokens > 0 {
		payload[maxTokensField] = maxTokens
	}
	if shouldDisableThinkingByDefault(provider) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}

	mergePayloadMap(payload, anyMapValue(extra["requestBody"]))
	mergePayloadMap(payload, anyMapValue(extra["testRequestBody"]))
	return payload
}

func buildResponsesTestPayload(provider *model.LLMProvider, message string) map[string]interface{} {
	extra := model.DecodeAnyMap(provider.ExtraJSON)
	maxOutputTokens := positiveIntFromAnyMap(extra,
		"testMaxOutputTokens",
		"test_max_output_tokens",
		"maxOutputTokens",
		"max_output_tokens",
	)
	if maxOutputTokens <= 0 {
		maxOutputTokens = 128
	}
	payload := map[string]interface{}{
		"model":             provider.DefaultModel,
		"input":             message,
		"max_output_tokens": maxOutputTokens,
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(provider.DefaultModel)), "gpt-5") {
		payload["reasoning"] = map[string]string{"effort": "high"}
	}

	mergePayloadMap(payload, anyMapValue(extra["requestBody"]))
	mergePayloadMap(payload, anyMapValue(extra["testRequestBody"]))
	return payload
}

func openAITestMaxTokensField(provider *model.LLMProvider, extra map[string]interface{}) string {
	if field := stringFromAnyMap(extra,
		"testMaxTokensField",
		"maxTokensField",
		"max_tokens_field",
		"maxCompletionTokensField",
	); field != "" {
		return field
	}
	if isXiaomiMiMoProvider(provider) {
		return "max_completion_tokens"
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(provider.DefaultModel)), "gpt-5") {
		return "max_completion_tokens"
	}
	return "max_tokens"
}

func openAITestMaxTokens(provider *model.LLMProvider, extra map[string]interface{}) int {
	if value := positiveIntFromAnyMap(extra,
		"testMaxTokens",
		"test_max_tokens",
		"testMaxCompletionTokens",
		"maxCompletionTokens",
		"max_completion_tokens",
	); value > 0 {
		return value
	}
	if isXiaomiMiMoProvider(provider) {
		return 128
	}
	return 16
}

func doLLMTestRequest(req *http.Request, result *model.TestLLMProviderResult, parser func([]byte) (string, error)) (*model.TestLLMProviderResult, error) {
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		result.Success = false
		result.Message = "模型连通性测试失败"
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()

	result.HTTPStatusCode = resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Success = false
		result.Message = "模型接口返回非成功状态"
		result.Error = truncateString(string(body), 2000)
		return result, nil
	}

	content, err := parser(body)
	if err != nil {
		result.Success = false
		result.Message = "模型接口返回格式不符合预期"
		result.Error = err.Error()
		result.Response = truncateString(string(body), 2000)
		return result, nil
	}

	result.Success = true
	result.Message = "模型连通性测试成功"
	result.Response = truncateString(content, 2000)
	return result, nil
}

func parseOpenAICompatibleResponse(body []byte) (string, error) {
	var response struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.Error.Message != "" {
		return "", fmt.Errorf("%s", response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("response choices is empty")
	}
	choice := response.Choices[0]
	content := firstNonEmpty(choice.Message.Content, choice.Text)
	if content == "" {
		if strings.TrimSpace(choice.Message.ReasoningContent) != "" {
			return "model returned reasoning output but no final content", nil
		}
		if strings.EqualFold(strings.TrimSpace(choice.FinishReason), "length") {
			return "model response reached token limit before final content", nil
		}
		return "", fmt.Errorf("response content is empty")
	}
	return content, nil
}

func parseResponsesAPIResponse(body []byte) (string, error) {
	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.Error.Message != "" {
		return "", fmt.Errorf("%s", response.Error.Message)
	}
	if strings.TrimSpace(response.OutputText) != "" {
		return strings.TrimSpace(response.OutputText), nil
	}
	for _, item := range response.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) != "" {
				return strings.TrimSpace(content.Text), nil
			}
		}
	}
	return "", fmt.Errorf("response content is empty")
}

func parseAnthropicResponse(body []byte) (string, error) {
	var response struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.Error.Message != "" {
		return "", fmt.Errorf("%s", response.Error.Message)
	}
	for _, item := range response.Content {
		if strings.TrimSpace(item.Text) != "" {
			return item.Text, nil
		}
	}
	return "", fmt.Errorf("response content is empty")
}

func validateHTTPProvider(provider *model.LLMProvider, apiKey string) error {
	if strings.TrimSpace(provider.BaseURL) == "" {
		return fmt.Errorf("apiUrl is required")
	}
	if strings.TrimSpace(provider.DefaultModel) == "" {
		return fmt.Errorf("modelCode is required")
	}

	defaults := model.DefaultsForProviderType(provider.ProviderType)
	if defaults.RequiresAPIKey && strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("api_key is required")
	}
	if provider.ChatPath == "" {
		provider.ChatPath = defaults.ChatPath
	}
	if provider.AuthType == "" {
		provider.AuthType = defaults.AuthType
	}
	return nil
}

func buildModelEndpoint(baseURL, chatPath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	chatPath = strings.TrimSpace(chatPath)
	if strings.HasPrefix(chatPath, "http://") || strings.HasPrefix(chatPath, "https://") {
		return chatPath
	}
	if chatPath == "" {
		return baseURL
	}
	return baseURL + "/" + strings.TrimLeft(chatPath, "/")
}

func applyProviderHeaders(req *http.Request, provider *model.LLMProvider) {
	for key, value := range model.DecodeStringMap(provider.HeadersJSON) {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		req.Header.Set(key, value)
	}
}

func applyProviderAuth(req *http.Request, provider *model.LLMProvider, apiKey string) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return
	}

	authType := strings.ToLower(strings.TrimSpace(provider.AuthType))
	authType = strings.ReplaceAll(authType, "_", "-")
	switch authType {
	case "none", "no-auth", "noauth":
		return
	case "", "bearer", "authorization", "authorization-bearer":
		req.Header.Set("Authorization", "Bearer "+apiKey)
	case "api-key", "apikey":
		req.Header.Set("api-key", apiKey)
	case "x-api-key", "x-apikey":
		req.Header.Set("x-api-key", apiKey)
	default:
		headerName := strings.TrimSpace(provider.AuthType)
		if !isSafeHeaderName(headerName) {
			headerName = "Authorization"
			apiKey = "Bearer " + apiKey
		}
		req.Header.Set(headerName, apiKey)
	}
}

func mergePayloadMap(payload map[string]interface{}, values map[string]interface{}) {
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		payload[key] = value
	}
}

func anyMapValue(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return typed
	default:
		return nil
	}
}

func stringFromAnyMap(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func positiveIntFromAnyMap(values map[string]interface{}, keys ...string) int {
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		result := 0
		switch typed := value.(type) {
		case int:
			result = typed
		case int64:
			result = int(typed)
		case float64:
			result = int(typed)
		case json.Number:
			if number, err := typed.Int64(); err == nil {
				result = int(number)
			}
		case string:
			_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &result)
		}
		if result > 0 {
			return result
		}
	}
	return 0
}

func shouldDisableThinkingByDefault(provider *model.LLMProvider) bool {
	return isXiaomiMiMoProvider(provider)
}

func isXiaomiMiMoProvider(provider *model.LLMProvider) bool {
	value := strings.ToLower(provider.BaseURL + " " + provider.DefaultModel + " " + provider.Name)
	return strings.Contains(value, "xiaomimimo.com") || strings.Contains(value, "mimo-v")
}

func isSafeHeaderName(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", r):
		default:
			return false
		}
	}
	return true
}

func normalizeTestTimeout(requestTimeout, providerTimeout int) time.Duration {
	timeoutSeconds := requestTimeout
	if timeoutSeconds <= 0 {
		timeoutSeconds = providerTimeout
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	if timeoutSeconds > 120 {
		timeoutSeconds = 120
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func truncateString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
