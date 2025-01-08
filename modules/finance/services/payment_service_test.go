package services_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"log"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
)

func TestPaymentsService_CRUD(t *testing.T) {
	ctx := testutils.GetTestContext()
	r, err := role.New(
		"",
		"",
		[]*permission.Permission{
			permissions.PaymentCreate,
			permissions.PaymentRead,
			permissions.PaymentUpdate,
			permissions.PaymentDelete,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Context = context.WithValue(ctx.Context, constants.UserKey, &user.User{
		Roles: []role.Role{r},
	})
	ctx.Context = context.WithValue(ctx.Context, constants.SessionKey, &session.Session{})
	defer func(Tx pgx.Tx, ctx context.Context) {
		err := Tx.Commit(ctx)
		if err != nil {
			log.Fatal(err)
		}
	}(ctx.Tx, ctx.Context)

	publisher := event.NewEventPublisher()
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

	if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if err := accountService.Create(
		ctx.Context, &moneyaccount.CreateDTO{
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
		ctx.Context,
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
		ctx.Context, &payment.CreateDTO{
			Amount:           100,
			AccountID:        1,
			TransactionDate:  shared.DateOnly(time.Now()),
			AccountingPeriod: shared.DateOnly(time.Now()),
			CounterpartyID:   1,
		},
	); err != nil {
		t.Fatal(err)
	}

	accountEntity, err := accountRepository.GetByID(ctx.Context, 1)
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance)
	}
}
