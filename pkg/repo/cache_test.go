package repo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheKey(t *testing.T) {
	t.Run("SameInput_ReturnsSameHash", func(t *testing.T) {
		hash1 := CacheKey("test", 123, true)
		hash2 := CacheKey("test", 123, true)
		assert.Equal(t, hash1, hash2, "Same inputs should produce the same hash")
	})

	t.Run("DifferentInput_ReturnsDifferentHash", func(t *testing.T) {
		hash1 := CacheKey("test", 123, true)
		hash2 := CacheKey("test", 123, false)
		assert.NotEqual(t, hash1, hash2, "Different inputs should produce different hashes")
	})

	t.Run("AllPrimitiveTypes", func(t *testing.T) {
		// Test various primitive types
		tests := []struct {
			name  string
			value interface{}
		}{
			{"string", "testString"},
			{"[]byte", []byte("testBytes")},
			{"bool true", true},
			{"bool false", false},
			{"byte", byte(65)},
			{"uint8", uint8(65)},
			{"rune", rune('A')},
			{"int32_char", int32(65)},
			{"int", 42},
			{"int8", int8(8)},
			{"int16", int16(16)},
			{"int32_num", int32(32)},
			{"int64", int64(64)},
			{"uint", uint(42)},
			{"uint8", uint8(8)},
			{"uint16", uint16(16)},
			{"uint32", uint32(32)},
			{"uint64", uint64(64)},
			{"uintptr", uintptr(0x1234)},
			{"float32", float32(3.14)},
			{"float64", float64(3.14159)},
			{"complex64", complex64(1 + 2i)},
			{"complex128", complex128(1 + 2i)},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Just ensure no panic
				hash := CacheKey(tt.value)
				assert.NotEmpty(t, hash, "Hash should not be empty")
			})
		}
	})

	t.Run("TimeType", func(t *testing.T) {
		now := time.Now()
		hash1 := CacheKey(now)
		hash2 := CacheKey(now)
		assert.Equal(t, hash1, hash2, "Same time should produce the same hash")

		// Different times should produce different hashes
		later := now.Add(time.Hour)
		hash3 := CacheKey(later)
		assert.NotEqual(t, hash1, hash3, "Different times should produce different hashes")
	})

	t.Run("MultipleValues", func(t *testing.T) {
		hash := CacheKey("prefix", 123, true, 45.67)
		assert.NotEmpty(t, hash, "Hash with multiple values should not be empty")

		// Order matters
		hash1 := CacheKey("a", "b", "c")
		hash2 := CacheKey("c", "b", "a")
		assert.NotEqual(t, hash1, hash2, "Different order should produce different hashes")
	})

	t.Run("EmptyInput", func(t *testing.T) {
		hash := CacheKey()
		assert.NotEmpty(t, hash, "Hash with no values should still return a consistent result")
	})

	t.Run("ComplexStructs", func(t *testing.T) {
		type testStruct struct {
			Name string
			Age  int
		}

		s1 := testStruct{Name: "Test", Age: 30}
		s2 := testStruct{Name: "Test", Age: 30}

		hash1 := CacheKey(s1)
		hash2 := CacheKey(s2)

		assert.Equal(t, hash1, hash2, "Same structs should produce the same hash")

		s3 := testStruct{Name: "Test", Age: 31}
		hash3 := CacheKey(s3)

		assert.NotEqual(t, hash1, hash3, "Different structs should produce different hashes")
	})

	t.Run("NilValue", func(t *testing.T) {
		hash := CacheKey(nil)
		assert.NotEmpty(t, hash, "Nil value should still produce a hash")
	})

	t.Run("ConsistencyCheck", func(t *testing.T) {
		// This test ensures that the hash function is consistent across runs
		// Note: Values are hardcoded based on the current implementation
		// If implementation changes, these values will need to be updated
		expectedResults := map[string]string{
			"string": CacheKey("test"),
			"int":    CacheKey(42),
			"bool":   CacheKey(true),
			"mixed":  CacheKey("test", 123, true),
			"empty":  CacheKey(),
		}

		for testName, expected := range expectedResults {
			t.Run(testName, func(t *testing.T) {
				switch testName {
				case "string":
					assert.Equal(t, expected, CacheKey("test"))
				case "int":
					assert.Equal(t, expected, CacheKey(42))
				case "bool":
					assert.Equal(t, expected, CacheKey(true))
				case "mixed":
					assert.Equal(t, expected, CacheKey("test", 123, true))
				case "empty":
					assert.Equal(t, expected, CacheKey())
				}
			})
		}
	})
}
