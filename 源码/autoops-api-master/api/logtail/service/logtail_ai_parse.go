package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	aiopsservice "dodevops-api/api/aiops/service"
	logtailmodel "dodevops-api/api/logtail/model"
)

const (
	logParseTemplateAISystemPrompt = `你是日志采集平台的 Vector VRL 解析规则生成器。只输出 JSON，不要输出 Markdown。
目标：根据样例日志生成可嵌入 Vector remap transform 的 VRL 脚本。
约束：
1. 输出必须是一个 JSON object。
2. 不要生成 sources/transforms/sinks，只生成 remap source 里的脚本片段。
3. 上游已经设置 ._raw = .message，下游会设置 ._msg/.timestamp/._time 和平台公共字段。
4. 如果样例是 JSON，使用 parsed, err = parse_json(._raw)，并在对象合并时使用 ., merge_err = merge(., parsed)。
5. 如果使用 parse_regex/parse_nginx_log 的结果合并，使用 . = merge(., xxx)，不要接收 merge_err。
6. 不要建议用户修改 nginx log_format，不要输出 nginx 配置片段，不要输出 $time_iso8601、escape=json 等 nginx 配置内容。
7. 对业务 nginx JSON 日志，保留原字段，同时补充标准字段：clientip->remote_addr，request->request_uri/request_path/request_query，size->body_bytes_sent，referer->http_referer，responsetime->request_time，upstreamtime->upstream_response_time。
8. 如果样例是 JSON-like 但不是严格 JSON，例如字段 post_body 的值包含未转义的 JSON 对象字符串，必须生成兜底逻辑：先 parse_json(._raw)，失败后用 parse_regex 从原始文本提取核心字段和 post_body，不能直接丢弃日志。
9. 解析失败时必须保留原始日志，并设置 .parse_status = "raw_fallback"；严格 JSON 成功时设置 .parse_status = "json_ok"；宽松兜底成功时设置 .parse_status = "loose_ok"。
10. request_body、post_body 等请求内容只保留字符串原文，不要 parse_json 后写入 .request_body_json/.post_body_json 等对象字段，避免 VictoriaLogs 将动态请求体展开成大量索引字段。
11. Vector file source 已经自带 timestamp 类型的 .timestamp，不要再次对现有 .timestamp 调用 parse_timestamp；日志中的 @timestamp、time 等业务时间字段应安全解析后覆盖 .timestamp，并可将原始值保留到 .event_timestamp。
12. 时间字段要安全解析，不要使用会中断事件的 bang 函数；不要写 timestamp_value = null；解析 @timestamp 时先用 timestamp_text = to_string(."@timestamp") ?? "" 转成字符串，并且 timestamp_text != "" 时才调用 parse_timestamp(timestamp_text, "%+")。
13. slice() 和 join() 都按可失败函数处理，不能直接嵌套或直接赋值给事件字段；必须分别接收错误并在成功后赋值，或使用已知安全的正则提取 request_path/request_query。
14. 输出 JSON 字段：name, parseMode, vrlScript, fields, fieldMappings, warnings。`

	maxLogParseTemplateSampleRunes = 6000
	maxLogParseTemplateAnswerRunes = 20000
	maxLogParseTemplateSampleLines = 5
	maxLogParseTemplateStringRunes = 320
	maxLogParseTemplateBodyRunes   = 640
	maxLogParseTemplateArrayItems  = 8
)

type logParseTemplateAIAnswer struct {
	Name          string            `json:"name"`
	Code          string            `json:"code"`
	ParseMode     string            `json:"parseMode"`
	VRLScript     string            `json:"vrlScript"`
	Fields        []string          `json:"fields"`
	FieldMappings map[string]string `json:"fieldMappings"`
	Warnings      []string          `json:"warnings"`
}

func (s *LogtailService) GenerateParseTemplateWithAI(ctx context.Context, req logtailmodel.LogParseTemplateAIGenerateRequest) (*logtailmodel.LogParseTemplateAIGenerateResponse, error) {
	req.SampleLog = strings.TrimSpace(req.SampleLog)
	if req.SampleLog == "" {
		return nil, errors.New("样例日志不能为空")
	}

	provider, apiKey, err := aiopsservice.ResolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	answer, err := aiopsservice.CallLLM(llmCtx, provider, apiKey, logParseTemplateAISystemPrompt, buildLogParseTemplateAIPrompt(req))
	if err != nil {
		return nil, err
	}
	answer = trimRunes(strings.TrimSpace(answer), maxLogParseTemplateAnswerRunes)

	parsed, err := parseLogParseTemplateAIAnswer(answer)
	if err != nil {
		return nil, err
	}
	template, err := buildLogParseTemplateFromAI(req, parsed)
	if err != nil {
		return nil, err
	}
	if req.Save {
		if err := s.CreateParseTemplate(&template); err != nil {
			return nil, err
		}
	}

	return &logtailmodel.LogParseTemplateAIGenerateResponse{
		UsedLLM:       true,
		Saved:         req.Save,
		Template:      template,
		Fields:        cleanStringList(parsed.Fields),
		FieldMappings: cleanStringMap(parsed.FieldMappings),
		Warnings:      cleanStringList(parsed.Warnings),
		RawAnswer:     answer,
	}, nil
}

func buildLogParseTemplateAIPrompt(req logtailmodel.LogParseTemplateAIGenerateRequest) string {
	rawLogType := strings.TrimSpace(firstNonEmpty(req.LogType, "json"))
	parseMode := normalizeCollectTaskParseMode(rawLogType)
	displayLogType := displayCollectTaskParseMode(parseMode)
	sample := optimizeLogSampleForAI(req.SampleLog)
	return fmt.Sprintf(`请为以下日志样例生成 Vector VRL 解析模板。

用户选择的日志类型: %s
建议解析模式: %s
写入日志的 log_type: %s
模板名称: %s

样例日志:
%s

说明：样例由后端做了最小化处理；<redacted> 表示敏感值，...[truncated]... 表示长字段值被截断。字段名和数据类型仍然有效，不要把这些占位内容写入解析规则。

请返回严格 JSON，例如：
{
  "name": "xxx",
  "parseMode": "nginx",
  "vrlScript": "parsed, err = parse_json(._raw)\n...",
  "fields": ["status", "request_path"],
  "fieldMappings": {"clientip": "remote_addr"},
  "warnings": []
}
`, rawLogType, parseMode, displayLogType, strings.TrimSpace(req.Name), sample)
}

func optimizeLogSampleForAI(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if raw == "" {
		return raw
	}
	if compacted, ok := compactJSONSampleForAI(raw); ok {
		return trimLogSampleForAI(compacted)
	}

	lines := strings.Split(raw, "\n")
	selected := make([]string, 0, minInt(len(lines), maxLogParseTemplateSampleLines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if compacted, ok := compactJSONSampleForAI(line); ok {
			line = compacted
		} else {
			line = redactSensitiveTextFieldsForAI(line)
		}
		selected = append(selected, line)
		if len(selected) >= maxLogParseTemplateSampleLines {
			break
		}
	}
	if len(selected) == 0 {
		return trimLogSampleForAI(redactSensitiveTextFieldsForAI(raw))
	}
	return trimLogSampleForAI(strings.Join(selected, "\n"))
}

func compactJSONSampleForAI(raw string) (string, bool) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", false
	}
	compacted := compactJSONValueForAI(value, "", 0)
	payload, err := marshalJSONForAI(compacted)
	if err != nil {
		return "", false
	}
	return payload, true
}

func marshalJSONForAI(value interface{}) (string, error) {
	var builder strings.Builder
	encoder := json.NewEncoder(&builder)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(builder.String(), "\n"), nil
}

func compactJSONValueForAI(value interface{}, field string, depth int) interface{} {
	if isSensitiveLogFieldForAI(field) {
		return "<redacted>"
	}
	if depth >= 8 {
		return "...[truncated]..."
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			result[key] = compactJSONValueForAI(item, key, depth+1)
		}
		return result
	case []interface{}:
		limit := minInt(len(typed), maxLogParseTemplateArrayItems)
		result := make([]interface{}, 0, limit)
		for index := 0; index < limit; index++ {
			result = append(result, compactJSONValueForAI(typed[index], field, depth+1))
		}
		return result
	case string:
		limit := maxLogParseTemplateStringRunes
		if isLargeContentLogFieldForAI(field) {
			limit = maxLogParseTemplateBodyRunes
		}
		return truncateLogFieldForAI(typed, limit)
	default:
		return value
	}
}

func isSensitiveLogFieldForAI(field string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	field = strings.NewReplacer("-", "_", ".", "_").Replace(field)
	for _, keyword := range []string{"token", "api_key", "apikey", "authorization", "cookie", "password", "passwd", "secret", "access_key"} {
		if strings.Contains(field, keyword) {
			return true
		}
	}
	return false
}

func isLargeContentLogFieldForAI(field string) bool {
	field = strings.ToLower(strings.TrimSpace(field))
	for _, keyword := range []string{"body", "content", "payload", "message", "stack", "exception", "sql"} {
		if strings.Contains(field, keyword) {
			return true
		}
	}
	return false
}

func truncateLogFieldForAI(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	marker := []rune("...[truncated]...")
	remaining := limit - len(marker)
	if remaining <= 0 {
		return string(marker)
	}
	head := remaining * 2 / 3
	tail := remaining - head
	return string(runes[:head]) + string(marker) + string(runes[len(runes)-tail:])
}

func trimLogSampleForAI(value string) string {
	return truncateLogFieldForAI(value, maxLogParseTemplateSampleRunes)
}

func redactSensitiveTextFieldsForAI(value string) string {
	for _, field := range []string{
		"authorization", "cookie", "set-cookie", "password", "passwd", "secret",
		"api_key", "lyj_openclaw_api_key", "x_zai_session_token", "session_token", "access_token",
	} {
		value = replaceJSONStringFieldForAI(value, field, "<redacted>")
	}
	return value
}

func replaceJSONStringFieldForAI(raw, field, replacement string) string {
	marker := `"` + field + `"`
	for offset := 0; offset < len(raw); {
		relative := strings.Index(raw[offset:], marker)
		if relative < 0 {
			break
		}
		start := offset + relative
		cursor := start + len(marker)
		for cursor < len(raw) && (raw[cursor] == ' ' || raw[cursor] == '\t') {
			cursor++
		}
		if cursor >= len(raw) || raw[cursor] != ':' {
			offset = start + len(marker)
			continue
		}
		cursor++
		for cursor < len(raw) && (raw[cursor] == ' ' || raw[cursor] == '\t') {
			cursor++
		}
		if cursor >= len(raw) || raw[cursor] != '"' {
			offset = cursor
			continue
		}
		valueStart := cursor
		cursor++
		escaped := false
		for cursor < len(raw) {
			if escaped {
				escaped = false
				cursor++
				continue
			}
			if raw[cursor] == '\\' {
				escaped = true
				cursor++
				continue
			}
			if raw[cursor] == '"' {
				encoded, _ := marshalJSONForAI(replacement)
				raw = raw[:valueStart] + encoded + raw[cursor+1:]
				offset = valueStart + len(encoded)
				break
			}
			cursor++
		}
		if cursor >= len(raw) {
			break
		}
	}
	return raw
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func parseLogParseTemplateAIAnswer(raw string) (logParseTemplateAIAnswer, error) {
	var answer logParseTemplateAIAnswer
	payload, err := extractJSONObject(raw)
	if err != nil {
		return answer, err
	}
	if err := json.Unmarshal([]byte(payload), &answer); err != nil {
		return answer, fmt.Errorf("解析 AI 返回 JSON 失败: %w", err)
	}
	answer.ParseMode = normalizeCollectTaskParseMode(answer.ParseMode)
	answer.VRLScript = strings.TrimSpace(answer.VRLScript)
	if answer.ParseMode == "" {
		answer.ParseMode = "json"
	}
	if !isSupportedLogParseMode(answer.ParseMode) {
		return answer, fmt.Errorf("AI 返回不支持的解析模式: %s", answer.ParseMode)
	}
	if answer.VRLScript == "" {
		return answer, errors.New("AI 返回缺少 vrlScript")
	}
	answer.VRLScript, answer.Warnings = repairGeneratedVRLScript(answer.VRLScript, answer.Warnings)
	if err := validateGeneratedVRLScript(answer.VRLScript); err != nil {
		return answer, err
	}
	return answer, nil
}

func isSupportedLogParseMode(mode string) bool {
	switch normalizeCollectTaskParseMode(mode) {
	case "raw", "json", "regex", "nginx", "nginx_json", "nginx_json_loose", "db_slow_log":
		return true
	default:
		return false
	}
}

func buildLogParseTemplateFromAI(req logtailmodel.LogParseTemplateAIGenerateRequest, answer logParseTemplateAIAnswer) (logtailmodel.LogParseTemplate, error) {
	name := firstNonEmpty(answer.Name, req.Name, "AI生成日志解析模板")
	requestedParseMode := normalizeCollectTaskParseMode(req.LogType)
	parseMode := normalizeCollectTaskParseMode(firstNonEmpty(answer.ParseMode, req.LogType, "json"))
	if requestedParseMode == "nginx_json_loose" {
		parseMode = requestedParseMode
		answer.VRLScript = builtinGeneratedVRLScript(writeVectorNginxJSONLooseParseScript)
	}
	if parseMode == "" {
		parseMode = "json"
	}
	fieldsMappingJSON, err := json.Marshal(cleanStringMap(answer.FieldMappings))
	if err != nil {
		return logtailmodel.LogParseTemplate{}, err
	}
	return logtailmodel.LogParseTemplate{
		ProjectID:         req.ProjectID,
		Name:              name,
		Code:              generateUniqueAIParseTemplateCode(answer.Code, name),
		ParseMode:         parseMode,
		InputFormat:       parseMode,
		FieldsMappingJSON: string(fieldsMappingJSON),
		VRLScript:         answer.VRLScript,
		Status:            logtailmodel.LogStatusEnabled,
		Version:           1,
		Remark:            "AI 根据样例日志生成",
	}, nil
}

func generateUniqueAIParseTemplateCode(rawCode, name string) string {
	base := normalizeLogtailCode(firstNonEmpty(rawCode, name, "ai_parse_template"))
	if base == "" {
		base = "ai_parse_template"
	}
	return fmt.Sprintf("%s_%s", base, randomLogtailCodeSuffix())
}

func extractJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("AI 返回为空")
	}
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return "", errors.New("AI 返回中未找到 JSON object")
	}
	return raw[start : end+1], nil
}

func validateGeneratedVRLScript(script string) error {
	lower := strings.ToLower(script)
	for _, keyword := range []string{"sources:", "transforms:", "sinks:", "type: file", "type: kafka"} {
		if strings.Contains(lower, keyword) {
			return fmt.Errorf("AI 返回不应包含完整 Vector 配置片段: %s", keyword)
		}
	}
	if strings.Contains(script, "merge!(.") {
		return errors.New("AI 返回不应使用会中断事件的 merge! 写法")
	}
	if strings.Contains(lower, "timestamp_value = null") {
		return errors.New("AI 返回包含不安全时间变量: timestamp_value = null")
	}
	if strings.Contains(lower, "parse_timestamp(timestamp_value") && !strings.Contains(lower, "timestamp_value = \"\"") && !strings.Contains(lower, "timestamp_value = to_string") {
		return errors.New("AI 返回的时间解析未先将 timestamp_value 转为字符串")
	}
	return nil
}

func repairGeneratedVRLScript(script string, warnings []string) (string, []string) {
	script = strings.TrimSpace(strings.ReplaceAll(script, "\r\n", "\n"))
	addWarning := func(message string) {
		for _, warning := range warnings {
			if warning == message {
				return
			}
		}
		warnings = append(warnings, message)
	}

	if strings.Contains(script, "timestamp_value = null") {
		script = strings.ReplaceAll(script, "timestamp_value = null", `timestamp_value = ""`)
		addWarning("后端已修复 timestamp_value = null，避免 Vector 将时间变量推断为 null 类型")
	}
	if strings.Contains(script, "., merge_err = merge(., request_parts)") {
		script = strings.ReplaceAll(script, "., merge_err = merge(., request_parts)", ". = merge(., request_parts)")
		addWarning("后端已修复 request_parts 的 merge 错误赋值")
	}
	if strings.Contains(script, "., merge_err = merge(., uri_parts)") {
		script = strings.ReplaceAll(script, "., merge_err = merge(., uri_parts)", ". = merge(., uri_parts)")
		addWarning("后端已修复 uri_parts 的 merge 错误赋值")
	}
	if strings.Contains(script, "., merge_err = merge(., nginx_parts)") {
		script = strings.ReplaceAll(script, "., merge_err = merge(., nginx_parts)", ". = merge(., nginx_parts)")
		addWarning("后端已修复 nginx_parts 的 merge 错误赋值")
	}
	if strings.Contains(script, `to_string(.timestamp) ?? ""`) {
		script = strings.ReplaceAll(script, `to_string(.timestamp) ?? ""`, `to_string(.timestamp)`)
		addWarning("后端已移除 Vector timestamp 字段不必要的错误合并操作")
	}
	if repaired, changed := repairGeneratedSliceJoin(script); changed {
		script = repaired
		addWarning("后端已修复 request_query 中未处理错误的 slice 调用")
	}
	return script, warnings
}

func repairGeneratedSliceJoin(script string) (string, bool) {
	const invalidExpression = `.request_query = join(slice(request_parts, 1), "?")`
	const fallibleJoinExpression = `.request_query = join(request_query_parts, "?")`
	lines := strings.Split(script, "\n")
	changed := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		switch trimmed {
		case invalidExpression:
			lines[index] = strings.Join([]string{
				indent + "request_query_parts, request_query_parts_err = slice(request_parts, 1)",
				indent + "if request_query_parts_err == null {",
				indent + `  request_query_value, request_query_join_err = join(request_query_parts, "?")`,
				indent + "  if request_query_join_err == null {",
				indent + "    .request_query = request_query_value",
				indent + "  }",
				indent + "}",
			}, "\n")
			changed = true
		case fallibleJoinExpression:
			lines[index] = strings.Join([]string{
				indent + `request_query_value, request_query_join_err = join(request_query_parts, "?")`,
				indent + "if request_query_join_err == null {",
				indent + "  .request_query = request_query_value",
				indent + "}",
			}, "\n")
			changed = true
		}
	}
	return strings.Join(lines, "\n"), changed
}

func builtinGeneratedVRLScript(write func(*strings.Builder)) string {
	var builder strings.Builder
	write(&builder)
	lines := strings.Split(strings.TrimSpace(builder.String()), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimPrefix(line, "      ")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func cleanStringList(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
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
	sort.Strings(result)
	return result
}

func cleanStringMap(values map[string]string) map[string]string {
	result := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	return result
}

func trimRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
