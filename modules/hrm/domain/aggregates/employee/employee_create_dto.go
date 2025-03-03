package employee

import (
	"context"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/money"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"time"
)

func parseTin(v string) (tax.Tin, error) {
	if v == "" {
		return tax.NilTin, nil
	}
	return tax.NewTin(v, country.Uzbekistan)
}

func parsePin(v string) (tax.Pin, error) {
	if v == "" {
		return tax.NilPin, nil
	}
	return tax.NewPin(v, country.Uzbekistan)
}

type CreateDTO struct {
	FirstName         string `validate:"required"`
	LastName          string `validate:"required"`
	MiddleName        string
	Email             string  `validate:"required,email"`
	Phone             string  `validate:"required"`
	Salary            float64 `validate:"required"`
	Tin               string
	Pin               string
	BirthDate         shared.DateOnly
	HireDate          shared.DateOnly
	ResignationDate   shared.DateOnly
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
		var translatedFieldName string
		switch err.Field() {
		case "FirstName":
		case "LastName":
		case "MiddleName":
		case "Email":
		case "Phone":
		case "BirthDate":
		case "HireDate":
		case "ResignationDate":
		case "PrimaryLanguage":
		case "SecondaryLanguage":
			translatedFieldName = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("Employees.Public.%s.Label", err.Field()),
			})
		case "Salary":
		case "Tin":
		case "Pin":
			translatedFieldName = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: fmt.Sprintf("Employees.Private.%s.Label", err.Field()),
			})
		}
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
	mail, err := internet.NewEmail(d.Email)
	if err != nil {
		return nil, err
	}
	tin, err := parseTin(d.Tin)
	if err != nil {
		return nil, err
	}
	pin, err := parsePin(d.Pin)
	if err != nil {
		return nil, err
	}
	return New(
		d.FirstName,
		d.LastName,
		d.MiddleName,
		d.Phone,
		mail,
		money.New(d.Salary, currency.UsdCode),
		tin,
		pin,
		NewLanguage(d.PrimaryLanguage, d.SecondaryLanguage),
		time.Time(d.HireDate),
		(*time.Time)(&d.ResignationDate),
		0,
		"",
	)
}
