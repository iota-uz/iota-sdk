package services_test

import (
	"github.com/iota-agency/iota-erp/internal/app/services"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/internal/testutils"
	"github.com/iota-agency/iota-erp/sdk/event"
	"testing"
	"time"
)

func TestPaymentsService_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	defer ctx.Tx.Commit()

	publisher := event.NewEventPublisher()
	currencyRepository := persistence.NewCurrencyRepository()
	accountRepository := persistence.NewMoneyAccountRepository()
	projectRepository := persistence.NewProjectRepository()
	stageRepository := persistence.NewProjectStageRepository()
	paymentRepository := persistence.NewPaymentRepository()
	paymentsService := services.NewPaymentService(paymentRepository, publisher)

	if err := currencyRepository.Create(ctx.Context, &currency.USD); err != nil {
		t.Fatal(err)
	}
	if err := accountRepository.Create(
		ctx.Context, &moneyAccount.Account{
			ID:            1,
			Name:          "test",
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
	if err := projectRepository.Create(
		ctx.Context, &project.Project{
			Id:   1,
			Name: "test",
		},
	); err != nil {
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
	if err := paymentsService.Create(
		ctx.Context, &payment.CreateDTO{
			CurrencyCode: string(currency.UsdCode),
			StageID:      1,
			Amount:       100,
			AccountID:    1,
		},
	); err != nil {
		t.Fatal(err)
	}
}
