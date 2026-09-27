package controllers_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

const historyItems = "(//a[starts-with(@href,'/finance/payments/') or starts-with(@href,'/finance/expenses/')]" +
	" | //button[starts-with(@hx-get,'/finance/transactions/')])"

type accountCardFixture struct {
	suite    *itf.Suite
	env      *itf.TestEnvironment
	accounts *services.MoneyAccountService
}

func newAccountCardFixture(t *testing.T) *accountCardFixture {
	t.Helper()
	user := itf.User(permissions.PaymentCreate, permissions.PaymentRead)
	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(user)
	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	accounts := itf.GetService[services.MoneyAccountService](env)
	suite.Register(controllers.NewMoneyAccountController(
		accounts,
		itf.GetService[services.TransactionService](env),
		itf.GetService[coreservices.CurrencyService](env),
	))
	return &accountCardFixture{suite: suite, env: env, accounts: accounts}
}

func (f *accountCardFixture) createAccount(t *testing.T, name string, balance float64) moneyAccountEntity.Account {
	t.Helper()
	created, err := f.accounts.Create(f.env.Ctx, moneyAccountEntity.New(
		name,
		money.NewFromFloat(balance, "USD"),
		moneyAccountEntity.WithTenantID(f.env.Tenant.ID),
		moneyAccountEntity.WithAccountNumber(name),
	))
	require.NoError(t, err)
	return created
}

func (f *accountCardFixture) history(t *testing.T, account moneyAccountEntity.Account, query string) *itf.HTML {
	t.Helper()
	return f.suite.GET(fmt.Sprintf("%s/%s/transactions?%s", MoneyAccountBasePath, account.ID(), query)).
		HTMX().
		Expect(t).
		Status(200).
		HTML()
}

func TestMoneyAccountController_Drawer_OpensCardView(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	account := f.createAccount(t, "Card", 100)

	html := f.suite.GET(fmt.Sprintf("%s/%s/drawer", MoneyAccountBasePath, account.ID())).
		HTMX().
		Expect(t).
		Status(200).
		HTML()

	html.Element(fmt.Sprintf("//div[@hx-get='/finance/balances/%s']", account.ID())).Exists()
	html.Element("//*[@x-data=\"{selectedTab: 'general'}\"]").Exists()
	require.Len(t, html.Elements(historyItems), 1)
}

func TestMoneyAccountController_History_ShowsBothSidesOfTheAccount(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	source := f.createAccount(t, "Source", 100)
	destination := f.createAccount(t, "Destination", 0)
	other := f.createAccount(t, "Other", 40)

	form := url.Values{}
	form.Set("DestinationAccountID", destination.ID().String())
	form.Set("Amount", "30")
	f.suite.POST(fmt.Sprintf("%s/%s/transfer", MoneyAccountBasePath, source.ID())).
		Form(form).
		Expect(t).
		Status(302)

	outgoing := f.history(t, source, "")
	require.Len(t, outgoing.Elements(historyItems), 2)
	require.Contains(t, outgoing.Element(historyItems+"[1]").Text(), "−$30.00")
	require.Contains(t, outgoing.Element(historyItems+"[1]").Text(), "Destination")
	require.Contains(t, outgoing.Element(historyItems+"[2]").Text(), "+$100.00")

	incoming := f.history(t, destination, "")
	require.Len(t, incoming.Elements(historyItems), 2)
	require.Contains(t, incoming.Element(historyItems+"[1]").Text(), "+$30.00")
	require.Contains(t, incoming.Element(historyItems+"[1]").Text(), "Source")

	transfers := f.history(t, destination, "type="+string(transaction.Transfer))
	require.Len(t, transfers.Elements(historyItems), 1)

	require.Len(t, f.history(t, other, "").Elements(historyItems), 1)

	registerFinancialOverview(f.suite, f.env)
	allOfDestination := f.suite.GET("/finance/transactions?account=" + destination.ID().String()).
		Expect(t).
		Status(200).
		HTML()
	require.Len(t, allOfDestination.Elements("//table//tbody//tr[@hx-get]"), 2)
}

func TestMoneyAccountController_History_LoadsNextPage(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	source := f.createAccount(t, "Source", 100)
	destination := f.createAccount(t, "Destination", 0)
	for _, amount := range []string{"10", "20"} {
		form := url.Values{}
		form.Set("DestinationAccountID", destination.ID().String())
		form.Set("Amount", amount)
		f.suite.POST(fmt.Sprintf("%s/%s/transfer", MoneyAccountBasePath, source.ID())).
			Form(form).
			Expect(t).
			Status(302)
	}

	first := f.history(t, source, "limit=2")
	require.Len(t, first.Elements(historyItems), 2)
	next := first.Element("//div[@hx-trigger='intersect once']").Attr("hx-get")
	require.Contains(t, next, "page=2")
	require.Contains(t, next, "limit=2")

	nextURL, err := url.Parse(next)
	require.NoError(t, err)
	last := f.history(t, source, nextURL.RawQuery)
	require.Len(t, last.Elements(historyItems), 1)
	require.Contains(t, last.Element(historyItems).Text(), "+$100.00")
	last.Element("//div[@hx-trigger='intersect once']").NotExists()
}

func TestMoneyAccountController_History_OpensPrimaryDocument(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	account := f.createAccount(t, "Payments", 0)

	category := createPaymentCategory(t, f.env.Ctx, itf.GetService[services.PaymentCategoryService](f.env), paymentCategoryEntity.New(
		"Sales",
		paymentCategoryEntity.WithTenantID(f.env.Tenant.ID),
	))
	client, err := itf.GetService[services.CounterpartyService](f.env).Create(f.env.Ctx, counterparty.New(
		"Client",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(f.env.Tenant.ID),
	))
	require.NoError(t, err)
	payment, err := itf.GetService[services.PaymentService](f.env).Create(f.env.Ctx, paymentAggregate.New(
		money.NewFromFloat(75, "USD"),
		category,
		paymentAggregate.WithTenantID(f.env.Tenant.ID),
		paymentAggregate.WithAccount(account),
		paymentAggregate.WithCounterpartyID(client.ID()),
		paymentAggregate.WithUser(f.env.User),
		paymentAggregate.WithTransactionDate(time.Now()),
		paymentAggregate.WithAccountingPeriod(time.Now()),
	))
	require.NoError(t, err)

	html := f.history(t, account, "")
	link := html.Element(fmt.Sprintf("//a[@href='/finance/payments/%s']", payment.ID()))
	link.Exists()
	require.Contains(t, link.Text(), "Sales")
	require.Contains(t, link.Text(), "Client")
	require.Contains(t, link.Text(), "+$75.00")
	html.Element("//button[contains(@hx-get, '/finance/transactions/')]").Exists()
}

func TestMoneyAccountController_Update_RecordsBalanceAdjustment(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	account := f.createAccount(t, "Adjusted", 100)

	update := func(balance string) {
		form := url.Values{}
		form.Set("Name", "Adjusted")
		form.Set("Balance", balance)
		form.Set("CurrencyCode", "USD")
		form.Set("AccountNumber", "Adjusted")
		f.suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, account.ID())).
			Form(form).
			Expect(t).
			Status(302)
	}

	update("150")
	update("120")
	update("120")

	adjustments := f.history(t, account, "type="+string(transaction.Adjustment))
	require.Len(t, adjustments.Elements(historyItems), 2)
	require.Contains(t, adjustments.Element(historyItems+"[1]").Text(), "−$30.00")
	require.Contains(t, adjustments.Element(historyItems+"[2]").Text(), "+$50.00")

	require.NoError(t, f.accounts.RecalculateBalance(f.env.Ctx, account.ID()))
	recalculated, err := f.accounts.GetByID(f.env.Ctx, account.ID())
	require.NoError(t, err)
	require.Equal(t, int64(12000), recalculated.Balance().Amount())
}

func TestMoneyAccountController_Update_ValidationKeepsInputOnEditTab(t *testing.T) {
	t.Parallel()
	f := newAccountCardFixture(t)
	account := f.createAccount(t, "Original", 100)

	form := url.Values{}
	form.Set("Name", "Renamed")
	form.Set("Balance", "-5")
	form.Set("CurrencyCode", "USD")
	form.Set("AccountNumber", "NEW-1")
	html := f.suite.POST(fmt.Sprintf("%s/%s", MoneyAccountBasePath, account.ID())).
		Form(form).
		HTMX().
		HTMXTarget(fmt.Sprintf("money-account-drawer-%s", account.ID())).
		Expect(t).
		Status(200).
		HTML()

	html.Element("//*[@x-data=\"{selectedTab: 'edit'}\"]").Exists()
	require.Equal(t, "Renamed", html.Element("//input[@name='Name']").Attr("value"))
	require.Equal(t, "-5", html.Element("//input[@name='Balance']").Attr("value"))
	require.Equal(t, "NEW-1", html.Element("//input[@name='AccountNumber']").Attr("value"))
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchanged, err := f.accounts.GetByID(f.env.Ctx, account.ID())
	require.NoError(t, err)
	require.Equal(t, "Original", unchanged.Name())
}
