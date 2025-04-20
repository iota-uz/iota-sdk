package validators

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/validators"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type UserValidator struct {
	repo user.Repository
}

func NewUserValidator(repo user.Repository) *UserValidator {
	return &UserValidator{repo: repo}
}

func (v *UserValidator) ValidateCreate(ctx context.Context, u user.User) error {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	errors := map[string]string{}

	if u.Phone().Value() != "" {
		exists, err := v.repo.PhoneExists(ctx, u.Phone().Value())
		if err != nil {
			return fmt.Errorf("failed to check phone existence: %w", err)
		}
		if exists {
			errors["Phone"] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "Users.Errors.PhoneUnique",
			})
		}
	}

	if len(errors) > 0 {
		return validators.NewValidationError(errors)
	}

	return nil
}
