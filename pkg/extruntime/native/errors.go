package native

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
)

// Issue is one validation issue raised by a Go handler.
type Issue struct {
	Code string
	// Path locates the offending input value (["phone"], ["items", "0"]).
	Path   []string
	Params any
}

// Validation builds a validation error (kind "validation", phase "input")
// from issues, the same wire error package code raises with
// `throw validation(...)`.
func Validation(issues ...Issue) error {
	we := &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.validation", Message: "invalid input"}
	for _, is := range issues {
		wi := pool.Issue{Code: is.Code, Phase: "input"}
		if len(is.Path) > 0 {
			wi.Path, _ = json.Marshal(is.Path)
		}
		if is.Params != nil {
			wi.Params, _ = json.Marshal(is.Params)
		}
		we.Issues = append(we.Issues, wi)
	}
	if len(issues) == 1 {
		we.Code = issues[0].Code
	}
	return we
}

// Domain builds a declared domain error of the contract (Spec.Errors).
func Domain(code string, params any) error {
	we := &pool.WireError{Kind: pool.ErrKindDomain, Code: code}
	if params != nil {
		we.Params, _ = json.Marshal(params)
	}
	return we
}

// Denied builds a denied error.
func Denied(code, message string) error {
	return &pool.WireError{Kind: pool.ErrKindDenied, Code: code, Message: message}
}

// AsError returns the structured runtime error carried by err (raised by a
// handler, a guard, the host or the data layer), if any.
func AsError(err error) (*pool.WireError, bool) {
	return pool.AsWireError(err)
}

// IsKind reports whether err carries a structured error of kind.
func IsKind(err error, kind pool.ErrorKind) bool {
	we, ok := AsError(err)
	return ok && we.Kind == kind
}

// HTTPStatus maps a dispatch error to an HTTP status for Go controllers:
// validation 422, denied 403, domain 409, anything else 500.
func HTTPStatus(err error) int {
	var we *pool.WireError
	if !errors.As(err, &we) {
		return http.StatusInternalServerError
	}
	switch we.Kind {
	case pool.ErrKindValidation:
		return http.StatusUnprocessableEntity
	case pool.ErrKindDenied:
		return http.StatusForbidden
	case pool.ErrKindDomain:
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
