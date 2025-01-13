package serrors

import "github.com/nicksnyder/go-i18n/v2/i18n"

type BaseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (b *BaseError) Error() string {
	return b.Message
}

func (b *BaseError) Localize(*i18n.Localizer) string {
	panic("implement me")
}

type Base interface {
	Error() string
	Localize(l *i18n.Localizer) string
}
