package dtos

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type ProjectStageCreateDTO struct {
	ProjectID      uuid.UUID `validate:"required"`
	StageNumber    int       `validate:"min=1"`
	Description    string    `validate:"max=1000"`
	TotalAmount    int64     `validate:"required,min=1"`
	StartDate      *time.Time
	PlannedEndDate *time.Time
	FactualEndDate *time.Time
}

type ProjectStageUpdateDTO struct {
	StageNumber    int    `validate:"min=1"`
	Description    string `validate:"max=1000"`
	TotalAmount    int64  `validate:"required,min=1"`
	StartDate      *time.Time
	PlannedEndDate *time.Time
	FactualEndDate *time.Time
}

func (dto *ProjectStageCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("ProjectStages.Single.%s", err.Field()),
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

func (dto *ProjectStageCreateDTO) ToEntity() projectstage.ProjectStage {
	return projectstage.New(
		dto.ProjectID,
		dto.StageNumber,
		dto.TotalAmount,
		projectstage.WithDescription(dto.Description),
		projectstage.WithStartDate(dto.StartDate),
		projectstage.WithPlannedEndDate(dto.PlannedEndDate),
		projectstage.WithFactualEndDate(dto.FactualEndDate),
	)
}

func (dto *ProjectStageUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("ProjectStages.Single.%s", err.Field()),
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

func (dto *ProjectStageUpdateDTO) Apply(existing projectstage.ProjectStage) projectstage.ProjectStage {
	updated := existing.UpdateStageNumber(dto.StageNumber)
	updated = updated.UpdateDescription(dto.Description)
	updated = updated.UpdateTotalAmount(dto.TotalAmount)
	updated = updated.UpdateStartDate(dto.StartDate)
	updated = updated.UpdatePlannedEndDate(dto.PlannedEndDate)
	updated = updated.UpdateFactualEndDate(dto.FactualEndDate)
	return updated
}
