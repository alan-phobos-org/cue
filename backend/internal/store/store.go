package store

import (
	cryptoRand "crypto/rand"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

type Item struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Link      *string   `json:"link,omitempty"` // Optional primary link (URL or file path)
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
		CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL UNIQUE,
			link TEXT,
			content TEXT NOT NULL DEFAULT '',
			created_by TEXT NOT NULL DEFAULT 'single-user-mode',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);

		CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
			title,
			content,
			link,
			content='items',
			content_rowid='rowid'
		);

		CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, title, content, link)
			VALUES (NEW.rowid, NEW.title, NEW.content, NEW.link);
		END;

		CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, content, link)
			VALUES ('delete', OLD.rowid, OLD.title, OLD.content, OLD.link);
		END;

		CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, title, content, link)
			VALUES ('delete', OLD.rowid, OLD.title, OLD.content, OLD.link);
			INSERT INTO items_fts(rowid, title, content, link)
			VALUES (NEW.rowid, NEW.title, NEW.content, NEW.link);
		END;

		-- Auth tables
		CREATE TABLE IF NOT EXISTS tokens (
			id TEXT PRIMARY KEY,
			user_cn TEXT NOT NULL,
			name TEXT NOT NULL,
			token_hash BLOB NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			last_used_at TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_tokens_user ON tokens(user_cn);
		CREATE INDEX IF NOT EXISTS idx_tokens_hash ON tokens(token_hash);

		CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value BLOB NOT NULL
		);
	`
	_, err := db.Exec(schema)
	return err
}

func (s *Store) Create(title, content string, link *string) (*Item, error) {
	id := uuid.New().String()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	_, err := s.db.Exec(
		"INSERT INTO items (id, title, link, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, title, link, content, nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return &Item{
		ID:        id,
		Title:     title,
		Link:      link,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (s *Store) Get(id string) (*Item, error) {
	row := s.db.QueryRow(
		"SELECT id, title, link, content, created_at, updated_at FROM items WHERE id = ?",
		id,
	)
	return scanItem(row)
}

func (s *Store) GetByTitle(title string) (*Item, error) {
	row := s.db.QueryRow(
		"SELECT id, title, link, content, created_at, updated_at FROM items WHERE title = ?",
		title,
	)
	return scanItem(row)
}

func (s *Store) Update(id, title, content string, link *string) (*Item, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	result, err := s.db.Exec(
		"UPDATE items SET title = ?, link = ?, content = ?, updated_at = ? WHERE id = ?",
		title, link, content, nowStr, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}

	return s.Get(id)
}

func (s *Store) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) List(limit, offset int) ([]Item, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		"SELECT id, title, link, content, created_at, updated_at FROM items ORDER BY updated_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	return scanItems(rows)
}

type SearchResult struct {
	Item    Item    `json:"item"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}

	// FTS5 search with BM25 ranking
	rows, err := s.db.Query(`
		SELECT i.id, i.title, i.link, i.content, i.created_at, i.updated_at,
			   bm25(items_fts) as rank,
			   snippet(items_fts, 1, '<mark>', '</mark>', '...', 20) as snippet
		FROM items_fts
		JOIN items i ON items_fts.rowid = i.rowid
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var item Item
		var createdAt, updatedAt string
		var link sql.NullString
		var r SearchResult

		err := rows.Scan(
			&item.ID, &item.Title, &link, &item.Content,
			&createdAt, &updatedAt, &r.Rank, &r.Snippet,
		)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if link.Valid {
			item.Link = &link.String
		}

		r.Item = item
		results = append(results, r)
	}

	return results, rows.Err()
}

func scanItem(row *sql.Row) (*Item, error) {
	var item Item
	var createdAt, updatedAt string
	var link sql.NullString

	err := row.Scan(&item.ID, &item.Title, &link, &item.Content, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if link.Valid {
		item.Link = &link.String
	}

	return &item, nil
}

func scanItems(rows *sql.Rows) ([]Item, error) {
	var items []Item
	for rows.Next() {
		var item Item
		var createdAt, updatedAt string
		var link sql.NullString

		err := rows.Scan(&item.ID, &item.Title, &link, &item.Content, &createdAt, &updatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if link.Valid {
			item.Link = &link.String
		}

		items = append(items, item)
	}
	return items, rows.Err()
}

// Token types and methods

// TokenInfo represents stored token metadata (without the actual token value).
type TokenInfo struct {
	ID         string     `json:"id"`
	UserCN     string     `json:"user_cn"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// GetOrCreateTokenSecret retrieves the HMAC secret for token signing,
// creating one if it doesn't exist.
func (s *Store) GetOrCreateTokenSecret() ([]byte, error) {
	var secret []byte
	err := s.db.QueryRow("SELECT value FROM config WHERE key = 'token_secret'").Scan(&secret)
	if err == sql.ErrNoRows {
		// Generate a new 32-byte secret
		secret = make([]byte, 32)
		if _, err := cryptoRand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate secret: %w", err)
		}
		_, err = s.db.Exec("INSERT INTO config (key, value) VALUES ('token_secret', ?)", secret)
		if err != nil {
			return nil, fmt.Errorf("store secret: %w", err)
		}
		return secret, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query secret: %w", err)
	}
	return secret, nil
}

// CreateToken stores a new token's metadata and hash.
func (s *Store) CreateToken(id, userCN, name string, tokenHash []byte, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	expiresAtStr := expiresAt.Format(time.RFC3339)

	_, err := s.db.Exec(
		"INSERT INTO tokens (id, user_cn, name, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, userCN, name, tokenHash, now, expiresAtStr,
	)
	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}
	return nil
}

// ListTokens returns all tokens for a given user.
func (s *Store) ListTokens(userCN string) ([]TokenInfo, error) {
	rows, err := s.db.Query(
		"SELECT id, user_cn, name, created_at, expires_at, last_used_at FROM tokens WHERE user_cn = ? ORDER BY created_at DESC",
		userCN,
	)
	if err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	var tokens []TokenInfo
	for rows.Next() {
		var t TokenInfo
		var createdAt, expiresAt string
		var lastUsedAt sql.NullString

		if err := rows.Scan(&t.ID, &t.UserCN, &t.Name, &createdAt, &expiresAt, &lastUsedAt); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}

		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
		if lastUsedAt.Valid {
			lu, _ := time.Parse(time.RFC3339, lastUsedAt.String)
			t.LastUsedAt = &lu
		}

		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteToken removes a token by ID (only if owned by the given user).
func (s *Store) DeleteToken(id, userCN string) error {
	result, err := s.db.Exec("DELETE FROM tokens WHERE id = ? AND user_cn = ?", id, userCN)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ValidateTokenHash checks if a token hash exists in the database and is not expired.
// Returns the token ID if found and valid, or sql.ErrNoRows if not found/expired.
func (s *Store) ValidateTokenHash(tokenHash []byte) (string, error) {
	var id string
	now := time.Now().UTC().Format(time.RFC3339)

	// Check both existence and expiration in one query for defense-in-depth
	err := s.db.QueryRow(
		"SELECT id FROM tokens WHERE token_hash = ? AND expires_at > ?",
		tokenHash, now,
	).Scan(&id)
	if err != nil {
		return "", err
	}

	// Update last_used_at
	s.db.Exec("UPDATE tokens SET last_used_at = ? WHERE token_hash = ?", now, tokenHash)

	return id, nil
}

// GetTokenByID retrieves a token by its ID.
func (s *Store) GetTokenByID(id string) (*TokenInfo, error) {
	var t TokenInfo
	var createdAt, expiresAt string
	var lastUsedAt sql.NullString

	err := s.db.QueryRow(
		"SELECT id, user_cn, name, created_at, expires_at, last_used_at FROM tokens WHERE id = ?",
		id,
	).Scan(&t.ID, &t.UserCN, &t.Name, &createdAt, &expiresAt, &lastUsedAt)
	if err != nil {
		return nil, err
	}

	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	if lastUsedAt.Valid {
		lu, _ := time.Parse(time.RFC3339, lastUsedAt.String)
		t.LastUsedAt = &lu
	}

	return &t, nil
}
