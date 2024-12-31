package persistence

import (
	"context"
	"errors"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/utils/repo"
)

var (
	ErrCounterpartyNotFound = errors.New("counterparty not found")
)

const (
	findCounterpartyQuery = `
		SELECT cp.id,
			cp.name,
			cp.tin,
			cp.type,
			cp.legal_type,
			cp.legal_address,
			cp.created_at,
			cp.updated_at
		FROM counterparty cp`
	countCounterpartyQuery  = `SELECT COUNT(*) as count FROM counterparty`
	insertCounterpartyQuery = `
		INSERT INTO counterparty (
		    tin,
			name,
			type,
			legal_type,
			legal_address,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	updateCounterpartyQuery = `
		UPDATE counterparty
		SET name = $1, tin = $2, type = $3, legal_type = $4, legal_address = $5, updated_at = $6
		WHERE id = $7`
	deleteCounterpartyQuery = `DELETE FROM counterparty WHERE id = $1`
)

type GormCounterpartyRepository struct{}

func NewCounterpartyRepository() counterparty.Repository {
	return &GormCounterpartyRepository{}
}

func (g *GormCounterpartyRepository) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error) {
	var args []interface{}
	where := []string{"1 = 1"}
	if params.CreatedAt.To != "" && params.CreatedAt.From != "" {
		where = append(where, fmt.Sprintf("cp.created_at BETWEEN $%d and $%d", len(where), len(where)+1))
		args = append(args, params.CreatedAt.From, params.CreatedAt.To)
	}
	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("cp.%s::VARCHAR ILIKE $%d", params.Field, len(where)))
		args = append(args, "%"+params.Query+"%")
	}
	q := repo.Join(
		findCounterpartyQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryCounterparties(ctx, q, args...)
}

func (g *GormCounterpartyRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, countCounterpartyQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormCounterpartyRepository) GetAll(ctx context.Context) ([]counterparty.Counterparty, error) {
	return g.queryCounterparties(ctx, findCounterpartyQuery)
}

func (g *GormCounterpartyRepository) GetByID(ctx context.Context, id uint) (counterparty.Counterparty, error) {
	counterparties, err := g.queryCounterparties(ctx, repo.Join(findCounterpartyQuery, "WHERE id = $1"), id)
	if err != nil {
		return nil, err
	}
	if len(counterparties) == 0 {
		return nil, ErrCounterpartyNotFound
	}
	return counterparties[0], nil
}

func (g *GormCounterpartyRepository) Create(ctx context.Context, data counterparty.Counterparty) (counterparty.Counterparty, error) {
	entity, err := toDBCounterparty(data)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.Name,
		entity.TIN,
		entity.Type,
		entity.LegalType,
		entity.LegalAddress,
		entity.CreatedAt,
		entity.UpdatedAt,
	}
	row := tx.QueryRow(ctx, insertCounterpartyQuery, args...)
	var id uint
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *GormCounterpartyRepository) Update(ctx context.Context, data counterparty.Counterparty) error {
	entity, err := toDBCounterparty(data)
	if err != nil {
		return err
	}
	args := []interface{}{
		entity.Name,
		entity.TIN,
		entity.Type,
		entity.LegalType,
		entity.LegalAddress,
		entity.UpdatedAt,
		entity.ID,
	}
	return g.execQuery(ctx, updateCounterpartyQuery, args...)
}

func (g *GormCounterpartyRepository) Delete(ctx context.Context, id uint) error {
	return g.execQuery(ctx, deleteCounterpartyQuery, id)
}

func (g *GormCounterpartyRepository) queryCounterparties(ctx context.Context, query string, args ...interface{}) ([]counterparty.Counterparty, error) {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dbRows []*models.Counterparty
	for rows.Next() {
		var r models.Counterparty
		if err := rows.Scan(
			&r.ID,
			&r.Name,
			&r.TIN,
			&r.Type,
			&r.LegalType,
			&r.LegalAddress,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, &r)
	}
	return mapping.MapDbModels(dbRows, toDomainCounterparty)
}

func (g *GormCounterpartyRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
