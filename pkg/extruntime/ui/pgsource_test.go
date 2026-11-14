package ui_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	crmpermissions "github.com/iota-uz/iota-sdk/modules/crm/permissions"
	crmcontrollers "github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	crmservices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/listext"
	"github.com/iota-uz/iota-sdk/pkg/listext/listexttest"
)

type fakeManifests struct {
	rev  string
	pkgs []ui.ActivePackage
	n    int
}

func (m *fakeManifests) ActivePackages(context.Context) (string, []ui.ActivePackage, error) {
	m.n++
	return m.rev, m.pkgs, nil
}

func TestPGSourceReadsOpenBindingsOnly(t *testing.T) {
	e := itf.Setup(t, itf.WithComponents(modules.Components()...))
	ctx := e.Ctx
	insert := func(kind, target, export string) string {
		var id string
		require.NoError(t, e.Tx.QueryRow(ctx, `INSERT INTO extruntime.bindings (slot_kind, target_id, package, version, export)
			VALUES ($1, $2, '@acme/shell', 'v2', $3) RETURNING id::text`, kind, target, export).Scan(&id))
		return id
	}
	shell := insert("shell", "core.shell", "shell")
	screen := insert("screen", "crm.routes.clients", "clients")
	insert("command", "acme.insurance.renew", "renew") // commands are not UI business

	mans := &fakeManifests{rev: "m1", pkgs: []ui.ActivePackage{shellPkg()}}
	src := ui.NewPGSource(e.Tx, mans, 0)
	snap, err := src.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, snap.Bindings, 2)
	byKind := map[string]ui.Binding{}
	for _, b := range snap.Bindings {
		byKind[b.SlotKind] = b
	}
	assert.Equal(t, shell, byKind[ui.SlotShell].ID)
	assert.Equal(t, "core.shell", byKind[ui.SlotShell].TargetID)
	assert.Equal(t, screen, byKind[ui.SlotScreen].ID)
	assert.Len(t, snap.Packages, 1)
	rev1 := snap.Revision

	// closing the shell binding (admin revert) is visible on the next read
	_, err = e.Tx.Exec(ctx, `UPDATE extruntime.bindings SET active_to = CURRENT_TIMESTAMP WHERE id = $1::uuid`, shell)
	require.NoError(t, err)
	snap, err = src.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, snap.Bindings, 1)
	assert.Equal(t, ui.SlotScreen, snap.Bindings[0].SlotKind)
	assert.NotEqual(t, rev1, snap.Revision, "the revision follows the bindings")

	// the snapshot is cached for the TTL and can be invalidated
	cached := ui.NewPGSource(e.Tx, mans, time.Hour)
	_, err = cached.Snapshot(ctx)
	require.NoError(t, err)
	_, err = cached.Snapshot(ctx)
	require.NoError(t, err)
	n := mans.n
	_, err = cached.Snapshot(ctx)
	require.NoError(t, err)
	assert.Equal(t, n, mans.n, "no reread within the TTL")
	cached.Invalidate()
	_, err = cached.Snapshot(ctx)
	require.NoError(t, err)
	assert.Equal(t, n+1, mans.n)
}

func TestColumnResourcesBecomeListContributionsInOrder(t *testing.T) {
	col := func(id, field string) manifest.Resource {
		return manifest.Resource{ID: id, Kind: manifest.ResColumn, List: "crm.lists.clients", Overlay: listexttest.OverlayID, Field: field,
			Label: manifest.Label{"en": field}, Filterable: true, Sortable: true, Exportable: id != "acme.insurance.hidden"}
	}
	src := &fakeSource{}
	src.set([]ui.ActivePackage{
		{Package: "@b/second", Version: "v1", ArtifactHash: "hhhhhhhh", Manifest: manifest.Manifest{Resources: []manifest.Resource{col("b.second.tier", "loyaltyTier")}}},
		pkgV1(col("acme.insurance.tier", "loyaltyTier"), col("acme.insurance.score", "score"),
			manifest.Resource{ID: "acme.insurance.other", Kind: manifest.ResColumn, List: "other.lists.x", Overlay: "o", Field: "f"},
			manifest.Resource{ID: "acme.insurance.broken", Kind: manifest.ResColumn, List: "crm.lists.clients"}),
	})
	rt, err := ui.NewRuntime(ui.Config{Source: src, Artifacts: fakeArtifacts{}, Host: hostManifest()})
	require.NoError(t, err)
	got, err := rt.Columns().Columns(context.Background(), "crm.lists.clients")
	require.NoError(t, err)
	ids := make([]string, 0, len(got))
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"acme.insurance.tier", "acme.insurance.score", "b.second.tier"}, ids, "packages in name order, resources in manifest order; incomplete declarations are dropped")
	assert.Equal(t, listext.Contribution{ID: "acme.insurance.tier", Package: pkgName, Version: "v1", Label: manifest.Label{"en": "loyaltyTier"},
		Overlay: listexttest.OverlayID, Field: "loyaltyTier", Filterable: true, Sortable: true, Exportable: true}, got[0])
	none, err := rt.Columns().Columns(context.Background(), "unknown.list")
	require.NoError(t, err)
	assert.Empty(t, none)
}

// The activated column resources of a package reach the real CRM clients
// list: through the runtime's column source, the typed data store and the
// list's repository query.
func TestActivatedColumnResourceAppearsInTheCRMClientsList(t *testing.T) {
	suite := itf.NewSuite(t, modules.Components()...)
	env := suite.Env()
	fx := listexttest.Install(t, env.Ctx, env.Tx, "crm.lists.clients", listexttest.Manifest(nil), true)

	src := &fakeSource{}
	src.set([]ui.ActivePackage{pkgV1(manifest.Resource{ID: listexttest.TierCol, Kind: manifest.ResColumn, List: "crm.lists.clients",
		Overlay: listexttest.OverlayID, Field: "loyaltyTier", Label: manifest.Label{"en": "Loyalty tier"}, Filterable: true, Sortable: true, Exportable: true})})
	rt, err := ui.NewRuntime(ui.Config{Source: src, Artifacts: fakeArtifacts{}, Host: hostManifest()})
	require.NoError(t, err)
	install := rt.Installer(fx.Store)

	clientService := itf.GetService[crmservices.ClientService](env)
	chatService := itf.GetService[crmservices.ChatService](env)
	c := crmcontrollers.NewClientController(env.App, clientService, chatService, nil, crmcontrollers.ClientControllerConfig{BasePath: "/crm/clients"})
	suite.AsUser(itf.User(crmpermissions.ClientRead)).
		WithMiddleware(func(ctx context.Context, _ *http.Request) context.Context { return install(ctx) }).
		Register(c)

	var gold, plain int64
	require.NoError(t, env.Tx.QueryRow(env.Ctx, `INSERT INTO clients (tenant_id, first_name, last_name) VALUES ($1,'Ann','Doe') RETURNING id`, env.Tenant.ID).Scan(&gold))
	require.NoError(t, env.Tx.QueryRow(env.Ctx, `INSERT INTO clients (tenant_id, first_name, last_name) VALUES ($1,'Bob','Doe') RETURNING id`, env.Tenant.ID).Scan(&plain))
	fx.Put(t, env.Ctx, env.Tx, spi.Subject{TenantID: env.Tenant.ID, UserID: 1, Permissions: []string{"Client.Read"}}, fmt.Sprint(gold), map[string]any{"loyaltyTier": "gold"})

	body := suite.GET("/crm/clients").Expect(t).Status(200).Body()
	assert.Contains(t, body, "Loyalty tier")
	assert.Contains(t, body, ">gold<")
	filtered := suite.GET("/crm/clients?ext." + listexttest.TierCol + "=gold").HTMX().HTMXTarget("clients-table-body").Expect(t).Status(200).Body()
	assert.Contains(t, filtered, "Ann Doe")
	assert.NotContains(t, filtered, "Bob Doe")

	// deactivating the package removes the column without touching the list code
	src.set(nil)
	body = suite.GET("/crm/clients").Expect(t).Status(200).Body()
	assert.NotContains(t, body, "Loyalty tier")
	assert.Contains(t, body, "Bob Doe")
}

// The component composes into a real application: its controller passes the
// route collision validation next to every module, and its middleware and
// spotlight provider are contributed.
func TestComponentComposesIntoAnApplication(t *testing.T) {
	src := &fakeSource{}
	src.set([]ui.ActivePackage{pkgV1()})
	comp, err := ui.NewComponent(ui.Config{Source: src, Artifacts: fakeArtifacts{}, Host: hostManifest()}, nil)
	require.NoError(t, err)
	env := itf.Setup(t, itf.WithComponents(append(modules.Components(), comp)...))

	var found bool
	for _, c := range env.App.Controllers() {
		if c.Descriptor().ID == ui.ControllerID {
			found = true
			assert.Equal(t, "/_ext/rpc", c.Descriptor().Routes[1].Path)
		}
	}
	assert.True(t, found, "the page host is a controller of the application")
	assert.NotEmpty(t, env.App.Middleware())
	var palette bool
	for _, p := range env.Container.SpotlightProviders() {
		palette = palette || p.ProviderID() == "extruntime.palette"
	}
	assert.True(t, palette)

	_, err = ui.NewComponent(ui.Config{}, nil)
	require.Error(t, err, "a component without a source is refused")
}

func TestPGSourceServesNoStaleSnapshotAfterAGenerationChange(t *testing.T) {
	e := itf.Setup(t, itf.WithComponents(modules.Components()...))
	ctx := e.Ctx
	mans := &fakeManifests{rev: "m1", pkgs: []ui.ActivePackage{shellPkg()}}
	var gen uint64
	src := ui.NewPGSource(e.Tx, mans, time.Hour).WithGeneration(func() uint64 { return gen })

	before, err := src.Snapshot(ctx)
	require.NoError(t, err)
	require.Empty(t, before.Bindings)

	_, err = e.Tx.Exec(ctx, `INSERT INTO extruntime.bindings (slot_kind, target_id, package, version, export)
		VALUES ('shell', 'app', '@acme/shell', 'v2', 'shell')`)
	require.NoError(t, err)
	cached, err := src.Snapshot(ctx)
	require.NoError(t, err)
	require.Empty(t, cached.Bindings, "within the TTL and the same generation the cache is served")

	gen++ // a binding or activation changed (catalog invalidation)
	after, err := src.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, after.Bindings, 1, "a new generation is read through, whatever the TTL")
}
