package clienthost

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/sdkidentity"
	"github.com/sirupsen/logrus"
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

func TestController_LogsRouteContextOnRenderFailure(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	controller, err := NewController("reports", testManifest(), []Route{{
		Spec: application.Get("/reports", application.ClientFeature("reports.configure", "reports"), application.Public()),
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{}, errors.New("build broke")
		},
	}}, WithLogger(logger))
	require.NoError(t, err)
	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/reports", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.Contains(t, output.String(), "controller.id=reports")
	require.Contains(t, output.String(), "route.id=reports.configure")
	require.Contains(t, output.String(), "stage=build")
	require.Contains(t, output.String(), "build broke")
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

func TestController_RejectsMixedRenderers(t *testing.T) {
	t.Parallel()
	build := func(_ context.Context, _ *http.Request) (RoutePayload, error) { return RoutePayload{}, nil }
	_, err := NewController("mixed", testManifest(), []Route{
		{Spec: application.Get("/legacy", application.RenderedBy(application.RouteRendererReact)), Build: build},
		{Spec: application.Get("/solid", application.ClientFeature("solid.route", "solid.feature"), application.Public()), Build: build},
	})
	require.ErrorContains(t, err, "mixes react and client renderers")
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

type testFeature struct {
	id string
}

func (feature testFeature) FeatureID() string  { return feature.id }
func (feature testFeature) SourcePath() string { return "./Screen.tsx" }

type testRenderedFeature struct {
	id    string
	props any
}

func (feature testRenderedFeature) FeatureID() string { return feature.id }
func (feature testRenderedFeature) Props() any        { return feature.props }

func TestController_DerivesImportedFeatureAndRendersItsProps(t *testing.T) {
	t.Parallel()
	feature := testFeature{id: "solid-derived"}
	spec := application.Get("/solid", application.ClientRoute("solid.route"), application.Authenticated())
	controller, err := NewController("solid", testManifest(), []Route{{
		Spec:    spec,
		Feature: feature,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Title: "Solid", Screen: testRenderedFeature{id: feature.id, props: map[string]string{"name": "Granite"}}}, nil
		},
	}})
	require.NoError(t, err)
	require.Equal(t, feature.id, controller.Descriptor().Routes[0].FeatureID)

	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/solid", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"featureId":"solid-derived"`)
	require.Contains(t, recorder.Body.String(), `"initial":{"name":"Granite"}`)
}

func TestController_RejectsMissingOrMismatchedImportedRender(t *testing.T) {
	t.Parallel()
	feature := testFeature{id: "solid-derived"}
	spec := application.Get("/solid", application.ClientRoute("solid.route"), application.Authenticated())
	for name, payload := range map[string]RoutePayload{
		"missing":  {},
		"mismatch": {Screen: testRenderedFeature{id: "other", props: struct{}{}}},
	} {
		t.Run(name, func(t *testing.T) {
			controller, err := NewController("solid", testManifest(), []Route{{
				Spec: spec, Feature: feature,
				Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) { return payload, nil },
			}})
			require.NoError(t, err)
			router := mux.NewRouter()
			controller.Register(router)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/solid", nil))
			require.Equal(t, http.StatusInternalServerError, recorder.Code)
		})
	}
}

func TestController_RejectsUnsafeJavaScriptIntegerProps(t *testing.T) {
	t.Parallel()
	feature := testFeature{id: "solid-derived"}
	controller, err := NewController("solid", testManifest(), []Route{{
		Spec:    application.Get("/solid", application.ClientRoute("solid.route"), application.Authenticated()),
		Feature: feature,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Screen: testRenderedFeature{id: feature.id, props: struct {
				Value uint64 `json:"value"`
			}{Value: 1 << 63}}}, nil
		},
	}})
	require.NoError(t, err)
	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/solid", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

type unsafeNumberMarshaler struct{}

func (unsafeNumberMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(`{"value":9007199254740993}`), nil
}

func TestController_RejectsUnsafeNumbersFromCustomJSONMarshalers(t *testing.T) {
	t.Parallel()
	feature := testFeature{id: "solid-derived"}
	controller, err := NewController("solid", testManifest(), []Route{{
		Spec:    application.Get("/solid", application.ClientRoute("solid.route"), application.Authenticated()),
		Feature: feature,
		Build: func(_ context.Context, _ *http.Request) (RoutePayload, error) {
			return RoutePayload{Screen: testRenderedFeature{id: feature.id, props: unsafeNumberMarshaler{}}}, nil
		},
	}})
	require.NoError(t, err)
	router := mux.NewRouter()
	controller.Register(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/solid", nil))
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}

func TestManifest_RoundTripsProvenance(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(testManifest())
	require.NoError(t, err)
	parsed, err := ParseManifest(payload)
	require.NoError(t, err)
	require.Equal(t, testManifest(), parsed)
}
