package handlers

import (
	"os"
	"path/filepath"
)

// GetProjectRoot attempts to find the project root directory
func GetProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for i := 0; i < 10; i++ { // Look up to 10 levels up
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root directory
		}
		dir = parent
	}

	// Fallback to current working directory if go.mod not found
	return cwd, nil
}