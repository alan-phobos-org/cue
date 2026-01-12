package store

import (
	"fmt"
	"os"
	"testing"
)

func TestIntegrationStore(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "cue-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, err := New(tmpFile.Name())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer s.Close()

	// Test Create
	t.Run("Create", func(t *testing.T) {
		item, err := s.Create("Test Item", "# Hello\n\nThis is content", nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if item.ID == "" {
			t.Error("expected ID")
		}
		if item.Title != "Test Item" {
			t.Errorf("title = %q, want %q", item.Title, "Test Item")
		}
	})

	// Test Create with link
	t.Run("CreateWithLink", func(t *testing.T) {
		link := "https://example.com"
		item, err := s.Create("Linked Item", "content", &link)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if item.Link == nil || *item.Link != link {
			t.Errorf("link = %v, want %q", item.Link, link)
		}
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		created, _ := s.Create("Get Test", "content", nil)
		item, err := s.Get(created.ID)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if item.Title != "Get Test" {
			t.Errorf("title = %q, want %q", item.Title, "Get Test")
		}
	})

	// Test GetByTitle
	t.Run("GetByTitle", func(t *testing.T) {
		s.Create("Unique Title", "content", nil)
		item, err := s.GetByTitle("Unique Title")
		if err != nil {
			t.Fatalf("GetByTitle: %v", err)
		}
		if item.Title != "Unique Title" {
			t.Errorf("title = %q, want %q", item.Title, "Unique Title")
		}
	})

	// Test Update
	t.Run("Update", func(t *testing.T) {
		created, _ := s.Create("Update Test", "old content", nil)
		link := "~/docs/test.md"
		updated, err := s.Update(created.ID, "Updated Title", "new content", &link)
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Title != "Updated Title" {
			t.Errorf("title = %q, want %q", updated.Title, "Updated Title")
		}
		if updated.Content != "new content" {
			t.Errorf("content = %q, want %q", updated.Content, "new content")
		}
		if updated.Link == nil || *updated.Link != link {
			t.Errorf("link = %v, want %q", updated.Link, link)
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		created, _ := s.Create("Delete Test", "content", nil)
		err := s.Delete(created.ID)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}
		_, err = s.Get(created.ID)
		if err == nil {
			t.Error("expected error after delete")
		}
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		// Create fresh store
		tmpFile2, _ := os.CreateTemp("", "cue-list-*.db")
		tmpFile2.Close()
		defer os.Remove(tmpFile2.Name())

		s2, _ := New(tmpFile2.Name())
		defer s2.Close()

		s2.Create("Item 1", "content 1", nil)
		s2.Create("Item 2", "content 2", nil)
		s2.Create("Item 3", "content 3", nil)

		items, err := s2.List(10, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(items) != 3 {
			t.Errorf("len = %d, want 3", len(items))
		}
	})

	// Test Search
	t.Run("Search", func(t *testing.T) {
		// Create fresh store
		tmpFile3, _ := os.CreateTemp("", "cue-search-*.db")
		tmpFile3.Close()
		defer os.Remove(tmpFile3.Name())

		s3, _ := New(tmpFile3.Name())
		defer s3.Close()

		s3.Create("SQLite Guide", "Full-text search with FTS5", nil)
		s3.Create("Go Patterns", "HTTP middleware patterns", nil)
		s3.Create("React Tips", "Server components and hooks", nil)

		results, err := s3.Search("SQLite", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
		if len(results) > 0 && results[0].Item.Title != "SQLite Guide" {
			t.Errorf("title = %q, want %q", results[0].Item.Title, "SQLite Guide")
		}
	})

	// Test Search in content
	t.Run("SearchContent", func(t *testing.T) {
		tmpFile4, _ := os.CreateTemp("", "cue-search2-*.db")
		tmpFile4.Close()
		defer os.Remove(tmpFile4.Name())

		s4, _ := New(tmpFile4.Name())
		defer s4.Close()

		s4.Create("Guide", "Full-text search with FTS5 extension", nil)

		results, err := s4.Search("FTS5", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("len = %d, want 1", len(results))
		}
	})
}

func TestDuplicateTitle(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-dup-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("Same Title", "content 1", nil)
	_, err := s.Create("Same Title", "content 2", nil)
	if err == nil {
		t.Error("expected error for duplicate title")
	}
}

func TestGetNonExistent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-get-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	_, err := s.Get("nonexistent-id")
	if err == nil {
		t.Error("expected error for non-existent ID")
	}
}

func TestGetByTitleNonExistent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-getbytitle-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	_, err := s.GetByTitle("No Such Title")
	if err == nil {
		t.Error("expected error for non-existent title")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-del-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	err := s.Delete("nonexistent-id")
	if err == nil {
		t.Error("expected error for deleting non-existent ID")
	}
}

func TestUpdateNonExistent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-upd-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	_, err := s.Update("nonexistent-id", "Title", "Content", nil)
	if err == nil {
		t.Error("expected error for updating non-existent ID")
	}
}

func TestEmptyContent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-empty-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	item, err := s.Create("Empty Content Item", "", nil)
	if err != nil {
		t.Fatalf("Create with empty content: %v", err)
	}
	if item.Content != "" {
		t.Errorf("content = %q, want empty", item.Content)
	}
}

func TestUnicodeContent(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-unicode-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	title := "日本語タイトル 🎉"
	content := "こんにちは世界!\n\nEmoji: 🚀 🌍 ❤️\n\nMath: ∑∫∂√"
	link := "https://例え.jp/パス"

	item, err := s.Create(title, content, &link)
	if err != nil {
		t.Fatalf("Create with unicode: %v", err)
	}

	fetched, err := s.Get(item.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Title != title {
		t.Errorf("title = %q, want %q", fetched.Title, title)
	}
	if fetched.Content != content {
		t.Errorf("content = %q, want %q", fetched.Content, content)
	}
	if fetched.Link == nil || *fetched.Link != link {
		t.Errorf("link = %v, want %q", fetched.Link, link)
	}
}

func TestSearchNoResults(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-searchnone-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("Alpha", "content about alpha", nil)

	results, err := s.Search("zzzznonexistent", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len = %d, want 0", len(results))
	}
}

func TestSearchInLink(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-searchlink-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	link := "https://github.com/unique-repo"
	s.Create("My Item", "basic content", &link)

	results, err := s.Search("github", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len = %d, want 1", len(results))
	}
}

func TestSearchWithQuotes(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-searchquote-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("Exact Match Test", "the quick brown fox jumps", nil)
	s.Create("Partial Match", "quick ideas and brown thoughts", nil)

	// FTS5 phrase search with quotes
	results, err := s.Search(`"quick brown"`, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("phrase search len = %d, want 1", len(results))
	}
}

func TestListPagination(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-page-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	for i := 0; i < 10; i++ {
		s.Create(fmt.Sprintf("Item %d", i), "content", nil)
	}

	// First page
	items, _ := s.List(3, 0)
	if len(items) != 3 {
		t.Errorf("page 1 len = %d, want 3", len(items))
	}

	// Second page
	items, _ = s.List(3, 3)
	if len(items) != 3 {
		t.Errorf("page 2 len = %d, want 3", len(items))
	}

	// Beyond data
	items, _ = s.List(10, 100)
	if len(items) != 0 {
		t.Errorf("beyond data len = %d, want 0", len(items))
	}
}

func TestListDefaultLimit(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-deflimit-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("Single Item", "content", nil)

	// Zero limit should use default (50)
	items, err := s.List(0, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("len = %d, want 1", len(items))
	}

	// Negative limit should use default
	items, err = s.List(-5, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("len = %d, want 1", len(items))
	}
}

func TestClearLinkOnUpdate(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-clearlink-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	link := "https://example.com"
	item, _ := s.Create("Has Link", "content", &link)

	// Update with nil link to clear it
	updated, err := s.Update(item.ID, "Has Link", "content", nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Link != nil {
		t.Errorf("link = %v, want nil", updated.Link)
	}
}

func TestUpdateToDuplicateTitle(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-upddup-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("First Title", "content 1", nil)
	item2, _ := s.Create("Second Title", "content 2", nil)

	// Try to update second item to have first item's title
	_, err := s.Update(item2.ID, "First Title", "content 2", nil)
	if err == nil {
		t.Error("expected error for updating to duplicate title")
	}
}

func TestSearchDefaultLimit(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "cue-searchlimit-*.db")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, _ := New(tmpFile.Name())
	defer s.Close()

	s.Create("Test Alpha", "alpha content", nil)

	// Zero limit should use default (20)
	results, err := s.Search("alpha", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("len = %d, want 1", len(results))
	}
}
