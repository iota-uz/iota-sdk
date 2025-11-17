package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/require"
)

var (
	CounterpartyBasePath = "/finance/counterparties"
)

func TestCounterpartiesController_List_Success(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
		counterparty.WithLegalAddress("123 Test Street"),
	)

	counterparty2 := counterparty.New(
		"Test Vendor",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
		counterparty.WithLegalAddress("456 Business Ave"),
	)

	_, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)
	_, err = service.Create(suite.Env().Ctx, counterparty2)
	require.NoError(t, err)

	response := suite.GET(CounterpartyBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Customer").
		Contains("Test Vendor")
}

func TestCounterpartiesController_List_HTMX_Request(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"HTMX Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	_, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	suite.GET(CounterpartyBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Counterparty")
}

func TestCounterpartiesController_GetNew_Success(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	response := suite.GET(CounterpartyBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Name']").Exists()
	html.Element("//select[@name='Type']").Exists()
	html.Element("//select[@name='LegalType']").Exists()
	html.Element("//input[@name='TIN']").Exists()
	html.Element("//textarea[@name='LegalAddress']").Exists()
}

func TestCounterpartiesController_Create_Success(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "New Test Counterparty")
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "789 New Street")

	suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	counterparties, err := service.GetAll(suite.Env().Ctx)
	require.NoError(t, err)
	require.Len(t, counterparties, 1)

	savedCounterparty := counterparties[0]
	require.Equal(t, "New Test Counterparty", savedCounterparty.Name())
	require.Equal(t, counterparty.Customer, savedCounterparty.Type())
	require.Equal(t, counterparty.Individual, savedCounterparty.LegalType())
	require.Equal(t, "789 New Street", savedCounterparty.LegalAddress())
}

func TestCounterpartiesController_Create_ValidationError(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "")

	response := suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	counterparties, err := service.GetAll(suite.Env().Ctx)
	require.NoError(t, err)
	require.Empty(t, counterparties)
}

func TestCounterpartiesController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Edit Test Counterparty",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
		counterparty.WithLegalAddress("Edit Street 123"),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Name']").Exists()
	require.Equal(t, "Edit Test Counterparty", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//select[@name='Type']").Exists()
	html.Element("//select[@name='LegalType']").Exists()
	html.Element("//textarea[@name='LegalAddress']").Exists()
	require.Equal(t, "Edit Street 123", html.Element("//textarea[@name='LegalAddress']").Text())
}

func TestCounterpartiesController_GetEdit_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", CounterpartyBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestCounterpartiesController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Original Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
		counterparty.WithLegalAddress("Original Address"),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Updated Counterparty Name")
	formData.Set("Type", string(counterparty.Supplier))
	formData.Set("LegalType", string(counterparty.LegalEntity))
	formData.Set("LegalAddress", "Updated Address")

	suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	updatedCounterparty, err := service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Counterparty Name", updatedCounterparty.Name())
	require.Equal(t, counterparty.Supplier, updatedCounterparty.Type())
	require.Equal(t, counterparty.LegalEntity, updatedCounterparty.LegalType())
	require.Equal(t, "Updated Address", updatedCounterparty.LegalAddress())
}

func TestCounterpartiesController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "")

	response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedCounterparty, err := service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Counterparty", unchangedCounterparty.Name())
}

func TestCounterpartiesController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Counterparty to Delete",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	existingCounterparty, err := service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
	require.NoError(t, err)
	require.Equal(t, "Counterparty to Delete", existingCounterparty.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	_, err = service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
	require.Error(t, err)
}

func TestCounterpartiesController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", CounterpartyBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestCounterpartiesController_Search_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Searchable Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	counterparty2 := counterparty.New(
		"Another Vendor",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	_, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)
	_, err = service.Create(suite.Env().Ctx, counterparty2)
	require.NoError(t, err)

	response := suite.GET(CounterpartyBasePath + "/search?q=Searchable").
		Expect(t).
		Status(200)

	response.Contains("Searchable Customer")
}

func TestCounterpartiesController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	suite.GET(CounterpartyBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}

func TestCounterpartiesController_Create_InvalidTINValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "Test Company")
	formData.Set("TIN", "invalid-tin") // Invalid TIN format
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "Test Address")

	response := suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(200) // Should return 200 with validation errors, not 500

	html := response.HTML()
	// Check that TIN field has validation error
	require.True(t, html.HasErrorFor("TIN"), "Expected TIN validation error to be displayed")

	// Verify no counterparty was created
	counterparties, err := service.GetAll(suite.Env().Ctx)
	require.NoError(t, err)
	require.Empty(t, counterparties)
}

func TestCounterpartiesController_Update_InvalidTINValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewModule()).
		AsUser(adminUser)

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create a counterparty first
	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Updated Company")
	formData.Set("TIN", "12345") // Invalid TIN format (too short)
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "Updated Address")

	response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(200) // Should return 200 with validation errors, not 500

	html := response.HTML()
	// Check that TIN field has validation error
	require.True(t, html.HasErrorFor("TIN"), "Expected TIN validation error to be displayed")

	// Verify counterparty was not updated
	unchangedCounterparty, err := service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Counterparty", unchangedCounterparty.Name())
}

// =====================================================================================
// NEW COMPREHENSIVE TDD TESTS FOR ISSUE #448 - TIN FIELD CLEARING ON VALIDATION ERRORS
// =====================================================================================

// TestCreate_ValidationError_PreservesFormData is the KEY TEST for issue #448
// This test exposes the bug where TIN field value is cleared when validation errors occur
func TestCreate_ValidationError_PreservesFormData(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	testCases := []struct {
		name             string
		invalidTIN       string
		validationPasses bool
		description      string
	}{
		{
			name:             "Invalid TIN - Wrong Length",
			invalidTIN:       "12345",
			validationPasses: false,
			description:      "TIN too short (5 digits instead of 9)",
		},
		{
			name:             "Invalid TIN - Non Numeric",
			invalidTIN:       "ABC123DEF",
			validationPasses: false,
			description:      "TIN contains letters",
		},
		{
			name:             "Invalid TIN - Special Characters",
			invalidTIN:       "123-45-678",
			validationPasses: false,
			description:      "TIN contains hyphens",
		},
		{
			name:             "Valid TIN Should Pass",
			invalidTIN:       "123456789",
			validationPasses: true,
			description:      "Valid 9-digit TIN should pass validation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{}
			formData.Set("Name", "Test Company")
			formData.Set("TIN", tc.invalidTIN)
			formData.Set("Type", string(counterparty.Customer))
			formData.Set("LegalType", string(counterparty.Individual))
			formData.Set("LegalAddress", "Test Address 123")

			response := suite.POST(CounterpartyBasePath).
				Form(formData).
				Expect(t)

			if tc.validationPasses {
				// Valid case should redirect successfully
				response.Status(302).RedirectTo(CounterpartyBasePath)
			} else {
				// Invalid case should return form with errors
				response.Status(200)

				html := response.HTML()

				// CRITICAL ASSERTION: TIN field should retain the submitted invalid value
				// This is the main bug - currently TIN field gets cleared on validation errors
				tinInput := html.Element("//input[@name='TIN']")
				tinInput.Exists()
				actualTINValue := tinInput.Attr("value")
				require.Equal(t, tc.invalidTIN, actualTINValue,
					"TIN field should preserve user's submitted value '%s' on validation error, but got '%s'. %s",
					tc.invalidTIN, actualTINValue, tc.description)

				// Verify other form fields are also preserved
				nameInput := html.Element("//input[@name='Name']")
				nameInput.Exists()
				require.Equal(t, "Test Company", nameInput.Attr("value"),
					"Name field should be preserved on validation error")

				addressTextarea := html.Element("//textarea[@name='LegalAddress']")
				addressTextarea.Exists()
				require.Equal(t, "Test Address 123", addressTextarea.Text(),
					"Legal Address field should be preserved on validation error")

				// Verify TIN error message is displayed
				require.True(t, html.HasErrorFor("TIN"),
					"TIN validation error should be displayed for input: %s", tc.invalidTIN)
			}
		})
	}
}

// TestCreate_MultipleValidationErrors_PreservesAllFields tests form preservation with multiple validation errors
func TestCreate_MultipleValidationErrors_PreservesAllFields(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	// Submit form with multiple validation errors
	formData := url.Values{}
	formData.Set("Name", "A")                                  // Too short (min 2 chars required)
	formData.Set("TIN", "invalid-tin-format")                  // Invalid TIN format
	formData.Set("Type", string(counterparty.Customer))        // Valid type to avoid 500 error
	formData.Set("LegalType", string(counterparty.Individual)) // Valid legal type to avoid 500 error
	formData.Set("LegalAddress", "Valid Address")              // This field should be valid

	response := suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(200) // Should return form with validation errors

	html := response.HTML()

	// CRITICAL: All field values should be preserved, even invalid ones
	nameInput := html.Element("//input[@name='Name']")
	nameInput.Exists()
	require.Equal(t, "A", nameInput.Attr("value"),
		"Name field should preserve invalid short value")

	tinInput := html.Element("//input[@name='TIN']")
	tinInput.Exists()
	require.Equal(t, "invalid-tin-format", tinInput.Attr("value"),
		"TIN field should preserve invalid format value - this is the main bug being tested")

	addressTextarea := html.Element("//textarea[@name='LegalAddress']")
	addressTextarea.Exists()
	require.Equal(t, "Valid Address", addressTextarea.Text(),
		"Legal Address field should preserve valid value")

	// Verify multiple validation errors are displayed
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"),
		"Multiple validation errors should be displayed")

	// Verify TIN error specifically exists
	require.True(t, html.HasErrorFor("TIN"),
		"TIN validation error should be displayed")
}

// TestUpdate_ValidationError_PreservesFormData is the KEY TEST for issue #448 in Update scenario
func TestUpdate_ValidationError_PreservesFormData(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create a counterparty with valid TIN first
	existingCounterparty := counterparty.New(
		"Original Company",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
		counterparty.WithLegalAddress("Original Address"),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, existingCounterparty)
	require.NoError(t, err)

	testCases := []struct {
		name        string
		invalidTIN  string
		description string
	}{
		{
			name:        "Invalid TIN - Too Short",
			invalidTIN:  "123",
			description: "TIN with only 3 digits",
		},
		{
			name:        "Invalid TIN - Contains Letters",
			invalidTIN:  "12A34B567",
			description: "TIN with mixed numbers and letters",
		},
		{
			name:        "Invalid TIN - Too Long",
			invalidTIN:  "1234567890123",
			description: "TIN with 13 digits (too long)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formData := url.Values{}
			formData.Set("Name", "Updated Company Name")
			formData.Set("TIN", tc.invalidTIN)
			formData.Set("Type", string(counterparty.Supplier))
			formData.Set("LegalType", string(counterparty.LegalEntity))
			formData.Set("LegalAddress", "Updated Address 456")

			response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
				Form(formData).
				Expect(t).
				Status(200) // Should return form with validation errors

			html := response.HTML()

			// CRITICAL ASSERTION: TIN field should show submitted value, NOT the stored/original value
			// This is the core issue - currently it shows empty or stored value instead of user input
			tinInput := html.Element("//input[@name='TIN']")
			tinInput.Exists()
			actualTINValue := tinInput.Attr("value")

			// The bug: actualTINValue will be empty or show stored value instead of tc.invalidTIN
			require.Equal(t, tc.invalidTIN, actualTINValue,
				"TIN field should preserve user's submitted invalid value '%s', but got '%s'. "+
					"This is the main bug: on validation errors, TIN field shows stored value instead of user input. %s",
				tc.invalidTIN, actualTINValue, tc.description)

			// Verify other submitted values are preserved (not stored values)
			nameInput := html.Element("//input[@name='Name']")
			nameInput.Exists()
			require.Equal(t, "Updated Company Name", nameInput.Attr("value"),
				"Name field should show submitted value, not stored value")

			addressTextarea := html.Element("//textarea[@name='LegalAddress']")
			addressTextarea.Exists()
			require.Equal(t, "Updated Address 456", addressTextarea.Text(),
				"Legal Address field should show submitted value, not stored value")

			// Verify TIN validation error is displayed
			require.True(t, html.HasErrorFor("TIN"),
				"TIN validation error should be displayed for input: %s", tc.invalidTIN)

			// Verify the original entity was NOT modified in database
			unchangedEntity, err := service.GetByID(suite.Env().Ctx, createdCounterparty.ID())
			require.NoError(t, err)
			require.Equal(t, "Original Company", unchangedEntity.Name(),
				"Original entity should remain unchanged when validation fails")
		})
	}
}

// TestUpdate_ValidTINWithOtherValidationErrors_PreservesUserInput tests edge case scenarios
func TestUpdate_ValidTINWithOtherValidationErrors_PreservesUserInput(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create counterparty with original TIN
	existingCounterparty := counterparty.New(
		"Test Company",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, existingCounterparty)
	require.NoError(t, err)

	// Submit valid TIN but invalid other fields
	formData := url.Values{}
	formData.Set("Name", "")                                   // Invalid - required field
	formData.Set("TIN", "987654321")                           // Valid TIN format
	formData.Set("Type", string(counterparty.Customer))        // Valid
	formData.Set("LegalType", string(counterparty.Individual)) // Valid
	formData.Set("LegalAddress", "New Address")                // Valid

	response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(200) // Should return form with validation errors

	html := response.HTML()

	// Even with valid TIN, the submitted value should be preserved
	tinInput := html.Element("//input[@name='TIN']")
	tinInput.Exists()
	require.Equal(t, "987654321", tinInput.Attr("value"),
		"Valid TIN input should be preserved even when other fields have validation errors")

	// Name should show empty value (what user submitted), not original value
	nameInput := html.Element("//input[@name='Name']")
	nameInput.Exists()
	require.Empty(t, nameInput.Attr("value"),
		"Name field should show submitted empty value, not stored value")

	// Address should show submitted value
	addressTextarea := html.Element("//textarea[@name='LegalAddress']")
	addressTextarea.Exists()
	require.Equal(t, "New Address", addressTextarea.Text(),
		"Address field should show submitted value")

	// Should have validation error for Name but not TIN
	require.False(t, html.HasErrorFor("TIN"),
		"Valid TIN should not have validation error")
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"),
		"Should have validation errors for other fields")
}

// TestCreate_EmptyTIN_ShouldBeAllowed tests that empty TIN is valid
func TestCreate_EmptyTIN_ShouldBeAllowed(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "Company Without TIN")
	formData.Set("TIN", "") // Empty TIN should be allowed
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "Test Address")

	suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(302). // Should succeed
		RedirectTo(CounterpartyBasePath)

	// Verify counterparty was created successfully
	counterparties, err := service.GetAll(suite.Env().Ctx)
	require.NoError(t, err)
	require.Len(t, counterparties, 1)

	savedCounterparty := counterparties[0]
	require.Equal(t, "Company Without TIN", savedCounterparty.Name())
	// Empty TIN is allowed and creates a TIN object with empty value (not nil)
	if savedCounterparty.Tin() != nil {
		require.Empty(t, savedCounterparty.Tin().Value(), "Empty TIN should have empty value")
	}
}

// TestCreate_HTMXRequest_ValidationError_PreservesTINField tests HTMX-specific behavior
func TestCreate_HTMXRequest_ValidationError_PreservesTINField(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	invalidTIN := "HTMX-INVALID-TIN"

	formData := url.Values{}
	formData.Set("Name", "HTMX Test Company")
	formData.Set("TIN", invalidTIN)
	formData.Set("Type", string(counterparty.Customer))
	formData.Set("LegalType", string(counterparty.Individual))
	formData.Set("LegalAddress", "HTMX Address")

	response := suite.POST(CounterpartyBasePath).
		Form(formData).
		HTMX(). // Add HTMX headers
		Expect(t).
		Status(200) // Should return form with errors

	html := response.HTML()

	// HTMX requests should also preserve TIN field value
	tinInput := html.Element("//input[@name='TIN']")
	tinInput.Exists()
	require.Equal(t, invalidTIN, tinInput.Attr("value"),
		"HTMX requests should also preserve TIN field value on validation errors")

	require.True(t, html.HasErrorFor("TIN"),
		"HTMX requests should display TIN validation errors")
}

// TestUpdate_HTMXRequest_ValidationError_PreservesTINField tests HTMX update behavior
func TestUpdate_HTMXRequest_ValidationError_PreservesTINField(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).
		WithModules(core.NewModule(&core.ModuleOptions{
			PermissionSchema: defaults.PermissionSchema(),
		}), finance.NewModule()).
		AsAdmin().
		Build()

	controller := controllers.NewCounterpartiesController(suite.Env().App)
	suite.Register(controller)

	service := suite.Env().App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create existing counterparty
	existingCounterparty := counterparty.New(
		"HTMX Update Test",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(suite.Env().Tenant.ID),
	)

	createdCounterparty, err := service.Create(suite.Env().Ctx, existingCounterparty)
	require.NoError(t, err)

	invalidTIN := "HTMX-UPDATE-INVALID"

	formData := url.Values{}
	formData.Set("Name", "HTMX Updated Company")
	formData.Set("TIN", invalidTIN)
	formData.Set("Type", string(counterparty.Supplier))
	formData.Set("LegalType", string(counterparty.LegalEntity))
	formData.Set("LegalAddress", "HTMX Updated Address")

	response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		HTMX(). // Add HTMX headers
		Expect(t).
		Status(200) // Should return form with errors

	html := response.HTML()

	// HTMX update requests should preserve user input, not stored values
	tinInput := html.Element("//input[@name='TIN']")
	tinInput.Exists()
	require.Equal(t, invalidTIN, tinInput.Attr("value"),
		"HTMX update requests should preserve submitted TIN value on validation errors")

	nameInput := html.Element("//input[@name='Name']")
	nameInput.Exists()
	require.Equal(t, "HTMX Updated Company", nameInput.Attr("value"),
		"HTMX update requests should preserve submitted Name value")

	require.True(t, html.HasErrorFor("TIN"),
		"HTMX update requests should display TIN validation errors")
}
