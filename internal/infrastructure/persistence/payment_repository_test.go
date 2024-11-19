package persistence_test

import (
	"github.com/iota-agency/iota-sdk/internal/testutils"
	"testing"
)

func TestGormPaymentRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer ctx.Tx.Commit()

	// TODO: implement this test
	//currencyRepository := persistence.NewCurrencyRepository()
	//accountRepository := persistence.NewMoneyAccountRepository()
	//paymentRepository := persistence.NewPaymentRepository()

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
	//if err := paymentRepository.Create(
	//	ctx.Context, &payment.Payment{
	//		ID:      1,
	//		StageID: 1,
	//		Amount:  100,
	//		Account: moneyAccount.Account{
	//			ID: 1,
	//		},
	//	},
	//); err != nil {
	//	t.Fatal(err)
	//}
	//
	//t.Run(
	//	//nolint:paralleltest
	//	"Count", func(t *testing.T) {
	//		count, err := paymentRepository.Count(ctx.Context)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		if count != 1 {
	//			t.Errorf("expected 1, got %d", count)
	//		}
	//	},
	//)
	//
	//t.Run(
	//	//nolint:paralleltest
	//	"GetPaginated", func(t *testing.T) {
	//		payments, err := paymentRepository.GetPaginated(ctx.Context, 1, 0, []string{})
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		if len(payments) != 1 {
	//			t.Errorf("expected 1, got %d", len(payments))
	//		}
	//		if payments[0].Amount != 100 {
	//			t.Errorf("expected 100, got %f", payments[0].Amount)
	//		}
	//	},
	//)
	//
	//t.Run(
	//	//nolint:paralleltest
	//	"GetAll", func(t *testing.T) {
	//		payments, err := paymentRepository.GetAll(ctx.Context)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		if len(payments) != 1 {
	//			t.Errorf("expected 1, got %d", len(payments))
	//		}
	//		if payments[0].Amount != 100 {
	//			t.Errorf("expected 100, got %f", payments[0].Amount)
	//		}
	//	},
	//)
	//
	//t.Run(
	//	//nolint:paralleltest
	//	"GetByID", func(t *testing.T) {
	//		paymentEntity, err := paymentRepository.GetByID(ctx.Context, 1)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		if paymentEntity.Amount != 100 {
	//			t.Errorf("expected 100, got %f", paymentEntity.Amount)
	//		}
	//	},
	//)
	//
	//t.Run(
	//	//nolint:paralleltest
	//	"Update", func(t *testing.T) {
	//		if err := paymentRepository.Update(
	//			ctx.Context, &payment.Payment{
	//				ID:     1,
	//				Amount: 200,
	//			},
	//		); err != nil {
	//			t.Fatal(err)
	//		}
	//		paymentEntity, err := paymentRepository.GetByID(ctx.Context, 1)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//		if paymentEntity.Amount != 200 {
	//			t.Errorf("expected 200, got %f", paymentEntity.Amount)
	//		}
	//	},
	//)
}
