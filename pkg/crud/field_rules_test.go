package crud_test

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create field based on type
func createField(name string, fieldType crud.FieldType, opts ...crud.FieldOption) crud.Field {
	switch fieldType {
	case crud.StringFieldType:
		return crud.NewStringField(name, opts...)
	case crud.IntFieldType:
		return crud.NewIntField(name, opts...)
	case crud.BoolFieldType:
		return crud.NewBoolField(name, opts...)
	case crud.FloatFieldType:
		return crud.NewFloatField(name, opts...)
	case crud.DecimalFieldType:
		return crud.NewDecimalField(name, opts...)
	case crud.DateFieldType:
		return crud.NewDateField(name, opts...)
	case crud.TimeFieldType:
		return crud.NewTimeField(name, opts...)
	case crud.DateTimeFieldType:
		return crud.NewDateTimeField(name, opts...)
	case crud.TimestampFieldType:
		return crud.NewTimestampField(name, opts...)
	case crud.UUIDFieldType:
		return crud.NewUUIDField(name, opts...)
	default:
		panic("unknown field type")
	}
}

func TestRequiredRule(t *testing.T) {
	rule := crud.RequiredRule()

	tests := []struct {
		name      string
		fieldType crud.FieldType
		value     any
		wantErr   bool
	}{
		{
			name:      "valid string value",
			fieldType: crud.StringFieldType,
			value:     "test",
			wantErr:   false,
		},
		{
			name:      "empty string should fail",
			fieldType: crud.StringFieldType,
			value:     "",
			wantErr:   true,
		},
		{
			name:      "non-empty number",
			fieldType: crud.IntFieldType,
			value:     123,
			wantErr:   false,
		},
		{
			name:      "zero number",
			fieldType: crud.IntFieldType,
			value:     0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("test", tt.fieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("nil value test with mock", func(t *testing.T) {
		mockField := &mockField{name: "test", fieldType: crud.StringFieldType}
		mockFV := &mockFieldValue{field: mockField, value: nil}
		err := rule(mockFV)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "is required")
	})
}

func TestMinLengthRule(t *testing.T) {
	rule := crud.MinLengthRule(5)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "string longer than min length",
			value:   "hello world",
			wantErr: false,
		},
		{
			name:    "string equal to min length",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "string shorter than min length",
			value:   "hi",
			wantErr: true,
		},
		{
			name:    "empty string should fail",
			value:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("test", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMinLengthRule_NonStringField(t *testing.T) {
	rule := crud.MinLengthRule(5)
	field := createField("test", crud.IntFieldType)
	fv := field.Value(123)

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min length rule only applies to string fields")
}

func TestMaxLengthRule(t *testing.T) {
	rule := crud.MaxLengthRule(5)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "string shorter than max length",
			value:   "hi",
			wantErr: false,
		},
		{
			name:    "string equal to max length",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "string longer than max length",
			value:   "hello world",
			wantErr: true,
		},
		{
			name:    "empty string should pass",
			value:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("test", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMaxLengthRule_NonStringField(t *testing.T) {
	rule := crud.MaxLengthRule(5)
	field := createField("test", crud.IntFieldType)
	fv := field.Value(123)

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max length rule only applies to string fields")
}

func TestEmailRule(t *testing.T) {
	rule := crud.EmailRule()

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "valid email",
			value:   "test@example.com",
			wantErr: false,
		},
		{
			name:    "valid email with subdomain",
			value:   "user@mail.example.com",
			wantErr: false,
		},
		{
			name:    "valid email with numbers",
			value:   "user123@example123.com",
			wantErr: false,
		},
		{
			name:    "invalid email without @",
			value:   "testexample.com",
			wantErr: true,
		},
		{
			name:    "invalid email without domain",
			value:   "test@",
			wantErr: true,
		},
		{
			name:    "invalid email without TLD",
			value:   "test@example",
			wantErr: true,
		},
		{
			name:    "empty string should pass",
			value:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("email", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEmailRule_NonStringField(t *testing.T) {
	rule := crud.EmailRule()
	field := createField("test", crud.IntFieldType)
	fv := field.Value(123)

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email rule only applies to string fields")
}

func TestPatternRule(t *testing.T) {
	rule := crud.PatternRule(`^\d{3}-\d{2}-\d{4}$`) // SSN pattern

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "valid pattern",
			value:   "123-45-6789",
			wantErr: false,
		},
		{
			name:    "invalid pattern - wrong format",
			value:   "1234567890",
			wantErr: true,
		},
		{
			name:    "invalid pattern - letters",
			value:   "abc-de-fghi",
			wantErr: true,
		},
		{
			name:    "empty string should pass",
			value:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("ssn", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPatternRule_NonStringField(t *testing.T) {
	rule := crud.PatternRule(`\d+`)
	field := createField("test", crud.IntFieldType)
	fv := field.Value(123)

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pattern rule only applies to string fields")
}

func TestMinValueRule(t *testing.T) {
	rule := crud.MinValueRule(10)

	t.Run("IntFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "int value above minimum",
				value:   15,
				wantErr: false,
			},
			{
				name:    "int value equal to minimum",
				value:   10,
				wantErr: false,
			},
			{
				name:    "int value below minimum",
				value:   5,
				wantErr: true,
			},
			{
				name:    "int32 value above minimum",
				value:   int32(15),
				wantErr: false,
			},
			{
				name:    "int64 value above minimum",
				value:   int64(15),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("age", crud.IntFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("FloatFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "float64 value above minimum",
				value:   15.5,
				wantErr: false,
			},
			{
				name:    "float64 value equal to minimum",
				value:   10.0,
				wantErr: false,
			},
			{
				name:    "float64 value below minimum",
				value:   5.5,
				wantErr: true,
			},
			{
				name:    "float32 value above minimum",
				value:   float32(15.5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("price", crud.FloatFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestMinValueRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.MinValueRule(10)
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "min value rule only applies to int and float fields")
}

func TestMaxValueRule(t *testing.T) {
	rule := crud.MaxValueRule(100)

	t.Run("IntFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "int value below maximum",
				value:   50,
				wantErr: false,
			},
			{
				name:    "int value equal to maximum",
				value:   100,
				wantErr: false,
			},
			{
				name:    "int value above maximum",
				value:   150,
				wantErr: true,
			},
			{
				name:    "int32 value below maximum",
				value:   int32(50),
				wantErr: false,
			},
			{
				name:    "int64 value below maximum",
				value:   int64(50),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("age", crud.IntFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("FloatFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "float64 value below maximum",
				value:   50.5,
				wantErr: false,
			},
			{
				name:    "float64 value equal to maximum",
				value:   100.0,
				wantErr: false,
			},
			{
				name:    "float64 value above maximum",
				value:   150.5,
				wantErr: true,
			},
			{
				name:    "float32 value below maximum",
				value:   float32(50.5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("price", crud.FloatFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestMaxValueRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.MaxValueRule(100)
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max value rule only applies to int and float fields")
}

func TestPositiveRule(t *testing.T) {
	rule := crud.PositiveRule()

	t.Run("IntFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "positive int",
				value:   5,
				wantErr: false,
			},
			{
				name:    "zero should fail",
				value:   0,
				wantErr: true,
			},
			{
				name:    "negative int should fail",
				value:   -5,
				wantErr: true,
			},
			{
				name:    "positive int32",
				value:   int32(5),
				wantErr: false,
			},
			{
				name:    "positive int64",
				value:   int64(5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("count", crud.IntFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("FloatFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "positive float64",
				value:   5.5,
				wantErr: false,
			},
			{
				name:    "zero should fail",
				value:   0.0,
				wantErr: true,
			},
			{
				name:    "negative float64 should fail",
				value:   -5.5,
				wantErr: true,
			},
			{
				name:    "positive float32",
				value:   float32(5.5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("price", crud.FloatFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestPositiveRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.PositiveRule()
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive rule only applies to int and float fields")
}

func TestNonNegativeRule(t *testing.T) {
	rule := crud.NonNegativeRule()

	t.Run("IntFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "positive int",
				value:   5,
				wantErr: false,
			},
			{
				name:    "zero should pass",
				value:   0,
				wantErr: false,
			},
			{
				name:    "negative int should fail",
				value:   -5,
				wantErr: true,
			},
			{
				name:    "positive int32",
				value:   int32(5),
				wantErr: false,
			},
			{
				name:    "positive int64",
				value:   int64(5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("count", crud.IntFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("FloatFieldType", func(t *testing.T) {
		tests := []struct {
			name    string
			value   any
			wantErr bool
		}{
			{
				name:    "positive float64",
				value:   5.5,
				wantErr: false,
			},
			{
				name:    "zero should pass",
				value:   0.0,
				wantErr: false,
			},
			{
				name:    "negative float64 should fail",
				value:   -5.5,
				wantErr: true,
			},
			{
				name:    "positive float32",
				value:   float32(5.5),
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				field := createField("balance", crud.FloatFieldType)
				fv := field.Value(tt.value)
				err := rule(fv)
				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})
}

func TestNonNegativeRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.NonNegativeRule()
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "non-negative rule only applies to int and float fields")
}

func TestInRule(t *testing.T) {
	rule := crud.InRule("red", "green", "blue")

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "value in allowed list",
			value:   "red",
			wantErr: false,
		},
		{
			name:    "another value in allowed list",
			value:   "blue",
			wantErr: false,
		},
		{
			name:    "value not in allowed list",
			value:   "yellow",
			wantErr: true,
		},
		{
			name:    "case sensitive - uppercase not allowed",
			value:   "RED",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("color", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInRule_MultipleTypes(t *testing.T) {
	rule := crud.InRule(1, 2, "three", true)

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "int value in list",
			value:   1,
			wantErr: false,
		},
		{
			name:    "string value in list",
			value:   "three",
			wantErr: false,
		},
		{
			name:    "bool value in list",
			value:   true,
			wantErr: false,
		},
		{
			name:    "value not in list",
			value:   "four",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockField := &mockField{name: "mixed", fieldType: crud.StringFieldType}
			mockFV := &mockFieldValue{field: mockField, value: tt.value}
			err := rule(mockFV)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotEmptyRule(t *testing.T) {
	rule := crud.NotEmptyRule()

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{
			name:    "non-empty string",
			value:   "hello",
			wantErr: false,
		},
		{
			name:    "empty string should fail",
			value:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only should fail",
			value:   "   ",
			wantErr: true,
		},
		{
			name:    "string with content and whitespace",
			value:   "  hello  ",
			wantErr: false,
		},
		{
			name:    "tabs and newlines should fail",
			value:   "\t\n\r",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("description", crud.StringFieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNotEmptyRule_NonStringField(t *testing.T) {
	rule := crud.NotEmptyRule()
	field := createField("test", crud.IntFieldType)
	fv := field.Value(123)

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not empty rule only applies to string fields")
}

func TestFutureDateRule(t *testing.T) {
	rule := crud.FutureDateRule()
	now := time.Now()

	tests := []struct {
		name      string
		fieldType crud.FieldType
		value     any
		wantErr   bool
	}{
		{
			name:      "future date should pass",
			fieldType: crud.DateFieldType,
			value:     now.Add(24 * time.Hour),
			wantErr:   false,
		},
		{
			name:      "past date should fail",
			fieldType: crud.DateFieldType,
			value:     now.Add(-24 * time.Hour),
			wantErr:   true,
		},
		{
			name:      "near past time should fail",
			fieldType: crud.TimeFieldType,
			value:     now.Add(-time.Second),
			wantErr:   true,
		},
		{
			name:      "future datetime should pass",
			fieldType: crud.DateTimeFieldType,
			value:     now.Add(time.Hour),
			wantErr:   false,
		},
		{
			name:      "future timestamp should pass",
			fieldType: crud.TimestampFieldType,
			value:     now.Add(time.Minute),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("event_date", tt.fieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFutureDateRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.FutureDateRule()
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "future date rule only applies to date/time fields")
}

func TestPastDateRule(t *testing.T) {
	rule := crud.PastDateRule()
	now := time.Now()

	tests := []struct {
		name      string
		fieldType crud.FieldType
		value     any
		wantErr   bool
	}{
		{
			name:      "past date should pass",
			fieldType: crud.DateFieldType,
			value:     now.Add(-24 * time.Hour),
			wantErr:   false,
		},
		{
			name:      "future date should fail",
			fieldType: crud.DateFieldType,
			value:     now.Add(24 * time.Hour),
			wantErr:   true,
		},
		{
			name:      "near future time should fail",
			fieldType: crud.TimeFieldType,
			value:     now.Add(time.Second),
			wantErr:   true,
		},
		{
			name:      "past datetime should pass",
			fieldType: crud.DateTimeFieldType,
			value:     now.Add(-time.Hour),
			wantErr:   false,
		},
		{
			name:      "past timestamp should pass",
			fieldType: crud.TimestampFieldType,
			value:     now.Add(-time.Minute),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := createField("birth_date", tt.fieldType)
			fv := field.Value(tt.value)
			err := rule(fv)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPastDateRule_UnsupportedFieldType(t *testing.T) {
	rule := crud.PastDateRule()
	field := createField("test", crud.StringFieldType)
	fv := field.Value("test")

	err := rule(fv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "past date rule only applies to date/time fields")
}

func TestFieldRulesIntegration(t *testing.T) {
	t.Run("multiple rules on same field", func(t *testing.T) {
		field := createField("username", crud.StringFieldType, crud.WithRules([]crud.FieldRule{
			crud.RequiredRule(),
			crud.MinLengthRule(3),
			crud.MaxLengthRule(20),
			crud.PatternRule(`^[a-zA-Z0-9_]+$`),
		}))

		tests := []struct {
			name    string
			value   string
			wantErr bool
		}{
			{
				name:    "valid username",
				value:   "user123",
				wantErr: false,
			},
			{
				name:    "too short",
				value:   "ab",
				wantErr: true,
			},
			{
				name:    "too long",
				value:   "verylongusernamethatexceedslimit",
				wantErr: true,
			},
			{
				name:    "invalid characters",
				value:   "user@name",
				wantErr: true,
			},
			{
				name:    "empty string",
				value:   "",
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				fv := field.Value(tt.value)

				var hasError bool
				for _, rule := range field.Rules() {
					if err := rule(fv); err != nil {
						hasError = true
						break
					}
				}

				if tt.wantErr {
					assert.True(t, hasError)
				} else {
					assert.False(t, hasError)
				}
			})
		}
	})

	t.Run("numeric field with multiple rules", func(t *testing.T) {
		field := createField("age", crud.IntFieldType, crud.WithRules([]crud.FieldRule{
			crud.RequiredRule(),
			crud.MinValueRule(0),
			crud.MaxValueRule(150),
			crud.NonNegativeRule(),
		}))

		tests := []struct {
			name    string
			value   int
			wantErr bool
		}{
			{
				name:    "valid age",
				value:   25,
				wantErr: false,
			},
			{
				name:    "too young",
				value:   -1,
				wantErr: true,
			},
			{
				name:    "too old",
				value:   200,
				wantErr: true,
			},
			{
				name:    "boundary value - zero",
				value:   0,
				wantErr: false,
			},
			{
				name:    "boundary value - max",
				value:   150,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				fv := field.Value(tt.value)

				var hasError bool
				for _, rule := range field.Rules() {
					if err := rule(fv); err != nil {
						hasError = true
						break
					}
				}

				if tt.wantErr {
					assert.True(t, hasError)
				} else {
					assert.False(t, hasError)
				}
			})
		}
	})
}

// Mock types for testing edge cases
type mockField struct {
	name      string
	fieldType crud.FieldType
}

func (m *mockField) Key() bool               { return false }
func (m *mockField) Name() string            { return m.name }
func (m *mockField) Type() crud.FieldType    { return m.fieldType }
func (m *mockField) Readonly() bool          { return false }
func (m *mockField) Searchable() bool        { return false }
func (m *mockField) Hidden() bool            { return false }
func (m *mockField) Rules() []crud.FieldRule { return nil }
func (m *mockField) InitialValue() any       { return nil }
func (m *mockField) Attrs() map[string]any   { return map[string]any{} }
func (m *mockField) Value(value any) crud.FieldValue {
	return &mockFieldValue{field: m, value: value}
}
func (m *mockField) AsStringField() (crud.StringField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not string", m.name, m.fieldType)
}
func (m *mockField) AsIntField() (crud.IntField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not int", m.name, m.fieldType)
}
func (m *mockField) AsBoolField() (crud.BoolField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not bool", m.name, m.fieldType)
}
func (m *mockField) AsFloatField() (crud.FloatField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not float", m.name, m.fieldType)
}
func (m *mockField) AsDateField() (crud.DateField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not date", m.name, m.fieldType)
}
func (m *mockField) AsTimeField() (crud.TimeField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not time", m.name, m.fieldType)
}
func (m *mockField) AsDateTimeField() (crud.DateTimeField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not datetime", m.name, m.fieldType)
}
func (m *mockField) AsTimestampField() (crud.TimestampField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not timestamp", m.name, m.fieldType)
}
func (m *mockField) AsUUIDField() (crud.UUIDField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not uuid", m.name, m.fieldType)
}
func (m *mockField) AsDecimalField() (crud.DecimalField, error) {
	return nil, fmt.Errorf("field type mismatch: field %q is %s, not decimal", m.name, m.fieldType)
}

type mockFieldValue struct {
	field crud.Field
	value any
}

func (m *mockFieldValue) Field() crud.Field { return m.field }
func (m *mockFieldValue) Value() any        { return m.value }
func (m *mockFieldValue) IsZero() bool      { return m.value == nil }
func (m *mockFieldValue) AsString() (string, error) {
	if s, ok := m.value.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("value is not a string")
}
func (m *mockFieldValue) AsInt() (int, error) {
	if i, ok := m.value.(int); ok {
		return i, nil
	}
	return 0, fmt.Errorf("value is not an int")
}
func (m *mockFieldValue) AsInt32() (int32, error) {
	if i, ok := m.value.(int32); ok {
		return i, nil
	}
	return 0, fmt.Errorf("value is not an int32")
}
func (m *mockFieldValue) AsInt64() (int64, error) {
	if i, ok := m.value.(int64); ok {
		return i, nil
	}
	return 0, fmt.Errorf("value is not an int64")
}
func (m *mockFieldValue) AsBool() (bool, error) {
	if b, ok := m.value.(bool); ok {
		return b, nil
	}
	return false, fmt.Errorf("value is not a bool")
}
func (m *mockFieldValue) AsFloat32() (float32, error) {
	if f, ok := m.value.(float32); ok {
		return f, nil
	}
	return 0, fmt.Errorf("value is not a float32")
}
func (m *mockFieldValue) AsFloat64() (float64, error) {
	if f, ok := m.value.(float64); ok {
		return f, nil
	}
	return 0, fmt.Errorf("value is not a float64")
}
func (m *mockFieldValue) AsDecimal() (string, error) {
	if s, ok := m.value.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", m.value), nil
}
func (m *mockFieldValue) AsTime() (time.Time, error) {
	if t, ok := m.value.(time.Time); ok {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("value is not a time.Time")
}
func (m *mockFieldValue) AsUUID() (uuid.UUID, error) {
	if u, ok := m.value.(uuid.UUID); ok {
		return u, nil
	}
	return uuid.UUID{}, fmt.Errorf("value is not a uuid.UUID")
}
