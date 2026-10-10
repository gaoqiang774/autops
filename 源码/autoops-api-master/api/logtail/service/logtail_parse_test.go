package service

import (
	"regexp"
	"strings"
	"testing"

	logtailmodel "dodevops-api/api/logtail/model"
)

func TestNormalizeCollectTaskForCreateAcceptsTxtLogType(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:    "Plain Text Task",
		LogType: "txt",
	}

	normalizeCollectTaskForCreate(task)

	if task.ParseModeSnapshot != "raw" {
		t.Fatalf("normalizeCollectTaskForCreate() parseModeSnapshot = %q, want %q", task.ParseModeSnapshot, "raw")
	}
	if task.LogType != "txt" {
		t.Fatalf("normalizeCollectTaskForCreate() logType = %q, want %q", task.LogType, "txt")
	}
}

func TestMergeCollectTaskMultilineSettingsDefaultsTextLogs(t *testing.T) {
	task := &logtailmodel.LogCollectTask{ParseModeSnapshot: "raw"}

	mergeCollectTaskMultilineSettings(task, nil)

	if task.MultilineEnabledSnapshot == nil || !*task.MultilineEnabledSnapshot {
		t.Fatal("mergeCollectTaskMultilineSettings() should enable multiline for text logs")
	}
	if task.MultilinePatternSnapshot != defaultTextMultilinePattern {
		t.Fatalf("mergeCollectTaskMultilineSettings() pattern = %q, want %q", task.MultilinePatternSnapshot, defaultTextMultilinePattern)
	}
	if err := validateCollectTaskMultilinePattern(task); err != nil {
		t.Fatalf("validateCollectTaskMultilinePattern() unexpected error = %v", err)
	}
}

func TestDefaultTextMultilinePatternMatchesCommonTimestamps(t *testing.T) {
	pattern := regexp.MustCompile(defaultTextMultilinePattern)
	timestamps := []string{
		"2026-08-12 14:00:13.285 INFO request failed",
		"2026/08/12 14:00:13 INFO request failed",
		"2026.08.12 14:00:13,285 INFO request failed",
		"2026-08-12T14:00:13.285Z stdout F request failed",
		"2026-08-12T14:00:13+08:00 INFO request failed",
		"[2026/08/12 18:24:04] request failed",
		"08-12 18:24:04.123 1234 5678 E AndroidRuntime: FATAL EXCEPTION",
		"Aug 12 18:24:04 hostname service[123]: request failed",
		"[12/Aug/2026:18:24:04 +0800] request failed",
		"[Wed Aug 12 18:24:04.123456 2026] request failed",
		"<34>1 2026-08-12T18:24:04.123Z host app 123 ID47 - request failed",
	}
	for _, timestamp := range timestamps {
		if !pattern.MatchString(timestamp) {
			t.Fatalf("defaultTextMultilinePattern should match %q", timestamp)
		}
	}
	for _, continuation := range []string{"java.lang.IllegalArgumentException: invalid method", "    at org.example.Main.run(Main.java:10)", "Caused by: timeout"} {
		if pattern.MatchString(continuation) {
			t.Fatalf("defaultTextMultilinePattern must not match continuation line %q", continuation)
		}
	}
}

func TestNormalizeDefaultTextMultilinePatternUpgradesLegacyDefault(t *testing.T) {
	for _, oldPattern := range []string{legacyDefaultTextMultilinePattern, previousDefaultTextMultilinePattern} {
		if got := normalizeDefaultTextMultilinePattern(oldPattern); got != defaultTextMultilinePattern {
			t.Fatalf("normalizeDefaultTextMultilinePattern() = %q, want %q", got, defaultTextMultilinePattern)
		}
	}
}

func TestMergeCollectTaskMultilineSettingsInheritsTemplate(t *testing.T) {
	task := &logtailmodel.LogCollectTask{ParseModeSnapshot: "regex"}
	template := &logtailmodel.LogParseTemplate{
		MultilineEnabled: true,
		MultilinePattern: `^ENTRY `,
	}

	mergeCollectTaskMultilineSettings(task, template)

	if task.MultilineEnabledSnapshot == nil || !*task.MultilineEnabledSnapshot {
		t.Fatal("mergeCollectTaskMultilineSettings() should inherit enabled state")
	}
	if task.MultilinePatternSnapshot != `^ENTRY ` {
		t.Fatalf("mergeCollectTaskMultilineSettings() pattern = %q, want %q", task.MultilinePatternSnapshot, `^ENTRY `)
	}
}

func TestWriteVectorHostFileSourceKeepsStackTraceAsOneEvent(t *testing.T) {
	var builder strings.Builder

	writeVectorHostFileSource(&builder, logtailmodel.LogCollectTask{
		ID:                1,
		Code:              "java_log",
		PathPattern:       "/mnt/java/nohup.out",
		ParseModeSnapshot: "raw",
	})

	content := builder.String()
	if !strings.Contains(content, "    multiline:\n") {
		t.Fatalf("writeVectorHostFileSource() content missing multiline config:\n%s", content)
	}
	if !strings.Contains(content, "      mode: halt_before\n") {
		t.Fatalf("writeVectorHostFileSource() content missing halt_before mode:\n%s", content)
	}
	if !strings.Contains(content, "      timeout_ms: 5000\n") {
		t.Fatalf("writeVectorHostFileSource() content missing multiline timeout:\n%s", content)
	}
}

func TestWriteVectorHostFileSourceRespectsExplicitMultilineDisable(t *testing.T) {
	var builder strings.Builder

	writeVectorHostFileSource(&builder, logtailmodel.LogCollectTask{
		ID:                       1,
		Code:                     "single_line",
		PathPattern:              "/var/log/single.log",
		ParseModeSnapshot:        "raw",
		MultilineEnabledSnapshot: boolPointer(false),
	})

	if content := builder.String(); strings.Contains(content, "    multiline:\n") {
		t.Fatalf("writeVectorHostFileSource() should not emit multiline config when disabled:\n%s", content)
	}
}

func TestNormalizeCollectTaskForCreateSetsDBSlowLogDefaults(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:    "MySQL Slow Log Task",
		LogType: "db_slow_log",
	}

	normalizeCollectTaskForCreate(task)
	mergeCollectTaskMultilineSettings(task, nil)

	if task.ParseModeSnapshot != "db_slow_log" {
		t.Fatalf("normalizeCollectTaskForCreate() parseModeSnapshot = %q, want %q", task.ParseModeSnapshot, "db_slow_log")
	}
	if task.MultilinePatternSnapshot != "^# Time: " {
		t.Fatalf("normalizeCollectTaskForCreate() multilinePatternSnapshot = %q, want %q", task.MultilinePatternSnapshot, "^# Time: ")
	}
}

func TestNormalizeCollectTaskForCreateAcceptsNginxJSONLogType(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:    "Nginx JSON Task",
		LogType: "nginx+json",
	}

	normalizeCollectTaskForCreate(task)

	if task.ParseModeSnapshot != "nginx_json" {
		t.Fatalf("normalizeCollectTaskForCreate() parseModeSnapshot = %q, want %q", task.ParseModeSnapshot, "nginx_json")
	}
	if task.LogType != "nginx_json" {
		t.Fatalf("normalizeCollectTaskForCreate() logType = %q, want %q", task.LogType, "nginx_json")
	}
}

func TestNormalizeCollectTaskForCreateAcceptsNginxJSONLooseLogType(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:    "Nginx JSON Loose Task",
		LogType: "nginx+json-loose",
	}

	normalizeCollectTaskForCreate(task)

	if task.ParseModeSnapshot != "nginx_json_loose" {
		t.Fatalf("normalizeCollectTaskForCreate() parseModeSnapshot = %q, want %q", task.ParseModeSnapshot, "nginx_json_loose")
	}
	if task.LogType != "nginx_json_loose" {
		t.Fatalf("normalizeCollectTaskForCreate() logType = %q, want %q", task.LogType, "nginx_json_loose")
	}
}

func TestNormalizeCollectTaskForUpdatePrefersExplicitLogType(t *testing.T) {
	existing := &logtailmodel.LogCollectTask{
		Name:              "nginx log",
		Code:              "nginx_log",
		ParseModeSnapshot: "nginx",
		ConfigVersion:     1,
	}
	incoming := &logtailmodel.LogCollectTask{
		LogType:           "nginx_json_loose",
		ParseModeSnapshot: "nginx",
	}

	normalizeCollectTaskForUpdate(existing, incoming)

	if incoming.ParseModeSnapshot != "nginx_json_loose" {
		t.Fatalf("normalizeCollectTaskForUpdate() parseModeSnapshot = %q, want %q", incoming.ParseModeSnapshot, "nginx_json_loose")
	}
	if incoming.LogType != "nginx_json_loose" {
		t.Fatalf("normalizeCollectTaskForUpdate() logType = %q, want %q", incoming.LogType, "nginx_json_loose")
	}
}

func TestNormalizeCollectTaskForUpdateAcceptsLogTypeAlias(t *testing.T) {
	existing := &logtailmodel.LogCollectTask{
		Name:              "slow log",
		Code:              "slow_log",
		ParseModeSnapshot: "raw",
		ConfigVersion:     1,
	}
	incoming := &logtailmodel.LogCollectTask{
		LogType: "db_slow_log",
	}

	normalizeCollectTaskForUpdate(existing, incoming)
	mergeCollectTaskMultilineSettings(incoming, nil)

	if incoming.ParseModeSnapshot != "db_slow_log" {
		t.Fatalf("normalizeCollectTaskForUpdate() parseModeSnapshot = %q, want %q", incoming.ParseModeSnapshot, "db_slow_log")
	}
	if incoming.LogType != "db_slow_log" {
		t.Fatalf("normalizeCollectTaskForUpdate() logType = %q, want %q", incoming.LogType, "db_slow_log")
	}
	if incoming.MultilinePatternSnapshot != "^# Time: " {
		t.Fatalf("normalizeCollectTaskForUpdate() multilinePatternSnapshot = %q, want %q", incoming.MultilinePatternSnapshot, "^# Time: ")
	}
}

func TestValidateCollectTaskRegexPattern(t *testing.T) {
	if err := validateCollectTaskRegexPattern(`^(?P<status>\d{3}) (?P<bytes>\d+)$`, `200 1234`); err != nil {
		t.Fatalf("validateCollectTaskRegexPattern() unexpected error = %v", err)
	}

	if err := validateCollectTaskRegexPattern(`^(\d{3}) (\d+)$`, `200 1234`); err == nil {
		t.Fatal("validateCollectTaskRegexPattern() should reject patterns without named capture groups")
	}

	if err := validateCollectTaskRegexPattern(`^(?P<bad-name>\d+)$`, `200`); err == nil {
		t.Fatal("validateCollectTaskRegexPattern() should reject invalid capture field names")
	}

	if err := validateCollectTaskRegexPattern(`^(?P<status>\d{3}) (?P<bytes>\d+)$`, `not matched`); err == nil {
		t.Fatal("validateCollectTaskRegexPattern() should reject non-matching samples")
	}
}

func TestWriteVectorParseScriptForNginxUsesBuiltinParser(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "nginx",
	}, nil)

	content := builder.String()
	if !strings.Contains(content, `parse_nginx_log(._raw, "combined")`) {
		t.Fatalf("writeVectorParseScript() content missing nginx parser:\n%s", content)
	}
	if !strings.Contains(content, `request_method`) {
		t.Fatalf("writeVectorParseScript() content missing request field extraction:\n%s", content)
	}
	if strings.Contains(content, "merge_err = merge") {
		t.Fatalf("writeVectorParseScript() nginx parser should not assign merge error:\n%s", content)
	}
}

func TestWriteVectorParseScriptHandlesFallibleSelfLogDowncase(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		PathPattern: "/var/log/messages",
	}, nil)

	content := builder.String()
	if !strings.Contains(content, `_msg_lower = downcase(._raw) ?? ""`) {
		t.Fatalf("writeVectorParseScript() self-log filter should handle fallible downcase:\n%s", content)
	}
}

func TestWriteVectorParseScriptForJSONHandlesFallibleMerge(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "json",
	}, nil)

	content := builder.String()
	if !strings.Contains(content, "., merge_err = merge(., parsed)") {
		t.Fatalf("writeVectorParseScript() json parser should handle fallible merge assignment:\n%s", content)
	}
}

func TestWriteVectorParseScriptForNginxJSON(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "nginx+json",
	}, nil)

	content := builder.String()
	for _, want := range []string{
		"parse_json(._raw)",
		"merge_err = merge(., parsed)",
		".remote_addr = .clientip",
		".request_uri = .request",
		"request_path",
		".body_bytes_sent = .size",
		".request_time = .responsetime",
		".upstream_response_time = .upstreamtime",
		`parse_timestamp(timestamp_text, "%Y-%m-%d %H:%M:%S")`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("writeVectorParseScript() nginx+json content missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, `parse_nginx_log(._raw, "combined")`) {
		t.Fatalf("writeVectorParseScript() nginx+json should not use combined parser:\n%s", content)
	}
}

func TestWriteVectorParseScriptForNginxJSONLoose(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "nginx+json-loose",
	}, nil)

	content := builder.String()
	for _, want := range []string{
		`.parse_status = "raw_fallback"`,
		"parse_json(._raw)",
		`.parse_status = "json_ok"`,
		`.parse_status = "loose_ok"`,
		`parse_regex(._raw, r'"post_body"\s*:\s*"(?P<loose_post_body>.*)"\s*}\s*$')`,
		`parse_regex(._raw, r'"request_body"\s*:\s*"(?P<loose_request_body>.*)"\s*}\s*$')`,
		".remote_addr = .clientip",
		".request_uri = .request",
		".body_bytes_sent = .size",
		".request_time = .responsetime",
		".upstream_response_time = .upstreamtime",
		`.event_timestamp = ."@timestamp"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("writeVectorParseScript() nginx+json-loose content missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, `parse_nginx_log(._raw, "combined")`) {
		t.Fatalf("writeVectorParseScript() nginx+json-loose should not use combined parser:\n%s", content)
	}
	for _, notWant := range []string{".post_body_json", ".request_body_json"} {
		if strings.Contains(content, notWant) {
			t.Fatalf("writeVectorParseScript() nginx+json-loose should not create %q:\n%s", notWant, content)
		}
	}
	if strings.Contains(content, "loose_@timestamp") {
		t.Fatalf("writeVectorParseScript() nginx+json-loose contains invalid VRL identifier:\n%s", content)
	}
}

func TestWriteVectorParseScriptForNginxJSONLooseIgnoresLegacyTemplate(t *testing.T) {
	var builder strings.Builder
	template := &logtailmodel.LogParseTemplate{
		VRLScript: `.request_query = join(request_query_parts, "?")`,
	}

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "nginx_json_loose",
	}, template)

	content := builder.String()
	if strings.Contains(content, `.request_query = join(request_query_parts, "?")`) {
		t.Fatalf("nginx_json_loose should ignore legacy template:\n%s", content)
	}
	for _, want := range []string{`.parse_status = "raw_fallback"`, `.parse_status = "json_ok"`, `.parse_status = "loose_ok"`} {
		if !strings.Contains(content, want) {
			t.Fatalf("nginx_json_loose builtin script missing %q:\n%s", want, content)
		}
	}
}

func TestWriteVectorParseScriptUsesTaskRegexPattern(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "regex",
		RegexPattern:      `^(?P<status>\d{3}) (?P<bytes>\d+)$`,
	}, nil)

	content := builder.String()
	if !strings.Contains(content, `parse_regex(._raw, r'^(?P<status>\d{3}) (?P<bytes>\d+)$')`) {
		t.Fatalf("writeVectorParseScript() content missing regex extraction:\n%s", content)
	}
	if !strings.Contains(content, "merge(., parsed)") {
		t.Fatalf("writeVectorParseScript() content should use object merge:\n%s", content)
	}
	if strings.Contains(content, "merge_err = merge") {
		t.Fatalf("writeVectorParseScript() regex parser should not assign merge error:\n%s", content)
	}
	if strings.Contains(content, "merge!(., parsed)") {
		t.Fatalf("writeVectorParseScript() contains aborting object merge:\n%s", content)
	}
}

func TestWriteVectorParseScriptForDBSlowLogUsesBuiltinParser(t *testing.T) {
	var builder strings.Builder

	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		ParseModeSnapshot: "db_slow_log",
	}, nil)

	content := builder.String()
	if !strings.Contains(content, `# Time:`) {
		t.Fatalf("writeVectorParseScript() content missing slow log time parsing:\n%s", content)
	}
	if !strings.Contains(content, `sql_text`) {
		t.Fatalf("writeVectorParseScript() content missing sql_text extraction:\n%s", content)
	}
	if !strings.Contains(content, `parse_timestamp(.unix_ts, "%s")`) {
		t.Fatalf("writeVectorParseScript() content missing unix timestamp fallback:\n%s", content)
	}
}
