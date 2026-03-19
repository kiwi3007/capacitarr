package services

import (
	"testing"
	"time"

	"capacitarr/internal/integrations"
)

// mockPreviewSource provides test data for analytics services.
type mockPreviewSource struct {
	items []integrations.MediaItem
}

func (m *mockPreviewSource) GetCachedItems() []integrations.MediaItem {
	return m.items
}

func sampleItems() []integrations.MediaItem {
	now := time.Now()
	sixMonthsAgo := now.Add(-180 * 24 * time.Hour)
	oneYearAgo := now.Add(-365 * 24 * time.Hour)

	return []integrations.MediaItem{
		{
			Title: "Serenity", Type: integrations.MediaTypeMovie,
			SizeBytes: 15 * 1024 * 1024 * 1024, Year: 2005,
			QualityProfile: "HD-1080p", Genre: "Sci-Fi", Rating: 7.4,
			PlayCount: 5, LastPlayed: &sixMonthsAgo,
			AddedAt: &oneYearAgo, IntegrationID: 1,
		},
		{
			Title: "Firefly", Type: integrations.MediaTypeShow,
			SizeBytes: 40 * 1024 * 1024 * 1024, Year: 2002,
			QualityProfile: "HD-1080p", Genre: "Sci-Fi", Rating: 9.0,
			PlayCount: 0, AddedAt: &oneYearAgo, IntegrationID: 1,
			SeriesStatus: "ended",
			OnWatchlist:  false, // Explicitly enriched — set to false by media server
			// To signal enrichment happened, we need at least one enrichment field set.
			// Use LastPlayed with zero time to indicate "checked but never played"
			LastPlayed: func() *time.Time { t := time.Time{}; return &t }(),
		},
		{
			Title: "The Expanse", Type: integrations.MediaTypeShow,
			SizeBytes: 60 * 1024 * 1024 * 1024, Year: 2015,
			QualityProfile: "HD-720p", Genre: "Sci-Fi", Rating: 8.5,
			PlayCount: 3, LastPlayed: &sixMonthsAgo,
			AddedAt: &sixMonthsAgo, IntegrationID: 2,
			IsRequested: true, RequestedBy: "mal", WatchedByRequestor: true,
		},
		{
			Title: "Unknown Movie", Type: integrations.MediaTypeMovie,
			SizeBytes: 2 * 1024 * 1024 * 1024, Year: 0,
			IntegrationID: 1,
		},
	}
}

func TestAnalyticsService_GetComposition(t *testing.T) {
	svc := NewAnalyticsService(&mockPreviewSource{items: sampleItems()})

	data := svc.GetComposition()
	if data.TotalItems != 4 {
		t.Errorf("expected 4 total items, got %d", data.TotalItems)
	}
	if data.TotalSizeBytes == 0 {
		t.Error("expected non-zero total size")
	}
	if len(data.QualityDistribution) == 0 {
		t.Error("expected non-empty quality distribution")
	}
	if len(data.GenreDistribution) == 0 {
		t.Error("expected non-empty genre distribution")
	}
	if len(data.TypeDistribution) == 0 {
		t.Error("expected non-empty type distribution")
	}
}

func TestAnalyticsService_GetCompositionEmpty(t *testing.T) {
	svc := NewAnalyticsService(&mockPreviewSource{items: nil})

	data := svc.GetComposition()
	if data.TotalItems != 0 {
		t.Errorf("expected 0 items for empty cache, got %d", data.TotalItems)
	}
}

func TestAnalyticsService_GetQualityDistribution(t *testing.T) {
	svc := NewAnalyticsService(&mockPreviewSource{items: sampleItems()})

	data := svc.GetQualityDistribution()
	if len(data.Profiles) == 0 {
		t.Error("expected non-empty profiles")
	}

	// HD-1080p should have 2 items (Serenity + Firefly)
	for _, p := range data.Profiles {
		if p.Name == "HD-1080p" && p.Count != 2 {
			t.Errorf("expected 2 items for HD-1080p, got %d", p.Count)
		}
	}
}

func TestAnalyticsService_GetSizeAnomalies(t *testing.T) {
	// Create items where one is clearly bloated for its quality
	items := []integrations.MediaItem{
		{Title: "Normal 720p", QualityProfile: "HD-720p", SizeBytes: 5 * 1024 * 1024 * 1024},
		{Title: "Normal 720p 2", QualityProfile: "HD-720p", SizeBytes: 6 * 1024 * 1024 * 1024},
		{Title: "Normal 720p 3", QualityProfile: "HD-720p", SizeBytes: 4 * 1024 * 1024 * 1024},
		{Title: "Bloated 720p", QualityProfile: "HD-720p", SizeBytes: 30 * 1024 * 1024 * 1024}, // 6x median
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	anomalies := svc.GetSizeAnomalies()

	if len(anomalies) == 0 {
		t.Error("expected at least one size anomaly")
	}
	if len(anomalies) > 0 && anomalies[0].Title != "Bloated 720p" {
		t.Errorf("expected 'Bloated 720p' as worst offender, got %q", anomalies[0].Title)
	}
}

func TestAnalyticsService_GetSizeAnomaliesEmpty(t *testing.T) {
	svc := NewAnalyticsService(&mockPreviewSource{items: nil})
	anomalies := svc.GetSizeAnomalies()
	if len(anomalies) != 0 {
		t.Errorf("expected 0 anomalies for empty cache, got %d", len(anomalies))
	}
}

// ─── Watch analytics tests ──────────────────────────────────────────────────

func TestWatchAnalyticsService_GetDeadContent(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	// Firefly: PlayCount=0, not on watchlist, added 1 year ago — should be "dead"
	report := svc.GetDeadContent(90)

	if report.TotalCount == 0 {
		t.Error("expected at least one dead content item")
	}

	found := false
	for _, item := range report.Items {
		if item.Title == "Firefly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Firefly to appear in dead content report")
	}
}

func TestWatchAnalyticsService_GetStaleContent(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	report := svc.GetStaleContent(90)

	// Serenity and The Expanse were watched 180 days ago — stale if threshold is 90 days
	if report.TotalCount == 0 {
		t.Error("expected at least one stale content item")
	}
}

func TestWatchAnalyticsService_GetPopularity(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	data := svc.GetPopularity()
	if len(data.TopItems) == 0 {
		t.Error("expected non-empty top items")
	}
}

func TestWatchAnalyticsService_GetRequestFulfillment(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	data := svc.GetRequestFulfillment()
	if data.TotalRequested != 1 {
		t.Errorf("expected 1 requested item, got %d", data.TotalRequested)
	}
	if data.Fulfilled != 1 {
		t.Errorf("expected 1 fulfilled, got %d", data.Fulfilled)
	}
	if data.FulfillmentPct != 100 {
		t.Errorf("expected 100%% fulfillment, got %.1f%%", data.FulfillmentPct)
	}
}

func TestWatchAnalyticsService_GetDeadContentEmpty(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: nil})
	report := svc.GetDeadContent(90)
	if report.TotalCount != 0 {
		t.Errorf("expected 0 dead items for empty cache, got %d", report.TotalCount)
	}
}
