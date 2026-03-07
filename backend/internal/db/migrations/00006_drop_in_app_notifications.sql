-- +goose Up
-- Remove the in_app_notifications table entirely. In-app notifications are
-- redundant with the activity_events table (ActivityPersister) and have been
-- removed from the application. The activity log now serves as the sole
-- system event feed.
DROP TABLE IF EXISTS in_app_notifications;

-- +goose Down
CREATE TABLE IF NOT EXISTS in_app_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    read BOOLEAN DEFAULT FALSE,
    event_type TEXT NOT NULL,
    created_at DATETIME
);
