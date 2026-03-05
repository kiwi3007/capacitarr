package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/notifications"
)

// evaluateAndCleanDisk scores all media items on a disk group and, when the
// threshold is breached, queues the highest-scoring candidates for deletion.
func evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, serviceClients map[uint]integrations.Integration, runStatsID uint) {
	var prefs db.PreferenceSet
	if err := db.DB.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return
	}

	if group.TotalBytes == 0 {
		return
	}
	currentPct := float64(group.UsedBytes) / float64(group.TotalBytes) * 100
	if currentPct < group.ThresholdPct {
		slog.Debug("Disk within threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"threshold", group.ThresholdPct)

		// Auto-clear all active snoozes when below threshold — gives a clean slate
		// for the next cleanup cycle. Resets rejected items back to pending.
		result := db.DB.Model(&db.ApprovalQueueItem{}).
			Where("status = ? AND snoozed_until IS NOT NULL", db.StatusRejected).
			Updates(map[string]interface{}{
				"status":        db.StatusPending,
				"snoozed_until": nil,
				"updated_at":    time.Now().UTC(),
			})
		if result.Error != nil {
			slog.Error("Failed to clear active snoozes", "component", "poller", "error", result.Error)
		}

		return
	}

	slog.Info("Disk threshold breached, evaluating media for deletion", "component", "poller",
		"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct), "threshold", group.ThresholdPct)

	// Notify: threshold breached
	notifications.Dispatch(notifications.NotificationEvent{
		Type:    notifications.EventThresholdBreach,
		Title:   "Threshold Breached",
		Message: fmt.Sprintf("Disk %s is at %.1f%% capacity (threshold: %.0f%%)", group.MountPath, currentPct, group.ThresholdPct),
		Fields: map[string]string{
			"Disk Group": group.MountPath,
			"Usage":      fmt.Sprintf("%.1f%%", currentPct),
			"Threshold":  fmt.Sprintf("%.0f%%", group.ThresholdPct),
		},
	})

	// Filter items on this mount
	var diskItems []integrations.MediaItem
	for _, item := range allItems {
		if strings.HasPrefix(item.Path, group.MountPath) {
			diskItems = append(diskItems, item)
		}
	}

	slog.Debug("Items on disk mount", "component", "poller",
		"mount", group.MountPath, "itemCount", len(diskItems))

	var rules []db.CustomRule
	if err := db.DB.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return
	}

	// Evaluate
	evaluated := engine.EvaluateMedia(diskItems, prefs, rules)
	atomic.AddInt64(&lastRunEvaluated, int64(len(evaluated)))

	// Count protected items for dashboard stats
	protectedCount := 0
	for _, ev := range evaluated {
		if ev.IsProtected {
			atomic.AddInt64(&lastRunProtected, 1)
			protectedCount++
		}
	}

	// Sort by score descending
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].Score > evaluated[j].Score // highest score first
	})

	targetBytesToFree := int64((currentPct - group.TargetPct) / 100.0 * float64(group.TotalBytes))
	if targetBytesToFree <= 0 {
		return
	}

	slog.Debug("Evaluation summary", "component", "poller",
		"mount", group.MountPath,
		"evaluated", len(evaluated),
		"protected", protectedCount,
		"targetBytesToFree", targetBytesToFree)

	var bytesFreed int64

	// Pre-build set of shows that have season-level entries in the evaluation results.
	// When season entries exist, prefer them over show-level entries so each season
	// can be individually approved/snoozed/deleted in the approval queue.
	showsWithSeasons := make(map[string]bool)
	for _, ev := range evaluated {
		if ev.Item.Type == integrations.MediaTypeSeason && ev.Item.ShowTitle != "" {
			showsWithSeasons[ev.Item.ShowTitle] = true
		}
	}

	for _, ev := range evaluated {
		if bytesFreed >= targetBytesToFree {
			break
		}
		if ev.IsProtected || ev.Score <= 0 {
			continue
		}

		// Dedup: skip show-level entries when season entries exist for the same show.
		// Season entries allow granular per-season approval and deletion.
		if ev.Item.Type == integrations.MediaTypeShow {
			if showsWithSeasons[ev.Item.Title] {
				continue
			}
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		if prefs.ExecutionMode == "auto" {
			client, ok := serviceClients[ev.Item.IntegrationID]
			if ok && client != nil {
				// Queue for background deletion so we don't block the poller
				select {
				case deleteQueue <- deleteJob{
					client:     client,
					item:       ev.Item,
					reason:     ev.Reason,
					score:      ev.Score,
					factors:    ev.Factors,
					runStatsID: runStatsID,
				}:
					bytesFreed += ev.Item.SizeBytes
					continue // Skip the synchronous DB insert below, worker handles it
				default:
					slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", ev.Item.Title)
					continue
				}
			} else {
				slog.Error("Integration client not found for deletion", "component", "poller",
					"operation", "resolve_client", "integrationId", ev.Item.IntegrationID)
				continue
			}
		} else if prefs.ExecutionMode == "approval" {
			// Skip items that are currently snoozed (rejected with an active snooze window)
			var snoozedCount int64
			db.DB.Model(&db.ApprovalQueueItem{}).Where(
				"media_name = ? AND media_type = ? AND status = ? AND snoozed_until IS NOT NULL AND snoozed_until > ?",
				ev.Item.Title, string(ev.Item.Type), db.StatusRejected, time.Now().UTC(),
			).Count(&snoozedCount)
			if snoozedCount > 0 {
				slog.Debug("Skipping snoozed item", "component", "poller", "media", ev.Item.Title)
				continue
			}

			// Upsert into approval_queue: create or update pending items
			factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck
			var existing db.ApprovalQueueItem
			result := db.DB.Where(
				"media_name = ? AND media_type = ? AND status = ?",
				ev.Item.Title, string(ev.Item.Type), db.StatusPending,
			).First(&existing)
			if result.Error == nil {
				// Update existing pending entry
				db.DB.Model(&existing).Updates(map[string]interface{}{
					"reason":         fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
					"score_details":  string(factorsJSON),
					"size_bytes":     ev.Item.SizeBytes,
					"integration_id": ev.Item.IntegrationID,
					"external_id":    ev.Item.ExternalID,
					"updated_at":     time.Now().UTC(),
				})
			} else {
				// Create new pending entry
				db.DB.Create(&db.ApprovalQueueItem{
					MediaName:     ev.Item.Title,
					MediaType:     string(ev.Item.Type),
					Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
					ScoreDetails:  string(factorsJSON),
					SizeBytes:     ev.Item.SizeBytes,
					IntegrationID: ev.Item.IntegrationID,
					ExternalID:    ev.Item.ExternalID,
					Status:        db.StatusPending,
					CreatedAt:     time.Now().UTC(),
					UpdatedAt:     time.Now().UTC(),
				})
			}

			bytesFreed += ev.Item.SizeBytes
			atomic.AddInt64(&lastRunFlagged, 1)
			slog.Info("Engine action taken", "component", "poller",
				"media", ev.Item.Title, "action", "queued_for_approval", "score", ev.Score, "freed", ev.Item.SizeBytes)
			continue
		}

		// Dry-run mode: write to audit_log table
		factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck
		integrationID := ev.Item.IntegrationID
		logEntry := db.AuditLogEntry{
			MediaName:     ev.Item.Title,
			MediaType:     string(ev.Item.Type),
			Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
			ScoreDetails:  string(factorsJSON),
			Action:        db.ActionDryRun,
			SizeBytes:     ev.Item.SizeBytes,
			IntegrationID: &integrationID,
			CreatedAt:     time.Now().UTC(),
		}

		// Dry-run dedup: upsert instead of creating duplicates
		var existing db.AuditLogEntry
		result := db.DB.Where(
			"media_name = ? AND media_type = ? AND action = ?",
			ev.Item.Title, string(ev.Item.Type), db.ActionDryRun,
		).First(&existing)
		if result.Error == nil {
			db.DB.Model(&existing).Updates(map[string]interface{}{
				"reason":         logEntry.Reason,
				"score_details":  logEntry.ScoreDetails,
				"size_bytes":     logEntry.SizeBytes,
				"integration_id": logEntry.IntegrationID,
				"created_at":     logEntry.CreatedAt,
			})
		} else {
			db.DB.Create(&logEntry)
		}

		bytesFreed += ev.Item.SizeBytes
		atomic.AddInt64(&lastRunFlagged, 1)
		slog.Info("Engine action taken", "component", "poller",
			"media", ev.Item.Title, "action", db.ActionDryRun, "score", ev.Score, "freed", ev.Item.SizeBytes)
	}
}
