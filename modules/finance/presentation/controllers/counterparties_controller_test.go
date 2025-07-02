package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	CounterpartyBasePath = "/finance/counterparties"
)

func TestCounterpartiesController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Test Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
		counterparty.WithLegalAddress("123 Test Street"),
	)

	counterparty2 := counterparty.New(
		"Test Vendor",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(env.Tenant.ID),
		counterparty.WithLegalAddress("456 Business Ave"),
	)

	_, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, counterparty2)
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
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"HTMX Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	suite.GET(CounterpartyBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Counterparty")
}

func TestCounterpartiesController_GetNew_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
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
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "New Test Counterparty")
	formData.Set("Type", "CUSTOMER")
	formData.Set("LegalType", "INDIVIDUAL")
	formData.Set("LegalAddress", "789 New Street")

	suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	counterparties, err := service.GetAll(env.Ctx)
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
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Type", "CUSTOMER")
	formData.Set("LegalType", "INDIVIDUAL")
	formData.Set("LegalAddress", "")

	response := suite.POST(CounterpartyBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	counterparties, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, counterparties)
}

func TestCounterpartiesController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Edit Test Counterparty",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(env.Tenant.ID),
		counterparty.WithLegalAddress("Edit Street 123"),
	)

	createdCounterparty, err := service.Create(env.Ctx, counterparty1)
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
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", CounterpartyBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestCounterpartiesController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Original Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
		counterparty.WithLegalAddress("Original Address"),
	)

	createdCounterparty, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Updated Counterparty Name")
	formData.Set("Type", "SUPPLIER")
	formData.Set("LegalType", "LEGAL_ENTITY")
	formData.Set("LegalAddress", "Updated Address")

	suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	updatedCounterparty, err := service.GetByID(env.Ctx, createdCounterparty.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Counterparty Name", updatedCounterparty.Name())
	require.Equal(t, counterparty.Supplier, updatedCounterparty.Type())
	require.Equal(t, counterparty.LegalEntity, updatedCounterparty.LegalType())
	require.Equal(t, "Updated Address", updatedCounterparty.LegalAddress())
}

func TestCounterpartiesController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Type", "CUSTOMER")
	formData.Set("LegalType", "INDIVIDUAL")
	formData.Set("LegalAddress", "")

	response := suite.POST(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedCounterparty, err := service.GetByID(env.Ctx, createdCounterparty.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Counterparty", unchangedCounterparty.Name())
}

func TestCounterpartiesController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Counterparty to Delete",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	existingCounterparty, err := service.GetByID(env.Ctx, createdCounterparty.ID())
	require.NoError(t, err)
	require.Equal(t, "Counterparty to Delete", existingCounterparty.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", CounterpartyBasePath, createdCounterparty.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(CounterpartyBasePath)

	_, err = service.GetByID(env.Ctx, createdCounterparty.ID())
	require.Error(t, err)
}

func TestCounterpartiesController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", CounterpartyBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestCounterpartiesController_Search_Success(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	counterparty1 := counterparty.New(
		"Searchable Customer",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	counterparty2 := counterparty.New(
		"Another Vendor",
		counterparty.Supplier,
		counterparty.LegalEntity,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, counterparty1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, counterparty2)
	require.NoError(t, err)

	response := suite.GET(CounterpartyBasePath + "/search?q=Searchable").
		Expect(t).
		Status(200)

	response.Contains("Searchable Customer")
}

func TestCounterpartiesController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewCounterpartiesController(env.App)
	suite.Register(controller)

	suite.GET(CounterpartyBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}
