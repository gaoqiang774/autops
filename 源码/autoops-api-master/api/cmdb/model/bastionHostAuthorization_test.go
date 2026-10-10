package model

import (
	"dodevops-api/common/util"
	"testing"
	"time"
)

func TestResolveAuthorizationStatus(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.Local)

	futureEffective := &util.HTime{Time: now.Add(2 * time.Hour)}
	if status := ResolveAuthorizationStatus(futureEffective, nil, now); status != AuthorizationStatusPending {
		t.Fatalf("expected pending status, got %s", status)
	}

	activeExpire := &util.HTime{Time: now.Add(2 * time.Hour)}
	if status := ResolveAuthorizationStatus(nil, activeExpire, now); status != AuthorizationStatusActive {
		t.Fatalf("expected active status, got %s", status)
	}

	expiredAt := &util.HTime{Time: now.Add(-time.Minute)}
	if status := ResolveAuthorizationStatus(nil, expiredAt, now); status != AuthorizationStatusExpired {
		t.Fatalf("expected expired status, got %s", status)
	}
}
