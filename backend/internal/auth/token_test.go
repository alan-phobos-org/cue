package auth

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, expiresAt, err := GenerateToken("testuser", 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	if expiresAt.Before(time.Now()) {
		t.Error("expected expiration in the future")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	token, _, err := GenerateToken("testuser", 1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ValidateToken(token, secret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.CN != "testuser" {
		t.Errorf("expected CN 'testuser', got %q", claims.CN)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	// Create token with negative duration (already expired)
	token, _, err := GenerateToken("testuser", -1*time.Hour, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	secret1 := []byte("test-secret-32-bytes-long-key!!")
	secret2 := []byte("different-secret-also-32-bytes!")

	token, _, err := GenerateToken("testuser", 1*time.Hour, secret1)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	_, err = ValidateToken(token, secret2)
	if err != ErrInvalidSignature {
		t.Errorf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestValidateToken_InvalidFormat(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-key!!")

	tests := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"no dot", "invalidtoken"},
		{"too many dots", "a.b.c"},
		{"invalid base64 payload", "!!!.valid"},
		{"invalid base64 sig", "dmFsaWQ.!!!"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateToken(tc.token, secret)
			if err == nil {
				t.Error("expected error for invalid token format")
			}
		})
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token-value"

	hash1 := HashToken(token)
	hash2 := HashToken(token)

	if len(hash1) != 32 {
		t.Errorf("expected 32-byte hash, got %d bytes", len(hash1))
	}

	// Same input should produce same hash
	for i := range hash1 {
		if hash1[i] != hash2[i] {
			t.Error("expected consistent hash output")
			break
		}
	}

	// Different input should produce different hash
	hash3 := HashToken("different-token")
	same := true
	for i := range hash1 {
		if hash1[i] != hash3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different hash for different input")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := GenerateSecret(32)
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	if len(secret1) != 32 {
		t.Errorf("expected 32-byte secret, got %d bytes", len(secret1))
	}

	secret2, err := GenerateSecret(32)
	if err != nil {
		t.Fatalf("GenerateSecret failed: %v", err)
	}

	// Two generated secrets should be different
	same := true
	for i := range secret1 {
		if secret1[i] != secret2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different random secrets")
	}
}

func TestGenerateTokenID(t *testing.T) {
	id1, err := GenerateTokenID()
	if err != nil {
		t.Fatalf("GenerateTokenID failed: %v", err)
	}

	if len(id1) < 4 || id1[:4] != "tok_" {
		t.Errorf("expected token ID starting with 'tok_', got %q", id1)
	}

	id2, err := GenerateTokenID()
	if err != nil {
		t.Fatalf("GenerateTokenID failed: %v", err)
	}

	if id1 == id2 {
		t.Error("expected different token IDs")
	}
}
