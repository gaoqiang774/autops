package service

import (
	"encoding/json"
	"strings"
	"testing"

	logtailmodel "dodevops-api/api/logtail/model"
)

func TestParseLogParseTemplateAIAnswerExtractsJSON(t *testing.T) {
	raw := "```json\n{" +
		`"name":"zeroai nginx json",` +
		`"parseMode":"nginx",` +
		`"vrlScript":"parsed, err = parse_json(._raw)\nif err == null && is_object(parsed) {\n  ., merge_err = merge(., parsed)\n}",` +
		`"fields":["status","request_path"],` +
		`"fieldMappings":{"clientip":"remote_addr"}` +
		"}\n```"

	got, err := parseLogParseTemplateAIAnswer(raw)
	if err != nil {
		t.Fatalf("parseLogParseTemplateAIAnswer() error = %v", err)
	}
	if got.ParseMode != "nginx" {
		t.Fatalf("ParseMode = %q, want nginx", got.ParseMode)
	}
	if !strings.Contains(got.VRLScript, "merge_err = merge(., parsed)") {
		t.Fatalf("VRLScript missing json merge handling: %s", got.VRLScript)
	}
	if got.FieldMappings["clientip"] != "remote_addr" {
		t.Fatalf("FieldMappings[clientip] = %q, want remote_addr", got.FieldMappings["clientip"])
	}
}

func TestBuildLogParseTemplateFromAI(t *testing.T) {
	template, err := buildLogParseTemplateFromAI(logtailmodel.LogParseTemplateAIGenerateRequest{
		ProjectID: 1,
		LogType:   "nginx",
		Name:      "ZeroAI Nginx",
	}, logParseTemplateAIAnswer{
		ParseMode: "nginx",
		VRLScript: "parsed, err = parse_json(._raw)",
		FieldMappings: map[string]string{
			"clientip": "remote_addr",
		},
	})
	if err != nil {
		t.Fatalf("buildLogParseTemplateFromAI() error = %v", err)
	}
	if template.ProjectID != 1 {
		t.Fatalf("ProjectID = %d, want 1", template.ProjectID)
	}
	if template.ParseMode != "nginx" {
		t.Fatalf("ParseMode = %q, want nginx", template.ParseMode)
	}
	if template.Code == "zeroai_nginx" {
		t.Fatalf("Code = %q, should include unique suffix", template.Code)
	}
	if !strings.Contains(template.FieldsMappingJSON, "remote_addr") {
		t.Fatalf("FieldsMappingJSON = %q, want remote_addr", template.FieldsMappingJSON)
	}
}

func TestGenerateUniqueAIParseTemplateCodeAddsSuffix(t *testing.T) {
	got := generateUniqueAIParseTemplateCode("nginx_nginx_ai_parse", "fallback")
	if !strings.HasPrefix(got, "nginx_nginx_ai_parse_") {
		t.Fatalf("generateUniqueAIParseTemplateCode() = %q, want prefix nginx_nginx_ai_parse_", got)
	}
	if got == "nginx_nginx_ai_parse" {
		t.Fatalf("generateUniqueAIParseTemplateCode() should not return unchanged code")
	}
}

func TestValidateGeneratedVRLScriptRejectsFullVectorConfig(t *testing.T) {
	if err := validateGeneratedVRLScript("sources:\n  app:\n    type: file"); err == nil {
		t.Fatal("validateGeneratedVRLScript() should reject full Vector config")
	}
}

func TestParseLogParseTemplateAIAnswerRejectsUnsupportedMode(t *testing.T) {
	_, err := parseLogParseTemplateAIAnswer(`{"parseMode":"yaml","vrlScript":".message = .message"}`)
	if err == nil {
		t.Fatal("parseLogParseTemplateAIAnswer() should reject unsupported parseMode")
	}
}

func TestParseLogParseTemplateAIAnswerAcceptsNginxJSONMode(t *testing.T) {
	got, err := parseLogParseTemplateAIAnswer(`{"parseMode":"nginx+json","vrlScript":"parsed, err = parse_json(._raw)"}`)
	if err != nil {
		t.Fatalf("parseLogParseTemplateAIAnswer() error = %v", err)
	}
	if got.ParseMode != "nginx_json" {
		t.Fatalf("ParseMode = %q, want nginx_json", got.ParseMode)
	}
}

func TestParseLogParseTemplateAIAnswerAcceptsNginxJSONLooseMode(t *testing.T) {
	got, err := parseLogParseTemplateAIAnswer(`{"parseMode":"nginx+json-loose","vrlScript":".parse_status = \"raw_fallback\""}`)
	if err != nil {
		t.Fatalf("parseLogParseTemplateAIAnswer() error = %v", err)
	}
	if got.ParseMode != "nginx_json_loose" {
		t.Fatalf("ParseMode = %q, want nginx_json_loose", got.ParseMode)
	}
}

func TestParseLogParseTemplateAIAnswerRepairsNullTimestampValue(t *testing.T) {
	got, err := parseLogParseTemplateAIAnswer(`{"parseMode":"nginx+json","vrlScript":"timestamp_value = null\nif exists(.\"@timestamp\") {\n  timestamp_value = to_string(.\"@timestamp\") ?? \"\"\n}\nts, ts_err = parse_timestamp(timestamp_value, \"%+\")"}`)
	if err != nil {
		t.Fatalf("parseLogParseTemplateAIAnswer() error = %v", err)
	}
	if strings.Contains(got.VRLScript, "timestamp_value = null") {
		t.Fatalf("VRLScript should repair null timestamp value: %s", got.VRLScript)
	}
	if !strings.Contains(got.VRLScript, `timestamp_value = ""`) {
		t.Fatalf("VRLScript missing repaired timestamp fallback: %s", got.VRLScript)
	}
	if len(got.Warnings) == 0 {
		t.Fatalf("Warnings should include repair note")
	}
}

func TestBuildLogParseTemplateFromAIUsesBuiltinNginxJSONLooseScript(t *testing.T) {
	template, err := buildLogParseTemplateFromAI(logtailmodel.LogParseTemplateAIGenerateRequest{
		ProjectID: 1,
		LogType:   "nginx+json-loose",
		Name:      "ZeroAI Nginx Loose",
	}, logParseTemplateAIAnswer{
		ParseMode: "nginx_json_loose",
		VRLScript: "timestamp_value = null",
	})
	if err != nil {
		t.Fatalf("buildLogParseTemplateFromAI() error = %v", err)
	}
	if template.ParseMode != "nginx_json_loose" {
		t.Fatalf("ParseMode = %q, want nginx_json_loose", template.ParseMode)
	}
	for _, want := range []string{
		`.parse_status = "raw_fallback"`,
		`parse_json(._raw)`,
		`loose_post_body`,
		`loose_request_body`,
		`timestamp_text = to_string(."@timestamp") ?? ""`,
	} {
		if !strings.Contains(template.VRLScript, want) {
			t.Fatalf("builtin VRLScript missing %q:\n%s", want, template.VRLScript)
		}
	}
	if strings.Contains(template.VRLScript, "timestamp_value = null") {
		t.Fatalf("builtin VRLScript should not keep AI script:\n%s", template.VRLScript)
	}
	for _, notWant := range []string{".post_body_json", ".request_body_json"} {
		if strings.Contains(template.VRLScript, notWant) {
			t.Fatalf("builtin VRLScript should not create dynamic body object %q:\n%s", notWant, template.VRLScript)
		}
	}
}

func TestOptimizeLogSampleForAICompactsJSONAndRedactsSecrets(t *testing.T) {
	sample := `{"status":200,"request":"/api/v1/chat?workspace=zero","request_body":"` + strings.Repeat("x", 2000) + `","x_zai_session_token":"very-secret-token","nested":{"items":[1,2,3,4,5,6,7,8,9,10]}}`

	got := optimizeLogSampleForAI(sample)
	if strings.Contains(got, "very-secret-token") {
		t.Fatalf("optimized sample leaked sensitive value: %s", got)
	}
	for _, want := range []string{`"status":200`, `"request_body"`, `"x_zai_session_token":"<redacted>"`, "...[truncated]..."} {
		if !strings.Contains(got, want) {
			t.Fatalf("optimized sample missing %q: %s", want, got)
		}
	}
	if len([]rune(got)) >= len([]rune(sample)) {
		t.Fatalf("optimized sample was not reduced: before=%d after=%d", len([]rune(sample)), len([]rune(got)))
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("optimized JSON is invalid: %v\n%s", err, got)
	}
	items := parsed["nested"].(map[string]interface{})["items"].([]interface{})
	if len(items) != maxLogParseTemplateArrayItems {
		t.Fatalf("optimized array length = %d, want %d", len(items), maxLogParseTemplateArrayItems)
	}
}

func TestOptimizeLogSampleForAILimitsLinesAndTotalSize(t *testing.T) {
	lines := make([]string, 0, 20)
	for index := 0; index < 20; index++ {
		lines = append(lines, strings.Repeat("log-data-", 1000))
	}

	got := optimizeLogSampleForAI(strings.Join(lines, "\n"))
	if len([]rune(got)) > maxLogParseTemplateSampleRunes {
		t.Fatalf("optimized sample length = %d, limit = %d", len([]rune(got)), maxLogParseTemplateSampleRunes)
	}
	if strings.Count(got, "\n") >= maxLogParseTemplateSampleLines {
		t.Fatalf("optimized sample kept too many lines: %d", strings.Count(got, "\n")+1)
	}
}

func TestBuildLogParseTemplateAIPromptUsesOptimizedSample(t *testing.T) {
	prompt := buildLogParseTemplateAIPrompt(logtailmodel.LogParseTemplateAIGenerateRequest{
		LogType:   "nginx+json-loose",
		SampleLog: `{"status":200,"x_zai_session_token":"do-not-send"}`,
	})
	if strings.Contains(prompt, "do-not-send") {
		t.Fatalf("AI prompt leaked sensitive sample value: %s", prompt)
	}
	if !strings.Contains(prompt, `"x_zai_session_token":"<redacted>"`) {
		t.Fatalf("AI prompt missing redacted field: %s", prompt)
	}
}

func TestRepairGeneratedVRLScriptFixesLegacyTimestampAndSlice(t *testing.T) {
	script := `if exists(.timestamp) {
  ts_value = to_string(.timestamp) ?? ""
  parsed_ts, ts_err = parse_timestamp(ts_value, "%+")
}
request_parts = split(.request, "?")
if length(request_parts) > 1 {
  .request_query = join(slice(request_parts, 1), "?")
}`

	got, warnings := repairGeneratedVRLScript(script, nil)
	for _, invalid := range []string{`to_string(.timestamp) ?? ""`, `join(slice(request_parts, 1), "?")`, `.request_query = join(request_query_parts, "?")`} {
		if strings.Contains(got, invalid) {
			t.Fatalf("repaired script still contains %q:\n%s", invalid, got)
		}
	}
	for _, want := range []string{`ts_value = to_string(.timestamp)`, `request_query_parts_err = slice(request_parts, 1)`, `request_query_join_err = join(request_query_parts, "?")`, `.request_query = request_query_value`} {
		if !strings.Contains(got, want) {
			t.Fatalf("repaired script missing %q:\n%s", want, got)
		}
	}
	if len(warnings) != 2 {
		t.Fatalf("repair warning count = %d, want 2: %#v", len(warnings), warnings)
	}
}

func TestWriteVectorTemplateScriptRepairsLegacyTemplate(t *testing.T) {
	template := &logtailmodel.LogParseTemplate{
		VRLScript: `.request_query = join(slice(request_parts, 1), "?")`,
	}
	var builder strings.Builder
	if !writeVectorTemplateScript(&builder, template) {
		t.Fatal("writeVectorTemplateScript() = false, want true")
	}
	if strings.Contains(builder.String(), "join(slice(") || strings.Contains(builder.String(), `.request_query = join(`) {
		t.Fatalf("runtime Vector script was not repaired:\n%s", builder.String())
	}
}

func TestRepairGeneratedVRLScriptFixesPreviouslyRepairedJoin(t *testing.T) {
	script := `request_query_parts, request_query_parts_err = slice(request_parts, 1)
if request_query_parts_err == null {
  .request_query = join(request_query_parts, "?")
}`

	got, _ := repairGeneratedVRLScript(script, nil)
	if strings.Contains(got, `.request_query = join(`) {
		t.Fatalf("repaired script still contains fallible join assignment:\n%s", got)
	}
	for _, want := range []string{`request_query_join_err = join(request_query_parts, "?")`, `.request_query = request_query_value`} {
		if !strings.Contains(got, want) {
			t.Fatalf("repaired script missing %q:\n%s", want, got)
		}
	}
}
