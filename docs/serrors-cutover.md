# Canonical serrors cutover

The cutover stacks on the compatible preview in SDK #1146. It removes the
legacy variadic constructor, mutable error fields, base and validation types,
JSON unmarshal helper and dedicated GraphQL unauthorized constructor.
The preview import is replaced by `pkg/serrors` and its presenter subpackages.

## Reproduction and review

Run the constructor migration against an unmodified candidate checkout:

```sh
go run ./tools/serrors-migrate -root /path/to/sdk -write
go run ./tools/serrors-db-boundaries -root /path/to/sdk
```

The constructor pass resolves the imported package alias and rewrites AST nodes.
It preserves operation context and wrapped causes, classifies explicit legacy
kinds, and reports ambiguous multiple arguments for manual review. It never
infers a public message from internal diagnostic text. A repeated pass on the
final tree reports zero constructors. The DB pass only touches error branches
following driver calls in repository source files; transaction/context/domain
wrappers remain wrappers. Existing explicitly classified errors survive FromDB.

Manual changes supplement the generated pass:

- KindValidation becomes Invalid; known DTO fields use immutable field violations
  and the validation bridge. Employee/counterparty forms retain field names,
  label references, tax details, and locale keys. Import and warehouse errors
  retain concrete types, context, localization keys and business reasons.
- Two-factor/checkpoint constructors previously passed multiple causes to E,
  which discarded all but the last. Multi now preserves both sentinel identity
  and the underlying cause. Unsupported JSON mapping is Unimplemented.
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
