package intl

import (
	"encoding/json"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	SupportedLanguages = []language.Tag{
		language.Russian,
		language.English,
		language.Uzbek,
	}
)

func LoadBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile("pkg/locales/en.json")
	bundle.MustLoadMessageFile("pkg/locales/ru.json")
	return bundle
}
