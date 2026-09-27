package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

var ErrAcceptanceDocumentNotFound = errors.New("acceptance document not found")

const (
	findAcceptanceQuery = `
		SELECT id, tenant_id, project_id, kind, number, document_date, amount,
			currency_id, status, description, created_at, updated_at
		FROM project_acceptance_documents`
	insertAcceptanceQuery = `
		INSERT INTO project_acceptance_documents (
			tenant_id, project_id, kind, number, document_date, amount,
			currency_id, status, description, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`
	updateAcceptanceQuery = `
		UPDATE project_acceptance_documents
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4`
	deleteAcceptanceQuery    = `DELETE FROM project_acceptance_documents WHERE id = $1 AND tenant_id = $2`
	signedAcceptanceSumQuery = `
		SELECT currency_id, SUM(amount)::bigint
		FROM project_acceptance_documents
		WHERE tenant_id = $1 AND project_id = ANY($2) AND status = 'SIGNED'
		GROUP BY currency_id`
)

type AcceptanceRepository struct{}

func NewAcceptanceRepository() acceptance.Repository {
	return &AcceptanceRepository{}
}

func (r *AcceptanceRepository) GetByID(ctx context.Context, id uuid.UUID) (acceptance.Document, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	documents, err := r.query(ctx, findAcceptanceQuery+` WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, ErrAcceptanceDocumentNotFound
	}
	return documents[0], nil
}

func (r *AcceptanceRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]acceptance.Document, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return r.query(ctx,
		findAcceptanceQuery+` WHERE project_id = $1 AND tenant_id = $2 ORDER BY document_date DESC, created_at DESC`,
		projectID, tenantID,
	)
}

func (r *AcceptanceRepository) SignedTotals(ctx context.Context, projectIDs []uuid.UUID) ([]*money.Money, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return sumByCurrency(ctx, signedAcceptanceSumQuery, tenantID, projectIDs)
}

func (r *AcceptanceRepository) Create(ctx context.Context, document acceptance.Document) (acceptance.Document, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if err := tx.QueryRow(ctx, insertAcceptanceQuery,
		tenantID,
		document.ProjectID(),
		string(document.Kind()),
		document.Number(),
		document.Date(),
		document.Amount().Amount(),
		document.Amount().Currency().Code,
		string(document.Status()),
		mapping.ValueToSQLNullString(document.Description()),
		document.CreatedAt(),
		document.UpdatedAt(),
	).Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *AcceptanceRepository) Update(ctx context.Context, document acceptance.Document) (acceptance.Document, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, updateAcceptanceQuery,
		string(document.Status()), document.UpdatedAt(), document.ID(), tenantID,
	); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, document.ID())
}

func (r *AcceptanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, deleteAcceptanceQuery, id, tenantID)
	return err
}

func (r *AcceptanceRepository) query(ctx context.Context, query string, args ...interface{}) ([]acceptance.Document, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	documents := make([]acceptance.Document, 0)
	for rows.Next() {
		var m models.AcceptanceDocument
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ProjectID, &m.Kind, &m.Number, &m.DocumentDate, &m.Amount,
			&m.CurrencyID, &m.Status, &m.Description, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		documents = append(documents, AcceptanceModelToDomain(m))
	}
	return documents, rows.Err()
}

// sumByCurrency runs a query returning (currency, amount) rows for the
// tenant's projects.
func sumByCurrency(ctx context.Context, query string, tenantID uuid.UUID, projectIDs []uuid.UUID) ([]*money.Money, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, tenantID, projectIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	totals := make([]*money.Money, 0)
	for rows.Next() {
		var currency string
		var amount int64
		if err := rows.Scan(&currency, &amount); err != nil {
			return nil, err
		}
		totals = append(totals, money.New(amount, currency))
	}
	return totals, rows.Err()
}
