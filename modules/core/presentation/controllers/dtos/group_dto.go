package dtos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
)

type CreateGroupDTO struct {
	Name        string   `validate:"required"`
	Description string   `validate:"omitempty"`
	RoleIDs     []string `validate:"omitempty,dive,required"`
}

func (dto *CreateGroupDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Groups.Single.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func (dto *CreateGroupDTO) ToEntity() (group.Group, error) {
	return group.New(
		group.TypeUser,
		dto.Name,
		group.WithDescription(dto.Description),
		group.WithCreatedAt(time.Now()),
		group.WithUpdatedAt(time.Now()),
	), nil
}

type UpdateGroupDTO struct {
	Name        string   `validate:"required"`
	Description string   `validate:"omitempty"`
	RoleIDs     []string `validate:"omitempty,dive,required"`
}

func (dto *UpdateGroupDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Groups.Single.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func (dto *UpdateGroupDTO) Apply(g group.Group, roles []role.Role) (group.Group, error) {
	if g.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	g = g.SetName(dto.Name).
		SetDescription(dto.Description).
		SetRoles(roles)

	return g, nil
}
