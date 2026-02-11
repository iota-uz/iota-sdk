package serrors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

type BaseError struct {
	Code         string            `json:"code"`
	Message      string            `json:"message"`
	LocaleKey    string            `json:"locale_key,omitempty"`
	TemplateData map[string]string `json:"-"`
}

func (b *BaseError) Error() string {
	return b.Message
}

func (b *BaseError) Localize(l *i18n.Localizer) string {
	if b.LocaleKey == "" {
		return b.Message
	}

	return l.MustLocalize(&i18n.LocalizeConfig{
		MessageID:    b.LocaleKey,
		TemplateData: b.TemplateData,
	})
}

type Base interface {
	Error() string
	Localize(l *i18n.Localizer) string
}

// NewError creates a new BaseError with the given code, message and locale key
func NewError(code string, message string, localeKey string) *BaseError {
	return &BaseError{
		Code:      code,
		Message:   message,
		LocaleKey: localeKey,
	}
}

// WithTemplateData adds template data to the error for localization
func (b *BaseError) WithTemplateData(data map[string]string) *BaseError {
	b.TemplateData = data
	return b
}

type Op string

type Kind int

const (
	Other Kind = iota
	Invalid
	KindValidation
	NotFound
	PermissionDenied
	Internal
)

type Error struct {
	Op      Op
	Kind    Kind
	Context string
	Err     error
}

func (e *Error) Error() string {
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(string(e.Op))
		b.WriteString(": ")
	}
	if e.Context != "" {
		b.WriteString(e.Context)
		b.WriteString(": ")
	}
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error {
	return e.Err
}

// ErrorKind returns a semantic error code string for the applet RPC error classifier.
// This satisfies the api.ErrorClassifier interface (applets framework) so that
// serrors.Error.Kind is properly mapped to RPC error codes.
//
// When the current error has Kind == Other (the zero value, i.e. no explicit kind was set),
// it walks the wrapped error chain to find the first non-zero Kind.
func (e *Error) ErrorKind() string {
	if e.Kind != Other {
		return kindToString(e.Kind)
	}
	// Walk chain to find first non-zero Kind.
	var inner *Error
	if errors.As(e.Err, &inner) {
		return inner.ErrorKind()
	}
	return ""
}

func kindToString(k Kind) string {
	switch k {
	case Invalid:
		return "invalid"
	case KindValidation:
		return "validation"
	case NotFound:
		return "not_found"
	case PermissionDenied:
		return "forbidden"
	case Internal:
		return "internal"
	default:
		return ""
	}
}

// Operation returns the Op string for structured error tracing.
func (e *Error) Operation() string {
	return string(e.Op)
}

func E(args ...interface{}) error {
	e := &Error{}
	var hasError bool
	for _, arg := range args {
		switch arg := arg.(type) {
		case Op:
			e.Op = arg
		case Kind:
			e.Kind = arg
		case string:
			if !hasError {
				e.Context = arg
			} else {
				e.Context = arg
			}
		case error:
			if e.Context == "" && !hasError {
				e.Err = arg
			} else {
				e.Err = arg
			}
			hasError = true
		}
	}
	if e.Context != "" && !hasError {
		e.Err = fmt.Errorf("%s", e.Context)
		e.Context = ""
	}
	return e
}
