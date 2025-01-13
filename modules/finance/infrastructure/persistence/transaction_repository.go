package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

const (
	transactionFindQuery = `
		SELECT id,
			amount,
			origin_account_id,
			destination_account_id,
			transaction_date,
			accounting_period,
			transaction_type,
			comment,
			created_at
		FROM transactions`
	transactionCountQuery  = `SELECT COUNT(*) as count FROM transactions`
	transactionInsertQuery = `
		INSERT INTO transactions (
			amount,
			origin_account_id,
			destination_account_id,
			transaction_date,
			accounting_period,
			transaction_type,
			comment
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	transactionUpdateQuery = `
		UPDATE transactions
		SET amount = $1,
			origin_account_id = $2,
			destination_account_id = $3,
			transaction_date = $4,
			accounting_period = $5,
			transaction_type = $6,
			comment = $7
		WHERE id = $8`
	transactionDeleteQuery = `DELETE FROM transactions WHERE id = $1`
)

type GormTransactionRepository struct{}

func NewTransactionRepository() transaction.Repository {
	return &GormTransactionRepository{}
}

func (g *GormTransactionRepository) GetPaginated(ctx context.Context, params *transaction.FindParams) ([]*transaction.Transaction, error) {
	where := []string{"1 = 1"}
	var args []interface{}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("created_at BETWEEN $%d and $%d", len(where), len(where)+1))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	q := repo.Join(
		transactionFindQuery,
		repo.JoinWhere(where...),
		"ORDER BY id DESC",
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryTransactions(ctx, q, args...)
}

func (g *GormTransactionRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, transactionCountQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormTransactionRepository) GetAll(ctx context.Context) ([]*transaction.Transaction, error) {
	return g.queryTransactions(ctx, transactionFindQuery)
}

func (g *GormTransactionRepository) GetByID(ctx context.Context, id uint) (*transaction.Transaction, error) {
	transactions, err := g.queryTransactions(ctx, repo.Join(transactionFindQuery, "WHERE id = $1"), id)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, ErrTransactionNotFound
	}
	return transactions[0], nil
}

func (g *GormTransactionRepository) Create(ctx context.Context, data *transaction.Transaction) error {
	entity := toDBTransaction(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	args := []interface{}{
		entity.Amount,
		entity.OriginAccountID,
		entity.DestinationAccountID,
		entity.TransactionDate,
		entity.AccountingPeriod,
		entity.TransactionType,
		entity.Comment,
	}
	return tx.QueryRow(ctx, transactionInsertQuery, args...).Scan(&data.ID)
}

func (g *GormTransactionRepository) Update(ctx context.Context, data *transaction.Transaction) error {
	dbTransaction := toDBTransaction(data)
	args := []interface{}{
		dbTransaction.Amount,
		dbTransaction.OriginAccountID,
		dbTransaction.DestinationAccountID,
		dbTransaction.TransactionDate,
		dbTransaction.AccountingPeriod,
		dbTransaction.TransactionType,
		dbTransaction.Comment,
		dbTransaction.ID,
	}
	return g.execQuery(ctx, transactionUpdateQuery, args...)
}

func (g *GormTransactionRepository) Delete(ctx context.Context, id uint) error {
	return g.execQuery(ctx, transactionDeleteQuery, id)
}

func (g *GormTransactionRepository) queryTransactions(ctx context.Context, query string, args ...interface{}) ([]*transaction.Transaction, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbRows []*models.Transaction
	for rows.Next() {
		r := &models.Transaction{}
		if err := rows.Scan(
			&r.ID,
			&r.Amount,
			&r.OriginAccountID,
			&r.DestinationAccountID,
			&r.TransactionDate,
			&r.AccountingPeriod,
			&r.TransactionType,
			&r.Comment,
			&r.CreatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mapping.MapDBModels(dbRows, toDomainTransaction)
}

func (g *GormTransactionRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
