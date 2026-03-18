package integrations

import (
	"fmt"
	"testing"
)

// ─── Test enricher doubles ──────────────────────────────────────────────────

type mockEnricher struct {
	name     string
	priority int
	enrichFn func(items []MediaItem) error
}

func (m *mockEnricher) Name() string  { return m.name }
func (m *mockEnricher) Priority() int { return m.priority }
func (m *mockEnricher) Enrich(items []MediaItem) error {
	if m.enrichFn != nil {
		return m.enrichFn(items)
	}
	return nil
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestEnrichmentPipeline_RunOrder(t *testing.T) {
	pipeline := NewEnrichmentPipeline()

	// Track execution order
	var order []string

	pipeline.Add(&mockEnricher{
		name:     "Third",
		priority: 30,
		enrichFn: func(_ []MediaItem) error {
			order = append(order, "Third")
			return nil
		},
	})
	pipeline.Add(&mockEnricher{
		name:     "First",
		priority: 10,
		enrichFn: func(_ []MediaItem) error {
			order = append(order, "First")
			return nil
		},
	})
	pipeline.Add(&mockEnricher{
		name:     "Second",
		priority: 20,
		enrichFn: func(_ []MediaItem) error {
			order = append(order, "Second")
			return nil
		},
	})

	items := []MediaItem{{Title: "Serenity"}}
	pipeline.Run(items)

	if len(order) != 3 {
		t.Fatalf("expected 3 enrichers to run, got %d", len(order))
	}
	if order[0] != "First" || order[1] != "Second" || order[2] != "Third" {
		t.Errorf("expected [First, Second, Third], got %v", order)
	}
}

func TestEnrichmentPipeline_FailureContinues(t *testing.T) {
	pipeline := NewEnrichmentPipeline()

	ran := make(map[string]bool)

	pipeline.Add(&mockEnricher{
		name:     "Succeeds",
		priority: 10,
		enrichFn: func(_ []MediaItem) error {
			ran["Succeeds"] = true
			return nil
		},
	})
	pipeline.Add(&mockEnricher{
		name:     "Fails",
		priority: 20,
		enrichFn: func(_ []MediaItem) error {
			ran["Fails"] = true
			return fmt.Errorf("connection timeout")
		},
	})
	pipeline.Add(&mockEnricher{
		name:     "AlsoSucceeds",
		priority: 30,
		enrichFn: func(_ []MediaItem) error {
			ran["AlsoSucceeds"] = true
			return nil
		},
	})

	items := []MediaItem{{Title: "Firefly"}}
	pipeline.Run(items)

	// All three should have run despite the failure
	for _, name := range []string{"Succeeds", "Fails", "AlsoSucceeds"} {
		if !ran[name] {
			t.Errorf("enricher %q did not run", name)
		}
	}
}

func TestEnrichmentPipeline_EmptyItems(t *testing.T) {
	pipeline := NewEnrichmentPipeline()
	ran := false
	pipeline.Add(&mockEnricher{
		name:     "ShouldNotRun",
		priority: 10,
		enrichFn: func(_ []MediaItem) error {
			ran = true
			return nil
		},
	})

	pipeline.Run(nil)
	if ran {
		t.Error("enricher should not run for nil items")
	}

	pipeline.Run([]MediaItem{})
	if ran {
		t.Error("enricher should not run for empty items")
	}
}

func TestEnrichmentPipeline_Count(t *testing.T) {
	pipeline := NewEnrichmentPipeline()
	if pipeline.Count() != 0 {
		t.Errorf("expected 0 enrichers, got %d", pipeline.Count())
	}

	pipeline.Add(&mockEnricher{name: "A", priority: 1})
	pipeline.Add(&mockEnricher{name: "B", priority: 2})
	if pipeline.Count() != 2 {
		t.Errorf("expected 2 enrichers, got %d", pipeline.Count())
	}
}

func TestEnrichmentPipeline_EnricherModifiesItems(t *testing.T) {
	pipeline := NewEnrichmentPipeline()

	pipeline.Add(&mockEnricher{
		name:     "SetPlayCount",
		priority: 10,
		enrichFn: func(items []MediaItem) error {
			for i := range items {
				items[i].PlayCount = 42
			}
			return nil
		},
	})

	items := []MediaItem{{Title: "Serenity", PlayCount: 0}}
	pipeline.Run(items)

	if items[0].PlayCount != 42 {
		t.Errorf("expected PlayCount 42, got %d", items[0].PlayCount)
	}
}
