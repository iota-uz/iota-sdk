package controllers_test

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
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
	t.Cleanup(func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	})

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	controller := controllers.NewUploadController(uploadService, uploadsCfg)
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
	t.Cleanup(func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	})

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	controller := controllers.NewUploadController(uploadService, uploadsCfg)
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

// Falsely green if the fixture is absent or the request never reaches the public file handler.
func TestUploadController_PrivateFileAccess_Returns404(t *testing.T) {
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	suite.Register(controllers.NewUploadController(uploadService, uploadsCfg))

	cases := []struct {
		name        string
		fixtureRoot string
		urlPath     string
	}{
		{
			name:        "private namespace",
			fixtureRoot: filepath.Join(uploadsPath, uploadsconfig.PrivateDirectory, testSubDir),
			urlPath:     "/" + filepath.ToSlash(filepath.Join(uploadsPath, uploadsconfig.PrivateDirectory, testSubDir, "confidential.pdf")),
		},
		{
			name:        "adjacent URL prefix",
			fixtureRoot: filepath.Join(uploadsPath+"private", testSubDir),
			urlPath:     "/" + filepath.ToSlash(filepath.Join(uploadsPath+"private", testSubDir, "confidential.pdf")),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, os.MkdirAll(tc.fixtureRoot, 0700))
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(tc.fixtureRoot)) })
			filePath := filepath.Join(tc.fixtureRoot, "confidential.pdf")
			require.NoError(t, os.WriteFile(filePath, []byte("private evidence"), 0600))
			_, err := os.ReadFile(filePath)
			require.NoError(t, err)
			suite.GET(tc.urlPath).Expect(t).Status(http.StatusNotFound)
		})
	}
}

func TestUploadController_SubdirectoryListing_Returns404(t *testing.T) {
	// Use consistent uploads path (singleton configuration)
	uploadsPath := "test-uploads"
	testSubDir := strings.ReplaceAll(t.Name(), "/", "-")
	testPath := filepath.Join(uploadsPath, testSubDir)
	err := os.MkdirAll(testPath, 0755)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	})

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	controller := controllers.NewUploadController(uploadService, uploadsCfg)
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
	t.Cleanup(func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	})

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	controller := controllers.NewUploadController(uploadService, uploadsCfg)
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
	t.Cleanup(func() {
		err := os.RemoveAll(testPath)
		require.NoError(t, err)
	})

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	uploadService := itf.GetService[services.UploadService](suite.Environment())
	uploadsCfg := itf.GetService[uploadsconfig.Config](suite.Environment())
	controller := controllers.NewUploadController(uploadService, uploadsCfg)
	suite.Register(controller)

	// Test that non-existent file returns 404
	urlPath := "/" + uploadsPath + "/" + testSubDir + "/nonexistent.txt"
	suite.GET(urlPath).Expect(t).Status(http.StatusNotFound)
}

func TestUploadController_ProfileImage(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).Build()
	env := suite.Environment()
	email, err := internet.NewEmail("profile-image@example.test")
	require.NoError(t, err)
	person, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(env.Ctx, user.New("Profile", "Image", email, user.UILanguageEN, user.WithTenantID(env.Tenant.ID)))
	require.NoError(t, err)
	suite.AsUser(person)
	uploads := itf.GetService[services.UploadService](env)
	cfg := itf.GetService[uploadsconfig.Config](env)
	suite.Register(controllers.NewUploadController(uploads, cfg))

	var photo bytes.Buffer
	require.NoError(t, png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	note := []byte("An ordinary text attachment.")
	post := func(path string, data *itf.MultipartData, field string, status int) {
		t.Helper()
		suite.POST(path).MultipartData(data.AddField("_name", field)).Expect(t).Status(status)
	}
	post("/uploads", itf.NewMultipart().AddFile("file", "photo.png", photo.Bytes()).AddFile("file", "note.png", note), "AvatarID", http.StatusUnsupportedMediaType)
	post("/uploads/avatar", itf.NewMultipart().AddFile("file", "note.txt", note), "AttachmentID", http.StatusUnsupportedMediaType)
	stored, err := uploads.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, stored)

	post("/uploads", itf.NewMultipart().AddFile("file", "notes.txt", note), "AttachmentID", http.StatusOK)
	stored, err = uploads.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	users := itf.GetService[services.UserService](env)
	_, err = users.UpdateSelf(env.Ctx, env.User.SetAvatarID(stored[0].ID()))
	require.ErrorIs(t, err, upload.ErrAvatarType)

	post("/uploads/avatar", itf.NewMultipart().AddFile("file", "photo.PNG", photo.Bytes()), "AvatarID", http.StatusOK)
	stored, err = uploads.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	for _, file := range stored {
		if file.Name() == "photo.PNG" {
			u, err := users.UpdateSelf(env.Ctx, env.User.SetAvatarID(file.ID()))
			require.NoError(t, err)
			require.Equal(t, file.ID(), u.AvatarID())
			return
		}
	}
	t.Fatal("profile image was not stored")
}
