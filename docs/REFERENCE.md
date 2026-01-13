# API Reference

Detailed endpoint specifications and technical reference for Cue.

**Read this file when:** implementing API changes, debugging HTTP issues, or extending endpoints.

---

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

---

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

---

## Search Behavior

1. User enters search term
2. API searches title and content using SQLite FTS5 (BM25 ranking)
3. Results returned ordered by relevance
4. If no exact title match exists, UI shows "Create new item: [term]" option

---

## Authentication Modes

### Single-User Mode (Default)
No authentication required. Use for personal deployments.

```bash
./build.sh deploy-local
```

### Multi-User Mode (mTLS)
Client certificate authentication with API tokens.

```bash
./build.sh deploy-local-multiuser
```

### Token Authentication
- Tokens generated via `/api/tokens` endpoint
- Include in requests: `Authorization: Bearer <token>`
- Token validation checks expiration at database level

---

## Configuration

### TLS Certificates

Generated via `./build.sh certs`:
```
certs/
├── ca.crt          # Certificate authority
├── ca.key          # CA private key
├── server.crt      # Server certificate
├── server.key      # Server private key
├── client.crt      # Client certificate (multi-user)
└── client.key      # Client private key
```

### Build-Time Configuration

Version injected via ldflags from git tags, exposed via `/api/status`.

---

## Directory Structure

```
cue/
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

---

## Security Patterns [READ IF: implementing security features]

### IP Extraction
Use `auth.ExtractSourceIP(r, cfg.TrustProxy)` - only trust headers behind proxy.

### URL Validation
Validate URLs before using in href attributes (see `isSafeUrl()` in App.tsx).

### Token Expiration
Check token expiration at database level as defense-in-depth.

### XSS Prevention
Custom markdown renderer uses `sanitizeUrl()` to block dangerous URL schemes (javascript:, data:, etc.).

---

## React Patterns [READ IF: modifying frontend]

- Use callback form for setState: `setItems(prev => prev.filter(...))`
- Wrap async handlers in useCallback with proper dependency arrays
- Include function dependencies in useCallback dependency arrays

---

## Code Review Notes (2026-01-11)

### Security Fixes Applied

| Issue | Fix |
|-------|-----|
| XSS in Markdown | Added `isSafeUrl()` and `sanitizeUrl()` |
| useCallback deps | Wrapped `saveItem`, added to dependency arrays |
| X-Forwarded-For spoofing | Added `TrustProxy` config, consolidated to `auth.ExtractSourceIP()` |
| Token expiration | Added check in `ValidateTokenHash()` query |
| MkdirAll error handling | Now properly checks and handles errors |
| CA cert double-read | Reuses caCertPool for TLS config |

### Remaining Technical Debt

- Custom markdown renderer could be replaced with proper library
- `created_by` column in items table is unused (has default value)
- macOS-specific commands in build.sh for multiuser deployment
