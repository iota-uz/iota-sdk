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

// ToJSLocale converts the page locale to JavaScript-compatible locale string.
// This is useful for JavaScript APIs like toLocaleString() and Intl.NumberFormat().
//
// Supported languages include English, Russian, Uzbek (Latin/Cyrillic), Kazakh,
// Kyrgyz, Tajik, Turkmen, Turkish, German, French, Spanish, Chinese, Arabic, and more.
//
// Unknown locales default to "en-US".
func (p *PageContext) ToJSLocale() string {
	locale := p.Locale.String()
	switch locale {
	// English variants
	case "en", "en-US":
		return "en-US"
	case "en-GB":
		return "en-GB"
	case "en-AU":
		return "en-AU"

	// Russian
	case "ru", "ru-RU":
		return "ru-RU"

	// Uzbek Latin
	case "uz", "uz-UZ", "uz-Latn", "uz-Latn-UZ", "oz":
		return "uz-UZ"

	// Uzbek Cyrillic
	case "uz-Cyrl", "uz-Cyrl-UZ":
		return "uz-Cyrl-UZ"

	// Kazakh
	case "kk", "kk-KZ":
		return "kk-KZ"

	// Kyrgyz
	case "ky", "ky-KG":
		return "ky-KG"

	// Tajik
	case "tg", "tg-TJ":
		return "tg-TJ"

	// Turkmen
	case "tk", "tk-TM":
		return "tk-TM"

	// Turkish
	case "tr", "tr-TR":
		return "tr-TR"

	// German
	case "de", "de-DE":
		return "de-DE"
	case "de-AT":
		return "de-AT"
	case "de-CH":
		return "de-CH"

	// French
	case "fr", "fr-FR":
		return "fr-FR"
	case "fr-CA":
		return "fr-CA"

	// Spanish
	case "es", "es-ES":
		return "es-ES"
	case "es-MX":
		return "es-MX"

	// Italian
	case "it", "it-IT":
		return "it-IT"

	// Portuguese
	case "pt", "pt-PT":
		return "pt-PT"
	case "pt-BR":
		return "pt-BR"

	// Chinese
	case "zh", "zh-CN":
		return "zh-CN"
	case "zh-TW":
		return "zh-TW"

	// Japanese
	case "ja", "ja-JP":
		return "ja-JP"

	// Korean
	case "ko", "ko-KR":
		return "ko-KR"

	// Arabic
	case "ar", "ar-SA":
		return "ar-SA"
	case "ar-AE":
		return "ar-AE"

	// Persian
	case "fa", "fa-IR":
		return "fa-IR"

	// Ukrainian
	case "uk", "uk-UA":
		return "uk-UA"

	// Polish
	case "pl", "pl-PL":
		return "pl-PL"

	// Hindi
	case "hi", "hi-IN":
		return "hi-IN"

	// Bengali
	case "bn", "bn-BD":
		return "bn-BD"

	// Vietnamese
	case "vi", "vi-VN":
		return "vi-VN"

	// Thai
	case "th", "th-TH":
		return "th-TH"

	default:
		// Default to English for unknown locales
		return "en-US"
	}
}
