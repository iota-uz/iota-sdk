package exitcode

import "fmt"

// Error is an error that carries an explicit process exit code.
// If Silent is true, callers may choose not to print the error.
type Error struct {
	Code   int
	Err    error
	Silent bool
	Usage  bool
}

func New(code int, err error) *Error {
	return &Error{Code: code, Err: err}
}

func InvalidUsage(err error) *Error {
	return &Error{Code: InvalidUsageCode, Err: err, Usage: true}
}

func SilentCode(code int) *Error {
	return &Error{Code: code, Silent: true}
}

func (e *Error) ExitCode() int { return e.Code }

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}
