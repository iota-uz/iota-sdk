# Browser test environments

`testenv` is opt-in infrastructure. Production bootstrap does not mount its
routes or register control credentials. Consumer scenario handlers keep their
domain vocabulary and call real services and repositories.

## Environment ownership

`NewCoordinator(adapter)` assigns an environment ID before calling the adapter.
`Start(ctx, Spec)` waits for `Adapter.Ready`, then checks descriptor fingerprints,
the URL and all required capabilities. Repeating the same run/slot/spec returns
the live descriptor; changing it, or reusing a stopped slot, returns
`resource_conflict`. A new retry uses a new slot and physical scope, retaining
its logical scenario seed.

The adapter must own every process, database, upload directory, queue namespace
and fake instance it allocates. It records partial resources by environment ID
before returning a startup failure. `Stop` cleans only these resources. The
coordinator attempts bounded cleanup even after startup context cancellation;
failed cleanup remains retryable with `Stop`.

Descriptors contain no credentials. Return test identities and the runtime
control token over a separate local channel. `pkg/dbctl/testdb` shares ITF's
database engine without depending on the testing harness. Its context-aware
`Create`/`Clone` never replace an existing database. Build a versioned baseline,
close its pools, then `Seal` it before cloning. Global roles are not cloned.

## Scenario registry

```go
registry := testenv.NewRegistry([]string{"postgres", "napp"}, true)
err := registry.Register(testenv.Definition{
    Name: "invoice", Version: "1", Isolation: "dedicated",
    InputSchema: map[string]any{"type": "object"},
    OutputSchema: map[string]any{"type": "object"},
}, prepareInvoice, disposeInvoice)
// The environment owner grants a scope before handing it to a test client.
err = registry.AllowScope(scopeID)
handler, err := testenv.NewHandler(registry, controlToken)
```

Schemas use JSON Schema 2020-12 and are compiled at registration. Preparation
validates version, capabilities, isolation and input before calling the handler;
output data is validated before publishing a result. Each scope serializes its
own preparation/disposal; different scopes can prepare concurrently.

The same canonical JSON input in a ready scope returns the saved result without
calling the handler again, including after a lost HTTP response. Another input
conflicts. Failed preparation runs compensating cleanup and remains unready;
dispose it and allocate a new physical scope before retrying. Cleanup must be
idempotent and must cover committed records, provider scripts, queues and files.
Handlers own transaction boundaries: publish a DB graph atomically and do not
return before its async requirements are durable.

The Go API and HTTP adapter share the same registry. HTTP requires an explicit
control token of at least 32 bytes in `Authorization: Bearer ...`:

- `GET /__test__/scenarios` returns registered definitions.
- `POST /__test__/scenarios/prepare` accepts `Input` and returns `Result`.
- `DELETE /__test__/scopes/{scopeId}` disposes an owned scope idempotently.

Example input:

```json
{"name":"invoice","version":"1","scopeId":"attempt-2","seed":"invoice-42","now":"2026-10-02T00:00:00Z","params":{"amount":10}}
```

Example result:

```json
{"name":"invoice","version":"1","scopeId":"attempt-2","seed":"invoice-42","now":"2026-10-02T00:00:00Z","refs":{"invoice":{"kind":"invoice","id":"42"}},"data":{"id":"42"}}
```

There is no HTTP API to create arbitrary scopes or execute SQL. Mount this
handler only inside a dedicated test environment. `AllowScope` is called by
its owner, not by untrusted clients. Credentials must stay out of replay
manifests, traces and scenario data.

## Deterministic controls

Inject `Clock` into actual time-sensitive services. `ManualClock.Advance` crosses
boundaries without sleeping; it does not automatically change SQL `NOW()` or
browser time. SQL queries must take explicit `asOf` where relevant.

`Gate` lets an actual provider adapter signal `Entered`, wait, and continue
only after `Release`; context cancellation also ends the wait. Release is
idempotent. A provider's scope cleanup must release all its gates. Provider
schemas, actual job handlers and conditions remain consumer-owned.

## Verification

`go test -race ./pkg/testenv` tests concurrent replay, credential/schema checks,
scope conflicts, compensation and lifecycle ownership. Tests characterize the
new API; they are not described as regressions of previously shipped endpoints.

Set `TESTENV_POSTGRES_DSN` to a dedicated cluster to run
`go test -race ./pkg/dbctl/testdb`. It creates UUID databases, checks non-public
schema cloning, concurrent destinations and non-destructive destination conflict.
It does not use or reset any existing consumer database.
