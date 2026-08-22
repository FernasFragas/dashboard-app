package store

import (
	"context"
	"testing"
)

func TestCreateMetric(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	m, err := s.CreateMetric(ctx, NewMetric{
		Name:  "p95 latency (cached)",
		Value: 0.74,
		Unit:  ptr("s"),
		Note:  ptr("after cache warm"),
	})
	if err != nil {
		t.Fatalf("create metric: %v", err)
	}

	if m.Value != 0.74 {
		t.Errorf("value = %v, want 0.74", m.Value)
	}
	if m.RecordedAt != fixedNow.Format("2006-01-02T15:04:05Z") {
		t.Errorf("recorded_at = %q, want the clock's value", m.RecordedAt)
	}
}

// Name is deliberately not a foreign key onto metric_defs: the Review screen's "other..."
// field must accept a new metric without a migration.
func TestCreateMetricAcceptsUnknownName(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	m, err := s.CreateMetric(ctx, NewMetric{Name: "a metric the plan never listed", Value: 1})
	if err != nil {
		t.Fatalf("create metric with a free-text name: %v", err)
	}

	if m.Name != "a metric the plan never listed" {
		t.Errorf("name = %q, want the free-text value", m.Name)
	}
}

func TestListMetricsFilters(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	readings := []NewMetric{
		{Name: "p95 latency (cached)", Value: 1.2, RecordedAt: "2026-09-20T09:00:00Z"},
		{Name: "p95 latency (cached)", Value: 0.9, RecordedAt: "2026-09-22T09:00:00Z"},
		{Name: "Cache hit rate", Value: 62, RecordedAt: "2026-09-22T09:00:00Z"},
	}

	for _, r := range readings {
		if _, err := s.CreateMetric(ctx, r); err != nil {
			t.Fatalf("create metric: %v", err)
		}
	}

	byName, err := s.ListMetrics(ctx, MetricFilter{Name: "p95 latency (cached)"})
	if err != nil {
		t.Fatalf("list by name: %v", err)
	}
	if len(byName) != 2 {
		t.Fatalf("readings = %d, want 2", len(byName))
	}
	if byName[0].Value != 0.9 {
		t.Errorf("first value = %v, want the newest reading", byName[0].Value)
	}

	bounded, err := s.ListMetrics(ctx, MetricFilter{From: "2026-09-21T00:00:00Z"})
	if err != nil {
		t.Fatalf("list by range: %v", err)
	}
	if len(bounded) != 2 {
		t.Errorf("readings in range = %d, want 2", len(bounded))
	}

	limited, err := s.ListMetrics(ctx, MetricFilter{Limit: 1})
	if err != nil {
		t.Fatalf("list with limit: %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("limited readings = %d, want 1", len(limited))
	}
}

func TestListMetricDefs(t *testing.T) {
	defs, err := newTestStore(t).ListMetricDefs(context.Background())
	if err != nil {
		t.Fatalf("list metric defs: %v", err)
	}

	if len(defs) != 1 {
		t.Fatalf("defs = %d, want 1 fixture", len(defs))
	}

	if defs[0].Target == nil || *defs[0].Target != "<0.8s" {
		t.Errorf("target = %v, want the plan's prose value", defs[0].Target)
	}
}
