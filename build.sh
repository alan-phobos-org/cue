#!/bin/bash
set -euo pipefail

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-X main.version=$VERSION"

# Default port - keep in sync with backend/cmd/cue/main.go DefaultPort
DEFAULT_PORT="31337"

show_help() {
    cat << EOF
Cue Build System - v$VERSION

Usage: $0 <target>

Build Targets:
  build             Build frontend and backend (outputs to bin/cue)
  build-backend     Build Go backend only
  build-frontend    Build React frontend only (outputs to backend/cmd/cue/dist/)

Test Targets:
  test              Run all fast tests (unit + integration + frontend)
  test-unit         Run backend unit tests only
  test-int          Run backend integration tests only
  test-frontend     Run frontend unit tests (vitest)
  test-system       Build and run system tests (full stack via HTTP)
  test-e2e          Run E2E browser tests (playwright, requires running server)

Quality Targets:
  lint              Run gofmt on backend, eslint on frontend
  check             Run lint + test (pre-commit check)

Distribution Targets:
  dist              Build release: lint, test, build, and package as tar.gz
  dist-all          Build release for all supported platforms

Release Targets:
  prepare-release   Run all checks and show changes since last tag
  release X.Y.Z     Create release commit and tag (requires CHANGELOG.md entry)

Development Targets:
  run               Build and run server (no TLS, http://localhost:$DEFAULT_PORT)
  dev-backend       Run backend directly via go run (TLS, no client auth)
  dev-frontend      Run Vite dev server with API proxy

Deployment Targets:
  deploy-local           Deploy locally in single-user mode (TLS, no auth)
  deploy-local-multiuser Deploy locally with mTLS client auth

Utility Targets:
  certs             Generate PKI (CA, server, client certificates)
  clean             Remove build artifacts (bin/, dist/, backend/cmd/cue/dist/)
  help              Show this help message

EOF
}

case "${1:-help}" in
    help|-h|--help)
        show_help
        exit 0
        ;;
    build)
        echo "Building cue $VERSION..."
        $0 build-frontend
        $0 build-backend
        ;;
    build-backend)
        echo "Building backend..."
        # Ensure frontend dist exists for embed (built by build-frontend)
        if [ ! -d backend/cmd/cue/dist ]; then
            echo "ERROR: backend/cmd/cue/dist not found. Run '$0 build-frontend' first."
            exit 1
        fi
        cd backend && go build -tags fts5 -ldflags "$LDFLAGS" -o ../bin/cue ./cmd/cue
        ;;
    build-frontend)
        echo "Building frontend..."
        # Vite outputs directly to backend/cmd/cue/dist (see vite.config.ts)
        cd frontend && npm install && npm run build
        ;;
    test)
        echo "Running all fast tests..."
        $0 test-unit
        $0 test-int
        $0 test-frontend
        ;;
    test-unit)
        echo "Running backend unit tests..."
        # Create stub dist directory if it doesn't exist (for embed directive)
        mkdir -p backend/cmd/cue/dist
        [ -f backend/cmd/cue/dist/index.html ] || echo '<!DOCTYPE html><html><body>stub</body></html>' > backend/cmd/cue/dist/index.html
        cd backend && go test -tags fts5 -v -short ./...
        ;;
    test-int)
        echo "Running backend integration tests..."
        # Create stub dist directory if it doesn't exist (for embed directive)
        mkdir -p backend/cmd/cue/dist
        [ -f backend/cmd/cue/dist/index.html ] || echo '<!DOCTYPE html><html><body>stub</body></html>' > backend/cmd/cue/dist/index.html
        cd backend && go test -tags fts5 -v -run Integration ./...
        ;;
    test-frontend)
        echo "Running frontend tests..."
        cd frontend && npm install && npm test
        ;;
    test-system)
        echo "Running system tests..."
        $0 build
        cd tests/system && go test -v ./...
        ;;
    test-e2e)
        echo "Running E2E tests (requires server on :$DEFAULT_PORT)..."
        cd frontend && npm run test:e2e
        ;;
    lint)
        echo "Running linters..."
        cd backend && gofmt -l -w .
        cd ../frontend && npm run lint 2>/dev/null || echo "No frontend lint configured, skipping"
        ;;
    check)
        echo "Running pre-commit checks..."
        $0 lint && $0 test
        ;;
    run)
        $0 build
        ./bin/cue
        ;;
    deploy-local)
        echo "Deploying locally (single-user mode)..."
        # Build dist if binary doesn't exist or is older than source
        DIST_TAR="dist/cue-${VERSION}-$(go env GOOS)-$(go env GOARCH).tar.gz"
        if [ ! -f bin/cue ] || [ ! -f "$DIST_TAR" ]; then
            echo "Building distribution..."
            $0 dist
        else
            echo "Using existing dist: $DIST_TAR"
        fi
        # Generate certs if they don't exist
        if [ ! -f certs/server.crt ]; then
            echo "Generating certificates..."
            $0 certs
        fi
        echo "Starting local instance (single-user mode, TLS)..."
        echo "  - Server: https://localhost:$DEFAULT_PORT"
        ./bin/cue -cert certs/server.crt -key certs/server.key
        ;;
    deploy-local-multiuser)
        echo "Deploying locally (multiuser mode with auth)..."
        # Build dist if binary doesn't exist or is older than source
        DIST_TAR="dist/cue-${VERSION}-$(go env GOOS)-$(go env GOARCH).tar.gz"
        if [ ! -f bin/cue ] || [ ! -f "$DIST_TAR" ]; then
            echo "Building distribution..."
            $0 dist
        else
            echo "Using existing dist: $DIST_TAR"
        fi
        # Generate certs if they don't exist
        if [ ! -f certs/ca.crt ]; then
            echo "Generating certificates..."
            $0 certs
        fi
        # Install CA to keychain if not already present
        if ! security find-certificate -c "Cue CA" ~/Library/Keychains/login.keychain-db >/dev/null 2>&1; then
            echo "Installing CA certificate to keychain..."
            security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db certs/ca.crt 2>/dev/null || true
        fi
        # Install client cert to keychain if not already present
        if ! security find-identity -p ssl-client 2>/dev/null | grep -q "localuser"; then
            echo "Installing client certificate to keychain..."
            # Convert to PKCS12 format (-legacy required for macOS keychain compatibility)
            openssl pkcs12 -export -out certs/client.p12 \
                -inkey certs/client.key -in certs/client.crt \
                -passout pass:cue -legacy 2>/dev/null
            # Import to login keychain
            security import certs/client.p12 -k ~/Library/Keychains/login.keychain-db \
                -P cue -T /usr/bin/security -T /Applications/Safari.app \
                -T "/Applications/Google Chrome.app" -T /Applications/Firefox.app 2>/dev/null || true
            rm -f certs/client.p12
            echo "Client certificate installed (CN=localuser)"
        fi
        echo "Starting local instance (multiuser mode)..."
        echo "  - Server: https://localhost:$DEFAULT_PORT"
        echo "  - CA: certs/ca.crt"
        echo "  - Client cert: certs/client.crt (CN=localuser)"
        ./bin/cue -cert certs/server.crt -key certs/server.key -ca certs/ca.crt
        ;;
    dev-backend)
        # Generate certs if they don't exist
        if [ ! -f certs/server.crt ]; then
            echo "Generating certificates..."
            $0 certs
        fi
        echo "Starting backend with TLS (no client auth)..."
        echo "  - Server: https://localhost:$DEFAULT_PORT"
        cd backend && go run -tags fts5 ./cmd/cue -cert ../certs/server.crt -key ../certs/server.key
        ;;
    dev-frontend)
        cd frontend && npm run dev
        ;;
    certs)
        mkdir -p certs
        # Generate CA
        openssl req -x509 -newkey rsa:4096 -keyout certs/ca.key -out certs/ca.crt \
            -days 365 -nodes -subj "/CN=Cue CA" 2>/dev/null
        # Generate server cert signed by CA (with SANs for localhost)
        openssl req -newkey rsa:4096 -keyout certs/server.key -out certs/server.csr \
            -nodes -subj "/CN=localhost" 2>/dev/null
        openssl x509 -req -in certs/server.csr -CA certs/ca.crt -CAkey certs/ca.key \
            -CAcreateserial -out certs/server.crt -days 365 \
            -extfile <(printf "subjectAltName=DNS:localhost,IP:127.0.0.1,IP:::1\nextendedKeyUsage=serverAuth") 2>/dev/null
        # Generate client cert signed by CA
        openssl req -newkey rsa:4096 -keyout certs/client.key -out certs/client.csr \
            -nodes -subj "/CN=localuser" 2>/dev/null
        openssl x509 -req -in certs/client.csr -CA certs/ca.crt -CAkey certs/ca.key \
            -CAcreateserial -out certs/client.crt -days 365 \
            -extfile <(printf "extendedKeyUsage=clientAuth") 2>/dev/null
        rm -f certs/*.csr
        echo "Generated PKI hierarchy in certs/:"
        echo "  CA:     ca.crt, ca.key"
        echo "  Server: server.crt, server.key (CN=localhost, SANs: localhost, 127.0.0.1, ::1)"
        echo "  Client: client.crt, client.key (CN=localuser)"
        ;;
    dist)
        echo "Building release distribution..."
        echo "Step 1/4: Linting..."
        $0 lint
        echo "Step 2/4: Running tests..."
        $0 test
        echo "Step 3/4: Building..."
        $0 build
        echo "Step 4/4: Packaging..."
        mkdir -p dist
        DIST_NAME="cue-${VERSION}-$(go env GOOS)-$(go env GOARCH)"
        tar -czf "dist/${DIST_NAME}.tar.gz" -C bin cue
        echo "Created dist/${DIST_NAME}.tar.gz"
        ;;
    dist-all)
        echo "Building release distributions for all platforms..."
        echo "Step 1/4: Linting..."
        $0 lint
        echo "Step 2/4: Running tests..."
        $0 test
        echo "Step 3/4: Building frontend..."
        $0 build-frontend
        echo "Step 4/4: Building and packaging for each platform..."
        mkdir -p dist
        # Note: FTS5 requires CGO. Cross-compilation needs appropriate C toolchain.
        # Native build (current platform)
        echo "  Building for $(go env GOOS)/$(go env GOARCH) (native)..."
        $0 build-backend
        DIST_NAME="cue-${VERSION}-$(go env GOOS)-$(go env GOARCH)"
        tar -czf "dist/${DIST_NAME}.tar.gz" -C bin cue
        echo "  Created dist/${DIST_NAME}.tar.gz"
        # Linux AMD64 (requires cross-compilation toolchain for CGO)
        if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then
            echo "  Building for linux/amd64..."
            cd backend && CC=x86_64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
                go build -tags fts5 -ldflags "$LDFLAGS" -o ../bin/cue-linux-amd64 ./cmd/cue && cd ..
            tar -czf "dist/cue-${VERSION}-linux-amd64.tar.gz" -C bin cue-linux-amd64
            rm -f bin/cue-linux-amd64
            echo "  Created dist/cue-${VERSION}-linux-amd64.tar.gz"
        else
            echo "  Skipping linux/amd64 (no cross-compiler: install musl-cross for CGO support)"
        fi
        echo "Distribution complete. Files in dist/"
        ls -la dist/
        ;;
    prepare-release)
        # Run all checks and tests required before release
        echo "=== Preparing release ==="
        echo ""

        # Step 1: Build check (format, lint, unit tests)
        echo "Step 1/4: Running build check..."
        $0 check
        echo "✓ Build check passed"
        echo ""

        # Step 2: System tests
        echo "Step 2/4: Running system tests..."
        $0 test-system
        echo "✓ System tests passed"
        echo ""

        # Step 3: Distribution build
        echo "Step 3/4: Building distribution..."
        $0 dist
        echo "✓ Distribution build passed"
        echo ""

        # Step 4: Show changes since last tag
        echo "Step 4/4: Changes since last release..."
        echo ""
        LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
        if [ -n "$LAST_TAG" ]; then
            echo "Last release: $LAST_TAG"
            echo ""
            echo "Commits since $LAST_TAG:"
            git log --oneline "$LAST_TAG"..HEAD
            echo ""
            echo "Files changed:"
            git diff --stat "$LAST_TAG"..HEAD | tail -1
        else
            echo "No previous release tag found"
            echo ""
            echo "All commits:"
            git log --oneline
        fi

        echo ""
        echo "=== Release preparation complete ==="
        echo ""
        echo "Next steps:"
        echo "  1. Review the changes above"
        echo "  2. Update CHANGELOG.md with release notes"
        echo "  3. Run: ./build.sh release <version>"
        echo ""
        echo "Example: ./build.sh release 0.1.0"
        ;;
    release)
        # Create a release commit and tag
        RELEASE_VERSION="${2:-}"

        if [ -z "$RELEASE_VERSION" ]; then
            echo "Usage: $0 release <version>"
            echo "Example: $0 release 0.1.0"
            exit 1
        fi

        # Validate version format (semver)
        if ! echo "$RELEASE_VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
            echo "ERROR: Invalid version format. Use semantic versioning (e.g., 0.1.0)"
            exit 1
        fi

        TAG="v$RELEASE_VERSION"

        # Check if tag already exists
        if git rev-parse "$TAG" >/dev/null 2>&1; then
            echo "ERROR: Tag $TAG already exists"
            exit 1
        fi

        # Check for uncommitted changes (excluding CHANGELOG.md which we expect to be modified)
        if ! git diff --quiet HEAD -- . ':!CHANGELOG.md'; then
            echo "ERROR: Uncommitted changes exist (other than CHANGELOG.md)"
            echo "Please commit or stash changes before releasing"
            git status --short
            exit 1
        fi

        # Check that CHANGELOG.md has an entry for this version
        if ! grep -q "## \[$RELEASE_VERSION\]" CHANGELOG.md; then
            echo "ERROR: CHANGELOG.md does not contain entry for version $RELEASE_VERSION"
            echo "Please add a '## [$RELEASE_VERSION]' section to CHANGELOG.md"
            exit 1
        fi

        # Check if CHANGELOG.md is modified (it should be, with the new version)
        if git diff --quiet HEAD -- CHANGELOG.md; then
            echo "WARNING: CHANGELOG.md has no uncommitted changes"
            echo "Did you forget to update the changelog?"
            read -p "Continue anyway? [y/N] " -n 1 -r
            echo
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        fi

        echo "Creating release $TAG..."

        # Stage and commit CHANGELOG.md if modified
        if ! git diff --quiet HEAD -- CHANGELOG.md; then
            git add CHANGELOG.md
        fi

        # Create release commit (if there are staged changes)
        if ! git diff --cached --quiet; then
            git commit -m "Release $TAG"
            echo "✓ Created release commit"
        else
            echo "No changes to commit"
        fi

        # Create annotated tag
        git tag -a "$TAG" -m "Release $TAG"
        echo "✓ Created tag $TAG"

        echo ""
        echo "=== Release $TAG created ==="
        echo ""
        echo "Next steps:"
        echo "  1. Review: git log -1 && git show $TAG"
        echo "  2. Push:   git push origin main $TAG"
        ;;
    clean)
        echo "Cleaning build artifacts..."
        rm -rf bin/ dist/ backend/cmd/cue/dist/ backend/cue.db
        echo "Cleaned: bin/, dist/, backend/cmd/cue/dist/, backend/cue.db"
        ;;
    *)
        echo "Unknown target: ${1:-}"
        echo ""
        show_help
        exit 1
        ;;
esac
