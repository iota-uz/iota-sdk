package catalog_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

func baseSet(t *testing.T, items ...native.Item) *native.Set {
	t.Helper()
	set := native.NewSet()
	require.NoError(t, set.Add(native.Entity(data.CoreUser()), native.Entity(data.CRMClient()), native.List(data.NativeClientList)))
	require.NoError(t, set.Add(items...))
	return set
}

func TestNativeListsAreKitSourcesOfTheirNamespace(t *testing.T) {
	n := catalog.Natives(baseSet(t, native.List("billing.lists.invoices")))
	require.Error(t, native.NewSet().Add(native.List("Not A List")))

	byNS := map[string][]string{}
	for _, s := range n.Sources() {
		byNS[s.Namespace] = s.Lists
	}
	require.Equal(t, []string{"crm.lists.clients"}, byNS["crm"], "a list is declared on the namespace it extends")
	require.Equal(t, []string{"billing.lists.invoices"}, byNS["billing"], "a namespace with only a list is still a source")
	require.Empty(t, byNS["core"])
	require.True(t, n.Reserved("billing.extra"), "a namespace with only a list is native")
	require.False(t, n.Reserved("acme"))
}

func TestSnapshotIsSortedAndStable(t *testing.T) {
	n := catalog.Natives(baseSet(t, native.List("billing.lists.invoices")))

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

func TestNativeSetPublishesEveryKindInTheKit(t *testing.T) {
	op := native.Operation[noteIn, noteIn]("billing.operations.addNote", native.Spec{}, nil)
	ev := native.Event[noteIn]("billing.invoice.noted.v1", native.EventSpec{Subject: "billing.invoice"})
	slot := manifest.Resource{ID: "billing.slots.invoiceHeader", Kind: manifest.ResSlot, Props: &manifest.Type{Kind: manifest.KindObject}}
	n := catalog.Natives(baseSet(t, op, ev, native.Route("billing.invoices", "/billing"), native.Declare(slot)))

	r, ok := n.Get("billing.operations.addNote")
	require.True(t, ok)
	require.Equal(t, op.Resource().ContractHash, r.ContractHash, "the kit and the catalog hash the same contract")
	r, ok = n.Get("billing.invoices")
	require.True(t, ok)
	require.Equal(t, manifest.ResRoute, r.Kind)
	require.Equal(t, "/billing", r.Path)
	byNS := map[string][]string{}
	for _, s := range n.Sources() {
		for _, r := range s.Resources {
			byNS[s.Namespace] = append(byNS[s.Namespace], r.ID)
		}
		byNS[s.Namespace+":lists"] = s.Lists
	}
	require.Equal(t, []string{"billing.invoice.noted.v1", "billing.invoices", "billing.operations.addNote", "billing.slots.invoiceHeader"}, byNS["billing"])
	require.Equal(t, []string{data.NativeClientList}, byNS["crm:lists"])
	_, ok = n.Get(data.NativeClient)
	require.True(t, ok, "the set carries the native entities")
	_, ok = n.Get(data.NativeClientList)
	require.False(t, ok, "a list is a kit reference, not a resource")
}

func TestNewNativeSetViewsEntitiesOnly(t *testing.T) {
	n := catalog.NewNativeSet(data.DefaultNatives())
	ids := []string{}
	for _, r := range n.Resources() {
		ids = append(ids, r.ID)
	}
	require.Equal(t, []string{data.NativeUser, data.NativeClient}, ids)
	require.Empty(t, catalog.NewNativeSet(nil).Resources())
}
