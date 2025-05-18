package persistence

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/pkg/errors"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

const (
	selectTransactionQuery = `
		SELECT 
		    bt.id,
		    bt.status,
		    bt.quantity,
		    bt.currency,
		    bt.gateway,
		    bt.details,
		    bt.created_at,
		    bt.updated_at
		FROM billing_transactions bt`

	countTransactionQuery = `SELECT COUNT(*) FROM billing_transactions bt`

	insertTransactionQuery = `
		INSERT INTO billing_transactions (
                                  status,
                                  quantity,
                                  currency,
                                  gateway,
                                  details,
                                  created_at
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	updateTransactionQuery = `
		UPDATE billing_transactions SET 
			status = $1, 
			quantity = $2, 
			currency = $3, 
			gateway = $4, 
			details = $5,
			updated_at = $6
		WHERE id = $7`

	deleteTransactionQuery = `DELETE FROM billing_transactions WHERE id = $1`
)

type BillingRepository struct{}

func NewBillingRepository() *BillingRepository {
	return &BillingRepository{}
}

func (r *BillingRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	var count int64
	if err := tx.QueryRow(ctx, countTransactionQuery).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count transactions")
	}
	return count, nil
}

func (r *BillingRepository) GetPaginated(ctx context.Context, params *billing.FindParams) ([]billing.Transaction, error) {
	var sortFields []string
	for _, f := range params.SortBy.Fields {
		switch f {
		case billing.CreatedAt:
			sortFields = append(sortFields, "bt.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}

	var args []interface{}
	where := []string{"1 = 1"}

	return r.queryTransactions(
		ctx,
		repo.Join(
			selectTransactionQuery,
			repo.JoinWhere(where...),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (r *BillingRepository) GetByID(ctx context.Context, id uuid.UUID) (billing.Transaction, error) {
	transactions, err := r.queryTransactions(ctx, selectTransactionQuery+" WHERE bt.id = $1", id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transaction with id %s", id)
	}
	if len(transactions) == 0 {
		return nil, ErrTransactionNotFound
	}
	return transactions[0], nil
}

func (r *BillingRepository) GetByDetailsField(ctx context.Context, field billing.DetailsField, value any) (billing.Transaction, error) {
	query := repo.Join(
		selectTransactionQuery,
		fmt.Sprintf("WHERE bt.details ->> '%s' = $1", field),
	)
	transactions, err := r.queryTransactions(ctx, query, value)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transactions with details field %s and value %v", field, value)
	}
	if len(transactions) == 0 {
		return nil, ErrTransactionNotFound
	}
	return transactions[0], nil
}

func (r *BillingRepository) GetAll(ctx context.Context) ([]billing.Transaction, error) {
	transactions, err := r.queryTransactions(ctx, selectTransactionQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all transactions")
	}
	return transactions, nil
}

func (r *BillingRepository) Save(ctx context.Context, data billing.Transaction) (billing.Transaction, error) {
	if data.ID() == uuid.Nil {
		return r.create(ctx, data)
	}
	return r.update(ctx, data)
}

func (r *BillingRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.deleteTransaction(ctx, id.String())
}

func (r *BillingRepository) create(ctx context.Context, data billing.Transaction) (billing.Transaction, error) {
	dbTransaction, err := ToDBTransaction(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to db transaction")
	}
	transactionID, err := r.insertTransaction(ctx, dbTransaction)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, uuid.MustParse(transactionID))
}

func (r *BillingRepository) update(ctx context.Context, data billing.Transaction) (billing.Transaction, error) {
	dbTransaction, err := ToDBTransaction(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to db transaction")
	}
	if err := r.updateTransaction(ctx, dbTransaction); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, data.ID())
}

func (r *BillingRepository) insertTransaction(ctx context.Context, transaction *models.Transaction) (string, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return "", errors.Wrap(err, "failed to get transaction")
	}

	if err := tx.QueryRow(
		ctx,
		insertTransactionQuery,
		transaction.Status,
		transaction.Quantity,
		transaction.Currency,
		transaction.Gateway,
		transaction.Details,
		transaction.CreatedAt,
	).Scan(&transaction.ID); err != nil {
		return "", errors.Wrap(err, "failed to insert transaction")
	}
	return transaction.ID, nil
}

func (r *BillingRepository) updateTransaction(ctx context.Context, transaction *models.Transaction) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	if _, err := tx.Exec(
		ctx,
		updateTransactionQuery,
		transaction.Status,
		transaction.Quantity,
		transaction.Currency,
		transaction.Gateway,
		transaction.Details,
		transaction.UpdatedAt,
		transaction.ID,
	); err != nil {
		return errors.Wrap(err, "failed to update transaction")
	}
	return nil
}

func (r *BillingRepository) deleteTransaction(ctx context.Context, id string) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	if _, err := tx.Exec(ctx, deleteTransactionQuery, id); err != nil {
		return errors.Wrapf(err, "failed to delete transaction with id %s", id)
	}
	return nil
}

func (r *BillingRepository) queryTransactions(ctx context.Context, query string, args ...interface{}) ([]billing.Transaction, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute transaction query")
	}
	defer rows.Close()

	dbTransactions := make([]*models.Transaction, 0)
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(
			&t.ID,
			&t.Status,
			&t.Quantity,
			&t.Currency,
			&t.Gateway,
			&t.Details,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan transaction")
		}
		dbTransactions = append(dbTransactions, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating transaction rows")
	}

	transactions := make([]billing.Transaction, 0, len(dbTransactions))

	for _, t := range dbTransactions {
		domainTransaction, err := ToDomainTransaction(t)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain transaction")
		}
		transactions = append(transactions, domainTransaction)
	}

	return transactions, nil
}
