package react

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAssetBundleCollectsStaticAndLazyGraphStylesheets(t *testing.T) {
	manifest := map[string]manifestEntry{
		"index.html":          {File: "assets/entry.js", CSS: []string{"assets/entry.css"}, Imports: []string{"src/shared.ts"}, DynamicImports: []string{"src/lazy-chart.ts"}},
		"src/shared.ts":       {File: "assets/shared.js", CSS: []string{"assets/shared.css"}, Imports: []string{"src/cycle.ts"}},
		"src/cycle.ts":        {File: "assets/cycle.js", CSS: []string{"assets/cycle.css"}, Imports: []string{"src/shared.ts"}},
		"src/lazy-chart.ts":   {File: "assets/chart.js", CSS: []string{"assets/chart.css"}},
		"src/unrelated.ts":    {File: "assets/unrelated.js", CSS: []string{"assets/unrelated.css"}},
		"unrelated-style.css": {File: "assets/detached.css"},
	}
	payload, err := json.Marshal(manifest)
	require.NoError(t, err)

	assets := loadAssetBundle(payload)

	assert.Equal(t, "assets/entry.js", assets.Entry)
	assert.Len(t, assets.Revision, 12)
	assert.Equal(t, []string{"assets/chart.css", "assets/cycle.css", "assets/entry.css", "assets/shared.css"}, assets.Stylesheets)
}

func TestAssetRevisionChangesWhenLazyChunkGraphChanges(t *testing.T) {
	t.Parallel()

	manifest := map[string]manifestEntry{
		"index.html":  {File: "assets/entry.js", DynamicImports: []string{"src/map.tsx"}},
		"src/map.tsx": {File: "assets/map-old.js"},
	}
	oldPayload, err := json.Marshal(manifest)
	require.NoError(t, err)
	oldAssets := loadAssetBundle(oldPayload)

	manifest["src/map.tsx"] = manifestEntry{File: "assets/map-new.js"}
	newPayload, err := json.Marshal(manifest)
	require.NoError(t, err)
	newAssets := loadAssetBundle(newPayload)

	require.NotEqual(t, oldAssets.Revision, newAssets.Revision)
	require.NotEqual(t,
		versionedAssetURL(DefaultAssetBasePath, oldAssets.Revision, oldAssets.Entry),
		versionedAssetURL(DefaultAssetBasePath, newAssets.Revision, newAssets.Entry),
	)
}

func TestAssetsReferenceResolvedFiles(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry.js")
	useAssetSource(t, dir)

	assets := Assets()
	require.Len(t, assets.Revision, 12)
	require.NotEmpty(t, assets.Stylesheets)
	_, err := fs.Stat(DistFS(), assets.Entry)
	require.NoError(t, err)
	for _, stylesheet := range assets.Stylesheets {
		_, err := fs.Stat(DistFS(), stylesheet)
		require.NoError(t, err)
	}
}

func TestStaticControllerServesHashedAssetsWithLongCache(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry.js")
	useAssetSource(t, dir)

	router := mux.NewRouter()
	NewStaticController().Register(router)

	assets := Assets()
	request := httptest.NewRequest(http.MethodGet, versionedAssetURL(DefaultAssetBasePath, assets.Revision, assets.Entry), nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, immutableCacheControl, response.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
}

func TestStaticControllerRejectsUnknownAssetRevision(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry.js")
	useAssetSource(t, dir)

	router := mux.NewRouter()
	NewStaticController().Register(router)

	request := httptest.NewRequest(http.MethodGet, versionedAssetURL(DefaultAssetBasePath, "obsolete", Assets().Entry), nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusNotFound, response.Code)
}

func writeManifest(t *testing.T, dir, entryFile string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".vite"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "assets"), 0o755))
	payload, err := json.Marshal(map[string]manifestEntry{
		"index.html": {File: "assets/" + entryFile, CSS: []string{"assets/entry.css"}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".vite", "manifest.json"), payload, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", entryFile), []byte("export default 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "entry.css"), []byte(".lens-root {}\n"), 0o644))
}

// useAssetSource swaps the package-level source for the duration of one test.
// The source is process-wide because it mirrors a deployment decision, so tests
// that touch it cannot run in parallel.
func useAssetSource(t *testing.T, dir string) {
	t.Helper()
	previous := source
	source = newAssetSource(dir)
	t.Cleanup(func() { source = previous })
}

func TestAssetsFollowTheDiskBundleWhenAssetsDirIsSet(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry-old.js")
	useAssetSource(t, dir)

	before := Assets()
	require.Equal(t, "assets/entry-old.js", before.Entry)
	require.Len(t, before.Revision, 12)
	_, err := fs.Stat(DistFS(), before.Entry)
	require.NoError(t, err)

	// A rebuild must be visible without restarting the process.
	writeManifest(t, dir, "entry-new.js")
	after := Assets()
	assert.Equal(t, "assets/entry-new.js", after.Entry)
	assert.NotEqual(t, before.Revision, after.Revision)
}

func requireCompatibilityPanic(t *testing.T, run func()) {
	t.Helper()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		run()
	}()

	require.NotNil(t, recovered)
	err, ok := recovered.(error)
	require.True(t, ok, "compatibility failure must be an error, got %T", recovered)
	require.ErrorContains(t, err, compatibilityAssetsHelp)
	require.ErrorContains(t, err, ".vite/manifest.json")
}

func TestLegacySurfacesFailClearlyWhenBundleIsMissing(t *testing.T) {
	useAssetSource(t, t.TempDir())

	requireCompatibilityPanic(t, func() {
		Assets()
	})
	requireCompatibilityPanic(t, func() {
		var output bytes.Buffer
		_ = LensDashboard("/lens/document").Render(context.Background(), &output)
	})
	requireCompatibilityPanic(t, func() {
		NewStaticController()
	})
}

func TestLensDashboardPointsAtTheRebuiltBundleWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry-old.js")
	useAssetSource(t, dir)

	render := func() string {
		var output bytes.Buffer
		require.NoError(t, LensDashboard("/lens/document").Render(context.Background(), &output))
		return output.String()
	}

	before := render()
	assert.Contains(t, before, "assets/entry-old.js")

	writeManifest(t, dir, "entry-new.js")

	after := render()
	assert.Contains(t, after, "assets/entry-new.js")
	assert.NotContains(t, after, "assets/entry-old.js")
	assert.Contains(t, after, versionedAssetURL(DefaultAssetBasePath, Assets().Revision, "assets/entry-new.js"))
}

func TestStaticControllerServesTheRebuiltRevisionFromDisk(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry-old.js")
	useAssetSource(t, dir)

	router := mux.NewRouter()
	NewStaticController().Register(router)
	writeManifest(t, dir, "entry-new.js")

	assets := Assets()
	request := httptest.NewRequest(http.MethodGet, versionedAssetURL(DefaultAssetBasePath, assets.Revision, assets.Entry), nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, immutableCacheControl, response.Header().Get("Cache-Control"))
}

func TestStaticControllerRegistrationIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry.js")
	useAssetSource(t, dir)

	router := mux.NewRouter()
	NewStaticController().Register(router)
	NewStaticController().Register(router)

	count := 0
	require.NoError(t, router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		if route.GetName() == staticRouteName {
			count++
		}
		return nil
	}))
	assert.Equal(t, 1, count)
}

func TestLensDashboardRendersAssetsAndAttributes(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, "entry.js")
	useAssetSource(t, dir)

	var output bytes.Buffer
	err := LensDashboard(
		"/lens/snapshots/example",
		WithLocale("uz"),
		WithTheme(ThemeDark),
		WithCSRF("csrf-token"),
	).Render(context.Background(), &output)
	require.NoError(t, err)

	html := output.String()
	assert.Contains(t, html, `<lens-dashboard src="/lens/snapshots/example" locale="uz" theme="dark" csrf="csrf-token">`)
	assert.Contains(t, html, `type="module"`)
	assets := Assets()
	assert.Contains(t, html, versionedAssetURL(DefaultAssetBasePath, assets.Revision, assets.Entry))
	for _, stylesheet := range assets.Stylesheets {
		assert.Contains(t, html, versionedAssetURL(DefaultAssetBasePath, assets.Revision, stylesheet))
	}
}

func TestLensDashboardCanOmitAssetTags(t *testing.T) {
	useAssetSource(t, t.TempDir())

	var output bytes.Buffer
	err := LensDashboard("/lens/document", WithoutAssets()).Render(context.Background(), &output)
	require.NoError(t, err)

	html := output.String()
	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<link")
}

func TestLensDashboardRendersReactOwnedSkeleton(t *testing.T) {
	spec := build.Dashboard("test", "Test",
		build.Row(panel.Stat("total", "Total", "total").Span(4).Terminal().Build()),
	).Build()
	var output bytes.Buffer
	err := LensDashboard("/lens/document", WithoutAssets(), WithSkeleton(spec)).Render(context.Background(), &output)
	require.NoError(t, err)

	html := output.String()
	assert.Contains(t, html, `class="lens-dashboard-skeleton"`)
	assert.Contains(t, html, `--lens-panel-span:4`)
	assert.Contains(t, html, `class="lens-skeleton-card" data-kind="stat"`)
}

func TestLensDashboardEmbedsProgressiveFirstPaintDocument(t *testing.T) {
	t.Parallel()

	doc := &document.DashboardDocument{Version: "1.0.0", SnapshotID: "shell-snapshot"}
	var output bytes.Buffer
	err := LensDashboard("/lens/document", WithoutAssets(), WithInitialDocument(doc)).Render(context.Background(), &output)
	require.NoError(t, err)

	html := output.String()
	assert.Contains(t, html, `initial-document="`)
	assert.NotContains(t, html, "shell-snapshot", "the JSON payload must remain attribute-safe")
}
