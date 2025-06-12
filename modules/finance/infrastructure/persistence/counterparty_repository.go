package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrCounterpartyNotFound = errors.New("counterparty not found")
)

const (
	findCounterpartyQuery = `
		SELECT cp.id,
			cp.tenant_id,
			cp.name,
			cp.tin,
			cp.type,
			cp.legal_type,
			cp.legal_address,
			cp.created_at,
			cp.updated_at
		FROM counterparty cp`
	countCounterpartyQuery  = `SELECT COUNT(*) as count FROM counterparty cp`
	insertCounterpartyQuery = `
		INSERT INTO counterparty (
			name,
		    tin,
			type,
			legal_type,
			legal_address,
			created_at,
			updated_at,
			tenant_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`
	updateCounterpartyQuery = `
		UPDATE counterparty
		SET name = $1, tin = $2, type = $3, legal_type = $4, legal_address = $5, updated_at = $6
		WHERE id = $7`
	deleteCounterpartyQuery = `DELETE FROM counterparty WHERE id = $1`
)

type GormCounterpartyRepository struct {
	fieldMap map[counterparty.Field]string
}

func NewCounterpartyRepository() counterparty.Repository {
	return &GormCounterpartyRepository{
		fieldMap: map[counterparty.Field]string{
			counterparty.NameField:         "cp.name",
			counterparty.TinField:          "cp.tin",
			counterparty.TypeField:         "cp.type",
			counterparty.LegalTypeField:    "cp.legal_type",
			counterparty.LegalAddressField: "cp.legal_address",
			counterparty.CreatedAtField:    "cp.created_at",
			counterparty.UpdatedAtField:    "cp.updated_at",
			counterparty.TenantIDField:     "cp.tenant_id",
		},
	}
}

func (g *GormCounterpartyRepository) buildCounterpartyFilters(params *counterparty.FindParams) ([]string, []interface{}, error) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	for _, filter := range params.Filters {
		column, ok := g.fieldMap[filter.Column]
		if !ok {
			return nil, nil, errors.Wrap(fmt.Errorf("unknown filter field: %v", filter.Column), "invalid filter")
		}

		where = append(where, filter.Filter.String(column, len(args)+1))
		args = append(args, filter.Filter.Value()...)
	}

	if params.Search != "" {
		index := len(args) + 1
		where = append(
			where,
			fmt.Sprintf(
				"(cp.name ILIKE $%d OR cp.tin ILIKE $%d)",
				index,
				index,
			),
		)
		args = append(args, "%"+params.Search+"%")
	}

	return where, args, nil
}

func (g *GormCounterpartyRepository) GetPaginated(ctx context.Context, params *counterparty.FindParams) ([]counterparty.Counterparty, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	where, args, err := g.buildCounterpartyFilters(params)
	if err != nil {
		return nil, err
	}

	where = append(where, fmt.Sprintf("cp.tenant_id = $%d", len(args)+1))
	args = append(args, tenantID)

	q := repo.Join(
		findCounterpartyQuery,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.fieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryCounterparties(ctx, q, args...)
}

func (g *GormCounterpartyRepository) Count(ctx context.Context, params *counterparty.FindParams) (int64, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant from context")
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	where, args, err := g.buildCounterpartyFilters(params)
	if err != nil {
		return 0, err
	}

	where = append(where, fmt.Sprintf("cp.tenant_id = $%d", len(args)+1))
	args = append(args, tenantID)

	query := repo.Join(
		countCounterpartyQuery,
		repo.JoinWhere(where...),
	)

	var count int64
	err = tx.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, "failed to count counterparties")
	}
	return count, nil
}

func (g *GormCounterpartyRepository) GetAll(ctx context.Context) ([]counterparty.Counterparty, error) {
	return g.queryCounterparties(ctx, findCounterpartyQuery)
}

func (g *GormCounterpartyRepository) GetByID(ctx context.Context, id uuid.UUID) (counterparty.Counterparty, error) {
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
	entity, err := ToDBCounterparty(data)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant from context: %w", err)
	}

	args := []interface{}{
		entity.Name,
		entity.Tin,
		entity.Type,
		entity.LegalType,
		entity.LegalAddress,
		entity.CreatedAt,
		entity.UpdatedAt,
		tenantID,
	}
	row := tx.QueryRow(ctx, insertCounterpartyQuery, args...)
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, id)
}

func (g *GormCounterpartyRepository) Update(ctx context.Context, data counterparty.Counterparty) (counterparty.Counterparty, error) {
	entity, err := ToDBCounterparty(data)
	if err != nil {
		return nil, err
	}
	args := []interface{}{
		entity.Name,
		entity.Tin,
		entity.Type,
		entity.LegalType,
		entity.LegalAddress,
		entity.UpdatedAt,
		entity.ID,
	}
	if err := g.execQuery(ctx, updateCounterpartyQuery, args...); err != nil {
		return nil, err
	}
	return g.GetByID(ctx, data.ID())
}

func (g *GormCounterpartyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return g.execQuery(ctx, deleteCounterpartyQuery, id)
}

func (g *GormCounterpartyRepository) queryCounterparties(ctx context.Context, query string, args ...interface{}) ([]counterparty.Counterparty, error) {
	tx, err := composables.UseTx(ctx)
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
			&r.TenantID,
			&r.Name,
			&r.Tin,
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
	return mapping.MapDBModels(dbRows, ToDomainCounterparty)
}

func (g *GormCounterpartyRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
