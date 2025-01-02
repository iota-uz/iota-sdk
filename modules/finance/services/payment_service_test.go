package services_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	persistence2 "github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/jackc/pgx/v5"
	"log"
	"testing"
	"time"
)

func TestPaymentsService_CRUD(t *testing.T) {
	ctx := testutils.GetTestContext()
	ctx.Context = context.WithValue(ctx.Context, constants.UserKey, &user.User{
		Roles: []*role.Role{
			{
				Permissions: []permission.Permission{
					permissions.PaymentCreate,
					permissions.PaymentRead,
					permissions.PaymentUpdate,
					permissions.PaymentDelete,
				},
			},
		},
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
	accountRepository := persistence2.NewMoneyAccountRepository()
	paymentRepository := persistence2.NewPaymentRepository()
	accountService := services.NewMoneyAccountService(
		accountRepository,
		persistence2.NewTransactionRepository(),
		publisher,
	)
	paymentsService := services.NewPaymentService(paymentRepository, publisher, accountService)

	if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if _, err := accountRepository.Create(
		ctx.Context, &moneyAccount.Account{
			ID:            1,
			Name:          "Test",
			AccountNumber: "123",
			Currency:      currency.USD,
			Balance:       100,
			Description:   "",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := paymentsService.Create(
		ctx.Context, &payment.CreateDTO{
			Amount:    100,
			AccountID: 1,
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
