package dtos

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type OnboardingDTO struct {
	FirstName       string `form:"FirstName"`
	LastName        string `form:"LastName"`
	MiddleName      string `form:"MiddleName"`
	Phone           string `form:"Phone"`
	Language        string `form:"Language"`
	NewPassword     string `form:"NewPassword"`
	ConfirmPassword string `form:"ConfirmPassword"`
}

// Ok validates the form against the SDK password policy and the languages the
// application supports.
func (d *OnboardingDTO) Ok(ctx context.Context, supportedLanguages []string) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	t := func(id string) string {
		return l.MustLocalize(&i18n.LocalizeConfig{MessageID: id})
	}
	errs := map[string]string{}
	if strings.TrimSpace(d.FirstName) == "" {
		errs["FirstName"] = t("Onboarding.Errors.FirstNameRequired")
	}
	if strings.TrimSpace(d.LastName) == "" {
		errs["LastName"] = t("Onboarding.Errors.LastNameRequired")
	}
	if !user.UILanguage(d.Language).IsValid() || !slices.Contains(supportedLanguages, d.Language) {
		errs["Language"] = t("Onboarding.Errors.LanguageRequired")
	}
	if d.Phone != "" {
		if _, err := phone.NewFromE164(d.Phone); err != nil {
			errs["Phone"] = t("Onboarding.Errors.PhoneInvalid")
		}
	}
	if messageID := PasswordPolicyMessageID(user.ValidatePassword(d.NewPassword)); messageID != "" {
		errs["NewPassword"] = t(messageID)
	}
	if d.ConfirmPassword == "" || d.NewPassword != d.ConfirmPassword {
		errs["ConfirmPassword"] = t("Onboarding.Errors.ConfirmationMismatch")
	}
	return errs, len(errs) == 0
}

func (d *OnboardingDTO) ToInput(avatarID uint) (services.OnboardingInput, error) {
	var p phone.Phone
	if d.Phone != "" {
		parsed, err := phone.NewFromE164(d.Phone)
		if err != nil {
			return services.OnboardingInput{}, err
		}
		p = parsed
	}
	return services.OnboardingInput{
		FirstName:  strings.TrimSpace(d.FirstName),
		LastName:   strings.TrimSpace(d.LastName),
		MiddleName: strings.TrimSpace(d.MiddleName),
		Phone:      p,
		AvatarID:   avatarID,
		Language:   user.UILanguage(d.Language),
		Password:   d.NewPassword,
	}, nil
}

// PasswordPolicyMessageID maps a password policy violation to its message.
func PasswordPolicyMessageID(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, user.ErrPasswordTooShort):
		return "Onboarding.Errors.PasswordTooShort"
	case errors.Is(err, user.ErrPasswordTooLong):
		return "Onboarding.Errors.PasswordTooLong"
	case errors.Is(err, user.ErrPasswordReusesTemporary):
		return "Onboarding.Errors.PasswordReusesTemporary"
	}
	return "Onboarding.Errors.PasswordInvalid"
}
