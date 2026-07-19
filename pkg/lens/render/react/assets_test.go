package react

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssetsReferenceEmbeddedFiles(t *testing.T) {
	assets := Assets()
	require.NotEmpty(t, assets.Stylesheets)
	_, err := fs.Stat(DistFS(), assets.Entry)
	require.NoError(t, err)
	for _, stylesheet := range assets.Stylesheets {
		_, err := fs.Stat(DistFS(), stylesheet)
		require.NoError(t, err)
	}
}

func TestStaticControllerServesHashedAssetsWithLongCache(t *testing.T) {
	router := mux.NewRouter()
	NewStaticController().Register(router)

	request := httptest.NewRequest(http.MethodGet, joinAssetURL(DefaultAssetBasePath, Assets().Entry), nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, immutableCacheControl, response.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
}

func TestLensDashboardRendersAssetsAndAttributes(t *testing.T) {
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
	assert.Contains(t, html, joinAssetURL(DefaultAssetBasePath, Assets().Entry))
	for _, stylesheet := range Assets().Stylesheets {
		assert.Contains(t, html, joinAssetURL(DefaultAssetBasePath, stylesheet))
	}
}

func TestLensDashboardCanOmitAssetTags(t *testing.T) {
	var output bytes.Buffer
	err := LensDashboard("/lens/document", WithoutAssets()).Render(context.Background(), &output)
	require.NoError(t, err)

	html := output.String()
	assert.NotContains(t, html, "<script")
	assert.NotContains(t, html, "<link")
}
