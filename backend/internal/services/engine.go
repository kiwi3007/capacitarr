package services

import (
	"sync/atomic"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// EngineService manages engine run triggers and stats.
type EngineService struct {
	db       *gorm.DB
	bus      *events.EventBus
	RunNowCh chan struct{} // Signals the poller to run immediately

	// Observable state
	lastEvaluated atomic.Int64
	lastFlagged   atomic.Int64
	lastProtected atomic.Int64
	pollRunning   atomic.Bool
}

// NewEngineService creates a new EngineService.
func NewEngineService(database *gorm.DB, bus *events.EventBus) *EngineService {
	return &EngineService{
		db:       database,
		bus:      bus,
		RunNowCh: make(chan struct{}, 1),
	}
}

// TriggerRun sends a signal to run the engine immediately.
// Returns "started" if the signal was sent, "already_running" if a run is in progress.
func (s *EngineService) TriggerRun() string {
	if s.pollRunning.Load() {
		return "already_running"
	}

	select {
	case s.RunNowCh <- struct{}{}:
		s.bus.Publish(events.ManualRunTriggeredEvent{})
		return "started"
	default:
		return "already_running"
	}
}

// SetRunning marks the engine as running or not running.
func (s *EngineService) SetRunning(running bool) {
	s.pollRunning.Store(running)
}

// IsRunning returns whether the engine is currently running.
func (s *EngineService) IsRunning() bool {
	return s.pollRunning.Load()
}

// SetLastRunStats updates the last run statistics.
func (s *EngineService) SetLastRunStats(evaluated, flagged, protected int) {
	s.lastEvaluated.Store(int64(evaluated))
	s.lastFlagged.Store(int64(flagged))
	s.lastProtected.Store(int64(protected))
}

// GetStats returns the current engine statistics as a map.
func (s *EngineService) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"running":       s.pollRunning.Load(),
		"lastEvaluated": s.lastEvaluated.Load(),
		"lastFlagged":   s.lastFlagged.Load(),
		"lastProtected": s.lastProtected.Load(),
	}

	// Get the latest run from the database
	var latest db.EngineRunStats
	if err := s.db.Order("run_at desc").First(&latest).Error; err == nil {
		stats["lastRunAt"] = latest.RunAt
		stats["lastDurationMs"] = latest.DurationMs
		stats["lastExecutionMode"] = latest.ExecutionMode
		stats["lastDeleted"] = latest.Deleted
		stats["lastFreedBytes"] = latest.FreedBytes
		if latest.ErrorMessage != "" {
			stats["lastError"] = latest.ErrorMessage
		}
	}

	return stats
}
