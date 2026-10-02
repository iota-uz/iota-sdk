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
own preparation/disposal; canceled waiters do not block behind preparation.
Shared scenarios in different scopes can prepare concurrently. A dedicated
scenario leases the entire environment until disposal, preventing another scope
from concurrently mutating global settings or baseline records.

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
- An environment-bound registry created by `NewEnvironmentRegistry` also accepts
  `POST /__test__/scopes` with `{ "scopeId": "<environmentId>-<physical-suffix>" }`.
  The credential authorizes allocation only inside that environment namespace.
  Repeating a live reservation is idempotent; a disposed ID cannot be reused.

Example input:

```json
{"name":"invoice","version":"1","scopeId":"attempt-2","seed":"invoice-42","now":"2026-10-02T00:00:00Z","params":{"amount":10}}
```

Example result:

```json
{"name":"invoice","version":"1","scopeId":"attempt-2","seed":"invoice-42","now":"2026-10-02T00:00:00Z","refs":{"invoice":{"kind":"invoice","id":"42"}},"data":{"id":"42"}}
```

There is no API to allocate scopes outside the owning environment or execute
SQL. Mount this handler only inside an isolated test environment. `AllowScope`
is called by its owner; authenticated isolated clients reserve namespaced scopes. Credentials must stay out of replay
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

## SDK ERP pilot

Build production web assets with `just lens build` before starting ERP. Build a
fresh sealed baseline with `go run ./cmd/testenv-baseline --name <unique-name>
--revision <immutable-sdk-revision>`. The command reuses the existing migration
manager and E2E seeder, writes the migration/seed/revision manifest and emits it
as JSON. It never resets an existing database. Supply the admin connection with
`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`; use an isolated local cluster.

Run `cmd/testenv` with the emitted template, schema/baseline fingerprints and
revision, `--sdk-erp`, `--control-token-file <private-local-file>` and application
command after `--` (for example `go run ./cmd/server`). Its authenticated readiness
probe checks `X-Test-Environment-ID`; clone preparation verifies the durable
manifest rather than trusting the requested spec. Each `start` allocates a new
DB/process/port. Stop or close the NDJSON stream to clean owned resources. On
Unix, commands receive their own process group, so descendants of a development
launcher are also stopped. Artifact logs remain for diagnosis.

The existing SDK presets are registered as dedicated version 1 scenarios. They
mutate baseline records and must be disposed before another scenario begins.
Existing reset/populate consumers remain compatible inside their dedicated
database. The full SDK ERP suite fixture migration and throughput benchmark are
not completed by this pilot. Generic Jobs/ProviderControls and injection of the
manual clock into production services also remain separate acceptance work.
