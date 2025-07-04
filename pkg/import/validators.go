package importpkg

import (
	"strconv"
	"strings"
	"time"
)

// RequiredValidator validates non-empty values
type RequiredValidator struct {
	errorFactory ErrorFactory
}

// NewRequiredValidator creates a new required validator
func NewRequiredValidator(errorFactory ErrorFactory) *RequiredValidator {
	return &RequiredValidator{errorFactory: errorFactory}
}

func (v *RequiredValidator) Validate(value string, col string, row uint) error {
	if strings.TrimSpace(value) == "" {
		return v.errorFactory.NewInvalidCellError(col, row)
	}
	return nil
}

// NumericValidator validates numeric values
type NumericValidator struct {
	errorFactory ErrorFactory
}

// NewNumericValidator creates a new numeric validator
func NewNumericValidator(errorFactory ErrorFactory) *NumericValidator {
	return &NumericValidator{errorFactory: errorFactory}
}

func (v *NumericValidator) Validate(value string, col string, row uint) error {
	if _, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err != nil {
		return v.errorFactory.NewInvalidCellError(col, row)
	}
	return nil
}

// DateValidator validates date values
type DateValidator struct {
	errorFactory ErrorFactory
	format       string
}

// NewDateValidator creates a new date validator
func NewDateValidator(errorFactory ErrorFactory, format string) *DateValidator {
	if format == "" {
		format = "2006-01-02"
	}
	return &DateValidator{
		errorFactory: errorFactory,
		format:       format,
	}
}

func (v *DateValidator) Validate(value string, col string, row uint) error {
	if _, err := time.Parse(v.format, strings.TrimSpace(value)); err != nil {
		return v.errorFactory.NewInvalidCellError(col, row)
	}
	return nil
}

// OneOfValidator validates against a list of allowed values
type OneOfValidator struct {
	errorFactory ErrorFactory
	allowed      []string
}

// NewOneOfValidator creates a new one-of validator
func NewOneOfValidator(errorFactory ErrorFactory, allowed []string) *OneOfValidator {
	return &OneOfValidator{
		errorFactory: errorFactory,
		allowed:      allowed,
	}
}

func (v *OneOfValidator) Validate(value string, col string, row uint) error {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	for _, allowedValue := range v.allowed {
		if trimmed == strings.ToLower(allowedValue) {
			return nil
		}
	}
	return v.errorFactory.NewInvalidCellError(col, row)
}

// CompositeValidator chains multiple validators
type CompositeValidator struct {
	validators []Validator
}

// NewCompositeValidator creates a new composite validator
func NewCompositeValidator(validators ...Validator) *CompositeValidator {
	return &CompositeValidator{validators: validators}
}

func (v *CompositeValidator) Validate(value string, col string, row uint) error {
	for _, validator := range v.validators {
		if err := validator.Validate(value, col, row); err != nil {
			return err
		}
	}
	return nil
}
