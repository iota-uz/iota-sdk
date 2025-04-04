package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

var (
	ErrTenantNotFound = fmt.Errorf("tenant not found")
)

const (
	tenantFindQuery = `SELECT id, name, domain, is_active, created_at, updated_at FROM tenants`
)

type TenantRepository struct{}

func NewTenantRepository() tenant.Repository {
	return &TenantRepository{}
}

func (r *TenantRepository) GetByID(ctx context.Context, id uuid.UUID) (*tenant.Tenant, error) {
	query := tenantFindQuery + " WHERE id = $1"
	tenants, err := r.queryTenants(ctx, query, id.String())
	if err != nil {
		return nil, err
	}

	if len(tenants) == 0 {
		return nil, ErrTenantNotFound
	}

	return tenants[0], nil
}

func (r *TenantRepository) GetByDomain(ctx context.Context, domain string) (*tenant.Tenant, error) {
	query := tenantFindQuery + " WHERE domain = $1"
	tenants, err := r.queryTenants(ctx, query, domain)
	if err != nil {
		return nil, err
	}

	if len(tenants) == 0 {
		return nil, ErrTenantNotFound
	}

	return tenants[0], nil
}

func (r *TenantRepository) Create(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error) {
	query := `
		INSERT INTO tenants (id, name, domain, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var idStr string
	if err := tx.QueryRow(
		ctx,
		query,
		t.ID().String(),
		t.Name(),
		t.Domain(),
		t.IsActive(),
		t.CreatedAt(),
		t.UpdatedAt(),
	).Scan(&idStr); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *TenantRepository) Update(ctx context.Context, t *tenant.Tenant) (*tenant.Tenant, error) {
	query := `
		UPDATE tenants
		SET name = $1, domain = $2, is_active = $3, updated_at = $4
		WHERE id = $5
		RETURNING id
	`
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var idStr string
	if err := tx.QueryRow(
		ctx,
		query,
		t.Name(),
		t.Domain(),
		t.IsActive(),
		t.UpdatedAt(),
		t.ID().String(),
	).Scan(&idStr); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *TenantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM tenants WHERE id = $1`
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, query, id.String())
	return err
}

func (r *TenantRepository) List(ctx context.Context) ([]*tenant.Tenant, error) {
	return r.queryTenants(ctx, tenantFindQuery)
}

func (r *TenantRepository) queryTenants(ctx context.Context, query string, args ...interface{}) ([]*tenant.Tenant, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var tenants []*tenant.Tenant
	for rows.Next() {
		var t models.Tenant
		if err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Domain,
			&t.IsActive,
			&t.CreatedAt,
			&t.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan tenant row")
		}
		tenants = append(tenants, toDomainTenant(&t))
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	return tenants, nil
}

func toDomainTenant(t *models.Tenant) *tenant.Tenant {
	id, err := uuid.Parse(t.ID)
	if err != nil {
		// Log error or handle it appropriately
		id = uuid.Nil
	}
	
	return tenant.New(
		t.Name,
		tenant.WithID(id),
		tenant.WithDomain(t.Domain.String),
		tenant.WithIsActive(t.IsActive),
		tenant.WithCreatedAt(t.CreatedAt),
		tenant.WithUpdatedAt(t.UpdatedAt),
	)
}
