-- +goose Up
ALTER TABLE preference_sets ADD COLUMN check_for_updates BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; handled by full rebuild if needed.
