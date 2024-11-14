package services_test

import (
	"context"
	moneyAccount "github.com/iota-agency/iota-erp/internal/domain/aggregates/money_account"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/payment"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	services2 "github.com/iota-agency/iota-erp/internal/services"
	"github.com/iota-agency/iota-erp/internal/testutils"
	"github.com/iota-agency/iota-erp/pkg/constants"
	"github.com/iota-agency/iota-erp/pkg/event"
	"testing"
	"time"
)

func TestPaymentsService_CRUD(t *testing.T) { //nolint:paralleltest
	ctx := testutils.GetTestContext()
	ctx.Context = context.WithValue(ctx.Context, constants.UserKey, &user.User{})
	ctx.Context = context.WithValue(ctx.Context, constants.SessionKey, &session.Session{})
	defer ctx.Tx.Commit()

	publisher := event.NewEventPublisher()
	currencyRepository := persistence.NewCurrencyRepository()
	accountRepository := persistence.NewMoneyAccountRepository()
	projectRepository := persistence.NewProjectRepository()
	stageRepository := persistence.NewProjectStageRepository()
	paymentRepository := persistence.NewPaymentRepository()
	accountService := services2.NewMoneyAccountService(accountRepository, publisher)
	paymentsService := services2.NewPaymentService(paymentRepository, publisher, accountService)

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
			ID:   1,
			Name: "test",
		},
	); err != nil {
		t.Fatal(err)
	}
	stageEntity := &stage.ProjectStage{
		ID:        1,
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

	accountEntity, err := accountRepository.GetByID(ctx.Context, 1)
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance)
	}
}
