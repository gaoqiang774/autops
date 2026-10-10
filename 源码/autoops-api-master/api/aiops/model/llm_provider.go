package model

import (
	"encoding/json"
	"strings"

	"dodevops-api/common/util"
)

const (
	LLMProviderStatusEnabled  = 1
	LLMProviderStatusDisabled = 2

	LLMProviderTypeOpenAI           = "openai"
	LLMProviderTypeOpenAICompatible = "openai_compatible"
	LLMProviderTypeAnthropic        = "anthropic"
	LLMProviderTypeCodex            = "codex"
	LLMProviderTypeDeepSeek         = "deepseek"
)

type LLMProvider struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	Name               string     `gorm:"size:128;not null;uniqueIndex:idx_aiops_llm_provider_name" json:"name"`
	ProviderType       string     `gorm:"size:64;not null;index" json:"providerType"`
	Protocol           string     `gorm:"size:64;not null" json:"protocol"`
	BaseURL            string     `gorm:"size:512" json:"baseUrl"`
	ChatPath           string     `gorm:"size:256" json:"chatPath"`
	APIKey             string     `gorm:"column:api_key;type:text" json:"-"`
	APISecret          string     `gorm:"column:api_secret;type:text" json:"-"`
	AuthType           string     `gorm:"size:32;not null;default:bearer" json:"authType"`
	Organization       string     `gorm:"size:128" json:"organization"`
	Project            string     `gorm:"size:128" json:"project"`
	DefaultModel       string     `gorm:"size:128" json:"defaultModel"`
	ModelsJSON         string     `gorm:"column:models;type:text" json:"-"`
	HeadersJSON        string     `gorm:"column:headers;type:text" json:"-"`
	ExtraJSON          string     `gorm:"column:extra;type:text" json:"-"`
	TimeoutSeconds     int        `gorm:"not null;default:60" json:"timeoutSeconds"`
	MaxRetries         int        `gorm:"not null;default:2" json:"maxRetries"`
	MaxContextMessages int        `gorm:"not null;default:20" json:"maxContextMessages"`
	Temperature        float64    `gorm:"type:decimal(5,3);not null;default:0.7" json:"temperature"`
	MaxTokens          int        `gorm:"not null;default:0" json:"maxTokens"`
	Status             int        `gorm:"not null;default:1;index" json:"status"`
	IsDefault          bool       `gorm:"not null;default:false;index" json:"isDefault"`
	Remark             string     `gorm:"type:text" json:"remark"`
	CreatedAt          util.HTime `json:"createdAt"`
	UpdatedAt          util.HTime `json:"updatedAt"`
}

func (LLMProvider) TableName() string {
	return "aiops_llm_provider"
}

type CreateLLMProviderDto struct {
	ModelName          string                 `json:"modelName"`
	ModelType          string                 `json:"modelType"`
	ModelCode          string                 `json:"modelCode"`
	APIURL             string                 `json:"apiUrl"`
	APIURLSnake        string                 `json:"api_url"`
	APIKeySnake        string                 `json:"api_key"`
	Name               string                 `json:"name"`
	ProviderType       string                 `json:"providerType"`
	Protocol           string                 `json:"protocol"`
	BaseURL            string                 `json:"baseUrl"`
	ChatPath           string                 `json:"chatPath"`
	APIKey             string                 `json:"apiKey"`
	APISecret          string                 `json:"apiSecret"`
	AuthType           string                 `json:"authType"`
	Organization       string                 `json:"organization"`
	Project            string                 `json:"project"`
	DefaultModel       string                 `json:"defaultModel"`
	Models             []string               `json:"models"`
	Headers            map[string]string      `json:"headers"`
	Extra              map[string]interface{} `json:"extra"`
	TimeoutSeconds     int                    `json:"timeoutSeconds"`
	MaxRetries         int                    `json:"maxRetries"`
	MaxContextMessages int                    `json:"maxContextMessages"`
	Temperature        *float64               `json:"temperature"`
	MaxTokens          int                    `json:"maxTokens"`
	Status             int                    `json:"status"`
	IsDefault          bool                   `json:"isDefault"`
	Remark             string                 `json:"remark"`
}

type UpdateLLMProviderDto struct {
	ID                 uint                   `json:"id" binding:"required"`
	ModelName          *string                `json:"modelName"`
	ModelType          *string                `json:"modelType"`
	ModelCode          *string                `json:"modelCode"`
	APIURL             *string                `json:"apiUrl"`
	APIURLSnake        *string                `json:"api_url"`
	APIKeySnake        *string                `json:"api_key"`
	Name               *string                `json:"name"`
	ProviderType       *string                `json:"providerType"`
	Protocol           *string                `json:"protocol"`
	BaseURL            *string                `json:"baseUrl"`
	ChatPath           *string                `json:"chatPath"`
	APIKey             *string                `json:"apiKey"`
	APISecret          *string                `json:"apiSecret"`
	ClearAPIKey        bool                   `json:"clearApiKey"`
	ClearAPISecret     bool                   `json:"clearApiSecret"`
	AuthType           *string                `json:"authType"`
	Organization       *string                `json:"organization"`
	Project            *string                `json:"project"`
	DefaultModel       *string                `json:"defaultModel"`
	Models             []string               `json:"models"`
	Headers            map[string]string      `json:"headers"`
	Extra              map[string]interface{} `json:"extra"`
	TimeoutSeconds     *int                   `json:"timeoutSeconds"`
	MaxRetries         *int                   `json:"maxRetries"`
	MaxContextMessages *int                   `json:"maxContextMessages"`
	Temperature        *float64               `json:"temperature"`
	MaxTokens          *int                   `json:"maxTokens"`
	Status             *int                   `json:"status"`
	IsDefault          *bool                  `json:"isDefault"`
	Remark             *string                `json:"remark"`
}

type ListLLMProviderQuery struct {
	Page         int    `form:"page"`
	PageSize     int    `form:"pageSize"`
	Keyword      string `form:"keyword"`
	ProviderType string `form:"providerType"`
	Protocol     string `form:"protocol"`
	Status       int    `form:"status"`
	OnlyEnabled  bool   `form:"onlyEnabled"`
}

type LLMProviderVO struct {
	ID                 uint                   `json:"id"`
	ModelName          string                 `json:"modelName"`
	ModelType          string                 `json:"modelType"`
	ModelCode          string                 `json:"modelCode"`
	APIURL             string                 `json:"apiUrl"`
	Name               string                 `json:"name"`
	ProviderType       string                 `json:"providerType"`
	Protocol           string                 `json:"protocol"`
	BaseURL            string                 `json:"baseUrl"`
	ChatPath           string                 `json:"chatPath"`
	AuthType           string                 `json:"authType"`
	Organization       string                 `json:"organization"`
	Project            string                 `json:"project"`
	DefaultModel       string                 `json:"defaultModel"`
	Models             []string               `json:"models"`
	Headers            map[string]string      `json:"headers"`
	Extra              map[string]interface{} `json:"extra"`
	TimeoutSeconds     int                    `json:"timeoutSeconds"`
	MaxRetries         int                    `json:"maxRetries"`
	MaxContextMessages int                    `json:"maxContextMessages"`
	Temperature        float64                `json:"temperature"`
	MaxTokens          int                    `json:"maxTokens"`
	Status             int                    `json:"status"`
	IsDefault          bool                   `json:"isDefault"`
	APIKeySet          bool                   `json:"apiKeySet"`
	APIKeyPreview      string                 `json:"apiKeyPreview"`
	APISecretSet       bool                   `json:"apiSecretSet"`
	Remark             string                 `json:"remark"`
	CreatedAt          util.HTime             `json:"createdAt"`
	UpdatedAt          util.HTime             `json:"updatedAt"`
}

type LLMProviderSecretVO struct {
	ID        uint   `json:"id"`
	APIKey    string `json:"apiKey"`
	APISecret string `json:"apiSecret"`
}

type TestLLMProviderDto struct {
	ID             uint   `json:"id" form:"id"`
	Message        string `json:"message" form:"message"`
	TimeoutSeconds int    `json:"timeoutSeconds" form:"timeoutSeconds"`
	CreateLLMProviderDto
}

type TestLLMProviderResult struct {
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	ModelName      string `json:"modelName"`
	ModelType      string `json:"modelType"`
	ModelCode      string `json:"modelCode"`
	APIURL         string `json:"apiUrl"`
	HTTPStatusCode int    `json:"httpStatusCode"`
	LatencyMs      int64  `json:"latencyMs"`
	Response       string `json:"response"`
	Error          string `json:"error,omitempty"`
}

type LLMProviderTypeOption struct {
	Type           string `json:"type"`
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	BaseURL        string `json:"baseUrl"`
	ChatPath       string `json:"chatPath"`
	AuthType       string `json:"authType"`
	DefaultModel   string `json:"defaultModel"`
	RequiresAPIKey bool   `json:"requiresApiKey"`
}

type providerDefaults struct {
	Protocol       string
	BaseURL        string
	ChatPath       string
	AuthType       string
	DefaultModel   string
	RequiresAPIKey bool
}

func NormalizeProviderType(providerType string) string {
	value := strings.ToLower(strings.TrimSpace(providerType))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")

	switch value {
	case "openai_compatible", "openai_compat", "compatible", "custom", "custom_http",
		"aliyun", "aliyun_bailian", "bailian", "dashscope", "qwen", "ollama":
		return LLMProviderTypeOpenAICompatible
	case "deepeck", "deepseek":
		return LLMProviderTypeDeepSeek
	default:
		return value
	}
}

func IsSupportedProviderType(providerType string) bool {
	switch NormalizeProviderType(providerType) {
	case LLMProviderTypeOpenAI,
		LLMProviderTypeOpenAICompatible,
		LLMProviderTypeAnthropic,
		LLMProviderTypeCodex,
		LLMProviderTypeDeepSeek:
		return true
	default:
		return false
	}
}

func ProviderTypeOptions() []LLMProviderTypeOption {
	options := []LLMProviderTypeOption{
		{Type: LLMProviderTypeOpenAI, Name: "OpenAI", Protocol: "openai_compatible"},
		{Type: LLMProviderTypeOpenAICompatible, Name: "OpenAI Compatible", Protocol: "openai_compatible"},
		{Type: LLMProviderTypeAnthropic, Name: "Anthropic Claude", Protocol: "anthropic"},
		{Type: LLMProviderTypeCodex, Name: "Codex", Protocol: "codex_cli"},
		{Type: LLMProviderTypeDeepSeek, Name: "DeepSeek", Protocol: "openai_compatible"},
	}

	for i := range options {
		defaults := DefaultsForProviderType(options[i].Type)
		options[i].Protocol = defaults.Protocol
		options[i].BaseURL = defaults.BaseURL
		options[i].ChatPath = defaults.ChatPath
		options[i].AuthType = defaults.AuthType
		options[i].DefaultModel = defaults.DefaultModel
		options[i].RequiresAPIKey = defaults.RequiresAPIKey
	}
	return options
}

func DefaultsForProviderType(providerType string) providerDefaults {
	switch NormalizeProviderType(providerType) {
	case LLMProviderTypeOpenAI:
		return providerDefaults{
			Protocol:       "openai_compatible",
			BaseURL:        "https://api.openai.com/v1",
			ChatPath:       "/chat/completions",
			AuthType:       "bearer",
			DefaultModel:   "gpt-4o-mini",
			RequiresAPIKey: true,
		}
	case LLMProviderTypeAnthropic:
		return providerDefaults{
			Protocol:       "anthropic",
			BaseURL:        "https://api.anthropic.com",
			ChatPath:       "/v1/messages",
			AuthType:       "x-api-key",
			DefaultModel:   "claude-3-5-sonnet-latest",
			RequiresAPIKey: true,
		}
	case LLMProviderTypeDeepSeek:
		return providerDefaults{
			Protocol:       "openai_compatible",
			BaseURL:        "https://api.deepseek.com/v1",
			ChatPath:       "/chat/completions",
			AuthType:       "bearer",
			DefaultModel:   "deepseek-chat",
			RequiresAPIKey: true,
		}
	case LLMProviderTypeCodex:
		return providerDefaults{
			Protocol:       "responses",
			BaseURL:        "https://api.openai.com/v1",
			ChatPath:       "/responses",
			AuthType:       "bearer",
			DefaultModel:   "gpt-5",
			RequiresAPIKey: true,
		}
	case LLMProviderTypeOpenAICompatible:
		return providerDefaults{
			Protocol:       "openai_compatible",
			ChatPath:       "/chat/completions",
			AuthType:       "bearer",
			RequiresAPIKey: true,
		}
	default:
		return providerDefaults{
			Protocol:       "openai_compatible",
			ChatPath:       "/chat/completions",
			AuthType:       "bearer",
			RequiresAPIKey: true,
		}
	}
}

func (p *LLMProvider) EncryptAPIKey() error {
	encrypted, err := encryptOptionalSecret(p.APIKey)
	if err != nil {
		return err
	}
	p.APIKey = encrypted
	return nil
}

func (p *LLMProvider) EncryptAPISecret() error {
	encrypted, err := encryptOptionalSecret(p.APISecret)
	if err != nil {
		return err
	}
	p.APISecret = encrypted
	return nil
}

func (p *LLMProvider) DecryptAPIKey() (string, error) {
	return decryptOptionalSecret(p.APIKey)
}

func (p *LLMProvider) DecryptAPISecret() (string, error) {
	return decryptOptionalSecret(p.APISecret)
}

func BuildLLMProviderVO(provider *LLMProvider) LLMProviderVO {
	vo := LLMProviderVO{
		ID:                 provider.ID,
		ModelName:          provider.Name,
		ModelType:          provider.ProviderType,
		ModelCode:          provider.DefaultModel,
		APIURL:             provider.BaseURL,
		Name:               provider.Name,
		ProviderType:       provider.ProviderType,
		Protocol:           provider.Protocol,
		BaseURL:            provider.BaseURL,
		ChatPath:           provider.ChatPath,
		AuthType:           provider.AuthType,
		Organization:       provider.Organization,
		Project:            provider.Project,
		DefaultModel:       provider.DefaultModel,
		Models:             DecodeStringSlice(provider.ModelsJSON),
		Headers:            DecodeStringMap(provider.HeadersJSON),
		Extra:              DecodeAnyMap(provider.ExtraJSON),
		TimeoutSeconds:     provider.TimeoutSeconds,
		MaxRetries:         provider.MaxRetries,
		MaxContextMessages: provider.MaxContextMessages,
		Temperature:        provider.Temperature,
		MaxTokens:          provider.MaxTokens,
		Status:             provider.Status,
		IsDefault:          provider.IsDefault,
		APIKeySet:          strings.TrimSpace(provider.APIKey) != "",
		APISecretSet:       strings.TrimSpace(provider.APISecret) != "",
		Remark:             provider.Remark,
		CreatedAt:          provider.CreatedAt,
		UpdatedAt:          provider.UpdatedAt,
	}

	if vo.APIKeySet {
		if secret, err := provider.DecryptAPIKey(); err == nil {
			vo.APIKeyPreview = MaskSecret(secret)
		}
	}
	return vo
}

func BuildLLMProviderVOs(providers []LLMProvider) []LLMProviderVO {
	result := make([]LLMProviderVO, 0, len(providers))
	for i := range providers {
		result = append(result, BuildLLMProviderVO(&providers[i]))
	}
	return result
}

func EncodeStringSlice(value []string) (string, error) {
	if value == nil {
		value = []string{}
	}
	return encodeJSON(value)
}

func EncodeStringMap(value map[string]string) (string, error) {
	if value == nil {
		value = map[string]string{}
	}
	return encodeJSON(value)
}

func EncodeAnyMap(value map[string]interface{}) (string, error) {
	if value == nil {
		value = map[string]interface{}{}
	}
	return encodeJSON(value)
}

func DecodeStringSlice(raw string) []string {
	var value []string
	if decodeJSON(raw, &value) != nil {
		return []string{}
	}
	if value == nil {
		return []string{}
	}
	return value
}

func DecodeStringMap(raw string) map[string]string {
	var value map[string]string
	if decodeJSON(raw, &value) != nil {
		return map[string]string{}
	}
	if value == nil {
		return map[string]string{}
	}
	return value
}

func DecodeAnyMap(raw string) map[string]interface{} {
	var value map[string]interface{}
	if decodeJSON(raw, &value) != nil {
		return map[string]interface{}{}
	}
	if value == nil {
		return map[string]interface{}{}
	}
	return value
}

func MaskSecret(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ""
	}
	if len(secret) <= 4 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}

func encodeJSON(value interface{}) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func decodeJSON(raw string, target interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "null"
	}
	return json.Unmarshal([]byte(raw), target)
}

func encryptOptionalSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	return util.AESEncrypt(secret)
}

func decryptOptionalSecret(secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	return util.AESDecrypt(secret)
}
