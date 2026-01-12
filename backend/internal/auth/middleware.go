package auth

import (
	"net/http"
	"strings"
)

// TokenValidator is called to validate a token and check if it's revoked.
// Returns the token ID if the token is valid, or an error if revoked/invalid.
type TokenValidator func(token string) (tokenID string, err error)

// SecurityLogger logs authentication events.
type SecurityLogger interface {
	LogAuthSuccess(user *UserContext, sourceIP string)
	LogAuthFailure(reason, details, sourceIP string)
}

// MiddlewareConfig configures the authentication middleware.
type MiddlewareConfig struct {
	Secret         []byte         // HMAC secret for token validation
	TokenValidator TokenValidator // Optional: validates token against DB (revocation check)
	Logger         SecurityLogger // Optional: logs auth events
	AuthEnabled    bool           // If false, all requests get single-user context
	TrustProxy     bool           // If true, trust X-Forwarded-For/X-Real-IP headers
}

// Middleware creates HTTP middleware that authenticates requests.
// It first checks for a valid client certificate, then falls back to Bearer token.
func Middleware(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sourceIP := ExtractSourceIP(r, cfg.TrustProxy)

			// If auth is disabled, use single-user mode
			if !cfg.AuthEnabled {
				ctx := WithUser(r.Context(), SingleUserContext())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Check client certificate first (highest trust)
			if user := ExtractUserFromCert(r); user != nil {
				if cfg.Logger != nil {
					cfg.Logger.LogAuthSuccess(user, sourceIP)
				}
				ctx := WithUser(r.Context(), user)
				w.Header().Set("X-Auth-User", user.CN)
				w.Header().Set("X-Auth-Method", "cert")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall back to Bearer token
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				tokenStr := strings.TrimPrefix(auth, "Bearer ")

				claims, err := ValidateToken(tokenStr, cfg.Secret)
				if err != nil {
					if cfg.Logger != nil {
						cfg.Logger.LogAuthFailure("invalid_token", err.Error(), sourceIP)
					}
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// Check revocation if validator is provided
				var tokenID string
				if cfg.TokenValidator != nil {
					tokenID, err = cfg.TokenValidator(tokenStr)
					if err != nil {
						if cfg.Logger != nil {
							cfg.Logger.LogAuthFailure("token_revoked", err.Error(), sourceIP)
						}
						http.Error(w, "Unauthorized", http.StatusUnauthorized)
						return
					}
				}

				user := &UserContext{
					CN:         claims.CN,
					AuthMethod: "token",
					TokenID:    tokenID,
				}

				if cfg.Logger != nil {
					cfg.Logger.LogAuthSuccess(user, sourceIP)
				}

				ctx := WithUser(r.Context(), user)
				w.Header().Set("X-Auth-User", user.CN)
				w.Header().Set("X-Auth-Method", "token")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// No valid authentication
			if cfg.Logger != nil {
				cfg.Logger.LogAuthFailure("no_credentials", "", sourceIP)
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// RequireCertAuth creates middleware that requires client certificate authentication.
// Token authentication is not accepted. Use for sensitive operations like token creation.
func RequireCertAuth(cfg MiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sourceIP := ExtractSourceIP(r, cfg.TrustProxy)

			// If auth is disabled, use single-user mode
			if !cfg.AuthEnabled {
				ctx := WithUser(r.Context(), SingleUserContext())
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Only accept client certificate
			if user := ExtractUserFromCert(r); user != nil {
				if cfg.Logger != nil {
					cfg.Logger.LogAuthSuccess(user, sourceIP)
				}
				ctx := WithUser(r.Context(), user)
				w.Header().Set("X-Auth-User", user.CN)
				w.Header().Set("X-Auth-Method", "cert")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if cfg.Logger != nil {
				cfg.Logger.LogAuthFailure("cert_required", "token auth not accepted", sourceIP)
			}
			http.Error(w, "Client certificate required", http.StatusUnauthorized)
		})
	}
}

// ExtractSourceIP gets the client IP from the request.
// If trustProxy is true, X-Forwarded-For and X-Real-IP headers are trusted.
// If trustProxy is false, only r.RemoteAddr is used (prevents IP spoofing).
func ExtractSourceIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Check X-Forwarded-For header first (for reverse proxies)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first IP in the chain (client's original IP)
			if idx := strings.Index(xff, ","); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		// Check X-Real-IP
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}
	// Fall back to RemoteAddr (always safe, can't be spoofed)
	return r.RemoteAddr
}
