# Canonical serrors cutover

The cutover stacks on the compatible preview in SDK #1146. It removes the
legacy variadic constructor, mutable error fields, base and validation types,
JSON unmarshal helper and dedicated GraphQL unauthorized constructor.
The preview import is replaced by `pkg/serrors` and its presenter subpackages.

## Final contracts and verification

The one-off migration tools and fixtures have been removed after the cutover.
New code uses typed constructors for explicit classification and `Wrap` or
`WrapContext` to preserve existing classification, operation context and causes.
Private diagnostic text never becomes a public message implicitly.

At concrete SQL/pgx/sqlx boundaries, use `FromDB` or `FromDBContext`.
Recognized constraints use `FromConstraint`; unknown constraints remain internal.
Existing explicit classification and cancellation retain priority over driver
projection. Domain calls and translated sentinels remain wrappers.

Verify the canonical API and mounted adapter contracts with:

```sh
go test -race ./pkg/serrors/... ./pkg/graphql/... ./pkg/appletengine/rpc/...
go vet ./...
```

Review the following contracts when extending error handling:

- Validation failures use Invalid, immutable field violations and the validation
  bridge. Forms retain field names, label references, tax details and locale keys.
  Import and warehouse errors retain concrete types and business reasons.
- Multiple causes use Multi to retain sentinel identity and underlying causes.
  Unsupported JSON mapping is Unimplemented.
- Missing authenticated users and invalid OIDC credentials are Unauthenticated;
  authenticated policy/access denial remains PermissionDenied.
- Recognized department unique violations are AlreadyExists, with the Code
  field violation, the original driver cause, and ErrDuplicateCode identity.
  Unknown constraints remain internal. Existing zero-row checks stay explicit.
- HTTP error bodies use explicit public projections. Noninternal semantic
  classification corrects statuses even when a route formerly passed 500.
  Plain text, JSON and HTML route formats remain route-owned. Unknown errors
  retain approved input/protocol status fallbacks and omit diagnostics.
- GraphQL retains protocol/transport status handling and UNAUTHORIZED auth codes.
  RPC preserves carrier/middleware/protocol/sentinel priority before semantic
  projection. Existing custom error carriers remain available.

## Consumer transition

Granite's migration prepares an atomic cutover rather than a permanent legacy
shim. The SDK breaking PR is a candidate: it must not be promoted until the
Granite mounted contract fixtures and preview workflow validate its immutable
revision, and production dependency checks pass with GOWORK=off. The candidate
uses the verified SDK release workflow in docs/sdk-releases.md; no version tag
is pushed manually. The consumer PR records its actual pin and preview results.

Shared composable authentication sentinels now carry semantic codes while retaining
sentinel identity. This prevents generic boundaries from classifying a missing
user/session as an internal error. GraphQL applications can declare specialized
public codes and typed extension values through the presenter carrier interface;
raw GraphQL extensions do not become public implicitly. Explicit outer error
classification suppresses a wrapped carrier.
