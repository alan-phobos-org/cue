# Cue Code Review

**Date:** 2026-01-11
**Scope:** Full codebase review for legacy drift, technical debt, bugs, security issues, and performance optimizations.

---

## Executive Summary

Cue is a well-structured, production-ready knowledge management application with comprehensive authentication, full-text search, and a clean architecture. The codebase shows good separation of concerns and solid test coverage. However, there are several areas that need attention, ranging from minor bugs to potential security concerns.

**Priority Legend:**
- 🔴 **Critical** - Security risk or data loss potential
- 🟠 **High** - Bugs that affect functionality
- 🟡 **Medium** - Technical debt or code quality issues
- 🟢 **Low** - Minor improvements or cleanup

---

## Security Issues

### 🔴 SEC-1: XSS Vulnerability in Markdown Renderer

**File:** `frontend/src/App.tsx:459-505`

The `renderMarkdown()` function uses `dangerouslySetInnerHTML` after a simple regex-based HTML escaping that may not cover all XSS vectors. The markdown parser allows links with arbitrary `href` values:

```typescript
// This allows javascript: URLs
.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
```

**Risk:** A malicious user could inject `javascript:` URLs or other XSS payloads.

**Fix:**
1. Validate link URLs to only allow `http://`, `https://`, and relative paths
2. Use a proper markdown library like `marked` or `remark` with sanitization
3. At minimum, add URL validation: `if (!/^(https?:\/\/|\/|\.\/|~)/.test(url)) return;`

---

### 🟠 SEC-2: X-Forwarded-For Header Spoofing

**File:** `backend/internal/auth/middleware.go:140-155` and `backend/internal/api/api.go:427-438`

The `extractSourceIP()` function trusts the `X-Forwarded-For` and `X-Real-IP` headers unconditionally:

```go
func extractSourceIP(r *http.Request) string {
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        // Take the first IP in the chain
        if idx := strings.Index(xff, ","); idx != -1 {
            return strings.TrimSpace(xff[:idx])
        }
        return strings.TrimSpace(xff)
    }
    ...
}
```

**Risk:** Without validation, an attacker can spoof their IP address in audit logs, potentially bypassing IP-based rate limiting or confusing forensic analysis.

**Fix:**
1. Add a configuration flag for trusted proxy IPs
2. Only trust `X-Forwarded-For` when the direct connection is from a trusted proxy
3. Consider using the rightmost non-trusted IP in the chain

---

### 🟡 SEC-3: Token Expiration Not Checked in Database Validation

**File:** `backend/internal/store/store.go:387-401`

The `ValidateTokenHash()` function checks if a token exists in the database but does not verify the `expires_at` timestamp:

```go
func (s *Store) ValidateTokenHash(tokenHash []byte) (string, error) {
    var id string
    err := s.db.QueryRow("SELECT id FROM tokens WHERE token_hash = ?", tokenHash).Scan(&id)
    if err != nil {
        return "", err
    }
    // Missing: check if token is expired!
    ...
}
```

**Risk:** Expired tokens can be used until the JWT expiration check in the middleware catches them, creating a window where revoked tokens may still work.

**Note:** The auth middleware does validate JWT expiration in `token.go:82-84`, so this is defense-in-depth rather than a critical issue.

**Fix:** Add expiration check to the database query:
```go
err := s.db.QueryRow(
    "SELECT id FROM tokens WHERE token_hash = ? AND expires_at > datetime('now')",
    tokenHash,
).Scan(&id)
```

---

### 🟡 SEC-4: Missing Rate Limiting

The application has no rate limiting on authentication endpoints or item creation.

**Risk:** Brute force attacks on token guessing, denial of service through item creation spam.

**Fix:** Add rate limiting middleware, especially for:
- `/api/tokens` (token creation)
- `/api/items` POST (item creation)
- Authentication failures

---

### 🟡 SEC-5: build.sh Uses Hardcoded Password for PKCS12

**File:** `build.sh:159-161`

```bash
openssl pkcs12 -export -out certs/client.p12 \
    -inkey certs/client.key -in certs/client.crt \
    -passout pass:cue -legacy 2>/dev/null
```

**Risk:** The password "cue" is hardcoded and publicly known.

**Fix:** Generate a random password or prompt the user for one.

---

## Bugs

### 🟠 BUG-1: React useCallback Missing Dependencies

**File:** `frontend/src/App.tsx:206-215`

The `autoSave` callback has dependencies that are not properly tracked:

```typescript
const autoSave = useCallback(() => {
    if (saveTimeoutRef.current) {
        clearTimeout(saveTimeoutRef.current)
    }
    saveTimeoutRef.current = window.setTimeout(() => {
        if (mode === 'edit' && !creating && selectedItem) {
            saveItem()  // saveItem is called but not in dependencies
        }
    }, 2000)
}, [mode, creating, selectedItem, editTitle, editContent, editLink])
// Missing: saveItem in dependencies
```

**Risk:** The autosave may capture stale closures of `saveItem`, causing data loss.

**Fix:** Add `saveItem` to the dependency array, or better yet, use the functional approach to avoid stale closures.

---

### 🟠 BUG-2: useEffect Missing saveItem Dependency

**File:** `frontend/src/App.tsx:246`

```typescript
useEffect(() => {
    ...
    if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault()
        if (mode === 'edit') saveItem()
    }
    ...
}, [mode, saveItem])  // saveItem added but not wrapped in useCallback
```

The `saveItem` function is not memoized with `useCallback`, which means it changes on every render, causing the effect to re-run unnecessarily.

**Fix:** Wrap `saveItem` in `useCallback` with appropriate dependencies.

---

### 🟡 BUG-3: MkdirAll Error Ignored

**File:** `backend/cmd/cue/main.go:53-55`

```go
if dir := filepath.Dir(*dbPath); dir != "." && dir != "" {
    os.MkdirAll(dir, 0755)  // Error ignored
}
```

**Risk:** If directory creation fails, the subsequent database open will fail with a confusing error message.

**Fix:** Check and handle the error:
```go
if err := os.MkdirAll(dir, 0755); err != nil {
    log.Fatalf("Failed to create database directory: %v", err)
}
```

---

### 🟡 BUG-4: Time Parsing Errors Silently Ignored

**File:** `backend/internal/store/store.go:235-236, 258-259, 278-280`

```go
item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
```

**Risk:** If the database somehow contains malformed timestamps, the code will silently use zero times.

**Fix:** Log a warning or return an error if time parsing fails.

---

### 🟡 BUG-5: CA Certificate Read Twice

**File:** `backend/cmd/cue/main.go:70-77, 172-174`

The CA certificate file is read twice - once for the auth config and once for TLS config:

```go
// First read (line 70-77)
caCert, err := os.ReadFile(*caFile)
...

// Second read (line 172-174)
caCert, _ := os.ReadFile(*caFile)  // Also ignores error!
caCertPool := x509.NewCertPool()
caCertPool.AppendCertsFromPEM(caCert)
```

**Risk:** The second read ignores errors, and reading the file twice is inefficient.

**Fix:** Reuse the first `caCertPool` instead of creating a new one.

---

### 🟢 BUG-6: Empty Todos Array in TokenSettings

**File:** `frontend/src/components/TokenSettings.tsx:89`

```typescript
setTokens(tokens.filter((t) => t.id !== id));
```

This references the component's `tokens` state rather than using the callback form.

**Risk:** If multiple delete operations happen rapidly, race conditions could occur.

**Fix:** Use the callback form:
```typescript
setTokens(prev => prev.filter((t) => t.id !== id));
```

---

## Technical Debt

### 🟡 DEBT-1: Duplicate extractSourceIP Function

**Files:**
- `backend/internal/api/api.go:427-438`
- `backend/internal/auth/middleware.go:140-155`

The same function is duplicated in two files.

**Fix:** Move to a shared utility package.

---

### 🟡 DEBT-2: Hardcoded Default TTL Values

**Files:**
- `backend/internal/api/api.go:36-41`
- `backend/cmd/cue/main.go:43-44`

Default TTL values (720h, 8760h) are defined in multiple places.

**Fix:** Define constants in a single location.

---

### 🟡 DEBT-3: Frontend Lacks Proper Markdown Library

**File:** `frontend/src/App.tsx:458-505`

The custom markdown renderer is fragile and doesn't handle:
- Nested lists
- Numbered lists properly
- Tables
- Task lists
- Strikethrough
- Many edge cases

**Fix:** Use a proper markdown library like `marked`, `markdown-it`, or `remark`.

---

### 🟡 DEBT-4: Missing TypeScript Strict Mode

**File:** `frontend/tsconfig.json`

The frontend should enable stricter TypeScript settings:
- `strictNullChecks`
- `noImplicitAny`

---

### 🟡 DEBT-5: No CORS Configuration

The API server doesn't configure CORS headers, which could be problematic if the frontend is served from a different origin.

---

### 🟢 DEBT-6: RequireCertAuth Middleware Not Used

**File:** `backend/internal/auth/middleware.go:106-137`

The `RequireCertAuth` middleware is defined but never used. The API handlers perform cert checks manually instead.

**Fix:** Either use the middleware consistently or remove it if the current approach is preferred.

---

### 🟢 DEBT-7: macOS-specific Commands in build.sh

**File:** `build.sh:151-168`

The `deploy-local-multiuser` target uses macOS `security` commands that won't work on Linux:

```bash
if ! security find-certificate -c "Cue CA" ~/Library/Keychains/login.keychain-db
```

**Fix:** Add OS detection or document this as macOS-only.

---

## Performance Optimizations

### 🟡 PERF-1: FTS5 Index Could Use content_rowid

**File:** `backend/internal/store/store.go:56-62`

The FTS5 table uses `content_rowid='rowid'` but `items.id` is TEXT, not INTEGER. This works because SQLite assigns internal rowids, but it's an implicit dependency.

**Fix:** Consider adding an INTEGER PRIMARY KEY to `items` for explicit rowid control, or document this behavior.

---

### 🟡 PERF-2: Frontend Fetches All Items on Load

**File:** `frontend/src/App.tsx:122-132`

The frontend fetches up to 50 items on initial load regardless of whether they're needed.

**Fix:** Implement virtual scrolling or pagination in the UI.

---

### 🟡 PERF-3: No Connection Pooling Configuration

**File:** `backend/internal/store/store.go:26-38`

The SQLite connection is opened without connection pool configuration.

**Fix:** For SQLite, set `db.SetMaxOpenConns(1)` to avoid SQLITE_BUSY errors, or use WAL mode and connection pooling appropriately.

---

### 🟢 PERF-4: Autosave Creates Multiple Timeouts

**File:** `frontend/src/App.tsx:206-215`

Each keystroke clears and creates a new timeout. This is correct behavior, but the timeout reference could be managed more efficiently with `useRef`.

**Current implementation is correct but could be cleaner with debounce utility.

---

## Legacy Drift

### 🟡 LEGACY-1: Created_by Column Unused

**File:** `backend/internal/store/store.go:51`

The `created_by` column is defined in the schema but never populated with actual user data:

```sql
created_by TEXT NOT NULL DEFAULT 'single-user-mode',
```

The `Create` method doesn't accept a `createdBy` parameter.

**Fix:** Either use this field for proper user attribution or remove it.

---

### 🟢 LEGACY-2: Audit Log Table Not Used

**File:** Referenced in `docs/AUTH_DESIGN.md` but not implemented in code

The design document references an `audit_log` table that doesn't exist in the current schema.

**Fix:** Either implement audit logging to the database or update the design doc.

---

## Minor Issues

### 🟢 MINOR-1: Go Module Path Mismatch

**File:** `backend/go.mod`

```go
module github.com/alanp/cue
```

This appears to be a personal module path but the repo is in `alan-phobos-org`.

**Fix:** Update module path to match actual repository location.

---

### 🟢 MINOR-2: Prototype Directory

**Directory:** `prototype/`

Contains a single `index.html` file that appears to be an early prototype.

**Fix:** Remove if no longer needed.

---

### 🟢 MINOR-3: Shortcuts Display Wrong Key on Non-Mac

**File:** `frontend/src/App.tsx:435`

```typescript
<span className="shortcuts"><kbd>Cmd</kbd> + <kbd>S</kbd> save
```

This shows `Cmd` even on Windows/Linux where it should be `Ctrl`.

**Fix:** Detect OS and show appropriate key.

---

## Suggested Fixes Priority Order

1. **🔴 SEC-1:** Fix XSS vulnerability in markdown renderer (Critical)
2. **🟠 SEC-2:** Address X-Forwarded-For spoofing (High)
3. **🟠 BUG-1, BUG-2:** Fix React dependency issues to prevent data loss (High)
4. **🟡 SEC-3:** Add token expiration check in database (Medium)
5. **🟡 BUG-3:** Handle MkdirAll errors (Medium)
6. **🟡 DEBT-1:** Consolidate duplicate functions (Medium)
7. **🟡 DEBT-3:** Replace custom markdown with proper library (Medium)
8. **🟡 SEC-4:** Add rate limiting (Medium)
9. **🟡 LEGACY-1:** Use or remove created_by column (Medium)
10. **🟢 Others:** Address remaining low-priority items as time permits

---

## Positive Observations

The codebase has several strengths worth noting:

1. **Clean Architecture:** Good separation between API, auth, and storage layers
2. **Comprehensive Testing:** Both unit and integration tests for backend
3. **Security Logging:** Well-thought-out security event logging with sanitization
4. **mTLS Implementation:** Solid mutual TLS authentication implementation
5. **Token Security:** Proper token hashing and HMAC-based validation
6. **Build System:** Comprehensive build script with multiple targets
7. **Documentation:** Detailed design documents and README
