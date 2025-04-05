package group

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type CreateDTO struct {
	Name        string   `form:"Name"`
	Description string   `form:"Description"`
	RoleIDs     []string `form:"RoleIDs"`
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
	Name        string   `form:"Name"`
	Description string   `form:"Description"`
	RoleIDs     []string `form:"RoleIDs"`
}

func (dto *UpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	errors := make(map[string]string)

	if dto.Name == "" {
		errors["Name"] = "Name is required"
	}

	return errors, len(errors) == 0
}

func (dto *UpdateDTO) ToEntity(id uuid.UUID) (Group, error) {
	if id == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	return New(dto.Name,
		WithID(id),
		WithDescription(dto.Description),
		WithUpdatedAt(time.Now()),
	), nil
}

// Using FindParams from group_repository.go