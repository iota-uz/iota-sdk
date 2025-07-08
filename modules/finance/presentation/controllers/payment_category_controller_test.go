package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/finance"
	paymentCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

var (
	PaymentCategoryBasePath = "/finance/payment-categories"
)

func TestPaymentCategoryController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

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
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Office Supplies").
		Contains("Travel")
}

func TestPaymentCategoryController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	category := paymentCategoryEntity.New(
		"HTMX Test Category",
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	suite.GET(PaymentCategoryBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Category")
}

func TestPaymentCategoryController_GetNew_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	response := suite.GET(PaymentCategoryBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Name']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
}

func TestPaymentCategoryController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	formData := url.Values{}
	formData.Set("Name", "New Test Category")
	formData.Set("Description", "New category description")

	suite.POST(PaymentCategoryBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentCategoryBasePath)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	savedCategory := categories[0]
	require.Equal(t, "New Test Category", savedCategory.Name())
	require.Equal(t, "New category description", savedCategory.Description())
}

func TestPaymentCategoryController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "Test description")

	response := suite.POST(PaymentCategoryBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, categories)
}

func TestPaymentCategoryController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

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
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Name']").Exists()
	require.Equal(t, "Edit Test Category", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists()
	require.Equal(t, "Category to edit", html.Element("//textarea[@name='Description']").Text())
}

func TestPaymentCategoryController_GetEdit_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestPaymentCategoryController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

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
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentCategoryBasePath)

	updatedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Category Name", updatedCategory.Name())
	require.Equal(t, "Updated description", updatedCategory.Description())
}

func TestPaymentCategoryController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

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
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Category", unchangedCategory.Name())
}

func TestPaymentCategoryController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

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
		Expect(t).
		Status(302).
		RedirectTo(PaymentCategoryBasePath)

	_, err = service.GetByID(env.Ctx, createdCategory.ID())
	require.Error(t, err)
}

func TestPaymentCategoryController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", PaymentCategoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestPaymentCategoryController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	controller := controllers.NewPaymentCategoriesController(env.App)
	suite.Register(controller)

	suite.GET(PaymentCategoryBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}
