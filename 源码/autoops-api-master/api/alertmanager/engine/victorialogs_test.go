package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"dodevops-api/api/alertmanager/model"
)

func TestEvaluateVictoriaLogsRuleFiresWhenHitsExceedThreshold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/select/logsql/hits" {
			t.Fatalf("path = %s, want /select/logsql/hits", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("query"); got != "error" {
			t.Fatalf("query = %q, want error", got)
		}
		if got := r.Form.Get("step"); got != "5m" {
			t.Fatalf("step = %q, want 5m", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hits": []map[string]interface{}{
				{"total": 12},
			},
		})
	}))
	defer server.Close()

	rule := model.AlertRule{
		ID:                7,
		Query:             "error",
		ConditionOperator: ">",
		ConditionValue:    10,
		Severity:          "warning",
		RuleConfig: model.JSONMap{
			"windowSeconds": 300,
		},
	}
	ds := model.AlertDatasource{
		DatasourceType: model.AlertDatasourceVictoriaLogs,
		URL:            server.URL,
		AuthType:       "none",
	}

	samples, err := evaluateRule(context.Background(), rule, ds)
	if err != nil {
		t.Fatalf("evaluateRule() error = %v", err)
	}
	if len(samples) != 1 {
		t.Fatalf("len(samples) = %d, want 1", len(samples))
	}
	if samples[0].Value != "12" {
		t.Fatalf("sample value = %q, want 12", samples[0].Value)
	}
	if samples[0].TriggerCondition != "error [window=5m] > 10" {
		t.Fatalf("condition = %q, want window condition", samples[0].TriggerCondition)
	}
}

func TestEvaluateVictoriaLogsRuleSkipsWhenHitsDoNotMatchThreshold(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hits": []map[string]interface{}{
				{"values": []int{1, 2}},
			},
		})
	}))
	defer server.Close()

	rule := model.AlertRule{
		ID:                8,
		Query:             "error",
		ConditionOperator: ">",
		ConditionValue:    10,
		Severity:          "warning",
	}
	ds := model.AlertDatasource{
		DatasourceType: model.AlertDatasourceVictoriaLogs,
		URL:            server.URL,
		AuthType:       "none",
	}

	samples, err := evaluateRule(context.Background(), rule, ds)
	if err != nil {
		t.Fatalf("evaluateRule() error = %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("len(samples) = %d, want 0", len(samples))
	}
}

func TestQueryVictoriaLogsCountSendsConfiguredHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("AccountID"); got != "100" {
			t.Fatalf("AccountID = %q, want 100", got)
		}
		if got := r.Header.Get("ProjectID"); got != "200" {
			t.Fatalf("ProjectID = %q, want 200", got)
		}
		if got := r.Header.Get("X-Test"); got != "ok" {
			t.Fatalf("X-Test = %q, want ok", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hits": []map[string]interface{}{
				{"total": strconv.Itoa(5)},
			},
		})
	}))
	defer server.Close()

	ds := model.AlertDatasource{
		DatasourceType: model.AlertDatasourceVictoriaLogs,
		URL:            server.URL,
		AuthType:       "none",
		Config: model.JSONMap{
			"accountID": "100",
			"projectID": "200",
			"headers": map[string]interface{}{
				"X-Test": "ok",
			},
		},
	}

	result, err := QueryVictoriaLogsCount(context.Background(), ds, "error", time.Time{}, 60)
	if err != nil {
		t.Fatalf("QueryVictoriaLogsCount() error = %v", err)
	}
	if result.Count != 5 {
		t.Fatalf("count = %d, want 5", result.Count)
	}
}

func TestQueryVictoriaLogsCountSendsScopeFilters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		raw := r.Form.Get("extra_filters")
		if raw == "" {
			t.Fatalf("extra_filters is empty")
		}
		var filters map[string]string
		if err := json.Unmarshal([]byte(raw), &filters); err != nil {
			t.Fatalf("decode extra_filters error = %v", err)
		}
		if filters["project_code"] != "project_1" || filters["store_code"] != "store_1" {
			t.Fatalf("filters = %#v, want project_1/store_1", filters)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"hits": []map[string]interface{}{
				{"total": 3},
			},
		})
	}))
	defer server.Close()

	ds := model.AlertDatasource{
		DatasourceType: model.AlertDatasourceVictoriaLogs,
		URL:            server.URL,
		AuthType:       "none",
	}

	result, err := QueryVictoriaLogsCount(context.Background(), ds, "error", time.Time{}, 60, model.JSONMap{
		"projectCode": "project_1",
		"storeCode":   "store_1",
	})
	if err != nil {
		t.Fatalf("QueryVictoriaLogsCount() error = %v", err)
	}
	if result.Count != 3 {
		t.Fatalf("count = %d, want 3", result.Count)
	}
}

func TestQueryVictoriaLogsSampleLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/select/logsql/query" {
			t.Fatalf("path = %s, want /select/logsql/query", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error = %v", err)
		}
		if got := r.Form.Get("limit"); got != "2" {
			t.Fatalf("limit = %q, want 2", got)
		}
		_, _ = w.Write([]byte("{\"_msg\":\"error one\",\"host_name\":\"app-1\"}\n{\"_msg\":\"error two\",\"host_name\":\"app-2\"}\n"))
	}))
	defer server.Close()

	ds := model.AlertDatasource{
		DatasourceType: model.AlertDatasourceVictoriaLogs,
		URL:            server.URL,
		AuthType:       "none",
	}

	result, err := QueryVictoriaLogsSampleLogs(context.Background(), ds, "error", time.Time{}, 300, 2)
	if err != nil {
		t.Fatalf("QueryVictoriaLogsSampleLogs() error = %v", err)
	}
	if result.Count != 2 || len(result.Logs) != 2 {
		t.Fatalf("logs count = %d len=%d, want 2", result.Count, len(result.Logs))
	}
	if result.Logs[0]["_msg"] != "error one" {
		t.Fatalf("first log _msg = %#v, want error one", result.Logs[0]["_msg"])
	}
}
