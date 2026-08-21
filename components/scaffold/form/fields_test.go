package form

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func renderField(t *testing.T, field Field) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, field.Component().Render(context.Background(), &buf))
	return buf.String()
}

func TestDateField_Component_RendersValue(t *testing.T) {
	t.Parallel()

	field := Date("birth_date", "Birth date").
		Default(time.Date(2025, 7, 3, 16, 15, 48, 0, time.UTC)).
		Build()

	require.Contains(t, renderField(t, field), `value="2025-07-03"`)
}

func TestDateTimeLocalField_Component_RendersValue(t *testing.T) {
	t.Parallel()

	field := DateTime("started_at", "Started at").
		Default(time.Date(2025, 7, 3, 16, 15, 48, 0, time.UTC)).
		Build()

	require.Contains(t, renderField(t, field), `value="2025-07-03T16:15"`)
}

func TestDateTimeLocalField_Component_OmitsZeroValue(t *testing.T) {
	t.Parallel()

	require.NotContains(t, renderField(t, DateTime("started_at", "Started at").Build()), "value=")
}
