package controllers_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/require"
)

var (
	InventoryBasePath = "/finance/inventory"
)

func TestInventoryController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", InventoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestInventoryController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	service := inventorySvc

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

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", InventoryBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestInventoryController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	controller := controllers.NewInventoryController(inventorySvc, currencySvc)
	suite.Register(controller)

	suite.GET(InventoryBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}

func TestInventoryController_List_FilterChips(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: &rbac.PermissionSchema{Sets: []rbac.PermissionSet{}},
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR, currency.JPY)

	inventorySvc := itf.GetService[services.InventoryService](env)
	currencySvc := itf.GetService[coreservices.CurrencyService](env)
	suite.Register(controllers.NewInventoryController(inventorySvc, currencySvc))

	for _, it := range []struct {
		name     string
		price    *money.Money
		quantity int
	}{
		{"Chip Cheap Bolt", money.NewFromFloat(5.50, "USD"), 1000},
		{"Chip Dear Drill", money.NewFromFloat(250.00, "USD"), 3},
		{"Chip Euro Saw", money.NewFromFloat(120.00, "EUR"), 40},
		// The yen has no minor digits: 150 is stored as 150, not 15000.
		{"Chip Yen Saw", money.NewFromFloat(150, "JPY"), 2},
	} {
		_, err := inventorySvc.Create(env.Ctx, inventory.New(it.name, it.price, it.quantity))
		require.NoError(t, err)
	}

	today := time.Now().Format("2006-01-02")
	cases := []struct {
		name    string
		query   string
		present []string
		absent  []string
	}{
		{"currency", "?f=currency:is:USD", []string{"Chip Cheap Bolt", "Chip Dear Drill"}, []string{"Chip Euro Saw"}},
		{"currency is not", "?f=currency:isnot:USD", []string{"Chip Euro Saw"}, []string{"Chip Cheap Bolt", "Chip Dear Drill"}},
		{"price in major units", "?f=price:between:100,200", []string{"Chip Euro Saw", "Chip Yen Saw"}, []string{"Chip Cheap Bolt", "Chip Dear Drill"}},
		{"price of a currency without minor digits", "?f=price:gt:149&f=currency:is:JPY", []string{"Chip Yen Saw"}, []string{"Chip Euro Saw"}},
		{"quantity", "?f=quantity:gt:10", []string{"Chip Cheap Bolt", "Chip Euro Saw"}, []string{"Chip Dear Drill"}},
		{"currency and price", "?f=currency:is:USD&f=price:lt:100", []string{"Chip Cheap Bolt"}, []string{"Chip Dear Drill", "Chip Euro Saw"}},
		{"search and chip", "?Search=Drill&f=currency:is:USD", []string{"Chip Dear Drill"}, []string{"Chip Cheap Bolt", "Chip Euro Saw"}},
		{"search and excluding chip", "?Search=Drill&f=currency:is:EUR", nil, []string{"Chip Dear Drill", "Chip Cheap Bolt", "Chip Euro Saw"}},
		{"created today", "?f=created_at:on:" + today, []string{"Chip Cheap Bolt", "Chip Euro Saw"}, nil},
		{"legacy created-at link", "?CreatedAt.From=2000-01-01T00:00:00Z&CreatedAt.To=2000-01-31T23:59:59Z", nil, []string{"Chip Cheap Bolt", "Chip Euro Saw"}},
	}
	for _, tc := range cases {
		response := suite.GET(InventoryBasePath + tc.query).HTMX().Expect(t).Status(200)
		for _, name := range tc.present {
			response.Contains(name)
		}
		for _, name := range tc.absent {
			response.NotContains(name)
		}
		// The chip bar rides along with the rows so it shows what is applied.
		response.Contains(`hx-swap-oob="outerHTML"`).Contains(`id="inventory-filters"`)
	}

	// A full page renders the builder with the applied chip inside the form.
	page := suite.GET(InventoryBasePath + "?f=currency:is:EUR").Expect(t).Status(200)
	page.Contains(`x-data="filterBuilder"`).Contains(`value="currency:is:EUR"`).NotContains("Chip Cheap Bolt")

	// The legacy link shows up as a created-at chip.
	suite.GET(InventoryBasePath + "?CreatedAt.From=2000-01-01&CreatedAt.To=2000-01-31").Expect(t).Status(200).
		Contains(`value="created_at:between:2000-01-01,2000-01-31"`)

	// Page links keep the chips and the search.
	paged := suite.GET(InventoryBasePath + "?limit=1&Search=Chip&f=currency:is:USD").HTMX().Expect(t).Status(200)
	paged.Contains("f=currency%3Ais%3AUSD").Contains("Search=Chip").Contains("page=2")

	// Back to a chip URL after a history-cache miss gets the whole page.
	suite.GET(InventoryBasePath + "?f=currency:is:EUR").HTMX().HTMXHistoryRestore().Expect(t).Status(200).
		Contains("<html").Contains(`x-data="filterBuilder"`)
}
