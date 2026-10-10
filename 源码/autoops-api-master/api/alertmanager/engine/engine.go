package engine

import (
	"context"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/dao"
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common"
	"dodevops-api/common/config"
	"dodevops-api/pkg/log"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type Engine struct {
	repo            *repository
	reloadInterval  time.Duration
	reloadMu        sync.Mutex
	mu              sync.Mutex
	workers         map[uint]workerHandle
	suppressedRules map[uint]struct{}
	runCtx          context.Context
	leader          bool
}

type workerHandle struct {
	updatedAt time.Time
	cancel    context.CancelFunc
}

var (
	defaultMu     sync.RWMutex
	defaultEngine *Engine
)

func New(db *gorm.DB) *Engine {
	return &Engine{
		repo:            newRepository(db),
		reloadInterval:  30 * time.Second,
		workers:         make(map[uint]workerHandle),
		suppressedRules: make(map[uint]struct{}),
	}
}

func SetDefault(engine *Engine) {
	defaultMu.Lock()
	defaultEngine = engine
	defaultMu.Unlock()
}

func StopRule(ruleID uint) {
	if engine := getDefault(); engine != nil {
		engine.StopRule(ruleID)
	}
}

func ResumeRule(ruleID uint) {
	if engine := getDefault(); engine != nil {
		engine.ResumeRule(ruleID)
	}
}

func ResumeRules(ruleIDs []uint) {
	if engine := getDefault(); engine != nil {
		engine.ResumeRules(ruleIDs)
	}
}

func TriggerReload() {
	if engine := getDefault(); engine != nil {
		go engine.Reload()
	}
}

func InvalidateSilenceCache() {
	if engine := getDefault(); engine != nil {
		engine.InvalidateSilenceCache()
	}
}

func InvalidateNotifyConfigCache() {
	if engine := getDefault(); engine != nil {
		engine.InvalidateNotifyConfigCache()
	}
}

func InvalidateDatasourceCache() {
	if engine := getDefault(); engine != nil {
		engine.InvalidateDatasourceCache()
	}
}

func ListActiveEventsFromState() ([]model.AlertActiveEvent, bool) {
	if engine := getDefault(); engine != nil {
		return engine.ListActiveEventsFromState()
	}
	return nil, false
}

func CacheActiveEventState(event model.AlertActiveEvent) {
	if engine := getDefault(); engine != nil {
		engine.CacheActiveEventState(event)
	}
}

func getDefault() *Engine {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultEngine
}

// IsLeader reports whether this process currently owns alert engine leadership.
func IsLeader() bool {
	if engine := getDefault(); engine != nil {
		return engine.isLeader()
	}
	return false
}

func (e *Engine) Run(ctx context.Context) {
	client := common.GetRedisClient()
	if client == nil && config.Config.Redis.Address != "" {
		client = redis.NewClient(&redis.Options{
			Addr:     config.Config.Redis.Address,
			Password: config.Config.Redis.Password,
			DB:       0,
		})
		defer client.Close()
	}
	e.runWithLeaderElection(ctx, client)
}

func (e *Engine) runLeader(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	e.reconcileExpiredSilences()
	e.reload(ctx)

	ticker := time.NewTicker(e.reloadInterval)
	defer ticker.Stop()
	defer e.stopAll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.reconcileExpiredSilences()
			e.reload(ctx)
		}
	}
}

func (e *Engine) Reload() {
	if !e.isLeader() {
		return
	}
	ctx := e.context()
	if ctx == nil {
		ctx = context.Background()
	}
	e.reload(ctx)
}

func (e *Engine) reconcileExpiredSilences() {
	e.repo.invalidateSilenceRules()
	events, err := dao.NewAlertManagerDaoWithDB(e.repo.db).ReconcileEventSilenceStates(time.Now(), "静默已到期", "alert-engine")
	if err != nil {
		log.Log().Warnf("alert engine reconcile expired silences failed: %v", err)
		return
	}
	for _, event := range events {
		e.repo.cacheActiveEvent(event)
	}
}

func (e *Engine) StopRule(ruleID uint) {
	e.mu.Lock()
	e.suppressedRules[ruleID] = struct{}{}
	if handle, exists := e.workers[ruleID]; exists {
		handle.cancel()
		delete(e.workers, ruleID)
	}
	e.mu.Unlock()
	e.repo.deleteRuleActiveEventState(ruleID)
}

func (e *Engine) ResumeRule(ruleID uint) {
	e.ResumeRules([]uint{ruleID})
}

func (e *Engine) ResumeRules(ruleIDs []uint) {
	e.mu.Lock()
	for _, ruleID := range ruleIDs {
		delete(e.suppressedRules, ruleID)
	}
	e.mu.Unlock()
	go e.Reload()
}

func (e *Engine) InvalidateSilenceCache() {
	e.repo.invalidateSilenceRules()
}

func (e *Engine) InvalidateNotifyConfigCache() {
	e.repo.invalidateNotifyConfig()
}

func (e *Engine) InvalidateDatasourceCache() {
	e.repo.invalidateDatasourceCache()
}

func (e *Engine) ListActiveEventsFromState() ([]model.AlertActiveEvent, bool) {
	return e.repo.listActiveEventsFromState()
}

func (e *Engine) CacheActiveEventState(event model.AlertActiveEvent) {
	e.repo.cacheActiveEvent(event)
}

func (e *Engine) setRunContext(ctx context.Context) {
	e.mu.Lock()
	e.runCtx = ctx
	e.mu.Unlock()
}

func (e *Engine) context() context.Context {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runCtx
}

func (e *Engine) setLeader(leader bool) {
	e.mu.Lock()
	e.leader = leader
	e.mu.Unlock()
}

func (e *Engine) isLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.leader
}

func (e *Engine) reload(parent context.Context) {
	e.reloadMu.Lock()
	defer e.reloadMu.Unlock()

	rules, err := e.repo.listEnabledRules()
	if err != nil {
		log.Log().Warnf("alert engine reload rules failed: %v", err)
		return
	}

	runnable := make(map[uint]struct{}, len(rules))
	startRules := make([]model.AlertRule, 0)
	for _, rule := range rules {
		available, err := e.repo.ruleDatasourceAvailable(rule)
		if err != nil {
			log.Log().Warnf("alert engine check datasource failed: rule=%d datasource=%d err=%v", rule.ID, rule.DatasourceID, err)
			runnable[rule.ID] = struct{}{}
			continue
		}
		if !available {
			continue
		}
		runnable[rule.ID] = struct{}{}
		if e.shouldStartWorker(rule) {
			startRules = append(startRules, rule)
		}
	}

	activeEventsByRule, err := e.preloadActiveEvents(startRules)
	if err != nil {
		log.Log().Warnf("alert engine preload active events failed: %v", err)
		return
	}

	for _, rule := range startRules {
		activeEvents := activeEventsByRule[rule.ID]
		if activeEvents == nil {
			activeEvents = []model.AlertActiveEvent{}
		}
		e.ensureWorker(parent, rule, activeEvents)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for ruleID, handle := range e.workers {
		if _, ok := runnable[ruleID]; ok {
			continue
		}
		handle.cancel()
		delete(e.workers, ruleID)
	}
}

func (e *Engine) shouldStartWorker(rule model.AlertRule) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, suppressed := e.suppressedRules[rule.ID]; suppressed {
		return false
	}
	existing, exists := e.workers[rule.ID]
	return !exists || !existing.updatedAt.Equal(rule.UpdatedAt.Time)
}

func (e *Engine) preloadActiveEvents(rules []model.AlertRule) (map[uint][]model.AlertActiveEvent, error) {
	ruleIDs := make([]uint, 0, len(rules))
	for _, rule := range rules {
		ruleIDs = append(ruleIDs, rule.ID)
	}
	return e.repo.listActiveEventsByRules(ruleIDs)
}

func (e *Engine) ensureWorker(parent context.Context, rule model.AlertRule, activeEvents []model.AlertActiveEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, suppressed := e.suppressedRules[rule.ID]; suppressed {
		return
	}
	if existing, exists := e.workers[rule.ID]; exists {
		if existing.updatedAt.Equal(rule.UpdatedAt.Time) {
			return
		}
		existing.cancel()
		delete(e.workers, rule.ID)
	}
	ctx, cancel := context.WithCancel(parent)
	e.workers[rule.ID] = workerHandle{updatedAt: rule.UpdatedAt.Time, cancel: cancel}
	go newWorker(rule, e.repo, activeEvents).run(ctx)
}

func (e *Engine) stopAll() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for ruleID, handle := range e.workers {
		handle.cancel()
		delete(e.workers, ruleID)
	}
}
