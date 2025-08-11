package models

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnsupportedLocale = errors.New("unsupported locale")
	ErrEmptyMultiLang    = errors.New("multilang object is empty")
	ErrInvalidLocaleCode = errors.New("invalid locale code format")
	ErrValueTooLong      = errors.New("translation value exceeds maximum length")
	ErrInvalidCharacters = errors.New("translation contains invalid characters")
)

const (
	MaxLocaleCodeLength = 10
	MaxValueLength      = 1000
)

// Locale code validation regex: 2-5 lowercase letters, optionally followed by dash and 2-8 more letters
var localeCodeRegex = regexp.MustCompile(`^[a-z]{2,5}(-[a-z]{2,8})?$`)

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

// MultiLangFromJSON creates a MultiLang from JSON bytes with validation
func MultiLangFromJSON(data []byte) (MultiLang, error) {
	var impl multiLangImpl
	err := json.Unmarshal(data, &impl)
	if err != nil {
		return nil, err
	}

	// Validate the parsed data
	if err := ValidateMultiLangData(impl.data); err != nil {
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

// NewMultiLangFromMap creates a new MultiLang from a map of locale codes to values with validation
func NewMultiLangFromMap(values map[string]string) (MultiLang, error) {
	data := make(map[string]string)
	for locale, value := range values {
		if value != "" {
			normalizedLocale := strings.ToLower(locale)

			// Validate locale code and value
			if err := ValidateLocaleCode(normalizedLocale); err != nil {
				return nil, err
			}
			if err := ValidateTranslationValue(value); err != nil {
				return nil, err
			}

			data[normalizedLocale] = value
		}
	}

	if len(data) == 0 {
		return &multiLangImpl{data: data}, nil // Allow empty MultiLang
	}

	return &multiLangImpl{data: data}, nil
}

// ValidateLocaleCode validates a locale code format and length
func ValidateLocaleCode(locale string) error {
	if len(locale) == 0 {
		return ErrInvalidLocaleCode
	}
	if len(locale) > MaxLocaleCodeLength {
		return ErrInvalidLocaleCode
	}
	if !localeCodeRegex.MatchString(locale) {
		return ErrInvalidLocaleCode
	}
	return nil
}

// ValidateTranslationValue validates a translation value for length and content safety
func ValidateTranslationValue(value string) error {
	if !utf8.ValidString(value) {
		return ErrInvalidCharacters
	}
	if len(value) > MaxValueLength {
		return ErrValueTooLong
	}

	// Check for null bytes and other control characters that could cause issues
	for _, r := range value {
		if r == 0 || (r < 32 && r != '\n' && r != '\r' && r != '\t') {
			return ErrInvalidCharacters
		}
	}

	return nil
}

// ValidateMultiLangData validates a map of locale codes to translation values
func ValidateMultiLangData(data map[string]string) error {
	if len(data) == 0 {
		return ErrEmptyMultiLang
	}

	for locale, value := range data {
		if err := ValidateLocaleCode(locale); err != nil {
			return err
		}
		if err := ValidateTranslationValue(value); err != nil {
			return err
		}
	}

	return nil
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

	// Additional validation on the data
	return ValidateMultiLangData(ml.GetAll())
}
