package service

import (
	"testing"
	"time"

	"dodevops-api/api/costcenter/model"
)

func TestBalanceAlertConditions(t *testing.T) {
	days := 4.5
	account := model.BillCloudAccount{BalanceAlertThreshold: 1000, BalanceAlertThresholdDays: 7}
	amount, availableDays := balanceAlertConditions(account, 900, &days)
	if !amount || !availableDays {
		t.Fatalf("expected both balance conditions to fire, got amount=%v days=%v", amount, availableDays)
	}
	amount, availableDays = balanceAlertConditions(account, 1000, nil)
	if amount || availableDays {
		t.Fatalf("threshold equality or unavailable estimate must not fire, got amount=%v days=%v", amount, availableDays)
	}
}

func TestBalanceDue(t *testing.T) {
	now := time.Date(2026, 7, 28, 10, 0, 0, 0, time.Local)
	last := now.Add(-61 * time.Minute)
	item := model.BillCloudAccount{
		Enabled: 1, BalanceSyncEnabled: 1, BalanceSyncIntervalMinutes: 60,
		BalanceStatus: model.BalanceStatusNormal, LastBalanceTime: &last,
	}
	if !balanceDue(&item, now) {
		t.Fatal("balance refresh should be due after configured interval")
	}
	item.BalanceStatus = model.BalanceStatusRefreshing
	if balanceDue(&item, now) {
		t.Fatal("refreshing cloud account must not be scheduled again")
	}
}
