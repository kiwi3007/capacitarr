# Sunset Mode: UI/UX Polish

**Status:** ✅ Complete  
**Priority:** UI Polish  
**Estimated Effort:** L (2-3 days)  
**Branch:** `feature/sunset-virtual-show-level-override` (from `feature/3.0`)

## Summary

Comprehensive UI/UX audit identified 14 issues across the sunset mode frontend. This plan addresses removal of experimental developer scaffolding, missing confirmation dialogs for destructive actions, missing loading/save feedback, sunset queue visibility, and i18n gaps for hardcoded English strings.

## Implementation

### Phase 1: Remove ECharts Experimental Bar

**File:** `frontend/app/components/rules/RuleDiskThresholds.vue`

- [x] Remove the ECharts progress bar and its "experimental" debug label (`RuleDiskThresholds.vue:149-165`)
- [x] Remove the "Original CSS bar (for comparison)" debug label (`RuleDiskThresholds.vue:166`)
- [x] Remove the ECharts option builder function and all ECharts-related imports/refs
- [x] Remove the `vue-echarts` component usage
- [x] Keep only the CSS progress bar with vue-motion

### Phase 2: Sunset Settings Card Polish

**File:** `frontend/app/components/settings/SettingsGeneral.vue`

- [x] Add `<SaveIndicator>` to Poster Overlay toggle (the only toggle missing it)
- [x] Add loading/disabled state to "Refresh Posters" button with spinner during API call
- [x] Add loading/disabled state to "Restore All Posters" button with spinner during API call

### Phase 3: Confirmation Dialogs

**Files:** `frontend/app/components/settings/SettingsGeneral.vue`, `frontend/app/components/SunsetQueueCard.vue`

- [x] Add confirmation dialog for "Restore All Posters" (destructive emergency action)
- [x] Add confirmation dialog for "Clear All" sunset queue (bulk destructive action)

### Phase 4: Sunset Queue Visibility

**File:** `frontend/app/components/SunsetQueueCard.vue`

- [x] Show the sunset queue card whenever any disk group is in sunset mode, even when the queue is empty
- [x] Add empty state using the existing `deletion.emptyInSunset` i18n key
- [x] Pass disk group mode information to the component (or fetch it)

### Phase 5: i18n Fixes

**Files:** Multiple frontend files + `frontend/app/locales/en.json`

- [x] `MediaPosterCard.vue` — Replace hardcoded sunset countdown strings with existing i18n keys (`sunset.lastDay`, `sunset.leavingTomorrow`, `sunset.leavingInDays`)
- [x] `SettingsGeneral.vue` — Add i18n keys for Daily Score Check section (6 hardcoded labels/descriptions)
- [x] `RuleDiskThresholds.vue` — Add shared i18n keys for mode labels and sunset marker text
- [x] `DiskGroupSection.vue` — Use shared mode label i18n keys
- [x] `useEngineControl.ts` — Use shared mode label i18n keys
- [x] `useSunsetQueue.ts` — Add i18n keys for 6 toast messages
- [x] `SettingsGeneral.vue` — Add i18n keys for 4 poster toast messages
- [x] `SunsetQueueCard.vue` — Add i18n keys for "Saved by popular demand" and "Saved" badge
- [x] `RuleDiskThresholds.vue` — Add i18n keys for CSS progress bar marker text ("Sunset X%")
- [x] Accessibility: Add `aria-label` to mode selector buttons, progress bar markers, Clear All button

### Phase 6: Verification

- [x] Run `make ci` and verify all checks pass

## Key Files

| File | Changes |
|------|---------|
| `frontend/app/components/rules/RuleDiskThresholds.vue` | Remove ECharts, i18n mode labels + markers, aria-labels |
| `frontend/app/components/settings/SettingsGeneral.vue` | SaveIndicator, loading states, confirmation dialog, i18n daily score check + toasts |
| `frontend/app/components/SunsetQueueCard.vue` | Visibility, confirmation dialog, i18n hardcoded strings, aria-label |
| `frontend/app/components/MediaPosterCard.vue` | i18n sunset countdown |
| `frontend/app/components/DiskGroupSection.vue` | i18n mode labels |
| `frontend/app/composables/useSunsetQueue.ts` | i18n toast messages |
| `frontend/app/composables/useEngineControl.ts` | i18n mode labels |
| `frontend/app/locales/en.json` | New i18n keys for all above |
