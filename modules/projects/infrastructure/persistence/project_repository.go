package persistence

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

var (
	ErrProjectNotFound = errors.New("project not found")
)

const (
	findProjectQuery = `
		SELECT p.id,
			p.tenant_id,
			p.counterparty_id,
			p.name,
			p.description,
			p.created_at,
			p.updated_at
		FROM projects p`
	insertProjectQuery = `
		INSERT INTO projects (
			tenant_id,
			counterparty_id,
			name,
			description,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`
	updateProjectQuery = `
		UPDATE projects
		SET counterparty_id = $1, name = $2, description = $3, updated_at = $4
		WHERE id = $5`
	deleteProjectQuery = `DELETE FROM projects WHERE id = $1`
)

type ProjectRepository struct {
	fieldMap map[project.Field]string
}

func NewProjectRepository() project.Repository {
	return &ProjectRepository{
		fieldMap: map[project.Field]string{
			project.ID:             "p.id",
			project.Name:           "p.name",
			project.CounterpartyID: "p.counterparty_id",
			project.Description:    "p.description",
			project.CreatedAt:      "p.created_at",
		},
	}
}

func (r *ProjectRepository) Save(ctx context.Context, proj project.Project) (project.Project, error) {
	exists, err := r.exists(ctx, proj.ID())
	if err != nil {
		return nil, err
	}
	if exists {
		return r.update(ctx, proj)
	}
	return r.create(ctx, proj)
}

func (r *ProjectRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, err
	}

	var count int
	err = tx.QueryRow(ctx, "SELECT COUNT(*) FROM projects WHERE id = $1 AND tenant_id = $2", id, tenantID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ProjectRepository) create(ctx context.Context, proj project.Project) (project.Project, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var id uuid.UUID
	if proj.ID() == uuid.Nil {
		id = uuid.New()
		proj = proj.UpdateName(proj.Name()).UpdateCounterpartyID(proj.CounterpartyID()).UpdateDescription(proj.Description())
		proj.SetID(id)
	} else {
		id = proj.ID()
	}

	err = tx.QueryRow(
		ctx,
		insertProjectQuery,
		tenantID,
		proj.CounterpartyID(),
		proj.Name(),
		mapping.ValueToSQLNullString(proj.Description()),
		proj.CreatedAt(),
		proj.UpdatedAt(),
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	proj.SetID(id)
	return proj, nil
}

func (r *ProjectRepository) update(ctx context.Context, proj project.Project) (project.Project, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		ctx,
		updateProjectQuery,
		proj.CounterpartyID(),
		proj.Name(),
		mapping.ValueToSQLNullString(proj.Description()),
		proj.UpdatedAt(),
		proj.ID(),
	)
	if err != nil {
		return nil, err
	}
	return proj, nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, deleteProjectQuery, id)
	return err
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (project.Project, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	projects, err := r.queryProjects(ctx, findProjectQuery+` WHERE p.id = $1 AND p.tenant_id = $2`, id, tenantID)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, ErrProjectNotFound
	}

	return projects[0], nil
}

func (r *ProjectRepository) GetAll(ctx context.Context) ([]project.Project, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	return r.queryProjects(ctx, findProjectQuery+` WHERE p.tenant_id = $1 ORDER BY p.created_at DESC`, tenantID)
}

func (r *ProjectRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]project.Project, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	orderBy := "p.created_at DESC"
	if len(sortBy) > 0 {
		orderBy = fmt.Sprintf("p.%s", sortBy[0])
	}

	query := findProjectQuery + ` WHERE p.tenant_id = $1 ORDER BY ` + orderBy + ` LIMIT $2 OFFSET $3`

	return r.queryProjects(ctx, query, tenantID, limit, offset)
}

func (r *ProjectRepository) GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]project.Project, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	query := findProjectQuery + ` WHERE p.tenant_id = $1 AND p.counterparty_id = $2 ORDER BY p.created_at DESC`

	return r.queryProjects(ctx, query, tenantID, counterpartyID)
}

func (r *ProjectRepository) queryProjects(ctx context.Context, query string, args ...interface{}) ([]project.Project, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbRows []*models.Project
	for rows.Next() {
		r := &models.Project{}
		if err := rows.Scan(
			&r.ID,
			&r.TenantID,
			&r.CounterpartyID,
			&r.Name,
			&r.Description,
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

	return mapping.MapViewModels(dbRows, func(model *models.Project) project.Project {
		return ProjectModelToDomain(*model)
	}), nil
}
