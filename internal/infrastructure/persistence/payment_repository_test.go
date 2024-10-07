package persistence

import (
	"os"
	"testing"

	"github.com/iota-agency/iota-erp/internal/configuration"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/internal/test_utils"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../"); err != nil {
		panic(err)
	}
	db, err := test_utils.DBSetup()
	if err != nil {
		panic(err)
	}

	code := m.Run()
	if err := db.Close(); err != nil {
		panic(err)
	}
	os.Exit(code)
}

func TestGormPaymentRepository_CRUD(t *testing.T) { //nolint:paralleltest
	currencyRepository := NewCurrencyRepository()
	accountRepository := NewMoneyAccountRepository()
	projectRepository := NewProjectRepository()
	stageRepository := NewProjectStageRepository()
	paymentRepository := NewPaymentRepository()
	ctx, tx, err := test_utils.GetTestContext(configuration.Use().DBOpts)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Commit()
	if err := currencyRepository.Create(ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if err := accountRepository.Create(ctx, &moneyAccount.Account{
		Id:            1,
		Name:          "test",
		AccountNumber: "123",
		Currency:      currency.USD,
		Balance:       100,
	}); err != nil {
		t.Fatal(err)
	}
	if err := projectRepository.Create(ctx, &project.Project{
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
	if err := stageRepository.Create(ctx, stageEntity); err != nil {
		t.Fatal(err)
	}
	if err := paymentRepository.Create(ctx, &payment.Payment{
		ID:           1,
		CurrencyCode: string(currency.UsdCode),
		StageID:      1,
		Amount:       100,
		AccountID:    1,
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("Count", func(t *testing.T) { //nolint:paralleltest
		count, err := paymentRepository.Count(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("expected 1, got %d", count)
		}
	})

	t.Run("GetPaginated", func(t *testing.T) { //nolint:paralleltest
		payments, err := paymentRepository.GetPaginated(ctx, 1, 0, []string{})
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
		payments, err := paymentRepository.GetAll(ctx)
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
		paymentEntity, err := paymentRepository.GetByID(ctx, 1)
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
