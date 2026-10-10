package service

import (
	"strings"
	"testing"

	logtailmodel "dodevops-api/api/logtail/model"
)

func TestMergeLogQueryFieldsIncludesBuiltinParseFields(t *testing.T) {
	fields := mergeLogQueryFields(nil)

	for _, want := range []string{
		"query_time",
		"rows_examined",
		"sql_text",
		"sql_operation",
		"db_user",
		"request_method",
		"remote_addr",
	} {
		if !containsString(fields, want) {
			t.Fatalf("mergeLogQueryFields() missing %q in %v", want, fields)
		}
	}
}

func TestMergeLogQueryFieldsOmitsVerboseRawFieldsByDefault(t *testing.T) {
	fields := mergeLogQueryFields(nil)
	for _, notWant := range []string{"_raw", "_msg"} {
		if containsString(fields, notWant) {
			t.Fatalf("mergeLogQueryFields() contains %q by default in %v", notWant, fields)
		}
	}

	fields = mergeLogQueryFields([]string{"_raw", "_msg"})
	for _, want := range []string{"_raw", "_msg"} {
		if !containsString(fields, want) {
			t.Fatalf("mergeLogQueryFields() missing requested %q in %v", want, fields)
		}
	}
}

func TestMergeLogQueryFieldsDeduplicatesRequestedParseField(t *testing.T) {
	fields := mergeLogQueryFields([]string{"query_time"})
	count := 0
	for _, field := range fields {
		if field == "query_time" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("query_time count = %d, want 1 in %v", count, fields)
	}
}

func TestPrepareLogQueryFieldsSkipsDefaultsForAggregateQuery(t *testing.T) {
	query := `log_type:nginx | stats by (request_path) count() as requests | sort by (requests desc)`

	gotQuery, fields, deferFields, err := prepareLogQueryFields(query, nil)
	if err != nil {
		t.Fatalf("prepareLogQueryFields() error = %v", err)
	}
	if gotQuery != query {
		t.Fatalf("prepareLogQueryFields() query = %q, want %q", gotQuery, query)
	}
	if len(fields) != 0 {
		t.Fatalf("prepareLogQueryFields() fields = %v, want empty", fields)
	}
	if !deferFields {
		t.Fatal("prepareLogQueryFields() deferFields = false, want true")
	}
}

func TestAppendFieldsPipeQuotesCompoundFieldNames(t *testing.T) {
	query, err := appendFieldsPipe("status:>=400", []string{"_time", "@timestamp", "kubernetes.pod-name"})
	if err != nil {
		t.Fatalf("appendFieldsPipe() error = %v", err)
	}
	want := `status:>=400 | fields _time, "@timestamp", "kubernetes.pod-name"`
	if query != want {
		t.Fatalf("appendFieldsPipe() = %q, want %q", query, want)
	}
}

func TestPrepareLogQueryFieldsSkipsDefaultsForDynamicJSON(t *testing.T) {
	query := `log_type:json service:order`

	gotQuery, fields, deferFields, err := prepareLogQueryFields(query, nil)
	if err != nil {
		t.Fatalf("prepareLogQueryFields() error = %v", err)
	}
	if gotQuery != query {
		t.Fatalf("prepareLogQueryFields() query = %q, want %q", gotQuery, query)
	}
	if len(fields) != 0 {
		t.Fatalf("prepareLogQueryFields() fields = %v, want empty", fields)
	}
	if !deferFields {
		t.Fatal("prepareLogQueryFields() deferFields = false, want true")
	}
}

func TestNormalizeVictoriaLogsQueryWrapsPlainKeywordForSubstringSearch(t *testing.T) {
	if got := normalizeVictoriaLogsQuery("zhangfan", ""); got != "*zhangfan*" {
		t.Fatalf("normalizeVictoriaLogsQuery() = %q, want %q", got, "*zhangfan*")
	}
	if got := normalizeVictoriaLogsQuery("", "zhangfan123"); got != "*zhangfan123*" {
		t.Fatalf("normalizeVictoriaLogsQuery() keyword = %q, want %q", got, "*zhangfan123*")
	}
}

func TestNormalizeVictoriaLogsQuerySearchesUUIDAcrossIdentifierFields(t *testing.T) {
	id := "e3bb3721-68f3-46f6-8fbf-0c6cd434cd72"
	want := `(*e3bb3721-68f3-46f6-8fbf-0c6cd434cd72* OR trace_id:="e3bb3721-68f3-46f6-8fbf-0c6cd434cd72" OR x_zai_trace_id:="e3bb3721-68f3-46f6-8fbf-0c6cd434cd72" OR span_id:="e3bb3721-68f3-46f6-8fbf-0c6cd434cd72" OR request_id:="e3bb3721-68f3-46f6-8fbf-0c6cd434cd72" OR session_id:="e3bb3721-68f3-46f6-8fbf-0c6cd434cd72")`

	if got := normalizeVictoriaLogsQuery(id, ""); got != want {
		t.Fatalf("normalizeVictoriaLogsQuery() = %q, want %q", got, want)
	}
	if got := normalizeVictoriaLogsQuery("", id); got != want {
		t.Fatalf("normalizeVictoriaLogsQuery() keyword = %q, want %q", got, want)
	}
}

func TestNormalizeVictoriaLogsQueryKeepsLogsQLSyntax(t *testing.T) {
	cases := []string{
		"*",
		"*zhangfan*",
		`log_type:db_slow_log query_time:>3`,
		`log_type:nginx | stats by (status) count() as total`,
	}

	for _, query := range cases {
		if got := normalizeVictoriaLogsQuery(query, ""); got != query {
			t.Fatalf("normalizeVictoriaLogsQuery(%q) = %q, want unchanged", query, got)
		}
	}
}

func TestNormalizeLogTimeForDisplay(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "UTC",
			value: "2026-08-07T01:31:42.295162734Z",
			want:  "2026-08-07T09:31:42.295162734+08:00",
		},
		{
			name:  "Shanghai offset",
			value: "2026-08-07T09:31:42.295162734+08:00",
			want:  "2026-08-07T09:31:42.295162734+08:00",
		},
		{
			name:  "local date without zone",
			value: "2026-08-07 09:31:42.295162734",
			want:  "2026-08-07T09:31:42.295162734+08:00",
		},
		{
			name:  "time only",
			value: "09:31:42.295",
			want:  "09:31:42.295",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeLogTimeForDisplay(tt.value); got != tt.want {
				t.Fatalf("normalizeLogTimeForDisplay(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeLogRowsForDisplayOnlyChangesTimeFields(t *testing.T) {
	rows := []map[string]interface{}{{
		"_time":     "2026-08-07T01:31:42Z",
		"timestamp": "2026-08-07 09:31:42",
		"message":   "upstream returned 2026-08-07T01:31:42Z",
		"_raw":      "2026-08-07T01:31:42Z original log",
	}}

	normalizeLogRowsForDisplay(rows)
	if rows[0]["_time"] != "2026-08-07T09:31:42+08:00" {
		t.Fatalf("_time = %#v", rows[0]["_time"])
	}
	if rows[0]["timestamp"] != "2026-08-07T09:31:42+08:00" {
		t.Fatalf("timestamp = %#v", rows[0]["timestamp"])
	}
	if rows[0]["message"] != "upstream returned 2026-08-07T01:31:42Z" || rows[0]["_raw"] != "2026-08-07T01:31:42Z original log" {
		t.Fatalf("raw log content was changed: %#v", rows[0])
	}
}

func TestNormalizeVictoriaLogsTimeRangeTreatsNaiveDateAsShanghai(t *testing.T) {
	start, end := normalizeVictoriaLogsTimeRange("2026-08-07 09:30:00", "2026-08-07T10:30:00+08:00")
	if start != "2026-08-07T01:30:00Z" {
		t.Fatalf("start = %q", start)
	}
	if end != "2026-08-07T02:30:00Z" {
		t.Fatalf("end = %q", end)
	}
}

func TestBuildLogQueryTableColumnsForDBSlowLog(t *testing.T) {
	columns := buildLogQueryTableColumns("structured", "db_slow_log", []map[string]interface{}{{
		"_time":         "2026-07-07T03:43:21Z",
		"query_time":    3.5,
		"rows_examined": 1043968,
		"db_user":       "cjjshome_rw",
		"database":      "jjsfang",
		"sql_operation": "SELECT",
		"sql_text":      "SELECT 1",
	}})

	for _, want := range []string{"_time", "query_time", "rows_examined", "db_user", "database", "sql_operation", "sql_text"} {
		if !tableColumnsContain(columns, want) {
			t.Fatalf("db_slow_log table columns missing %q in %#v", want, columns)
		}
	}
}

func TestBuildLogAnalysisPresetsForNginx(t *testing.T) {
	presets := buildLogAnalysisPresets("nginx")
	for _, want := range []string{"nginx_status_ratio", "nginx_error_class_ratio", "nginx_top_request_path", "nginx_top_remote_addr", "nginx_error_request_path"} {
		found := false
		for _, preset := range presets {
			if preset.Key == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("nginx analysis presets missing %q in %#v", want, presets)
		}
	}
}

func TestBuildLogAnalysisPresetsForDBSlowLogIncludesQueryTimeRatio(t *testing.T) {
	presets := buildLogAnalysisPresets("db_slow_log")
	found := false
	for _, preset := range presets {
		if preset.Key == "db_slow_log_query_time_ratio" {
			found = true
			if !strings.Contains(preset.Query, "gt_5s") || !strings.Contains(preset.Query, "gt_10s") || !strings.Contains(preset.Query, "gt_20s") {
				t.Fatalf("query time ratio preset query missing buckets: %s", preset.Query)
			}
			if strings.Contains(preset.Query, "count(query_time:") || !strings.Contains(preset.Query, "count() if (query_time:>5)") {
				t.Fatalf("query time ratio preset uses invalid conditional stats syntax: %s", preset.Query)
			}
		}
	}
	if !found {
		t.Fatalf("db_slow_log analysis presets missing query time ratio: %#v", presets)
	}
}

func TestBuildLogQuickFilters(t *testing.T) {
	slowFilters := buildLogQuickFilters("db_slow_log")
	for _, want := range []string{"db_slow_log_gt_5s", "db_slow_log_gt_10s", "db_slow_log_gt_20s"} {
		if !quickFiltersContain(slowFilters, want) {
			t.Fatalf("slow log quick filters missing %q in %#v", want, slowFilters)
		}
	}

	nginxFilters := buildLogQuickFilters("nginx")
	for _, want := range []string{"nginx_404", "nginx_405", "nginx_503"} {
		if !quickFiltersContain(nginxFilters, want) {
			t.Fatalf("nginx quick filters missing %q in %#v", want, nginxFilters)
		}
	}
}

func TestLogIndexFieldDefaultsUsesCuratedFields(t *testing.T) {
	fields := logIndexFieldDefaults(`log_type:db_slow_log query_time:>3`)

	for _, want := range []string{"collect_task_name", "collect_task_code", "log_source", "log_type", "host_name", "log_file_name", "query_time", "rows_examined", "db_user"} {
		if !containsString(fields, want) {
			t.Fatalf("logIndexFieldDefaults() missing %q in %v", want, fields)
		}
	}
	for _, notWant := range []string{"_time", "_msg", "message", "project_code", "store_code", "agent_id", "host_id"} {
		if containsString(fields, notWant) {
			t.Fatalf("%s should stay out of index fields: %v", notWant, fields)
		}
	}
	if containsString(fields, "sql_text") {
		t.Fatalf("sql_text should stay out of index fields: %v", fields)
	}
}

func TestLogIndexFieldDefaultsIncludesKubernetesFacets(t *testing.T) {
	fields := logIndexFieldDefaults("*")
	for _, want := range []string{
		"k8s_log_source",
		"k8s_cluster_name",
		"k8s_namespace",
		"k8s_workload_type",
		"k8s_workload_name",
		"k8s_pod_name",
		"k8s_container_name",
	} {
		if !containsString(fields, want) {
			t.Fatalf("logIndexFieldDefaults() missing k8s facet %q in %v", want, fields)
		}
	}
}

func TestDetectLogTypeFromFieldItemsFallsBackToHits(t *testing.T) {
	got := detectLogTypeFromFieldItems("*", []logtailmodel.LogQueryFieldItem{
		{Value: "request_path", Hits: 10},
		{Value: "status", Hits: 10},
	})
	if got != "nginx" {
		t.Fatalf("detectLogTypeFromFieldItems() = %q, want nginx", got)
	}
}

func TestMergeLogFieldItemsOnlyHits(t *testing.T) {
	got := mergeLogFieldItems([]logtailmodel.LogQueryFieldItem{
		{Value: "_time", Hits: 10},
		{Value: "service", Hits: 0},
		{Value: "request_path", Hits: 10},
		{Value: "project_code", Hits: 10},
	}, []string{"_time", "service", "request_path"}, "", true, true)

	if containsFieldItems(got, "service") {
		t.Fatalf("zero-hit field should not be returned: %#v", got)
	}
	if containsFieldItems(got, "project_code") {
		t.Fatalf("raw field outside defaults should not be returned: %#v", got)
	}
	if !containsFieldItems(got, "request_path") {
		t.Fatalf("request_path should be returned: %#v", got)
	}
}

func TestBuildOtherLogFieldItemsKeepsDynamicFieldsSeparate(t *testing.T) {
	raw := []logtailmodel.LogQueryFieldItem{
		{Value: "_time", Hits: 10},
		{Value: "query_time", Hits: 8},
		{Value: "project_code", Hits: 10},
		{Value: "business_order_id", Hits: 3},
	}
	indexed := []logtailmodel.LogQueryFieldItem{
		{Value: "_time", Hits: 10},
		{Value: "query_time", Hits: 8},
	}

	other := buildOtherLogFieldItems(raw, indexed, "")
	if len(other) != 1 || other[0].Value != "business_order_id" {
		t.Fatalf("buildOtherLogFieldItems() = %#v, want business_order_id only", other)
	}
}

func TestBuildNormalLogFieldItemsFiltersSystemNoise(t *testing.T) {
	raw := []logtailmodel.LogQueryFieldItem{
		{Value: "_time", Hits: 10},
		{Value: "_msg", Hits: 10},
		{Value: "message", Hits: 10},
		{Value: "_raw", Hits: 10},
		{Value: "offset", Hits: 10},
		{Value: "partition", Hits: 10},
		{Value: "topic", Hits: 10},
		{Value: "business_order_id", Hits: 8},
		{Value: "post_body", Hits: 5},
		{Value: "request_body_json.query", Hits: 5},
		{Value: "request_body_json.json_query.file.url", Hits: 4},
		{Value: "post_body_json.pageNum", Hits: 3},
		{Value: "status", Hits: 10},
	}
	indexed := []logtailmodel.LogQueryFieldItem{
		{Value: "status", Hits: 10},
	}

	got := buildNormalLogFieldItems(raw, indexed, "")

	for _, notWant := range []string{"_time", "_msg", "message", "_raw", "offset", "partition", "topic", "status", "request_body_json.query", "request_body_json.json_query.file.url", "post_body_json.pageNum"} {
		if containsFieldItems(got, notWant) {
			t.Fatalf("buildNormalLogFieldItems() should filter %q: %#v", notWant, got)
		}
	}
	for _, want := range []string{"business_order_id", "post_body"} {
		if !containsFieldItems(got, want) {
			t.Fatalf("buildNormalLogFieldItems() missing %q: %#v", want, got)
		}
	}
}

func TestLogIndexFieldDefaultsForNginxIncludesUsefulFacets(t *testing.T) {
	fields := logIndexFieldDefaultsForType("nginx")
	for _, want := range []string{
		"app",
		"host_name",
		"file_path",
		"collect_task_name",
		"collect_task_code",
		"remote_addr",
		"clientip",
		"status",
		"request_path",
		"domain",
		"request_id",
		"x_zai_trace_id",
		"error",
		"timeout",
	} {
		if !containsString(fields, want) {
			t.Fatalf("logIndexFieldDefaultsForType(nginx) missing %q in %v", want, fields)
		}
	}
	for _, notWant := range []string{"_time", "_msg", "message", "_raw", "offset", "partition", "topic"} {
		if containsString(fields, notWant) {
			t.Fatalf("logIndexFieldDefaultsForType(nginx) should not include %q in %v", notWant, fields)
		}
	}
}

func TestEnrichLogFieldItemsAddsRatioAndPercent(t *testing.T) {
	got := enrichLogFieldItems([]logtailmodel.LogQueryFieldItem{
		{Value: "status", Hits: 25},
		{Value: "request_path", Hits: 100},
	}, 100)

	if got[0].Total != 100 || got[0].Ratio != 0.25 || got[0].Percent != 25 {
		t.Fatalf("first item ratio = %#v, want total=100 ratio=0.25 percent=25", got[0])
	}
	if got[1].Total != 100 || got[1].Ratio != 1 || got[1].Percent != 100 {
		t.Fatalf("single full-category ratio = %#v, want 100%%", got[1])
	}
}

func TestMaxLogFieldItemHits(t *testing.T) {
	got := maxLogFieldItemHits([]logtailmodel.LogQueryFieldItem{
		{Value: "_time", Hits: 7505},
		{Value: "request_query", Hits: 7383},
	})
	if got != 7505 {
		t.Fatalf("maxLogFieldItemHits() = %d, want 7505", got)
	}
}

func TestBuildLogFieldValueItemsAndOther(t *testing.T) {
	items := buildLogFieldValueItems([]map[string]interface{}{
		{"status": "200", "hits": float64(80)},
		{"status": "404", "hits": float64(15)},
		{"status": "503", "hits": float64(5)},
	}, "status", "", 5)
	items = appendOtherLogFieldItem(items, 120)
	items = enrichLogFieldItems(items, 120)

	if len(items) != 4 {
		t.Fatalf("items len = %d, want 4: %#v", len(items), items)
	}
	if items[0].Value != "200" || items[0].Hits != 80 || items[0].Percent != 66.67 {
		t.Fatalf("first item = %#v, want 200 hits=80 percent=66.67", items[0])
	}
	other := items[3]
	if other.Value != "其他" || other.Hits != 20 || !other.IsOther || other.Percent != 16.67 {
		t.Fatalf("other item = %#v, want hits=20 percent=16.67", other)
	}
}

func TestAppendOtherLogFieldItemSkipsFullCategory(t *testing.T) {
	items := appendOtherLogFieldItem([]logtailmodel.LogQueryFieldItem{
		{Value: "200", Hits: 100},
	}, 100)
	if len(items) != 1 {
		t.Fatalf("items len = %d, want no other item: %#v", len(items), items)
	}
}

func TestBuildLogDownloadContentCSV(t *testing.T) {
	data, contentType, ext, err := buildLogDownloadContent([]map[string]interface{}{
		{"_time": "2026-07-07T10:20:00Z", "message": "hello,error"},
	}, []string{"_time", "message"}, "csv")
	if err != nil {
		t.Fatalf("buildLogDownloadContent() error = %v", err)
	}
	text := string(data)
	if contentType != "text/csv; charset=utf-8" || ext != "csv" {
		t.Fatalf("contentType/ext = %s/%s, want csv", contentType, ext)
	}
	if !strings.Contains(text, "_time,message") || !strings.Contains(text, `"hello,error"`) {
		t.Fatalf("csv content = %q, missing header or escaped value", text)
	}
}

func TestBuildLogDownloadContentJSONL(t *testing.T) {
	data, contentType, ext, err := buildLogDownloadContent([]map[string]interface{}{
		{"message": "hello"},
	}, nil, "jsonl")
	if err != nil {
		t.Fatalf("buildLogDownloadContent() error = %v", err)
	}
	if contentType != "application/x-ndjson; charset=utf-8" || ext != "jsonl" {
		t.Fatalf("contentType/ext = %s/%s, want jsonl", contentType, ext)
	}
	if !strings.Contains(string(data), `"message":"hello"`) {
		t.Fatalf("jsonl content = %q, missing message", string(data))
	}
}

func TestBuildLogDownloadContentTXTUsesRawText(t *testing.T) {
	data, contentType, ext, err := buildLogDownloadContent([]map[string]interface{}{
		{"_raw": "raw line", "message": "message line"},
	}, nil, "txt")
	if err != nil {
		t.Fatalf("buildLogDownloadContent() error = %v", err)
	}
	if contentType != "text/plain; charset=utf-8" || ext != "log" {
		t.Fatalf("contentType/ext = %s/%s, want txt", contentType, ext)
	}
	if string(data) != "raw line\n" {
		t.Fatalf("txt content = %q, want raw line", string(data))
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func quickFiltersContain(items []logtailmodel.LogQueryQuickFilter, want string) bool {
	for _, item := range items {
		if item.Key == want {
			return true
		}
	}
	return false
}

func tableColumnsContain(columns []logtailmodel.LogQueryTableColumn, want string) bool {
	for _, column := range columns {
		if column.Field == want {
			return true
		}
	}
	return false
}

func containsFieldItems(items []logtailmodel.LogQueryFieldItem, want string) bool {
	for _, item := range items {
		if item.Value == want {
			return true
		}
	}
	return false
}
