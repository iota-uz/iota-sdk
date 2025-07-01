package controllers_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1x1 transparent PNG
const PngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII="

func setupSettingsControllerTest(t *testing.T) (*controllertest.Suite, *services.TenantService, *services.UploadService) {
	t.Helper()
	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN))

	suite.WithMiddleware(func(ctx context.Context, r *http.Request) context.Context {
		props := sidebar.Props{
			TabGroups: sidebar.TabGroupCollection{
				Groups: []sidebar.TabGroup{
					{
						Label: "Core",
						Value: "core",
						Items: []sidebar.Item{},
					},
				},
			},
		}
		return context.WithValue(ctx, constants.SidebarPropsKey, props)
	})

	controller := controllers.NewSettingsController(suite.Environment().App)
	suite.Register(controller)

	tenantService := suite.Environment().App.Service(services.TenantService{}).(*services.TenantService)
	uploadService := suite.Environment().App.Service(services.UploadService{}).(*services.UploadService)

	return suite, tenantService, uploadService
}

func TestSettingsController_GetLogo(t *testing.T) {
	suite, tenantService, uploadService := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Create dummy upload files
	logoContent, err := base64.StdEncoding.DecodeString(PngBase64)
	require.NoError(t, err)

	logoUpload, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	logoCompactUpload, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "logo_compact.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	// Update tenant with logo IDs
	logoID := int(logoUpload.ID())
	logoCompactID := int(logoCompactUpload.ID())
	testTenant.SetLogoID(&logoID)
	testTenant.SetLogoCompactID(&logoCompactID)
	_, err = tenantService.Update(suite.Environment().Ctx, testTenant)
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	resp := suite.GET("/settings/logo").Expect(t).Status(http.StatusOK)

	resp.Contains("Logo settings")
	resp.Contains(logoUpload.Path())
	resp.Contains(logoCompactUpload.Path())
}

func TestSettingsController_PostLogo_Success(t *testing.T) {
	suite, tenantService, uploadService := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Create dummy upload files for new logos
	logoContent, err := base64.StdEncoding.DecodeString(PngBase64)
	require.NoError(t, err)

	newLogoUpload, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "new_logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	newLogoCompactUpload, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "new_logo_compact.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	formData := url.Values{
		"LogoID":        {fmt.Sprintf("%d", newLogoUpload.ID())},
		"LogoCompactID": {fmt.Sprintf("%d", newLogoCompactUpload.ID())},
	}

	resp := suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusOK) // Should return 200 OK with updated form

	resp.Contains(newLogoUpload.Path())
	resp.Contains(newLogoCompactUpload.Path())

	// Verify tenant was updated in the database
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Equal(t, int(newLogoUpload.ID()), *updatedTenant.LogoID())
	assert.Equal(t, int(newLogoCompactUpload.ID()), *updatedTenant.LogoCompactID())
}

func TestSettingsController_PostLogo_ValidationError(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	formData := url.Values{
		"LogoID": {"invalid"}, // Invalid ID
	}

	resp := suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusBadRequest) // Should return 400 Bad Request due to form parsing error

	resp.Contains("Invalid Integer Value") // Check for parsing error message

	// Verify tenant was NOT updated in the database
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
}

func TestSettingsController_PostLogo_FileUpload(t *testing.T) {
	suite, tenantService, uploadService := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Create a dummy file for upload using the upload service
	fileContent, err := base64.StdEncoding.DecodeString(PngBase64)
	require.NoError(t, err)
	fileName := "test_image.png"

	// Upload the file first
	uploadedLogo, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: fileName,
		Size: len(fileContent),
		File: bytes.NewReader(fileContent),
	})
	require.NoError(t, err)

	// Then update the tenant with the uploaded logo ID
	formData := url.Values{
		"LogoID": {fmt.Sprintf("%d", uploadedLogo.ID())},
	}

	suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusOK)

	// Verify the logo was uploaded and tenant updated
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.NotNil(t, updatedTenant.LogoID())
	assert.Equal(t, int(uploadedLogo.ID()), *updatedTenant.LogoID())

	// Clean up the uploaded file from disk (optional, but good practice for real tests)
	_ = os.Remove(filepath.Join("uploads", uploadedLogo.Path()))
}

// Edge case tests for potential 500 errors
func TestSettingsController_PostLogo_NonExistentUploadID(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Use a non-existent upload ID (should be high enough to not exist)
	formData := url.Values{
		"LogoID": {"999999"},
	}

	// The controller should validate upload existence and return 400 Bad Request
	resp := suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusBadRequest)

	// Should contain appropriate error message
	resp.Contains("Logo upload not found")

	// Verify tenant was NOT updated due to validation failure
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
}

func TestSettingsController_PostLogo_ZeroValues(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Use zero values (should be ignored by controller logic)
	formData := url.Values{
		"LogoID":        {"0"},
		"LogoCompactID": {"0"},
	}

	suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusOK)

	// Verify tenant was NOT updated (zero values should be ignored)
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
	assert.Nil(t, updatedTenant.LogoCompactID())
}

func TestSettingsController_PostLogo_WithExistingLogos(t *testing.T) {
	suite, tenantService, uploadService := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Create existing logos
	logoContent, err := base64.StdEncoding.DecodeString(PngBase64)
	require.NoError(t, err)

	existingLogo, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "existing_logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	existingCompactLogo, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "existing_compact_logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	// Set tenant with existing logos
	existingLogoID := int(existingLogo.ID())
	existingCompactLogoID := int(existingCompactLogo.ID())
	testTenant.SetLogoID(&existingLogoID)
	testTenant.SetLogoCompactID(&existingCompactLogoID)
	_, err = tenantService.Update(suite.Environment().Ctx, testTenant)
	require.NoError(t, err)

	// Create new logos to replace the existing ones
	newLogo, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "new_logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Replace existing logos with new ones
	formData := url.Values{
		"LogoID": {fmt.Sprintf("%d", newLogo.ID())},
		// Keep existing compact logo by not specifying LogoCompactID
	}

	suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusOK)

	// Verify tenant was updated correctly
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Equal(t, int(newLogo.ID()), *updatedTenant.LogoID())
	assert.Equal(t, existingCompactLogoID, *updatedTenant.LogoCompactID()) // Should remain unchanged
}

func TestSettingsController_PostLogo_ExtremelyLargeValues(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Use extremely large values that won't exist in the database
	formData := url.Values{
		"LogoID": {"9223372036854775807"}, // Max int64
	}

	// This should return 400 since the upload doesn't exist
	resp := suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusBadRequest)

	// Should contain upload not found error
	resp.Contains("Logo upload not found")

	// Verify tenant was not updated
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
}

func TestSettingsController_PostLogo_EmptyForm(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Submit completely empty form
	formData := url.Values{}

	suite.POST("/settings/logo").
		Form(formData).
		Expect(t).
		Status(http.StatusOK)

	// Verify tenant was not modified
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
	assert.Nil(t, updatedTenant.LogoCompactID())
}

func TestSettingsController_PostLogo_MalformedContentType(t *testing.T) {
	suite, tenantService, _ := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Send request with JSON content type but form data (should parse as empty form)
	// This actually succeeds because JSON parsing returns empty form values
	suite.POST("/settings/logo").
		Header("Content-Type", "application/json").
		JSON(map[string]interface{}{"LogoID": 1}).
		Expect(t).
		Status(http.StatusOK) // Empty form gets processed successfully

	// Verify tenant was not modified since no valid form data was parsed
	updatedTenant, err := tenantService.GetByID(suite.Environment().Ctx, testTenant.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedTenant.LogoID())
}

func TestSettingsController_GetLogo_WithNonExistentUploads(t *testing.T) {
	suite, tenantService, uploadService := setupSettingsControllerTest(t)

	// Create a test tenant
	testTenant, err := tenantService.Create(suite.Environment().Ctx, "Test Tenant", "test.com")
	require.NoError(t, err)

	// Create a valid upload first, then delete it to simulate missing upload
	logoContent, err := base64.StdEncoding.DecodeString(PngBase64)
	require.NoError(t, err)

	tempUpload, err := uploadService.Create(suite.Environment().Ctx, &upload.CreateDTO{
		Name: "temp_logo.png",
		Size: len(logoContent),
		File: bytes.NewReader(logoContent),
	})
	require.NoError(t, err)

	// Set tenant with the upload ID
	logoID := int(tempUpload.ID())
	testTenant.SetLogoID(&logoID)
	_, err = tenantService.Update(suite.Environment().Ctx, testTenant)
	require.NoError(t, err)

	// Now delete the upload to simulate missing file
	_, err = uploadService.Delete(suite.Environment().Ctx, tempUpload.ID())
	require.NoError(t, err)

	// Simulate user context for the request
	user := user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, user.WithTenantID(testTenant.ID()))
	suite.AsUser(user)

	// Should handle missing uploads gracefully (logoProps handles this)
	resp := suite.GET("/settings/logo").Expect(t).Status(http.StatusOK)

	// Page should still render without crashing
	resp.Contains("Logo settings")
}
