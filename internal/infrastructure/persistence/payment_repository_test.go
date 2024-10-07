package persistence

import (
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/internal/testutils"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestGormPaymentRepository_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	currencyRepository := NewCurrencyRepository()
	accountRepository := NewMoneyAccountRepository()
	projectRepository := NewProjectRepository()
	stageRepository := NewProjectStageRepository()
	paymentRepository := NewPaymentRepository()

	if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if err := accountRepository.Create(ctx.Context, &moneyAccount.Account{
		Id:            1,
		Name:          "test",
		AccountNumber: "123",
		Currency:      currency.USD,
		Balance:       100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := projectRepository.Create(ctx.Context, &project.Project{
		Id:   1,
		Name: "test",
	}); err != nil {
		t.Fatal(err)
	}
	stageEntity := &stage.ProjectStage{
		Id:        1,
		Name:      "test",
		ProjectID: 1,
	}
	if err := stageRepository.Create(ctx.Context, stageEntity); err != nil {
		t.Fatal(err)
	}
	if err := paymentRepository.Create(ctx.Context, &payment.Payment{
		ID:           1,
		CurrencyCode: string(currency.UsdCode),
		StageID:      1,
		Amount:       100,
		AccountID:    1,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("Count", func(t *testing.T) { //nolint:paralleltest
		count, err := paymentRepository.Count(ctx.Context)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("expected 1, got %d", count)
		}
	})

	t.Run("GetPaginated", func(t *testing.T) { //nolint:paralleltest
		payments, err := paymentRepository.GetPaginated(ctx.Context, 1, 0, []string{})
		if err != nil {
			t.Fatal(err)
		}
		if len(payments) != 1 {
			t.Errorf("expected 1, got %d", len(payments))
		}
		if payments[0].Amount != 100 {
			t.Errorf("expected 100, got %f", payments[0].Amount)
		}
	})

	t.Run("GetAll", func(t *testing.T) { //nolint:paralleltest
		payments, err := paymentRepository.GetAll(ctx.Context)
		if err != nil {
			t.Fatal(err)
		}
		if len(payments) != 1 {
			t.Errorf("expected 1, got %d", len(payments))
		}
		if payments[0].Amount != 100 {
			t.Errorf("expected 100, got %f", payments[0].Amount)
		}
	})

	t.Run("GetByID", func(t *testing.T) { //nolint:paralleltest
		paymentEntity, err := paymentRepository.GetByID(ctx.Context, 1)
		if err != nil {
			t.Fatal(err)
		}
		if paymentEntity.Amount != 100 {
			t.Errorf("expected 100, got %f", paymentEntity.Amount)
		}
		if paymentEntity.CurrencyCode != string(currency.UsdCode) {
			t.Errorf("expected %s, got %s", currency.UsdCode, paymentEntity.CurrencyCode)
		}
	})

	// t.Run("Update", func(t *testing.T) {
	//	if err := paymentRepository.Update(ctx, &payment.Payment{
	//		ID:     1,
	//		Amount: 200,
	//	}); err != nil {
	//		t.Fatal(err)
	//	}
	//	paymentEntity, err := paymentRepository.GetByID(ctx, 1)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	if paymentEntity.Amount != 200 {
	//		t.Errorf("expected 200, got %f", paymentEntity.Amount)
	//	}
	//	if paymentEntity.CurrencyCode != string(currency.UsdCode) {
	//		t.Errorf("expected %s, got %s", currency.UsdCode, paymentEntity.CurrencyCode)
	//	}
	// })
}
