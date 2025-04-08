package user

import (
	"context"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string
	Email      string `validate:"required,email"`
	Phone      string
	Password   string
	RoleIDs    []uint
	AvatarID   uint
	UILanguage string `validate:"required"`
}

type UpdateDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string
	Email      string `validate:"required,email"`
	Phone      string
	Password   string
	RoleIDs    []uint
	AvatarID   uint
	UILanguage string
}

func (u *CreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	// TODO: Use composables.UseLocalizer(ctx) instead of ctx.Value(constants.LocalizerKey)
	l, ok := ctx.Value(constants.LocalizerKey).(*i18n.Localizer)
	if !ok {
		panic("localizer not found in context")
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(u)
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

func (u *UpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	// TODO: Use composables.UseLocalizer(ctx) instead of ctx.Value(constants.LocalizerKey)
	l, ok := ctx.Value(constants.LocalizerKey).(*i18n.Localizer)
	if !ok {
		panic("localizer not found in context")
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(u)
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

func (u *CreateDTO) ToEntity() (User, error) {
	roles := make([]role.Role, len(u.RoleIDs))
	for i, id := range u.RoleIDs {
		r := role.New("", role.WithID(id))
		roles[i] = r
	}

	email, err := internet.NewEmail(u.Email)
	if err != nil {
		return nil, err
	}

	options := []Option{
		WithMiddleName(u.MiddleName),
		WithPassword(u.Password),
		WithRoles(roles),
		WithAvatarID(u.AvatarID),
	}

	if u.Phone != "" {
		p, err := phone.NewFromE164(u.Phone)
		if err != nil {
			return nil, err
		}
		options = append(options, WithPhone(p))
	}

	return New(
		u.FirstName,
		u.LastName,
		email,
		UILanguage(u.UILanguage),
		options...,
	), nil
}

func (u *UpdateDTO) ToEntity(id uint) (User, error) {
	roles := make([]role.Role, len(u.RoleIDs))
	for i, rID := range u.RoleIDs {
		r := role.New("", role.WithID(rID))
		roles[i] = r
	}

	email, err := internet.NewEmail(u.Email)
	if err != nil {
		return nil, err
	}

	options := []Option{
		WithID(id),
		WithMiddleName(u.MiddleName),
		WithPassword(u.Password),
		WithRoles(roles),
		WithAvatarID(u.AvatarID),
	}

	if u.Phone != "" {
		p, err := phone.NewFromE164(u.Phone)
		if err != nil {
			return nil, err
		}
		options = append(options, WithPhone(p))
	}

	return New(
		u.FirstName,
		u.LastName,
		email,
		UILanguage(u.UILanguage),
		options...,
	), nil
}
