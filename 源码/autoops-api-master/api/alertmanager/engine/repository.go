package engine

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/model"
	alertnotify "dodevops-api/api/alertmanager/notify"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"dodevops-api/pkg/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	timelineQueueSize              = 2048
	notifyRecordQueueSize          = 4096
	writeBatchSize                 = 100
	writeBatchInterval             = 100 * time.Millisecond
	activeEventDBUpdateMinInterval = 30 * time.Second
	silenceCacheTTL                = 15 * time.Second
	notifyConfigCacheTTL           = 15 * time.Second
	notifyGroupStateKeyPrefix      = "alertmanager:notify:group:"
	notifyGroupStateTTL            = 7 * 24 * time.Hour
	notifyGroupStateTimeout        = 2 * time.Second
)

const reserveNotifyGroupScript = `
local now = tonumber(ARGV[1])
local interval = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
local previous = 0
for i, key in ipairs(KEYS) do
	local value = redis.call("GET", key)
	if value then
		local last = tonumber(value)
		if last and last > 0 then
			if last > previous then
				previous = last
			end
			if now < last + interval then
				return {0, tostring(last)}
			end
		end
	end
end
for i, key in ipairs(KEYS) do
	redis.call("SET", key, ARGV[1], "EX", ttl)
end
return {1, tostring(previous)}
`

type repository struct {
	db                     *gorm.DB
	writeDB                *gorm.DB
	timelineQueue          chan model.AlertEventTimeline
	timelineWorkerOnce     sync.Once
	notifyRecordQueue      chan model.AlertNotifyRecord
	notifyRecordWorkerOnce sync.Once
	activeState            *activeStateStore
	silenceCache           []model.AlertSilenceRule
	silenceCacheExpiresAt  time.Time
	silenceCacheReady      bool
	silenceCacheMu         sync.Mutex
	notifyCacheMu          sync.RWMutex
	notifyRuleCache        map[uint]cachedNotifyRule
	notifyChannelCache     map[uint]cachedNotifyChannel
	notifyTemplateCache    map[uint]cachedNotifyTemplate
	notifyRouteRuleCache   []model.AlertNotifyRouteRule
	notifyRouteCacheExpiry time.Time
	notifyRouteCacheReady  bool
	notifyGroupMu          sync.RWMutex
	notifyGroupLastSent    map[string]time.Time
	datasourceCache        map[string]cachedDatasource
	datasourceCacheMu      sync.RWMutex
	datasourceCacheTTL     time.Duration
}

type cachedDatasource struct {
	datasource model.AlertDatasource
	expiresAt  time.Time
}

type cachedNotifyRule struct {
	item      model.AlertNotifyRule
	expiresAt time.Time
}

type cachedNotifyChannel struct {
	item      model.AlertNotifyChannel
	expiresAt time.Time
}

type cachedNotifyTemplate struct {
	item      model.AlertNotifyTemplate
	expiresAt time.Time
}

func newRepository(db *gorm.DB) *repository {
	return &repository{
		db:                  db,
		writeDB:             db.Session(&gorm.Session{SkipDefaultTransaction: true}),
		timelineQueue:       make(chan model.AlertEventTimeline, timelineQueueSize),
		notifyRecordQueue:   make(chan model.AlertNotifyRecord, notifyRecordQueueSize),
		activeState:         newActiveStateStore(common.GetRedisClient()),
		notifyRuleCache:     make(map[uint]cachedNotifyRule),
		notifyChannelCache:  make(map[uint]cachedNotifyChannel),
		notifyTemplateCache: make(map[uint]cachedNotifyTemplate),
		notifyGroupLastSent: make(map[string]time.Time),
		datasourceCache:     make(map[string]cachedDatasource),
		datasourceCacheTTL:  30 * time.Second,
	}
}

func (r *repository) listEnabledRules() ([]model.AlertRule, error) {
	var rules []model.AlertRule
	err := r.db.Where("enabled = ?", model.RuleStatusEnabled).
		Order("id ASC").
		Find(&rules).Error
	return rules, err
}

func (r *repository) getDatasourceForRule(rule model.AlertRule) (model.AlertDatasource, error) {
	cacheKey := datasourceCacheKey(rule)
	if datasource, ok := r.cachedDatasource(cacheKey, time.Now()); ok {
		return datasource, nil
	}

	var datasource model.AlertDatasource
	if rule.DatasourceID > 0 {
		err := r.db.Where("id = ? AND enabled = ?", rule.DatasourceID, model.RuleStatusEnabled).
			First(&datasource).Error
		if err != nil {
			return datasource, err
		}
		r.setCachedDatasource(cacheKey, datasource, time.Now())
		return datasource, nil
	}
	datasourceType := rule.DatasourceType
	if datasourceType == "" {
		datasourceType = model.AlertDatasourcePrometheus
	}
	err := r.db.Where("datasource_type = ? AND enabled = ?", datasourceType, model.RuleStatusEnabled).
		Order("is_default DESC, builtin DESC, id ASC").
		First(&datasource).Error
	if err != nil {
		return datasource, err
	}
	r.setCachedDatasource(cacheKey, datasource, time.Now())
	return datasource, nil
}

func (r *repository) ruleDatasourceAvailable(rule model.AlertRule) (bool, error) {
	_, err := r.getDatasourceForRule(rule)
	if err == nil {
		return true, nil
	}
	if isDatasourceUnavailableError(err) {
		return false, nil
	}
	return false, err
}

func isDatasourceUnavailableError(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func datasourceCacheKey(rule model.AlertRule) string {
	if rule.DatasourceID > 0 {
		return fmt.Sprintf("id:%d", rule.DatasourceID)
	}
	datasourceType := strings.TrimSpace(rule.DatasourceType)
	if datasourceType == "" {
		datasourceType = model.AlertDatasourcePrometheus
	}
	return "type:" + datasourceType
}

func (r *repository) cachedDatasource(key string, now time.Time) (model.AlertDatasource, bool) {
	r.datasourceCacheMu.RLock()
	item, ok := r.datasourceCache[key]
	r.datasourceCacheMu.RUnlock()
	if !ok || now.After(item.expiresAt) {
		return model.AlertDatasource{}, false
	}
	return item.datasource, true
}

func (r *repository) setCachedDatasource(key string, datasource model.AlertDatasource, now time.Time) {
	r.datasourceCacheMu.Lock()
	r.datasourceCache[key] = cachedDatasource{
		datasource: datasource,
		expiresAt:  now.Add(r.datasourceCacheTTL),
	}
	r.datasourceCacheMu.Unlock()
}

func (r *repository) invalidateDatasourceCache() {
	r.datasourceCacheMu.Lock()
	r.datasourceCache = make(map[string]cachedDatasource)
	r.datasourceCacheMu.Unlock()
}

func (r *repository) listActiveEventsByRule(ruleID uint) ([]model.AlertActiveEvent, error) {
	var events []model.AlertActiveEvent
	err := r.db.Where("rule_id = ?", ruleID).Find(&events).Error
	return events, err
}

func (r *repository) listActiveEventsByRules(ruleIDs []uint) (map[uint][]model.AlertActiveEvent, error) {
	grouped := make(map[uint][]model.AlertActiveEvent, len(ruleIDs))
	if len(ruleIDs) == 0 {
		return grouped, nil
	}
	if redisEvents, missingRuleIDs, ok := r.activeState.listByRules(ruleIDs); ok {
		for ruleID, events := range redisEvents {
			grouped[ruleID] = events
		}
		ruleIDs = missingRuleIDs
		if len(ruleIDs) == 0 {
			return grouped, nil
		}
	}

	var events []model.AlertActiveEvent
	err := r.db.Where("rule_id IN ? AND status <> ?", ruleIDs, model.EventStatusResolved).Find(&events).Error
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		grouped[event.RuleID] = append(grouped[event.RuleID], event)
		r.cacheActiveEvent(event)
	}
	return grouped, nil
}

func (r *repository) listActiveEventsFromState() ([]model.AlertActiveEvent, bool) {
	if r.activeState == nil {
		return nil, false
	}
	return r.activeState.listAll()
}

func (r *repository) upsertActiveEvent(rule model.AlertRule, ds model.AlertDatasource, sample alertSample, status string, silenced bool, now time.Time, existing *model.AlertActiveEvent) (model.AlertActiveEvent, error) {
	nowH := util.HTime{Time: now}
	event := model.AlertActiveEvent{
		Fingerprint:      sample.Fingerprint,
		RuleID:           rule.ID,
		RuleName:         rule.Name,
		DatasourceID:     ds.ID,
		DatasourceType:   ds.DatasourceType,
		Severity:         firstNonEmpty(sample.Severity, rule.Severity),
		Status:           status,
		Silenced:         boolToInt(silenced),
		Labels:           sample.Labels,
		Annotations:      rule.Annotations,
		Value:            sample.Value,
		TriggerQuery:     sample.TriggerQuery,
		TriggerCondition: sample.TriggerCondition,
		EventDetail:      eventDetailText(rule.Annotations, rule.Note),
		BusinessID:       rule.BusinessID,
		LastEvaluatedAt:  nowH,
		UpdatedAt:        nowH,
	}
	if existing == nil || existing.ID == 0 {
		event.FirstTriggeredAt = nowH
		event.CreatedAt = nowH
		result := r.writeDB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "fingerprint"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"rule_id":           event.RuleID,
				"rule_name":         event.RuleName,
				"datasource_id":     event.DatasourceID,
				"datasource_type":   event.DatasourceType,
				"severity":          event.Severity,
				"status":            event.Status,
				"silenced":          event.Silenced,
				"labels":            event.Labels,
				"annotations":       event.Annotations,
				"value":             event.Value,
				"trigger_query":     event.TriggerQuery,
				"trigger_condition": event.TriggerCondition,
				"event_detail":      event.EventDetail,
				"business_id":       event.BusinessID,
				"last_evaluated_at": nowH,
				"updated_at":        nowH,
			}),
		}).Create(&event)
		if result.Error != nil {
			return event, result.Error
		}
		inserted := result.RowsAffected == 1 && event.ID > 0
		if !inserted {
			duplicated, findErr := r.getActiveEventByFingerprint(sample.Fingerprint)
			if findErr != nil {
				return event, findErr
			}
			r.cacheActiveEvent(*duplicated)
			return *duplicated, nil
		}
		if event.ID == 0 {
			created, findErr := r.getActiveEventByFingerprint(sample.Fingerprint)
			if findErr != nil {
				return event, findErr
			}
			event = *created
		}
		r.enqueueTimeline(model.AlertEventTimeline{
			EventID:   event.ID,
			Action:    "triggered",
			Message:   firstNonEmpty(sample.TriggerCondition, sample.TriggerQuery, fmt.Sprintf("rule %s triggered", rule.Name)),
			Operator:  "alert-engine",
			Metadata:  model.JSONMap{"status": status, "value": sample.Value, "severity": event.Severity},
			CreatedAt: nowH,
		})
		r.cacheActiveEvent(event)
		return event, nil
	}

	updates := map[string]interface{}{
		"status":            status,
		"silenced":          event.Silenced,
		"severity":          firstNonEmpty(sample.Severity, rule.Severity),
		"labels":            sample.Labels,
		"annotations":       rule.Annotations,
		"value":             sample.Value,
		"trigger_query":     sample.TriggerQuery,
		"trigger_condition": sample.TriggerCondition,
		"event_detail":      eventDetailText(rule.Annotations, rule.Note),
		"business_id":       rule.BusinessID,
		"last_evaluated_at": nowH,
		"updated_at":        nowH,
	}
	conditionChanged := triggerConditionChanged(*existing, sample)
	resetActiveEvent := existing.FirstTriggeredAt.IsZero() || existing.Status == model.EventStatusResolved || conditionChanged
	if resetActiveEvent {
		updates["first_triggered_at"] = nowH
		updates["recovered_at"] = nil
		updates["last_notified_at"] = nil
		updates["notify_count"] = 0
	}
	if shouldSkipActiveEventDBUpdate(*existing, event, resetActiveEvent, conditionChanged, now) {
		updated := updatedActiveEvent(*existing, event, resetActiveEvent, nowH)
		r.cacheActiveEvent(updated)
		return updated, nil
	}
	err := r.writeDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.AlertActiveEvent{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return err
		}
		if conditionChanged {
			return tx.Where("event_id = ?", existing.ID).Delete(&model.AlertEventTimeline{}).Error
		}
		return nil
	})
	if err != nil {
		return model.AlertActiveEvent{}, err
	}
	if conditionChanged {
		r.enqueueTimeline(model.AlertEventTimeline{
			EventID:   existing.ID,
			Action:    "triggered",
			Message:   firstNonEmpty(sample.TriggerCondition, sample.TriggerQuery, "告警已重新触发"),
			Operator:  "alert-engine",
			Metadata:  model.JSONMap{"status": status, "value": sample.Value, "severity": firstNonEmpty(sample.Severity, rule.Severity), "resetReason": "trigger_condition_changed"},
			CreatedAt: nowH,
		})
	} else if existing.Status == model.EventStatusPending && status == model.EventStatusFiring {
		r.enqueueTimeline(model.AlertEventTimeline{
			EventID:   existing.ID,
			Action:    "firing",
			Message:   "告警进入触发状态",
			Operator:  "alert-engine",
			Metadata:  model.JSONMap{"status": status, "value": sample.Value, "severity": firstNonEmpty(sample.Severity, rule.Severity)},
			CreatedAt: nowH,
		})
	}
	updated := updatedActiveEvent(*existing, event, resetActiveEvent, nowH)
	r.cacheActiveEvent(updated)
	return updated, nil
}

func shouldSkipActiveEventDBUpdate(existing, event model.AlertActiveEvent, resetActiveEvent, conditionChanged bool, now time.Time) bool {
	if resetActiveEvent || conditionChanged || existing.ID == 0 {
		return false
	}
	if existing.Status != event.Status || existing.Silenced != event.Silenced {
		return false
	}
	if existing.RuleName != event.RuleName ||
		existing.DatasourceID != event.DatasourceID ||
		existing.DatasourceType != event.DatasourceType ||
		existing.Severity != event.Severity ||
		existing.TriggerQuery != event.TriggerQuery ||
		existing.TriggerCondition != event.TriggerCondition ||
		existing.EventDetail != event.EventDetail ||
		existing.BusinessID != event.BusinessID {
		return false
	}
	if existing.UpdatedAt.IsZero() {
		return false
	}
	return now.Sub(existing.UpdatedAt.Time) < activeEventDBUpdateMinInterval
}

func (r *repository) getActiveEventByFingerprint(fingerprint string) (*model.AlertActiveEvent, error) {
	var event model.AlertActiveEvent
	err := r.db.Where("fingerprint = ?", fingerprint).First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *repository) cacheActiveEvent(event model.AlertActiveEvent) {
	if r.activeState == nil {
		return
	}
	r.activeState.save(event)
}

func (r *repository) deleteActiveEventState(event model.AlertActiveEvent) {
	if r.activeState == nil {
		return
	}
	r.activeState.delete(event)
}

func (r *repository) deleteRuleActiveEventState(ruleID uint) {
	if r.activeState == nil {
		return
	}
	r.activeState.deleteRule(ruleID)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (r *repository) hasActiveSilence(sample alertSample, rule model.AlertRule, now time.Time) (bool, error) {
	list, err := r.activeSilenceRules(now)
	if err != nil {
		return false, err
	}
	for _, item := range list {
		if silenceRuleMatches(item, sample, rule) {
			return true, nil
		}
	}
	return false, nil
}

// silenceRuleMatches supports Alertmanager-style matcher items as well as the
// direct matcher map used by event-created silences.
func silenceRuleMatches(item model.AlertSilenceRule, sample alertSample, rule model.AlertRule) bool {
	return model.SilenceRuleMatches(item, model.SilenceMatchContext{
		Fingerprint:  sample.Fingerprint,
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		DatasourceID: rule.DatasourceID,
		Severity:     firstNonEmpty(sample.Severity, rule.Severity),
		BusinessID:   rule.BusinessID,
		Labels:       sample.Labels,
	})
}

func (r *repository) activeSilenceRules(now time.Time) ([]model.AlertSilenceRule, error) {
	r.silenceCacheMu.Lock()
	defer r.silenceCacheMu.Unlock()

	if r.silenceCacheReady && now.Before(r.silenceCacheExpiresAt) {
		return r.silenceCache, nil
	}

	var list []model.AlertSilenceRule
	if err := r.db.Where("enabled = ? AND start_time <= ? AND end_time >= ?", model.RuleStatusEnabled, now, now).
		Find(&list).Error; err != nil {
		if r.silenceCacheReady {
			log.Log().Warnf("alert engine refresh silence cache failed, using stale cache: err=%v", err)
			return r.silenceCache, nil
		}
		return nil, err
	}
	r.silenceCache = list
	r.silenceCacheReady = true
	r.silenceCacheExpiresAt = now.Add(silenceCacheTTL)
	return r.silenceCache, nil
}

func (r *repository) invalidateSilenceRules() {
	r.silenceCacheMu.Lock()
	r.silenceCache = nil
	r.silenceCacheReady = false
	r.silenceCacheExpiresAt = time.Time{}
	r.silenceCacheMu.Unlock()
}

func (r *repository) resolveEvent(event model.AlertActiveEvent, now time.Time) error {
	nowH := util.HTime{Time: now}
	err := r.writeDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&model.AlertHistoryEvent{
			Fingerprint:      event.Fingerprint,
			ActiveEventID:    event.ID,
			RuleID:           event.RuleID,
			RuleName:         event.RuleName,
			DatasourceID:     event.DatasourceID,
			DatasourceType:   event.DatasourceType,
			EventType:        model.EventStatusResolved,
			Severity:         event.Severity,
			Labels:           event.Labels,
			Annotations:      event.Annotations,
			Value:            event.Value,
			TriggerQuery:     event.TriggerQuery,
			TriggerCondition: event.TriggerCondition,
			EventDetail:      event.EventDetail,
			AssetID:          event.AssetID,
			BusinessID:       event.BusinessID,
			FirstTriggeredAt: event.FirstTriggeredAt,
			RecoveredAt:      nowH,
			ClaimUser:        event.ClaimUser,
			OccurredAt:       nowH,
			CreatedAt:        nowH,
		}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AlertActiveEvent{}, event.ID).Error
	})
	if err != nil {
		return err
	}
	r.deleteActiveEventState(event)
	return nil
}

func (r *repository) ListEnabledNotifyRulesByIDs(ids []uint) ([]model.AlertNotifyRule, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	now := time.Now()
	rules := make([]model.AlertNotifyRule, 0, len(ids))
	missingIDs := make([]uint, 0)
	seen := make(map[uint]struct{}, len(ids))

	r.notifyCacheMu.RLock()
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if cached, ok := r.notifyRuleCache[id]; ok && now.Before(cached.expiresAt) {
			rules = append(rules, cached.item)
			continue
		}
		missingIDs = append(missingIDs, id)
	}
	r.notifyCacheMu.RUnlock()

	if len(missingIDs) > 0 {
		var missingRules []model.AlertNotifyRule
		if err := r.db.Where("id IN ? AND enabled = ?", missingIDs, model.RuleStatusEnabled).
			Find(&missingRules).Error; err != nil {
			return nil, err
		}
		r.cacheNotifyRules(missingRules, now)
		rules = append(rules, missingRules...)
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Priority > rules[j].Priority
	})
	return rules, nil
}

func (r *repository) GetNotifyChannel(id uint) (model.AlertNotifyChannel, error) {
	if item, ok := r.cachedNotifyChannel(id, time.Now()); ok {
		return item, nil
	}
	var item model.AlertNotifyChannel
	if err := r.db.Where("id = ? AND enabled = ?", id, model.RuleStatusEnabled).First(&item).Error; err != nil {
		return item, err
	}
	r.cacheNotifyChannel(item, time.Now())
	return item, nil
}

func (r *repository) GetNotifyGroupByID(id uint) (model.AlertNotifyGroup, error) {
	var item model.AlertNotifyGroup
	if id == 0 {
		return item, gorm.ErrRecordNotFound
	}
	if err := r.db.Where("id = ? AND enabled = ?", id, model.RuleStatusEnabled).First(&item).Error; err != nil {
		return item, err
	}
	return item, nil
}

func (r *repository) GetNotifyGroupByBusinessID(businessID, channelType string) (model.AlertNotifyGroup, error) {
	var item model.AlertNotifyGroup
	businessID = strings.TrimSpace(businessID)
	if businessID == "" {
		return item, gorm.ErrRecordNotFound
	}
	db := r.db.Where("business_id = ? AND enabled = ?", businessID, model.RuleStatusEnabled)
	if strings.TrimSpace(channelType) != "" {
		db = db.Where("channel_type = ?", strings.TrimSpace(channelType))
	}
	if err := db.Order("updated_at DESC, id DESC").First(&item).Error; err != nil {
		return item, err
	}
	return item, nil
}

func (r *repository) ListEnabledNotifyRouteRules() ([]model.AlertNotifyRouteRule, error) {
	now := time.Now()
	r.notifyCacheMu.RLock()
	if r.notifyRouteCacheReady && now.Before(r.notifyRouteCacheExpiry) {
		items := append([]model.AlertNotifyRouteRule(nil), r.notifyRouteRuleCache...)
		r.notifyCacheMu.RUnlock()
		return items, nil
	}
	r.notifyCacheMu.RUnlock()

	var items []model.AlertNotifyRouteRule
	if err := r.db.Where("enabled = ?", model.RuleStatusEnabled).
		Order("priority DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	r.notifyCacheMu.Lock()
	r.notifyRouteRuleCache = append([]model.AlertNotifyRouteRule(nil), items...)
	r.notifyRouteCacheExpiry = now.Add(notifyConfigCacheTTL)
	r.notifyRouteCacheReady = true
	r.notifyCacheMu.Unlock()
	return items, nil
}

func (r *repository) GetNotifyTemplate(id uint) (model.AlertNotifyTemplate, error) {
	if item, ok := r.cachedNotifyTemplate(id, time.Now()); ok {
		return item, nil
	}
	var item model.AlertNotifyTemplate
	if err := r.db.Where("id = ? AND enabled = ?", id, model.RuleStatusEnabled).First(&item).Error; err != nil {
		return item, err
	}
	r.cacheNotifyTemplate(item, time.Now())
	return item, nil
}

func (r *repository) cacheNotifyRules(rules []model.AlertNotifyRule, now time.Time) {
	if len(rules) == 0 {
		return
	}
	r.notifyCacheMu.Lock()
	for _, rule := range rules {
		r.notifyRuleCache[rule.ID] = cachedNotifyRule{item: rule, expiresAt: now.Add(notifyConfigCacheTTL)}
	}
	r.notifyCacheMu.Unlock()
}

func (r *repository) cachedNotifyChannel(id uint, now time.Time) (model.AlertNotifyChannel, bool) {
	r.notifyCacheMu.RLock()
	cached, ok := r.notifyChannelCache[id]
	r.notifyCacheMu.RUnlock()
	if !ok || now.After(cached.expiresAt) {
		return model.AlertNotifyChannel{}, false
	}
	return cached.item, true
}

func (r *repository) cacheNotifyChannel(item model.AlertNotifyChannel, now time.Time) {
	r.notifyCacheMu.Lock()
	r.notifyChannelCache[item.ID] = cachedNotifyChannel{item: item, expiresAt: now.Add(notifyConfigCacheTTL)}
	r.notifyCacheMu.Unlock()
}

func (r *repository) cachedNotifyTemplate(id uint, now time.Time) (model.AlertNotifyTemplate, bool) {
	r.notifyCacheMu.RLock()
	cached, ok := r.notifyTemplateCache[id]
	r.notifyCacheMu.RUnlock()
	if !ok || now.After(cached.expiresAt) {
		return model.AlertNotifyTemplate{}, false
	}
	return cached.item, true
}

func (r *repository) cacheNotifyTemplate(item model.AlertNotifyTemplate, now time.Time) {
	r.notifyCacheMu.Lock()
	r.notifyTemplateCache[item.ID] = cachedNotifyTemplate{item: item, expiresAt: now.Add(notifyConfigCacheTTL)}
	r.notifyCacheMu.Unlock()
}

func (r *repository) invalidateNotifyConfig() {
	r.notifyCacheMu.Lock()
	r.notifyRuleCache = make(map[uint]cachedNotifyRule)
	r.notifyChannelCache = make(map[uint]cachedNotifyChannel)
	r.notifyTemplateCache = make(map[uint]cachedNotifyTemplate)
	r.notifyRouteRuleCache = nil
	r.notifyRouteCacheExpiry = time.Time{}
	r.notifyRouteCacheReady = false
	r.notifyCacheMu.Unlock()
}

func (r *repository) CreateNotifyRecord(item *model.AlertNotifyRecord) error {
	if item == nil {
		return nil
	}
	r.enqueueNotifyRecord(*item)
	return nil
}

func (r *repository) MarkEventNotified(eventID uint64, now time.Time) error {
	return r.writeDB.Model(&model.AlertActiveEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"last_notified_at": util.HTime{Time: now},
			"notify_count":     gorm.Expr("notify_count + ?", 1),
			"updated_at":       util.HTime{Time: now},
		}).Error
}

func (r *repository) MarkEventsNotified(eventIDs []uint64, now time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	return r.writeDB.Model(&model.AlertActiveEvent{}).
		Where("id IN ?", eventIDs).
		Updates(map[string]interface{}{
			"last_notified_at": util.HTime{Time: now},
			"notify_count":     gorm.Expr("notify_count + ?", 1),
			"updated_at":       util.HTime{Time: now},
		}).Error
}

func (r *repository) ReserveNotifyGroup(groupKey string, now time.Time, intervalSeconds int64) (bool, time.Time, error) {
	stateKeys := notifyGroupStateKeys(groupKey)
	if len(stateKeys) == 0 || intervalSeconds <= 0 {
		return true, time.Time{}, nil
	}
	stateKey := stateKeys[0]
	if now.IsZero() {
		now = time.Now()
	}

	if client := common.GetRedisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), notifyGroupStateTimeout)
		result, err := client.Eval(ctx, reserveNotifyGroupScript, stateKeys, now.Unix(), intervalSeconds, intervalSeconds).Result()
		cancel()
		if err != nil {
			log.Log().Warnf("alert engine reserve notify group in redis failed: key=%s err=%v", stateKey, err)
			allowed, lastSentAt := r.reserveNotifyGroupInMemory(stateKeys, now, intervalSeconds)
			return allowed, lastSentAt, nil
		}
		allowed, lastSentAt, parseErr := parseNotifyGroupReserveResult(result)
		if parseErr != nil {
			log.Log().Warnf("alert engine parse reserve notify group result failed: key=%s result=%v err=%v", stateKey, result, parseErr)
			allowed, lastSentAt := r.reserveNotifyGroupInMemory(stateKeys, now, intervalSeconds)
			return allowed, lastSentAt, nil
		}
		if allowed {
			if dbAllowed, dbLastSentAt := r.reserveNotifyGroupByRecord(groupKey, stateKeys, now, intervalSeconds); !dbAllowed {
				return false, dbLastSentAt, nil
			}
			r.setNotifyGroupLastSentAtInMemory(stateKeys, now)
			return true, lastSentAt, nil
		}
		r.setNotifyGroupLastSentAtInMemory(stateKeys, lastSentAt)
		return false, lastSentAt, nil
	}

	allowed, lastSentAt := r.reserveNotifyGroupInMemory(stateKeys, now, intervalSeconds)
	if allowed {
		if dbAllowed, dbLastSentAt := r.reserveNotifyGroupByRecord(groupKey, stateKeys, now, intervalSeconds); !dbAllowed {
			return false, dbLastSentAt, nil
		}
	}
	return allowed, lastSentAt, nil
}

// ReleaseNotifyGroup clears a reservation made before a send attempt failed.
// Successful sends remain governed by the persisted notify record.
func (r *repository) ReleaseNotifyGroup(groupKey string) error {
	stateKeys := notifyGroupStateKeys(groupKey)
	if len(stateKeys) == 0 {
		return nil
	}
	r.notifyGroupMu.Lock()
	for _, key := range stateKeys {
		delete(r.notifyGroupLastSent, key)
	}
	r.notifyGroupMu.Unlock()
	if client := common.GetRedisClient(); client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), notifyGroupStateTimeout)
		defer cancel()
		if err := client.Del(ctx, stateKeys...).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (r *repository) reserveNotifyGroupByRecord(groupKey string, stateKeys []string, now time.Time, intervalSeconds int64) (bool, time.Time) {
	lastSentAt, err := r.lastNotifyGroupSuccessRecordAt(groupKey)
	if err != nil {
		log.Log().Warnf("alert engine query notify group record failed: groupKey=%s err=%v", groupKey, err)
		return true, time.Time{}
	}
	if lastSentAt.IsZero() {
		return true, time.Time{}
	}
	if now.Before(lastSentAt.Add(time.Duration(intervalSeconds) * time.Second)) {
		r.setNotifyGroupLastSentAtInMemory(stateKeys, lastSentAt)
		r.restoreNotifyGroupRedisState(stateKeys, lastSentAt, intervalSeconds)
		return false, lastSentAt
	}
	return true, lastSentAt
}

func (r *repository) lastNotifyGroupSuccessRecordAt(groupKey string) (time.Time, error) {
	var item model.AlertNotifyRecord
	db := r.db.
		Select("created_at").
		Where("event_type = ? AND status = ?", model.EventDisplayStatusActive, model.NotifyStatusSuccess)
	if notifyRuleID, ruleID, ok := parseNotifyGroupKeyIDs(groupKey); ok && !notifyGroupKeyHasRouteDimension(groupKey) {
		db = db.Where("(notify_group_key = ? OR (notify_rule_id = ? AND rule_id = ?))", groupKey, notifyRuleID, ruleID)
	} else {
		db = db.Where("notify_group_key = ?", groupKey)
	}
	err := db.
		Order("created_at DESC, id DESC").
		Limit(1).
		First(&item).Error
	if err == nil {
		return item.CreatedAt.Time, nil
	}
	if err == gorm.ErrRecordNotFound {
		return time.Time{}, nil
	}
	return time.Time{}, err
}

func parseNotifyGroupKeyIDs(groupKey string) (uint, uint, bool) {
	var notifyRuleID uint
	var ruleID uint
	for _, part := range strings.Split(groupKey, "|") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil || parsed == 0 {
			continue
		}
		switch strings.TrimSpace(key) {
		case "notify":
			notifyRuleID = uint(parsed)
		case "rule":
			ruleID = uint(parsed)
		}
	}
	return notifyRuleID, ruleID, notifyRuleID > 0 && ruleID > 0
}

func notifyGroupKeyHasRouteDimension(groupKey string) bool {
	for _, part := range strings.Split(groupKey, "|") {
		key, _, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "business", "service", "route":
			return true
		}
	}
	return false
}

func (r *repository) restoreNotifyGroupRedisState(stateKeys []string, lastSentAt time.Time, intervalSeconds int64) {
	if len(stateKeys) == 0 || lastSentAt.IsZero() || intervalSeconds <= 0 {
		return
	}
	client := common.GetRedisClient()
	if client == nil {
		return
	}
	ttl := lastSentAt.Add(time.Duration(intervalSeconds) * time.Second).Sub(time.Now())
	if ttl <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyGroupStateTimeout)
	defer cancel()
	for _, stateKey := range stateKeys {
		if stateKey == "" {
			continue
		}
		if err := client.Set(ctx, stateKey, lastSentAt.Unix(), ttl).Err(); err != nil {
			log.Log().Warnf("alert engine restore notify group redis state failed: key=%s err=%v", stateKey, err)
		}
	}
}

func (r *repository) setNotifyGroupLastSentAtInMemory(stateKeys []string, now time.Time) {
	r.notifyGroupMu.Lock()
	for _, stateKey := range stateKeys {
		if stateKey == "" {
			continue
		}
		r.notifyGroupLastSent[stateKey] = now
	}
	for key, lastSentAt := range r.notifyGroupLastSent {
		if now.Sub(lastSentAt) > notifyGroupStateTTL {
			delete(r.notifyGroupLastSent, key)
		}
	}
	r.notifyGroupMu.Unlock()
}

func (r *repository) reserveNotifyGroupInMemory(stateKeys []string, now time.Time, intervalSeconds int64) (bool, time.Time) {
	r.notifyGroupMu.Lock()
	defer r.notifyGroupMu.Unlock()
	var latest time.Time
	for _, stateKey := range stateKeys {
		lastSentAt, ok := r.notifyGroupLastSent[stateKey]
		if !ok || lastSentAt.IsZero() {
			continue
		}
		if lastSentAt.After(latest) {
			latest = lastSentAt
		}
		if now.Before(lastSentAt.Add(time.Duration(intervalSeconds) * time.Second)) {
			return false, lastSentAt
		}
	}
	for _, stateKey := range stateKeys {
		r.notifyGroupLastSent[stateKey] = now
	}
	for key, cachedLastSentAt := range r.notifyGroupLastSent {
		if now.Sub(cachedLastSentAt) > notifyGroupStateTTL {
			delete(r.notifyGroupLastSent, key)
		}
	}
	return true, latest
}

func notifyGroupStateKey(groupKey string) string {
	groupKey = strings.TrimSpace(groupKey)
	if groupKey == "" {
		return ""
	}
	sum := sha1.Sum([]byte(groupKey))
	return notifyGroupStateKeyPrefix + hex.EncodeToString(sum[:])
}

func notifyGroupStateKeys(groupKey string) []string {
	stateKey := notifyGroupStateKey(groupKey)
	if stateKey == "" {
		return nil
	}
	keys := []string{stateKey}
	if notifyRuleID, _, ok := parseNotifyGroupKeyIDs(groupKey); ok && !notifyGroupKeyHasRouteDimension(groupKey) {
		legacyKey := notifyGroupStateKey(fmt.Sprintf("notify:%d", notifyRuleID))
		if legacyKey != "" && legacyKey != stateKey {
			keys = append(keys, legacyKey)
		}
	}
	return keys
}

func parseNotifyGroupReserveResult(result interface{}) (bool, time.Time, error) {
	values, ok := result.([]interface{})
	if !ok || len(values) < 2 {
		return false, time.Time{}, fmt.Errorf("unexpected result type: %T", result)
	}

	allowedInt, err := redisResultInt64(values[0])
	if err != nil {
		return false, time.Time{}, err
	}
	lastUnix, err := redisResultInt64(values[1])
	if err != nil {
		return false, time.Time{}, err
	}
	var lastSentAt time.Time
	if lastUnix > 0 {
		lastSentAt = time.Unix(lastUnix, 0)
	}
	return allowedInt == 1, lastSentAt, nil
}

func redisResultInt64(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected redis result value type: %T", value)
	}
}

func (r *repository) notifyRuleGroupID(rule model.AlertRule) uint {
	return rule.GroupID
}

func (r *repository) sendNotification(ctx context.Context, rule model.AlertRule, event model.AlertActiveEvent, eventType string, now time.Time) ([]uint64, error) {
	return r.sendNotifications(ctx, rule, []model.AlertActiveEvent{event}, eventType, now)
}

func (r *repository) sendNotifications(ctx context.Context, rule model.AlertRule, events []model.AlertActiveEvent, eventType string, now time.Time) ([]uint64, error) {
	dispatchable, err := r.filterSilencedNotifications(events, now)
	if err != nil {
		return nil, err
	}
	if len(dispatchable) == 0 {
		return nil, nil
	}
	dispatcher := alertnotify.NewDispatcher(r)
	return dispatcher.Dispatch(ctx, alertnotify.DispatchRequest{
		Rule:        rule,
		Events:      dispatchable,
		EventType:   eventType,
		RuleGroupID: r.notifyRuleGroupID(rule),
		Now:         now,
	})
}

// filterSilencedNotifications deliberately bypasses the short-lived engine
// cache. Silence rules can be created through another API instance, so the
// final check before an external send must use current shared database state.
func (r *repository) filterSilencedNotifications(events []model.AlertActiveEvent, now time.Time) ([]model.AlertActiveEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}
	var rules []model.AlertSilenceRule
	if err := r.db.Where("enabled = ? AND start_time <= ? AND end_time >= ?", model.RuleStatusEnabled, now, now).
		Find(&rules).Error; err != nil {
		return nil, err
	}
	return eventsWithoutMatchingSilences(events, rules), nil
}

func eventsWithoutMatchingSilences(events []model.AlertActiveEvent, rules []model.AlertSilenceRule) []model.AlertActiveEvent {
	filtered := make([]model.AlertActiveEvent, 0, len(events))
	for _, event := range events {
		silenced := event.Silenced == 1 || event.Status == model.EventStatusSilenced
		if !silenced {
			context := model.SilenceContextFromEvent(event)
			for _, rule := range rules {
				if model.SilenceRuleMatches(rule, context) {
					silenced = true
					break
				}
			}
		}
		if !silenced {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (r *repository) appendTimeline(eventID uint64, action, message string, now time.Time) error {
	r.enqueueTimeline(model.AlertEventTimeline{
		EventID:   eventID,
		Action:    action,
		Message:   message,
		Operator:  "alert-engine",
		CreatedAt: util.HTime{Time: now},
	})
	return nil
}

func (r *repository) enqueueTimeline(item model.AlertEventTimeline) {
	r.timelineWorkerOnce.Do(r.startTimelineWorker)
	select {
	case r.timelineQueue <- item:
	default:
		log.Log().Warnf("alert engine timeline queue full, dropped timeline: event=%d action=%s", item.EventID, item.Action)
	}
}

func (r *repository) startTimelineWorker() {
	go func() {
		r.consumeTimelineQueue()
	}()
}

func (r *repository) consumeTimelineQueue() {
	ticker := time.NewTicker(writeBatchInterval)
	defer ticker.Stop()
	batch := make([]model.AlertEventTimeline, 0, writeBatchSize)
	for {
		select {
		case item := <-r.timelineQueue:
			batch = append(batch, item)
			if len(batch) >= writeBatchSize {
				r.flushTimelines(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			r.flushTimelines(batch)
			batch = batch[:0]
		}
	}
}

func (r *repository) flushTimelines(items []model.AlertEventTimeline) {
	if len(items) == 0 {
		return
	}
	if err := r.writeDB.CreateInBatches(items, writeBatchSize).Error; err != nil {
		log.Log().Warnf("alert engine append timeline batch failed: count=%d err=%v", len(items), err)
	}
}

func (r *repository) enqueueNotifyRecord(item model.AlertNotifyRecord) {
	r.notifyRecordWorkerOnce.Do(r.startNotifyRecordWorker)
	select {
	case r.notifyRecordQueue <- item:
	default:
		log.Log().Warnf("alert engine notify record queue full, dropped record: event=%d rule=%d channel=%d", item.EventID, item.NotifyRuleID, item.ChannelID)
	}
}

func (r *repository) startNotifyRecordWorker() {
	go func() {
		r.consumeNotifyRecordQueue()
	}()
}

func (r *repository) consumeNotifyRecordQueue() {
	ticker := time.NewTicker(writeBatchInterval)
	defer ticker.Stop()
	batch := make([]model.AlertNotifyRecord, 0, writeBatchSize)
	for {
		select {
		case item := <-r.notifyRecordQueue:
			batch = append(batch, item)
			if len(batch) >= writeBatchSize {
				r.flushNotifyRecords(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) == 0 {
				continue
			}
			r.flushNotifyRecords(batch)
			batch = batch[:0]
		}
	}
}

func (r *repository) flushNotifyRecords(items []model.AlertNotifyRecord) {
	if len(items) == 0 {
		return
	}
	if err := r.writeDB.CreateInBatches(items, writeBatchSize).Error; err != nil {
		log.Log().Warnf("alert engine create notify record batch failed: count=%d err=%v", len(items), err)
	}
}

func updatedActiveEvent(existing, event model.AlertActiveEvent, resetActiveEvent bool, now util.HTime) model.AlertActiveEvent {
	existing.RuleName = event.RuleName
	existing.DatasourceID = event.DatasourceID
	existing.DatasourceType = event.DatasourceType
	existing.Severity = event.Severity
	existing.Status = event.Status
	existing.Labels = event.Labels
	existing.Annotations = event.Annotations
	existing.Value = event.Value
	existing.TriggerQuery = event.TriggerQuery
	existing.TriggerCondition = event.TriggerCondition
	existing.EventDetail = event.EventDetail
	existing.BusinessID = event.BusinessID
	existing.Silenced = event.Silenced
	existing.LastEvaluatedAt = now
	existing.UpdatedAt = now
	if resetActiveEvent {
		existing.FirstTriggeredAt = now
		existing.RecoveredAt = util.HTime{}
		existing.LastNotifiedAt = util.HTime{}
		existing.NotifyCount = 0
	}
	return existing
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func eventDetailText(annotations model.JSONMap, note string) string {
	for _, key := range []string{"description", "summary", "message", "detail", "comments"} {
		if annotations == nil {
			break
		}
		value := fmt.Sprint(annotations[key])
		if value != "" && value != "<nil>" {
			return value
		}
	}
	return note
}
