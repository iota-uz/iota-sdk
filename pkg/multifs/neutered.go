package multifs

import (
	"net/http"
	"os"
)

// NeuteredFileSystem wraps an http.FileSystem to disable directory listings.
// When a directory is requested, it returns os.ErrNotExist (404) to prevent
// exposing the file structure. Only files can be accessed directly by their path.
//
// This implementation prioritizes security by blocking all directory access,
// including directories with index.html files. If index.html serving is required,
// it should be explicitly handled at the application routing level.
type NeuteredFileSystem struct {
	fs http.FileSystem
}

// NewNeuteredFileSystem creates a new NeuteredFileSystem that wraps the given
// http.FileSystem and disables directory listings.
func NewNeuteredFileSystem(fs http.FileSystem) http.FileSystem {
	return &NeuteredFileSystem{fs: fs}
}

// Open opens the named file from the underlying filesystem.
// If the requested path is a directory, it returns os.ErrNotExist to prevent
// directory listing and returns 404 Not Found.
func (nfs *NeuteredFileSystem) Open(name string) (http.File, error) {
	file, err := nfs.fs.Open(name)
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}

	// Block all directory access for security
	if stat.IsDir() {
		file.Close()
		return nil, os.ErrNotExist
	}

	// It's a regular file, serve it normally
	return file, nil
}
