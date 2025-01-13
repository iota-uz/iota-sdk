// Package multifs MultiHashFS combines multiple hashfs instances to serve files from each.
package multifs

import (
	"github.com/benbjohnson/hashfs"
	"net/http"
	"os"
)

type MultiHashFS struct {
	instances []http.FileSystem
}

// New creates a new MultiHashFS instance and converts each hashfs.FS to an http.FileSystem.
func New(instances ...*hashfs.FS) *MultiHashFS {
	fileSystems := make([]http.FileSystem, len(instances))
	for i, fs := range instances {
		fileSystems[i] = http.FS(fs) // Convert hashfs.FS to http.FileSystem
	}
	return &MultiHashFS{instances: fileSystems}
}

// Open attempts to open a file from any of the hashfs instances.
func (m *MultiHashFS) Open(name string) (http.File, error) {
	for _, fs := range m.instances {
		file, err := fs.Open(name)
		if err == nil {
			return file, nil
		}
	}
	return nil, os.ErrNotExist // Return os.ErrNotExist if the file is not found in any instance
}
