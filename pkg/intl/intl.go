package intl

import (
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
