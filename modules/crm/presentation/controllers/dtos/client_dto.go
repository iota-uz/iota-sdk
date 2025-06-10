package dtos

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
)

type CreateClientDTO struct {
	FirstName      string `validate:"required"`
	LastName       string `validate:"required"`
	MiddleName     string `validate:"omitempty"`
	Phone          string `validate:"required"`
	Email          string `validate:"omitempty,email"`
	Address        string `validate:"omitempty"`
	PassportSeries string `validate:"omitempty"`
	PassportNumber string `validate:"omitempty"`
	Pin            string `validate:"omitempty"`
	CountryCode    string `validate:"omitempty"`
}

func (d *CreateClientDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *CreateClientDTO) ToEntity(tenantID uuid.UUID) (client.Client, error) {
	// Create options slice
	var opts []client.Option

	// Add tenant ID
	opts = append(opts, client.WithTenantID(tenantID))

	// Add address if provided
	if d.Address != "" {
		opts = append(opts, client.WithAddress(d.Address))
	}

	if d.Phone != "" {
		phone, err := phone.NewFromE164(d.Phone)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithPhone(phone))
	}

	// Add email if provided
	if d.Email != "" {
		email, err := internet.NewEmail(d.Email)
		if err != nil {
			return nil, err
		}
		opts = append(opts, client.WithEmail(email))
	}

	// Add passport if both series and number provided
	if d.PassportSeries != "" && d.PassportNumber != "" {
		passport := passport.New(d.PassportSeries, d.PassportNumber)
		opts = append(opts, client.WithPassport(passport))
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
		opts = append(opts, client.WithPin(pin))
	}

	if d.LastName != "" {
		opts = append(opts, client.WithLastName(d.LastName))
	}

	if d.MiddleName != "" {
		opts = append(opts, client.WithMiddleName(d.MiddleName))
	}

	return client.New(d.FirstName, opts...)
}

type UpdateClientPersonalDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string `validate:"omitempty"`
	Phone      string `validate:"omitempty"`
	Email      string `validate:"omitempty,email"`
	Address    string `validate:"omitempty"`
}

func (d *UpdateClientPersonalDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *UpdateClientPersonalDTO) Apply(entity client.Client) (client.Client, error) {
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

type UpdateClientPassportDTO struct {
	PassportSeries string `validate:"omitempty"`
	PassportNumber string `validate:"omitempty"`
}

func (d *UpdateClientPassportDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *UpdateClientPassportDTO) Apply(entity client.Client) (client.Client, error) {
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

type UpdateClientTaxDTO struct {
	Pin         string `validate:"omitempty"`
	CountryCode string `validate:"omitempty"`
}

func (d *UpdateClientTaxDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *UpdateClientTaxDTO) Apply(entity client.Client) (client.Client, error) {
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

type UpdateClientNotesDTO struct {
	Comments string `validate:"omitempty"`
}

func (d *UpdateClientNotesDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *UpdateClientNotesDTO) Apply(entity client.Client) (client.Client, error) {
	updated := entity

	if entity.Comments() != d.Comments {
		updated = entity.SetComments(d.Comments)
	}

	return updated, nil
}
