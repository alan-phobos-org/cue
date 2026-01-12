// Package auth provides client certificate and API token authentication.
package auth

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"
)

// UserContext holds authenticated user information extracted from
// a client certificate or API token.
type UserContext struct {
	CN         string    // Common Name - primary identifier
	DN         string    // Full Distinguished Name (for LDAP lookup)
	Serial     string    // Certificate serial number (empty for token auth)
	NotAfter   time.Time // Certificate expiration (zero for token auth)
	AuthMethod string    // "cert", "token", or "none"
	TokenID    string    // Token ID if authenticated via token
}

type contextKey string

const userContextKey contextKey = "user"

// GetUser retrieves the authenticated user from the request context.
// Returns nil if no user is authenticated.
func GetUser(ctx context.Context) *UserContext {
	if user, ok := ctx.Value(userContextKey).(*UserContext); ok {
		return user
	}
	return nil
}

// WithUser returns a new context with the user attached.
func WithUser(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// ExtractUserFromCert extracts user identity from a verified client certificate.
// Returns nil if no valid certificate is present.
func ExtractUserFromCert(r *http.Request) *UserContext {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return nil
	}
	cert := r.TLS.PeerCertificates[0]
	return &UserContext{
		CN:         cert.Subject.CommonName,
		DN:         cert.Subject.String(),
		Serial:     cert.SerialNumber.String(),
		NotAfter:   cert.NotAfter,
		AuthMethod: "cert",
	}
}

// ExtractUserFromTLSState extracts user identity from a TLS connection state.
// Useful for testing without an http.Request.
func ExtractUserFromTLSState(state *tls.ConnectionState) *UserContext {
	if state == nil || len(state.PeerCertificates) == 0 {
		return nil
	}
	cert := state.PeerCertificates[0]
	return &UserContext{
		CN:         cert.Subject.CommonName,
		DN:         cert.Subject.String(),
		Serial:     cert.SerialNumber.String(),
		NotAfter:   cert.NotAfter,
		AuthMethod: "cert",
	}
}

// SingleUserContext returns a user context for single-user mode (no auth).
func SingleUserContext() *UserContext {
	return &UserContext{
		CN:         "single-user-mode",
		AuthMethod: "none",
	}
}
