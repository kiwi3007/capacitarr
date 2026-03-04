-- +goose Up
ALTER TABLE auth_configs ADD COLUMN api_key_hint TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; handled by full rebuild if needed.
