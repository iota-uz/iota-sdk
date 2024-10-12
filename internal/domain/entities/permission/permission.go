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

func (p *Permission) Equals(p2 Permission) bool {
	if p.Modifier == ModifierAll {
		return p.Resource == p2.Resource && p.Action == p2.Action
	}
	return p.Resource == p2.Resource && p.Action == p2.Action && p.Modifier == p2.Modifier
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
