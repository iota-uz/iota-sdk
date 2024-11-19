package position

import (
	"time"

	model "github.com/iota-agency/iota-sdk/pkg/interfaces/graph/gqlmodels"
)

type Position struct {
	ID          int64
	Name        string
	Description *string
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

func (p *Position) ToGraph() *model.Position {
	return &model.Position{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   *p.CreatedAt,
		UpdatedAt:   *p.UpdatedAt,
	}
}
