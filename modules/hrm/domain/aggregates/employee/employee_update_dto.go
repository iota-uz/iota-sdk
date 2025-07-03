package employee

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/money"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/shared"
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
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	// Create validation errors collection
	validationErrors := make(serrors.ValidationErrors)

	// Process standard validator errors
	errs := constants.Validate.Struct(d)
	if errs != nil {
		validatorErrs := errs.(validator.ValidationErrors)

		// Field name mapping function
		getFieldLocaleKey := func(field string) string {
			switch field {
			case "FirstName", "LastName", "MiddleName", "Email", "Phone", "BirthDate",
				"HireDate", "ResignationDate", "PrimaryLanguage", "SecondaryLanguage":
				return fmt.Sprintf("Employees.Public.%s.Label", field)
			case "Salary", "Tin", "Pin":
				return fmt.Sprintf("Employees.Private.%s.Label", field)
			default:
				return ""
			}
		}

		// Process validator errors to our custom format
		for field, err := range serrors.ProcessValidatorErrors(validatorErrs, getFieldLocaleKey) {
			validationErrors[field] = err
		}
	}

	// Custom Email validation
	if d.Email != "" {
		if _, err := internet.NewEmail(d.Email); err != nil {
			validationErrors["Email"] = serrors.NewInvalidEmailError(
				"Email",
				"Employees.Public.Email.Label",
			)
		}
	}

	// Custom TIN validation
	if d.Tin != "" {
		if _, err := parseTin(d.Tin); err != nil {
			validationErrors["Tin"] = serrors.NewInvalidTINError(
				"Tin",
				"Employees.Private.TIN.Label",
				err.Error(),
			)
		}
	}

	// Custom PIN validation
	if d.Pin != "" {
		if _, err := parsePin(d.Pin); err != nil {
			validationErrors["Pin"] = serrors.NewInvalidPINError(
				"Pin",
				"Employees.Private.Pin.Label",
				err.Error(),
			)
		}
	}

	// Localize all validation errors
	errorMessages := serrors.LocalizeValidationErrors(validationErrors, l)
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
	var opts []Option
	if d.AvatarID != 0 {
		opts = append(opts, WithAvatarID(d.AvatarID))
	}
	if d.Notes != "" {
		opts = append(opts, WithNotes(d.Notes))
	}
	if !time.Time(d.ResignationDate).IsZero() {
		resignationDate := time.Time(d.ResignationDate)
		opts = append(opts, WithResignationDate(&resignationDate))
	}
	if !time.Time(d.BirthDate).IsZero() {
		opts = append(opts, WithBirthDate(time.Time(d.BirthDate)))
	}
	opts = append(opts, WithUpdatedAt(time.Now()))

	return NewWithID(
		id,
		uuid.Nil,
		d.FirstName,
		d.LastName,
		d.MiddleName,
		d.Phone,
		mail,
		money.NewFromFloat(d.Salary, string(currency.UsdCode)),
		tin,
		pin,
		NewLanguage(d.PrimaryLanguage, d.SecondaryLanguage),
		time.Time(d.HireDate),
		opts...,
	), nil
}
