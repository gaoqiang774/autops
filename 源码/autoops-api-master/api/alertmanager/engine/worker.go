package engine

import (
	"context"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/pkg/log"
)

type worker struct {
	rule              model.AlertRule
	repo              *repository
	activeEvents      map[string]model.AlertActiveEvent
	activeEventsReady bool
}

func newWorker(rule model.AlertRule, repo *repository, activeEvents []model.AlertActiveEvent) *worker {
	w := &worker{rule: rule, repo: repo}
	if activeEvents != nil {
		w.setActiveEventCache(activeEvents)
	}
	return w
}

func (w *worker) run(ctx context.Context) {
	interval := time.Duration(w.rule.EvalIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}

	if delay := initialEvalDelay(w.rule.ID, interval); delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}

	w.evaluateOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.evaluateOnce(ctx)
		}
	}
}

func (w *worker) evaluateOnce(ctx context.Context) {
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	ds, err := w.repo.getDatasourceForRule(w.rule)
	if err != nil {
		if isDatasourceUnavailableError(err) {
			return
		}
		log.Log().Warnf("alert engine datasource unavailable: rule=%d datasource=%d err=%v", w.rule.ID, w.rule.DatasourceID, err)
		return
	}

	samples, err := evaluateRule(evalCtx, w.rule, ds)
	if err != nil {
		log.Log().Warnf("alert engine evaluate failed: rule=%d name=%s err=%v", w.rule.ID, w.rule.Name, err)
		return
	}

	if err := w.ensureActiveEventCache(); err != nil {
		log.Log().Warnf("alert engine load active event cache failed: rule=%d err=%v", w.rule.ID, err)
		return
	}

	now := time.Now()
	current := make(map[string]struct{}, len(samples))
	firingEvents := make([]model.AlertActiveEvent, 0, len(samples))
	for _, sample := range samples {
		current[sample.Fingerprint] = struct{}{}
		existing, ok := w.activeEvents[sample.Fingerprint]
		silenced, err := w.repo.hasActiveSilence(sample, w.rule, now)
		if err != nil {
			log.Log().Warnf("alert engine check silence failed: rule=%d fingerprint=%s err=%v", w.rule.ID, sample.Fingerprint, err)
			continue
		}
		conditionChanged := ok && triggerConditionChanged(existing, sample)
		status := w.nextFiringStatus(existing, ok, conditionChanged, now)
		if silenced {
			status = model.EventStatusSilenced
		}
		var existingPtr *model.AlertActiveEvent
		if ok {
			existingPtr = &existing
		}
		event, err := w.repo.upsertActiveEvent(w.rule, ds, sample, status, silenced, now, existingPtr)
		if err != nil {
			log.Log().Warnf("alert engine upsert active event failed: rule=%d fingerprint=%s err=%v", w.rule.ID, sample.Fingerprint, err)
			continue
		}
		w.activeEvents[event.Fingerprint] = event
		if !silenced && status == model.EventStatusFiring {
			firingEvents = append(firingEvents, event)
		}
	}
	if len(firingEvents) > 0 {
		notifiedIDs, err := w.repo.sendNotifications(evalCtx, w.rule, firingEvents, model.EventStatusFiring, now)
		if err != nil {
			log.Log().Warnf("alert engine send notifications failed: rule=%d count=%d err=%v", w.rule.ID, len(firingEvents), err)
		} else {
			w.markCachedEventsNotified(notifiedIDs, now)
		}
	}

	resolvedEvents := make([]model.AlertActiveEvent, 0)
	for fingerprint, event := range w.activeEvents {
		if _, ok := current[fingerprint]; ok {
			continue
		}
		if !w.shouldResolve(event, now) {
			continue
		}
		resolvedEvent := event
		resolvedEvent.Status = model.EventStatusResolved
		resolvedEvent.RecoveredAt.Time = now
		resolvedEvents = append(resolvedEvents, resolvedEvent)
		if err := w.repo.resolveEvent(event, now); err != nil {
			log.Log().Warnf("alert engine resolve event failed: rule=%d fingerprint=%s err=%v", w.rule.ID, fingerprint, err)
			continue
		}
		delete(w.activeEvents, fingerprint)
		_ = w.repo.appendTimeline(event.ID, "resolved", "告警已恢复", now)
	}
	if len(resolvedEvents) > 0 {
		if _, err := w.repo.sendNotifications(evalCtx, w.rule, resolvedEvents, model.EventStatusResolved, now); err != nil {
			log.Log().Warnf("alert engine send recovered notifications failed: rule=%d count=%d err=%v", w.rule.ID, len(resolvedEvents), err)
		}
	}
}

func (w *worker) ensureActiveEventCache() error {
	if w.activeEventsReady {
		return nil
	}
	activeEvents, err := w.repo.listActiveEventsByRule(w.rule.ID)
	if err != nil {
		return err
	}
	w.activeEvents = make(map[string]model.AlertActiveEvent, len(activeEvents))
	for _, event := range activeEvents {
		if event.Status == model.EventStatusResolved {
			continue
		}
		w.activeEvents[event.Fingerprint] = event
	}
	w.activeEventsReady = true
	return nil
}

func (w *worker) setActiveEventCache(activeEvents []model.AlertActiveEvent) {
	w.activeEvents = make(map[string]model.AlertActiveEvent, len(activeEvents))
	for _, event := range activeEvents {
		if event.Status == model.EventStatusResolved {
			continue
		}
		w.activeEvents[event.Fingerprint] = event
	}
	w.activeEventsReady = true
}

func (w *worker) markCachedEventsNotified(eventIDs []uint64, now time.Time) {
	if len(eventIDs) == 0 {
		return
	}
	notified := make(map[uint64]struct{}, len(eventIDs))
	for _, eventID := range eventIDs {
		notified[eventID] = struct{}{}
	}
	for fingerprint, event := range w.activeEvents {
		if _, ok := notified[event.ID]; !ok {
			continue
		}
		event.LastNotifiedAt.Time = now
		event.NotifyCount++
		event.UpdatedAt.Time = now
		w.activeEvents[fingerprint] = event
		w.repo.cacheActiveEvent(event)
	}
}

func (w *worker) nextFiringStatus(existing model.AlertActiveEvent, ok, conditionChanged bool, now time.Time) string {
	requiredHold := w.requiredFiringHoldDuration()
	if requiredHold <= 0 {
		return model.EventStatusFiring
	}
	if !ok || conditionChanged || existing.FirstTriggeredAt.IsZero() {
		return model.EventStatusPending
	}
	if !now.Before(existing.FirstTriggeredAt.Time.Add(requiredHold)) {
		return model.EventStatusFiring
	}
	return model.EventStatusPending
}

func (w *worker) requiredFiringHoldDuration() time.Duration {
	if w.rule.ForDurationSeconds <= 0 {
		return 0
	}
	intervalSeconds := w.rule.EvalIntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	// The first matched evaluation already counts as the first consecutive hit.
	requiredSeconds := w.rule.ForDurationSeconds - intervalSeconds
	if requiredSeconds < 0 {
		requiredSeconds = 0
	}
	return time.Duration(requiredSeconds) * time.Second
}

func triggerConditionChanged(existing model.AlertActiveEvent, sample alertSample) bool {
	if existing.TriggerCondition != "" && sample.TriggerCondition != "" {
		return existing.TriggerCondition != sample.TriggerCondition
	}
	if existing.TriggerQuery != "" && sample.TriggerQuery != "" {
		return existing.TriggerQuery != sample.TriggerQuery
	}
	return false
}

func (w *worker) shouldResolve(event model.AlertActiveEvent, now time.Time) bool {
	if w.rule.RecoverDurationSeconds <= 0 {
		return true
	}
	last := event.LastEvaluatedAt.Time
	if last.IsZero() {
		last = event.UpdatedAt.Time
	}
	if last.IsZero() {
		return true
	}
	return now.Sub(last) >= time.Duration(w.rule.RecoverDurationSeconds)*time.Second
}

func initialEvalDelay(ruleID uint, interval time.Duration) time.Duration {
	if ruleID == 0 || interval <= time.Second {
		return 0
	}
	maxDelay := 30 * time.Second
	if interval < maxDelay {
		maxDelay = interval
	}
	slots := int(maxDelay / time.Second)
	if slots <= 1 {
		return 0
	}
	return time.Duration(int(ruleID)%slots) * time.Second
}
