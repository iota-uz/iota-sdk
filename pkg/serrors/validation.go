package serrors

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
)

// ValidationError represents a field validation error
type ValidationError struct {
	BaseError
	Field string `json:"field"`
}

// ValidationErrors is a map of field names to validation errors
type ValidationErrors map[string]*ValidationError

// NewValidationError creates a new validation error
func NewValidationError(field string, code string, message string, localeKey string) *ValidationError {
	return &ValidationError{
		BaseError: BaseError{
			Code:      code,
			Message:   message,
			LocaleKey: localeKey,
		},
		Field: field,
	}
}

// NewFieldRequiredError creates a new required field error
func NewFieldRequiredError(field string, fieldLocaleKey string) *ValidationError {
	return NewValidationError(
		field,
		"VALIDATION_REQUIRED",
		fmt.Sprintf("Field %s is required", field),
		"ValidationErrors.required",
	).WithFieldName(fieldLocaleKey)
}

// NewInvalidEmailError creates a new invalid email error
func NewInvalidEmailError(field string, fieldLocaleKey string) *ValidationError {
	return NewValidationError(
		field,
		"VALIDATION_EMAIL",
		fmt.Sprintf("Field %s must be a valid email", field),
		"ValidationErrors.email",
	).WithFieldName(fieldLocaleKey)
}

// NewInvalidTINError creates a new invalid TIN error
func NewInvalidTINError(field string, fieldLocaleKey string, details string) *ValidationError {
	// Use the clean error message directly if it's already user-friendly
	message := details
	if message == "" {
		message = "Invalid TIN format"
	}

	return NewValidationError(
		field,
		"VALIDATION_TIN",
		message,
		"ValidationErrors.invalidTIN",
	).WithFieldName(fieldLocaleKey).WithDetails(details)
}

// NewInvalidPINError creates a new invalid PIN error
func NewInvalidPINError(field string, fieldLocaleKey string, details string) *ValidationError {
	return NewValidationError(
		field,
		"VALIDATION_PIN",
		fmt.Sprintf("Invalid PIN format: %s", details),
		"ValidationErrors.invalidPIN",
	).WithFieldName(fieldLocaleKey).WithDetails(details)
}

// WithFieldName adds the field name to the template data
func (e *ValidationError) WithFieldName(fieldLocaleKey string) *ValidationError {
	if e.TemplateData == nil {
		e.TemplateData = make(map[string]string)
	}
	e.TemplateData["Field"] = fieldLocaleKey
	return e
}

// WithDetails adds error details to the template data
func (e *ValidationError) WithDetails(details string) *ValidationError {
	if e.TemplateData == nil {
		e.TemplateData = make(map[string]string)
	}
	e.TemplateData["Details"] = details
	return e
}

// LocalizeValidationErrors localizes all validation errors in the map
func LocalizeValidationErrors(errs ValidationErrors, l *i18n.Localizer) map[string]string {
	result := make(map[string]string)

	for field, err := range errs {
		if fieldKey, ok := err.TemplateData["Field"]; ok {
			// If the field key is already a message ID, localize it
			if fieldKey != "" {
				localizedField := l.MustLocalize(&i18n.LocalizeConfig{
					MessageID: fieldKey,
				})
				err.TemplateData["Field"] = localizedField
			}
		}

		result[field] = err.Localize(l)
	}

	return result
}

// ProcessValidatorErrors converts validator.ValidationErrors to our ValidationErrors
func ProcessValidatorErrors(errs validator.ValidationErrors, fieldIDMapping func(string) string) ValidationErrors {
	result := make(ValidationErrors)

	for _, err := range errs {
		field := err.Field()
		fieldID := fieldIDMapping(field)

		switch err.Tag() {
		case "required":
			result[field] = NewFieldRequiredError(field, fieldID)
		case "email":
			result[field] = NewInvalidEmailError(field, fieldID)
		default:
			// Generic validation error
			result[field] = NewValidationError(
				field,
				fmt.Sprintf("VALIDATION_%s", err.Tag()),
				err.Error(),
				fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			).WithFieldName(fieldID)
		}
	}

	return result
}
