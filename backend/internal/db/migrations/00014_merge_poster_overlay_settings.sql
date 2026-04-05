-- +goose Up
-- Merge poster_overlay_enabled boolean into poster_overlay_style.
-- If overlays were disabled, set style to "off"; otherwise keep the existing style.
-- Then drop the now-redundant boolean column.
UPDATE preference_sets
SET poster_overlay_style = 'off'
WHERE poster_overlay_enabled = 0;

-- SQLite requires a full table rebuild to drop a column (pre-3.35.0 compat).
-- However, goose + modernc/sqlite supports ALTER TABLE ... DROP COLUMN.
ALTER TABLE preference_sets DROP COLUMN poster_overlay_enabled;

-- +goose Down
-- Re-add the boolean column, derive its value from poster_overlay_style.
ALTER TABLE preference_sets ADD COLUMN poster_overlay_enabled INTEGER NOT NULL DEFAULT 1;

UPDATE preference_sets
SET poster_overlay_enabled = 0
WHERE poster_overlay_style = 'off';

-- Revert "off" style back to the previous default "countdown".
UPDATE preference_sets
SET poster_overlay_style = 'countdown'
WHERE poster_overlay_style = 'off';
