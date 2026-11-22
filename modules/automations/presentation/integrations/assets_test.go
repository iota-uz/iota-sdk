package integrations

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A release candidate stamps a new SDK version into Go and the client host without rebuilding the
// committed bundle: the embedded bundle is served as that release, so the page's identity check passes.
func TestEmbeddedBundleServesAStampedRelease(t *testing.T) {
	t.Parallel()
	files, err := bundleFiles()
	require.NoError(t, err)
	const candidate = "9.9.9"
	b, err := bundleFor(files, candidate)
	require.NoError(t, err)
	assert.Equal(t, candidate, b.Manifest.SDKReleaseVersion)
	require.True(t, strings.HasPrefix(b.Manifest.Entry, AssetsPath+"/"+candidate+"/assets/"), b.Manifest.Entry)

	get := func(p string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		b.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		return rec
	}
	rec := get(b.Manifest.Entry)
	require.Equal(t, http.StatusOK, rec.Code)
	body, _ := io.ReadAll(rec.Body)
	assert.Contains(t, string(body), `"`+candidate+`"`, "the client host carries the serving release")
	assert.NotContains(t, string(body), "0.0.0-iota-automations-release")
	assert.Contains(t, rec.Header().Get("Content-Type"), "javascript")
	for _, s := range b.Manifest.Styles {
		assert.Equal(t, http.StatusOK, get(s).Code)
	}
	// Another release's URL and the build metadata are not served.
	assert.Equal(t, http.StatusNotFound, get(strings.Replace(b.Manifest.Entry, candidate, "0.0.1", 1)).Code)
	assert.Equal(t, http.StatusNotFound, get(AssetsPath+"/"+candidate+"/bundle.json").Code)

	// The binary's own release validates.
	current, err := DefaultBundle()
	require.NoError(t, err)
	require.NoError(t, current.Manifest.Validate())
}

// The bundle's utilities load after the application's stylesheet in the same cascade layer: unscoped,
// they override the layout (an app page lost its sidebar). They apply only inside the client route.
func TestBundleUtilitiesAreScopedToTheClientRoute(t *testing.T) {
	t.Parallel()
	files, err := bundleFiles()
	require.NoError(t, err)
	b, err := bundleFor(files, "9.9.9")
	require.NoError(t, err)
	require.NotEmpty(t, b.Manifest.Styles)
	for _, s := range b.Manifest.Styles {
		rec := httptest.NewRecorder()
		b.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, s, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		css := rec.Body.String()
		assert.Contains(t, css, "@layer utilities{:is(#iota-client-route,#iota-client-route-portals){")
		assert.Equal(t, 1, strings.Count(css, "@layer utilities{"), "a single, scoped utilities layer")
	}
}
