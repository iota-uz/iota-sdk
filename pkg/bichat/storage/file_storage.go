package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// FileMetadata contains metadata about a stored file.
type FileMetadata struct {
	// ContentType is the MIME type of the file.
	ContentType string

	// Size is the file size in bytes.
	Size int64

	// ExpiresAt is the optional expiration time for temporary files.
	ExpiresAt *time.Time
}

// FileStorage provides an interface for storing and retrieving files.
// Implementations can use local filesystem, S3, GCS, etc.
type FileStorage interface {
	// Save stores a file and returns its URL.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - filename: Original filename (with extension)
	//   - content: File content reader
	//   - metadata: File metadata (content type, size, expiration)
	//
	// Returns:
	//   - url: Public URL to access the file
	//   - error: Any error during storage
	Save(ctx context.Context, filename string, content io.Reader, metadata FileMetadata) (url string, error error)

	// Get retrieves a file by its URL.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - url: File URL returned by Save
	//
	// Returns:
	//   - reader: File content reader (caller must close)
	//   - error: Any error during retrieval
	Get(ctx context.Context, url string) (io.ReadCloser, error)

	// Delete removes a file by its URL.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - url: File URL returned by Save
	//
	// Returns:
	//   - error: Any error during deletion
	Delete(ctx context.Context, url string) error

	// Exists checks if a file exists at the given URL.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - url: File URL returned by Save
	//
	// Returns:
	//   - exists: True if file exists
	//   - error: Any error during check
	Exists(ctx context.Context, url string) (bool, error)
}

// LocalFileStorage implements FileStorage using local filesystem.
//
// Files are stored in baseDir with unique names (UUID + extension).
// URLs are constructed as baseURL + filename.
//
// Example:
//
//	storage := storage.NewLocalFileStorage("/var/lib/bichat/files", "https://example.com/files")
//	url, _ := storage.Save(ctx, "report.pdf", pdfData, metadata)
//	// Returns: https://example.com/files/550e8400-e29b-41d4-a716-446655440000.pdf
type LocalFileStorage struct {
	baseDir string // Local directory for file storage
	baseURL string // Base URL for file access
}

// NewLocalFileStorage creates a local filesystem storage backend.
//
// Parameters:
//   - baseDir: Directory for storing files (will be created if missing)
//   - baseURL: Base URL for accessing files (e.g., "https://example.com/files")
//
// Example:
//
//	storage := storage.NewLocalFileStorage(
//	    "/var/lib/bichat/files",
//	    "https://example.com/files",
//	)
func NewLocalFileStorage(baseDir, baseURL string) (FileStorage, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalFileStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}, nil
}

// Save stores a file in the local filesystem.
func (s *LocalFileStorage) Save(ctx context.Context, filename string, content io.Reader, metadata FileMetadata) (string, error) {
	const op = "LocalFileStorage.Save"

	// Generate unique filename (UUID + original extension)
	ext := filepath.Ext(filename)
	uniqueName := uuid.New().String() + ext
	filePath := filepath.Join(s.baseDir, uniqueName)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("%s: failed to create file: %w", op, err)
	}
	defer func() { _ = file.Close() }()

	// Copy content to file
	written, err := io.Copy(file, content)
	if err != nil {
		// Clean up partial file
		_ = os.Remove(filePath)
		return "", fmt.Errorf("%s: failed to write content: %w", op, err)
	}

	// Verify size matches metadata if provided
	if metadata.Size > 0 && written != metadata.Size {
		_ = os.Remove(filePath)
		return "", fmt.Errorf("%s: size mismatch (expected %d, got %d)", op, metadata.Size, written)
	}

	// Construct URL
	url := fmt.Sprintf("%s/%s", s.baseURL, uniqueName)

	return url, nil
}

// Get retrieves a file from the local filesystem.
func (s *LocalFileStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	const op = "LocalFileStorage.Get"

	// Extract filename from URL
	filename := filepath.Base(url)
	filePath := filepath.Join(s.baseDir, filename)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: file not found", op)
		}
		return nil, fmt.Errorf("%s: failed to open file: %w", op, err)
	}

	return file, nil
}

// Delete removes a file from the local filesystem.
func (s *LocalFileStorage) Delete(ctx context.Context, url string) error {
	const op = "LocalFileStorage.Delete"

	// Extract filename from URL
	filename := filepath.Base(url)
	filePath := filepath.Join(s.baseDir, filename)

	// Delete file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			// File already gone - not an error
			return nil
		}
		return fmt.Errorf("%s: failed to delete file: %w", op, err)
	}

	return nil
}

// Exists checks if a file exists in the local filesystem.
func (s *LocalFileStorage) Exists(ctx context.Context, url string) (bool, error) {
	const op = "LocalFileStorage.Exists"

	// Extract filename from URL
	filename := filepath.Base(url)
	filePath := filepath.Join(s.baseDir, filename)

	// Check file existence
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("%s: failed to check file: %w", op, err)
	}

	return true, nil
}

// NoOpFileStorage is a no-op storage backend that doesn't persist files.
// Useful for testing or when file storage is disabled.
type NoOpFileStorage struct{}

// NewNoOpFileStorage creates a no-op storage backend.
func NewNoOpFileStorage() FileStorage {
	return &NoOpFileStorage{}
}

// Save returns a placeholder URL without storing the file.
func (s *NoOpFileStorage) Save(ctx context.Context, filename string, content io.Reader, metadata FileMetadata) (string, error) {
	// Drain content reader
	_, _ = io.Copy(io.Discard, content)
	// Return placeholder URL
	return fmt.Sprintf("http://localhost/files/%s", filename), nil
}

// Get returns an error as no files are actually stored.
func (s *NoOpFileStorage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("NoOpFileStorage: file not found")
}

// Delete is a no-op.
func (s *NoOpFileStorage) Delete(ctx context.Context, url string) error {
	return nil
}

// Exists always returns false.
func (s *NoOpFileStorage) Exists(ctx context.Context, url string) (bool, error) {
	return false, nil
}
