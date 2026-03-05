package routes

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/poller"
)

// RegisterApprovalRoutes sets up the API endpoints for the approval queue.
func RegisterApprovalRoutes(g *echo.Group, database *gorm.DB) {
	// List approval queue items
	g.GET("/approval-queue", func(c echo.Context) error {
		limit := 200
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 2000 {
			limit = 2000
		}

		items := make([]db.ApprovalQueueItem, 0)
		query := database.Model(&db.ApprovalQueueItem{})

		if status := c.QueryParam("status"); status != "" {
			query = query.Where("status = ?", status)
		}

		if err := query.Order("created_at desc").Limit(limit).Find(&items).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch approval queue"})
		}

		return c.JSON(http.StatusOK, items)
	})

	// Approve a queued item: queue it for deletion
	g.POST("/approval-queue/:id/approve", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.ApprovalQueueItem
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
		}

		if entry.Status != db.StatusPending {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Entry is not pending",
			})
		}

		// Safety check: block approvals when deletions are disabled
		var prefs db.PreferenceSet
		if err := database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to load preferences for approval check", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to load preferences",
			})
		}
		if !prefs.DeletionsEnabled {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "Deletions are currently disabled in settings. Enable deletions before approving items.",
			})
		}

		// Look up the integration to construct a client
		var integration db.IntegrationConfig
		if err := database.First(&integration, entry.IntegrationID).Error; err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Integration not found",
			})
		}

		client := poller.CreateClient(integration.Type, integration.URL, integration.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Unsupported integration type",
			})
		}

		// Reconstruct the MediaItem from stored approval data
		item := integrations.MediaItem{
			ExternalID:    entry.ExternalID,
			IntegrationID: entry.IntegrationID,
			Type:          integrations.MediaType(entry.MediaType),
			Title:         entry.MediaName,
			SizeBytes:     entry.SizeBytes,
		}

		// Parse stored score details back into factors
		var factors []engine.ScoreFactor
		if entry.ScoreDetails != "" {
			if err := json.Unmarshal([]byte(entry.ScoreDetails), &factors); err != nil {
				slog.Warn("Failed to parse score details for approval", "id", entry.ID, "error", err)
			}
		}

		// Queue for background deletion
		if err := poller.QueueDeletion(client, item, entry.Reason, 0, factors, 0); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{
				"error": "Deletion queue is full, try again later",
			})
		}

		// Update queue entry to "approved"
		if err := database.Model(&entry).Update("status", db.StatusApproved).Error; err != nil {
			slog.Error("Failed to update queue entry to approved", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to update queue entry",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "approved"})
	})

	// Reject a queued item: snooze it
	g.POST("/approval-queue/:id/reject", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.ApprovalQueueItem
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
		}

		if entry.Status != db.StatusPending {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Entry is not pending",
			})
		}

		// Load preferences to get configured snooze duration
		var prefs db.PreferenceSet
		if err := database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to load preferences for snooze duration", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to load preferences",
			})
		}

		snoozedUntil := prefs.SnoozeDurationHours

		var svc db.ApprovalQueueItem
		database.First(&svc, id) // Refresh

		// Use direct DB update since we're still wiring services
		if err := database.Model(&entry).Updates(map[string]interface{}{
			"status":        db.StatusRejected,
			"snoozed_until": snoozedUntil,
		}).Error; err != nil {
			slog.Error("Failed to reject queue entry", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to reject queue entry",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "rejected"})
	})

	// Unsnooze a rejected item: clear snooze and reset to pending
	g.POST("/approval-queue/:id/unsnooze", func(c echo.Context) error {
		id := c.Param("id")

		var entry db.ApprovalQueueItem
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Approval queue entry not found"})
		}

		if entry.Status != db.StatusRejected {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Entry is not rejected/snoozed",
			})
		}

		if err := database.Model(&entry).Updates(map[string]interface{}{
			"snoozed_until": nil,
			"status":        db.StatusPending,
		}).Error; err != nil {
			slog.Error("Failed to unsnooze queue entry", "id", entry.ID, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to unsnooze queue entry",
			})
		}

		// Reload the updated entry
		if err := database.First(&entry, id).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reload queue entry"})
		}

		return c.JSON(http.StatusOK, entry)
	})
}
