package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dodevops-api/api/aiops/model"
)

func TestOpenAICompatibleProviderUsesAPIKeyHeaderAndXiaomiPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("api-key"); got != "secret" {
			t.Fatalf("api-key header = %q, want %q", got, "secret")
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization header = %q, want empty", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["model"]; got != "mimo-v2.5-pro" {
			t.Fatalf("model = %v, want mimo-v2.5-pro", got)
		}
		if got := payload["max_completion_tokens"]; got != float64(128) {
			t.Fatalf("max_completion_tokens = %v, want 128", got)
		}
		if _, ok := payload["max_tokens"]; ok {
			t.Fatalf("max_tokens should not be sent for MiMo: %#v", payload)
		}
		thinking, ok := payload["thinking"].(map[string]interface{})
		if !ok || thinking["type"] != "disabled" {
			t.Fatalf("thinking = %#v, want disabled thinking", payload["thinking"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()

	provider := &model.LLMProvider{
		Name:         "MiMo",
		ProviderType: model.LLMProviderTypeOpenAICompatible,
		BaseURL:      server.URL + "/v1",
		ChatPath:     "/chat/completions",
		AuthType:     "api-key",
		DefaultModel: "mimo-v2.5-pro",
	}
	result, err := testOpenAICompatibleProvider(context.Background(), provider, "secret", "ping", &model.TestLLMProviderResult{})
	if err != nil {
		t.Fatalf("test provider: %v", err)
	}
	if !result.Success {
		t.Fatalf("result success = false: %#v", result)
	}
	if result.Response != "OK" {
		t.Fatalf("response = %q, want OK", result.Response)
	}
}

func TestOpenAICompatibleProviderDefaultsToBearerAndMaxTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := payload["max_tokens"]; !ok {
			t.Fatalf("max_tokens was not sent: %#v", payload)
		}
		if _, ok := payload["max_completion_tokens"]; ok {
			t.Fatalf("max_completion_tokens should not be sent by default: %#v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()

	provider := &model.LLMProvider{
		Name:         "OpenAI Compatible",
		ProviderType: model.LLMProviderTypeOpenAICompatible,
		BaseURL:      server.URL + "/v1",
		ChatPath:     "/chat/completions",
		AuthType:     "bearer",
		DefaultModel: "custom-model",
	}
	result, err := testOpenAICompatibleProvider(context.Background(), provider, "secret", "ping", &model.TestLLMProviderResult{})
	if err != nil {
		t.Fatalf("test provider: %v", err)
	}
	if !result.Success {
		t.Fatalf("result success = false: %#v", result)
	}
}

func TestCodexProviderWithBaseURLCanUseHTTPTest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization header = %q, want bearer token", got)
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := payload["model"]; got != "gpt-5.4" {
			t.Fatalf("model = %v, want gpt-5.4", got)
		}
		if _, ok := payload["max_output_tokens"]; !ok {
			t.Fatalf("max_output_tokens was not sent for gpt-5 model: %#v", payload)
		}
		reasoning, ok := payload["reasoning"].(map[string]interface{})
		if !ok || reasoning["effort"] != "high" {
			t.Fatalf("reasoning = %#v, want high effort", payload["reasoning"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"OK"}`))
	}))
	defer server.Close()

	provider := &model.LLMProvider{
		Name:         "gpt-5.4",
		ProviderType: model.LLMProviderTypeCodex,
		Protocol:     "codex_cli",
		BaseURL:      server.URL,
		AuthType:     "none",
		DefaultModel: "gpt-5.4",
	}
	normalizeCodexProvider(provider)
	result, err := testResponsesProvider(context.Background(), provider, "secret", "ping", &model.TestLLMProviderResult{})
	if err != nil {
		t.Fatalf("test provider: %v", err)
	}
	if !result.Success {
		t.Fatalf("result success = false: %#v", result)
	}
}

func TestResponsesAnalysisDefaultsToLowReasoningForGPT5(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		reasoning, ok := payload["reasoning"].(map[string]interface{})
		if !ok || reasoning["effort"] != "low" {
			t.Fatalf("reasoning = %#v, want low effort", payload["reasoning"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"OK"}`))
	}))
	defer server.Close()

	provider := &model.LLMProvider{
		Name:         "gpt-5.5",
		ProviderType: model.LLMProviderTypeCodex,
		BaseURL:      server.URL + "/v1",
		ChatPath:     "/responses",
		AuthType:     "bearer",
		DefaultModel: "gpt-5.5",
	}
	content, err := callResponsesAPI(context.Background(), provider, "secret", "system", "logs")
	if err != nil {
		t.Fatalf("call responses api: %v", err)
	}
	if content != "OK" {
		t.Fatalf("content = %q, want OK", content)
	}
}

func TestResponsesAnalysisRequestBodyCanOverrideReasoning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		reasoning, ok := payload["reasoning"].(map[string]interface{})
		if !ok || reasoning["effort"] != "medium" {
			t.Fatalf("reasoning = %#v, want medium effort", payload["reasoning"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"OK"}`))
	}))
	defer server.Close()

	extra, err := model.EncodeAnyMap(map[string]interface{}{
		"analysisRequestBody": map[string]interface{}{
			"reasoning": map[string]string{"effort": "medium"},
		},
	})
	if err != nil {
		t.Fatalf("encode extra: %v", err)
	}
	provider := &model.LLMProvider{
		Name:         "gpt-5.5",
		ProviderType: model.LLMProviderTypeCodex,
		BaseURL:      server.URL + "/v1",
		ChatPath:     "/responses",
		AuthType:     "bearer",
		DefaultModel: "gpt-5.5",
		ExtraJSON:    extra,
	}
	content, err := callResponsesAPI(context.Background(), provider, "secret", "system", "logs")
	if err != nil {
		t.Fatalf("call responses api: %v", err)
	}
	if content != "OK" {
		t.Fatalf("content = %q, want OK", content)
	}
}

func TestParseResponsesAPIResponseReadsOutputText(t *testing.T) {
	body := []byte(`{"output_text":"OK"}`)

	content, err := parseResponsesAPIResponse(body)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if content != "OK" {
		t.Fatalf("content = %q, want OK", content)
	}
}

func TestParseOpenAICompatibleResponseAcceptsReasoningOnlyChoice(t *testing.T) {
	body := []byte(`{"choices":[{"finish_reason":"length","message":{"content":"","reasoning_content":"internal reasoning"}}]}`)

	content, err := parseOpenAICompatibleResponse(body)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if content != "model returned reasoning output but no final content" {
		t.Fatalf("content = %q, want reasoning-only connectivity message", content)
	}
}

func TestLLMTestRequestTreatsHTTP200ParseFailureAsInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":""}}]}`))
	}))
	defer server.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	result, err := doLLMTestRequest(req, &model.TestLLMProviderResult{}, parseOpenAICompatibleResponse)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if result.Success {
		t.Fatalf("result success = true, want false: %#v", result)
	}
	if result.Error == "" {
		t.Fatalf("result error should be preserved for diagnostics")
	}
	if result.Response == "" {
		t.Fatalf("response should keep raw body for diagnostics")
	}
}
