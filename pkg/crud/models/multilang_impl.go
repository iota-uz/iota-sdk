package models

import (
	"encoding/json"
	"strings"
)

// multiLangImpl is the default implementation of MultiLang interface
type multiLangImpl struct {
	data map[string]string
}

func (m *multiLangImpl) Get(locale string) (string, error) {
	if m.data == nil {
		return "", ErrUnsupportedLocale
	}

	normalizedLocale := strings.ToLower(locale)
	value, exists := m.data[normalizedLocale]
	if !exists {
		return "", ErrUnsupportedLocale
	}

	return value, nil
}

func (m *multiLangImpl) Set(locale, value string) (MultiLang, error) {
	normalizedLocale := strings.ToLower(locale)

	// Create a copy of the current data
	newData := make(map[string]string)
	if m.data != nil {
		for k, v := range m.data {
			newData[k] = v
		}
	}

	// Set the new value
	newData[normalizedLocale] = value

	return &multiLangImpl{data: newData}, nil
}

func (m *multiLangImpl) IsEmpty() bool {
	if len(m.data) == 0 {
		return true
	}

	// Check if all values are empty strings
	for _, value := range m.data {
		if value != "" {
			return false
		}
	}

	return true
}

func (m *multiLangImpl) Default() string {
	if m.data == nil {
		return ""
	}

	// Priority order: en -> ru -> uz -> first available
	priorities := []string{"en", "ru", "uz"}

	for _, locale := range priorities {
		if value, exists := m.data[locale]; exists && value != "" {
			return value
		}
	}

	// If no priority language found, return first non-empty value
	for _, value := range m.data {
		if value != "" {
			return value
		}
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

func (m *multiLangImpl) GetAll() map[string]string {
	if m.data == nil {
		return make(map[string]string)
	}

	// Return a copy to prevent external modification
	result := make(map[string]string)
	for k, v := range m.data {
		result[k] = v
	}
	return result
}

func (m *multiLangImpl) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func (m *multiLangImpl) String() string {
	return m.Default()
}

func (m *multiLangImpl) MarshalJSON() ([]byte, error) {
	if m.data == nil {
		return json.Marshal(map[string]string{})
	}
	return json.Marshal(m.data)
}

func (m *multiLangImpl) UnmarshalJSON(data []byte) error {
	var temp map[string]string

	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	m.data = temp
	return nil
}
