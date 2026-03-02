-- +goose Up
CREATE TABLE IF NOT EXISTS notification_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    webhook_url TEXT,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    on_threshold_breach BOOLEAN NOT NULL DEFAULT 1,
    on_deletion_executed BOOLEAN NOT NULL DEFAULT 1,
    on_engine_error BOOLEAN NOT NULL DEFAULT 1,
    on_engine_complete BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS in_app_notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    message TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    read BOOLEAN NOT NULL DEFAULT 0,
    event_type TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_in_app_notifications_read ON in_app_notifications(read);
CREATE INDEX idx_in_app_notifications_created_at ON in_app_notifications(created_at);

-- +goose Down
DROP TABLE IF EXISTS in_app_notifications;
DROP TABLE IF EXISTS notification_configs;
