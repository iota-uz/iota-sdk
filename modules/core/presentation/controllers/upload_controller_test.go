package controllers_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestUploadController_DirectoryListing_Returns404(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a test file in the uploads directory
	uploadsPath := filepath.Join("static")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(uploadsPath, "test.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Test that directory access returns 404
	suite.GET("/static/").Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileAccess_ReturnsFile(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a test file in the uploads directory
	uploadsPath := filepath.Join("static")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)

	testContent := "test file content"
	testFile := filepath.Join(uploadsPath, "document.txt")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)
	defer os.Remove(testFile)

	// Test that file access works
	response := suite.GET("/static/document.txt").Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), testContent)
}

func TestUploadController_SubdirectoryListing_Returns404(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	subDir := filepath.Join("static", "images")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(subDir)

	testFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)

	// Test that subdirectory access returns 404
	suite.GET("/static/images/").Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileInSubdirectory_ReturnsFile(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	subDir := filepath.Join("static", "images")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)
	defer os.RemoveAll(subDir)

	imageContent := "fake image data"
	imageFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(imageFile, []byte(imageContent), 0644)
	require.NoError(t, err)

	// Test that file access works
	response := suite.GET("/static/images/photo.jpg").Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), imageContent)
}

func TestUploadController_NonExistentFile_Returns404(t *testing.T) {
	t.Parallel()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Test that non-existent file returns 404
	suite.GET("/static/nonexistent.txt").Expect(t).Status(http.StatusNotFound)
}
