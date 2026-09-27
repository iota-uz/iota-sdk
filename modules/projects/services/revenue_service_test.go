package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financepermissions "github.com/iota-uz/iota-sdk/modules/finance/permissions"
	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

type invoicesStub []*money.Money

func (s invoicesStub) Invoiced(context.Context, []uuid.UUID) ([]*money.Money, error) {
	return s, nil
}

func TestRevenueService_KeepsBasesAndCurrenciesApart(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithComponents(
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		finance.NewComponent(),
		projects.NewComponent(),
	), itf.WithUser(itf.User(financepermissions.PaymentCreate)))

	currencyService := itf.GetService[coreservices.CurrencyService](env)
	for _, c := range []currency.Currency{currency.USD, currency.EUR} {
		require.NoError(t, currencyService.Create(env.Ctx, &currency.CreateDTO{
			Code:   string(c.Code()),
			Name:   c.Name(),
			Symbol: string(c.Symbol()),
		}))
	}

	client, err := itf.GetService[financeservices.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"Revenue Client",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	projectService := itf.GetService[services.ProjectService](env)
	first := project.New("First", client.ID(), project.WithTenantID(env.Tenant.ID), project.WithContract(money.New(100000, "USD")))
	second := project.New("Second", client.ID(), project.WithTenantID(env.Tenant.ID), project.WithContract(money.New(50000, "USD")))
	require.NoError(t, projectService.Create(env.Ctx, first))
	require.NoError(t, projectService.Create(env.Ctx, second))

	acceptanceService := itf.GetService[services.AcceptanceService](env)
	date := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	for _, document := range []acceptance.Document{
		acceptance.New(first.ID(), acceptance.KindAct, "A-1", date, money.New(40000, "USD"), acceptance.WithTenantID(env.Tenant.ID)),
		acceptance.New(second.ID(), acceptance.KindDeliveryNote, "N-1", date, money.New(20000, "EUR"), acceptance.WithTenantID(env.Tenant.ID)),
	} {
		created, err := acceptanceService.Create(env.Ctx, document)
		require.NoError(t, err)
		_, err = acceptanceService.Sign(env.Ctx, created.ProjectID(), created.ID())
		require.NoError(t, err)
	}
	_, err = acceptanceService.Create(env.Ctx, acceptance.New(
		first.ID(), acceptance.KindOther, "D-1", date, money.New(99900, "USD"), acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	account, err := itf.GetService[financeservices.MoneyAccountService](env).Create(env.Ctx, moneyaccount.New(
		"Cash", money.New(0, "USD"), moneyaccount.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	category, err := itf.GetService[financeservices.PaymentCategoryService](env).Create(env.Ctx, paymentcategory.New(
		"Sales", paymentcategory.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	paid, err := itf.GetService[financeservices.PaymentService](env).Create(env.Ctx, payment.New(
		money.New(30000, "USD"),
		category,
		payment.WithTenantID(env.Tenant.ID),
		payment.WithAccount(account),
		payment.WithCounterpartyID(client.ID()),
		payment.WithTransactionDate(date),
		payment.WithAccountingPeriod(date),
	))
	require.NoError(t, err)
	stage := projectstage.New(first.ID(), 1, 100000)
	require.NoError(t, itf.GetService[services.ProjectStageService](env).Create(env.Ctx, stage))
	_, err = env.Tx.Exec(env.Ctx,
		"INSERT INTO project_stage_payments (project_stage_id, payment_id) VALUES ($1, $2)", stage.ID(), paid.ID())
	require.NoError(t, err)

	revenueService := services.NewRevenueService(
		persistence.NewProjectRepository(),
		persistence.NewAcceptanceRepository(),
		persistence.NewProjectStageRepository(),
		invoicesStub{money.New(35000, "USD")},
	)

	lines, err := revenueService.ClientRevenue(env.Ctx, client.ID())
	require.NoError(t, err)
	require.Len(t, lines, 2)

	eur, usd := lines[0], lines[1]
	require.Equal(t, "EUR", eur.Contract.Currency().Code)
	require.Equal(t, int64(0), eur.Contract.Amount())
	require.Equal(t, int64(20000), eur.Accepted.Amount())
	require.Equal(t, int64(-20000), eur.Open.Amount())

	require.Equal(t, "USD", usd.Contract.Currency().Code)
	require.Equal(t, int64(150000), usd.Contract.Amount())
	require.Equal(t, int64(40000), usd.Accepted.Amount())
	require.Equal(t, int64(110000), usd.Open.Amount())
	require.Equal(t, int64(35000), usd.Invoiced.Amount())
	require.Equal(t, int64(30000), usd.Paid.Amount())
}

func TestAcceptanceService_StatusChangeChecksStoredStatus(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithComponents(
		core.NewComponent(&core.ModuleOptions{PermissionSchema: defaults.PermissionSchema()}),
		finance.NewComponent(),
		projects.NewComponent(),
	), itf.WithUser(itf.User()))

	require.NoError(t, itf.GetService[coreservices.CurrencyService](env).Create(env.Ctx, &currency.CreateDTO{
		Code:   string(currency.USD.Code()),
		Name:   currency.USD.Name(),
		Symbol: string(currency.USD.Symbol()),
	}))
	client, err := itf.GetService[financeservices.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"Status Client", counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	p := project.New("Status Project", client.ID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(env.Ctx, p))

	acceptanceService := itf.GetService[services.AcceptanceService](env)
	draft, err := acceptanceService.Create(env.Ctx, acceptance.New(
		p.ID(), acceptance.KindAct, "S-1", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		money.New(10000, "USD"), acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	_, err = acceptanceService.Cancel(env.Ctx, p.ID(), draft.ID())
	require.NoError(t, err)

	// A sign that read the draft before the cancel must not revive it.
	_, err = persistence.NewAcceptanceRepository().UpdateStatus(
		env.Ctx, draft.UpdateStatus(acceptance.StatusSigned), acceptance.StatusDraft,
	)
	require.ErrorIs(t, err, acceptance.ErrStatus)

	stored, err := acceptanceService.GetByID(env.Ctx, draft.ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.StatusCancelled, stored.Status())
}
