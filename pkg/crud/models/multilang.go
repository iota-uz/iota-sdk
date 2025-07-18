package models

import (
	"encoding/json"
	"errors"
)

var (
	ErrUnsupportedLocale = errors.New("unsupported locale")
	ErrEmptyMultiLang    = errors.New("multilang object is empty")
)

// MultiLang represents a multilingual string supporting uz/ru/en languages
type MultiLang interface {
	// Core methods
	Get(locale string) (string, error)
	Set(locale, value string) (MultiLang, error)
	IsEmpty() bool
	Default() string
	HasLocale(locale string) bool
	GetWithFallback(locale string) string

	// Serialization
	ToJSON() ([]byte, error)
	String() string

	// JSON serialization support
	json.Marshaler
	json.Unmarshaler
}

// MultiLangFromJSON creates a MultiLang from JSON bytes
func MultiLangFromJSON(data []byte) (MultiLang, error) {
	var impl multiLangImpl
	err := json.Unmarshal(data, &impl)
	if err != nil {
		return nil, err
	}
	return &impl, nil
}

// MultiLangFromString creates a MultiLang from JSON string
func MultiLangFromString(jsonStr string) (MultiLang, error) {
	return MultiLangFromJSON([]byte(jsonStr))
}

// NewMultiLang creates a new MultiLang with provided values
func NewMultiLang(uz, ru, en string) MultiLang {
	return &multiLangImpl{
		UZ: uz,
		RU: ru,
		EN: en,
	}
}

// ValidateMultiLang validates that the value is a MultiLang and not empty
func ValidateMultiLang(v interface{}) error {
	ml, ok := v.(MultiLang)
	if !ok {
		return errors.New("value is not a MultiLang object")
	}
	if ml.IsEmpty() {
		return ErrEmptyMultiLang
	}
	return nil
}
