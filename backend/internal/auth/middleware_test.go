package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware_AuthDisabled(t *testing.T) {
	cfg := MiddlewareConfig{
		AuthEnabled: false,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			t.Error("expected user in context")
			return
		}
		if user.CN != "single-user-mode" {
			t.Errorf("expected single-user-mode, got %q", user.CN)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_ValidCert(t *testing.T) {
	cert := generateTestCertForMiddleware(t, "testuser")

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      []byte("test-secret-32-bytes-long-key!!"),
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			t.Error("expected user in context")
			return
		}
		if user.CN != "testuser" {
			t.Errorf("expected CN 'testuser', got %q", user.CN)
		}
		if user.AuthMethod != "cert" {
			t.Errorf("expected AuthMethod 'cert', got %q", user.AuthMethod)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if rec.Header().Get("X-Auth-User") != "testuser" {
		t.Errorf("expected X-Auth-User header 'testuser', got %q", rec.Header().Get("X-Auth-User"))
	}

	if rec.Header().Get("X-Auth-Method") != "cert" {
		t.Errorf("expected X-Auth-Method header 'cert', got %q", rec.Header().Get("X-Auth-Method"))
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, _, err := GenerateToken("tokenuser", 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      secret,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			t.Error("expected user in context")
			return
		}
		if user.CN != "tokenuser" {
			t.Errorf("expected CN 'tokenuser', got %q", user.CN)
		}
		if user.AuthMethod != "token" {
			t.Errorf("expected AuthMethod 'token', got %q", user.AuthMethod)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	if rec.Header().Get("X-Auth-User") != "tokenuser" {
		t.Errorf("expected X-Auth-User header 'tokenuser', got %q", rec.Header().Get("X-Auth-User"))
	}

	if rec.Header().Get("X-Auth-Method") != "token" {
		t.Errorf("expected X-Auth-Method header 'token', got %q", rec.Header().Get("X-Auth-Method"))
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      secret,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for invalid token")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, _, err := GenerateToken("tokenuser", -1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      secret,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for expired token")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_NoAuth(t *testing.T) {
	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      []byte("test-secret-32-bytes-long-key!!"),
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without auth")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_TokenValidator(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, _, err := GenerateToken("tokenuser", 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Token validator that rejects the token (simulates revocation)
	validator := func(tok string) (string, error) {
		return "", ErrTokenRevoked
	}

	cfg := MiddlewareConfig{
		AuthEnabled:    true,
		Secret:         secret,
		TokenValidator: validator,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for revoked token")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestMiddleware_WithLogger(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")
	var logBuf bytes.Buffer
	logger := NewSecurityLogger(&logBuf)

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      secret,
		Logger:      logger,
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test auth failure logging
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logOutput := logBuf.String()
	if logOutput == "" {
		t.Error("expected log output for auth failure")
	}
}

func TestRequireCertAuth_AuthDisabled(t *testing.T) {
	cfg := MiddlewareConfig{
		AuthEnabled: false,
	}

	handler := RequireCertAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			t.Error("expected user in context")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireCertAuth_RejectsToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, _, err := GenerateToken("tokenuser", 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	cfg := MiddlewareConfig{
		AuthEnabled: true,
		Secret:      secret,
	}

	handler := RequireCertAuth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for token auth")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func generateTestCertForMiddleware(t *testing.T, cn string) *x509.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(1 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}
