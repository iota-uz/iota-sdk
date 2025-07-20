package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrAccountNotFound = errors.New("money account not found")
)

const (
	findQuery = `
		SELECT ma.id,
			ma.tenant_id,
			ma.name,
			ma.account_number,
			ma.description,
			ma.balance,
			ma.balance_currency_id,
			ma.created_at,
			ma.updated_at
		FROM money_accounts ma
	`
	countQuery              = `SELECT COUNT(*) as count FROM money_accounts WHERE tenant_id = $1`
	recalculateBalanceQuery = `
		UPDATE money_accounts
		SET balance = (
			SELECT COALESCE(SUM(
				CASE 
					WHEN destination_account_id = $1 THEN 
						CASE 
							WHEN t.transaction_type = 'EXCHANGE' AND t.destination_amount IS NOT NULL THEN t.destination_amount
							ELSE t.amount
						END
					WHEN origin_account_id = $1 THEN -t.amount
					ELSE 0
				END
			), 0)
			FROM transactions t 
			WHERE origin_account_id = $1 OR destination_account_id = $1
		)
		WHERE id = $1 AND tenant_id = $2`
	insertQuery = `
		INSERT INTO money_accounts (
			tenant_id,
			name,
			account_number,
			description,
			balance,
			balance_currency_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	updateQuery = `
		UPDATE money_accounts
		SET name = $1, account_number = $2, description = $3, balance = $4, balance_currency_id = $5, updated_at = $6
		WHERE id = $7 AND tenant_id = $8`
	deleteRelatedQuery = `DELETE FROM transactions WHERE origin_account_id = $1 OR destination_account_id = $1 AND tenant_id = $2;`
	deleteQuery        = `DELETE FROM money_accounts WHERE id = $1 AND tenant_id = $2;`
)

type GormMoneyAccountRepository struct {
	fieldMap map[moneyaccount.Field]string
}

func NewMoneyAccountRepository() moneyaccount.Repository {
	return &GormMoneyAccountRepository{
		fieldMap: map[moneyaccount.Field]string{
			moneyaccount.ID:            "ma.id",
			moneyaccount.Name:          "ma.name",
			moneyaccount.AccountNumber: "ma.account_number",
			moneyaccount.Balance:       "ma.balance",
			moneyaccount.Description:   "ma.description",
			moneyaccount.CurrencyCode:  "ma.balance_currency_id",
			moneyaccount.CreatedAt:     "ma.created_at",
			moneyaccount.UpdatedAt:     "ma.updated_at",
		},
	}
}

func (g *GormMoneyAccountRepository) GetPaginated(ctx context.Context, params *moneyaccount.FindParams) ([]moneyaccount.Account, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"ma.tenant_id = $1"}
	args := []interface{}{tenantID}

	// Handle search
	if params.Search != "" {
		where = append(where, fmt.Sprintf("(ma.name ILIKE $%d OR ma.account_number ILIKE $%d OR ma.description ILIKE $%d)", len(args)+1, len(args)+1, len(args)+1))
		args = append(args, "%"+params.Search+"%")
	}

	// Handle filters
	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, fmt.Errorf("invalid filter: unknown filter field: %v", filter.Column)
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	q := repo.Join(
		findQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryAccounts(ctx, q, args...)
}

func (g *GormMoneyAccountRepository) Count(ctx context.Context, params *moneyaccount.FindParams) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	where := []string{"ma.tenant_id = $1"}
	args := []interface{}{tenantID}

	// Handle search
	if params.Search != "" {
		where = append(where, fmt.Sprintf("(ma.name ILIKE $%d OR ma.account_number ILIKE $%d OR ma.description ILIKE $%d)", len(args)+1, len(args)+1, len(args)+1))
		args = append(args, "%"+params.Search+"%")
	}

	// Handle filters
	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return 0, fmt.Errorf("invalid filter: unknown filter field: %v", filter.Column)
		}
		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	query := repo.Join(
		"SELECT COUNT(*) FROM money_accounts ma",
		repo.JoinWhere(where...),
	)

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormMoneyAccountRepository) GetAll(ctx context.Context) ([]moneyaccount.Account, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	query := repo.Join(findQuery, "WHERE ma.tenant_id = $1")
	return g.queryAccounts(ctx, query, tenantID)
}

func (g *GormMoneyAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (moneyaccount.Account, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	accounts, err := g.queryAccounts(ctx, repo.Join(findQuery, "WHERE ma.id = $1 AND ma.tenant_id = $2"), id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrAccountNotFound
	}
	return accounts[0], nil
}

func (g *GormMoneyAccountRepository) RecalculateBalance(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	err = g.execQuery(ctx, recalculateBalanceQuery, id, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to recalculate balance")
	}
	return nil
}

func (g *GormMoneyAccountRepository) Create(ctx context.Context, data moneyaccount.Account) (moneyaccount.Account, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	data = data.UpdateTenantID(tenantID)
	entity := ToDBMoneyAccount(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.TenantID,
		entity.Name,
		entity.AccountNumber,
		entity.Description,
		entity.Balance,
		entity.BalanceCurrencyID,
		entity.CreatedAt,
		entity.UpdatedAt,
	}
	row := tx.QueryRow(ctx, insertQuery, args...)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *GormMoneyAccountRepository) Update(ctx context.Context, data moneyaccount.Account) (moneyaccount.Account, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	data = data.UpdateTenantID(tenantID)
	dbAccount := ToDBMoneyAccount(data)
	args := []interface{}{
		dbAccount.Name,
		dbAccount.AccountNumber,
		dbAccount.Description,
		dbAccount.Balance,
		dbAccount.BalanceCurrencyID,
		dbAccount.UpdatedAt,
		dbAccount.ID,
		dbAccount.TenantID,
	}
	if err := g.execQuery(ctx, updateQuery, args...); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormMoneyAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tenant from context: %w", err)
	}

	if err := g.execQuery(ctx, deleteRelatedQuery, id, tenantID); err != nil {
		return err
	}
	return g.execQuery(ctx, deleteQuery, id, tenantID)
}

func (g *GormMoneyAccountRepository) queryAccounts(ctx context.Context, query string, args ...interface{}) ([]moneyaccount.Account, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbRows []*models.MoneyAccount
	for rows.Next() {
		r := models.MoneyAccount{}
		if err := rows.Scan(
			&r.ID,
			&r.TenantID,
			&r.Name,
			&r.AccountNumber,
			&r.Description,
			&r.Balance,
			&r.BalanceCurrencyID,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, &r)
	}
	return mapping.MapDBModels(dbRows, ToDomainMoneyAccount)
}

func (g *GormMoneyAccountRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
