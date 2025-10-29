package details

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCashDetails(t *testing.T) {
	t.Run("with no options", func(t *testing.T) {
		d := NewCashDetails()
		require.NotNil(t, d)
		assert.NotNil(t, d.Data())
		assert.Empty(t, d.Data())
	})

	t.Run("with data option", func(t *testing.T) {
		data := map[string]any{
			"key":   "value",
			"count": 42,
		}
		d := NewCashDetails(CashWithData(data))
		require.NotNil(t, d)
		assert.Equal(t, data, d.Data())
	})

	t.Run("with nil data option", func(t *testing.T) {
		d := NewCashDetails(CashWithData(nil))
		require.NotNil(t, d)
		assert.Nil(t, d.Data())
	})
}

func TestCashDetails_Data(t *testing.T) {
	data := map[string]any{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}
	d := NewCashDetails(CashWithData(data))
	assert.Equal(t, data, d.Data())
}

func TestCashDetails_SetData(t *testing.T) {
	t.Run("replaces data immutably", func(t *testing.T) {
		originalData := map[string]any{"key1": "value1"}
		original := NewCashDetails(CashWithData(originalData))

		newData := map[string]any{"key2": "value2"}
		modified := original.SetData(newData)

		// Verify immutability - original unchanged
		assert.Equal(t, originalData, original.Data())
		assert.Equal(t, newData, modified.Data())
		assert.NotEqual(t, original.Data(), modified.Data())
	})

	t.Run("can set nil data", func(t *testing.T) {
		original := NewCashDetails(CashWithData(map[string]any{"key": "value"}))
		modified := original.SetData(nil)

		assert.NotNil(t, original.Data())
		assert.Nil(t, modified.Data())
	})

	t.Run("can set empty map", func(t *testing.T) {
		original := NewCashDetails(CashWithData(map[string]any{"key": "value"}))
		modified := original.SetData(map[string]any{})

		assert.NotEmpty(t, original.Data())
		assert.Empty(t, modified.Data())
	})
}

func TestCashDetails_Get(t *testing.T) {
	t.Run("returns value for existing key", func(t *testing.T) {
		data := map[string]any{
			"name":   "John",
			"age":    30,
			"active": true,
		}
		d := NewCashDetails(CashWithData(data))

		assert.Equal(t, "John", d.Get("name"))
		assert.Equal(t, 30, d.Get("age"))
		assert.Equal(t, true, d.Get("active"))
	})

	t.Run("returns nil for non-existing key", func(t *testing.T) {
		d := NewCashDetails(CashWithData(map[string]any{"key": "value"}))
		assert.Nil(t, d.Get("nonexistent"))
	})

	t.Run("returns nil when data is nil", func(t *testing.T) {
		d := NewCashDetails(CashWithData(nil))
		assert.Nil(t, d.Get("any"))
	})

	t.Run("returns nil when data is empty", func(t *testing.T) {
		d := NewCashDetails()
		assert.Nil(t, d.Get("any"))
	})
}

func TestCashDetails_Set(t *testing.T) {
	t.Run("sets new key immutably", func(t *testing.T) {
		original := NewCashDetails(CashWithData(map[string]any{"key1": "value1"}))
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
		original := NewCashDetails(CashWithData(map[string]any{"key": "original"}))
		modified := original.Set("key", "updated")

		assert.Equal(t, "original", original.Get("key"))
		assert.Equal(t, "updated", modified.Get("key"))
	})

	t.Run("can set various types", func(t *testing.T) {
		d := NewCashDetails()

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
		d := NewCashDetails(CashWithData(map[string]any{"key": "value"}))
		modified := d.Set("key", nil)

		assert.Equal(t, "value", d.Get("key"))
		assert.Nil(t, modified.Get("key"))
	})

	t.Run("maintains immutability with empty data", func(t *testing.T) {
		original := NewCashDetails()
		modified := original.Set("key", "value")

		assert.Empty(t, original.Data())
		assert.Equal(t, 1, len(modified.Data()))
		assert.Equal(t, "value", modified.Get("key"))
	})
}

func TestCashDetails_ChainedSetters(t *testing.T) {
	t.Run("can chain Set operations", func(t *testing.T) {
		d := NewCashDetails().
			Set("key1", "value1").
			Set("key2", "value2").
			Set("key3", "value3")

		assert.Equal(t, 3, len(d.Data()))
		assert.Equal(t, "value1", d.Get("key1"))
		assert.Equal(t, "value2", d.Get("key2"))
		assert.Equal(t, "value3", d.Get("key3"))
	})

	t.Run("can chain SetData and Set operations", func(t *testing.T) {
		d := NewCashDetails(CashWithData(map[string]any{"initial": "data"})).
			Set("key1", "value1").
			SetData(map[string]any{"replaced": "data"}).
			Set("key2", "value2")

		assert.Equal(t, 2, len(d.Data()))
		assert.Equal(t, "data", d.Get("replaced"))
		assert.Equal(t, "value2", d.Get("key2"))
		assert.Nil(t, d.Get("initial"))
		assert.Nil(t, d.Get("key1"))
	})
}

func TestCashDetails_EdgeCases(t *testing.T) {
	t.Run("empty string key", func(t *testing.T) {
		d := NewCashDetails().Set("", "empty-key-value")
		assert.Equal(t, "empty-key-value", d.Get(""))
	})

	t.Run("special characters in key", func(t *testing.T) {
		d := NewCashDetails().
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
		d := NewCashDetails()
		for i := 0; i < 100; i++ {
			key := string(rune('a' + i%26))
			d = d.Set(key, i)
		}
		assert.NotEmpty(t, d.Data())
	})
}
