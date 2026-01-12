# Authentication Design: Client Certificate + API Token

## Overview

This document outlines the design for user authentication in Cue using client certificates signed by a provided Certificate Authority (CA). The system trusts any certificate properly signed by the CA without maintaining local user state.

## Design Principles

1. **Stateless authentication** - No local user database; trust derives from certificate validity
2. **CA-based trust** - Any certificate signed by the configured CA is trusted
3. **API token support** - Allow certificate-authenticated users to generate tokens for programmatic access
4. **Future extensibility** - Design accommodates LDAP integration for user attribute lookup
5. **Simplicity over features** - Prefer simple, auditable implementations

## Architecture

### Authentication Flow

```
┌─────────────┐     TLS Handshake      ┌─────────────────────────┐
│   Client    │────────────────────────│     Cue Server      │
│             │   Client Certificate   │                         │
└─────────────┘                        └─────────────────────────┘
                                                  │
                                                  ▼
                                       ┌─────────────────────────┐
                                       │  Certificate Validation │
                                       │  - Signed by CA?        │
                                       │  - Not expired?         │
                                       │  - Not revoked? (CRL)   │
                                       └─────────────────────────┘
                                                  │
                                                  ▼
                                       ┌─────────────────────────┐
                                       │  Extract Identity       │
                                       │  - Subject CN           │
                                       │  - Subject DN (for LDAP)│
                                       └─────────────────────────┘
```

### API Token Flow

```
┌─────────────┐   POST /api/tokens     ┌─────────────────────────┐
│   Client    │   (with client cert)   │     Cue Server      │
│             │───────────────────────▶│                         │
└─────────────┘                        └─────────────────────────┘
                                                  │
                                                  ▼
                                       ┌─────────────────────────┐
                                       │  Generate Token         │
                                       │  - HMAC-signed          │
                                       │  - Contains user CN     │
                                       │  - Expiration embedded  │
                                       └─────────────────────────┘
                                                  │
                                                  ▼
┌─────────────┐   GET /api/items       ┌─────────────────────────┐
│ API Client  │   Authorization: Bearer │     Cue Server      │
│             │───────────────────────▶│  Validates token sig    │
└─────────────┘                        └─────────────────────────┘
```

## Implementation Details

### 1. TLS Configuration

Extend the existing TLS setup in `main.go`:

```go
// Command-line flags
caFile := flag.String("ca", "", "CA certificate for client verification")

// TLS config with mutual TLS
tlsConfig := &tls.Config{
    ClientAuth: tls.RequireAndVerifyClientCert,
    ClientCAs:  caCertPool,
    MinVersion: tls.VersionTLS12,
}
```

**Configuration:**
- `-ca` flag specifies the CA certificate file
- When provided, server requires valid client certificates
- When omitted, server runs without authentication (development mode)

### 2. User Identity Extraction

Extract user identity from the verified certificate in middleware:

```go
type UserContext struct {
    CN       string   // Common Name - primary identifier
    DN       string   // Full Distinguished Name (for LDAP lookup)
    Serial   string   // Certificate serial number
    NotAfter time.Time
}

func extractUser(r *http.Request) *UserContext {
    if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
        return nil
    }
    cert := r.TLS.PeerCertificates[0]
    return &UserContext{
        CN:       cert.Subject.CommonName,
        DN:       cert.Subject.String(),
        Serial:   cert.SerialNumber.String(),
        NotAfter: cert.NotAfter,
    }
}
```

### 3. API Token Generation

Tokens are self-contained, signed strings that encode user identity and expiration:

**Token Structure:**
```
base64(payload).base64(signature)

payload = {
    "cn": "username",
    "iat": 1704067200,
    "exp": 1735689600
}
```

**Token Endpoint:**

```go
// POST /api/tokens
// Requires client certificate authentication
// Body: { "name": "my-automation", "expires_in": "720h" }
// Response: { "token": "eyJ...", "expires_at": "2025-01-01T00:00:00Z" }
```

**Token Validation:**

```go
func validateToken(tokenString string, secret []byte) (*TokenClaims, error) {
    parts := strings.Split(tokenString, ".")
    if len(parts) != 2 {
        return nil, ErrInvalidToken
    }

    payload, _ := base64.RawURLEncoding.DecodeString(parts[0])
    sig, _ := base64.RawURLEncoding.DecodeString(parts[1])

    expectedSig := hmac.New(sha256.New, secret)
    expectedSig.Write(payload)
    if !hmac.Equal(sig, expectedSig.Sum(nil)) {
        return nil, ErrInvalidSignature
    }

    var claims TokenClaims
    json.Unmarshal(payload, &claims)

    if time.Now().Unix() > claims.Exp {
        return nil, ErrTokenExpired
    }

    return &claims, nil
}
```

### 4. Authentication Middleware

```go
func authMiddleware(next http.Handler, secret []byte) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Check client certificate first
        if user := extractUser(r); user != nil {
            ctx := context.WithValue(r.Context(), userKey, user)
            next.ServeHTTP(w, r.WithContext(ctx))
            return
        }

        // Fall back to Bearer token
        auth := r.Header.Get("Authorization")
        if strings.HasPrefix(auth, "Bearer ") {
            token := strings.TrimPrefix(auth, "Bearer ")
            if claims, err := validateToken(token, secret); err == nil {
                user := &UserContext{CN: claims.CN}
                ctx := context.WithValue(r.Context(), userKey, user)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
        }

        http.Error(w, "Unauthorized", http.StatusUnauthorized)
    })
}
```

### 5. Token Secret Management

The HMAC secret for signing tokens:

```go
// Generate on first run, store in database
func getOrCreateTokenSecret(db *sql.DB) ([]byte, error) {
    var secret []byte
    err := db.QueryRow("SELECT value FROM config WHERE key = 'token_secret'").Scan(&secret)
    if err == sql.ErrNoRows {
        secret = make([]byte, 32)
        rand.Read(secret)
        db.Exec("INSERT INTO config (key, value) VALUES ('token_secret', ?)", secret)
    }
    return secret, nil
}
```

## Future LDAP Integration

The design supports future LDAP integration for enhanced user attributes:

```go
// Future: LDAPUserProvider interface
type UserProvider interface {
    // GetUserAttributes looks up additional user info by DN
    GetUserAttributes(dn string) (*UserAttributes, error)
}

type LDAPProvider struct {
    ServerURL string
    BaseDN    string
    BindDN    string
    BindPW    string
}

type UserAttributes struct {
    Email       string
    DisplayName string
    Groups      []string
}
```

**Integration Points:**
- Certificate DN maps to LDAP DN for lookups
- Groups from LDAP could enable role-based access
- Display names could enhance UI/audit logs

## API Changes

### New Endpoints

| Method | Endpoint | Auth Required | Description |
|--------|----------|---------------|-------------|
| POST | `/api/tokens` | Certificate | Generate API token |
| GET | `/api/tokens` | Certificate | List active tokens for user |
| DELETE | `/api/tokens/{id}` | Certificate | Revoke a token |
| GET | `/api/whoami` | Any | Return current user identity |

### Response Headers

All authenticated responses include:
```
X-Auth-User: <CN>
X-Auth-Method: cert|token
```

## Configuration

### Feature Flag: Enabling Authentication

Authentication is **disabled by default** for development convenience. It is enabled by providing the `-ca` flag with a valid CA certificate path.

```go
// Feature detection at startup
authEnabled := *caFile != ""

if authEnabled {
    log.Printf("Authentication enabled: requiring client certificates signed by %s", *caFile)
    // Apply auth middleware to protected routes
} else {
    log.Printf("WARNING: Authentication disabled - running in development mode")
    // All routes are publicly accessible
}
```

**Behavior by mode:**

| Mode | CA Flag | TLS Required | Token Generation | Protected Routes |
|------|---------|--------------|------------------|------------------|
| Development | omitted | No | Disabled | Public |
| Production | provided | Yes | Enabled | Require auth |

**Startup validation:**
- If `-ca` is provided but file doesn't exist → exit with error
- If `-ca` is provided but file is invalid → exit with error
- If `-cert`/`-key` provided without `-ca` → TLS enabled, auth disabled (server-only TLS)
- If `-ca` provided without `-cert`/`-key` → exit with error (mTLS requires server cert)

### Server Flags

```
-ca string
    CA certificate file for client verification (enables auth when set)
-auth-required
    When set with -ca, reject requests without valid auth (default: true when -ca set)
-token-ttl duration
    Default token expiration (default: 720h)
-token-max-ttl duration
    Maximum allowed token expiration (default: 8760h)
```

### Environment Variables (Alternative)

```
CUE_CA_FILE=/path/to/ca.crt
CUE_AUTH_REQUIRED=true
CUE_TOKEN_TTL=720h
CUE_TOKEN_MAX_TTL=8760h
```

## Security Considerations

1. **Token storage** - Tokens are stateless; revocation requires short TTLs or a revocation list
2. **Secret rotation** - Token secret rotation invalidates all tokens; plan for this
3. **Certificate revocation** - Consider CRL/OCSP checking for production
4. **Audit logging** - Log all authentication events with user identity
5. **HTTPS only** - Never run authenticated endpoints over plain HTTP
6. **Tokens are non-recoverable** - See Token Display Policy below

### Token Display Policy

**Tokens are shown exactly once at creation time and cannot be recovered.**

This is a deliberate security design:

```go
// POST /api/tokens response - token shown only here
{
    "id": "tok_abc123",           // Stored, used for listing/deletion
    "name": "my-automation",      // User-provided label
    "token": "eyJhbGc...",        // THE ACTUAL TOKEN - shown only once
    "created_at": "2025-01-10T...",
    "expires_at": "2025-02-10T..."
}

// GET /api/tokens response - token NOT included
[
    {
        "id": "tok_abc123",
        "name": "my-automation",
        "created_at": "2025-01-10T...",
        "expires_at": "2025-02-10T...",
        "last_used_at": "2025-01-10T..."  // Optional: track usage
    }
]
```

**Implementation:**
- Token value is never stored in the database
- Only a hash (SHA-256) of the token is stored for validation
- `GET /api/tokens` returns metadata only (id, name, expiry)
- UI displays token once in a copyable modal with clear "won't be shown again" warning
- If user loses token, they must create a new one

**Database schema:**
```sql
CREATE TABLE tokens (
    id TEXT PRIMARY KEY,          -- e.g., "tok_abc123"
    user_cn TEXT NOT NULL,        -- Owner's certificate CN
    name TEXT NOT NULL,           -- User-provided label
    token_hash BLOB NOT NULL,     -- SHA-256(token) for validation
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_used_at TEXT             -- Updated on each use
);
```

**Token validation flow:**
```go
func validateToken(tokenString string, db *sql.DB) (*TokenClaims, error) {
    // Parse and verify HMAC signature first (fast, no DB)
    claims, err := verifyTokenSignature(tokenString, secret)
    if err != nil {
        return nil, err
    }

    // Check token hasn't been revoked (DB lookup by hash)
    hash := sha256.Sum256([]byte(tokenString))
    var exists bool
    db.QueryRow("SELECT 1 FROM tokens WHERE token_hash = ?", hash[:]).Scan(&exists)
    if !exists {
        return nil, ErrTokenRevoked
    }

    // Update last_used_at
    db.Exec("UPDATE tokens SET last_used_at = ? WHERE token_hash = ?",
        time.Now().UTC().Format(time.RFC3339), hash[:])

    return claims, nil
}
```

## Testability

This section outlines the testing strategy, balancing coverage against test runtime. The existing codebase runs fast tests in ~1 second; auth tests should maintain this standard.

### Unit Tests (~100ms target)

Test pure functions and logic in isolation without I/O:

| Component | Tests | Notes |
|-----------|-------|-------|
| `validateToken()` | Valid token, expired token, invalid signature, malformed input | Mock time for expiration tests |
| `extractUser()` | Valid cert, nil TLS, empty peer certs, various DN formats | Use `tls.ConnectionState` structs directly |
| `verifyTokenSignature()` | Correct HMAC, wrong secret, tampered payload | Pure crypto, very fast |
| Token parsing | Base64 decode, JSON unmarshal edge cases | No external deps |

```go
// Example: unit test with no I/O
func TestValidateToken_Expired(t *testing.T) {
    secret := []byte("test-secret")
    token := createTestToken(secret, time.Now().Add(-1*time.Hour)) // Expired

    _, err := validateToken(token, secret)
    if err != ErrTokenExpired {
        t.Errorf("expected ErrTokenExpired, got %v", err)
    }
}
```

### Integration Tests (~500ms target)

Test components with real dependencies (database, TLS):

| Component | Tests | Notes |
|-----------|-------|-------|
| Token storage | Create, list, delete, hash lookup | In-memory SQLite |
| Secret management | Generate, persist, retrieve | In-memory SQLite |
| Auth middleware | Full request cycle with mock certs | `httptest.Server` |
| Certificate validation | Valid CA, wrong CA, expired cert | Test certs generated at test init |

**Test certificate generation:**
```go
// testdata/certs.go - generated once, reused across tests
var (
    testCA     *x509.Certificate
    testCAKey  *rsa.PrivateKey
    validCert  *tls.Certificate  // Signed by testCA
    wrongCert  *tls.Certificate  // Signed by different CA
    expiredCert *tls.Certificate // Signed by testCA, expired
)

func init() {
    // Generate test PKI hierarchy (~10ms once)
    testCA, testCAKey = generateTestCA()
    validCert = generateCert(testCA, testCAKey, "testuser", time.Hour)
    // ...
}
```

**Middleware integration test:**
```go
func TestAuthMiddleware_ValidCert(t *testing.T) {
    handler := authMiddleware(echoHandler, testSecret)
    server := httptest.NewUnstartedServer(handler)
    server.TLS = &tls.Config{
        ClientCAs:  testCAPool,
        ClientAuth: tls.RequireAndVerifyClientCert,
    }
    server.StartTLS()
    defer server.Close()

    client := &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                Certificates: []tls.Certificate{*validCert},
                RootCAs:      testCAPool,
            },
        },
    }

    resp, _ := client.Get(server.URL + "/api/items")
    if resp.StatusCode != 200 {
        t.Errorf("expected 200, got %d", resp.StatusCode)
    }
}
```

### System Tests (~2s target)

End-to-end tests with real binary, real TLS, real database:

| Scenario | Tests | Notes |
|----------|-------|-------|
| Auth disabled mode | Server starts without -ca, all endpoints public | Fast startup test |
| Auth enabled mode | Server requires valid certs, rejects invalid | Full mTLS handshake |
| Token lifecycle | Create token → use token → list tokens → revoke → verify revoked | Full CRUD |
| Mixed auth | Same endpoint accessed via cert and token | Both auth methods work |
| Invalid configurations | -ca without -cert, missing files | Error handling |

```go
func TestSystem_TokenLifecycle(t *testing.T) {
    // Start server with auth enabled
    cmd := exec.Command("./bin/cue",
        "-addr", ":0",
        "-ca", "testdata/ca.crt",
        "-cert", "testdata/server.crt",
        "-key", "testdata/server.key",
    )
    // ... start and get port

    // Create token via cert auth
    resp := clientWithCert.Post("/api/tokens", `{"name":"test"}`)
    token := parseToken(resp)

    // Use token for API call
    req, _ := http.NewRequest("GET", "/api/items", nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp = clientNoAuth.Do(req)
    assert(resp.StatusCode == 200)

    // Revoke and verify
    clientWithCert.Delete("/api/tokens/" + tokenID)
    resp = clientNoAuth.Do(req)
    assert(resp.StatusCode == 401)
}
```

### Test Runtime Budget

| Category | Target | Rationale |
|----------|--------|-----------|
| Unit tests | < 100ms | No I/O, run on every save |
| Integration tests | < 500ms | Real DB but in-memory, test certs pre-generated |
| System tests | < 2s | Binary startup (~200ms), TLS handshakes |
| **Total `make test`** | **< 3s** | Maintains existing fast feedback loop |

### What NOT to Test

To stay within runtime budget, defer these to manual/staging testing:

- CRL/OCSP validation (requires network, external dependencies)
- LDAP integration (requires LDAP server, future feature)
- Certificate edge cases (malformed ASN.1, rare extensions)
- Load testing / concurrent token validation

### Test Infrastructure

**New test files:**
```
backend/internal/auth/
├── auth.go           # Main auth package
├── auth_test.go      # Unit tests
├── middleware.go     # HTTP middleware
├── middleware_test.go # Integration tests with httptest
├── token.go          # Token generation/validation
├── token_test.go     # Unit tests
└── testdata/
    ├── ca.crt        # Test CA certificate
    ├── ca.key        # Test CA private key
    ├── valid.crt     # Valid client cert
    ├── valid.key
    ├── expired.crt   # Expired client cert
    └── wrong-ca.crt  # Cert from different CA
```

**Makefile additions:**
```makefile
test-auth:
	go test -tags fts5 -v ./backend/internal/auth/...

test-auth-cover:
	go test -tags fts5 -coverprofile=coverage.out ./backend/internal/auth/...
	go tool cover -html=coverage.out
```

## Security Best Practices Review

This section documents how the design aligns with industry best practices and notes trade-offs made for simplicity.

### OWASP Alignment

| OWASP Guideline | Status | Implementation |
|-----------------|--------|----------------|
| Use strong authentication | ✅ | mTLS with CA validation; HMAC-SHA256 tokens |
| Implement proper session management | ✅ | Stateless tokens with expiration; no server-side sessions |
| Protect credentials in transit | ✅ | TLS 1.2+ required when auth enabled |
| Store credentials securely | ✅ | Token hashes only; HMAC secret in DB |
| Implement account lockout | ⏭️ | Deferred - cert auth has no passwords to brute force |
| Log authentication events | ✅ | Design includes audit logging requirement |

### Token Security

| Best Practice | Implementation | Trade-off |
|--------------|----------------|-----------|
| Short-lived tokens | Default 30 days, max 1 year | Longer than ideal, but reduces user friction |
| Token rotation | Not automatic | Users create new tokens; old ones expire naturally |
| Secure token generation | `crypto/rand` for entropy | Standard Go secure random |
| Token binding | Tokens not bound to IP/device | Simplicity over security; acceptable for internal tools |
| Constant-time comparison | `hmac.Equal()` for signatures | Prevents timing attacks |

### Certificate Security

| Best Practice | Implementation | Trade-off |
|--------------|----------------|-----------|
| CA validation | Required when auth enabled | Trusts all certs from CA - simple but coarse |
| Certificate pinning | Not implemented | Unnecessary with private CA |
| Revocation checking | Deferred to Phase 2 | CRL/OCSP adds complexity and latency |
| Key usage validation | Not checked | Simplicity; assumes CA issues correct certs |

### Cryptographic Choices

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Token signature | HMAC-SHA256 | Standard, fast, sufficient for symmetric auth |
| Token hash storage | SHA-256 | One-way, no need for bcrypt (tokens are high entropy) |
| TLS version | 1.2 minimum | Broad compatibility; 1.3 would be stricter |
| Secret size | 256 bits (32 bytes) | Standard for HMAC-SHA256 |

**Why not JWT?**
- JWTs add complexity (header, algorithm negotiation, library dependencies)
- Our tokens are simpler: `base64(payload).base64(hmac)`
- No need for asymmetric signatures (single server)
- Smaller token size

**Why HMAC instead of asymmetric signatures?**
- Single server deployment - no need to verify tokens elsewhere
- Faster validation (~1000x faster than RSA)
- Simpler implementation with fewer failure modes

### Known Limitations (Accepted Trade-offs)

1. **No token revocation list (Phase 1)**: Relies on expiration. Acceptable because:
   - Short default TTL (30 days)
   - Internal tool with trusted users
   - Can add revocation in Phase 2 if needed

2. **No rate limiting on auth**: Relies on TLS handshake cost as natural rate limit. Could add rate limiting at reverse proxy layer.

3. **No IP binding for tokens**: Tokens work from any IP. Acceptable for:
   - Internal networks
   - Users with dynamic IPs
   - Adds revocation list if this becomes a concern

4. **Single HMAC secret**: All tokens signed with same secret. Rotation invalidates all tokens. Acceptable because:
   - Simple key management
   - Tokens have expiration anyway
   - Can implement key rotation with grace period later

5. **No certificate chain validation beyond CA**: Trusts any cert signed by CA directly. Acceptable because:
   - Private CA for internal use
   - No intermediate CA complexity
   - Simple trust model

### Security Checklist for Implementation

Before shipping, verify:

- [ ] TLS 1.2+ enforced when `-ca` flag provided
- [ ] Token secret generated with `crypto/rand`
- [ ] Token signatures use constant-time comparison
- [ ] Tokens never logged (even at debug level)
- [ ] Token hash stored, never plaintext
- [ ] Auth failures logged with timestamp and source IP
- [ ] Development mode (no `-ca`) logs clear warning
- [ ] Invalid cert errors don't leak CA details
- [ ] Token endpoint requires cert auth (no bootstrap problem)

## Database Schema

Two schema variants exist depending on authentication mode. Both are maintained separately.

### Schema: Auth-Disabled Mode

The existing schema unchanged - no user tracking:

```sql
-- items table (existing)
CREATE TABLE items (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    link TEXT,
    content TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE items_fts USING fts5(title, content, content=items, content_rowid=id);
```

### Schema: Auth-Enabled Mode

Extended schema with user tracking and token storage:

```sql
-- items table (extended)
CREATE TABLE items (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    link TEXT,
    content TEXT,
    created_by TEXT NOT NULL DEFAULT 'single-user-mode',  -- Certificate CN
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE VIRTUAL TABLE items_fts USING fts5(title, content, content=items, content_rowid=id);

-- tokens table (new)
CREATE TABLE tokens (
    id TEXT PRIMARY KEY,              -- e.g., "tok_abc123"
    user_cn TEXT NOT NULL,            -- Owner's certificate CN
    name TEXT NOT NULL,               -- User-provided label
    token_hash BLOB NOT NULL,         -- SHA-256(token) for validation
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_used_at TEXT                 -- Updated on each use
);

CREATE INDEX idx_tokens_user ON tokens(user_cn);
CREATE INDEX idx_tokens_hash ON tokens(token_hash);

-- config table (for token secret)
CREATE TABLE config (
    key TEXT PRIMARY KEY,
    value BLOB NOT NULL
);

-- audit_log table (for security events)
CREATE TABLE audit_log (
    id INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    event_type TEXT NOT NULL,         -- 'auth_success', 'auth_failure', 'token_created', etc.
    user_cn TEXT,                     -- NULL for failed auth attempts
    source_ip TEXT,
    details TEXT                      -- JSON with event-specific data
);

CREATE INDEX idx_audit_timestamp ON audit_log(timestamp);
CREATE INDEX idx_audit_user ON audit_log(user_cn);
```

### Single-User Mode

When authentication is disabled, all items are attributed to the special user `single-user-mode`:

- This provides consistent schema between modes
- Allows testing the full UI without certificate setup
- Items created in single-user mode retain their attribution when auth is later enabled
- The `created_by` field is informational only - no access control is enforced

### User Attribution (Not Authorization)

**Important**: The `created_by` field tracks who created an item for audit/attribution purposes. It does NOT restrict access:

- All authenticated users can read all items
- All authenticated users can edit/delete all items
- Search returns all items regardless of creator
- This is intentional for a knowledge management tool where sharing is the default

Future authorization (Phase 3) would add separate `owner` and `acl` fields if access control becomes needed.

## Frontend Integration

### Authentication State

The frontend discovers authentication state on startup:

```typescript
// api/client.ts
interface WhoAmI {
  authenticated: boolean;
  user?: {
    cn: string;           // Certificate Common Name
    authMethod: 'cert' | 'token';
  };
  mode: 'single-user' | 'authenticated';
}

async function whoami(): Promise<WhoAmI> {
  const resp = await fetch('/api/whoami');
  return resp.json();
}
```

**Caching strategy**: Call `/api/whoami` once on app load, store in React state. No localStorage caching - fresh check on each page load is simpler and ensures state accuracy.

### `/api/whoami` Endpoint

```go
// GET /api/whoami
// Always returns 200 - distinguishes modes via response body

// Auth-enabled mode, valid certificate:
{
  "authenticated": true,
  "user": {
    "cn": "alice",
    "auth_method": "cert"
  },
  "mode": "authenticated"
}

// Auth-enabled mode, valid token:
{
  "authenticated": true,
  "user": {
    "cn": "alice",
    "auth_method": "token"
  },
  "mode": "authenticated"
}

// Auth-disabled mode (single-user):
{
  "authenticated": true,
  "user": {
    "cn": "single-user-mode",
    "auth_method": "none"
  },
  "mode": "single-user"
}

// Auth-enabled mode, no valid auth:
// Returns 401 - caught by error handler, redirects to error page
```

**Security note**: The `/api/whoami` endpoint intentionally does NOT return:
- Certificate details beyond CN
- Token IDs or metadata
- Internal server state

### User Indicator

Display the current user in the header, next to the Cue title:

```tsx
// components/Header.tsx
function Header({ user }: { user: WhoAmI }) {
  return (
    <header>
      <h1>Cue</h1>
      {user.authenticated && (
        <span className="user-indicator">
          {user.user?.cn}
        </span>
      )}
    </header>
  );
}
```

Styling: subtle, non-intrusive - just the CN text, perhaps with a small user icon.

### Token Management Page

**Route**: `/settings/tokens`

**Components**:

```tsx
// pages/TokenSettings.tsx
function TokenSettings() {
  const [tokens, setTokens] = useState<TokenInfo[]>([]);
  const [newTokenName, setNewTokenName] = useState('');
  const [showTokenModal, setShowTokenModal] = useState(false);
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  // Load existing tokens
  useEffect(() => {
    fetchTokens().then(setTokens);
  }, []);

  const handleCreate = async () => {
    const result = await createToken({ name: newTokenName });
    setCreatedToken(result.token);  // The actual token value
    setShowTokenModal(true);
    setNewTokenName('');
    // Refresh list after modal closes
  };

  const handleDelete = async (id: string) => {
    await deleteToken(id);
    setTokens(tokens.filter(t => t.id !== id));
  };

  return (
    <div className="token-settings">
      <h2>API Tokens</h2>

      {/* Create new token */}
      <div className="create-token">
        <input
          placeholder="Token name (e.g., 'my-script')"
          value={newTokenName}
          onChange={e => setNewTokenName(e.target.value)}
        />
        <button onClick={handleCreate}>Create Token</button>
      </div>

      {/* Token list */}
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Created</th>
            <th>Expires</th>
            <th>Last Used</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {tokens.map(token => (
            <tr key={token.id}>
              <td>{token.name}</td>
              <td>{formatDate(token.created_at)}</td>
              <td>{formatDate(token.expires_at)}</td>
              <td>{token.last_used_at ? formatDate(token.last_used_at) : 'Never'}</td>
              <td>
                <button onClick={() => handleDelete(token.id)}>Revoke</button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {/* Token created modal */}
      {showTokenModal && (
        <TokenCreatedModal
          token={createdToken!}
          onClose={() => {
            setShowTokenModal(false);
            setCreatedToken(null);
            fetchTokens().then(setTokens);  // Refresh list
          }}
        />
      )}
    </div>
  );
}
```

**Token Created Modal**:

```tsx
// components/TokenCreatedModal.tsx
function TokenCreatedModal({ token, onClose }: { token: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(token);
    setCopied(true);
    setTimeout(() => onClose(), 1500);  // Redirect after brief confirmation
  };

  return (
    <div className="modal-overlay">
      <div className="modal">
        <h3>Token Created</h3>
        <p className="warning">
          Copy this token now. It will not be shown again.
        </p>
        <code className="token-display">{token}</code>
        <button onClick={handleCopy}>
          {copied ? 'Copied!' : 'Copy to Clipboard'}
        </button>
      </div>
    </div>
  );
}
```

### Error Page

When authentication fails (401), redirect to a styled error page:

**Route**: `/auth-error`

```tsx
// pages/AuthError.tsx
function AuthError() {
  return (
    <div className="error-page">
      <h1>Authentication Required</h1>
      <p>
        This Cue instance requires a valid client certificate to access.
      </p>

      <h2>What you need</h2>
      <ul>
        <li>A client certificate signed by this server's Certificate Authority</li>
        <li>The certificate installed in your browser</li>
      </ul>

      <h2>Getting a certificate</h2>
      <p>
        Contact your system administrator to obtain a client certificate.
        They will need to sign it with the CA configured for this server.
      </p>

      <h2>Troubleshooting</h2>
      <ul>
        <li><strong>Certificate not recognized?</strong> Ensure it's installed in your browser's certificate store</li>
        <li><strong>Certificate expired?</strong> Request a new certificate from your administrator</li>
        <li><strong>Using an API token?</strong> Tokens must be passed in the Authorization header, not via browser</li>
      </ul>
    </div>
  );
}
```

**Redirect logic** (in API client):

```typescript
// api/client.ts
async function apiRequest(url: string, options?: RequestInit) {
  const resp = await fetch(url, options);

  if (resp.status === 401) {
    window.location.href = '/auth-error';
    throw new Error('Authentication required');
  }

  return resp;
}
```

### Testing the Frontend

Frontend auth components are testable via Vitest:

```typescript
// __tests__/TokenSettings.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { TokenSettings } from '../pages/TokenSettings';

// Mock the API
vi.mock('../api/client', () => ({
  fetchTokens: vi.fn(() => Promise.resolve([
    { id: 'tok_1', name: 'test-token', created_at: '2025-01-01', expires_at: '2025-02-01' }
  ])),
  createToken: vi.fn(() => Promise.resolve({ token: 'new-token-value' })),
  deleteToken: vi.fn(() => Promise.resolve()),
}));

test('displays existing tokens', async () => {
  render(<TokenSettings />);
  expect(await screen.findByText('test-token')).toBeInTheDocument();
});

test('creates new token and shows modal', async () => {
  render(<TokenSettings />);

  fireEvent.change(screen.getByPlaceholderText(/token name/i), {
    target: { value: 'my-new-token' }
  });
  fireEvent.click(screen.getByText('Create Token'));

  expect(await screen.findByText(/will not be shown again/i)).toBeInTheDocument();
  expect(screen.getByText('new-token-value')).toBeInTheDocument();
});

test('copies token and redirects', async () => {
  const mockClipboard = { writeText: vi.fn(() => Promise.resolve()) };
  Object.assign(navigator, { clipboard: mockClipboard });

  render(<TokenSettings />);
  // ... create token flow ...

  fireEvent.click(screen.getByText('Copy to Clipboard'));
  expect(mockClipboard.writeText).toHaveBeenCalledWith('new-token-value');
});
```

## Audit Logging

All security-relevant events are logged to `security.log` in a structured format.

### Log File Location

```go
// Default: ./security.log (same directory as cue binary)
// Override: -security-log=/var/log/cue/security.log
```

### Log Format

JSON Lines format, one event per line:

```json
{"ts":"2025-01-10T14:30:00Z","event":"auth_success","user":"alice","method":"cert","ip":"192.168.1.10"}
{"ts":"2025-01-10T14:30:05Z","event":"token_created","user":"alice","token_id":"tok_abc123","ip":"192.168.1.10"}
{"ts":"2025-01-10T14:31:00Z","event":"auth_success","user":"alice","method":"token","token_id":"tok_abc123","ip":"192.168.1.50"}
{"ts":"2025-01-10T14:32:00Z","event":"auth_failure","reason":"invalid_cert","ip":"192.168.1.99"}
{"ts":"2025-01-10T14:33:00Z","event":"token_revoked","user":"alice","token_id":"tok_abc123","ip":"192.168.1.10"}
```

### Event Types

| Event | Fields | Description |
|-------|--------|-------------|
| `auth_success` | user, method, token_id (if token) | Successful authentication |
| `auth_failure` | reason, details | Failed authentication attempt |
| `token_created` | user, token_id, name, expires_at | New API token generated |
| `token_revoked` | user, token_id | Token deleted by user |
| `token_expired` | token_id | Token rejected due to expiration |
| `server_start` | mode, ca_file (if auth) | Server startup |
| `server_stop` | reason | Server shutdown |

### Sanitization Rules

**Never log**:
- Token values (the actual secret)
- Certificate private key material
- Full certificate contents
- HMAC secrets

**Always sanitize**:
- Truncate long strings (max 200 chars)
- Escape special characters in user-provided names
- Mask any accidentally-included secrets with `[REDACTED]`

```go
// logging/security.go
type SecurityLogger struct {
    file *os.File
    mu   sync.Mutex
}

func (l *SecurityLogger) Log(event SecurityEvent) {
    event.Timestamp = time.Now().UTC().Format(time.RFC3339)
    event.sanitize()  // Remove/mask sensitive fields

    l.mu.Lock()
    defer l.mu.Unlock()

    json.NewEncoder(l.file).Encode(event)
}

func (e *SecurityEvent) sanitize() {
    // Ensure no token values leak
    e.TokenValue = ""

    // Truncate user-provided strings
    if len(e.Details) > 200 {
        e.Details = e.Details[:200] + "..."
    }

    // Mask anything that looks like a secret
    e.Details = secretPattern.ReplaceAllString(e.Details, "[REDACTED]")
}
```

### Log Rotation

For Phase 1, manual rotation. Document the recommended approach:

```bash
# Rotate logs (cue reopens file on SIGHUP)
mv security.log security.log.1
kill -HUP $(pidof cue)
gzip security.log.1
```

Future: integrate with standard log rotation (logrotate, systemd journal).

## Token Validation Cache (Backlog)

**Status**: Designed but not implemented in Phase 1.

### Problem

Each token validation requires a database lookup to check revocation status. At ~1ms per query, this is acceptable for low-to-medium traffic but could become a bottleneck.

### Proposed Design

In-memory cache with short TTL:

```go
type TokenCache struct {
    cache    map[string]*cacheEntry  // key: token hash
    mu       sync.RWMutex
    ttl      time.Duration           // e.g., 30 seconds
    maxSize  int                     // e.g., 10000 entries
}

type cacheEntry struct {
    valid     bool
    claims    *TokenClaims
    expiresAt time.Time  // Cache expiry, not token expiry
}

func (c *TokenCache) Validate(token string, db *sql.DB) (*TokenClaims, error) {
    hash := sha256.Sum256([]byte(token))
    hashStr := hex.EncodeToString(hash[:])

    // Check cache first
    c.mu.RLock()
    if entry, ok := c.cache[hashStr]; ok && time.Now().Before(entry.expiresAt) {
        c.mu.RUnlock()
        if !entry.valid {
            return nil, ErrTokenRevoked
        }
        return entry.claims, nil
    }
    c.mu.RUnlock()

    // Cache miss - check database
    claims, err := validateTokenFromDB(token, db)

    // Update cache
    c.mu.Lock()
    c.cache[hashStr] = &cacheEntry{
        valid:     err == nil,
        claims:    claims,
        expiresAt: time.Now().Add(c.ttl),
    }
    c.evictOldest()  // Keep cache bounded
    c.mu.Unlock()

    return claims, err
}
```

### Trade-offs

| Aspect | Without Cache | With Cache |
|--------|---------------|------------|
| Revocation latency | Immediate | Up to TTL (30s) |
| DB load | 1 query/request | 1 query/TTL/token |
| Memory | None | ~1KB per cached token |
| Complexity | Simple | Moderate |

### When to Implement

Add caching when:
- Token validation shows up in profiling
- Request latency p99 exceeds targets
- Database becomes a bottleneck

**Not needed for**:
- < 100 requests/second
- Single-user deployments
- Development/testing

## Migration Path

### Phase 1: Basic Auth (This Design)
- Client certificate validation
- Token generation and validation
- User attribution via `created_by` field
- Security audit logging
- Frontend token management

### Phase 2: Enhanced Features (Future)
- LDAP integration for user attributes
- Token validation cache
- CRL/OCSP certificate validation
- Log aggregation/querying UI

### Phase 3: Authorization (Future)
- Per-item access control (optional)
- Role-based access control via LDAP groups
- Sharing/collaboration features

## Design Questions

1. **Token revocation strategy**: Should we implement a token revocation list (stored in DB) or rely solely on short TTLs? Short TTLs are simpler but less flexible. *Recommendation: Start with short TTLs (30 days default); add revocation list only if needed.*

2. **Multiple tokens per user**: Should users be able to create multiple named tokens (for different integrations) or just one active token? *Recommendation: Allow multiple named tokens for flexibility in automation scenarios.*

3. **Certificate field for identity**: Should we use the certificate's Common Name (CN), email (SAN), or full Distinguished Name (DN) as the primary user identifier? *Recommendation: Use CN as primary identifier; it's the most readable and commonly used.*

4. **Graceful degradation**: When running without a CA configured, should the server allow anonymous access or refuse to start? *Recommendation: Allow anonymous access for development; log a warning on startup.*

5. **Token scope limitations**: Should tokens have the same permissions as certificate auth, or should we support scoped tokens (read-only, specific endpoints)? *Recommendation: Start with full-access tokens; add scopes only when concrete use cases emerge.*
