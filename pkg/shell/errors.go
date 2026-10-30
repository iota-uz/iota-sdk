package shell

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeInvalidInput          ErrorCode = "invalid_input"
	CodeForbidden             ErrorCode = "forbidden"
	CodeLimitExceeded         ErrorCode = "limit_exceeded"
	CodeBusy                  ErrorCode = "busy"
	CodeDependencyUnavailable ErrorCode = "dependency_unavailable"
)

// ErrFileNotAccessible is returned by a FileResolver for a file_ref that does
// not exist or is not visible to the caller.
var ErrFileNotAccessible = errors.New("shell: file is not accessible")

// Error is a tool error raised before (or instead of) a shell result.
type Error struct {
	Code         ErrorCode `json:"code"`
	Field        string    `json:"field,omitempty"`
	Message      string    `json:"message"`
	RetryAfterMS int64     `json:"retry_after_ms,omitempty"`
	Err          error     `json:"-"`
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("shell: %s: %s: %s", e.Code, e.Field, e.Message)
	}
	return fmt.Sprintf("shell: %s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func newError(code ErrorCode, field, message string) *Error {
	return &Error{Code: code, Field: field, Message: message}
}
