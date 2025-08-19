package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

// SQL queries
const (
	selectTransactionsSQL = `SELECT
		t.id, t.tenant_id, t.amount, t.origin_account_id, t.destination_account_id,
		t.transaction_date, t.accounting_period, t.transaction_type, t.comment,
		t.exchange_rate, t.destination_amount, t.created_at,
		-- Origin account
		oa.name as origin_account_name, oa.account_number as origin_account_number, 
		oa.balance_currency_id as origin_currency,
		-- Destination account
		da.name as destination_account_name, da.account_number as destination_account_number,
		da.balance_currency_id as destination_currency,
		-- Expense data
		e.id as expense_id, ec.id as expense_category_id, ec.name as expense_category_name,
		-- Payment data
		p.id as payment_id, pc.id as payment_category_id, pc.name as payment_category_name,
		cp.id as counterparty_id, cp.name as counterparty_name, cp.tin as counterparty_tin
	FROM transactions t
	LEFT JOIN money_accounts oa ON t.origin_account_id = oa.id
	LEFT JOIN money_accounts da ON t.destination_account_id = da.id
	LEFT JOIN expenses e ON t.id = e.transaction_id
	LEFT JOIN expense_categories ec ON e.category_id = ec.id
	LEFT JOIN payments p ON t.id = p.transaction_id
	LEFT JOIN payment_categories pc ON p.payment_category_id = pc.id
	LEFT JOIN counterparty cp ON p.counterparty_id = cp.id`

	selectTransactionByIDSQL = selectTransactionsSQL + `
	WHERE t.id = $1 AND t.tenant_id = $2`

	countTransactionsSQL = `SELECT COUNT(DISTINCT t.id) FROM transactions t`
)

type Field = string

// Field constants for sorting and filtering
const (
	FieldID                   Field = "id"
	FieldAmount               Field = "amount"
	FieldTransactionDate      Field = "transaction_date"
	FieldAccountingPeriod     Field = "accounting_period"
	FieldTransactionType      Field = "transaction_type"
	FieldOriginAccountID      Field = "origin_account_id"
	FieldDestinationAccountID Field = "destination_account_id"
	FieldCreatedAt            Field = "created_at"
	FieldTenantID             Field = "tenant_id"
)

type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	SortBy  SortBy
	Search  string
	Filters []Filter
}

type TransactionQueryRepository interface {
	FindTransactions(ctx context.Context, params *FindParams) ([]*viewmodels.Transaction, int, error)
	FindTransactionByID(ctx context.Context, transactionID uuid.UUID) (*viewmodels.Transaction, error)
}

type pgTransactionQueryRepository struct{}

func NewPgTransactionQueryRepository() TransactionQueryRepository {
	return &pgTransactionQueryRepository{}
}

func (r *pgTransactionQueryRepository) fieldMapping() map[Field]string {
	return map[Field]string{
		FieldID:                   "t.id",
		FieldAmount:               "t.amount",
		FieldTransactionDate:      "t.transaction_date",
		FieldAccountingPeriod:     "t.accounting_period",
		FieldTransactionType:      "t.transaction_type",
		FieldOriginAccountID:      "t.origin_account_id",
		FieldDestinationAccountID: "t.destination_account_id",
		FieldCreatedAt:            "t.created_at",
		FieldTenantID:             "t.tenant_id",
	}
}

func (r *pgTransactionQueryRepository) buildFilterConditionsWithStartIndex(filters []Filter, startIndex int) ([]string, []interface{}) {
	if len(filters) == 0 {
		return []string{}, []interface{}{}
	}

	var conditions []string
	var args []interface{}

	for _, f := range filters {
		fieldName := r.fieldMapping()[f.Column]
		if fieldName == "" {
			continue
		}
		condition := f.Filter.String(fieldName, startIndex+len(args))
		if condition != "" {
			conditions = append(conditions, condition)
			args = append(args, f.Filter.Value()...)
		}
	}

	return conditions, args
}

func (r *pgTransactionQueryRepository) FindTransactions(ctx context.Context, params *FindParams) ([]*viewmodels.Transaction, int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to get tenant ID")
	}

	// Build conditions and args, starting with tenant filter
	conditions := []string{"t.tenant_id = $1"}
	args := []interface{}{tenantID}

	// Add filter conditions
	if len(params.Filters) > 0 {
		filterConditions, filterArgs := r.buildFilterConditionsWithStartIndex(params.Filters, len(args)+1)
		conditions = append(conditions, filterConditions...)
		args = append(args, filterArgs...)
	}

	// Add search conditions if provided
	if params.Search != "" {
		searchFilter := r.buildSearchFilter(params.Search, len(args)+1)
		conditions = append(conditions, searchFilter.condition)
		args = append(args, searchFilter.args...)
	}

	whereClause := repo.JoinWhere(conditions...)

	// Count query
	countQuery := repo.Join(countTransactionsSQL, whereClause)
	var count int
	err = tx.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to count transactions")
	}

	// Build main query
	query := repo.Join(
		selectTransactionsSQL,
		whereClause,
		params.SortBy.ToSQL(r.fieldMapping()),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to find transactions")
	}
	defer rows.Close()

	transactions, err := r.scanTransactions(rows)
	if err != nil {
		return nil, 0, err
	}

	return transactions, count, nil
}

func (r *pgTransactionQueryRepository) FindTransactionByID(ctx context.Context, transactionID uuid.UUID) (*viewmodels.Transaction, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID")
	}

	row := tx.QueryRow(ctx, selectTransactionByIDSQL, transactionID, tenantID)
	return r.scanTransaction(row)
}

// scanTransactions scans multiple transaction rows
func (r *pgTransactionQueryRepository) scanTransactions(rows interface {
	Next() bool
	Scan(...interface{}) error
}) ([]*viewmodels.Transaction, error) {
	transactions := make([]*viewmodels.Transaction, 0)

	for rows.Next() {
		transaction, err := r.scanTransaction(rows)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan transaction")
		}
		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// scanTransaction scans a single transaction row with all joined data
func (r *pgTransactionQueryRepository) scanTransaction(row interface{ Scan(...interface{}) error }) (*viewmodels.Transaction, error) {
	var dbTransaction models.Transaction
	var originAccountName, originAccountNumber, originCurrency *string
	var destinationAccountName, destinationAccountNumber, destinationCurrency *string
	var expenseID, expenseCategoryID, expenseCategoryName *string
	var paymentID, paymentCategoryID, paymentCategoryName *string
	var counterpartyID, counterpartyName, counterpartyTIN *string

	err := row.Scan(
		&dbTransaction.ID,
		&dbTransaction.TenantID,
		&dbTransaction.Amount,
		&dbTransaction.OriginAccountID,
		&dbTransaction.DestinationAccountID,
		&dbTransaction.TransactionDate,
		&dbTransaction.AccountingPeriod,
		&dbTransaction.TransactionType,
		&dbTransaction.Comment,
		&dbTransaction.ExchangeRate,
		&dbTransaction.DestinationAmount,
		&dbTransaction.CreatedAt,
		// Origin account
		&originAccountName,
		&originAccountNumber,
		&originCurrency,
		// Destination account
		&destinationAccountName,
		&destinationAccountNumber,
		&destinationCurrency,
		// Expense data
		&expenseID,
		&expenseCategoryID,
		&expenseCategoryName,
		// Payment data
		&paymentID,
		&paymentCategoryID,
		&paymentCategoryName,
		&counterpartyID,
		&counterpartyName,
		&counterpartyTIN,
	)
	if err != nil {
		return nil, err
	}

	// Determine currency - use origin account currency if available, otherwise destination
	var currencyCode string
	if originCurrency != nil {
		currencyCode = *originCurrency
	} else if destinationCurrency != nil {
		currencyCode = *destinationCurrency
	} else {
		currencyCode = "USD" // fallback
	}

	// Create money objects
	amount := money.New(dbTransaction.Amount, currencyCode)
	amountWithCurrency := amount.Display()

	// Create viewmodel
	vm := &viewmodels.Transaction{
		ID:                 dbTransaction.ID,
		Amount:             fmt.Sprintf("%.2f", amount.AsMajorUnits()),
		AmountWithCurrency: amountWithCurrency,
		TransactionDate:    dbTransaction.TransactionDate,
		AccountingPeriod:   dbTransaction.AccountingPeriod,
		TransactionType:    dbTransaction.TransactionType,
		Comment:            dbTransaction.Comment,
		CreatedAt:          dbTransaction.CreatedAt,
	}

	// Map transaction type for display
	switch transaction.Type(dbTransaction.TransactionType) {
	case transaction.Deposit:
		vm.TypeBadgeClass = "badge-success"
	case transaction.Withdrawal:
		vm.TypeBadgeClass = "badge-danger"
	case transaction.Transfer:
		vm.TypeBadgeClass = "badge-info"
	case transaction.Exchange:
		vm.TypeBadgeClass = "badge-warning"
	default:
		vm.TypeBadgeClass = "badge-primary"
	}

	// Set origin account information
	if dbTransaction.OriginAccountID.Valid && originAccountName != nil {
		vm.OriginAccount = &viewmodels.Account{
			ID:       dbTransaction.OriginAccountID.String,
			Name:     *originAccountName,
			Currency: *originCurrency,
		}
		if originAccountNumber != nil {
			vm.OriginAccount.Number = *originAccountNumber
		}
	}

	// Set destination account information
	if dbTransaction.DestinationAccountID.Valid && destinationAccountName != nil {
		vm.DestinationAccount = &viewmodels.Account{
			ID:       dbTransaction.DestinationAccountID.String,
			Name:     *destinationAccountName,
			Currency: *destinationCurrency,
		}
		if destinationAccountNumber != nil {
			vm.DestinationAccount.Number = *destinationAccountNumber
		}
	}

	// Set exchange information
	if dbTransaction.ExchangeRate.Valid {
		vm.ExchangeRate = fmt.Sprintf("%.8f", dbTransaction.ExchangeRate.Float64)
	}

	if dbTransaction.DestinationAmount.Valid && destinationCurrency != nil {
		destAmount := money.New(dbTransaction.DestinationAmount.Int64, *destinationCurrency)
		vm.DestinationAmountWithCurrency = destAmount.Display()
	}

	// Set category information
	if expenseCategoryID != nil && expenseCategoryName != nil {
		vm.Category = &viewmodels.Category{
			ID:   *expenseCategoryID,
			Name: *expenseCategoryName,
			Type: "expense",
		}
	} else if paymentCategoryID != nil && paymentCategoryName != nil {
		vm.Category = &viewmodels.Category{
			ID:   *paymentCategoryID,
			Name: *paymentCategoryName,
			Type: "payment",
		}
	}

	// Set counterparty information
	if counterpartyID != nil && counterpartyName != nil {
		vm.Counterparty = &viewmodels.CounterpartyInfo{
			ID:   *counterpartyID,
			Name: *counterpartyName,
		}
		if counterpartyTIN != nil {
			vm.Counterparty.TIN = *counterpartyTIN
		}
	}

	return vm, nil
}

// buildSearchFilter creates a search condition for transaction search
func (r *pgTransactionQueryRepository) buildSearchFilter(search string, startIndex int) struct {
	condition string
	args      []interface{}
} {
	searchQuery := strings.TrimSpace(search)
	placeholder := fmt.Sprintf("$%d", startIndex)
	searchCondition := fmt.Sprintf(`(
		t.comment ILIKE %s OR
		oa.name ILIKE %s OR
		da.name ILIKE %s OR
		ec.name ILIKE %s OR
		pc.name ILIKE %s OR
		cp.name ILIKE %s
	)`, placeholder, placeholder, placeholder, placeholder, placeholder, placeholder)

	return struct {
		condition string
		args      []interface{}
	}{
		condition: searchCondition,
		args:      []interface{}{"%" + searchQuery + "%"},
	}
}
