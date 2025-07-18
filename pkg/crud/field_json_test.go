package crud

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/stretchr/testify/assert"
)

type testStruct struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestNewJSONField(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{
		Validator: func(v testStruct) error {
			return nil
		},
	})

	assert.Equal(t, "test_json", field.Name())
	assert.Equal(t, JSONFieldType, field.Type())
	assert.NotNil(t, field.Validator())
}

func TestJSONField_Value_SerializesCorrectly(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{})

	testData := testStruct{
		Name: "John",
		Age:  30,
	}

	fieldValue := field.Value(testData)

	assert.JSONEq(t, `{"name":"John","age":30}`, fieldValue.Value().(string))
}

func TestJSONField_Value_NilHandling(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{})

	fieldValue := field.Value(nil)

	assert.Nil(t, fieldValue.Value())
}

func TestJSONField_Value_StringHandling(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{})

	jsonStr := `{"name":"Jane","age":25}`
	fieldValue := field.Value(jsonStr)

	assert.JSONEq(t, jsonStr, fieldValue.Value().(string))
}

func TestJSONField_Value_MapHandling(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{})

	mapData := map[string]interface{}{
		"name": "Jane",
		"age":  25,
	}

	fieldValue := field.Value(mapData)

	// Should be converted to JSON string
	result := fieldValue.Value().(string)
	assert.Contains(t, result, "Jane")
	assert.Contains(t, result, "25")
}

func TestJSONField_InitialValue_DeserializesCorrectly(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{}, WithInitialValue(func() any {
		return `{"name":"Jane","age":25}`
	}))

	result := field.InitialValue()

	expected := testStruct{
		Name: "Jane",
		Age:  25,
	}

	assert.Equal(t, expected, result)
}

func TestJSONField_InitialValue_EmptyString(t *testing.T) {
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{}, WithInitialValue(func() any {
		return ""
	}))

	result := field.InitialValue()

	assert.Nil(t, result)
}

func TestJSONField_ValidationCalled(t *testing.T) {
	validationCalled := false
	field := NewJSONField[testStruct]("test_json", JSONFieldConfig[testStruct]{
		Validator: func(v testStruct) error {
			validationCalled = true
			return nil
		},
	})

	testData := testStruct{
		Name: "John",
		Age:  30,
	}

	field.Value(testData)

	assert.True(t, validationCalled)
}

func TestJSONField_ValidationError(t *testing.T) {
	field := NewJSONField[models.MultiLang]("test_json", JSONFieldConfig[models.MultiLang]{
		Validator: func(ml models.MultiLang) error {
			if ml.IsEmpty() {
				return models.ErrEmptyMultiLang
			}
			return nil
		},
	})

	emptyData := models.NewMultiLang("", "", "")

	assert.Panics(t, func() {
		fieldValue := field.Value(emptyData)
		fieldValue.Value() // This should panic
	})
}

func TestJSONField_InterfaceType(t *testing.T) {
	// Test with interface type
	type Stringer interface {
		String() string
	}

	field := NewJSONField[Stringer]("test_interface", JSONFieldConfig[Stringer]{})

	// MultiLang implements String() method, so it satisfies Stringer interface
	testData := models.NewMultiLang("", "", "English")

	fieldValue := field.Value(testData)
	assert.NotNil(t, fieldValue)

	// Should be able to serialize interface
	result := fieldValue.Value().(string)
	assert.Contains(t, result, "English")
}
