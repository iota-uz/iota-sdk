package controllers_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestUploadController_DirectoryListing_Returns404(t *testing.T) {
	// Create isolated temp directory for this test within working directory
	// Use unique name based on test name to prevent conflicts
	uploadsPath := "test-uploads-" + strings.ReplaceAll(t.Name(), "/", "-")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(uploadsPath) })

	t.Setenv("UPLOADS_PATH", uploadsPath)

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a test file in the uploads directory
	testFile := filepath.Join(uploadsPath, "test.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)

	// Test that directory access returns 404
	urlPath := "/" + uploadsPath + "/"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileAccess_ReturnsFile(t *testing.T) {
	// Create isolated temp directory for this test within working directory
	uploadsPath := "test-uploads-" + strings.ReplaceAll(t.Name(), "/", "-")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(uploadsPath) })

	t.Setenv("UPLOADS_PATH", uploadsPath)

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a test file in the uploads directory
	testContent := "test file content"

	// Get absolute path to ensure file is created in the right location
	workDir, err := os.Getwd()
	require.NoError(t, err)

	absUploadsPath := filepath.Join(workDir, uploadsPath)
	testFile := filepath.Join(absUploadsPath, "document.txt")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Test that file access works
	urlPath := "/" + uploadsPath + "/document.txt"
	response := suite.GET(urlPath).Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), testContent)
}

func TestUploadController_SubdirectoryListing_Returns404(t *testing.T) {
	// Create isolated temp directory for this test within working directory
	uploadsPath := "test-uploads-" + strings.ReplaceAll(t.Name(), "/", "-")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(uploadsPath) })

	t.Setenv("UPLOADS_PATH", uploadsPath)

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	subDir := filepath.Join(uploadsPath, "images")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)

	// Test that subdirectory access returns 404
	urlPath := "/" + uploadsPath + "/images/"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileInSubdirectory_ReturnsFile(t *testing.T) {
	// Create isolated temp directory for this test within working directory
	uploadsPath := "test-uploads-" + strings.ReplaceAll(t.Name(), "/", "-")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(uploadsPath) })

	t.Setenv("UPLOADS_PATH", uploadsPath)

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	workDir, err := os.Getwd()
	require.NoError(t, err)

	absUploadsPath := filepath.Join(workDir, uploadsPath)
	subDir := filepath.Join(absUploadsPath, "images")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	imageContent := "fake image data"
	imageFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(imageFile, []byte(imageContent), 0644)
	require.NoError(t, err)

	// Test that file access works
	urlPath := "/" + uploadsPath + "/images/photo.jpg"
	response := suite.GET(urlPath).Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), imageContent)
}

func TestUploadController_NonExistentFile_Returns404(t *testing.T) {
	// Create isolated temp directory for this test within working directory
	uploadsPath := "test-uploads-" + strings.ReplaceAll(t.Name(), "/", "-")
	err := os.MkdirAll(uploadsPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(uploadsPath) })

	t.Setenv("UPLOADS_PATH", uploadsPath)

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Test that non-existent file returns 404
	urlPath := "/" + uploadsPath + "/nonexistent.txt"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}
