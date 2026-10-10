package service

import (
	"fmt"
	"strings"

	"dodevops-api/api/aiops/dao"
	"dodevops-api/api/aiops/model"
)

type LLMProviderService struct {
	dao *dao.LLMProviderDao
}

func NewLLMProviderService() *LLMProviderService {
	return &LLMProviderService{
		dao: dao.NewLLMProviderDao(),
	}
}

func (s *LLMProviderService) Create(dto *model.CreateLLMProviderDto) (*model.LLMProviderVO, error) {
	provider, err := buildProviderFromCreateDTO(dto)
	if err != nil {
		return nil, err
	}

	if err := provider.EncryptAPIKey(); err != nil {
		return nil, err
	}
	if err := provider.EncryptAPISecret(); err != nil {
		return nil, err
	}

	if err := s.dao.Create(provider); err != nil {
		return nil, err
	}

	vo := model.BuildLLMProviderVO(provider)
	return &vo, nil
}

func (s *LLMProviderService) Update(dto *model.UpdateLLMProviderDto) (*model.LLMProviderVO, error) {
	existing, err := s.dao.GetByID(dto.ID)
	if err != nil {
		return nil, err
	}

	updates, setDefault, err := buildProviderUpdates(existing, dto)
	if err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		if err := s.dao.Update(dto.ID, updates, setDefault); err != nil {
			return nil, err
		}
	}

	updated, err := s.dao.GetByID(dto.ID)
	if err != nil {
		return nil, err
	}
	vo := model.BuildLLMProviderVO(updated)
	return &vo, nil
}

func (s *LLMProviderService) Delete(id uint) error {
	return s.dao.Delete(id)
}

func (s *LLMProviderService) GetByID(id uint) (*model.LLMProviderVO, error) {
	provider, err := s.dao.GetByID(id)
	if err != nil {
		return nil, err
	}
	vo := model.BuildLLMProviderVO(provider)
	return &vo, nil
}

func (s *LLMProviderService) List(query model.ListLLMProviderQuery) ([]model.LLMProviderVO, int64, error) {
	query = normalizeListQuery(query)
	providers, total, err := s.dao.List(query)
	if err != nil {
		return nil, 0, err
	}
	return model.BuildLLMProviderVOs(providers), total, nil
}

func (s *LLMProviderService) GetEnabled() ([]model.LLMProviderVO, error) {
	providers, err := s.dao.GetEnabled()
	if err != nil {
		return nil, err
	}
	return model.BuildLLMProviderVOs(providers), nil
}

func (s *LLMProviderService) SetDefault(id uint) (*model.LLMProviderVO, error) {
	if err := s.dao.SetDefault(id); err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *LLMProviderService) GetSecret(id uint) (*model.LLMProviderSecretVO, error) {
	provider, err := s.dao.GetByID(id)
	if err != nil {
		return nil, err
	}

	apiKey, err := provider.DecryptAPIKey()
	if err != nil {
		return nil, err
	}
	apiSecret, err := provider.DecryptAPISecret()
	if err != nil {
		return nil, err
	}

	return &model.LLMProviderSecretVO{
		ID:        provider.ID,
		APIKey:    apiKey,
		APISecret: apiSecret,
	}, nil
}

func (s *LLMProviderService) ProviderTypes() []model.LLMProviderTypeOption {
	return model.ProviderTypeOptions()
}

func buildProviderFromCreateDTO(dto *model.CreateLLMProviderDto) (*model.LLMProvider, error) {
	name := firstNonEmpty(dto.ModelName, dto.Name)
	if name == "" {
		return nil, fmt.Errorf("modelName is required")
	}

	providerType := model.NormalizeProviderType(firstNonEmpty(dto.ModelType, dto.ProviderType))
	if providerType == "" {
		return nil, fmt.Errorf("modelType is required")
	}
	if !model.IsSupportedProviderType(providerType) {
		return nil, fmt.Errorf("unsupported modelType: %s", providerType)
	}

	defaults := model.DefaultsForProviderType(providerType)
	protocol := firstNonEmpty(dto.Protocol, defaults.Protocol, "openai_compatible")
	baseURL := firstNonEmpty(dto.APIURL, dto.APIURLSnake, dto.BaseURL, defaults.BaseURL)
	chatPath := firstNonEmpty(dto.ChatPath, defaults.ChatPath)
	authType := firstNonEmpty(dto.AuthType, defaults.AuthType, "bearer")
	defaultModel := firstNonEmpty(dto.ModelCode, dto.DefaultModel, defaults.DefaultModel)

	if providerType == model.LLMProviderTypeOpenAICompatible && strings.TrimSpace(baseURL) == "" {
		return nil, fmt.Errorf("apiUrl is required for openai_compatible modelType")
	}
	if strings.TrimSpace(defaultModel) == "" {
		return nil, fmt.Errorf("modelCode is required")
	}

	modelsJSON, err := model.EncodeStringSlice(cleanStringSlice(dto.Models))
	if err != nil {
		return nil, err
	}
	headersJSON, err := model.EncodeStringMap(dto.Headers)
	if err != nil {
		return nil, err
	}
	extraJSON, err := model.EncodeAnyMap(dto.Extra)
	if err != nil {
		return nil, err
	}

	temperature := 0.7
	if dto.Temperature != nil {
		temperature = *dto.Temperature
	}
	if err := validateGenerationConfig(dto.TimeoutSeconds, dto.MaxRetries, dto.MaxContextMessages, temperature, dto.MaxTokens, dto.Status, true); err != nil {
		return nil, err
	}

	timeoutSeconds := dto.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	maxRetries := dto.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}
	maxContextMessages := dto.MaxContextMessages
	if maxContextMessages <= 0 {
		maxContextMessages = 20
	}
	status := dto.Status
	if status == 0 {
		status = model.LLMProviderStatusEnabled
	}

	return &model.LLMProvider{
		Name:               name,
		ProviderType:       providerType,
		Protocol:           strings.TrimSpace(protocol),
		BaseURL:            strings.TrimSpace(baseURL),
		ChatPath:           strings.TrimSpace(chatPath),
		APIKey:             firstNonEmpty(dto.APIKey, dto.APIKeySnake),
		APISecret:          strings.TrimSpace(dto.APISecret),
		AuthType:           strings.TrimSpace(authType),
		Organization:       strings.TrimSpace(dto.Organization),
		Project:            strings.TrimSpace(dto.Project),
		DefaultModel:       strings.TrimSpace(defaultModel),
		ModelsJSON:         modelsJSON,
		HeadersJSON:        headersJSON,
		ExtraJSON:          extraJSON,
		TimeoutSeconds:     timeoutSeconds,
		MaxRetries:         maxRetries,
		MaxContextMessages: maxContextMessages,
		Temperature:        temperature,
		MaxTokens:          dto.MaxTokens,
		Status:             status,
		IsDefault:          dto.IsDefault,
		Remark:             strings.TrimSpace(dto.Remark),
	}, nil
}

func buildProviderUpdates(existing *model.LLMProvider, dto *model.UpdateLLMProviderDto) (map[string]interface{}, bool, error) {
	updates := make(map[string]interface{})
	setDefault := false

	if nameValue := firstStringPointer(dto.ModelName, dto.Name); nameValue != nil {
		name := strings.TrimSpace(*nameValue)
		if name == "" {
			return nil, false, fmt.Errorf("modelName cannot be empty")
		}
		updates["name"] = name
	}

	providerTypeChanged := false
	if providerTypeValue := firstStringPointer(dto.ModelType, dto.ProviderType); providerTypeValue != nil {
		providerType := model.NormalizeProviderType(*providerTypeValue)
		if providerType == "" {
			return nil, false, fmt.Errorf("modelType cannot be empty")
		}
		if !model.IsSupportedProviderType(providerType) {
			return nil, false, fmt.Errorf("unsupported modelType: %s", providerType)
		}
		updates["provider_type"] = providerType
		providerTypeChanged = providerType != existing.ProviderType
	}

	effectiveProviderType := existing.ProviderType
	if value, ok := updates["provider_type"].(string); ok {
		effectiveProviderType = value
	}
	defaults := model.DefaultsForProviderType(effectiveProviderType)

	applyStringUpdate(updates, "protocol", dto.Protocol)
	applyStringUpdate(updates, "base_url", firstStringPointer(dto.APIURL, dto.APIURLSnake, dto.BaseURL))
	applyStringUpdate(updates, "chat_path", dto.ChatPath)
	applyStringUpdate(updates, "auth_type", dto.AuthType)
	applyStringUpdate(updates, "organization", dto.Organization)
	applyStringUpdate(updates, "project", dto.Project)
	applyStringUpdate(updates, "default_model", firstStringPointer(dto.ModelCode, dto.DefaultModel))
	applyStringUpdate(updates, "remark", dto.Remark)

	if providerTypeChanged {
		applyDefaultIfMissing(updates, "protocol", defaults.Protocol)
		applyDefaultIfMissing(updates, "base_url", defaults.BaseURL)
		applyDefaultIfMissing(updates, "chat_path", defaults.ChatPath)
		applyDefaultIfMissing(updates, "auth_type", defaults.AuthType)
		applyDefaultIfMissing(updates, "default_model", defaults.DefaultModel)
	}

	if apiKeyValue := firstStringPointer(dto.APIKey, dto.APIKeySnake); apiKeyValue != nil {
		encrypted, err := encryptAPIKeyValue(*apiKeyValue)
		if err != nil {
			return nil, false, err
		}
		updates["api_key"] = encrypted
	}
	if dto.ClearAPIKey {
		updates["api_key"] = ""
	}
	if dto.APISecret != nil {
		encrypted, err := encryptAPISecretValue(*dto.APISecret)
		if err != nil {
			return nil, false, err
		}
		updates["api_secret"] = encrypted
	}
	if dto.ClearAPISecret {
		updates["api_secret"] = ""
	}

	if dto.Models != nil {
		modelsJSON, err := model.EncodeStringSlice(cleanStringSlice(dto.Models))
		if err != nil {
			return nil, false, err
		}
		updates["models"] = modelsJSON
	}
	if dto.Headers != nil {
		headersJSON, err := model.EncodeStringMap(dto.Headers)
		if err != nil {
			return nil, false, err
		}
		updates["headers"] = headersJSON
	}
	if dto.Extra != nil {
		extraJSON, err := model.EncodeAnyMap(dto.Extra)
		if err != nil {
			return nil, false, err
		}
		updates["extra"] = extraJSON
	}

	if dto.TimeoutSeconds != nil {
		if *dto.TimeoutSeconds <= 0 {
			return nil, false, fmt.Errorf("timeoutSeconds must be greater than 0")
		}
		updates["timeout_seconds"] = *dto.TimeoutSeconds
	}
	if dto.MaxRetries != nil {
		if *dto.MaxRetries < 0 {
			return nil, false, fmt.Errorf("maxRetries cannot be negative")
		}
		updates["max_retries"] = *dto.MaxRetries
	}
	if dto.MaxContextMessages != nil {
		if *dto.MaxContextMessages <= 0 {
			return nil, false, fmt.Errorf("maxContextMessages must be greater than 0")
		}
		updates["max_context_messages"] = *dto.MaxContextMessages
	}
	if dto.Temperature != nil {
		if err := validateTemperature(*dto.Temperature); err != nil {
			return nil, false, err
		}
		updates["temperature"] = *dto.Temperature
	}
	if dto.MaxTokens != nil {
		if *dto.MaxTokens < 0 {
			return nil, false, fmt.Errorf("maxTokens cannot be negative")
		}
		updates["max_tokens"] = *dto.MaxTokens
	}
	if dto.Status != nil {
		if err := validateStatus(*dto.Status, false); err != nil {
			return nil, false, err
		}
		updates["status"] = *dto.Status
	}
	if dto.IsDefault != nil {
		updates["is_default"] = *dto.IsDefault
		setDefault = *dto.IsDefault
		if *dto.IsDefault {
			updates["status"] = model.LLMProviderStatusEnabled
		}
	}

	return updates, setDefault, nil
}

func normalizeListQuery(query model.ListLLMProviderQuery) model.ListLLMProviderQuery {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	query.ProviderType = model.NormalizeProviderType(query.ProviderType)
	query.Protocol = strings.TrimSpace(query.Protocol)
	return query
}

func validateGenerationConfig(timeoutSeconds, maxRetries, maxContextMessages int, temperature float64, maxTokens int, status int, allowDefaultStatus bool) error {
	if timeoutSeconds < 0 {
		return fmt.Errorf("timeoutSeconds cannot be negative")
	}
	if maxRetries < 0 {
		return fmt.Errorf("maxRetries cannot be negative")
	}
	if maxContextMessages < 0 {
		return fmt.Errorf("maxContextMessages cannot be negative")
	}
	if maxTokens < 0 {
		return fmt.Errorf("maxTokens cannot be negative")
	}
	if err := validateTemperature(temperature); err != nil {
		return err
	}
	return validateStatus(status, allowDefaultStatus)
}

func validateTemperature(temperature float64) error {
	if temperature < 0 || temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	return nil
}

func validateStatus(status int, allowDefault bool) error {
	if allowDefault && status == 0 {
		return nil
	}
	switch status {
	case model.LLMProviderStatusEnabled, model.LLMProviderStatusDisabled:
		return nil
	default:
		return fmt.Errorf("status must be 1(enabled) or 2(disabled)")
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

func firstStringPointer(values ...*string) *string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func cleanStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func applyStringUpdate(updates map[string]interface{}, column string, value *string) {
	if value == nil {
		return
	}
	updates[column] = strings.TrimSpace(*value)
}

func applyDefaultIfMissing(updates map[string]interface{}, column string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, ok := updates[column]; ok {
		return
	}
	updates[column] = strings.TrimSpace(value)
}

func encryptAPIKeyValue(value string) (string, error) {
	provider := model.LLMProvider{APIKey: strings.TrimSpace(value)}
	if err := provider.EncryptAPIKey(); err != nil {
		return "", err
	}
	return provider.APIKey, nil
}

func encryptAPISecretValue(value string) (string, error) {
	provider := model.LLMProvider{APISecret: strings.TrimSpace(value)}
	if err := provider.EncryptAPISecret(); err != nil {
		return "", err
	}
	return provider.APISecret, nil
}
