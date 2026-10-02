package validate

import (
	"errors"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
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

func Fields(err error, fieldID func(string) string) map[string]error {
	result := make(map[string]error)
	for _, field := range serrors.FieldsOf(From(err, fieldID)) {
		result[field.Field] = serrors.NewInvalid("").WithFields(field)
	}
	return result
}

func TIN(field, label, details string) error { return tax(field, label, details, "invalidTIN") }
func PIN(field, label, details string) error { return tax(field, label, details, "invalidPIN") }
func tax(field, label, details, reason string) error {
	return serrors.NewInvalid("").WithFields(serrors.FieldViolation{Field: field, Reason: serrors.Reason(reason), Message: serrors.Message{ID: "ValidationErrors." + reason, Args: map[string]serrors.Value{"Field": serrors.Reference(serrors.Message{ID: label}), "Details": serrors.Text(details)}}})
}
func Map(fields map[string]error, l *i18n.Localizer) map[string]string {
	result := make(map[string]string)
	for _, err := range fields {
		for field, message := range FieldMap(err, l) {
			result[field] = message
		}
	}
	return result
}
func Email(field, label string) error { return tax(field, label, "", "email") }
