package integrations

import (
	"log/slog"
	"sort"
)

// Enricher is a composable enrichment step that augments media items with data
// from external services. Each enricher wraps one or more integration clients.
// Adding a new enrichment source = one file implementing Enricher.
type Enricher interface {
	// Name returns the human-readable name for logging.
	Name() string
	// Priority returns the execution order (lower = earlier). Enrichers with
	// the same priority run in registration order.
	Priority() int
	// Enrich augments items in-place with data from the enricher's source.
	// Non-fatal errors are logged and do not stop the pipeline.
	Enrich(items []MediaItem) error
}

// EnrichmentPipeline runs a sequence of enrichers in priority order.
type EnrichmentPipeline struct {
	enrichers []Enricher
}

// NewEnrichmentPipeline creates an empty pipeline.
func NewEnrichmentPipeline() *EnrichmentPipeline {
	return &EnrichmentPipeline{}
}

// Add registers an enricher in the pipeline.
func (p *EnrichmentPipeline) Add(e Enricher) {
	p.enrichers = append(p.enrichers, e)
}

// Run executes all enrichers in priority order. Failures are logged but do not
// stop the pipeline — subsequent enrichers still run.
func (p *EnrichmentPipeline) Run(items []MediaItem) {
	if len(items) == 0 || len(p.enrichers) == 0 {
		return
	}

	// Sort by priority (stable sort preserves registration order for same priority)
	sorted := make([]Enricher, len(p.enrichers))
	copy(sorted, p.enrichers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})

	for _, e := range sorted {
		slog.Info("Running enricher", "component", "enrichment", "enricher", e.Name(),
			"priority", e.Priority(), "itemCount", len(items))
		if err := e.Enrich(items); err != nil {
			slog.Warn("Enrichment failed", "component", "enrichment",
				"enricher", e.Name(), "error", err)
		}
	}
}

// Count returns the number of registered enrichers.
func (p *EnrichmentPipeline) Count() int {
	return len(p.enrichers)
}
