package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/money"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
)

// setupTestData creates necessary test data and returns account and counterparty
func setupTestData(ctx context.Context, t *testing.T, f *testFixtures) (moneyaccount.Account, counterparty.Counterparty) {
	t.Helper()

	// Create currency
	currencyRepo := corepersistence.NewCurrencyRepository()
	if err := currencyRepo.Create(ctx, &currency.USD); err != nil {
		t.Fatal(err)
	}

	// Create account through service
	account := moneyaccount.New(
		"Test",
		money.New(10000, "USD"),
		moneyaccount.WithAccountNumber("123"),
	)
	account, err := f.accountService.Create(ctx, account)
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
	testTenantID := uuid.New()
	createdCounterparty, err := counterpartyRepo.Create(ctx, counterparty.New(
		"Test",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(testTenantID),
		counterparty.WithTin(tin),
		counterparty.WithLegalAddress(""),
	))
	if err != nil {
		t.Fatal(err)
	}

	return account, createdCounterparty
}

func TestPaymentsService_CRUD(t *testing.T) {
	t.Parallel()
	f := setupTest(t,
		permissions.PaymentCreate,
		permissions.PaymentRead,
		permissions.PaymentUpdate,
		permissions.PaymentDelete,
	)
	account, createdCounterparty := setupTestData(f.ctx, t, f)
	accountRepository := persistence.NewMoneyAccountRepository()

	// Create payment category with tenant ID
	category := paymentcategory.New("Test Category", paymentcategory.WithTenantID(f.tenantID))

	// Create payment entity
	paymentEntity := payment.New(
		money.New(10000, "USD"),
		category,
		payment.WithTenantID(f.tenantID),
		payment.WithCounterpartyID(createdCounterparty.ID()),
		payment.WithTransactionDate(time.Now()),
		payment.WithAccountingPeriod(time.Now()),
		payment.WithAccount(account),
	)

	if err := f.paymentsService.Create(f.ctx, paymentEntity); err != nil {
		t.Fatal(err)
	}

	accountEntity, err := accountRepository.GetByID(f.ctx, account.ID())
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance().AsMajorUnits() != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance().AsMajorUnits())
	}
}
