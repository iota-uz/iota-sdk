package controllers_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestStaticFilesController_DirectoryListing_Returns404(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.css")
	err := os.WriteFile(testFile, []byte("body { color: red; }"), 0644)
	require.NoError(t, err)

	// Create a hashfs instance
	fs := hashfs.NewFS(os.DirFS(tmpDir))

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewStaticFilesController([]*hashfs.FS{fs})
	suite.Register(controller)

	// Test that directory access returns 404
	response := suite.GET("/assets/").Expect(t).Status(http.StatusNotFound)
	require.NotEmpty(t, response.Body())
}

func TestStaticFilesController_FileAccess_ReturnsFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory with test files
	tmpDir := t.TempDir()
	testContent := "body { color: blue; }"
	testFile := filepath.Join(tmpDir, "style.css")
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create a hashfs instance
	fs := hashfs.NewFS(os.DirFS(tmpDir))

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewStaticFilesController([]*hashfs.FS{fs})
	suite.Register(controller)

	// Get the hashed filename
	hashedName := fs.HashName("style.css")

	// Test that file access works
	response := suite.GET("/assets/" + hashedName).Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), testContent)
	require.Equal(t, "public, max-age=3600", response.Header("Cache-Control"))
}

func TestStaticFilesController_NonExistentFile_Returns404(t *testing.T) {
	t.Parallel()

	// Create an empty temporary directory
	tmpDir := t.TempDir()

	// Create a hashfs instance
	fs := hashfs.NewFS(os.DirFS(tmpDir))

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewStaticFilesController([]*hashfs.FS{fs})
	suite.Register(controller)

	// Test that non-existent file returns 404
	suite.GET("/assets/nonexistent.css").Expect(t).Status(http.StatusNotFound)
}
