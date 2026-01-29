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
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testPath) })

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a test file in the uploads directory
	testFile := filepath.Join(testPath, "test.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)

	// Test that directory access returns 404
	urlPath := "/" + uploadsPath + "/"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileAccess_ReturnsFile(t *testing.T) {
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testPath) })

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
	testFile := filepath.Join(absUploadsPath, testSubDir, "document.txt")
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Test that file access works
	urlPath := "/" + uploadsPath + "/" + testSubDir + "/document.txt"
	response := suite.GET(urlPath).Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), testContent)
}

func TestUploadController_SubdirectoryListing_Returns404(t *testing.T) {
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testPath) })

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	subDir := filepath.Join(testPath, "images")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	testFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(testFile, []byte("fake image data"), 0644)
	require.NoError(t, err)

	// Test that subdirectory access returns 404
	urlPath := "/" + uploadsPath + "/" + testSubDir + "/images/"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_FileInSubdirectory_ReturnsFile(t *testing.T) {
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testPath) })

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Create a subdirectory with a file
	workDir, err := os.Getwd()
	require.NoError(t, err)

	absUploadsPath := filepath.Join(workDir, uploadsPath)
	subDir := filepath.Join(absUploadsPath, testSubDir, "images")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	imageContent := "fake image data"
	imageFile := filepath.Join(subDir, "photo.jpg")
	err = os.WriteFile(imageFile, []byte(imageContent), 0644)
	require.NoError(t, err)

	// Test that file access works
	urlPath := "/" + uploadsPath + "/" + testSubDir + "/images/photo.jpg"
	response := suite.GET(urlPath).Expect(t).Status(http.StatusOK)
	require.Contains(t, response.Body(), imageContent)
}

func TestUploadController_NonExistentFile_Returns404(t *testing.T) {
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(testPath) })

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}))
	controller := controllers.NewUploadController(suite.Environment().App)
	suite.Register(controller)

	// Test that non-existent file returns 404
	urlPath := "/" + uploadsPath + "/" + testSubDir + "/nonexistent.txt"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}
