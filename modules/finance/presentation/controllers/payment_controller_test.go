package controllers_test

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/require"
)

var (
	PaymentBasePath = "/finance/payments"
)

func createPaymentCategory(
	t *testing.T,
	ctx context.Context,
	service *services.PaymentCategoryService,
	category paymentCategoryEntity.PaymentCategory,
) paymentCategoryEntity.PaymentCategory {
	t.Helper()
	created, err := service.Create(ctx, category)
	require.NoError(t, err)
	return created
}

func TestPaymentController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Test Payment Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Test Payment Category",
		paymentCategoryEntity.WithDescription("Test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(150.00, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Test payment 1"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	payment2 := paymentAggregate.New(
		money.NewFromFloat(250.50, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Test payment 2"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	_, err = paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)
	_, err = paymentService.Create(env.Ctx, payment2)
	require.NoError(t, err)

	response := suite.GET(PaymentBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("$150.00").
		Contains("$250.50")
}

func TestPaymentController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"HTMX Test Account",
		money.NewFromFloat(500.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"HTMX Test Category",
		paymentCategoryEntity.WithDescription("HTMX test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"HTMX Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(75.25, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("HTMX Test Payment"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	_, err = paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)

	suite.GET(PaymentBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("$75.25")
}

func TestPaymentController_GetNew_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Test Category",
		paymentCategoryEntity.WithDescription("Test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_ = createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	response := suite.GET(PaymentBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Amount']").Exists()
	html.Element("//select[@name='AccountID']").Exists()
	html.Element("//select[@name='PaymentCategoryID']").Exists()
	// CounterpartyID is a combobox component which has hidden input and select
	html.Element("//*[@name='CounterpartyID']").Exists()
	html.Element("//textarea[@name='Comment']").Exists()
	html.Element("//input[@name='TransactionDate']").Exists()
	html.Element("//input[@name='AccountingPeriod']").Exists()
}

func TestPaymentController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Test Category",
		paymentCategoryEntity.WithDescription("Test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "175.50")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("PaymentCategoryID", createdCategory.ID().String())
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Comment", "New test payment")
	formData.Set("TransactionDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentBasePath)

	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	savedPayment := payments[0]
	require.Equal(t, int64(17550), savedPayment.Amount().Amount())
	require.Equal(t, "New test payment", savedPayment.Comment())
}

func TestPaymentController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)

	formData := url.Values{}
	formData.Set("Amount", "-100")
	formData.Set("AccountID", "invalid-uuid")
	formData.Set("PaymentCategoryID", "")
	formData.Set("CounterpartyID", "invalid-uuid")
	formData.Set("Comment", "")
	formData.Set("TransactionDate", "")
	formData.Set("AccountingPeriod", "")

	response := suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, payments)
}

func TestPaymentController_GetEdit_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Edit Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Edit Test Category",
		paymentCategoryEntity.WithDescription("Edit test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Edit Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(300.00, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Edit test payment"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	createdPayment1, err := paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s", PaymentBasePath, createdPayment1.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Amount']").Exists()
	html.Element("//select[@name='AccountID']").Exists()
	html.Element("//select[@name='PaymentCategoryID']").Exists()
	// CounterpartyID is a combobox component
	html.Element("//*[@name='CounterpartyID']").Exists()
	html.Element("//textarea[@name='Comment']").Exists()
	require.Equal(t, "Edit test payment", html.Element("//textarea[@name='Comment']").Text())
}

func TestPaymentController_GetEdit_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", PaymentBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestPaymentController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Update Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Update Test Category",
		paymentCategoryEntity.WithDescription("Update test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Update Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Original payment"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	createdPayment1, err := paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "350.75")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("PaymentCategoryID", createdCategory.ID().String())
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Comment", "Updated payment comment")
	formData.Set("TransactionDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(fmt.Sprintf("%s/%s", PaymentBasePath, createdPayment1.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentBasePath)

	updatedPayment, err := paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.NoError(t, err)

	require.Equal(t, int64(35075), updatedPayment.Amount().Amount())
	require.Equal(t, "Updated payment comment", updatedPayment.Comment())
}

func TestPaymentController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Test Category",
		paymentCategoryEntity.WithDescription("Test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Test payment"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	createdPayment1, err := paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("Amount", "-100")
	formData.Set("AccountID", "invalid-uuid")
	formData.Set("PaymentCategoryID", "invalid-uuid")
	formData.Set("CounterpartyID", "invalid-uuid")
	formData.Set("Comment", "")

	response := suite.POST(fmt.Sprintf("%s/%s", PaymentBasePath, createdPayment1.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedPayment, err := paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.NoError(t, err)
	require.Equal(t, "Test payment", unchangedPayment.Comment())
}

func TestPaymentController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentDelete,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Delete Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Delete Test Category",
		paymentCategoryEntity.WithDescription("Delete test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Delete Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	payment1 := paymentAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdCategory,
		paymentAggregate.WithTenantID(env.Tenant.ID),
		paymentAggregate.WithAccount(createdAccount),
		paymentAggregate.WithCounterpartyID(createdCounterparty.ID()),
		paymentAggregate.WithUser(adminUser),
		paymentAggregate.WithComment("Payment to Delete"),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	)

	createdPayment1, err := paymentService.Create(env.Ctx, payment1)
	require.NoError(t, err)

	existingPayment, err := paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.NoError(t, err)
	require.Equal(t, "Payment to Delete", existingPayment.Comment())

	suite.DELETE(fmt.Sprintf("%s/%s", PaymentBasePath, createdPayment1.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(PaymentBasePath)

	_, err = paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.Error(t, err)
}

func TestPaymentController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentDelete,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", PaymentBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestPaymentController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	suite.GET(PaymentBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}

func TestPaymentController_Create_TransactionDateValidation(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Test Category",
		paymentCategoryEntity.WithDescription("Test category"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Test 1: Create payment with empty transaction date (should fail validation)
	formData := url.Values{}
	formData.Set("Amount", "100.00")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("PaymentCategoryID", createdCategory.ID().String())
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Comment", "Payment with empty date")
	formData.Set("TransactionDate", "")
	formData.Set("AccountingPeriod", time.Now().Format(time.DateOnly))

	// This should fail validation since TransactionDate is required
	response := suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(200) // Returns form with validation errors

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	// Verify no payment was created due to validation error
	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, payments)

	// Test 2: Create payment with valid transaction date (should succeed)
	now := time.Now()
	formData.Set("TransactionDate", now.Format(time.DateOnly))

	suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentBasePath)

	payments, err = paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	savedPayment := payments[0]
	require.Equal(t, int64(10000), savedPayment.Amount().Amount())
	require.Equal(t, "Payment with empty date", savedPayment.Comment())

	// Verify transaction date is set to the provided date
	require.Equal(t, now.Format(time.DateOnly), savedPayment.TransactionDate().Format(time.DateOnly))
	require.NotEqual(t, "0001-01-01", savedPayment.TransactionDate().Format(time.DateOnly))
}

func TestPaymentController_Create_VerifyIncomeStatementIntegration(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	paymentService := env.App.Service(services.PaymentService{}).(*services.PaymentService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	paymentCategoryService := env.App.Service(services.PaymentCategoryService{}).(*services.PaymentCategoryService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)
	financialReportService := env.App.Service(services.FinancialReportService{}).(*services.FinancialReportService)

	account := moneyAccountEntity.New(
		"Revenue Test Account",
		money.NewFromFloat(5000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := paymentCategoryEntity.New(
		"Software Development Revenue",
		paymentCategoryEntity.WithDescription("Income from software development"),
		paymentCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory := createPaymentCategory(t, env.Ctx, paymentCategoryService, category)

	counterparty1 := counterparty.New(
		"Client Company",
		counterparty.Customer,
		counterparty.LegalEntity,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create payment with specific date within current year
	currentYear := time.Now().Year()
	paymentDate := time.Date(currentYear, time.July, 15, 0, 0, 0, 0, time.UTC)

	formData := url.Values{}
	formData.Set("Amount", "2500.00")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("PaymentCategoryID", createdCategory.ID().String())
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Comment", "Q3 Software Development Payment")
	formData.Set("TransactionDate", paymentDate.Format(time.DateOnly))
	formData.Set("AccountingPeriod", paymentDate.Format(time.DateOnly))

	suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(PaymentBasePath)

	// Verify payment was created
	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	savedPayment := payments[0]
	require.Equal(t, int64(250000), savedPayment.Amount().Amount())
	require.Equal(t, "Q3 Software Development Payment", savedPayment.Comment())
	require.Equal(t, paymentDate.Format(time.DateOnly), savedPayment.TransactionDate().Format(time.DateOnly))

	// Generate income statement for the year and verify payment appears in revenue
	startDate := time.Date(currentYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(currentYear, time.December, 31, 23, 59, 59, 0, time.UTC)

	incomeStatement, err := financialReportService.GenerateIncomeStatement(env.Ctx, startDate, endDate)
	require.NoError(t, err)
	require.NotNil(t, incomeStatement)

	// Verify revenue section contains our payment
	revenueSection := incomeStatement.RevenueSection
	require.Equal(t, "Revenue", revenueSection.Title)
	require.NotEmpty(t, revenueSection.LineItems)

	// Find our category in the revenue line items
	var foundCategory bool
	for _, lineItem := range revenueSection.LineItems {
		if lineItem.Name == "Software Development Revenue" {
			foundCategory = true
			require.Equal(t, int64(250000), lineItem.Amount.Amount())
			require.Equal(t, "USD", lineItem.Amount.Currency().Code)
			require.Greater(t, lineItem.Percentage, 0.0)
			break
		}
	}
	require.True(t, foundCategory, "Payment category should appear in income statement revenue")

	// Verify total revenue includes our payment
	require.Equal(t, int64(250000), incomeStatement.RevenueSection.Subtotal.Amount())
	require.True(t, incomeStatement.IsProfit(), "Should show profit when we have revenue and no expenses")
}

func TestPaymentController_Create_WithoutCategoryVerifyIncomeStatement(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.Register(controller)

	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)
	financialReportService := env.App.Service(services.FinancialReportService{}).(*services.FinancialReportService)

	account := moneyAccountEntity.New(
		"Uncategorized Revenue Account",
		money.NewFromFloat(3000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	counterparty1 := counterparty.New(
		"Uncategorized Client",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create payment without category (empty PaymentCategoryID)
	currentYear := time.Now().Year()
	paymentDate := time.Date(currentYear, time.August, 10, 0, 0, 0, 0, time.UTC)

	formData := url.Values{}
	formData.Set("Amount", "1500.00")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("PaymentCategoryID", "") // No category
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Comment", "Uncategorized Revenue Payment")
	formData.Set("TransactionDate", paymentDate.Format(time.DateOnly))
	formData.Set("AccountingPeriod", paymentDate.Format(time.DateOnly))

	// This should fail validation since PaymentCategoryID is required
	response := suite.POST(PaymentBasePath).
		Form(formData).
		Expect(t).
		Status(200) // Returns form with validation errors

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	// Verify no payment was created due to validation error
	startDate := time.Date(currentYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(currentYear, time.December, 31, 23, 59, 59, 0, time.UTC)

	incomeStatement, err := financialReportService.GenerateIncomeStatement(env.Ctx, startDate, endDate)
	require.NoError(t, err)
	require.NotNil(t, incomeStatement)

	// Revenue should be empty since no valid payment was created
	require.Empty(t, incomeStatement.RevenueSection.LineItems)
	require.Equal(t, int64(0), incomeStatement.RevenueSection.Subtotal.Amount())
}
