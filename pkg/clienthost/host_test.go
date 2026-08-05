package clienthost

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
	"github.com/stretchr/testify/require"
)

func testManifest() Manifest {
	return Manifest{
		Package: "@iota-uz/example", PackageVersion: "0.0.0-sha-abc",
		SDKCommit: "0123456789abcdef0123456789abcdef01234567", SDKReleaseVersion: sdkidentity.ReleaseVersion,
		ProtocolVersion: ProtocolVersion, Entry: "/assets/example.js", Styles: []string{"/assets/example.css"},
		Integrity: map[string]string{"/assets/example.js": "sha384-example"},
	}
}

func TestController_ServesCanonicalBootstrapAndDescriptor(t *testing.T) {
	t.Parallel()

	spec := application.Get("/reports", application.RenderedBy(application.RouteRendererReact))
	controller, err := NewController("reports", testManifest(), []Route{{
		Spec: spec,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Title: "Reports", Initial: map[string]string{"value": "</script>"}}, nil
		},
	}}, WithSession(func(*http.Request) (SessionContext, error) { return SessionContext{Theme: "dark", CSRF: "token"}, nil }))
	require.NoError(t, err)
	require.Equal(t, application.RouteRendererReact, controller.Descriptor().Routes[0].Renderer)

	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, `id="iota-client-route-mount"`)
	require.Contains(t, body, `data-sdk-commit="0123456789abcdef0123456789abcdef01234567"`)
	require.Contains(t, body, `integrity="sha384-example"`)
	require.Contains(t, body, `"protocolVersion":"1.0.0"`)
	require.Contains(t, body, `"sdkReleaseVersion":"0.5.0"`)
	require.Contains(t, body, `"sdkCommit":"0123456789abcdef0123456789abcdef01234567"`)
	require.Contains(t, body, `"theme":"dark"`)
	require.Contains(t, body, `"csrf":"token"`)
	require.NotContains(t, body, `{"value":"</script>"}`)
}

func TestController_RejectsNonReactRoutesAndProtocolMismatch(t *testing.T) {
	t.Parallel()

	_, err := NewController("bad", testManifest(), []Route{{Spec: application.Get("/bad"), Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil }}})
	require.ErrorContains(t, err, "must declare react renderer")
	manifest := testManifest()
	manifest.ProtocolVersion = "2.0.0"
	require.ErrorContains(t, manifest.Validate(), "incompatible")
}

func TestManifest_RoundTripsProvenance(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(testManifest())
	require.NoError(t, err)
	parsed, err := ParseManifest(payload)
	require.NoError(t, err)
	require.Equal(t, testManifest(), parsed)
}
