package frame

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClonePreservesFieldLookups(t *testing.T) {
	t.Parallel()

	fr, err := New("sales",
		Field{Name: "label", Type: FieldTypeString, Values: []any{"OSAGO"}},
		Field{Name: "value", Type: FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)

	cloned := fr.Clone()
	field, ok := cloned.Field("label")
	require.True(t, ok)
	require.NotNil(t, field)
	require.Equal(t, "label", field.Name)
}
