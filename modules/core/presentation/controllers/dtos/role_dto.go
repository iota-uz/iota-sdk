package dtos

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type CreateRoleDTO struct {
	Name        string `validate:"required"`
	Description string
	Permissions map[string]string
}

func (r *CreateRoleDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(r)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Roles.Single.%s.Label", err.Field()),
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

func (r *CreateRoleDTO) ToEntity(rbac permission.RBAC) (role.Role, error) {
	perms := make([]*permission.Permission, 0, len(r.Permissions))
	for permID := range r.Permissions {
		permUUID, err := uuid.Parse(permID)
		if err != nil {
			return nil, err
		}
		perm, err := rbac.Get(permUUID)
		if err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return role.New(
		r.Name, r.Description, perms,
	)
}

type UpdateRoleDTO struct {
	Name        string `validate:"required"`
	Description string
	Permissions map[string]string
}

func (r *UpdateRoleDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(r)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Roles.Single.%s.Label", err.Field()),
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

func (r *UpdateRoleDTO) ToEntity(roleEntity role.Role, rbac permission.RBAC) (role.Role, error) {
	perms := make([]*permission.Permission, 0, len(r.Permissions))
	for permID := range r.Permissions {
		permUUID, err := uuid.Parse(permID)
		if err != nil {
			return nil, err
		}
		perm, err := rbac.Get(permUUID)
		if err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return roleEntity.SetName(r.Name).SetDescription(r.Description).SetPermissions(perms), nil
}
