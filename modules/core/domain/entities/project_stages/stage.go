package project_stages

import "time"

type ProjectStage struct {
	ID        uint
	ProjectID uint
	Name      string
	Margin    float64
	Risks     float64
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateDTO struct {
	ProjectID uint      `schema:"project_id,required"`
	Name      string    `schema:"name,required"`
	Margin    float64   `schema:"margin,required"`
	Risks     float64   `schema:"risks,required"`
	StartDate time.Time `schema:"start_date,required"`
	EndDate   time.Time `schema:"end_date,required"`
}

type UpdateDTO struct {
	ProjectID uint      `schema:"project_id"`
	Name      string    `schema:"name"`
	Margin    float64   `schema:"margin"`
	Risks     float64   `schema:"risks"`
	StartDate time.Time `schema:"start_date"`
	EndDate   time.Time `schema:"end_date"`
}

func (p *CreateDTO) ToEntity() *ProjectStage {
	return &ProjectStage{
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Margin:    p.Margin,
		Risks:     p.Risks,
		StartDate: p.StartDate,
		EndDate:   p.EndDate,
	}
}

func (p *UpdateDTO) ToEntity(id uint) *ProjectStage {
	return &ProjectStage{
		ID:        id,
		ProjectID: p.ProjectID,
		Name:      p.Name,
		Margin:    p.Margin,
		Risks:     p.Risks,
		StartDate: p.StartDate,
		EndDate:   p.EndDate,
	}
}
