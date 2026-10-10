package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"dodevops-api/api/alertmanager/model"

	"gorm.io/gorm"
)

// ExternalEventInput lets business modules reuse alert event lifecycle and notification delivery.
type ExternalEventInput struct {
	Rule             model.AlertRule
	Fingerprint      string
	Labels           model.JSONMap
	Annotations      model.JSONMap
	Value            string
	Severity         string
	TriggerCondition string
	EventDetail      string
	Firing           bool
	Now              time.Time
}

type ExternalEventResult struct {
	EventID  uint64
	Notified bool
	Resolved bool
}

var externalRepositories = struct {
	sync.Mutex
	items map[*gorm.DB]*repository
}{items: map[*gorm.DB]*repository{}}

func PublishExternalEvent(ctx context.Context, db *gorm.DB, input ExternalEventInput) (ExternalEventResult, error) {
	if db == nil || input.Rule.ID == 0 || input.Fingerprint == "" {
		return ExternalEventResult{}, errors.New("外部告警事件参数不完整")
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	repo := externalRepository(db)
	datasource, err := repo.getDatasourceForRule(input.Rule)
	if err != nil {
		return ExternalEventResult{}, err
	}
	existing, err := repo.getActiveEventByFingerprint(input.Fingerprint)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ExternalEventResult{}, err
	}
	if !input.Firing {
		if errors.Is(err, gorm.ErrRecordNotFound) || existing == nil {
			return ExternalEventResult{Resolved: true}, nil
		}
		resolved := *existing
		resolved.Status = model.EventStatusResolved
		resolved.RecoveredAt.Time = now
		if err := repo.resolveEvent(*existing, now); err != nil {
			return ExternalEventResult{}, err
		}
		_ = repo.appendTimeline(existing.ID, "resolved", "告警已恢复", now)
		notified, notifyErr := repo.sendNotification(ctx, input.Rule, resolved, model.EventStatusResolved, now)
		return ExternalEventResult{EventID: existing.ID, Notified: len(notified) > 0, Resolved: true}, notifyErr
	}
	var existingPtr *model.AlertActiveEvent
	if err == nil {
		existingPtr = existing
	}
	sample := alertSample{
		Fingerprint: input.Fingerprint, Labels: input.Labels, Value: input.Value,
		Severity: input.Severity, TriggerCondition: input.TriggerCondition,
	}
	if input.Annotations != nil {
		input.Rule.Annotations = input.Annotations
	}
	if input.EventDetail != "" {
		input.Rule.Note = input.EventDetail
	}
	event, err := repo.upsertActiveEvent(input.Rule, datasource, sample, model.EventStatusFiring, false, now, existingPtr)
	if err != nil {
		return ExternalEventResult{}, err
	}
	notified, notifyErr := repo.sendNotification(ctx, input.Rule, event, model.EventStatusFiring, now)
	return ExternalEventResult{EventID: event.ID, Notified: len(notified) > 0}, notifyErr
}

func externalRepository(db *gorm.DB) *repository {
	externalRepositories.Lock()
	defer externalRepositories.Unlock()
	if repo := externalRepositories.items[db]; repo != nil {
		return repo
	}
	repo := newRepository(db)
	externalRepositories.items[db] = repo
	return repo
}
