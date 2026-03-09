package apex

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestBuildActionJSNormalizesTimeCategories(t *testing.T) {
	t.Parallel()

	fr, err := frame.New("sales",
		frame.Field{Name: "category", Type: frame.FieldTypeTime, Values: []any{"2026-03-09T00:00:00Z"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Revenue"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	js := string(buildActionJS(
		&action.Spec{Kind: action.KindNavigate, URL: "/reports"},
		fr,
		panel.FieldMapping{Category: "category", Series: "series", Value: "value"},
		nil,
	))

	require.Contains(t, js, "normalizeCategoryValue")
	require.Contains(t, js, "toISOString().slice(0, 10)")
}
