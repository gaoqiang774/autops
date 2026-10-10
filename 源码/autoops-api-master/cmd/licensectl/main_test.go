package main

import (
	"testing"
	"time"
)

func TestParseExpirationPresets(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.Local)
	tests := []struct {
		value string
		want  time.Time
	}{
		{value: "3m", want: now.AddDate(0, 3, 0)},
		{value: "6m", want: now.AddDate(0, 6, 0)},
		{value: "1y", want: now.AddDate(1, 0, 0)},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := parseExpiration(now, tt.value, "")
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("expiration = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseExpirationUnlimitedAndCustomDate(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.Local)
	unlimited, err := parseExpiration(now, "unlimited", "")
	if err != nil || !unlimited.IsZero() {
		t.Fatalf("unlimited = %v, %v", unlimited, err)
	}
	custom, err := parseExpiration(now, "", "2026-08-31")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 31, 23, 59, 59, 0, time.Local)
	if !custom.Equal(want) {
		t.Fatalf("custom expiration = %v, want %v", custom, want)
	}
}
