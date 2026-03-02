package poller

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
)

// setupStatsTestDB creates an in-memory SQLite database with migrations applied,
// seeds default preferences, and sets the global db.DB pointer for use by
// GetWorkerMetrics (which reads from the package-level db.DB).
func setupStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default preferences (mirrors db.Init behaviour)
	pref := db.PreferenceSet{
		ID:                  1,
		ExecutionMode:       "dry-run",
		LogLevel:            "info",
		PollIntervalSeconds: 300,
	}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	// Set the global db.DB used by GetWorkerMetrics
	db.DB = database

	return database
}

func TestGetWorkerMetrics_EmptyState(t *testing.T) {
	setupStatsTestDB(t)

	metrics := GetWorkerMetrics()

	// Verify expected keys exist
	requiredKeys := []string{
		"executionMode", "isRunning", "pollIntervalSeconds",
		"queueDepth", "lastRunEvaluated", "lastRunFlagged",
		"lastRunFreedBytes", "lastRunEpoch", "currentlyDeleting",
		"protectedCount", "evaluated", "actioned", "freedBytes",
		"processed", "failed",
	}
	for _, key := range requiredKeys {
		if _, ok := metrics[key]; !ok {
			t.Errorf("Expected key %q in metrics, but it was missing", key)
		}
	}

	// Verify defaults
	if mode, ok := metrics["executionMode"].(string); !ok || mode != "dry-run" {
		t.Errorf("Expected executionMode 'dry-run', got %v", metrics["executionMode"])
	}
	if running, ok := metrics["isRunning"].(bool); !ok || running {
		t.Errorf("Expected isRunning=false, got %v", metrics["isRunning"])
	}
	if interval, ok := metrics["pollIntervalSeconds"].(int); !ok || interval != 300 {
		t.Errorf("Expected pollIntervalSeconds=300, got %v", metrics["pollIntervalSeconds"])
	}

	// With no engine runs, cumulative totals should be zero
	if evaluated, ok := metrics["evaluated"].(int64); !ok || evaluated != 0 {
		t.Errorf("Expected evaluated=0, got %v", metrics["evaluated"])
	}
	if actioned, ok := metrics["actioned"].(int64); !ok || actioned != 0 {
		t.Errorf("Expected actioned=0, got %v", metrics["actioned"])
	}
	if freed, ok := metrics["freedBytes"].(int64); !ok || freed != 0 {
		t.Errorf("Expected freedBytes=0, got %v", metrics["freedBytes"])
	}
}

func TestGetWorkerMetrics_WithEngineRuns(t *testing.T) {
	database := setupStatsTestDB(t)

	// Seed some engine run stats
	runs := []db.EngineRunStats{
		{
			RunAt:         time.Now().Add(-2 * time.Hour),
			Evaluated:     100,
			Flagged:       10,
			FreedBytes:    5000000000,
			ExecutionMode: "dry-run",
			DurationMs:    1200,
		},
		{
			RunAt:         time.Now().Add(-1 * time.Hour),
			Evaluated:     150,
			Flagged:       20,
			FreedBytes:    8000000000,
			ExecutionMode: "dry-run",
			DurationMs:    1500,
		},
	}
	for _, run := range runs {
		if err := database.Create(&run).Error; err != nil {
			t.Fatalf("Failed to seed engine run: %v", err)
		}
	}

	metrics := GetWorkerMetrics()

	// Cumulative totals should sum both runs
	evaluated := metrics["evaluated"].(int64)
	if evaluated != 250 {
		t.Errorf("Expected cumulative evaluated=250, got %d", evaluated)
	}

	actioned := metrics["actioned"].(int64)
	if actioned != 30 {
		t.Errorf("Expected cumulative actioned=30, got %d", actioned)
	}

	freed := metrics["freedBytes"].(int64)
	if freed != 13000000000 {
		t.Errorf("Expected cumulative freedBytes=13000000000, got %d", freed)
	}

	// Last run values should come from the most recent run (by run_at DESC)
	lastRunEval := metrics["lastRunEvaluated"].(int64)
	if lastRunEval != 150 {
		t.Errorf("Expected lastRunEvaluated=150 (most recent), got %d", lastRunEval)
	}

	lastRunFlagged := metrics["lastRunFlagged"].(int64)
	if lastRunFlagged != 20 {
		t.Errorf("Expected lastRunFlagged=20 (most recent), got %d", lastRunFlagged)
	}

	lastRunFreed := metrics["lastRunFreedBytes"].(int64)
	if lastRunFreed != 8000000000 {
		t.Errorf("Expected lastRunFreedBytes=8000000000 (most recent), got %d", lastRunFreed)
	}
}

func TestGetWorkerMetrics_ExecutionModeFromPrefs(t *testing.T) {
	database := setupStatsTestDB(t)

	// Update execution mode to "auto"
	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").
		Update("execution_mode", "auto").Error; err != nil {
		t.Fatalf("Failed to update prefs: %v", err)
	}

	metrics := GetWorkerMetrics()

	if mode, ok := metrics["executionMode"].(string); !ok || mode != "auto" {
		t.Errorf("Expected executionMode 'auto', got %v", metrics["executionMode"])
	}
}
