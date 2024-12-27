package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-agency/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/utils/repo"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

type GormTransactionRepository struct{}

func NewTransactionRepository() transaction.Repository {
	return &GormTransactionRepository{}
}

func (g *GormTransactionRepository) GetPaginated(
	ctx context.Context, params *transaction.FindParams,
) ([]*transaction.Transaction, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return nil, err
	}
	where, args := []string{"1 = 1"}, []interface{}{}
	if params.ID != 0 {
		where, args = append(where, fmt.Sprintf("ex.id = $%d", len(args)+1)), append(args, params.ID)
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where, args = append(where, fmt.Sprintf("ex.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2)), append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	rows, err := pool.Query(ctx, `
		SELECT id, amount, origin_account_id, destination_account_id, transaction_date, accounting_period, transaction_type, comment, created_at
		FROM transactions
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		`+repo.FormatLimitOffset(params.Limit, params.Offset),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	transactions := make([]*transaction.Transaction, 0)
	for rows.Next() {
		var transaction models.Transaction
		var comment sql.NullString
		if err := rows.Scan(
			&transaction.ID,
			&transaction.Amount,
			&transaction.OriginAccountID,
			&transaction.DestinationAccountID,
			&transaction.TransactionDate,
			&transaction.AccountingPeriod,
			&transaction.TransactionType,
			&comment,
			&transaction.CreatedAt,
		); err != nil {
			return nil, err
		}
		transaction.Comment = comment.String
		domainTransaction, err := toDomainTransaction(&transaction)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, domainTransaction)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return transactions, nil
}

func (g *GormTransactionRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UsePool(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) as count FROM transactions
	`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormTransactionRepository) GetAll(ctx context.Context) ([]*transaction.Transaction, error) {
	return g.GetPaginated(ctx, &transaction.FindParams{
		Limit: 100000,
	})
}

func (g *GormTransactionRepository) GetByID(ctx context.Context, id uint) (*transaction.Transaction, error) {
	transactions, err := g.GetPaginated(ctx, &transaction.FindParams{
		ID: id,
	})
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, ErrTransactionNotFound
	}
	return transactions[0], nil
}

func (g *GormTransactionRepository) Create(ctx context.Context, data *transaction.Transaction) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	entity := toDBTransaction(data)
	if err := tx.QueryRow(ctx, `
		INSERT INTO transactions (amount, origin_account_id, destination_account_id, transaction_date, accounting_period, transaction_type, comment) 
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, entity.Amount, entity.OriginAccountID, entity.DestinationAccountID, entity.TransactionDate, entity.AccountingPeriod, entity.TransactionType, entity.Comment).Scan(&data.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormTransactionRepository) Update(ctx context.Context, data *transaction.Transaction) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	entity := toDBTransaction(data)
	if _, err := tx.Exec(ctx, `
		UPDATE transactions
		SET
		amount = $1, origin_account_id = $2,
		destination_account_id = $3, transaction_date = $4,
		accounting_period = $5, transaction_type = $6
		comment = $7
		WHERE id = $8
	`, entity.Amount, entity.OriginAccountID, entity.DestinationAccountID, entity.TransactionDate, entity.AccountingPeriod, entity.TransactionType, entity.Comment, entity.ID); err != nil {
		return err
	}
	return nil
}

func (g *GormTransactionRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		DELETE FROM transactions WHERE id = $1
	`, id); err != nil {
		return err
	}
	return nil
}
