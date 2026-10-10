package engine

import (
	"testing"
	"time"

	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
)

func TestNextFiringStatusUsesConsecutiveCountSemantics(t *testing.T) {
	firstTriggeredAt := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	w := &worker{rule: model.AlertRule{
		EvalIntervalSeconds: 60,
		ForDurationSeconds:  180,
	}}

	if got := w.nextFiringStatus(model.AlertActiveEvent{}, false, false, firstTriggeredAt); got != model.EventStatusPending {
		t.Fatalf("first evaluation status = %s, want %s", got, model.EventStatusPending)
	}

	existing := model.AlertActiveEvent{
		Status:           model.EventStatusPending,
		FirstTriggeredAt: util.HTime{Time: firstTriggeredAt},
	}
	if got := w.nextFiringStatus(existing, true, false, firstTriggeredAt.Add(60*time.Second)); got != model.EventStatusPending {
		t.Fatalf("second evaluation status = %s, want %s", got, model.EventStatusPending)
	}
	if got := w.nextFiringStatus(existing, true, false, firstTriggeredAt.Add(120*time.Second)); got != model.EventStatusFiring {
		t.Fatalf("third evaluation status = %s, want %s", got, model.EventStatusFiring)
	}
}

func TestNextFiringStatusFiresImmediatelyForOneConsecutiveHit(t *testing.T) {
	w := &worker{rule: model.AlertRule{
		EvalIntervalSeconds: 60,
		ForDurationSeconds:  60,
	}}

	got := w.nextFiringStatus(model.AlertActiveEvent{}, false, false, time.Now())

	if got != model.EventStatusFiring {
		t.Fatalf("status = %s, want %s", got, model.EventStatusFiring)
	}
}
