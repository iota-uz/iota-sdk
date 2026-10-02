# Canonical typed errors

`pkg/serrors` is the canonical typed error API. Legacy `E`, mutable error fields,
base errors and validation constructors are removed. SDK #788 is a coordinated
breaking cutover; consumers must migrate before selecting this candidate.

## Classification and identity

Construct errors with `NewInvalid`, `NewNotFound`, `NewPermissionDenied`, or the
other typed constructors. The constructor text is internal diagnostic text.
`New(Code, diagnostic)` is available for repository mappings. `Wrap(op, err)`
adds an operation without changing the effective code; it returns nil for nil.

`CodeOf` finds the first explicit classification from outside inward, traversing
joined branches left to right. Unknown errors classify as Internal. An explicit
outer Internal overrides an inner Invalid. An unclassified first branch does
not hide a classification in a later branch. Context cancellation and deadline
have distinct codes. `HasCode` checks only this effective classification.

Use `errors.Is` and `errors.As` for cause identity and driver types. Equal codes
do not imply equal errors. `Multi` joins causes without discarding identity.
Builders and returned maps, fields, count pointers, and nested message arguments
are copied so shared sentinel errors remain safe to reuse.

```go
err := serrors.Wrap("orders.Create", serrors.NewInvalid("invalid order"))
if serrors.HasCode(err, serrors.Invalid) {
    // The boundary chooses the response for its own contract.
}
```

Compiling examples and precedence/identity tests live beside the package.

## Public messages and fields

`WithPublic(Message{ID, Args, Count})` explicitly authorizes localization data.
`Message{Text: "A safe literal message"}` explicitly declares a public literal
when there is no localization ID. A missing nonempty ID still uses the generic
safe fallback, even when Text is supplied.
Arguments use `Text`, `Number`, `Boolean`, or `Reference` to another Message;
arbitrary objects are not accepted. `WithReason` declares a stable, safe domain
reason, and `WithFields` declares safe field names, reasons, and message IDs.
Do not put SQL, credentials, personal data, or provider response bodies in them.

`Public(err, localizer)` produces only code, authorized reason, localized message,
and safe field violations. Missing or empty translations produce a safe generic
message, never diagnostic text. Generic translations use `Serrors.<code>`;
consumers should provide these in every supported language. A new explicit outer
classification blocks inherited inner public data. `FieldMap` adapts the safe
projection to existing templ form maps without changing field names.

`validate.From` bridges validator.v10 errors using `ValidationErrors.<tag>`
message IDs and optional translated field references. An error without a known
field stays Invalid without an invented field violation.

## Database ownership

`FromDB` accepts a real error. It classifies SQL/pgx no rows as NotFound,
cancellation as Canceled, deadline as Timeout, and unknown errors as Internal.
Known PostgreSQL constraints are mapped only by explicit repository rules:

```go
err = serrors.FromConstraint("products.Create", err, serrors.Constraint{
    SQLState: "23505", Name: "products_sku_key", Reason: "duplicate_sku",
    Message: serrors.Message{ID: "Products.Errors.DuplicateSKU"},
})
```

An owned 23505 constraint maps to AlreadyExists, 23503 to Conflict, and 23502 or
23514 to Invalid. Unknown names and SQLSTATEs remain Internal. Rules must not
reveal objects the caller cannot access. PostgreSQL NOT NULL errors without a
constraint name require an explicitly owned `Table` and `Column` rule for 23502.
A successful UPDATE with zero affected
rows has no driver error: the repository separately decides whether that means
NotFound, optimistic Conflict, or an allowed no-op.

## Boundary adapters

- `serrorhttp.Write` defaults to RFC 9457 problem JSON for explicitly selected
  routes. `Profile` supplies an existing route's status mapping, encoder,
  content type, and headers. Business FailedPrecondition maps to 409 by default;
  an HTTP conditional-request profile can explicitly choose 412. Authentication
  profiles must supply their own `WWW-Authenticate` challenge.
- `serrorhttp.WriteForm` requires a route-owned renderer retaining form state.
  HTMX Invalid errors return 200; the route supplies target and swap. Ordinary
  HTML status, redirect, flash, and trigger policies stay with the route. Never
  copy a diagnostic error into a redirect URL.
- `serrorgql.Presenter` installs through the existing GraphQL error-presenter
  hook. It projects execution errors and preserves path and locations.
  Unauthenticated retains `extensions.code=UNAUTHORIZED`. GraphQL parse and
  validation protocol errors retain their transport classification. Do not
  replace the HTTP handler or force protocol failures to HTTP 200.
- `serrorrpc.Project` is for the dispatcher's general classification branch.
  Explicit typed carriers, middleware/auth, applet sentinels, and numeric
  protocol codes keep precedence. Safe fields map to validation; permission
  denial maps to forbidden. The core `ErrorKind` method remains a compatibility
  classifier rather than a complete transport policy.
- `serrorlog.Attributes` emits code, operation, operation trace, safe reason,
  and a boundary-supplied request ID. It does not emit arbitrary Meta or causes.
  Cancellation logs at debug, ordinary client failures at info, rate limits at
  warn, and dependency/internal failures at error. The boundary owns terminal
  logging, provider metadata allowlists, and any restricted diagnostic sinks.

Timeout and Unavailable never authorize automatic retry of an external write.
The integration workflow owns idempotency and resolution of an unknown outcome.
Specialized MCP/BiChat contracts require their own diagnostic disclosure policy.

## Preview and release

Use the SDK PR with `sdkctl use <PR>` in an isolated consumer preview; run its
contract fixtures on the pinned dependency and the exact preview head. Keep
intentional security/status changes separate from preserved contracts. The
cutover requires the Granite pilot and baseline before removing legacy exports.
After reviewed merges, use the verified release procedure in
[sdk-releases.md](sdk-releases.md). PR creation alone does not verify a release
or authorize deployment.
