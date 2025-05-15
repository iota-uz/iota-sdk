package crud

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned when an entity is not found
var ErrNotFound = errors.New("entity not found")

// ErrValidation is returned for validation errors
var ErrValidation = errors.New("validation error")

// FieldError represents a single field validation error
type FieldError struct {
	Field   string
	Message string
}

// ValidationError collects field-level validation errors
type ValidationError struct {
	Errors []FieldError
}

func (ve ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}

	var sb strings.Builder
	sb.WriteString("validation errors: ")
	for i, err := range ve.Errors {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return sb.String()
}
