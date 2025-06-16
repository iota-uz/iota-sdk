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
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	PaymentBasePath = "/finance/payments"
)

func createPaymentCategory(t *testing.T, ctx context.Context, service *services.PaymentCategoryService, category paymentCategoryEntity.PaymentCategory) paymentCategoryEntity.PaymentCategory {
	created, err := service.Create(ctx, category)
	require.NoError(t, err)
	return created
}

func TestPaymentController_List_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains(t, "$150.00").
		Contains(t, "$250.50")
}

func TestPaymentController_List_HTMX_Request(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		Expect().
		Status(t, 200).
		Contains(t, "$75.25")
}

func TestPaymentController_GetNew_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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

	response := suite.GET(PaymentBasePath+"/new").
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//form[@hx-post]").Exists(t)
	html.Element("//input[@name='Amount']").Exists(t)
	html.Element("//select[@name='AccountID']").Exists(t)
	html.Element("//select[@name='PaymentCategoryID']").Exists(t)
	// CounterpartyID is a combobox component which has hidden input and select
	html.Element("//*[@name='CounterpartyID']").Exists(t)
	html.Element("//textarea[@name='Comment']").Exists(t)
	html.Element("//input[@name='TransactionDate']").Exists(t)
	html.Element("//input[@name='AccountingPeriod']").Exists(t)
}

func TestPaymentController_Create_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentBasePath)

	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, payments, 1)

	savedPayment := payments[0]
	require.Equal(t, int64(17550), savedPayment.Amount().Amount())
	require.Equal(t, "New test payment", savedPayment.Comment())
}

func TestPaymentController_Create_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	payments, err := paymentService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, payments)
}

func TestPaymentController_GetEdit_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//input[@name='Amount']").Exists(t)
	html.Element("//select[@name='AccountID']").Exists(t)
	html.Element("//select[@name='PaymentCategoryID']").Exists(t)
	// CounterpartyID is a combobox component
	html.Element("//*[@name='CounterpartyID']").Exists(t)
	html.Element("//textarea[@name='Comment']").Exists(t)
	require.Equal(t, "Edit test payment", html.Element("//textarea[@name='Comment']").Text())
}

func TestPaymentController_GetEdit_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", PaymentBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestPaymentController_Update_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentBasePath)

	updatedPayment, err := paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.NoError(t, err)

	require.Equal(t, int64(35075), updatedPayment.Amount().Amount())
	require.Equal(t, "Updated payment comment", updatedPayment.Comment())
}

func TestPaymentController_Update_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedPayment, err := paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.NoError(t, err)
	require.Equal(t, "Test payment", unchangedPayment.Comment())
}

func TestPaymentController_Delete_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentDelete,
		permissions.PaymentCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

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
		Expect().
		Status(t, 302).
		RedirectTo(t, PaymentBasePath)

	_, err = paymentService.GetByID(env.Ctx, createdPayment1.ID())
	require.Error(t, err)
}

func TestPaymentController_Delete_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
		permissions.PaymentDelete,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", PaymentBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestPaymentController_InvalidUUID(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.PaymentRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewPaymentsController(env.App)
	suite.RegisterController(controller)

	suite.GET(PaymentBasePath+"/invalid-uuid").
		Expect().
		Status(t, 404)
}
