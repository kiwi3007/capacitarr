# Sunset Overlay Display Style (Countdown / Simple)

**Status:** ✅ Complete
**Priority:** Feature (minor)
**Estimated Effort:** S (half day)

## Summary

Add a "Display style" preference for sunset overlay text with two modes:

| Value | Internal Key | Poster Banner | Dashboard / Queue |
|-------|-------------|---------------|-------------------|
| **Countdown** (default) | `countdown` | "Leaving in 7 days" / "Leaving tomorrow" / "Last day" | Same |
| **Simple** | `simple` | "Leaving soon" | "Leaving soon" |

Simple mode collapses all countdown states into a single "Leaving soon" message — useful for users sharing their media server who don't want to broadcast the exact deletion timeline.

## Current Behavior

All three render surfaces use the same graduated countdown text:

| Surface | Renderer | File |
|---------|----------|------|
| Poster image overlay (JPEG baked) | `poster.ComposeOverlay()` → `countdownText()` | `backend/internal/poster/overlay.go` |
| Dashboard poster card (HTML) | `sunsetLabel` computed | `frontend/app/components/MediaPosterCard.vue` |
| Sunset queue list (HTML) | `formatDaysRemaining()` | `frontend/app/components/SunsetQueueCard.vue` |

The backend `countdownText()` is hardcoded:
```go
func countdownText(daysRemaining int) string {
    switch {
    case daysRemaining <= 0: return "Last day"
    case daysRemaining == 1: return "Leaving tomorrow"
    default: return fmt.Sprintf("Leaving in %d days", daysRemaining)
    }
}
```

## Implementation Steps

### Phase 1: Backend

#### Step 1.1 — Database migration
- [x] **File:** `backend/internal/db/migrations/00013_poster_overlay_style.sql`
- Add column: `ALTER TABLE preference_sets ADD COLUMN poster_overlay_style TEXT NOT NULL DEFAULT 'countdown'`
- Down: drop column via table recreation (SQLite)

#### Step 1.2 — Model update
- [x] **File:** `backend/internal/db/models.go`
- Add to `PreferenceSet`: `PosterOverlayStyle string` with `gorm:"default:'countdown';not null"` and `json:"posterOverlayStyle"`
- Place adjacent to `PosterOverlayEnabled`

#### Step 1.3 — Settings service
- [x] **File:** `backend/internal/services/settings.go`
- Add `PosterOverlayStyle *string` to `SunsetPreferencePatch`
- Handle in `PatchSunsetPreferences()`: map to `"poster_overlay_style"` column
- Validate value is `"countdown"` or `"simple"`; reject unknown values

#### Step 1.4 — Poster overlay package
- [x] **File:** `backend/internal/poster/overlay.go`
- Change `ComposeOverlay` signature: `ComposeOverlay(original []byte, daysRemaining int, style string) ([]byte, error)`
- Change `countdownText` signature: `countdownText(daysRemaining int, style string) string`
- When `style == "simple"`: return `"Leaving soon"` unconditionally (all day values)

#### Step 1.5 — Poster overlay service
- [x] **File:** `backend/internal/services/poster_overlay.go`
- `UpdateOverlay`: add `style string` parameter, pass through to `poster.ComposeOverlay`
- `UpdateAll`: add `style string` parameter, pass through to `UpdateOverlay`

#### Step 1.6 — Callers of UpdateOverlay/UpdateAll
- [x] **File:** `backend/internal/jobs/cron.go` — read `prefs.PosterOverlayStyle`, pass to `UpdateAll`
- [x] **File:** `backend/routes/sunset.go` — refresh-posters route reads preference, passes style
- [x] **File:** `backend/internal/services/sunset.go` — `QueueSunset` calls `UpdateOverlay` for initial poster; thread style through

#### Step 1.7 — Backup/restore
- [x] **File:** `backend/internal/services/backup.go`
- Add `PosterOverlayStyle string` to `PreferencesExport`
- Include in export marshal and import apply

#### Step 1.8 — Backend tests
- [x] **File:** `backend/internal/poster/overlay_test.go`
  - Update existing `TestCountdownText` cases to pass `"countdown"` style
  - Add new cases for `"simple"` style (all day values → "Leaving soon")
  - Update `TestComposeOverlay_*` calls with style parameter
- [x] **File:** `backend/internal/services/settings_test.go` (if exists)
  - Add patch test for `posterOverlayStyle`

### Phase 2: Frontend

#### Step 2.1 — TypeScript types
- [x] **File:** `frontend/app/types/api.ts`
- Add `posterOverlayStyle: string` to `Preferences` interface

#### Step 2.2 — i18n keys
- [x] **File:** `frontend/app/locales/en.json`
  - Add: `"settings.posterOverlayStyle": "Display Style"`
  - Add: `"settings.posterOverlayStyleDesc": "Countdown shows the exact days remaining. Simple shows only \"Leaving soon\" — useful when sharing your server with others."`
  - Add: `"settings.posterOverlayStyleCountdown": "Countdown"`
  - Add: `"settings.posterOverlayStyleSimple": "Simple"`
  - Add: `"sunset.leavingSoon": "Leaving soon"`
- [x] All other locale files: add `"sunset.leavingSoon": "Leaving soon"` (English fallback; translators update later)

#### Step 2.3 — Settings UI
- [x] **File:** `frontend/app/components/settings/SettingsGeneral.vue`
- Add a `UiSelect` dropdown under the Poster Overlays section, adjacent to the enable toggle
- Only visible when `posterOverlayEnabled` is true
- Values: `"countdown"` (label: Countdown), `"simple"` (label: Simple)
- Wire to `patchPreference('posterOverlayStyle', 'sunset', 'posterOverlayStyle', value)`
- Add reactive ref `posterOverlayStyle` with default `'countdown'`, hydrate from prefs

#### Step 2.4 — Dashboard poster card
- [x] **File:** `frontend/app/components/MediaPosterCard.vue`
- The `sunsetLabel` computed needs access to the display style preference
- When style is `"simple"`: return `t('sunset.leavingSoon')` for all values of `sunsetDaysRemaining`
- Preference can be injected via prop (from parent) or composed via a shared composable

#### Step 2.5 — Sunset queue card
- [x] **File:** `frontend/app/components/SunsetQueueCard.vue`
- `formatDaysRemaining()` and any inline countdown text: respect the style preference
- Same logic: when `"simple"`, return `t('sunset.leavingSoon')`

### Phase 3: Verify

#### Step 3.1 — Run `make ci`
- [x] Confirm lint, test, and security checks pass

## Design Decisions

1. **Single "Leaving soon" for all countdown values in simple mode.** No graduated vague messages ("Leaving eventually" → "Leaving soon"). One message, simple concept.
2. **Global preference, not per-disk-group.** Display style is a presentation concern, same scope as `posterOverlayEnabled`.
3. **No automatic poster refresh on style change.** Existing overlays update on the next daily cron run or manual "Refresh Posters" click. Same behavior as when `sunsetDays` changes.
4. **Poster overlay text is always English.** The backend `countdownText()` produces English text baked into JPEG images. This is existing behavior — "Leaving soon" follows the same pattern. The frontend HTML surfaces use i18n.
