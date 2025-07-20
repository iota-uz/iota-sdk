package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type ProjectCreateDTO struct {
	CounterpartyID uuid.UUID `validate:"required"`
	Name           string    `validate:"required,min=2,max=255"`
	Description    string    `validate:"max=1000"`
}

type ProjectUpdateDTO struct {
	CounterpartyID uuid.UUID `validate:"required"`
	Name           string    `validate:"required,min=2,max=255"`
	Description    string    `validate:"max=1000"`
}

func (dto *ProjectCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Projects.Single.%s", err.Field()),
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

func (dto *ProjectCreateDTO) ToEntity(tenantID uuid.UUID) (project.Project, error) {
	entity := project.New(
		dto.Name,
		dto.CounterpartyID,
		project.WithTenantID(tenantID),
		project.WithDescription(dto.Description),
	)
	return entity, nil
}

func (dto *ProjectUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Projects.Single.%s", err.Field()),
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

func (dto *ProjectUpdateDTO) Apply(existing project.Project) (project.Project, error) {
	updated := existing.UpdateCounterpartyID(dto.CounterpartyID)
	updated = updated.UpdateName(dto.Name)
	updated = updated.UpdateDescription(dto.Description)
	return updated, nil
}
