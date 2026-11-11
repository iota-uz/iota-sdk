package catalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
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

type noteIn struct {
	Text string `json:"text"`
}

func TestDefaultNativesPublishGoContractsInTheKit(t *testing.T) {
	set := native.NewSet()
	op := native.Operation[noteIn, noteIn]("billing.operations.addNote", native.Spec{}, nil)
	require.NoError(t, set.Register(op))
	n, err := catalog.DefaultNatives(data.DefaultNatives(), set, nil, manifest.Resource{ID: "billing.invoices", Kind: manifest.ResRoute, Path: "/billing"})
	require.NoError(t, err)

	r, ok := n.Get("billing.operations.addNote")
	require.True(t, ok)
	require.Equal(t, op.Resource().ContractHash, r.ContractHash, "the kit and the catalog hash the same contract")
	byNS := map[string][]string{}
	for _, s := range n.Sources() {
		for _, r := range s.Resources {
			byNS[s.Namespace] = append(byNS[s.Namespace], r.ID)
		}
		byNS[s.Namespace+":lists"] = s.Lists
	}
	require.Equal(t, []string{"billing.invoices", "billing.operations.addNote"}, byNS["billing"])
	require.Equal(t, []string{data.NativeClientList}, byNS["crm:lists"], "the default native list")

	_, err = catalog.DefaultNatives(data.DefaultNatives(), set, nil, op.Resource())
	require.Error(t, err, "an ID is published once")

	all, err := catalog.Defaults()
	require.NoError(t, err)
	_, ok = all.Get(data.NativeClient)
	require.True(t, ok, "Defaults carries the native entities")
}
