package models

import (
	"encoding/json"
	"strings"
)

// multiLangImpl is the default implementation of MultiLang interface
type multiLangImpl struct {
	UZ string `json:"uz"`
	RU string `json:"ru"`
	EN string `json:"en"`
}

func (m *multiLangImpl) Get(locale string) (string, error) {
	switch strings.ToLower(locale) {
	case "uz":
		return m.UZ, nil
	case "ru":
		return m.RU, nil
	case "en":
		return m.EN, nil
	default:
		return "", ErrUnsupportedLocale
	}
}

func (m *multiLangImpl) Set(locale, value string) (MultiLang, error) {
	result := &multiLangImpl{
		UZ: m.UZ,
		RU: m.RU,
		EN: m.EN,
	}

	switch strings.ToLower(locale) {
	case "uz":
		result.UZ = value
	case "ru":
		result.RU = value
	case "en":
		result.EN = value
	default:
		return m, ErrUnsupportedLocale
	}
	return result, nil
}

func (m *multiLangImpl) IsEmpty() bool {
	return m.UZ == "" && m.RU == "" && m.EN == ""
}

func (m *multiLangImpl) Default() string {
	if m.EN != "" {
		return m.EN
	}
	if m.RU != "" {
		return m.RU
	}
	if m.UZ != "" {
		return m.UZ
	}
	return ""
}

func (m *multiLangImpl) HasLocale(locale string) bool {
	value, err := m.Get(locale)
	return err == nil && value != ""
}

func (m *multiLangImpl) GetWithFallback(locale string) string {
	if value, err := m.Get(locale); err == nil && value != "" {
		return value
	}
	return m.Default()
}

func (m *multiLangImpl) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *multiLangImpl) String() string {
	return m.Default()
}

func (m *multiLangImpl) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		UZ string `json:"uz"`
		RU string `json:"ru"`
		EN string `json:"en"`
	}{
		UZ: m.UZ,
		RU: m.RU,
		EN: m.EN,
	})
}

func (m *multiLangImpl) UnmarshalJSON(data []byte) error {
	var temp struct {
		UZ string `json:"uz"`
		RU string `json:"ru"`
		EN string `json:"en"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	m.UZ = temp.UZ
	m.RU = temp.RU
	m.EN = temp.EN

	return nil
}
