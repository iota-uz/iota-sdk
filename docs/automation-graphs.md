# ADR: Automation graphs

Status: accepted. Decision owner: SDK maintainers.

Automation graphs let a tenant describe "when this happens, do that" as a visual
graph. The graph runs on a Go workflow engine, is edited in a browser editor, and is
extended by modules and applications through registries. This ADR fixes the runtime,
the graph model, the package boundaries and the security rules. The interfaces it names
are the contracts of the implementation issues #1320, #1321, #1322 and #1330; the rest
of the work (#1325–#1350) refers here for decisions.

## 1. Goals and non-goals of v1

Goals:

- A visual graph workflow engine in Go with an editor written in Solid.
- Node types, triggers and connectors are registered by modules and applications, not
  hard-coded.
- Multi-tenancy, RBAC and audit are properties of the engine, not of each node.

Non-goals:

- Temporal or any infrastructure beyond PostgreSQL.
- Two-way synchronisation between a text DSL and the canvas. The graph JSON is the only
  source of truth.
- Running untrusted Go or Python. The Code node runs JavaScript/TypeScript in a sandbox
  (§9) and nothing else.

## 2. Runtime

### Decision D1: variant H — a Go-implemented workflow contract on the existing engine

A graph run is an instance of the Go-implemented workflow contract `automation.run` on
the extension-runtime engine (`docs/extension-runtime.md`, "Native workflow
implementations"). There is one engine, one set of `extruntime` tables and one
scheduler; the graph interpreter is an implementation of a contract, not a second engine.

- **State.** Runs, tokens, steps and waits live in the `automation.*` tables. They are
  written inside the engine's fenced `Transaction`, in the same transaction as the
  occurrence receipt. A run that lost its lease cannot write anything.
- **Journal.** For native contracts the engine journal is only a journal of receipts:
  consumed signals, task decisions, timers and steps. It is read lazily, so waking an
  instance costs O(new inputs), not O(journal size).
- **External calls.** A node with an external effect runs as a child instance of the
  contract `automation.call`, started from the run. The run itself never holds a
  database transaction across a network call.
- **Status.** The status of a run belongs to the engine (`workflow_instances.status`).
  `automation.*` tables do not duplicate it; they join it or react through the
  engine's `StatusObserver`.

### Rejected runtime alternatives

- **Replay interpreter** (re-execute the graph from the journal on every wake): after the
  interpreter binary is upgraded, replay repeats the effects of completed iterations of
  live runs, and a nondeterminism check cannot detect it.
- **Own executor** (claim, lease and heartbeat in the automation module): the engine's
  claim, fence and heartbeat are not exported, and `human_tasks` are bound by foreign key
  to engine instances, so this would be a second workflow engine.
- **Temporal:** adds a second stateful infrastructure service next to PostgreSQL, which
  v1 excludes.

## 3. Decisions D1–D7

| # | Decision |
|---|---|
| D1 | Runtime: variant H (§2). |
| D2 | Own graph format, using an established workflow vocabulary (behaviors, outcomes as output handles, field language, `nodes.<id>.output`, intent before call, immutable versions, layout stored apart from the version). The conformance fixtures are copied once from the product that established the vocabulary; there is no synchronisation. |
| D3 | The full v1 semantics, including parallel, join and foreach. |
| D4 | Expressions in CEL on `cel.dev/cel-go`. |
| D5 | The Code node runs on our own fork of kafig; TypeScript is compiled with esbuild. |
| D6 | The editor is our own Solid wrapper over `@xyflow/system` 1.0.0, pinned to the exact version. |
| D7 | One event catalog: `native.Event` is the only source of triggerable domain events. |

D7 was a proposal before this ADR. It is recorded here as accepted because the
notification bridge (#1325) and the trigger catalog (#1330) are built on it. The owner
confirms it when reviewing this ADR.

### Rejected alternatives for D4–D6

- **`fastschema/qjs`** (Code node): it offers memory, stack and time limits and Go function
  binding, but the §9 requirements of per-call output-size limits, network access only
  through an SSRF-checked host function, and secret masking are ours to enforce at the
  sandbox boundary, which we chose to do in our own fork rather than demonstrated to be impossible
  otherwise; kafig is that fork.
- **goja** (Code node): interrupt-only resource control (no memory limit) and weaker
  coverage of the ECMAScript output of esbuild.
- **`@dschz/solid-flow`** (editor): a third-party wrapper that follows `@xyflow/system`
  releases with a delay and fixes our rendering and versions to its choices; our own thin
  wrapper over the pinned system package avoids that.

## 4. Package boundaries

```
pkg/automation/
  graph/       graph format, catalog types, validator, pure interpreter, dry run
  expr/        field language, CEL environment, templates
  engine/      the automation.run / automation.call implementations
  catalog/     registries of node types, trigger sources, connectors; tenant catalog
  connectors/  connector interfaces, encrypted connection store, OAuth2, usage
  code/        Code node sandbox
  builtin/     built-in node types
modules/automations/        module: tables, controllers, pages, permissions
web/automation-editor/      Solid editor, published as @iota-uz/sdk/automation-editor
```

Dependency rule:

- `graph` and `expr` are pure: no database, no I/O, no imports of SDK modules,
  `pkg/extruntime/...` or `pgx`. `graph` imports `expr`; `expr` never imports `graph`.
- `catalog` depends on `graph`, `expr` and the native-event interface. `connectors`
  imports `catalog`, never the reverse.
- `engine` depends on interfaces only: the registry (`Registry.Node`), the connection
  store (`ConnectionStore`) and the engine's `NativeRunner`. It does not import modules.
- `modules/automations` and `builtin` sit at the edge and wire the rest together.

## 5. Graph model and execution semantics

### Format

A graph version (`graph.Graph`) has a name, `run_as`, 1–40 triggers, up to 200 nodes
and 500 edges. A node has an `id`, a `type` (a `NodeSpec` key of the catalog) and
`params`. An edge goes from `{node, outcome}` to a node id or `ignore`; a trigger's only
outcome is `fired`. The version is immutable. Id, version number, enabled flag and
canvas layout belong to the owner (`modules/automations`), not to the version content.
The format is published as JSON Schema (`graph.JSONSchema()`) for clients in other
languages.

Each node type has a **behavior**: `branch`, `switch`, `wait`, `effect`, `call`, `end`,
and (D3) `parallel`, `join`, `foreach`, `collect`. Behaviors are an extension point
(`BehaviorImpl`); a node whose behavior the interpreter does not execute is rejected
when the catalog is loaded and on `nodes[i].type`. A subject is `{entity, key}` for any
SDK or application entity, plus `conversation` and `none`.

Expressions are CEL with strict type checking at save time. The allowed functions are the
standard CEL functions, `ext.Strings`, `ext.Bindings` and `join(list<string>, sep)` (the
function form of `list.join(sep)`: `join(issue.labels, "\n• ")`), minus the overloads
`timestamp_to_string`, `duration_to_string`, `uint64_to_int64`, `timestamp_to_int64` and
`duration_to_int64`; the `uint` and `bytes` types are not part of the subset (so
`string(timestamp(...))` is rejected at save time); regular expressions are RE2;
there is no clock function — `now` is fixed per step. A reference to missing context
produces `missing_context` (a path, not an error) and is handled by the engine, not
treated as `false`.

A graph may have triggers about different subjects (a record, an incoming mail, a
message). The validator reads every node by the triggers it can be reached from: the
subject variables (`<entity>.*`), `event.*` and the subject restriction of a node type
(`human` takes a card) are checked against what **all** of those triggers have, with the
same type. A variable some other trigger of the graph has but one of these lacks is
`unknown_variable_for_trigger`; its message names the node, the variable and the trigger.
A mixed subject of the catalog only names such a graph; it does not stand for the
subjects of its triggers. The interpreter runs by the subject of the event that started the
run; its expressions are compiled in the scope of the same triggers, and a node several
event kinds lead to is given the whole event, not only the fields the check allows.

### Tokens

Execution is token-based: a run holds one token on the main path and `parallel` and
`foreach` create child tokens, which `join` and `collect` merge. A token moves along
edges by the outcome of the node it just left. Waiting, timing out and task decisions
resolve through signals on the engine journal.

### Intent → call → apply

A node with an external effect executes in three steps:

1. **Intent.** In a fenced `Transaction` the engine records the intent and the
   idempotency key `run:node:token:seq`, and pre-allocates the invocation id.
2. **Call.** The effect runs outside any transaction through `Effect`. The engine marks
   the occurrence started and counts the attempt.
3. **Apply.** In a second fenced `Transaction` the engine stores the output, chooses the
   outcome and moves the token.

If a process dies after the intent and an attempt may have executed, the next attempt is
told so (`attempt > 1`). For a non-idempotent node the
attempt is ambiguous, so the engine does not repeat it. The call ends on the node's
`failed` outcome, and the failure carries the error code `outcome_unknown`; there is no
separate `outcome_unknown` edge. A graph that wires `failed` can branch on that code. If
`failed` is not wired, the child `automation.call` instance and the run block, and an
operator resolves or retries them through the engine's management API.

### Retries

The engine retries only nodes that declare themselves idempotent (same key on every
attempt) and calls that answer 429 with a recorded `RetryAfter` inside the deadline. A recorded 429
is a known refusal, so it is safe to retry; an ambiguous attempt (see above) is not.
Everything else fails, or fails with `outcome_unknown`.

### Lock order

Subject first, instance second. A consumer's own lock (for example an advisory lock on the
subject) is taken inside the `Transaction` callback before any access to the instance
row; the engine takes the instance lock only when the transaction ends. Signal delivery
and task decisions follow the same order.

### Limits

Per graph: ≤ 200 nodes, ≤ 500 edges, ≤ 40 triggers; expression limits on AST nodes,
depth, literal size and CEL cost; 1 MiB for an event payload or a node output. Per
contract: concurrency (default 4 runs per process, its own slot pool) and a per-run
timeout (default 5 minutes until the next suspension).

### Cancellation

Terminating a run terminates its unfinished children (`TerminateTree`, by `parent_id`).
When a run completes or fails, its waiting occurrences and open tasks are cancelled in the
same transaction, so a cancelled task disappears from the inbox. A step already running
finishes; later host calls are rejected.

### Versions and `StartV2`

A published version never changes. Editing creates a new version. A run keeps the version
it started on. `StartV2` (restart) starts a new instance on the version the caller selects, with
`predecessor_id` set so history stays linked. It does not require the target to be newer
(same rule as workflow start-v2 in the extension runtime).

### Run-as

Run-as is a property of the graph: `owner` (default) or `initiator`. `owner` runs as the
human who last enabled the graph; that identity is stored as the trigger owner
(`trigger_owners`) and changes only when a human enables the graph again. `initiator` runs as the user who caused the event, with their
current permissions, and falls back to the owner when the event has no initiator. A
blocked or deleted subject blocks the run; it does not silently run with different rights.
The subject's permissions are reloaded on every step, not once per run, so a long-running
run honors permission changes made while it waits; a subject that lost the right blocks
the run.

### The engine (`pkg/automation/engine`)

`engine.NewRuntime` builds the engine over the workflow engine, the catalog registry and the
user loader. A composition registers four things from it: `Runtime.Natives()` (the native
contract `automation.run`, with its own `Limits.Concurrency` slots) through
`wiring.ContributeNativeWorkflows`, `Runtime.Consumer()` (outbox consumer `automations`)
through `wiring.ContributeConsumers`, `Runtime.Schedules()` through
`wiring.ContributeSchedules`, and `engine.Triggers()` in the catalog registry (the schedule,
manual and `automation.run.finished.v1` events). `Runtime.Serve` runs the maintenance loop:
queued conversation runs, runs of disabled graphs, retention.

- **State.** A run is an `automation.runs` row and an instance whose input is `{run_id}`. Its
  tokens, steps and waits are written in fenced `NativeRun.Transaction`s named `tx#<tx_seq>`;
  each transaction takes the subject's advisory lock first and sets
  `app.automation_cause`, so events it publishes carry `{run_id, depth}` in
  `outbox_events.cause`.
- **Inputs.** Matched events and executor results arrive on the signal `inbox`: the consumer
  signals a matched event wait (`{wait_id, event}`; a new wait also looks up, once, the events
  delivered while its own transaction was open), a call executor of #1336 signals a
  `WaitResult`. The run consumes the signal `inbox#<inbox_seq>` and applies it in the next
  transaction, which moves `inbox_seq` on. A timer is the sleep occurrence `timer:<wait_id>`.
- **Task decisions.** A task the core opens for a wait (`WaitSpec.Task`) is the occurrence
  `task:<wait_id>`: the runner reads its decision through `NativeRun.Task` and applies it
  (`TaskResolver`, or the decision's `outcome`). Its decision never arrives on `inbox`; a
  `TaskBridge` that sets `WaitSpec.Task` adds no delivery of its own.
- **Wait filters** read `now` as the time of the event they test, at the router and in the run.
- **Owner notice.** A failed run publishes `automation.run.failed.v1` once; the composition
  contributes `ownernotice.Contribute`, whose core notification definition delivers it to the
  graph's owner.
- **Run history.** Each step keeps the sensitive event fields the run had read when it ran
  (`steps.sensitive`); `GetRun` hides the data of a step whose fields the viewer may not read.
- **Node executors.** An effect node type implements `engine.EffectNode` (applied in the step's
  transaction); call and human nodes go through `Config.Calls` / `Config.Tasks`; a Code node
  type implements `engine.CodeNode` so `EnableReview` shows its code and digest.

### Calls, answers, inbound and authoring (#1336)

- **Calls.** `Config.Calls` left nil is the engine's `CallRunner`: each call node runs in a child
  instance of the native contract `automation.call` (registered by `Runtime.Natives()` next to
  `automation.run`), started in the step's transaction with the start key
  `automation.call:<run:node:token:seq>`. The child opens the connection
  (`Config.Credentials`, a `connectors.CredentialSource`), checks the run-as's right to the node,
  calls `Execute` in the journaled `Effect` `call#k` and signals the parent's `inbox` with a
  `WaitResult` in the transaction of its receipt. Credentials are opened outside the `Effect` and
  never journaled; outputs and error messages are masked with them. A 429 (`RetryAfter` or
  `rate_limited`) and a retryable error of an idempotent node are repeated after
  `max(RetryAfter, backoff)` while the deadline holds (`CallLimits`); an attempt after a crash of a
  node that is not idempotent is `outcome_unknown`: the failed outcome when it leads to a node,
  otherwise the child blocks (`workflow.Ambiguous`) and the run shows `blocked` (`call_blocked`).
  `Runtime.Operations()` resolves or retries such a call (`Automations.Operate`). A failed call or
  effect puts `output.error {code, message, retryable}` (`nodes.<id>.output.error.message`). A node type
  whose static `Output` declares its own `error` keeps that field as it declares it (a string error has
  no `.message`); an `OutputFrom` schema's `error` is replaced by the object.
- **Ask.** A call node that implements `engine.Asker` gets a correlation token
  (`CorrelationFrom(ctx)`, base62, 12 characters, `automation.correlations`) before `Execute`;
  after it the parent opens an event wait that only inbound events carrying the token wake;
  `Asker.Answer` maps the event to the outcome, the wait's timeout to `timeout`. A single wait
  closes with the first answer: a later one is `ErrCorrelationClosed`. An ask also implements `engine.AnswerEventer`: it names
  its answer event and single policy before sending, the token is issued with them, and only that
  event takes it, before or after the wait opens (an ask without it is refused when a graph is saved).
- **Inbound.** `Runtime.Inbound()` writes connector events into the outbox in the caller's
  transaction, deduplicated by `automation.inbound_dedup` and with
  `cause = {inbound: {connection_id, correlation, by_self}}`. `engine.InboundHandler` serves the
  webhooks of `connectors.Connector`s that implement `WebhookReceiver` (404 for an unknown or
  foreign connection and a failed `Verify`, 413 over `MaxBody`, 200 after the commit, 5xx on a
  database failure); `Runtime.NewPoller` polls `Poller` connectors of connections an enabled graph
  uses and moves the cursor (`UpdateInbound`) after the commit. A trigger's `collect` merges
  inbound events with one key inside a window into one run (`events`; `automation.inbound_collect`).
- **Loops.** A trigger does not start on an event its own graph's step published (the `Origin` in
  `cause.automation_id`) nor on a message its connector sent itself (`event.by_self`), unless the
  trigger sets `include_self`.
- **Human node.** `engine.NewHumanNode()` (type `human`) with `engine.NewHumanBridge` as
  `Config.Tasks` opens a task of #1327 (assignees, quorum, veto, delegation, escalation); its
  buttons are its outcomes, its timeout `timeout`; the observer below also refuses a decision without
  the comment its button requires or with fields its form refuses. `engine.NewTaskAssignedObserver` is the
  `workflow.TaskObserver` that publishes `automation.task.assigned.v1` (`engine.TaskAssigned`)
  with the newly assigned users on creation, delegation and escalation, in the task's transaction.
- **Context.** `Runtime.BuildRunContext` is the one builder of a run's context (live trigger and
  step, dry run, test node): `event`, `events`, `nodes`, the subject's variables, `subject {entity,
  key, url}`, `run {id, url}` (`Config.Links`, `engine.NewLinks`), `tenant`, `now`.
  `engine.CheckRunContext` lists the required fields a context leaves out.
- **Authoring.** `Runtime.Authoring()`: `DryRun` runs the interpreter with the node types'
  `DryRun` modes (an `execute` call node runs for real unless `Decisions` is `scripted`; an effect node
  never runs in a dry run, `execute` or not: its effects are traced, as `capture`; `capture` traces the
  rendered params, `forbid` stops on entry; every node type declares its mode, `dry_run_undeclared`);
  `cached` decisions of `engine.DecisionNode`s come from `automation.decision_cache` (30 days).
  `TestNode` runs one node once as the author: no run, no steps; an effect node's transaction is
  rolled back. `SearchSubjects` uses an optional `SubjectSearcher` of the subject loader.

## 6. Triggers and the event catalog

- **Domain events** come from the outbox through the event catalog (`native.Event`, D7).
  One engine-wide consumer reads each event once and fans it out to the enabled graphs
  whose triggers match. A subscription per graph is rejected: it multiplies queries and
  retention-window scans by the number of graphs of all tenants.
- **Schedule** triggers use the engine scheduler.
- **Manual** triggers start a run on request.
- **Connector inbound** (messages, polling) are trigger sources contributed by connectors.

Not in v1: a webhook trigger and a trigger on the business-history record.

A trigger carries a CEL filter and `params` checked against the event's declared trigger
parameters. A consumer works only inside the tenant of the event it received.

## 7. Connectors and credentials

- A connector contributes node types and trigger sources and declares how it
  authenticates (API key, basic, OAuth2, custom).
- Credentials of a connection are stored in `automation.connections`, encrypted with
  AES-256-GCM. The key comes from configuration; a fresh nonce is used per write and the
  AAD binds the row to `tenant_id|id|kind`, so a row moved to another tenant or kind does
  not decrypt. The keyring has several keys; the active one encrypts, and `Rewrap`
  re-encrypts older rows, so keys can be rotated. Without a configured key the store
  reports `ErrNotConfigured` and tenant connectors are unavailable.
- **Platform credentials** (from application configuration) are available to tenants by
  the connector's policy: `auto` for everybody, `opt_in` only after an explicit decision
  of the tenant administrator. The platform AI key is `opt_in`. Usage is recorded
  separately for platform and tenant credentials.
- A secret never appears in `Connection`, in `List`, in errors or in logs. The engine
  decrypts credentials and hands them to the node for the call; a node does not store them.
- A node that needs a connector refers to it with the reserved param `connection`. A
  tenant without a working connection of the needed kind does not see the node or trigger
  in its catalog.

Implementation notes (#1330):

- `catalog.NodeType.Requires` lists kinds the node works with: any one of them makes it
  available. The registry adds the reserved `connection` param (a connection ID or
  `platform:<kind>`) to the params of such nodes and triggers; a node may not declare it.
  Left out, the only available connection is used (`catalog.ResolveConnection`);
  `catalog.ConnectionRule` reports `connection_unavailable` and, when several fit,
  `connection_required`.
- The keyring is `AUTOMATION_CONNECTIONS_KEYS` (`automationconfig`): `id:key[,id:key]`,
  32-byte keys in base64 or hex; `AUTOMATION_CONNECTIONS_ACTIVE` names the active key
  (default: the first). To rotate with several replicas: deploy the new key listed but not
  active, then make it active, run `Store.Rewrap` until it returns 0, then drop the old key.
- The engine opens a call's connection with `connectors.CredentialSource.Open`.
- `connectors.SecretResolver` also takes the `Directory`, which says which credential
  fields are `x-secret`.

## 8. RBAC and agents

Permissions: `Automations.Read`, `Automations.Author`, `Automations.Enable`,
`Automations.Operate`, and `Connections.Manage`. A node or event type may require an
additional permission; the catalog an author sees is filtered by it, and sensitive event
fields are removed from the schema for authors who lack the right.

Three actions are human-only and fail for an agent with `HUMAN_REQUIRED`:

- enabling a graph;
- answering a human node (a task);
- managing connections.

MCP operations for automations are declared in the SDK operation registry (#1084), which
is where this rule is enforced.

## 9. Code node

The Code node lets an author write a TypeScript function that runs in the kafig sandbox.

- Contract: `run(ctx, env)`. `ctx` carries the validated node inputs; `env` carries the
  controlled host functions.
- CPU, memory, wall-clock and output size are limited per call.
- Network access goes through a host function with an SSRF policy; the sandbox has no
  direct network, file or process access.
- Secrets are masked in logs and outputs.
- Before a graph that contains a Code node is enabled, a human sees its code and the
  names of the secrets it can reach (never their values). Enabling is a human-only action (§8).

The sandbox is `pkg/automation/code` (#1329):

- `Compiler` transpiles with esbuild (ES2020 IIFE, types stripped, never checked); any
  static `import`/`require` is a compile error with its position; the source is at most
  100 000 characters.
- `Engine` compiles the kafig runtime once per process and gives every run a fresh
  instance: a wasm page cap (default 2048 pages, 128 MiB) is the hard memory limit, the
  context deadline (default 30 s, at most 5 min, closing the instance when it passes) is
  the hard time limit. Concurrent runs are bounded by a memory budget
  (`budget / (4 × cap)`) and per tenant.
- The Go router of host calls is the trust boundary: `fetch`, `log`, `uuid`, `digest`,
  reading a response body and the one-time result are the only calls, and each parameter
  is checked in Go. The driver removes kafig's `host` global and freezes `JSON` before
  the code runs, so the code reaches the host only through `env`. A fetch request is
  serialized once from null-prototype objects and staged by a synchronous call, where Go
  admits its exact bytes (16 MiB of staged requests per run); the async call kafig
  queues carries only the ticket, and the driver caps the number of fetches. Response
  bodies wait in Go until read, at most 64 MiB per run.
- `NewGuardedFetcher` is the network policy: https on port 443, no local or metadata
  names, a name with any private DNS answer is refused as a whole, the dialed address is
  checked again, at most 20 redirects each checked the same way, only safelisted headers
  (no `Authorization`, `Cookie` or API-key headers) once a redirect leaves the origin, no
  proxy, the body capped after decoding.
- Output, logs, error messages and stacks are masked (`Masker`: exact, JSON-escaped and
  URL-encoded forms) before they leave the package. Stacks are mapped to the source.
- `Declarations` writes the `.d.ts` that types `ctx` and `env` in the editor.

**Open item:** type checking of TypeScript in the editor was a proposal before this ADR. It is not decided here and remains open.

## 10. Phases and the notification replacement

The work is split so that each part is shippable and refers to this ADR:

1. Foundations: always-on event core with Go consumers (#1320); Go workflow
   implementations in the engine (#1321); graph format, CEL and the pure interpreter
   (#1322); registries and the connection store (#1330).
2. The graph engine, tables and dry run; the editor; the first built-in nodes and
   connectors.
3. Parallel, join and foreach (#1328); the Code node (#1329); human tasks (#1327);
   schedules (#1326).
4. The notification replacement (#1325).

**Notification rules.** A notification rule becomes a preset graph over the existing
delivery layer. The delivery layer (channels, preferences, persistence, realtime) stays
as is; what changes is how a rule decides to notify. The same event catalog drives both.

## 11. Consequences

- The extension runtime engine gains native contracts, so the core event outbox and the
  workflow engine run without Node and without `EXTRUNTIME_ENABLED`.
- The graph format is owned by the SDK; fixtures protect compatibility.
- The Code node and connector credentials are the main security surface; both are
  confined by the decisions above.
