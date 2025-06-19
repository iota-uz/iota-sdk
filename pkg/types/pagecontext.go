package types

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"net/url"
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
	Locale    language.Tag
	URL       *url.URL
	Localizer *i18n.Localizer
}

func (p *PageContext) T(k string, args ...map[string]interface{}) string {
	if len(args) > 1 {
		panic("T(): too many arguments")
	}
	if len(args) == 0 {
		return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: k})
	}
	return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: k, TemplateData: args[0]})
}

func (p *PageContext) TSafe(k string, args ...map[string]interface{}) string {
	if len(args) > 1 {
		panic("T(): too many arguments")
	}

	cfg := &i18n.LocalizeConfig{MessageID: k}
	if len(args) == 1 {
		cfg.TemplateData = args[0]
	}

	result, err := p.Localizer.Localize(cfg)
	if err != nil {
		return ""
	}

	return result
}
