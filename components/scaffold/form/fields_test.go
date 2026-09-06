package form

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderField(t *testing.T, field Field) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, field.Component().Render(context.Background(), &buf))
	return buf.String()
}

func TestDateField_Component_Value(t *testing.T) {
	t.Parallel()

	defaultVal := time.Date(2025, 7, 3, 16, 15, 48, 0, time.UTC)
	storedVal := time.Date(2024, 11, 27, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		field Field
		want  string
	}{
		{
			name:  "DefaultRendered",
			field: Date("birth_date", "Birth date").Default(defaultVal).Build(),
			want:  `value="2025-07-03"`,
		},
		{
			name:  "StoredValueRendered",
			field: Date("birth_date", "Birth date").Build().WithValue(storedVal),
			want:  `value="2024-11-27"`,
		},
		{
			name:  "StoredValueOverridesDefault",
			field: Date("birth_date", "Birth date").Default(defaultVal).Build().WithValue(storedVal),
			want:  `value="2024-11-27"`,
		},
		{
			name:  "ZeroValueOmitted",
			field: Date("birth_date", "Birth date").Build(),
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered := renderField(t, tt.field)
			if tt.want == "" {
				assert.NotContains(t, rendered, "value=")
				return
			}
			assert.Contains(t, rendered, tt.want)
		})
	}
}

func TestDateTimeLocalField_Component_Value(t *testing.T) {
	t.Parallel()

	defaultVal := time.Date(2025, 7, 3, 16, 15, 48, 0, time.UTC)
	storedVal := time.Date(2024, 11, 27, 9, 30, 0, 0, time.UTC)

	tests := []struct {
		name  string
		field Field
		want  string
	}{
		{
			name:  "DefaultRendered",
			field: DateTime("started_at", "Started at").Default(defaultVal).Build(),
			want:  `value="2025-07-03T16:15"`,
		},
		{
			name:  "StoredValueRendered",
			field: DateTime("started_at", "Started at").Build().WithValue(storedVal),
			want:  `value="2024-11-27T09:30"`,
		},
		{
			name:  "StoredValueOverridesDefault",
			field: DateTime("started_at", "Started at").Default(defaultVal).Build().WithValue(storedVal),
			want:  `value="2024-11-27T09:30"`,
		},
		{
			name:  "ZeroValueOmitted",
			field: DateTime("started_at", "Started at").Build(),
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered := renderField(t, tt.field)
			if tt.want == "" {
				assert.NotContains(t, rendered, "value=")
				return
			}
			assert.Contains(t, rendered, tt.want)
		})
	}
}
