package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

type deleteJob struct {
	client  integrations.Integration
	item    integrations.MediaItem
	reason  string
	score   float64
	factors []engine.ScoreFactor
}

var deleteQueue = make(chan deleteJob, 500)

var (
	metricsProcessed int64
	metricsFailed    int64

	// Currently-deleting item name (atomic.Value storing string)
	currentlyDeletingVal atomic.Value
)

// init starts the background deletion worker before anything else.
func init() {
	go deletionWorker()
}

func deletionWorker() {
	// Rate limit: 1 deletion every 3 seconds to protect disk I/O, burst of 1.
	// This is much smarter than arbitrary sleeps, as it smooths out load dynamically.
	limiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

	for job := range deleteQueue {
		// Wait blocks until a token is available
		_ = limiter.Wait(context.Background()) //nolint:errcheck // context.Background() never cancels

		currentlyDeletingVal.Store(job.item.Title)

		// ╔══════════════════════════════════════════════════════════╗
		// ║  SAFETY GUARD: Deletions are disabled until testing     ║
		// ║  Remove this block when ready for production testing.   ║
		// ╚══════════════════════════════════════════════════════════╝
		slog.Warn("SAFETY GUARD: Delete skipped (deletions disabled in codebase)",
			"component", "poller",
			"item", job.item.Title,
			"type", job.item.Type,
			"size", job.item.SizeBytes,
			"score", job.score,
		)
		currentlyDeletingVal.Store("")
		atomic.AddInt64(&metricsProcessed, 1)

		// Still log to audit as "Dry-Delete" so the UI shows activity
		factorsJSON, _ := json.Marshal(job.factors) //nolint:errcheck // marshal of known-safe struct
		logEntry := db.AuditLog{
			MediaName:    job.item.Title,
			MediaType:    string(job.item.Type),
			Reason:       fmt.Sprintf("Score: %.2f (%s)", job.score, job.reason),
			ScoreDetails: string(factorsJSON),
			Action:       "Dry-Delete",
			SizeBytes:    job.item.SizeBytes,
			CreatedAt:    time.Now(),
		}

		/* DISABLED: Actual deletion — uncomment when ready for production testing
		if err := job.client.DeleteMediaItem(job.item); err != nil {
			slog.Error("Background deletion failed", "component", "poller", "operation", "delete_media", "item", job.item.Title, "error", err)
			atomic.AddInt64(&metricsFailed, 1)
			currentlyDeletingVal.Store("")
			continue
		}

		currentlyDeletingVal.Store("")
		atomic.AddInt64(&metricsProcessed, 1)

		// Increment lifetime stats (atomic DB update, not for dry-runs)
		db.DB.Model(&db.LifetimeStats{}).Where("id = 1").
			UpdateColumns(map[string]interface{}{
				"total_bytes_reclaimed": gorm.Expr("total_bytes_reclaimed + ?", job.item.SizeBytes),
				"total_items_removed":   gorm.Expr("total_items_removed + ?", 1),
			})

		factorsJSON, _ := json.Marshal(job.factors)
		logEntry := db.AuditLog{
			MediaName:    job.item.Title,
			MediaType:    string(job.item.Type),
			Reason:       fmt.Sprintf("Score: %.2f (%s)", job.score, job.reason),
			ScoreDetails: string(factorsJSON),
			Action:       "Deleted",
			SizeBytes:    job.item.SizeBytes,
			CreatedAt:    time.Now(),
		}
		*/
		if err := db.DB.Create(&logEntry).Error; err != nil {
			slog.Error("Failed to create audit log entry", "component", "poller", "operation", "create_audit_log", "error", err)
		}

		slog.Info("Background engine action completed", "component", "poller",
			"media", job.item.Title, "action", "Deleted", "freed", job.item.SizeBytes)
	}
}
