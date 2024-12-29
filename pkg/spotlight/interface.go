package spotlight

import (
	"github.com/a-h/templ"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Spotlight interface {
	Find(localizer *i18n.Localizer, q string) []Item
	Register(...Item)
}

type Item interface {
	Icon() templ.Component
	Localized(localizer *i18n.Localizer) string
	Link() string
}
