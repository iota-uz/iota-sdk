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

A graph run that starts executes as an instance of the Go-implemented workflow contract `automation.run` on
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
  mcpops/      MCP operations of agents: read tools, validate, dry run, changes kinds, guard
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

- **State.** Every run has an `automation.runs` row; it executes as an `automation.run`
  instance (input `{run_id}`, `runs.instance_id`) only once it starts. A run that never
  starts has no instance: `filter_failed`, one blocked at the start (`no_owner`,
  `depth_exceeded`), or one cancelled while queued. Run history therefore reads
  `automation.runs` with a left join to the instance, never an inner join. Its
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

### Built-in nodes (#1337)

`pkg/automation/builtin` registers the node types every tenant's catalog has without a connector; the
`automations` module registers them in its `catalog.Registry` when it is composed (`builtin.Register` for
an application that builds its own registry). Params are in the field language; a `"*"` segment of a
`Values`/`Templates`/`Expressions`/`Mixed` path stands for every key of a map param (`values.*`,
`headers.*`), and each key's expression is checked at save time at its own path
(`nodes[i].params.values.total`). A `Mixed` param is CEL or, with `{{ }}`, a template.

| Type | Behavior | Params | Outcomes | Output |
|---|---|---|---|---|
| `set` | effect (in the step) | `values: map<name, cel>` | `done` | one field per name (`OutputKeys`) |
| `set_state` | effect (in the step) | `key` (tpl, ≤ 200 chars), `value` (cel, ≤ 16 KiB as JSON; absent with `delete`), `ttl` (`20m`, `2h`, `7d`: 1 minute to 365 days; default none), `delete` | `set`, `failed` | `{key, deleted, expires_at?}` |
| `code` | call | `lang ts\|js`, `source` ≤ 100 000, `secrets[]`, `connections[]`, `fetch`, `timeout` (30s, ≤ 5m), `outcomes[]` (≤ 20, default `done`) | declared, `failed` | `{output, logs}` |
| `http` | call | `method`, `url` (tpl), `headers`/`query` (map of tpl), `body {kind none\|json\|form\|text, value: cel\|tpl, template: tpl}`, `auth {secret (a NAME), scheme bearer\|basic\|header, header} \| {connection}`, `timeout` (30s, ≤ 120s), `success` (cel over `status`, `headers`, `body`, `url`; default `status < 400`), `idempotency_header` | `success`, `failed` | `{status, headers, body, url, body_omitted?}` |
| `start_automation` | effect (in the step) | `automation_id`, `trigger_id?`, `input: map<name, cel>`, `subject` (cel `{entity, key}`; default the run's) | `started`, `failed` | `{run_id}` |
| `run_automation` | wait with effect | as `start_automation`, `timeout` (1h, ≤ 30d) | `done`, `failed`, `timeout` | `{run_id, status, output, output_withheld}` |
| `action` | call | `action` (a key), `args: {<name>: tpl for a string type \| cel}` | the action's, `failed` | the action's |

- **Code.** The source compiles at save time (`engine.NodeValidator`): a syntax error or an import is
  `VALIDATION_FAILED` at `nodes[i].params.source` with line and column. The node declares
  `NodeSpec.StepContext`: the step's context (`event`, `nodes`, the subject's variables, `subject`, `run`,
  `tenant`, `now`) is its `ctx`. The engine keeps that context in `automation.call_contexts` for the call's
  child instance and drops it with the call's result. Only the declared secrets are in `env.secrets`;
  `env.fetch` exists with `fetch: true`. A failure is `failed` with `output.error.code` the sandbox's
  reason (`timeout`, `exception`, `network_blocked`, `unknown_outcome`, `secret_missing`,
  `connection_missing`, `connection_origin_mismatch`, ...) and the masked logs; a run that ended without a
  result is `outcome_unknown`. The node is not idempotent: it is never repeated after an unknown outcome.
  `EnableReview` lists its source, secrets, fetch and connections.
- **History.** `automation.run` and `automation.call` are private native contracts
  (`workflow.NativeContract.Private`): the workflow history API (`List`, `Get`, `Extensions.Read`) shows
  their shape and leaves out their input, results, errors, blocked detail, signal payloads and task
  decisions. The data of
  a run is read through `GetRun`, which hides the steps a viewer may not read.
- **Secrets.** Code and HTTP read a declared name (`^[A-Z][A-Z0-9_]{0,63}$`) from the tenant's store, the
  container's `host.SecretResolver`: `CONNECTION__FIELD` is the secret field `field` of the tenant's
  connection `connection` (`TELEGRAM__TOKEN` → `telegram.token`, an `x-secret` field of a connected
  connection); any other name goes to the resolver as it is (what the application adds behind it).
- **Connections in code.** A connector that implements `connectors.CodeAccessor` declares its API's
  `BaseURL` and the methods of `env.connections.<name>` (`get(path)`, `list(path)`, each an HTTP method);
  `Authorize` adds the credentials to a request in Go, so no token reaches the sandbox. Every value of the
  credentials and of the authorization it added is redacted from the response before the code sees it:
  header names and values, the URL, and the body (as JSON when it is JSON: strings, keys and numbers
  decoded first, so an escaped spelling is found). A path or URL off the base URL's origin, and a redirect
  off it, is `connection_origin_mismatch` (the fetcher's `SameOrigin`); a name that is not a connected
  connection with code access is `connection_missing`. `auth: {connection}` does the same for an HTTP node.
- **HTTP.** The request goes through `code.NewGuardedFetcher` (https on 443, every hop's DNS checked,
  `Authorization`/`Cookie` and every non-safelisted header dropped once a redirect leaves the origin, and
  the headers of a secret's authorization and the idempotency key whatever their names: `Sensitive`).
  With `auth` the URL names its https host before any template (`https://api.example.com/{{ ... }}`). A
  refusal is `network_blocked` with the host only; a DNS or dial failure is `unavailable` (nothing was
  sent); a body over 10 MiB or a header section over 64 KiB (`code.WithMaxHeaderBytes`) is
  `response_too_large`; a 429 with `Retry-After` is repeated by the engine's
  rule. Only `GET`/`HEAD` and a request with `idempotency_header` (it carries the call's idempotency key)
  are idempotent (`catalog.IdempotentCall`); the header is one of the request's own (not one the
  transport sets: `Host`, `Content-Length`, `Connection`, `Authorization`, ...), and such a changing
  request is not followed off its origin (`redirect_refused`), where its key would not go. A response the success expression rejects takes
  `failed` with the response in the output and `output.error.code = http_status`. The node type declares
  `NodeSpec.MaxOutputBytes` above the engine's payload limit (6× the body and the headers, the worst JSON
  escaping), so any body up to 10 MiB comes back with its headers. `body.value` is CEL or a `{{ }}`
  template: as a template, the text of a `text` body, the JSON text of an object or array of a `json`
  body, the encoded form of a `form` body; as CEL, its value, so a CEL string of a `json` body is a JSON
  string whatever its text (the step passes the mode to the node: `graph.Effect.Templated`,
  `catalog.CallInput.Templated`). `body.template` is always a template.
- **Another graph.** The run-as needs `Automations.Operate`, as for a manual start (`forbidden`). Both
  nodes start the target's enabled version in the step's transaction (`engine.RunStarter`,
  `Runtime.StartFromStep`) through `trigger_id`, else its first «Called by an automation» trigger
  (`automation.called.v1`, params `{input_schema}`, payload `{input, caller_run_id, caller_automation_id,
  initiator_user_id}`) by id, else its first manual one, else its first trigger. Through a called or
  manual trigger `input` is `event.input`, validated by the trigger's `input_schema`; through an event or
  schedule trigger `input` is the event's payload, checked against the event's schema: its required
  fields and their types (`invalid_input` otherwise). A graph of another tenant or a disabled one is `not_enabled`. The started run's depth is the
  run's plus one, so `A → B → A` ends at `Limits.MaxDepth` with `depth_exceeded`. Its data derives from
  what the step's run read: it inherits the run's sensitive fields (a viewer who may not read them does
  not see its steps), and its run-as must be allowed to read them (`forbidden`). The run keeps that
  provenance (`automation.runs.inherited`): a restart carries it and checks the new run-as again. It is about the run's
  subject unless `subject` says otherwise; `run_automation`'s run on the run's conversation takes its turn
  inside the waiting runs' (the run and the ancestors that wait for it on that conversation, however
  deep), `start_automation`'s queues behind them. `run_automation`'s run is a child
  instance of the step's run (cancelling the run terminates it); its wait listens for
  `automation.run.finished.v1` about that run (`engine.EffectWaitKeyer`) and takes `done` or `failed`
  by its status (`engine.WakeResolver`). Its output comes from the run when the event withheld it (the run
  read sensitive fields) and this run's run-as may read them; otherwise `output: null,
  output_withheld: true`. A run started by `start_automation` lives on its own. The composition provides
  `engine.RunStarter` (the `*engine.Runtime`; `modules/automations` provides it whenever it runs the
  engine); without it the nodes fail with `runs_unavailable`.
- **Run output.** An `end` node's `output: map<name, cel>` is the run's output, which a
  `run_automation` reads as `nodes.<id>.output.output.<name>`; an `end` without it gives its params
  (`{reason}`), as before. The graph version may declare `output_schema` (JSON Schema of `type:
  object`): a save checks every `end` against it (required names present, no name a closed schema
  does not take, each expression's CEL type one the property's `type` takes; a timestamp is a string,
  a duration a number), and the run checks the computed values against it when it ends: a mismatch
  fails the run with `INVALID_OUTPUT`.
- **Before the call.** A failure to read a secret or open a connection, or of the compiler, is
  `unavailable` (retryable), never `outcome_unknown`: nothing ran.
- **Actions.** A graph names an action as `{"type": "action", "params": {"action": "core.user.block",
  "args": {...}}}`. The `action` node dispatches (`graph.Dispatch`, `graph.Resolve`) to the node type
  `action.<key>`, which carries the action's `ActionSpec` (label, group, args and output as
  `manifest.Type`, outcomes, `Permission`, `Idempotent`, `Subjects`, declared `Errors`): its args, outcomes
  and output are checked like any node's. A module registers an action once, in its `native.Set`:
  `actions.Native(template, build)` is the action's command contract, implemented by the action `build`
  makes from the application's dependencies; the set refuses a key twice. The automations module makes
  the node types of the set's action commands when the application starts. A call of an action is
  `wf.call` of its command (`catalog.CommandCall`, `NativeRun.CallAs`) under the call's invocation ID
  (from its idempotency key): the workflow journals it and the host records the invocation, so a repeat
  returns the recorded outcome and one that may have run is never repeated blindly. A command that has not
  started runs within the call's deadline and holds one of the tenant's call slots (`CallLimits`); a
  recorded one is read back. The action of a command is bound to its definition in the set
  (`actions.TemplateOf`), never to its ID. The command runs the
  action in a host transaction as the run-as (`actions.InvocationID(ctx)` is the invocation's ID). An
  author without the permission does not see the action; the run-as's permission is checked on every
  call (`forbidden`, nothing done); args are validated before the command (`invalid_args`); a declared
  `*ActionError` is `failed` with its code, dots and all. `servicectx.Context` builds the services' context
  (the run-as as the user). The SDK's actions: `core.user.block`, `core.user.unblock`,
  `core.group.add_user`, `core.group.remove_user`, `crm.chat.send_message`.

### Memory between runs: `set_state` and `state` (#1390)

A graph remembers facts between its runs (a chat already has a task, a release is already announced, how many
times a rule fired). `automation.state(tenant_id, automation_id, key, value jsonb, expires_at, updated_at,
updated_by_run)` holds them per tenant and automation; `pkg/automation/state` is its one writer and reader.

- **Write.** The `set_state` node (`behavior=effect`, outcomes `set` and `failed`) writes or deletes one key in
  the transaction of the step, so the write commits with the step or not at all, and a node that fails
  writes nothing. It declares `DryRun: capture`: a dry run and a backtest trace the rendered key and value
  and write nothing. A refusal is the node's `failed` outcome with `output.error.code`:
  `state_value_too_large` (over 16 KiB as JSON), `state_value_invalid` (no JSON form, or a NUL character),
  `state_key_invalid` (empty or over 200 characters),
  `state_ttl_invalid` and `state_limit_exceeded` (more than 10 000 live keys; expired ones do not count, and
  rewriting a key that is there is always allowed).
- **Read.** In the expressions and templates of a step (not of a trigger's filter), `state` is a lookup-only
  map: `state.get(key)` is the value or `null`, `state.has(key)` tells whether the key is there, and
  `state[key]` / `key in state` work too. `get` and `has` are member functions of every `map<string, dyn>`.
  Keys are read one at a time from the step's own transaction, so a run sees the values as of its step
  (its own earlier writes included), and a key read twice in one step has one value. A key past its
  `expires_at` is not seen, whether or not the sweep has removed its row (`Runtime.Prune` removes them in
  batches). A subject variable named `state` wins over the memory.
- **Dry run.** `DryRunInput.AutomationID` (the saved graph; `id` of `automation_dry_run` and of the editor's
  `automations.dry_run`) lets the dry run read that graph's memory; it never writes it.
- **People and agents.** `Runtime.Memory()` lists keys (with a search), reads, sets and deletes them with
  `Automations.Read` / `Automations.Author`; deleting a graph drops its keys. The editor's graph settings
  show the list (search, edit the value as JSON with a lifetime, delete), through
  `automations.state_list|state_set|state_delete`. `mcpops.WithMemory(rt.Memory())` adds the MCP tools
  `automation_state_list` and `automation_state_get` (read) and the guarded changes `automation.state.set` and
  `automation.state.delete` (author).

### Effect nodes beyond a single outcome, and the notification nodes (#1338)

An effect node normally takes its done outcome (or its failed outcome when `ApplyEffect` fails). Three
optional interfaces of the node type widen that:

- `engine.OutcomeEffectNode` (`ApplyEffectResult`) picks one of the outcomes the node type declares, with its
  output; an outcome it does not declare is the node's failure (`effect_failed`). `TestNode` shows it.
- `engine.EffectPreviewer` (`PreviewEffect`) is what a dry run shows for the node instead of the bare
  captured effect: its outcome and output computed without writing or sending. The step stays `captured`.
- `engine.EffectPreparer` (`PrepareEffect`) reads before the step's transaction opens, when the node must not
  wait for a connection while the transaction holds one; the result reaches the effect as `StepContext.Prepared`.
- `engine.GraphChecker` (`CheckNode`) adds rules over the whole graph that uses the node (what its trigger
  events offer), checked when the graph is saved, dry run and enabled.

`StepContext` carries the step's `NodeID` and `StepKey()` (run, node, token and sequence: the same when the step
is applied again) and `Trigger`, the event that started the run (key, id, the payload the run-as could read and
`ActorUserID`, the run's initiator, which is what the event's `actor` recipient resolves to). A dry run and a node
test give the preview the event that started the dry run (the first one a trigger took, as the interpreter chose
it) or the test's event, with the author as initiator.

`pkg/automation/builtin/notify` holds two effect nodes and the notification definition `automations.notify.v1`.
The `automations` module registers both with the composition (`notify.Register` over its catalog registry with
core's `AudienceSender`, notification catalog and contact resolvers; `notify.Contribute` for the definition), so
they are offered wherever the module and core are composed. Both queue the notice with
`notifications.AudienceSender.SendTx` in the step's transaction, so it commits with the step; the delivery layer
(channel preferences, retries, the subject access check per user) does the rest after the commit. The nodes
resolve no audience and check no access themselves.

| Node | Params | Outcomes | Output |
|---|---|---|---|
| `notify` | `recipients {users (CEL list of user IDs), groups, roles, participants}`, `title`, `body`, `action_url` (templates), `channels` (empty: in-app), `level` info\|warning\|critical | `sent`, `no_recipients`, `failed` | `notice_id`, `recipients`, `skipped [{kind, ref, reason}]` |
| `send_to_contact` | `record` (CEL `{entity, key}`; default the run's subject), `channel` (an external channel), `subject`, `body` | `sent`, `no_contact`, `failed` | `notice_id`, `channel`, `skipped` |

- The `Producer` of a send is the run plus `node:token:seq` of the step, so applying the same step again
  returns the notice it queued (no second notice); a second visit of the node is a new notice. `Source` is the
  run's trigger event, used only to resolve `participants`.
- An audience that resolves to nobody queues nothing: `no_recipients`. `send_to_contact` checks the record's
  contact **before the step's transaction opens**: the node implements `engine.EffectPreparer`, so the runner
  rolls the step back at the effect, calls `PrepareEffect` with no transaction and no connection held, and
  takes the step again with the result in `StepContext.Prepared`. The preparation resolves the audience with
  `AudienceRequest.CheckContacts` (the channel first, then the entity's contact resolver for an address on it;
  a record without one is left out with the reason `no_address`); the node then takes `no_contact`, and a dry
  run agrees. The send itself stays atomic with the step (`SendTx` with no second check), and a resolver that
  takes its own connection, such as a package's (`notifications.ContactChecker`, which runs the package's query
  without an invocation record, `host.RunRequest.Unrecorded`), never waits for one the transaction holds, even
  on a pool without a spare connection. The step taken again applies the effects it was prepared for, so an
  expression that renders differently the second time (on `now`) cannot send what was not checked; it does so
  only while the subject's variables the run-as may read are unchanged, so a right lost in between is honored
  by the fresh evaluation, and a step whose fresh evaluation fails takes that failure. A preparation that no
  longer fits (the variables changed) is not handed to the node: the step is prepared again, at most three times;
  if what it reads keeps changing, the step is not applied but retried later (a platform error), so a node that
  asked for a check never acts without it. A node applied without the runner's preparation (a test of the node)
  has the delivery layer check the contact in the transaction instead. A resolver that is neither a
  `ContactChecker` nor a `TxSafeContactResolver`, or one that cannot be asked yet (`ErrContactUnverifiable`),
  leaves its record unchecked: the worker records a missing address at delivery. A channel that is not
  available is `failed` / `unavailable` whether or not the record has an address.
- `participants` need every trigger event that can reach the node to have a notification definition with those
  recipient keys; otherwise the graph is not saved (`params` error on `recipients.participants`). A run without
  an event (manual, schedule) fails with `no_trigger_event`.
- `action_url` is a path of the application, or a URL on the application's host (`Config.BaseURL`), which is
  stored as its path; anything else is refused when the graph is saved (a link that starts with a template is
  checked when it is rendered, `invalid_input`). `critical` is the SDK's `error` level. A title is at most 300
  bytes and a text 10000 bytes, as the inbox counts them (a Cyrillic character is two).
- A `FailedPrecondition` of the delivery layer (platform not configured, channel not available) is the failed
  outcome with code `unavailable`; any other error carries its code (`invalid`, `conflict`, ...).
- `skipped` comes with the receipt of the send (`AudienceReceipt.Skipped`): the resolution that queued the notice
  names who it left out, so the step reads nothing more while it holds its transaction.
- A dry run shows `AudienceSender.Resolve`: `recipients`, `skipped` and `resolved {users, records}` in the trace
  (for `send_to_contact` also the records and the skip reasons when the record has no address, never an address);
  nothing is queued.
- The catalog view gives each event `participants` (the recipient keys of its notification definition, filled
  through `catalog.WithParticipants`), and the notify params schema marks the field `x-event-participants`: the
  editor shows `recipients.participants` only where every trigger event that reaches the node offers some (a
  field that already holds a value stays visible so it can be cleared).

### The module wires the engine and serves its pages (#1339)

- **Wiring.** `modules/automations` builds `engine.Runtime` whenever the composition has the extension
  core (`wiring.NewComponent`: the workflow engine and the subject loader) and contributes
  `Runtime.Natives()`, `Runtime.Consumer()` and `Runtime.Schedules()`; the API and worker processes run
  `Runtime.Serve` and the inbound poller as a hook (`automations.WithoutBackground()` keeps them out of
  a process). The module's catalog registry carries `engine.Triggers()`, the application's native
  events (an event whose payload the catalog cannot express is left out with a warning) and the
  built-in and flow nodes. `automations.WithEngineConfig` sets limits, a subject loader or a task
  bridge; `automations.WithoutEngine()` turns the engine and the pages off. The connection store's
  usage index is `Runtime.UsageIndex()` unless the application gives one, so «Integrations» refuses
  to delete a connection a graph uses (`CONNECTION_IN_USE` with the graph).
- **Pages.** `/automations` (list, `Automations.Read`), `/automations/new` (`Automations.Author`),
  `/automations/{id}` and `/automations/{id}/runs/{run}` are one Solid screen
  (`modules/automations/presentation/graphs/web`) with its own embedded bundle
  (`/assets/automation-graphs/<release>/…`); the editor of `@iota-uz/sdk/automation-editor` is a lazy
  chunk of the graph page. Its RPC (`POST /automations/rpc`, contract in the issue) is a thin layer
  over `engine.Service`, `engine.Authoring` and `engine.Operations`: the tenant, the user and the
  origin come from the session. An interactive session is `human`; an agent session
  (`agentsession.Audience`), a request with an `Authorization` header or one without a session is
  `agent`, which the engine refuses to enable (`HUMAN_REQUIRED`).
- **Review before enabling.** `automations.enable_review` returns the Code nodes (language, source,
  secret names, fetch), the triggers, `run_as` and the code digest; `automations.enable` takes the
  digest the person saw and enables only the latest version: a version saved after the review is
  `review_required` and the dialog reloads with the new code.
- **Layout revision.** `automation.automations.layout_revision` counts layout changes.
  `Service.SaveLayoutAt(base)` (`automations.save_layout {id, base_revision, positions}`) refuses a stale
  base with `*LayoutConflictError` (`VERSION_CONFLICT` with the revision and positions of now): the
  editor keeps its positions unsaved and offers «Refresh» (`EditorProps.remoteLayout`).
- **Webhooks.** `engine.InboundHandler` is mounted at `/automations/hooks/{connection_id}` and
  `/automations/hooks/platform/{kind}` with no session or tenant middleware; each request is
  authenticated by the connector's `Verify`. The controller marks its route with
  `middleware.CSRFExemptRoute`: a CSRF middleware of the SDK (`middleware.CSRF`, `CSRFFromConfig`)
  installed with `Router.Use` on the router the route is registered on lets it through, and only it —
  the same path on another router, other routes and requests that match no route stay protected. An
  application adds no exemption of its own; one that wraps the whole server in a CSRF handler outside
  the router (where no route is matched yet) has to leave `/automations/hooks/` out of that wrapper.
- **Lists.** `Service.Overviews` gives the list its latest version's author and triggers, the last run
  and the count of runs that need attention (failed, blocked, `filter_failed`); `ListFilter.Attention`
  keeps graphs that have some; `Service.CountLive` names what disabling or deleting cancels.

### Backtests: datasets, history import, labels and the report (#1389)

`pkg/automation/backtest` runs a saved version of a graph over past and synthetic events without sending
anything out and reports what the graph would have done for each of them.

- **Datasets** (`automation.datasets`, at most 20 per automation, 5 000 events each) belong to an
  automation. An example (`automation.dataset_examples`) is a copy of an event (`{kind, ...payload}`), so
  pruning the outbox keeps it. Sources: the SDK's journal (`extruntime.outbox_events`) over a window,
  for the events of the latest version's triggers by default; the history of a connection whose
  connector implements `backtest.HistoryImporter` (the email connector reads its IMAP mailbox read-only:
  nothing is marked read or moved); a module's `backtest.HistorySource` (`automations.WithHistorySources`);
  synthetic events an author writes. A payload keeps what the importing author's catalog view declares:
  sensitive fields the author may not read are not copied. An event already in the dataset (its outbox
  id, a Message-ID) is a duplicate; past the limit the import counts what it left out (`overflow`).
  Importing a connection's history needs the right to the connection (`backtest.DefaultConnectionAccess`:
  `Connections.Manage` or its creator).
- **Snapshot at the event.** An example keeps its subject's state: `SnapshotLoader.SnapshotAt` when the
  engine's subject loader implements it, else the record as read at import, marked
  `snapshot_approximate`, with the permissions of the author it was read as (`snapshot_read_with`). A
  backtest runs with that state (`DryRunInput.Vars`) and the event's time as `now` (`DryRunInput.Now`).
  The state is the runner's when they are its author and still hold those permissions, whatever the
  record became since (changed values, a deleted record); anyone else gets it when the record as the
  loader gives it to them now holds each of its fields and list items. Otherwise the example runs
  without it: the result says `snapshot_withheld` and the report counts them (`snapshots_withheld`). The automation's memory between runs is read as it is now (`DryRunInput.AutomationID`); a backtest never writes it.
- **Run.** `Service.Run` queues a backtest; the worker (`Svc.Serve`, run by the module with the engine)
  claims it with a lease and resumes after the examples it has results for. Each example is
  `engine.Authoring.DryRun` with `RealCalls: decisions`: effects are captured with their params, execute
  calls other than decisions of models are answered from the context. `cached` (default) takes a
  decision from the decision cache and asks the provider once (paid) for one it lacks, so a person's
  cached backtest also shows, before it runs, its bound in calls and their estimated cost; an agent's cached
  backtest reads the cache only (`RealCallsNone`: a missing decision is reported `missing`, never a
  mismatch); `live` asks every decision again and is a person's (`HUMAN_REQUIRED`). `Service.Estimate`
  shows before the run how many calls it may make (a bound) and their estimated cost from earlier backtests
  (an average, not a bound). The estimate names the dataset's examples and the version whose decision nodes it
  counted (`Estimate.Revision`, never empty); a run given that revision is refused when either changed since
  (`ErrStaleEstimate`), so it never runs more than the estimate counted. A live run requires it.
- **Fencing paid calls (#1464).** A worker that stalls past `LeaseTTL` may still be inside a provider
  call when another worker takes the backtest. Each example's dry run carries an `engine.CallFence`:
  every real call gets the idempotency key `backtest:<backtest>:<example>:<node>:<n>:<digest>` (the n-th
  call of the node in that example and a digest of the request it renders, params and step context; it
  never names the lease, so the next worker repeats the same request under the same key, while a request
  rendered differently, say after the automation's memory changed, gets another key and never the first
  answer), and the lease is checked right before each call (`CallFence.Hold`), so a superseded worker
  makes no call after the one in flight and keeps no result. A connector whose provider answers a repeated
  key with the first answer and does not charge again declares `catalog.IdempotencyKeyed`
  (`SupportsIdempotencyKey`): its repeats are paid once. **Residual risk:** the next worker replays an
  unfinished example from its trigger, so a provider that cannot deduplicate (today every decision
  connector: OpenAI-compatible chat and JEV take no idempotency key) is paid again for every call the
  superseded worker made in it: the completed ones and the one in flight when the lease passed. That is
  at most one example's calls per unfinished example (a live backtest runs one example at a time; a
  person's cached one up to `Parallel`). `Hold` keeps the superseded worker from making the calls after
  those. `backtest.Config.StrictFencing`
  refuses such a backtest instead (`engine.ErrUnfenced`, at `Run` and again before each call), at the price
  of running no paid backtest over those connectors.
- **Reads** apply the reader's current rights, the importer's included: an example from a connection or a
  module source the reader may not read keeps its kind only (no title, subject, effects or trace); any
  other event is projected on the reader's catalog view, and when that drops a field the effects' params
  and the trace (rendered from the whole event) are not shown; the subject's snapshot is never returned.
  A live backtest runs one example at a time.
- **Labels** (`automation.dataset_labels`): `expected` comes with a synthetic event, `manual` is a
  person's and wins. Labels belong to the dataset and apply to every backtest over it, earlier ones
  included (the report is computed when it is read).
- **Report.** Precision and recall per classifier (every decision node and every labeled node) and
  outcome, the mismatches with the labels, the effects the graph would have made, the cost of the
  decisions (live and cached, from the `usage` the AI nodes report) and `Service.Compare` of two
  backtests over their shared examples.
- **UI and MCP.** The section «Backtests» is `/automations/{id}/backtests` (`Automations.Author`) of the
  automations screen; the editor only links to it («Backtests: N» in the host's actions). MCP:
  `mcpops.WithBacktests(svc)` adds `automation_backtests`, `automation_backtest_dataset_create`,
  `automation_backtest_examples_add`, `automation_backtest_label`, `automation_backtest_run` (cached only)
  and `automation_backtest_report`; `automations.Backtests(container)` gives the composed service.

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

MCP operations for automations (#1340, `pkg/automation/mcpops`) live in the shared registry
of `pkg/mcp` (#1084) and are reached through `lookup` / `execute` (#1314). Every call acts as
`Actor{Origin: agent}` for the MCP principal's user; access is the grant's scopes
(`automations:read`, `automations:author`) intersected with `Automations.Read` /
`Automations.Author`, checked on every call. An author reads the graphs it edits
(`Service.Get`/`GetVersion` admit `Automations.Author`).

- Read-only tools: `automation_catalog` (projected for the principal), `automation_list`,
  `automation_get` (with `revision` and `layout_revision`), `automation_runs`,
  `automation_run_get` (steps derived from fields the principal may not read are hidden),
  `automation_validate`, `automation_dry_run`. A dry run takes ≤ 50 events (the entries of a
  batch counted one by one) and answers calls from context: `decisions` is `scripted` (default),
  `cached` (decision nodes from the decision cache, else context) or `live`. Real calls happen
  only with `allow_live: true`, only for decision nodes, at most 20; beyond either limit the
  answer is `limit_exceeded`. Effect previews are offline (`StepContext.Offline`): only an
  `engine.OfflinePreviewer` previews, without package code (`send_to_contact` does not check the
  record's contact, `contact_checked: false`); any other effect is captured from context. Test node
  is not offered: it makes a real external effect.
- Writes are `changes.Kind`s of entity `automation` behind `prepare_changes` / `apply_changes`:
  `automation.create` (`{graph, layout?}`), `automation.update` (`{graph, layout?, layout_revision?,
  note?}`, `expected_revision` is the latest version; a stale one is the shared `revision_mismatch`;
  `layout_revision` is required with `layout`; a stale one is `VERSION_CONFLICT` on
  `/fields/layout_revision` in plan and at apply too: a layout another write moved after the plan
  makes the apply fail with `conflict` (`SaveInput.StrictLayout`, checked under the graph's lock) and
  nothing of the plan is saved — no version, no positions. The editor saves without `StrictLayout`:
  there a stale base keeps the positions another tab moved, #1339) and `automation.layout`
  (`fields: {id, base_revision, positions}`, the base is `layout_revision` — `0` for a new graph; a
  stale one is `VERSION_CONFLICT` with the current revision, in plan and at apply through
  `SaveLayoutAt`). Revisions are accepted as JSON integers or the strings `automation_get` returns. Plan validates
  the graph as the principal (issues under `/fields/graph/...`). Fields naming what only a person
  does — `enabled`, `enabled_version`, `decision`, `decisions`, `task`, `connection`,
  `connections` — are issues `HUMAN_REQUIRED` with the page where the person does it, and the plan
  is invalid; any other unknown field is `invalid_input`. No change needs approval: a saved
  version never runs until a person enables it. Apply works in the transaction of the operation
  (`engine.WithTx`, reads included), so a retried `apply_changes` never saves a graph twice and an
  apply needs one connection; the receipt item's `result` is
  `{id, version, enabled: false | <enabled_version>, editor_url}`.
- Audit: a plan's `HUMAN_REQUIRED` refusals are entries of the MCP engine's audit (`mcp.Audit`).
  Every applied change is an audit event (tool `changes:automation.<op>`, `OperationID`, `Item`)
  stored in `mcp.operation_audit` in the transaction that commits the receipt, naming the
  principal and client that applied it, and delivered after it; one left undelivered (lost commit response, stopped process) is delivered by the next call
  about the operation or by `DispatchEffects` (`changes.Config.Auditor`).
- A receipt item's `result` is shown on a retried `apply_changes` and on `operation_get` only
  while the caller still passes the kind's policy; otherwise only status, ids and revisions.
- Composition: `pkg/mcp/mcpwiring.NewComponent` is the application's MCP composition. It is on
  when the application passes its `Registry` (and then requires `Authorizer`, an
  `mcp.ConfirmingAuditor` as `Auditor`, and `ApprovalURL`; no environment flag). At start it
  registers the tools modules contribute (`mcpwiring.ContributeTools`) and the changes tools
  over the contributed kinds (`mcpwiring.ContributeKinds`) with `changes.Config.Auditor` set, and,
  in a process with the API or worker capability, calls `DispatchEffects` every
  `DispatchInterval` (5s). The automations module contributes the `mcpops` tools and kinds with
  its pages under `http.origin` as URLs, and its engine runs with `Config.Links` =
  `engine.NewLinks(origin, "<automations path>/%s/runs/%s")` (`WithEngineConfig` may replace it).
  The gateway is built over the registry after `composition.Start`.
- `HUMAN_REQUIRED` is the shared failure `approval_required` / `open_approval` with an issue of
  code `HUMAN_REQUIRED` and the URL of the page where the person acts. The engine marks
  `automation.run` and `automation.call` human-only (`workflow.NativeContract.HumanOnly`), so the
  extension runtime's workflow actions (signal, decide task, terminate, start v2, resolve, retry,
  resume) on their instances and tasks answer it with the run's page (`Config.Links.RunURL`)
  wherever the workflow tools are registered, the standard wiring included. Such an instance or task
  never reaches the workflow service from these tools: a caller who passes the action's own check
  (`Extensions.Operate`, or the right to decide the task) gets `HUMAN_REQUIRED` with the page, anyone
  else that check's refusal without it. Reads stay (`ext_workflow_list`, `ext_workflow_get`),
  reduced to an allowlist of fields with no business data (no input, results, errors, signal
  payloads, task titles, decisions or delegation comments): `automation_run_get` shows the rest
  projected for the viewer.

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
