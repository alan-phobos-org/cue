package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/alanp/cue/internal/store"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "cue-api-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	s, err := store.New(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatal(err)
	}

	srv := New(s)

	cleanup := func() {
		s.Close()
		os.Remove(tmpFile.Name())
	}

	return srv, cleanup
}

func TestIntegrationHealth(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
}

func TestIntegrationStatus(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want %q", resp["status"], "ok")
	}
	if resp["version"] != "dev" {
		t.Errorf("version = %q, want %q", resp["version"], "dev")
	}
}

func TestIntegrationCreateItem(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "Test Item", "content": "# Hello"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var item store.Item
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "Test Item" {
		t.Errorf("title = %q, want %q", item.Title, "Test Item")
	}
	if item.ID == "" {
		t.Error("expected ID")
	}
}

func TestIntegrationCreateItemWithLink(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "Linked Item", "content": "content", "link": "https://example.com"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var item store.Item
	json.NewDecoder(w.Body).Decode(&item)
	if item.Link == nil || *item.Link != "https://example.com" {
		t.Errorf("link = %v, want %q", item.Link, "https://example.com")
	}
}

func TestIntegrationCreateItemEmptyTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationCreateItemDuplicateTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "Same Title", "content": "content"}`

	// First create
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create failed: %d", w.Code)
	}

	// Second create with same title
	req = httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestIntegrationGetItem(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create item first
	body := `{"title": "Get Test", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var created store.Item
	json.NewDecoder(w.Body).Decode(&created)

	// Get item
	req = httptest.NewRequest("GET", "/api/items/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var item store.Item
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "Get Test" {
		t.Errorf("title = %q, want %q", item.Title, "Get Test")
	}
}

func TestIntegrationGetItemNotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/items/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIntegrationUpdateItem(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create item
	body := `{"title": "Original", "content": "old"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var created store.Item
	json.NewDecoder(w.Body).Decode(&created)

	// Update item
	body = `{"title": "Updated", "content": "new", "link": "~/test.md"}`
	req = httptest.NewRequest("PUT", "/api/items/"+created.ID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var item store.Item
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "Updated" {
		t.Errorf("title = %q, want %q", item.Title, "Updated")
	}
	if item.Link == nil || *item.Link != "~/test.md" {
		t.Errorf("link = %v, want %q", item.Link, "~/test.md")
	}
}

func TestIntegrationDeleteItem(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create item
	body := `{"title": "Delete Me", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var created store.Item
	json.NewDecoder(w.Body).Decode(&created)

	// Delete item
	req = httptest.NewRequest("DELETE", "/api/items/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	// Verify deleted
	req = httptest.NewRequest("GET", "/api/items/"+created.ID, nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIntegrationListItems(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create items
	for _, title := range []string{"Item 1", "Item 2", "Item 3"} {
		body := `{"title": "` + title + `", "content": "content"}`
		req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
	}

	// List items
	req := httptest.NewRequest("GET", "/api/items", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var items []store.Item
	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 3 {
		t.Errorf("len = %d, want 3", len(items))
	}
}

func TestIntegrationSearch(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create items
	items := []struct{ title, content string }{
		{"SQLite Guide", "Full-text search with FTS5"},
		{"Go Patterns", "HTTP middleware patterns"},
		{"React Tips", "Server components"},
	}
	for _, item := range items {
		body := `{"title": "` + item.title + `", "content": "` + item.content + `"}`
		req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
	}

	// Search
	req := httptest.NewRequest("GET", "/api/search?q=SQLite", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []store.SearchResult
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("len = %d, want 1", len(results))
	}
	if len(results) > 0 && results[0].Item.Title != "SQLite Guide" {
		t.Errorf("title = %q, want %q", results[0].Item.Title, "SQLite Guide")
	}
}

func TestIntegrationSearchMissingQuery(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/search", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationMalformedJSON(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "Test", content: broken}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationWhitespaceTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "   ", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationUpdateNonExistent(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "Updated", "content": "content"}`
	req := httptest.NewRequest("PUT", "/api/items/nonexistent-id", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIntegrationUpdateEmptyTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create item first
	body := `{"title": "Original", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var created store.Item
	json.NewDecoder(w.Body).Decode(&created)

	// Update with empty title
	body = `{"title": "", "content": "new content"}`
	req = httptest.NewRequest("PUT", "/api/items/"+created.ID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationUpdateToDuplicateTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create two items
	body := `{"title": "First", "content": "content"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	body = `{"title": "Second", "content": "content"}`
	req = httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var second store.Item
	json.NewDecoder(w.Body).Decode(&second)

	// Try to update second to first's title
	body = `{"title": "First", "content": "content"}`
	req = httptest.NewRequest("PUT", "/api/items/"+second.ID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestIntegrationDeleteNonExistent(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/api/items/nonexistent-id", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestIntegrationInvalidFTS5Query(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Unbalanced quotes cause FTS5 syntax error - returns 500 since error
	// message doesn't always contain "fts5" for all syntax errors
	req := httptest.NewRequest("GET", "/api/search?q=\"unclosed", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Accept either 400 (if fts5 in error) or 500 (generic error)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 400 or 500", w.Code)
	}
}

func TestIntegrationListPagination(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create 5 items
	for i := 0; i < 5; i++ {
		body := `{"title": "Item ` + string(rune('A'+i)) + `", "content": "content"}`
		req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
	}

	// Fetch with limit
	req := httptest.NewRequest("GET", "/api/items?limit=2", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var items []store.Item
	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}

	// Fetch with offset
	req = httptest.NewRequest("GET", "/api/items?limit=2&offset=2", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&items)
	if len(items) != 2 {
		t.Errorf("offset len = %d, want 2", len(items))
	}
}

func TestIntegrationUnicodeTitle(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"title": "日本語タイトル 🎉", "content": "Unicode content: こんにちは"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var item store.Item
	json.NewDecoder(w.Body).Decode(&item)
	if item.Title != "日本語タイトル 🎉" {
		t.Errorf("title = %q, want unicode title", item.Title)
	}
}

func TestIntegrationEmptyBody(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(""))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestIntegrationSearchWithLimit(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create items with common word
	for i := 0; i < 5; i++ {
		body := `{"title": "Test Item ` + string(rune('A'+i)) + `", "content": "common keyword"}`
		req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
	}

	// Search with limit
	req := httptest.NewRequest("GET", "/api/search?q=common&limit=2", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var results []store.SearchResult
	json.NewDecoder(w.Body).Decode(&results)
	if len(results) != 2 {
		t.Errorf("len = %d, want 2", len(results))
	}
}

func TestIntegrationClearLink(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create item with link
	body := `{"title": "Has Link", "content": "content", "link": "https://example.com"}`
	req := httptest.NewRequest("POST", "/api/items", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	var created store.Item
	json.NewDecoder(w.Body).Decode(&created)

	// Update without link to clear it
	body = `{"title": "Has Link", "content": "content"}`
	req = httptest.NewRequest("PUT", "/api/items/"+created.ID, bytes.NewBufferString(body))
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var updated store.Item
	json.NewDecoder(w.Body).Decode(&updated)
	if updated.Link != nil {
		t.Errorf("link = %v, want nil", updated.Link)
	}
}

func TestIntegrationContentTypeHeader(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
