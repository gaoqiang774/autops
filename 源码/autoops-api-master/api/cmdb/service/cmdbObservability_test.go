package service

import (
	"errors"
	"testing"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	logtailmodel "dodevops-api/api/logtail/model"
)

func TestParseObservabilityDurationSupportsDays(t *testing.T) {
	duration, err := parseObservabilityDuration("7d")
	if err != nil {
		t.Fatalf("parseObservabilityDuration() error = %v", err)
	}
	if duration != 7*24*time.Hour {
		t.Fatalf("duration = %v, want 7d", duration)
	}

	_, err = parseObservabilityDuration("31d")
	if !errors.Is(err, ErrInvalidObservabilityWindow) {
		t.Fatalf("31d error = %v, want ErrInvalidObservabilityWindow", err)
	}
}

func TestObservabilityProjectIDsDoNotExpandLogStoreOwnerProject(t *testing.T) {
	props := map[string]interface{}{
		"storeId":   float64(9),
		"projectId": float64(1),
	}
	ids := observabilityProjectIDsFromNode(cmdbmodel.CmdbResourceTypeLogStore, cmdbmodel.CmdbResourceTypeLogStore, props)
	if len(ids) != 0 {
		t.Fatalf("log store owner project expanded as a bound project: %v", ids)
	}

	props["logProjectId"] = float64(2)
	ids = observabilityProjectIDsFromNode(cmdbmodel.CmdbResourceTypeLogStore, cmdbmodel.CmdbResourceTypeLogStore, props)
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("explicit logProjectId not detected: %v", ids)
	}
}

func TestObservabilityProjectNodeUsesProjectIDMetadata(t *testing.T) {
	ids := observabilityProjectIDsFromNode(cmdbmodel.CmdbResourceTypeLogProject, cmdbmodel.CmdbResourceTypeLogProject, map[string]interface{}{
		"projectId": float64(3),
	})
	if len(ids) != 1 || ids[0] != 3 {
		t.Fatalf("log project projectId not detected: %v", ids)
	}
}

func TestObservabilityRelatedNodeIDsOnlyPropagateDedicatedLogNodes(t *testing.T) {
	incoming := map[uint][]uint{
		61: {60},
		75: {69},
	}

	nginxNodes := observabilityRelatedNodeIDs(61, cmdbmodel.CmdbResourceTypeNginx, incoming)
	if len(nginxNodes) != 1 || nginxNodes[0] != 61 {
		t.Fatalf("nginx metrics propagated to upstream nodes: %v", nginxNodes)
	}

	logNodes := observabilityRelatedNodeIDs(75, cmdbmodel.CmdbResourceTypeLogStore, incoming)
	if len(logNodes) != 2 || logNodes[0] != 69 || logNodes[1] != 75 {
		t.Fatalf("dedicated log node did not associate its parent resource: %v", logNodes)
	}
}

func TestAggregateBusinessSLAMetrics(t *testing.T) {
	observations := []businessStoreObservation{
		{Snapshot: &logtailmodel.LogSLAStoreSnapshot{Metrics: logtailmodel.LogSLAMetrics{
			RequestTotal: 100, HTTP4xx: 5, HTTP5xx: 1, SlowRequestCount: 3, AvgLatencyMs: 100, P95LatencyMs: 500,
		}}},
		{Snapshot: &logtailmodel.LogSLAStoreSnapshot{Metrics: logtailmodel.LogSLAMetrics{
			RequestTotal: 300, HTTP4xx: 15, HTTP5xx: 3, SlowRequestCount: 9, AvgLatencyMs: 200, P95LatencyMs: 900,
		}}},
	}
	topology := map[string]interface{}{
		"rootId": uint(1),
		"nodes": []map[string]interface{}{{
			"id":         uint(1),
			"properties": map[string]interface{}{"sloTarget": 99.0},
		}},
	}

	metrics := aggregateBusinessSLAMetrics(observations, topology)
	if metrics.RequestTotal != 400 || metrics.HTTP4xx != 20 || metrics.HTTP5xx != 4 {
		t.Fatalf("unexpected counts: %#v", metrics)
	}
	if metrics.AvgLatencyMs != 175 || metrics.P95LatencyMs != 900 {
		t.Fatalf("unexpected latency aggregation: %#v", metrics)
	}
	if metrics.AvailabilityRate != 99 || !metrics.SLOMet || metrics.SLOStatus != "met" {
		t.Fatalf("unexpected SLO result: %#v", metrics)
	}
	if metrics.ErrorBudgetConsumedPercent != 100 {
		t.Fatalf("error budget consumed = %v, want 100", metrics.ErrorBudgetConsumedPercent)
	}
}

func TestBuildBusinessNodeSignalsUsesWorstP95(t *testing.T) {
	observations := []businessStoreObservation{
		{
			Binding: businessStoreBinding{Store: logtailmodel.LogStore{ID: 10}, NodeIDs: []uint{2}},
			Snapshot: &logtailmodel.LogSLAStoreSnapshot{Metrics: logtailmodel.LogSLAMetrics{
				RequestTotal: 100, HTTP5xx: 1, AvgLatencyMs: 100, P95LatencyMs: 500,
			}},
		},
		{
			Binding: businessStoreBinding{Store: logtailmodel.LogStore{ID: 11}, NodeIDs: []uint{2}},
			Snapshot: &logtailmodel.LogSLAStoreSnapshot{Metrics: logtailmodel.LogSLAMetrics{
				RequestTotal: 100, HTTP5xx: 9, AvgLatencyMs: 300, P95LatencyMs: 3500,
			}},
		},
	}

	signals := buildBusinessNodeSignals(observations)
	if len(signals) != 1 {
		t.Fatalf("signals length = %d, want 1", len(signals))
	}
	if signals[0].P95LatencyMs != 3500 || signals[0].AvgLatencyMs != 200 {
		t.Fatalf("unexpected latency: %#v", signals[0])
	}
	if signals[0].HealthStatus != cmdbmodel.CmdbHealthStatusCritical {
		t.Fatalf("health status = %s, want critical", signals[0].HealthStatus)
	}
}
