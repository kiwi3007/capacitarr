-- +goose Up
-- Remove the Library entity — an unused abstraction layer between DiskGroups
-- and Integrations. The Library table, its FK columns on other tables, and
-- associated indexes are dropped.
--
-- SQLite requires indexes to be dropped BEFORE the column they reference,
-- otherwise DROP COLUMN fails with "error in index after drop column".

-- 1. Drop indexes on library_id columns first
DROP INDEX IF EXISTS idx_integration_configs_library_id;
DROP INDEX IF EXISTS idx_custom_rules_library_id;
DROP INDEX IF EXISTS idx_library_histories_library_id;

-- 2. Drop the library_id FK columns
ALTER TABLE integration_configs DROP COLUMN library_id;
ALTER TABLE custom_rules DROP COLUMN library_id;
ALTER TABLE library_histories DROP COLUMN library_id;

-- 3. Drop the libraries table and its index
DROP INDEX IF EXISTS idx_libraries_disk_group_id;
DROP TABLE IF EXISTS libraries;

-- +goose Down
-- Recreate the libraries table and restore FK columns

CREATE TABLE libraries (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL,
    threshold_pct REAL    DEFAULT NULL,
    target_pct    REAL    DEFAULT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_libraries_disk_group_id ON libraries(disk_group_id);

ALTER TABLE integration_configs ADD COLUMN library_id INTEGER REFERENCES libraries(id) ON DELETE SET NULL;
CREATE INDEX idx_integration_configs_library_id ON integration_configs(library_id);

ALTER TABLE custom_rules ADD COLUMN library_id INTEGER REFERENCES libraries(id) ON DELETE CASCADE;
CREATE INDEX idx_custom_rules_library_id ON custom_rules(library_id);

ALTER TABLE library_histories ADD COLUMN library_id INTEGER REFERENCES libraries(id) ON DELETE CASCADE;
CREATE INDEX idx_library_histories_library_id ON library_histories(library_id);
