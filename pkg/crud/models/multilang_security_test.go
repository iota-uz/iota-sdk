package models

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMultiLang_SecurityValidation tests various security scenarios to prevent XSS and injection attacks
func TestMultiLang_SecurityValidation(t *testing.T) {
	tests := []struct {
		name      string
		locale    string
		value     string
		wantErr   bool
		errorType error
	}{
		{
			name:      "valid locale and value",
			locale:    "en",
			value:     "Hello World",
			wantErr:   false,
			errorType: nil,
		},
		{
			name:      "script tag in value",
			locale:    "en",
			value:     "<script>alert('xss')</script>",
			wantErr:   false, // HTML content is allowed in values, but will be sanitized on frontend
			errorType: nil,
		},
		{
			name:      "script tag in locale code",
			locale:    "<script>",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "null byte in locale",
			locale:    "en\x00",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "null byte in value",
			locale:    "en",
			value:     "test\x00value",
			wantErr:   true,
			errorType: ErrInvalidCharacters,
		},
		{
			name:      "control characters in value",
			locale:    "en",
			value:     "test\x01\x02value",
			wantErr:   true,
			errorType: ErrInvalidCharacters,
		},
		{
			name:      "allowed control characters (newline, tab, carriage return)",
			locale:    "en",
			value:     "line1\nline2\tindented\rcarriage",
			wantErr:   false,
			errorType: nil,
		},
		{
			name:      "very long locale code",
			locale:    strings.Repeat("a", 20),
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "very long value",
			locale:    "en",
			value:     strings.Repeat("a", 2000),
			wantErr:   true,
			errorType: ErrValueTooLong,
		},
		{
			name:      "SQL injection attempt in locale",
			locale:    "'; DROP TABLE users; --",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "JavaScript injection in locale",
			locale:    "javascript:alert(1)",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "Unicode normalization attack",
			locale:    "en",
			value:     "test\u202e\u202d", // Right-to-left override characters
			wantErr:   false,              // Unicode characters are allowed in values
			errorType: nil,
		},
		{
			name:      "empty locale code",
			locale:    "",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "whitespace-only locale",
			locale:    "   ",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "valid locale with variant",
			locale:    "uz-cyrl",
			value:     "test",
			wantErr:   false,
			errorType: nil,
		},
		{
			name:      "invalid locale with too many parts",
			locale:    "en-us-west-coast",
			value:     "test",
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test via Set method
			ml, err := NewMultiLangFromMap(map[string]string{})
			require.NoError(t, err)

			result, err := ml.Set(tt.locale, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)

				// Verify the value was set correctly
				retrievedValue, getErr := result.Get(tt.locale)
				require.NoError(t, getErr)
				assert.Equal(t, tt.value, retrievedValue)
			}
		})
	}
}

// TestMultiLangFromJSON_SecurityValidation tests JSON parsing security
func TestMultiLangFromJSON_SecurityValidation(t *testing.T) {
	tests := []struct {
		name      string
		jsonInput string
		wantErr   bool
		errorType error
	}{
		{
			name:      "valid JSON",
			jsonInput: `{"en":"Hello","ru":"Привет"}`,
			wantErr:   false,
			errorType: nil,
		},
		{
			name:      "JSON with script tags in values",
			jsonInput: `{"en":"<script>alert('xss')</script>","ru":"test"}`,
			wantErr:   false, // HTML in values is allowed but will be sanitized on frontend
			errorType: nil,
		},
		{
			name:      "JSON with invalid locale codes",
			jsonInput: `{"<script>":"test","en":"valid"}`,
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "JSON with null bytes in values",
			jsonInput: `{"en":"test\u0000value"}`,
			wantErr:   true,
			errorType: ErrInvalidCharacters,
		},
		{
			name:      "JSON with too long values",
			jsonInput: `{"en":"` + strings.Repeat("a", 2000) + `"}`,
			wantErr:   true,
			errorType: ErrValueTooLong,
		},
		{
			name:      "JSON with too long locale codes",
			jsonInput: `{"` + strings.Repeat("a", 20) + `":"test"}`,
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "JSON with empty locale codes",
			jsonInput: `{"":"test","en":"valid"}`,
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name:      "deeply nested JSON attack attempt",
			jsonInput: `{"en":{"nested":"value"}}`,
			wantErr:   true, // JSON unmarshaling will fail - values must be strings
			errorType: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MultiLangFromJSON([]byte(tt.jsonInput))

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestMultiLangFromMap_SecurityValidation tests map creation security
func TestMultiLangFromMap_SecurityValidation(t *testing.T) {
	tests := []struct {
		name      string
		inputMap  map[string]string
		wantErr   bool
		errorType error
	}{
		{
			name: "valid map",
			inputMap: map[string]string{
				"en": "Hello",
				"ru": "Привет",
			},
			wantErr:   false,
			errorType: nil,
		},
		{
			name: "map with invalid locale codes",
			inputMap: map[string]string{
				"<script>": "malicious",
				"en":       "valid",
			},
			wantErr:   true,
			errorType: ErrInvalidLocaleCode,
		},
		{
			name: "map with null bytes",
			inputMap: map[string]string{
				"en": "test\x00value",
			},
			wantErr:   true,
			errorType: ErrInvalidCharacters,
		},
		{
			name: "map with control characters",
			inputMap: map[string]string{
				"en": "test\x01\x02value",
			},
			wantErr:   true,
			errorType: ErrInvalidCharacters,
		},
		{
			name: "map with allowed control characters",
			inputMap: map[string]string{
				"en": "line1\nline2\tindented\rcarriage",
			},
			wantErr:   false,
			errorType: nil,
		},
		{
			name:      "empty map (should be allowed)",
			inputMap:  map[string]string{},
			wantErr:   false,
			errorType: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewMultiLangFromMap(tt.inputMap)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorType != nil {
					require.ErrorIs(t, err, tt.errorType)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

// TestValidationFunctions tests individual validation functions
func TestValidationFunctions(t *testing.T) {
	t.Run("ValidateLocaleCode", func(t *testing.T) {
		tests := []struct {
			name    string
			locale  string
			wantErr bool
		}{
			{"valid short locale", "en", false},
			{"valid locale with variant", "uz-cyrl", false},
			{"empty locale", "", true},
			{"too long locale", strings.Repeat("a", 20), true},
			{"invalid characters", "<script>", true},
			{"numbers in locale", "en1", true},
			{"uppercase letters", "EN", true},
			{"valid 3-letter locale", "uzb", false},
			{"valid 5-letter locale", "uzbek", false},
			{"invalid 6-letter locale", "uzbeki", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateLocaleCode(tt.locale)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("ValidateTranslationValue", func(t *testing.T) {
		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{"valid value", "Hello World", false},
			{"empty value", "", false},
			{"value with newlines", "line1\nline2", false},
			{"value with tabs", "col1\tcol2", false},
			{"value with carriage return", "text\rmore", false},
			{"value with null byte", "test\x00value", true},
			{"value with control chars", "test\x01value", true},
			{"very long value", strings.Repeat("a", 2000), true},
			{"max length value", strings.Repeat("a", 1000), false},
			{"unicode value", "тест 测试 テスト", false},
			{"emoji value", "Hello 👋 World", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := ValidateTranslationValue(tt.value)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}
