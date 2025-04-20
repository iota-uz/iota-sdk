package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveAccountDTO struct {
	FirstName  string `validate:"required"`
	LastName   string `validate:"required"`
	MiddleName string
	UILanguage string `validate:"required"`
	AvatarID   uint
}

func (d *SaveAccountDTO) Ok(ctx context.Context) (map[string]string, bool) {
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

func (d *SaveAccountDTO) Apply(u user.User) (user.User, error) {
	lang, err := user.NewUILanguage(d.UILanguage)
	if err != nil {
		return nil, err
	}
	updated := u.
		SetName(d.FirstName, d.LastName, d.MiddleName).
		SetAvatarID(d.AvatarID).
		SetUILanguage(lang).
		// set to empty without hashing because an account cannot change its password and empty
		// password is ignored by UserService.Update
		SetPasswordUnsafe("")
	return updated, nil
}
