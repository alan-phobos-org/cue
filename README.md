# Cue

A knowledge management tool with a REST API backend (Go) and React frontend (TypeScript). Store items with titles, optional links, and markdown content with full-text search.

## Prerequisites

- Go 1.21+
- Node.js 18+
- SQLite with FTS5 support

## Quick Start

```bash
# Build and run (single-user mode, no auth)
./build.sh run
```

Server starts at http://localhost:31337

## Installation

### Build from Source

```bash
# Build backend and frontend
./build.sh build

# Binary output: bin/cue
```

### Run Tests

```bash
# All fast tests (~1s)
./build.sh test

# E2E browser tests (requires running server)
./build.sh test-e2e
```

## Deployment Modes

### Single-User Mode (No Authentication)

For personal use on a trusted network. No certificates required.

```bash
# Build, test, and start with TLS (self-signed)
./build.sh deploy-local
```

This will:
1. Build the frontend and backend
2. Run tests
3. Generate self-signed certificates (if not present)
4. Start the server at https://localhost:31337

All endpoints are publicly accessible in this mode.

### Multi-User Mode (mTLS Authentication)

For shared deployments requiring user authentication via client certificates.

```bash
# Build, test, and start with mTLS
./build.sh deploy-local-multiuser
```

This will:
1. Build the frontend and backend
2. Run tests
3. Generate a CA and certificates (if not present)
4. Install CA and client certificate to macOS keychain
5. Start the server at https://localhost:31337

Users must present a valid client certificate signed by the CA to access the application.

#### Generated Certificates

After running `./build.sh certs`, the `certs/` directory contains:

| File | Description |
|------|-------------|
| `ca.crt`, `ca.key` | Certificate Authority |
| `server.crt`, `server.key` | Server certificate (CN=localhost) |
| `client.crt`, `client.key` | Client certificate (CN=localuser) |

#### Creating Additional User Certificates

```bash
# Generate a new client certificate
openssl req -newkey rsa:4096 -keyout certs/alice.key -out certs/alice.csr \
    -nodes -subj "/CN=alice"
openssl x509 -req -in certs/alice.csr -CA certs/ca.crt -CAkey certs/ca.key \
    -CAcreateserial -out certs/alice.crt -days 365 \
    -extfile <(printf "extendedKeyUsage=clientAuth")
rm certs/alice.csr
```

#### Installing Client Certificate (macOS)

```bash
# Convert to PKCS12 format
openssl pkcs12 -export -out certs/alice.p12 \
    -inkey certs/alice.key -in certs/alice.crt \
    -passout pass:changeme -legacy

# Import to keychain
security import certs/alice.p12 -k ~/Library/Keychains/login.keychain-db \
    -P changeme -T /Applications/Safari.app
```

## Configuration

### Command-Line Flags

```
-addr string     Listen address (default ":31337")
-db string       Database file path (default "cue.db")
-cert string     TLS certificate file
-key string      TLS private key file
-ca string       CA certificate for client verification (enables multi-user auth)
```

### Running Modes

| Flags | Mode | Description |
|-------|------|-------------|
| (none) | HTTP | Plain HTTP, no auth |
| `-cert -key` | HTTPS | TLS enabled, no client auth |
| `-cert -key -ca` | mTLS | TLS + client certificate required |

## Development

```bash
# Run backend with hot reload
./build.sh dev-backend

# Run frontend dev server (proxies API to backend)
./build.sh dev-frontend

# Run linters
./build.sh lint

# Pre-commit check (lint + test)
./build.sh check
```

## API

### Items

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/items` | List all items |
| GET | `/api/items?q=search` | Search items |
| GET | `/api/items/:id` | Get item by ID |
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

### Example: Create an Item

```bash
curl -X POST https://localhost:31337/api/items \
    --cacert certs/ca.crt \
    --cert certs/client.crt \
    --key certs/client.key \
    -H "Content-Type: application/json" \
    -d '{"title": "Example", "content": "# Hello\n\nMarkdown content here."}'
```

### Example: Use API Token

```bash
# Create token (requires client cert)
TOKEN=$(curl -s -X POST https://localhost:31337/api/tokens \
    --cacert certs/ca.crt \
    --cert certs/client.crt \
    --key certs/client.key \
    -H "Content-Type: application/json" \
    -d '{"name": "my-script"}' | jq -r .token)

# Use token for API calls
curl https://localhost:31337/api/items \
    --cacert certs/ca.crt \
    -H "Authorization: Bearer $TOKEN"
```

## Architecture

```
backend/
  cmd/cue/          # Main entry point
  internal/
    api/            # REST API handlers
    auth/           # mTLS auth, tokens, middleware
    store/          # SQLite + FTS5 storage

frontend/
  src/
    App.tsx         # Main React component
    api/client.ts   # API client
    components/     # UI components
```

The frontend is embedded in the Go binary for single-binary deployment.

## License

MIT
