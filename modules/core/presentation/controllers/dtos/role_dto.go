package dtos

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/validators"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
)

type CreateRoleDTO struct {
	Name        string            `validate:"required"`
	Description string            `validate:"required" label:"_Description"`
	Permissions map[string]string `validate:"omitempty,dive,required"`
}

type UpdateRoleDTO struct {
	Name        string            `validate:"required"`
	Description string            `validate:"required" label:"_Description"`
	Permissions map[string]string `validate:"omitempty,dive,required"`
}

func (dto *CreateRoleDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Roles.Single.%s.Label", validators.FieldLabel(dto, err)),
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

func (dto *UpdateRoleDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Roles.Single.%s.Label", validators.FieldLabel(dto, err)),
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

// getPermissionByID looks up a permission by ID from the schema
func getPermissionByID(schema *rbac.PermissionSchema, permID uuid.UUID) (*permission.Permission, error) {
	if schema == nil {
		return nil, fmt.Errorf("permission schema not available")
	}

	for _, set := range schema.Sets {
		for _, perm := range set.Permissions {
			if perm.ID == permID {
				return perm, nil
			}
		}
	}
	return nil, fmt.Errorf("permission with ID %s not found", permID)
}

func (dto *CreateRoleDTO) ToEntity(schema *rbac.PermissionSchema) (role.Role, error) {
	perms := make([]*permission.Permission, 0, len(dto.Permissions))
	for permID := range dto.Permissions {
		permUUID, err := uuid.Parse(permID)
		if err != nil {
			return nil, err
		}
		perm, err := getPermissionByID(schema, permUUID)
		if err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}

	options := []role.Option{
		role.WithDescription(dto.Description),
		role.WithPermissions(perms),
	}

	return role.New(dto.Name, options...), nil
}

func (dto *UpdateRoleDTO) Apply(roleEntity role.Role, schema *rbac.PermissionSchema) (role.Role, error) {
	perms := make([]*permission.Permission, 0, len(dto.Permissions))
	for permID := range dto.Permissions {
		permUUID, err := uuid.Parse(permID)
		if err != nil {
			return nil, err
		}
		perm, err := getPermissionByID(schema, permUUID)
		if err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return roleEntity.SetName(dto.Name).SetDescription(dto.Description).SetPermissions(perms), nil
}
