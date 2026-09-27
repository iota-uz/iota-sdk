package document

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPanelKindsCoverContractAndReferenceRealFields(t *testing.T) {
	t.Parallel()

	kinds := PanelKinds()
	require.Len(t, kinds, 19)
	panelType := reflect.TypeOf(Panel{})
	jsonFields := make(map[string]struct{}, panelType.NumField())
	for index := 0; index < panelType.NumField(); index++ {
		name := panelType.Field(index).Tag.Get("json")
		for position, character := range name {
			if character == ',' {
				name = name[:position]
				break
			}
		}
		jsonFields[name] = struct{}{}
	}
	for kind, metadata := range kinds {
		require.NotEmpty(t, kind)
		require.NotEmpty(t, metadata.Semantics)
		for _, field := range metadata.ConfigFields {
			_, ok := jsonFields[field]
			require.Truef(t, ok, "%s references missing Panel field %s", kind, field)
		}
	}
}

func TestPanelKindsReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()

	kinds := PanelKinds()
	metadata := kinds[PanelKindTable]
	metadata.ConfigFields[0] = "changed"
	kinds[PanelKindTable] = metadata
	require.Equal(t, []string{"columns", "table"}, PanelKinds()[PanelKindTable].ConfigFields)
}
