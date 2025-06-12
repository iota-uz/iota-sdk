package persistence_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToDBTransaction(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()
	originAccountID := uuid.New()
	destinationAccountID := uuid.New()

	tests := []struct {
		name        string
		transaction transaction.Transaction
		validateFn  func(t *testing.T, dbTransaction *models.Transaction)
	}{
		{
			name: "transaction with both origin and destination accounts",
			transaction: transaction.New(
				money.NewFromFloat(100.50, "USD"),
				transaction.Transfer,
				transaction.WithID(uuid.New()),
				transaction.WithTenantID(testTenantID),
				transaction.WithComment("Test transfer"),
				transaction.WithOriginAccountID(originAccountID),
				transaction.WithDestinationAccountID(destinationAccountID),
				transaction.WithCreatedAt(now),
			),
			validateFn: func(t *testing.T, dbTransaction *models.Transaction) {
				t.Helper()

				assert.Equal(t, int64(10050), dbTransaction.Amount, "Amount should match")
				assert.Equal(t, testTenantID.String(), dbTransaction.TenantID, "TenantID should match")
				assert.Equal(t, "Test transfer", dbTransaction.Comment, "Comment should match")
				assert.Equal(t, string(transaction.Transfer), dbTransaction.TransactionType, "TransactionType should match")

				assert.True(t, dbTransaction.OriginAccountID.Valid, "OriginAccountID should be valid")
				assert.Equal(t, originAccountID.String(), dbTransaction.OriginAccountID.String, "OriginAccountID should match")

				assert.True(t, dbTransaction.DestinationAccountID.Valid, "DestinationAccountID should be valid")
				assert.Equal(t, destinationAccountID.String(), dbTransaction.DestinationAccountID.String, "DestinationAccountID should match")
			},
		},
		{
			name: "transaction with nil origin account (initial balance)",
			transaction: transaction.New(
				money.NewFromFloat(500.00, "USD"),
				transaction.Deposit,
				transaction.WithID(uuid.New()),
				transaction.WithTenantID(testTenantID),
				transaction.WithComment("Initial balance"),
				transaction.WithOriginAccountID(uuid.Nil),
				transaction.WithDestinationAccountID(destinationAccountID),
				transaction.WithCreatedAt(now),
			),
			validateFn: func(t *testing.T, dbTransaction *models.Transaction) {
				t.Helper()

				assert.Equal(t, int64(50000), dbTransaction.Amount, "Amount should match")
				assert.Equal(t, "Initial balance", dbTransaction.Comment, "Comment should match")
				assert.Equal(t, string(transaction.Deposit), dbTransaction.TransactionType, "TransactionType should match")

				assert.False(t, dbTransaction.OriginAccountID.Valid, "OriginAccountID should be NULL for uuid.Nil")

				assert.True(t, dbTransaction.DestinationAccountID.Valid, "DestinationAccountID should be valid")
				assert.Equal(t, destinationAccountID.String(), dbTransaction.DestinationAccountID.String, "DestinationAccountID should match")
			},
		},
		{
			name: "withdrawal transaction with nil destination account",
			transaction: transaction.New(
				money.NewFromFloat(250.75, "USD"),
				transaction.Withdrawal,
				transaction.WithID(uuid.New()),
				transaction.WithTenantID(testTenantID),
				transaction.WithComment("Cash withdrawal"),
				transaction.WithOriginAccountID(originAccountID),
				transaction.WithDestinationAccountID(uuid.Nil),
				transaction.WithCreatedAt(now),
			),
			validateFn: func(t *testing.T, dbTransaction *models.Transaction) {
				t.Helper()

				assert.Equal(t, int64(25075), dbTransaction.Amount, "Amount should match")
				assert.Equal(t, "Cash withdrawal", dbTransaction.Comment, "Comment should match")
				assert.Equal(t, string(transaction.Withdrawal), dbTransaction.TransactionType, "TransactionType should match")

				assert.True(t, dbTransaction.OriginAccountID.Valid, "OriginAccountID should be valid")
				assert.Equal(t, originAccountID.String(), dbTransaction.OriginAccountID.String, "OriginAccountID should match")

				assert.False(t, dbTransaction.DestinationAccountID.Valid, "DestinationAccountID should be NULL for uuid.Nil")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := persistence.ToDBTransaction(tt.transaction)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDomainTransaction(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()
	originAccountID := uuid.New()
	destinationAccountID := uuid.New()

	tests := []struct {
		name          string
		dbTransaction *models.Transaction
		wantErr       bool
		validateFn    func(t *testing.T, transaction transaction.Transaction)
	}{
		{
			name: "transaction with both accounts",
			dbTransaction: &models.Transaction{
				ID:                   uuid.New().String(),
				TenantID:             testTenantID.String(),
				Amount:               7525,
				OriginAccountID:      mapping.UUIDToSQLNullString(originAccountID),
				DestinationAccountID: mapping.UUIDToSQLNullString(destinationAccountID),
				TransactionDate:      now,
				AccountingPeriod:     now,
				TransactionType:      string(transaction.Transfer),
				Comment:              "Test transaction",
				CreatedAt:            now,
			},
			validateFn: func(t *testing.T, tr transaction.Transaction) {
				t.Helper()

				assert.Equal(t, 75.25, tr.Amount().AsMajorUnits(), "Amount should match")
				assert.Equal(t, testTenantID, tr.TenantID(), "TenantID should match")
				assert.Equal(t, "Test transaction", tr.Comment(), "Comment should match")
				assert.Equal(t, transaction.Transfer, tr.TransactionType(), "TransactionType should match")
				assert.Equal(t, originAccountID, tr.OriginAccountID(), "OriginAccountID should match")
				assert.Equal(t, destinationAccountID, tr.DestinationAccountID(), "DestinationAccountID should match")
			},
		},
		{
			name: "transaction with null origin account",
			dbTransaction: &models.Transaction{
				ID:                   uuid.New().String(),
				TenantID:             testTenantID.String(),
				Amount:               15000,
				OriginAccountID:      sql.NullString{Valid: false},
				DestinationAccountID: mapping.UUIDToSQLNullString(destinationAccountID),
				TransactionDate:      now,
				AccountingPeriod:     now,
				TransactionType:      string(transaction.Deposit),
				Comment:              "Initial deposit",
				CreatedAt:            now,
			},
			validateFn: func(t *testing.T, tr transaction.Transaction) {
				t.Helper()

				assert.Equal(t, 150.00, tr.Amount().AsMajorUnits(), "Amount should match")
				assert.Equal(t, "Initial deposit", tr.Comment(), "Comment should match")
				assert.Equal(t, transaction.Deposit, tr.TransactionType(), "TransactionType should match")
				assert.Equal(t, uuid.Nil, tr.OriginAccountID(), "OriginAccountID should be uuid.Nil for NULL")
				assert.Equal(t, destinationAccountID, tr.DestinationAccountID(), "DestinationAccountID should match")
			},
		},
		{
			name: "transaction with invalid transaction type",
			dbTransaction: &models.Transaction{
				ID:                   uuid.New().String(),
				TenantID:             testTenantID.String(),
				Amount:               5000,
				OriginAccountID:      mapping.UUIDToSQLNullString(originAccountID),
				DestinationAccountID: mapping.UUIDToSQLNullString(destinationAccountID),
				TransactionDate:      now,
				AccountingPeriod:     now,
				TransactionType:      "invalid_type",
				Comment:              "Test transaction",
				CreatedAt:            now,
			},
			wantErr: true,
		},
		{
			name: "transaction with invalid tenant ID",
			dbTransaction: &models.Transaction{
				ID:                   uuid.New().String(),
				TenantID:             "invalid-uuid",
				Amount:               5000,
				OriginAccountID:      mapping.UUIDToSQLNullString(originAccountID),
				DestinationAccountID: mapping.UUIDToSQLNullString(destinationAccountID),
				TransactionDate:      now,
				AccountingPeriod:     now,
				TransactionType:      string(transaction.Transfer),
				Comment:              "Test transaction",
				CreatedAt:            now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := persistence.ToDomainTransaction(tt.dbTransaction)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				return
			}

			require.NoError(t, err, "Should not return an error")
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDBExpenseCategory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()

	tests := []struct {
		name       string
		category   category.ExpenseCategory
		validateFn func(t *testing.T, dbCategory *models.ExpenseCategory)
	}{
		{
			name: "expense category with description",
			category: category.New(
				"Office Supplies",
				category.WithID(uuid.New()),
				category.WithTenantID(testTenantID),
				category.WithDescription("Office supplies and materials"),
				category.WithCreatedAt(now),
				category.WithUpdatedAt(now),
			),
			validateFn: func(t *testing.T, dbCategory *models.ExpenseCategory) {
				t.Helper()

				assert.Equal(t, "Office Supplies", dbCategory.Name, "Name should match")
				assert.Equal(t, testTenantID.String(), dbCategory.TenantID, "TenantID should match")
				assert.True(t, dbCategory.Description.Valid, "Description should be valid")
				assert.Equal(t, "Office supplies and materials", dbCategory.Description.String, "Description should match")
			},
		},
		{
			name: "expense category without description",
			category: category.New(
				"Travel Expenses",
				category.WithID(uuid.New()),
				category.WithTenantID(testTenantID),
				category.WithCreatedAt(now),
				category.WithUpdatedAt(now),
			),
			validateFn: func(t *testing.T, dbCategory *models.ExpenseCategory) {
				t.Helper()

				assert.Equal(t, "Travel Expenses", dbCategory.Name, "Name should match")
				assert.Equal(t, testTenantID.String(), dbCategory.TenantID, "TenantID should match")
				assert.False(t, dbCategory.Description.Valid, "Description should be NULL for empty string")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := persistence.ToDBExpenseCategory(tt.category)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDomainExpenseCategory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()

	tests := []struct {
		name       string
		dbCategory *models.ExpenseCategory
		wantErr    bool
		validateFn func(t *testing.T, category category.ExpenseCategory)
	}{
		{
			name: "expense category with description",
			dbCategory: &models.ExpenseCategory{
				ID:          uuid.New().String(),
				TenantID:    testTenantID.String(),
				Name:        "Marketing",
				Description: sql.NullString{String: "Marketing and advertising expenses", Valid: true},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			validateFn: func(t *testing.T, cat category.ExpenseCategory) {
				t.Helper()

				assert.Equal(t, "Marketing", cat.Name(), "Name should match")
				assert.Equal(t, testTenantID, cat.TenantID(), "TenantID should match")
				assert.Equal(t, "Marketing and advertising expenses", cat.Description(), "Description should match")
			},
		},
		{
			name: "expense category without description",
			dbCategory: &models.ExpenseCategory{
				ID:          uuid.New().String(),
				TenantID:    testTenantID.String(),
				Name:        "Utilities",
				Description: sql.NullString{Valid: false},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			validateFn: func(t *testing.T, cat category.ExpenseCategory) {
				t.Helper()

				assert.Equal(t, "Utilities", cat.Name(), "Name should match")
				assert.Equal(t, testTenantID, cat.TenantID(), "TenantID should match")
				assert.Equal(t, "", cat.Description(), "Description should be empty")
			},
		},
		{
			name: "expense category with invalid tenant ID",
			dbCategory: &models.ExpenseCategory{
				ID:          uuid.New().String(),
				TenantID:    "invalid-uuid",
				Name:        "Test Category",
				Description: sql.NullString{Valid: false},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := persistence.ToDomainExpenseCategory(tt.dbCategory)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				return
			}

			require.NoError(t, err, "Should not return an error")
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDBMoneyAccount(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()

	tests := []struct {
		name       string
		account    moneyaccount.Account
		validateFn func(t *testing.T, dbAccount *models.MoneyAccount)
	}{
		{
			name: "complete money account",
			account: moneyaccount.New(
				"Main Checking Account",
				money.NewFromFloat(1500.75, "USD"),
				moneyaccount.WithID(uuid.New()),
				moneyaccount.WithTenantID(testTenantID),
				moneyaccount.WithAccountNumber("123456789"),
				moneyaccount.WithDescription("Primary business checking account"),
				moneyaccount.WithCreatedAt(now),
				moneyaccount.WithUpdatedAt(now),
			),
			validateFn: func(t *testing.T, dbAccount *models.MoneyAccount) {
				t.Helper()

				assert.Equal(t, "Main Checking Account", dbAccount.Name, "Name should match")
				assert.Equal(t, testTenantID.String(), dbAccount.TenantID, "TenantID should match")
				assert.Equal(t, "123456789", dbAccount.AccountNumber, "AccountNumber should match")
				assert.Equal(t, int64(150075), dbAccount.Balance, "Balance should match")
				assert.Equal(t, "USD", dbAccount.BalanceCurrencyID, "BalanceCurrencyID should match")
				assert.Equal(t, "Primary business checking account", dbAccount.Description, "Description should match")
			},
		},
		{
			name: "minimal money account",
			account: moneyaccount.New(
				"Savings Account",
				money.New(0, "USD"),
				moneyaccount.WithID(uuid.New()),
				moneyaccount.WithTenantID(testTenantID),
				moneyaccount.WithAccountNumber("987654321"),
				moneyaccount.WithCreatedAt(now),
				moneyaccount.WithUpdatedAt(now),
			),
			validateFn: func(t *testing.T, dbAccount *models.MoneyAccount) {
				t.Helper()

				assert.Equal(t, "Savings Account", dbAccount.Name, "Name should match")
				assert.Equal(t, "987654321", dbAccount.AccountNumber, "AccountNumber should match")
				assert.Equal(t, int64(0), dbAccount.Balance, "Balance should match")
				assert.Equal(t, "", dbAccount.Description, "Description should be empty")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := persistence.ToDBMoneyAccount(tt.account)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDomainMoneyAccount(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()

	tests := []struct {
		name       string
		dbAccount  *models.MoneyAccount
		wantErr    bool
		validateFn func(t *testing.T, account moneyaccount.Account)
	}{
		{
			name: "complete money account",
			dbAccount: &models.MoneyAccount{
				ID:                uuid.New().String(),
				TenantID:          testTenantID.String(),
				Name:              "Business Account",
				AccountNumber:     "ACC-001",
				Description:       "Main business account",
				Balance:           250050,
				BalanceCurrencyID: "USD",
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			validateFn: func(t *testing.T, account moneyaccount.Account) {
				t.Helper()

				assert.Equal(t, "Business Account", account.Name(), "Name should match")
				assert.Equal(t, testTenantID, account.TenantID(), "TenantID should match")
				assert.Equal(t, "ACC-001", account.AccountNumber(), "AccountNumber should match")
				assert.Equal(t, "Main business account", account.Description(), "Description should match")
				assert.Equal(t, 2500.50, account.Balance().AsMajorUnits(), "Balance should match")
				assert.Equal(t, "USD", account.Balance().Currency().Code, "Currency code should match")
			},
		},
		{
			name: "money account with invalid tenant ID",
			dbAccount: &models.MoneyAccount{
				ID:                uuid.New().String(),
				TenantID:          "invalid-uuid",
				Name:              "Test Account",
				AccountNumber:     "ACC-002",
				Balance:           10000,
				BalanceCurrencyID: "USD",
				CreatedAt:         now,
				UpdatedAt:         now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := persistence.ToDomainMoneyAccount(tt.dbAccount)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				return
			}

			require.NoError(t, err, "Should not return an error")
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDBCounterparty(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTin, _ := tax.NewTin("123456789", country.Uzbekistan)

	tests := []struct {
		name         string
		counterparty counterparty.Counterparty
		wantErr      bool
		validateFn   func(t *testing.T, dbCounterparty *models.Counterparty)
	}{
		{
			name: "complete counterparty",
			counterparty: counterparty.New(
				"Test Company LLC",
				counterparty.Customer,
				counterparty.LLC,
				counterparty.WithID(uuid.New()),
				counterparty.WithTenantID(uuid.New()),
				counterparty.WithTin(testTin),
				counterparty.WithLegalAddress("123 Business St, Tashkent"),
				counterparty.WithCreatedAt(now),
				counterparty.WithUpdatedAt(now),
			),
			validateFn: func(t *testing.T, dbCounterparty *models.Counterparty) {
				t.Helper()

				assert.Equal(t, "123456789", dbCounterparty.Tin.String, "TIN should match")
				assert.Equal(t, "Test Company LLC", dbCounterparty.Name, "Name should match")
				assert.Equal(t, string(counterparty.Customer), dbCounterparty.Type, "Type should match")
				assert.Equal(t, string(counterparty.LLC), dbCounterparty.LegalType, "LegalType should match")
				assert.Equal(t, "123 Business St, Tashkent", dbCounterparty.LegalAddress, "LegalAddress should match")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := persistence.ToDBCounterparty(tt.counterparty)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				return
			}

			require.NoError(t, err, "Should not return an error")
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestToDomainCounterparty(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name           string
		dbCounterparty *models.Counterparty
		wantErr        bool
		validateFn     func(t *testing.T, counterparty counterparty.Counterparty)
	}{
		{
			name: "complete counterparty",
			dbCounterparty: &models.Counterparty{
				ID:           uuid.New().String(),
				Tin:          sql.NullString{String: "123456789", Valid: true},
				Name:         "Supplier Corp",
				Type:         string(counterparty.Supplier),
				LegalType:    string(counterparty.JSC),
				LegalAddress: "456 Supplier Ave, Samarkand",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			validateFn: func(t *testing.T, cp counterparty.Counterparty) {
				t.Helper()

				assert.Equal(t, "123456789", cp.Tin().Value(), "TIN should match")
				assert.Equal(t, "Supplier Corp", cp.Name(), "Name should match")
				assert.Equal(t, counterparty.Supplier, cp.Type(), "Type should match")
				assert.Equal(t, counterparty.JSC, cp.LegalType(), "LegalType should match")
				assert.Equal(t, "456 Supplier Ave, Samarkand", cp.LegalAddress(), "LegalAddress should match")
			},
		},
		{
			name: "counterparty with invalid type",
			dbCounterparty: &models.Counterparty{
				ID:           uuid.New().String(),
				Tin:          sql.NullString{String: "123456789012", Valid: true},
				Name:         "Test Corp",
				Type:         "invalid_type",
				LegalType:    string(counterparty.LLC),
				LegalAddress: "Test Address",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			wantErr: true,
		},
		{
			name: "counterparty with invalid legal type",
			dbCounterparty: &models.Counterparty{
				ID:           uuid.New().String(),
				Tin:          sql.NullString{String: "123456789012", Valid: true},
				Name:         "Test Corp",
				Type:         string(counterparty.Customer),
				LegalType:    "invalid_legal_type",
				LegalAddress: "Test Address",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			wantErr: true,
		},
		{
			name: "counterparty with invalid TIN",
			dbCounterparty: &models.Counterparty{
				ID:           uuid.New().String(),
				Tin:          sql.NullString{String: "invalid_tin", Valid: true},
				Name:         "Test Corp",
				Type:         string(counterparty.Customer),
				LegalType:    string(counterparty.LLC),
				LegalAddress: "Test Address",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := persistence.ToDomainCounterparty(tt.dbCounterparty)

			if tt.wantErr {
				assert.Error(t, err, "Expected an error")
				return
			}

			require.NoError(t, err, "Should not return an error")
			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

// Round-trip tests to ensure data integrity
func TestTransactionRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()
	originAccountID := uuid.New()
	destinationAccountID := uuid.New()

	original := transaction.New(
		money.NewFromFloat(1000.50, "USD"),
		transaction.Transfer,
		transaction.WithID(uuid.New()),
		transaction.WithTenantID(testTenantID),
		transaction.WithComment("Round trip test"),
		transaction.WithOriginAccountID(originAccountID),
		transaction.WithDestinationAccountID(destinationAccountID),
		transaction.WithTransactionDate(now),
		transaction.WithAccountingPeriod(now),
		transaction.WithCreatedAt(now),
	)

	// Convert to DB model
	dbTransaction := persistence.ToDBTransaction(original)

	// Convert back to domain model
	result, err := persistence.ToDomainTransaction(dbTransaction)
	require.NoError(t, err, "Round trip should not fail")

	// Verify all fields match
	assert.Equal(t, original.Amount().Amount(), result.Amount().Amount(), "Amount should match")
	assert.Equal(t, original.TenantID(), result.TenantID(), "TenantID should match")
	assert.Equal(t, original.Comment(), result.Comment(), "Comment should match")
	assert.Equal(t, original.TransactionType(), result.TransactionType(), "TransactionType should match")
	assert.Equal(t, original.OriginAccountID(), result.OriginAccountID(), "OriginAccountID should match")
	assert.Equal(t, original.DestinationAccountID(), result.DestinationAccountID(), "DestinationAccountID should match")
}

func TestExpenseCategoryRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Now()
	testTenantID := uuid.New()

	original := category.New(
		"IT Equipment",
		category.WithID(uuid.New()),
		category.WithTenantID(testTenantID),
		category.WithDescription("Computer equipment and software"),
		category.WithCreatedAt(now),
		category.WithUpdatedAt(now),
	)

	// Convert to DB model
	dbCategory := persistence.ToDBExpenseCategory(original)

	// Convert back to domain model
	result, err := persistence.ToDomainExpenseCategory(dbCategory)
	require.NoError(t, err, "Round trip should not fail")

	// Verify all fields match
	assert.Equal(t, original.Name(), result.Name(), "Name should match")
	assert.Equal(t, original.TenantID(), result.TenantID(), "TenantID should match")
	assert.Equal(t, original.Description(), result.Description(), "Description should match")
}
