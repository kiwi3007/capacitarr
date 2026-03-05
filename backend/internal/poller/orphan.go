package poller

import (
	"log/slog"
	"time"

	"capacitarr/internal/db"
)

// RecoverOrphanedApprovals reverts any approval queue entries stuck in "approved"
// status back to "pending". This handles the case where a user approved an item
// but the deletion worker never processed it (e.g. the server restarted before
// the deletion queue was drained). Called on startup and at the beginning of
// each poll cycle.
func RecoverOrphanedApprovals() {
	type orphan struct {
		ID        uint
		MediaName string
	}

	var orphans []orphan
	if err := db.DB.Raw("SELECT id, media_name FROM approval_queue WHERE status = ?", db.StatusApproved).Scan(&orphans).Error; err != nil {
		slog.Error("Failed to query orphaned approvals", "component", "poller", "operation", "recover_orphans", "error", err)
		return
	}

	if len(orphans) == 0 {
		return
	}

	for _, o := range orphans {
		slog.Info("Recovering orphaned approval back to queue",
			"component", "poller",
			"id", o.ID,
			"media", o.MediaName,
		)
	}

	if err := db.DB.Model(&db.ApprovalQueueItem{}).
		Where("status = ?", db.StatusApproved).
		Updates(map[string]interface{}{
			"status":     db.StatusPending,
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
		slog.Error("Failed to revert orphaned approvals", "component", "poller", "operation", "recover_orphans", "error", err)
		return
	}

	slog.Info("Orphaned approvals recovered", "component", "poller", "count", len(orphans))
}
