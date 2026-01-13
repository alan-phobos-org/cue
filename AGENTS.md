# Cue - Agent Instructions

Instructions for AI coding agents working on this project.

## Vision

Solve the problem of "I know I wrote this down somewhere" across messy networks and scattered notes. Help teams rediscover misplaced knowledge and share accumulated expertise without overhead. Should feel as fast as grepping local files. Single-binary deployment is non-negotiable; complexity belongs in the build, not the user's environment. Must work on mobile for capturing things on the go.

## Project Overview
Knowledge management tool with REST API backend (Go) and React frontend (TypeScript).
Items have titles, optional primary links (URL or file path), and markdown content.

## Commands
Run `./build.sh help` for full details. Key commands:
- `./build.sh build` - Build backend and frontend
- `./build.sh run` - Build and run server (no TLS, http://localhost:31337)
- `./build.sh dist` - Full release: lint, test, build, package as tar.gz
- `./build.sh dist-all` - Build releases for all supported platforms
- `./build.sh deploy-local` - Deploy single-user mode (uses dist, TLS)
- `./build.sh deploy-local-multiuser` - Deploy with mTLS auth (uses dist)
- `./build.sh test` - Run all fast tests (unit + integration + frontend)
- `./build.sh test-unit` - Backend unit tests only
- `./build.sh test-int` - Backend integration tests only
- `./build.sh test-frontend` - Frontend unit tests (vitest, fast)
- `./build.sh test-system` - System tests (full stack via HTTP)
- `./build.sh test-e2e` - E2E browser tests (playwright, requires running server)
- `./build.sh dev-backend` - Run backend via go run (TLS)
- `./build.sh dev-frontend` - Run Vite dev server (with API proxy)
- `./build.sh certs` - Generate PKI (CA, server, client certs)
- `./build.sh lint` - Run linters
- `./build.sh check` - Run lint + test (pre-commit check)
- `./build.sh prepare-release` - Run all checks and show changes since last tag
- `./build.sh release X.Y.Z` - Create release commit and tag
- `./build.sh clean` - Remove build artifacts

## Architecture
- Backend: Go with SQLite+FTS5 storage (`backend/`)
- Frontend: TypeScript/React built with Vite (`frontend/`)
- API-first design - all functionality accessible via REST
- Frontend embedded in Go binary for single-binary deployment
- Versioning: git tags/describe, injected at build via ldflags, exposed via `/api/status`

## Key Files
- `backend/internal/store/store.go` - SQLite storage with FTS5 search
- `backend/internal/api/api.go` - REST API handlers
- `backend/internal/auth/` - mTLS auth, token generation, middleware
- `frontend/src/App.tsx` - Main React component
- `frontend/src/api/client.ts` - API client

## Documentation
- `README.md` - User-facing quickstart and deployment guide
- `CHANGELOG.md` - Release notes and version history
- `docs/PLAN.md` - Project vision, current stage, and backlog
- `docs/DESIGN.md` - High-level architecture and future features
- `docs/AUTH_DESIGN.md` - Authentication system design

## Testing
Tests require FTS5 build tag (handled by build.sh).
Run tests frequently after changes.
- Fast tests: `./build.sh test` (~1s) - vitest for frontend, go test for backend
- Full E2E: `./build.sh test-e2e` - playwright browser tests (slower, more thorough)

## Code Style
- Go: gofmt, short variable names
- TypeScript: standard React patterns
- Minimal error handling in MVP

## Workflow
- All project documentation lives in `docs/` (except `README.md`, `CHANGELOG.md`, and this file)
- Update `docs/PLAN.md` when features are completed or backlog changes
- Update `docs/DESIGN.md` when architecture evolves

## Release Process
To create a release:
```bash
# Step 1: Run all automated checks and tests
./build.sh prepare-release

# Step 2: Update CHANGELOG.md with release notes
# Add a new section: ## [X.Y.Z] - YYYY-MM-DD

# Step 3: Review docs (AGENTS.md, README.md, docs/) for completed work
# Remove or mark done any TODO items, planned features now implemented, etc.

# Step 4: Create the release commit and tag
./build.sh release X.Y.Z

# Step 5: Push to remote
git push origin main vX.Y.Z
```

The `prepare-release` target runs: check (lint + tests), system tests, dist build, shows changes since last tag.
The `release` target: validates semver, checks CHANGELOG.md entry, creates release commit (if needed), creates annotated tag.

## Code Review Notes (2026-01-11)

A full code review was conducted. Key findings documented in `docs/CODE_REVIEW.md`.

### Security Fixes Applied
1. **XSS in Markdown Renderer** - FIXED: Added `isSafeUrl()` and `sanitizeUrl()` to block javascript:, data:, and other dangerous URL schemes
2. **React useCallback Dependencies** - FIXED: Wrapped `saveItem` in useCallback, added to dependency arrays
3. **X-Forwarded-For Spoofing** - FIXED: Added `TrustProxy` config option, consolidated to `auth.ExtractSourceIP()`
4. **Token Expiration Check** - FIXED: Added expiration check in `ValidateTokenHash()` database query
5. **MkdirAll Error Handling** - FIXED: Now properly checks and handles directory creation errors
6. **CA Certificate Double-Read** - FIXED: Reuses caCertPool for TLS config

### Remaining Technical Debt
- Custom markdown renderer could still be replaced with proper library (marked, remark)
- `created_by` column in items table is unused (has default value, low priority)
- macOS-specific commands in build.sh for multiuser deployment

### Security Patterns
- Use `auth.ExtractSourceIP(r, cfg.TrustProxy)` for IP extraction - only trust headers behind proxy
- Validate URLs before using in href attributes (see `isSafeUrl()` in App.tsx)
- Check token expiration at database level as defense-in-depth

### React Patterns
- Use callback form for setState when referencing previous state: `setItems(prev => prev.filter(...))`
- Wrap async handlers in useCallback with proper dependency arrays
- Include function dependencies in useCallback dependency arrays

## Agent Workflows

### What's Next

When asked "what's next" or similar, run this workflow to provide a concise project status summary:

**1. Run Status Command**
```bash
./build.sh status
```
This provides working copy state, remote sync, CI status, releases, and recent commits.

**2. Review Plan**
Read `docs/PLAN.md` and compare against the status output:
- Current phase completion vs what's been released
- Next planned milestone or backlog items ready to start
- Any blockers or dependencies

**3. Summary Report**
Combine the status output with plan review to provide:
- Current state (working copy, CI health, version)
- Plan progress (what phase we're in, what's next)
- **Suggested next step**: one clear recommendation

Keep the report brief (10-15 lines max). Focus on actionable information.