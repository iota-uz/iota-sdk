package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// setupTestData creates necessary test data and returns account and counterparty
func setupTestData(ctx context.Context, t *testing.T, f *itf.TestEnvironment) (moneyaccount.Account, counterparty.Counterparty) {
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
	account, err := getAccountService(f).Create(ctx, account)
	if err != nil {
		t.Fatal(err)
	}

	// Create counterparty
	counterpartyRepo := persistence.NewCounterpartyRepository()
	tin, err := tax.NewTin("123456789", country.Uzbekistan)
	if err != nil {
		t.Fatal(err)
	}

	// Get tenant ID from context
	testTenantID, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
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
	account, createdCounterparty := setupTestData(f.Ctx, t, f)
	accountRepository := persistence.NewMoneyAccountRepository()

	// Create payment category with tenant ID
	tenantID, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	category := paymentcategory.New("Test Category", paymentcategory.WithTenantID(tenantID))
	createdCategory, err := getPaymentCategoryService(f).Create(f.Ctx, category)
	if err != nil {
		t.Fatal(err)
	}

	// Create payment entity
	paymentEntity := payment.New(
		money.New(10000, "USD"),
		createdCategory,
		payment.WithTenantID(tenantID),
		payment.WithCounterpartyID(createdCounterparty.ID()),
		payment.WithTransactionDate(time.Now()),
		payment.WithAccountingPeriod(time.Now()),
		payment.WithAccount(account),
	)

	_, err = getPaymentService(f).Create(f.Ctx, paymentEntity)
	if err != nil {
		t.Fatal(err)
	}

	accountEntity, err := accountRepository.GetByID(f.Ctx, account.ID())
	if err != nil {
		t.Fatal(err)
	}
	if accountEntity.Balance().AsMajorUnits() != 200 {
		t.Fatalf("expected balance to be 200, got %f", accountEntity.Balance().AsMajorUnits())
	}
}
