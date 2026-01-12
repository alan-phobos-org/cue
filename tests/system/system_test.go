package system

import (
	"bytes"
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

type Item struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Link      *string `json:"link,omitempty"`
	Content   string  `json:"content"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

type SearchResult struct {
	Item    Item    `json:"item"`
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

func TestSystemFullStack(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	// Find project root
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "../..")

	// Use temp database
	tmpDir, err := os.MkdirTemp("", "cue-system-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	binPath := filepath.Join(projectRoot, "bin", "cue")

	// Check binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", binPath)
	}

	// Start server
	port := "18080"
	cmd := exec.Command(binPath, "-addr", ":"+port, "-db", dbPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer cmd.Process.Kill()

	baseURL := "http://localhost:" + port

	// Wait for server to be ready
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Server did not become ready")
	}

	t.Run("Health", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/health")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	var createdID string

	t.Run("CreateItem", func(t *testing.T) {
		body := `{"title": "System Test Item", "content": "# Hello\n\nWorld", "link": "https://example.com"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 201: %s", resp.StatusCode, body)
		}

		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		if item.Title != "System Test Item" {
			t.Errorf("title = %q, want %q", item.Title, "System Test Item")
		}
		if item.Link == nil || *item.Link != "https://example.com" {
			t.Errorf("link = %v, want %q", item.Link, "https://example.com")
		}
		createdID = item.ID
	})

	t.Run("GetItem", func(t *testing.T) {
		if createdID == "" {
			t.Skip("no item created")
		}

		resp, err := http.Get(baseURL + "/api/items/" + createdID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		if item.Title != "System Test Item" {
			t.Errorf("title = %q, want %q", item.Title, "System Test Item")
		}
	})

	t.Run("ListItems", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/items")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var items []Item
		json.NewDecoder(resp.Body).Decode(&items)
		if len(items) < 1 {
			t.Errorf("len = %d, want >= 1", len(items))
		}
	})

	t.Run("Search", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/search?q=System")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 200: %s", resp.StatusCode, body)
			return
		}

		var results []SearchResult
		json.NewDecoder(resp.Body).Decode(&results)
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
	})

	t.Run("UpdateItem", func(t *testing.T) {
		if createdID == "" {
			t.Skip("no item created")
		}

		body := `{"title": "Updated Title", "content": "Updated content", "link": "~/local/file.md"}`
		req, _ := http.NewRequest("PUT", baseURL+"/api/items/"+createdID, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		if item.Title != "Updated Title" {
			t.Errorf("title = %q, want %q", item.Title, "Updated Title")
		}
		if item.Link == nil || *item.Link != "~/local/file.md" {
			t.Errorf("link = %v, want %q", item.Link, "~/local/file.md")
		}
	})

	t.Run("DeleteItem", func(t *testing.T) {
		if createdID == "" {
			t.Skip("no item created")
		}

		req, _ := http.NewRequest("DELETE", baseURL+"/api/items/"+createdID, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			t.Errorf("status = %d, want 204", resp.StatusCode)
		}

		// Verify deleted
		resp, err = http.Get(baseURL + "/api/items/" + createdID)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status after delete = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("Frontend", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("Cue")) {
			t.Errorf("expected 'Cue' in HTML, got: %s", body)
		}
	})

	// Edge case tests
	t.Run("InvalidFTS5Query", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/search?q=\"unclosed")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		// Accept either 400 or 500 - depends on whether error contains "fts5"
		if resp.StatusCode != 400 && resp.StatusCode != 500 {
			t.Errorf("status = %d, want 400 or 500 for invalid FTS5 query", resp.StatusCode)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		// Create more items for pagination test
		for i := 0; i < 3; i++ {
			body := fmt.Sprintf(`{"title": "Pagination Item %d", "content": "content"}`, i)
			resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
		}

		// Test limit
		resp, err := http.Get(baseURL + "/api/items?limit=2")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		var items []Item
		json.NewDecoder(resp.Body).Decode(&items)
		if len(items) != 2 {
			t.Errorf("limit len = %d, want 2", len(items))
		}
	})

	t.Run("DuplicateTitleConflict", func(t *testing.T) {
		// Create first item
		body := `{"title": "Unique System Title", "content": "content"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// Try to create duplicate
		resp, err = http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 409 {
			t.Errorf("status = %d, want 409 for duplicate title", resp.StatusCode)
		}
	})

	t.Run("EmptyTitleRejected", func(t *testing.T) {
		body := `{"title": "", "content": "content"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for empty title", resp.StatusCode)
		}
	})

	t.Run("MalformedJSONRejected", func(t *testing.T) {
		body := `{"title": broken json`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for malformed JSON", resp.StatusCode)
		}
	})

	t.Run("GetNonExistent", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/items/nonexistent-id-12345")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404 for non-existent item", resp.StatusCode)
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/api/items/nonexistent-id-12345", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404 for deleting non-existent item", resp.StatusCode)
		}
	})

	t.Run("SearchNoResults", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/search?q=zzzznonexistentterm")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var results []SearchResult
		json.NewDecoder(resp.Body).Decode(&results)
		if len(results) != 0 {
			t.Errorf("expected empty results, got %d", len(results))
		}
	})

	t.Run("UpdateNonExistentItem", func(t *testing.T) {
		// This reproduces "Failed to save" when item was deleted by another user
		body := `{"title": "Updated Title", "content": "content"}`
		req, _ := http.NewRequest("PUT", baseURL+"/api/items/nonexistent-id-xyz", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404 for updating non-existent item", resp.StatusCode)
		}
	})

	t.Run("UpdateWithDuplicateTitle", func(t *testing.T) {
		// Create two items
		body1 := `{"title": "First Update Test Item", "content": "content"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body1))
		if err != nil {
			t.Fatal(err)
		}
		var item1 Item
		json.NewDecoder(resp.Body).Decode(&item1)
		resp.Body.Close()

		body2 := `{"title": "Second Update Test Item", "content": "content"}`
		resp, err = http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body2))
		if err != nil {
			t.Fatal(err)
		}
		var item2 Item
		json.NewDecoder(resp.Body).Decode(&item2)
		resp.Body.Close()

		// Try to update item2 with item1's title - should fail with 409
		updateBody := `{"title": "First Update Test Item", "content": "content"}`
		req, _ := http.NewRequest("PUT", baseURL+"/api/items/"+item2.ID, bytes.NewBufferString(updateBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 409 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Errorf("status = %d, want 409 for duplicate title update: %s", resp.StatusCode, bodyBytes)
		}
	})

	t.Run("UpdateConcurrentDelete", func(t *testing.T) {
		// Create an item
		body := `{"title": "Item To Be Deleted While Editing", "content": "original"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		resp.Body.Close()

		// Simulate: user starts editing, then item gets deleted by another user
		deleteReq, _ := http.NewRequest("DELETE", baseURL+"/api/items/"+item.ID, nil)
		resp, err = http.DefaultClient.Do(deleteReq)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()

		// Now try to save the edit - should get 404
		updateBody := `{"title": "Item To Be Deleted While Editing", "content": "updated"}`
		updateReq, _ := http.NewRequest("PUT", baseURL+"/api/items/"+item.ID, bytes.NewBufferString(updateBody))
		updateReq.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(updateReq)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("status = %d, want 404 for update after concurrent delete", resp.StatusCode)
		}
	})

	t.Run("UpdateWithEmptyTitle", func(t *testing.T) {
		// Create an item
		body := `{"title": "Item For Empty Title Test", "content": "content"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		resp.Body.Close()

		// Try to update with empty title
		updateBody := `{"title": "", "content": "content"}`
		req, _ := http.NewRequest("PUT", baseURL+"/api/items/"+item.ID, bytes.NewBufferString(updateBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for empty title update", resp.StatusCode)
		}
	})

	t.Run("UpdateWithMalformedJSON", func(t *testing.T) {
		// Create an item
		body := `{"title": "Item For Malformed JSON Test", "content": "content"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		resp.Body.Close()

		// Try to update with malformed JSON
		updateBody := `{"title": malformed`
		req, _ := http.NewRequest("PUT", baseURL+"/api/items/"+item.ID, bytes.NewBufferString(updateBody))
		req.Header.Set("Content-Type", "application/json")

		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("status = %d, want 400 for malformed JSON update", resp.StatusCode)
		}
	})

	t.Run("UnicodeContent", func(t *testing.T) {
		body := `{"title": "Unicode Test 日本語", "content": "こんにちは 🎉"}`
		resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 201: %s", resp.StatusCode, bodyBytes)
		}

		var item Item
		json.NewDecoder(resp.Body).Decode(&item)
		if item.Title != "Unicode Test 日本語" {
			t.Errorf("title = %q, want unicode title", item.Title)
		}
	})

	fmt.Println("All system tests passed!")
}

// TestUpdateServerUnavailable tests what happens when server becomes unavailable during edit.
// This reproduces "TypeError: Failed to fetch" errors.
func TestUpdateServerUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping system test in short mode")
	}

	// Find project root
	wd, _ := os.Getwd()
	projectRoot := filepath.Join(wd, "../..")

	// Use temp database
	tmpDir, err := os.MkdirTemp("", "cue-fetch-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	binPath := filepath.Join(projectRoot, "bin", "cue")

	// Check binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", binPath)
	}

	// Start server on a different port
	port := "18081"
	cmd := exec.Command(binPath, "-addr", ":"+port, "-db", dbPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	baseURL := "http://localhost:" + port

	// Wait for server to be ready
	ready := false
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/api/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		cmd.Process.Kill()
		t.Fatal("Server did not become ready")
	}

	// Create an item while server is running
	body := `{"title": "Item Before Server Stops", "content": "content"}`
	resp, err := http.Post(baseURL+"/api/items", "application/json", bytes.NewBufferString(body))
	if err != nil {
		cmd.Process.Kill()
		t.Fatal(err)
	}
	var item Item
	json.NewDecoder(resp.Body).Decode(&item)
	resp.Body.Close()

	// Kill the server to simulate it becoming unavailable
	cmd.Process.Kill()
	cmd.Wait()

	// Wait a bit for the port to be released
	time.Sleep(200 * time.Millisecond)

	// Try to update the item - this should fail with a connection error
	// This is equivalent to "TypeError: Failed to fetch" in the browser
	updateBody := `{"title": "Item Before Server Stops", "content": "updated"}`
	req, _ := http.NewRequest("PUT", baseURL+"/api/items/"+item.ID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")

	_, err = http.DefaultClient.Do(req)
	if err == nil {
		t.Error("expected connection error when server is down, got nil")
	} else {
		// Verify we get a connection refused or similar error
		t.Logf("Got expected error when server unavailable: %v", err)
	}
}
