package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMultiLang(t *testing.T) {
	ml := NewMultiLang("O'zbek", "Русский", "English")

	// Test through interface methods
	uz, err := ml.Get("uz")
	require.NoError(t, err)
	assert.Equal(t, "O'zbek", uz)

	ru, err := ml.Get("ru")
	require.NoError(t, err)
	assert.Equal(t, "Русский", ru)

	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "English", en)
}

func TestNewMultiLangFromMap(t *testing.T) {
	data := map[string]string{
		"en": "English",
		"ru": "Русский",
		"fr": "Français",
		"de": "Deutsch",
	}

	ml, err := NewMultiLangFromMap(data)
	require.NoError(t, err)

	// Test that all values are accessible
	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "English", en)

	ru, err := ml.Get("ru")
	require.NoError(t, err)
	assert.Equal(t, "Русский", ru)

	fr, err := ml.Get("fr")
	require.NoError(t, err)
	assert.Equal(t, "Français", fr)

	de, err := ml.Get("de")
	require.NoError(t, err)
	assert.Equal(t, "Deutsch", de)

	// Test GetAll method
	allValues := ml.GetAll()
	assert.Equal(t, data, allValues)
}

func TestMultiLang_Get(t *testing.T) {
	ml := NewMultiLang("O'zbek", "Русский", "English")

	tests := []struct {
		name     string
		locale   string
		expected string
		wantErr  bool
	}{
		{
			name:     "get uzbe value",
			locale:   "uz",
			expected: "O'zbek",
			wantErr:  false,
		},
		{
			name:     "get russian value",
			locale:   "ru",
			expected: "Русский",
			wantErr:  false,
		},
		{
			name:     "get english value",
			locale:   "en",
			expected: "English",
			wantErr:  false,
		},
		{
			name:     "get uppercase locale",
			locale:   "EN",
			expected: "English",
			wantErr:  false,
		},
		{
			name:     "get unsupported locale",
			locale:   "fr",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ml.Get(tt.locale)
			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, ErrUnsupportedLocale, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMultiLang_Set(t *testing.T) {
	ml, err := NewMultiLangFromMap(map[string]string{})
	require.NoError(t, err)

	tests := []struct {
		name    string
		locale  string
		value   string
		wantErr bool
		checkFn func(MultiLang)
	}{
		{
			name:    "set uzbek value",
			locale:  "uz",
			value:   "O'zbek",
			wantErr: false,
			checkFn: func(result MultiLang) {
				val, err := result.Get("uz")
				require.NoError(t, err)
				assert.Equal(t, "O'zbek", val)
			},
		},
		{
			name:    "set russian value",
			locale:  "ru",
			value:   "Русский",
			wantErr: false,
			checkFn: func(result MultiLang) {
				val, err := result.Get("ru")
				require.NoError(t, err)
				assert.Equal(t, "Русский", val)
			},
		},
		{
			name:    "set new language (french)",
			locale:  "fr",
			value:   "Français",
			wantErr: false,
			checkFn: func(result MultiLang) {
				val, err := result.Get("fr")
				require.NoError(t, err)
				assert.Equal(t, "Français", val)
			},
		},
		{
			name:    "set uppercase locale",
			locale:  "EN",
			value:   "English",
			wantErr: false,
			checkFn: func(result MultiLang) {
				val, err := result.Get("en")
				require.NoError(t, err)
				assert.Equal(t, "English", val)
			},
		},
		{
			name:    "set empty locale code (should fail validation)",
			locale:  "",
			value:   "Some Value",
			wantErr: true,
			checkFn: func(result MultiLang) {
				// Empty locale codes are now rejected for security
				// result should be nil since setting failed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ml.Set(tt.locale, tt.value)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			tt.checkFn(result)
		})
	}
}

func TestMultiLang_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		ml       MultiLang
		expected bool
	}{
		{
			name:     "completely empty",
			ml:       NewMultiLang("", "", ""),
			expected: true,
		},
		{
			name:     "only uzbek filled",
			ml:       NewMultiLang("O'zbek", "", ""),
			expected: false,
		},
		{
			name:     "only russian filled",
			ml:       NewMultiLang("", "Русский", ""),
			expected: false,
		},
		{
			name:     "only english filled",
			ml:       NewMultiLang("", "", "English"),
			expected: false,
		},
		{
			name:     "all filled",
			ml:       NewMultiLang("O'zbek", "Русский", "English"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ml.IsEmpty())
		})
	}
}

func TestMultiLang_Default(t *testing.T) {
	tests := []struct {
		name     string
		ml       MultiLang
		expected string
	}{
		{
			name:     "all empty",
			ml:       NewMultiLang("", "", ""),
			expected: "",
		},
		{
			name:     "only uzbek",
			ml:       NewMultiLang("O'zbek", "", ""),
			expected: "O'zbek",
		},
		{
			name:     "only russian",
			ml:       NewMultiLang("", "Русский", ""),
			expected: "Русский",
		},
		{
			name:     "only english",
			ml:       NewMultiLang("", "", "English"),
			expected: "English",
		},
		{
			name:     "english and russian",
			ml:       NewMultiLang("", "Русский", "English"),
			expected: "English",
		},
		{
			name:     "all filled",
			ml:       NewMultiLang("O'zbek", "Русский", "English"),
			expected: "English",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.ml.Default())
		})
	}
}

func TestMultiLang_HasLocale(t *testing.T) {
	ml := NewMultiLang("O'zbek", "", "English")

	tests := []struct {
		name     string
		locale   string
		expected bool
	}{
		{
			name:     "has uzbek",
			locale:   "uz",
			expected: true,
		},
		{
			name:     "does not have russian",
			locale:   "ru",
			expected: false,
		},
		{
			name:     "has english",
			locale:   "en",
			expected: true,
		},
		{
			name:     "unsupported locale",
			locale:   "fr",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ml.HasLocale(tt.locale))
		})
	}
}

func TestMultiLang_GetWithFallback(t *testing.T) {
	ml := NewMultiLang("O'zbek", "", "English")

	tests := []struct {
		name     string
		locale   string
		expected string
	}{
		{
			name:     "get existing uzbek",
			locale:   "uz",
			expected: "O'zbek",
		},
		{
			name:     "get non-existing russian, fallback to english",
			locale:   "ru",
			expected: "English",
		},
		{
			name:     "get existing english",
			locale:   "en",
			expected: "English",
		},
		{
			name:     "get unsupported locale, fallback to english",
			locale:   "fr",
			expected: "English",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ml.GetWithFallback(tt.locale))
		})
	}
}

func TestMultiLang_ToJSON(t *testing.T) {
	ml := NewMultiLang("O'zbek", "Русский", "English")

	jsonData, err := ml.ToJSON()
	require.NoError(t, err)

	expected := `{"uz":"O'zbek","ru":"Русский","en":"English"}`
	assert.JSONEq(t, expected, string(jsonData))
}

func TestMultiLang_String(t *testing.T) {
	ml := NewMultiLang("O'zbek", "Русский", "English")

	assert.Equal(t, "English", ml.String())
}

func TestMultiLang_GetAll(t *testing.T) {
	// Test with map constructor
	data := map[string]string{
		"en": "English",
		"ru": "Русский",
		"fr": "Français",
		"de": "Deutsch",
	}
	ml, err := NewMultiLangFromMap(data)
	require.NoError(t, err)

	result := ml.GetAll()
	assert.Equal(t, data, result)

	// Test with legacy constructor
	ml2 := NewMultiLang("O'zbek", "Русский", "English")
	expected := map[string]string{
		"uz": "O'zbek",
		"ru": "Русский",
		"en": "English",
	}
	result2 := ml2.GetAll()
	assert.Equal(t, expected, result2)
}

func TestMultiLangFromJSON(t *testing.T) {
	jsonData := `{"uz":"O'zbek","ru":"Русский","en":"English"}`

	ml, err := MultiLangFromJSON([]byte(jsonData))
	require.NoError(t, err)

	// Test through interface methods
	uz, err := ml.Get("uz")
	require.NoError(t, err)
	assert.Equal(t, "O'zbek", uz)

	ru, err := ml.Get("ru")
	require.NoError(t, err)
	assert.Equal(t, "Русский", ru)

	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "English", en)
}

func TestMultiLangFromJSON_DynamicLanguages(t *testing.T) {
	// Test with additional languages beyond the original three
	jsonData := `{"en":"English","ru":"Русский","fr":"Français","de":"Deutsch","es":"Español"}`

	ml, err := MultiLangFromJSON([]byte(jsonData))
	require.NoError(t, err)

	// Test that all languages are accessible
	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "English", en)

	fr, err := ml.Get("fr")
	require.NoError(t, err)
	assert.Equal(t, "Français", fr)

	de, err := ml.Get("de")
	require.NoError(t, err)
	assert.Equal(t, "Deutsch", de)

	es, err := ml.Get("es")
	require.NoError(t, err)
	assert.Equal(t, "Español", es)

	// Test GetAll returns all languages
	allValues := ml.GetAll()
	expectedValues := map[string]string{
		"en": "English",
		"ru": "Русский",
		"fr": "Français",
		"de": "Deutsch",
		"es": "Español",
	}
	assert.Equal(t, expectedValues, allValues)
}

func TestMultiLangFromString(t *testing.T) {
	jsonStr := `{"uz":"O'zbek","ru":"Русский","en":"English"}`

	ml, err := MultiLangFromString(jsonStr)
	require.NoError(t, err)

	// Test through interface methods
	uz, err := ml.Get("uz")
	require.NoError(t, err)
	assert.Equal(t, "O'zbek", uz)

	ru, err := ml.Get("ru")
	require.NoError(t, err)
	assert.Equal(t, "Русский", ru)

	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "English", en)
}

func TestMultiLangFromJSON_InvalidJSON(t *testing.T) {
	invalidJSON := `{"uz":"O'zbek","ru":"Русский","en":}`

	_, err := MultiLangFromJSON([]byte(invalidJSON))
	require.Error(t, err)
}

func TestValidateMultiLang(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		errType error
	}{
		{
			name:    "valid multilang",
			input:   NewMultiLang("O'zbek", "Русский", "English"),
			wantErr: false,
		},
		{
			name:    "empty multilang",
			input:   NewMultiLang("", "", ""),
			wantErr: true,
			errType: ErrEmptyMultiLang,
		},
		{
			name:    "not a multilang object",
			input:   "string",
			wantErr: true,
		},
		{
			name:    "partially filled multilang",
			input:   NewMultiLang("O'zbek", "", ""),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultiLang(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					assert.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMultiLang_MarshalJSON(t *testing.T) {
	ml := NewMultiLang("O'zbek", "Русский", "English")

	jsonData, err := ml.MarshalJSON()
	require.NoError(t, err)

	expected := `{"uz":"O'zbek","ru":"Русский","en":"English"}`
	assert.JSONEq(t, expected, string(jsonData))
}

func TestMultiLang_UnmarshalJSON(t *testing.T) {
	jsonData := `{"uz":"Test UZ","ru":"Test RU","en":"Test EN"}`

	ml := NewMultiLang("", "", "")
	err := ml.UnmarshalJSON([]byte(jsonData))
	require.NoError(t, err)

	// Test through interface methods
	uz, err := ml.Get("uz")
	require.NoError(t, err)
	assert.Equal(t, "Test UZ", uz)

	ru, err := ml.Get("ru")
	require.NoError(t, err)
	assert.Equal(t, "Test RU", ru)

	en, err := ml.Get("en")
	require.NoError(t, err)
	assert.Equal(t, "Test EN", en)
}
