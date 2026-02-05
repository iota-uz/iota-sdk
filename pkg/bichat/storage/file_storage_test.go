package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalFileStorage_Save(t *testing.T) {
	t.Parallel()

	// Create temporary directory for test
	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello, World!")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        int64(len(content)),
	}

	// Save file
	url, err := storage.Save(ctx, "test.txt", bytes.NewReader(content), metadata)
	require.NoError(t, err)
	assert.Contains(t, url, "http://localhost/files/")
	assert.Contains(t, url, ".txt")

	// Verify file exists on disk
	filename := filepath.Base(url)
	filePath := filepath.Join(tmpDir, filename)
	savedContent, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, content, savedContent)
}

func TestLocalFileStorage_SaveSizeMismatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Hello")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        100, // Wrong size
	}

	// Save should fail due to size mismatch
	_, err = storage.Save(ctx, "test.txt", bytes.NewReader(content), metadata)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func TestLocalFileStorage_Get(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()
	originalContent := []byte("Test Content")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        int64(len(originalContent)),
	}

	// Save file
	url, err := storage.Save(ctx, "test.txt", bytes.NewReader(originalContent), metadata)
	require.NoError(t, err)

	// Get file
	reader, err := storage.Get(ctx, url)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	// Read content
	retrievedContent, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, originalContent, retrievedContent)
}

func TestLocalFileStorage_GetNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()

	// Try to get non-existent file
	_, err = storage.Get(ctx, "http://localhost/files/nonexistent.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "file not found")
}

func TestLocalFileStorage_Delete(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()
	content := []byte("Delete me")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        int64(len(content)),
	}

	// Save file
	url, err := storage.Save(ctx, "test.txt", bytes.NewReader(content), metadata)
	require.NoError(t, err)

	// Verify file exists
	exists, err := storage.Exists(ctx, url)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete file
	err = storage.Delete(ctx, url)
	require.NoError(t, err)

	// Verify file no longer exists
	exists, err = storage.Exists(ctx, url)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalFileStorage_DeleteNonExistent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()

	// Delete non-existent file should not error
	err = storage.Delete(ctx, "http://localhost/files/nonexistent.txt")
	assert.NoError(t, err)
}

func TestLocalFileStorage_Exists(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()

	// Check non-existent file
	exists, err := storage.Exists(ctx, "http://localhost/files/missing.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Save file
	content := []byte("Exists test")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        int64(len(content)),
	}
	url, err := storage.Save(ctx, "test.txt", bytes.NewReader(content), metadata)
	require.NoError(t, err)

	// Check existing file
	exists, err = storage.Exists(ctx, url)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalFileStorage_LargeFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()

	// Create large content (1 MB)
	content := bytes.Repeat([]byte("A"), 1024*1024)
	metadata := FileMetadata{
		ContentType: "application/octet-stream",
		Size:        int64(len(content)),
	}

	// Save large file
	url, err := storage.Save(ctx, "large.bin", bytes.NewReader(content), metadata)
	require.NoError(t, err)

	// Retrieve and verify
	reader, err := storage.Get(ctx, url)
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()

	retrieved, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Len(t, retrieved, len(content))
}

func TestLocalFileStorage_PreserveExtension(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storage, err := NewLocalFileStorage(tmpDir, "http://localhost/files")
	require.NoError(t, err)

	ctx := context.Background()
	testCases := []string{
		"document.pdf",
		"report.xlsx",
		"image.png",
		"data.json",
		"archive.tar.gz",
	}

	for _, filename := range testCases {
		t.Run(filename, func(t *testing.T) {
			content := []byte("test")
			metadata := FileMetadata{
				ContentType: "application/octet-stream",
				Size:        int64(len(content)),
			}

			url, err := storage.Save(ctx, filename, bytes.NewReader(content), metadata)
			require.NoError(t, err)

			// Verify extension is preserved
			ext := filepath.Ext(filename)
			assert.True(t, strings.HasSuffix(url, ext),
				"URL %s should end with extension %s", url, ext)
		})
	}
}

func TestNoOpFileStorage(t *testing.T) {
	t.Parallel()

	storage := NewNoOpFileStorage()
	ctx := context.Background()
	content := []byte("test")
	metadata := FileMetadata{
		ContentType: "text/plain",
		Size:        int64(len(content)),
	}

	// Save returns placeholder URL
	url, err := storage.Save(ctx, "test.txt", bytes.NewReader(content), metadata)
	require.NoError(t, err)
	assert.Contains(t, url, "test.txt")

	// Get always fails
	_, err = storage.Get(ctx, url)
	require.Error(t, err)

	// Delete is no-op
	err = storage.Delete(ctx, url)
	require.NoError(t, err)

	// Exists always returns false
	exists, err := storage.Exists(ctx, url)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestNewLocalFileStorage_CreatesDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storageDir := filepath.Join(tmpDir, "nested", "storage", "path")

	// Directory doesn't exist yet
	_, err := os.Stat(storageDir)
	assert.True(t, os.IsNotExist(err))

	// Create storage - should create directory
	storage, err := NewLocalFileStorage(storageDir, "http://localhost/files")
	require.NoError(t, err)
	require.NotNil(t, storage)

	// Directory should now exist
	info, err := os.Stat(storageDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
