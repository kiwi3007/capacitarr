-- +goose Up
ALTER TABLE protection_rules ADD COLUMN sort_order INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite doesn't support DROP COLUMN
