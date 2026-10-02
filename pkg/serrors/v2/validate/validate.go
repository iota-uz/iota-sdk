// Package validate adapts validator.v10 errors into safe field violations.
package validate

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
)

func From(err error, fieldID func(string) string) error {
	if err == nil {
		return nil
	}
	result := serrors.NewInvalid("").WithCause(err)
	var validations validator.ValidationErrors
	if !errors.As(err, &validations) {
		return result
	}
	fields := make([]serrors.FieldViolation, 0, len(validations))
	for _, v := range validations {
		field := v.Field()
		if field == "" {
			continue
		}
		message := serrors.Message{ID: "ValidationErrors." + v.Tag()}
		if fieldID != nil {
			message.Args = map[string]serrors.Value{"Field": serrors.Reference(serrors.Message{ID: fieldID(field)})}
		}
		fields = append(fields, serrors.FieldViolation{Field: field, Reason: serrors.Reason(v.Tag()), Message: message})
	}
	return result.WithFields(fields...)
}

func FieldMap(err error, l *i18n.Localizer) map[string]string { return serrors.FieldMap(err, l) }
