package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrPaymentNotFound = errors.New("payment not found")
)

const (
	paymentFindQuery = `
		SELECT p.id,
		p.counterparty_id,
		p.created_at,
		p.updated_at,
		t.id,
		t.tenant_id,
		t.amount,
		t.destination_account_id,
		t.origin_account_id,
		t.accounting_period,
		t.transaction_date,
		t.transaction_type,
		t.comment,
		t.created_at
		FROM payments p LEFT JOIN transactions t ON t.id = p.transaction_id`
	paymentCountQuery  = `SELECT COUNT(*) as count FROM payments p LEFT JOIN transactions t ON t.id = p.transaction_id WHERE t.tenant_id = $1`
	paymentInsertQuery = `
	INSERT INTO payments (
		tenant_id,
		counterparty_id,
		transaction_id,
		created_at,
		updated_at
	)
	VALUES ($1, $2, $3, $4, $5) RETURNING id`
	paymentUpdateQuery        = `UPDATE payments SET counterparty_id = $1, updated_at = $2 WHERE id = $3`
	paymentDeleteRelatedQuery = `DELETE FROM transactions WHERE id = $1 AND tenant_id = $2`
	paymentDeleteQuery        = `DELETE FROM payments WHERE id = $1`
)

type GormPaymentRepository struct{}

func NewPaymentRepository() payment.Repository {
	return &GormPaymentRepository{}
}

func (g *GormPaymentRepository) GetPaginated(ctx context.Context, params *payment.FindParams) ([]payment.Payment, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"t.tenant_id = $1"}
	args := []interface{}{tenantID}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("p.created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("p.%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
		args = append(args, "%"+params.Query+"%")
	}
	q := repo.Join(
		paymentFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryPayments(ctx, q, args...)
}

func (g *GormPaymentRepository) Count(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, paymentCountQuery, tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormPaymentRepository) GetAll(ctx context.Context) ([]payment.Payment, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(paymentFindQuery, "WHERE t.tenant_id = $1")
	return g.queryPayments(ctx, query, tenantID)
}

func (g *GormPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (payment.Payment, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	payments, err := g.queryPayments(ctx, repo.Join(paymentFindQuery, "WHERE p.id = $1 AND t.tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get payment by id")
	}
	if len(payments) == 0 {
		return nil, ErrPaymentNotFound
	}
	return payments[0], nil
}

func (g *GormPaymentRepository) Create(ctx context.Context, data payment.Payment) (payment.Payment, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID on the domain entity
	data = data.UpdateTenantID(tenantID)

	dbPayment, dbTransaction := ToDBPayment(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(
		ctx,
		transactionInsertQuery,
		dbTransaction.TenantID,
		dbTransaction.Amount,
		dbTransaction.OriginAccountID,
		dbTransaction.DestinationAccountID,
		dbTransaction.TransactionDate,
		dbTransaction.AccountingPeriod,
		dbTransaction.TransactionType,
		dbTransaction.Comment,
	).Scan(&dbPayment.TransactionID); err != nil {
		return nil, errors.Wrap(err, "failed to create transaction")
	}
	row := tx.QueryRow(
		ctx,
		paymentInsertQuery,
		dbPayment.TenantID,
		dbPayment.CounterpartyID,
		dbPayment.TransactionID,
		dbPayment.CreatedAt,
		dbPayment.UpdatedAt,
	)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, errors.Wrap(err, "failed to create payment")
	}
	return g.GetByID(ctx, id)
}

func (g *GormPaymentRepository) Update(ctx context.Context, data payment.Payment) (payment.Payment, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID on the domain entity
	data = data.UpdateTenantID(tenantID)

	dbPayment, dbTransaction := ToDBPayment(data)
	if err := g.execQuery(
		ctx,
		paymentUpdateQuery,
		dbPayment.CounterpartyID,
		dbPayment.UpdatedAt,
		dbPayment.ID,
	); err != nil {
		return nil, err
	}
	if err := g.execQuery(
		ctx,
		transactionUpdateQuery,
		dbTransaction.Amount,
		dbTransaction.OriginAccountID,
		dbTransaction.DestinationAccountID,
		dbTransaction.TransactionDate,
		dbTransaction.AccountingPeriod,
		dbTransaction.TransactionType,
		dbTransaction.Comment,
		dbTransaction.ID,
		dbTransaction.TenantID,
	); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormPaymentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	entity, err := g.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := g.execQuery(ctx, paymentDeleteQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, paymentDeleteRelatedQuery, entity.TransactionID(), tenantID)
}

func (g *GormPaymentRepository) queryPayments(ctx context.Context, query string, args ...interface{}) ([]payment.Payment, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entities []payment.Payment
	for rows.Next() {
		var paymentRow models.Payment
		var transactionRow models.Transaction
		if err := rows.Scan(
			&paymentRow.ID,
			&paymentRow.CounterpartyID,
			&paymentRow.CreatedAt,
			&paymentRow.UpdatedAt,
			&transactionRow.ID,
			&transactionRow.TenantID,
			&transactionRow.Amount,
			&transactionRow.DestinationAccountID,
			&transactionRow.OriginAccountID,
			&transactionRow.AccountingPeriod,
			&transactionRow.TransactionDate,
			&transactionRow.TransactionType,
			&transactionRow.Comment,
			&transactionRow.CreatedAt,
		); err != nil {
			return nil, err
		}
		entity, err := ToDomainPayment(&paymentRow, &transactionRow)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (g *GormPaymentRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
