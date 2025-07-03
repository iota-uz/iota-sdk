package controllers_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

var (
	MoneyAccountBasePath = "/finance/accounts"
)

func createCurrencies(t *testing.T, ctx context.Context, currencies ...*currency.Currency) {
	t.Helper()
	currencyRepo := persistence.NewCurrencyRepository()
	for _, curr := range currencies {
		err := currencyRepo.Create(ctx, curr)
		require.NoError(t, err)
	}
}

func TestMoneyAccountController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

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
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Account 1").
		Contains("Test Account 2")
}

func TestMoneyAccountController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

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
		Expect(t).
		Status(200).
		Contains("HTMX Test Account")
}

func TestMoneyAccountController_GetNew_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	response := suite.GET(MoneyAccountBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Name']").Exists()
	html.Element("//input[@name='Balance']").Exists()
	html.Element("//select[@name='CurrencyCode']").Exists()
	html.Element("//input[@name='AccountNumber']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
	html.Element("//option[@value='USD']").Exists()
	html.Element("//option[@value='EUR']").Exists()
}

func TestMoneyAccountController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	formData := url.Values{}
	formData.Set("Name", "New Test Account")
	formData.Set("Balance", "1500.25")
	formData.Set("CurrencyCode", "USD")
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "New account description")

	suite.POST(MoneyAccountBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(MoneyAccountBasePath)

	accounts, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, accounts, 1)

	savedAccount := accounts[0]
	require.Equal(t, "New Test Account", savedAccount.Name())
	require.Equal(t, "ACC123", savedAccount.AccountNumber())
	require.Equal(t, "New account description", savedAccount.Description())
	require.Equal(t, "USD", savedAccount.Balance().Currency().Code)
	require.Equal(t, int64(150025), savedAccount.Balance().Amount())
}

func TestMoneyAccountController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Balance", "-100")
	formData.Set("CurrencyCode", "INVALID")
	formData.Set("AccountNumber", "ACC123")
	formData.Set("Description", "Test description")

	response := suite.POST(MoneyAccountBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	accounts, err := service.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, accounts)
}

func TestMoneyAccountController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

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
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Name']").Exists()
	require.Equal(t, "Edit Test Account", html.Element("//input[@name='Name']").Attr("value"))

	html.Element("//input[@name='AccountNumber']").Exists()
	require.Equal(t, "EDIT001", html.Element("//input[@name='AccountNumber']").Attr("value"))

	html.Element("//textarea[@name='Description']").Exists()
	require.Equal(t, "Account to edit", html.Element("//textarea[@name='Description']").Text())
}

func TestMoneyAccountController_GetEdit_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestMoneyAccountController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD, &currency.EUR)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

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

	formData := url.Values{}
	formData.Set("Name", "Updated Account Name")
	formData.Set("Balance", "750.50")
	formData.Set("CurrencyCode", "EUR")
	formData.Set("AccountNumber", "UPD001")
	formData.Set("Description", "Updated description")

	suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(MoneyAccountBasePath)

	updatedAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)

	require.Equal(t, "Updated Account Name", updatedAccount.Name())
	require.Equal(t, "UPD001", updatedAccount.AccountNumber())
	require.Equal(t, "Updated description", updatedAccount.Description())
	require.Equal(t, "EUR", updatedAccount.Balance().Currency().Code)
	require.Equal(t, int64(75050), updatedAccount.Balance().Amount())
}

func TestMoneyAccountController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	balance := money.NewFromFloat(500.00, "USD")
	account := moneyAccountEntity.New(
		"Test Account",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Name", "")
	formData.Set("Balance", "-100")
	formData.Set("CurrencyCode", "INVALID")
	formData.Set("AccountNumber", "")
	formData.Set("Description", "")

	response := suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)
	require.Equal(t, "Test Account", unchangedAccount.Name())
}

func TestMoneyAccountController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	service := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	balance := money.NewFromFloat(100.00, "USD")
	account := moneyAccountEntity.New(
		"Account to Delete",
		balance,
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := service.Create(env.Ctx, account)
	require.NoError(t, err)

	existingAccount, err := service.GetByID(env.Ctx, createdAccount.ID())
	require.NoError(t, err)
	require.Equal(t, "Account to Delete", existingAccount.Name())

	suite.DELETE(fmt.Sprintf("%s/%s", MoneyAccountBasePath, createdAccount.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(MoneyAccountBasePath)

	_, err = service.GetByID(env.Ctx, createdAccount.ID())
	require.Error(t, err)
}

func TestMoneyAccountController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", MoneyAccountBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestMoneyAccountController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewMoneyAccountController(env.App)
	suite.Register(controller)

	suite.GET(MoneyAccountBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}
