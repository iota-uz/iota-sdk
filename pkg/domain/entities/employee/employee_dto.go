package employee

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

type CreateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	Coefficient float64
}

type UpdateDTO struct {
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	Coefficient float64
}

func (d *CreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Users.Single.%s", err.Field()),
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

func (d *UpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Users.Single.%s", err.Field()),
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

func (d *CreateDTO) ToEntity() *Employee {
	return &Employee{
		ID:          0,
		FirstName:   d.FirstName,
		LastName:    d.LastName,
		MiddleName:  d.MiddleName,
		Email:       d.Email,
		Phone:       d.Phone,
		Salary:      d.Salary,
		Coefficient: d.Coefficient,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (d *UpdateDTO) ToEntity(id uint) *Employee {
	return &Employee{
		ID:          id,
		FirstName:   d.FirstName,
		LastName:    d.LastName,
		MiddleName:  d.MiddleName,
		Email:       d.Email,
		Phone:       d.Phone,
		Salary:      d.Salary,
		Coefficient: d.Coefficient,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
