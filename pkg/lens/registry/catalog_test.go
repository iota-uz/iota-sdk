package registry

import (
	"context"
	"testing"
	"testing/fstest"

	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/stretchr/testify/require"
)

func TestCatalogFSListsAndLoadsPresetEntries(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"sales.json": &fstest.MapFile{
			Data: []byte(`{"id":"sales","title":"Sales","description":"Preset sales dashboard"}`),
		},
	}

	catalog, err := CatalogFS(fsys, "sales.json")
	require.NoError(t, err)

	entries, err := catalog.Entries(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "sales.json", entries[0].Key)
	require.Equal(t, EntrySourcePreset, entries[0].Source)
	require.True(t, entries[0].ReadOnly)
	require.Equal(t, "sales", entries[0].DocumentID)
	require.Equal(t, "Sales", entries[0].Title)

	doc, err := catalog.Load(context.Background(), "sales.json")
	require.NoError(t, err)
	require.Equal(t, lensspec.LiteralText("Sales").Resolve(""), doc.Title.Resolve(""))
}
