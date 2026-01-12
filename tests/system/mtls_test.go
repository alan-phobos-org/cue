package system

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

type WhoAmI struct {
	Authenticated bool `json:"authenticated"`
	User          *struct {
		CN         string `json:"cn"`
		AuthMethod string `json:"auth_method"`
	} `json:"user"`
	Mode string `json:"mode"`
}

type TokenResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

func TestMTLSAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mTLS system test in short mode")
	}

	// Find project root
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "../..")
	certsDir := filepath.Join(projectRoot, "certs")

	// Check certs exist
	caCert := filepath.Join(certsDir, "ca.crt")
	serverCert := filepath.Join(certsDir, "server.crt")
	serverKey := filepath.Join(certsDir, "server.key")
	clientCert := filepath.Join(certsDir, "client.crt")
	clientKey := filepath.Join(certsDir, "client.key")

	for _, f := range []string{caCert, serverCert, serverKey, clientCert, clientKey} {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Fatalf("Certificate not found: %s. Run './build.sh certs' first.", f)
		}
	}

	// Use temp database
	tmpDir, err := os.MkdirTemp("", "cue-mtls-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	secLogPath := filepath.Join(tmpDir, "security.log")
	binPath := filepath.Join(projectRoot, "bin", "cue")

	// Check binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run './build.sh build' first.", binPath)
	}

	// Start server with mTLS
	port := "18443"
	cmd := exec.Command(binPath,
		"-addr", ":"+port,
		"-db", dbPath,
		"-cert", serverCert,
		"-key", serverKey,
		"-ca", caCert,
		"-security-log", secLogPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	baseURL := "https://localhost:" + port

	// Load CA cert for verifying server
	caCertPEM, err := os.ReadFile(caCert)
	if err != nil {
		t.Fatalf("Failed to read CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		t.Fatal("Failed to parse CA cert")
	}

	// Load client cert
	clientTLSCert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		t.Fatalf("Failed to load client cert: %v", err)
	}

	// HTTP client WITHOUT client cert (should fail auth)
	noAuthClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	// HTTP client WITH client cert (should succeed)
	authClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      caCertPool,
				Certificates: []tls.Certificate{clientTLSCert},
			},
		},
	}

	// Wait for server to be ready using authenticated client
	ready := false
	for i := 0; i < 50; i++ {
		resp, err := authClient.Get(baseURL + "/api/health")
		if err != nil {
			t.Logf("Waiting for server (attempt %d): %v", i+1, err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Server did not become ready")
	}

	t.Run("HealthNoAuth", func(t *testing.T) {
		// Health endpoint should work without auth
		resp, err := noAuthClient.Get(baseURL + "/api/health")
		if err != nil {
			t.Fatalf("Health request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 200: %s", resp.StatusCode, body)
		}
	})

	t.Run("WhoAmINoAuth", func(t *testing.T) {
		resp, err := noAuthClient.Get(baseURL + "/api/whoami")
		if err != nil {
			t.Fatalf("WhoAmI request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should get 401 without client cert
		if resp.StatusCode != 401 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 401: %s", resp.StatusCode, body)
		}
	})

	t.Run("WhoAmIWithCert", func(t *testing.T) {
		resp, err := authClient.Get(baseURL + "/api/whoami")
		if err != nil {
			t.Fatalf("WhoAmI request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
		}

		var whoami WhoAmI
		if err := json.NewDecoder(resp.Body).Decode(&whoami); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !whoami.Authenticated {
			t.Error("expected authenticated = true")
		}
		if whoami.User == nil {
			t.Fatal("expected user info")
		}
		if whoami.User.CN != "localuser" {
			t.Errorf("cn = %q, want 'localuser'", whoami.User.CN)
		}
		if whoami.User.AuthMethod != "cert" {
			t.Errorf("auth_method = %q, want 'cert'", whoami.User.AuthMethod)
		}
		if whoami.Mode != "authenticated" {
			t.Errorf("mode = %q, want 'authenticated'", whoami.Mode)
		}
	})

	t.Run("ItemsNoAuth", func(t *testing.T) {
		resp, err := noAuthClient.Get(baseURL + "/api/items")
		if err != nil {
			t.Fatalf("Items request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 401 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 401: %s", resp.StatusCode, body)
		}
	})

	t.Run("ItemsWithCert", func(t *testing.T) {
		resp, err := authClient.Get(baseURL + "/api/items")
		if err != nil {
			t.Fatalf("Items request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 200: %s", resp.StatusCode, body)
		}
	})

	t.Run("CreateItemWithCert", func(t *testing.T) {
		body := `{"title": "mTLS Test Item", "content": "Created with client cert"}`
		resp, err := authClient.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("Create item request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 201: %s", resp.StatusCode, body)
		}
	})

	var apiToken string

	t.Run("CreateToken", func(t *testing.T) {
		body := `{"name": "test-token"}`
		resp, err := authClient.Post(baseURL+"/api/tokens", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("Create token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 201: %s", resp.StatusCode, body)
		}

		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if tokenResp.Token == "" {
			t.Error("expected token in response")
		}
		apiToken = tokenResp.Token
		t.Logf("Created token: %s (truncated)", apiToken[:min(20, len(apiToken))])
	})

	t.Run("TokenAuth", func(t *testing.T) {
		if apiToken == "" {
			t.Skip("no token created")
		}

		// Use client without cert, but with Bearer token
		req, _ := http.NewRequest("GET", baseURL+"/api/whoami", nil)
		req.Header.Set("Authorization", "Bearer "+apiToken)

		resp, err := noAuthClient.Do(req)
		if err != nil {
			t.Fatalf("Token auth request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
		}

		var whoami WhoAmI
		if err := json.NewDecoder(resp.Body).Decode(&whoami); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if !whoami.Authenticated {
			t.Error("expected authenticated = true")
		}
		if whoami.User == nil {
			t.Fatal("expected user info")
		}
		if whoami.User.AuthMethod != "token" {
			t.Errorf("auth_method = %q, want 'token'", whoami.User.AuthMethod)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/api/items", nil)
		req.Header.Set("Authorization", "Bearer invalid-token-12345")

		resp, err := noAuthClient.Do(req)
		if err != nil {
			t.Fatalf("Invalid token request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 401 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 401: %s", resp.StatusCode, body)
		}
	})

	fmt.Println("All mTLS tests passed!")
}
