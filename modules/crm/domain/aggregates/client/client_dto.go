package client

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type CreateDTO struct {
	FirstName      string `validate:"required"`
	LastName       string `validate:"required"`
	MiddleName     string
	Phone          string
	Email          string `validate:"omitempty,email"`
	Address        string
	PassportSeries string
	PassportNumber string
	Pin            string
	CountryCode    string
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
			MessageID: fmt.Sprintf("Clients.Single.%s.Label", err.Field()),
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

func (d *CreateDTO) ToEntity() (Client, error) {
	// Create options slice
	opts := []Option{}

	// Add address if provided
	if d.Address != "" {
		opts = append(opts, WithAddress(d.Address))
	}

	if d.Phone != "" {
		phone, err := phone.NewFromE164(d.Phone)
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithPhone(phone))
	}

	// Add email if provided
	if d.Email != "" {
		email, err := internet.NewEmail(d.Email)
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithEmail(email))
	}

	// Add passport if both series and number provided
	if d.PassportSeries != "" && d.PassportNumber != "" {
		passport := passport.New(d.PassportSeries, d.PassportNumber)
		opts = append(opts, WithPassport(passport))
	}

	// Add PIN if provided with country code
	if d.Pin != "" && d.CountryCode != "" {
		c, err := country.New(d.CountryCode)
		if err != nil {
			return nil, err
		}
		pin, err := tax.NewPin(d.Pin, c)
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithPin(pin))
	}

	return New(
		d.FirstName,
		d.LastName,
		d.MiddleName,
		opts...,
	)
}

type UpdatePersonalDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string
	Phone      string
	Email      string `validate:"omitempty,email"`
	Address    string
}

func (d *UpdatePersonalDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Clients.Single.%s.Label", err.Field()),
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

func (d *UpdatePersonalDTO) Apply(entity Client) (Client, error) {
	p, err := phone.NewFromE164(d.Phone)
	if err != nil {
		return nil, err
	}

	updated := entity.SetName(d.FirstName, d.LastName, d.MiddleName).SetPhone(p)

	if d.Address != "" {
		updated = updated.SetAddress(d.Address)
	}

	if d.Email != "" {
		email, err := internet.NewEmail(d.Email)
		if err != nil {
			return nil, err
		}
		updated = updated.SetEmail(email)
	}

	return updated, nil
}

type UpdatePassportDTO struct {
	PassportSeries string
	PassportNumber string
}

func (d *UpdatePassportDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Clients.Single.%s.Label", err.Field()),
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

func (d *UpdatePassportDTO) Apply(entity Client) (Client, error) {
	updated := entity

	if d.PassportSeries != "" && d.PassportNumber != "" {
		if entity.Passport() != nil && entity.Passport().ID() != uuid.Nil {
			passport := passport.New(
				d.PassportSeries,
				d.PassportNumber,
				passport.WithID(entity.Passport().ID()),
			)
			updated = updated.SetPassport(passport)
		} else {
			passport := passport.New(d.PassportSeries, d.PassportNumber)
			updated = updated.SetPassport(passport)
		}
	}

	return updated, nil
}

type UpdateTaxDTO struct {
	Pin         string
	CountryCode string
}

func (d *UpdateTaxDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Clients.Single.%s.Label", err.Field()),
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

func (d *UpdateTaxDTO) Apply(entity Client) (Client, error) {
	updated := entity

	if d.Pin != "" && d.CountryCode != "" {
		c, err := country.New(d.CountryCode)
		if err != nil {
			return nil, err
		}
		pin, err := tax.NewPin(d.Pin, c)
		if err != nil {
			return nil, err
		}
		updated = updated.SetPIN(pin)
	}

	return updated, nil
}
