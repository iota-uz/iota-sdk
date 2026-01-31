package sources

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/iota-uz/iota-sdk/pkg/bichat/kb"
)

// fileSystemSource indexes documents from a filesystem directory.
type fileSystemSource struct {
	root            string
	extensions      []string
	ignorePatterns  []string
	recursive       bool
	extractTitle    bool
	includeMetadata bool
}

// FSOption is a functional option for configuring FileSystemSource.
type FSOption func(*fileSystemSource)

// WithExtensions filters files by extension (e.g., ".md", ".txt").
// If empty, all files are indexed.
func WithExtensions(exts ...string) FSOption {
	return func(fs *fileSystemSource) {
		fs.extensions = exts
	}
}

// WithIgnorePatterns excludes files matching these patterns (glob-style).
func WithIgnorePatterns(patterns ...string) FSOption {
	return func(fs *fileSystemSource) {
		fs.ignorePatterns = patterns
	}
}

// WithRecursive enables recursive directory traversal.
func WithRecursive(recursive bool) FSOption {
	return func(fs *fileSystemSource) {
		fs.recursive = recursive
	}
}

// WithExtractTitle attempts to extract title from first heading or filename.
func WithExtractTitle(extract bool) FSOption {
	return func(fs *fileSystemSource) {
		fs.extractTitle = extract
	}
}

// WithIncludeMetadata includes file metadata (size, modified time) in document metadata.
func WithIncludeMetadata(include bool) FSOption {
	return func(fs *fileSystemSource) {
		fs.includeMetadata = include
	}
}

// NewFileSystemSource creates a DocumentSource that indexes files from a directory.
func NewFileSystemSource(root string, opts ...FSOption) kb.DocumentSource {
	fs := &fileSystemSource{
		root:            root,
		extensions:      []string{".md", ".txt", ".html"},
		ignorePatterns:  []string{},
		recursive:       true,
		extractTitle:    true,
		includeMetadata: true,
	}

	for _, opt := range opts {
		opt(fs)
	}

	return fs
}

// List implements DocumentSource.
func (fs *fileSystemSource) List(ctx context.Context) ([]kb.Document, error) {
	var docs []kb.Document

	err := filepath.Walk(fs.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			if !fs.recursive && path != fs.root {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be indexed
		if !fs.shouldIndex(path) {
			return nil
		}

		// Read and create document
		doc, docErr := fs.createDocument(path, info)
		if docErr != nil {
			// Skip files that cannot be read/processed, but continue with other files
			// We intentionally don't return the error to allow processing remaining files
			//nolint:nilerr // Intentionally skipping unreadable files to continue indexing
			return nil
		}

		docs = append(docs, doc)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return docs, nil
}

// Watch implements DocumentSource.
func (fs *fileSystemSource) Watch(ctx context.Context) (<-chan kb.DocumentChange, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Add root directory to watcher
	if err := watcher.Add(fs.root); err != nil {
		if closeErr := watcher.Close(); closeErr != nil {
			return nil, fmt.Errorf("failed to watch directory: %w (close error: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	// If recursive, add subdirectories
	if fs.recursive {
		err := filepath.Walk(fs.root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			if closeErr := watcher.Close(); closeErr != nil {
				return nil, fmt.Errorf("failed to watch subdirectories: %w (close error: %v)", err, closeErr)
			}
			return nil, fmt.Errorf("failed to watch subdirectories: %w", err)
		}
	}

	changes := make(chan kb.DocumentChange, 100)

	go func() {
		defer close(changes)
		defer func() {
			if err := watcher.Close(); err != nil {
				// TODO: log watcher close error
				_ = err
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return

			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				// Only process files we care about
				if !fs.shouldIndex(event.Name) {
					continue
				}

				var change kb.DocumentChange

				switch {
				case event.Op&fsnotify.Create == fsnotify.Create:
					info, err := os.Stat(event.Name)
					if err != nil {
						continue
					}
					doc, err := fs.createDocument(event.Name, info)
					if err != nil {
						continue
					}
					change = kb.DocumentChange{
						Type:     kb.ChangeTypeCreate,
						Document: &doc,
					}

				case event.Op&fsnotify.Write == fsnotify.Write:
					info, err := os.Stat(event.Name)
					if err != nil {
						continue
					}
					doc, err := fs.createDocument(event.Name, info)
					if err != nil {
						continue
					}
					change = kb.DocumentChange{
						Type:     kb.ChangeTypeUpdate,
						Document: &doc,
					}

				case event.Op&fsnotify.Remove == fsnotify.Remove:
					change = kb.DocumentChange{
						Type: kb.ChangeTypeDelete,
						ID:   fs.generateDocumentID(event.Name),
					}

				default:
					continue
				}

				select {
				case changes <- change:
				case <-ctx.Done():
					return
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Log error but continue watching
				_ = err
			}
		}
	}()

	return changes, nil
}

// shouldIndex checks if a file should be indexed based on extension and patterns.
func (fs *fileSystemSource) shouldIndex(path string) bool {
	// Check ignore patterns
	for _, pattern := range fs.ignorePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
	}

	// Check extensions
	if len(fs.extensions) == 0 {
		return true
	}

	ext := filepath.Ext(path)
	for _, allowed := range fs.extensions {
		if ext == allowed {
			return true
		}
	}

	return false
}

// createDocument creates a Document from a file.
func (fs *fileSystemSource) createDocument(path string, info os.FileInfo) (kb.Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return kb.Document{}, fmt.Errorf("failed to read file: %w", err)
	}

	doc := kb.Document{
		ID:        fs.generateDocumentID(path),
		Path:      path,
		Content:   string(content),
		Type:      fs.detectDocumentType(path),
		Metadata:  make(map[string]string),
		UpdatedAt: info.ModTime(),
	}

	// Extract title
	if fs.extractTitle {
		doc.Title = fs.extractTitleFromContent(string(content), path)
	} else {
		doc.Title = filepath.Base(path)
	}

	// Add metadata
	if fs.includeMetadata {
		doc.Metadata["size"] = fmt.Sprintf("%d", info.Size())
		doc.Metadata["modified"] = info.ModTime().Format(time.RFC3339)
		doc.Metadata["extension"] = filepath.Ext(path)
	}

	return doc, nil
}

// generateDocumentID creates a unique ID for a document based on its path.
func (fs *fileSystemSource) generateDocumentID(path string) string {
	// Use relative path from root to ensure consistency
	relPath, err := filepath.Rel(fs.root, path)
	if err != nil {
		relPath = path
	}

	// Generate hash for consistent ID
	hash := sha256.Sum256([]byte(relPath))
	return fmt.Sprintf("fs_%x", hash[:16])
}

// detectDocumentType determines document type from file extension.
func (fs *fileSystemSource) detectDocumentType(path string) kb.DocumentType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown":
		return kb.DocumentTypeMarkdown
	case ".html", ".htm":
		return kb.DocumentTypeHTML
	case ".pdf":
		return kb.DocumentTypePDF
	case ".json":
		return kb.DocumentTypeJSON
	case ".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".rs":
		return kb.DocumentTypeCode
	default:
		return kb.DocumentTypeText
	}
}

// extractTitleFromContent extracts title from markdown heading or filename.
func (fs *fileSystemSource) extractTitleFromContent(content string, path string) string {
	lines := strings.Split(content, "\n")

	// Try to find markdown heading
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			// Remove leading # and trim
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if title != "" {
				return title
			}
		}
		// Stop after first non-empty, non-heading line
		if line != "" && !strings.HasPrefix(line, "#") {
			break
		}
	}

	// Fallback to filename without extension
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}

	return name
}
