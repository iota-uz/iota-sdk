package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
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

func (dto *CreateUserDTO) Ok(ctx context.Context) (map[string]string, bool) {
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

func (dto *UpdateUserDTO) Ok(ctx context.Context) (map[string]string, bool) {
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

func (dto *CreateUserDTO) ToEntity() (user.User, error) {
	roles := make([]role.Role, len(dto.RoleIDs))
	for i, rID := range dto.RoleIDs {
		r := role.New("", role.WithID(rID))
		roles[i] = r
	}

	groupUUIDs := make([]uuid.UUID, len(dto.GroupIDs))
	for i, gID := range dto.GroupIDs {
		groupUUID, err := uuid.Parse(gID)
		if err != nil {
			return nil, err
		}
		groupUUIDs[i] = groupUUID
	}

	email, err := internet.NewEmail(dto.Email)
	if err != nil {
		return nil, err
	}

	options := []user.Option{
		user.WithMiddleName(dto.MiddleName),
		user.WithPassword(dto.Password),
		user.WithRoles(roles),
		user.WithGroupIDs(groupUUIDs),
		user.WithAvatarID(dto.AvatarID),
	}

	if dto.Phone != "" {
		p, err := phone.NewFromE164(dto.Phone)
		if err != nil {
			return nil, err
		}
		options = append(options, user.WithPhone(p))
	}

	u := user.New(
		dto.FirstName,
		dto.LastName,
		email,
		user.UILanguage(dto.Language),
		options...,
	)

	if dto.Password != "" {
		u, err = u.SetPassword(dto.Password)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}

func (dto *UpdateUserDTO) Apply(u user.User, roles []role.Role, permissions []*permission.Permission) (user.User, error) {
	if u.ID() == 0 {
		return nil, errors.New("id cannot be 0")
	}

	email, err := internet.NewEmail(dto.Email)
	if err != nil {
		return nil, err
	}

	groupUUIDs := make([]uuid.UUID, len(dto.GroupIDs))
	for i, gID := range dto.GroupIDs {
		groupUUID, err := uuid.Parse(gID)
		if err != nil {
			return nil, err
		}
		groupUUIDs[i] = groupUUID
	}

	u = u.SetName(dto.FirstName, dto.LastName, dto.MiddleName).
		SetEmail(email).
		SetUILanguage(user.UILanguage(dto.Language)).
		SetRoles(roles).
		SetGroupIDs(groupUUIDs).
		SetPermissions(permissions)

	if dto.Phone != "" {
		p, err := phone.NewFromE164(dto.Phone)
		if err != nil {
			return nil, err
		}
		u = u.SetPhone(p)
	}

	if dto.Password != "" {
		u, err = u.SetPassword(dto.Password)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}
