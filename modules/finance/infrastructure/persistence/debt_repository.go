package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrDebtNotFound = errors.New("debt not found")
)

const (
	debtFindQuery = `
		SELECT id, tenant_id, type, status, counterparty_id, 
			   original_amount, original_amount_currency_id,
			   outstanding_amount, outstanding_currency_id,
			   description, due_date, settlement_transaction_id,
			   created_at, updated_at
		FROM debts`
	debtCountQuery  = `SELECT COUNT(*) as count FROM debts WHERE tenant_id = $1`
	debtInsertQuery = `
		INSERT INTO debts (
			tenant_id, type, status, counterparty_id,
			original_amount, original_amount_currency_id,
			outstanding_amount, outstanding_currency_id,
			description, due_date, settlement_transaction_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id`
	debtUpdateQuery = `
		UPDATE debts SET 
			type = $1, status = $2, counterparty_id = $3,
			original_amount = $4, original_amount_currency_id = $5,
			outstanding_amount = $6, outstanding_currency_id = $7,
			description = $8, due_date = $9, settlement_transaction_id = $10,
			updated_at = $11
		WHERE id = $12 AND tenant_id = $13`
	debtDeleteQuery = `DELETE FROM debts WHERE id = $1 AND tenant_id = $2`
)

type GormDebtRepository struct{}

func NewDebtRepository() debt.Repository {
	return &GormDebtRepository{}
}

func (g *GormDebtRepository) Count(ctx context.Context) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, debtCountQuery, tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormDebtRepository) GetAll(ctx context.Context) ([]debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(debtFindQuery, "WHERE tenant_id = $1")
	return g.queryDebts(ctx, query, tenantID)
}

func (g *GormDebtRepository) GetPaginated(ctx context.Context, params *debt.FindParams) ([]debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"tenant_id = $1"}
	args := []interface{}{tenantID}

	if params.CounterpartyID != nil {
		where = append(where, fmt.Sprintf("counterparty_id = $%d", len(args)+1))
		args = append(args, params.CounterpartyID)
	}

	if params.Type != nil {
		where = append(where, fmt.Sprintf("type = $%d", len(args)+1))
		args = append(args, string(*params.Type))
	}

	if params.Status != nil {
		where = append(where, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, string(*params.Status))
	}

	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("created_at BETWEEN $%d and $%d", len(args)+1, len(args)+2))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}

	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
		args = append(args, "%"+params.Query+"%")
	}

	q := repo.Join(
		debtFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryDebts(ctx, q, args...)
}

func (g *GormDebtRepository) GetByID(ctx context.Context, id uuid.UUID) (debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	debts, err := g.queryDebts(ctx, repo.Join(debtFindQuery, "WHERE id = $1 AND tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get debt by id")
	}
	if len(debts) == 0 {
		return nil, ErrDebtNotFound
	}
	return debts[0], nil
}

func (g *GormDebtRepository) GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(debtFindQuery, "WHERE counterparty_id = $1 AND tenant_id = $2")
	return g.queryDebts(ctx, query, counterpartyID, tenantID)
}

func (g *GormDebtRepository) Create(ctx context.Context, data debt.Debt) (debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID on the domain entity
	data = data.UpdateTenantID(tenantID)

	dbDebt := ToDBDebt(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	row := tx.QueryRow(
		ctx,
		debtInsertQuery,
		dbDebt.TenantID,
		dbDebt.Type,
		dbDebt.Status,
		dbDebt.CounterpartyID,
		dbDebt.OriginalAmount,
		dbDebt.OriginalAmountCurrencyID,
		dbDebt.OutstandingAmount,
		dbDebt.OutstandingCurrencyID,
		dbDebt.Description,
		dbDebt.DueDate,
		dbDebt.SettlementTransactionID,
		dbDebt.CreatedAt,
		dbDebt.UpdatedAt,
	)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, errors.Wrap(err, "failed to create debt")
	}
	return g.GetByID(ctx, id)
}

func (g *GormDebtRepository) Update(ctx context.Context, data debt.Debt) (debt.Debt, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	// Set tenant ID on the domain entity
	data = data.UpdateTenantID(tenantID)

	dbDebt := ToDBDebt(data)
	if err := g.execQuery(
		ctx,
		debtUpdateQuery,
		dbDebt.Type,
		dbDebt.Status,
		dbDebt.CounterpartyID,
		dbDebt.OriginalAmount,
		dbDebt.OriginalAmountCurrencyID,
		dbDebt.OutstandingAmount,
		dbDebt.OutstandingCurrencyID,
		dbDebt.Description,
		dbDebt.DueDate,
		dbDebt.SettlementTransactionID,
		dbDebt.UpdatedAt,
		dbDebt.ID,
		dbDebt.TenantID,
	); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormDebtRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	return g.execQuery(ctx, debtDeleteQuery, id, tenantID)
}

func (g *GormDebtRepository) queryDebts(ctx context.Context, query string, args ...interface{}) ([]debt.Debt, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities := make([]debt.Debt, 0)
	for rows.Next() {
		var debtRow models.Debt
		if err := rows.Scan(
			&debtRow.ID,
			&debtRow.TenantID,
			&debtRow.Type,
			&debtRow.Status,
			&debtRow.CounterpartyID,
			&debtRow.OriginalAmount,
			&debtRow.OriginalAmountCurrencyID,
			&debtRow.OutstandingAmount,
			&debtRow.OutstandingCurrencyID,
			&debtRow.Description,
			&debtRow.DueDate,
			&debtRow.SettlementTransactionID,
			&debtRow.CreatedAt,
			&debtRow.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entity, err := ToDomainDebt(&debtRow)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (g *GormDebtRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
