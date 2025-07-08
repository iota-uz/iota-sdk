package types

import (
	"net/url"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"
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
	prefix    string
}

func (p *PageContext) T(k string, args ...map[string]interface{}) string {
	if len(args) > 1 {
		panic("T(): too many arguments")
	}

	messageID := k
	if p.prefix != "" {
		messageID = p.prefix + "." + k
	}

	if len(args) == 0 {
		return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: messageID})
	}
	return p.Localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: messageID, TemplateData: args[0]})
}

func (p *PageContext) TSafe(k string, args ...map[string]interface{}) string {
	if len(args) > 1 {
		panic("T(): too many arguments")
	}

	messageID := k
	if p.prefix != "" {
		messageID = p.prefix + "." + k
	}

	cfg := &i18n.LocalizeConfig{MessageID: messageID}
	if len(args) == 1 {
		cfg.TemplateData = args[0]
	}

	result, err := p.Localizer.Localize(cfg)
	if err != nil {
		return ""
	}

	return result
}

// Namespace returns a new PageContext with the specified prefix.
// All translation calls on the returned context will be prefixed with the given namespace.
func (p *PageContext) Namespace(prefix string) *PageContext {
	return &PageContext{
		Locale:    p.Locale,
		URL:       p.URL,
		Localizer: p.Localizer,
		prefix:    prefix,
	}
}
