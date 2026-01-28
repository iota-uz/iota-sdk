package multifs_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/multifs"
	"github.com/stretchr/testify/require"
)

func TestNeuteredFileSystem_DirectoryWithoutIndex_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with a file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Try to open the directory (should fail)
	file, err := fs.Open("/")
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)
}

func TestNeuteredFileSystem_DirectoryWithIndex_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with index.html
	tmpDir := t.TempDir()
	indexContent := "<!DOCTYPE html><html><body>Index</body></html>"
	indexFile := filepath.Join(tmpDir, "index.html")
	err := os.WriteFile(indexFile, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Open the directory (should return error, not index.html)
	file, err := fs.Open("/")
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)

	// However, accessing index.html directly should work
	indexFileHandle, err := fs.Open("/index.html")
	require.NoError(t, err)
	require.NotNil(t, indexFileHandle)
	defer indexFileHandle.Close()

	// Read content
	content, err := io.ReadAll(indexFileHandle)
	require.NoError(t, err)
	require.Equal(t, indexContent, string(content))
}

func TestNeuteredFileSystem_RegularFile_ReturnsFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with a file
	tmpDir := t.TempDir()
	testContent := "test file content"
	testFile := filepath.Join(tmpDir, "document.txt")
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Open the file
	file, err := fs.Open("/document.txt")
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, testContent, string(content))

	// Verify it's not a directory
	stat, err := file.Stat()
	require.NoError(t, err)
	require.False(t, stat.IsDir())
}

func TestNeuteredFileSystem_SubdirectoryWithoutIndex_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Add a file in subdirectory
	testFile := filepath.Join(subDir, "file.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Try to open the subdirectory (should fail)
	file, err := fs.Open("/subdir")
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)
}

func TestNeuteredFileSystem_SubdirectoryWithIndex_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Add index.html in subdirectory
	indexContent := "Subdirectory Index"
	indexFile := filepath.Join(subDir, "index.html")
	err = os.WriteFile(indexFile, []byte(indexContent), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Open the subdirectory (should return error, not index.html)
	file, err := fs.Open("/subdir")
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)

	// However, accessing index.html directly should work
	indexFileHandle, err := fs.Open("/subdir/index.html")
	require.NoError(t, err)
	require.NotNil(t, indexFileHandle)
	defer indexFileHandle.Close()

	// Read content
	content, err := io.ReadAll(indexFileHandle)
	require.NoError(t, err)
	require.Equal(t, indexContent, string(content))
}

func TestNeuteredFileSystem_NonExistentFile_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary empty directory
	tmpDir := t.TempDir()

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Try to open non-existent file
	file, err := fs.Open("/nonexistent.txt")
	require.Error(t, err)
	require.Nil(t, file)
}

func TestNeuteredFileSystem_FileInSubdirectory_ReturnsFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "images")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Add a file in subdirectory
	imageContent := "fake image data"
	imageFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(imageFile, []byte(imageContent), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Open the file in subdirectory
	file, err := fs.Open("/images/photo.jpg")
	require.NoError(t, err)
	require.NotNil(t, file)
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, imageContent, string(content))
}

func TestNeuteredFileSystem_WithTrailingSlash_ReturnsError(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with a file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Create neutered filesystem
	fs := multifs.NewNeuteredFileSystem(http.Dir(tmpDir))

	// Try to open directory with trailing slash
	file, err := fs.Open("/")
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Nil(t, file)
}
