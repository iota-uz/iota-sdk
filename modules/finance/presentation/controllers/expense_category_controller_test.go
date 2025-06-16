package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/finance"
	expenseCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	ExpenseCategoryBasePath = "/finance/expense-categories"
)

func TestExpenseCategoryController_List_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
		permissions.ExpenseCategoryCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category1 := expenseCategoryEntity.New(
		"Office Supplies",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
		expenseCategoryEntity.WithDescription("Office supplies and equipment"),
	)

	category2 := expenseCategoryEntity.New(
		"Travel",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
		expenseCategoryEntity.WithDescription("Travel expenses"),
	)

	_, err := service.Create(env.Ctx, category1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, category2)
	require.NoError(t, err)

	response := suite.GET(ExpenseCategoryBasePath).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains(t, "Office Supplies").
		Contains(t, "Travel")
}

func TestExpenseCategoryController_List_HTMX_Request(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category := expenseCategoryEntity.New(
		"HTMX Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	suite.GET(ExpenseCategoryBasePath).
		HTMX().
		Expect().
		Status(t, 200).
		Contains(t, "HTMX Test Category")
}

func TestExpenseCategoryController_GetNew_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	response := suite.GET(ExpenseCategoryBasePath+"/new").
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//form[@hx-post]").Exists(t)
	html.Element("//input[@name='Name']").Exists(t)
	html.Element("//textarea[@name='Description']").Exists(t)
}

func TestExpenseCategoryController_Create_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	formData := url.Values{}
	formData.Set("Name", "New Test Category")
	formData.Set("Description", "New category description")

	suite.POST(ExpenseCategoryBasePath).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseCategoryBasePath)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, categories, 1)

	savedCategory := categories[0]
	require.Equal(t, "New Test Category", savedCategory.Name())
	require.Equal(t, "New category description", savedCategory.Description())
}

func TestExpenseCategoryController_Create_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "Test description")

	response := suite.POST(ExpenseCategoryBasePath).
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.Greater(t, len(html.Elements("//small[@data-testid='field-error']")), 0)

	categories, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, categories)
}

func TestExpenseCategoryController_GetEdit_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
		permissions.ExpenseCategoryUpdate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category := expenseCategoryEntity.New(
		"Edit Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
		expenseCategoryEntity.WithDescription("Category to edit"),
	)

	_, err := service.Create(env.Ctx, category)
	require.NoError(t, err)

	createdCategory, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, createdCategory, 1)

	response := suite.GET(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, createdCategory[0].ID().String())).
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//input[@name='Name']").Exists(t)
	require.Equal(t, "Edit Test Category", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists(t)
	require.Equal(t, "Category to edit", html.Element("//textarea[@name='Description']").Text())
}

func TestExpenseCategoryController_GetEdit_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestExpenseCategoryController_Update_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryUpdate,
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category := expenseCategoryEntity.New(
		"Original Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
		expenseCategoryEntity.WithDescription("Original description"),
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

	suite.POST(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, createdCategory.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseCategoryBasePath)

	updatedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Category Name", updatedCategory.Name())
	require.Equal(t, "Updated description", updatedCategory.Description())
}

func TestExpenseCategoryController_Update_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryUpdate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
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

	response := suite.POST(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, createdCategory.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.Greater(t, len(html.Elements("//small[@data-testid='field-error']")), 0)

	unchangedCategory, err := service.GetByID(env.Ctx, createdCategory.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Category", unchangedCategory.Name())
}

func TestExpenseCategoryController_Delete_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryDelete,
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.ExpenseCategoryService{}).(*services.ExpenseCategoryService)

	category := expenseCategoryEntity.New(
		"Category to Delete",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
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

	suite.DELETE(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, createdCategory.ID().String())).
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseCategoryBasePath)

	_, err = service.GetByID(env.Ctx, createdCategory.ID())
	require.Error(t, err)
}

func TestExpenseCategoryController_Delete_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryDelete,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", ExpenseCategoryBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestExpenseCategoryController_InvalidUUID(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCategoryRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()

	controller := controllers.NewExpenseCategoriesController(env.App)
	suite.RegisterController(controller)

	suite.GET(ExpenseCategoryBasePath+"/invalid-uuid").
		Expect().
		Status(t, 404)
}
