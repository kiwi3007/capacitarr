-- +goose Up
-- Add poster overlay display style preference: "countdown" (default, existing
-- behavior showing "Leaving in X days") or "simple" (showing only "Leaving soon").
ALTER TABLE preference_sets ADD COLUMN poster_overlay_style TEXT NOT NULL DEFAULT 'countdown';

-- +goose Down
ALTER TABLE preference_sets DROP COLUMN poster_overlay_style;
