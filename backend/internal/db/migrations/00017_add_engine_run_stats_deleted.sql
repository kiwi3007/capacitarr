-- +goose Up
ALTER TABLE engine_run_stats ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; handled by full rebuild if needed.
