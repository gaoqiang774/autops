package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	"dodevops-api/pkg/logx"

	"github.com/gin-gonic/gin"
)

const (
	maxKnowledgeUploadBytes = 5 * 1024 * 1024
	defaultKnowledgeLimit   = 10
)

var (
	markdownHeadingPattern              = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	knowledgeNumberedLinePattern        = regexp.MustCompile(`^(\d{1,3})(?:[)）]|[、.．])\s*(.+)$`)
	knowledgeChineseSectionPattern      = regexp.MustCompile(`^([一二三四五六七八九十]+)[、.．)）]\s*(.+)$`)
	knowledgeParenChineseSectionPattern = regexp.MustCompile(`^[（(]([一二三四五六七八九十]+)[）)]\s*(.+)$`)
	knowledgeMultiSpacePattern          = regexp.MustCompile(`[ \t]{2,}`)
	knowledgeMarkdownOrderedListPattern = regexp.MustCompile(`^\d+\.\s+`)
	knowledgeDatePrefixPattern          = regexp.MustCompile(`^\d{4}[-/]`)
)

type knowledgeSectionDefinition struct {
	Index   int
	Title   string
	Aliases []string
}

var knowledgeIncidentSectionDefinitions = []knowledgeSectionDefinition{
	{Index: 1, Title: "事件经过", Aliases: []string{"事件经过", "故障经过", "问题经过", "时间线", "处理过程"}},
	{Index: 2, Title: "影响范围", Aliases: []string{"影响范围", "业务影响", "影响面", "影响"}},
	{Index: 3, Title: "根本原因", Aliases: []string{"根本原因", "故障原因", "原因分析", "问题原因", "原因"}},
	{Index: 4, Title: "解决方案", Aliases: []string{"解决方案", "处理方案", "恢复方案", "修复方案", "处置方案"}},
	{Index: 5, Title: "相关责任人", Aliases: []string{"相关责任人", "责任人", "负责人", "参与人员"}},
}

var knowledgeGenericSectionAliases = []string{
	"背景", "故障现象", "现象", "排查思路", "排查过程", "验证方式", "回滚方案",
	"风险", "建议", "注意事项", "前置条件", "操作步骤", "安装步骤", "命令示例",
	"参考链接", "总结", "复盘结论", "Runbook 建议", "Runbook建议",
}

type KnowledgeService struct {
	dao    *aiopsdao.KnowledgeDao
	llmDao *aiopsdao.LLMProviderDao
}

func NewKnowledgeService() *KnowledgeService {
	return &KnowledgeService{
		dao:    aiopsdao.NewKnowledgeDao(),
		llmDao: aiopsdao.NewLLMProviderDao(),
	}
}

func (s *KnowledgeService) CreateDocument(c *gin.Context, dto *aiopsmodel.KnowledgeDocumentDTO) (*aiopsmodel.KnowledgeDocumentVO, error) {
	if dto == nil {
		return nil, errors.New("request is nil")
	}
	doc, err := s.buildKnowledgeDocument(c, dto)
	if err != nil {
		return nil, err
	}
	chunks := buildKnowledgeChunks(doc.ContentMarkdown)
	doc.ChunkCount = len(chunks)
	doc.SearchStatus = aiopsmodel.KnowledgeSearchReady
	if err := s.dao.CreateDocument(doc, chunks); err != nil {
		return nil, err
	}
	if strings.TrimSpace(doc.Summary) == "" {
		s.enqueueKnowledgeSummary(doc.ID, "create")
	}
	return buildKnowledgeDocumentVO(doc), nil
}

func (s *KnowledgeService) ImportDocument(c *gin.Context, fileHeader *multipart.FileHeader, dto *aiopsmodel.KnowledgeDocumentDTO) (*aiopsmodel.KnowledgeDocumentVO, error) {
	if dto == nil {
		dto = &aiopsmodel.KnowledgeDocumentDTO{}
	}
	content := strings.TrimSpace(dto.ContentMarkdown)
	filename := ""
	ext := ""
	if fileHeader != nil {
		if fileHeader.Size > maxKnowledgeUploadBytes {
			return nil, fmt.Errorf("file too large, max %d bytes", maxKnowledgeUploadBytes)
		}
		filename = filepath.Base(fileHeader.Filename)
		ext = strings.ToLower(filepath.Ext(filename))
		if ext != ".md" && ext != ".markdown" && ext != ".txt" {
			return nil, errors.New("only .md, .markdown and .txt files are supported in this phase")
		}
		file, err := fileHeader.Open()
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxKnowledgeUploadBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxKnowledgeUploadBytes {
			return nil, fmt.Errorf("file too large, max %d bytes", maxKnowledgeUploadBytes)
		}
		content = strings.TrimSpace(string(data))
	}
	if content == "" {
		return nil, errors.New("file or contentMarkdown is required")
	}
	if !utf8.ValidString(content) {
		return nil, errors.New("file must be utf-8 text")
	}
	if strings.TrimSpace(dto.Title) == "" {
		if filename != "" {
			dto.Title = strings.TrimSuffix(filename, ext)
		} else {
			dto.Title = inferKnowledgeTitleFromContent(content)
		}
	}
	dto.ContentMarkdown = normalizeKnowledgeMarkdown(dto.Title, dto.DocType, content)
	doc, err := s.buildKnowledgeDocument(c, dto)
	if err != nil {
		return nil, err
	}
	doc.SourceFilename = filename
	chunks := buildKnowledgeChunks(doc.ContentMarkdown)
	doc.ChunkCount = len(chunks)
	doc.SearchStatus = aiopsmodel.KnowledgeSearchReady
	if err := s.dao.CreateDocument(doc, chunks); err != nil {
		return nil, err
	}
	s.enqueueKnowledgeSummary(doc.ID, "import")
	return buildKnowledgeDocumentVO(doc), nil
}

func (s *KnowledgeService) UpdateDocument(c *gin.Context, id uint, dto *aiopsmodel.KnowledgeDocumentDTO) (*aiopsmodel.KnowledgeDocumentVO, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	if dto == nil {
		return nil, errors.New("request is nil")
	}
	existing, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	doc, err := s.buildKnowledgeDocument(c, dto)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{
		"title":             doc.Title,
		"doc_type":          doc.DocType,
		"status":            doc.Status,
		"group_id":          doc.GroupID,
		"group_name":        doc.GroupName,
		"category":          doc.Category,
		"severity":          doc.Severity,
		"service_name":      doc.ServiceName,
		"env":               doc.Env,
		"tags_json":         doc.TagsJSON,
		"keywords_json":     doc.KeywordsJSON,
		"alert_names_json":  doc.AlertNamesJSON,
		"log_keywords_json": doc.LogKeywordsJSON,
		"owners_json":       doc.OwnersJSON,
		"content_markdown":  doc.ContentMarkdown,
		"summary":           doc.Summary,
		"rollback_plan":     doc.RollbackPlan,
		"updated_by":        doc.UpdatedBy,
		"summary_status":    doc.SummaryStatus,
		"summary_error":     doc.SummaryError,
	}
	contentChanged := existing.ContentMarkdown != doc.ContentMarkdown
	chunks := buildKnowledgeChunks(doc.ContentMarkdown)
	if contentChanged {
		updates["search_status"] = aiopsmodel.KnowledgeSearchReady
		updates["search_error"] = ""
		updates["search_version"] = existing.SearchVersion + 1
		updates["chunk_count"] = len(chunks)
	}
	if err := s.dao.UpdateDocument(id, updates, chunks, contentChanged); err != nil {
		return nil, err
	}
	updated, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	if contentChanged && strings.TrimSpace(dto.Summary) == "" {
		s.enqueueKnowledgeSummary(id, "update")
	}
	return buildKnowledgeDocumentVO(updated), nil
}

func (s *KnowledgeService) DeleteDocument(id uint) error {
	if id == 0 {
		return errors.New("id is required")
	}
	return s.dao.DeleteDocument(id)
}

func (s *KnowledgeService) GetDocument(id uint) (*aiopsmodel.KnowledgeDocumentVO, error) {
	doc, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	s.hydrateKnowledgeDocumentGroups([]*aiopsmodel.KnowledgeDocument{doc})
	return buildKnowledgeDocumentVO(doc), nil
}

func (s *KnowledgeService) ListDocuments(query aiopsmodel.KnowledgeDocumentQuery) ([]aiopsmodel.KnowledgeDocumentVO, int64, aiopsmodel.KnowledgeDocumentQuery, error) {
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	if query.GroupID > 0 && query.Recursive {
		groupIDs, err := NewKnowledgeGroupService().DescendantIDs(query.GroupID)
		if err != nil {
			return nil, 0, query, err
		}
		query.GroupIDs = groupIDs
	}
	docs, total, err := s.dao.ListDocuments(query)
	if err != nil {
		return nil, 0, query, err
	}
	docPointers := make([]*aiopsmodel.KnowledgeDocument, 0, len(docs))
	for i := range docs {
		docPointers = append(docPointers, &docs[i])
	}
	s.hydrateKnowledgeDocumentGroups(docPointers)
	return buildKnowledgeDocumentVOs(docs), total, query, nil
}

func (s *KnowledgeService) SearchDocuments(req *aiopsmodel.KnowledgeSearchRequest) ([]aiopsmodel.KnowledgeSearchItem, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		return nil, errors.New("query is required")
	}
	if req.Limit <= 0 {
		req.Limit = defaultKnowledgeLimit
	}
	req.QueryTerms = tokenizeKnowledgeQuery(req.Query, 16)
	if req.GroupID > 0 {
		groupIDs, err := NewKnowledgeGroupService().DescendantIDs(req.GroupID)
		if err != nil {
			return nil, err
		}
		req.GroupIDs = groupIDs
	}
	docs, err := s.dao.SearchDocuments(*req)
	if err != nil {
		return nil, err
	}
	docPointers := make([]*aiopsmodel.KnowledgeDocument, 0, len(docs))
	for i := range docs {
		docPointers = append(docPointers, &docs[i])
	}
	s.hydrateKnowledgeDocumentGroups(docPointers)
	items := make([]aiopsmodel.KnowledgeSearchItem, 0, len(docs))
	for _, doc := range docs {
		items = append(items, scoreKnowledgeDocument(doc, req.Query))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		return items[i].Score > items[j].Score
	})
	if len(items) > req.Limit {
		items = items[:req.Limit]
	}
	return items, nil
}

func (s *KnowledgeService) SummarizeDocument(ctx context.Context, id uint, req *aiopsmodel.KnowledgeSummaryRequest) (*aiopsmodel.KnowledgeSummaryResult, error) {
	doc, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	_ = s.dao.UpdateDocument(id, map[string]interface{}{"summary_status": aiopsmodel.KnowledgeSummaryProcessing, "summary_error": ""}, nil, false)
	result := buildRuleKnowledgeSummary(doc)
	if req == nil {
		req = &aiopsmodel.KnowledgeSummaryRequest{}
	}
	provider, apiKey, err := s.resolveKnowledgeLLMProvider(req.ProviderID)
	if err == nil && provider != nil {
		timeout := time.Duration(provider.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		llmCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		answer, callErr := callLLMForLogAnalysis(llmCtx, provider, apiKey, buildKnowledgeSummaryPrompt(doc))
		if callErr == nil {
			if parsed, parseErr := parseKnowledgeSummary(answer); parseErr == nil {
				result = parsed
				result.UsedLLM = true
				result.ProviderID = provider.ID
				result.ProviderName = provider.Name
				result.ProviderType = provider.ProviderType
				result.ModelCode = provider.DefaultModel
			}
		}
	}
	result.DocumentID = doc.ID
	updates := map[string]interface{}{
		"summary":               result.Summary,
		"symptoms_json":         encodeKnowledgeStringSlice(result.Symptoms),
		"root_causes_json":      encodeKnowledgeStringSlice(result.RootCauses),
		"resolution_steps_json": encodeKnowledgeStringSlice(result.ResolutionSteps),
		"verification_json":     encodeKnowledgeStringSlice(result.Verification),
		"keywords_json":         encodeKnowledgeStringSlice(mergeKnowledgeStringSlices(decodeKnowledgeStringSlice(doc.KeywordsJSON), result.Keywords)),
		"summary_status":        aiopsmodel.KnowledgeSummaryCompleted,
		"summary_error":         "",
	}
	if err := s.dao.UpdateDocument(doc.ID, updates, nil, false); err != nil {
		_ = s.dao.UpdateDocument(doc.ID, map[string]interface{}{"summary_status": aiopsmodel.KnowledgeSummaryFailed, "summary_error": err.Error()}, nil, false)
		return nil, err
	}
	return result, nil
}

func (s *KnowledgeService) RefreshSearchData(id uint) (*aiopsmodel.KnowledgeDocumentVO, error) {
	if id == 0 {
		return nil, errors.New("id is required")
	}
	doc, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	chunks := buildKnowledgeChunks(doc.ContentMarkdown)
	updates := map[string]interface{}{
		"search_status":  aiopsmodel.KnowledgeSearchReady,
		"search_error":   "",
		"search_version": doc.SearchVersion + 1,
		"chunk_count":    len(chunks),
		"keywords_json": encodeKnowledgeStringSlice(mergeKnowledgeStringSlices(
			decodeKnowledgeStringSlice(doc.KeywordsJSON),
			extractKnowledgeKeywords(doc.ContentMarkdown, 24),
		)),
	}
	if err := s.dao.RebuildDocumentChunks(id, chunks, updates); err != nil {
		_ = s.dao.UpdateDocument(id, map[string]interface{}{
			"search_status": aiopsmodel.KnowledgeSearchFailed,
			"search_error":  err.Error(),
		}, nil, false)
		return nil, err
	}
	updated, err := s.dao.GetDocument(id)
	if err != nil {
		return nil, err
	}
	s.hydrateKnowledgeDocumentGroups([]*aiopsmodel.KnowledgeDocument{updated})
	return buildKnowledgeDocumentVO(updated), nil
}

func (s *KnowledgeService) enqueueKnowledgeSummary(documentID uint, trigger string) {
	if documentID == 0 {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		logx.Infof("knowledge summary auto task started, documentId=%d, trigger=%s", documentID, trigger)
		if _, err := NewKnowledgeService().SummarizeDocument(ctx, documentID, &aiopsmodel.KnowledgeSummaryRequest{}); err != nil {
			logx.Errorf("knowledge summary auto task failed, documentId=%d, trigger=%s, error=%v", documentID, trigger, err)
			return
		}
		logx.Infof("knowledge summary auto task completed, documentId=%d, trigger=%s", documentID, trigger)
	}()
}

func (s *KnowledgeService) buildKnowledgeDocument(c *gin.Context, dto *aiopsmodel.KnowledgeDocumentDTO) (*aiopsmodel.KnowledgeDocument, error) {
	title := strings.TrimSpace(dto.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	content := strings.TrimSpace(dto.ContentMarkdown)
	if content == "" {
		return nil, errors.New("contentMarkdown is required")
	}
	userID := knowledgeUserID(c)
	groupID, groupName, category, groupPath, err := NewKnowledgeGroupService().ResolveGroup(dto.GroupID, dto.GroupName, dto.Category, userID)
	if err != nil {
		return nil, err
	}
	docType := normalizeKnowledgeDocType(dto.DocType)
	status := normalizeKnowledgeStatus(dto.Status)
	env := normalizeKnowledgeEnv(dto.Env)
	summaryStatus := aiopsmodel.KnowledgeSummaryPending
	if strings.TrimSpace(dto.Summary) != "" {
		summaryStatus = aiopsmodel.KnowledgeSummaryCompleted
	}
	return &aiopsmodel.KnowledgeDocument{
		Title:           title,
		DocType:         docType,
		Status:          status,
		GroupID:         groupID,
		GroupPath:       groupPath,
		GroupName:       groupName,
		Category:        category,
		Severity:        strings.TrimSpace(dto.Severity),
		ServiceName:     strings.TrimSpace(dto.ServiceName),
		Env:             env,
		TagsJSON:        encodeKnowledgeStringSlice(dto.Tags),
		KeywordsJSON:    encodeKnowledgeStringSlice(dto.Keywords),
		AlertNamesJSON:  encodeKnowledgeStringSlice(dto.AlertNames),
		LogKeywordsJSON: encodeKnowledgeStringSlice(dto.LogKeywords),
		OwnersJSON:      encodeKnowledgeStringSlice(dto.Owners),
		ContentMarkdown: normalizeKnowledgeMarkdown(title, docType, content),
		Summary:         strings.TrimSpace(dto.Summary),
		RollbackPlan:    strings.TrimSpace(dto.RollbackPlan),
		SummaryStatus:   summaryStatus,
		SearchStatus:    aiopsmodel.KnowledgeSearchReady,
		SearchVersion:   1,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}, nil
}

func (s *KnowledgeService) hydrateKnowledgeDocumentGroups(documents []*aiopsmodel.KnowledgeDocument) {
	groups, _, err := aiopsdao.NewKnowledgeGroupDao().List()
	if err != nil {
		return
	}
	for _, doc := range documents {
		if doc == nil || doc.GroupID == 0 {
			continue
		}
		if labels, ok := knowledgeGroupLabels(groups, doc.GroupID); ok {
			doc.GroupPath = strings.Join(labels, " / ")
		}
	}
}

func (s *KnowledgeService) resolveKnowledgeLLMProvider(providerID uint) (*aiopsmodel.LLMProvider, string, error) {
	querySvc := &AssistantQueryService{llmDao: s.llmDao}
	return querySvc.resolveLLMProvider(providerID)
}

func normalizeKnowledgeMarkdown(title, docType, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}
	if strings.HasPrefix(content, "---") {
		return content
	}
	content = autoFormatKnowledgeMarkdown(content, docType)
	if !strings.HasPrefix(content, "# ") {
		content = "# " + strings.TrimSpace(title) + "\n\n" + content
	}
	return content
}

func inferKnowledgeTitleFromContent(content string) string {
	for _, line := range normalizeKnowledgeInputLines(content) {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			return truncateKnowledgeText(line, 80)
		}
	}
	return "未命名知识文档"
}

func autoFormatKnowledgeMarkdown(content, docType string) string {
	lines := normalizeKnowledgeInputLines(content)
	if len(lines) == 0 {
		return ""
	}
	result := make([]string, 0, len(lines)*2)
	inFence := false
	inTable := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			appendKnowledgeBlankLine(&result)
			inTable = false
			continue
		}
		if strings.HasPrefix(line, "```") {
			appendKnowledgeNonBlankLine(&result, line)
			inFence = !inFence
			inTable = false
			continue
		}
		if inFence {
			appendKnowledgeNonBlankLine(&result, raw)
			continue
		}
		line = normalizeKnowledgeTextPunctuation(line)
		if strings.HasPrefix(line, "#") || isKnowledgeMarkdownListLine(line) || strings.HasPrefix(line, ">") || strings.HasPrefix(line, "|") || strings.HasPrefix(line, "---") {
			appendKnowledgeNonBlankLine(&result, line)
			inTable = strings.HasPrefix(line, "|")
			continue
		}
		wasInTable := inTable
		if tableLine, ok := tryFormatKnowledgeTableLine(line, inTable); ok {
			appendKnowledgeNonBlankLine(&result, tableLine)
			if !wasInTable {
				appendKnowledgeNonBlankLine(&result, buildKnowledgeMarkdownTableSeparator(tableLine))
			}
			inTable = true
			continue
		}
		inTable = false
		if item, ok := tryFormatKnowledgeOrderedList(line); ok {
			appendKnowledgeNonBlankLine(&result, item)
			continue
		}
		if item, ok := tryFormatKnowledgeKeyValueLine(line); ok {
			appendKnowledgeNonBlankLine(&result, item)
			continue
		}
		if heading, ok := tryFormatKnowledgeHeading(line, docType); ok {
			appendKnowledgeBlankLine(&result)
			appendKnowledgeNonBlankLine(&result, heading)
			appendKnowledgeBlankLine(&result)
			continue
		}
		appendKnowledgeNonBlankLine(&result, line)
	}
	return compactKnowledgeMarkdown(result)
}

func normalizeKnowledgeInputLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	rawLines := strings.Split(content, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, " \t")
		lines = append(lines, line)
	}
	return lines
}

func normalizeKnowledgeTextPunctuation(line string) string {
	line = strings.ReplaceAll(line, "：", ": ")
	line = strings.ReplaceAll(line, "；", ";")
	line = strings.ReplaceAll(line, "，", ", ")
	line = strings.ReplaceAll(line, "。", ".")
	line = knowledgeMultiSpacePattern.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}

func tryFormatKnowledgeHeading(line, docType string) (string, bool) {
	if match := knowledgeChineseSectionPattern.FindStringSubmatch(line); len(match) == 3 {
		title := strings.TrimSpace(match[2])
		if normalized, ok := normalizeKnowledgeSectionTitle(title, docType); ok {
			return normalized, true
		}
		return "## " + match[1] + "、" + title, true
	}
	if match := knowledgeParenChineseSectionPattern.FindStringSubmatch(line); len(match) == 3 {
		title := strings.TrimSpace(match[2])
		if normalized, ok := normalizeKnowledgeSectionTitle(title, docType); ok {
			return normalized, true
		}
		return "## " + title, true
	}
	return normalizeKnowledgeSectionTitle(line, docType)
}

func normalizeKnowledgeSectionTitle(title, docType string) (string, bool) {
	cleanTitle := strings.TrimSpace(strings.Trim(title, ":："))
	cleanTitle = strings.TrimSpace(cleanTitle)
	if cleanTitle == "" {
		return "", false
	}
	if docType == aiopsmodel.KnowledgeDocTypeIncident {
		for _, section := range knowledgeIncidentSectionDefinitions {
			for _, alias := range section.Aliases {
				if cleanTitle == alias || strings.Contains(cleanTitle, alias) && len([]rune(cleanTitle)) <= 16 {
					return fmt.Sprintf("## %d. %s", section.Index, section.Title), true
				}
			}
		}
	}
	for _, alias := range knowledgeGenericSectionAliases {
		if cleanTitle == alias || strings.Contains(cleanTitle, alias) && len([]rune(cleanTitle)) <= 16 {
			return "## " + cleanTitle, true
		}
	}
	return "", false
}

func tryFormatKnowledgeOrderedList(line string) (string, bool) {
	match := knowledgeNumberedLinePattern.FindStringSubmatch(line)
	if len(match) != 3 {
		return "", false
	}
	body := strings.TrimSpace(match[2])
	if body == "" {
		return "", false
	}
	return match[1] + ". " + body, true
}

func tryFormatKnowledgeKeyValueLine(line string) (string, bool) {
	idx := strings.Index(line, ": ")
	if idx <= 0 || idx > 24 {
		return "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(line[idx+2:])
	if key == "" || value == "" {
		return "", false
	}
	if strings.ContainsAny(key, "，,。.!！？?；;") {
		return "", false
	}
	return "- **" + key + "**: " + value, true
}

func tryFormatKnowledgeTableLine(line string, inTable bool) (string, bool) {
	if strings.Contains(line, "\t") {
		cells := cleanKnowledgeStringSlice(strings.Split(line, "\t"))
		if len(cells) >= 2 {
			return buildKnowledgeMarkdownTableLine(cells), true
		}
	}
	if !inTable {
		return "", false
	}
	cells := knowledgeMultiSpacePattern.Split(line, -1)
	cells = cleanKnowledgeStringSlice(cells)
	if len(cells) < 2 {
		return "", false
	}
	return buildKnowledgeMarkdownTableLine(cells), true
}

func buildKnowledgeMarkdownTableLine(cells []string) string {
	clean := make([]string, 0, len(cells))
	for _, cell := range cells {
		cell = strings.ReplaceAll(strings.TrimSpace(cell), "|", "\\|")
		if cell != "" {
			clean = append(clean, cell)
		}
	}
	if len(clean) == 0 {
		return ""
	}
	return "| " + strings.Join(clean, " | ") + " |"
}

func buildKnowledgeMarkdownTableSeparator(tableLine string) string {
	tableLine = strings.TrimSpace(tableLine)
	if !strings.HasPrefix(tableLine, "|") || !strings.HasSuffix(tableLine, "|") {
		return "| --- |"
	}
	cells := strings.Split(strings.Trim(tableLine, "|"), "|")
	parts := make([]string, 0, len(cells))
	for range cells {
		parts = append(parts, "---")
	}
	return "| " + strings.Join(parts, " | ") + " |"
}

func isKnowledgeMarkdownListLine(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if strings.HasPrefix(line, "- [ ] ") || strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") {
		return true
	}
	return knowledgeMarkdownOrderedListPattern.MatchString(line)
}

func appendKnowledgeBlankLine(lines *[]string) {
	if len(*lines) == 0 || (*lines)[len(*lines)-1] == "" {
		return
	}
	*lines = append(*lines, "")
}

func appendKnowledgeNonBlankLine(lines *[]string, line string) {
	line = strings.TrimRight(line, " \t")
	if strings.TrimSpace(line) == "" {
		appendKnowledgeBlankLine(lines)
		return
	}
	*lines = append(*lines, line)
}

func compactKnowledgeMarkdown(lines []string) string {
	result := make([]string, 0, len(lines)*2)
	for i, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			appendKnowledgeBlankLine(&result)
			continue
		}
		if strings.HasPrefix(line, "## ") && len(result) > 0 {
			appendKnowledgeBlankLine(&result)
		}
		appendKnowledgeNonBlankLine(&result, line)
		if strings.HasPrefix(line, "| ") && i+1 < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "| ") {
			appendKnowledgeBlankLine(&result)
		}
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

func buildKnowledgeChunks(markdown string) []aiopsmodel.KnowledgeChunk {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return nil
	}
	matches := markdownHeadingPattern.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return []aiopsmodel.KnowledgeChunk{newKnowledgeChunk(1, "全文", markdown)}
	}
	chunks := make([]aiopsmodel.KnowledgeChunk, 0, len(matches))
	for i, match := range matches {
		start := match[0]
		end := len(markdown)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		heading := strings.TrimSpace(markdown[match[4]:match[5]])
		content := strings.TrimSpace(markdown[start:end])
		if content != "" {
			chunks = append(chunks, newKnowledgeChunk(len(chunks)+1, heading, content))
		}
	}
	return chunks
}

func newKnowledgeChunk(index int, heading, content string) aiopsmodel.KnowledgeChunk {
	sum := sha256.Sum256([]byte(content))
	return aiopsmodel.KnowledgeChunk{
		ChunkIndex:   index,
		Heading:      truncateKnowledgeText(heading, 240),
		SectionPath:  truncateKnowledgeText(heading, 1000),
		Content:      content,
		ContentHash:  hex.EncodeToString(sum[:]),
		KeywordsJSON: encodeKnowledgeStringSlice(extractKnowledgeKeywords(content, 12)),
	}
}

func buildRuleKnowledgeSummary(doc *aiopsmodel.KnowledgeDocument) *aiopsmodel.KnowledgeSummaryResult {
	content := doc.ContentMarkdown
	return &aiopsmodel.KnowledgeSummaryResult{
		DocumentID:      doc.ID,
		Summary:         firstKnowledgeSectionLine(content, "根本原因", doc.Title),
		Symptoms:        extractKnowledgeSectionBullets(content, "事件经过", 5),
		RootCauses:      extractKnowledgeSectionBullets(content, "根本原因", 5),
		ResolutionSteps: extractKnowledgeSectionBullets(content, "解决方案", 8),
		Verification:    extractKnowledgeSectionBullets(content, "验证方式", 5),
		Keywords:        extractKnowledgeKeywords(content, 12),
		UsedLLM:         false,
	}
}

func buildKnowledgeSummaryPrompt(doc *aiopsmodel.KnowledgeDocument) string {
	payload := map[string]interface{}{
		"title":           doc.Title,
		"docType":         doc.DocType,
		"serviceName":     doc.ServiceName,
		"contentMarkdown": doc.ContentMarkdown,
	}
	data, _ := json.Marshal(payload)
	return "你是 AutoOps 运维知识库助手。请阅读 Markdown 故障文档，输出严格 JSON，不要输出解释文字。字段：summary 字符串，symptoms 数组，rootCauses 数组，resolutionSteps 数组，verification 数组，keywords 数组。要求总结可用于后续 AI 运维智能体检索和引用。\n\nJSON 输入：\n" + string(data)
}

func parseKnowledgeSummary(answer string) (*aiopsmodel.KnowledgeSummaryResult, error) {
	answer = strings.TrimSpace(answer)
	answer = strings.TrimPrefix(answer, "```json")
	answer = strings.TrimPrefix(answer, "```")
	answer = strings.TrimSuffix(answer, "```")
	var parsed aiopsmodel.KnowledgeSummaryResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func scoreKnowledgeDocument(doc aiopsmodel.KnowledgeDocument, query string) aiopsmodel.KnowledgeSearchItem {
	query = strings.ToLower(strings.TrimSpace(query))
	terms := tokenizeKnowledgeQuery(query, 16)
	if len(terms) == 0 && query != "" {
		terms = []string{query}
	}
	fields := map[string]string{
		"title":           doc.Title,
		"summary":         doc.Summary,
		"groupName":       doc.GroupName,
		"category":        doc.Category,
		"serviceName":     doc.ServiceName,
		"tags":            doc.TagsJSON,
		"keywords":        doc.KeywordsJSON,
		"alertNames":      doc.AlertNamesJSON,
		"logKeywords":     doc.LogKeywordsJSON,
		"contentMarkdown": doc.ContentMarkdown,
	}
	weights := map[string]float64{
		"title": 6, "summary": 5, "groupName": 4, "category": 4, "serviceName": 4, "tags": 4, "keywords": 4, "alertNames": 4, "logKeywords": 4, "contentMarkdown": 1,
	}
	score := 0.0
	matched := []string{}
	for field, value := range fields {
		lowerValue := strings.ToLower(value)
		matchedTerms := 0
		for _, term := range terms {
			if strings.Contains(lowerValue, term) {
				matchedTerms++
			}
		}
		if matchedTerms > 0 {
			coverage := float64(matchedTerms) / float64(len(terms))
			score += weights[field] * (0.5 + coverage)
			if query != "" && strings.Contains(lowerValue, query) {
				score += weights[field]
			}
			matched = append(matched, field)
		}
	}
	if score == 0 {
		score = 0.1
	}
	return aiopsmodel.KnowledgeSearchItem{
		ID:            doc.ID,
		Title:         doc.Title,
		DocType:       doc.DocType,
		Status:        doc.Status,
		GroupID:       doc.GroupID,
		GroupPath:     doc.GroupPath,
		GroupName:     doc.GroupName,
		Category:      doc.Category,
		ServiceName:   doc.ServiceName,
		Env:           doc.Env,
		Summary:       doc.Summary,
		MatchedFields: matched,
		Score:         score,
		Snippet:       buildKnowledgeSnippet(doc.ContentMarkdown, query),
		UpdatedAt:     doc.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func tokenizeKnowledgeQuery(query string, limit int) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	stopWords := map[string]struct{}{
		"怎么": {}, "如何": {}, "什么": {}, "是否": {}, "一下": {}, "帮我": {}, "请问": {}, "可以": {}, "应该": {}, "需要": {},
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, 16)
	appendTerm := func(term string) bool {
		term = strings.TrimSpace(term)
		if term == "" || utf8.RuneCountInString(term) < 2 {
			return false
		}
		if _, stopped := stopWords[term]; stopped {
			return false
		}
		if _, exists := seen[term]; exists {
			return false
		}
		seen[term] = struct{}{}
		result = append(result, term)
		return limit > 0 && len(result) >= limit
	}

	flush := func(segment []rune, isHan bool) bool {
		if len(segment) == 0 {
			return false
		}
		if !isHan || len(segment) <= 4 {
			return appendTerm(string(segment))
		}
		for _, size := range []int{4, 2, 3} {
			for start := 0; start+size <= len(segment); start++ {
				if appendTerm(string(segment[start : start+size])) {
					return true
				}
			}
		}
		return false
	}

	segment := make([]rune, 0, len(query))
	segmentIsHan := false
	hasSegment := false
	for _, r := range []rune(query) {
		isHan := unicode.Is(unicode.Han, r)
		isWord := isHan || unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.'
		if !isWord || hasSegment && isHan != segmentIsHan {
			if flush(segment, segmentIsHan) {
				return result
			}
			segment = segment[:0]
			hasSegment = false
		}
		if isWord {
			segment = append(segment, r)
			segmentIsHan = isHan
			hasSegment = true
		}
	}
	flush(segment, segmentIsHan)
	return result
}

func buildKnowledgeDocumentVO(doc *aiopsmodel.KnowledgeDocument) *aiopsmodel.KnowledgeDocumentVO {
	if doc == nil {
		return nil
	}
	return &aiopsmodel.KnowledgeDocumentVO{
		ID:              doc.ID,
		Title:           doc.Title,
		DocType:         doc.DocType,
		Status:          doc.Status,
		GroupID:         doc.GroupID,
		GroupPath:       doc.GroupPath,
		GroupName:       doc.GroupName,
		Category:        doc.Category,
		Severity:        doc.Severity,
		ServiceName:     doc.ServiceName,
		Env:             doc.Env,
		Tags:            decodeKnowledgeStringSlice(doc.TagsJSON),
		Keywords:        decodeKnowledgeStringSlice(doc.KeywordsJSON),
		AlertNames:      decodeKnowledgeStringSlice(doc.AlertNamesJSON),
		LogKeywords:     decodeKnowledgeStringSlice(doc.LogKeywordsJSON),
		Owners:          decodeKnowledgeStringSlice(doc.OwnersJSON),
		ContentMarkdown: doc.ContentMarkdown,
		Summary:         doc.Summary,
		SummaryStatus:   buildKnowledgeSummaryStatus(doc),
		SummaryError:    doc.SummaryError,
		SearchStatus:    normalizeKnowledgeSearchStatus(doc.SearchStatus),
		SearchError:     doc.SearchError,
		SearchVersion:   doc.SearchVersion,
		ChunkCount:      doc.ChunkCount,
		Symptoms:        decodeKnowledgeStringSlice(doc.SymptomsJSON),
		RootCauses:      decodeKnowledgeStringSlice(doc.RootCausesJSON),
		ResolutionSteps: decodeKnowledgeStringSlice(doc.ResolutionStepsJSON),
		Verification:    decodeKnowledgeStringSlice(doc.VerificationJSON),
		RollbackPlan:    doc.RollbackPlan,
		SourceFilename:  doc.SourceFilename,
		CreatedBy:       doc.CreatedBy,
		UpdatedBy:       doc.UpdatedBy,
		CreatedAt:       doc.CreatedAt,
		UpdatedAt:       doc.UpdatedAt,
	}
}

func buildKnowledgeDocumentVOs(docs []aiopsmodel.KnowledgeDocument) []aiopsmodel.KnowledgeDocumentVO {
	result := make([]aiopsmodel.KnowledgeDocumentVO, 0, len(docs))
	for i := range docs {
		if vo := buildKnowledgeDocumentVO(&docs[i]); vo != nil {
			result = append(result, *vo)
		}
	}
	return result
}

func buildKnowledgeSummaryStatus(doc *aiopsmodel.KnowledgeDocument) string {
	if doc != nil && strings.TrimSpace(doc.SummaryStatus) != "" {
		return doc.SummaryStatus
	}
	if doc == nil || strings.TrimSpace(doc.Summary) == "" {
		return "pending"
	}
	return "completed"
}

func normalizeKnowledgeSearchStatus(value string) string {
	if strings.TrimSpace(value) == aiopsmodel.KnowledgeSearchFailed {
		return aiopsmodel.KnowledgeSearchFailed
	}
	return aiopsmodel.KnowledgeSearchReady
}

func normalizeKnowledgeDocType(value string) string {
	switch strings.TrimSpace(value) {
	case aiopsmodel.KnowledgeDocTypeRunbook, aiopsmodel.KnowledgeDocTypeFAQ, aiopsmodel.KnowledgeDocTypeNote:
		return strings.TrimSpace(value)
	default:
		return aiopsmodel.KnowledgeDocTypeIncident
	}
}

func normalizeKnowledgeStatus(value string) string {
	switch strings.TrimSpace(value) {
	case aiopsmodel.KnowledgeStatusPublished, aiopsmodel.KnowledgeStatusDisabled:
		return strings.TrimSpace(value)
	default:
		return aiopsmodel.KnowledgeStatusDraft
	}
}

func normalizeKnowledgeEnv(value string) string {
	if env := strings.TrimSpace(value); env != "" {
		return env
	}
	return "prod"
}

func encodeKnowledgeStringSlice(items []string) string {
	clean := cleanKnowledgeStringSlice(items)
	data, _ := json.Marshal(clean)
	return string(data)
}

func decodeKnowledgeStringSlice(value string) []string {
	var items []string
	_ = json.Unmarshal([]byte(value), &items)
	return cleanKnowledgeStringSlice(items)
}

func cleanKnowledgeStringSlice(items []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mergeKnowledgeStringSlices(items ...[]string) []string {
	result := []string{}
	for _, list := range items {
		result = append(result, list...)
	}
	return cleanKnowledgeStringSlice(result)
}

func extractKnowledgeKeywords(content string, limit int) []string {
	candidates := []string{"mysql", "主从", "主从延迟", "proxysql", "sql", "ddl", "nginx", "502", "redis", "cpu", "内存", "磁盘", "告警", "日志", "电子合同", "成交单"}
	result := []string{}
	lower := strings.ToLower(content)
	for _, item := range candidates {
		if strings.Contains(lower, strings.ToLower(item)) {
			result = append(result, item)
		}
	}
	if limit > 0 && len(result) > limit {
		return result[:limit]
	}
	return result
}

func firstKnowledgeSectionLine(content, heading, fallback string) string {
	items := extractKnowledgeSectionBullets(content, heading, 1)
	if len(items) > 0 {
		return items[0]
	}
	return fallback
}

func extractKnowledgeSectionBullets(content, heading string, limit int) []string {
	section := extractKnowledgeSection(content, heading)
	if section == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	result := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "-*0123456789）、. ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result = append(result, truncateKnowledgeText(line, 240))
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func extractKnowledgeSection(content, heading string) string {
	matches := markdownHeadingPattern.FindAllStringSubmatchIndex(content, -1)
	for i, match := range matches {
		title := strings.TrimSpace(content[match[4]:match[5]])
		if !strings.Contains(title, heading) {
			continue
		}
		start := match[1]
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		return strings.TrimSpace(content[start:end])
	}
	return ""
}

func buildKnowledgeSnippet(content, query string) string {
	if query == "" {
		return truncateKnowledgeText(content, 300)
	}
	lower := strings.ToLower(content)
	needle := strings.ToLower(query)
	idx := strings.Index(lower, needle)
	if idx < 0 {
		for _, term := range tokenizeKnowledgeQuery(query, 16) {
			if termIndex := strings.Index(lower, term); termIndex >= 0 {
				idx = termIndex
				needle = term
				break
			}
		}
	}
	if idx < 0 {
		return truncateKnowledgeText(content, 300)
	}
	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + 180
	if end > len(content) {
		end = len(content)
	}
	return strings.TrimSpace(content[start:end])
}

func truncateKnowledgeText(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len([]rune(value)) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max]) + "..."
}
