package types

import (
	"errors"
	"fmt"
	"time"
)

// ErrorCode represents a structured error code.
type ErrorCode string

const (
	// ErrCodeNotFound indicates a requested resource was not found
	ErrCodeNotFound ErrorCode = "NOT_FOUND"

	// ErrCodeValidation indicates invalid input or validation failure
	ErrCodeValidation ErrorCode = "VALIDATION"

	// ErrCodeOverflow indicates a capacity limit was exceeded
	ErrCodeOverflow ErrorCode = "OVERFLOW"

	// ErrCodeRateLimit indicates a rate limit was exceeded
	ErrCodeRateLimit ErrorCode = "RATE_LIMIT"

	// ErrCodeProvider indicates an error from an external provider
	ErrCodeProvider ErrorCode = "PROVIDER"

	// ErrCodeTimeout indicates an operation timed out
	ErrCodeTimeout ErrorCode = "TIMEOUT"

	// ErrCodeCancelled indicates an operation was cancelled
	ErrCodeCancelled ErrorCode = "CANCELLED"

	// ErrCodePermission indicates insufficient permissions
	ErrCodePermission ErrorCode = "PERMISSION"

	// ErrCodeInternal indicates an internal system error
	ErrCodeInternal ErrorCode = "INTERNAL"
)

// Error is a rich error type with structured error codes and metadata.
type Error struct {
	// Code is the error code
	Code ErrorCode

	// Message is a human-readable error message
	Message string

	// Cause is the underlying error that caused this error
	Cause error

	// Details contains additional structured error information
	Details map[string]any

	// Retryable indicates whether the operation can be retried
	Retryable bool
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause error.
func (e *Error) Unwrap() error {
	return e.Cause
}

// Is returns true if the target error has the same error code.
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// NotFoundError creates a NOT_FOUND error for a missing resource.
func NotFoundError(resource, id string) *Error {
	return &Error{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
		Details: map[string]any{
			"resource": resource,
			"id":       id,
		},
		Retryable: false,
	}
}

// ValidationError creates a VALIDATION error for invalid input.
func ValidationError(field, reason string) *Error {
	return &Error{
		Code:    ErrCodeValidation,
		Message: fmt.Sprintf("validation failed for field '%s': %s", field, reason),
		Details: map[string]any{
			"field":  field,
			"reason": reason,
		},
		Retryable: false,
	}
}

// OverflowError creates an OVERFLOW error when capacity is exceeded.
func OverflowError(requested, available int) *Error {
	return &Error{
		Code:    ErrCodeOverflow,
		Message: fmt.Sprintf("capacity exceeded: requested %d, available %d", requested, available),
		Details: map[string]any{
			"requested": requested,
			"available": available,
		},
		Retryable: false,
	}
}

// ProviderError creates a PROVIDER error for external provider failures.
func ProviderError(provider string, statusCode int, message string, retryable bool) *Error {
	return &Error{
		Code:    ErrCodeProvider,
		Message: fmt.Sprintf("provider error from %s: %s", provider, message),
		Details: map[string]any{
			"provider":    provider,
			"status_code": statusCode,
		},
		Retryable: retryable,
	}
}

// RateLimitError creates a RATE_LIMIT error.
func RateLimitError(retryAfter time.Duration) *Error {
	return &Error{
		Code:    ErrCodeRateLimit,
		Message: fmt.Sprintf("rate limit exceeded, retry after %v", retryAfter),
		Details: map[string]any{
			"retry_after_seconds": retryAfter.Seconds(),
		},
		Retryable: true,
	}
}

// TimeoutError creates a TIMEOUT error for operations that exceed time limits.
func TimeoutError(operation string, duration time.Duration) *Error {
	return &Error{
		Code:    ErrCodeTimeout,
		Message: fmt.Sprintf("operation '%s' timed out after %v", operation, duration),
		Details: map[string]any{
			"operation":       operation,
			"timeout_seconds": duration.Seconds(),
		},
		Retryable: true,
	}
}

// CancelledError creates a CANCELLED error for cancelled operations.
func CancelledError(operation string) *Error {
	return &Error{
		Code:    ErrCodeCancelled,
		Message: fmt.Sprintf("operation '%s' was cancelled", operation),
		Details: map[string]any{
			"operation": operation,
		},
		Retryable: false,
	}
}

// PermissionError creates a PERMISSION error for authorization failures.
func PermissionError(action, resource string) *Error {
	return &Error{
		Code:    ErrCodePermission,
		Message: fmt.Sprintf("permission denied: cannot %s %s", action, resource),
		Details: map[string]any{
			"action":   action,
			"resource": resource,
		},
		Retryable: false,
	}
}

// InternalError creates an INTERNAL error for unexpected system failures.
func InternalError(message string, cause error) *Error {
	return &Error{
		Code:      ErrCodeInternal,
		Message:   message,
		Cause:     cause,
		Retryable: false,
	}
}
