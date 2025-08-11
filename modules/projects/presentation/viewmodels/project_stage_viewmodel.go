package viewmodels

import "time"

type ProjectStageViewModel struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id"`
	StageNumber    int        `json:"stage_number"`
	Description    string     `json:"description"`
	TotalAmount    int64      `json:"total_amount"`
	PaidAmount     int64      `json:"paid_amount"`
	StartDate      *time.Time `json:"start_date"`
	PlannedEndDate *time.Time `json:"planned_end_date"`
	FactualEndDate *time.Time `json:"factual_end_date"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
