package serrors

import "github.com/iota-uz/go-i18n/v2/i18n"

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
