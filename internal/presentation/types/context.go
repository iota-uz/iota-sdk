package types

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type PageContext struct {
	Title         string
	Locale        string
	Localizer     *i18n.Localizer
	UniTranslator ut.Translator
	Pathname      string
}

func (p *PageContext) T(k string) string {
	return k
}
