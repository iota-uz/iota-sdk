package models

import (
	"database/sql"
	"time"
)

type Project struct {
	ID             string
	TenantID       string
	CounterpartyID string
	Name           string
	Description    sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProjectStage struct {
	ID             string
	ProjectID      string
	StageNumber    int
	Description    sql.NullString
	TotalAmount    int64
	StartDate      sql.NullTime
	PlannedEndDate sql.NullTime
	FactualEndDate sql.NullTime
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ProjectStagePayment struct {
	ID             string
	ProjectStageID string
	PaymentID      string
	CreatedAt      time.Time
}
