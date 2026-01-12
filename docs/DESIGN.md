# Cue - Design Document

## Overview

Cue is a web-based knowledge management tool for user-populated information on various topics. Items have titles and markdown descriptions with links to URLs and network shares.

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Web UI        │     │   REST API      │     │   Storage       │
│   (TypeScript)  │────▶│   (Go)          │────▶│   (SQLite+FTS5) │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │
        │              ┌────────┴────────┐
        │              │ Future clients: │
        │              │ - Chrome ext    │
        │              │ - macOS app     │
        │              └─────────────────┘
```

## Technology Choices

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Backend | **Go** | Statically typed, single binary deployment, excellent HTTP stdlib |
| Frontend | **TypeScript + React** | Industry standard, good markdown editor ecosystem |
| Storage | **SQLite with FTS5** | Zero-config, full-text search built-in, handles 1000+ items easily, single file |
| Markdown Editor | **CodeMirror 6** | Modern, extensible, good markdown support |

## Data Model

### Item
```typescript
interface Item {
  id: string;           // UUID
  title: string;        // Unique, searchable
  link?: string;        // Optional URL or file path
  content: string;      // Markdown body
  createdAt: string;    // ISO 8601
  updatedAt: string;    // ISO 8601
}
```

## REST API

### Items
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/items` | List all items |
| GET | `/api/items?q=term` | Full-text search with BM25 ranking |
| GET | `/api/items/:id` | Get single item |
| POST | `/api/items` | Create item |
| PUT | `/api/items/:id` | Update item |
| DELETE | `/api/items/:id` | Delete item |

### Authentication (Multi-User Mode)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/whoami` | Current user info |
| POST | `/api/tokens` | Create API token |
| GET | `/api/tokens` | List user's tokens |
| DELETE | `/api/tokens/:id` | Revoke token |

### System
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check (always public) |
| GET | `/api/status` | Version and server info |

## Search Behavior

1. User enters search term
2. API searches title and content using SQLite FTS5 (BM25 ranking)
3. Results returned ordered by relevance
4. If no exact title match exists, UI shows "Create new item: [term]" option

## Test Architecture

```
tests/
├── unit/           # Pure function tests, no I/O
├── integration/    # API tests against real DB
└── system/         # Full stack tests via HTTP
```

All tests runnable via single command: `./build.sh test`

## Implementation Status

### Completed (v0.1.0)
- Working Go backend with SQLite+FTS5
- React frontend with search + create/edit
- TLS with self-signed certificates
- Single-user mode (no auth)
- Multi-user mode with mTLS client certificates
- Token-based API authentication
- Health and status endpoints
- Full test suite

### In Progress
- Frontend token management UI
- User indicator in header
- Security audit logging

### Designed (See [AUTH_DESIGN.md](AUTH_DESIGN.md))
- CRL/OCSP certificate revocation
- Token validation caching
- LDAP integration for user attributes

For current backlog and priorities, see [PLAN.md](PLAN.md).

## Directory Structure

```
cue/
├── AGENTS.md           # AI agent instructions
├── README.md           # User quickstart guide
├── CHANGELOG.md        # Release notes
├── build.sh            # Build/test commands
├── docs/
│   ├── DESIGN.md       # This document
│   ├── PLAN.md         # Project plan and backlog
│   └── AUTH_DESIGN.md  # Authentication design
├── backend/
│   ├── cmd/cue/        # Main entry point
│   ├── internal/
│   │   ├── api/        # HTTP handlers
│   │   ├── auth/       # mTLS auth, tokens, middleware
│   │   └── store/      # SQLite storage
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── api/        # API client
│   │   └── App.tsx
│   └── package.json
└── certs/              # Certificates (gitignored)
```

## Design Decisions

- **Metadata**: Minimal - title, link, content, timestamps only
- **Versioning**: None - current state only
- **Attachments**: Links only - no file uploads
- **Backend**: Go with SQLite+FTS5
- **Search results**: Title + content snippet (~100 chars)
- **Authentication**: mTLS client certificates + API tokens
- **Authorization**: None (all authenticated users have full access)

For detailed authentication design, see [AUTH_DESIGN.md](AUTH_DESIGN.md).
