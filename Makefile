.PHONY: lint format check build down clean help

# ─── Code Quality ─────────────────────────────────────────────────────────────

## Run ESLint (auto-fix) + Go vet
lint:
	@echo "→ Linting frontend..."
	cd frontend && pnpm lint:fix
	@echo "→ Linting backend..."
	cd backend && go vet ./...
	@echo "✓ Lint complete"

## Run Prettier (auto-fix)
format:
	@echo "→ Formatting frontend..."
	cd frontend && pnpm format
	@echo "✓ Format complete"

## Verify code quality (no auto-fixes — CI-safe)
check:
	@echo "→ Checking frontend lint..."
	cd frontend && pnpm lint
	@echo "→ Checking frontend format..."
	cd frontend && pnpm format:check
	@echo "→ Checking backend..."
	cd backend && go vet ./...
	@echo "✓ All checks passed"

# ─── Docker ───────────────────────────────────────────────────────────────────

## Build and start via Docker Compose
build:
	docker compose up -d --build

## Stop and remove containers
down:
	docker compose down

## Full clean: remove containers, volumes, and build cache
clean:
	docker compose down -v
	docker builder prune -f

# ─── Help ─────────────────────────────────────────────────────────────────────

## Show available targets
help:
	@echo "Capacitarr Development Commands"
	@echo "================================"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint     - Auto-fix lint issues (ESLint + go vet)"
	@echo "  make format   - Auto-format code (Prettier)"
	@echo "  make check    - Verify code quality (CI-safe, no auto-fix)"
	@echo ""
	@echo "Docker:"
	@echo "  make build    - Build and start via Docker Compose"
	@echo "  make down     - Stop containers"
	@echo "  make clean    - Remove containers, volumes, and build cache"
	@echo ""
	@echo "Workflow: make lint format → commit → make build"
