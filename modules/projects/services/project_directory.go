package services

import (
	"context"

	financeservices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// ProjectDirectory offers projects to finance so debts can be linked to them.
type ProjectDirectory struct {
	projectService *ProjectService
}

func NewProjectDirectory(projectService *ProjectService) financeservices.ProjectDirectory {
	return &ProjectDirectory{projectService: projectService}
}

// Projects lists the tenant's projects, or none when the user cannot read them.
func (d *ProjectDirectory) Projects(ctx context.Context) ([]financeservices.ProjectRef, error) {
	if canRead := composables.CanUser(ctx, permissions.ProjectRead) == nil; !canRead {
		return nil, nil
	}
	projects, err := d.projectService.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]financeservices.ProjectRef, 0, len(projects))
	for _, p := range projects {
		refs = append(refs, financeservices.ProjectRef{ID: p.ID(), Name: p.Name()})
	}
	return refs, nil
}
