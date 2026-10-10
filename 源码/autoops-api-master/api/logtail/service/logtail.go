package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	logtaildao "dodevops-api/api/logtail/dao"
	logtailmodel "dodevops-api/api/logtail/model"
	"dodevops-api/common/config"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

var logtailCodeInvalidChars = regexp.MustCompile(`[^a-z0-9_]+`)
var logtailNamedCapturePattern = regexp.MustCompile(`\(\?P<([A-Za-z_][A-Za-z0-9_]*)>`)

const (
	legacyDefaultTextMultilinePattern   = `^(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}([.,]\d+)?|\[\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2})`
	previousDefaultTextMultilinePattern = `^(\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}([.,]\d+)?|\[\d{4}[-/]\d{2}[-/]\d{2}[ T]\d{2}:\d{2}:\d{2}([.,]\d+)?\]?)`
	defaultTextMultilinePattern         = `^(?:\[?(?:\d{4}[-/.]\d{2}[-/.]\d{2}|\d{2}[-/]\d{2})[ T]\d{2}:\d{2}:\d{2}(?:[.,]\d+)?(?:Z|[+-]\d{2}:?\d{2})?\]?|[A-Z][a-z]{2}[ ]+\d{1,2}[ T]\d{2}:\d{2}:\d{2}|\[\d{1,2}/[A-Z][a-z]{2}/\d{4}:\d{2}:\d{2}:\d{2}(?: [+-]\d{4})?\]|\[[A-Z][a-z]{2} [A-Z][a-z]{2}[ ]+\d{1,2} \d{2}:\d{2}:\d{2}(?:\.\d+)? \d{4}\]|<\d{1,3}>\d?[ ]+\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})`
	defaultSlowLogMultilinePattern      = `^# Time: `
)

var ErrCollectTaskDeleteRequiresStopped = errors.New("采集任务正在采集中，请先停止采集后再删除")
var ErrCollectTaskStopUnsupported = errors.New("当前接口仅支持停止主机日志采集任务，K8S 采集请使用 K8S Collector 卸载/更新接口")

type CollectTaskConflictError struct {
	Kind         string
	ExistingID   uint
	ExistingName string
}

func (e *CollectTaskConflictError) Error() string {
	if e == nil {
		return "采集任务已存在"
	}
	name := strings.TrimSpace(e.ExistingName)
	if name == "" {
		name = "已有任务"
	}
	if e.ExistingID == 0 {
		if e.Kind == "name" {
			return "同一项目下已存在同名采集任务，请修改任务名称"
		}
		return "已存在相同采集配置的任务，请勿重复创建"
	}
	if e.Kind == "name" {
		return fmt.Sprintf("同一项目下已存在同名采集任务「%s」（ID: %d），请修改任务名称", name, e.ExistingID)
	}
	return fmt.Sprintf("已存在相同采集配置的任务「%s」（ID: %d），请勿重复创建", name, e.ExistingID)
}

type LogtailService struct {
	projectDAO    *logtaildao.LogProjectDAO
	storeDAO      *logtaildao.LogStoreDAO
	templateDAO   *logtaildao.LogParseTemplateDAO
	taskDAO       *logtaildao.LogCollectTaskDAO
	ruleDAO       *logtaildao.LogAlertRuleDAO
	alertEventDAO *logtaildao.LogAlertEventDAO
	agentDAO      *logtaildao.LogAgentDAO
	operationDAO  *logtaildao.LogAgentOperationDAO
	k8sOpDAO      *logtaildao.LogK8SCollectorOperationDAO
	k8sClusterDAO *logtaildao.LogK8SClusterCollectorOperationDAO
}

func NewLogtailService() *LogtailService {
	return &LogtailService{
		projectDAO:    logtaildao.NewLogProjectDAO(),
		storeDAO:      logtaildao.NewLogStoreDAO(),
		templateDAO:   logtaildao.NewLogParseTemplateDAO(),
		taskDAO:       logtaildao.NewLogCollectTaskDAO(),
		ruleDAO:       logtaildao.NewLogAlertRuleDAO(),
		alertEventDAO: logtaildao.NewLogAlertEventDAO(),
		agentDAO:      logtaildao.NewLogAgentDAO(),
		operationDAO:  logtaildao.NewLogAgentOperationDAO(),
		k8sOpDAO:      logtaildao.NewLogK8SCollectorOperationDAO(),
		k8sClusterDAO: logtaildao.NewLogK8SClusterCollectorOperationDAO(),
	}
}

func (s *LogtailService) VictoriaLogsHealth() (map[string]interface{}, error) {
	baseURL := strings.TrimSpace(config.Config.Logtail.VictoriaLogs.URL)
	if baseURL == "" {
		return nil, errors.New("VictoriaLogs URL 未配置")
	}
	timeout := time.Duration(config.Config.Logtail.VictoriaLogs.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}
	start := time.Now()
	resp, err := client.Get(baseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return map[string]interface{}{
		"url":            baseURL,
		"statusCode":     resp.StatusCode,
		"statusText":     resp.Status,
		"latencyMs":      time.Since(start).Milliseconds(),
		"healthy":        resp.StatusCode >= 200 && resp.StatusCode < 500,
		"checkedAt":      time.Now().Format(time.RFC3339),
		"timeoutSeconds": config.Config.Logtail.VictoriaLogs.TimeoutSeconds,
	}, nil
}

func (s *LogtailService) CreateProject(item *logtailmodel.LogProject) error {
	ensureProjectCode(item)
	if err := validateProject(item); err != nil {
		return err
	}
	return s.projectDAO.Create(item)
}

func (s *LogtailService) UpdateProject(item *logtailmodel.LogProject) error {
	if item.ID == 0 {
		return errors.New("项目ID不能为空")
	}
	existing, err := s.projectDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.Code) == "" {
		item.Code = existing.Code
	}
	if err := validateProject(item); err != nil {
		return err
	}
	existing.Name = item.Name
	existing.Code = item.Code
	existing.Description = item.Description
	existing.OwnerType = item.OwnerType
	existing.OwnerID = item.OwnerID
	existing.OwnerName = item.OwnerName
	existing.Env = item.Env
	existing.Status = item.Status
	existing.ExtraJSON = item.ExtraJSON
	existing.UpdatedBy = item.UpdatedBy
	return s.projectDAO.Update(existing)
}

func (s *LogtailService) DeleteProject(id uint) error {
	if id == 0 {
		return errors.New("项目ID不能为空")
	}
	return s.projectDAO.Delete(id)
}

func (s *LogtailService) GetProject(id uint) (*logtailmodel.LogProject, error) {
	if id == 0 {
		return nil, errors.New("项目ID不能为空")
	}
	return s.projectDAO.GetByID(id)
}

func isLogtailRootAdmin(c *gin.Context) bool {
	if c == nil {
		return true
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return false
	}
	return admin.ID == 1 || admin.Username == "admin" || admin.Username == "root"
}

func (s *LogtailService) authorizedLogProjectIDs(c *gin.Context) ([]uint, error) {
	if isLogtailRootAdmin(c) {
		return nil, nil
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return []uint{}, nil
	}
	authDao := cmdbdao.NewBastionAssetAuthorizationDao()
	return authDao.GetUserAuthorizedAssetIDs(admin.ID, cmdbmodel.AssetTypeLogProject)
}

func (s *LogtailService) authorizedLogStoreIDs(c *gin.Context, projectIDs []uint) ([]uint, error) {
	if isLogtailRootAdmin(c) {
		return nil, nil
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return []uint{}, nil
	}
	authDao := cmdbdao.NewBastionAssetAuthorizationDao()
	allowedProjectIDs, err := authDao.GetUserAuthorizedAssetIDs(admin.ID, cmdbmodel.AssetTypeLogProject)
	if err != nil {
		return nil, err
	}
	if len(projectIDs) > 0 {
		filter := map[uint]struct{}{}
		for _, id := range projectIDs {
			filter[id] = struct{}{}
		}
		kept := make([]uint, 0, len(allowedProjectIDs))
		for _, id := range allowedProjectIDs {
			if _, ok := filter[id]; ok {
				kept = append(kept, id)
			}
		}
		allowedProjectIDs = kept
	}
	if len(allowedProjectIDs) == 0 {
		return []uint{}, nil
	}

	ids := make([]uint, 0)
	seen := map[uint]struct{}{}
	addID := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	allScopeProjectIDs := make([]uint, 0, len(allowedProjectIDs))
	for _, projectID := range allowedProjectIDs {
		hasAll, err := authDao.CheckUserAssetPermission(admin.ID, cmdbmodel.AssetTypeLogProject, projectID, cmdbmodel.AssetScopeAll, "")
		if err != nil {
			return nil, err
		}
		if hasAll {
			allScopeProjectIDs = append(allScopeProjectIDs, projectID)
		}
	}
	allScopeStoreIDs, err := s.storeDAO.ListIDsByProjectIDs(allScopeProjectIDs)
	if err != nil {
		return nil, err
	}
	for _, storeID := range allScopeStoreIDs {
		addID(storeID)
	}

	scoped, err := authDao.GetUserAuthorizedScopeValuesByAssetIDs(admin.ID, cmdbmodel.AssetTypeLogProject, allowedProjectIDs, cmdbmodel.AssetScopeLogStore)
	if err != nil {
		return nil, err
	}
	for _, values := range scoped {
		for _, value := range values {
			addID(parseUintString(value))
		}
	}
	return ids, nil
}

func parseUintString(value string) uint {
	id, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return uint(id)
}

func (s *LogtailService) ensureLogStoreQueryPermission(c *gin.Context, store *logtailmodel.LogStore) error {
	if isLogtailRootAdmin(c) {
		return nil
	}
	if store == nil || store.ID == 0 || store.ProjectID == 0 {
		return errors.New("非管理员查询日志必须传入已授权的日志库ID")
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return errors.New("用户未登录")
	}
	authDao := cmdbdao.NewBastionAssetAuthorizationDao()
	allowed, err := authDao.CheckUserAssetPermission(
		admin.ID,
		cmdbmodel.AssetTypeLogProject,
		store.ProjectID,
		cmdbmodel.AssetScopeLogStore,
		strconv.FormatUint(uint64(store.ID), 10),
	)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("无日志库访问权限")
	}
	return nil
}

func (s *LogtailService) ListProjects(c *gin.Context, query logtailmodel.LogProjectQuery) ([]logtailmodel.LogProject, int64, logtailmodel.LogProjectQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	var items []logtailmodel.LogProject
	var total int64
	var err error
	if isLogtailRootAdmin(c) {
		items, total, err = s.projectDAO.List(query)
	} else {
		projectIDs, authErr := s.authorizedLogProjectIDs(c)
		if authErr != nil {
			return nil, 0, query, authErr
		}
		items, total, err = s.projectDAO.ListByIDs(query, projectIDs)
	}
	if err != nil {
		return items, total, query, err
	}
	if err := s.fillProjectStoreCounts(items); err != nil {
		return items, total, query, err
	}
	return items, total, query, err
}

func (s *LogtailService) ListProjectStoreCounts(c *gin.Context, projectIDs []uint) ([]logtailmodel.LogProjectStoreCount, error) {
	if !isLogtailRootAdmin(c) {
		authorizedIDs, err := s.authorizedLogProjectIDs(c)
		if err != nil {
			return nil, err
		}
		if len(projectIDs) > 0 {
			filter := map[uint]struct{}{}
			for _, id := range projectIDs {
				filter[id] = struct{}{}
			}
			kept := make([]uint, 0, len(authorizedIDs))
			for _, id := range authorizedIDs {
				if _, ok := filter[id]; ok {
					kept = append(kept, id)
				}
			}
			authorizedIDs = kept
		}
		projectIDs = authorizedIDs
		if len(projectIDs) == 0 {
			return []logtailmodel.LogProjectStoreCount{}, nil
		}
	}
	return s.storeDAO.CountByProject(projectIDs)
}

func (s *LogtailService) CreateStore(item *logtailmodel.LogStore) error {
	ensureStoreCode(item)
	if err := validateStore(item); err != nil {
		return err
	}
	return s.storeDAO.Create(item)
}

func (s *LogtailService) fillProjectStoreCounts(items []logtailmodel.LogProject) error {
	if len(items) == 0 {
		return nil
	}
	projectIDs := make([]uint, 0, len(items))
	indexByID := make(map[uint]int, len(items))
	for i := range items {
		projectIDs = append(projectIDs, items[i].ID)
		indexByID[items[i].ID] = i
	}
	counts, err := s.storeDAO.CountByProject(projectIDs)
	if err != nil {
		return err
	}
	for _, item := range counts {
		idx, ok := indexByID[item.ProjectID]
		if !ok {
			continue
		}
		items[idx].StoreCount = item.StoreCount
		items[idx].EnabledStoreCount = item.EnabledStoreCount
	}
	return nil
}

func (s *LogtailService) UpdateStore(item *logtailmodel.LogStore) error {
	if item.ID == 0 {
		return errors.New("日志库ID不能为空")
	}
	existing, err := s.storeDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(item.Code) == "" {
		item.Code = existing.Code
	}
	if err := validateStore(item); err != nil {
		return err
	}
	existing.ProjectID = item.ProjectID
	existing.Name = item.Name
	existing.Code = item.Code
	existing.Description = item.Description
	existing.StoreType = item.StoreType
	existing.RetentionDays = item.RetentionDays
	existing.Status = item.Status
	existing.Sort = item.Sort
	existing.VlogsTenant = item.VlogsTenant
	existing.ExtraJSON = item.ExtraJSON
	existing.UpdatedBy = item.UpdatedBy
	return s.storeDAO.Update(existing)
}

func (s *LogtailService) DeleteStore(id uint) error {
	if id == 0 {
		return errors.New("日志库ID不能为空")
	}
	return s.storeDAO.Delete(id)
}

func (s *LogtailService) GetStore(id uint) (*logtailmodel.LogStore, error) {
	if id == 0 {
		return nil, errors.New("日志库ID不能为空")
	}
	return s.storeDAO.GetByID(id)
}

func (s *LogtailService) ListStores(c *gin.Context, query logtailmodel.LogStoreQuery) ([]logtailmodel.LogStore, int64, logtailmodel.LogStoreQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	var items []logtailmodel.LogStore
	var total int64
	var err error
	if isLogtailRootAdmin(c) {
		items, total, err = s.storeDAO.List(query)
	} else {
		var projectFilter []uint
		if query.ProjectID > 0 {
			projectFilter = []uint{query.ProjectID}
		}
		storeIDs, authErr := s.authorizedLogStoreIDs(c, projectFilter)
		if authErr != nil {
			return nil, 0, query, authErr
		}
		items, total, err = s.storeDAO.ListByIDs(query, storeIDs)
	}
	return items, total, query, err
}

func (s *LogtailService) CreateParseTemplate(item *logtailmodel.LogParseTemplate) error {
	if err := validateTemplate(item); err != nil {
		return err
	}
	return s.templateDAO.Create(item)
}

func (s *LogtailService) UpdateParseTemplate(item *logtailmodel.LogParseTemplate) error {
	if item.ID == 0 {
		return errors.New("解析模板ID不能为空")
	}
	if err := validateTemplate(item); err != nil {
		return err
	}
	existing, err := s.templateDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	existing.ProjectID = item.ProjectID
	existing.Name = item.Name
	existing.Code = item.Code
	existing.ParseMode = item.ParseMode
	existing.InputFormat = item.InputFormat
	existing.MultilineEnabled = item.MultilineEnabled
	existing.MultilinePattern = item.MultilinePattern
	existing.TimeKey = item.TimeKey
	existing.TimeFormat = item.TimeFormat
	existing.LevelKey = item.LevelKey
	existing.MessageKey = item.MessageKey
	existing.TraceIDKey = item.TraceIDKey
	existing.SpanIDKey = item.SpanIDKey
	existing.FieldsMappingJSON = item.FieldsMappingJSON
	existing.LabelsMappingJSON = item.LabelsMappingJSON
	existing.VRLScript = item.VRLScript
	existing.Status = item.Status
	existing.Version = item.Version
	existing.Remark = item.Remark
	existing.UpdatedBy = item.UpdatedBy
	return s.templateDAO.Update(existing)
}

func (s *LogtailService) DeleteParseTemplate(id uint) error {
	if id == 0 {
		return errors.New("解析模板ID不能为空")
	}
	return s.templateDAO.Delete(id)
}

func (s *LogtailService) GetParseTemplate(id uint) (*logtailmodel.LogParseTemplate, error) {
	if id == 0 {
		return nil, errors.New("解析模板ID不能为空")
	}
	return s.templateDAO.GetByID(id)
}

func (s *LogtailService) ListParseTemplates(query logtailmodel.LogParseTemplateQuery) ([]logtailmodel.LogParseTemplate, int64, logtailmodel.LogParseTemplateQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.templateDAO.List(query)
	return items, total, query, err
}

func (s *LogtailService) CreateCollectTask(item *logtailmodel.LogCollectTask) error {
	normalizeCollectTaskForCreate(item)
	if err := s.applyCollectTaskMultilineSettings(item); err != nil {
		return err
	}
	if err := validateCollectTask(item); err != nil {
		return err
	}
	setCollectTaskDedupKeys(item)
	if err := s.ensureCollectTaskUnique(item); err != nil {
		return err
	}
	if err := s.taskDAO.Create(item); err != nil {
		return s.resolveCollectTaskWriteError(item, err)
	}
	s.autoApplyCollectTask(item)
	return nil
}

func (s *LogtailService) autoApplyCollectTask(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}

	item.AutoApplyTriggered = false
	item.AutoApplyStatus = ""
	item.AutoApplyMessage = ""
	item.AutoApplySummary = nil

	if ok, message := shouldAutoApplyCollectTask(item); !ok {
		item.AutoApplyStatus = "skipped"
		item.AutoApplyMessage = message
		return
	}

	item.AutoApplyTriggered = true
	resp, err := s.ApplyCollectTask(item.ID, logtailmodel.LogCollectTaskApplyRequest{
		Operator: firstNonEmpty(item.UpdatedBy, item.CreatedBy, "admin"),
		Force:    true,
	})
	if err != nil {
		item.AutoApplyStatus = "failed"
		item.AutoApplyMessage = "自动下发失败: " + err.Error()
		return
	}

	summary := resp.Summary
	item.AutoApplySummary = &summary
	item.AutoApplyStatus = collectTaskAutoApplyStatus(summary)
	item.AutoApplyMessage = collectTaskAutoApplyMessage(summary)
}

func shouldAutoApplyCollectTask(item *logtailmodel.LogCollectTask) (bool, string) {
	if item == nil {
		return false, "采集任务不能为空"
	}
	if item.Status != logtailmodel.LogStatusEnabled {
		return false, disabledCollectTaskConfigMessage()
	}

	switch strings.ToLower(strings.TrimSpace(item.CollectType)) {
	case "host", "logtail_host":
		if strings.EqualFold(strings.TrimSpace(item.ExecuteMode), "all_agents") {
			return false, "主机日志采集不再支持全部 Agent 自动下发，请选择具体 Agent"
		}
		if !collectTaskHasExplicitTargets(item) {
			return false, "主机日志采集未选择执行 Agent，跳过自动下发"
		}
		return true, ""
	case "k8s", logtailmodel.LogCollectTypeKubernetes, "kubernetes_workload", "k8s_workload":
		if strings.TrimSpace(item.ExecuteTargetsJSON) == "" {
			return false, "K8S日志采集未选择目标，跳过自动下发"
		}
		return true, ""
	case "":
		return false, "采集类型不能为空，跳过自动下发"
	default:
		return false, "当前采集类型暂不支持自动下发"
	}
}

func collectTaskAutoApplyStatus(summary logtailmodel.LogCollectTaskApplySummary) string {
	created := summary.InstallCreated + summary.ReloadCreated

	switch {
	case summary.MatchedAgents == 0:
		return "skipped"
	case summary.Failed > 0 && created == 0 && summary.Skipped == 0:
		return "failed"
	case summary.Failed > 0:
		return "partial_failed"
	case created > 0:
		return "created"
	default:
		return "skipped"
	}
}

func collectTaskAutoApplyMessage(summary logtailmodel.LogCollectTaskApplySummary) string {
	created := summary.InstallCreated + summary.ReloadCreated

	switch {
	case summary.MatchedAgents == 0:
		return "未匹配到可自动下发的 Agent"
	case summary.Failed > 0 && created == 0 && summary.Skipped == 0:
		return fmt.Sprintf("自动下发失败，失败 %d 台 Agent", summary.Failed)
	case summary.Failed > 0:
		return fmt.Sprintf(
			"已部分自动下发，安装 %d，重载 %d，跳过 %d，失败 %d",
			summary.InstallCreated,
			summary.ReloadCreated,
			summary.Skipped,
			summary.Failed,
		)
	case created > 0:
		return fmt.Sprintf(
			"已自动下发，安装 %d，重载 %d，跳过 %d",
			summary.InstallCreated,
			summary.ReloadCreated,
			summary.Skipped,
		)
	default:
		return "目标 Agent 已存在待执行操作或配置已生效"
	}
}

func (s *LogtailService) UpdateCollectTask(item *logtailmodel.LogCollectTask) error {
	if item.ID == 0 {
		return errors.New("采集任务ID不能为空")
	}
	existing, err := s.taskDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	normalizeCollectTaskForUpdate(existing, item)
	if err := s.applyCollectTaskMultilineSettings(item); err != nil {
		return err
	}
	item.ConfigVersion = nextCollectTaskConfigVersion(existing, item)
	if err := validateCollectTask(item); err != nil {
		return err
	}
	setCollectTaskDedupKeys(item)
	if err := s.ensureCollectTaskUnique(item); err != nil {
		return err
	}
	existing.ProjectID = item.ProjectID
	existing.StoreID = item.StoreID
	existing.Name = item.Name
	existing.Code = item.Code
	existing.NameKey = item.NameKey
	existing.ConfigKey = item.ConfigKey
	existing.CollectType = item.CollectType
	existing.ExecuteMode = item.ExecuteMode
	existing.ExecuteTargetsJSON = item.ExecuteTargetsJSON
	existing.PathPattern = item.PathPattern
	existing.PathPatternsJSON = item.PathPatternsJSON
	existing.K8SLogSource = item.K8SLogSource
	existing.ParseTemplateID = item.ParseTemplateID
	existing.ParseModeSnapshot = item.ParseModeSnapshot
	existing.SampleLog = item.SampleLog
	existing.RegexPattern = item.RegexPattern
	existing.MultilineEnabledSnapshot = item.MultilineEnabledSnapshot
	existing.MultilinePatternSnapshot = item.MultilinePatternSnapshot
	existing.LabelsJSON = item.LabelsJSON
	existing.FiltersJSON = item.FiltersJSON
	existing.ConfigVersion = item.ConfigVersion
	existing.Status = item.Status
	existing.LastError = item.LastError
	existing.Remark = item.Remark
	existing.UpdatedBy = item.UpdatedBy
	if err := s.taskDAO.Update(existing); err != nil {
		return s.resolveCollectTaskWriteError(item, err)
	}
	s.autoApplyCollectTask(item)
	return nil
}

func (s *LogtailService) DeleteCollectTask(id uint) error {
	if id == 0 {
		return errors.New("采集任务ID不能为空")
	}
	item, err := s.taskDAO.GetByID(id)
	if err != nil {
		return err
	}
	if isHostCollectType(item.CollectType) {
		agents, _, err := s.listAgentsForCollectTask(*item, logtailmodel.LogAgentQuery{Page: 1})
		if err != nil {
			return err
		}
		if err := ensureHostCollectTaskDeletable(agents); err != nil {
			return err
		}
	} else if err := ensureCollectTaskStoppedForDelete(item); err != nil {
		return err
	}
	return s.taskDAO.Delete(id)
}

func ensureCollectTaskStoppedForDelete(item *logtailmodel.LogCollectTask) error {
	if item == nil {
		return errors.New("采集任务不存在")
	}
	if item.Status != logtailmodel.LogStatusDisabled {
		return ErrCollectTaskDeleteRequiresStopped
	}
	return nil
}

func ensureHostCollectTaskDeletable(agents []logtailmodel.LogAgentVO) error {
	for _, agent := range agents {
		vectorStatus := normalizeVectorStatus(agent.VectorStatus)
		configStatus := strings.TrimSpace(agent.ConfigStatus)
		if vectorStatus == logtailmodel.VectorStatusError || configStatus == logtailmodel.LogAgentConfigStatusFailed {
			return errors.New("采集任务命中主机存在异常状态，请先处理或卸载后再删除")
		}
		if vectorStatus == logtailmodel.VectorStatusRunning && configStatus == logtailmodel.LogAgentConfigStatusApplied {
			return ErrCollectTaskDeleteRequiresStopped
		}
	}
	return nil
}

func (s *LogtailService) GetCollectTask(id uint) (*logtailmodel.LogCollectTask, error) {
	if id == 0 {
		return nil, errors.New("采集任务ID不能为空")
	}
	item, err := s.taskDAO.GetByID(id)
	if err != nil {
		return nil, err
	}
	hydrateCollectTaskPresentation(item)
	return item, nil
}

func (s *LogtailService) ListCollectTasks(query logtailmodel.LogCollectTaskQuery) ([]logtailmodel.LogCollectTask, int64, logtailmodel.LogCollectTaskQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.taskDAO.List(query)
	for i := range items {
		hydrateCollectTaskPresentation(&items[i])
	}
	return items, total, query, err
}

func (s *LogtailService) CreateAlertRule(item *logtailmodel.LogAlertRule) error {
	if err := validateAlertRule(item); err != nil {
		return err
	}
	return s.ruleDAO.Create(item)
}

func (s *LogtailService) UpdateAlertRule(item *logtailmodel.LogAlertRule) error {
	if item.ID == 0 {
		return errors.New("告警规则ID不能为空")
	}
	if err := validateAlertRule(item); err != nil {
		return err
	}
	existing, err := s.ruleDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	existing.ProjectID = item.ProjectID
	existing.StoreID = item.StoreID
	existing.Name = item.Name
	existing.Code = item.Code
	existing.RuleType = item.RuleType
	existing.Severity = item.Severity
	existing.ScanIntervalMinutes = item.ScanIntervalMinutes
	existing.WindowMinutes = item.WindowMinutes
	existing.Threshold = item.Threshold
	existing.SilenceMinutes = item.SilenceMinutes
	existing.KeywordsJSON = item.KeywordsJSON
	existing.FieldConditionsJSON = item.FieldConditionsJSON
	existing.QueryJSON = item.QueryJSON
	existing.NotifyChannelsJSON = item.NotifyChannelsJSON
	existing.LabelsJSON = item.LabelsJSON
	existing.AnnotationsJSON = item.AnnotationsJSON
	existing.Status = item.Status
	existing.LastEvalAt = item.LastEvalAt
	existing.Remark = item.Remark
	existing.UpdatedBy = item.UpdatedBy
	return s.ruleDAO.Update(existing)
}

func (s *LogtailService) DeleteAlertRule(id uint) error {
	if id == 0 {
		return errors.New("告警规则ID不能为空")
	}
	return s.ruleDAO.Delete(id)
}

func (s *LogtailService) GetAlertRule(id uint) (*logtailmodel.LogAlertRule, error) {
	if id == 0 {
		return nil, errors.New("告警规则ID不能为空")
	}
	return s.ruleDAO.GetByID(id)
}

func (s *LogtailService) ListAlertRules(query logtailmodel.LogAlertRuleQuery) ([]logtailmodel.LogAlertRule, int64, logtailmodel.LogAlertRuleQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.ruleDAO.List(query)
	return items, total, query, err
}

func (s *LogtailService) CreateAlertEvent(item *logtailmodel.LogAlertEvent) error {
	if err := validateAlertEvent(item); err != nil {
		return err
	}
	return s.alertEventDAO.Create(item)
}

func (s *LogtailService) UpdateAlertEvent(item *logtailmodel.LogAlertEvent) error {
	if item.ID == 0 {
		return errors.New("告警事件ID不能为空")
	}
	if err := validateAlertEvent(item); err != nil {
		return err
	}
	existing, err := s.alertEventDAO.GetByID(item.ID)
	if err != nil {
		return err
	}
	existing.RuleID = item.RuleID
	existing.ProjectID = item.ProjectID
	existing.StoreID = item.StoreID
	existing.SourceName = item.SourceName
	existing.Severity = item.Severity
	existing.Status = item.Status
	existing.TriggerReason = item.TriggerReason
	existing.Summary = item.Summary
	existing.MatchCount = item.MatchCount
	existing.FirstMatchTime = item.FirstMatchTime
	existing.LastMatchTime = item.LastMatchTime
	existing.FiredAt = item.FiredAt
	existing.RecoveredAt = item.RecoveredAt
	existing.SampleQueryJSON = item.SampleQueryJSON
	existing.SampleEventIDs = item.SampleEventIDs
	existing.DiagnosisReportID = item.DiagnosisReportID
	existing.NotifyStatus = item.NotifyStatus
	existing.NotifyResult = item.NotifyResult
	return s.alertEventDAO.Update(existing)
}

func (s *LogtailService) DeleteAlertEvent(id uint) error {
	if id == 0 {
		return errors.New("告警事件ID不能为空")
	}
	return s.alertEventDAO.Delete(id)
}

func (s *LogtailService) GetAlertEvent(id uint) (*logtailmodel.LogAlertEvent, error) {
	if id == 0 {
		return nil, errors.New("告警事件ID不能为空")
	}
	return s.alertEventDAO.GetByID(id)
}

func (s *LogtailService) ListAlertEvents(query logtailmodel.LogAlertEventQuery) ([]logtailmodel.LogAlertEvent, int64, logtailmodel.LogAlertEventQuery, error) {
	query.Page, query.PageSize = normalizePage(query.Page, query.PageSize)
	items, total, err := s.alertEventDAO.List(query)
	return items, total, query, err
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func validateProject(item *logtailmodel.LogProject) error {
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("项目名称不能为空")
	}
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("项目编码不能为空")
	}
	if item.Status == 0 {
		item.Status = logtailmodel.LogStatusEnabled
	}
	return nil
}

func ensureProjectCode(item *logtailmodel.LogProject) {
	if strings.TrimSpace(item.Code) != "" {
		item.Code = normalizeLogtailCode(item.Code)
		if item.Code != "" {
			return
		}
	}
	item.Code = generateLogtailCode("project", item.Name)
}

func ensureStoreCode(item *logtailmodel.LogStore) {
	if strings.TrimSpace(item.Code) != "" {
		item.Code = normalizeLogtailCode(item.Code)
		if item.Code != "" {
			return
		}
	}
	item.Code = generateLogtailCode("store", item.Name)
}

func ensureCollectTaskCode(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.Code) != "" {
		item.Code = normalizeLogtailCode(item.Code)
		if item.Code != "" {
			return
		}
	}
	item.Code = generateLogtailCode("collect", item.Name)
}

func generateLogtailCode(prefix, name string) string {
	base := normalizeLogtailCode(name)
	if base == "" {
		base = prefix
	}
	return fmt.Sprintf("%s_%s", base, randomLogtailCodeSuffix())
}

func normalizeLogtailCode(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", "_")
	raw = logtailCodeInvalidChars.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "_")
	for strings.Contains(raw, "__") {
		raw = strings.ReplaceAll(raw, "__", "_")
	}
	return raw
}

func randomLogtailCodeSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func validateStore(item *logtailmodel.LogStore) error {
	if item.ProjectID == 0 {
		return errors.New("项目ID不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("日志库名称不能为空")
	}
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("日志库编码不能为空")
	}
	if item.RetentionDays <= 0 {
		item.RetentionDays = 7
	}
	if item.Status == 0 {
		item.Status = logtailmodel.LogStatusEnabled
	}
	return nil
}

func validateTemplate(item *logtailmodel.LogParseTemplate) error {
	if item.ProjectID == 0 {
		return errors.New("项目ID不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("解析模板名称不能为空")
	}
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("解析模板编码不能为空")
	}
	if strings.TrimSpace(item.ParseMode) == "" {
		return errors.New("解析模式不能为空")
	}
	if item.Version <= 0 {
		item.Version = 1
	}
	if item.Status == 0 {
		item.Status = logtailmodel.LogStatusEnabled
	}
	return nil
}

func validateCollectTask(item *logtailmodel.LogCollectTask) error {
	if item.ProjectID == 0 {
		return errors.New("项目ID不能为空")
	}
	if item.StoreID == 0 {
		return errors.New("目标日志库不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("采集任务名称不能为空")
	}
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("采集任务编码不能为空")
	}
	if strings.TrimSpace(item.CollectType) == "" {
		return errors.New("采集类型不能为空")
	}
	if strings.TrimSpace(item.ExecuteMode) == "" {
		return errors.New("执行模式不能为空")
	}
	if isHostCollectType(item.CollectType) {
		if strings.EqualFold(strings.TrimSpace(item.ExecuteMode), "all_agents") {
			return errors.New("主机日志采集不支持全部 Agent，请选择具体 Agent")
		}
		if !collectTaskHasExplicitTargets(item) {
			return errors.New("主机日志采集必须选择执行 Agent")
		}
	}
	if isKubernetesCollectType(item.CollectType) {
		if err := validateKubernetesCollectTask(item); err != nil {
			return err
		}
	} else if len(collectTaskPathPatterns(item)) == 0 {
		return errors.New("日志路径不能为空")
	}
	mode := normalizeCollectTaskParseMode(item.ParseModeSnapshot)
	switch mode {
	case "", "raw", "json", "regex", "nginx", "nginx_json", "nginx_json_loose", "db_slow_log":
	default:
		return fmt.Errorf("不支持的日志类型: %s", item.ParseModeSnapshot)
	}
	if err := validateCollectTaskRegexPattern(item.RegexPattern, item.SampleLog); err != nil {
		return err
	}
	if err := validateCollectTaskMultilinePattern(item); err != nil {
		return err
	}
	if item.Status == 0 {
		item.Status = logtailmodel.LogStatusEnabled
	}
	return nil
}

func normalizeCollectTaskForCreate(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	ensureCollectTaskCode(item)
	normalizeCollectTaskParseSettings(item)
	normalizeCollectTaskPaths(item)
	normalizeCollectTaskExecutionSettings(item)
	if strings.TrimSpace(item.ParseModeSnapshot) == "" && item.ParseTemplateID == 0 {
		item.ParseModeSnapshot = "raw"
	}
	item.LogType = displayCollectTaskParseMode(item.ParseModeSnapshot)
	if item.ConfigVersion <= 0 {
		item.ConfigVersion = 1
	}
}

func setCollectTaskDedupKeys(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	nameKey := collectTaskNameDedupKey(item)
	configKey := collectTaskConfigDedupKey(item)
	item.NameKey = &nameKey
	item.ConfigKey = &configKey
}

func collectTaskNameDedupKey(item *logtailmodel.LogCollectTask) string {
	if item == nil {
		return ""
	}
	identity := fmt.Sprintf("project=%d\nname=%s", item.ProjectID, normalizeCollectTaskNameForDedup(item.Name))
	return collectTaskIdentityHash([]byte(identity))
}

func collectTaskConfigDedupKey(item *logtailmodel.LogCollectTask) string {
	if item == nil {
		return ""
	}
	paths := append([]string(nil), collectTaskPathPatterns(item)...)
	sort.Strings(paths)
	identity := struct {
		ProjectID                uint     `json:"projectId"`
		StoreID                  uint     `json:"storeId"`
		CollectType              string   `json:"collectType"`
		ExecuteMode              string   `json:"executeMode"`
		Targets                  []string `json:"targets"`
		Paths                    []string `json:"paths"`
		K8SLogSource             string   `json:"k8sLogSource"`
		ParseTemplateID          uint     `json:"parseTemplateId"`
		ParseModeSnapshot        string   `json:"parseModeSnapshot"`
		RegexPattern             string   `json:"regexPattern"`
		MultilineEnabledSnapshot string   `json:"multilineEnabledSnapshot"`
		MultilinePatternSnapshot string   `json:"multilinePatternSnapshot"`
		LabelsJSON               string   `json:"labelsJson"`
		FiltersJSON              string   `json:"filtersJson"`
	}{
		ProjectID:                item.ProjectID,
		StoreID:                  item.StoreID,
		CollectType:              strings.ToLower(strings.TrimSpace(item.CollectType)),
		ExecuteMode:              strings.ToLower(strings.TrimSpace(item.ExecuteMode)),
		Targets:                  collectTaskTargetSignatures(item),
		Paths:                    paths,
		K8SLogSource:             normalizeKubernetesLogSource(item.K8SLogSource),
		ParseTemplateID:          item.ParseTemplateID,
		ParseModeSnapshot:        normalizeCollectTaskParseMode(item.ParseModeSnapshot),
		RegexPattern:             strings.TrimSpace(item.RegexPattern),
		MultilineEnabledSnapshot: boolPointerValue(item.MultilineEnabledSnapshot),
		MultilinePatternSnapshot: strings.TrimSpace(item.MultilinePatternSnapshot),
		LabelsJSON:               canonicalCollectTaskJSON(item.LabelsJSON),
		FiltersJSON:              canonicalCollectTaskJSON(item.FiltersJSON),
	}
	payload, _ := json.Marshal(identity)
	return collectTaskIdentityHash(payload)
}

func normalizeCollectTaskNameForDedup(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func collectTaskTargetSignatures(item *logtailmodel.LogCollectTask) []string {
	if item == nil {
		return nil
	}
	if isKubernetesCollectType(item.CollectType) {
		targets, err := parseIndexedKubernetesCollectTargetsBySource(item.ExecuteTargetsJSON, normalizeKubernetesLogSource(item.K8SLogSource))
		if err == nil {
			result := make([]string, 0, len(targets))
			for _, indexedTarget := range targets {
				target := indexedTarget.Target
				result = append(result, fmt.Sprintf(
					"%d|%s|%s|%s",
					target.ClusterID,
					strings.ToLower(strings.TrimSpace(target.Namespace)),
					strings.ToLower(strings.TrimSpace(target.WorkloadType)),
					strings.ToLower(strings.TrimSpace(target.WorkloadName)),
				))
			}
			sort.Strings(result)
			return result
		}
	}
	if isHostCollectType(item.CollectType) {
		agentIDs, hostIDs := parseCollectTaskTargetIDs(item.ExecuteTargetsJSON)
		sort.Slice(agentIDs, func(i, j int) bool { return agentIDs[i] < agentIDs[j] })
		sort.Slice(hostIDs, func(i, j int) bool { return hostIDs[i] < hostIDs[j] })
		return []string{
			"agents:" + joinCollectTaskUintIDs(agentIDs),
			"hosts:" + joinCollectTaskUintIDs(hostIDs),
		}
	}
	return []string{canonicalCollectTaskJSON(item.ExecuteTargetsJSON)}
}

func joinCollectTaskUintIDs(values []uint) string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		items = append(items, strconv.FormatUint(uint64(value), 10))
	}
	return strings.Join(items, ",")
}

func canonicalCollectTaskJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var value interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return string(payload)
}

func collectTaskIdentityHash(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (s *LogtailService) ensureCollectTaskUnique(item *logtailmodel.LogCollectTask) error {
	if item == nil {
		return errors.New("采集任务不能为空")
	}
	candidates, err := s.taskDAO.ListForDedup(item.ProjectID, item.ID)
	if err != nil {
		return err
	}
	nameKey := collectTaskStringKey(item.NameKey)
	configKey := collectTaskStringKey(item.ConfigKey)
	for index := range candidates {
		candidate := &candidates[index]
		setCollectTaskDedupKeys(candidate)
		if nameKey != "" && collectTaskStringKey(candidate.NameKey) == nameKey {
			return &CollectTaskConflictError{Kind: "name", ExistingID: candidate.ID, ExistingName: candidate.Name}
		}
	}
	for index := range candidates {
		candidate := &candidates[index]
		if configKey != "" && collectTaskStringKey(candidate.ConfigKey) == configKey {
			return &CollectTaskConflictError{Kind: "config", ExistingID: candidate.ID, ExistingName: candidate.Name}
		}
	}
	return nil
}

func collectTaskStringKey(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (s *LogtailService) resolveCollectTaskWriteError(item *logtailmodel.LogCollectTask, writeErr error) error {
	if !isCollectTaskUniqueConstraintError(writeErr) {
		return writeErr
	}
	if conflictErr := s.ensureCollectTaskUnique(item); conflictErr != nil {
		var typedConflict *CollectTaskConflictError
		if errors.As(conflictErr, &typedConflict) {
			return typedConflict
		}
	}
	kind := "config"
	if strings.Contains(strings.ToLower(writeErr.Error()), "name_key") {
		kind = "name"
	}
	return &CollectTaskConflictError{Kind: kind}
}

func isCollectTaskUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		message = strings.ToLower(mysqlErr.Message)
	}
	return strings.Contains(message, "uk_log_collect_task_name_key") ||
		strings.Contains(message, "uk_log_collect_task_config_key")
}

func normalizeCollectTaskForUpdate(existing, item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	if existing == nil {
		normalizeCollectTaskForCreate(item)
		return
	}

	if strings.TrimSpace(item.Code) == "" {
		item.Code = existing.Code
	} else {
		item.Code = normalizeLogtailCode(item.Code)
	}
	if strings.TrimSpace(item.ExecuteTargetsJSON) == "" {
		item.ExecuteTargetsJSON = existing.ExecuteTargetsJSON
	}
	if strings.TrimSpace(item.K8SLogSource) == "" {
		item.K8SLogSource = existing.K8SLogSource
	}
	if strings.TrimSpace(item.PathPattern) == "" && strings.TrimSpace(item.PathPatternsJSON) == "" && len(item.PathPatterns) == 0 {
		item.PathPattern = existing.PathPattern
		item.PathPatternsJSON = existing.PathPatternsJSON
	}
	multilineEnabledProvided := item.MultilineEnabledSnapshot != nil
	if item.ParseTemplateID == 0 {
		item.ParseTemplateID = existing.ParseTemplateID
	}
	if !multilineEnabledProvided {
		item.MultilineEnabledSnapshot = existing.MultilineEnabledSnapshot
	}
	normalizeCollectTaskParseSettings(item)
	if strings.TrimSpace(item.SampleLog) == "" {
		item.SampleLog = existing.SampleLog
	}
	if strings.TrimSpace(item.RegexPattern) == "" {
		item.RegexPattern = existing.RegexPattern
	}
	if strings.TrimSpace(item.ParseModeSnapshot) == "" {
		item.ParseModeSnapshot = existing.ParseModeSnapshot
	}
	if !multilineEnabledProvided && strings.TrimSpace(item.MultilinePatternSnapshot) == "" {
		item.MultilinePatternSnapshot = existing.MultilinePatternSnapshot
	}
	if strings.TrimSpace(item.LabelsJSON) == "" {
		item.LabelsJSON = existing.LabelsJSON
	}
	if strings.TrimSpace(item.FiltersJSON) == "" {
		item.FiltersJSON = existing.FiltersJSON
	}
	normalizeCollectTaskPaths(item)
	if strings.TrimSpace(item.ParseModeSnapshot) == "" && item.ParseTemplateID == 0 {
		item.ParseModeSnapshot = "raw"
	}
	normalizeCollectTaskExecutionSettings(item)
}

func nextCollectTaskConfigVersion(existing, item *logtailmodel.LogCollectTask) int {
	if item != nil && item.ConfigVersion > 0 {
		return item.ConfigVersion
	}
	if existing == nil || existing.ConfigVersion <= 0 {
		return 1
	}
	if collectTaskConfigChanged(existing, item) {
		return existing.ConfigVersion + 1
	}
	return existing.ConfigVersion
}

func collectTaskConfigChanged(existing, item *logtailmodel.LogCollectTask) bool {
	if existing == nil || item == nil {
		return false
	}
	return existing.ProjectID != item.ProjectID ||
		existing.StoreID != item.StoreID ||
		existing.Name != item.Name ||
		existing.Code != item.Code ||
		existing.CollectType != item.CollectType ||
		existing.ExecuteMode != item.ExecuteMode ||
		existing.ExecuteTargetsJSON != item.ExecuteTargetsJSON ||
		existing.PathPattern != item.PathPattern ||
		existing.PathPatternsJSON != item.PathPatternsJSON ||
		existing.K8SLogSource != item.K8SLogSource ||
		existing.ParseTemplateID != item.ParseTemplateID ||
		existing.ParseModeSnapshot != item.ParseModeSnapshot ||
		existing.SampleLog != item.SampleLog ||
		existing.RegexPattern != item.RegexPattern ||
		boolPointerValue(existing.MultilineEnabledSnapshot) != boolPointerValue(item.MultilineEnabledSnapshot) ||
		existing.MultilinePatternSnapshot != item.MultilinePatternSnapshot ||
		existing.LabelsJSON != item.LabelsJSON ||
		existing.FiltersJSON != item.FiltersJSON ||
		existing.Status != item.Status
}

func normalizeCollectTaskPaths(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	patterns := collectTaskPathPatterns(item)
	item.PathPatterns = patterns
	if len(patterns) == 0 {
		item.PathPattern = ""
		item.PathPatternsJSON = ""
		return
	}
	item.PathPattern = strings.Join(patterns, "\n")
	if payload, err := json.Marshal(patterns); err == nil {
		item.PathPatternsJSON = string(payload)
	}
}

func hydrateCollectTaskPathPatterns(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	item.PathPatterns = collectTaskPathPatterns(item)
}

func hydrateCollectTaskPresentation(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	hydrateCollectTaskPathPatterns(item)
	item.LogType = displayCollectTaskParseMode(item.ParseModeSnapshot)
}

func normalizeCollectTaskExecutionSettings(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	item.CollectType = normalizeCollectTaskType(item.CollectType)
	item.ExecuteMode = strings.TrimSpace(item.ExecuteMode)
	item.ExecuteTargetsJSON = strings.TrimSpace(item.ExecuteTargetsJSON)
	if isKubernetesCollectType(item.CollectType) {
		item.K8SLogSource = normalizeKubernetesLogSource(item.K8SLogSource)
		if item.K8SLogSource == "" {
			if len(collectTaskPathPatterns(item)) > 0 {
				item.K8SLogSource = logtailmodel.LogK8SLogSourceNodeFile
			} else {
				item.K8SLogSource = logtailmodel.LogK8SLogSourceStdout
			}
		}
		item.ExecuteTargetsJSON = stripKubernetesTargetContainers(item.ExecuteTargetsJSON)
	} else {
		item.K8SLogSource = ""
	}
}

func stripKubernetesTargetContainers(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	var targets []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return raw
	}
	for _, target := range targets {
		delete(target, "containers")
		delete(target, "containerNames")
		delete(target, "containerName")
	}
	payload, err := json.Marshal(targets)
	if err != nil {
		return raw
	}
	return string(payload)
}

func normalizeCollectTaskType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "host", "logtail_host":
		return strings.ToLower(strings.TrimSpace(raw))
	case "k8s", "kubernetes", "kubernetes_workload", "k8s_workload":
		return logtailmodel.LogCollectTypeKubernetes
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func normalizeKubernetesLogSource(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "stdout", "stderr", "standard", "standard_output", "container_stdout":
		return logtailmodel.LogK8SLogSourceStdout
	case "node_file", "host_path", "hostpath", "host_path_file", "node_path", "node_path_file":
		return logtailmodel.LogK8SLogSourceNodeFile
	case "container_file", "file", "path", "container_path":
		return logtailmodel.LogK8SLogSourceContainerFile
	case "both", "all", "stdout+container_file", "stdout_and_container_file", "stdout,container_file", "container_file,stdout", "standard_output+container_file":
		return logtailmodel.LogK8SLogSourceBoth
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func kubernetesLogSourceIncludes(raw, source string) bool {
	normalized := normalizeKubernetesLogSource(raw)
	target := normalizeKubernetesLogSource(source)
	if normalized == logtailmodel.LogK8SLogSourceBoth {
		return target == logtailmodel.LogK8SLogSourceStdout || target == logtailmodel.LogK8SLogSourceContainerFile
	}
	return normalized != "" && normalized == target
}

func normalizeCollectTaskParseSettings(item *logtailmodel.LogCollectTask) {
	if item == nil {
		return
	}
	if strings.TrimSpace(item.LogType) != "" {
		item.ParseModeSnapshot = item.LogType
	}
	item.ParseModeSnapshot = normalizeCollectTaskParseMode(item.ParseModeSnapshot)
	item.SampleLog = strings.TrimSpace(item.SampleLog)
	item.RegexPattern = strings.TrimSpace(item.RegexPattern)
	item.LogType = displayCollectTaskParseMode(item.ParseModeSnapshot)
}

func (s *LogtailService) applyCollectTaskMultilineSettings(item *logtailmodel.LogCollectTask) error {
	if item == nil {
		return nil
	}
	if item.MultilineEnabledSnapshot != nil && !*item.MultilineEnabledSnapshot {
		item.MultilinePatternSnapshot = ""
		return nil
	}

	var template *logtailmodel.LogParseTemplate
	if item.ParseTemplateID > 0 {
		loaded, err := s.templateDAO.GetByID(item.ParseTemplateID)
		if err != nil {
			return fmt.Errorf("查询解析模板失败: %w", err)
		}
		template = loaded
	}
	mergeCollectTaskMultilineSettings(item, template)
	return nil
}

func mergeCollectTaskMultilineSettings(item *logtailmodel.LogCollectTask, template *logtailmodel.LogParseTemplate) {
	pattern := normalizeDefaultTextMultilinePattern(item.MultilinePatternSnapshot)
	if pattern == "" && template != nil && template.MultilineEnabled {
		pattern = strings.TrimSpace(template.MultilinePattern)
	}
	enabled := item.MultilineEnabledSnapshot != nil && *item.MultilineEnabledSnapshot
	if item.MultilineEnabledSnapshot == nil {
		enabled = pattern != "" || normalizeCollectTaskParseMode(item.ParseModeSnapshot) == "raw" || normalizeCollectTaskParseMode(item.ParseModeSnapshot) == "db_slow_log"
	}
	if enabled && pattern == "" {
		pattern = defaultMultilinePatternForMode(item.ParseModeSnapshot)
	}
	item.MultilineEnabledSnapshot = boolPointer(enabled)
	item.MultilinePatternSnapshot = pattern
}

func normalizeDefaultTextMultilinePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == legacyDefaultTextMultilinePattern || pattern == previousDefaultTextMultilinePattern {
		return defaultTextMultilinePattern
	}
	return pattern
}

func defaultMultilinePatternForMode(mode string) string {
	switch normalizeCollectTaskParseMode(mode) {
	case "raw":
		return defaultTextMultilinePattern
	case "db_slow_log":
		return defaultSlowLogMultilinePattern
	default:
		return ""
	}
}

func validateCollectTaskMultilinePattern(item *logtailmodel.LogCollectTask) error {
	if item == nil || item.MultilineEnabledSnapshot == nil || !*item.MultilineEnabledSnapshot {
		return nil
	}
	pattern := strings.TrimSpace(item.MultilinePatternSnapshot)
	if pattern == "" {
		return errors.New("启用多行合并时，日志起始正则不能为空")
	}
	if strings.ContainsAny(pattern, "\r\n") {
		return errors.New("日志起始正则不能包含换行符")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("日志起始正则无效: %w", err)
	}
	return nil
}

func boolPointer(value bool) *bool {
	return &value
}

func boolPointerValue(value *bool) string {
	if value == nil {
		return "unset"
	}
	return strconv.FormatBool(*value)
}

func normalizeCollectTaskParseMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "raw":
		return "raw"
	case "txt", "text", "plain":
		return "raw"
	case "json":
		return "json"
	case "regex":
		return "regex"
	case "nginx":
		return "nginx"
	case "nginx_json", "nginx+json", "json_nginx", "json+nginx", "nginx-json", "json-nginx":
		return "nginx_json"
	case "nginx_json_loose", "nginx+json-loose", "nginx-json-loose", "nginx+loose-json", "nginx_loose_json", "json_loose_nginx", "json+nginx-loose":
		return "nginx_json_loose"
	case "db_slow_log", "slow_log", "mysql_slow", "mysql_slow_log":
		return "db_slow_log"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func displayCollectTaskParseMode(raw string) string {
	switch normalizeCollectTaskParseMode(raw) {
	case "":
		return ""
	case "raw":
		return "txt"
	case "json":
		return "json"
	case "regex":
		return "regex"
	case "nginx":
		return "nginx"
	case "nginx_json":
		return "nginx_json"
	case "nginx_json_loose":
		return "nginx_json_loose"
	case "db_slow_log":
		return "db_slow_log"
	default:
		return strings.TrimSpace(raw)
	}
}

func validateCollectTaskRegexPattern(pattern, sample string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	if strings.ContainsAny(pattern, "\r\n") {
		return errors.New("解析正则不支持换行")
	}
	if strings.Contains(pattern, "'") {
		return errors.New("解析正则暂不支持单引号，请调整为不包含单引号的写法")
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("解析正则格式不正确: %w", err)
	}
	matches := logtailNamedCapturePattern.FindAllStringSubmatch(pattern, -1)
	if len(matches) == 0 {
		return errors.New("解析正则至少需要一个命名捕获组，例如 (?P<status>\\d+)")
	}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if !vectorFieldRegexp.MatchString(match[1]) {
			return fmt.Errorf("解析正则捕获组字段名不合法: %s", match[1])
		}
	}
	sample = strings.TrimSpace(sample)
	if sample != "" {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("解析正则格式不正确: %w", err)
		}
		if !compiled.MatchString(sample) {
			return errors.New("解析正则与提供的日志例子不匹配")
		}
	}
	return nil
}

func isHostCollectType(collectType string) bool {
	switch strings.ToLower(strings.TrimSpace(collectType)) {
	case logtailmodel.LogCollectTypeHost, logtailmodel.LogCollectTypeLogtailHost:
		return true
	default:
		return false
	}
}

func isKubernetesCollectType(collectType string) bool {
	switch strings.ToLower(strings.TrimSpace(collectType)) {
	case "k8s", logtailmodel.LogCollectTypeKubernetes, "kubernetes_workload", "k8s_workload":
		return true
	default:
		return false
	}
}

func validateKubernetesCollectTask(item *logtailmodel.LogCollectTask) error {
	if item == nil {
		return errors.New("K8S日志采集任务不能为空")
	}
	switch normalizeKubernetesLogSource(item.K8SLogSource) {
	case logtailmodel.LogK8SLogSourceStdout:
	case logtailmodel.LogK8SLogSourceNodeFile:
		if len(collectTaskPathPatterns(item)) == 0 {
			return errors.New("节点文件日志采集必须填写宿主机日志路径")
		}
	case logtailmodel.LogK8SLogSourceContainerFile:
		if len(collectTaskPathPatterns(item)) == 0 {
			return errors.New("容器内文件日志采集必须填写日志路径")
		}
	case logtailmodel.LogK8SLogSourceBoth:
		if len(collectTaskPathPatterns(item)) == 0 {
			return errors.New("同时采集标准输出和容器内文件时必须填写容器内日志路径")
		}
	default:
		return fmt.Errorf("不支持的K8S日志来源: %s", item.K8SLogSource)
	}
	if err := validateKubernetesCollectTargets(item.ExecuteTargetsJSON, normalizeKubernetesLogSource(item.K8SLogSource)); err != nil {
		return err
	}
	return nil
}

func validateKubernetesCollectTargets(raw, source string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			return errors.New("节点文件日志采集必须选择K8S集群")
		}
		return errors.New("K8S日志采集必须选择工作负载")
	}
	var targets []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &targets); err != nil {
		return errors.New("K8S日志采集目标格式不正确")
	}
	if len(targets) == 0 {
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			return errors.New("节点文件日志采集必须选择K8S集群")
		}
		return errors.New("K8S日志采集必须选择工作负载")
	}
	for index, target := range targets {
		if mapUintValue(target, "clusterId", "clusterID") == 0 {
			return fmt.Errorf("K8S日志采集目标第%d项缺少clusterId", index+1)
		}
		if source == logtailmodel.LogK8SLogSourceNodeFile {
			continue
		}
		if mapStringValue(target, "namespace", "namespaceName") == "" {
			return fmt.Errorf("K8S日志采集目标第%d项缺少namespace", index+1)
		}
		if mapStringValue(target, "workloadType", "type") == "" {
			return fmt.Errorf("K8S日志采集目标第%d项缺少workloadType", index+1)
		}
		if mapStringValue(target, "workloadName", "name") == "" {
			return fmt.Errorf("K8S日志采集目标第%d项缺少workloadName", index+1)
		}
	}
	return nil
}

func mapStringValue(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			value := strings.TrimSpace(fmt.Sprint(raw))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func mapUintValue(values map[string]interface{}, keys ...string) uint {
	for _, key := range keys {
		if raw, ok := values[key]; ok {
			switch typed := raw.(type) {
			case float64:
				if typed > 0 {
					return uint(typed)
				}
			case int:
				if typed > 0 {
					return uint(typed)
				}
			case uint:
				return typed
			case string:
				var parsed uint64
				if _, err := fmt.Sscan(strings.TrimSpace(typed), &parsed); err == nil && parsed > 0 {
					return uint(parsed)
				}
			}
		}
	}
	return 0
}

func collectTaskHasExplicitTargets(item *logtailmodel.LogCollectTask) bool {
	if item == nil {
		return false
	}
	agentIDs, hostIDs := parseCollectTaskTargetIDs(item.ExecuteTargetsJSON)
	return len(agentIDs) > 0 || len(hostIDs) > 0
}

func collectTaskPathPatterns(item *logtailmodel.LogCollectTask) []string {
	if item == nil {
		return nil
	}
	if len(item.PathPatterns) > 0 {
		return normalizePathPatternList(item.PathPatterns)
	}
	if raw := strings.TrimSpace(item.PathPatternsJSON); raw != "" {
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err == nil {
			return normalizePathPatternList(values)
		}
	}
	return splitPathPatterns(item.PathPattern)
}

func splitPathPatterns(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(strings.ReplaceAll(raw, "\r\n", "\n"), func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	})
	return normalizePathPatternList(parts)
}

func normalizePathPatternList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		path := strings.TrimSpace(value)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		result = append(result, path)
		seen[path] = struct{}{}
	}
	return result
}

func validateAlertRule(item *logtailmodel.LogAlertRule) error {
	if item.ProjectID == 0 {
		return errors.New("项目ID不能为空")
	}
	if item.StoreID == 0 {
		return errors.New("日志库ID不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("告警规则名称不能为空")
	}
	if strings.TrimSpace(item.Code) == "" {
		return errors.New("告警规则编码不能为空")
	}
	if strings.TrimSpace(item.RuleType) == "" {
		return errors.New("规则类型不能为空")
	}
	if strings.TrimSpace(item.Severity) == "" {
		return errors.New("严重程度不能为空")
	}
	if item.ScanIntervalMinutes <= 0 {
		item.ScanIntervalMinutes = 5
	}
	if item.WindowMinutes <= 0 {
		item.WindowMinutes = 5
	}
	if item.Threshold <= 0 {
		item.Threshold = 1
	}
	if item.SilenceMinutes <= 0 {
		item.SilenceMinutes = 30
	}
	if item.Status == 0 {
		item.Status = logtailmodel.LogStatusEnabled
	}
	return nil
}

func validateAlertEvent(item *logtailmodel.LogAlertEvent) error {
	if item.RuleID == 0 {
		return errors.New("规则ID不能为空")
	}
	if item.ProjectID == 0 {
		return errors.New("项目ID不能为空")
	}
	if item.StoreID == 0 {
		return errors.New("日志库ID不能为空")
	}
	if strings.TrimSpace(item.Severity) == "" {
		return errors.New("严重程度不能为空")
	}
	if strings.TrimSpace(item.Status) == "" {
		item.Status = "firing"
	}
	if item.FiredAt == nil {
		now := time.Now()
		item.FiredAt = &now
	}
	if strings.TrimSpace(item.Summary) == "" {
		item.Summary = fmt.Sprintf("rule=%d 触发日志告警", item.RuleID)
	}
	return nil
}
