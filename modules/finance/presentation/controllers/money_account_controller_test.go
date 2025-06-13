package controllers_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	MoneyAccountBasePath = "/finance/accounts"
)

// createCurrencies creates standard currencies in a committed transaction
func createCurrencies(t *testing.T, ctx context.Context, currencies ...*currency.Currency) {
	currencyRepo := persistence.NewCurrencyRepository()
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		for _, curr := range currencies {
			if createErr := currencyRepo.Create(txCtx, curr); createErr != nil {
				return createErr
			}
		}
		return nil
	})
	require.NoError(t, err)
}

func TestMoneyAccountController_List_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create test accounts
	account1 := moneyAccountEntity.New(
		"Test Account 1",
		money.NewFromFloat(1000.50, string(currency.UsdCode)),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("ACC001"),
		moneyAccountEntity.WithDescription("Test account 1 description"),
	)

	account2 := moneyAccountEntity.New(
		"Test Account 2",
		money.NewFromFloat(2500.75, string(currency.EurCode)),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("ACC002"),
		moneyAccountEntity.WithDescription("Test account 2 description"),
	)

	_, err := service.Create(env.Ctx, account1)
	require.NoError(t, err)
	_, err = service.Create(env.Ctx, account2)
	require.NoError(t, err)

	response := suite.GET(MoneyAccountBasePath).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2, "Should have at least 2 account rows")

	response.Contains(t, "Test Account 1").
		Contains(t, "Test Account 2")
}

func TestMoneyAccountController_List_HTMX_Request(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"HTMX Test Account",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	suite.GET(MoneyAccountBasePath).
		HTMX().
		Expect().
		Status(t, 200).
		Contains(t, "HTMX Test Account")
}

func TestMoneyAccountController_GetNew_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	response := suite.GET(MoneyAccountBasePath+"/new").
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	// Check form exists
	html.Element("//form[@hx-post]").Exists(t)

	// Check required form fields
	html.Element("//input[@name='Name']").Exists(t)
	html.Element("//input[@name='Balance']").Exists(t)
	html.Element("//select[@name='CurrencyCode']").Exists(t)
	html.Element("//input[@name='AccountNumber']").Exists(t)
	html.Element("//textarea[@name='Description']").Exists(t)

	// Check currency options are populated
	html.Element("//option[@value='USD']").Exists(t)
	html.Element("//option[@value='EUR']").Exists(t)
}

func TestMoneyAccountController_Create_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	formData := url.Values{}
	formData.Set("Name", "New Test Account")
	formData.Set("Balance", "1500.25")
	formData.Set("CurrencyCode", "USD")
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "New account description")

	suite.POST(MoneyAccountBasePath).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, MoneyAccountBasePath)

	// Verify account was created
	accounts, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 1, "One account should be created")

	savedAccount := accounts[0]
	require.Equal(t, "New Test Account", savedAccount.Name())
	require.Equal(t, "ACC123", savedAccount.AccountNumber())
	require.Equal(t, "New account description", savedAccount.Description())
	require.Equal(t, "USD", savedAccount.Balance().Currency().Code)
	require.Equal(t, int64(150025), savedAccount.Balance().Amount())
}

func TestMoneyAccountController_Create_ValidationError(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Prepare invalid form data (missing required fields)
	formData := url.Values{}
	formData.Set("Name", "")                // Required field is empty
	formData.Set("Balance", "-100")         // Negative balance
	formData.Set("CurrencyCode", "INVALID") // Invalid currency code
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "Test description")

	response := suite.POST(MoneyAccountBasePath).
		WithForm(formData).
		Expect().
		Status(t, 200) // Returns form with errors

	html := response.HTML(t)

	// Should contain validation error messages
	require.Greater(t, len(html.Elements("//small[@data-testid='field-error']")), 0, "Should have validation error indicators")

	// Verify no account was created
	accounts, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, accounts, "No accounts should be created with validation errors")
}

func TestMoneyAccountController_GetEdit_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create test account
	balance := money.NewFromFloat(1000.00, "USD")
	account := moneyAccountEntity.New(
		"Edit Test Account",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("EDIT001"),
		moneyAccountEntity.WithDescription("Account to edit"),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	// Check that form is pre-populated with existing values
	html.Element("//input[@name='Name']").Exists(t)
	require.Equal(t, "Edit Test Account", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//input[@name='AccountNumber']").Exists(t)
	require.Equal(t, "EDIT001", html.Element("//input[@name='AccountNumber']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists(t)
	require.Equal(t, "Account to edit", html.Element("//textarea[@name='Description']").Text())
}

func TestMoneyAccountController_GetEdit_NotFound(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	// Create request with non-existent UUID
	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500) // Should return error
}

func TestMoneyAccountController_Update_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create test account
	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"Original Account",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("ORIG001"),
		moneyAccountEntity.WithDescription("Original description"),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	// Prepare update form data
	formData := url.Values{}
	formData.Set("Name", "Updated Account Name")
	formData.Set("Balance", "750.50")
	formData.Set("CurrencyCode", "EUR")
	formData.Set("AccountNumber", "UPD001")
	formData.Set("Description", "Updated description")

	suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, MoneyAccountBasePath)

	// Verify account was updated
	updatedAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Account Name", updatedAccount.Name())
	require.Equal(t, "UPD001", updatedAccount.AccountNumber())
	require.Equal(t, "Updated description", updatedAccount.Description())
	require.Equal(t, "EUR", updatedAccount.Balance().Currency().Code)
	require.Equal(t, int64(75050), updatedAccount.Balance().Amount())
}

func TestMoneyAccountController_Update_ValidationError(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create test account
	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"Test Account",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	// Prepare invalid update form data
	formData := url.Values{}
	formData.Set("Name", "")                // Empty name should fail validation
	formData.Set("Balance", "-100")         // Negative balance
	formData.Set("CurrencyCode", "INVALID") // Invalid currency code
	formData.Set("AccountNumber", "")
	formData.Set("Description", "")

	response := suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 200) // Returns form with errors

	html := response.HTML(t)

	// Should contain validation error messages
	require.Greater(t, len(html.Elements("//small[@data-testid='field-error']")), 0, "Should have validation error indicators")

	// Verify account was not updated
	unchangedAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Account", unchangedAccount.Name()) // Should remain unchanged
}

func TestMoneyAccountController_Delete_Success(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	// Create test account
	balance := money.NewFromFloat(100.00, "USD")
	account := moneyAccountEntity.New(
		"Account to Delete",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	// Verify account exists
	existingAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)
	require.Equal(t, "Account to Delete", existingAccount.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		Expect().
		Status(t, 302).
		RedirectTo(t, MoneyAccountBasePath)

	// Verify account was deleted
	_, err = service.GetByID(env.Ctx, createdAccount.ID())
	require.Error(t, err, "Account should be deleted and not found")
}

func TestMoneyAccountController_Delete_NotFound(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	// Create request with non-existent UUID
	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500) // Should return error
}

func TestMoneyAccountController_InvalidUUID(t *testing.T) {
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
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.RegisterController(controller)

	// Test with invalid UUID format
	suite.GET(MoneyAccountBasePath+"/invalid-uuid").
		Expect().
		Status(t, 404) // Should return not found
}
