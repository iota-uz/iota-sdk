package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	coregraph "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	warehousegraph "github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

type schemaOnly struct{ schema *ast.Schema }

func (s schemaOnly) Schema() *ast.Schema { return s.schema }

func (schemaOnly) Complexity(string, string, int, map[string]any) (int, bool) { return 0, false }

func (schemaOnly) Exec(context.Context) gqlgraphql.ResponseHandler {
	return func(context.Context) *gqlgraphql.Response { return &gqlgraphql.Response{} }
}

func loadSchema(t *testing.T, sdl string) *ast.Schema {
	t.Helper()
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "test.graphql", Input: AuthDirectivesSDL + sdl})
	require.NoError(t, err)
	return schema
}

type recordingLogger struct{ messages []string }

func (l *recordingLogger) Warnf(format string, args ...any) {
	l.messages = append(l.messages, fmt.Sprintf(format, args...))
}

type stubAuthorizer struct {
	authenticated bool
	granted       map[string]bool
}

func (a stubAuthorizer) Authenticated(context.Context) error {
	if !a.authenticated {
		return serrors.NewUnauthenticated("no session")
	}
	return nil
}

func (a stubAuthorizer) Can(_ context.Context, permissions []string) error {
	for _, p := range permissions {
		if !a.granted[p] {
			return serrors.NewPermissionDenied(p)
		}
	}
	return nil
}

func TestValidateAuthDirectives(t *testing.T) {
	t.Run("all root fields annotated", func(t *testing.T) {
		schema := loadSchema(t, `
			type Query {
				ping: String @public
				me: String @authenticated
				report: String @can(permissions: ["Report.Read"])
			}
			type Mutation { save: Boolean @can(permissions: "Report.Write") }
			type Subscription { ticks: Int @authenticated }
		`)
		require.NoError(t, ValidateAuthDirectives(schema))
	})

	t.Run("missing and invalid directives are listed", func(t *testing.T) {
		schema := loadSchema(t, `
			type Insurance { id: ID!, owner: String @public @authenticated }
			type Query {
				insuranceByID(id: Int!): Insurance
				ping: String @public
				both: String @public @authenticated
				empty: String @can(permissions: [])
			}
			type Mutation { save: Boolean }
		`)
		err := ValidateAuthDirectives(schema)
		var missing *MissingAuthDirectivesError
		require.ErrorAs(t, err, &missing)
		require.Equal(t, []string{"Mutation.save", "Query.insuranceByID"}, missing.Missing)
		require.Len(t, missing.Invalid, 3)
		require.Contains(t, err.Error(), "Query.both")
		require.Contains(t, err.Error(), "Query.empty")
		require.Contains(t, err.Error(), "Insurance.owner")
	})

	t.Run("sdk schemas are fully annotated", func(t *testing.T) {
		require.NoError(t, ValidateAuthDirectives(coregraph.NewExecutableSchema(coregraph.Config{}).Schema()))
		require.NoError(t, ValidateAuthDirectives(warehousegraph.NewExecutableSchema(warehousegraph.Config{}).Schema()))
	})
}

func TestNewHandler_AuthDirectivesMode(t *testing.T) {
	sdl := `type Query { insuranceByID(id: Int!): String, ping: String @public }`

	t.Run("enforce mode refuses to build", func(t *testing.T) {
		exec := executor.New(schemaOnly{schema: loadSchema(t, sdl)})
		require.PanicsWithError(t,
			"graphql: root fields without @public, @authenticated or @can: Query.insuranceByID",
			func() { NewHandler(exec, nil) },
		)
	})

	t.Run("enforce mode refuses executors added later", func(t *testing.T) {
		handler := NewBaseServer(coregraph.NewExecutableSchema(coregraph.Config{}), nil)
		require.Panics(t, func() {
			handler.AddExecutor(executor.New(schemaOnly{schema: loadSchema(t, sdl)}))
		})
	})

	t.Run("warn mode logs the missing fields", func(t *testing.T) {
		logger := &recordingLogger{}
		exec := executor.New(schemaOnly{schema: loadSchema(t, sdl)})
		require.NotPanics(t, func() {
			NewHandler(exec, nil, WithAuthDirectivesMode(AuthDirectivesWarn), WithLogger(logger))
		})
		require.Len(t, logger.messages, 1)
		require.Contains(t, logger.messages[0], "Query.insuranceByID")
	})

	t.Run("warn mode still refuses malformed directives", func(t *testing.T) {
		exec := executor.New(schemaOnly{schema: loadSchema(t, `type Query { x: String @public @authenticated }`)})
		require.Panics(t, func() { NewHandler(exec, nil, WithAuthDirectivesMode(AuthDirectivesWarn)) })
	})
}

func fieldCtx(t *testing.T, schema *ast.Schema, typeName, fieldName string, parent *gqlgraphql.FieldContext) *gqlgraphql.FieldContext {
	t.Helper()
	def := schema.Types[typeName].Fields.ForName(fieldName)
	require.NotNil(t, def)
	return &gqlgraphql.FieldContext{
		Parent: parent,
		Object: typeName,
		Field: gqlgraphql.CollectedField{
			Field: &ast.Field{Name: fieldName, Definition: def},
		},
	}
}

// withFieldChain nests each field context under the previous one.
func withFieldChain(ctx context.Context, chain []*gqlgraphql.FieldContext) context.Context {
	if len(chain) == 0 {
		return ctx
	}
	return withFieldChain(gqlgraphql.WithFieldContext(ctx, chain[0]), chain[1:])
}

func TestAuthExtension_InterceptField(t *testing.T) {
	schema := loadSchema(t, `
		type Policy { id: ID!, premium: Int @can(permissions: ["Policy.Finance"]), note: String }
		type Query {
			ping: String @public
			legacy: String
			policy: Policy @authenticated
			audit: String @can(permissions: ["Audit.Read", "Audit.Export"])
		}
	`)
	policy := &struct{ Owner string }{Owner: "alice"}
	resolved := 0
	next := func(value any) gqlgraphql.Resolver {
		return func(context.Context) (any, error) {
			resolved++
			return value, nil
		}
	}
	// run resolves the last field context; the preceding ones are its ancestors
	// (WithFieldContext links each to the previous one as Parent).
	run := func(ext authExtension, chain ...*gqlgraphql.FieldContext) func(value any) (any, error) {
		return func(value any) (any, error) {
			resolved = 0
			return ext.InterceptField(withFieldChain(context.Background(), chain), next(value))
		}
	}
	root := func(name string) *gqlgraphql.FieldContext { return fieldCtx(t, schema, "Query", name, nil) }

	anonymous := authExtension{authorizer: stubAuthorizer{}}
	member := authExtension{authorizer: stubAuthorizer{authenticated: true, granted: map[string]bool{"Audit.Read": true}}}

	t.Run("public field resolves anonymously", func(t *testing.T) {
		res, err := run(anonymous, root("ping"))("pong")
		require.NoError(t, err)
		require.Equal(t, "pong", res)
	})

	t.Run("authenticated field is denied before the resolver", func(t *testing.T) {
		_, err := run(anonymous, root("policy"))(policy)
		require.True(t, serrors.HasCode(err, serrors.Unauthenticated))
		require.Zero(t, resolved)
	})

	t.Run("missing authorizer denies guarded fields", func(t *testing.T) {
		_, err := run(authExtension{}, root("policy"))(policy)
		require.Error(t, err)
		require.Zero(t, resolved)
	})

	t.Run("can requires every permission", func(t *testing.T) {
		_, err := run(member, root("audit"))("log")
		require.True(t, serrors.HasCode(err, serrors.PermissionDenied))
		require.Zero(t, resolved)

		full := authExtension{authorizer: stubAuthorizer{authenticated: true, granted: map[string]bool{"Audit.Read": true, "Audit.Export": true}}}
		res, err := run(full, root("audit"))("log")
		require.NoError(t, err)
		require.Equal(t, "log", res)
	})

	t.Run("undirected root field is denied in enforce mode and allowed in warn mode", func(t *testing.T) {
		_, err := run(member, root("legacy"))("x")
		require.True(t, serrors.HasCode(err, serrors.PermissionDenied))
		require.Zero(t, resolved)

		warn := member
		warn.mode = AuthDirectivesWarn
		res, err := run(warn, root("legacy"))("x")
		require.NoError(t, err)
		require.Equal(t, "x", res)
	})

	t.Run("nested directive is enforced, nested undirected field inherits", func(t *testing.T) {
		listItem := &gqlgraphql.FieldContext{Index: new(int)}

		_, err := run(member, root("policy"), listItem, fieldCtx(t, schema, "Policy", "premium", nil))(10)
		require.True(t, serrors.HasCode(err, serrors.PermissionDenied))

		res, err := run(anonymous, root("policy"), listItem, fieldCtx(t, schema, "Policy", "note", nil))("n")
		require.NoError(t, err)
		require.Equal(t, "n", res)
	})

	t.Run("object authorizer can deny a resolved object", func(t *testing.T) {
		var seen []string
		owner := member
		owner.objectAuthorizer = ObjectAuthorizerFunc(func(_ context.Context, fc *gqlgraphql.FieldContext, obj any) error {
			seen = append(seen, fc.Field.Name)
			if obj.(*struct{ Owner string }).Owner != "bob" {
				return serrors.NewPermissionDenied("not owner")
			}
			return nil
		})

		res, err := run(owner, root("policy"))(policy)
		require.Nil(t, res)
		require.True(t, serrors.HasCode(err, serrors.PermissionDenied))
		require.Equal(t, 1, resolved)

		// nil results and public fields skip the hook.
		_, err = run(owner, root("policy"))((*struct{ Owner string })(nil))
		require.NoError(t, err)
		_, err = run(owner, root("ping"))("pong")
		require.NoError(t, err)
		require.Equal(t, []string{"policy"}, seen)
	})
}

type gqlResponse struct {
	Data   map[string]any `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

func postQuery(t *testing.T, h http.Handler, query string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, gqlResponse) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"query": query})
	require.NoError(t, err)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/query", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for _, m := range mutate {
		m(req)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	var body gqlResponse
	if strings.HasPrefix(res.Header().Get("Content-Type"), "application/json") {
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body), res.Body.String())
	}
	return res, body
}

func TestHandler_OperationLimits(t *testing.T) {
	// Resolvers are nil: reaching one would panic, so a clean error proves the
	// operation was rejected before execution.
	newHandler := func(opts ...HandlerOption) *Handler {
		return NewBaseServer(coregraph.NewExecutableSchema(coregraph.Config{}), nil, opts...)
	}
	deep := `{ users(offset: 0, limit: 1, ascending: true) { data { id } } }`

	t.Run("depth limit", func(t *testing.T) {
		res, body := postQuery(t, newHandler(WithDepthLimit(2)), deep)
		require.Equal(t, http.StatusUnprocessableEntity, res.Code)
		require.Len(t, body.Errors, 1)
		require.EqualValues(t, 2, body.Errors[0].Extensions["limit"])
		require.Equal(t, errDepthLimit, body.Errors[0].Extensions["code"])
	})

	t.Run("complexity limit", func(t *testing.T) {
		_, body := postQuery(t, newHandler(WithComplexityLimit(2)), deep)
		require.Len(t, body.Errors, 1)
		require.Equal(t, errComplexityLimit, body.Errors[0].Extensions["code"])
	})

	t.Run("anonymous call to an authenticated field never reaches the resolver", func(t *testing.T) {
		res, body := postQuery(t, newHandler(), deep)
		require.Equal(t, http.StatusOK, res.Code)
		require.NotEmpty(t, body.Errors)
		require.Equal(t, "UNAUTHORIZED", body.Errors[0].Extensions["code"])
	})

	t.Run("body size limit", func(t *testing.T) {
		big := `{ hello(name: "` + strings.Repeat("a", 2048) + `") }`
		res, body := postQuery(t, newHandler(WithMaxBodyBytes(1024)), big)
		require.Equal(t, http.StatusRequestEntityTooLarge, res.Code)
		require.Contains(t, body.Errors[0].Message, "exceeds 1024 bytes")
	})

	t.Run("full introspection fits the default limits", func(t *testing.T) {
		original := registerIntrospection
		t.Cleanup(func() { registerIntrospection = original })
		registerIntrospection = func(server *Handler, exec *executor.Executor) {
			server.Use(map[*executor.Executor]gqlgraphql.HandlerExtension{exec: extension.Introspection{}})
		}
		_, body := postQuery(t, newHandler(), introspectionQuery)
		require.Empty(t, body.Errors)
		require.Contains(t, body.Data, "__schema")
	})
}

func TestHandler_GETTransport(t *testing.T) {
	get := func(h http.Handler) int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/query?query="+url.QueryEscape(`{ users(offset: 0, limit: 1, ascending: true) { total } }`), nil)
		res := httptest.NewRecorder()
		h.ServeHTTP(res, req)
		return res.Code
	}
	schema := coregraph.NewExecutableSchema(coregraph.Config{})

	require.Equal(t, http.StatusBadRequest, get(NewBaseServer(schema, nil)))
	require.Equal(t, http.StatusOK, get(NewBaseServer(schema, nil, WithGETTransport())))
}

func TestSameOriginValidator_MissingOrigin(t *testing.T) {
	validator := SameOriginValidator(&httpconfig.Config{Port: 3200, Domain: "erp.example.com"}, &appconfig.Config{Environment: "production"})
	for site, want := range map[string]bool{
		"":            true, // non-browser client
		"same-origin": true,
		"none":        true,
		"same-site":   false,
		"cross-site":  false,
	} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/query", nil)
		if site != "" {
			req.Header.Set("Sec-Fetch-Site", site)
		}
		require.Equal(t, want, validator(req), site)
	}
}

func TestSelectionDepth_FragmentCycleTerminates(t *testing.T) {
	doc := &ast.QueryDocument{Fragments: ast.FragmentDefinitionList{
		{Name: "A", SelectionSet: ast.SelectionSet{&ast.FragmentSpread{Name: "A"}, &ast.Field{Name: "x"}}},
	}}
	depth := selectionDepth(ast.SelectionSet{&ast.FragmentSpread{Name: "A"}}, doc, map[string]bool{}, 10)
	require.Equal(t, 1, depth)
}

const introspectionQuery = `query IntrospectionQuery {
  __schema {
    queryType { name } mutationType { name } subscriptionType { name }
    types { ...FullType }
    directives { name description locations args { ...InputValue } }
  }
}
fragment FullType on __Type {
  kind name description
  fields(includeDeprecated: true) { name description args { ...InputValue } type { ...TypeRef } isDeprecated deprecationReason }
  inputFields { ...InputValue }
  interfaces { ...TypeRef }
  enumValues(includeDeprecated: true) { name description isDeprecated deprecationReason }
  possibleTypes { ...TypeRef }
}
fragment InputValue on __InputValue { name description type { ...TypeRef } defaultValue }
fragment TypeRef on __Type {
  kind name
  ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name ofType { kind name } } } } } } }
}`
