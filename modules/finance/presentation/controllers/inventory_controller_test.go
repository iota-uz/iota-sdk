package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

var (
	InventoryBasePath = "/finance/inventory"
)

func TestInventoryController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item1 := inventory.New(
		"Test Product 1",
		money.NewFromFloat(15.99, "USD"),
		100,
		inventory.WithDescription("Test product 1 description"),
	)

	item2 := inventory.New(
		"Test Product 2",
		money.NewFromFloat(25.50, "EUR"),
		50,
		inventory.WithDescription("Test product 2 description"),
	)

	_, err := service.Create(env.Ctx, item1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, item2)
	require.NoError(t, err)

	response := suite.GET(InventoryBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Product 1").
		Contains("Test Product 2")
}

func TestInventoryController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item := inventory.New(
		"HTMX Test Product",
		money.NewFromFloat(10.00, "USD"),
		25,
		inventory.WithDescription("HTMX test product"),
	)

	_, err := service.Create(env.Ctx, item)
	require.NoError(t, err)

	suite.GET(InventoryBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Product")
}

func TestInventoryController_GetNew_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	response := suite.GET(InventoryBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Name']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
	html.Element("//select[@name='CurrencyCode']").Exists()
	html.Element("//input[@name='Price']").Exists()
	html.Element("//input[@name='Quantity']").Exists()
	html.Element("//option[@value='USD']").Exists()
	html.Element("//option[@value='EUR']").Exists()
}

func TestInventoryController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	formData := url.Values{}
	formData.Set("Name", "New Test Product")
	formData.Set("Description", "New product description")
	formData.Set("CurrencyCode", "USD")
	formData.Set("Price", "29.99")
	formData.Set("Quantity", "150")

	suite.POST(InventoryBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(InventoryBasePath)

	items, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)

	savedItem := items[0]
	require.Equal(t, "New Test Product", savedItem.Name())
	require.Equal(t, "New product description", savedItem.Description())
	require.Equal(t, "USD", savedItem.Price().Currency().Code)
	require.Equal(t, int64(2999), savedItem.Price().Amount())
	require.Equal(t, 150, savedItem.Quantity())
}

func TestInventoryController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "")
	formData.Set("CurrencyCode", "USD")
	formData.Set("Price", "-10")
	formData.Set("Quantity", "-5")

	response := suite.POST(InventoryBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	items, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestInventoryController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item := inventory.New(
		"Edit Test Product",
		money.NewFromFloat(19.99, "USD"),
		75,
		inventory.WithDescription("Product to edit"),
	)

	createdItem, err := service.Create(env.Ctx, item)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s", InventoryBasePath, createdItem.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Name']").Exists()
	require.Equal(t, "Edit Test Product", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists()
	require.Equal(t, "Product to edit", html.Element("//textarea[@name='Description']").Text())

	html.Element("//select[@name='CurrencyCode']").Exists()
	html.Element("//input[@name='Price']").Exists()
	html.Element("//input[@name='Quantity']").Exists()
}

func TestInventoryController_GetEdit_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", InventoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestInventoryController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item := inventory.New(
		"Original Product",
		money.NewFromFloat(10.00, "USD"),
		100,
		inventory.WithDescription("Original description"),
	)

	createdItem, err := service.Create(env.Ctx, item)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "Updated Product Name")
	formData.Set("Description", "Updated product description")
	formData.Set("CurrencyCode", "EUR")
	formData.Set("Price", "35.75")
	formData.Set("Quantity", "200")

	suite.POST(fmt.Sprintf("%s/%s", InventoryBasePath, createdItem.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(InventoryBasePath)

	updatedItem, err := service.GetByID(env.Ctx, createdItem.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Product Name", updatedItem.Name())
	require.Equal(t, "Updated product description", updatedItem.Description())
	require.Equal(t, "EUR", updatedItem.Price().Currency().Code)
	require.Equal(t, int64(3575), updatedItem.Price().Amount())
	require.Equal(t, 200, updatedItem.Quantity())
}

func TestInventoryController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item := inventory.New(
		"Test Product",
		money.NewFromFloat(10.00, "USD"),
		50,
		inventory.WithDescription("Test description"),
	)

	createdItem, err := service.Create(env.Ctx, item)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Description", "")
	formData.Set("CurrencyCode", "USD")
	formData.Set("Price", "-25")
	formData.Set("Quantity", "-10")

	response := suite.POST(fmt.Sprintf("%s/%s", InventoryBasePath, createdItem.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedItem, err := service.GetByID(env.Ctx, createdItem.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Product", unchangedItem.Name())
}

func TestInventoryController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.InventoryService{}).(*services.InventoryService)

	item := inventory.New(
		"Product to Delete",
		money.NewFromFloat(15.00, "USD"),
		30,
		inventory.WithDescription("Product to be deleted"),
	)

	createdItem, err := service.Create(env.Ctx, item)
	require.NoError(t, err)

	existingItem, err := service.GetByID(env.Ctx, createdItem.ID())
	require.NoError(t, err)
	require.Equal(t, "Product to Delete", existingItem.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", InventoryBasePath, createdItem.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(InventoryBasePath)

	_, err = service.GetByID(env.Ctx, createdItem.ID())
	require.Error(t, err)
}

func TestInventoryController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", InventoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestInventoryController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewInventoryController(env.App)
	suite.Register(controller)

	suite.GET(InventoryBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}
