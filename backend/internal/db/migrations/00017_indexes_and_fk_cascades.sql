-- +goose Up
-- Add composite indexes for common query patterns (Finding 6.1) and fix
-- missing FK cascades on sunset_queue (Finding 6.2).

-- ============================================================================
-- Composite Indexes (Finding 6.1)
-- ============================================================================

-- Used by ApprovalService.ListQueue() filtering by status + disk group
CREATE INDEX IF NOT EXISTS idx_approval_queue_status_disk_group
    ON approval_queue(status, disk_group_id);

-- Used by ApprovalService.BulkUpsertPending() conflict resolution
CREATE INDEX IF NOT EXISTS idx_approval_queue_media_status
    ON approval_queue(media_name, media_type, status);

-- Used by SunsetService.ListSunsettedKeys() and Escalate()
CREATE INDEX IF NOT EXISTS idx_sunset_queue_disk_group_status
    ON sunset_queue(disk_group_id, status);

-- Used by MetricsService rollup queries
CREATE INDEX IF NOT EXISTS idx_library_histories_rollup
    ON library_histories(disk_group_id, resolution, timestamp);

-- ============================================================================
-- FK Cascades on sunset_queue (Finding 6.2)
-- ============================================================================
-- SQLite does not support ALTER TABLE ... ADD CONSTRAINT, so we must recreate
-- the table with the correct FK constraints and copy data.

-- Step 1: Create new table with proper cascades
CREATE TABLE sunset_queue_new (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name            TEXT NOT NULL,
    media_type            TEXT NOT NULL,
    tmdb_id               INTEGER,
    integration_id        INTEGER REFERENCES integration_configs(id) ON DELETE CASCADE,
    external_id           TEXT,
    size_bytes            INTEGER NOT NULL DEFAULT 0,
    score                 REAL NOT NULL DEFAULT 0,
    score_details         TEXT,
    poster_url            TEXT,
    disk_group_id         INTEGER NOT NULL REFERENCES disk_groups(id) ON DELETE CASCADE,
    collection_group      TEXT,
    trigger               TEXT NOT NULL DEFAULT 'engine',
    deletion_date         DATE NOT NULL,
    label_applied         INTEGER NOT NULL DEFAULT 0,
    poster_overlay_active INTEGER NOT NULL DEFAULT 0,
    expired_at            DATETIME,
    status                TEXT NOT NULL DEFAULT 'pending',
    saved_at              DATETIME,
    saved_score           REAL NOT NULL DEFAULT 0,
    saved_reason          TEXT,
    created_at            DATETIME,
    updated_at            DATETIME
);

-- Step 2: Copy data
INSERT INTO sunset_queue_new
    SELECT id, media_name, media_type, tmdb_id, integration_id, external_id,
           size_bytes, score, score_details, poster_url, disk_group_id,
           collection_group, "trigger", deletion_date, label_applied,
           poster_overlay_active, expired_at, status, saved_at, saved_score,
           saved_reason, created_at, updated_at
    FROM sunset_queue;

-- Step 3: Drop old table and rename
DROP TABLE sunset_queue;
ALTER TABLE sunset_queue_new RENAME TO sunset_queue;

-- Step 4: Recreate indexes (including the new composite one)
CREATE INDEX IF NOT EXISTS idx_sunset_queue_disk_group ON sunset_queue(disk_group_id);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_tmdb_id ON sunset_queue(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_media_name ON sunset_queue(media_name);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_deletion_date ON sunset_queue(deletion_date);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_disk_group_status ON sunset_queue(disk_group_id, status);


-- +goose Down
-- Drop composite indexes (originals are restored by the baseline + earlier migrations)
DROP INDEX IF EXISTS idx_approval_queue_status_disk_group;
DROP INDEX IF EXISTS idx_approval_queue_media_status;
DROP INDEX IF EXISTS idx_sunset_queue_disk_group_status;
DROP INDEX IF EXISTS idx_library_histories_rollup;

-- Reverse the sunset_queue FK cascade change (restore bare REFERENCES)
CREATE TABLE sunset_queue_old (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    media_name            TEXT NOT NULL,
    media_type            TEXT NOT NULL,
    tmdb_id               INTEGER,
    integration_id        INTEGER REFERENCES integration_configs(id),
    external_id           TEXT,
    size_bytes            INTEGER NOT NULL DEFAULT 0,
    score                 REAL NOT NULL DEFAULT 0,
    score_details         TEXT,
    poster_url            TEXT,
    disk_group_id         INTEGER NOT NULL REFERENCES disk_groups(id),
    collection_group      TEXT,
    trigger               TEXT NOT NULL DEFAULT 'engine',
    deletion_date         DATE NOT NULL,
    label_applied         INTEGER NOT NULL DEFAULT 0,
    poster_overlay_active INTEGER NOT NULL DEFAULT 0,
    expired_at            DATETIME,
    status                TEXT NOT NULL DEFAULT 'pending',
    saved_at              DATETIME,
    saved_score           REAL NOT NULL DEFAULT 0,
    saved_reason          TEXT,
    created_at            DATETIME,
    updated_at            DATETIME
);

INSERT INTO sunset_queue_old
    SELECT id, media_name, media_type, tmdb_id, integration_id, external_id,
           size_bytes, score, score_details, poster_url, disk_group_id,
           collection_group, "trigger", deletion_date, label_applied,
           poster_overlay_active, expired_at, status, saved_at, saved_score,
           saved_reason, created_at, updated_at
    FROM sunset_queue;

DROP TABLE sunset_queue;
ALTER TABLE sunset_queue_old RENAME TO sunset_queue;

CREATE INDEX IF NOT EXISTS idx_sunset_queue_disk_group ON sunset_queue(disk_group_id);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_tmdb_id ON sunset_queue(tmdb_id);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_media_name ON sunset_queue(media_name);
CREATE INDEX IF NOT EXISTS idx_sunset_queue_deletion_date ON sunset_queue(deletion_date);
