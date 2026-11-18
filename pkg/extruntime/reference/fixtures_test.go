package reference_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type fakeSource struct {
	active  []catalog.ActiveVersion
	natives *catalog.NativeSet
}

func (f fakeSource) Active(context.Context) ([]catalog.ActiveVersion, error) { return f.active, nil }
func (f fakeSource) Natives() *catalog.NativeSet                             { return f.natives }

type noteIn struct {
	Text string `json:"text"`
}

func nativeSet(t *testing.T) *catalog.NativeSet {
	t.Helper()
	set := native.NewSet()
	require.NoError(t, set.Add(
		native.Entity(data.CoreUser()), native.Entity(data.CRMClient()),
		native.Operation[noteIn, noteIn]("crm.operations.addNote", native.Spec{Permission: "Client.Update", Label: manifest.Label{"en": "Add note", "ru": "Добавить заметку"}}, nil),
		native.Query[noteIn, noteIn]("crm.queries.openNotes", native.Spec{}, nil),
		native.Event[noteIn]("crm.client.noted.v1", native.EventSpec{Subject: data.NativeClient}),
		native.Route("crm.clients", "/crm/clients"),
	))
	return catalog.Natives(set)
}

func str(optional bool) manifest.Type {
	return manifest.Type{Kind: manifest.KindString, Optional: optional}
}

// loyalty is the package under test: private data, a gated command, an open
// query, an overlay on a gated native entity, endpoints, a schedule, a
// subscription, a guard and a secret that must never be shown.
func loyalty() catalog.ActiveVersion {
	obj := func(ps ...manifest.Property) *manifest.Type {
		return &manifest.Type{Kind: manifest.KindObject, Properties: ps}
	}
	card := manifest.Type{Kind: manifest.KindRef, Entity: "acme.loyalty.card"}
	return catalog.ActiveVersion{
		VersionID: "v1", Namespace: "acme.loyalty", Version: "1.2.0",
		Manifest: manifest.Manifest{
			Schema: manifest.SchemaVersion, Namespace: "acme.loyalty",
			Resources: []manifest.Resource{
				{ID: "acme.loyalty.card", Kind: manifest.ResEntity, Label: manifest.Label{"en": "Card", "ru": "Карта"},
					Fields: []manifest.Field{
						{ID: "number", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 16}, Unique: true, Label: manifest.Label{"en": "Number", "ru": "Номер"}},
						{ID: "balance", Type: manifest.Type{Kind: manifest.KindDecimal, Precision: 12, Scale: 2}},
						{ID: "secretNote", Type: str(true)},
					},
					Access: &manifest.Access{
						Read:   []manifest.ReadRule{{Permission: "Loyalty.ReadCards", All: true}},
						Fields: map[string][]string{"secretNote": {"Loyalty.Audit"}},
					}},
				{ID: "acme.loyalty.clientExtra", Kind: manifest.ResOverlay, Base: "crm.client",
					Fields: []manifest.Field{{ID: "tier", Type: manifest.Type{Kind: manifest.KindEnum, Values: []string{"gold", "silver"}}}}},
				{ID: "acme.loyalty.notRegistered", Kind: manifest.ResDomainError, Params: obj(manifest.Property{Name: "id", Type: manifest.Type{Kind: manifest.KindUUID}})},
				{ID: "acme.loyalty.award", Kind: manifest.ResCommand, Permission: "Loyalty.Manage", Label: manifest.Label{"en": "Award points", "ru": "Начислить баллы"},
					Description: "Awards points to a card.", Errors: []string{"acme.loyalty.notRegistered"},
					Input:  obj(manifest.Property{Name: "card", Type: card}, manifest.Property{Name: "points", Type: manifest.Type{Kind: manifest.KindInt}}),
					Output: obj(manifest.Property{Name: "balance", Type: manifest.Type{Kind: manifest.KindDecimal, Precision: 12, Scale: 2}})},
				{ID: "acme.loyalty.settle", Kind: manifest.ResCommand, Atomic: true, Permission: "Loyalty.Manage",
					Input: obj(), Output: obj()},
				{ID: "acme.loyalty.tiers", Kind: manifest.ResQuery,
					Input:  obj(),
					Output: &manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindString}}},
				{ID: "acme.loyalty.awarded.v1", Kind: manifest.ResEvent, Subject: "acme.loyalty.card",
					Payload: obj(manifest.Property{Name: "points", Type: manifest.Type{Kind: manifest.KindInt}})},
				{ID: "acme.loyalty.reminder.v1", Kind: manifest.ResNotice, Payload: obj()},
				{ID: "acme.loyalty.campaign", Kind: manifest.ResWorkflow, Permission: "Loyalty.Manage", Input: obj(),
					Signals: map[string]manifest.Type{"stop": {Kind: manifest.KindObject}}},
				{ID: "acme.loyalty.cards", Kind: manifest.ResPage, Path: "/loyalty/cards", Permission: "Loyalty.ReadCards",
					Nav: &manifest.Nav{Label: manifest.Label{"en": "Cards", "ru": "Карты"}, Group: "acme.loyalty"}},
			},
			Subscriptions: []manifest.Subscription{{ID: "acme.loyalty.onAwarded", Export: "onAwardedSecretExport", Event: "acme.loyalty.awarded.v1"}},
			Guards:        []manifest.Guard{{ID: "acme.loyalty.awardGuard", Export: "awardGuardExport", Target: "acme.loyalty.award", Phase: manifest.GuardInput}},
			Templates:     []manifest.Template{{Export: "reminderTemplates", Notice: "acme.loyalty.reminder.v1", Channels: []string{"sms", "email"}}},
			Schedules:     []manifest.Schedule{{ID: "acme.loyalty.nightly", Export: "nightlyExport", Cron: "0 3 * * *", Timezone: "Asia/Tashkent"}},
			Endpoints: []manifest.Endpoint{
				{ID: "acme.loyalty.hook", Export: "hookExport", Method: "POST", Path: "/webhooks/stripe", Public: true},
				{ID: "acme.loyalty.export", Export: "exportExport", Method: "GET", Path: "/export"},
			},
			Capabilities: manifest.Capabilities{Network: []string{"api.stripe.com"}, Secrets: []string{"STRIPE_SECRET_KEY"}},
		},
	}
}

func newBuilder(t *testing.T) *reference.Builder {
	t.Helper()
	return reference.New(fakeSource{active: []catalog.ActiveVersion{loyalty()}, natives: nativeSet(t)})
}

func reader(locale string, perms ...string) spi.Subject {
	return spi.Subject{UserID: 7, Kind: spi.SubjectInitiator, Permissions: perms, Locale: locale}
}

func admin() spi.Subject { return reader("en", reference.PermRead) }

func itemIDs(m *reference.Model) []string {
	var ids []string
	for _, ns := range m.Namespaces {
		for _, it := range ns.Items {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

func namespaceNames(m *reference.Model) []string {
	names := []string{}
	for _, ns := range m.Namespaces {
		names = append(names, ns.Name)
	}
	return names
}

// renderEvil documents a package whose texts try to smuggle markup in.
func renderEvil(t *testing.T) []byte {
	t.Helper()
	av := loyalty()
	av.Manifest.Resources = append(av.Manifest.Resources, manifest.Resource{
		ID: "acme.loyalty.evil", Kind: manifest.ResQuery, Description: "<script>alert(1)</script> [click](javascript:alert(1))",
		Label: manifest.Label{"en": "<b>x</b>"}, Input: &manifest.Type{Kind: manifest.KindObject}, Output: &manifest.Type{Kind: manifest.KindObject},
	})
	b := reference.New(fakeSource{active: []catalog.ActiveVersion{av}, natives: nativeSet(t)})
	m, err := b.Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	out, ok := reference.RenderNamespace(m, "acme.loyalty")
	require.True(t, ok)
	return out
}
