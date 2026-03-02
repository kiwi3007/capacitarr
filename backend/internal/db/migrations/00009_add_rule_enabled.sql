-- +goose Up
ALTER TABLE protection_rules ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
-- SQLite doesn't support DROP COLUMN
