# Brittleness Hardening Plan

**Status:** ✅ Complete  
**Created:** 2026-03-26  
**Category:** Architecture / Code Quality  
**Branch:** `refactor/brittleness-hardening`

## Overview

Comprehensive hardening of 14 identified brittleness points in the Capacitarr codebase, plus two targeted structural improvements. The issues were identified through a full codebase audit covering the backend service layer, integration clients, event system, poller, database layer, notification dispatch, CSP handling, and test infrastructure.

No major architecture restructure is needed — the service layer, capability-interface pattern, and event bus are all architecturally sound. The brittleness is at integration edges: wiring safety, serialization assumptions, duplicated constants, and pattern fragility.

## Phases

### Phase 1: Safety Nets (Fail-Fast on Startup)

Highest risk reduction, smallest changes. Ensures bugs surface at startup rather than at 3 AM during a deletion cycle.

#### Step 1.1 — Add `Wired()` methods to all services with lazy dependencies

**Files to modify:**
- `internal/services/deletion.go` — add `Wired() bool` checking `settings`, `engine`, `metrics` (note: `approvalReturner` excluded — it is nil-guarded at the call site and legitimately optional)
- `internal/services/settings.go` — add `Wired() bool` checking `deletionClearer`
- `internal/services/integration.go` — add `Wired() bool` checking `diskGroups`
- `internal/services/diskgroup.go` — add `Wired() bool` checking `engine`
- `internal/services/backup.go` — add `Wired() bool` checking `diskGroups`
- `internal/services/metrics.go` — add `Wired() bool` checking `settings`
- `internal/services/preview.go` — add `Wired() bool` checking `integrations`, `preferences`, `rules`, `diskGroups`, `approvalQueue`, `deletionState`
- `internal/services/rules.go` — add `Wired() bool` checking `preview`, `integrations`
- `internal/services/watch_analytics.go` — add `Wired() bool` checking `rules`, `diskGroups`
- `internal/services/data.go` — add `Wired() bool` checking `preview`
- `internal/services/migration.go` — add `Wired() bool` checking `engineSvc`

Each `Wired()` method returns `true` only when all lazily-injected dependencies are non-nil. Pattern:

```go
func (s *DeletionService) Wired() bool {
    return s.settings != nil && s.engine != nil && s.metrics != nil && s.approvalReturner != nil
}
```

#### Step 1.2 — Add `Validate()` to `services.Registry`

**Files to modify:**
- `internal/services/registry.go` — add `Validate()` method and call it at end of `NewRegistry()`

`Validate()` calls `Wired()` on every service and panics with a descriptive message listing all unwired dependencies. This mirrors the existing fail-fast pattern in `main.go` (e.g., `getSubFS()` panics on embed errors).

#### Step 1.3 — Add startup guard to `DeletionService.Start()`

**Files to modify:**
- `internal/services/deletion.go` — add `Wired()` check at top of `Start()`

Panics if `Start()` is called before `SetDependencies()`. Catches misuse in tests that construct a `DeletionService` directly without the registry.

#### Step 1.4 — Use `engine.DefaultFactors()` in test helper seeding

**Files to modify:**
- `internal/testutil/testutil.go` — replace hardcoded 7-entry `defaultWeights` list with the same bridge pattern used in `main.go`

```go
defaultFactors := engine.DefaultFactors()
factorDefaults := make([]db.FactorDefault, len(defaultFactors))
for i, f := range defaultFactors {
    factorDefaults[i] = db.FactorDefault{Key: f.Key(), DefaultWeight: f.DefaultWeight()}
}
db.SeedFactorWeights(database, factorDefaults)
```

Ensures tests always use the same factor set as production. Adding a new scoring factor automatically flows into all tests.

#### Step 1.5 — Run `make ci` and verify all tests pass

---

### Phase 2: Observability (Log Problems, Don't Silence Them)

Ensures problems that cannot be prevented are at least visible.

#### Step 2.1 — Extract CSP policy to shared function

**Files to create:**
- `routes/csp.go` — new file with `BuildCSP(nonce string) string` function

**Files to modify:**
- `main.go` — security headers middleware calls `routes.BuildCSP(nonce)` instead of inline string
- `internal/testutil/testutil.go` — security headers middleware calls `routes.BuildCSP(nonce)` instead of inline string

Single source of truth for CSP policy. Eliminates the `http:` in `img-src` drift between production and test.

#### Step 2.2 — Add panic recovery state cleanup to poller

**Files to modify:**
- `internal/poller/poller.go` — expand `safePoll()` recovery handler to:
  1. Publish `EngineErrorEvent` with the panic message
  2. Call `reg.Deletion.SignalBatchSize(0)` to unblock the notification two-gate flush
  3. Call `reg.Preview.Invalidate()` to clear potentially stale preview cache

#### Step 2.3 — Add stale accumulator warning to notification dispatch

**Files to modify:**
- `internal/services/notification_dispatch.go` — in the `EngineStartEvent` handler, log a warning if the previous accumulator exists but neither gate fired (indicates dropped events from previous cycle)

Also add a `createdAt` timestamp field to `cycleAccumulator` for age reporting in the warning message.

#### Step 2.4 — Add optional per-subscriber buffer size to EventBus

**Files to modify:**
- `internal/events/bus.go` — add `SubscribeWithBuffer(size int) chan Event` method

**Files to modify (consumers):**
- `internal/services/notification_dispatch.go` — use `SubscribeWithBuffer(512)` instead of `Subscribe()` (notification dispatch processes events quickly; larger buffer prevents dropped gate events during high-volume deletion cycles)

The default `Subscribe()` remains at 256 for all other subscribers.

#### Step 2.5 — Run `make ci` and verify all tests pass

---

### Phase 3: Serialization & Pattern Safety

Eliminates assumptions about data formats.

#### Step 3.1 — Replace SSE JSON byte manipulation with structured marshal

**Files to modify:**
- `internal/events/sse_broadcaster.go` — replace the manual `strip trailing '}'` logic in `broadcast()` with a `ssePayload` wrapper type that implements `MarshalJSON()`

The custom `MarshalJSON()` preserves the performance benefit of the byte-manipulation approach for the happy path (struct events produce `{...}`) but adds an explicit format check and a safe fallback for non-object events.

**Files to create or modify for tests:**
- `internal/events/sse_broadcaster_test.go` — add test case for `ssePayload.MarshalJSON()` with both object and edge-case inputs

#### Step 3.2 — Replace CSP nonce hardcoded patterns with regex + validation

**Files to modify:**
- `csp.go` — replace `bytes.Replace` calls in `injectNoncePlaceholders()` with a compiled regex that handles whitespace/attribute variations

Add a `NonceInjectionCount()` helper or have `injectNoncePlaceholders()` return the number of replacements made, so `buildHTMLTemplates()` in `main.go` can log a warning when zero scripts were nonced.

**Files to modify:**
- `main.go` (or wherever `buildHTMLTemplates` is defined) — add startup warning log when nonce injection found zero scripts

**Files to create or modify for tests:**
- `csp_test.go` — add test cases for regex-based nonce injection with various HTML formats

#### Step 3.3 — Replace global factory map with `sync.Map`

**Files to modify:**
- `internal/integrations/factory.go` — change `factoryRegistry` from `map[string]IntegrationFactory` to `sync.Map`
- Update `RegisterFactory()`, `CreateClient()`, `RegisteredTypes()`, `HasFactory()` to use `sync.Map` methods

This makes the factory registry safe for concurrent reads during test parallelism.

#### Step 3.4 — Run `make ci` and verify all tests pass

---

### Phase 4: Structural Improvements

Targeted restructures that reduce bug surface area for entire classes of issues.

#### Step 4.1 — Extract `arrBaseClient` embedded struct

**Files to create:**
- `internal/integrations/arr_base.go` — new file containing:
  - `arrBaseClient` struct with `URL`, `APIKey`, `APIPrefix` fields
  - `doRequest()` method
  - `TestConnection()` method
  - `GetDiskSpace()` method (delegates to `arrFetchDiskSpace`)
  - `GetRootFolders()` method (delegates to `arrFetchRootFolders`)
  - `GetQualityProfiles()` method (delegates to `arrFetchQualityProfiles`)
  - `GetTags()` method (delegates to `arrFetchTags`)
  - `GetLanguages()` method (delegates to `arrFetchLanguages`)

**Files to modify:**
- `internal/integrations/sonarr.go` — embed `arrBaseClient`, remove duplicated methods, keep only `GetMediaItems()` and `DeleteMediaItem()`
- `internal/integrations/radarr.go` — same refactor
- `internal/integrations/lidarr.go` — same refactor
- `internal/integrations/readarr.go` — same refactor

Constructors change from:

```go
func NewSonarrClient(url, apiKey string) *SonarrClient {
    return &SonarrClient{URL: strings.TrimRight(url, "/"), APIKey: apiKey}
}
```

To:

```go
func NewSonarrClient(url, apiKey string) *SonarrClient {
    return &SonarrClient{
        arrBaseClient: arrBaseClient{
            URL:       strings.TrimRight(url, "/"),
            APIKey:    apiKey,
            APIPrefix: "/api/v3",
        },
    }
}
```

All existing tests must pass without changes since the behavior is identical.

#### Step 4.2 — Add `CollectionNameFetcher` capability interface

**Files to modify:**
- `internal/integrations/types.go` — add `CollectionNameFetcher` interface:
  ```go
  type CollectionNameFetcher interface {
      GetCollectionNames() ([]string, error)
  }
  ```
- `internal/services/integration.go` — refactor `FetchCollectionValues()` to use factory + type assertion instead of switch statement:
  ```go
  client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
  if fetcher, ok := client.(integrations.CollectionNameFetcher); ok {
      names, fetchErr = fetcher.GetCollectionNames()
  }
  ```

The Plex, Jellyfin, and Emby clients already implement `GetCollectionNames()` — they satisfy the interface without changes.

#### Step 4.3 — Run `make ci` and verify all tests pass

---

### Phase 5: Notification Restructure (Explicit Cycle Digest) — ✅ Complete

Replace the fragile two-gate accumulator with explicit orchestration.

**Commit:** `9da5180` — `refactor(notifications): replace event accumulator with explicit cycle digest`

#### Step 5.1 — Add `FlushCycleDigest()` method to `NotificationDispatchService` ✅

**Files modified:**
- `internal/services/notification_dispatch.go` — added `FlushCycleDigest(digest notifications.CycleDigest)` method that:
  1. Sets version from the service
  2. Checks for update info if version checker is available
  3. Dispatches the digest to all enabled channels via `dispatchDigest()`

Called directly by the poller, not via the event bus.

#### Step 5.2 — Build cycle digest in the poller ✅

**Files modified:**
- `internal/poller/poller.go` — at the end of `poll()`, after `SignalBatchSize()`, builds a `notifications.CycleDigest` struct from the poller's own atomic counters and calls `reg.NotificationDispatch.FlushCycleDigest(digest)`
- `internal/poller/poller.go` — added `lastRunCollections int64` atomic counter to Poller struct for tracking collection group expansions
- `internal/poller/evaluate.go` — added `atomic.AddInt64(&p.lastRunFreedBytes, ...)` for auto mode (was previously only tracked for dry-run/approval) and `atomic.AddInt64(&p.lastRunCollections, 1)` when collections are expanded
- `internal/poller/poller.go` — removed `SignalBatchSize(0)` from panic recovery (no longer needed — digest is flushed by the poller, not gated by events)

**Deviation from plan:** The plan stated "No new data plumbing needed" but `lastRunFreedBytes` was not tracked for auto mode and collection groups were not tracked at the poller level. Added both: `lastRunCollections` counter on the Poller struct and `lastRunFreedBytes` accumulation for auto mode in evaluate.go.

#### Step 5.3 — Remove accumulator logic from event handler ✅

**Files modified:**
- `internal/services/notification_dispatch.go` — removed the `cycleAccumulator` struct, `newCycleAccumulator()`, `buildDigest()`, and `tryFlush()`. Removed all event handlers from `handle()`: `EngineStartEvent`, `EngineCompleteEvent`, `DeletionSuccessEvent`, `DeletionDryRunEvent`, `DeletionFailedEvent`, `DeletionBatchCompleteEvent`. Also removed the `accumulator` field from the struct, removed the unused `time` import, replaced `SubscribeWithBuffer(512)` with `Subscribe()` (default 256 — sufficient for immediate alerts only), and updated doc comments.
- `internal/events/types.go` — updated `DeletionBatchCompleteEvent` doc comment (removed "gate 2" reference)
- `internal/notifications/sender.go` — updated `CycleDigest` and `Alert` doc comments (removed "two-gate flush" and "event accumulation" references)

All immediate alert handlers unchanged.

#### Step 5.4 — Update notification dispatch tests ✅

**Files modified:**
- `internal/services/notification_dispatch_test.go` — rewrote all digest tests to call `FlushCycleDigest()` directly instead of simulating the two-gate event sequence. Removed `TestNotificationDispatch_TwoGateFlush` and `TestNotificationDispatch_ReverseGateOrder` (no longer relevant). Added `TestNotificationDispatch_FlushCycleDigest`, `TestNotificationDispatch_FlushCycleDigest_DryRun`, `TestNotificationDispatch_FlushCycleDigest_CollectionGroups`, and `TestNotificationDispatch_VersionPopulated`. All immediate alert tests unchanged.

#### Step 5.5 — Run `make ci` and verify all tests pass ✅

All 15 notification dispatch tests pass. Full CI pipeline (lint, test, security) passes.

---

### Phase 6: Database Layer Cleanup — ✅ Complete

**Commit:** `481a149` — `refactor(db): remove GORM retry callbacks, add WithRetry utility`

#### Step 6.1 — Remove GORM retry callbacks ✅

**Files modified:**
- `internal/db/retry.go` — removed `RegisterRetryCallbacks()` and `retryOnBusy()`. Kept `isSQLiteBusy()` and `cryptoRandInt64()` (used by `WithRetry()`).
- `internal/db/db.go` — removed the `RegisterRetryCallbacks(database)` call and its comment from `Init()`

**Deviation from plan:** The plan called for increasing `busy_timeout` from 5000 to 10000. This was omitted — with `SetMaxOpenConns(1)` serializing all internal access, the 5s timeout is sufficient for the only remaining scenario (external processes holding the lock).

#### Step 6.2 — Add `WithRetry()` utility function ✅

**Files modified:**
- `internal/db/retry.go` — added `WithRetry(fn func() error, maxAttempts int) error` with exponential backoff and jitter on `SQLITE_BUSY`. When `maxAttempts <= 0`, defaults to 3.
- `internal/db/retry_test.go` — added 5 tests: `TestWithRetry_SuccessOnFirstAttempt`, `TestWithRetry_NonBusyErrorNotRetried`, `TestWithRetry_BusyRetriedAndSucceeds`, `TestWithRetry_BusyExhausted`, `TestWithRetry_DefaultMaxAttempts`.

#### Step 6.3 — Parameterize `hasColumn()` for multiple tables ✅

**Files modified:**
- `internal/db/migrate.go` — renamed `hasColumn()` to `hasColumnInTable()` with a `tableName` parameter. Instead of a simple string whitelist with `fmt.Sprintf` (which semgrep correctly flags as SQL injection risk), uses a `tableColumnCheckers` lookup map of pre-built query functions with hardcoded SQL literals. Each table entry is a closure that calls the PRAGMA with the table name baked into the string constant, so no runtime string formatting touches SQL.

#### Step 6.4 — Run `make ci` and verify all tests pass ✅

All db tests pass. Full CI pipeline (lint, test, security) passes with 0 semgrep findings.

---

### Phase 7: Frontend SSE Cleanup — ✅ Complete

**Commit:** `0dac048` — `refactor(frontend): add auto-cleanup scope to SSE event handlers`

#### Step 7.1 — Add auto-cleanup scope parameter to SSE `on()` function ✅

**Files modified:**
- `frontend/app/composables/useEventStream.ts` — added optional `scope` parameter to `on()` that auto-registers an `onUnmounted` cleanup callback via `scope.onUnmounted(() => off(eventType, handler))`.
- `frontend/app/app.vue` — updated 3 integration event handlers to pass `{ onUnmounted }` scope; removed manual `onUnmounted` cleanup block and `sseOff` destructure.
- `frontend/app/pages/index.vue` — updated ~16 event handlers to pass `{ onUnmounted }` scope; removed 25-line manual `onUnmounted` cleanup block and `sseOff` destructure.
- `frontend/app/composables/usePreview.ts` — updated 6 event handlers to pass `{ onUnmounted }` scope; removed manual `onUnmounted` cleanup block and `off` destructure.
- `frontend/app/composables/usePreview.test.ts` — updated assertion to expect the third `scope` argument.

Singleton composables (`useEngineControl`, `useApprovalQueue`, `useSnoozedItems`, `useDeletionQueue`) were not modified — their handlers intentionally persist for the app lifetime.

#### Step 7.2 — Run frontend tests and verify they pass ✅

All 111 tests pass (5 test files). Full CI pipeline (lint, test, security) passes.

---

## Files Changed Summary

| Phase | New Files | Modified Files |
|-------|-----------|----------------|
| 1 | 0 | ~13 (services + testutil) |
| 2 | 1 (routes/csp.go) | ~4 (main.go, testutil, poller, notification_dispatch, bus) |
| 3 | 0 | ~4 (sse_broadcaster, csp.go, main.go, factory.go) |
| 4 | 1 (arr_base.go) | ~6 (sonarr, radarr, lidarr, readarr, types.go, integration.go) |
| 5 | 0 | ~3 (notification_dispatch.go, poller.go, tests) |
| 6 | 0 | ~3 (retry.go, db.go, migrate.go) |
| 7 | 0 | ~1+ (useEventStream.ts + consumer components) |

## Risk Assessment

All phases are backward-compatible. No database migration needed. No API contract changes. No frontend API changes. Tests must pass at the end of each phase before proceeding.

Phase 5 (notification restructure) is the highest-risk change since it modifies the notification delivery mechanism. It should be tested manually by verifying Discord/Apprise notifications fire correctly after an engine cycle.

## Commit Strategy

No commits during a phase — all changes within a phase are made as working tree modifications. Only commit once `make ci` passes at the end of each phase. One commit per phase, following conventional commits:

1. `refactor(services): add startup validation for lazy dependencies`
2. `refactor(routes): extract CSP policy, improve panic recovery and event bus`
3. `fix(events): use structured JSON marshal for SSE, regex-based CSP nonce, sync.Map factory`
4. `refactor(integrations): extract arrBaseClient, add CollectionNameFetcher capability`
5. `refactor(notifications): replace event accumulator with explicit cycle digest`
6. `refactor(db): remove GORM retry callbacks, add WithRetry utility`
7. `refactor(frontend): add auto-cleanup scope to SSE event handlers`
