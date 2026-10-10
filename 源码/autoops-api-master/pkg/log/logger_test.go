package log

import "testing"

func TestShouldSkipGinLogging(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "exact heartbeat path",
			path: "/api/v1/monitor/agent/heartbeat",
			want: true,
		},
		{
			name: "route prefix heartbeat path",
			path: "/api/v1/cmdb/agent/heartbeat",
			want: true,
		},
		{
			name: "logtail agent path",
			path: "/api/v1/logtail/agent/status",
			want: true,
		},
		{
			name: "route prefix logtail agent path",
			path: "/api/v1/logtail/agent/operations",
			want: true,
		},
		{
			name: "non skipped path",
			path: "/api/v1/logtail/victorialogs/health",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipGinLogging(tt.path); got != tt.want {
				t.Fatalf("shouldSkipGinLogging(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
