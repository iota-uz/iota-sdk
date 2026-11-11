# Running the extension runtime in an application

This is the integrator's guide: how an application built on iota-sdk turns the extension runtime on, what it contributes
to it from Go, and what it operates afterwards. [`authoring.md`](authoring.md) is the package author's manual (TypeScript);
[`../extension-runtime.md`](../extension-runtime.md) is the contract between the platform's own slices.

What is verified and what is not. The configuration, the composition (`pkg/extruntime/wiring`) and the path
"enable by environment → activate a package → call its command" are exercised by tests in this repository: the unit tests of
the wiring and the config, and `cmd/server`'s `TestServerComposition_ExtensionRuntimeSmoke`, which composes the server's
real component set on PostgreSQL, runs the Node workers of the built `@iota-uz/sdk` package and drives `/_ext/manage` and
`/_ext/rpc`. The browser side of an application (its own frontend bundle with the runtime host entry) is exercised by the
browser suite of §11 against a host bundle built from the SDK's entry with a minimal recipe, not by an application's real
frontend build; the other tests use a stub host manifest. The sections on it say what the server requires and no more.

## 1. Enable the runtime

### 1.1 What the deployment needs

| Requirement | Detail |
|---|---|
| Go module | `github.com/iota-uz/iota-sdk` at release *R* (`pkg/sdkidentity.ReleaseVersion`). |
| `@iota-uz/sdk` | The npm package at **exactly** *R*, installed with its dependencies (the build worker needs the native `esbuild` package). The Go module and the package are one release unit ([`../client-host-release-train.md`](../client-host-release-train.md)); update `go.mod`, `go.sum`, `package.json` and the JS lockfile together. |
| Node.js | `>= 20.19.0` (the package's `engines`), on `PATH` or named by `EXTRUNTIME_NODEBINARY`. `npm` as well when packages have their own dependencies (builds run `npm ci --ignore-scripts`). |
| PostgreSQL | The application's own database. The runtime keeps its tables in schema `extruntime` and one schema `x_<namespace>` per package; both are created by migrations/activation, never by the Node workers. |
| `git` | Only for importing packages from Git (`/_ext/manage/import/git`). |

The workers are **not** embedded in the Go binary and are never taken from the Go module cache. They are read from the
installed package: `dist/runtime/host/worker.mjs` (the invocation pool) and `dist/runtime/build/worker.mjs` (check, build,
test, developer kit). At startup `runtime.ResolveDist` verifies that `package.json` names `@iota-uz/sdk` at version *R*,
that `dist/sdk-identity.json` carries release *R*, a full source commit and a compatible protocol, and that both workers
exist. Every failure names the file it read and the fix. The SDK servers do this while composing the application, so a
bad installation stops the process at start instead of failing the first invocation.

Shipping the package next to the binary (Dockerfile sketch):

```dockerfile
FROM node:22-bookworm-slim AS js
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev          # installs @iota-uz/sdk@<R> and its esbuild

FROM golang:1.24 AS go
# ... go build -o /out/server ./cmd/server

FROM node:22-bookworm-slim
WORKDIR /app
COPY --from=js /app/node_modules ./node_modules
COPY --from=js /app/package.json ./package.json
COPY --from=go /out/server /app/server
ENV EXTRUNTIME_ENABLED=true EXTRUNTIME_APPROOT=/app EXTRUNTIME_CACHEDIR=/tmp/extruntime
CMD ["/app/server"]
```

The Node image supplies the Node requirement; a Go-only base image cannot run the workers. When `node_modules` is not under
the application root, point `EXTRUNTIME_DISTDIR` (or `IOTA_SDK_DIST`) at the `@iota-uz/sdk` directory itself.

### 1.2 Configuration

The section is `pkg/config/stdconfig/extruntimeconfig` (prefix `extruntime`, one underscore per dot). It is
**disabled by default**; while `EXTRUNTIME_ENABLED` is not `true` nothing else in the section is validated or used, and the
application composes exactly as before.

| Variable | Default | Meaning |
|---|---|---|
| `EXTRUNTIME_ENABLED` | `false` | Turns the runtime on. |
| `EXTRUNTIME_POOLSIZE` | `2` | Node worker processes (1–64). See §1.4. |
| `EXTRUNTIME_NODEBINARY` | `node` | Node executable of the workers. |
| `EXTRUNTIME_APPROOT` | working dir | JS root holding `node_modules/@iota-uz/sdk`. |
| `EXTRUNTIME_DISTDIR` | `$IOTA_SDK_DIST`, else `<APPROOT>/node_modules/@iota-uz/sdk` | The `@iota-uz/sdk` package directory. |
| `EXTRUNTIME_CACHEDIR` | `<tmp>/iota-extruntime` | Where artifacts are materialized for the workers and the asset route. A cache rebuilt from the database: it need not persist or be shared between instances. |
| `EXTRUNTIME_WORKFLOWCONCURRENCY` | engine default | Workflow instances run at once. |
| `EXTRUNTIME_DISABLEBACKGROUND` | `false` | Keeps the workflow engine, event dispatcher and scheduler of this process off. The API, endpoints and RPC are unaffected. |
| `EXTRUNTIME_UI_HOSTMANIFEST` | – | Path of the client-host manifest JSON of the application's frontend build. Setting it mounts the UI slice: package pages, navigation, shell bindings, `/_ext/rpc` and `/_ext/assets`. Without it the browser RPC does not exist. |
| `EXTRUNTIME_UI_SOURCETTL` | `2s` | Cache of the activated-UI snapshot. |
| `EXTRUNTIME_CONFIGURATOR_ENABLED` | `false` | Mounts the configurator page. Requires the host manifest. |
| `EXTRUNTIME_CONFIGURATOR_PATH` | `/extensions` | Its path. |
| `EXTRUNTIME_CONFIGURATOR_TYPESDIR` | `<APPROOT>/node_modules` when present | Tree Monaco's type information is served from. |
| `EXTRUNTIME_ENDPOINTS_PUBLICTENANT` | – | Tenant ID for requests without a session (webhooks). See §5. |
| `EXTRUNTIME_ENDPOINTS_PUBLICTENANTBYHOST` | `false` | Resolve that tenant from the `Host` header against `tenants.domain`. Exclusive with the above. |
| `EXTRUNTIME_ENDPOINTS_MAXBODYBYTES` / `_TIMEOUT` | 1 MiB / 30s | Body and invocation bounds of package endpoints. |
| `EXTRUNTIME_GIT_ALLOWEDSCHEMES` / `_BINARY` / `_TIMEOUT` | `https,ssh` / `git` / `2m` | Package imports from Git. `file` lets a caller read any repository on the server. |
| `EXTRUNTIME_GIT_ALLOWEDHOSTS` | – (any host with public addresses) | Comma-separated hosts https/ssh imports may use: `git.example.com` or `*.example.com`. **Set it in production.** |
| `EXTRUNTIME_GIT_ALLOWPRIVATENETWORKS` | `false` | Lets imports reach loopback, link-local (cloud metadata), private, shared (CGNAT) and reserved addresses. Leave off unless the Git server is on your own network. |

Rights: the management API and the configurator authorize every action against `Extensions.Read`, `Extensions.Author`,
`Extensions.Review`, `Extensions.Publish` and `Extensions.Operate` (`modules/core/permissions`). Grant them to the roles that
author, review, publish and operate; nothing in the runtime bypasses RBAC.

### 1.3 The SDK servers

`cmd/server` and `cmd/worker` append one component, built by `pkg/extruntime/wiring`, to the SDK modules:

```go
wiring.NewComponent(wiring.Options{Natives: modules.Natives()})
```

Its `Build` reads the section from the config source. Disabled: it logs one line and contributes nothing. Enabled: it
composes `runtime.NewComponent` with the native set of `modules.Natives()`, verifies the `@iota-uz/sdk` installation,
loads the host manifest, and the runtime starts and stops with the container's lifecycle hooks:

* `cmd/server` (API + worker capabilities): management API, package endpoints, UI slice/configurator when configured, and
  the background parts (catalog listener, workflow engine, event dispatcher, scheduler, notification adapter).
* `cmd/worker` (worker capability only): the background parts, so workflows, events and schedules can run in a dedicated
  process. Run it with the same `EXTRUNTIME_*` settings and the same `@iota-uz/sdk`. Workflow instances are claimed with
  row locks; event and schedule delivery is not designed around several dispatchers on one database, so when you run a
  dedicated worker set `EXTRUNTIME_DISABLEBACKGROUND=true` on the API processes (this repository does not test several
  background processes against one database).

The SDK servers publish no MCP tools: they host no MCP gateway (§7).

### 1.4 Pool sizing

The pool is `EXTRUNTIME_POOLSIZE` Node processes, **one invocation at a time each**; further invocations wait in an
unbounded FIFO. Every package invocation takes a slot: browser RPC, package endpoints, event subscriptions, schedules, workflow runs,
guards, replacements of native commands and the contact resolvers and delivery policies of packages. (The Go defaults of
native contracts run in the Go process and use none.) Nested calls
(`tx.call`, `ctx.call`) reuse the parent's slot, so a parent can never deadlock the pool. Builds, checks and tests run in
separate build workers (two at a time) and do not use the pool.

Size it to the peak number of invocations that are *running* together, not requests per second: an invocation that waits on
an external HTTP call holds its slot while it waits, so a package that calls slow services needs more slots than one that
only reads and writes. Start with the default, watch `started_at`/`finished_at` of `extruntime.invocations` (and the
queue behind the pool), and raise it when invocations wait under load. Each worker is a full Node process with its own heap; the
workers never inherit the host's environment.

## 2. Composing the runtime in an application

An application's `main` follows `cmd/server`. Use `wiring.NewComponent` when the EXTRUNTIME_* section should drive it:

```go
natives := append(modules.Natives(), app.Natives)          // the SDK modules' natives, then the application's
components := append(modules.Components(), app.Component(), wiring.NewComponent(wiring.Options{
    Natives:     natives,
    Secrets:     appSecrets,           // host.SecretResolver for secrets.get
    MCPRegistry: mcpRegistry,          // nil when the application has no MCP gateway (§7)
    Tune: func(cfg *runtime.Config) error {
        cfg.Events.PollInterval = time.Second       // Go-only settings: events, schedules, cfg.Configurator.Types, ...
        return nil
    },
}))
```

`wiring.Options.Section` supplies the section as a value instead of the config source (tests, embedded setups).
`wiring.RuntimeConfig(section, opts)` returns the `runtime.Config` and the verified `runtime.Dist` for applications that
construct `runtime.NewComponent(cfg)` themselves, as `pkg/extruntime/e2e` does.

The runtime builds on the `core` module (notification services, users, permissions) and resolves from the container what it
needs: `*pgxpool.Pool`, `*services.NotificationDispatchService`, `*notifications.Catalog`, `ContentRenderers`,
`ContactResolvers`, `DeliveryPolicies`, `user.Repository`. It provides `*runtime.Runtime`, `*manage.Service`,
`*catalog.Catalog`, `spi.Catalog` and `native.Dispatcher`.

## 3. Contributing the application's natives

A **native** resource is one the Go application owns and the runtime only publishes: entities, lists, events, operations,
commands, queries, routes and plain declarations such as slots. Packages see them through the developer kit
(`@iota-kit/<namespace>`), reference them, subscribe to them, guard and (for commands) replace them, but never write
around them. Each module exports one `native.Contributor`; the application composes them once:

```go
// app/natives.go
func Natives(s *native.Set) error {
    return s.Add(
        native.Entity(TicketEntity),            // read-only table packages query; writes go through operations
        native.List("app.lists.tickets"),       // a list packages extend with columns
        TicketClosedEvent,                      // native.Event[Payload]
        CloseTicket,                            // native.Operation — the invariant behind every close
        SaveTicket,                             // native.Command   — the published, replaceable intent
        native.Route("app.tickets", "/tickets"),
        native.Declare(manifest.Resource{ID: "app.slots.ticketSidebar", Kind: manifest.ResSlot, Props: ticketSidebarProps}),
    )
}
```

The CRM module is the reference consumer: `modules/crm/natives.go`, `modules/crm/services/client_commands.go`,
`client_events.go`, `modules/crm/extension` and `modules/core/services/user_events.go`. The pieces:

* **Entities** (`native.Entity(data.NativeEntity{...})`): table, key column and type, tenant column, typed fields,
  `ReadPermission` and per-field `FieldPermissions`. The generic data API reads them with the viewer's rights; a generic
  write is refused (`data.nativeWrite`).
* **Lists** (`native.List(id)`): the ID packages extend with `column(...)` contracts and overlays. The Go list/export must
  consult `pkg/listext` for the contributed columns, filters and sorting (CRM's `ClientController` is the reference).
* **Events** (`native.Event[P](id, native.EventSpec{Subject, Description})`): versioned IDs (`app.ticket.closed.v1`) with a
  payload struct whose wire type is derived by `native.TypeOf` (int64 fields need `wire:"int64"` and `,string`; floats,
  maps and interfaces are refused). Publish them **in the write transaction**:

  ```go
  _, err = events.Publish(ctx, tx.DB(), spi.Scope{Subject: spi.Subject{TenantID: tenant, UserID: actorID, Kind: spi.SubjectInitiator}},
      TicketClosedEvent, &ref, TicketClosedPayload{TicketID: id})
  ```

  The row commits or rolls back with the change it reports; subscribers run after the commit with a receipt each. Anything
  that must happen only after the commit and is not a package concern (the module's own eventbus events) goes in
  `tx.AfterCommit`.
* **Operations and commands** (`native.Operation | AtomicCommand | Command | Query[In, Out](id, native.Spec{Permission, ...}, impl)`):
  `impl` is a factory over `native.Deps` (the container; `native.Use[*TicketService](d)`) that returns the typed Go default.
  *Operations* hold the reusable invariants (authorization, validation, persistence, history, events) and run in the caller's
  transaction; *atomic commands* are public contracts that run in one transaction; *ordinary commands* may call external
  systems and open their own transactions. `Spec.Permission` is enforced by the host on every invocation, whichever
  implementation is bound. Inside a handler `ctx` carries the transaction (`composables.UseTx`), so repositories join it.
* **Routes and declarations**: `native.Route`/`native.RouteWith` publish a page route (a package page can replace the page
  when an administrator binds it); `native.Declare` adds slots and permissions. Pages render slots with `shellext.Slot`.

**Every entry point goes through the host.** A native command/operation that has a Go default also has an owner-installed
replacement path (`/_ext/manage/bindings`) and package guards. They only take effect for callers that dispatch through the
host, so the application's controllers, services, jobs and importers call the contract, not the underlying service:

```go
// the runtime provides native.Dispatcher; without it ResolveDispatcher falls back to the Go defaults in-process
dispatcher, err := native.ResolveDispatcher(container, ticketContracts)      // fallback set: your contracts alone
out, err := SaveTicket.Dispatch(r.Context(), dispatcher, SaveTicketInput{...})
```

`HostDispatcher` acts as the session user (`runtime.SubjectFromContext`); a dispatch error keeps its Go chain
(`native.AsError` for the structured kinds: validation, denied, domain). Background code with no session builds its own
`runtime.HostDispatcher{Subject: ..., Trigger: host.TriggerSchedule}`.

## 4. Notifications: providers, contacts, policy

The runtime sends through the core module's notification platform; the application composes the providers in
`core.ModuleOptions` (replace the `core` component of `modules.Components()` with your own options):

```go
core.NewComponent(&core.ModuleOptions{
    PermissionSchema:    defaults.PermissionSchema(),
    NotificationChannels: []notifications.ChannelAdapter{emailAdapter, smsAdapter, pushAdapter},   // Channel() + Send()
    NotificationWebPush:  &webPushOptions,                                                         // webpush.FromEnv(os.LookupEnv)
    NotificationContactResolvers: map[string]notifications.ContactResolver{"crm.client": clientContacts},
    NotificationDeliveryPolicy:   businessConsentPolicy,                                           // consulted first
    NotificationAddressResolver:  addresses,
})
```

* **Channel providers** are `notifications.ChannelAdapter` values for `email`, `sms` and `push` (in-app is built in). An SMS
  gateway such as `pkg/eskiz` is wrapped in an adapter; it is not a channel by itself. Tests use
  `notificationstest.NewSMS()/NewEmail()/NewPush()`.
* **Contact resolvers** turn a record (`recipient.record(ref)`) into targets, per entity. Go resolvers are registered in
  `NotificationContactResolvers` or at runtime on `*notifications.ContactResolvers`; a package can add its own
  (`contactResolver(...)`), run as the trigger owner of the resolver.
* **Delivery policy** is composed: the Go policy of `NotificationDeliveryPolicy` is consulted first, then the policies of
  active packages ordered by policy ID and package; any denial wins and is stored as a skipped delivery. A package policy
  needs a trigger owner or its deliveries wait (§6.3). The SDK has no consent schema; the application's Go policy or a
  package that owns the consent data decides.

## 5. Public endpoints and their tenant

`/_ext/endpoints/<namespace>/<path>` serves a package's endpoints. Authenticated requests run as the session user; a
`public` endpoint runs as the **trigger owner** of its tenant (§6.3). A webhook carries no session, so the tenant must come
from somewhere:

1. `EXTRUNTIME_ENDPOINTS_PUBLICTENANT=<tenant uuid>` — single-tenant deployments.
2. `EXTRUNTIME_ENDPOINTS_PUBLICTENANTBYHOST=true` — the tenant whose `tenants.domain` equals the request's `Host` (port and
   case ignored). A host naming no active tenant, or two, resolves to nothing, so the endpoint answers 503
   `endpoint.noTenant` instead of acting in a guessed tenant. Answers are cached for 30 seconds.
3. `wiring.Options.PublicTenant` / `runtime.EndpointConfig.PublicTenant` — a function of the request, which wins over both.
4. Neither: the tenant the application's middleware put in the request context; without one the answer is 503.

## 6. Operating it

All of this is `/_ext/manage` (the configurator and the MCP tools call the same service), audited, and authorized per
action.

### 6.1 Package lifecycle

draft → `check` → `build` → `test` → `publish` (immutable version) → `review` (bound to the exact source and artifact
hashes) → `activate`. Activation applies the package's schema (additive; incompatible changes are refused), starts its
resources, subscriptions, schedules and endpoints, and replaces the previous active version atomically.

* **Deactivate** (`POST /packages/{ns}/deactivate`, `Extensions.Publish`) takes the package out of service: its resources
  leave the catalog, its bindings end (a replaced native command falls back to its Go default), its subscriptions and
  schedules stop. **Tables and data are kept**; workflow instances of the package block with `packageDeactivated` and
  continue after a new activation (`resume`).
* **Purge** (`POST /packages/{ns}/purge`, `Extensions.Publish`) deletes the data: the package's entity tables and the
  recorded schema state. It is refused while the package is active or live instances still pin it. It cannot be undone;
  deactivate is the default way to switch a package off.
* **Git and export**: `POST /import/git` (a commit; schemes per `EXTRUNTIME_GIT_ALLOWEDSCHEMES`), `/import/tree`, and
  `GET /packages/{ns}/export` return the draft as a tree to commit. Source control is the author's. An import is an
  outgoing request made by the server on a caller's behalf, so it is bounded: the host must match
  `EXTRUNTIME_GIT_ALLOWEDHOSTS` (when set), is resolved by the server, and every address must be public (unless
  `EXTRUNTIME_GIT_ALLOWPRIVATENETWORKS`); https connects to the addresses that were checked and does not follow
  redirects. Only the requested commit is fetched (`--depth=1`); a server that will not serve an unadvertised commit
  is asked for branch tips only, so the commit must then be one. The download, each file and the whole tree are
  size-limited (128 MiB tree, 8 MiB per file, 5000 files) and the sizes are checked before any content is read.
  Failures return stable reasons (`git_destination_denied`, `git_fetch_failed`, `git_commit_not_found`,
  `git_too_large`, `git_unexpected_output`); what Git itself printed is only in the server log.

### 6.2 Events: requeue

A subscription delivers each event with a receipt (`done`, `failed` with backoff, `dead`, `blocked`). `blocked` means the
subscription's owner is missing or refused; `dead` means the attempts ran out. After the cause is fixed,
`POST /subscriptions/{subscription}/requeue` (`Extensions.Operate`; body `{"event": "<id>"}` for one event) turns the
tenant's dead and blocked receipts back into due deliveries with no attempts counted, and the dispatcher retries them with
the subscription's current handler. Infrastructure outages (database, lost worker) never dead-letter an event. Schedule runs
have no requeue: a slot older than 15 minutes is skipped by design.

### 6.3 Trigger owners

A schedule, a subscription on a user-less event, a contact resolver, a delivery policy and a public endpoint run as the
**owner-user** assigned for the tenant (`PUT /triggers/{trigger}/owner`, `{"userId": 42}`, `Extensions.Publish`;
`DELETE` removes it; `GET /owners` lists). The trigger runs with the owner's **current** rights. Assigning is lending
permissions out, so the actor may name themselves or a user whose every permission the actor holds too. A tenant without an
owner gets *blocked* runs, not a failed activation. Blocked or deleted owners block their triggers until replaced.

### 6.4 Workflows and blocked instances

`GET /workflows` lists instances; `GET /workflows/{id}` returns the history. Statuses: `running`, `waiting`, `completed`,
`failed`, `terminated`, `blocked`. A blocked instance carries a reason, and the action that fits it (`Extensions.Operate`):

| Blocked because | Action |
|---|---|
| a call or step is `ambiguous` (a lost worker or an outage left an ordinary command's outcome unknown), `nondeterminism` | `POST /workflows/{id}/resolve` `{name, n, result}` completes the occurrence with the result you verified; or `retry` `{name, n}` forgets it so the next run executes it again; then `POST /workflows/{id}/resume` |
| `denied`, owner/user blocked | fix the rights or the owner, then `resume` |
| `packageDeactivated` | activate a version of the package, then `resume` |
| unfixable | `POST /workflows/{id}/terminate` (completed effects stay), then `start-v2` to begin a new instance on the active (or a named) version |

`signal`, `tasks/{id}/decide` deliver signals and human-task decisions with the caller's current rights.

## 7. MCP: personal credentials map to users

The management service is available as MCP tools (`ext_package_*`, `ext_check|build|test`, `ext_publish`, `ext_review`,
`ext_activate`, `ext_bind`, `ext_owner_*`, `ext_workflow_*`, `ext_subscription_requeue`, `ext_invoke`, the kit and catalog
tools, ...). The SDK registers them into the application's shared `pkg/mcp` registry; **the SDK servers have none**, because
the MCP protocol adapter (`mcpgateway`) is a separate Go module an application composes itself:

```go
registry := mcp.NewRegistry()
// ... the application's other tools ...
components = append(components, wiring.NewComponent(wiring.Options{Natives: natives, MCPRegistry: registry}))

engine, _ := mcp.NewEngine(registry, rbacauth.New(loadUser))       // loadUser reads the account fresh on every call
server, _ := mcpgateway.NewServer(engine, impl, nil)
handler := mcpgateway.OAuth{ /* Issuer, KeySet, Accounts: AccountResolverFunc(principalOf) */ }.Handler(server, nil)
```

The tools are registered when the runtime starts (`mcptools.Extension`), so build the engine and the gateway after the
registry is filled, or from the container after start.

Identity is never a shared bearer. The OAuth access token (or API key) of a **person** resolves, through the application's
`AccountResolver`, to `mcp.Principal{TenantID, UserID, ClientID, Scopes}`. `rbacauth` re-reads that account on every call
(a blocked user or revoked permission is effective immediately), checks the grant's scopes
(`extensions:read|manage|invoke`, which only narrow RBAC) and puts the user into the context; the tools derive the acting
subject from it, so the audit row names the user as actor and `mcp:<client>` as credential. A service principal cannot
manage extensions. A separate public MCP (for example a read-only analytics assistant) is simply another registry/engine;
it does not need these tools.

## 8. The configurator and its client bundle

`EXTRUNTIME_CONFIGURATOR_ENABLED=true` mounts `/extensions` (Monaco editor, versions, review/activation, bindings,
workflows). The page is a Solid screen of **the application's** client host, not a second host, so the application's frontend
build must include it:

1. **The host bundle and its manifest.** The frontend build (Vite) bundles the runtime host entry
   (`node_modules/@iota-uz/sdk/dist/runtime/client-host/entry.js`: it mounts package pages, slots, cell renderers and the
   shell) and the application's Solid features, and emits a `clienthost.Manifest` JSON (`package`, `packageVersion`,
   `sdkCommit`, `sdkReleaseVersion`, `protocolVersion`, `entry`, `styles`, `integrity`). Its release/commit/protocol must
   be the Go SDK's; `EXTRUNTIME_UI_HOSTMANIFEST` points at it and a stale bundle fails at startup.
2. **`solidFeatures` merge.** The configurator's screen is generated into
   `pkg/extruntime/configurator/web/solid-features.generated.ts` (`solidFeatures`, a `SolidFeatureCatalog`). The
   application merges it with its own catalog (`iota-solid` generates the app's) and passes the result to
   `mountSolidFeatureCatalogFromDocument`:

   ```ts
   import { solidFeatures as configurator } from "<path to iota-sdk>/pkg/extruntime/configurator/web/solid-features.generated"
   import { solidFeatures as app } from "./solid-features.generated"
   mountSolidFeatureCatalogFromDocument({ catalog: { ...app, ...configurator } })
   ```

   The path depends on how the application vendors the SDK sources; this repository does not ship a ready bundler recipe.
3. **`monaco-editor`.** A peer of the configurator (`^0.52`): install it in the application's JS workspace together with
   `solid-js`; the editor loads lazily on first use.
4. **Types.** `Config.Types` is a `fs.FS` shaped like `node_modules` that gives Monaco the types of `@iota-uz/sdk`,
   `solid-js` and `csstype`. The SDK wiring uses `<APPROOT>/node_modules` (override with `EXTRUNTIME_CONFIGURATOR_TYPESDIR`);
   an application calling `runtime.NewComponent` sets `Configurator.Types` itself. Without it the editor still has the
   developer kit; server-side diagnostics are unaffected.

## 9. Exporting the application's native catalog (offline kits)

Package authors work offline against a snapshot of the natives. Three sources, same document (`iota.catalog/1`):

* `GET /_ext/manage/catalog/natives` of a running application (needs `Extensions.Read`) — the exact set the process serves.
* `cmd/extruntime-catalog` — renders the snapshot of the **SDK modules alone** (`native.Compose(modules.Natives()...)`) to
  `web/runtime/catalog/sdk-natives.json`, which ships in `@iota-uz/sdk`; `iota-ext kit generate` uses it by default. It does
  not know the application's natives.
* The application's own command, for CI and offline kits:

  ```go
  set, _ := native.Compose(append(modules.Natives(), app.Natives)...)
  doc, _ := catalog.Natives(set).Snapshot().JSON()
  os.WriteFile("app-natives.json", doc, 0o644)
  ```

Authors then run `iota-ext kit generate --catalog app-natives.json` (a snapshot that already contains the SDK natives is
passed alone) or `iota-ext kit pull --server <url>` for the live kit including the active packages' contracts. Commit the
snapshot and regenerate it in CI whenever natives change; export names are derived from IDs, so an added native can lengthen
an existing name when two IDs collide.

## 10. Checklist: Granite (EAI-UZ/granite#3780, "Целевой API платформы расширений")

This maps the epic's nine consumer-scope items to the SDK API an application uses. It is a plan for the integrator; it does
not state that any Granite work is done, and each row's last column is what to verify in Granite.

| # | Item of the epic | SDK API / where | Granite does | Verify |
|---|---|---|---|---|
| 1 | Contracts and public kit | `native.Set` (`Entity`, `List`, `Event`, `Declare`, `Route`), `GET /_ext/manage/kit`, `catalog/natives`, `iota-ext kit generate --catalog` (§3, §9) | Writes a `Natives` contributor per module; exposes core entities (read-only), queries, operations, atomic/ordinary commands, business events with typed errors and permissions; exports its snapshot | A package author with no Granite checkout builds against `app-natives.json`; names match the live kit |
| 2 | Command boundary | `native.Operation` for invariants, `native.Command`/`AtomicCommand` for replaceable intents, `Def.Dispatch(ctx, native.Dispatcher, in)`, `/bindings`, package `guard`/`precheck` | Moves the invariant logic into operations; changes **every** entry point of a published command (HTTP handlers, jobs, importers, MCP) to `Dispatch`; keeps services as Go API only behind the operations | A bound replacement and a guard take effect from every entry point (the CRM `saveClient` forms are the model); unbinding restores the Go default |
| 3 | Resources and access | `data.NativeEntity{ReadPermission, FieldPermissions}`, TS `overlay`/`column`, `native.List`, `pkg/listext` | Registers entities read-only and writes through operations; integrates `listext` in the lists and exports it wants extendable | An overlay column appears in the standard list and export, honours access, filter and sort before pagination, and an absent overlay row reads as the default |
| 4 | Events and processes | `events.Publish` in the write transaction, `Tx.AfterCommit`, TS `subscribe`/`schedule`/workflows, trigger owners (§6.3), requeue (§6.2) | Publishes the first core business events in the operations that change the data; assigns owners per tenant | Rollback publishes nothing; a committed event starts the handler once (stable receipt); a user-less trigger runs as its owner |
| 5 | Runtime and package lifecycle | §1, §6.1: `wiring.NewComponent`, `EXTRUNTIME_*`, Git import/export, deactivate/purge | Ships `@iota-uz/sdk@R` and Node with the service, sets the section, grants `Extensions.*` | Publishing and activating change behaviour without a Go restart; deactivate keeps data |
| 6 | UI and shell | `EXTRUNTIME_UI_HOSTMANIFEST`, runtime host entry, `native.Route`/`RouteWith`, slots (`native.Declare`, `shellext.Slot`), shell binding | Builds the host bundle with the SDK's host entry and its Solid features (#3781/#3782), publishes native routes and slots on legacy and Solid pages | A shell binding wraps legacy full-page, HTMX and Solid screens; navigation, locale and theme do not depend on a private layout |
| 7 | Notifications | `core.ModuleOptions` channels/web push/contact resolvers/delivery policy (§4); TS `notice`, `templates`, `recipient` | Wraps its SMS/e-mail providers as `ChannelAdapter`s, registers contact resolvers for external parties, supplies the consent policy | Fake in-app/e-mail/SMS/push deliveries reach external recipients without ERP users; template version and outcome are visible per delivery |
| 8 | MCP | `mcptools` via `wiring.Options.MCPRegistry`, `rbacauth`, `mcpgateway.OAuth` (§7) | Registers the tools in Granite's administrative MCP registry and resolves personal credentials to users | Tool calls audit the person and `mcp:<client>`; revoking a permission takes effect on the next call; the public MCP stays separate |
| 9 | Pilot: renewal SMS campaign | a TS package: workflow (audience → message → send → wait → finish), `endpoint` for the gateway callback, notifications | Writes the package against synthetic fixtures and fake adapters; no marketing primitives in Go | The campaign runs from restart through waits, decisions and terminate/`start-v2` with explicit owners |

Open items that are Granite's, not the SDK's: the frontend host bundle and manifest (#3781, #3782), the atomic bump of Go
and npm pins (#3783), and the real providers behind the channel adapters.

## 11. Testing the browser side

`pkg/extruntime/e2e/browser` drives a real Chromium (Playwright) against the application that `pkg/extruntime/e2e` composes
for its server-side scenarios: the SDK's modules on PostgreSQL, session authentication, the real Node workers and build
worker, and the extension runtime UI. It asserts what the HTML-only scenarios cannot: that the canonical client host mounts,
disposes and recovers what the server describes.

| Spec | Verifies |
|---|---|
| `runtime-page.spec.ts` | a Solid page with a navigation entry: `useQuery` renders data, `useCommand` writes and the UI updates, typed route params arrive decoded, the root, its slot contributions and every listener are released when the document goes away or the route is unmounted, back/forward mounts once per visit, a user without the permission gets neither page nor menu entry |
| `shell.spec.ts` | a bound shell renderer wraps a legacy Templ/HTMX page (an HTMX swap, an Alpine script and a modal keep working in the host-owned outlet) and a Solid page; `?iota-shell=standard` and closing the binding restore the standard shell; theme (the legacy `<html>` class follows) and locale are the user's |
| `slots.spec.ts` | slot contributions on the CRM client page (full page and HTMX drawer) and on a Solid page; the audit contribution is absent from the HTML for a user without the permission; an HTMX swap of the region (or closing the drawer) disposes the root, a swap that brings the slot back mounts once |
| `cells.spec.ts` | a cell renderer island in the CRM clients list: the package component replaces the standard cell, the standard value is still there without JavaScript or when the renderer cannot load, rows swapped by HTMX get and lose islands |
| `bundle-failure.spec.ts` | the host's loading state, and the error state with Retry when the page module or the page's lazy component chunk answers 404; Retry reloads the document, writes nothing, and recovers |

**How it runs.** `TestBrowserServer` (`pkg/extruntime/e2e/browser_server_test.go`) is skipped unless
`IOTA_EXT_BROWSER_STATE` names a file. When the Playwright global setup starts it, it composes the application, activates
the fixture packages (`testdata/browser` plus the scenario fixtures) through the management API, mints session cookies for
four users (`admin`, `auditor`, `member`, `viewer`), serves the application on `127.0.0.1:4290` with the static assets
(htmx, Alpine, the theme script) and the **real host bundle** at `/assets/runtime-host.js`, and writes the URL and cookies
to the state file. Specs bind and unbind the shell through `/_ext/manage` as the administrator. The bundle is built by
`browser/build-host.mjs`: it is the minimal form of what an application's Vite build does with
`@iota-uz/sdk/runtime/client-host/entry` (the `@iota-uz/sdk/client-host` alias, Solid's browser export conditions, the
SDK identity stamped with the commit of the host manifest). Do not let a `tsconfig` `paths` entry point `solid-js` at a
directory in an application bundle: it bypasses the package's export conditions and bundles Solid's server build, which
fails in the browser with "Client-only API called on the server side".

**Locally** (Node 20+, Go, the database of the other e2e tests — the `DB_*` variables or `.env`):

```bash
just extruntime-browser                       # install deps, fetch Chromium, run the whole suite (~1 min after the first Go build)
just extruntime-browser slots.spec.ts         # one spec
just extruntime-browser -g "Retry" --headed   # any `playwright test` flag
```

To iterate on specs without rebuilding and restarting the server, start `TestBrowserServer` yourself (a compiled binary
reading its stdin until it closes; env `IOTA_EXT_BROWSER_STATE`, `IOTA_EXT_BROWSER_HOST_JS` — the output of
`node pkg/extruntime/e2e/browser/build-host.mjs <file>` — and `IOTA_EXT_BROWSER_PORT`) and run Playwright with
`IOTA_EXT_BROWSER_ATTACH=<state file>`; the setup then attaches instead of starting a server.

**In CI** run the same recipe on a runner with PostgreSQL configured like the Go test job, Node, Go and the Chromium system
libraries (`npx playwright install-deps chromium`). Failures leave screenshots and traces in
`pkg/extruntime/e2e/browser/test-results` (and an HTML report in `playwright-report` when `CI` is set).

**Limits.** The stylesheet compiled by `just css` is not required (an empty `main.min.css` is served when it is absent), so
the specs assert on DOM and behavior, never on layout; the realtime socket (`/ws`) of the standard layout is not served;
and restoring a page from the browser's back/forward cache is not exercised: the layout's pages were not bfcache-eligible
in the Chromium used (`pageshow.persisted` stayed false with the feature enabled), so the host's `pagehide` disposal never
met a `pageshow` restore there.
