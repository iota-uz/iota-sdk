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
	paymentCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	PaymentCategoryBasePath = "/finance/payment-categories"
)

func TestPaymentCategoryController_List_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category1 := paymentCategoryEntity.New(
		"Office Supplies",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
		paymentCategoryEntity.WithDescription("Office supplies and equipment"),
	)

	category2 := paymentCategoryEntity.New(
		"Travel",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
		paymentCategoryEntity.WithDescription("Travel expenses"),
	)

	_, err := service.Create(env.Ctx, category1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, category2)
	require.NoError(t, err)

	response := suite.GET(PaymentCategoryBasePath).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains(t, "Office Supplies").
		Contains(t, "Travel")
}

func TestPaymentCategoryController_List_HTMX_Request(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"HTMX Test Category",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	suite.GET(PaymentCategoryBasePath).
		HTMX().
		Expect().
		Status(t, 200).
		Contains(t, "HTMX Test Category")
}

func TestPaymentCategoryController_GetNew_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	response := suite.GET(PaymentCategoryBasePath+"/new").
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//form[@hx-post]").Exists(t)
	html.Element("//input[@name='Name']").Exists(t)
	html.Element("//textarea[@name='Description']").Exists(t)
}

func TestPaymentCategoryController_Create_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	formData := url.Values{}
	formData.Set("Name", "New Test Category")
	formData.Set("Description", "New category description")

	suite.POST(PaymentCategoryBasePath).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentCategoryBasePath)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	savedCategory := categories[0]
	require.Equal(t, "New Test Category", savedCategory.Name())
	require.Equal(t, "New category description", savedCategory.Description())
}

func TestPaymentCategoryController_Create_ValidationError(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "Test description")

	response := suite.POST(PaymentCategoryBasePath).
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, categories)
}

func TestPaymentCategoryController_GetEdit_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"Edit Test Category",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
		paymentCategoryEntity.WithDescription("Category to edit"),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	createdCategory, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, createdCategory, 1)

	response := suite.GET(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, createdCategory[0].ID().String())).
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//input[@name='Name']").Exists(t)
	require.Equal(t, "Edit Test Category", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists(t)
	require.Equal(t, "Category to edit", html.Element("//textarea[@name='Description']").Text())
}

func TestPaymentCategoryController_GetEdit_NotFound(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestPaymentCategoryController_Update_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"Original Category",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
		paymentCategoryEntity.WithDescription("Original description"),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	createdCategory := categories[0]

	formData := url.Values{}
	formData.Set("Name", "Updated Category Name")
	formData.Set("Description", "Updated description")

	suite.POST(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, createdCategory.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentCategoryBasePath)

	updatedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Category Name", updatedCategory.Name())
	require.Equal(t, "Updated description", updatedCategory.Description())
}

func TestPaymentCategoryController_Update_ValidationError(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"Test Category",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	createdCategory := categories[0]

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "")

	response := suite.POST(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, createdCategory.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Category", unchangedCategory.Name())
}

func TestPaymentCategoryController_Delete_Success(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"Category to Delete",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	createdCategory := categories[0]

	existingCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)
	require.Equal(t, "Category to Delete", existingCategory.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, createdCategory.ID().String())).
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentCategoryBasePath)

	_, err = service.GetByID(env.Ctx, createdCategory.ID())
	require.Error(t, err)
}

func TestPaymentCategoryController_Delete_NotFound(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestPaymentCategoryController_InvalidUUID(t *testing.T) {
	adminUser := user.New(
		"Admin",
		"User",
		internet.MustParseEmail("admin@example.com"),
		user.UILanguageEN,
		user.WithID(1),
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.RegisterController(controller)

	suite.GET(PaymentCategoryBasePath+"/invalid-uuid").
		Expect().
		Status(t, 404)
}
