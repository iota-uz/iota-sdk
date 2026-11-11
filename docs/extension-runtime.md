# Extension runtime platform — implementation contract

Canonical requirements: iota-sdk #411 (epic) and #412–#420, #947/#953, #920/#925–#927,
EAI-UZ/granite#3780. This file is the **shared implementation contract** between the
owners of the slices below. A slice owner may not invent a second registry, transport,
query API or workflow engine; changes to anything in this file go through its owner
(the orchestrator) first.

## 0. Vocabulary

| Term | Meaning |
|---|---|
| package | an extension: `package.json` with an `iota` section, ESM sources, optional npm deps |
| namespace | stable dotted owner prefix of every resource ID (`acme.insurance`) |
| resource ID | `<namespace>.<name>`; declared exactly once, in a contract value |
| contract | pure value describing a resource (no handler). Lives in the **contracts graph** |
| implementation | server value bound to a contract by typed reference: `implement(Contract, handler)` |
| version | immutable build of a package: source hash, lockfile, artifacts, manifest, contract resolution snapshot |
| activation | which version of a package is live; one active version per package |
| binding | which implementation is active for a command contract; **one per application** |
| invocation | one execution of a command/query/operation/subscription; records selected binding |
| occurrence | one durable workflow primitive call: `(instance, name, n)` where `n` counts repeats of `name` |

## 1. Package layout and graphs

```jsonc
// package.json
{
  "name": "@acme/insurance",
  "version": "0.1.0",
  "type": "module",
  "iota": {
    "namespace": "acme.insurance",
    "contracts": "./src/contracts.ts",   // pure graph: may import only @iota-uz/sdk/runtime, kits, other pure files, pure npm
    "server": "./src/server.ts",         // server graph: implementations; Node/npm allowed
    "client": "./src/client.tsx",        // browser graph: pages/slots/columns/shell (Solid)
    "capabilities": { "network": ["sms.example.uz"], "secrets": ["SMS_TOKEN"] }
  },
  "dependencies": { "@iota-uz/sdk": "*" }
}
```

* Three ESM graphs built separately. The registry is the set of **exported values** of the
  three entry modules carrying the SDK brand (`Symbol.for("iota.resource")`). No global
  `register()`, no hand-written list.
* Client graph build fails (source-located diagnostic) when it reaches: the server entry,
  any file reachable only from server, `@iota-uz/sdk/runtime/server`, or a Node builtin.
  There is **no** handler stripping.
* Contracts graph build fails if it reaches `@iota-uz/sdk/runtime/server`, `.../client`,
  Node builtins or `solid-js`.
* Own draft imports its own local contracts (`./contracts`). Core and foreign packages are
  imported from the developer kit: `@iota-kit/<namespace>` (generated `.d.ts` + pure JS refs,
  never handlers). Kit = catalog snapshot exported by the management backend.

## 2. TypeScript public API (`@iota-uz/sdk/runtime*`)

> **Superseded where it differs.** The examples in this section are the original design sketch. §9 and §9.1 (and the
> author-facing manual [`extensions/authoring.md`](extensions/authoring.md)) are authoritative; the examples below are
> stale in places: write rules are `writeRule(id, Entity, …)`, operation guards `guard(id, Operation, …)` and ordinary
> command prechecks `precheck(id, Command, …)` (not `guard(Policy, "write", …)`); errors are thrown as `Err.of(params)` or
> `validation(Issue.at(path, params))` (there is no `tx.issue`); native lists and entities come from the generated kit
> (`crm.Clients`, `core.User`, whose names depend on the catalog) rather than `crm.lists.Clients`; `tx.start` needs
> `{ key }` outside subscriptions and workflows; `tx.get(overlay, ref)` and `tx.put(overlay, ref, values)` replace
> overlay reads/writes; `column`'s `id` is the full namespaced ID; `t.ref`/`field.ref` take entities only.

Subpaths (published inside the single `@iota-uz/sdk` distribution):

* `@iota-uz/sdk/runtime` — pure: `t` (value schema), `entity`, `overlay`, `field`, `query`,
  `command`, `operation`, `event`, `workflow`, `notice`, `route`, `permission`, `issue`,
  `domainError`, `column`, types `Ref<E>`, `Infer<S>`.
* `@iota-uz/sdk/runtime/server` — `implement`, `subscribe`, `guard`, `schedule`,
  `endpoint`, `templates`, context/tx/wf types.
* `@iota-uz/sdk/runtime/client` — `page`, `navigation`, `slot`, `shell`, `useCommand`,
  `useQuery`, `cell` renderers; Solid.
* `@iota-uz/sdk/runtime/testing` — synthetic fixtures + fake effects harness.

### 2.1 Values (`t`) and wire

`t.string() t.int() t.int64() t.decimal({precision,scale}) t.bool() t.date() t.datetime()
t.uuid() t.enum([...]) t.json<T>() t.ref(Entity) t.object({...}) t.array(s)` with
`.optional()` (key may be absent) and `.nullable()`.

Wire (JSON) is exact: `int` = JSON number, must be a safe integer; `int64` = decimal
**string** (TS type `bigint`-free branded `Int64 = string & {__int64}`); `decimal` = string
(`Decimal = string & {__decimal}`); `date` = `"YYYY-MM-DD"` (`ISODate`); `datetime` = RFC3339
with offset (`ISODateTime`); `ref` = `{"entity":"<id>","key":"<string>"}`. Every schema
serializes into the manifest (§4) so Go validates input/output independently of TS.

### 2.2 Entities, overlays, refs

```ts
export const Policy = entity("acme.insurance.policy", {
  number:    field.text({ unique: true }),
  holder:    field.ref(Customer),
  premium:   field.decimal({ precision: 18, scale: 2 }),
  expiresOn: field.date().index(),
  status:    field.enum(["draft", "active", "expired"]).default("draft"),
  notes:     field.text().optional(),
  extra:     field.json<{ tags: string[] }>().optional(),   // JSONB only when declared
}, {
  access: {
    read:  [{ when: ManagePolicies, rows: "all" }, { rows: { holder: { owner: $.user } } }],
  },
});
export type Policy = typeof Policy;
```

* One PostgreSQL table per entity (§5); field key = stable field ID; rename only with
  `field.text().renamedFrom("oldKey")`.
* `Ref<typeof Policy>` = `{ entity: "acme.insurance.policy"; key: string }` branded by the
  entity ID literal; refs to different entities are not assignable to each other.
* Native (core/Granite) entities come from the kit: `core.User`, `crm.Client` — kind
  `"native"`. Generic `tx.insert/update/delete` accept only kind `"own"` (type error +
  runtime rejection). Native writes go through published operations (`core.users.rename`).
* `overlay("acme.insurance.clientExtra", crm.Client, { loyaltyTier: field.enum([...]).optional() })`
  — separate 1:1 typed table keyed by base key. `tx.get(ClientExtra, clientRef)` returns
  `Overlay | null` — absence is explicit in the type. Overlay fields default-read as declared
  default or `null`; same semantics in query/filter/sort/list/export.
* `column(crm.lists.Clients, { id: "loyaltyTier", field: ClientExtra.fields.loyaltyTier,
  label: {en,ru,uz}, filterable, sortable, exportable })` — pure contract consumed by Go
  native list + export (§7). Optional client `cell` renderer lives in client graph.

### 2.3 Commands, operations, queries, errors

```ts
// contracts
export const NotActive = domainError("acme.insurance.notActive", t.object({ status: t.string() }));
export const ExtendExpiry = operation("acme.insurance.extendExpiry", {
  input: t.object({ policy: t.ref(Policy), months: t.int() }),
  output: t.object({ expiresOn: t.date() }),
  errors: [NotActive],
});
export const RenewPolicy = command("acme.insurance.renewPolicy", { input, output, errors });          // ordinary intent
export const ActivatePolicy = command("acme.insurance.activatePolicy", { atomic: true, input, output });
export const Expiring = query("acme.insurance.expiring", { input, output });

// server
export const extendExpiry = implement(ExtendExpiry, async (tx, input) => { ... });       // (Tx, In) => Out
export const renew = implement(RenewPolicy, async (ctx, input) => {                     // (CommandContext, In) => Out
  const quote = await fetch(...);                    // normal Node/npm, no tx open
  return ctx.transaction(async (tx) => tx.call(ExtendExpiry, {...}));
});
export const activate = implement(ActivatePolicy, async (tx, input) => { ... });        // atomic: whole handler in one Go tx
export const expiring = implement(Expiring, async (q, input) => q.find(Policy, {...}));  // read-only QueryContext
```

* `Tx` (Go-owned transaction handle): `get find count insert update delete call emit notify
  start issue`. `tx.call` accepts **only** `Operation | AtomicCommand` (type-level + runtime).
  Nested `ctx.transaction` inside a tx is rejected.
* `CommandContext`: `user`, `invocation`, `transaction(cb)`, `query` (read-only),
  `call(Command)` (new invocation), `secrets`, `log`. No `ctx.base`, no around-chain.
* Atomicity is part of the PUBLIC contract (`atomic: true`). Any replacement implementation
  of an atomic command gets `Tx`; ordinary replacement gets `CommandContext`.
* Errors: `throw NotActive.of({status})` — declared domain errors (closed per contract);
  validation issues are extensible and namespaced: `issue("acme.insurance.tooLong",
  t.object({max:t.int()}))`, raised as `tx.issue(TooLong, {path:["notes"], params:{max:10}})`
  / `throw validation([...])`. Wire: `{kind:"domain"|"validation"|"denied"|"internal",
  code, params, issues:[{code, phase:"input"|"write", source, path, params}]}`.
* `guard(ExtendExpiry, async (tx, input) => ...)` — additive transactional guard on an
  operation (same tx). `guard(RenewPolicy, async (q, input) => ...)` on an ordinary command —
  **preliminary** input check (read snapshot), named so. `guard(Policy, "write", async (tx,
  { existing, proposed, changed }) => ...)` — entity write rule in the writing tx.
  Guards are independent of the active command implementation.
* `implement(core.commands.RejectIncoming, ...)` from a package = replacement candidate;
  becomes active only through an explicit binding (§6). One active binding per command per
  application; no fallback on error.

### 2.4 Events and subscriptions

```ts
export const PolicyRenewed = event("acme.insurance.policyRenewed.v1", { subject: Policy, payload: t.object({...}) });
// server
export const onRenewed = subscribe("acme.insurance.onRenewed", PolicyRenewed, async (tx, e) => {
  await tx.start(RenewalFollowUp, { policy: e.subject });
});
```

* `tx.emit` enqueues atomically with the data (outbox row in the same Go tx).
* After commit, each active subscription gets its own transaction; receipt
  `(subscription_id, event_id)` is written in that tx; re-delivery after commit is a no-op.
  `tx.start` inside it writes a start-intent keyed `(subscription, event)`, so no duplicate
  workflow. Core events are published by Go modules through the same outbox (`events.Publish
  (ctx, tx, ...)`), declared in the native catalog with a versioned payload.
* Subscription runs as the event initiator (current rights); events without a user run as
  the subscription's explicit owner-user assigned at activation.

### 2.5 Workflows

```ts
export const RenewalCampaign = workflow("acme.insurance.renewalCampaign", {
  input: t.object({ before: t.date() }),
  signals: { decision: t.object({ approved: t.bool() }) },
});
// server
export const renewalCampaign = implement(RenewalCampaign, async (wf, input) => {
  const targets = await wf.transaction("load", (tx) => tx.find(Policy, {...}));   // DB effect + receipt, one Go tx
  for (const p of targets) {
    await wf.step("sms", async ({ idempotencyKey }) => sms.send(..., idempotencyKey)); // re-runnable effect, occurrence n
  }
  const d = await wf.task("approve", { subject: p, assignee: user, decision: t.object({approved: t.bool()}), timeout: "3d" });
  await wf.sleep("cooldown", "1d");
  const r = await wf.call("renew", RenewPolicy, {...});   // new command invocation → active binding, receipt
});
```

* Replay engine (Go-owned journal). Every durable primitive takes a stable `name`; the
  occurrence key is `(instance, name, n)`. Completed occurrences return the recorded result
  without executing. `sleep`/`waitFor`/`task` on an unmet condition **suspend**: the Node
  invocation ends, worker and tx are released; Go timers/signals resume by replaying.
* The instance pins its package version (code/npm/templates). `wf.call` creates a new
  invocation that selects the **current** active binding and records it; replay returns
  the receipt and never re-dispatches.
* `wf.transaction` writes DB effects + occurrence receipt in one Go tx (dedupe of committed
  DB effects, not exactly-once network). `wf.step` may repeat; receives a stable provider
  idempotency key `"<instance>:<name>:<n>"`.
* Terminate stops future steps (does not compensate). Start v2 = new instance with
  `predecessor_id`. Current rights checked on each new action; session expiry does not stop
  instances; denied → `blocked` outcome with diagnostic.

### 2.6 Notifications

```ts
export const RenewalReminder = notice("acme.insurance.renewalReminder.v1", { subject: Policy, payload: t.object({...}) });
// server
export const reminderTemplates = templates(RenewalReminder, {
  sms: ({ payload, locale }) => `...`,
  email: ({ payload }) => ({ subject: "...", html: "...", text: "..." }),
  inApp: ({ payload }) => ({ title: "...", body: "..." }),
});
await tx.notify(RenewalReminder, {
  subject: policyRef, payload,
  to: [recipient.user(userRef), recipient.record(customerRef), recipient.address({ sms: "+998..." })],
  channels: ["sms", "email"],
});
```

Recipient (user / record via published contact resolver / direct address) ≠ subject ≠
resolved channel target/device. Enqueue is atomic with the tx; render (pinned template
version) and provider delivery run after commit through the **core notification service**
(#920/#925/#926). Rule-mode publication uses the same notice IDs via the core catalog
(`DefinitionSource`).

### 2.7 UI

```ts
// contracts (only when the route is shared with server/foreign packages)
export const PolicyRoute = route("acme.insurance.policy", { path: "/insurance/policies/:id", params: t.object({ id: t.uuid() }) });
// client.tsx
export const policies = page("acme.insurance.policies", {
  path: "/insurance/policies", nav: { label: {...}, group: "acme.insurance", icon: "shield" },
  permission: ReadPolicies, component: lazy(() => import("./PoliciesScreen")),
});
export const policy = page(PolicyRoute, { component: lazy(() => import("./PolicyScreen")) });
export const nav = navigation({ groups: [...], patches: [{ ref: "core.users", hide: true }] });
export const clientTierCell = cell(ClientsTierColumn, (p) => <Badge>{p.value}</Badge>);
export const clientCard = slot(crm.ClientSidebar, (p) => <Card client={p.clientId} />, { order: 10, permission: Manage });
export const shell = shellRenderer((p) => <MyShell {...p} />);  // whole authenticated shell
// contracts.ts: a slot of the package's own page — <Slot slot={Widgets} props={{ title }} /> in a client page
export const Widgets = defineSlot("acme.insurance.widgets", { props: t.object({ title: t.string() }) });
```

Pages are mounted by the canonical client host inside `layouts.Authenticated` (same
boundary as legacy pages). Nav is projected by Go from the activated manifest; Node is not
used per render.

## 3. Wire protocol Go ⇄ Node (pool)

Transport: worker stdio, frames = 4-byte big-endian length + UTF-8 JSON. Protocol version
`iota.runtime/1`; mismatch → worker rejected.

```
Go → Node  {"t":"hello","protocol":"iota.runtime/1"}
Node → Go  {"t":"ready","protocol":"iota.runtime/1","pid":123}
Go → Node  {"t":"invoke","id":"inv-…","artifact":{"hash":"…","dir":"/cache/…"},"kind":"command|atomic|operation|query|subscription|workflow|guard|template|endpoint|build|extract",
            "export":"renew","resource":"acme.insurance.renewPolicy","input":…,"ctx":{tenant,user,invocation,occurrence?,deadline,tx?}}
Node → Go  {"t":"call","inv":"inv-…","id":7,"method":"tx.begin|tx.end|tx.get|tx.find|…|wf.step.begin|…","args":…}
Go → Node  {"t":"reply","inv":"inv-…","id":7,"ok":true,"value":…} | {"ok":false,"error":{…}}
Node → Go  {"t":"done","id":"inv-…","ok":true,"value":…} | {"ok":false,"error":{kind,code,message,params,issues,suspend?}}
Go → Node  {"t":"cancel","id":"inv-…"}
Node → Go  {"t":"log","inv":"…","level":"info","msg":"…"}
```

* Go serializes calls per tx handle and owns deadline/commit/rollback.
* As implemented (host slice): invoke kinds `command|atomic|operation|query|subscription|
  workflow|guard|template|endpoint|schedule` (build/extract run in a separate build worker).
  `ctx` carries `tenant, user{id (int64 string), kind, locale, permissions[]}, invocation,
  deadline, tx?, trigger, parent, instance`. Host methods: `tx.begin|end|get|find|count|insert|
  update|delete|putOverlay|call|emit|notify|start|signal|query`, `q.get|find|count|query` (lazy REPEATABLE READ
  READ ONLY snapshot), `invoke.command`, `invoke.signal`, `secrets.get`, `wf.step.begin|end`,
  `wf.transaction.begin|end|call`, `wf.sleep|waitFor|task|call|start|now|uuid`. Replies:
  `tx.start`/`wf.start` → `{instance}`, `wf.waitFor` → `{outcome, payload}`. Nested invocations
  (`tx.call`, `ctx.call`) run on the parent's worker without taking a pool slot. Stray console
  output becomes `log` frames; closing stdin shuts a worker down.

* Artifacts are immutable directories addressed by content hash; a worker `import()`s
  `<dir>/server.mjs` once per hash. Two versions = two module instances in the same worker.
* Worker env is minimal (`PATH`, `NODE_ENV`, `TZ`, `IOTA_RUNTIME_*`); secrets only through
  host calls for declared capabilities.

## 4. Manifest (build output, stored per version)

`manifest.json` extracted by running the contracts + server + client graphs' exports in the
build worker:

```jsonc
{ "schema": "iota.manifest/1", "namespace": "acme.insurance",
  "resources": [
    {"id":"acme.insurance.policy","kind":"entity","fields":[{"id":"number","type":{"kind":"string"},"unique":true}, …],"access":{…}},
    {"id":"acme.insurance.renewPolicy","kind":"command","atomic":false,"input":{…},"output":{…},"errors":["acme.insurance.notActive"]},
    {"id":"acme.insurance.policies","kind":"page","path":"/insurance/policies","nav":{…},"chunk":"client/policies-…js"}
  ],
  "implementations": [{"export":"renew","implements":"acme.insurance.renewPolicy","kind":"command"}],
  "subscriptions": [...], "guards": [...], "templates": [...], "dependencies": {"core":"<kit hash>"} }
```

Go types live in `pkg/extruntime/manifest` (single owner) and are the **only** Go
representation of resources.

## 5. Persistence (schema `extruntime`, PostgreSQL)

Owned by slice owners but all DDL in one migration set:

* management: `packages`, `package_files` (draft tree), `versions` (immutable, hashes,
  provenance git|runtime, manifest jsonb, resolution snapshot, review), `artifacts` (hash →
  bytea), `activations` (history), `command_bindings` (command id → package version export,
  one active), `owners` (trigger owner-user per subscription/schedule).
* data: tenant-scoped tables per entity in schema `x_<namespace with _>`:
  `tenant_id uuid, id uuid pk, <typed columns>, created_at, updated_at, row_version`;
  overlays: `tenant_id, base_key <base key type>, <columns>, PK(tenant_id, base_key)`.
  `entity_schemas` (applied field IDs/types per version + migration history).
* execution: `invocations` (contract, selected binding/version, subject user, outcome,
  partial-commit flag), `occurrences` (receipts: instance/name/n/result, input hash),
  `outbox_events`, `subscription_receipts`, `workflow_instances` (pinned version, owner,
  predecessor, status), `workflow_timers`, `workflow_signals`, `human_tasks`.

## 6. Go host (`pkg/extruntime/...`)

| Package | Owner slice | Responsibility |
|---|---|---|
| `manifest` | contracts | manifest Go types, schema validation of wire values |
| `pool` | runtime | Node worker pool, protocol, supervisor, generations |
| `host` | runtime | invocation dispatch, tx handles, host-call router, bindings |
| `data` | data | DDL from manifest, typed query compiler w/ access, overlays, native adapters |
| `events` | workflows | outbox, subscriptions, receipts |
| `workflow` | workflows | replay engine, timers, signals, tasks, terminate/start-v2 |
| `notify` | notifications | adapter to core notifications |
| `manage` | management | drafts, build, check, fake-test, review, publish, activate; kit export |
| `ui` | UI | page route host, nav projection, native column/export providers, shell activation |
| `mcp` | Granite/MCP | MCP tools over `manage` |

The Node side lives in `web/runtime` (workspace package, built into `@iota-uz/sdk` subpaths
`./runtime`, `./runtime/server`, `./runtime/client`, `./runtime/testing`, and the
`runtime-host` worker entry).

## 7. Guarantees and honest limits

* Shared Node pool is a trusted domain; no memory sandbox between packages.
* `wf.transaction` dedupes committed DB effects; external network is never exactly-once.
* Ordinary command may commit partially before a lost reply; it is not blindly retried.
* Old instances keep their pinned artifacts; Go host/native operation implementations may
  change under a compatible public contract; old Go binaries are not retained.

## 8. Durable engine and events (design for the workflows slice)

### Engine qualification

DBOS (TS and Go) was qualified **by analysis** against the author API (no experiment run yet):
* DBOS transactions run on DBOS's own data source connection; `wf.transaction` requires the
  occurrence receipt to commit in the **Go-owned** tx together with DB effects — not expressible.
* DBOS recovers workflows by application version per process; the shared pool runs v1 and v2
  of several packages side by side and selects artifacts per instance.
* Waiting in DBOS Go keeps the workflow goroutine/Node invocation alive or requires our own
  re-entry journal anyway.

Decision: a Go-owned **replay journal engine** over PostgreSQL (not a queue): deterministic
re-execution of the pinned TS function, journal of completed occurrences, durable timers,
signals, human tasks, leases and crash recovery. Revisit if a candidate supports external
transaction receipts and per-instance artifact pinning.

### Tables (`extruntime`)

* `workflow_instances(id, tenant_id, workflow, package, version, artifact_hash, export, input,
  status running|waiting|completed|failed|terminated|blocked, result, error, subject_user_id,
  subject_kind initiator|owner, predecessor_id, start_key unique, wake_at, lease_owner,
  lease_until, run_count, created_at, updated_at)`
* `workflow_occurrences(instance_id, name, n, kind, status started|completed|failed|waiting,
  input_hash, result, error, attempts, wake_at, invocation_id, PK(instance_id,name,n))`
* `workflow_signals(id, instance_id, name, payload, consumed_name, consumed_n, created_at)`
* `human_tasks(id, tenant_id, instance_id, name, n, subject_entity, subject_key,
  assignee_user_id, decision_schema, status open|decided|expired|cancelled, decision,
  decided_by, due_at)`
* `outbox_events(seq bigserial, id uuid, tenant_id, event, subject_entity, subject_key,
  payload, initiator_user_id, occurred_at)`; `subscription_cursors(subscription, from_seq)`;
  `subscription_receipts(subscription, event_id, invocation_id, PK(subscription,event_id))`;
  `trigger_owners(trigger_id, tenant_id, user_id)`; `schedule_runs(schedule, slot)`.

### Primitive semantics (occurrence = (instance, name, n), n = per-run counter of `name`)

| Call | First execution | Replay |
|---|---|---|
| `wf.transaction(name, cb)` | Go tx; cb data calls; receipt row inserted **in the same tx**; commit | return stored result |
| `wf.step(name, fx)` | short tx marks `started`, attempts++; fx runs in Node with key `instance:name:n`; result recorded | completed → stored; `started` → fx runs again (at-least-once, same key) |
| `wf.sleep(name, d)` | record `wake_at`; suspend | `now ≥ wake_at` → complete |
| `wf.waitFor(name, signal, {timeout})` | consume unconsumed signal or suspend | consumed → payload |
| `wf.task(name, spec)` | create `human_tasks` row; suspend | decided → `{outcome:"decided", decision, by}`; due → `{outcome:"timeout"}` |
| `wf.call(name, command, input)` | new invocation selecting the **current** active binding (recorded); atomic: receipt in the command tx; ordinary: `started` recorded before dispatch | completed → stored; `started` without result → instance `blocked` (`ambiguous`, manual review) |
| `wf.start(name, workflow, input)` | instance row with `start_key=instance:name:n` | stored instance ref |

* A different kind at the same (name, n) → `nondeterminism`, instance `blocked`.
* Suspend ends the Node invocation (worker freed, no tx held). A scheduler wakes instances on
  `wake_at`/signal/task decision; runs are claimed with `FOR UPDATE SKIP LOCKED` + lease;
  expired leases are re-claimed after restart and replayed.
* Each run loads the subject's **current** permissions; disabled/denied → `blocked` with a
  diagnostic. Session expiry is irrelevant.
* Terminate: status `terminated`, in-flight run cancelled, further host calls rejected; done
  effects stay. Start v2: new instance (any version) with `predecessor_id`; history linked.

### Events

* `tx.emit` / Go `events.Publish(ctx, tx, …)` insert into `outbox_events` in the caller's tx.
* Dispatcher (after commit; LISTEN/NOTIFY + poll) delivers to each **active** subscription in
  `seq` order from its cursor (cursor initialized at activation → no automatic replay of the
  journal on handler update). Each delivery = own tx: handler (Tx) + receipt; duplicate
  receipt → skipped. `tx.start` inside uses `start_key = subscription:event_id#n` (n = order of
  the keyless `tx.start` calls inside the delivery).
* Subject: event initiator's current rights; no initiator → `trigger_owners` owner-user
  (assigned at activation; activation fails without one).

## 9. Revisions after architecture review (2026-11-11, Opus 5.5) — supersede §2–§8 where they differ

**Persistence identities (decided now to avoid a second schema change)**
* Single `extruntime.bindings(id, slot_kind command|screen|shell, target_id, package, version,
  export, activated_by, active_from, active_to)` — one open row per (slot_kind, target_id)
  application-wide. Replaces `command_bindings`; shell/screen replacement and revert use it.
* Activation is **application-wide**; per-tenant trigger owners in `trigger_owners(trigger_id,
  tenant_id, user_id)`. A tenant without an owner gets a `blocked` trigger run, not a failed
  activation. `activations` carry `state active|disabled|purged`.
* Fields: `entity_schemas` assigns each field an immutable **physical column** at first apply;
  field keys (and `renamedFrom` aliases) map to it per package version. DataStore calls carry
  `spi.Scope{Subject, Caller{Package, Version}}` and resolve keys through the caller version's
  map. Activation is blocked when a running instance's pinned schema is incompatible.
* `Resource.ContractHash` (Go `manifest.ContractHashOf`) recorded in `invocations`,
  `workflow_occurrences`, bindings; kit snapshots in `catalog_snapshots(hash, content)`.
* `invocations` add `parent_invocation_id, root_id, trigger_kind ui|mcp|endpoint|schedule|
  subscription|workflow, trigger_ref, binding_id, contract_hash, result, committed_tx_count`.
* `workflow_instances` add `parent_instance_id, start_invocation_id, blocked_reason,
  lease_epoch`; `start_key` unique per `(tenant_id, start_key)`; start-v2 has `start_key=null`.
  `workflow_signals` add `sender_user_id, signal_key unique`. `outbox_events` add
  `producer_invocation_id`. `schedule_runs(tenant_id, schedule, slot)`; `Schedule.Timezone`.
* Single table list: no `workflow_timers` table (wake_at columns), `trigger_owners` (not `owners`).

**Events**: no seq cursor. Every committed (visible) event lacking a done/dead receipt of the
subscription is eligible, so late-committing lower-seq events are never skipped and no long
transaction elsewhere in the cluster delays delivery (an earlier xmin-horizon gate was removed for
that reason, 2026-11-11). Delivery order is by seq among visible rows, not a strict commit order;
the activation watermark only excludes events older than activation. Per-row delivery state:
`subscription_receipts(subscription, event_id, status pending|done|failed|dead, attempts,
next_at, last_error, invocation_id)` with retry/backoff and dead-letter; one poisoned event
does not block the subscription.

**Engine**: every state write is fenced `WHERE id=$1 AND lease_epoch=$e` in the same tx as
the occurrence write (stale runner cannot record). `invoke` for a workflow run carries the
journal of completed occurrences so replay is answered inside Node (no O(n²) host calls);
fan-out via child `wf.start`. Concurrent primitives (`Promise.all`) get `n` at call time; the
run suspends only after in-flight steps record. Signal consumption + occurrence completion
commit in one tx; task decision vs timeout is a compare-and-set on `human_tasks.status`.
`wf.call` of an ordinary command: host increments `committed_tx_count` inside each
`ctx.transaction`; on replay an existing outcome is returned, otherwise `blocked: ambiguous`
showing the commit count. Management actions for blocked occurrences: `resolve(occurrence,
result)`, `retry(occurrence)`, `resume`. Terminate cancels open tasks/timers, rejects signals,
does not cascade to children. `wf.transaction(name, cb | Operation | AtomicCommand, input?)`
stores `input_hash`; mismatch on replay → `blocked: nondeterminism`. `wf.now(name)`,
`wf.uuid(name)` journaled helpers. Step/transaction results must be JSON (`Json` type) or
declare `{output: schema}`.

**Author API**: `precheck(id, OrdinaryCommand, (q, input) => …)` (preliminary snapshot),
`guard(id, Operation|AtomicCommand, (tx, input) => …)`, `writeRule(id, Entity, (tx, {existing,
proposed, changed}) => …)`; guards have stable IDs used as issue `source`. One error path:
`throw Err.of(params)` for declared domain errors and `throw validation(Issue.at(path,
params), …)` for issues (no `tx.issue`). `tx.start(W, input, {key})` — key required outside
subscriptions/workflows. Overlay write `tx.put(Overlay, baseRef, values)`. Templates include
`push`. Resource kinds `contactResolver` / `deliveryPolicy` for #926 public contracts.
Endpoints get `EndpointContext` (ordinary ctx + `request`, returns `{status, headers, body}`);
authenticated → actual user, public → trigger owner. Filter keys may be ref paths
(`"holder.owner"`), compiled to explicit joins. Navigation adds `workspaces` and `palette`.
Build/extract run in a separate build worker, not the invocation pool.

**UI host (as implemented)**: `bindings.target_id` for `slot_kind='screen'` is a native route
ID (registered by the host app in `ui.Config.NativeRoutes`) or a package page ID; for
`slot_kind='shell'` the newest open row wins. Browser endpoints: `POST /_ext/rpc` (commands and
queries as the session user; operations are never callable from the browser) and
`GET /_ext/assets/<artifactHash>/<path>` (immutable). Client bundles externalize `solid-js`,
`solid-js/web` and `@iota-uz/sdk/runtime/client`, resolved from the host's shared module
registry (`globalThis[Symbol.for("iota.runtime.shared")]`) so there is one Solid and one
runtime context per document. Native list columns: `pkg/listext` (reference consumer: CRM
clients list + CSV/Excel export).

**Slots, cells and screen replacement (as implemented)**
* **A slot is declared by its host page**: `manifest.Resource{Kind: slot}` without `target` (stable ID, typed `props`,
  optional `permission`). Native pages declare theirs as plain native resources (CRM: `crm.slots.clientSidebar`, props
  `{clientId: int}`, permission `Client.Read`, in `modules/crm/extension`); packages declare theirs with `defineSlot` in the
  *contracts* graph, so other packages reach them through the kit (`slotRef(id, propsSchema)` types the contributions).
* **Contributions are additive**: `slot(ref, Component, {order?, permission?})` is a client resource with `target`, `order`,
  `permission`; no `bindings` row. The index orders a slot's contributions by (order, package, ID). For a viewer the server
  keeps the ones whose own permission the viewer holds, and none at all when the slot's declaration requires a permission the
  viewer lacks. Props passed by the host page are validated against the declared type before they are emitted.
* **Rendering.** Go pages call `shellext.Slot(ctx, id, props)`: a mount element
  `<div data-iota-slot=ID data-iota-slot-config={props, contributions[{id,package,version,module,export}], env} hx-disable>`
  and, once per request, the runtime host entry; with no applicable contribution nothing is rendered. Solid pages and the
  shell renderer get the same contributions from `PageData.slots` / `RuntimeContext.slots` and mount them with
  `<Slot slot={…} props={…}/>` (`@iota-uz/sdk/runtime/client`; `ref` is not usable as a prop name, Solid reserves it). The
  browser host (`client-host/islands.ts`) mounts each element into its own Solid root and disposes it when the element leaves
  the document (HTMX swap, boosted navigation, `pagehide`); a module that resolves after disposal is dropped.
* **Cells.** `cell(column, Component)` is a client resource with `target` = column ID. The native list renders the standard
  cell for the column's type either way and wraps it in `<span data-iota-cell=… data-iota-cell-config={contribution, value,
  row, env} hx-disable>`; the host replaces the fallback once the component has rendered. `value` is wire JSON, `row` the
  readable contributed values by column ID plus `ref`. Filters, sorting and exports never use the renderer. One renderer per
  column (the declaring package wins, then the first by package name).
* **Screens.** A native route is a `route` resource (CRM: `crm.routes.clientView`, `/crm/clients/:id`, params `{id: int}`,
  permission `Client.Read`) published like any native resource and resolved through the catalog, so a package page for it
  (`page(crm.ClientView, …)`) gets typed params. Such a page is a candidate: it is not served until a `screen` binding exists,
  the replacement requires the route's permission as well as its own, and HTMX partial requests (not boosted) keep reaching
  the native handler. A request routed by the previous snapshot finishes on it (no 404 while a binding is reverted).
* The composition root passes the CRM's declarations to the runtime until native registration is unified:
  `runtime.Config.NativeResources = crmext.Resources(basePath)` (`cmd/extruntime-catalog` does the same for the offline kit).

**Data access, execution and assets (as implemented, post-audit)**
* Overlay rows are readable (Get/Find/Count, filters, `NativeOverlayJoin` for native lists and
  exports) only together with their base row: the native `ReadPermission`, or the base entity's
  read rules; `PutOverlay` demands the same. Field access is symmetric: a field a subject cannot
  read cannot be written by it (`ErrFieldNotAccessible`, no value in the error), which also closes
  the "patch changes nothing" equality oracle. A required hidden field therefore cannot be
  supplied on insert by a subject without the permission.
* Ref-path filters into an entity whose own read rule uses a ref path group the rule's joins with
  the joined table (`LEFT JOIN (t AS j1 LEFT JOIN … AS j2 …) ON …`); aliases come from one counter.
* A version runs only while its newest review is an approval bound to its exact source and artifact
  hashes. `manage.Service.ResolveArtifact` (the only door to the pool: invocations, workflow
  start-v2 and pinned instances, guards, subscriptions, schedules, endpoints, notifications)
  enforces it on every call, not only `Activate`. A rejection after activation stops delivery:
  event/schedule/endpoint sources skip the version, `Implementations.Active` fails.
* `GET /_ext/assets/<hash>/<path>` serves only `client/**`, and only for hashes of the artifacts of
  currently activated versions (screen and shell bindings resolve only into those). `UIArtifacts`
  repeats the `client/` confinement.
* `AssignOwner` (needs `extensions.publish`) accepts the caller, or a user whose every current
  permission (`manage.Config.Subjects`, direct/role/group) the caller holds; without a rights
  source only self-assignment. Refusals are audited.

**Engine/events (as implemented)**: `subscription_receipts.status` ∈ `done|failed|dead|blocked`
(`blocked` = missing owner / refused subject, re-checked later). `workflow_occurrences.status`
adds `cancelled` (set by Terminate). Activation stores `events.Watermark()` as the
subscription's `AfterSeq`. The management slice supplies `workflow.Implementations`,
`events.SubscriptionSource`, `events.ScheduleSource`. The workflow journal travels in
`InvokeContext.Occurrence.journal`. Known limits: concurrent waits in `Promise.all` suspend
sequentially; `wf.user.has()` reads current rights and is not journaled; event delivery is not strictly
commit-ordered across concurrent producers; schedule slots older
than `Lookback` (15 min) are skipped.

**Engine/events hardening (as implemented)**:
* `tx.start` without an explicit key derives `base#n`: inside `wf.transaction` / an atomic `wf.call` the
  base is the occurrence `instance:name:n`, inside an ordinary command called by `wf.call` it is
  `wf.call:<invocationId>`, inside subscriptions and schedules `subscription:event` / `schedule:id:slot`;
  `n` counts the keyless starts of that unit, so a replay yields the same keys and a later occurrence can
  never collide with an earlier child.
* `DecideTask` locks the instance row before the task (same order as every run write) and wakes it under
  that lock, like `deliverSignal`; `Resolve`/`Retry` read `blocked` under `FOR UPDATE` in the tx that edits
  the journal.
* Infrastructure failures (database/connection errors, lost worker, closed or full pool, cancelled run)
  are `host.InfraError`: a host call that hits one aborts the workflow run (further calls get
  `workflow.runAborted`), the lease is released and the run is replayed after the backoff
  (`failures` counts; `MaxRunFailures` blocks with `failing`). They are never journaled as `OccFailed`;
  an ordinary `wf.call` that hit one stays `started`, its invocation is recorded `unknown`, and the replay
  blocks it `ambiguous` (resolve/retry) like a lost worker. Events and schedules retry such a failure after
  the backoff without counting an attempt, so an outage cannot dead-letter anything.
* `events.RequeueReceipts` (`Service.RequeueReceipts`, `POST /_ext/manage/subscriptions/{id}/requeue`,
  operate permission, audited as `events.requeue`) turns `dead` and `blocked` receipts of the tenant back
  into due `failed` ones with no attempts counted. Schedule runs have no requeue: a dead slot is outside the
  15 min lookback anyway.
* `tx.start` / `wf.start` check the workflow contract's `Permission` against the subject (`tx.start`:
  reloaded current rights, `workflow.startDenied`; `wf.start`: the run's subject, instance `blocked: denied`).
* A run of an instance whose package has no active version blocks it with `packageDeactivated`
  (`workflow.PackageGate`, implemented by `manage` over the catalog); `Resume` continues it once the package
  is active again. Subscriptions and schedules of a deactivated package are not listed by the sources, so
  nothing is delivered or fired for them. A schedule never fires slots older than its activation
  (`ScheduleDef.ActiveFrom`).
* A transaction cannot commit while an invocation joined to it (`tx.call`, `tx.query`) is still running or
  was cut short because its caller finished: an un-awaited `tx.call` rolls the transaction back with
  `runtime.nestedInFlight` instead of committing half of it.

### 9.1 TypeScript authoring API audit (supersedes §2 where it differs)

**Kit entry.** `@iota-uz/sdk/runtime/kit` (`foreign`, `nativeEntity`, `nativeList`) is internal: only generated
`@iota-kit/*` modules import it. The author-facing `@iota-uz/sdk/runtime` has no way to mint a contract outside the
package namespace, and `contract()` is not exported. `collectManifest` rejects any contract whose ID is outside the
namespace unless it was issued by the kit; a locally built `operation('core.x', …)` that is only implemented is an error,
and so is a kit contract inside the package's own namespace. Go still re-validates every foreign reference and
implementation against the catalog snapshot at publish (separate slice).

**Rows and refs.** Every entity row from `get/find/insert/update` has `ref: Ref<E>` (attached in the host context
layer; overlay rows have none). `recipient.user` and `wf.task({assignee})` take `UserRef` (`Ref` to `core.user`; a
`Principal` is still accepted as an assignee). `$.user` only types/validates against `core.user` refs. `t.ref` /
`field.ref` take entities only; `ref(entity, key)` checks the key against the key type. A union of entities in
`tx.get/update/delete/put` is a type error. `ref` is a reserved field key.

**Fields.** `.optional()` is optional+nullable: `T | null` on read, `T | null | undefined` on write. `.default(x)` is
validated against the column and emitted in the manifest even when falsy (`false`, `0`, `""`, `[]`, `null`); it reads `T`
only for a non-nullable column. `access.read: []` is an error; `rows: 'none'` is the explicit deny. `t.json<T>` needs a
JSON `T` and rejects `null` unless `.nullable()` (as Go does); `NaN`/`±Infinity` fail `encode`.
`t.row(Entity|Overlay)` and `t.fields(Entity, pick?)` describe rows/fields for command outputs.

**Filters.** Ranges only on int/int64/decimal/date/datetime/string, `like` only on strings, nothing on json; `key` takes
`eq`/`in`; `createdAt`/`updatedAt` filter and order (also through ref paths). `orderBy` takes orderable paths only.

**Routes.** `route(id, {path, params})`: `params` must be exactly the `:name` segments (each a literal or whole
segment), required, kind string/uuid/int/int64 — types and runtime. `href` validates, percent-encodes and refuses
empty/`.`/`..`. `page(foreignRoute, …)` (route from the kit) is a screen replacement candidate: ID
`<namespace>.<exportName>`, `route` = replaced route; it is active only through a `screen` binding.

**Manifest.** Implicit client IDs lower the first character of the export name (`PolicyBadge` → `ns.policyBadge`).
In-package `nav.group`, `nav.parent`, palette `target` and nav patch refs must resolve; foreign ones are checked at
activation. A branded value nested in an exported plain object/array is an error ("export resources as top-level
named exports").

**Errors.** `validation()` accepts only `Issue.at(...)` instances; `ContractError` has no public constructor. Client:
`isError(err, DomainErr)` / `issuesOf(err, Issue)` narrow `CallError` / `RemoteError` with typed params.

**Server contexts.** `tx.query / q.query / ctx.query(Q, input)` run a query in the same snapshot/tx (host methods
`tx.query`, `q.query`). `tx.start` returns `WorkflowInstance<W>`; `tx.signal(instance, name, payload)` and
`ctx.signal(…)` are typed by the workflow (host methods `tx.signal`, `invoke.signal`; args
`{workflow, instance, name, payload}`). `wf.secrets` and `log` on Tx/Command/Query/Workflow contexts. `precheck`,
`guard` and `writeRule` get a read-only `GuardContext` (get/find/count/query/user/invocation/log; reject by throwing).
Endpoint requests have `body` (parsed), `rawBody: Uint8Array`, `text()`; the invoke input carries `rawBody` as base64.
Handlers may be sync. `tx.call(ordinaryCommand)` etc. fail with a readable message in the type error.
`Tx<'plain'>` is not assignable to `Tx<'derived'>`.

**Build (as implemented)**: one Node build process per operation (`check|build|test|kit`). Kit
modules (`@iota-kit/*`) are **bundled** into artifacts (self-contained, reproducible from tree +
kit hash); only `@iota-uz/sdk/runtime*` stays external for server/contracts graphs and is
resolved by the worker loader. Client bundles externalize `solid-js`, `solid-js/web`,
`solid-js/store` and `@iota-uz/sdk/runtime/client` through the shared registry. Generic
writes are allowed only to the caller package's own entities/overlays (`ErrForeignWrite`
otherwise); other packages' data is changed through their published operations. Native
entities carry `keyType`; slot declarations carry `props`.

**Host rules (as implemented)**: guard and query invocations may only call read methods
(`tx.get|find|count|query`, `q.*`, `secrets.get`); any write or `wf.*` call returns
`runtime.readOnlyContext` — enforced by the Go router, not only by the TS context.
`invoke.command`, `invoke.signal` and `wf.call` return `runtime.intentInTransaction` while a Go tx
is open in the invocation. `tx.query`/`q.query` return `runtime.nativeQueryUnavailable` for a
query with no implementation at all (neither a Go default nor a bound package export; Go-implemented
native queries run, see §9.3).
`wf.step`/`wf.transaction` results are JSON-normalized on first execution. HTTP endpoints:
`/_ext/endpoints/<namespace>/<path>` (sync response; session user, or the trigger owner for
`public` endpoints; tenant of an unauthenticated request via `runtime.Config.Endpoints.PublicTenant`;
`Set-Cookie` from handlers is dropped).

### 9.2 Integration notes from the end-to-end suite

* **Native lists** are declared by Go (`runtime.Config.NativeLists`, default `crm.lists.clients`; `catalog.NativeSet.RegisterList`),
  carried to the kit as `KitSource.Lists` and published as a reference owned by the namespace the list ID extends. An author
  writes `column(crm.ListsClients, {...})` (the name is the PascalCase of the last non-clashing ID segments: the route
  `crm.clients` already holds `Clients`). A list is not discovered from columns any more.
* `overlay(id, base, fields, { access })` accepts `access` (`fields` permissions and `read` rules), compiled into the overlay
  resource; the native list and the export hide a field the viewer may not read.
* The composition root passes the workflow engine as `host.Config.Signals`, so `tx.signal` / `ctx.signal` (including from a
  public endpoint) reach instances.
* `pkg/extruntime/e2e` drives the composed `runtime.Component` (real Node pool, real Node build worker built from
  `web/runtime`, real PostgreSQL through `pkg/itf`, real session middleware) through `/_ext/manage`, `/_ext/rpc`, page
  routes, the CRM clients list/export and `/_ext/endpoints`. Its packages are real TypeScript trees in `testdata/`, written
  with the public author API only. Prerequisites: `pnpm install` and `pnpm --filter @iota-uz/sdk-client-host-internal build`
  (the SDK emit type-checks `src/client-host/entry.ts` against the client-host declarations). Run with
  `go test -p 1 ./pkg/extruntime/e2e/`.

### 9.3 Native command boundary (as implemented)

* **Authoring (`pkg/extruntime/native`).** A module declares a native contract once, as a package-level value:
  `native.Operation | AtomicCommand | Command | Query[In, Out](id, native.Spec{...}, impl)` registered with
  `native.MustRegister` into `native.Default`. The wire contract is derived from the Go structs by `native.TypeOf`
  (explicit `json` names; `wire:"int64|decimal,precision=P,scale=S|date|datetime|uuid|enum=a|b|ref=<entity>|string,maxLength=N"`;
  int64 fields need `wire:"int64"` + `,string`; a pointer field is an optional key and needs `omitempty`; floats, maps,
  interfaces, embedded and recursive types fail loudly) or given explicitly in `Spec.Input/Output`. `impl` is a factory over
  `native.Deps` (the composition container) returning the typed Go default (`OperationFunc(ctx, native.Tx, in)`,
  `CommandFunc(ctx, native.CommandContext, in)`, `QueryFunc(ctx, native.QueryContext, in)`); it is instantiated per
  application on first use (`native.Bind`). `Spec.Permission` is an RBAC permission name: it is not a catalog resource, so
  it is not published in the contract; the host enforces it from the definition on every invocation, whichever
  implementation is bound. Native contracts reach the catalog and the kit through `catalog.DefaultNatives`;
  `catalog.Defaults()` is every native resource of the binary without application configuration (offline kits).
* **Dispatch.** A native contract without an open `slot_kind='command'` binding runs its Go default
  (`host.Config.Natives`, `Binding.Native`, recorded with an empty package/version/export); with a binding the package
  export runs instead — one active implementation, no fallback on error; unbinding restores the Go default. Everything
  else is the package path: input/output validation, permission, prechecks and operation guards of packages
  (`Catalog.Guards`), invocation records, nesting limit, closed domain errors. Operations and atomic commands run inside
  the caller's transaction (`tx.call`) or one the host opens; queries in the caller's transaction/snapshot (`tx.query`,
  `q.query`) or a read-only snapshot; ordinary commands get `CommandContext.Transaction` / `Call` / `Query`. The handler's
  `ctx` carries the transaction (`composables.UseTx`, so repositories join it), the subject's tenant and the pool, plus the
  root request's values (user, session) when the root came from HTTP. `Tx.AfterCommit` runs callbacks after the
  transaction commits (module events published post-commit); it is refused in a caller-owned transaction (subscription
  delivery, schedule slot). A package invocation nested under a Go frame runs on the worker of the nearest package
  ancestor, or takes a pool slot under a Go root.
* **Entry points.** Go code calls `Def.Dispatch(ctx, dispatcher, in)`. The runtime component provides
  `native.Dispatcher` (`runtime.HostDispatcher`: root invocation as the session user, trigger `ui`); without the runtime
  `native.ResolveDispatcher` falls back to `native.Direct` (Go defaults in-process, same validation and transaction shape,
  `Spec.Permission`/`Authorize` checked against the request user). Reference consumer: CRM — `crm.operations.upsertClient`
  (authorization Client.Create/Client.Update, value-object validation as issues `crm.invalidPhone|invalidEmail|invalidPin|
  invalidCountry|clientNotFound`, repository save + history in the caller's transaction, client events after commit) and
  `crm.commands.saveClient` (ordinary; Go default = one transaction around `upsertClient`). The client create form, the four
  edit forms and the chat "new chat" form dispatch `saveClient`; `ClientService.Create/Update` remain as Go API. A
  dispatch error renders 422 (validation) / 403 (denied) / 409 (domain).
* **Error mapping.** Data-layer failures reaching package code map to stable kinds instead of `runtime.hostCall`:
  unique → validation `data.unique` (issue phase `write`, path = field), reference → validation `data.reference`,
  native/foreign generic write → denied `data.nativeWrite` / `data.foreignWrite`, read denied → denied `data.denied`,
  hidden field → denied `data.fieldNotAccessible` (no field values), not found → domain `data.notFound`, row version
  conflict → domain `data.versionConflict`. The two `data.*` domain codes pass the closed-errors check of every contract.
  A Go default's own error is returned to Go callers unchanged (its chain intact) and recorded as internal.
* **Limits.** Package code nested under a Go default shares the Go frame's hold of the transaction: concurrent host calls
  from it (`Promise.all`) fail fast (`repo.ErrTxInUse`) instead of queueing — the same rule as write-rule guards.
  `upsertClient` cannot run inside a subscription delivery (after-commit callbacks are refused there). Other writers of
  CRM clients (inbound Twilio messages, `ClientService.Create` callers) do not go through the command.

