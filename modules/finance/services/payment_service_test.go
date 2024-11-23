package services_test

import (
	"context"
	"github.com/iota-agency/iota-sdk/modules/finance/permissions"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/testutils"
	"testing"
)

func TestPaymentsService_CRUD(t *testing.T) { //nolint:paralleltest
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
	// TODO: implement this test
	//ctx.Context = context.WithValue(ctx.Context, constants.SessionKey, &session.Session{})
	//defer ctx.Tx.Commit()
	//
	//publisher := event.NewEventPublisher()
	//currencyRepository := persistence.NewCurrencyRepository()
	//accountRepository := persistence.NewMoneyAccountRepository()
	//paymentRepository := persistence.NewPaymentRepository()
	//accountService := services.NewMoneyAccountService(accountRepository, publisher)
	//paymentsService := services.NewPaymentService(paymentRepository, publisher, accountService)
	//
	//if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
	//	t.Fatal(err)
	//}
	//if err := accountRepository.Create(
	//	ctx.Context, &moneyAccount.Account{
	//		ID:            1,
	//		Name:          "test",
	//		AccountNumber: "123",
	//		Currency:      currency.USD,
	//		Balance:       100,
	//		Description:   "",
	//		CreatedAt:     time.Now(),
	//		UpdatedAt:     time.Now(),
	//	},
	//); err != nil {
	//	t.Fatal(err)
	//}
	//if err := paymentsService.Create(
	//	ctx.Context, &payment.CreateDTO{
	//		StageID:   1,
	//		Amount:    100,
	//		AccountID: 1,
	//	},
	//); err != nil {
	//	t.Fatal(err)
	//}
	//
	//accountEntity, err := accountRepository.GetByID(ctx.Context, 1)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//if accountEntity.Balance != 200 {
	//	t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance)
	//}
}
