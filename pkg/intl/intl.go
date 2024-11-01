package intl

import (
	"encoding/json"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

type SupportedLanguage struct {
	Code        string
	VerboseName string
	Tag         language.Tag
}

var (
	SupportedLanguages = []SupportedLanguage{
		{
			Code:        "ru",
			VerboseName: "Русский",
			Tag:         language.Russian,
		},
		{
			Code:        "en",
			VerboseName: "English",
			Tag:         language.English,
		},
		{
			Code:        "uz",
			VerboseName: "O'zbekcha",
			Tag:         language.Uzbek,
		},
	}
)

func LoadBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile("pkg/locales/en.json")
	bundle.MustLoadMessageFile("pkg/locales/ru.json")
	return bundle
}
