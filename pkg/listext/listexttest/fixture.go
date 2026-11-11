// Package listexttest builds a real extension data store, an activated
// overlay package and a column source for tests of native lists. It is for
// tests only.
package listexttest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/listext"
)

// Package identity of the fixture package.
const (
	Package   = "@acme/insurance"
	Version   = "v1"
	Namespace = "acme.insurance"

	OverlayID = Namespace + ".clientExtra"
	TierCol   = Namespace + ".clientTier"
	ScoreCol  = Namespace + ".clientScore"
	VipCol    = Namespace + ".clientVip"
	NotesCol  = Namespace + ".clientNotes"
	// VipPermission is required to read the vip field.
	VipPermission = "Extra.Vip"
)

// Catalog resolves the resources of activated fixture packages.
type Catalog struct {
	res map[string]spi.ResolvedResource
}

func (c *Catalog) Resolve(_ context.Context, id string) (spi.ResolvedResource, bool, error) {
	r, ok := c.res[id]
	return r, ok, nil
}

func (c *Catalog) Entities(context.Context) ([]spi.ResolvedResource, error) {
	var out []spi.ResolvedResource
	for _, r := range c.res {
		out = append(out, r)
	}
	return out, nil
}

func (c *Catalog) Guards(context.Context, string, manifest.GuardPhase) ([]spi.ResolvedGuard, error) {
	return nil, nil
}

// Source is a column source with a settable set of contributions, standing in
// for the activated manifests.
type Source struct {
	ByList map[string][]listext.Contribution
}

func (s *Source) Columns(_ context.Context, list string) ([]listext.Contribution, error) {
	return append([]listext.Contribution(nil), s.ByList[list]...), nil
}

// Fixture is an activated package with an overlay on crm.client.
type Fixture struct {
	Store   *data.Store
	Catalog *Catalog
	Source  *Source
	Engine  *listext.Engine
	Natives *data.NativeRegistry
}

func raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Manifest is the fixture package: an overlay of crm.client with a tier enum
// (default "none"), a score and a vip field readable only with VipPermission.
// overlayAccess optionally restricts which overlay rows are readable.
func Manifest(overlayAccess *manifest.Access) *manifest.Manifest {
	acc := overlayAccess
	if acc == nil {
		acc = &manifest.Access{}
	}
	if acc.Fields == nil {
		acc.Fields = map[string][]string{}
	}
	acc.Fields["vip"] = []string{VipPermission}
	return &manifest.Manifest{
		Schema: manifest.SchemaVersion, Namespace: Namespace,
		Resources: []manifest.Resource{
			{ID: OverlayID, Kind: manifest.ResOverlay, Base: data.NativeClient, Fields: []manifest.Field{
				{ID: "loyaltyTier", Type: manifest.Type{Kind: manifest.KindEnum, Values: []string{"none", "silver", "gold"}, Optional: true, Nullable: true}, Default: raw("none")},
				{ID: "score", Type: manifest.Type{Kind: manifest.KindInt, Optional: true, Nullable: true}},
				{ID: "vip", Type: manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
				{ID: "notes", Type: manifest.Type{Kind: manifest.KindString, Optional: true, Nullable: true}},
			}, Access: acc},
		},
	}
}

// Contributions are the fixture's column resources for the list.
func Contributions(list string) []listext.Contribution {
	label := func(en, ru string) manifest.Label { return manifest.Label{"en": en, "ru": ru} }
	return []listext.Contribution{
		{ID: TierCol, Package: Package, Version: Version, Label: label("Loyalty tier", "Уровень"), Overlay: OverlayID, Field: "loyaltyTier", Filterable: true, Sortable: true, Exportable: true},
		{ID: ScoreCol, Package: Package, Version: Version, Label: label("Score", "Баллы"), Overlay: OverlayID, Field: "score", Filterable: true, Sortable: true, Exportable: true},
		{ID: VipCol, Package: Package, Version: Version, Label: label("VIP", "VIP"), Overlay: OverlayID, Field: "vip", Filterable: true, Sortable: true, Exportable: true},
		{ID: NotesCol, Package: Package, Version: Version, Label: label("Notes", "Заметки"), Overlay: OverlayID, Field: "notes", Exportable: false},
	}
}

// Install applies and activates the fixture package inside tx and registers
// its column contributions for list. A nil activate leaves the source empty
// (package absent).
func Install(t *testing.T, ctx context.Context, tx pgx.Tx, list string, m *manifest.Manifest, activate bool) *Fixture {
	t.Helper()
	cat := &Catalog{res: map[string]spi.ResolvedResource{}}
	nat := data.DefaultNatives()
	store := data.NewStore(cat, nat, nil)
	src := &Source{ByList: map[string][]listext.Contribution{}}
	if activate {
		_, err := data.Prepare(ctx, tx, data.NewCompiler(nat, cat), Package, Version, m, true)
		require.NoError(t, err)
		for _, r := range m.Resources {
			cat.res[r.ID] = spi.ResolvedResource{Resource: r, Package: Package, Version: Version}
		}
		src.ByList[list] = Contributions(list)
	}
	return &Fixture{Store: store, Catalog: cat, Source: src, Natives: nat, Engine: listext.NewEngine(src, store)}
}

// Put writes the overlay row of a client as a subject holding perms.
func (f *Fixture) Put(t *testing.T, ctx context.Context, tx pgx.Tx, sub spi.Subject, clientID string, values map[string]any) {
	t.Helper()
	_, err := f.Store.PutOverlay(ctx, tx, spi.Scope{Subject: sub, Caller: spi.Caller{Package: Package, Version: Version}}, OverlayID, clientID, raw(values))
	require.NoError(t, err)
}
