package main

import (
	"crypto/tls"
	"crypto/x509"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alanp/cue/internal/api"
	"github.com/alanp/cue/internal/auth"
	"github.com/alanp/cue/internal/store"
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

// DefaultPort is the default listen port for the server.
const DefaultPort = "31337"

//go:embed dist
var frontendFS embed.FS

func main() {
	// Handle -version flag before other flags
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println(version)
		os.Exit(0)
	}

	addr := flag.String("addr", ":"+DefaultPort, "listen address")
	dbPath := flag.String("db", "cue.db", "database path")
	certFile := flag.String("cert", "", "TLS certificate file")
	keyFile := flag.String("key", "", "TLS key file")
	caFile := flag.String("ca", "", "CA certificate for client verification (enables auth)")
	securityLog := flag.String("security-log", "security.log", "security audit log file")
	tokenTTL := flag.Duration("token-ttl", 720*time.Hour, "default token expiration")
	tokenMaxTTL := flag.Duration("token-max-ttl", 8760*time.Hour, "maximum token expiration")
	flag.Parse()

	// Validate flag combinations
	if *caFile != "" && (*certFile == "" || *keyFile == "") {
		log.Fatal("Error: -ca requires -cert and -key for mTLS")
	}

	// Ensure db directory exists
	if dir := filepath.Dir(*dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create database directory: %v", err)
		}
	}

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer s.Close()

	// Auth configuration
	authEnabled := *caFile != ""
	var authCfg api.AuthConfig
	var secLogger *auth.FileSecurityLogger
	var caCertPool *x509.CertPool

	if authEnabled {
		// Load CA certificate (once, reused for TLS config)
		caCert, err := os.ReadFile(*caFile)
		if err != nil {
			log.Fatalf("Failed to read CA certificate: %v", err)
		}
		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			log.Fatal("Failed to parse CA certificate")
		}

		// Get or create token secret
		secret, err := s.GetOrCreateTokenSecret()
		if err != nil {
			log.Fatalf("Failed to get token secret: %v", err)
		}

		// Setup security logger
		secLogger, err = auth.NewFileSecurityLogger(*securityLog)
		if err != nil {
			log.Fatalf("Failed to open security log: %v", err)
		}
		defer secLogger.Close()

		authCfg = api.AuthConfig{
			Enabled:    true,
			Secret:     secret,
			DefaultTTL: *tokenTTL,
			MaxTTL:     *tokenMaxTTL,
			Logger:     secLogger,
		}

		secLogger.LogServerStart("authenticated", *caFile)
		log.Printf("Authentication enabled: requiring client certificates signed by %s", *caFile)
	} else {
		log.Printf("WARNING: Authentication disabled - running in development mode")
	}

	apiServer := api.NewWithAuth(s, authCfg, version)

	// Create main mux
	mux := http.NewServeMux()

	// Apply auth middleware if enabled
	var apiHandler http.Handler = apiServer
	if authEnabled {
		tokenValidator := func(token string) (string, error) {
			hash := auth.HashToken(token)
			return s.ValidateTokenHash(hash)
		}

		middlewareCfg := auth.MiddlewareConfig{
			Secret:         authCfg.Secret,
			TokenValidator: tokenValidator,
			Logger:         secLogger,
			AuthEnabled:    true,
		}

		apiHandler = auth.Middleware(middlewareCfg)(apiServer)
	}

	// Public API routes (no auth required - used by load balancers)
	mux.HandleFunc("GET /api/health", apiServer.HandleHealth)
	mux.HandleFunc("GET /api/status", apiServer.HandleStatus)

	// Protected API routes
	mux.Handle("/api/", apiHandler)

	// Frontend static files
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		log.Fatalf("Failed to get dist fs: %v", err)
	}
	fileServer := http.FileServer(http.FS(distFS))

	// Serve frontend with SPA fallback
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve static file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if file exists
		if f, err := distFS.Open(path[1:]); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("Starting server on %s", *addr)

	if *certFile != "" && *keyFile != "" {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Configure client certificate verification if CA provided
		if authEnabled && caCertPool != nil {
			tlsConfig.ClientCAs = caCertPool
			tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
		}

		server := &http.Server{
			Addr:      *addr,
			Handler:   mux,
			TLSConfig: tlsConfig,
		}

		log.Printf("TLS enabled with cert=%s key=%s", *certFile, *keyFile)
		if authEnabled {
			log.Printf("mTLS enabled: client certificates will be verified against %s", *caFile)
		}
		log.Fatal(server.ListenAndServeTLS(*certFile, *keyFile))
	} else {
		log.Fatal(http.ListenAndServe(*addr, mux))
	}
}
