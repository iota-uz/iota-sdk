package kb_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

func TestBleveIndex_CRUD(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test index
	tmpDir, err := os.MkdirTemp("", "bleve-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "test.bleve")
	indexer, searcher, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	ctx := context.Background()

	// Create a test document
	doc := kb.Document{
		ID:      "doc1",
		Title:   "Test Document",
		Content: "This is a test document with some content.",
		Path:    "/test/doc1.md",
		Type:    kb.DocumentTypeMarkdown,
		Metadata: map[string]string{
			"author": "test",
		},
		UpdatedAt: time.Now(),
	}

	// Test Create (IndexDocument)
	err = indexer.IndexDocument(ctx, doc)
	if err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}

	// Test Read (GetDocument)
	retrieved, err := searcher.GetDocument(ctx, "doc1")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected document to be retrieved")
	}
	if retrieved.ID != "doc1" {
		t.Errorf("Expected ID 'doc1', got '%s'", retrieved.ID)
	}
	if retrieved.Title != "Test Document" {
		t.Errorf("Expected title 'Test Document', got '%s'", retrieved.Title)
	}

	// Test Update (IndexDocument with same ID)
	doc.Content = "Updated content"
	err = indexer.IndexDocument(ctx, doc)
	if err != nil {
		t.Fatalf("Update IndexDocument failed: %v", err)
	}

	updated, err := searcher.GetDocument(ctx, "doc1")
	if err != nil {
		t.Fatalf("GetDocument after update failed: %v", err)
	}
	if !strings.Contains(updated.Content, "Updated content") {
		t.Error("Expected content to be updated")
	}

	// Test Delete
	err = indexer.DeleteDocument(ctx, "doc1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	deleted, err := searcher.GetDocument(ctx, "doc1")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Unexpected error after delete: %v", err)
	}
	if deleted != nil {
		t.Error("Expected document to be deleted")
	}
}

func TestBleveSearch_Basic(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-search-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "search.bleve")
	indexer, searcher, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	ctx := context.Background()

	// Index multiple documents
	docs := []kb.Document{
		{
			ID:      "1",
			Title:   "Go Programming",
			Content: "Go is a statically typed, compiled programming language.",
			Type:    kb.DocumentTypeMarkdown,
		},
		{
			ID:      "2",
			Title:   "Python Programming",
			Content: "Python is a high-level, interpreted programming language.",
			Type:    kb.DocumentTypeMarkdown,
		},
		{
			ID:      "3",
			Title:   "Database Design",
			Content: "PostgreSQL is a powerful relational database system.",
			Type:    kb.DocumentTypeMarkdown,
		},
	}

	err = indexer.IndexDocuments(ctx, docs)
	if err != nil {
		t.Fatalf("IndexDocuments failed: %v", err)
	}

	// Give index time to process
	time.Sleep(100 * time.Millisecond)

	// Test search
	tests := []struct {
		name          string
		query         string
		expectedCount int
		shouldContain string
	}{
		{
			name:          "search for programming",
			query:         "programming",
			expectedCount: 2,
			shouldContain: "Programming",
		},
		{
			name:          "search for Go",
			query:         "Go",
			expectedCount: 1,
			shouldContain: "Go",
		},
		{
			name:          "search for database",
			query:         "database",
			expectedCount: 1,
			shouldContain: "Database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searcher.Search(ctx, tt.query, kb.SearchOptions{TopK: 10})
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}

			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}

			if len(results) > 0 {
				found := false
				for _, result := range results {
					if strings.Contains(result.Document.Title, tt.shouldContain) ||
						strings.Contains(result.Document.Content, tt.shouldContain) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected results to contain '%s'", tt.shouldContain)
				}
			}
		})
	}
}

func TestBleveSearch_Options(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-options-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "options.bleve")
	indexer, searcher, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	ctx := context.Background()

	// Index documents with different relevance
	docs := []kb.Document{
		{ID: "1", Title: "Go Basics", Content: "Introduction to Go programming"},
		{ID: "2", Title: "Advanced Go", Content: "Advanced Go programming techniques"},
		{ID: "3", Title: "Go Tips", Content: "Tips and tricks for Go developers"},
		{ID: "4", Title: "Python Guide", Content: "Python programming basics"},
	}

	err = indexer.IndexDocuments(ctx, docs)
	if err != nil {
		t.Fatalf("IndexDocuments failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	t.Run("TopK limits results", func(t *testing.T) {
		results, err := searcher.Search(ctx, "Go", kb.SearchOptions{TopK: 2})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) > 2 {
			t.Errorf("Expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("MinScore filters low-relevance results", func(t *testing.T) {
		// Search with high minimum score
		results, err := searcher.Search(ctx, "Go", kb.SearchOptions{
			TopK:     10,
			MinScore: 0.5,
		})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		// Verify all results meet minimum score
		for _, result := range results {
			if result.Score < 0.5 {
				t.Errorf("Result score %f is below minimum %f", result.Score, 0.5)
			}
		}
	})

	t.Run("Search returns scores", func(t *testing.T) {
		results, err := searcher.Search(ctx, "Go programming", kb.SearchOptions{TopK: 10})
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Fatal("Expected at least one result")
		}

		// Verify scores are positive and descending
		for i, result := range results {
			if result.Score <= 0 {
				t.Errorf("Result %d has non-positive score: %f", i, result.Score)
			}

			if i > 0 && result.Score > results[i-1].Score {
				t.Errorf("Results not in descending score order at index %d", i)
			}
		}
	})
}

func TestBleveIndex_Stats(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-stats-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "stats.bleve")
	indexer, _, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	ctx := context.Background()

	// Initially empty
	stats := indexer.GetStats()
	if stats.DocumentCount != 0 {
		t.Errorf("Expected 0 documents initially, got %d", stats.DocumentCount)
	}

	// Index some documents
	docs := []kb.Document{
		{ID: "1", Title: "Doc 1", Content: "Content 1"},
		{ID: "2", Title: "Doc 2", Content: "Content 2"},
		{ID: "3", Title: "Doc 3", Content: "Content 3"},
	}

	err = indexer.IndexDocuments(ctx, docs)
	if err != nil {
		t.Fatalf("IndexDocuments failed: %v", err)
	}

	// Verify stats updated
	stats = indexer.GetStats()
	if stats.DocumentCount != 3 {
		t.Errorf("Expected 3 documents, got %d", stats.DocumentCount)
	}

	// Delete one document
	err = indexer.DeleteDocument(ctx, "1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	stats = indexer.GetStats()
	if stats.DocumentCount != 2 {
		t.Errorf("Expected 2 documents after delete, got %d", stats.DocumentCount)
	}
}

func TestBleveIndex_Rebuild(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-rebuild-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "rebuild.bleve")
	indexer, searcher, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	ctx := context.Background()

	// Index initial documents
	initialDocs := []kb.Document{
		{ID: "old1", Title: "Old Doc 1", Content: "Old content 1"},
		{ID: "old2", Title: "Old Doc 2", Content: "Old content 2"},
	}

	err = indexer.IndexDocuments(ctx, initialDocs)
	if err != nil {
		t.Fatalf("IndexDocuments failed: %v", err)
	}

	// Create a mock source with new documents
	source := &mockDocumentSource{
		docs: []kb.Document{
			{ID: "new1", Title: "New Doc 1", Content: "New content 1"},
			{ID: "new2", Title: "New Doc 2", Content: "New content 2"},
		},
	}

	// Rebuild index from source
	err = indexer.Rebuild(ctx, source)
	if err != nil {
		t.Fatalf("Rebuild failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Old documents should be gone
	oldDoc, _ := searcher.GetDocument(ctx, "old1")
	if oldDoc != nil {
		t.Error("Expected old document to be removed after rebuild")
	}

	// New documents should be present
	newDoc, err := searcher.GetDocument(ctx, "new1")
	if err != nil {
		t.Fatalf("Failed to get new document: %v", err)
	}
	if newDoc == nil {
		t.Fatal("Expected new document to be indexed")
	}
	if newDoc.Title != "New Doc 1" {
		t.Errorf("Expected title 'New Doc 1', got '%s'", newDoc.Title)
	}

	// Verify document count
	stats := indexer.GetStats()
	if stats.DocumentCount != 2 {
		t.Errorf("Expected 2 documents after rebuild, got %d", stats.DocumentCount)
	}
}

func TestBleveIndex_ContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-ctx-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "ctx.bleve")
	indexer, _, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}
	defer func() { _ = indexer.Close() }()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	doc := kb.Document{
		ID:      "test",
		Title:   "Test",
		Content: "Test content",
	}

	// Operation with cancelled context should fail
	err = indexer.IndexDocument(ctx, doc)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestBleveIndex_IsAvailable(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "bleve-available-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	indexPath := filepath.Join(tmpDir, "available.bleve")
	_, searcher, err := kb.NewBleveIndex(indexPath)
	if err != nil {
		t.Fatalf("Failed to create Bleve index: %v", err)
	}

	// Should be available after creation
	if !searcher.IsAvailable() {
		t.Error("Expected index to be available after creation")
	}
}

// mockDocumentSource is a simple in-memory document source for testing
type mockDocumentSource struct {
	docs []kb.Document
}

func (m *mockDocumentSource) List(ctx context.Context) ([]kb.Document, error) {
	return m.docs, nil
}

var errMockWatchNotImplemented = errors.New("mock document source: watch not implemented")

func (m *mockDocumentSource) Watch(ctx context.Context) (<-chan kb.DocumentChange, error) {
	// Not implemented for this test
	return nil, errMockWatchNotImplemented
}
