package ui_test

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/ui"
)

const (
	utilities  = "client/tailwind.css"
	modulesCSS = "client/index-AB12CD34.css"
)

func styleURL(hash, file string) string { return "/_ext/assets/" + hash + "/" + file }

// clientManifest is the client build's own manifest (web/runtime/src/build/pipeline.ts, iota.client/1).
func clientManifestOf(css ...string) string {
	doc := struct {
		Schema    string   `json:"schema"`
		Namespace string   `json:"namespace"`
		Entry     string   `json:"entry"`
		Chunks    []string `json:"chunks"`
		CSS       []string `json:"css"`
		Shared    []string `json:"shared"`
		Exports   []string `json:"exports"`
	}{"iota.client/1", "acme.insurance", "client/index.js", []string{}, css, []string{}, []string{}}
	b, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func styledArtifacts(hash string, css ...string) fakeArtifacts {
	a := fakeArtifacts{hash + "/" + ui.ClientManifestPath: clientManifestOf(css...)}
	for _, f := range css {
		a[hash+"/"+f] = "/* " + f + " */"
	}
	return a
}

func linkTags(body string) []string {
	return regexp.MustCompile(`<link rel="stylesheet" href="[^"]*" data-iota-ext-css/?>`).FindAllString(body, -1)
}

func TestPageHTMLLinksTheVersionStylesheetsOnceBeforeTheMount(t *testing.T) {
	e := newEnvWithArtifacts(t, styledArtifacts(hashV1, utilities, modulesCSS), "Policy.Read")
	e.src.set([]ui.ActivePackage{pkgV1()})

	status, body := e.get(t, "/insurance/policies")
	require.Equal(t, http.StatusOK, status, body)
	want := []string{
		`<link rel="stylesheet" href="` + styleURL(hashV1, utilities) + `" data-iota-ext-css>`,
		`<link rel="stylesheet" href="` + styleURL(hashV1, modulesCSS) + `" data-iota-ext-css>`,
	}
	assert.Equal(t, want, linkTags(body), "each stylesheet of the version is linked once, utilities first")
	mount := strings.Index(body, `id="iota-client-route-mount"`)
	require.Positive(t, mount)
	for _, l := range want {
		i := strings.Index(body, l)
		require.GreaterOrEqual(t, i, 0)
		assert.Less(t, i, mount, "linked before the page's mount element, so the first paint is styled")
	}

	// The same URLs reach the browser host in the page data, which ensures them before the first render.
	var rc clienthost.RouteContext
	scriptJSON(t, body, "iota-client-context", &rc)
	var data ui.PageData
	require.NoError(t, json.Unmarshal(rc.Initial, &data))
	assert.Equal(t, []string{styleURL(hashV1, utilities), styleURL(hashV1, modulesCSS)}, data.Page.Styles)

	// And the stylesheets are served by the immutable asset route.
	resp := e.suite.GET(styleURL(hashV1, utilities)).Expect(t).Status(200)
	assert.Contains(t, resp.Header("Content-Type"), "text/css")
	assert.Equal(t, "private, max-age=31536000, immutable", resp.Header("Cache-Control"))
	assert.Equal(t, "/* "+utilities+" */", resp.Body())
}

func TestVersionWithoutStylesheetsLinksNothing(t *testing.T) {
	for name, artifacts := range map[string]fakeArtifacts{
		"no client manifest":    {},
		"no css in manifest":    {hashV1 + "/" + ui.ClientManifestPath: clientManifestOf()},
		"unreadable manifest":   {hashV1 + "/" + ui.ClientManifestPath: "{not json"},
		"escaping and foreign":  {hashV1 + "/" + ui.ClientManifestPath: clientManifestOf("../server.mjs", "/etc/x.css", "client/app.js", "other/x.css", "./client/x.css")},
		"stylesheet not served": {hashV1 + "/" + ui.ClientManifestPath: clientManifestOf("server.css")},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnvWithArtifacts(t, artifacts, "Policy.Read")
			e.src.set([]ui.ActivePackage{pkgV1()})
			status, body := e.get(t, "/insurance/policies")
			require.Equal(t, http.StatusOK, status, "the page is served whatever the manifest says")
			assert.Empty(t, linkTags(body))
			var rc clienthost.RouteContext
			scriptJSON(t, body, "iota-client-context", &rc)
			var data ui.PageData
			require.NoError(t, json.Unmarshal(rc.Initial, &data))
			assert.Empty(t, data.Page.Styles)
		})
	}
}

// countingArtifacts counts the opens of client manifests.
type countingArtifacts struct {
	fakeArtifacts
	manifests atomic.Int32
}

func (c *countingArtifacts) Open(ctx context.Context, hash, p string) (ui.ArtifactFile, error) {
	if p == ui.ClientManifestPath {
		c.manifests.Add(1)
	}
	return c.fakeArtifacts.Open(ctx, hash, p)
}

func TestStylesheetListIsReadOncePerArtifactAndAfterTheSourceRefreshes(t *testing.T) {
	e := newEnvWithArtifacts(t, styledArtifacts(hashV1, utilities), "Policy.Read")
	counting := &countingArtifacts{fakeArtifacts: styledArtifacts(hashV1, utilities)}
	rt, err := ui.NewRuntime(ui.Config{Source: e.src, Artifacts: counting, Host: hostManifest()})
	require.NoError(t, err)
	e.src.set([]ui.ActivePackage{pkgV1()})

	for range 5 {
		ix, err := rt.Index(context.Background())
		require.NoError(t, err)
		require.Len(t, ix.Pages, 3)
		assert.Equal(t, []string{styleURL(hashV1, utilities)}, ix.Pages[0].Styles)
	}
	assert.EqualValues(t, 1, counting.manifests.Load(), "an artifact is immutable: one read")

	// A new snapshot revision rebuilds the index; the artifact it names is still the same.
	e.src.set([]ui.ActivePackage{pkgV1()})
	_, err = rt.Index(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, counting.manifests.Load())
}

func TestSourceProvidedStylesAreUsedAsTheyAre(t *testing.T) {
	pkg := pkgV1()
	pkg.Styles = []string{utilities}
	e := newEnv(t, "Policy.Read") // no client manifest in the store
	e.src.set([]ui.ActivePackage{pkg})
	_, body := e.get(t, "/insurance/policies")
	assert.Equal(t, []string{`<link rel="stylesheet" href="` + styleURL(hashV1, utilities) + `" data-iota-ext-css>`}, linkTags(body))
}

func TestSlotIslandsCarryTheStylesOfTheirVersion(t *testing.T) {
	artifacts := styledArtifacts(hashPanel, utilities, modulesCSS)
	e := newSlotEnvWithArtifacts(t, artifacts, declaredPanel, map[string]any{"clientId": 5}, "Panel.Read")
	e.src.set([]ui.ActivePackage{
		panelPkg("@acme/alpha", "acme.alpha", slotContribution("acme.alpha.a", panelSlot, 1, ""), slotContribution("acme.alpha.b", panelSlot, 2, "")),
	})
	status, body := e.get(t, "/panel")
	require.Equal(t, http.StatusOK, status)
	_, mods := slotConfig(t, body)
	require.Len(t, mods, 2)
	for _, m := range mods {
		assert.Equal(t, []string{styleURL(hashPanel, utilities), styleURL(hashPanel, modulesCSS)}, m.Styles)
	}
	// Islands never write links: the browser host owns them in the document head, once, however many islands
	// (and fragments) of the version appear. The page's own HTML carries none for a slot.
	assert.Empty(t, linkTags(body))
}

func TestShellRendererCarriesTheStylesOfItsVersion(t *testing.T) {
	e := newEnvWithArtifacts(t, styledArtifacts(hashShel, utilities), "Policy.Read")
	shell := ui.Binding{ID: "s-1", SlotKind: ui.SlotShell, TargetID: "core.shell", Package: "@acme/shell", Version: "v2", Export: "shell", ActiveFrom: time.Now()}
	e.src.set([]ui.ActivePackage{pkgV1(), shellPkg()}, shell)
	_, body := e.get(t, "/legacy")
	var rc ui.RuntimeContext
	scriptJSON(t, body, ui.RuntimeContextElementID, &rc)
	require.NotNil(t, rc.Shell)
	assert.Equal(t, []string{styleURL(hashShel, utilities)}, rc.Shell.Styles)
}
