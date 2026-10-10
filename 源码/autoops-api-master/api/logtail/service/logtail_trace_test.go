package service

import (
	"strings"
	"testing"
	"time"

	logtailmodel "dodevops-api/api/logtail/model"
)

func TestBuildLogTraceQuerySearchesMessageAndIdentifierFields(t *testing.T) {
	traceID := "879bdf41-7d12-4d37-8538-bcc73f82cbe0"
	query := buildLogTraceQuery(traceID)
	for _, want := range []string{
		"*" + traceID + "*",
		`trace_id:="` + traceID + `"`,
		`x_zai_trace_id:="` + traceID + `"`,
		`span_id:="` + traceID + `"`,
		`request_id:="` + traceID + `"`,
		`session_id:="` + traceID + `"`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("trace query %q missing %q", query, want)
		}
	}
}

func TestBuildLogTraceAnalysisCorrelatesServicesWithoutClaimingCallOrder(t *testing.T) {
	traceID := "879bdf41-7d12-4d37-8538-bcc73f82cbe0"
	rows := []logTraceRow{
		{
			Store: logtailmodel.LogStore{ID: 1, Code: "zero_ai_im_46b95c48", Name: "zero-ai-im"},
			Row: map[string]interface{}{
				"_time":             "2026-07-21T11:03:01.480681293Z",
				"service":           "zero-ai-im",
				"app":               "zero_ai_im_46b95c48",
				"level":             "INFO",
				"collect_task_code": "zero_ai_im_4660526a",
				"message":           "forward.begin headers={'X-ZAI-Internal-Key': 'internal-secret'}",
			},
		},
		{
			Store: logtailmodel.LogStore{ID: 2, Code: "zero_ai_agent_e3e1893b", Name: "zero-ai-agent"},
			Row: map[string]interface{}{
				"_time":   "2026-07-21T11:03:53.186623694Z",
				"service": "zero-ai-agent",
				"level":   "INFO",
				"message": "curl -X POST 'https://zero-ai-api.leyoujia.com/im_api/openclaw/contacts' -H 'Authorization: Bearer bearer-secret' -H 'X-ZAI-Session-Token: session-secret'",
			},
		},
		{
			Store: logtailmodel.LogStore{ID: 3, Code: "zero_ai_nginx_0ad6c306", Name: "zeroai-gateway"},
			Row: map[string]interface{}{
				"_time":             "2026-07-21T11:03:53.210Z",
				"collect_task_name": "zeroai-gateway",
				"app":               "zero_ai_nginx_0ad6c306",
				"request_method":    "POST",
				"request_path":      "/im_api/openclaw/contacts",
				"request_time":      "0.024",
				"status":            200.0,
				"level":             "INFO",
			},
		},
		{
			Store: logtailmodel.LogStore{ID: 4, Code: "zero_ai_model_1bf85c34", Name: "zero-ai-model"},
			Row: map[string]interface{}{
				"_time":       "2026-07-21T11:04:00.644Z",
				"service":     "model_layer",
				"event":       "model.call",
				"duration_ms": 2409.0,
				"status":      "success",
				"level":       "INFO",
				"trace_id":    traceID,
				"message":     "deepseek/deepseek-v4-flash 64635 tokens",
			},
		},
	}

	result := buildLogTraceAnalysis(
		logtailmodel.LogProject{ID: 1, Code: "zero_ai", Name: "zero-ai"},
		traceID,
		"2026-07-21T11:00:00Z",
		"2026-07-21T11:05:00Z",
		rows,
		100,
	)

	if result.Mode != "log_correlation" || result.HasCallOrder {
		t.Fatalf("unexpected correlation mode: %#v", result)
	}
	if result.EventCount != 4 || result.ServiceCount != 4 || result.Status != "success" {
		t.Fatalf("unexpected trace summary: %#v", result)
	}
	if result.Events[0].Service != "zero-ai-im" || result.Events[3].Service != "model_layer" {
		t.Fatalf("events are not sorted by observed time: %#v", result.Events)
	}
	agent := result.Events[1]
	if agent.RequestMethod != "POST" || agent.RequestPath != "/im_api/openclaw/contacts" {
		t.Fatalf("curl operation was not extracted: %#v", agent)
	}
	if !agent.IsKeyEvent || agent.EventType != "http_request" || agent.DisplayName != "POST /im_api/openclaw/contacts" {
		t.Fatalf("curl event was not classified for timeline display: %#v", agent)
	}
	if result.Events[0].IsKeyEvent || result.Events[0].EventType != "internal_log" {
		t.Fatalf("plain INFO log should be collapsible in timeline: %#v", result.Events[0])
	}
	if !result.Events[3].IsKeyEvent || result.Events[3].EventType != "model_call" || result.Events[3].DisplayName != "model.call" {
		t.Fatalf("model event was not classified for timeline display: %#v", result.Events[3])
	}
	for _, secret := range []string{"internal-secret", "bearer-secret", "session-secret"} {
		for _, event := range result.Events {
			if strings.Contains(event.Message, secret) {
				t.Fatalf("trace evidence leaked %q: %s", secret, event.Message)
			}
		}
	}
	if result.Events[2].DurationMs != 24 || result.Events[3].DurationMs != 2409 {
		t.Fatalf("unexpected normalized durations: %#v", result.Events)
	}
}

func TestBuildLogTraceAnalysisPromotesHTTPFailure(t *testing.T) {
	result := buildLogTraceAnalysis(
		logtailmodel.LogProject{ID: 1, Code: "zero_ai", Name: "zero-ai"},
		"879bdf41-7d12-4d37-8538-bcc73f82cbe0",
		"2026-07-21T11:00:00Z",
		"2026-07-21T11:05:00Z",
		[]logTraceRow{{
			Store: logtailmodel.LogStore{ID: 1, Code: "gateway", Name: "gateway"},
			Row: map[string]interface{}{
				"_time":   "2026-07-21T11:03:53Z",
				"service": "gateway",
				"status":  503.0,
			},
		}},
		100,
	)

	if result.Status != "error" || result.ErrorCount != 1 || result.Services[0].Status != "error" {
		t.Fatalf("HTTP failure was not promoted: %#v", result)
	}
}

func TestBuildLogTraceListStatsQueryUsesStorageSideAggregation(t *testing.T) {
	traceID := "879bdf41-7d12-4d37-8538-bcc73f82cbe0"
	query := buildLogTraceListStatsQuery("x_zai_trace_id", traceID, 51)
	for _, want := range []string{
		`x_zai_trace_id:="` + traceID + `"`,
		`stats by (x_zai_trace_id)`,
		`min(_time) as first_seen`,
		`max(_time) as last_seen`,
		`max(responsetime) as max_response_seconds`,
		`status:failed`,
		`sort by (last_seen desc)`,
		`limit 51`,
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("trace list query %q missing %q", query, want)
		}
	}
}

func TestParseLogTraceListStatNormalizesStatusAndNginxDuration(t *testing.T) {
	stat, ok := parseLogTraceListStat(
		logtailmodel.LogStore{ID: 1, Code: "gateway", Name: "gateway"},
		"trace_id",
		map[string]interface{}{
			"trace_id":             "879bdf41-7d12-4d37-8538-bcc73f82cbe0",
			"first_seen":           "2026-07-21T11:03:53Z",
			"last_seen":            "2026-07-21T11:04:00Z",
			"event_count":          "2",
			"status_errors":        "1",
			"max_response_seconds": "65.076",
		},
	)
	if !ok {
		t.Fatal("expected a valid trace list stat")
	}
	if stat.ErrorCount != 1 || stat.DurationMs != 65076 {
		t.Fatalf("unexpected normalized stat: %#v", stat)
	}
}

func TestBuildLogTraceListItemsMergesStoresAndAliasDuplicates(t *testing.T) {
	traceID := "879bdf41-7d12-4d37-8538-bcc73f82cbe0"
	parseTime := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	modelStore := logtailmodel.LogStore{ID: 1, Code: "zero_ai_model", Name: "zero-ai-model"}
	stats := []logTraceListStat{
		{
			TraceID: traceID, Store: modelStore,
			FirstSeen: parseTime("2026-07-21T11:04:00.640Z"), LastSeen: parseTime("2026-07-21T11:04:00.644Z"),
			EventCount: 2, DurationMs: 2409, Service: "model_layer", Operation: "model.call", OperationRank: 1,
		},
		// The same store can expose the same event through both trace field aliases.
		{
			TraceID: traceID, Store: modelStore,
			FirstSeen: parseTime("2026-07-21T11:04:00.640Z"), LastSeen: parseTime("2026-07-21T11:04:00.644Z"),
			EventCount: 2, DurationMs: 2409, Service: "model_layer", Operation: "model.call", OperationRank: 1,
		},
		{
			TraceID: traceID, Store: logtailmodel.LogStore{ID: 2, Code: "zeroai_nginx", Name: "zeroai-nginx"},
			FirstSeen: parseTime("2026-07-21T11:03:55Z"), LastSeen: parseTime("2026-07-21T11:04:00Z"),
			EventCount: 1, DurationMs: 65076, Service: "zeroai-nginx", Operation: "POST /zero-ai-api/v1/chat/stream", OperationRank: 2,
		},
		{
			TraceID: traceID, Store: logtailmodel.LogStore{ID: 3, Code: "zeroai_gateway", Name: "zeroai-gateway"},
			FirstSeen: parseTime("2026-07-21T11:03:53Z"), LastSeen: parseTime("2026-07-21T11:03:53.024Z"),
			EventCount: 1, ErrorCount: 1, DurationMs: 24, Service: "zeroai-gateway", Operation: "POST /im_api/openclaw/contacts", OperationRank: 2,
		},
	}

	items := buildLogTraceListItems(stats)
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	item := items[0]
	if item.EventCount != 4 {
		t.Fatalf("event count = %d, want alias-deduplicated 4", item.EventCount)
	}
	if item.ServiceCount != 3 || item.ErrorCount != 1 || item.Status != "error" {
		t.Fatalf("unexpected merged summary: %#v", item)
	}
	if item.PrimaryOperation != "POST /zero-ai-api/v1/chat/stream" {
		t.Fatalf("primary operation = %q", item.PrimaryOperation)
	}
	if item.MaxDurationMs != 65076 {
		t.Fatalf("max duration = %v", item.MaxDurationMs)
	}
	if len(filterLogTraceListItems(items, "error", "gateway")) != 1 {
		t.Fatal("error/service filter should keep the trace")
	}
	if len(filterLogTraceListItems(items, "success", "")) != 0 {
		t.Fatal("success filter should exclude the failed trace")
	}
}
