package eskiz

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrInvalidPhoneNumber = errors.New("invalid phone number")
	ErrInvalidMessage     = errors.New("invalid message")
	ErrTokenRefreshFailed = errors.New("token refresh failed")
	ErrNilResponse        = errors.New("response cannot be nil")
	ErrNilContext         = errors.New("context cannot be nil")
	ErrMessageTooLong     = errors.New("message exceeds maximum length")
	ErrServiceUnavailable = errors.New("service unavailable")
)

type APIError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("API error %d: %s - %s", e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

func NewAPIError(statusCode int, message, details string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Details:    details,
	}
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 ||
			apiErr.StatusCode == http.StatusTooManyRequests ||
			apiErr.StatusCode == http.StatusRequestTimeout
	}

	return errors.Is(err, ErrServiceUnavailable) ||
		errors.Is(err, ErrTokenRefreshFailed)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
