// Package services contains business logic separated from HTTP handlers.
// Services are injectable and publish typed events to the event bus.
package services

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// ApprovalService manages the approval queue lifecycle.
type ApprovalService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(database *gorm.DB, bus *events.EventBus) *ApprovalService {
	return &ApprovalService{db: database, bus: bus}
}

// Approve marks a pending item as approved and returns it.
func (s *ApprovalService) Approve(entryID uint) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("entry is not pending (current status: %s)", entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
		"status":     db.StatusApproved,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to approve entry: %w", err)
	}

	s.bus.Publish(events.ApprovalApprovedEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
		SizeBytes: entry.SizeBytes,
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// Reject marks a pending item as rejected and sets a snooze duration.
func (s *ApprovalService) Reject(entryID uint, snoozeDurationHours int) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("entry is not pending (current status: %s)", entry.Status)
	}

	snoozedUntil := time.Now().UTC().Add(time.Duration(snoozeDurationHours) * time.Hour)

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
		"status":        db.StatusRejected,
		"snoozed_until": snoozedUntil,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to reject entry: %w", err)
	}

	s.bus.Publish(events.ApprovalRejectedEvent{
		EntryID:        entry.ID,
		MediaName:      entry.MediaName,
		MediaType:      entry.MediaType,
		SnoozeDuration: fmt.Sprintf("%dh", snoozeDurationHours),
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// Unsnooze clears the snooze on a rejected item and resets it to pending.
func (s *ApprovalService) Unsnooze(entryID uint) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusRejected {
		return nil, fmt.Errorf("entry is not rejected (current status: %s)", entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
		"status":        db.StatusPending,
		"snoozed_until": nil,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to unsnooze entry: %w", err)
	}

	s.bus.Publish(events.ApprovalUnsnoozedEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// BulkUnsnooze clears all active snoozes and resets items to pending.
// This is called when disk usage drops below threshold.
func (s *ApprovalService) BulkUnsnooze() (int, error) {
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ? AND snoozed_until IS NOT NULL", db.StatusRejected).
		Updates(map[string]interface{}{
			"status":        db.StatusPending,
			"snoozed_until": nil,
			"updated_at":    time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to bulk unsnooze: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalBulkUnsnoozedEvent{Count: count})
		slog.Info("Bulk unsnoozed approval items", "component", "services", "count", count)
	}

	return count, nil
}

// CleanExpiredSnoozes clears stale snoozed_until values where the snooze has expired,
// and resets those items to pending.
func (s *ApprovalService) CleanExpiredSnoozes() (int, error) {
	now := time.Now().UTC()
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ? AND snoozed_until IS NOT NULL AND snoozed_until <= ?", db.StatusRejected, now).
		Updates(map[string]interface{}{
			"status":        db.StatusPending,
			"snoozed_until": nil,
			"updated_at":    now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to clean expired snoozes: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		slog.Info("Cleaned expired snoozes", "component", "services", "count", count)
	}

	return count, nil
}

// RecoverOrphans finds approved items that have no corresponding active deletion
// and requeues them as pending. This handles recovery after crashes or restarts.
func (s *ApprovalService) RecoverOrphans() (int, error) {
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ?", db.StatusApproved).
		Updates(map[string]interface{}{
			"status":     db.StatusPending,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to recover orphaned approvals: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalOrphansRecoveredEvent{Count: count})
		slog.Info("Recovered orphaned approval items", "component", "services", "count", count)
	}

	return count, nil
}
