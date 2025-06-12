package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
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
			tenant_id,
			amount,
			origin_account_id,
			destination_account_id,
			transaction_date,
			accounting_period,
			transaction_type,
			comment,
			created_at
		FROM transactions`
	transactionCountQuery  = `SELECT COUNT(*) as count FROM transactions WHERE tenant_id = $1`
	transactionInsertQuery = `
		INSERT INTO transactions (
			tenant_id,
			amount,
			origin_account_id,
			destination_account_id,
			transaction_date,
			accounting_period,
			transaction_type,
			comment
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	transactionUpdateQuery = `
		UPDATE transactions
		SET amount = $1,
			origin_account_id = $2,
			destination_account_id = $3,
			transaction_date = $4,
			accounting_period = $5,
			transaction_type = $6,
			comment = $7
		WHERE id = $8 AND tenant_id = $9`
	transactionDeleteQuery = `DELETE FROM transactions WHERE id = $1 AND tenant_id = $2`
)

type PgTransactionRepository struct{}

func NewTransactionRepository() transaction.Repository {
	return &PgTransactionRepository{}
}

func (g *PgTransactionRepository) GetPaginated(ctx context.Context, params *transaction.FindParams) ([]transaction.Transaction, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"tenant_id = $1"}
	args := []interface{}{tenantID}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2))
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

func (g *PgTransactionRepository) Count(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, transactionCountQuery, tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *PgTransactionRepository) GetAll(ctx context.Context) ([]transaction.Transaction, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(transactionFindQuery, "WHERE tenant_id = $1")
	return g.queryTransactions(ctx, query, tenantID)
}

func (g *PgTransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (transaction.Transaction, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	transactions, err := g.queryTransactions(ctx, repo.Join(transactionFindQuery, "WHERE id = $1 AND tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, ErrTransactionNotFound
	}
	return transactions[0], nil
}

func (g *PgTransactionRepository) Create(ctx context.Context, data transaction.Transaction) (transaction.Transaction, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	data = data.UpdateTenantID(tenantID)
	entity := ToDBTransaction(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.TenantID,
		entity.Amount,
		entity.OriginAccountID,
		entity.DestinationAccountID,
		entity.TransactionDate,
		entity.AccountingPeriod,
		entity.TransactionType,
		entity.Comment,
	}
	var id uuid.UUID
	err = tx.QueryRow(ctx, transactionInsertQuery, args...).Scan(&id)
	if err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *PgTransactionRepository) Update(ctx context.Context, data transaction.Transaction) (transaction.Transaction, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	data = data.UpdateTenantID(tenantID)
	dbTransaction := ToDBTransaction(data)
	args := []interface{}{
		dbTransaction.Amount,
		dbTransaction.OriginAccountID,
		dbTransaction.DestinationAccountID,
		dbTransaction.TransactionDate,
		dbTransaction.AccountingPeriod,
		dbTransaction.TransactionType,
		dbTransaction.Comment,
		dbTransaction.ID,
		dbTransaction.TenantID,
	}
	err = g.execQuery(ctx, transactionUpdateQuery, args...)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(dbTransaction.ID)
	if err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *PgTransactionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return g.execQuery(ctx, transactionDeleteQuery, id, tenantID)
}

func (g *PgTransactionRepository) queryTransactions(ctx context.Context, query string, args ...interface{}) ([]transaction.Transaction, error) {
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
			&r.TenantID,
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
	return mapping.MapDBModels(dbRows, ToDomainTransaction)
}

func (g *PgTransactionRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
