package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/logging"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/testutils"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// testFixtures contains common test dependencies
type testFixtures struct {
	ctx             context.Context
	pool            *pgxpool.Pool
	publisher       eventbus.EventBus
	paymentsService *services.PaymentService
	accountService  *services.MoneyAccountService
}

// setupTest creates all necessary dependencies for tests
func setupTest(t *testing.T, permissions ...*permission.Permission) *testFixtures {
	t.Helper()

	testutils.CreateDB(t.Name())
	pool := testutils.NewPool(testutils.DbOpts(t.Name()))

	ctx := composables.WithUser(context.Background(), testutils.MockUser(permissions...))
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		pool.Close()
	})

	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithSession(ctx, &session.Session{})

	publisher := eventbus.NewEventPublisher(logging.ConsoleLogger(logrus.WarnLevel))
	app := setupApplication(t, pool, publisher)

	// Run migrations to ensure all tables are created (including tenants table)
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}

	// Create a test tenant and add it to the context (after migrations have created the table)
	tenant, err := testutils.CreateTestTenant(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx = composables.WithTenantID(ctx, tenant.ID)

	return &testFixtures{
		ctx:             ctx,
		pool:            pool,
		publisher:       publisher,
		paymentsService: app.Service(services.PaymentService{}).(*services.PaymentService),
		accountService:  app.Service(services.MoneyAccountService{}).(*services.MoneyAccountService),
	}
}

// setupApplication initializes and configures the application
func setupApplication(t *testing.T, pool *pgxpool.Pool, publisher eventbus.EventBus) application.Application {
	t.Helper()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		EventBus: publisher,
		Logger:   logging.ConsoleLogger(logrus.WarnLevel),
	})
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		t.Fatal(err)
	}
	if err := app.Migrations().Run(); err != nil {
		t.Fatal(err)
	}
	return app
}

// setupTestData creates necessary test data
func setupTestData(ctx context.Context, t *testing.T, f *testFixtures) {
	t.Helper()

	// Create currency
	currencyRepo := corepersistence.NewCurrencyRepository()
	if err := currencyRepo.Create(ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}

	// Create account
	account := moneyaccount.New(
		"Test",
		currency.USD,
		moneyaccount.WithAccountNumber("123"),
		moneyaccount.WithBalance(100),
	)
	err := f.accountService.Create(ctx, account)
	if err != nil {
		t.Fatal(err)
	}

	// Create counterparty
	counterpartyRepo := persistence.NewCounterpartyRepository()
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}

	// Create the counterparty - the repository itself will set the tenant ID
	_, err = counterpartyRepo.Create(ctx, counterparty.New(
		tin,
		"Test",
		counterparty.Customer,
		counterparty.LLC,
		"",
	))
	if err != nil {
		t.Fatal(err)
	}
}

func TestPaymentsService_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t,
		permissions.PaymentCreate,
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentDelete,
	)
	setupTestData(f.ctx, t, f)
	accountRepository := persistence.NewMoneyAccountRepository()

	// Create payment category
	category := paymentcategory.New("Test Category")

	// Create payment entity
	paymentEntity := payment.New(
		100,
		category,
		payment.WithCounterpartyID(1),
		payment.WithTransactionDate(time.Now()),
		payment.WithAccountingPeriod(time.Now()),
	)

	if err := f.paymentsService.Create(f.ctx, paymentEntity); err != nil {
		t.Fatal(err)
	}

	accountEntity, err := accountRepository.GetByID(f.ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance() != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance())
	}
}
