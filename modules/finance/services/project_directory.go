package services

import (
	"context"

	"github.com/google/uuid"
)

// ProjectRef names a project a debt can be linked to.
type ProjectRef struct {
	ID   uuid.UUID
	Name string
}

// ProjectDirectory lists the projects debts can be linked to. Finance has no
// projects of its own: the projects module provides the directory, and
// without it debts are simply not linked to projects.
type ProjectDirectory interface {
	Projects(ctx context.Context) ([]ProjectRef, error)
}

type noProjects struct{}

// NewNoProjects is the directory used when no module provides projects.
func NewNoProjects() ProjectDirectory {
	return noProjects{}
}

func (noProjects) Projects(context.Context) ([]ProjectRef, error) {
	return nil, nil
}
