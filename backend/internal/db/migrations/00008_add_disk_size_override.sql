-- +goose Up
ALTER TABLE disk_groups ADD COLUMN total_bytes_override INTEGER DEFAULT NULL;

-- +goose Down
ALTER TABLE disk_groups DROP COLUMN total_bytes_override;
