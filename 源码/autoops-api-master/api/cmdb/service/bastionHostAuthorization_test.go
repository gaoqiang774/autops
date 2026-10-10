package service

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/util"
	"testing"
	"time"
)

func TestNormalizeAuthorizationWindowDefaultsEffectiveTime(t *testing.T) {
	expireAt := &util.HTime{Time: time.Now().Add(2 * time.Hour)}

	effectiveAt, normalizedExpireAt, err := normalizeAuthorizationWindow(nil, expireAt)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if effectiveAt == nil {
		t.Fatal("expected effective time to be defaulted")
	}
	if normalizedExpireAt == nil || !normalizedExpireAt.Time.Equal(expireAt.Time) {
		t.Fatal("expected expire time to be preserved")
	}
	if effectiveAt.Time.After(time.Now().Add(2 * time.Second)) {
		t.Fatal("expected effective time to default to now")
	}
}

func TestNormalizeAuthorizationWindowRejectsInvalidExpireTime(t *testing.T) {
	effectiveAt := &util.HTime{Time: time.Now().Add(time.Hour)}
	expireAt := &util.HTime{Time: time.Now().Add(30 * time.Minute)}

	if _, _, err := normalizeAuthorizationWindow(effectiveAt, expireAt); err == nil {
		t.Fatal("expected error when expire time is not after effective time")
	}
}

func TestMergeAuthorizationInputsUsesPerHostLifecycle(t *testing.T) {
	sharedExpireTime := &util.HTime{Time: time.Date(2026, 4, 30, 0, 0, 0, 0, time.Local)}
	hostEffectiveTime := &util.HTime{Time: time.Date(2026, 4, 25, 9, 0, 0, 0, time.Local)}
	hostExpireTime := &util.HTime{Time: time.Date(2026, 5, 1, 9, 0, 0, 0, time.Local)}

	inputs, err := mergeAuthorizationInputs(
		[]uint{1, 2},
		nil,
		nil,
		sharedExpireTime,
		[]model.AuthorizationHostLifecycleDto{
			{
				HostID:        2,
				EffectiveTime: hostEffectiveTime,
				ExpireTime:    hostExpireTime,
			},
			{
				HostID: 3,
			},
		},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(inputs) != 3 {
		t.Fatalf("expected 3 inputs, got %d", len(inputs))
	}

	if inputs[1].HostID != 2 {
		t.Fatalf("expected second input to be host 2, got %d", inputs[1].HostID)
	}
	if inputs[1].EffectiveTime == nil || !inputs[1].EffectiveTime.Time.Equal(hostEffectiveTime.Time) {
		t.Fatal("expected host 2 to use its own effective time")
	}
	if inputs[1].ExpireTime == nil || !inputs[1].ExpireTime.Time.Equal(hostExpireTime.Time) {
		t.Fatal("expected host 2 to use its own expire time")
	}

	if inputs[2].HostID != 3 {
		t.Fatalf("expected third input to be host 3, got %d", inputs[2].HostID)
	}
	if inputs[2].ExpireTime == nil || !inputs[2].ExpireTime.Time.Equal(sharedExpireTime.Time) {
		t.Fatal("expected host 3 to inherit shared expire time")
	}
}

func TestMergeAuthorizationInputsRejectsDuplicateDetailedHost(t *testing.T) {
	_, err := mergeAuthorizationInputs(
		nil,
		nil,
		nil,
		nil,
		[]model.AuthorizationHostLifecycleDto{
			{HostID: 1},
			{HostID: 1},
		},
	)
	if err == nil {
		t.Fatal("expected duplicate detailed host to fail")
	}
}
