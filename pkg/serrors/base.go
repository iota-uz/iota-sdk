package serrors

import "github.com/nicksnyder/go-i18n/v2/i18n"

type Base struct {
	Code    string
	Message string
}

func (b *Base) Error() string {
	return b.Message
}

func (b *Base) Localize(*i18n.Localizer) string {
	panic("implement me")
}

type BaseError interface {
	Error() string
	Localize(*i18n.Localizer) string
}
