package persistence

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

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
	updateAcceptanceStatusQuery = `
		UPDATE project_acceptance_documents
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4 AND status = ANY($5)`
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
	const op serrors.Op = "AcceptanceRepository.GetByID"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	documents, err := r.query(ctx, findAcceptanceQuery+` WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if len(documents) == 0 {
		return nil, serrors.Wrap(op, acceptance.ErrNotFound)
	}
	return documents[0], nil
}

func (r *AcceptanceRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]acceptance.Document, error) {
	const op serrors.Op = "AcceptanceRepository.GetByProjectID"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	documents, err := r.query(ctx,
		findAcceptanceQuery+` WHERE project_id = $1 AND tenant_id = $2 ORDER BY document_date DESC, created_at DESC`,
		projectID, tenantID,
	)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return documents, nil
}

func (r *AcceptanceRepository) SignedTotals(ctx context.Context, projectIDs []uuid.UUID) ([]*money.Money, error) {
	const op serrors.Op = "AcceptanceRepository.SignedTotals"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	totals, err := sumByCurrency(ctx, signedAcceptanceSumQuery, tenantID, projectIDs)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return totals, nil
}

func (r *AcceptanceRepository) Create(ctx context.Context, document acceptance.Document) (acceptance.Document, error) {
	const op serrors.Op = "AcceptanceRepository.Create"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
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
		return nil, serrors.FromDB(op, err)
	}
	created, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return created, nil
}

func (r *AcceptanceRepository) UpdateStatus(
	ctx context.Context, document acceptance.Document, from ...acceptance.Status,
) (acceptance.Document, error) {
	const op serrors.Op = "AcceptanceRepository.UpdateStatus"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	statuses := make([]string, 0, len(from))
	for _, status := range from {
		statuses = append(statuses, string(status))
	}
	tag, err := tx.Exec(ctx, updateAcceptanceStatusQuery,
		string(document.Status()), document.UpdatedAt(), document.ID(), tenantID, statuses,
	)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, serrors.Wrap(op, acceptance.ErrStatus)
	}
	updated, err := r.GetByID(ctx, document.ID())
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return updated, nil
}

func (r *AcceptanceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "AcceptanceRepository.Delete"
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := tx.Exec(ctx, deleteAcceptanceQuery, id, tenantID); err != nil {
		return serrors.FromDB(op, err)
	}
	return nil
}

func (r *AcceptanceRepository) query(ctx context.Context, query string, args ...interface{}) ([]acceptance.Document, error) {
	const op serrors.Op = "AcceptanceRepository.query"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()

	documents := make([]acceptance.Document, 0)
	for rows.Next() {
		var m models.AcceptanceDocument
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.ProjectID, &m.Kind, &m.Number, &m.DocumentDate, &m.Amount,
			&m.CurrencyID, &m.Status, &m.Description, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		documents = append(documents, AcceptanceModelToDomain(m))
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return documents, nil
}

// sumByCurrency runs a query returning (currency, amount) rows for the
// tenant's projects.
func sumByCurrency(ctx context.Context, query string, tenantID uuid.UUID, projectIDs []uuid.UUID) ([]*money.Money, error) {
	const op serrors.Op = "persistence.sumByCurrency"
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	rows, err := tx.Query(ctx, query, tenantID, projectIDs)
	if err != nil {
		return nil, serrors.FromDB(op, err)
	}
	defer rows.Close()

	totals := make([]*money.Money, 0)
	for rows.Next() {
		var currency string
		var amount int64
		if err := rows.Scan(&currency, &amount); err != nil {
			return nil, serrors.FromDB(op, err)
		}
		totals = append(totals, money.New(amount, currency))
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return totals, nil
}
