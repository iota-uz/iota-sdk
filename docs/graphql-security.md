# GraphQL security defaults

`pkg/graphql.Handler` denies by default. Every executor it serves (the root
schema, `AddExecutor`, and app schemas registered through `GraphSchemas()`)
gets the same guards.

## Auth directives

Every root field of `Query`, `Mutation` and `Subscription` must carry exactly
one of:

| Directive | Meaning |
| --- | --- |
| `@public` | reachable without a session |
| `@authenticated` | needs `Authorizer.Authenticated` to pass |
| `@can(permissions: ["A", "B"])` | needs an authenticated caller holding every permission |

Nested fields may carry the same directives; a nested field without one
inherits its parent's decision.

Declare the directives in the schema (copy `graphql.AuthDirectivesSDL`) and let
the handler enforce them instead of generated code:

```yaml
# gqlgen.yml
directives:
  public: { skip_runtime: true }
  authenticated: { skip_runtime: true }
  can: { skip_runtime: true }
```

A schema with a root field without a directive does not start: `NewHandler` /
`AddExecutor` panic with the list of fields. Catch it in CI earlier:

```go
func TestSchemaHasAuthDirectives(t *testing.T) {
	require.NoError(t, graphql.ValidateAuthDirectives(NewExecutableSchema(Config{}).Schema()))
}
```

For a migration window use `graphql.WithAuthDirectivesMode(graphql.AuthDirectivesWarn)`:
missing directives are logged once at startup and those fields stay reachable;
annotated fields are already enforced.

## Authorizers

- `graphql.WithAuthorizer(a)` backs `@authenticated` / `@can`. Without one every
  guarded field is denied. The core GraphQL controller installs
  `authorizers.SessionDirectiveAuthorizer`, which uses the session user and its
  effective permission names.
- `graphql.WithObjectAuthorizer(o)` is the ownership hook. It runs after a
  protected field (non-`@public` root field, or a nested field with
  `@authenticated` / `@can`) resolved to a non-nil value and can deny that
  object, e.g. a policy that belongs to another customer.

## Operation limits

| Option | Default | Effect |
| --- | --- | --- |
| `WithDepthLimit(n)` | 15 | rejects deeper selections (introspection subtrees not counted), code `DEPTH_LIMIT_EXCEEDED` |
| `WithComplexityLimit(n)` | 1000 | gqlgen complexity, code `COMPLEXITY_LIMIT_EXCEEDED` |
| `WithMaxBodyBytes(n)` | 1 MiB | JSON body cap, HTTP 413; multipart keeps `uploadsconfig` limits |
| `WithGETTransport()` | off | enables queries over GET |

Limits are checked while the operation context is created, before any resolver
runs; the response is HTTP 422. Zero or a negative value disables a limit.

`SameOriginValidator` still accepts requests without an `Origin` header (API
clients) but rejects them when `Sec-Fetch-Site` says the browser sent them
cross-site.

## Wiring in an application

```go
bootstrap.InstallCoreControllers(
	controllers.WithGraphQLHandlerOptions(
		graphql.WithObjectAuthorizer(graphql.ObjectAuthorizerFunc(ownsObject)),
		graphql.WithComplexityLimit(2000),
	),
)
```

The options apply to `/graphql/core/query` and to every per-schema endpoint and
run after the defaults, so they can replace the authorizer too.
