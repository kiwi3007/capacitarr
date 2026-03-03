# CI/CD Polish and Cleanup

**Status:** 📋 Planned

**Prerequisite:** [Build, CI/CD, and Publishing Overhaul](20260303T1551Z-build-cicd-publishing.md) — ✅ Complete

---

## Overview

Follow-up items discovered during the CI/CD overhaul implementation. These are polish, optimization, and documentation fixes — the core pipeline is functional.

---

## Phase 1: CI Pipeline Fixes

### 1.1 Fix `build:docker` warning

The `build:docker` smoke test job produces: `WARNING: No output specified with docker-container driver`. Add `--output=type=cacheonly` to explicitly declare the build is verification-only.

**File:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml)

```yaml
build:docker:
  script:
    - docker buildx build --platform linux/amd64,linux/arm64 --output=type=cacheonly .
```

### 1.2 Add `interruptible: true` to non-release CI jobs

When multiple pushes happen quickly, older pipelines waste CI minutes. Adding `interruptible: true` to lint/test/build/security jobs allows GitLab to auto-cancel superseded pipelines.

**File:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml)

### 1.3 Fix duplicate pipeline on tag+branch push

`git push origin main --tags` creates two pipelines (branch + tag). Options:
- **Option A:** Document two-step push: `git push origin main` then `git push origin vX.Y.Z`
- **Option B:** Use GitLab's `auto_cancel.on_new_commit` setting to cancel the branch pipeline when the tag pipeline starts

**Files:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml), [`docs/releasing.md`](../releasing.md)

---

## Phase 2: Changelog Improvements

### 2.1 Slim down changelog output

The changelog currently includes every conventional commit type. Trim to user-facing changes only.

**Keep in changelog:**
- 🚀 Features (`feat`)
- 🐛 Bug Fixes (`fix`)
- ⚡ Performance (`perf`)
- 🛡️ Security (commits with "security" in body)
- ◀️ Revert (`revert`)

**Skip from changelog (add `skip = true`):**
- 📚 Documentation (`docs`)
- 🚜 Refactor (`refactor`)
- ⚙️ Miscellaneous Tasks (`chore`)

**File:** [`cliff.toml`](../../cliff.toml) — update `commit_parsers` to add `skip = true` for `docs`, `refactor`, and `chore`

---

## Phase 3: Documentation Updates

### 3.1 Update "Nuxt 3" references to "Nuxt 4"

The project uses Nuxt 4 (`nuxt: ^4.3.1` in [`frontend/package.json`](../../frontend/package.json)). Multiple docs reference "Nuxt 3":

- [`README.md`](../../README.md) — "Nuxt 3 frontend", "Nuxt 3 (Vue 3) frontend"
- [`docs/index.md`](../index.md) — if applicable
- Any other documentation files

### 3.2 Update releasing docs for two-step push

Document that `git push origin main --tags` creates duplicate pipelines, and recommend the two-step approach:

```bash
git push origin main        # branch pipeline (lint/test/build/security/pages)
git push origin vX.Y.Z      # tag pipeline (full release)
```

**File:** [`docs/releasing.md`](../releasing.md)

---

## Phase 4: Cleanup

### 4.1 Clean up stale tags and container images

Delete via GitLab web UI:
- **Tags:** `v0.1.0`, `v0.1.1` (created during CI debugging, point to intermediate commits)
- **Container images:** Any images tagged `0.1.0`, `0.1.1`, `0.1.2` (from failed/partial release pipelines)
- **Releases:** Delete any auto-created releases from the old release job or failed goreleaser runs

---

## Files to Modify

| File | Phase | Description |
|------|-------|-------------|
| `.gitlab-ci.yml` | 1.1, 1.2, 1.3 | Add `--output=type=cacheonly`, `interruptible: true`, pipeline dedup |
| `cliff.toml` | 2.1 | Skip docs/refactor/chore from changelog |
| `README.md` | 3.1 | Update Nuxt 3 → Nuxt 4 |
| `docs/releasing.md` | 3.2 | Document two-step push workflow |
| `docs/index.md` | 3.1 | Update Nuxt 3 → Nuxt 4 (if applicable) |
