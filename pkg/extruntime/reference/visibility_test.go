package reference_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

func TestExtensionsReadSeesEverything(t *testing.T) {
	t.Parallel()
	m, err := newBuilder(t).Model(context.Background(), admin(), "")
	require.NoError(t, err)
	require.True(t, m.Full)
	require.Equal(t, []string{"acme.loyalty", "core", "crm"}, namespaceNames(m))
	ids := itemIDs(m)
	for _, want := range []string{
		"acme.loyalty.card", "acme.loyalty.clientExtra", "acme.loyalty.award", "acme.loyalty.settle", "acme.loyalty.tiers",
		"acme.loyalty.awarded.v1", "acme.loyalty.campaign", "core.user", "crm.client", "crm.operations.addNote", "crm.clients",
	} {
		require.Contains(t, ids, want)
	}
	ns, _ := m.Namespace("acme.loyalty")
	require.Len(t, ns.Endpoints, 2)
	require.Len(t, ns.Schedules, 1)
	require.Len(t, ns.Subscriptions, 1)
	require.Len(t, ns.Guards, 1)
}

func TestReaderWithoutPermissionsSeesOnlyOpenResources(t *testing.T) {
	t.Parallel()
	m, err := newBuilder(t).Model(context.Background(), reader("en"), "")
	require.NoError(t, err)
	require.False(t, m.Full)
	// no permission gates: the query, the event, the notice, the domain error,
	// the route and the declared event; entities and gated resources are hidden
	require.ElementsMatch(t, []string{
		"acme.loyalty.notRegistered", "acme.loyalty.tiers", "acme.loyalty.awarded.v1", "acme.loyalty.reminder.v1",
		"crm.queries.openNotes", "crm.client.noted.v1", "crm.clients",
	}, itemIDs(m))
	require.Equal(t, []string{"acme.loyalty", "crm"}, namespaceNames(m), "core holds only a gated entity and disappears")
	ns, _ := m.Namespace("acme.loyalty")
	require.Empty(t, ns.Endpoints, "package endpoints are for integrators: Extensions.Read only")
	require.Empty(t, ns.Schedules)
	require.Empty(t, ns.Subscriptions)
	require.Empty(t, ns.Guards)
	for _, it := range ns.Items {
		require.Empty(t, it.Subscribers, "subscriptions are operational and stay with Extensions.Read")
	}
}

func TestGatedResourcesAppearWithTheirPermission(t *testing.T) {
	t.Parallel()
	m, err := newBuilder(t).Model(context.Background(), reader("en", "Loyalty.Manage"), "")
	require.NoError(t, err)
	ids := itemIDs(m)
	require.Contains(t, ids, "acme.loyalty.award")
	require.Contains(t, ids, "acme.loyalty.settle")
	require.Contains(t, ids, "acme.loyalty.campaign")
	require.NotContains(t, ids, "acme.loyalty.card", "the entity has its own read rule")
	require.NotContains(t, ids, "crm.operations.addNote")
	award, _ := m.Lookup("acme.loyalty.award")
	require.Empty(t, award.Guards, "which guards other packages put on a target stays with Extensions.Read")

	m, err = newBuilder(t).Model(context.Background(), admin(), "")
	require.NoError(t, err)
	award, _ = m.Lookup("acme.loyalty.award")
	require.Len(t, award.Guards, 1)
}

func TestEntityNeedsAReadRuleAndAnOverlayNeedsItsBase(t *testing.T) {
	t.Parallel()
	b := newBuilder(t)
	m, err := b.Model(context.Background(), reader("en", "Loyalty.ReadCards"), "")
	require.NoError(t, err)
	require.Contains(t, itemIDs(m), "acme.loyalty.card")
	require.Contains(t, itemIDs(m), "acme.loyalty.cards", "the page is gated by the same permission")
	require.NotContains(t, itemIDs(m), "acme.loyalty.clientExtra", "the base entity crm.client needs Client.Read")
	require.NotContains(t, itemIDs(m), "crm.client")

	m, err = b.Model(context.Background(), reader("en", "Client.Read"), "")
	require.NoError(t, err)
	require.Contains(t, itemIDs(m), "crm.client")
	require.Contains(t, itemIDs(m), "acme.loyalty.clientExtra")
	require.NotContains(t, itemIDs(m), "acme.loyalty.card")
}

func TestFieldsNeedTheirPermissions(t *testing.T) {
	t.Parallel()
	fields := func(s spi.Subject) []string {
		m, err := newBuilder(t).Model(context.Background(), s, "")
		require.NoError(t, err)
		card, ok := m.Lookup("acme.loyalty.card")
		require.True(t, ok)
		var ids []string
		for _, f := range card.Fields {
			ids = append(ids, f.ID)
		}
		return ids
	}
	require.Equal(t, []string{"number", "balance"}, fields(reader("en", "Loyalty.ReadCards")))
	require.Equal(t, []string{"number", "balance", "secretNote"}, fields(reader("en", "Loyalty.ReadCards", "Loyalty.Audit")))
	require.Equal(t, []string{"number", "balance", "secretNote"}, fields(admin()))
}

func TestVisibleRule(t *testing.T) {
	t.Parallel()
	lookup := func(id string) (manifest.Resource, bool) {
		if id == "base" {
			return manifest.Resource{ID: "base", Kind: manifest.ResEntity, Access: &manifest.Access{Read: []manifest.ReadRule{{Permission: "B.Read", All: true}}}}, true
		}
		return manifest.Resource{}, false
	}
	filtered := &manifest.Access{Read: []manifest.ReadRule{{Permission: "F.Read", Rows: &manifest.Filter{}}}}
	denyAll := &manifest.Access{Read: []manifest.ReadRule{{Permission: "", All: false}}}
	for _, tc := range []struct {
		name string
		res  manifest.Resource
		who  spi.Subject
		want bool
	}{
		{"open command", manifest.Resource{Kind: manifest.ResCommand}, reader("en"), true},
		{"gated command, held", manifest.Resource{Kind: manifest.ResCommand, Permission: "X.Do"}, reader("en", "X.Do"), true},
		{"gated command, missing", manifest.Resource{Kind: manifest.ResCommand, Permission: "X.Do"}, reader("en", "X.Other"), false},
		{"gated command, admin", manifest.Resource{Kind: manifest.ResCommand, Permission: "X.Do"}, admin(), true},
		{"entity without rules", manifest.Resource{Kind: manifest.ResEntity}, reader("en"), true},
		{"entity with a filtered rule, held", manifest.Resource{Kind: manifest.ResEntity, Access: filtered}, reader("en", "F.Read"), true},
		{"entity with a filtered rule, missing", manifest.Resource{Kind: manifest.ResEntity, Access: filtered}, reader("en"), false},
		{"entity whose only rule grants no rows", manifest.Resource{Kind: manifest.ResEntity, Access: denyAll}, reader("en"), false},
		{"entity denied, admin", manifest.Resource{Kind: manifest.ResEntity, Access: denyAll}, admin(), true},
		{"overlay, base held", manifest.Resource{Kind: manifest.ResOverlay, Base: "base"}, reader("en", "B.Read"), true},
		{"overlay, base missing", manifest.Resource{Kind: manifest.ResOverlay, Base: "base"}, reader("en"), false},
		{"overlay, unknown base", manifest.Resource{Kind: manifest.ResOverlay, Base: "gone"}, reader("en"), false},
		{"overlay, own rule missing", manifest.Resource{Kind: manifest.ResOverlay, Base: "base", Access: filtered}, reader("en", "B.Read"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, reference.Visible(tc.who, tc.res, lookup))
		})
	}
}

func TestEndpointVisibility(t *testing.T) {
	t.Parallel()
	public, private := reference.EndpointDoc{Public: true}, reference.EndpointDoc{}
	for _, e := range []reference.EndpointDoc{public, private} {
		require.False(t, reference.EndpointVisible(spi.Subject{}, e), "no session")
		require.False(t, reference.EndpointVisible(reader("en"), e), "a signed-in user does not integrate")
		require.True(t, reference.EndpointVisible(admin(), e))
	}
}

func TestNamespacesLeftEmptyAreHidden(t *testing.T) {
	t.Parallel()
	b := reference.New(fakeSource{natives: nativeSet(t)})
	m, err := b.Model(context.Background(), reader("en"), "")
	require.NoError(t, err)
	require.Equal(t, []string{"crm"}, namespaceNames(m), "core has only user, which needs User.Read")
	m, err = b.Model(context.Background(), reader("en", "User.Read"), "")
	require.NoError(t, err)
	require.Equal(t, []string{"core", "crm"}, namespaceNames(m))
}

func TestColumnOfAnOverlayFieldNeedsTheFieldPermission(t *testing.T) {
	t.Parallel()
	overlay := manifest.Resource{
		ID: "acme.x.extra", Kind: manifest.ResOverlay, Base: "base",
		Access: &manifest.Access{Fields: map[string][]string{"note": {"X.Note"}}},
	}
	lookup := func(id string) (manifest.Resource, bool) { return overlay, id == overlay.ID }
	column := func(field string) manifest.Resource {
		return manifest.Resource{ID: "acme.x.col", Kind: manifest.ResColumn, Overlay: overlay.ID, Field: field}
	}
	require.True(t, reference.Visible(reader("en"), column("tier"), lookup), "an open field")
	require.False(t, reference.Visible(reader("en"), column("note"), lookup), "the column would name a field the reader cannot read")
	require.True(t, reference.Visible(reader("en", "X.Note"), column("note"), lookup))
	require.True(t, reference.Visible(admin(), column("note"), lookup))
}
