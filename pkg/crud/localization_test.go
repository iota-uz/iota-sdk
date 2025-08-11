package crud

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestWithLocalizationKey проверяет, что опция WithLocalizationKey работает корректно
func TestWithLocalizationKey(t *testing.T) {
	// Создаём поле без кастомного ключа локализации
	field1 := NewStringField("type")

	// LocalizationKey должен вернуть пустую строку (по умолчанию)
	assert.Empty(t, field1.LocalizationKey(), "LocalizationKey should return empty string by default")

	// Создаём поле с кастомным ключом локализации
	field2 := NewStringField("type", WithLocalizationKey("products.Fields.ItemType"))

	// LocalizationKey должен вернуть кастомный ключ
	assert.Equal(t, "products.Fields.ItemType", field2.LocalizationKey(), "LocalizationKey should return custom key")

	// Проверяем, что другие свойства поля не изменились
	assert.Equal(t, "type", field2.Name())
	assert.Equal(t, StringFieldType, field2.Type())
}

// TestWithLocalizationKeyForReservedWords демонстрирует использование для зарезервированных слов
func TestWithLocalizationKeyForReservedWords(t *testing.T) {
	tests := []struct {
		fieldName   string
		customKey   string
		description string
	}{
		{
			fieldName:   "type",
			customKey:   "products.Fields.ItemType",
			description: "type is reserved keyword in Go",
		},
		{
			fieldName:   "order",
			customKey:   "products.Fields.SortOrder",
			description: "order is reserved keyword in SQL",
		},
		{
			fieldName:   "select",
			customKey:   "products.Fields.SelectOption",
			description: "select is reserved keyword in SQL",
		},
		{
			fieldName:   "default",
			customKey:   "products.Fields.DefaultValue",
			description: "default is reserved keyword in Go/SQL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.fieldName, func(t *testing.T) {
			// Создаём поле с зарезервированным именем и кастомным ключом локализации
			field := NewStringField(tt.fieldName, WithLocalizationKey(tt.customKey))

			// Проверяем, что имя поля остается оригинальным
			assert.Equal(t, tt.fieldName, field.Name(), "Field name should remain original")

			// Проверяем, что ключ локализации кастомный
			assert.Equal(t, tt.customKey, field.LocalizationKey(), "Should use custom localization key")

			t.Logf("✓ Field '%s' uses custom key '%s' (%s)", tt.fieldName, tt.customKey, tt.description)
		})
	}
}

// TestLocalizationKeyWithOtherOptions проверяет совместимость с другими опциями
func TestLocalizationKeyWithOtherOptions(t *testing.T) {
	field := NewStringField(
		"type",
		WithRequired(),
		WithLocalizationKey("products.Fields.ItemType"),
		WithMaxLen(50),
		WithMinLen(1),
	)

	// Проверяем, что все опции работают вместе
	assert.Equal(t, "type", field.Name())
	assert.Equal(t, "products.Fields.ItemType", field.LocalizationKey())
	assert.Len(t, field.Rules(), 3, "Should have 3 rules: required, maxLen, minLen")

	// Проверяем attrs
	attrs := field.Attrs()
	assert.Equal(t, 50, attrs["maxLen"])
	assert.Equal(t, 1, attrs["minLen"])
}
