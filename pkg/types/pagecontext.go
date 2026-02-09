package types

import (
	"net/url"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/i18nutil"
	"golang.org/x/text/language"
)

// PageContextProvider is an interface for managing page-level localization and metadata.
// This interface enables child projects to extend PageContext behavior with custom fields
// (tenant branding, feature flags, analytics) and override methods (custom translations, logging)
// without modifying SDK code.
//
// # Usage Pattern for Child Projects
//
// Child projects can implement custom PageContext types by embedding the interface:
//
//	type CustomPageContext struct {
//	    base types.PageContextProvider
//	    TenantBranding BrandData
//	    FeatureFlags   map[string]bool
//	    Analytics      AnalyticsConfig
//	}
//
//	// Override T() to provide tenant-specific translations
//	func (c *CustomPageContext) T(key string, args ...map[string]interface{}) string {
//	    // Check for tenant-specific translation override
//	    if override := c.lookupTenantTranslation(key); override != "" {
//	        return override
//	    }
//	    // Fall back to base implementation
//	    return c.base.T(key, args...)
//	}
//
//	// Override TSafe() similarly if needed
//	func (c *CustomPageContext) TSafe(key string, args ...map[string]interface{}) string {
//	    // Custom logic here
//	    return c.base.TSafe(key, args...)
//	}
//
// The interface allows:
// - Composition-based extension (embedding rather than inheritance)
// - Custom fields and business logic
// - Method overriding for enhanced functionality
// - Backward compatibility with existing SDK code
type PageContextProvider interface {
	// T translates a message ID to the current locale with optional template data.
	// If a prefix was set via Namespace(), it will be prepended to the message ID.
	T(key string, args ...map[string]interface{}) string

	// TSafe is like T but returns an empty string on error instead of panicking.
	TSafe(key string, args ...map[string]interface{}) string

	// Namespace returns a new PageContextProvider with the specified prefix.
	// All translation calls on the returned context will be prefixed with the given namespace.
	Namespace(prefix string) PageContextProvider

	// ToJSLocale converts the page locale to JavaScript-compatible locale string.
	// This is useful for JavaScript APIs like toLocaleString() and Intl.NumberFormat().
	// Unknown locales default to "en-US".
	ToJSLocale() string

	// GetLocale returns the language.Tag for the current page context.
	GetLocale() language.Tag

	// GetURL returns the *url.URL for the current request.
	GetURL() *url.URL

	// GetLocalizer returns the *i18n.Localizer for the current page context.
	GetLocalizer() *i18n.Localizer
}

// PageContext provides localization and page metadata for template rendering.
//
// Deprecated: Use PageContextProvider interface instead. PageContext is maintained
// for backward compatibility but child projects should prefer implementing custom
// types against the PageContextProvider interface for extensibility. The concrete
// struct will remain supported indefinitely.
//
// See PageContextProvider documentation for extension examples.
type PageContext struct {
	Locale    language.Tag
	URL       *url.URL
	Localizer *i18n.Localizer
	prefix    string
}

// Verify PageContext implements PageContextProvider interface at compile time.
var _ PageContextProvider = (*PageContext)(nil)

func (p *PageContext) T(k string, args ...map[string]interface{}) string {
	if len(args) > 1 {
		panic("T(): too many arguments")
	}

	messageID := k
	if p.prefix != "" {
		messageID = p.prefix + "." + k
	}

	if len(args) == 0 {
		return i18nutil.MustLocalize(p.Localizer, &i18n.LocalizeConfig{MessageID: messageID})
	}
	return i18nutil.MustLocalize(p.Localizer, &i18n.LocalizeConfig{MessageID: messageID, TemplateData: args[0]})
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

// Namespace returns a new PageContextProvider with the specified prefix.
// All translation calls on the returned context will be prefixed with the given namespace.
func (p *PageContext) Namespace(prefix string) PageContextProvider {
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

// GetLocale returns the language.Tag for the current page context.
func (p *PageContext) GetLocale() language.Tag {
	return p.Locale
}

// GetURL returns the *url.URL for the current request.
func (p *PageContext) GetURL() *url.URL {
	return p.URL
}

// GetLocalizer returns the *i18n.Localizer for the current page context.
func (p *PageContext) GetLocalizer() *i18n.Localizer {
	return p.Localizer
}
