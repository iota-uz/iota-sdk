package serrors

import (
	"strings"

	"github.com/iota-uz/go-i18n/v2/i18n"
)

type PublicField struct {
	Field   string `json:"field"`
	Reason  Reason `json:"reason,omitempty"`
	Message string `json:"message"`
}
type Projection struct {
	Code    string        `json:"code"`
	Reason  Reason        `json:"reason,omitempty"`
	Message string        `json:"message"`
	Fields  []PublicField `json:"fields,omitempty"`
}

func Localize(message Message, l *i18n.Localizer, fallback string) string {
	if message.ID == "" && strings.TrimSpace(message.Text) != "" {
		return message.Text
	}
	if l == nil || message.ID == "" {
		return fallback
	}
	args := make(map[string]any, len(message.Args))
	for key, value := range message.Args {
		if value.message != nil {
			text := Localize(*value.message, l, "")
			if text == "" {
				return fallback
			}
			args[key] = text
		} else {
			args[key] = value.scalar
		}
	}
	cfg := &i18n.LocalizeConfig{MessageID: message.ID, TemplateData: args}
	if len(args) == 0 {
		cfg.TemplateData = nil
	}
	if message.Count != nil {
		cfg.PluralCount = *message.Count
	}
	text, err := l.Localize(cfg)
	if err != nil || strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}

func Public(err error, l *i18n.Localizer) Projection {
	code := CodeOf(err)
	fallback := "An unexpected error occurred."
	switch code {
	case Internal:
		fallback = "An unexpected error occurred."
	case Invalid:
		fallback = "Check the supplied information."
	case NotFound:
		fallback = "The requested item was not found."
	case AlreadyExists:
		fallback = "The item already exists."
	case Conflict:
		fallback = "The operation conflicts with the current state."
	case FailedPrecondition:
		fallback = "The operation cannot be completed in the current state."
	case PermissionDenied:
		fallback = "You do not have permission to perform this operation."
	case Unauthenticated:
		fallback = "Authentication is required."
	case RateLimited:
		fallback = "Too many requests. Please try again later."
	case Unavailable:
		fallback = "The service is temporarily unavailable."
	case Timeout:
		fallback = "The operation timed out."
	case Canceled:
		fallback = "The operation was canceled."
	case Unimplemented:
		fallback = "This operation is not supported."
	}
	fallback = Localize(Message{ID: "Serrors." + code.String()}, l, fallback)
	result := Projection{Code: code.String(), Reason: ReasonOf(err), Message: Localize(MessageOf(err), l, fallback)}
	for _, field := range FieldsOf(err) {
		if field.Field == "" {
			continue
		}
		result.Fields = append(result.Fields, PublicField{Field: field.Field, Reason: field.Reason, Message: Localize(field.Message, l, Localize(Message{ID: "Serrors.invalid"}, l, "Check the supplied information."))})
	}
	return result
}

func FieldMap(err error, l *i18n.Localizer) map[string]string {
	result := make(map[string]string)
	for _, field := range Public(err, l).Fields {
		result[field.Field] = field.Message
	}
	return result
}

func NewInternal(msg string) *Error           { return New(Internal, msg) }
func NewInvalid(msg string) *Error            { return New(Invalid, msg) }
func NewNotFound(msg string) *Error           { return New(NotFound, msg) }
func NewAlreadyExists(msg string) *Error      { return New(AlreadyExists, msg) }
func NewConflict(msg string) *Error           { return New(Conflict, msg) }
func NewFailedPrecondition(msg string) *Error { return New(FailedPrecondition, msg) }
func NewPermissionDenied(msg string) *Error   { return New(PermissionDenied, msg) }
func NewUnauthenticated(msg string) *Error    { return New(Unauthenticated, msg) }
func NewRateLimited(msg string) *Error        { return New(RateLimited, msg) }
func NewUnavailable(msg string) *Error        { return New(Unavailable, msg) }
func NewTimeout(msg string) *Error            { return New(Timeout, msg) }
func NewCanceled(msg string) *Error           { return New(Canceled, msg) }
func NewUnimplemented(msg string) *Error      { return New(Unimplemented, msg) }
