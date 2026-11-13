package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/openapi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/reference"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/runtime"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type noteIO struct {
	Text string `json:"text"`
}

func referenceOf(t *testing.T) *reference.Builder {
	t.Helper()
	set := native.NewSet()
	require.NoError(t, set.Add(
		native.Entity(data.CoreUser()), native.Entity(data.CRMClient()),
		native.Command[noteIO, noteIO]("crm.commands.addNote", native.Spec{Permission: "Client.Update"}, nil),
		native.Query[noteIO, noteIO]("crm.queries.notes", native.Spec{}, nil),
	))
	return reference.New(reference.Natives(catalog.Natives(set)))
}

func asReader(perms ...string) spi.Subject {
	return spi.Subject{UserID: 1, Kind: spi.SubjectInitiator, Permissions: perms}
}

func normalize(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] != '{' {
			b.WriteByte(path[i])
			continue
		}
		j := strings.IndexByte(path[i:], '}')
		name, _, _ := strings.Cut(path[i+1:i+j], ":")
		b.WriteString("{" + name + "}")
		i += j
	}
	return b.String()
}

// The management API is mounted and documented from one table: every route
// the router registers is an operation of the spec and the other way round.
func TestRegisteredManageRoutesAreExactlyTheSpecPaths(t *testing.T) {
	t.Parallel()
	root, ok := (&runtime.API{}).Handler().(*mux.Router)
	require.True(t, ok)
	registered := map[string]bool{}
	require.NoError(t, root.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tpl, err := route.GetPathTemplate()
		if err != nil {
			return nil //nolint:nilerr // a prefix-only route has no template
		}
		methods, err := route.GetMethods()
		if err != nil || len(methods) == 0 {
			return nil //nolint:nilerr // the PathPrefix route has no methods
		}
		for _, m := range methods {
			registered[m+" "+normalize(tpl)] = true
		}
		return nil
	}))
	require.NotEmpty(t, registered)

	spec := map[string]bool{}
	for _, op := range runtime.ManageOperations(openapi.NewRegistry()) {
		key := op.Method + " " + op.Path
		require.False(t, spec[key], "%s documented twice", key)
		spec[key] = true
	}
	require.Equal(t, keys(registered), keys(spec))

	summaries := map[string]bool{}
	for _, r := range runtime.ManageRoutes() {
		summaries[r.Method+" "+r.Path] = true
	}
	require.Equal(t, keys(registered), keys(summaries), "the Help Center API document lists the same routes")
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestOpenAPIDocumentIsValidAndIncludesTheManagementAPIForReaders(t *testing.T) {
	t.Parallel()
	b := referenceOf(t)
	full, err := b.FullModel(context.Background(), "en")
	require.NoError(t, err)
	raw, err := runtime.BuildOpenAPI(full, "sid")
	require.NoError(t, err)
	require.NoError(t, openapi.Validate(raw))

	var doc struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
		Comps   struct {
			Schemas map[string]any `json:"schemas"`
			Sec     map[string]any `json:"securitySchemes"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Equal(t, "3.1.0", doc.OpenAPI)
	require.Contains(t, doc.Paths, "/_ext/rpc")
	require.Contains(t, doc.Paths, "/_ext/manage/packages/{ns}/files/{path}")
	require.Contains(t, doc.Paths, "/_ext/manage/reference")
	require.Contains(t, doc.Comps.Schemas, "Package")
	require.Contains(t, doc.Comps.Schemas, "VersionDetail")
	require.Contains(t, doc.Comps.Schemas, "ManageError")
	require.Contains(t, doc.Comps.Sec, "sessionCookie")

	// every management operation names its permission, the reference included
	publish := doc.Paths["/_ext/manage/packages/{ns}/publish"]["post"].(map[string]any)
	require.Equal(t, "Extensions.Publish", publish["x-required-permission"])
	reference := doc.Paths["/_ext/manage/reference"]["get"].(map[string]any)
	require.Equal(t, "Extensions.Read", reference["x-required-permission"])

	// readers without Extensions.Read get the contracts, never the management API
	limited, err := b.Model(context.Background(), asReader(), "en")
	require.NoError(t, err)
	raw, err = runtime.BuildOpenAPI(limited, "sid")
	require.NoError(t, err)
	require.NoError(t, openapi.Validate(raw))
	require.NotContains(t, string(raw), "/_ext/manage")
	require.Contains(t, string(raw), "/_ext/rpc")
}

func TestOpenAPIEndpointServesTheReadersDocument(t *testing.T) {
	t.Parallel()
	subject := func(who *spi.Subject) runtime.SubjectFunc {
		return func(*http.Request) (spi.Subject, error) {
			if who == nil {
				return spi.Subject{}, errors.New("no session")
			}
			return *who, nil
		}
	}
	get := func(who *spi.Subject) *httptest.ResponseRecorder {
		h := runtime.NewOpenAPI(referenceOf(t), subject(who), nil)
		router := mux.NewRouter()
		router.Path(runtime.OpenAPIPath).Handler(h)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, runtime.OpenAPIPath, nil))
		return rec
	}

	rec := get(nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	reader := asReader()
	rec = get(&reader)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	require.NoError(t, openapi.Validate(rec.Body.Bytes()))
	require.NotContains(t, rec.Body.String(), "/_ext/manage")

	admin := asReader(reference.PermRead)
	rec = get(&admin)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "/_ext/manage/packages")
}

func TestOpenAPIDocumentsEachGroupsOwnErrorsAndPermissions(t *testing.T) {
	t.Parallel()
	full, err := referenceOf(t).FullModel(context.Background(), "en")
	require.NoError(t, err)
	raw, err := runtime.BuildOpenAPI(full, "sid")
	require.NoError(t, err)
	var doc struct {
		Paths map[string]map[string]map[string]any `json:"paths"`
		Comps struct {
			Schemas   map[string]map[string]any `json:"schemas"`
			Responses map[string]any            `json:"responses"`
		} `json:"components"`
	}
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Empty(t, doc.Comps.Responses, "no shared default error response")

	defaultSchema := func(path, method string) any {
		responses := doc.Paths[path][method]["responses"].(map[string]any)
		def, ok := responses["default"].(map[string]any)
		if !ok {
			return nil
		}
		return def["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"]
	}
	require.Equal(t, "#/components/schemas/RpcFailure", defaultSchema("/_ext/rpc", "post"), "the RPC's failures are its own envelope")
	for path, methods := range doc.Paths {
		for method := range methods {
			switch {
			case strings.HasPrefix(path, "/_ext/manage/"):
				require.Equal(t, "#/components/schemas/ManageError", defaultSchema(path, method), "%s %s", method, path)
			case strings.HasPrefix(path, "/_ext/endpoints/"):
				require.Nil(t, defaultSchema(path, method), "a package endpoint's failures are the package's")
			}
		}
	}

	// A task may be decided by its assignee as well as by Extensions.Operate.
	decide := doc.Paths["/_ext/manage/tasks/{id}/decide"]["post"]
	require.Contains(t, decide["description"], "`Extensions.Operate`, or you are the assignee of the task")
	require.Contains(t, decide["description"], "hold the permission it is assigned to", "a task may be assigned to a permission")

	// Retry names the occurrence; unlike resolve it takes no result.
	retryBody := doc.Paths["/_ext/manage/workflows/{id}/retry"]["post"]["requestBody"].(map[string]any)
	ref := retryBody["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"].(string)
	retry := doc.Comps.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")]
	props := retry["properties"].(map[string]any)
	require.Contains(t, props, "name")
	require.NotContains(t, props, "result")
	resolveBody := doc.Paths["/_ext/manage/workflows/{id}/resolve"]["post"]["requestBody"].(map[string]any)
	require.Contains(t, resolveBody["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["$ref"], "occurrenceRequest")
}

// The reference is served to the configurator and needs Extensions.Read; the
// summary the Help Center lists says so.
func TestManageRoutesListThePermissionOfTheReferenceAndTheTaskAlternative(t *testing.T) {
	t.Parallel()
	byRoute := map[string]reference.APIRoute{}
	for _, r := range runtime.ManageRoutes() {
		byRoute[r.Method+" "+r.Path] = r
	}
	require.Equal(t, reference.PermRead, byRoute["GET /_ext/manage/reference"].Permission)
	decide := byRoute["POST /_ext/manage/tasks/{id}/decide"]
	require.Equal(t, "Extensions.Operate", decide.Permission)
	require.Contains(t, decide.Alternative, "assignee")
}
