package user

import "errors"

type UILanguage string

func NewUILanguage(l string) (UILanguage, error) {
	language := UILanguage(l)
	if !language.IsValid() {
		return "", errors.New("invalid language")
	}
	return language, nil
}

func (l UILanguage) IsValid() bool {
	switch l {
	case UILanguageEN, UILanguageRU, UILanguageUZ:
		return true
	}
	return false
}
