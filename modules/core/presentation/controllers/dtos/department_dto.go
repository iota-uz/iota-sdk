// Package dtos provides this package.
package dtos

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/department"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/validators"
)

// CreateDepartmentDTO carries the create-department form. Name is a per-locale
// map populated from inputs named Name[<locale>]; at least one locale value is
// required. ParentID is optional (empty string means a root department).
type CreateDepartmentDTO struct {
	Name     map[string]string `validate:"required,min=1"`
	Code     string            `validate:"required"`
	ParentID string            `validate:"omitempty,uuid"`
	Order    int               `validate:"omitempty"`
	Status   string            `validate:"required,oneof=active inactive"`
}

// UpdateDepartmentDTO carries the edit-department form. See CreateDepartmentDTO
// for field semantics.
type UpdateDepartmentDTO struct {
	Name     map[string]string `validate:"required,min=1"`
	Code     string            `validate:"required"`
	ParentID string            `validate:"omitempty,uuid"`
	Order    int               `validate:"omitempty"`
	Status   string            `validate:"required,oneof=active inactive"`
}

// orgRequiredLocales mirrors services.orgRequiredLocales: every multilingual
// organizational name/title must carry a non-empty value for each of these
// locales, otherwise the service rejects the write as an internal error. The
// DTO layer validates the same set so the form can surface a field error.
var orgRequiredLocales = []string{"en", "ru", "uz", "uz-Cyrl"}

// missingRequiredLocales reports whether the per-locale map omits or blanks any
// required locale. Keys are matched case-insensitively because the form submits
// UI locale codes (e.g. "uz-Cyrl") while storage normalizes to lowercase.
func missingRequiredLocales(values map[string]string) bool {
	present := make(map[string]struct{}, len(values))
	for locale, value := range values {
		if strings.TrimSpace(value) != "" {
			present[strings.ToLower(locale)] = struct{}{}
		}
	}
	for _, locale := range orgRequiredLocales {
		if _, ok := present[strings.ToLower(locale)]; !ok {
			return true
		}
	}
	return false
}

func departmentValidationErrors(ctx context.Context, dto interface{}, name map[string]string) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("Departments.Single.%s", validators.FieldLabel(dto, err)),
			})
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
				TemplateData: map[string]string{
					"Field": translatedFieldName,
				},
			})
		}
	}

	// The form always submits a Name[<locale>] map, so the struct-level
	// `required,min=1` tag only confirms the map exists. Reject blank or
	// incomplete locales here so the drawer shows a field error instead of a
	// later 500 from the service.
	if _, exists := errorMessages["Name"]; !exists && missingRequiredLocales(name) {
		errorMessages["Name"] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: "ValidationErrors.required",
			TemplateData: map[string]string{
				"Field": l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Departments.Single.Name"}),
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

func (dto *CreateDepartmentDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return departmentValidationErrors(ctx, dto, dto.Name)
}

func (dto *UpdateDepartmentDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return departmentValidationErrors(ctx, dto, dto.Name)
}

// parseDepartmentOptions builds the shared aggregate options for the optional
// parent and order from a DTO. It returns an error only when ParentID is set
// but malformed (validation should have caught this already).
func parseDepartmentOptions(parentID string, order int) ([]department.Option, error) {
	opts := []department.Option{department.WithOrder(order)}
	if parentID != "" {
		pid, err := uuid.Parse(parentID)
		if err != nil {
			return nil, err
		}
		opts = append(opts, department.WithParentID(&pid))
	}
	return opts, nil
}

func (dto *CreateDepartmentDTO) ToEntity() (department.Department, error) {
	name, err := models.NewMultiLangFromMap(dto.Name)
	if err != nil {
		return nil, err
	}
	opts, err := parseDepartmentOptions(dto.ParentID, dto.Order)
	if err != nil {
		return nil, err
	}
	opts = append(opts, department.WithStatus(department.Status(dto.Status)))
	return department.New(dto.Code, name, opts...), nil
}

func (dto *UpdateDepartmentDTO) Apply(d department.Department) (department.Department, error) {
	if d.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}
	name, err := models.NewMultiLangFromMap(dto.Name)
	if err != nil {
		return nil, err
	}

	var parentID *uuid.UUID
	if dto.ParentID != "" {
		pid, err := uuid.Parse(dto.ParentID)
		if err != nil {
			return nil, err
		}
		parentID = &pid
	}

	d = d.SetCode(dto.Code).
		SetName(name).
		SetParentID(parentID).
		SetOrder(dto.Order).
		SetStatus(department.Status(dto.Status))

	return d, nil
}
