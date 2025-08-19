package projectstage

import (
	"time"

	"github.com/google/uuid"
)

type ProjectStage interface {
	ID() uuid.UUID
	SetID(uuid.UUID)

	ProjectID() uuid.UUID

	StageNumber() int
	UpdateStageNumber(int) ProjectStage

	Description() string
	UpdateDescription(string) ProjectStage

	TotalAmount() int64
	UpdateTotalAmount(int64) ProjectStage

	StartDate() *time.Time
	UpdateStartDate(*time.Time) ProjectStage

	PlannedEndDate() *time.Time
	UpdatePlannedEndDate(*time.Time) ProjectStage

	FactualEndDate() *time.Time
	UpdateFactualEndDate(*time.Time) ProjectStage

	CreatedAt() time.Time
	UpdatedAt() time.Time

	// Calculated fields
	PaidAmount() int64
	RemainingAmount() int64
}
