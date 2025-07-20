package models

import (
	"encoding/json"
	"errors"
	"strings"
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

	// Map access for renderers
	GetAll() map[string]string

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

// NewMultiLang creates a new MultiLang with provided values (backward compatibility)
func NewMultiLang(uz, ru, en string) MultiLang {
	data := make(map[string]string)
	if uz != "" {
		data["uz"] = uz
	}
	if ru != "" {
		data["ru"] = ru
	}
	if en != "" {
		data["en"] = en
	}
	return &multiLangImpl{data: data}
}

// NewMultiLangFromMap creates a new MultiLang from a map of locale codes to values
func NewMultiLangFromMap(values map[string]string) MultiLang {
	data := make(map[string]string)
	for locale, value := range values {
		if value != "" {
			data[strings.ToLower(locale)] = value
		}
	}
	return &multiLangImpl{data: data}
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
