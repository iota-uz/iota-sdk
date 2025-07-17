package crud_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedJsonSchemas(t *testing.T) {
	t.Run("JsonField with MultiLang schema - array format", func(t *testing.T) {
		// Create a JsonField with MultiLang schema
		field := crud.NewJsonField("title", crud.WithMultiLang())

		// Check that schema type is set correctly
		assert.Equal(t, "multilang", field.SchemaType())

		// Create multilingual data in new array format
		mlData := []crud.LangEntry{
			{Code: "ru", Text: "Заголовок"},
			{Code: "uz", Text: "Sarlavha"},
			{Code: "en", Text: "Title"},
			{Code: "de", Text: "Titel"}, // Add extra language
		}

		// Create field value
		fieldValue := field.Value(mlData)

		// Test AsMultiLang
		ml, err := fieldValue.AsMultiLang()
		require.NoError(t, err)

		// Test dynamic access
		assert.Equal(t, "Заголовок", ml.Get("ru"))
		assert.Equal(t, "Sarlavha", ml.Get("uz"))
		assert.Equal(t, "Title", ml.Get("en"))
		assert.Equal(t, "Titel", ml.Get("de"))

		// Test convenience methods still work
		assert.Equal(t, "Заголовок", ml.Russian())
		assert.Equal(t, "Sarlavha", ml.Uzbek())
		assert.Equal(t, "Title", ml.English())
	})

	t.Run("JsonField with MultiLang schema - backward compatibility", func(t *testing.T) {
		// Test backward compatibility with map format
		field := crud.NewJsonField("title", crud.WithMultiLang())

		// Create multilingual data in old map format
		mlData := map[string]string{
			"ru": "Заголовок",
			"uz": "Sarlavha",
			"en": "Title",
		}

		// Create field value
		fieldValue := field.Value(mlData)

		// Test AsMultiLang
		ml, err := fieldValue.AsMultiLang()
		require.NoError(t, err)
		assert.Equal(t, "Заголовок", ml.Get("ru"))
		assert.Equal(t, "Sarlavha", ml.Get("uz"))
		assert.Equal(t, "Title", ml.Get("en"))
	})
}

func TestTypedJsonSchemasErrorHandling(t *testing.T) {
	t.Run("wrong schema type casting", func(t *testing.T) {
		// Create JsonField without MultiLang schema
		field := crud.NewJsonField("data")

		mlData := map[string]string{
			"ru": "Заголовок",
			"uz": "Sarlavha",
			"en": "Title",
		}

		fieldValue := field.Value(mlData)

		// Try to cast to MultiLang (should fail)
		_, err := fieldValue.AsMultiLang()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a MultiLang field")
	})
}
