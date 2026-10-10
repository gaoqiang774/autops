package service

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateMCPArguments(t *testing.T) {
	schema := mcpObjectSchema(map[string]string{"assetId": "number", "sql": "string"}, []string{"assetId", "sql"})
	if err := ValidateMCPArguments(schema, map[string]interface{}{"assetId": float64(1), "sql": "SELECT 1"}); err != nil {
		t.Fatalf("valid arguments rejected: %v", err)
	}
	if err := ValidateMCPArguments(schema, map[string]interface{}{"assetId": "1", "sql": "SELECT 1"}); err == nil || !strings.Contains(err.Error(), "assetId") {
		t.Fatalf("invalid number type error = %v", err)
	}
	if err := ValidateMCPArguments(schema, map[string]interface{}{"assetId": float64(1)}); err == nil || !strings.Contains(err.Error(), "sql is required") {
		t.Fatalf("missing required error = %v", err)
	}
}

func TestRedactMCPValueRecursively(t *testing.T) {
	value := map[string]interface{}{
		"password": "plain", "nested": map[string]interface{}{"api_key": "secret", "safe": "visible"},
		"rows": []interface{}{map[string]interface{}{"accessKey": "key"}},
	}
	redacted := RedactMCPValue(value).(map[string]interface{})
	if redacted["password"] != "***REDACTED***" {
		t.Fatalf("password was not redacted: %#v", redacted)
	}
	nested := redacted["nested"].(map[string]interface{})
	if nested["api_key"] != "***REDACTED***" || nested["safe"] != "visible" {
		t.Fatalf("nested redaction mismatch: %#v", nested)
	}
	row := redacted["rows"].([]interface{})[0].(map[string]interface{})
	if row["accessKey"] != "***REDACTED***" {
		t.Fatalf("array redaction mismatch: %#v", row)
	}
}

func TestLimitMCPResultRows(t *testing.T) {
	limited := LimitMCPResultRows(map[string]interface{}{
		"items": []interface{}{1, 2, 3, 4},
	}, 2).(map[string]interface{})
	items := limited["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("limited row count = %d, want 2", len(items))
	}
}

func TestAllowMCPRate(t *testing.T) {
	mcpRateState.Lock()
	mcpRateState.items = map[string]mcpRateWindow{}
	mcpRateState.Unlock()
	now := time.Now()
	if !allowMCPRate("tool", 99, 2, now) || !allowMCPRate("tool", 99, 2, now) {
		t.Fatal("requests inside limit should be allowed")
	}
	if allowMCPRate("tool", 99, 2, now) {
		t.Fatal("request above limit should be rejected")
	}
	if !allowMCPRate("tool", 99, 2, now.Add(time.Minute)) {
		t.Fatal("new minute window should be allowed")
	}
	if !errors.Is(ErrMCPRateLimited, ErrMCPRateLimited) {
		t.Fatal("rate limit sentinel must be comparable")
	}
}
