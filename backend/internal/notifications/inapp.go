package notifications

import (
	"fmt"
	"time"

	"capacitarr/internal/db"
)

// severityForEvent maps event types to in-app notification severity levels.
func severityForEvent(eventType string) string {
	switch eventType {
	case EventThresholdBreach:
		return "warning"
	case EventDeletionExecuted:
		return "info"
	case EventEngineError:
		return "error"
	case EventEngineComplete:
		return "success"
	default:
		return "info"
	}
}

// SendInApp creates an InAppNotification record in the database.
func SendInApp(event NotificationEvent) error {
	record := db.InAppNotification{
		Title:     event.Title,
		Message:   event.Message,
		Severity:  severityForEvent(event.Type),
		EventType: event.Type,
		CreatedAt: time.Now(),
	}

	if err := db.DB.Create(&record).Error; err != nil {
		return fmt.Errorf("create in-app notification: %w", err)
	}

	return nil
}
