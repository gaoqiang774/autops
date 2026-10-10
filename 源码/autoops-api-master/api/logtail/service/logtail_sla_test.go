package service

import (
	"strings"
	"testing"

	logtailmodel "dodevops-api/api/logtail/model"
)

func TestBuildLogSLAQueriesUseConditionalStatsSyntax(t *testing.T) {
	queries := []string{
		buildLogSLASummaryQuery(1),
		buildLogSLATopAPIQuery(),
	}
	for _, query := range queries {
		if strings.Contains(query, "count(status:") || strings.Contains(query, "count(request_time:") {
			t.Fatalf("query contains unsupported conditional count syntax: %s", query)
		}
		if !strings.Contains(query, "count() if (status:>=400 status:<500)") {
			t.Fatalf("query missing LogsQL conditional stats syntax: %s", query)
		}
	}
	if !strings.Contains(queries[0], "count() if (request_time:>1)") {
		t.Fatalf("summary query missing slow request condition: %s", queries[0])
	}
}

func TestParseLogSLAMetrics(t *testing.T) {
	metrics := parseLogSLAMetrics([]map[string]interface{}{{
		"request_total":       "1000",
		"http_4xx":            40.0,
		"http_5xx":            10.0,
		"slow_request_count":  25.0,
		"avg_latency_seconds": "0.125",
		"p95_latency_seconds": 0.8,
	}})

	if metrics.RequestTotal != 1000 || metrics.HTTP4xx != 40 || metrics.HTTP5xx != 10 {
		t.Fatalf("unexpected counts: %#v", metrics)
	}
	if metrics.SuccessRate != 95 || metrics.AvailabilityRate != 99 || metrics.ErrorRate != 5 {
		t.Fatalf("unexpected rates: %#v", metrics)
	}
	if metrics.AvgLatencyMs != 125 || metrics.P95LatencyMs != 800 {
		t.Fatalf("unexpected latency: %#v", metrics)
	}
}

func TestParseLogSLATrendGroupsHTTPStatusClasses(t *testing.T) {
	trend := parseLogSLATrend([]logtailmodel.LogQueryHitSeries{
		{Fields: map[string]string{"status": "200"}, Timestamps: []string{"2026-07-17T08:00:00Z"}, Values: []int64{90}},
		{Fields: map[string]string{"status": "404"}, Timestamps: []string{"2026-07-17T08:00:00Z"}, Values: []int64{7}},
		{Fields: map[string]string{"status": "503"}, Timestamps: []string{"2026-07-17T08:00:00Z"}, Values: []int64{3}},
	})

	if len(trend) != 1 {
		t.Fatalf("trend length = %d, want 1", len(trend))
	}
	if trend[0].RequestTotal != 100 || trend[0].HTTP4xx != 7 || trend[0].HTTP5xx != 3 {
		t.Fatalf("unexpected trend point: %#v", trend[0])
	}
}

func TestParseLogSLARecentErrorsReturnsWhitelistedFields(t *testing.T) {
	items := parseLogSLARecentErrors([]map[string]interface{}{{
		"@timestamp":           "2026-07-17T15:51:51+08:00",
		"request_method":       "GET",
		"request":              "/api/orders",
		"status":               503.0,
		"responsetime":         "0.051",
		"lyj_openclaw_api_key": "must-not-be-returned",
		"x_zai_trace_id":       "trace-1",
	}})

	if len(items) != 1 || items[0].RequestPath != "/api/orders" || items[0].LatencyMs != 51 || items[0].TraceID != "trace-1" {
		t.Fatalf("unexpected recent error: %#v", items)
	}
}
