package handlers

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetProjectRoot(t *testing.T) {
	t.Run("Find project root", func(t *testing.T) {
		root, err := GetProjectRoot()
		assert.NoError(t, err)
		assert.NotEmpty(t, root)
		
		// Verify that the returned path contains a go.mod file
		_, err = os.Stat(filepath.Join(root, "go.mod"))
		assert.NoError(t, err, "Project root should contain go.mod file")
	})
}