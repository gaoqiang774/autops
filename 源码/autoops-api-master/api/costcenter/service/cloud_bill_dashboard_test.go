package service

import (
	"reflect"
	"testing"
)

func TestOperatorForKeyType(t *testing.T) {
	tests := []struct {
		keyType int
		want    string
		wantErr bool
	}{
		{1, "aliyun", false},
		{2, "tencentcloud", false},
		{3, "baidu", false},
		{4, "huawei", false},
		{6, "volcengine", false},
		{5, "", true},
	}
	for _, test := range tests {
		got, err := operatorForKeyType(test.keyType)
		if got != test.want || (err != nil) != test.wantErr {
			t.Fatalf("operatorForKeyType(%d) = %q, %v", test.keyType, got, err)
		}
	}
}

func TestLastMonthsEndingAt(t *testing.T) {
	want := []string{"2025-11", "2025-12", "2026-01", "2026-02"}
	if got := lastMonthsEndingAt("2026-02", 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("lastMonthsEndingAt() = %#v, want %#v", got, want)
	}
}

func TestChangeRate(t *testing.T) {
	if got := changeRate(10, 0); got != nil {
		t.Fatalf("zero baseline should return nil, got %v", *got)
	}
	got := changeRate(125, 100)
	if got == nil || *got != 25 {
		t.Fatalf("changeRate() = %v, want 25", got)
	}
}

func TestNormalizeMonth(t *testing.T) {
	if got, err := normalizeMonth("2026-7"); err == nil || got != "" {
		t.Fatalf("non-padded month accepted: %q, %v", got, err)
	}
	if got, err := normalizeMonth("2026-07"); err != nil || got != "2026-07" {
		t.Fatalf("valid month rejected: %q, %v", got, err)
	}
}
