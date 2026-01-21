package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alanp/cue/internal/auth"
	"github.com/alanp/cue/internal/store"
)

// AuthConfig holds authentication configuration for the API server.
type AuthConfig struct {
	Enabled    bool                     // Whether auth is enabled
	Secret     []byte                   // HMAC secret for tokens
	DefaultTTL time.Duration            // Default token expiration
	MaxTTL     time.Duration            // Maximum token expiration
	Logger     *auth.FileSecurityLogger // Security logger
	TrustProxy bool                     // Whether to trust X-Forwarded-For headers
}

type Server struct {
	store   *store.Store
	mux     *http.ServeMux
	authCfg AuthConfig
	version string
}

func New(s *store.Store) *Server {
	return NewWithAuth(s, AuthConfig{}, "dev")
}

func NewWithAuth(s *store.Store, authCfg AuthConfig, version string) *Server {
	if authCfg.DefaultTTL == 0 {
		authCfg.DefaultTTL = 720 * time.Hour // 30 days
	}
	if authCfg.MaxTTL == 0 {
		authCfg.MaxTTL = 8760 * time.Hour // 1 year
	}
	if version == "" {
		version = "dev"
	}
	srv := &Server{store: s, mux: http.NewServeMux(), authCfg: authCfg, version: version}
	srv.routes()
	return srv
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/status", s.HandleStatus)
	s.mux.HandleFunc("GET /api/health", s.HandleHealth)
	s.mux.HandleFunc("GET /api/items", s.handleListItems)
	s.mux.HandleFunc("POST /api/items", s.handleCreateItem)
	s.mux.HandleFunc("GET /api/items/{id}", s.handleGetItem)
	s.mux.HandleFunc("PUT /api/items/{id}", s.handleUpdateItem)
	s.mux.HandleFunc("DELETE /api/items/{id}", s.handleDeleteItem)
	s.mux.HandleFunc("GET /api/search", s.handleSearch)

	// Auth endpoints
	s.mux.HandleFunc("GET /api/whoami", s.handleWhoAmI)
	s.mux.HandleFunc("POST /api/tokens", s.handleCreateToken)
	s.mux.HandleFunc("GET /api/tokens", s.handleListTokens)
	s.mux.HandleFunc("DELETE /api/tokens/{id}", s.handleDeleteToken)
}

func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": s.version,
		"status":  "ok",
	})
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleListItems(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, err := s.store.List(limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []store.Item{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

type createItemRequest struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Link    *string `json:"link,omitempty"`
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	item, err := s.store.Create(req.Title, req.Content, req.Link)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, "title already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleGetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	item, err := s.store.Get(id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

type updateItemRequest struct {
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Link    *string `json:"link,omitempty"`
}

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}

	item, err := s.store.Update(id, req.Title, req.Content, req.Link)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			http.Error(w, "title already exists", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(item)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := s.store.Delete(id)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "q parameter required", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	results, err := s.store.Search(query, limit)
	if err != nil {
		// FTS5 query syntax errors
		if strings.Contains(err.Error(), "fts5") {
			http.Error(w, "invalid search query", http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if results == nil {
		results = []store.SearchResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Auth handlers

type whoAmIResponse struct {
	Authenticated bool      `json:"authenticated"`
	User          *userInfo `json:"user,omitempty"`
	Mode          string    `json:"mode"`
}

type userInfo struct {
	CN         string `json:"cn"`
	AuthMethod string `json:"auth_method"`
}

func (s *Server) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())

	w.Header().Set("Content-Type", "application/json")

	if user == nil {
		// No user in context - auth middleware not applied or auth disabled
		if !s.authCfg.Enabled {
			json.NewEncoder(w).Encode(whoAmIResponse{
				Authenticated: true,
				User: &userInfo{
					CN:         "single-user-mode",
					AuthMethod: "none",
				},
				Mode: "single-user",
			})
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mode := "authenticated"
	if user.AuthMethod == "none" {
		mode = "single-user"
	}

	json.NewEncoder(w).Encode(whoAmIResponse{
		Authenticated: true,
		User: &userInfo{
			CN:         user.CN,
			AuthMethod: user.AuthMethod,
		},
		Mode: mode,
	})
}

type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn string `json:"expires_in,omitempty"` // e.g., "720h"
}

type createTokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token"` // Only shown once
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// In auth-enabled mode, require certificate auth for token creation
	if s.authCfg.Enabled && user.AuthMethod != "cert" && user.AuthMethod != "none" {
		http.Error(w, "Client certificate required to create tokens", http.StatusUnauthorized)
		return
	}

	var req createTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	// Parse expiration duration
	ttl := s.authCfg.DefaultTTL
	if req.ExpiresIn != "" {
		parsed, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			http.Error(w, "invalid expires_in duration", http.StatusBadRequest)
			return
		}
		if parsed <= 0 {
			http.Error(w, "expires_in must be positive", http.StatusBadRequest)
			return
		}
		ttl = parsed
	}

	// Enforce max TTL
	if ttl > s.authCfg.MaxTTL {
		ttl = s.authCfg.MaxTTL
	}

	// Generate token
	tokenID, err := auth.GenerateTokenID()
	if err != nil {
		http.Error(w, "failed to generate token ID", http.StatusInternalServerError)
		return
	}

	token, expiresAt, err := auth.GenerateToken(user.CN, ttl, s.authCfg.Secret)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	// Store token hash
	tokenHash := auth.HashToken(token)
	if err := s.store.CreateToken(tokenID, user.CN, req.Name, tokenHash, expiresAt); err != nil {
		http.Error(w, "failed to store token", http.StatusInternalServerError)
		return
	}

	// Log token creation
	if s.authCfg.Logger != nil {
		s.authCfg.Logger.LogTokenCreated(user.CN, tokenID, req.Name, expiresAt.Format(time.RFC3339), auth.ExtractSourceIP(r, s.authCfg.TrustProxy))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createTokenResponse{
		ID:        tokenID,
		Name:      req.Name,
		Token:     token,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	tokens, err := s.store.ListTokens(user.CN)
	if err != nil {
		http.Error(w, "failed to list tokens", http.StatusInternalServerError)
		return
	}

	if tokens == nil {
		tokens = []store.TokenInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// In auth-enabled mode, require certificate auth for token deletion
	if s.authCfg.Enabled && user.AuthMethod != "cert" && user.AuthMethod != "none" {
		http.Error(w, "Client certificate required to delete tokens", http.StatusUnauthorized)
		return
	}

	tokenID := r.PathValue("id")

	err := s.store.DeleteToken(tokenID, user.CN)
	if err == sql.ErrNoRows {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to delete token", http.StatusInternalServerError)
		return
	}

	// Log token revocation
	if s.authCfg.Logger != nil {
		s.authCfg.Logger.LogTokenRevoked(user.CN, tokenID, auth.ExtractSourceIP(r, s.authCfg.TrustProxy))
	}

	w.WriteHeader(http.StatusNoContent)
}
