package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken     = errors.New("invalid token format")
	ErrInvalidSignature = errors.New("invalid token signature")
	ErrTokenExpired     = errors.New("token has expired")
	ErrTokenRevoked     = errors.New("token has been revoked")
)

// TokenClaims represents the payload of an API token.
type TokenClaims struct {
	CN  string `json:"cn"`  // User's Common Name
	IAT int64  `json:"iat"` // Issued At (Unix timestamp)
	EXP int64  `json:"exp"` // Expiration (Unix timestamp)
}

// GenerateToken creates a new signed API token for the given user.
// The token format is: base64(payload).base64(hmac-sha256(payload))
func GenerateToken(cn string, expiresIn time.Duration, secret []byte) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(expiresIn)

	claims := TokenClaims{
		CN:  cn,
		IAT: now.Unix(),
		EXP: expiresAt.Unix(),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("marshal claims: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := computeHMAC(payload, secret)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)

	token := payloadB64 + "." + sigB64
	return token, expiresAt, nil
}

// ValidateToken verifies the token signature and checks expiration.
// Returns the claims if valid, or an error otherwise.
func ValidateToken(tokenString string, secret []byte) (*TokenClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expectedSig := computeHMAC(payload, secret)
	if !hmac.Equal(sig, expectedSig) {
		return nil, ErrInvalidSignature
	}

	var claims TokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.EXP {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

// computeHMAC returns the HMAC-SHA256 of the data using the given secret.
func computeHMAC(data, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return h.Sum(nil)
}

// HashToken returns the SHA-256 hash of a token for storage.
// Only the hash is stored; the original token cannot be recovered.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

// GenerateSecret creates a cryptographically secure random secret.
func GenerateSecret(size int) ([]byte, error) {
	secret := make([]byte, size)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	return secret, nil
}

// GenerateTokenID creates a unique token identifier (e.g., "tok_abc123").
func GenerateTokenID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}
	return "tok_" + base64.RawURLEncoding.EncodeToString(b), nil
}
