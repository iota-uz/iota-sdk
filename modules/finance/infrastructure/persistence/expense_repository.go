package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

const (
	// SQL queries
	expenseFindQuery = `
		SELECT ex.id, ex.transaction_id, ex.category_id, ex.tenant_id, ex.created_at, ex.updated_at,
		tr.amount, tr.transaction_date, tr.accounting_period, tr.transaction_type, tr.comment,
		tr.origin_account_id, tr.destination_account_id
		FROM expenses ex LEFT JOIN transactions tr on tr.id = ex.transaction_id`

	expenseCountQuery = `SELECT COUNT(ex.id) FROM expenses ex`

	expenseInsertQuery = `
		INSERT INTO expenses (transaction_id, category_id, tenant_id)
		VALUES ($1, $2, $3)
		RETURNING id`

	expenseUpdateQuery = `
		UPDATE expenses
		SET transaction_id = $1, category_id = $2
		WHERE id = $3`

	expenseDeleteQuery = `DELETE FROM expenses where id = $1`
)

var (
	ErrExpenseNotFound = errors.New("expense not found")
)

type GormExpenseRepository struct {
	categoryRepo    category.Repository
	transactionRepo transaction.Repository
	fieldMap        map[expense.Field]string
}

func NewExpenseRepository(categoryRepo category.Repository, transactionRepo transaction.Repository) expense.Repository {
	return &GormExpenseRepository{
		categoryRepo:    categoryRepo,
		transactionRepo: transactionRepo,
		fieldMap: map[expense.Field]string{
			expense.ID:            "ex.id",
			expense.TransactionID: "ex.transaction_id",
			expense.CategoryID:    "ex.category_id",
			expense.CreatedAt:     "ex.created_at",
			expense.UpdatedAt:     "ex.updated_at",
		},
	}
}

func (g *GormExpenseRepository) buildExpenseFilters(params *expense.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}

		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	// Search support
	if params.Search != "" {
		index := len(args) + 1
		where = append(
			where,
			fmt.Sprintf(
				"(tr.comment ILIKE $%d)",
				index,
			),
		)
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *GormExpenseRepository) queryExpenses(ctx context.Context, query string, args ...interface{}) ([]expense.Expense, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	// First collect all DB row data
	type expenseData struct {
		expense       models.Expense
		transaction   models.Transaction
		domainExpense expense.Expense
	}
	expensesData := make([]expenseData, 0)

	for rows.Next() {
		var dbExpense models.Expense
		var dbTransaction models.Transaction
		if err := rows.Scan(
			&dbExpense.ID,
			&dbExpense.TransactionID,
			&dbExpense.CategoryID,
			&dbExpense.TenantID,
			&dbExpense.CreatedAt,
			&dbExpense.UpdatedAt,
			&dbTransaction.Amount,
			&dbTransaction.TransactionDate,
			&dbTransaction.AccountingPeriod,
			&dbTransaction.TransactionType,
			&dbTransaction.Comment,
			&dbTransaction.OriginAccountID,
			&dbTransaction.DestinationAccountID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan expense row")
		}

		domainExpense, err := ToDomainExpense(&dbExpense, &dbTransaction)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain expense")
		}

		expensesData = append(expensesData, expenseData{
			expense:       dbExpense,
			transaction:   dbTransaction,
			domainExpense: domainExpense,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	// Now fetch all categories in a single batch
	expenses := make([]expense.Expense, 0, len(expensesData))
	for _, data := range expensesData {
		domainCategory, err := g.categoryRepo.GetByID(ctx, uuid.MustParse(data.expense.CategoryID))
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get category for expense ID: %s", data.expense.ID))
		}

		// Create a new expense with the retrieved category
		exp := expense.New(
			data.domainExpense.Amount(),
			data.domainExpense.Account(),
			domainCategory,
			data.domainExpense.Date(),
			expense.WithID(data.domainExpense.ID()),
			expense.WithComment(data.domainExpense.Comment()),
			expense.WithTransactionID(data.domainExpense.TransactionID()),
			expense.WithAccountingPeriod(data.domainExpense.AccountingPeriod()),
			expense.WithCreatedAt(data.domainExpense.CreatedAt()),
			expense.WithUpdatedAt(data.domainExpense.UpdatedAt()),
		)
		expenses = append(expenses, exp)
	}

	return expenses, nil
}

func (g *GormExpenseRepository) GetPaginated(ctx context.Context, params *expense.FindParams) ([]expense.Expense, error) {
	where, args, err := g.buildExpenseFilters(params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build filters")
	}

	query := repo.Join(
		expenseFindQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.queryExpenses(ctx, query, args...)
}

func (g *GormExpenseRepository) Count(ctx context.Context, params *expense.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := g.buildExpenseFilters(params)
	if err != nil {
		return 0, errors.Wrap(err, "failed to build filters")
	}

	query := repo.Join(
		expenseCountQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count expenses")
	}
	return count, nil
}

func (g *GormExpenseRepository) GetAll(ctx context.Context) ([]expense.Expense, error) {
	query := expenseFindQuery
	return g.queryExpenses(ctx, query)
}

func (g *GormExpenseRepository) GetByID(ctx context.Context, id uuid.UUID) (expense.Expense, error) {
	query := repo.Join(
		expenseFindQuery,
		repo.JoinWhere("ex.id = $1"),
	)

	expenses, err := g.queryExpenses(ctx, query, id)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to get expense with ID: %s", id))
	}
	if len(expenses) == 0 {
		return nil, errors.Wrap(ErrExpenseNotFound, fmt.Sprintf("id: %s", id))
	}
	return expenses[0], nil
}

func (g *GormExpenseRepository) Create(ctx context.Context, data expense.Expense) (expense.Expense, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}
	expenseRow, transactionRow := ToDBExpense(data)
	createdTransaction, err := g.transactionRepo.Create(ctx, transactionRow)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create transaction")
	}

	var id uuid.UUID
	if err := tx.QueryRow(
		ctx,
		expenseInsertQuery,
		createdTransaction.ID(),
		expenseRow.CategoryID,
		expenseRow.TenantID,
	).Scan(&id); err != nil {
		return nil, errors.Wrap(err, "failed to create expense")
	}
	return g.GetByID(ctx, id)
}

func (g *GormExpenseRepository) Update(ctx context.Context, data expense.Expense) (expense.Expense, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}
	expenseRow, transactionRow := ToDBExpense(data)
	updatedTransaction, err := g.transactionRepo.Update(ctx, transactionRow)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update transaction")
	}
	if _, err := tx.Exec(
		ctx,
		expenseUpdateQuery,
		updatedTransaction.ID().String(),
		expenseRow.CategoryID,
		expenseRow.ID,
	); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to update expense with ID: %s", expenseRow.ID))
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormExpenseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}
	if _, err := tx.Exec(ctx, expenseDeleteQuery, id); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete expense with ID: %s", id))
	}
	return nil
}
