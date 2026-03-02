# Release Workflow

Capacitarr uses an automated release pipeline powered by [git-cliff](https://git-cliff.org/) and GitLab CI/CD. Releases follow [Semantic Versioning](https://semver.org/) and are driven entirely by [Conventional Commits](https://www.conventionalcommits.org/).

## How It Works

### Automatic Releases on `main`

When commits are pushed (or merged) to the `main` branch, the CI pipeline:

1. **Determines the next version** — `git cliff --bumped-version` analyzes all conventional commits since the last tag and calculates the appropriate semantic version bump.
2. **Checks for changes** — if the bumped version matches the current tag, no release is created (idempotent).
3. **Generates release notes** — `git cliff --unreleased --strip header` produces a changelog body from all unreleased commits.
4. **Updates version files** — `package.json` (root) and `frontend/package.json` are updated with the new version number.
5. **Updates CHANGELOG.md** — `git cliff --bump -o CHANGELOG.md` regenerates the full changelog.
6. **Creates a GitLab release** — using the `release-cli`, a tagged release is created with the generated changelog as release notes.

### Changelog Generation on Tags

When a tag is pushed (e.g., from a manual release), the `changelog` job:

1. Runs `git cliff --latest --strip header` to generate release notes for the tagged version.
2. Saves the output as a `release_notes.md` CI artifact for download or further use.

## Semantic Versioning

Version numbers follow the `MAJOR.MINOR.PATCH` format. The version bump is determined automatically from commit messages:

| Commit Type | Version Bump | Example |
|-------------|-------------|---------|
| `feat:` | **MINOR** (0.1.0 → 0.2.0) | `feat: add disk group filtering` |
| `fix:` | **PATCH** (0.1.0 → 0.1.1) | `fix: correct capacity calculation` |
| `docs:` | **PATCH** (0.1.0 → 0.1.1) | `docs: update deployment guide` |
| `refactor:` | **PATCH** (0.1.0 → 0.1.1) | `refactor: simplify poller logic` |
| `perf:` | **PATCH** (0.1.0 → 0.1.1) | `perf: optimize database queries` |
| `chore:` | **PATCH** (0.1.0 → 0.1.1) | `chore: update dependencies` |
| Any type with `BREAKING CHANGE:` footer or `!` | **MAJOR** (0.1.0 → 1.0.0) | `feat!: redesign API endpoints` |

### Skipped Types

The following commit types are excluded from the changelog but still count toward version determination:

- `test` — test additions/changes
- `ci` — CI/CD pipeline changes
- `style` — code style/formatting changes
- `build` — build system changes
- `chore(release)` — release preparation commits

## git-cliff Configuration

The changelog is configured in [`cliff.toml`](../cliff.toml) at the project root. Key settings:

- **Conventional commits parsing** — only conventional commit messages are included
- **Commit grouping** — commits are organized by type with emoji prefixes:
  - 🚀 Features (`feat`)
  - 🐛 Bug Fixes (`fix`)
  - 🚜 Refactor (`refactor`)
  - 📚 Documentation (`docs`)
  - ⚡ Performance (`perf`)
  - ⚙️ Miscellaneous Tasks (`chore`)
  - 🛡️ Security (commits with "security" in the body)
  - ◀️ Revert (`revert`)
- **Commit links** — each changelog entry links to the commit on GitLab
- **Sorted oldest-first** — commits within each group appear in chronological order

## CI Pipeline Jobs

### `changelog` Job

| Property | Value |
|----------|-------|
| **Stage** | `release` |
| **Trigger** | Tag pushes only |
| **Image** | `orhunp/git-cliff:latest` |
| **Artifact** | `release_notes.md` (expires in 1 week) |

### `release` Job

| Property | Value |
|----------|-------|
| **Stage** | `release` |
| **Trigger** | Pushes to `main` only (not tags) |
| **Image** | `registry.gitlab.com/gitlab-org/release-cli:latest` |
| **Artifacts** | `CHANGELOG.md`, `package.json`, `frontend/package.json` (expires in 1 week) |
| **Creates** | Git tag + GitLab release with changelog notes |

## Manual Release

If you need to trigger a release manually outside of CI:

```bash
# 1. Determine the next version
git cliff --bumped-version

# 2. Generate the full changelog
git cliff --bump -o CHANGELOG.md

# 3. Update package.json version (strip the 'v' prefix)
VERSION=$(git cliff --bumped-version)
SEMVER=${VERSION#v}
npm version "$SEMVER" --no-git-tag-version
cd frontend && npm version "$SEMVER" --no-git-tag-version && cd ..

# 4. Commit and tag
git add CHANGELOG.md package.json frontend/package.json
git commit -m "chore(release): $VERSION"
git tag "$VERSION"

# 5. Push with tags
git push origin main --tags
```

There is also a convenience script in the root `package.json`:

```bash
npm run release
```

This runs the full release flow locally (changelog generation, version bump, commit, and tag).

## Prerequisites

For the automatic release pipeline to work correctly:

1. **Use Conventional Commits** — all commits on `main` must follow the [Conventional Commits](https://www.conventionalcommits.org/) format. Non-conventional commits are filtered out.
2. **Merge to `main`** — releases are only triggered from the `main` branch. Feature branches and merge requests do not create releases.
3. **Do not manually create tags** on `main` that conflict with the `vMAJOR.MINOR.PATCH` format — let the pipeline manage tags.
4. **CI/CD variables** — the pipeline uses the GitLab-provided `CI_JOB_TOKEN` for creating releases; no additional tokens are needed.
