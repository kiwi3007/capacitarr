-- +goose Up
ALTER TABLE approval_queue ADD COLUMN force_delete BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE approval_queue DROP COLUMN force_delete;
