package group

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
)

type CreateDTO struct {
	Name        string
	Description string
	RoleIDs     []string
}

func (dto *CreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	errors := make(map[string]string)

	if dto.Name == "" {
		errors["Name"] = "Name is required"
	}

	return errors, len(errors) == 0
}

func (dto *CreateDTO) ToEntity() (Group, error) {
	return New(dto.Name,
		WithDescription(dto.Description),
		WithCreatedAt(time.Now()),
		WithUpdatedAt(time.Now()),
	), nil
}

type UpdateDTO struct {
	Name        string
	Description string
	RoleIDs     []string
}

func (d *UpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	errors := make(map[string]string)

	if d.Name == "" {
		errors["Name"] = "Name is required"
	}

	return errors, len(errors) == 0
}

func (d *UpdateDTO) Apply(g Group, roles []role.Role) (Group, error) {
	if g.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	g = g.SetName(d.Name).
		SetDescription(d.Description).
		SetRoles(roles)

	return g, nil
}
