// Package dtos provides this package.
package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/userposition"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/validators"
)

// CreateUserPositionDTO carries the create-position form. Title is a per-locale
// map populated from inputs named Title[<locale>]; at least one locale value is
// required. UserID and DepartmentID reference existing tenant entities.
type CreateUserPositionDTO struct {
	Title        map[string]string `validate:"required,min=1"`
	UserID       uint              `validate:"required"`
	DepartmentID string            `validate:"required,uuid"`
	IsManager    bool              `validate:"omitempty"`
	IsPrimary    bool              `validate:"omitempty"`
	Status       string            `validate:"required,oneof=active inactive"`
}

// UpdateUserPositionDTO carries the edit-position form. See
// CreateUserPositionDTO for field semantics.
//
// The assigned user is immutable after creation: the userposition aggregate has
// no SetUserID, so Apply intentionally never reassigns it. UserID is retained
// here only so the shared edit form (which submits the current user via a
// disabled select + hidden input) passes the same validation as create; any
// submitted value is ignored. To move a position to a different user, delete and
// recreate it.
type UpdateUserPositionDTO struct {
	Title        map[string]string `validate:"required,min=1"`
	UserID       uint              `validate:"required"`
	DepartmentID string            `validate:"required,uuid"`
	IsManager    bool              `validate:"omitempty"`
	IsPrimary    bool              `validate:"omitempty"`
	Status       string            `validate:"required,oneof=active inactive"`
}

func userPositionValidationErrors(ctx context.Context, dto interface{}, title map[string]string) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("Positions.Single.%s", validators.FieldLabel(dto, err)),
			})
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
				TemplateData: map[string]string{
					"Field": translatedFieldName,
				},
			})
		}
	}

	// The form always submits a Title[<locale>] map, so the struct-level
	// `required,min=1` tag only confirms the map exists. Reject blank or
	// incomplete locales here so the drawer shows a field error instead of a
	// later 500 from the service.
	if _, exists := errorMessages["Title"]; !exists && missingRequiredLocales(title) {
		errorMessages["Title"] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ValidationErrors.required",
			TemplateData: map[string]string{
				"Field": l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Positions.Single.Title"}),
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func (dto *CreateUserPositionDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return userPositionValidationErrors(ctx, dto, dto.Title)
}

func (dto *UpdateUserPositionDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return userPositionValidationErrors(ctx, dto, dto.Title)
}

func (dto *CreateUserPositionDTO) ToEntity() (userposition.UserPosition, error) {
	title, err := models.NewMultiLangFromMap(dto.Title)
	if err != nil {
		return nil, err
	}
	departmentID, err := uuid.Parse(dto.DepartmentID)
	if err != nil {
		return nil, err
	}
	return userposition.New(
		dto.UserID,
		departmentID,
		title,
		userposition.WithIsManager(dto.IsManager),
		userposition.WithIsPrimary(dto.IsPrimary),
		userposition.WithStatus(userposition.Status(dto.Status)),
	), nil
}

func (dto *UpdateUserPositionDTO) Apply(p userposition.UserPosition) (userposition.UserPosition, error) {
	if p.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}
	title, err := models.NewMultiLangFromMap(dto.Title)
	if err != nil {
		return nil, err
	}
	departmentID, err := uuid.Parse(dto.DepartmentID)
	if err != nil {
		return nil, err
	}

	p = p.SetTitle(title).
		SetDepartmentID(departmentID).
		SetIsManager(dto.IsManager).
		SetIsPrimary(dto.IsPrimary).
		SetStatus(userposition.Status(dto.Status))

	return p, nil
}
