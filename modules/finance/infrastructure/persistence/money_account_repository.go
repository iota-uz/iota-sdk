package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
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
			ma.name,
			ma.account_number,
			ma.description,
			ma.balance,
			ma.balance_currency_id,
			ma.created_at,
			ma.updated_at,
			c.code,
			c.name,
			c.symbol,
			c.created_at,
			c.updated_at
		FROM money_accounts ma LEFT JOIN currencies c ON c.code = ma.balance_currency_id`
	countQuery              = `SELECT COUNT(*) as count FROM money_accounts`
	recalculateBalanceQuery = `
		UPDATE money_accounts
		SET balance = (SELECT sum(t.amount) FROM transactions t WHERE origin_account_id = $1 OR destination_account_id = $2)
		WHERE id = $3`
	insertQuery = `
		INSERT INTO money_accounts (
			name,
			account_number,
			description,
			balance,
			balance_currency_id,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	updateQuery = `
		UPDATE money_accounts
		SET name = $1, account_number = $2, description = $3, balance = $4, balance_currency_id = $5, updated_at = $6
		WHERE id = $7`
	deleteRelatedQuery = `DELETE FROM transactions WHERE origin_account_id = $1 OR destination_account_id = $1;`
	deleteQuery        = `DELETE FROM money_accounts WHERE id = $1;`
)

type GormMoneyAccountRepository struct{}

func NewMoneyAccountRepository() moneyaccount.Repository {
	return &GormMoneyAccountRepository{}
}

func (g *GormMoneyAccountRepository) GetPaginated(ctx context.Context, params *moneyaccount.FindParams) ([]*moneyaccount.Account, error) {
	var args []interface{}
	where := []string{"1 = 1"}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("wo.created_at BETWEEN $%d and $%d", len(where), len(where)+1))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("wo.%s::VARCHAR ILIKE $%d", params.Field, len(where)))
		args = append(args, "%"+params.Query+"%")
	}
	q := repo.Join(
		findQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryAccounts(ctx, q, args...)
}

func (g *GormMoneyAccountRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, countQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormMoneyAccountRepository) GetAll(ctx context.Context) ([]*moneyaccount.Account, error) {
	return g.queryAccounts(ctx, findQuery)
}

func (g *GormMoneyAccountRepository) GetByID(ctx context.Context, id uint) (*moneyaccount.Account, error) {
	accounts, err := g.queryAccounts(ctx, repo.Join(findQuery, "WHERE id = $1"), id)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, ErrAccountNotFound
	}
	return accounts[0], nil
}

func (g *GormMoneyAccountRepository) RecalculateBalance(ctx context.Context, id uint) error {
	err := g.execQuery(ctx, recalculateBalanceQuery, id, id, id)
	if err != nil {
		return errors.Wrap(err, "failed to recalculate balance")
	}
	return nil
}

func (g *GormMoneyAccountRepository) Create(ctx context.Context, data *moneyaccount.Account) (*moneyaccount.Account, error) {
	entity := toDBMoneyAccount(data)
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.Name,
		entity.AccountNumber,
		entity.Description,
		entity.Balance,
		entity.BalanceCurrencyID,
		entity.CreatedAt,
		entity.UpdatedAt,
	}
	row := tx.QueryRow(ctx, insertQuery, args...)
	var id uint
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *GormMoneyAccountRepository) Update(ctx context.Context, data *moneyaccount.Account) error {
	dbAccount := toDBMoneyAccount(data)
	args := []interface{}{
		dbAccount.Name,
		dbAccount.AccountNumber,
		dbAccount.Description,
		dbAccount.Balance,
		dbAccount.BalanceCurrencyID,
		dbAccount.UpdatedAt,
		dbAccount.ID,
	}
	return g.execQuery(ctx, updateQuery, args...)
}

func (g *GormMoneyAccountRepository) Delete(ctx context.Context, id uint) error {
	if err := g.execQuery(ctx, deleteRelatedQuery, id); err != nil {
		return err
	}
	return g.execQuery(ctx, deleteQuery, id)
}

func (g *GormMoneyAccountRepository) queryAccounts(ctx context.Context, query string, args ...interface{}) ([]*moneyaccount.Account, error) {
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
		r := models.MoneyAccount{
			Currency: &coremodels.Currency{},
		}
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.AccountNumber,
			&r.Description,
			&r.Balance,
			&r.BalanceCurrencyID,
			&r.CreatedAt,
			&r.UpdatedAt,
			&r.Currency.Code,
			&r.Currency.Name,
			&r.Currency.Symbol,
			&r.Currency.CreatedAt,
			&r.Currency.UpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, &r)
	}
	return mapping.MapDBModels(dbRows, toDomainMoneyAccount)
}

func (g *GormMoneyAccountRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
