package validation

import "fmt"

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Code    string
}

func (e ValidationError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Field, e.Message, e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrorCode represents predefined error codes
type ValidationErrorCode string

const (
	ErrCodeRequired      ValidationErrorCode = "REQUIRED"
	ErrCodeInvalid       ValidationErrorCode = "INVALID"
	ErrCodeDuplicate     ValidationErrorCode = "DUPLICATE"
	ErrCodeOutOfBounds   ValidationErrorCode = "OUT_OF_BOUNDS"
	ErrCodeOverlap       ValidationErrorCode = "OVERLAP"
	ErrCodeInvalidFormat ValidationErrorCode = "INVALID_FORMAT"
	ErrCodeInvalidRange  ValidationErrorCode = "INVALID_RANGE"
)

// NewValidationError creates a new validation error with a code
func NewValidationError(field, message string, code ValidationErrorCode) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
		Code:    string(code),
	}
}

// ValidationErrors represents a collection of validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}

	if len(e) == 1 {
		return e[0].Error()
	}

	return fmt.Sprintf("validation failed with %d errors: %s", len(e), e[0].Error())
}

// HasErrors returns true if there are any validation errors
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// GetErrorsByField returns all errors for a specific field
func (e ValidationErrors) GetErrorsByField(field string) []ValidationError {
	var fieldErrors []ValidationError
	for _, err := range e {
		if err.Field == field {
			fieldErrors = append(fieldErrors, err)
		}
	}
	return fieldErrors
}

// GetErrorsByCode returns all errors with a specific code
func (e ValidationErrors) GetErrorsByCode(code ValidationErrorCode) []ValidationError {
	var codeErrors []ValidationError
	for _, err := range e {
		if err.Code == string(code) {
			codeErrors = append(codeErrors, err)
		}
	}
	return codeErrors
}

// First returns the first validation error, or nil if none exist
func (e ValidationErrors) First() *ValidationError {
	if len(e) == 0 {
		return nil
	}
	return &e[0]
}

// GridLayoutError represents a grid layout specific error
type GridLayoutError struct {
	Message string
	Panels  []string
	Code    string
}

func (e GridLayoutError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("grid layout error: %s (panels: %v, code: %s)", e.Message, e.Panels, e.Code)
	}
	return fmt.Sprintf("grid layout error: %s (panels: %v)", e.Message, e.Panels)
}

// NewGridLayoutError creates a new grid layout error
func NewGridLayoutError(message string, panels []string, code ValidationErrorCode) GridLayoutError {
	return GridLayoutError{
		Message: message,
		Panels:  panels,
		Code:    string(code),
	}
}

// PanelValidationError represents a panel-specific validation error
type PanelValidationError struct {
	PanelID string
	Field   string
	Message string
	Code    string
}

func (e PanelValidationError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("panel %s: %s: %s (%s)", e.PanelID, e.Field, e.Message, e.Code)
	}
	return fmt.Sprintf("panel %s: %s: %s", e.PanelID, e.Field, e.Message)
}

// NewPanelValidationError creates a new panel validation error
func NewPanelValidationError(panelID, field, message string, code ValidationErrorCode) PanelValidationError {
	return PanelValidationError{
		PanelID: panelID,
		Field:   field,
		Message: message,
		Code:    string(code),
	}
}
