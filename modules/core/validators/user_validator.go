package validators

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/validators"
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

	errorMessages := map[string]string{}

	if phone := u.Phone(); phone != nil && phone.Value() != "" {
		if exists, err := v.repo.PhoneExists(ctx, phone.Value()); err != nil {
			return fmt.Errorf("failed to check phone existence: %w", err)
		} else if exists {
			errorMessages["Phone"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Users.Errors.PhoneUnique"})
		}
	}

	if email := u.Email(); email != nil && email.Value() != "" {
		if exists, err := v.repo.EmailExists(ctx, email.Value()); err != nil {
			return fmt.Errorf("failed to check email existence: %w", err)
		} else if exists {
			errorMessages["Email"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Users.Errors.EmailUnique"})
		}
	}

	if len(errorMessages) > 0 {
		return validators.NewValidationError(errorMessages)
	}

	return nil
}

func (v *UserValidator) ValidateUpdate(ctx context.Context, u user.User) error {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	errorMessages := map[string]string{}

	if phone := u.Phone(); phone != nil && phone.Value() != "" {
		found, err := v.repo.GetByPhone(ctx, phone.Value())
		if err != nil && !errors.Is(err, persistence.ErrUserNotFound) {
			return fmt.Errorf("failed to get user by phone: %w", err)
		}
		if err == nil && found.ID() != u.ID() {
			errorMessages["Phone"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Users.Errors.PhoneUnique"})
		}
	}

	if email := u.Email(); email != nil && email.Value() != "" {
		found, err := v.repo.GetByEmail(ctx, email.Value())
		if err != nil && !errors.Is(err, persistence.ErrUserNotFound) {
			return fmt.Errorf("failed to get user by email: %w", err)
		}
		if err == nil && found.ID() != u.ID() {
			errorMessages["Email"] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Users.Errors.EmailUnique"})
		}
	}

	if len(errorMessages) > 0 {
		return validators.NewValidationError(errorMessages)
	}

	return nil
}
