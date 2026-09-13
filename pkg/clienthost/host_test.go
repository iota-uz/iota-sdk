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

	spec := application.Get("/reports", application.ClientFeature("reports.configure", "reports"), application.Authenticated())
	controller, err := NewController("reports", testManifest(), []Route{{
		Spec: spec,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Title: "Reports", Initial: map[string]string{"value": "</script>"}}, nil
		},
	}}, WithSession(func(*http.Request) (SessionContext, error) {
		return SessionContext{Theme: "dark", CSRF: "token", Locale: "uz", Messages: map[string]string{"save": "Saqlash"}, Permissions: []string{"reports.edit"}}, nil
	}), WithServices(ServiceEndpoints{RPC: "/rpc"}))
	require.NoError(t, err)
	require.Equal(t, application.RouteRendererClient, controller.Descriptor().Routes[0].Renderer)

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
	require.Contains(t, body, `"sdkReleaseVersion":"`+sdkidentity.ReleaseVersion+`"`)
	require.Contains(t, body, `"sdkCommit":"0123456789abcdef0123456789abcdef01234567"`)
	require.Contains(t, body, `"theme":"dark"`)
	require.Contains(t, body, `"csrf":"token"`)
	require.Contains(t, body, `"bootstrapVersion":"1.0.0"`)
	require.Contains(t, body, `"featureId":"reports"`)
	require.Contains(t, body, `"language":"uz"`)
	require.Contains(t, body, `"save":"Saqlash"`)
	require.Contains(t, body, `"rpc":"/rpc"`)
	require.NotContains(t, body, `{"value":"</script>"}`)
}

func TestController_DefaultsEmptySessionLocale(t *testing.T) {
	t.Parallel()

	spec := application.Get("/reports", application.ClientFeature("reports.configure", "reports"), application.Authenticated())
	controller, err := NewController("reports", testManifest(), []Route{{
		Spec: spec,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Initial: map[string]string{}}, nil
		},
	}}, WithSession(func(*http.Request) (SessionContext, error) {
		return SessionContext{}, nil
	}))
	require.NoError(t, err)

	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"language":"en"`)
}

func TestController_RejectsInvalidClientRoutesAndProtocolMismatch(t *testing.T) {
	t.Parallel()

	_, err := NewController("bad", testManifest(), []Route{{Spec: application.Get("/bad"), Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil }}})
	require.ErrorContains(t, err, "must declare client renderer")
	_, err = NewController("bad", testManifest(), []Route{{Spec: application.Get("/bad", application.RenderedBy(application.RouteRendererClient)), Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil }}})
	require.ErrorContains(t, err, "requires route and feature identity")
	manifest := testManifest()
	manifest.ProtocolVersion = "2.0.0"
	require.ErrorContains(t, manifest.Validate(), "incompatible")
}

func TestController_PreservesLegacyReactRoute(t *testing.T) {
	t.Parallel()
	_, err := NewController("legacy", testManifest(), []Route{{
		Spec:  application.Get("/legacy", application.RenderedBy(application.RouteRendererReact)),
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil },
	}})
	require.NoError(t, err)
}

func TestController_RequiresExplicitAccessAndUniqueIdentity(t *testing.T) {
	t.Parallel()
	build := func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil }
	implicit := application.Get("/implicit", application.ClientFeature("implicit", "feature"))
	_, err := NewController("implicit", testManifest(), []Route{{Spec: implicit, Build: build}})
	require.ErrorContains(t, err, "requires explicit access")

	first := application.Get("/first", application.ClientFeature("duplicate", "feature"), application.Public())
	second := application.Get("/second", application.ClientFeature("duplicate", "feature"), application.Authenticated())
	_, err = NewController("duplicate", testManifest(), []Route{{Spec: first, Build: build}, {Spec: second, Build: build}})
	require.ErrorContains(t, err, "duplicate route id")
}

func TestManifest_RoundTripsProvenance(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(testManifest())
	require.NoError(t, err)
	parsed, err := ParseManifest(payload)
	require.NoError(t, err)
	require.Equal(t, testManifest(), parsed)
}
