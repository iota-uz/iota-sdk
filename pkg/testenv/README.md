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

Controls attach to a registry. They share its scope ownership: every call names
a live physical scope, and `Dispose` forgets each attached control (in reverse
attach order) before running scenario cleanup. A failed control cleanup returns
`cleanup_failed` and leaves the scope retryable.

```go
clock := testenv.NewManualClock(time.Now())
registry.UseClock(clock)                 // dedicated only; inject clock into real services
jobs, _ := testenv.NewJobs(registry)
providers, _ := testenv.NewProviders(registry)
files, _ := testenv.NewScopedFiles(registry, artifactDir)
_ = registry.OnDispose("queues", func(ctx context.Context, scope string) error {
    return streams.Delete(ctx, testenv.QueueNamespace("jobs", scope))
})
registry.SetBuildInfo(testenv.BuildInfo{BuildRevision: rev, SchemaFingerprint: s, BaselineFingerprint: b})
```

**Correlation.** `WithCorrelation`/`CorrelationFrom` carry `{scopeId, executionId}`
through contexts. The registry passes it to scenario handlers and jobs, and
provider fakes read it. An HTTP header alone does not isolate anything, so persist
the scope ID on async records and propagate the context into workers.

**Clock.** `ManualClock` must be injected into the actual time-sensitive services;
production keeps `SystemClock`. When a clock is attached, preparing a dedicated
scenario sets it to `Input.Now`. Only the scope holding the dedicated lease may
advance it (`POST /__test__/scopes/{id}/clock/advance {"durationMs":n}`). Shared
environments answer `missing_capability`. The clock does not change SQL `NOW()`:
queries whose temporal semantics are tested take an explicit `asOf` from the
clock. Freezing only browser time does not count as controlling server time, so
read `GET /__test__/clock` and pass it to Playwright's `page.clock`.

**Jobs.** `jobs.Register(JobDefinition{Name, Dedicated}, handler)` registers the
actual handler or worker invocation, not a substitute result. `RegisterCondition`
adds a named check over durable state. `Run(ctx, scope, job, refs)` returns an
execution ID. `Wait(ctx, scope, execution, condition, timeoutMs)` takes
`timeoutMs` as an integer in 1..120000 and returns `reached`, or a typed
`execution_failed`/`timeout` error whose `Details` holds the `WaitResult` (state,
checks, last condition error). When the job fails, the wait fails immediately.
When the job succeeds without reaching the condition, conditions keep being
observed until the deadline, so downstream async work can converge. Canceling
the wait context stops waiting and reports the execution state. Dedicated jobs
(periodic tasks, settings) are refused in shared environments. `ConditionCompleted`
waits for the execution itself.

**Providers.** A consumer fake declares `ProviderDefinition{Name, ResponseSchema}`
and calls `providers.Call(ctx, provider, operation, key)` from inside its adapter.
Scripts (`{"steps":[{"operation","outcome":"success|refusal|timeout","hold","response"}]}`)
are validated against the declared schema; success steps must carry a
`response`. Each call consumes the first queued step whose `operation` is
empty or equal to the call's operation, so steps for different operations are
not globally ordered (`send`, `refund` queued: a `refund` call takes the second).
`Calls` returns `sequence, operation, key, outcome, correlation`. A step with
`hold` records the call as `held` and waits until `Release`, the call's context
ends, or the scope is forgotten. `WaitHeld` lets a test act while that call is
in flight, which makes races deterministic. A call with no matching step, or no
scope correlation, gets `ErrUnscripted`/`ErrUncorrelated` instead of an invented
success. `ForgetScope` ends held calls and removes only that scope's scripts and calls.

**Files and queues.** `ScopedFiles.Dir(scope)` is removed on dispose. Queue/stream
namespaces derived with `QueueNamespace` are removed by an `OnDispose` hook.

HTTP routes (same credential and ownership checks): `GET /__test__/controls`,
`GET /__test__/clock`, `POST /__test__/scopes/{id}/clock/advance`,
`POST /__test__/scopes/{id}/jobs`, `GET /__test__/scopes/{id}/executions/{x}`,
`POST /__test__/scopes/{id}/executions/{x}/wait`,
`POST|GET /__test__/scopes/{id}/providers/{p}/scripts|calls`,
`POST /__test__/scopes/{id}/providers/{p}/gates/{g}/wait|release`,
`DELETE /__test__/scopes/{id}/providers`. `testenv.Routes()` lists all routes
for hosts that register them explicitly.

## Manifests and replay

`GET /__test__/scopes/{id}/manifest` (`Registry.Manifest`) returns the logical
seed, scenario/version/params/time, build and baseline fingerprints, actual
result identifiers, the controlled clock, and snapshots of attached
`SnapshotResource`s (provider scripts as submitted by the test; scripts the
scenario callback submits itself are recreated by replay and left out).
Credentials are never part of it: control tokens and test identities go to the
isolated client separately, and scenario params must not carry secrets. Export
is opt-in per provider: only providers registered with `CredentialFree: true`
are exported; a scope holding scripts of any other provider refuses export.
A manifest whose indented JSON exceeds `MaxManifestBytes` (4 MiB) is refused at
export, which is also the HTTP and CLI replay limit.

`POST /__test__/scopes/{id}/replay` (`Registry.Replay`) prepares the same
logical input in a new physical scope, then restores controls and the clock.
A manifest from another build or baseline, or replay in an environment that
does not declare its fingerprints, returns `version_mismatch`. Repeating the
same replay returns the same result without re-applying controls. If a restore
fails, the scope's controls are dropped and it must be disposed.

Business identifiers that come from the logical seed reproduce across fresh
environments. When their space is small, handlers must check occupancy and
probe for a free value. Hashing is not a uniqueness guarantee.

## Go, CLI and HTTP

`testenv.Control` is the shared surface. `testenv.Local(registry)` is the
in-process Go implementation; `testenv.NewClient(baseURL, token, environmentID)`
calls the HTTP routes and checks the environment header. `pkg/testenv/cli.Run`
implements `scenarios | prepare | dispose | manifest | replay` over any
`Control`, so a consumer binary can embed it with its own registry.
`cmd/testenv-scenario` runs it against an environment over HTTP:

```sh
go run ./cmd/testenv-scenario --base-url "$URL" --environment-id "$ENV" \
  --control-token-file token prepare --scope "$ENV-a" --manifest out.json < input.json
go run ./cmd/testenv-scenario ... replay --scope "$ENV2-a" --manifest out.json
```

## Verification

`go test -race ./pkg/testenv/...` tests concurrent replay, credential/schema checks,
scope conflicts, compensation, lifecycle ownership, jobs, provider gates, clock
control, scoped cleanup, manifests and Go/CLI/HTTP equivalence. With
`TESTENV_POSTGRES_DSN`, `TestPostgresScenarioControlsEndToEnd` drives the real
HTTP adapter over owned UUID databases. It covers version/schema/output drift, staged
partial failure and compensation, concurrent prepares, a gated dunning race, a
clock boundary, replay on a fresh database, and identifier collisions. Point the
variable at an isolated cluster to run it; without it the test is skipped. Tests characterize the
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
database; the legacy `/__test__/reset` truncation runs only when a consumer
calls it, never as a default of the scoped registry. SDK ERP records the
environment's build/baseline fingerprints (passed by `cmd/testenv`) for
manifest replay. Injecting the manual clock into specific production services
is per-consumer wiring.

`--sdk-erp` also gives each environment its own upload root
(`<artifactDirectory>/uploads`), an OIDC issuer bound to its port when
`OIDC_CRYPTOKEY` is set, and removes inherited `REDIS_*`/`RATELIMIT_REDIS*`
variables: SDK ERP has no per-environment Redis key namespace, so isolated
processes run without Redis instead of sharing streams with another worker.
`process.Config.Unset` is the generic form of that filter. `--database-prefix`
names owned clones `<prefix><environment id without dashes>`.
`cmd/testenv-baseline --drop <name>` drops a baseline the command built.

The SDK ERP Playwright suites (`e2e/`) use this lifecycle locally and in CI:
global setup builds one sealed baseline per run, and each worker owns a
`cmd/testenv` driver, a cloned database and a server process. See
`e2e/README.md` ("Environment lifecycle").
