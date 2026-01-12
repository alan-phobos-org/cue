package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net/http"
	"testing"
	"time"
)

func TestGetUser_NotSet(t *testing.T) {
	ctx := context.Background()
	user := GetUser(ctx)
	if user != nil {
		t.Error("expected nil user for empty context")
	}
}

func TestWithUser_GetUser(t *testing.T) {
	ctx := context.Background()
	expectedUser := &UserContext{
		CN:         "testuser",
		DN:         "CN=testuser,O=TestOrg",
		AuthMethod: "cert",
	}

	ctx = WithUser(ctx, expectedUser)
	user := GetUser(ctx)

	if user == nil {
		t.Fatal("expected user in context")
	}

	if user.CN != expectedUser.CN {
		t.Errorf("expected CN %q, got %q", expectedUser.CN, user.CN)
	}

	if user.DN != expectedUser.DN {
		t.Errorf("expected DN %q, got %q", expectedUser.DN, user.DN)
	}

	if user.AuthMethod != expectedUser.AuthMethod {
		t.Errorf("expected AuthMethod %q, got %q", expectedUser.AuthMethod, user.AuthMethod)
	}
}

func TestExtractUserFromCert_NoTLS(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.TLS = nil

	user := ExtractUserFromCert(req)
	if user != nil {
		t.Error("expected nil user when TLS is nil")
	}
}

func TestExtractUserFromCert_NoCerts(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: nil,
	}

	user := ExtractUserFromCert(req)
	if user != nil {
		t.Error("expected nil user when no peer certificates")
	}
}

func TestExtractUserFromCert_ValidCert(t *testing.T) {
	cert := generateTestCert(t, "testuser", "TestOrg")

	req, _ := http.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	user := ExtractUserFromCert(req)
	if user == nil {
		t.Fatal("expected user from certificate")
	}

	if user.CN != "testuser" {
		t.Errorf("expected CN 'testuser', got %q", user.CN)
	}

	if user.AuthMethod != "cert" {
		t.Errorf("expected AuthMethod 'cert', got %q", user.AuthMethod)
	}

	if user.Serial == "" {
		t.Error("expected non-empty serial number")
	}
}

func TestExtractUserFromTLSState_Valid(t *testing.T) {
	cert := generateTestCert(t, "testuser", "TestOrg")

	state := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	user := ExtractUserFromTLSState(state)
	if user == nil {
		t.Fatal("expected user from TLS state")
	}

	if user.CN != "testuser" {
		t.Errorf("expected CN 'testuser', got %q", user.CN)
	}
}

func TestExtractUserFromTLSState_Nil(t *testing.T) {
	user := ExtractUserFromTLSState(nil)
	if user != nil {
		t.Error("expected nil user for nil TLS state")
	}
}

func TestSingleUserContext(t *testing.T) {
	user := SingleUserContext()

	if user == nil {
		t.Fatal("expected non-nil user")
	}

	if user.CN != "single-user-mode" {
		t.Errorf("expected CN 'single-user-mode', got %q", user.CN)
	}

	if user.AuthMethod != "none" {
		t.Errorf("expected AuthMethod 'none', got %q", user.AuthMethod)
	}
}

// generateTestCert creates a self-signed certificate for testing.
func generateTestCert(t *testing.T, cn, org string) *x509.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{org},
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
