package ui_test

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shellext"
)

const (
	panelSlot = "acme.slots.panel"
	hashPanel = "9a9e19a9e19a9e19"
)

func slotContribution(id, target string, order int, perm string) manifest.Resource {
	return manifest.Resource{ID: id, Kind: manifest.ResSlot, Target: target, Order: order, Permission: perm, Chunk: "client/slots.js", Export: id[len(id)-1:]}
}

func panelPkg(name, ns string, res ...manifest.Resource) ui.ActivePackage {
	return ui.ActivePackage{Package: name, Version: "1.0.0", ArtifactHash: hashPanel, Manifest: manifest.Manifest{Schema: manifest.SchemaVersion, Namespace: ns, Resources: res}}
}

// panelPage is a Templ page that hosts a slot, like the CRM client view.
type panelPage struct{ props any }

func (p panelPage) Register(r *mux.Router) {
	s := r.PathPrefix("/panel").Subrouter()
	s.Use(middleware.Authorize(), middleware.RedirectNotAuthenticated(), middleware.ProvideUser(), middleware.ProvideDynamicLogo(), middleware.WithPageContext(), middleware.NavItems())
	s.HandleFunc("", func(w http.ResponseWriter, req *http.Request) {
		child := templ.ComponentFunc(func(ctx context.Context, w2 io.Writer) error {
			return shellext.Slot(ctx, panelSlot, p.props).Render(ctx, w2)
		})
		ctx := templ.WithChildren(req.Context(), child)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := layouts.Authenticated(layouts.AuthenticatedProps{BaseProps: layouts.BaseProps{Title: "Panel"}}).Render(ctx, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods(http.MethodGet)
}

var slotElementRe = regexp.MustCompile(`<div ` + ui.SlotAttr + `="([^"]*)" ` + ui.SlotConfigAttr + `="([^"]*)" hx-disable></div>`)

func slotConfig(t *testing.T, body string) (map[string]json.RawMessage, []shellext.Module) {
	t.Helper()
	m := slotElementRe.FindStringSubmatch(body)
	if m == nil {
		return nil, nil
	}
	var cfg struct {
		Props         json.RawMessage   `json:"props"`
		Contributions []shellext.Module `json:"contributions"`
	}
	require.NoError(t, json.Unmarshal([]byte(html.UnescapeString(m[2])), &cfg))
	props := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal(cfg.Props, &props))
	return props, cfg.Contributions
}

func newSlotEnv(t *testing.T, decl map[string]manifest.Resource, props any, perms ...string) *env {
	t.Helper()
	suite := itf.NewSuite(t, modules.Components()...)
	src := &fakeSource{}
	src.set(nil)
	rt, err := ui.NewRuntime(ui.Config{
		Source:    src,
		Artifacts: fakeArtifacts{},
		Catalog:   fakeCatalog(decl),
		Host:      hostManifest(),
	})
	require.NoError(t, err)
	ps := make([]permission.Permission, 0, len(perms))
	for _, p := range perms {
		ps = append(ps, perm(p))
	}
	install := rt.Installer(nil)
	suite.AsUser(itf.User(ps...)).WithMiddleware(func(ctx context.Context, _ *http.Request) context.Context { return install(ctx) })
	suite.Register(panelPage{props: props})
	return &env{suite: suite, src: src, rt: rt}
}

func ids(mods []shellext.Module) []string {
	out := make([]string, 0, len(mods))
	for _, m := range mods {
		out = append(out, m.ID)
	}
	return out
}

var declaredPanel = map[string]manifest.Resource{
	panelSlot: {ID: panelSlot, Kind: manifest.ResSlot, Props: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "clientId", Type: manifest.Type{Kind: manifest.KindInt}},
	}}},
}

func TestSlotContributionsAreOrderedAndFilteredPerContribution(t *testing.T) {
	e := newSlotEnv(t, declaredPanel, map[string]any{"clientId": 5}, "Panel.Read")
	status, body := e.get(t, "/panel")
	require.Equal(t, http.StatusOK, status)
	assert.NotContains(t, body, ui.SlotAttr, "nothing contributes")
	assert.NotContains(t, body, "/assets/runtime-host.js", "the host loads only when an island needs it")

	e.src.set([]ui.ActivePackage{
		panelPkg("@acme/zeta", "acme.zeta", slotContribution("acme.zeta.late", panelSlot, 10, "")),
		panelPkg("@acme/alpha", "acme.alpha",
			slotContribution("acme.alpha.mid", panelSlot, 10, ""),
			slotContribution("acme.alpha.first", panelSlot, -3, ""),
			slotContribution("acme.alpha.secret", panelSlot, 0, "Panel.Secret"),
			slotContribution("acme.alpha.other", "acme.slots.elsewhere", 0, "")),
	})
	status, body = e.get(t, "/panel")
	require.Equal(t, http.StatusOK, status)
	props, mods := slotConfig(t, body)
	assert.JSONEq(t, `5`, string(props["clientId"]))
	// order, then package, then ID; the permissioned one is absent for this viewer; another slot's is not here.
	assert.Equal(t, []string{"acme.alpha.first", "acme.alpha.mid", "acme.zeta.late"}, ids(mods))
	assert.NotContains(t, body, "acme.alpha.secret")
	assert.Len(t, regexp.MustCompile(`src="/assets/runtime-host.js"`).FindAllStringIndex(body, -1), 1, "the runtime host is written once")
	assert.Equal(t, "/_ext/assets/"+hashPanel+"/client/slots.js", mods[0].Module)

	privileged := newSlotEnv(t, declaredPanel, map[string]any{"clientId": 5}, "Panel.Read", "Panel.Secret")
	privileged.src.set([]ui.ActivePackage{panelPkg("@acme/alpha", "acme.alpha", slotContribution("acme.alpha.secret", panelSlot, 0, "Panel.Secret"))})
	_, body = privileged.get(t, "/panel")
	_, mods = slotConfig(t, body)
	assert.Equal(t, []string{"acme.alpha.secret"}, ids(mods))
}

func TestSlotRendersNothingUnlessTheDeclarationAdmitsIt(t *testing.T) {
	contribute := []ui.ActivePackage{panelPkg("@acme/alpha", "acme.alpha", slotContribution("acme.alpha.a", panelSlot, 0, ""))}

	// An undeclared slot: a programming error, rendered empty.
	e := newSlotEnv(t, map[string]manifest.Resource{}, map[string]any{"clientId": 5})
	e.src.set(contribute)
	_, body := e.get(t, "/panel")
	assert.NotContains(t, body, ui.SlotAttr)

	// A declaration with a permission the viewer lacks.
	guarded := map[string]manifest.Resource{panelSlot: {ID: panelSlot, Kind: manifest.ResSlot, Permission: "Panel.Admin"}}
	e = newSlotEnv(t, guarded, map[string]any{"clientId": 5})
	e.src.set(contribute)
	_, body = e.get(t, "/panel")
	assert.NotContains(t, body, ui.SlotAttr)
	e = newSlotEnv(t, guarded, map[string]any{"clientId": 5}, "Panel.Admin")
	e.src.set(contribute)
	_, body = e.get(t, "/panel")
	assert.Contains(t, body, ui.SlotAttr)

	// Props that do not match the declared type never reach a contribution.
	e = newSlotEnv(t, declaredPanel, map[string]any{"clientId": "five"})
	e.src.set(contribute)
	_, body = e.get(t, "/panel")
	assert.NotContains(t, body, ui.SlotAttr)

	// The page and the shell cannot be broken by a non-object prop either.
	e = newSlotEnv(t, declaredPanel, 7)
	e.src.set(contribute)
	status, body := e.get(t, "/panel")
	assert.Equal(t, http.StatusOK, status)
	assert.NotContains(t, body, ui.SlotAttr)
}

func TestIndexOrdersSlotsAndAttachesCellRenderers(t *testing.T) {
	column := manifest.Resource{ID: "acme.alpha.tier", Kind: manifest.ResColumn, List: "crm.lists.clients", Overlay: "acme.alpha.extra", Field: "tier"}
	cell := func(id, target string) manifest.Resource {
		return manifest.Resource{ID: id, Kind: manifest.ResCell, Target: target, Chunk: "client/cells.js", Export: "cell"}
	}
	ix := ui.BuildIndex(&ui.Snapshot{Revision: "1", Packages: []ui.ActivePackage{
		panelPkg("@acme/alpha", "acme.alpha", column, cell("acme.alpha.ownCell", "acme.alpha.tier"), slotContribution("acme.alpha.x", panelSlot, 2, "")),
		panelPkg("@acme/beta", "acme.beta", cell("acme.beta.theirCell", "acme.alpha.tier"), cell("acme.beta.stray", "acme.alpha.unknown"), slotContribution("acme.beta.y", panelSlot, 1, "")),
	}}, nil)

	assert.Equal(t, []string{"acme.beta.y", "acme.alpha.x"}, []string{ix.Slots[panelSlot][0].ID, ix.Slots[panelSlot][1].ID})
	require.Len(t, ix.Columns["crm.lists.clients"], 1)
	renderer := ix.Columns["crm.lists.clients"][0].Cell
	require.NotNil(t, renderer)
	assert.Equal(t, "acme.alpha.ownCell", renderer.ID, "the package that declares the column wins")
	assert.Equal(t, "/_ext/assets/"+hashPanel+"/client/cells.js", renderer.Module)
	assert.Equal(t, "cell", renderer.Export)
	assert.Len(t, ix.Diagnostics, 2, "the loser and the cell of an unknown column are reported, nothing blocks")
}

func TestPageForAPlatformRouteIsOnlyAReplacementCandidate(t *testing.T) {
	foreign := manifest.Resource{ID: "acme.alpha.clientView", Kind: manifest.ResPage, Path: "/crm/clients/:id", Route: "crm.routes.clientView", Chunk: "client/p.js", Export: "clientView"}
	own := manifest.Resource{ID: "acme.alpha.page", Kind: manifest.ResPage, Path: "/alpha", Chunk: "client/p.js", Export: "page"}
	snap := func(bindings ...ui.Binding) *ui.Snapshot {
		return &ui.Snapshot{Revision: "1", Packages: []ui.ActivePackage{panelPkg("@acme/alpha", "acme.alpha", foreign, own)}, Bindings: bindings}
	}
	ix := ui.BuildIndex(snap(), ui.NativeRoutes{"crm.routes.clientView": "/crm/clients/:id"})
	_, served := ix.Resolve("/crm/clients/5")
	assert.False(t, served, "a page for a route it does not own waits for a screen binding")
	_, served = ix.Resolve("/alpha")
	assert.True(t, served)

	ix = ui.BuildIndex(snap(ui.Binding{ID: "b", SlotKind: ui.SlotScreen, TargetID: "crm.routes.clientView", Package: "@acme/alpha", Version: "1.0.0", Export: "clientView"}),
		ui.NativeRoutes{"crm.routes.clientView": "/crm/clients/:id"})
	route, served := ix.Resolve("/crm/clients/5")
	require.True(t, served)
	assert.Equal(t, "acme.alpha.clientView", route.Page.ID)
	assert.Equal(t, "crm.routes.clientView", route.ReplacedRoute)
	assert.Equal(t, map[string]string{"id": "5"}, route.Params)
}
