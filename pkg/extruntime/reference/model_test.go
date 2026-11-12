package reference_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

func TestModelDescribesContractsWithPackageAndVersion(t *testing.T) {
	t.Parallel()
	m, err := newBuilder(t).Model(context.Background(), admin(), "en")
	require.NoError(t, err)

	award, ok := m.Lookup("acme.loyalty.award")
	require.True(t, ok)
	require.Equal(t, "acme.loyalty", award.Package)
	require.Equal(t, "1.2.0", award.Version)
	require.Equal(t, "Loyalty.Manage", award.Permission)
	require.Equal(t, "Award points", award.Label)
	require.Equal(t, []string{"acme.loyalty.notRegistered"}, award.Errors)
	require.Equal(t, "{ card: ref<acme.loyalty.card>, points: int }", award.Input.Signature)
	require.NoError(t, manifest.ValidateValue(*award.Resource().Input, award.Input.Example))
	require.Equal(t, "additionalProperties", firstKey(award.Input.Schema, "additionalProperties"))

	settle, _ := m.Lookup("acme.loyalty.settle")
	require.True(t, settle.Atomic)

	card, _ := m.Lookup("acme.loyalty.card")
	require.Equal(t, manifest.ResEntity, card.Kind)
	require.NotNil(t, card.Record)
	require.NoError(t, manifest.ValidateValue(recordType(card.Resource()), card.Record.Example))
	require.Equal(t, []string{"Loyalty.Audit"}, card.Fields[2].Permissions)
	require.Equal(t, "Number", card.Fields[0].Label)
	require.True(t, card.Fields[0].Unique)

	ev, _ := m.Lookup("acme.loyalty.awarded.v1")
	require.Equal(t, "acme.loyalty.card", ev.Subject)
	require.Equal(t, []string{"acme.loyalty.onAwarded"}, ev.Subscribers)

	notice, _ := m.Lookup("acme.loyalty.reminder.v1")
	require.Equal(t, []string{"email", "sms"}, notice.Channels, "only the channel names of the templates, never their code")

	wf, _ := m.Lookup("acme.loyalty.campaign")
	require.Len(t, wf.Signals, 1)
	require.Equal(t, "stop", wf.Signals[0].Name)

	page, _ := m.Lookup("acme.loyalty.cards")
	require.Equal(t, "Cards", page.Nav.Label)

	native, _ := m.Lookup("crm.operations.addNote")
	require.True(t, native.Native)
	require.Empty(t, native.Package)
	require.Equal(t, "Client.Update", native.Permission, "the permission of a native contract comes from its definition")

	ns, _ := m.Namespace("acme.loyalty")
	require.NotNil(t, ns.Package)
	require.Equal(t, "1.2.0", ns.Package.Version)
	hook := ns.Endpoints[len(ns.Endpoints)-1]
	require.Equal(t, "/_ext/endpoints/acme.loyalty/webhooks/stripe", hook.URL)
	require.True(t, hook.Public)
}

func recordType(r manifest.Resource) manifest.Type {
	props := []manifest.Property{{Name: "key", Type: manifest.Type{Kind: manifest.KindString}}}
	for _, f := range r.Fields {
		props = append(props, manifest.Property{Name: f.ID, Type: f.Type})
	}
	return manifest.Type{Kind: manifest.KindObject, Properties: props}
}

func firstKey(m map[string]any, key string) string {
	if _, ok := m[key]; ok {
		return key
	}
	return ""
}

func TestLabelsFollowTheReadersLocale(t *testing.T) {
	t.Parallel()
	b := newBuilder(t)
	label := func(locale, id string) string {
		m, err := b.Model(context.Background(), admin(), locale)
		require.NoError(t, err)
		it, _ := m.Lookup(id)
		return it.Label
	}
	require.Equal(t, "Начислить баллы", label("ru", "acme.loyalty.award"))
	require.Equal(t, "Начислить баллы", label("ru-RU", "acme.loyalty.award"))
	require.Equal(t, "Award points", label("uz", "acme.loyalty.award"), "no uz label: English")
	require.Equal(t, "Award points", label("", "acme.loyalty.award"))
	require.Empty(t, label("en", "acme.loyalty.tiers"), "no label at all")
	m, err := b.Model(context.Background(), reader("ru", "Loyalty.ReadCards"), "")
	require.NoError(t, err)
	card, _ := m.Lookup("acme.loyalty.card")
	require.Equal(t, "Карта", card.Label, "the subject's locale applies without an explicit one")
	require.Equal(t, "Номер", card.Fields[0].Label)
}

func TestModelNeverCarriesSecretsSourceOrExports(t *testing.T) {
	t.Parallel()
	m, err := newBuilder(t).Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	for _, forbidden := range []string{
		"STRIPE_SECRET_KEY", "api.stripe.com", "onAwardedSecretExport", "awardGuardExport", "reminderTemplates", "nightlyExport", "hookExport",
	} {
		require.NotContains(t, string(raw), forbidden)
	}
}

func TestModelIsCachedPerCatalogGenerationButFilteredPerReader(t *testing.T) {
	t.Parallel()
	b := newBuilder(t)
	a, err := b.Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	n, err := b.Model(context.Background(), reader("en"), "en")
	require.NoError(t, err)
	require.Greater(t, len(itemIDs(a)), len(itemIDs(n)))
	a2, err := b.Model(context.Background(), admin(), "en")
	require.NoError(t, err)
	require.Equal(t, itemIDs(a), itemIDs(a2))
}
