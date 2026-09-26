package controllers_test

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	debtAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

type obligationFixture struct {
	suite          *itf.Suite
	env            *itf.TestEnvironment
	debtService    *services.DebtService
	counterpartyID uuid.UUID
	usdAccountID   uuid.UUID
	uzsAccountID   uuid.UUID
}

func newObligationFixture(t *testing.T) *obligationFixture {
	t.Helper()
	user := itf.User(
		permissions.DebtCreate,
		permissions.DebtRead,
		permissions.DebtUpdate,
	)
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(user)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.UZS)

	suite.Register(newDebtsController(env))

	moneyAccountService := itf.GetService[services.MoneyAccountService](env)
	usdAccount, err := moneyAccountService.Create(env.Ctx, moneyAccountEntity.New(
		"Dollar account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("USD-1"),
	))
	require.NoError(t, err)
	uzsAccount, err := moneyAccountService.Create(env.Ctx, moneyAccountEntity.New(
		"Sum account",
		money.NewFromFloat(5000000.00, "UZS"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber("UZS-1"),
	))
	require.NoError(t, err)

	createdCounterparty, err := itf.GetService[services.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"Obligation Counterparty",
		counterparty.Supplier,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	return &obligationFixture{
		suite:          suite,
		env:            env,
		debtService:    itf.GetService[services.DebtService](env),
		counterpartyID: createdCounterparty.ID(),
		usdAccountID:   usdAccount.ID(),
		uzsAccountID:   uzsAccount.ID(),
	}
}

func (f *obligationFixture) createPayable(t *testing.T, amount float64, accountID *uuid.UUID) debtAggregate.Debt {
	t.Helper()
	created, err := f.debtService.Create(f.env.Ctx, debtAggregate.New(
		debtAggregate.DebtTypePayable,
		money.NewFromFloat(amount, "USD"),
		debtAggregate.WithTenantID(f.env.Tenant.ID),
		debtAggregate.WithCounterpartyID(f.counterpartyID),
		debtAggregate.WithDescription("Supplier invoice"),
		debtAggregate.WithMoneyAccountID(accountID),
	))
	require.NoError(t, err)
	return created
}

func (f *obligationFixture) obligationForm(amount, accountID string) url.Values {
	form := url.Values{}
	form.Set("CounterpartyID", f.counterpartyID.String())
	form.Set("Type", "PAYABLE")
	form.Set("Amount", amount)
	form.Set("CurrencyCode", "USD")
	form.Set("MoneyAccountID", accountID)
	form.Set("Description", "Supplier invoice")
	form.Set("DueDate", "2026-10-15")
	return form
}

func TestDebtController_Create_ObligationOnAccount(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)

	response := f.suite.POST(DebtBasePath).
		Form(f.obligationForm("300.00", f.usdAccountID.String())).
		HTMX().
		HTMXTarget("debt-create-drawer").
		Expect(t).
		Status(200)
	require.Equal(t, DebtBasePath, response.Header("HX-Redirect"))

	debts, err := f.debtService.GetAll(f.env.Ctx)
	require.NoError(t, err)
	require.Len(t, debts, 1)
	saved := debts[0]
	require.Equal(t, debtAggregate.DebtTypePayable, saved.Type())
	require.Equal(t, int64(30000), saved.OriginalAmount().Amount())
	require.Equal(t, "USD", saved.OriginalAmount().Currency().Code)
	require.Equal(t, int64(30000), saved.OutstandingAmount().Amount())
	require.NotNil(t, saved.MoneyAccountID())
	require.Equal(t, f.usdAccountID, *saved.MoneyAccountID())
	require.Nil(t, saved.ProjectID())
	require.Equal(t, "2026-10-15", saved.DueDate().Format("2006-01-02"))
}

func TestDebtController_Create_AccountCurrencyMismatchKeepsInput(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)

	response := f.suite.POST(DebtBasePath).
		Form(f.obligationForm("300.00", f.uzsAccountID.String())).
		HTMX().
		HTMXTarget("debt-create-drawer").
		Expect(t).
		Status(200)
	require.Empty(t, response.Header("HX-Redirect"))

	html := response.HTML()
	require.NotEmpty(t, html.Element("//small[@data-testid='field-error']").Text())
	html.Element(fmt.Sprintf("//select[@name='MoneyAccountID']/option[@value='%s' and @selected]", f.uzsAccountID)).Exists()
	html.Element("//select[@name='CurrencyCode']/option[@value='USD' and @selected]").Exists()
	html.Element("//input[@name='Amount' and @value='300.00']").Exists()
	require.Equal(t, "Supplier invoice", html.Element("//textarea[@name='Description']").Text())

	debts, err := f.debtService.GetAll(f.env.Ctx)
	require.NoError(t, err)
	require.Empty(t, debts)
}

func TestDebtController_Update_KeepsSettledPart(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)
	created := f.createPayable(t, 300, &f.usdAccountID)

	_, err := f.debtService.Settle(f.env.Ctx, created.ID(), 100, nil)
	require.NoError(t, err)

	response := f.suite.POST(fmt.Sprintf("%s/%s", DebtBasePath, created.ID())).
		Form(f.obligationForm("500.00", "")).
		HTMX().
		Expect(t).
		Status(200)
	require.Equal(t, DebtBasePath, response.Header("HX-Redirect"))

	updated, err := f.debtService.GetByID(f.env.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, int64(50000), updated.OriginalAmount().Amount())
	require.Equal(t, int64(40000), updated.OutstandingAmount().Amount())
	require.Equal(t, debtAggregate.DebtStatusPartial, updated.Status())
	require.Nil(t, updated.MoneyAccountID())
}

func TestDebtController_Update_ValidationErrorKeepsInput(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)
	created := f.createPayable(t, 300, nil)

	form := f.obligationForm("0", f.usdAccountID.String())
	form.Set("Description", "Corrected basis")
	response := f.suite.POST(fmt.Sprintf("%s/%s", DebtBasePath, created.ID())).
		Form(form).
		HTMX().
		Expect(t).
		Status(200)
	require.Empty(t, response.Header("HX-Redirect"))

	html := response.HTML()
	require.Equal(t, "Corrected basis", html.Element("//textarea[@name='Description']").Text())
	html.Element(fmt.Sprintf("//select[@name='MoneyAccountID']/option[@value='%s' and @selected]", f.usdAccountID)).Exists()

	unchanged, err := f.debtService.GetByID(f.env.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, "Supplier invoice", unchanged.Description())
	require.Nil(t, unchanged.MoneyAccountID())
}

func TestDebtController_Cancel(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)
	created := f.createPayable(t, 300, &f.usdAccountID)

	f.suite.POST(fmt.Sprintf("%s/%s/cancel", DebtBasePath, created.ID())).
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	cancelled, err := f.debtService.GetByID(f.env.Ctx, created.ID())
	require.NoError(t, err)
	require.Equal(t, debtAggregate.DebtStatusCancelled, cancelled.Status())
	require.True(t, cancelled.OutstandingAmount().IsZero())
	require.Equal(t, int64(30000), cancelled.OriginalAmount().Amount())

	f.suite.POST(fmt.Sprintf("%s/%s/cancel", DebtBasePath, created.ID())).
		Expect(t).
		Status(409)
}

func TestDebtController_EditDrawer_ClosedDebtHasNoActions(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)
	created := f.createPayable(t, 300, nil)

	open := f.suite.GET(fmt.Sprintf("%s/%s/drawer", DebtBasePath, created.ID())).
		Expect(t).
		Status(200).
		HTML()
	open.Element(fmt.Sprintf("//form[@hx-post='%s/%s/settle']", DebtBasePath, created.ID())).Exists()
	open.Element(fmt.Sprintf("//button[@hx-post='%s/%s/cancel']", DebtBasePath, created.ID())).Exists()

	_, err := f.debtService.Cancel(f.env.Ctx, created.ID())
	require.NoError(t, err)

	closed := f.suite.GET(fmt.Sprintf("%s/%s/drawer", DebtBasePath, created.ID())).
		Expect(t).
		Status(200).
		HTML()
	closed.Element(fmt.Sprintf("//form[@hx-post='%s/%s/settle']", DebtBasePath, created.ID())).NotExists()
	closed.Element(fmt.Sprintf("//button[@hx-post='%s/%s/cancel']", DebtBasePath, created.ID())).NotExists()
}

func TestFinancialOverviewController_Balances(t *testing.T) {
	t.Parallel()
	f := newObligationFixture(t)
	f.suite.Register(controllers.NewFinancialOverviewController(
		itf.GetService[services.PaymentService](f.env),
		itf.GetService[services.MoneyAccountService](f.env),
		itf.GetService[services.CounterpartyService](f.env),
		itf.GetService[services.PaymentCategoryService](f.env),
		itf.GetService[services.TransactionService](f.env),
		itf.GetService[services.BalanceService](f.env),
	))

	f.createPayable(t, 300, &f.usdAccountID)
	f.createPayable(t, 50, nil)
	cancelled := f.createPayable(t, 1000, &f.usdAccountID)
	_, err := f.debtService.Cancel(f.env.Ctx, cancelled.ID())
	require.NoError(t, err)
	_, err = f.debtService.Create(f.env.Ctx, debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(70, "USD"),
		debtAggregate.WithTenantID(f.env.Tenant.ID),
		debtAggregate.WithCounterpartyID(f.counterpartyID),
		debtAggregate.WithDescription("Owed to us"),
	))
	require.NoError(t, err)

	balances, err := itf.GetService[services.BalanceService](f.env).Balances(f.env.Ctx)
	require.NoError(t, err)
	require.Len(t, balances, 2)
	usd := balances[0]
	require.Equal(t, "USD", usd.OnAccounts.Currency().Code)
	require.Equal(t, int64(100000), usd.OnAccounts.Amount())
	require.Equal(t, int64(35000), usd.Reserved.Amount())
	require.Equal(t, int64(65000), usd.Available.Amount())
	uzs := balances[1]
	require.Equal(t, "UZS", uzs.OnAccounts.Currency().Code)
	require.True(t, uzs.Reserved.IsZero())

	account, err := itf.GetService[services.BalanceService](f.env).AccountBalance(f.env.Ctx, f.usdAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(30000), account.Reserved.Amount())
	require.Equal(t, int64(70000), account.Available.Amount())

	f.suite.GET("/finance/balances").
		Expect(t).
		Status(200).
		Contains("$1,000.00").
		Contains("$350.00").
		Contains("$650.00")
}
