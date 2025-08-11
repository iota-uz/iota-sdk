package projectstage

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	ProjectID
	StageNumber
	Description
	TotalAmount
	StartDate
	PlannedEndDate
	FactualEndDate
	CreatedAt
	UpdatedAt
)

type SortBy = repo.SortBy[Field]

type Repository interface {
	Save(ctx context.Context, stage ProjectStage) (ProjectStage, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (ProjectStage, error)
	GetAll(ctx context.Context) ([]ProjectStage, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]ProjectStage, error)
	Count(ctx context.Context) (int64, error)
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]ProjectStage, error)
	GetNextStageNumber(ctx context.Context, projectID uuid.UUID) (int, error)
	UpdatePaidAmounts(ctx context.Context, stageID uuid.UUID) error
}
