package employee

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/email"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tax"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

type CreateDTO struct {
	FirstName         string
	LastName          string
	MiddleName        string
	Email             string
	Phone             string
	Salary            float64
	Tin               string
	Pin               string
	PrimaryLanguage   string
	SecondaryLanguage string
	AvatarID          uint
	Notes             string
}

type UpdateDTO struct {
	FirstName         string
	LastName          string
	MiddleName        string
	Email             string
	Phone             string
	Salary            float64
	Tin               string
	Pin               string
	PrimaryLanguage   string
	SecondaryLanguage string
	AvatarID          uint
	Notes             string
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

func (d *CreateDTO) ToEntity() (Employee, error) {
	mail, err := email.New(d.Email)
	if err != nil {
		return nil, err
	}
	tin, err := tax.NewTin(d.Tin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	pin, err := tax.NewPin(d.Pin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	return New(
		d.FirstName,
		d.LastName,
		d.MiddleName,
		d.Phone,
		mail,
		d.Salary,
		tin,
		pin,
		NewLanguage(d.PrimaryLanguage, d.SecondaryLanguage),
		0,
		"",
	)
}

func (d *UpdateDTO) ToEntity(id uint) (Employee, error) {
	mail, err := email.New(d.Email)
	if err != nil {
		return nil, err
	}
	tin, err := tax.NewTin(d.Tin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	pin, err := tax.NewPin(d.Pin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}

	return NewWithID(
		id,
		d.FirstName,
		d.LastName,
		d.MiddleName,
		d.Phone,
		mail,
		d.Salary,
		tin,
		pin,
		NewLanguage(d.PrimaryLanguage, d.SecondaryLanguage),
		0,
		"",
		time.Now(),
		time.Now(),
	), nil
}
