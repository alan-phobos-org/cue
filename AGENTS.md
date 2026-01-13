# Cue - Agent Instructions

Instructions for AI coding agents working on this project.

## Vision

Solve the problem of "I know I wrote this down somewhere" across messy networks and scattered notes. Help teams rediscover misplaced knowledge and share accumulated expertise without overhead. Should feel as fast as grepping local files. Single-binary deployment is non-negotiable; complexity belongs in the build, not the user's environment. Must work on mobile for capturing things on the go.

## Documentation

| Document | Purpose | Read When |
|----------|---------|-----------|
| [AGENTS.md](AGENTS.md) | Development workflow, quick reference | Always |
| [README.md](README.md) | User quickstart, deployment | Getting started |
| [docs/REFERENCE.md](docs/REFERENCE.md) | API specs, data model, security | Implementing API changes |
| [docs/DESIGN.md](docs/DESIGN.md) | Architecture, tech choices | Major refactoring |
| [docs/PLAN.md](docs/PLAN.md) | Project plan, backlog | Planning work |
| [docs/AUTH_DESIGN.md](docs/AUTH_DESIGN.md) | Auth system design | Auth changes |
| [CHANGELOG.md](CHANGELOG.md) | Release notes | Preparing releases |

## Quick Reference

### Build Commands

| Command | Purpose |
|---------|---------|
| `./build.sh check` | **Pre-commit** (lint + test) |
| `./build.sh build` | Build backend and frontend |
| `./build.sh run` | Build and run (no TLS, localhost:31337) |
| `./build.sh test` | All fast tests (~1s) |
| `./build.sh dist` | Full release: lint, test, build, package |

### Key Files

| File | Purpose |
|------|---------|
| `backend/internal/store/store.go` | SQLite storage with FTS5 search |
| `backend/internal/api/api.go` | REST API handlers |
| `backend/internal/auth/` | mTLS auth, tokens, middleware |
| `frontend/src/App.tsx` | Main React component |
| `frontend/src/api/client.ts` | API client |

## Workflows

| Trigger | Action |
|---------|--------|
| Before any commit | `./build.sh check` |
| "what's next", "status" | `./build.sh status` → read `docs/PLAN.md` → summarize (10-15 lines) |
| "prepare release" | `./build.sh prepare-release` → update CHANGELOG.md → `./build.sh release X.Y.Z` → push |
| "run locally" | `./build.sh run` (http://localhost:31337) |
| "deploy with TLS" | `./build.sh deploy-local` |
| "deploy multi-user" | `./build.sh deploy-local-multiuser` |

## Architecture

```
Web UI (TypeScript) → REST API (Go) → SQLite+FTS5
```

- API-first design - all functionality via REST
- Frontend embedded in Go binary for single-binary deployment
- Versioning from git tags, exposed via `/api/status`
- Database migrations: versioned in `store.go` (add new `migrateVN` functions to `migrations` slice)

---

## Testing [READ IF: implementing features, fixing bugs]

Tests require FTS5 build tag (handled by build.sh).

| Command | Purpose | Speed |
|---------|---------|-------|
| `./build.sh test` | Unit + integration + frontend | ~1s |
| `./build.sh test-unit` | Backend unit tests only | <1s |
| `./build.sh test-int` | Backend integration tests | <1s |
| `./build.sh test-frontend` | Frontend unit tests (vitest) | <1s |
| `./build.sh test-system` | Full stack via HTTP | ~5s |
| `./build.sh test-e2e` | Playwright browser tests | ~30s |

---

## Release Process [READ IF: user explicitly requests release]

```bash
# 1. Run all checks
./build.sh prepare-release

# 2. Update CHANGELOG.md (add: ## [X.Y.Z] - YYYY-MM-DD)

# 3. Review docs for completed TODOs

# 4. Create release
./build.sh release X.Y.Z

# 5. Push
git push origin main vX.Y.Z
```

---

## Code Style

- Go: gofmt, short variable names
- TypeScript: standard React patterns
- Minimal error handling in MVP

## Workflow Notes

- All project documentation lives in `docs/` (except README.md, CHANGELOG.md, and this file)
- Update `docs/PLAN.md` when features are completed
- Update `docs/DESIGN.md` when architecture evolves
