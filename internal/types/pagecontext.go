package types

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/presentation/viewmodels"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type PageData struct {
	Title       string
	Description string
}

func NewPageData(title string, description string) *PageData {
	return &PageData{
		Title:       title,
		Description: description,
	}
}

type PageContext struct {
	Title         string
	Locale        string
	Localizer     *i18n.Localizer
	UniTranslator ut.Translator
	User          *user.User
	UserViewModel *viewmodels.User
	NavItems      []NavigationItem
	Pathname      string
}

func (p *PageContext) T(k string) string {
	return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: k})
}
