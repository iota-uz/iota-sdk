package dtos

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type CreateUserDTO struct {
	FirstName  string   `validate:"required"`
	LastName   string   `validate:"required"`
	MiddleName string   `validate:"omitempty"`
	Email      string   `validate:"required,email"`
	Phone      string   `validate:"omitempty"`
	Password   string   `validate:"omitempty"`
	RoleIDs    []uint   `validate:"omitempty,dive,required"`
	GroupIDs   []string `validate:"omitempty,dive,required"`
	AvatarID   uint     `validate:"omitempty,gt=0"`
	Language   string   `validate:"required"`
}

type UpdateUserDTO struct {
	FirstName  string   `validate:"required"`
	LastName   string   `validate:"required"`
	MiddleName string   `validate:"omitempty"`
	Email      string   `validate:"required,email"`
	Phone      string   `validate:"omitempty"`
	Password   string   `validate:"omitempty"`
	RoleIDs    []uint   `validate:"omitempty,dive,required"`
	GroupIDs   []string `validate:"omitempty,dive,required"`
	AvatarID   uint     `validate:"omitempty,gt=0"`
	Language   string   `validate:"required"`
}

func (u *CreateUserDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (u *UpdateUserDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (u *CreateUserDTO) ToEntity() (user.User, error) {
	roles := make([]role.Role, len(u.RoleIDs))
	for i, rID := range u.RoleIDs {
		r := role.New("", role.WithID(rID))
		roles[i] = r
	}

	groupUUIDs := make([]uuid.UUID, len(u.GroupIDs))
	for i, gID := range u.GroupIDs {
		groupUUID, err := uuid.Parse(gID)
		if err != nil {
			return nil, err
		}
		groupUUIDs[i] = groupUUID
	}

	email, err := internet.NewEmail(u.Email)
	if err != nil {
		return nil, err
	}

	options := []user.Option{
		user.WithMiddleName(u.MiddleName),
		user.WithPassword(u.Password),
		user.WithRoles(roles),
		user.WithGroupIDs(groupUUIDs),
		user.WithAvatarID(u.AvatarID),
	}

	if u.Phone != "" {
		p, err := phone.NewFromE164(u.Phone)
		if err != nil {
			return nil, err
		}
		options = append(options, user.WithPhone(p))
	}

	return user.New(
		u.FirstName,
		u.LastName,
		email,
		user.UILanguage(u.Language),
		options...,
	), nil
}

func (u *UpdateUserDTO) ToEntity(id uint) (user.User, error) {
	roles := make([]role.Role, len(u.RoleIDs))
	for i, rID := range u.RoleIDs {
		r := role.New("", role.WithID(rID))
		roles[i] = r
	}

	groupUUIDs := make([]uuid.UUID, len(u.GroupIDs))
	for i, gID := range u.GroupIDs {
		groupUUID, err := uuid.Parse(gID)
		if err != nil {
			return nil, err
		}
		groupUUIDs[i] = groupUUID
	}

	email, err := internet.NewEmail(u.Email)
	if err != nil {
		return nil, err
	}

	options := []user.Option{
		user.WithID(id),
		user.WithMiddleName(u.MiddleName),
		user.WithPassword(u.Password),
		user.WithRoles(roles),
		user.WithGroupIDs(groupUUIDs),
		user.WithAvatarID(u.AvatarID),
	}

	if u.Phone != "" {
		p, err := phone.NewFromE164(u.Phone)
		if err != nil {
			return nil, err
		}
		options = append(options, user.WithPhone(p))
	}

	return user.New(
		u.FirstName,
		u.LastName,
		email,
		user.UILanguage(u.Language),
		options...,
	), nil
}
