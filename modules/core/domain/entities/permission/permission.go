package permission

import (
	"github.com/google/uuid"
)

type Permission struct {
	ID       uuid.UUID
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
