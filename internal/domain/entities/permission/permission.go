package permission

import (
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-erp/sdk/mapper"
)

type Permission struct {
	ID       uint
	Name     string
	Resource Resource
	Action   Action
	Modifier Modifier
}

func (p *Permission) ToGraph() *model.Permission {
	return &model.Permission{
		ID:          int64(p.ID),
		Resource:    (*string)(&p.Resource),
		Action:      (*string)(&p.Action),
		Modifier:    (*string)(&p.Modifier),
		Description: mapper.Pointer(""),
	}
}
