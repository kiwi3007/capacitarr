package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"capacitarr/internal/db"
)

// AuditLogService manages the append-only audit log (deletion/dry-run history).
type AuditLogService struct {
	db *gorm.DB
}

// NewAuditLogService creates a new AuditLogService.
func NewAuditLogService(database *gorm.DB) *AuditLogService {
	return &AuditLogService{db: database}
}

// Create appends a new audit log entry. Entries are immutable after creation.
func (s *AuditLogService) Create(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create audit log entry: %w", err)
	}
	return nil
}

// UpsertDryRun creates or updates a dry-run audit log entry.
// If an entry with the same media_name, media_type, and action already exists,
// it is updated. Otherwise, a new entry is created.
func (s *AuditLogService) UpsertDryRun(entry db.AuditLogEntry) error {
	entry.CreatedAt = time.Now().UTC()

	// Try to find an existing dry-run entry for the same media
	var existing db.AuditLogEntry
	result := s.db.Where(
		"media_name = ? AND media_type = ? AND action = ?",
		entry.MediaName, entry.MediaType, entry.Action,
	).First(&existing)

	if result.Error == nil {
		// Update existing entry
		return s.db.Model(&existing).Updates(map[string]interface{}{
			"reason":         entry.Reason,
			"score_details":  entry.ScoreDetails,
			"size_bytes":     entry.SizeBytes,
			"integration_id": entry.IntegrationID,
			"created_at":     entry.CreatedAt,
		}).Error
	}

	// Create new entry
	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error
}

// PruneOlderThan deletes audit log entries older than the given duration.
// Returns the number of entries deleted.
func (s *AuditLogService) PruneOlderThan(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil // 0 = keep forever
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.AuditLogEntry{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune audit log: %w", result.Error)
	}
	return result.RowsAffected, nil
}
