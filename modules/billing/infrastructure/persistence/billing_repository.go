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
	"strings"
)

var (
	ErrTransactionNotFound = errors.New("transaction not found")
)

const (
	selectTransactionQuery = `
		SELECT 
		    bt.id,
		    bt.tenant_id,
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
								  tenant_id,
                                  status,
                                  quantity,
                                  currency,
                                  gateway,
                                  details,
                                  created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`

	updateTransactionQuery = `
		UPDATE billing_transactions SET 
			tenant_id = $1,
			status = $2, 
			quantity = $3, 
			currency = $4, 
			gateway = $5, 
			details = $6,
			updated_at = $7
		WHERE id = $8`

	deleteTransactionQuery = `DELETE FROM billing_transactions WHERE id = $1`
)

type BillingRepository struct {
	fieldMap map[billing.Field]string
}

func NewBillingRepository() *BillingRepository {
	return &BillingRepository{
		fieldMap: map[billing.Field]string{
			billing.CreatedAt:     "bt.created_at",
			billing.TenantIDField: "bt.tenant_id",
		},
	}
}

func (r *BillingRepository) Count(ctx context.Context, params *billing.FindParams) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	where, args, err := r.buildBillingFilters(params)
	if err != nil {
		return 0, err
	}

	baseQuery := countTransactionQuery

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count transactions")
	}
	return count, nil
}

func (r *BillingRepository) GetPaginated(ctx context.Context, params *billing.FindParams) ([]billing.Transaction, error) {
	where, args, err := r.buildBillingFilters(params)
	if err != nil {
		return nil, err
	}

	baseQuery := selectTransactionQuery

	query := repo.Join(
		baseQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(r.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	transactions, err := r.queryTransactions(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get paginated transactions")
	}
	return transactions, nil
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

func (r *BillingRepository) GetByDetailsFields(
	ctx context.Context,
	gateway billing.Gateway,
	filters []billing.DetailsFieldFilter,
) ([]billing.Transaction, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("at least one filter is required")
	}

	clauses := make([]string, 0, len(filters)+1)
	args := make([]any, 0, len(filters)+1)

	clauses = append(clauses, fmt.Sprintf("bt.gateway = $%d", len(args)+1))
	args = append(args, gateway)

	for _, f := range filters {
		if len(f.Path) == 0 {
			return nil, fmt.Errorf("filter path cannot be empty")
		}
		jsonPath := "bt.details"
		for _, segment := range f.Path[:len(f.Path)-1] {
			jsonPath += fmt.Sprintf(" -> '%s'", segment)
		}
		last := f.Path[len(f.Path)-1]
		jsonField := fmt.Sprintf("%s ->> '%s'", jsonPath, last)

		switch f.Operator {
		case billing.OpEqual, billing.OpGreater, billing.OpLess, billing.OpGTE, billing.OpLTE:
			clauses = append(clauses, fmt.Sprintf("%s %s $%d", jsonField, f.Operator, len(args)+1))
			args = append(args, fmt.Sprintf("%v", f.Value))

		case billing.OpBetween:
			rangeVals, ok := f.Value.([2]any)
			if !ok {
				return nil, fmt.Errorf("value for 'between' must be [2]any")
			}
			clauses = append(clauses, fmt.Sprintf("(%s)::bigint BETWEEN $%d AND $%d", jsonField, len(args)+1, len(args)+2))
			args = append(args, rangeVals[0], rangeVals[1])

		default:
			return nil, fmt.Errorf("unsupported operator: %s", f.Operator)
		}
	}

	whereClause := "WHERE " + strings.Join(clauses, " AND ")

	query := repo.Join(
		selectTransactionQuery,
		whereClause,
	)

	transactions, err := r.queryTransactions(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transactions with gateway %s and filters: %+v", gateway, filters)
	}
	return transactions, nil
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
		transaction.TenantID,
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
		transaction.TenantID,
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
			&t.TenantID,
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

func (r *BillingRepository) buildBillingFilters(params *billing.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	var args []interface{}

	for _, filter := range params.Filters {
		column, ok := r.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		index := len(args) + 1
		where = append(where, fmt.Sprintf("(g.name ILIKE $%d OR g.description ILIKE $%d)", index, index))
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}
