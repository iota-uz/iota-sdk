package details

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIntegratorDetails(t *testing.T) {
	t.Run("with no options", func(t *testing.T) {
		d := NewIntegratorDetails()
		require.NotNil(t, d)
		assert.NotNil(t, d.Data())
		assert.Empty(t, d.Data())
		assert.Equal(t, int32(0), d.ErrorCode())
		assert.Equal(t, "", d.ErrorNote())
	})

	t.Run("with data option", func(t *testing.T) {
		data := map[string]any{
			"key":   "value",
			"count": 42,
		}
		d := NewIntegratorDetails(IntegratorWithData(data))
		require.NotNil(t, d)
		assert.Equal(t, data, d.Data())
		assert.Equal(t, int32(0), d.ErrorCode())
		assert.Equal(t, "", d.ErrorNote())
	})

	t.Run("with error code option", func(t *testing.T) {
		errorCode := int32(404)
		d := NewIntegratorDetails(IntegratorWithErrorCode(errorCode))
		require.NotNil(t, d)
		assert.Equal(t, errorCode, d.ErrorCode())
		assert.NotNil(t, d.Data())
		assert.Empty(t, d.Data())
	})

	t.Run("with error note option", func(t *testing.T) {
		errorNote := "Not found"
		d := NewIntegratorDetails(IntegratorWithErrorNote(errorNote))
		require.NotNil(t, d)
		assert.Equal(t, errorNote, d.ErrorNote())
	})

	t.Run("with all options", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		errorCode := int32(500)
		errorNote := "Internal error"

		d := NewIntegratorDetails(
			IntegratorWithData(data),
			IntegratorWithErrorCode(errorCode),
			IntegratorWithErrorNote(errorNote),
		)

		assert.Equal(t, data, d.Data())
		assert.Equal(t, errorCode, d.ErrorCode())
		assert.Equal(t, errorNote, d.ErrorNote())
	})

	t.Run("with nil data option", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithData(nil))
		require.NotNil(t, d)
		assert.Nil(t, d.Data())
	})
}

func TestIntegratorDetails_Data(t *testing.T) {
	data := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}
	d := NewIntegratorDetails(IntegratorWithData(data))
	assert.Equal(t, data, d.Data())
}

func TestIntegratorDetails_SetData(t *testing.T) {
	t.Run("replaces data immutably", func(t *testing.T) {
		originalData := map[string]any{"key1": "value1"}
		original := NewIntegratorDetails(IntegratorWithData(originalData))

		newData := map[string]any{"key2": "value2"}
		modified := original.SetData(newData)

		// Verify immutability - original unchanged
		assert.Equal(t, originalData, original.Data())
		assert.Equal(t, newData, modified.Data())
		assert.NotEqual(t, original.Data(), modified.Data())
	})

	t.Run("can set nil data", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithData(map[string]any{"key": "value"}))
		modified := original.SetData(nil)

		assert.NotNil(t, original.Data())
		assert.Nil(t, modified.Data())
	})

	t.Run("can set empty map", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithData(map[string]any{"key": "value"}))
		modified := original.SetData(map[string]any{})

		assert.NotEmpty(t, original.Data())
		assert.Empty(t, modified.Data())
	})
}

func TestIntegratorDetails_Get(t *testing.T) {
	t.Run("returns value for existing key", func(t *testing.T) {
		data := map[string]any{
			"name":   "John",
			"age":    30,
			"active": true,
		}
		d := NewIntegratorDetails(IntegratorWithData(data))

		assert.Equal(t, "John", d.Get("name"))
		assert.Equal(t, 30, d.Get("age"))
		assert.Equal(t, true, d.Get("active"))
	})

	t.Run("returns nil for non-existing key", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithData(map[string]any{"key": "value"}))
		assert.Nil(t, d.Get("nonexistent"))
	})

	t.Run("returns nil when data is nil", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithData(nil))
		assert.Nil(t, d.Get("any"))
	})

	t.Run("returns nil when data is empty", func(t *testing.T) {
		d := NewIntegratorDetails()
		assert.Nil(t, d.Get("any"))
	})
}

func TestIntegratorDetails_Set(t *testing.T) {
	t.Run("sets new key immutably", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithData(map[string]any{"key1": "value1"}))
		modified := original.Set("key2", "value2")

		// Verify immutability
		assert.Equal(t, 1, len(original.Data()))
		assert.Equal(t, "value1", original.Get("key1"))
		assert.Nil(t, original.Get("key2"))

		assert.Equal(t, 2, len(modified.Data()))
		assert.Equal(t, "value1", modified.Get("key1"))
		assert.Equal(t, "value2", modified.Get("key2"))
	})

	t.Run("updates existing key immutably", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithData(map[string]any{"key": "original"}))
		modified := original.Set("key", "updated")

		assert.Equal(t, "original", original.Get("key"))
		assert.Equal(t, "updated", modified.Get("key"))
	})

	t.Run("can set various types", func(t *testing.T) {
		d := NewIntegratorDetails()

		d = d.Set("string", "text")
		d = d.Set("int", 42)
		d = d.Set("bool", true)
		d = d.Set("float", 3.14)
		d = d.Set("map", map[string]any{"nested": "value"})
		d = d.Set("slice", []string{"a", "b", "c"})

		assert.Equal(t, "text", d.Get("string"))
		assert.Equal(t, 42, d.Get("int"))
		assert.Equal(t, true, d.Get("bool"))
		assert.Equal(t, 3.14, d.Get("float"))
		assert.Equal(t, map[string]any{"nested": "value"}, d.Get("map"))
		assert.Equal(t, []string{"a", "b", "c"}, d.Get("slice"))
	})

	t.Run("can set nil value", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithData(map[string]any{"key": "value"}))
		modified := d.Set("key", nil)

		assert.Equal(t, "value", d.Get("key"))
		assert.Nil(t, modified.Get("key"))
	})

	t.Run("maintains immutability with empty data", func(t *testing.T) {
		original := NewIntegratorDetails()
		modified := original.Set("key", "value")

		assert.Empty(t, original.Data())
		assert.Equal(t, 1, len(modified.Data()))
		assert.Equal(t, "value", modified.Get("key"))
	})
}

func TestIntegratorDetails_ErrorCode(t *testing.T) {
	t.Run("returns error code", func(t *testing.T) {
		errorCode := int32(404)
		d := NewIntegratorDetails(IntegratorWithErrorCode(errorCode))
		assert.Equal(t, errorCode, d.ErrorCode())
	})

	t.Run("returns default zero value", func(t *testing.T) {
		d := NewIntegratorDetails()
		assert.Equal(t, int32(0), d.ErrorCode())
	})
}

func TestIntegratorDetails_SetErrorCode(t *testing.T) {
	t.Run("sets error code immutably", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithErrorCode(404))
		modified := original.SetErrorCode(500)

		assert.Equal(t, int32(404), original.ErrorCode())
		assert.Equal(t, int32(500), modified.ErrorCode())
	})

	t.Run("can set zero error code", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithErrorCode(404))
		modified := original.SetErrorCode(0)

		assert.Equal(t, int32(404), original.ErrorCode())
		assert.Equal(t, int32(0), modified.ErrorCode())
	})
}

func TestIntegratorDetails_ErrorNote(t *testing.T) {
	t.Run("returns error note", func(t *testing.T) {
		errorNote := "Not found"
		d := NewIntegratorDetails(IntegratorWithErrorNote(errorNote))
		assert.Equal(t, errorNote, d.ErrorNote())
	})

	t.Run("returns default empty string", func(t *testing.T) {
		d := NewIntegratorDetails()
		assert.Equal(t, "", d.ErrorNote())
	})
}

func TestIntegratorDetails_SetErrorNote(t *testing.T) {
	t.Run("sets error note immutably", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithErrorNote("Original error"))
		modified := original.SetErrorNote("New error")

		assert.Equal(t, "Original error", original.ErrorNote())
		assert.Equal(t, "New error", modified.ErrorNote())
	})

	t.Run("can set empty error note", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithErrorNote("Some error"))
		modified := original.SetErrorNote("")

		assert.Equal(t, "Some error", original.ErrorNote())
		assert.Equal(t, "", modified.ErrorNote())
	})
}

func TestIntegratorDetails_ChainedSetters(t *testing.T) {
	t.Run("can chain Set operations", func(t *testing.T) {
		d := NewIntegratorDetails().
			Set("key1", "value1").
			Set("key2", "value2").
			Set("key3", "value3")

		assert.Equal(t, 3, len(d.Data()))
		assert.Equal(t, "value1", d.Get("key1"))
		assert.Equal(t, "value2", d.Get("key2"))
		assert.Equal(t, "value3", d.Get("key3"))
	})

	t.Run("can chain SetData and Set operations", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithData(map[string]any{"initial": "data"})).
			Set("key1", "value1").
			SetData(map[string]any{"replaced": "data"}).
			Set("key2", "value2")

		assert.Equal(t, 2, len(d.Data()))
		assert.Equal(t, "data", d.Get("replaced"))
		assert.Equal(t, "value2", d.Get("key2"))
		assert.Nil(t, d.Get("initial"))
		assert.Nil(t, d.Get("key1"))
	})

	t.Run("can chain all setters", func(t *testing.T) {
		d := NewIntegratorDetails().
			Set("key", "value").
			SetErrorCode(404).
			SetErrorNote("Not found").
			SetData(map[string]any{"new": "data"})

		assert.Equal(t, 1, len(d.Data()))
		assert.Equal(t, "data", d.Get("new"))
		assert.Nil(t, d.Get("key"))
		assert.Equal(t, int32(404), d.ErrorCode())
		assert.Equal(t, "Not found", d.ErrorNote())
	})
}

func TestIntegratorDetails_Immutability(t *testing.T) {
	t.Run("multiple modifications don't affect original", func(t *testing.T) {
		original := NewIntegratorDetails(
			IntegratorWithData(map[string]any{"key": "original"}),
			IntegratorWithErrorCode(200),
			IntegratorWithErrorNote("OK"),
		)

		modified1 := original.Set("key2", "value2")
		modified2 := original.SetErrorCode(404)
		modified3 := original.SetErrorNote("Error")

		// Original remains unchanged
		assert.Equal(t, 1, len(original.Data()))
		assert.Equal(t, "original", original.Get("key"))
		assert.Equal(t, int32(200), original.ErrorCode())
		assert.Equal(t, "OK", original.ErrorNote())

		// Each modification is independent
		assert.Equal(t, 2, len(modified1.Data()))
		assert.Equal(t, "value2", modified1.Get("key2"))
		assert.Equal(t, int32(200), modified1.ErrorCode())

		assert.Equal(t, int32(404), modified2.ErrorCode())
		assert.Equal(t, "OK", modified2.ErrorNote())

		assert.Equal(t, "Error", modified3.ErrorNote())
		assert.Equal(t, int32(200), modified3.ErrorCode())
	})

	t.Run("deep copy in Set prevents mutation", func(t *testing.T) {
		original := NewIntegratorDetails(IntegratorWithData(map[string]any{"key1": "value1"}))
		modified := original.Set("key2", "value2")

		// Modifying original's data map should not affect modified
		originalData := original.Data()
		originalData["key3"] = "value3"

		assert.Equal(t, 2, len(modified.Data()))
		assert.Nil(t, modified.Get("key3"))
	})
}

func TestIntegratorDetails_EdgeCases(t *testing.T) {
	t.Run("empty string key", func(t *testing.T) {
		d := NewIntegratorDetails().Set("", "empty-key-value")
		assert.Equal(t, "empty-key-value", d.Get(""))
	})

	t.Run("special characters in key", func(t *testing.T) {
		d := NewIntegratorDetails().
			Set("key.with.dots", "value1").
			Set("key-with-dashes", "value2").
			Set("key_with_underscores", "value3").
			Set("key with spaces", "value4")

		assert.Equal(t, "value1", d.Get("key.with.dots"))
		assert.Equal(t, "value2", d.Get("key-with-dashes"))
		assert.Equal(t, "value3", d.Get("key_with_underscores"))
		assert.Equal(t, "value4", d.Get("key with spaces"))
	})

	t.Run("large number of keys", func(t *testing.T) {
		d := NewIntegratorDetails()
		for i := 0; i < 100; i++ {
			key := string(rune('a' + i%26))
			d = d.Set(key, i)
		}
		assert.NotEmpty(t, d.Data())
	})

	t.Run("negative error code", func(t *testing.T) {
		d := NewIntegratorDetails(IntegratorWithErrorCode(-1))
		assert.Equal(t, int32(-1), d.ErrorCode())
	})

	t.Run("large error code", func(t *testing.T) {
		largeCode := int32(2147483647) // max int32
		d := NewIntegratorDetails(IntegratorWithErrorCode(largeCode))
		assert.Equal(t, largeCode, d.ErrorCode())
	})

	t.Run("unicode in error note", func(t *testing.T) {
		unicode := "测试 тест اختبار 🔥"
		d := NewIntegratorDetails(IntegratorWithErrorNote(unicode))
		assert.Equal(t, unicode, d.ErrorNote())
	})

	t.Run("complex map structures", func(t *testing.T) {
		complexMap := map[string]any{
			"string": "value",
			"number": 42,
			"float":  3.14,
			"bool":   true,
			"nested": map[string]any{
				"inner": "value",
			},
			"array": []string{"a", "b", "c"},
		}

		d := NewIntegratorDetails(IntegratorWithData(complexMap))
		assert.Equal(t, complexMap, d.Data())
	})
}
