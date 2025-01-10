package services_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
)

func TestPaymentsService_CRUD(t *testing.T) {
	testutils.CreateDB(t.Name())

	pool := testutils.NewPool(testutils.DbOpts(t.Name()))
	defer pool.Close()
	ctx := composables.WithUser(
		context.Background(),
		testutils.MockUser(
			permissions.PaymentCreate,
			permissions.PaymentRead,
			permissions.PaymentUpdate,
			permissions.PaymentDelete,
		),
	)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}()
	ctx = composables.WithTx(ctx, tx)
	ctx = composables.WithSession(ctx, &session.Session{})

	publisher := event.NewEventPublisher()
	app := application.New(pool, publisher)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		t.Fatal(err)
	}
	if err := app.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	currencyRepository := corepersistence.NewCurrencyRepository()
	accountRepository := persistence.NewMoneyAccountRepository()
	paymentRepository := persistence.NewPaymentRepository()
	counterpartyRepository := persistence.NewCounterpartyRepository()
	accountService := services.NewMoneyAccountService(
		accountRepository,
		persistence.NewTransactionRepository(),
		publisher,
	)
	paymentsService := services.NewPaymentService(paymentRepository, publisher, accountService)

	if err := currencyRepository.Create(ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if err := accountService.Create(
		ctx, &moneyaccount.CreateDTO{
			Name:          "Test",
			AccountNumber: "123",
			Balance:       100,
			CurrencyCode:  string(currency.UsdCode),
			Description:   "",
		},
	); err != nil {
		t.Fatal(err)
	}
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := counterpartyRepository.Create(
		ctx,
		counterparty.New(
			tin,
			"Test",
			counterparty.Customer,
			counterparty.LLC,
			"",
		),
	); err != nil {
		t.Fatal(err)
	}
	if err := paymentsService.Create(
		ctx, &payment.CreateDTO{
			Amount:           100,
			AccountID:        1,
			TransactionDate:  shared.DateOnly(time.Now()),
			AccountingPeriod: shared.DateOnly(time.Now()),
			CounterpartyID:   1,
		},
	); err != nil {
		t.Fatal(err)
	}

	accountEntity, err := accountRepository.GetByID(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance)
	}
}
