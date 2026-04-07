# Brittleness & Rearchitecture Audit

**Status:** ✅ Complete
**Created:** 2026-04-07
**Scope:** Full-stack audit — backend, frontend, tests, integrations, dead code

---

## Executive Summary

A comprehensive code audit of the entire Capacitarr codebase identified **62 discrete findings** across 8 categories. The integration wiring layer is exemplary — all 11 integrations are fully wired from frontend to backend with compile-time verification. The backend service layer is well-tested (~93% file coverage, 740+ test functions). However, several structural and reliability issues warrant attention, organized below from highest to lowest severity.

**No features are "claimed but not implemented."** All 11 integrations, all advertised features, and all completed plan files correspond to real code. The two deferred plans (absolute byte thresholds, Bubble Tea TUI) are correctly marked as such.

---

## Category 1: Process Safety (Crash Prevention)

### Finding 1.1: Unrecovered panics in parallel fetch goroutines
- **Severity:** Critical
- **Files:** `backend/internal/poller/fetch.go` (goroutines at lines ~84, ~136, ~240)
- **Issue:** `fetchAllIntegrations()` spawns goroutines for parallel connection testing, media fetching, and disk space fetching. None of these goroutines have `defer func() { if r := recover(); r != nil { ... } }()`. A panic in any integration client (e.g., nil pointer from unexpected API response) crashes the entire Capacitarr process. The parent `safePoll()` recovery in `poller.go` only covers the main goroutine, not these children.
- **Fix:** Add deferred panic recovery to all goroutines in `fetch.go`. Capture panics as errors and surface them through the existing `fetchResult` / error channel pattern.
- **Effort:** Small

### Finding 1.2: Unbounded notification dispatch goroutines
- **Severity:** High
- **Files:** `backend/internal/services/notification_dispatch.go` (~lines 330, 390)
- **Issue:** Fire-and-forget goroutines are spawned for each notification delivery (Discord/Apprise). No concurrency limit. If a webhook endpoint is slow (30s timeout) and events arrive rapidly (e.g., 50 deletions in a batch), 50+ goroutines accumulate, each holding a TCP connection. No backpressure mechanism exists.
- **Fix:** Use a bounded worker pool (e.g., `chan struct{}` semaphore or `errgroup` with limit) to cap concurrent notification deliveries. Suggested limit: 5 concurrent.
- **Effort:** Small

### Finding 1.3: Missing panic recovery in background goroutines
- **Severity:** Medium
- **Files:** `main.go` (~line 379), `backend/internal/services/recovery.go` (~line 97), `backend/internal/services/notification_dispatch.go` (~lines 330, 390)
- **Issue:** The startup self-test goroutine, recovery tick goroutine, and notification sender goroutines all lack panic recovery. A panic in any of these silently kills the goroutine — the process continues running but the subsystem is dead.
- **Fix:** Add deferred recovery to all long-running background goroutines. Log recovered panics at `slog.Error` level and publish a system event.
- **Effort:** Small

---

## Category 2: Error Handling Gaps

### Finding 2.1: Discarded error in sunset escalation
- **Severity:** High
- **File:** `backend/internal/services/sunset.go` (~line 762)
- **Issue:** `_ = deps.Deletion.QueueDeletion(...)` discards the error when a sunset item expires and should be queued for deletion. If the deletion queue is full (500-item cap), the item is marked as escalated but never actually deleted. No event, log, or error signal is produced. The item is silently lost.
- **Fix:** Return the error; if queueing fails, revert the sunset item's status back to active, log at `slog.Error`, and publish a failure event.
- **Effort:** Small

### Finding 2.2: Discarded error in disk group upsert during sync
- **Severity:** High
- **File:** `backend/internal/services/integration.go` (~line 962)
- **Issue:** `_, _ = s.diskGroups.Upsert(d)` in `SyncAll()` silently discards the upsert error. The sync operation appears to succeed while disk group data becomes stale.
- **Fix:** Collect errors and include them in the sync response. Surface failed upserts to the user.
- **Effort:** Small

### Finding 2.3: Discarded rollup checkpoint errors
- **Severity:** Medium
- **File:** `backend/internal/jobs/cron.go` (~lines 27, 48, 69)
- **Issue:** `_ = reg.Metrics.SetRollupCheckpoint(...)` errors are discarded in all three rollup jobs (hourly, daily, weekly). If a checkpoint write fails, the next tick will re-process already-rolled-up data, causing duplicate aggregates.
- **Fix:** Log the error and publish a system warning event. Consider making the rollup itself idempotent to tolerate duplicates.
- **Effort:** Small

### Finding 2.4: runStatsID defaults to 0 on CreateRunStats failure
- **Severity:** Medium
- **File:** `backend/internal/poller/poller.go` (~lines 217-219)
- **Issue:** If `CreateRunStats()` fails, `runStatsID` defaults to 0. All subsequent `UpdateRunStats()` and `IncrementDeletedStats()` calls operate on row 0 (non-existent), silently failing. The engine run completes but no stats are recorded.
- **Fix:** Either skip stats updates when `runStatsID == 0` (check explicitly), or treat the failure as non-fatal and log it with a distinct event.
- **Effort:** Small

### Finding 2.5: Recovery state divergence on DB update failure
- **Severity:** Medium
- **File:** `backend/internal/services/recovery.go` (~line 294)
- **Issue:** `_ = r.integrationSvc.UpdateSyncStatusDirect(...)` — if the DB update fails after a recovery probe succeeds, the in-memory tracker says "recovered" but the database still shows "failed." On next restart, the integration will appear failed again.
- **Fix:** Return the error and revert the in-memory tracker state.
- **Effort:** Small

---

## Category 3: God Function Decomposition

### Finding 3.1: `evaluateAndCleanDisk()` — 481 lines
- **Severity:** High (regression risk)
- **File:** `backend/internal/poller/evaluate.go` (lines 22-503)
- **Issue:** This single function handles: early-return threshold checks and sunset dispatch (lines 22-64), mount-path filtering (lines 76-103), scoring and candidate selection (lines 105-135), show/season dedup and snooze filtering (lines 137-216), collection expansion and protection (lines 222-368), multi-mode dispatch for auto/approval/dry-run (lines 370-460), approval batch upsert (lines 462-473), queue reconciliation (lines 475-487), and diagnostic logging (lines 489-502). It is the highest-risk function in the codebase for regression bugs. Every new feature touching deletion logic must modify this function.
- **Note:** `evaluateSunsetMode()` (lines 505-646) is already a separate function, called from line 43. It does not need extraction — only `evaluateAndCleanDisk` itself needs decomposition.
- **Naming principle:** Function names containing "and" signal bundled responsibilities. The current name `evaluateAndCleanDisk` is an "and" — evaluate *and* clean. The decomposition must avoid "and" names throughout.
- **Fix:** Decompose into single-responsibility functions:
  ```
  evaluateDiskGroup()       <- renamed orchestrator (~40 lines: threshold checks,
                               mount filtering, delegation, return)
    |-- scoreCandidates()    <- engine evaluation + byte-budget selection (lines 105-135)
    |-- filterCandidates()   <- show/season dedup, snooze skip, zero-score skip (lines 137-216)
    |-- expandCollections()  <- collection resolution + member protection (lines 222-368)
    |-- dispatchByMode()     <- per-item routing: auto/approval/dry-run (lines 370-460)
    \-- reconcileQueue()     <- batch upsert + stale item dismissal (lines 462-501)
  ```
  Each extracted function does one thing. The orchestrator's name describes its scope (a disk group evaluation), not its internal steps.
- **Effort:** Medium (refactor-only, no behavior change)

### Finding 3.2: `poll()` — 310 lines
- **Severity:** Medium
- **File:** `backend/internal/poller/poller.go` (~lines 169-481)
- **Issue:** Large orchestration function handling 10+ distinct phases. While orchestration functions are naturally longer, this one mixes coordination with inline business logic (e.g., disk group upsert logic at ~lines 350-380).
- **Fix:** Extract `prepareContext()`, `processMediaMounts()`, and `finalizeCycle()` helpers.
- **Effort:** Medium

### Finding 3.3: `processJob()` — 240 lines
- **Severity:** Medium
- **File:** `backend/internal/services/deletion.go` (~lines 541-782)
- **Issue:** Handles cancellation check, mode-change check, dry-run determination, actual deletion, stats increment, audit logging, event publishing, approval cleanup, sunset cleanup. Distinct execution paths (actual vs. dry-run) are interleaved through conditional branches.
- **Fix:** Extract `executeDeletion()` and `executeDryRun()` as separate methods. Move cleanup (approval, sunset, audit) into a `postDeletion()` helper.
- **Effort:** Medium

---

## Category 4: Context Propagation *(Deferred — separate plan)*

### Finding 4.1: No context propagation for HTTP cancellation
### Finding 4.2: No total operation timeout for paginated fetches

These findings are **deferred to a dedicated follow-up plan**. Context propagation is a cross-cutting signature refactor that threads `context.Context` through ~40+ function signatures across every integration client, `DoAPIRequest`, the poller, and the enrichment pipeline. It has zero behavioral change for users but is a large mechanical refactor. Mixing it with the safety/error fixes in this plan would make code review unwieldy and risk contaminating safety fixes with refactor noise.

**Separate plan scope:**
- Thread `context.Context` from the poller through all integration HTTP calls
- Replace `context.Background()` in `DoAPIRequest` variants with caller-provided contexts
- Add per-operation timeouts via `context.WithTimeout`
- Add total operation timeouts for paginated fetches (Jellyfin, Emby, Tautulli, Tracearr)
- Cancel in-flight requests on graceful shutdown

---

## Category 5: Dead Code & Unused Dependencies

### Finding 5.1: Dead frontend UI component directories
- **Severity:** Low
- **Files:** `frontend/app/components/ui/dashboard-card/`, `number-field/`, `sheet/`, `toggle-group/`, `toggle/`
- **Issue:** 5 shadcn-vue component families are installed but never imported by any application component. They add to bundle size and maintenance surface.
- **Fix:** Remove the directories. If needed later, regenerate via `npx shadcn-vue@latest add <component>`.
- **Effort:** Small

### Finding 5.2: Migrate from custom `useToast` to `vue-sonner` (Sonner)
- **Severity:** Low
- **Files:** `frontend/package.json`, `frontend/app/composables/useToast.ts`, `frontend/app/components/ToastContainer.vue`, `frontend/app/components/ui/sonner/`
- **Background:** `vue-sonner` is already installed and the shadcn-vue `Sonner.vue` wrapper already exists with Lucide icons and design-token integration. However, it was never mounted. Instead, a custom `useToast` composable (38 lines) and `ToastContainer.vue` (53 lines) were written. Both systems do the same thing — display typed notification toasts with auto-remove.
- **Analysis — why switch to vue-sonner:**
  1. **Feature gap:** The custom toast is a minimal implementation (push + setTimeout remove). Sonner provides stacking with smooth animations, swipe-to-dismiss, promise toasts (`toast.promise()`), action buttons (`{ action: { label, onClick } }`), rich descriptions, and accessible ARIA attributes — all out of the box.
  2. **Maintenance:** The shadcn-vue project officially recommends Sonner as its toast solution (it is in the component registry). Keeping a parallel custom system means maintaining two toast paradigms. The shadcn-vue `Sonner.vue` wrapper is already themed to the design system (uses `--popover`, `--popover-foreground`, `--border`, `--radius` CSS variables).
  3. **API simplicity:** Sonner's `toast()` / `toast.success()` / `toast.error()` is a direct import — no composable needed. The 128+ `addToast()` call sites would become `toast.success('message')` or `toast.error('message')` with identical semantics.
  4. **The wrapper is ready:** `ui/sonner/Sonner.vue` already swaps in Lucide icons for all 6 icon slots (success, info, warning, error, loading, close). It just needs to be mounted in `app.vue`.
- **Fix:**
  1. Mount `<Sonner />` in `app.vue` (add `import 'vue-sonner/style.css'`)
  2. Replace all `addToast(msg, 'success')` -> `toast.success(msg)`, `addToast(msg, 'error')` -> `toast.error(msg)`, `addToast(msg, 'info')` -> `toast.info(msg)` across ~128 call sites
  3. Delete `useToast.ts` and `ToastContainer.vue`
  4. Position the Sonner above the bottom toolbar (CSS offset)
- **Effort:** Small-Medium (mechanical find-replace across ~128 call sites, plus testing positioning)

### Finding 5.3: Remove `@tanstack/vue-table`
- **Severity:** Low
- **Files:** `frontend/package.json`, `frontend/app/components/ui/table/utils.ts`
- **Current state:** `@tanstack/vue-table` is installed. The only usage is `ui/table/utils.ts` which exports a `valueUpdater` utility function. That function is never imported outside the `ui/table/` directory. The actual tables (`LibraryTable.vue`, audit log) are built with manual `v-for` rendering against plain arrays + `@tanstack/vue-virtual` for virtualization.
- **Analysis — should we use it instead?** TanStack Table provides reactive column definitions, sorting, filtering, pagination, and row selection as a headless state manager. The existing tables already implement all of these features manually. Adopting TanStack Table would mean rewriting the table logic to use its API (`useVueTable`, `flexRender`, column helpers). This is significant effort for two tables that already work, and it adds a conceptual dependency. The `@tanstack/vue-virtual` virtualizer (which IS used) is the piece of TanStack that provides real value here.
- **Recommendation:** Remove the dependency and delete `ui/table/utils.ts`. The tables work without it, and adopting it now would be a rewrite for no user-facing benefit. If a future feature needs TanStack Table's column state management (e.g., user-configurable columns, multi-sort), it can be re-added then.
- **Effort:** Trivial

### Finding 5.4: Redundant `@radix-icons/vue` alongside `lucide-vue-next`
- **Severity:** Low
- **Files:** `frontend/package.json`, 11 shadcn-vue UI component files
- **Issue:** Two icon libraries are bundled. `lucide-vue-next` is the primary (43+ uses). `@radix-icons/vue` is only used in 11 UI component internals. All have direct Lucide equivalents.
- **Detailed icon mapping for replacement:**

  | File | Radix Import | Lucide Replacement |
  |------|-------------|-------------------|
  | `ui/sheet/SheetContent.vue` | `Cross2Icon` | `XIcon` |
  | `ui/select/SelectTrigger.vue` | `ChevronDownIcon` | `ChevronDownIcon` |
  | `ui/select/SelectScrollUpButton.vue` | `ChevronUpIcon` | `ChevronUpIcon` |
  | `ui/select/SelectScrollDownButton.vue` | `ChevronDownIcon` | `ChevronDownIcon` |
  | `ui/select/SelectItem.vue` | `CheckIcon` | `CheckIcon` |
  | `ui/dropdown-menu/DropdownMenuSubTrigger.vue` | `ChevronRightIcon` | `ChevronRightIcon` |
  | `ui/dropdown-menu/DropdownMenuRadioItem.vue` | `DotFilledIcon` | `CircleIcon` (with `class="size-2 fill-current"`) |
  | `ui/dropdown-menu/DropdownMenuCheckboxItem.vue` | `CheckIcon` | `CheckIcon` |
  | `ui/dialog/DialogScrollContent.vue` | `Cross2Icon` | `XIcon` |
  | `ui/dialog/DialogContent.vue` | `Cross2Icon` | `XIcon` |
  | `ui/command/CommandInput.vue` | `MagnifyingGlassIcon` | `SearchIcon` |

- **Notes:**
  - 5 icons (`ChevronDownIcon`, `ChevronUpIcon`, `ChevronRightIcon`, `CheckIcon`) share the same name in both libraries — only the import path changes.
  - `Cross2Icon` -> `XIcon` (Lucide's equivalent of a close/X icon)
  - `MagnifyingGlassIcon` -> `SearchIcon`
  - `DotFilledIcon` -> `CircleIcon` with `class="size-2 fill-current"` to match the filled-dot appearance. Verify the visual size matches.
- **Fix:** Change 11 import statements, verify visual parity, remove `@radix-icons/vue` from `package.json`.
- **Effort:** Small

### Finding 5.5: Unused `ts-node` dev dependency
- **Severity:** Low
- **Files:** `frontend/package.json`
- **Issue:** `ts-node` is listed as a dev dependency but not referenced in any script or config.
- **Fix:** Remove from `devDependencies`.
- **Effort:** Trivial

### Finding 5.6: Unused backend API endpoints
- **Severity:** Low (not dead code — available for API consumers, but not used by the UI)
- **Files:** Various `backend/routes/*.go`
- **Issue:** 11+ backend endpoints have no frontend consumer.
- **Overlap analysis — which unused endpoints are redundant with used ones?**

  | Unused Endpoint | Overlaps With | Verdict |
  |----------------|---------------|---------|
  | `GET /metrics/history` | `GET /engine/history` | **Partial overlap.** `/metrics/history` returns `library_histories` (raw capacity time-series by resolution). `/engine/history` returns `engine_run_stats` (per-run evaluation stats). Different data sources, different purposes. **Keep both.** |
  | `GET /lifetime-stats` | `GET /worker/stats` | **Partial overlap.** `/lifetime-stats` returns cumulative counters (total deletions ever, total bytes freed ever). `/worker/stats` returns current-cycle stats (last run time, poll interval, running state). **Keep both** — complementary. |
  | `GET /dashboard-stats` | `GET /worker/stats` + `GET /lifetime-stats` | **Superset.** `/dashboard-stats` aggregates lifetime stats, protected count, and library growth rate into one call. The frontend could use this instead of making separate calls. **Keep — useful for future dashboard enhancement or headless consumers.** |
  | `GET /audit-log/recent` | `GET /activity/recent` | **Different data.** `/audit-log/recent` returns deletion audit entries (who deleted what, when, dry-run vs actual). `/activity/recent` returns system events (engine started, integration added, etc.). **Keep both** — different tables, different purposes. |
  | `GET /audit-log/grouped` | `GET /audit-log` | **Presentation variant.** Returns the same audit data grouped by engine run. The frontend's library page uses the flat `/audit-log` with its own grouping. **Keep** — useful for headless/CLI. |
  | `PATCH /preferences/{group}` | `PUT /preferences` | **Subset.** The PATCH endpoints update one field group without touching others. The frontend uses the monolithic `PUT`. **Keep** — better for headless consumers that don't want to send the full preferences blob. |
  | `POST /approval-queue/group/{approve,reject}` | Individual approve/reject per item | **Collection-level.** Approves/rejects all items in a collection group atomically. Frontend does individual approvals. **Keep** — the frontend should adopt these for the "Approve All" / "Reject All" buttons on collection groups. |
  | `POST /integrations/sync` | Automatic sync during poll | **Manual trigger.** Forces an immediate integration sync (re-fetch disk groups). Not exposed in UI. **Keep** — useful for headless/CLI, could be exposed as a button. |
  | `GET /integrations/health` | SSE `integration_recovered` events | **Different delivery.** REST endpoint returns full health state vs. SSE which pushes incremental changes. **Keep** — useful for polling-based headless consumers. |
  | `POST /sunset-queue/refresh-labels` | Automatic label application during poll | **Manual trigger.** Forces re-application of sunset labels. **Keep** — useful for recovery after media server restart. |
  | `GET /deletion-queue/grace-period` | SSE `deletion_grace_period` events | **Same pattern as health.** REST for polling, SSE for push. **Keep.** |

- **Recommendation:** No endpoints should be removed. None are truly redundant — they serve different data, different presentation, or different delivery mechanisms (REST vs SSE). Document all of them in the API docs. The collection group approve/reject endpoints are candidates for frontend adoption.
- **Effort:** N/A (no code change)

---

## Category 6: Database Schema Gaps

### Finding 6.1: Missing composite indexes
- **Severity:** Medium (performance degrades as data grows)
- **Files:** `backend/internal/db/models.go` or Goose migrations
- **Issue:** Several common query patterns lack composite indexes:
  1. `approval_queue (status, disk_group_id)` — used by `ListQueue()` filtering
  2. `approval_queue (media_name, media_type, status)` — used by `BulkUpsertPending()` conflict resolution
  3. `sunset_queue (disk_group_id, status)` — used by `ListSunsettedKeys()` and `Escalate()`
  4. `library_histories (disk_group_id, resolution, timestamp)` — used by rollup queries
- **Fix:** Add a Goose migration (00017) creating the composite indexes.
- **Effort:** Small

### Finding 6.2: Missing foreign key constraints — cascades are the proper fix
- **Severity:** Medium (orphaned data on integration/disk group deletion)
- **Files:** `backend/internal/db/models.go`, Goose migrations
- **Current orphan cleanup mechanisms:**
  The codebase already has *some* application-level orphan cleanup, but it is incomplete and inconsistent:
  1. **`IntegrationService.Delete()`** — when the *last* integration is deleted, removes all disk groups. But deleting a non-last integration does NOT clean up approval queue, sunset queue, or custom rules that reference it.
  2. **`MappingService.GarbageCollect()`** — explicitly removes orphaned mappings for deleted integrations. The code comment at line 176 literally says: *"The ON DELETE CASCADE FK should handle this, but belt-and-suspenders for cases where FK enforcement is disabled or the cascade didn't fire."* This is an FK cascade that DOES exist on `media_server_mappings` (migration `00008`), with GC as a safety net.
  3. **`BackupService` sync mode** — cleans up orphaned integrations, rules, channels, and disk groups during import. But this only runs during import, not during normal integration deletion.
  4. **`ApprovalService.RecoverOrphans()`** — recovers "approved" items with no active deletion job. This handles a different kind of orphan (state-machine orphans, not FK orphans).
  5. **Poller `ReconcileActiveMounts()`** — marks disk groups stale/reaps them. This handles disk groups that disappear from integration reports, not FK cleanup.
- **The migration SQL already uses cascades in many places:** The baseline migration (`00001`) and subsequent migrations define `ON DELETE CASCADE` on `disk_group_integrations`, `library_histories`, `media_server_mappings`, `custom_rules`, and `audit_log` tables. The tables that are *missing* cascades are specifically `approval_queue` and `sunset_queue` for their `integration_id` columns (they use bare `REFERENCES` without `ON DELETE`).
- **Analysis — cascades vs application-level cleanup:**
  Cascades are the proper solution here. The codebase *already* relies on cascades for `media_server_mappings`, `disk_group_integrations`, `library_histories`, and `custom_rules`. The missing cascades on `approval_queue` and `sunset_queue` are inconsistencies, not a design choice. The `MappingService.GarbageCollect()` comment explicitly acknowledges cascades as the primary mechanism. Application-level cleanup should remain as belt-and-suspenders (as it already does for mappings), not as the primary orphan prevention.
- **Fix:** Add a Goose migration adding:
  - `approval_queue.integration_id` -> `ON DELETE CASCADE` (if integration is deleted, its approval entries are meaningless)
  - `sunset_queue.integration_id` -> `ON DELETE CASCADE` (if integration is deleted, its sunset entries are meaningless)
  - `sunset_queue.disk_group_id` -> `ON DELETE CASCADE` (sunset entries are per-disk-group; if the group is gone, so are they)
  - `approval_queue.disk_group_id` -> `ON DELETE SET NULL` (keep the entry but clear the group reference, since items may still be visible in the queue)

  Note: SQLite doesn't support `ALTER TABLE ... ADD CONSTRAINT`. This requires the standard SQLite dance: create new table with constraints -> copy data -> drop old -> rename. Bundle with Finding 6.1's composite indexes in one migration.
- **Effort:** Medium (SQLite table recreation pattern + testing cascade behavior)

### Finding 6.3: Large transactions blocking single-writer
- **Severity:** Low (mitigated by SQLite WAL mode)
- **Files:** `backend/internal/services/approval.go`, `mapping.go`
- **Issue:** `BulkUpsertPending()` and `BulkUpsert()` (mappings) process hundreds of items in a single transaction. With SQLite's single-writer model, this blocks all other writes for the duration.
- **Recommendation:** Consider chunked transactions (e.g., 50 items per transaction) for very large libraries. Not urgent — WAL mode allows concurrent reads.
- **Effort:** Small

---

## Category 7: Frontend Architecture Issues *(Deferred — separate plan)*

The following frontend findings are **deferred to a dedicated follow-up plan**. They are pure refactoring work (page decomposition, i18n fixes, error handling improvements) with no safety implications. Mixing them with the backend safety/reliability fixes in this plan would dilute focus and make the PR scope unwieldy.

### Findings to carry forward:
- **7.1** — Dashboard page (1189 lines) needs decomposition into ~5 sub-components
- **7.2** — Help page (1202 lines) needs decomposition into ~13 sub-components
- **7.3** — Hardcoded English in Collection Deletion section (i18n gap)
- **7.4** — Silent error handling in 9+ locations (console.warn instead of user feedback)
- **7.5** — HMR duplicate SSE handler accumulation (dev-only)

These should be a single "Frontend Architecture Polish" plan covering component extraction, error surfacing, and i18n consistency.

---

## Category 8: Test Coverage Gaps

### Finding 8.1: Frontend test coverage is critically low (14%)
- **Severity:** High
- **Scope:** 22 composables (3 tested), 7 utils (2 tested), 8 pages (0 tested)
- **Priority targets:**
  1. `useApi.ts` — core API client, every feature depends on it
  2. `useEventStream.ts` — SSE reconnection is a common failure point
  3. `useApprovalQueue.ts` — complex state machine with optimistic updates
  4. `useDeletionQueue.ts` — deletion state management
  5. `groupPreview.ts` — show/season grouping logic with edge cases
  6. `plexOAuth.ts` — Plex OAuth flow (failure locks users out)
- **Effort:** Large (incremental — start with highest-risk composables)

### Finding 8.2: `services/schema.go` has no tests (330 lines)
- **Severity:** High
- **File:** `backend/internal/services/schema.go`
- **Issue:** Schema validation runs at startup and is the safety net for DB integrity. If it has bugs (false positive = crash on valid schema, false negative = silent data corruption), there are no tests to catch them.
- **Fix:** Add tests using the in-memory SQLite pattern. Test valid schema passes, invalid schema detected, missing columns, extra columns.
- **Effort:** Small

### Finding 8.3: `migration/migrate.go` has no tests
- **Severity:** High
- **File:** `backend/internal/migration/migrate.go`
- **Issue:** The 1.x -> 2.0 migration logic is untested. `detect.go` is tested, but `migrate.go` (which does the actual data migration) is not. A regression here could cause data loss during version upgrades.
- **Fix:** Add tests with a synthetic 1.x database fixture. Test the full migration path.
- **Effort:** Medium

### Finding 8.4: `poster_overlay.go` has only nil-guard tests
- **Severity:** Medium
- **File:** `backend/internal/services/poster_overlay_test.go`
- **Issue:** Only 2 tests exist — nil-guard and empty-queue. No tests for actual overlay rendering (countdown badge, simple badge, text truncation, image compositing). A rendering bug could corrupt posters in users' media libraries.
- **Fix:** Add tests with test fixture images. Verify overlay dimensions, text placement, and edge cases (long titles, zero-day countdown).
- **Effort:** Medium

---

## Category 9: Hardcoded Values Worth Making Configurable

These are not bugs but represent operator pain points that could be addressed with configuration:

| Value | File | Current | Recommendation |
|-------|------|---------|----------------|
| Deletion rate limit | `deletion.go` | 1 per 3s | Configurable (env var or preference) |
| Deletion queue max | `deletion.go` | 500 items | Configurable (env var) |
| HTTP client timeout | `httpclient.go` | 30s global | Per-integration override |
| SSE ring buffer | `sse_broadcaster.go` | 100 events | Configurable (env var) |

---

## Implementation Priority

### Phase 1: Critical Safety (Effort: ~1 day)
- [x] 1.1 — Add panic recovery to parallel fetch goroutines
- [x] 1.2 — Bound notification dispatch goroutines (semaphore with maxConcurrentNotifications=5)
- [x] 1.3 — Add panic recovery to all background goroutines
- [x] 2.1 — Fix sunset escalation error handling
- [x] 2.2 — Fix disk group upsert error handling

### Phase 2: Error Handling & Data Integrity (Effort: ~1 day)
- [x] 2.3 — Fix rollup checkpoint error handling
- [x] 2.4 — Guard against runStatsID == 0
- [x] 2.5 — Fix recovery state divergence
- [x] 6.1 — Add composite database indexes (Goose migration 00017)
- [x] 6.2 — Add missing FK cascades on sunset_queue (same Goose migration)

### Phase 3: God Function Decomposition (Effort: ~2 days)
- [x] 3.1 — Decompose `evaluateAndCleanDisk()` (481 lines) -> `evaluateDiskGroup()` + `scoreCandidates()` + `filterCandidates()` + `expandCollections()` + `dispatchByMode()` + `reconcileQueue()` + `dispatchFiltered()`
- [x] 3.2 — Decompose `poll()` (310 lines) -> `prepareContext()` + `processMediaMounts()` + `finalizeCycle()`
- [x] 3.3 — Decompose `processJob()` (240 lines) -> `executeDryRun()` + `executeDeletion()` + `postDeletion()`

### Phase 4: Dead Code & Dependency Cleanup (Effort: ~1 day)
- [x] 5.1 — Remove 5 dead UI component directories (dashboard-card, number-field, sheet, toggle-group, toggle)
- [x] 5.2 — Migrate from custom `useToast` to vue-sonner `Sonner` (mounted Sonner, replaced ~106 call sites across 14 files, deleted useToast.ts + ToastContainer.vue)
- [x] 5.3 — Remove `@tanstack/vue-table` + `ui/table/utils.ts`
- [x] 5.4 — Replace 10 `@radix-icons/vue` imports with Lucide equivalents, removed `@radix-icons/vue` from package.json
- [x] 5.5 — Remove `ts-node`

### Phase 5: Test Coverage (Effort: ~3 days, incremental)
- [x] 8.2 — Add `schema.go` tests (6 tests: valid schema, missing column, extra column tolerated, missing table, error formatting, repair)
- [x] 8.3 — Add `migration/migrate.go` tests (8 tests: empty v1, integrations, preferences, disk groups, rules, notifications, idempotency, overseerr transform)
- [x] 8.4 — Expand poster overlay tests (11 new tests: countdown badge, zero-day, negative days, simple badge, long title, saved badge, restore original, download poster, no poster URL, cache validation)
- [x] 8.1 — Frontend composable tests: `useApi` (12 tests), `useEventStream` (20 tests), `useApprovalQueue` (34 tests)

### Deferred to separate plans:
- **Context propagation** (4.1, 4.2) — separate plan for threading `context.Context` through the integration HTTP layer
- **Frontend architecture polish** (7.1-7.5) — separate plan for page decomposition, i18n, error surfacing

---

## What Was NOT Found (Positive Audit Results)

1. **Integration wiring is complete and fully consistent.** All 11 integrations are registered in the factory, validated at startup, wired through the enrichment pipeline, exercised by the poller, and surfaced in the frontend. Zero gaps.

2. **No features are "claimed but unimplemented."** Every README claim, every completed plan file, and every CHANGELOG entry corresponds to real code.

3. **No TODO/FIXME/HACK comments anywhere** in either backend or frontend. The codebase is clean of deferred work markers.

4. **Route handlers have zero direct DB access.** The service layer architecture is fully enforced.

5. **All SSE event names match exactly** between frontend constants and backend event types. Zero naming mismatches.

6. **All API endpoint paths match exactly** between frontend API calls and backend route registrations. Zero path mismatches.

7. **Backend test quality is excellent.** Tests cover error paths, edge cases, concurrent access, event verification, and multi-service integration workflows.

8. **Security posture is strong.** bcrypt passwords, SHA-256 API key hashing, JWT with strict cookie flags, CSP nonces, rate limiting, SSRF protection, and 7 blocking security scanners in CI.

9. **No `any` types, `@ts-ignore`, or `@ts-expect-error` directives** in the frontend. Clean type discipline.

10. **No circular import dependencies** detected in either backend or frontend.
