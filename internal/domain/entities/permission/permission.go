package permission

import (
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Permission struct {
	ID          uint
	Resource    Resource
	Action      Action
	Description string
	Modifier    string
}

func (p *Permission) ToGraph() *model.Permission {
	return &model.Permission{
		ID:          int64(p.ID),
		Description: &p.Description,
		Resource:    (*string)(&p.Resource),
		Action:      (*string)(&p.Action),
		Modifier:    &p.Modifier,
	}
}
