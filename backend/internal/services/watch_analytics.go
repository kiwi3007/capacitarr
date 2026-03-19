package services

import (
	"math"
	"sort"
	"strings"
	"time"

	"capacitarr/internal/integrations"
)

// WatchAnalyticsService provides watch-intelligence analytics (dead content,
// stale content, popularity, request fulfillment). Requires enrichment data
// from a media server — items without enrichment are excluded to avoid
// false positives.
type WatchAnalyticsService struct {
	preview PreviewDataSource
}

// NewWatchAnalyticsService creates a new WatchAnalyticsService.
func NewWatchAnalyticsService(preview PreviewDataSource) *WatchAnalyticsService {
	return &WatchAnalyticsService{preview: preview}
}

// ─── Dead content ───────────────────────────────────────────────────────────

// DeadContentItem is an item that has never been watched and isn't on a watchlist.
type DeadContentItem struct {
	Title         string `json:"title"`
	Type          string `json:"type"`
	SizeBytes     int64  `json:"sizeBytes"`
	DaysInLibrary int    `json:"daysInLibrary"`
	IntegrationID uint   `json:"integrationId"`
}

// DeadContentReport is the response for the dead content analytics endpoint.
type DeadContentReport struct {
	Items      []DeadContentItem `json:"items"`
	TotalCount int               `json:"totalCount"`
	TotalSize  int64             `json:"totalSize"`
}

// GetDeadContent returns items with PlayCount == 0, not on watchlist,
// and added more than minAgeDays ago.
func (s *WatchAnalyticsService) GetDeadContent(minAgeDays int) *DeadContentReport {
	items := s.preview.GetCachedItems()
	now := time.Now()
	minAge := time.Duration(minAgeDays) * 24 * time.Hour

	var dead []DeadContentItem
	var totalSize int64

	for _, item := range items {
		// Only include items that have enrichment data (to avoid false positives)
		if !hasEnrichmentData(item) {
			continue
		}
		if item.PlayCount > 0 || item.OnWatchlist {
			continue
		}
		if item.AddedAt == nil || now.Sub(*item.AddedAt) < minAge {
			continue
		}

		daysInLib := int(now.Sub(*item.AddedAt).Hours() / 24)
		dead = append(dead, DeadContentItem{
			Title:         item.Title,
			Type:          string(item.Type),
			SizeBytes:     item.SizeBytes,
			DaysInLibrary: daysInLib,
			IntegrationID: item.IntegrationID,
		})
		totalSize += item.SizeBytes
	}

	// Sort by size descending (biggest dead items first)
	sort.Slice(dead, func(i, j int) bool {
		return dead[i].SizeBytes > dead[j].SizeBytes
	})

	return &DeadContentReport{
		Items:      dead,
		TotalCount: len(dead),
		TotalSize:  totalSize,
	}
}

// ─── Stale content ──────────────────────────────────────────────────────────

// StaleContentItem is an item that was watched but hasn't been touched in a long time.
type StaleContentItem struct {
	Title            string  `json:"title"`
	Type             string  `json:"type"`
	SizeBytes        int64   `json:"sizeBytes"`
	DaysSinceWatched int     `json:"daysSinceWatched"`
	PlayCount        int     `json:"playCount"`
	StalenessScore   float64 `json:"stalenessScore"`
	IntegrationID    uint    `json:"integrationId"`
}

// StaleContentReport is the response for the stale content analytics endpoint.
type StaleContentReport struct {
	Items      []StaleContentItem `json:"items"`
	TotalCount int                `json:"totalCount"`
	TotalSize  int64              `json:"totalSize"`
}

// GetStaleContent returns items where LastPlayed > staleDays ago and PlayCount > 0.
func (s *WatchAnalyticsService) GetStaleContent(staleDays int) *StaleContentReport {
	items := s.preview.GetCachedItems()
	now := time.Now()
	staleDuration := time.Duration(staleDays) * 24 * time.Hour

	var stale []StaleContentItem
	var totalSize int64

	for _, item := range items {
		if !hasEnrichmentData(item) {
			continue
		}
		if item.PlayCount == 0 || item.LastPlayed == nil {
			continue
		}
		if now.Sub(*item.LastPlayed) < staleDuration {
			continue
		}

		daysSince := int(now.Sub(*item.LastPlayed).Hours() / 24)
		score := stalenessScore(item, daysSince)

		stale = append(stale, StaleContentItem{
			Title:            item.Title,
			Type:             string(item.Type),
			SizeBytes:        item.SizeBytes,
			DaysSinceWatched: daysSince,
			PlayCount:        item.PlayCount,
			StalenessScore:   math.Round(score*100) / 100,
			IntegrationID:    item.IntegrationID,
		})
		totalSize += item.SizeBytes
	}

	// Sort by staleness score descending
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].StalenessScore > stale[j].StalenessScore
	})

	return &StaleContentReport{
		Items:      stale,
		TotalCount: len(stale),
		TotalSize:  totalSize,
	}
}

// ─── Popularity ─────────────────────────────────────────────────────────────

// PopularityEntry is a genre×year cell in the popularity heatmap.
type PopularityEntry struct {
	Genre     string `json:"genre"`
	Year      string `json:"year"`
	PlayCount int    `json:"playCount"`
	ItemCount int    `json:"itemCount"`
}

// RankedItem is a single item in the top/bottom lists.
type RankedItem struct {
	Title     string `json:"title"`
	PlayCount int    `json:"playCount"`
	SizeBytes int64  `json:"sizeBytes"`
}

// PopularityData holds heatmap data and ranked lists.
type PopularityData struct {
	Heatmap  []PopularityEntry `json:"heatmap"`
	TopItems []RankedItem      `json:"topItems"`
	LowItems []RankedItem      `json:"lowItems"`
}

// GetPopularity returns popularity analytics (heatmap + ranked lists).
func (s *WatchAnalyticsService) GetPopularity() *PopularityData {
	items := s.preview.GetCachedItems()

	// Build heatmap: genre × decade → play count
	type heatKey struct{ genre, decade string }
	heatmap := make(map[heatKey]*PopularityEntry)
	var enrichedItems []integrations.MediaItem

	for _, item := range items {
		if !hasEnrichmentData(item) {
			continue
		}
		enrichedItems = append(enrichedItems, item)

		genre := item.Genre
		if genre == "" {
			genre = unknownLabel
		}
		decade := decadeLabel(item.Year)
		key := heatKey{genre, decade}
		if _, ok := heatmap[key]; !ok {
			heatmap[key] = &PopularityEntry{Genre: genre, Year: decade}
		}
		heatmap[key].PlayCount += item.PlayCount
		heatmap[key].ItemCount++
	}

	heatmapSlice := make([]PopularityEntry, 0, len(heatmap))
	for _, v := range heatmap {
		heatmapSlice = append(heatmapSlice, *v)
	}

	// Ranked lists
	sort.Slice(enrichedItems, func(i, j int) bool {
		return enrichedItems[i].PlayCount > enrichedItems[j].PlayCount
	})

	topN := 20
	if len(enrichedItems) < topN {
		topN = len(enrichedItems)
	}
	topItems := make([]RankedItem, topN)
	for i := 0; i < topN; i++ {
		topItems[i] = RankedItem{
			Title:     enrichedItems[i].Title,
			PlayCount: enrichedItems[i].PlayCount,
			SizeBytes: enrichedItems[i].SizeBytes,
		}
	}

	// Bottom items (least watched, excluding unwatched)
	var watchedItems []integrations.MediaItem
	for _, item := range enrichedItems {
		if item.PlayCount > 0 {
			watchedItems = append(watchedItems, item)
		}
	}
	sort.Slice(watchedItems, func(i, j int) bool {
		return watchedItems[i].PlayCount < watchedItems[j].PlayCount
	})
	lowN := 20
	if len(watchedItems) < lowN {
		lowN = len(watchedItems)
	}
	lowItems := make([]RankedItem, lowN)
	for i := 0; i < lowN; i++ {
		lowItems[i] = RankedItem{
			Title:     watchedItems[i].Title,
			PlayCount: watchedItems[i].PlayCount,
			SizeBytes: watchedItems[i].SizeBytes,
		}
	}

	return &PopularityData{
		Heatmap:  heatmapSlice,
		TopItems: topItems,
		LowItems: lowItems,
	}
}

// ─── Request fulfillment ────────────────────────────────────────────────────

// RequestFulfillmentData holds request fulfillment statistics.
type RequestFulfillmentData struct {
	TotalRequested   int             `json:"totalRequested"`
	Fulfilled        int             `json:"fulfilled"`      // Watched by requestor
	Unfulfilled      int             `json:"unfulfilled"`    // Not watched by requestor
	FulfillmentPct   float64         `json:"fulfillmentPct"` // 0-100
	UnfulfilledItems []RequestedItem `json:"unfulfilledItems"`
}

// RequestedItem is a requested media item with fulfillment status.
type RequestedItem struct {
	Title              string `json:"title"`
	RequestedBy        string `json:"requestedBy"`
	WatchedByRequestor bool   `json:"watchedByRequestor"`
	SizeBytes          int64  `json:"sizeBytes"`
}

// GetRequestFulfillment returns request fulfillment analytics.
func (s *WatchAnalyticsService) GetRequestFulfillment() *RequestFulfillmentData {
	items := s.preview.GetCachedItems()

	var totalRequested, fulfilled int
	var unfulfilled []RequestedItem

	for _, item := range items {
		if !item.IsRequested {
			continue
		}
		totalRequested++
		if item.WatchedByRequestor {
			fulfilled++
		} else {
			unfulfilled = append(unfulfilled, RequestedItem{
				Title:              item.Title,
				RequestedBy:        item.RequestedBy,
				WatchedByRequestor: false,
				SizeBytes:          item.SizeBytes,
			})
		}
	}

	pct := 0.0
	if totalRequested > 0 {
		pct = math.Round(float64(fulfilled)/float64(totalRequested)*10000) / 100
	}

	return &RequestFulfillmentData{
		TotalRequested:   totalRequested,
		Fulfilled:        fulfilled,
		Unfulfilled:      totalRequested - fulfilled,
		FulfillmentPct:   pct,
		UnfulfilledItems: unfulfilled,
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// hasEnrichmentData returns true if the item has been through the enrichment
// pipeline (has watch data or watchlist status). Items without enrichment
// should be excluded from watch analytics to avoid false positives.
func hasEnrichmentData(item integrations.MediaItem) bool {
	return item.PlayCount > 0 || item.LastPlayed != nil || item.OnWatchlist || item.IsRequested || len(item.WatchedByUsers) > 0
}

// stalenessScore calculates a staleness score for content that was watched
// but hasn't been touched in a long time.
// Formula: daysSinceLastPlayed / 365 * (seriesEnded ? 1.5 : 1.0) * (!onWatchlist ? 1.2 : 0.5)
func stalenessScore(item integrations.MediaItem, daysSince int) float64 {
	base := float64(daysSince) / 365.0

	// Ended series are more stale
	statusMultiplier := 1.0
	if strings.ToLower(item.SeriesStatus) == "ended" {
		statusMultiplier = 1.5
	}

	// Watchlisted items are less stale
	watchlistMultiplier := 1.2
	if item.OnWatchlist {
		watchlistMultiplier = 0.5
	}

	return base * statusMultiplier * watchlistMultiplier
}
