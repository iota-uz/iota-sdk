package types

import "github.com/nicksnyder/go-i18n/v2/i18n"

type PageContext struct {
	Title     string
	Lang      string
	Localizer *i18n.Localizer
}

func (p *PageContext) T(k string) string {
	return k
}
