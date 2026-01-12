package auth

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSecurityLogger_AuthSuccess(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecurityLogger(&buf)

	user := &UserContext{
		CN:         "testuser",
		AuthMethod: "cert",
	}

	logger.LogAuthSuccess(user, "192.168.1.1")

	output := buf.String()
	if output == "" {
		t.Fatal("expected log output")
	}

	var event SecurityEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if event.Event != "auth_success" {
		t.Errorf("expected event 'auth_success', got %q", event.Event)
	}

	if event.UserCN != "testuser" {
		t.Errorf("expected user 'testuser', got %q", event.UserCN)
	}

	if event.AuthMethod != "cert" {
		t.Errorf("expected method 'cert', got %q", event.AuthMethod)
	}

	if event.SourceIP != "192.168.1.1" {
		t.Errorf("expected IP '192.168.1.1', got %q", event.SourceIP)
	}

	if event.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestSecurityLogger_AuthFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecurityLogger(&buf)

	logger.LogAuthFailure("invalid_token", "token expired", "192.168.1.2")

	output := buf.String()
	var event SecurityEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if event.Event != "auth_failure" {
		t.Errorf("expected event 'auth_failure', got %q", event.Event)
	}

	if event.Reason != "invalid_token" {
		t.Errorf("expected reason 'invalid_token', got %q", event.Reason)
	}
}

func TestSecurityLogger_TokenCreated(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecurityLogger(&buf)

	logger.LogTokenCreated("testuser", "tok_123", "my-automation", "2025-02-01T00:00:00Z", "192.168.1.3")

	output := buf.String()
	var event SecurityEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if event.Event != "token_created" {
		t.Errorf("expected event 'token_created', got %q", event.Event)
	}

	if event.TokenID != "tok_123" {
		t.Errorf("expected token_id 'tok_123', got %q", event.TokenID)
	}
}

func TestSecurityLogger_TokenRevoked(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecurityLogger(&buf)

	logger.LogTokenRevoked("testuser", "tok_123", "192.168.1.4")

	output := buf.String()
	var event SecurityEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if event.Event != "token_revoked" {
		t.Errorf("expected event 'token_revoked', got %q", event.Event)
	}
}

func TestSecurityLogger_ServerStart(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecurityLogger(&buf)

	logger.LogServerStart("authenticated", "/path/to/ca.crt")

	output := buf.String()
	var event SecurityEvent
	if err := json.Unmarshal([]byte(output), &event); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}

	if event.Event != "server_start" {
		t.Errorf("expected event 'server_start', got %q", event.Event)
	}

	if !strings.Contains(event.Details, "authenticated") {
		t.Errorf("expected details to contain 'authenticated', got %q", event.Details)
	}
}

func TestSanitize_TruncatesLongStrings(t *testing.T) {
	longString := strings.Repeat("a", 300)
	sanitized := sanitize(longString)

	if len(sanitized) > 204 { // 200 + "..."
		t.Errorf("expected truncated string, got length %d", len(sanitized))
	}

	if !strings.HasSuffix(sanitized, "...") {
		t.Error("expected string to end with '...'")
	}
}

func TestSanitize_RedactsSecrets(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"jwt-like", "token: eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		{"hex-string", "secret: 0123456789abcdef0123456789abcdef"},
		{"base64", "key: YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo="},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sanitized := sanitize(tc.input)
			if !strings.Contains(sanitized, "[REDACTED]") {
				t.Errorf("expected secret to be redacted in %q, got %q", tc.input, sanitized)
			}
		})
	}
}

func TestSanitize_PreservesNormalStrings(t *testing.T) {
	input := "user logged in from 192.168.1.1"
	sanitized := sanitize(input)

	if sanitized != input {
		t.Errorf("expected normal string to be preserved, got %q", sanitized)
	}
}
