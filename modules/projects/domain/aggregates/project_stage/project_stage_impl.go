package projectstage

import (
	"time"

	"github.com/google/uuid"
)

type Option func(ps *projectStage)

func WithID(id uuid.UUID) Option {
	return func(ps *projectStage) {
		ps.id = id
	}
}

func WithStageNumber(stageNumber int) Option {
	return func(ps *projectStage) {
		ps.stageNumber = stageNumber
	}
}

func WithDescription(description string) Option {
	return func(ps *projectStage) {
		ps.description = description
	}
}

func WithTotalAmount(totalAmount int64) Option {
	return func(ps *projectStage) {
		ps.totalAmount = totalAmount
	}
}

func WithStartDate(startDate *time.Time) Option {
	return func(ps *projectStage) {
		ps.startDate = startDate
	}
}

func WithPlannedEndDate(plannedEndDate *time.Time) Option {
	return func(ps *projectStage) {
		ps.plannedEndDate = plannedEndDate
	}
}

func WithFactualEndDate(factualEndDate *time.Time) Option {
	return func(ps *projectStage) {
		ps.factualEndDate = factualEndDate
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(ps *projectStage) {
		ps.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(ps *projectStage) {
		ps.updatedAt = updatedAt
	}
}

func WithPaidAmount(paidAmount int64) Option {
	return func(ps *projectStage) {
		ps.paidAmount = paidAmount
	}
}

func New(
	projectID uuid.UUID,
	stageNumber int,
	totalAmount int64,
	opts ...Option,
) ProjectStage {
	ps := &projectStage{
		id:             uuid.New(),
		projectID:      projectID,
		stageNumber:    stageNumber,
		description:    "",
		totalAmount:    totalAmount,
		startDate:      nil,
		plannedEndDate: nil,
		factualEndDate: nil,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
		paidAmount:     0,
	}

	for _, opt := range opts {
		opt(ps)
	}
	return ps
}

type projectStage struct {
	id             uuid.UUID
	projectID      uuid.UUID
	stageNumber    int
	description    string
	totalAmount    int64
	startDate      *time.Time
	plannedEndDate *time.Time
	factualEndDate *time.Time
	createdAt      time.Time
	updatedAt      time.Time
	paidAmount     int64
}

func (ps *projectStage) ID() uuid.UUID {
	return ps.id
}

func (ps *projectStage) SetID(id uuid.UUID) {
	ps.id = id
}

func (ps *projectStage) ProjectID() uuid.UUID {
	return ps.projectID
}

func (ps *projectStage) StageNumber() int {
	return ps.stageNumber
}

func (ps *projectStage) UpdateStageNumber(stageNumber int) ProjectStage {
	res := *ps
	res.stageNumber = stageNumber
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) Description() string {
	return ps.description
}

func (ps *projectStage) UpdateDescription(description string) ProjectStage {
	res := *ps
	res.description = description
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) TotalAmount() int64 {
	return ps.totalAmount
}

func (ps *projectStage) UpdateTotalAmount(totalAmount int64) ProjectStage {
	res := *ps
	res.totalAmount = totalAmount
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) StartDate() *time.Time {
	return ps.startDate
}

func (ps *projectStage) UpdateStartDate(startDate *time.Time) ProjectStage {
	res := *ps
	res.startDate = startDate
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) PlannedEndDate() *time.Time {
	return ps.plannedEndDate
}

func (ps *projectStage) UpdatePlannedEndDate(plannedEndDate *time.Time) ProjectStage {
	res := *ps
	res.plannedEndDate = plannedEndDate
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) FactualEndDate() *time.Time {
	return ps.factualEndDate
}

func (ps *projectStage) UpdateFactualEndDate(factualEndDate *time.Time) ProjectStage {
	res := *ps
	res.factualEndDate = factualEndDate
	res.updatedAt = time.Now()
	return &res
}

func (ps *projectStage) CreatedAt() time.Time {
	return ps.createdAt
}

func (ps *projectStage) UpdatedAt() time.Time {
	return ps.updatedAt
}

func (ps *projectStage) PaidAmount() int64 {
	return ps.paidAmount
}

func (ps *projectStage) RemainingAmount() int64 {
	return ps.totalAmount - ps.paidAmount
}
