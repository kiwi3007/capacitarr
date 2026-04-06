-- +goose Up
ALTER TABLE preference_sets ADD COLUMN backup_retention_days INTEGER NOT NULL DEFAULT 7;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35.0; this is a best-effort rollback.
-- The column will remain but is harmless if unused.
