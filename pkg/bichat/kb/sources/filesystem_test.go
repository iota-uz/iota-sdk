package sources_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb/sources"
	"github.com/stretchr/testify/require"
)

func TestFileSystemSource_List(t *testing.T) {
	t.Parallel()

	// Create temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "fs-source-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	testFiles := map[string]string{
		"doc1.md":  "# Document 1\n\nThis is document 1 content.",
		"doc2.txt": "This is a text document.",
		"doc3.md":  "# Another Document\n\nMore content here.",
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", name, err)
		}
	}

	// Create filesystem source
	source := sources.NewFileSystemSource(tmpDir)

	ctx := context.Background()
	docs, err := source.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify document count (should include .md and .txt files)
	if len(docs) < 2 {
		t.Errorf("Expected at least 2 documents, got %d", len(docs))
	}

	// Verify documents have required fields
	for _, doc := range docs {
		if doc.ID == "" {
			t.Error("Document missing ID")
		}
		if doc.Content == "" {
			t.Error("Document missing content")
		}
		if doc.Path == "" {
			t.Error("Document missing path")
		}
		if doc.Type == "" {
			t.Error("Document missing type")
		}
	}
}

func TestFileSystemSource_Recursive(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-recursive-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create nested directory structure
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create files at different levels
	_ = os.WriteFile(filepath.Join(tmpDir, "root.md"), []byte("# Root"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, "sub.md"), []byte("# Sub"), 0644)

	ctx := context.Background()

	t.Run("recursive enabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithRecursive(true),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Should find both files
		if len(docs) < 2 {
			t.Errorf("Expected at least 2 documents with recursive, got %d", len(docs))
		}
	})

	t.Run("recursive disabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithRecursive(false),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Should only find root file
		if len(docs) != 1 {
			t.Errorf("Expected 1 document without recursive, got %d", len(docs))
		}

		// Verify it's the root file
		found := false
		for _, doc := range docs {
			if strings.Contains(doc.Path, "root.md") {
				found = true
			}
		}
		if !found {
			t.Error("Expected to find root.md")
		}
	})
}

func TestFileSystemSource_Extensions(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-ext-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create files with different extensions
	files := map[string]string{
		"doc.md":   "Markdown content",
		"doc.txt":  "Text content",
		"doc.html": "HTML content",
		"doc.go":   "Go code",
		"doc.json": "JSON data",
	}

	for name, content := range files {
		_ = os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
	}

	ctx := context.Background()

	t.Run("filter markdown only", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithExtensions(".md"),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) != 1 {
			t.Errorf("Expected 1 markdown document, got %d", len(docs))
		}

		if len(docs) > 0 && !strings.HasSuffix(docs[0].Path, ".md") {
			t.Error("Expected only .md files")
		}
	})

	t.Run("filter multiple extensions", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithExtensions(".md", ".txt"),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) != 2 {
			t.Errorf("Expected 2 documents (.md and .txt), got %d", len(docs))
		}

		for _, doc := range docs {
			ext := filepath.Ext(doc.Path)
			if ext != ".md" && ext != ".txt" {
				t.Errorf("Unexpected extension: %s", ext)
			}
		}
	})

	t.Run("no filter (all files)", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithExtensions(), // Empty extensions = all files
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		// Note: Default NewFileSystemSource has extensions, so we need empty list
		// This test verifies the filter behavior
		if len(docs) == 0 {
			t.Error("Expected some documents when no extension filter")
		}
	})
}

func TestFileSystemSource_ExtractTitle(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-title-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create markdown file with heading
	mdContent := `# Main Title

This is the content.

## Subheading

More content.
`
	_ = os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte(mdContent), 0644)

	ctx := context.Background()

	t.Run("extract title enabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithExtractTitle(true),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) == 0 {
			t.Fatal("Expected at least one document")
		}

		// Title should be extracted from heading
		if docs[0].Title != "Main Title" {
			t.Errorf("Expected title 'Main Title', got '%s'", docs[0].Title)
		}
	})

	t.Run("extract title disabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithExtractTitle(false),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) == 0 {
			t.Fatal("Expected at least one document")
		}

		// Title should be filename
		if docs[0].Title != "doc.md" {
			t.Errorf("Expected title 'doc.md', got '%s'", docs[0].Title)
		}
	})
}

func TestFileSystemSource_IncludeMetadata(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-metadata-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	_ = os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte("Content"), 0644)

	ctx := context.Background()

	t.Run("metadata enabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithIncludeMetadata(true),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) == 0 {
			t.Fatal("Expected at least one document")
		}

		// Check metadata fields
		doc := docs[0]
		if doc.Metadata == nil {
			t.Fatal("Expected metadata to be set")
		}

		if _, ok := doc.Metadata["size"]; !ok {
			t.Error("Expected 'size' in metadata")
		}
		if _, ok := doc.Metadata["modified"]; !ok {
			t.Error("Expected 'modified' in metadata")
		}
		if _, ok := doc.Metadata["extension"]; !ok {
			t.Error("Expected 'extension' in metadata")
		}
	})

	t.Run("metadata disabled", func(t *testing.T) {
		source := sources.NewFileSystemSource(tmpDir,
			sources.WithIncludeMetadata(false),
		)

		docs, err := source.List(ctx)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}

		if len(docs) == 0 {
			t.Fatal("Expected at least one document")
		}

		// Metadata should be empty or minimal
		doc := docs[0]
		if len(doc.Metadata) > 0 {
			// Check that standard metadata fields are not present
			if _, ok := doc.Metadata["size"]; ok {
				t.Error("Expected 'size' NOT in metadata when disabled")
			}
		}
	})
}

func TestFileSystemSource_IgnorePatterns(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-ignore-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	_ = os.WriteFile(filepath.Join(tmpDir, "include.md"), []byte("Include"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "ignore.md"), []byte("Ignore"), 0644)
	_ = os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("Test"), 0644)

	ctx := context.Background()

	source := sources.NewFileSystemSource(tmpDir,
		sources.WithIgnorePatterns("ignore.*"),
	)

	docs, err := source.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify ignored file is not included
	for _, doc := range docs {
		if strings.Contains(doc.Path, "ignore.md") {
			t.Error("Expected ignore.md to be excluded")
		}
	}

	// Verify other files are included
	foundInclude := false
	foundTest := false
	for _, doc := range docs {
		if strings.Contains(doc.Path, "include.md") {
			foundInclude = true
		}
		if strings.Contains(doc.Path, "test.md") {
			foundTest = true
		}
	}

	if !foundInclude {
		t.Error("Expected include.md to be indexed")
	}
	if !foundTest {
		t.Error("Expected test.md to be indexed")
	}
}

func TestFileSystemSource_DocumentTypes(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-types-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create files of different types
	files := map[string]kb.DocumentType{
		"doc.md":   kb.DocumentTypeMarkdown,
		"doc.txt":  kb.DocumentTypeText,
		"doc.html": kb.DocumentTypeHTML,
		"doc.json": kb.DocumentTypeJSON,
		"doc.go":   kb.DocumentTypeCode,
	}

	for name := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0644))
	}

	ctx := context.Background()

	source := sources.NewFileSystemSource(tmpDir,
		sources.WithExtensions(".md", ".txt", ".html", ".json", ".go"),
	)

	docs, err := source.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify document types are detected correctly
	typeMap := make(map[string]kb.DocumentType)
	for _, doc := range docs {
		ext := filepath.Ext(doc.Path)
		typeMap[ext] = doc.Type
	}

	expectedTypes := map[string]kb.DocumentType{
		".md":   kb.DocumentTypeMarkdown,
		".txt":  kb.DocumentTypeText,
		".html": kb.DocumentTypeHTML,
		".json": kb.DocumentTypeJSON,
		".go":   kb.DocumentTypeCode,
	}

	for ext, expectedType := range expectedTypes {
		if actualType, ok := typeMap[ext]; ok {
			if actualType != expectedType {
				t.Errorf("Extension %s: expected type %s, got %s", ext, expectedType, actualType)
			}
		}
	}
}

func TestFileSystemSource_ContextCancellation(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-ctx-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte("Content"), 0644))

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	source := sources.NewFileSystemSource(tmpDir)

	// List should respect cancellation
	_, err = source.List(ctx)
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}

func TestFileSystemSource_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	ctx := context.Background()

	source := sources.NewFileSystemSource(tmpDir)

	docs, err := source.List(ctx)
	if err != nil {
		t.Fatalf("List failed on empty directory: %v", err)
	}

	if len(docs) != 0 {
		t.Errorf("Expected 0 documents from empty directory, got %d", len(docs))
	}
}

func TestFileSystemSource_UniqueIDs(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "fs-ids-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create multiple files
	for i := 0; i < 5; i++ {
		name := filepath.Join(tmpDir, "doc"+string(rune('0'+i))+".md")
		require.NoError(t, os.WriteFile(name, []byte("Content"), 0644))
	}

	ctx := context.Background()

	source := sources.NewFileSystemSource(tmpDir)

	docs, err := source.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify all IDs are unique
	ids := make(map[string]bool)
	for _, doc := range docs {
		if ids[doc.ID] {
			t.Errorf("Duplicate ID found: %s", doc.ID)
		}
		ids[doc.ID] = true
	}

	if len(ids) != len(docs) {
		t.Errorf("Expected %d unique IDs, got %d", len(docs), len(ids))
	}
}
