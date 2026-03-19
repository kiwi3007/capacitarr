package services

import (
	"fmt"
	"math"
	"sort"

	"capacitarr/internal/integrations"
)

const unknownLabel = "Unknown"

// AnalyticsService provides library composition and quality analytics.
// All computations run over the in-memory preview cache — no DB queries needed.
type AnalyticsService struct {
	preview PreviewDataSource
}

// PreviewDataSource is the interface for accessing preview cache data.
// Satisfied by PreviewService.
type PreviewDataSource interface {
	GetCachedItems() []integrations.MediaItem
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(preview PreviewDataSource) *AnalyticsService {
	return &AnalyticsService{preview: preview}
}

// ─── Composition analytics ──────────────────────────────────────────────────

// CompositionData holds all composition breakdown data.
type CompositionData struct {
	QualityDistribution []NameCount `json:"qualityDistribution"`
	GenreDistribution   []NameCount `json:"genreDistribution"`
	YearDistribution    []NameCount `json:"yearDistribution"`
	TypeDistribution    []NameCount `json:"typeDistribution"`
	TotalItems          int         `json:"totalItems"`
	TotalSizeBytes      int64       `json:"totalSizeBytes"`
}

// NameCount is a label + count pair for chart data.
type NameCount struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"sizeBytes"`
}

// GetComposition returns library composition breakdowns.
func (s *AnalyticsService) GetComposition() *CompositionData {
	items := s.preview.GetCachedItems()

	qualityMap := make(map[string]*NameCount)
	genreMap := make(map[string]*NameCount)
	yearMap := make(map[string]*NameCount)
	typeMap := make(map[string]*NameCount)

	var totalSize int64
	for _, item := range items {
		totalSize += item.SizeBytes

		// Quality
		qp := item.QualityProfile
		if qp == "" {
			qp = unknownLabel
		}
		if _, ok := qualityMap[qp]; !ok {
			qualityMap[qp] = &NameCount{Name: qp}
		}
		qualityMap[qp].Count++
		qualityMap[qp].SizeBytes += item.SizeBytes

		// Genre
		genre := item.Genre
		if genre == "" {
			genre = unknownLabel
		}
		if _, ok := genreMap[genre]; !ok {
			genreMap[genre] = &NameCount{Name: genre}
		}
		genreMap[genre].Count++
		genreMap[genre].SizeBytes += item.SizeBytes

		// Year (grouped by decade)
		yearStr := decadeLabel(item.Year)
		if _, ok := yearMap[yearStr]; !ok {
			yearMap[yearStr] = &NameCount{Name: yearStr}
		}
		yearMap[yearStr].Count++
		yearMap[yearStr].SizeBytes += item.SizeBytes

		// Type
		t := string(item.Type)
		if t == "" {
			t = "unknown"
		}
		if _, ok := typeMap[t]; !ok {
			typeMap[t] = &NameCount{Name: t}
		}
		typeMap[t].Count++
		typeMap[t].SizeBytes += item.SizeBytes
	}

	return &CompositionData{
		QualityDistribution: sortedNameCounts(qualityMap),
		GenreDistribution:   sortedNameCounts(genreMap),
		YearDistribution:    sortedNameCounts(yearMap),
		TypeDistribution:    sortedNameCounts(typeMap),
		TotalItems:          len(items),
		TotalSizeBytes:      totalSize,
	}
}

// ─── Quality analytics ──────────────────────────────────────────────────────

// QualityDistribution holds detailed quality breakdown data.
type QualityDistribution struct {
	Profiles []QualityProfile `json:"profiles"`
}

// QualityProfile is a quality tier with count and size.
type QualityProfile struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"sizeBytes"`
}

// GetQualityDistribution returns detailed quality profile breakdown.
func (s *AnalyticsService) GetQualityDistribution() *QualityDistribution {
	items := s.preview.GetCachedItems()
	profileMap := make(map[string]*QualityProfile)

	for _, item := range items {
		qp := item.QualityProfile
		if qp == "" {
			qp = unknownLabel
		}
		if _, ok := profileMap[qp]; !ok {
			profileMap[qp] = &QualityProfile{Name: qp}
		}
		profileMap[qp].Count++
		profileMap[qp].SizeBytes += item.SizeBytes
	}

	profiles := make([]QualityProfile, 0, len(profileMap))
	for _, p := range profileMap {
		profiles = append(profiles, *p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Count > profiles[j].Count
	})

	return &QualityDistribution{Profiles: profiles}
}

// ─── Bloat detection ────────────────────────────────────────────────────────

// SizeAnomaly represents an item whose size is anomalous for its quality profile.
type SizeAnomaly struct {
	Title          string  `json:"title"`
	QualityProfile string  `json:"qualityProfile"`
	SizeBytes      int64   `json:"sizeBytes"`
	MedianBytes    int64   `json:"medianBytes"`
	Ratio          float64 `json:"ratio"` // item size / median size
	IntegrationID  uint    `json:"integrationId"`
}

// GetSizeAnomalies returns items that are > 2x the median size for their quality profile.
func (s *AnalyticsService) GetSizeAnomalies() []SizeAnomaly {
	items := s.preview.GetCachedItems()

	// Group sizes by quality profile
	profileSizes := make(map[string][]int64)
	profileItems := make(map[string][]integrations.MediaItem)
	for _, item := range items {
		if item.SizeBytes == 0 {
			continue
		}
		qp := item.QualityProfile
		if qp == "" {
			continue // Skip items with unknown quality profile
		}
		profileSizes[qp] = append(profileSizes[qp], item.SizeBytes)
		profileItems[qp] = append(profileItems[qp], item)
	}

	var anomalies []SizeAnomaly
	for qp, sizes := range profileSizes {
		if len(sizes) < 3 {
			continue // Need at least 3 items for meaningful median
		}
		median := medianInt64(sizes)
		if median == 0 {
			continue
		}
		for _, item := range profileItems[qp] {
			ratio := float64(item.SizeBytes) / float64(median)
			if ratio > 2.0 {
				anomalies = append(anomalies, SizeAnomaly{
					Title:          item.Title,
					QualityProfile: qp,
					SizeBytes:      item.SizeBytes,
					MedianBytes:    median,
					Ratio:          math.Round(ratio*100) / 100,
					IntegrationID:  item.IntegrationID,
				})
			}
		}
	}

	// Sort by ratio descending (worst offenders first)
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].Ratio > anomalies[j].Ratio
	})

	return anomalies
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func decadeLabel(year int) string {
	if year <= 0 {
		return unknownLabel
	}
	decade := (year / 10) * 10
	return fmt.Sprintf("%ds", decade)
}

func sortedNameCounts(m map[string]*NameCount) []NameCount {
	result := make([]NameCount, 0, len(m))
	for _, v := range m {
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})
	return result
}

func medianInt64(vals []int64) int64 {
	sorted := make([]int64, len(vals))
	copy(sorted, vals)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}
