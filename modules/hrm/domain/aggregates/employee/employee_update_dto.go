package employee

import (
	"context"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/money"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UpdateDTO struct {
	FirstName         string
	LastName          string
	MiddleName        string
	Email             string
	Phone             string
	Salary            float64
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
	if _, err := internet.NewEmail(d.Email); err != nil {
		errorMessages["Email"] = err.Error()
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *UpdateDTO) ToEntity(id uint) (Employee, error) {
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
	return NewWithID(
		id,
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
		time.Now(),
		time.Now(),
	), nil
}
