package catalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
)

func TestNativeListsAreKitSourcesOfTheirNamespace(t *testing.T) {
	n := catalog.NewNativeSet(data.DefaultNatives())
	require.NoError(t, n.RegisterList(data.NativeClientList))
	require.NoError(t, n.RegisterList("billing.lists.invoices"))
	require.Error(t, n.RegisterList("Not A List"))

	byNS := map[string][]string{}
	for _, s := range n.Sources() {
		byNS[s.Namespace] = s.Lists
	}
	require.Equal(t, []string{"crm.lists.clients"}, byNS["crm"], "a list is declared on the namespace it extends")
	require.Equal(t, []string{"billing.lists.invoices"}, byNS["billing"], "a namespace with only a list is still a source")
	require.Empty(t, byNS["core"])
}

func TestSnapshotIsSortedAndStable(t *testing.T) {
	n := catalog.NewNativeSet(data.DefaultNatives())
	require.NoError(t, n.RegisterList(data.NativeClientList))
	require.NoError(t, n.RegisterList("billing.lists.invoices"))

	snap := n.Snapshot()
	require.Equal(t, catalog.SnapshotSchema, snap.Schema)
	names := []string{}
	for _, s := range snap.Sources {
		names = append(names, s.Namespace)
		require.NotNil(t, s.Resources)
	}
	require.Equal(t, []string{"billing", "core", "crm"}, names)

	first, err := snap.JSON()
	require.NoError(t, err)
	second, err := n.Snapshot().JSON()
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, byte('\n'), first[len(first)-1])
}
