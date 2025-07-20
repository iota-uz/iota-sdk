package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrProjectStageNotFound = errors.New("project stage not found")
)

const (
	findProjectStageQuery = `
		SELECT ps.id,
			ps.project_id,
			ps.stage_number,
			ps.description,
			ps.total_amount,
			ps.start_date,
			ps.planned_end_date,
			ps.factual_end_date,
			ps.created_at,
			ps.updated_at
		FROM project_stages ps
		JOIN projects p ON ps.project_id = p.id`
	insertProjectStageQuery = `
		INSERT INTO project_stages (
			project_id,
			stage_number,
			description,
			total_amount,
			start_date,
			planned_end_date,
			factual_end_date,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`
	updateProjectStageQuery = `
		UPDATE project_stages
		SET stage_number = $1, description = $2, total_amount = $3, 
			start_date = $4, planned_end_date = $5, factual_end_date = $6, updated_at = $7
		WHERE id = $8`
	deleteProjectStageQuery = `DELETE FROM project_stages WHERE id = $1`
	getNextStageNumberQuery = `
		SELECT COALESCE(MAX(stage_number), 0) + 1 
		FROM project_stages 
		WHERE project_id = $1`
	getPaidAmountQuery = `
		SELECT COALESCE(SUM(t.amount), 0)
		FROM project_stage_payments psp
		JOIN payments p ON psp.payment_id = p.id
		JOIN transactions t ON p.transaction_id = t.id
		WHERE psp.project_stage_id = $1`
)

type ProjectStageRepository struct {
	fieldMap map[projectstage.Field]string
}

func NewProjectStageRepository() projectstage.Repository {
	return &ProjectStageRepository{
		fieldMap: map[projectstage.Field]string{
			projectstage.ID:             "ps.id",
			projectstage.ProjectID:      "ps.project_id",
			projectstage.StageNumber:    "ps.stage_number",
			projectstage.Description:    "ps.description",
			projectstage.TotalAmount:    "ps.total_amount",
			projectstage.StartDate:      "ps.start_date",
			projectstage.PlannedEndDate: "ps.planned_end_date",
			projectstage.FactualEndDate: "ps.factual_end_date",
			projectstage.CreatedAt:      "ps.created_at",
		},
	}
}

func (r *ProjectStageRepository) Save(ctx context.Context, stage projectstage.ProjectStage) (projectstage.ProjectStage, error) {
	exists, err := r.exists(ctx, stage.ID())
	if err != nil {
		return nil, err
	}
	if exists {
		return r.update(ctx, stage)
	}
	return r.create(ctx, stage)
}

func (r *ProjectStageRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, err
	}

	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM project_stages ps JOIN projects p ON ps.project_id = p.id WHERE ps.id = $1 AND p.tenant_id = $2", id, tenantID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ProjectStageRepository) create(ctx context.Context, stage projectstage.ProjectStage) (projectstage.ProjectStage, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if stage.ID() == uuid.Nil {
		id = uuid.New()
		stage.SetID(id)
	} else {
		id = stage.ID()
	}

	err = tx.QueryRow(
		ctx,
		insertProjectStageQuery,
		stage.ProjectID(),
		stage.StageNumber(),
		mapping.ValueToSQLNullString(stage.Description()),
		stage.TotalAmount(),
		mapping.PointerToSQLNullTime(stage.StartDate()),
		mapping.PointerToSQLNullTime(stage.PlannedEndDate()),
		mapping.PointerToSQLNullTime(stage.FactualEndDate()),
		stage.CreatedAt(),
		stage.UpdatedAt(),
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	stage.SetID(id)
	return stage, nil
}

func (r *ProjectStageRepository) update(ctx context.Context, stage projectstage.ProjectStage) (projectstage.ProjectStage, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		ctx,
		updateProjectStageQuery,
		stage.StageNumber(),
		mapping.ValueToSQLNullString(stage.Description()),
		stage.TotalAmount(),
		mapping.PointerToSQLNullTime(stage.StartDate()),
		mapping.PointerToSQLNullTime(stage.PlannedEndDate()),
		mapping.PointerToSQLNullTime(stage.FactualEndDate()),
		stage.UpdatedAt(),
		stage.ID(),
	)
	if err != nil {
		return nil, err
	}
	return stage, nil
}

func (r *ProjectStageRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, deleteProjectStageQuery, id)
	return err
}

func (r *ProjectStageRepository) GetByID(ctx context.Context, id uuid.UUID) (projectstage.ProjectStage, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	stages, err := r.queryProjectStages(ctx, findProjectStageQuery+` WHERE ps.id = $1 AND p.tenant_id = $2`, id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(stages) == 0 {
		return nil, ErrProjectStageNotFound
	}

	stage := stages[0]

	// Get paid amount
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var paidAmount int64
	err = tx.QueryRow(ctx, getPaidAmountQuery, id).Scan(&paidAmount)
	if err != nil {
		paidAmount = 0
	}

	// Update with paid amount
	stageWithPaid := projectstage.New(
		stage.ProjectID(),
		stage.StageNumber(),
		stage.TotalAmount(),
		projectstage.WithID(stage.ID()),
		projectstage.WithDescription(stage.Description()),
		projectstage.WithStartDate(stage.StartDate()),
		projectstage.WithPlannedEndDate(stage.PlannedEndDate()),
		projectstage.WithFactualEndDate(stage.FactualEndDate()),
		projectstage.WithCreatedAt(stage.CreatedAt()),
		projectstage.WithUpdatedAt(stage.UpdatedAt()),
		projectstage.WithPaidAmount(paidAmount),
	)

	return stageWithPaid, nil
}

func (r *ProjectStageRepository) GetAll(ctx context.Context) ([]projectstage.ProjectStage, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	return r.queryProjectStages(ctx, findProjectStageQuery+` WHERE p.tenant_id = $1 ORDER BY ps.stage_number ASC`, tenantID)
}

func (r *ProjectStageRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]projectstage.ProjectStage, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	orderBy := "ps.stage_number ASC"
	if len(sortBy) > 0 {
		orderBy = fmt.Sprintf("ps.%s", sortBy[0])
	}

	query := findProjectStageQuery + ` WHERE p.tenant_id = $1 ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	return r.queryProjectStages(ctx, query, tenantID, limit, offset)
}

func (r *ProjectStageRepository) GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]projectstage.ProjectStage, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	query := findProjectStageQuery + ` WHERE ps.project_id = $1 AND p.tenant_id = $2 ORDER BY ps.stage_number ASC`

	return r.queryProjectStages(ctx, query, projectID, tenantID)
}

func (r *ProjectStageRepository) GetNextStageNumber(ctx context.Context, projectID uuid.UUID) (int, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 1, err
	}

	var nextNumber int
	err = tx.QueryRow(ctx, getNextStageNumberQuery, projectID).Scan(&nextNumber)
	if err != nil {
		return 1, err
	}
	return nextNumber, nil
}

func (r *ProjectStageRepository) UpdatePaidAmounts(ctx context.Context, stageID uuid.UUID) error {
	// This method is called after payment assignments change
	// The paid amount calculation is done in GetByID method
	return nil
}

func (r *ProjectStageRepository) queryProjectStages(ctx context.Context, query string, args ...interface{}) ([]projectstage.ProjectStage, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbRows []*models.ProjectStage
	for rows.Next() {
		r := &models.ProjectStage{}
		if err := rows.Scan(
			&r.ID,
			&r.ProjectID,
			&r.StageNumber,
			&r.Description,
			&r.TotalAmount,
			&r.StartDate,
			&r.PlannedEndDate,
			&r.FactualEndDate,
			&r.CreatedAt,
			&r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		dbRows = append(dbRows, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mapping.MapViewModels(dbRows, func(model *models.ProjectStage) projectstage.ProjectStage {
		return ProjectStageModelToDomain(*model)
	}), nil
}
