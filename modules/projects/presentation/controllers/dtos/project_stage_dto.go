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
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ProjectStageCreateDTO struct {
	ProjectID      string `validate:"required,uuid"`
	StageNumber    int    `validate:"min=1"`
	Desc           string `validate:"max=1000"`
	TotalAmount    int64  `validate:"required,min=1"`
	StartDate      *shared.DateOnly
	PlannedEndDate *shared.DateOnly
	FactualEndDate *shared.DateOnly
}

type ProjectStageUpdateDTO struct {
	StageNumber    int    `validate:"min=1"`
	Desc           string `validate:"max=1000"`
	TotalAmount    int64  `validate:"required,min=1"`
	StartDate      *shared.DateOnly
	PlannedEndDate *shared.DateOnly
	FactualEndDate *shared.DateOnly
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
	projectID, err := uuid.Parse(dto.ProjectID)
	if err != nil {
		panic(err)
	}

	var startDate, plannedEndDate, factualEndDate *time.Time

	if dto.StartDate != nil {
		t := time.Time(*dto.StartDate)
		startDate = &t
	}
	if dto.PlannedEndDate != nil {
		t := time.Time(*dto.PlannedEndDate)
		plannedEndDate = &t
	}
	if dto.FactualEndDate != nil {
		t := time.Time(*dto.FactualEndDate)
		factualEndDate = &t
	}

	return projectstage.New(
		projectID,
		dto.StageNumber,
		dto.TotalAmount,
		projectstage.WithDescription(dto.Desc),
		projectstage.WithStartDate(startDate),
		projectstage.WithPlannedEndDate(plannedEndDate),
		projectstage.WithFactualEndDate(factualEndDate),
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
	var startDate, plannedEndDate, factualEndDate *time.Time

	if dto.StartDate != nil {
		t := time.Time(*dto.StartDate)
		startDate = &t
	}
	if dto.PlannedEndDate != nil {
		t := time.Time(*dto.PlannedEndDate)
		plannedEndDate = &t
	}
	if dto.FactualEndDate != nil {
		t := time.Time(*dto.FactualEndDate)
		factualEndDate = &t
	}

	updated := existing.UpdateStageNumber(dto.StageNumber)
	updated = updated.UpdateDescription(dto.Desc)
	updated = updated.UpdateTotalAmount(dto.TotalAmount)
	updated = updated.UpdateStartDate(startDate)
	updated = updated.UpdatePlannedEndDate(plannedEndDate)
	updated = updated.UpdateFactualEndDate(factualEndDate)
	return updated
}
