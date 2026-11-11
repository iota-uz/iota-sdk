# Writing extension packages

An **extension package** adds data, behaviour, workflows, notifications and UI to an iota-sdk application without
touching its Go code. You write TypeScript against `@iota-uz/sdk`, test it on your machine, and publish it through the
application's configurator UI or MCP tools. This guide is the author's manual; the implementation contract between the
platform's own slices is [`../extension-runtime.md`](../extension-runtime.md) (read it only if you build the platform).
Enabling the runtime in an application and operating it is [`consumer.md`](consumer.md).

Everything here compiles and passes: the complete example package is [`example/`](example) (`acme.loyalty`, a loyalty
programme for CRM clients), and the code blocks below that start with `// example/<file>` are quoted from it. CI
typechecks, builds and tests the example with the same CLI you use.

## 1. Concepts in five minutes

| Term | Meaning |
|---|---|
| **package** | a directory with a `package.json` that has an `iota` section, and TypeScript sources |
| **namespace** | the dotted owner prefix of every ID the package declares: `acme.loyalty`. Two or more lowercase segments; the first must not be a platform namespace (`core`, `crm`, …) |
| **resource** | anything the package declares: entity, overlay, command, query, operation, event, notice, workflow, permission, error, route, page, column… Its ID is `<namespace>.<name>` |
| **contract** | the *description* of a resource (`command("acme.loyalty.redeem", { input, output })`). Pure data: no handler |
| **implementation** | a handler bound to a contract by typed reference: `implement(Redeem, async (ctx, input) => …)` |
| **kit** | generated `@iota-kit/*` modules: typed references to what the platform and other packages publish (`@iota-kit/core`, `@iota-kit/crm`, …). References only, never handlers |
| **version** | an immutable build of the package. One version is *active* for the whole application |
| **binding** | which implementation serves a *replaceable* contract (a native command, a screen, the shell). One active binding per contract |

A package has up to three **graphs**, each an ES module entry declared in `package.json`:

| graph | file | may import | runs |
|---|---|---|---|
| contracts | `src/contracts.ts` | `@iota-uz/sdk/runtime`, `@iota-kit/*`, pure files, pure npm packages | everywhere (it is pure data) |
| server | `src/server.ts` | everything above, `@iota-uz/sdk/runtime/server`, Node built-ins, npm | in the platform's Node worker pool |
| client | `src/client.tsx` | contracts, `@iota-uz/sdk/runtime/client`, `solid-js`, browser-safe npm | in the browser (Solid) |

The registry of a package is the set of **named exports** of the three entry modules that carry the SDK brand. There is
no `register()` call and no manifest to write: export a contract, a page or an implementation and it exists. A violated
graph rule (the client importing the server, a contract importing Node) is a build error with a file and line.

Two things surprise newcomers:

* **Data lives in Go.** Your handlers never open a database connection. They receive a *transaction handle* (`tx`) or a
  read-only context and ask the platform to read and write. A transaction is opened and committed by Go; your Node code
  only asks for it.
* **Handlers replay or restart.** Workflow code is replayed from a journal; an ordinary command may be retried by an
  operator. Write them so that effects are named (`wf.step("voucher", …)`) and idempotent where they leave the system.

## 2. Quick start

```sh
npm install --save-dev @iota-uz/sdk            # provides the iota-ext command; esbuild comes with it
npx iota-ext init loyalty --namespace acme.loyalty
cd loyalty
npm install                                    # editor support and the lockfile publishing needs; the kit is not in node_modules
npx iota-ext check                             # typecheck + graph rules + manifest
npx iota-ext test                              # run *.test.ts on the in-memory test host
npx iota-ext build                             # artifacts and manifest in .iota/build
```

`init` writes `package.json`, `tsconfig.json`, `src/contracts.ts`, `src/server.ts`, `src/client.tsx` (a page),
`src/server.test.ts` (a passing test) and generates the kit for the SDK's own natives (`@iota-kit/core`,
`@iota-kit/crm`) **offline** into `.iota/kit/`. Everything works before you have access to any server. When you do,
`iota-ext kit pull` replaces the offline kit with the application's real one (see §5).

`iota-ext` is the `bin` of `@iota-uz/sdk` (Node ≥ 20.19). Full reference: §16.

`init` pins `"@iota-uz/sdk"` in `package.json` to the **exact version of the CLI that ran it** (`iota-ext --version`), not a
range. The extension runtime (`iota-ext`, `@iota-uz/sdk/runtime/*`) ships from that release on; earlier public builds of the
package have no `dist/runtime`, and a caret range could resolve to one of them. Keep the dependency on a release that
contains the runtime, and when you upgrade the SDK, upgrade it together with a fresh `kit generate`/`kit pull`. If you
install the SDK from a tarball or a registry mirror, the dependency points there instead; the rule is the same.

## 3. Package layout

```text
loyalty/
├─ package.json          the iota section: namespace, entries, capabilities
├─ package-lock.json     required to publish when the package has any dependency besides @iota-uz/sdk
├─ tsconfig.json         for your editor (see §4)
├─ .iota/kit/            the generated developer kit (§5): commit it
├─ .iota/build/          `iota-ext build` output: do not commit
└─ src/
   ├─ contracts.ts       contracts graph
   ├─ server.ts          server graph
   ├─ client.tsx         client graph (pages, navigation, cells)
   ├─ *.tsx              screens, loaded lazily from client.tsx
   └─ *.test.ts          tests (run by `iota-ext test`; never part of an artifact)
```

```json
{
  "name": "@acme/loyalty",
  "type": "module",
  "iota": {
    "namespace": "acme.loyalty",
    "contracts": "./src/contracts.ts",
    "server": "./src/server.ts",
    "client": "./src/client.tsx",
    "capabilities": { "network": ["api.partner.example"], "secrets": ["PARTNER_TOKEN"] }
  },
  "dependencies": { "@iota-uz/sdk": "0.5.6" }
}
```

* Every entry is optional, but at least one is required. Paths must stay inside the package.
* `capabilities.network` lists the hosts server code may reach with `fetch`; `capabilities.secrets` lists the secret
  names `ctx.secrets.get("NAME")` may read. Anything undeclared is refused. Secrets are never in the source or the
  artifact: an administrator provides their values to the application.
* Any other dependency (npm packages for the server or client graph) needs a committed `package-lock.json`; the platform
  installs with `npm ci --ignore-scripts`. `@iota-uz/sdk` itself is provided by the platform and never installed from
  the registry.
* Kit modules and npm dependencies are **bundled** into the artifacts; only `@iota-uz/sdk/runtime*` stays external.
  Client chunks do not carry Solid either: the host shares one Solid with every package.

## 4. TypeScript configuration

The platform compiles with fixed options so that `iota-ext check`, the configurator and the server always agree:
`strict`, target ES2022, `module: ESNext`, `moduleResolution: Bundler`, `jsx: preserve` with
`jsxImportSource: "solid-js"`, libs `ES2022` + `DOM`, Node typings (`@types/node`), `skipLibCheck`. Your `tsconfig.json`
only serves the editor and should mirror those options. The scaffold writes:

```json
{
  "compilerOptions": {
    "target": "ES2022", "module": "ESNext", "moduleResolution": "Bundler",
    "strict": true, "noEmit": true, "skipLibCheck": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "types": ["node"],
    "paths": { "@iota-kit/*": ["./.iota/kit/*"] },
    "jsx": "preserve", "jsxImportSource": "solid-js"
  },
  "include": ["src"]
}
```

* Install `typescript`, `@types/node` and (for a client graph) `solid-js` as dev dependencies; `npm install` after
  `init` does it. The SDK's own typings come from `@iota-uz/sdk`.
* `types: ["node"]` is what lets tests import `node:assert`; you do not need it if you use the SDK's `assert` (§14).
* Import local files without extensions (`import * as c from "./contracts"`); the bundler resolves them.
* `paths` is the one alias you need: it lets the editor (and your own `tsc`) resolve `@iota-kit/*` from `.iota/kit/`, where
  the kit lives (§5). `iota-ext` does not read it: it resolves the kit itself. `iota-ext kit generate` adds the entry to a
  `tsconfig.json` written before the kit moved out of `node_modules` (a tsconfig with comments is left alone and the
  command prints the line to add).
* Do not rely on other options the platform does not use (other path aliases, `experimentalDecorators`, JSX other than Solid).

## 5. The developer kit

Contracts you did not declare (users, CRM clients, other packages' commands) are referenced through **kit modules**:

```ts
import * as core from "@iota-kit/core"   // core.User, core.Created (the user-created event)
import * as crm from "@iota-kit/crm"     // crm.Client, crm.ListsClients (the native list), crm.Created, crm.Updated, crm.SaveClient, crm.UpsertClient
```

A kit module is generated from a **catalog snapshot**: native entities (with fields and key types), native lists, native
events and the Go-implemented commands, operations and queries of the application, plus the public contracts of the
packages that are active. You get one of three ways:

| how | command | when |
|---|---|---|
| the SDK's own natives, offline | `iota-ext kit generate` (also done by `init`) | no server yet; everything the SDK itself publishes |
| an application's snapshot file | `iota-ext kit generate --catalog app.json` | the application exported its native catalog; works offline |
| the live kit | `iota-ext kit pull --server https://erp.example.com --token-env IOTA_TOKEN` | you can reach the application's `/_ext/manage` API |

`kit generate` and `kit pull` write the modules to **`.iota/kit/<ns>/`** (`index.js`, `index.d.ts`, `package.json`) and
the snapshot in `.iota/kit/kit.json`: its hash and the SHA-256 of every file. The kit is deliberately **not** in
`node_modules`: a package manager deletes what it did not install, so a generated `node_modules/@iota-kit` vanished on
every `npm install`. `.iota/kit` is yours to commit (it pins the snapshot the package was written against and needs no
server to restore); `iota-ext` and its build worker resolve `@iota-kit/*` from it (the application builds against its own
kit), your editor through the `paths` entry of `tsconfig.json` (§4). A package that still has the old `node_modules/@iota-kit` layout keeps
working (the CLI says so); the next `kit generate` or `kit pull` moves it. `iota-ext kit status` verifies the kit
against its record: each module is `ok`, `missing` (a module or file is gone), `modified` (a file was edited) or
`unrecorded`, and the exit code is 1 when the kit is damaged. The token is read
from the environment variable you name (never from the command line); `--cookie-env VAR` sends a session cookie header
instead. The user needs the extensions *read* permission. An application exports a snapshot with
`GET /_ext/manage/catalog/natives` or `catalog.Natives(set).Snapshot()` over its composed `native.Set`; the SDK's own
(the set of `modules.Natives()`) is committed as
`web/runtime/catalog/sdk-natives.json` (regenerate it with `go run ./cmd/extruntime-catalog`) and ships in the
distribution. Rerun the command whenever the application's natives or other packages change; the build pins the kit hash
in the manifest.

What a module exports:

* **Native entities** (`core.User`, `crm.Client`) with their real fields and **key type** (`int64` for both; a ref's key
  is a decimal string such as `"42"`). You can read them, reference them in fields, put an overlay on them, but not
  write them with `tx.insert/update/delete` (§6).
* **Native lists** (`crm.ListsClients`) that your `column(...)` contracts extend.
* **Native routes** (`crm.Clients`, the CRM clients page) you link to or replace with a screen binding.
* **Events** (`core.Created`, `crm.Created`, `crm.Updated`), to subscribe to. Native events are published by Go in
  the transaction of the change they report, so a subscription sees them only after that change committed and never
  for a change that rolled back.
* **Native commands, operations and queries**, implemented in Go (`crm.SaveClient`, `crm.UpsertClient`). You call them,
  guard them and — for commands — replace them (§7.4).
* The public contracts of other active packages.

Export names are the PascalCase of the last segment of the ID, a version suffix dropped; when two IDs would collide
they take more segments (`crm.clients` the route and `crm.lists.clients` the list become `Clients` and
`ListsClients`). The offline SDK kit and an application built on the SDK modules are generated from the same module
registrations, so they agree on these names; an application that adds natives of its own can add names (and push an
existing one to more segments when it collides), so pull the kit of the application you target.

If an import fails, the module is not in the kit: the diagnostic is `graph.unknownKit` (once per file and module, instead of
TS2307) and names the commands that install it (`kit generate`, `kit generate --catalog <file>`, `kit pull`). The follow-on
type errors it causes (implicit `any`, missing properties on values typed through the missing module) are hidden, with one
`kit.followOn` warning saying how many, so the first error you see is the cause. When the kit's files are gone or edited, the
hint says so and points at `kit status`.

## 6. Data

### 6.1 Entities

```ts
// example/src/contracts.ts
export const Reward = entity("acme.loyalty.reward", {
  name: field.text({ maxLength: 80 }).unique(),
  cost: field.int(),
  active: field.bool().default(true),
})
```

Each entity is one tenant-scoped PostgreSQL table owned by your package. Every row also has `key` (a uuid), `ref` (the
typed pointer to itself: `row.ref`), `createdAt`, `updatedAt` and `rowVersion`. These names are reserved.

Field builders: `field.text({maxLength})` (`string` is an alias), `int`, `int64` (a decimal string on the wire),
`decimal({precision, scale})` (a string), `bool`, `date` (`"YYYY-MM-DD"`), `datetime` (RFC 3339), `uuid`,
`enum([...])`, `json<T>()` (JSON `T` only; not filterable), `ref(Entity)`. Modifiers:

* `.optional()` – nullable and may be omitted on insert: `T | null` on read, `T | null | undefined` on write.
* `.nullable()` – a nullable column that must be given.
* `.default(x)` – may be omitted on insert; the default is validated and recorded even when falsy.
* `.unique()`, `.index()`, `.label({...})`, `.renamedFrom("oldKey")` (the column follows the field across renames).

Use `t.*` (the same builders without column semantics) for inputs and outputs.

### 6.2 Reading and writing

```ts
// example/src/server.ts
export const catalog = implement(c.Catalog, async (q, input) => {
  const rewards = await q.find(c.Reward, {
    where: input.onlyActive === true ? { active: true } : {},
    orderBy: [{ field: "cost" }, { field: "name" }],
  })
  return rewards.map((r) => ({ reward: r.ref, name: r.name, cost: r.cost, active: r.active }))
})
```

`get(entity, ref)`, `find(entity, { where, orderBy, limit, offset })` (default limit 100, maximum 1000) and
`count(entity, where)` exist on every context. `tx.insert`, `tx.update(entity, ref, patch, { rowVersion })`,
`tx.delete` and `tx.put(overlay, baseRef, values)` exist on transactions only. Rules the types and the host enforce:

* **Filters** are a conjunction of conditions on fields, system columns (`createdAt`, `updatedAt`, `key`) or dotted paths
  through refs (`"holder.name"`, at most four segments). A bare value means equality. Operators: `eq ne in isNull`,
  ranges `lt lte gt gte` on `int int64 decimal date datetime string`, `like` on strings only (`%` and `_`), `or: [...]`.
  `null` follows SQL: `eq: null` is `IS NULL`, `ne: x` is `IS DISTINCT FROM`.
* **Ordering** works on `int int64 decimal date datetime string` fields and `createdAt`/`updatedAt` (also through
  refs). Nulls sort last ascending. Ordering by an enum, bool, uuid or ref field is a type error whose text says so.
* Read **access rules** and **hidden fields** (below) apply before filtering, counting and paging.
* `tx.update(..., { rowVersion })` fails with `data.versionConflict` unless the row is still at that version.
* `tx.insert/update/delete` take **your package's own** entities only. A native or foreign entity gives a type error
  that names the published operation to call instead (`tx.call(crm.UpsertClient, …)`), and a runtime `data.nativeWrite`
  / `data.foreignWrite` denial if you cast around it.

### 6.3 Access rules

Rules and gates refer to **permissions**, which you declare in the contracts graph. A permission is a named capability with a
label for the role editor; the ID lives in your namespace, and the label is a map of locale to text:

```ts
// example/src/contracts.ts
export const Manage = permission("acme.loyalty.manage", { label: { en: "Manage loyalty", ru: "Управление лояльностью" } })
```

```ts
// example/src/contracts.ts
export const Redemption = entity(
  "acme.loyalty.redemption",
  {
    client: field.ref(crm.Client).index(),
    reward: field.ref(Reward),
    cost: field.int(),
    status: field.enum(["pending", "approved", "rejected"]).default("pending"),
    requestedBy: field.ref(core.User),
  },
  // Managers see every redemption, everybody else the ones they requested.
  { access: { read: [{ when: Manage, rows: "all" }, { rows: { requestedBy: $.user } }] } },
)
```

`access.read` is a list of rules; a row is visible when any rule whose permission the user holds matches. `rows` is
`"all"`, `"none"` (an explicit deny; an empty list is an error) or a filter. `$.user` stands for the acting user and
compares only with refs to `core.user`. `access.fields` lists the permissions needed to read a field; a user who cannot
read a field cannot write it either. These rules hold on **every surface**: handlers, the native lists, exports, MCP.

> **Contract permission vs read rules.** A contract's `permission` (on a command, query or operation) is a **gate on the
> call**: the host checks that the acting user holds it before dispatching, from the browser (`POST /_ext/rpc`), MCP,
> workflows and triggers alike. Read rules are **row filters inside the data layer**: they decide which rows a read
> returns, whoever calls and through whichever contract. Use the first to say who may *do* something, the second to say
> who may *see* which records. A page's `permission` only controls whether the page and its menu entry are offered.

### 6.4 Overlays and list columns

```ts
// example/src/contracts.ts
export const ClientLoyalty = overlay(
  "acme.loyalty.clientLoyalty",
  crm.Client,
  {
    tier: field.enum(["standard", "silver", "gold"]).default("standard"),
    points: field.int().default(0),
    note: field.text().optional(),
  },
  { access: { fields: { note: [Manage] } } },
)
```

An overlay is a separate 1:1 table keyed by the base record's key; it is how you add fields to a native entity. Every
overlay field must be optional, nullable or defaulted, because most clients have no overlay row: `tx.get(overlay, ref)`
returns `null` for them (absence is explicit in the type) and the defaults apply to filters and lists. Write with
`tx.put(overlay, baseRef, { points })`; omitted fields keep their value.

```ts
// example/src/contracts.ts
export const TierColumn = column(crm.ListsClients, {
  id: "acme.loyalty.tier",
  field: ClientLoyalty.fields.tier,
  label: { en: "Loyalty tier", ru: "Уровень" },
  filterable: true,
  sortable: true,
  exportable: true,
})
```

`column` adds an overlay field to a native list: display, filter, sort and CSV/Excel export. **Its `id` must be the full
namespaced ID** (`acme.loyalty.tier`, not `tier`); a short one fails the manifest check. A custom renderer for the cell
is a client-graph value (§13.1).

## 7. Contracts and handlers

### 7.1 The four callable kinds

| kind | context | transaction | callable from |
|---|---|---|---|
| `query` | `QueryContext` (read-only) | a read snapshot | browser, MCP, other handlers (`q.query`, `tx.query`, `ctx.query`) |
| `command` (ordinary) | `CommandContext` | opens its own with `ctx.transaction(cb)` (zero or more times) | browser, MCP, workflows (`wf.call`), `ctx.call` |
| `command` with `atomic: true` | `Tx` | the whole handler is one transaction | as above, and `tx.call` |
| `operation` | `Tx` | joins the caller's | `tx.call`, `wf.transaction(name, Op, input)`; **never** from the browser |

An **operation** is the reusable, atomic domain primitive: other packages and your own commands call it, and guards
attach to it. Atomicity (`atomic: true`) is part of the public contract: a replacement implementation of an atomic
command also receives a `Tx`.

```ts
// example/src/contracts.ts
export const AdjustPoints = operation("acme.loyalty.adjustPoints", {
  input: t.object({ client: t.ref(crm.Client), delta: t.int() }),
  output: t.object({ points: t.int() }),
  errors: [NotEnoughPoints],
})
```

Inputs and outputs are validated against their schemas by Go on every call, independently of TypeScript. `errors` is the
closed list of domain errors the contract may raise.

### 7.2 Handlers

```ts
// example/src/server.ts
export const adjustPoints = implement(c.AdjustPoints, async (tx, input) => {
  const current = (await tx.get(c.ClientLoyalty, input.client))?.points ?? 0
  const next = current + input.delta
  if (next < 0) throw c.NotEnoughPoints.of({ have: current, need: -input.delta })
  const row = await tx.put(c.ClientLoyalty, input.client, { points: next })
  await tx.emit(c.PointsChanged, { subject: input.client, payload: { points: next, delta: input.delta } })
  return { points: row.points }
})
```

A handler may return synchronously; `async` is the norm. Each context also has `user` (the acting principal and
`user.has(permission)`), `invocation`, and `log`. Command contexts have `secrets`.

### 7.3 Transaction semantics

* **A transaction is Go's.** `ctx.transaction(cb)` asks Go to open one, runs `cb(tx)`, then commits (or rolls back when
  `cb` throws). It cannot be nested. Reads and writes inside see the transaction's own writes.
* **An atomic command or operation is one transaction** from the first line to the return.
* **`tx.call(Operation | AtomicCommand, input)` joins the caller's transaction**: one commit, one rollback. Ordinary
  commands and queries are rejected at the type level with a message that names the right call
  (`ctx.call`, `tx.query`).
* **`tx.emit`, `tx.notify`, `tx.start`, `tx.signal` are enqueued with the data.** They take effect when the transaction
  commits and vanish on rollback. After commit, each active subscription gets its own transaction; notices are rendered
  and sent after commit.
* **An ordinary command is not atomic.** Each `ctx.transaction` commits on its own, so a crash between two of them
  leaves the first committed; the command is never retried blindly. Put work that must be all-or-nothing in one
  transaction, or make the command atomic.
* **Outside calls belong between transactions.** Call `fetch`, read secrets and wait *outside* `ctx.transaction`, as
  `redeem` does below; do not hold a database transaction open across the network.
* **One thing at a time inside a transaction.** Concurrent calls on one `tx` (`Promise.all`) fail fast; `await` them.
* **`tx.start(Workflow, input, { key })`** needs a `key` outside subscriptions and workflows (the key dedupes the start:
  the same key never starts a second instance). Inside a subscription or a workflow the key is derived and optional.
  Omitting it where it is required is a compile error (`Expected 3 arguments`) and the CLI adds the hint
  `tx.start needs a start key`.
* **`ctx.query` reads a snapshot**; inside a transaction `tx.get/find/count/query` read that transaction's state.

```ts
// example/src/server.ts
export const redeem = implement(c.Redeem, async (ctx, input) => {
  const reward = await ctx.query.get(c.Reward, input.reward)
  if (reward === null || !reward.active) throw c.RewardUnavailable.of({ reward: input.reward.key })
  const stock = await fetch(`https://api.partner.example/stock/${encodeURIComponent(reward.name)}`, {
    headers: { authorization: `Bearer ${await ctx.secrets.get("PARTNER_TOKEN")}` },
  })
  if (!stock.ok) throw c.RewardUnavailable.of({ reward: input.reward.key })

  const redemption = await ctx.transaction(async (tx) => {
    await tx.call(c.AdjustPoints, { client: input.client, delta: -reward.cost })
    const row = await tx.insert(c.Redemption, {
      client: input.client, reward: input.reward, cost: reward.cost, requestedBy: ref(core.User, ctx.user.userId),
    })
    // Outside subscriptions and workflows a start needs a key: the same key never starts a second instance.
    await tx.start(c.Approval, { redemption: row.ref }, { key: `approval:${row.key}` })
    return row
  })
  return { redemption: redemption.ref }
})
```

### 7.4 Calling and replacing native commands

Go modules publish **native contracts**: operations, atomic commands, ordinary commands and queries implemented in Go and
visible in the kit. The CRM publishes two for its clients:

* `crm.UpsertClient` (operation): the invariant behind every client write: authorization, value validation
  (phone, e-mail, PIN, country), persistence, change history and the client events (`crm.Created` / `crm.Updated`,
  in the same transaction, §9).
* `crm.SaveClient` (ordinary command): the intent the client forms run; its Go default calls `UpsertClient` in one
  transaction.

You **call** a native operation like your own: `tx.call(crm.UpsertClient, { client, phone })`. A package that wants to
change *what saving a client means* **replaces the command**: it implements `crm.SaveClient` and writes through the
same operation, so the invariants and every guard on the operation still hold.

```ts
// example/src/server.ts
export const saveClient = implement(crm.SaveClient, async (ctx, input) =>
  ctx.transaction(async (tx) => {
    const saved = await tx.call(crm.UpsertClient, input)
    if (saved.created) await tx.call(c.AdjustPoints, { client: saved.client, delta: 50 })
    return saved
  }),
)
```

Publishing this does **not** change behaviour by itself: an implementation of a contract you do not own is a
*replacement candidate*. It runs when an administrator binds it (`ext_bind` over MCP, `POST /_ext/manage/bindings` with
`{"slotKind":"command","targetId":"crm.commands.saveClient","package":"acme.loyalty","version":"…","export":"saveClient"}`,
or the configurator's Bindings tab). One implementation is active per command for the whole application, there is no
fallback if it fails, and unbinding (`ext_unbind`) returns the Go default. Rules to remember:

* The replacement has the shape of the original: ordinary command → `CommandContext`, atomic → `Tx`.
* A generic write to `crm.client` is refused (`data.nativeWrite`); the operation is the only way in.
* A guard on `crm.UpsertClient` (yours, or another package's) applies to every caller, replacement or Go default alike.
* In tests the Go default does not exist: bind a double of the operation (§14.4).

## 8. Errors and validation

Handlers signal failure by **throwing**; the host turns it into a wire error `{kind, code, params, issues}`:

| you throw | kind | use for |
|---|---|---|
| `NotEnoughPoints.of({ have, need })` | `domain` | an expected business outcome a caller can handle; declared in the contract's `errors` (a closed list) |
| `validation(CostMustBePositive.at(["cost"], { min: 1 }))` | `validation` | field-level problems; any number of namespaced issues |
| *(host)* | `denied` | permission or access refusals (`data.denied`, `data.fieldNotAccessible`, `data.nativeWrite`) |
| any other `Error` | `internal` | bugs; the message is hidden from the caller |

```ts
// example/src/contracts.ts
export const NotEnoughPoints = domainError("acme.loyalty.notEnoughPoints", t.object({ have: t.int(), need: t.int() }))
export const RewardUnavailable = domainError("acme.loyalty.rewardUnavailable", t.object({ reward: t.string() }))
export const CostMustBePositive = issue("acme.loyalty.costMustBePositive", t.object({ min: t.int() }))
```

```ts
// example/src/server.ts
export const createReward = implement(c.CreateReward, async (tx, input) => {
  if (input.cost <= 0) throw validation(c.CostMustBePositive.at(["cost"], { min: 1 }))
  const row = await tx.insert(c.Reward, { name: input.name.trim(), cost: input.cost })
  return { reward: row.ref }
})
```

`validation()` accepts only `Issue.at(...)` results; `ContractError` has no public constructor. The host enforces the closed `errors` list: a domain error the contract does not declare is rejected. The data layer raises stable codes you can match in tests:
`data.notFound`, `data.versionConflict` (domain), `data.unique`, `data.reference` (validation, issue path = field),
`data.denied`, `data.fieldNotAccessible`, `data.nativeWrite`, `data.foreignWrite` (denied).

In the browser, `isError(err, NotEnoughPoints)` narrows a failed call to the declared error with typed `params`, and
`issuesOf(err, CostMustBePositive)` returns the typed issues (see `RewardsScreen.tsx`).

Guards are *additive* checks that run in the same transaction regardless of who implements the contract:

```ts
// example/src/server.ts
export const deltaLimit = guard("acme.loyalty.deltaLimit", c.AdjustPoints, async (_ctx, input) => {
  if (Math.abs(input.delta) > 10_000) throw validation(c.DeltaTooLarge.at(["delta"], { max: 10_000 }))
})
```

```ts
// example/src/server.ts
export const rewardRule = writeRule("acme.loyalty.rewardRule", c.Reward, async (_ctx, change) => {
  if (change.proposed !== null && change.proposed.cost > 1_000_000) throw validation(c.CostMustBePositive.at(["cost"], { min: 1 }))
})
```

* `guard(id, Operation | AtomicCommand, (ctx, input) => …)` checks an operation or atomic command call.
* `writeRule(id, Entity, (ctx, { op, existing, proposed, changed }) => …)` runs in the writing transaction before any
  write of that entity, whatever code path writes it.
* `precheck(id, OrdinaryCommand, (q, input) => …)` is a *preliminary* check on a read snapshot for an ordinary command:
  useful for early feedback, not a race-free guarantee.

All three get a **read-only** context (`get find count query user log`) and reject by throwing. Their IDs become the
`source` of the issues they raise. A guard on an operation applies to every caller, including other packages and Go.

## 9. Events and subscriptions

```ts
// example/src/contracts.ts
export const PointsChanged = event("acme.loyalty.pointsChanged.v1", {
  subject: crm.Client,
  payload: t.object({ points: t.int(), delta: t.int() }),
})
```

```ts
// example/src/server.ts
export const onPointsChanged = subscribe("acme.loyalty.onPointsChanged", c.PointsChanged, async (tx, event) => {
  const tier = event.payload.points >= 1000 ? "gold" : event.payload.points >= 300 ? "silver" : "standard"
  await tx.put(c.ClientLoyalty, event.subject, { tier })
})
```

* Event IDs end in a version (`.v1`); a published payload never changes, publish `.v2` instead.
* `tx.emit(Event, { subject, payload })` is atomic with the data. After the commit, every active subscription receives
  the event in **its own transaction**; a receipt per `(subscription, event)` makes redelivery a no-op, and one failing
  event is retried with back-off and finally dead-lettered without blocking the others.
* A subscription runs as the event's initiator, or — for events without a user — as the **owner-user** an administrator
  assigns to the trigger (`ext_owner_assign`). Without an owner the delivery is `blocked`, not lost.
* Subscribe to platform events through the kit (`subscribe("…", core.Created, handler)`). `tx.start` inside a
  subscription needs no key: the delivery derives one, so redelivery cannot start a second workflow.
* Platform events are published by Go through the same outbox, **in the transaction of the change**: `core.Created`
  (`core.user.created.v1`) when a user is created; `crm.Created` / `crm.Updated` (`crm.client.created.v1` /
  `crm.client.updated.v1`, subject `crm.client`) by `crm.UpsertClient`, so every client write that goes through it
  — the CRM forms, a `crm.SaveClient` replacement, your own `tx.call(crm.UpsertClient, …)` — publishes one. The
  payload carries the client's published fields (`clientId`, `firstName`, optional `lastName`, `middleName`,
  `phone`, `email`; never PIN or passport) and, for updates, `changed`: the `crm.client` field IDs the write changed.
  A save that rolls back publishes nothing.

```ts
export const onClientUpdated = subscribe("acme.loyalty.onClientUpdated", crm.Updated, async (tx, event) => {
  if (event.payload.changed.includes("phone")) tx.log.info("client phone changed", { client: event.payload.clientId })
})
```

## 10. Workflows

A workflow is a long-running function that survives restarts. The platform stores a **journal** of completed steps and
**replays** your function from the top, answering finished primitives from the journal; a primitive that cannot finish
yet (a sleep, a signal, a human task) *suspends* the run and frees the worker.

```ts
// example/src/contracts.ts
export const Approval = workflow("acme.loyalty.approval", {
  input: t.object({ redemption: t.ref(Redemption) }),
  output: t.object({ approved: t.bool(), voucher: t.string().nullable() }),
  signals: { withdraw: t.object({ reason: t.string() }) },
})
```

```ts
// example/src/server.ts
export const approval = implement(c.Approval, async (wf, input) => {
  const verdict = await wf.task("approve", {
    subject: input.redemption,
    assignee: wf.user,
    decision: t.object({ approved: t.bool() }),
    timeout: "3d",
  })
  const approved = verdict.outcome === "decided" && verdict.decision.approved

  // An external effect: may run again after a crash, so it gets a provider idempotency key.
  const voucher = approved
    ? await wf.step("voucher", async ({ idempotencyKey }) => {
        const res = await fetch("https://api.partner.example/vouchers", {
          method: "POST",
          headers: { "content-type": "application/json", "idempotency-key": idempotencyKey },
          body: JSON.stringify({ redemption: input.redemption.key }),
        })
        const body = (await res.json()) as { code: string }
        return { code: body.code }
      })
    : null

  // Data effects commit together with their journal entry. The result must be JSON.
  await wf.transaction("settle", async (tx) => {
    const row = await tx.get(c.Redemption, input.redemption)
    if (row === null) return { settled: false }
    const reward = await tx.get(c.Reward, row.reward)
    await tx.update(c.Redemption, row.ref, { status: approved ? "approved" : "rejected" })
    if (!approved) await tx.call(c.AdjustPoints, { client: row.client, delta: row.cost })
    await tx.notify(c.RedemptionResult, {
      subject: row.ref,
      payload: { reward: reward?.name ?? "", approved },
      to: [recipient.record(row.client)],
      channels: ["sms", "email"],
    })
    return { settled: true }
  })
  return { approved, voucher: voucher?.code ?? null }
})
```

| primitive | what it does |
|---|---|
| `wf.transaction(name, cb)` | a Go transaction; the data effects and the journal entry commit together, so a replay does not repeat them. Also `(name, { output: schema }, cb)` and `(name, Operation \| AtomicCommand, input)` |
| `wf.step(name, effect)` | a re-runnable external effect (HTTP, e-mail). **At-least-once**: it may run again after a crash, with the same `idempotencyKey` (`<instance>:<name>:<n>`) |
| `wf.sleep(name, "30d")` | durable timer (`ms s m h d`) |
| `wf.waitFor(name, Signal)` | wait for a signal sent with `tx.signal` / `ctx.signal` / an endpoint; with `{ timeout }` it returns `{ outcome: "received" \| "timeout" }` |
| `wf.task(name, { subject, assignee, decision, timeout })` | a human task; completes with `{ outcome: "decided", decision, by }` or `"timeout"` |
| `wf.call(name, Command, input)` | a new command invocation that uses the *currently active* binding; the receipt is journaled |
| `wf.start(name, Workflow, input)` | start a child instance |
| `wf.now(name)`, `wf.uuid(name)` | journaled clock and id, deterministic across replays |

Rules for replay-safe code:

* **Every primitive takes a stable `name`.** Its identity is `(instance, name, n)`, where `n` counts repeated uses of the
  same name in one run. In a loop put the item in the name (`send:${policy.number}`), so a changed list does not
  re-attribute results.
* **Code between primitives must be deterministic.** Use `wf.now` / `wf.uuid` instead of `Date.now()` / `crypto`. Do not
  branch on anything that can change between replays (the current time, random numbers, unjournaled reads); read it in a
  `wf.transaction` or `wf.step` and branch on the recorded result.
* **`wf.transaction` and `wf.step` results must be JSON** (strings, numbers, booleans, `null`, arrays and plain objects
  of those). A result with a `Date`, a `Map`, a class or `undefined` fields is a type error whose text says
  "result is not JSON"; return the fields you need, not the row object. Replays return the recorded JSON.
* **Effects that leave the system go in `wf.step`**, with the idempotency key as the provider's idempotency header.
  Effects on your own data go in `wf.transaction`.
* A workflow is pinned to the package version that started it; new versions affect new instances only. An administrator
  can terminate, resume or restart (start-v2) an instance, deliver a signal, decide a task, or resolve a blocked step
  (`ext_workflow_*`).
* `Promise.all` over primitives suspends sequentially in this release; prefer a loop or child workflows for fan-out.

Start a workflow with `tx.start(W, input, { key })` (§7.3) or `wf.start`. Send it a signal with
`tx.signal(instance, "name", payload)` using the handle `tx.start` returned, or `ctx.signal({ workflow: W.id, instance }, …)`.

## 11. Notifications

A **notice** is a typed message with a payload; **templates** turn it into SMS, e-mail, in-app or push content;
`tx.notify` sends it to recipients over channels. Recipient, subject and channel target are three different things:

* the **recipient** is who gets it: `recipient.user(ref(core.User, "42"))`, `recipient.record(clientRef)` (a record whose
  contact data a contact resolver provides), or `recipient.address({ sms: "+998…", email: "…" })`;
* the **subject** is the record the message is about (`subject: row.ref`);
* the **target** is the resolved phone number, address or device.

```ts
// example/src/contracts.ts
export const RedemptionResult = notice("acme.loyalty.redemptionResult.v1", {
  subject: Redemption,
  payload: t.object({ reward: t.string(), approved: t.bool() }),
})
```

```ts
// example/src/server.ts
export const resultTemplates = templates(c.RedemptionResult, {
  sms: ({ payload }) => (payload.approved ? `Your reward "${payload.reward}" is approved.` : `Your reward "${payload.reward}" was declined.`),
  email: ({ payload, locale }) => ({
    subject: payload.approved ? "Reward approved" : "Reward declined",
    html: `<p lang="${locale}">${payload.reward}</p>`,
    text: payload.reward,
  }),
})
```

A record recipient needs a **contact resolver**: a contract for the entity, declared in the contracts graph, and its
implementation, bound in the server graph:

```ts
// example/src/contracts.ts
export const ClientContacts = contactResolver("acme.loyalty.clientContacts", crm.Client)
```

```ts
// example/src/server.ts
export const clientContacts = implement(c.ClientContacts, async (q, record) => {
  const client = await q.get(crm.Client, record)
  if (client === null) return {}
  return {
    ...(client.phone === null ? {} : { sms: [client.phone] }),
    ...(client.email === null ? {} : { email: [client.email] }),
  }
})
```

* `tx.notify(Notice, { subject, payload, to: [...], channels: [...] })` is enqueued **with the transaction**; rendering
  (with the template version pinned when it was sent) and provider delivery happen after the commit and never roll it
  back. A notice needs templates in some active package, or the call fails.
* A template renderer exists per channel (`sms`, `email`, `inApp`, `push`); a channel without a renderer is skipped for
  that notice. Renderers receive `{ payload, subject, locale }` and may be async.
* `recipient.record(ref)` needs the **contact resolver** above for that entity (`contactResolver(id, Entity)` +
  `implement`); it returns the targets per channel (`sms`, `email`, `push`); a `push` target is a device token and each
  token is its own delivery with its own outcome. It has no `inApp` target: an in-app message addresses an ERP user.
* A **delivery policy** (`deliveryPolicy(id, Entity)` + `implement`) is where consent and opt-out live: the SDK has no
  consent schema, the package that owns the data decides. See [Delivery policies](#delivery-policies).
* `recipient.user(...)` addresses an ERP user through the platform's notification preferences; it takes a ref to
  `core.user`.
* **Mixed recipients.** `channels` apply to **every** recipient of the call: the platform tries each recipient × channel
  pair and records a skipped delivery (with the reason) for a pair that has no target, instead of failing the call. A
  record recipient is reachable over `sms`, `email` and `push` only if its resolver returns targets; `inApp` reaches ERP users
  only (`recipient.user`), so `to: [recipient.record(client), recipient.user(agent)], channels: ["sms", "inApp"]` sends the
  SMS to the client, the in-app message to the agent, and records `record × inApp` and (with no phone on the user) `user × sms`
  as skipped. That is rarely what you mean: **send one `tx.notify` per audience**, each with the channels that fit it
  (`wf.transaction` can hold both calls), and read the skipped rows in `host.deliveries` (§14.2) when a message does not
  arrive.
* Use a stable `name`/key on repeated sends in workflows (`wf.transaction("send:${id}", …)`) so a replay does not notify
  twice.

### Delivery policies

```ts
// example/src/contracts.ts
export const ClientConsent = deliveryPolicy("acme.loyalty.clientConsent", crm.Client)
```

```ts
// example/src/server.ts
export const clientConsent = implement(c.ClientConsent, async (q, request) => {
  const loyalty = await q.get(c.ClientLoyalty, request.recipient.ref)
  if (request.channel === "sms" && loyalty?.smsOptOut === true) return { allow: false, reason: "sms_opt_out" }
  return { allow: true }
})
```

Once per notice, channel and **target** about to be created for a record recipient of the policy's entity, the platform
calls every active policy of that entity. The call is made after the sender's transaction committed and never inside
any write transaction; it runs as the **trigger owner** of the policy (assign one like for a contact resolver:
`PUT /_ext/manage/triggers/{policyId}/owner`, `ext_owner_assign`) with that user's current rights. A policy of a
package that is not active (deactivated, replaced, never activated) is simply not consulted.

The request and the answer are a **public, versioned contract**; fields are only ever added. The test host evaluates bound
policies with the same request (§14.2).

```jsonc
// request (the second argument of the handler)
{
  "notice":    { "key": "acme.loyalty.redemption_result.v1", "version": 1, "snapshot": "acme.loyalty@1.4.0" },
  "subject":   { "entity": "acme.loyalty.redemption", "key": "r-1" },   // null when the notice has none
  "recipient": { "kind": "record", "ref": { "entity": "crm.client", "key": "c-1" }, "address": null },
  "channel":   "sms",                                                   // "sms" | "email" | "push"
  "target":    { "address": "+998901234567", "deviceId": null }         // deviceId only for a registered push device
}
// answer
{ "allow": true }
{ "allow": false, "reason": "sms_opt_out" }
```

* `notice.key` is the catalog key of the notice: its ID with each segment in snake_case and a trailing `.vN`.
  `snapshot` is the package version that rendered it.
* `recipient.kind` is `"record"` today (`ref` set, `address` null). Handle kinds you do not know by allowing; new kinds
  will only be added with their own fields. Address recipients (a bare phone or email) have no entity to bind a policy
  to and are governed by the application's Go-configured policy only.
* `reason` is optional: a short machine code of `[a-z0-9_.-]`, at most 64 characters, without personal data. A denial
  is stored per target as a **skipped** delivery with error code `policy_denied:<reason>` (or `policy_denied`); the
  provider is not called and sibling targets (other channels, the other device) are unaffected.
* Composition is deterministic: the Go-configured `ModuleOptions.NotificationDeliveryPolicy` is consulted first, then
  the policies of active packages ordered by policy ID and package. **Any denial wins**; the first denial's reason is the
  one stored and later policies are not asked.
* A policy that cannot answer is never an implicit allow. No owner assigned, a handler that throws, or an answer that is
  not `{ allow: boolean, reason?: string }` all leave the delivery waiting and retried (visible as `unresolved`, and
  `failed` with `resolution_attempt_limit` when attempts run out, after which an operator can retry it). Assign the
  owner or fix the policy and the waiting deliveries proceed.

## 12. Triggers

Besides calls from users and other code, a package can run on a schedule and answer HTTP requests.

```ts
// example/src/server.ts
export const closeStale = schedule("acme.loyalty.closeStale", { cron: "0 3 * * *", timezone: "Asia/Tashkent" }, async (ctx) => {
  const cutoff: ISODateTime = isoDateTime(new Date(Date.parse(ctx.slot) - 7 * 86_400_000).toISOString())
  await ctx.transaction(async (tx) => {
    const stale = await tx.find(c.Redemption, { where: { status: "pending", createdAt: { lt: cutoff } } })
    for (const row of stale) {
      await tx.update(c.Redemption, row.ref, { status: "rejected" })
      await tx.call(c.AdjustPoints, { client: row.client, delta: row.cost })
    }
  })
})
```

```ts
// example/src/server.ts
export const publicCatalog = endpoint("acme.loyalty.publicCatalog", { method: "GET", path: "/catalog", public: true }, async (ctx) => {
  const rewards = await ctx.query.find(c.Reward, { where: { active: true }, orderBy: [{ field: "name" }] })
  return { status: 200, body: rewards.map((r) => ({ name: r.name, cost: r.cost })) }
})
```

* `schedule(id, cron | { cron, timezone }, handler)`: a 5-field cron expression and an optional IANA time zone. `ctx.slot`
  is the scheduled instant. A slot older than 15 minutes (after downtime) is skipped, not replayed.
* `endpoint(id, { method, path, public }, handler)` is served at `/_ext/endpoints/<namespace>/<path>`; the handler gets the
  command context plus `ctx.request` (`method path headers query body rawBody text()`) and returns
  `{ status, headers?, body? }`. `rawBody` is for signature checks. Response `headers` are an allowlist (content type,
  caching, `ETag`, `Content-Disposition`, a same-origin `Location` on a redirect, your own `X-` headers); `Set-Cookie`, CORS
  and `Content-Security-Policy` are dropped, and HTML/SVG/XML is served sandboxed (`Content-Security-Policy: sandbox`).
* **Who runs it.** A schedule, a subscription on a user-less event and a *public* endpoint run as the trigger's
  **owner-user**, assigned per tenant by an administrator (`ext_owner_assign`) and executed with that user's current
  rights. An authenticated (non-public) endpoint runs as the calling user; a *public* endpoint ignores the caller's session
  and always runs as the owner, even when it is called from a signed-in browser. A trigger without an owner is `blocked`.

## 13. User interface

The client graph exports Solid pages, navigation, slot contributions, cell renderers and (rarely) a shell. Pages are mounted by the canonical
client host inside the application's authenticated layout, the same boundary as the built-in pages.

```tsx
// example/src/client.tsx
export const rewards = page("acme.loyalty.rewards", {
  path: "/loyalty/rewards",
  nav: { label: { en: "Rewards", ru: "Награды" }, group: "acme.loyalty.menu", icon: "gift" },
  permission: c.Manage,
  component: lazy(() => import("./RewardsScreen")),
})
```

```tsx
// example/src/client.tsx
export const nav = navigation({
  groups: [{ id: "acme.loyalty.menu", label: { en: "Loyalty", ru: "Лояльность" } }],
})
```

* **`nav.group` is the ID of a group declared by `navigation({ groups })`** (here `acme.loyalty.menu`), in this package or,
  checked at activation, in another one. `nav.parent`, palette targets and `patches` refs must resolve the same way. An
  unknown group is a manifest error. The page's own ID (`acme.loyalty.rewards`) is separate.
* `page(id, { path, nav?, permission?, component })` declares a page by ID and path; `:name` segments arrive as string
  `props.params`. `page(Route, { component })` binds a page to a typed route and gets the ID `<route id>.page`; link to a
  route with `href(Route, { id })`, which validates and percent-encodes.
* `navigation({ groups, patches, workspaces, palette })` — `patches` hide, rename or move existing nodes by resource ID;
  `workspaces` and `palette` add workspaces and command-palette entries.
* **Data from the browser**: `useQuery(Q, input)` and `useCommand(C)` from `@iota-uz/sdk/runtime/client` call
  `POST /_ext/rpc` as the signed-in user. Queries and commands are callable from the browser; **operations never are**.
  The contract's `permission` is checked there, and read rules filter the rows.
* **Page for a platform route.** `page(foreignRoute, …)` with a route from the kit is a *screen replacement candidate*,
  active only through a `screen` binding (like command replacement, §7.4); see §13.3.

### 13.1 Slots

A *slot* is a place in a page where other packages mount components. The **page that owns it declares it**: a stable ID,
the typed props it passes to every contribution and, optionally, a permission. Contributions are **additive**: they need no
binding, they are active while their package version is, and every contribution of every active package is mounted.

**Slots of the platform** are declared by Go and arrive in the kit like any other contract. The CRM declares
`crm.slots.clientSidebar` (props `{ clientId }`) on its client view page and in the client drawer; contribute to it with the
typed reference:

```tsx
// example/src/client.tsx
export const loyaltyCard = slot(crm.ClientSidebar, (props) => <p class="loyalty-card">Loyalty for client {props.clientId}</p>, {
  order: 10,
  permission: c.Manage,
})
```

* `slot(target, Component, { order?, permission?, id? })`. `target` is the typed reference from the kit (`crm.ClientSidebar`) or
  from your own `defineSlot`; a plain slot ID string is accepted but its props are untyped. The component receives the props of
  the declaring page.
* **Order.** Contributions to one slot are mounted by ascending `order` (default 0), ties broken by package namespace, then
  contribution ID. The order is the same on every render.
* **Permission.** The server decides who gets a contribution: the slot's own permission (set by its declaration) and the
  contribution's `permission` must both be held by the viewer. A contribution the viewer may not have is **absent from the page
  data / HTML**, not hidden by the browser. The permission is the same kind of name as a page's (`permission(...)`).
* A broken contribution (its module fails to load, or its component throws) is reported to the console and leaves the page and
  the other contributions alone. A module that arrives after the user already left the page is dropped.

**Your own slots.** Declare a slot in the contracts graph (it is a contract: other packages import it through the kit) and
render it in a page of yours with `<Slot>`:

```ts
// example/src/contracts.ts
export const RewardActions = defineSlot("acme.loyalty.rewardActions", { props: t.object({ reward: t.string() }) })
```

```tsx
// example/src/RewardsScreen.tsx
{r.name}: {r.cost} <Slot slot={c.RewardActions} props={{ reward: r.name }} />
```

```tsx
// example/src/client.tsx
export const exportAction = slot(c.RewardActions, (props) => <button type="button">Export {props.reward}</button>)
```

`<Slot slot={…} props={…}/>` comes from `@iota-uz/sdk/runtime/client` and works in a package page and in a shell renderer. (The
prop is `slot`, not `ref`: Solid's compiler reserves `ref` on every component.) `props` must match the declared props.

**Slots in Go-rendered (Templ/HTMX) pages** use `shellext.Slot(ctx, "module.slots.name", props)` in the template. It renders a
mount element with the props and the contributions the viewer gets; nothing when none applies. The browser runtime mounts each
contribution into its own Solid root and releases it when the element is swapped out by HTMX or the page is left. A Go page
declares its slot as a native resource (`manifest.Resource{ID, Kind: slot, Props, Permission}`), see
`modules/crm/extension` for the CRM's.

### 13.2 Cell renderers

```tsx
// example/src/client.tsx
export const tierCell = cell(c.TierColumn, (props) => <span class={`tier tier-${props.value}`}>{props.value}</span>)
```

`cell(column, component)` customises how a `column(...)` renders in the native list. Without it the value is shown as text.

* The list stays **server-rendered**: the cell is the standard cell for the column's type, wrapped in a mount element. Your
  component replaces it in the browser once the module has loaded; if it never loads or throws, the standard cell stays. The
  page reads the same without JavaScript.
* `props.value` is the column's value as wire JSON (`int` is a number; `int64` and `decimal` are strings; `null` when absent).
  `props.row` is the row's contributed values by column ID (only columns the viewer may read) and `ref`, the native record
  (`{ entity, key }`). Access to the column follows the overlay field's access rules: a viewer who may not read the column gets
  no cell and no value.
* **Exports, filters and sorting never use the renderer**: they read the raw value.
* One renderer serves a column. If several packages supply one, the package that declares the column wins, otherwise the first
  by package name; the others are ignored and reported.

### 13.3 Replacing a native screen

`page(crm.ClientView, { component })` with a route from the kit makes your page a *screen replacement candidate* for that
route; the CRM client view (`crm.routes.clientView`, `/crm/clients/:id`) is the reference:

```tsx
// example/src/client.tsx
export const clientView = page(crm.ClientView, { component: lazy(() => import("./ClientScreen")) })
```

```tsx
// example/src/ClientScreen.tsx
export default function ClientScreen(props: PageProps<{ id: number }>) {
```

* The page is **not served** by itself. An administrator binds it: `POST /_ext/manage/bindings` with
  `{"slotKind":"screen","targetId":"crm.routes.clientView","package":"…","version":"…","export":"clientView"}` (or `ext_bind`).
  One package page holds a route at a time; unbinding restores the native page without a restart.
* **Typed params.** The route declares its parameters (`id` is an `int`), so `props.params.id` is a number, decoded from the path
  before the page renders; a path the type refuses (`/crm/clients/abc`) is a 404 and never reaches your page.
* **Access never widens.** The replacement is served only to viewers who hold what the native screen requires (the route's
  permission, `Client.Read` here) and what your page's own `permission` requires.
* Only the *screen* is replaced: HTMX requests for fragments of the native page (the client drawer) still reach the native
  handlers.

### 13.4 The shell

`shellRenderer((p) => <MyShell {...p} />)` replaces the whole authenticated shell (navigation, header) around every page,
once an administrator binds it to the `shell` slot. The component receives the host-owned `outlet` element that already
contains the current page; render it exactly once:

```tsx
export const shell = shellRenderer((p) => (
  <div class="shell">
    <nav><For each={p.nav.groups}>{(group) => <For each={group.items}>{(item) => <a href={item.href}>{item.label}</a>}</For>}</For></nav>
    <header>{p.user.name}</header>
    <main>{p.outlet}</main>
  </div>
))
```

Client chunks must not import the server entry, anything only the server reaches, `@iota-uz/sdk/runtime/server` or a Node
built-in (`graph.serverInClient`); Solid and the runtime client are not bundled: the host provides them.

## 14. Testing

`iota-ext test` runs every `*.test.ts` file against an **in-memory test host**: the real contract definitions, your
real handlers, no database, no worker pool, no network. Tests are fast and deterministic, and run before you publish.

```ts
// example/src/server.test.ts
import {
  assert, createTestHost, fixture, overlayFixture, test, type TestHostOptions,
} from "@iota-uz/sdk/runtime/testing"
```

Import everything test-related from **`@iota-uz/sdk/runtime/testing`**. (`@iota-uz/sdk/testing`, without `runtime`, is a different
package, the Playwright helpers.)

```ts
// example/src/server.test.ts
function host(options: TestHostOptions = {}) {
  return createTestHost({
    implementations: [s.adjustPoints, s.createReward, s.redeem, s.catalog, s.approval, s.reminder, s.clientContacts, s.clientConsent, s.resultTemplates, s.reminderTemplates],
    user: { permissions: [c.Manage.id] },
    secrets: { PARTNER_TOKEN: "token-for-tests" },
    ...options,
    fixtures: [ali, agent, ...(options.fixtures ?? [])],
  })
}
```

Register cases with `test(name, fn)` (and group with `describe(name, body)`). A case passes unless it throws or rejects.
The command type-checks the package first and runs nothing when it does not compile; failures print the assertion
message and the line in **your** `.ts` file (the bundles are source-mapped), `--json` gives the same as data.

### 14.1 The host

| `createTestHost({ … })` option | meaning |
|---|---|
| `fixtures` | initial rows: `fixture(Entity, values, { key })` and `overlayFixture(Overlay, baseRef, values)`; defaults apply (`createdAt` / `updatedAt` default to `EPOCH` on every entity), values are validated; native entities need an explicit `key` |
| `implementations` | handlers `tx.call` / `ctx.call` / `wf.call` dispatch to, by contract; also `templates(...)`, and the `contactResolver` and `deliveryPolicy` implementations notice delivery uses |
| `templates` | `templates(...)` exports (alternatively list them in `implementations`) |
| `contacts` | `(record) => ContactTargets \| undefined`: contact data of record recipients, answered before a bound resolver |
| `guards` | `guard`, `precheck` and `writeRule` values to apply |
| `entities` | entity definitions the host cannot learn from calls (targets of ref paths) |
| `user` | `{ id, permissions, locale, kind }` of the acting principal; read rules and hidden fields follow `permissions` |
| `secrets` | values for `ctx.secrets.get` |
| `fakes` | your own `fakeEffects()` instance |
| `now` | start of the host clock |
| `snapshot` | the `package@version` that delivery policies receive as `notice.snapshot` (default `<owner>.<module>@0.0.0`) |

| `host.` | meaning |
|---|---|
| `transaction(cb)` | runs `cb(tx)` in one transaction: commit, or rollback when it throws. `host.transaction((tx) => impl.handler(tx, input))` |
| `command()` / `query()` | an ordinary-command context (`ctx.transaction`, `ctx.query`, `ctx.call`, secrets) / a read-only context |
| `deliver(subscription, { subject, payload })` | deliver an event to a subscription in its own transaction |
| `workflow(impl, input)` | a driven run: `run()`, `decide(task, decision)`, `signal(...)`, `tasks`, `journal`; `run()` returns `completed`, `suspended` (on a sleep, signal or task) or `failed` |
| `clock.advance("30d")` | move time; timers fire on the next `run()` |
| `table(Entity)`, `overlayRow(Overlay, ref)` | committed rows |
| `emitted`, `notices`, `started`, `signals`, `logs` | what committed transactions recorded (rolled back work is not there) |
| `deliveries` | the outcome of every notice delivery (§14.2) |
| `fakes` | `sms`, `email`, `push`, `inApp`, `http` |

What the host reproduces from the Go data layer: SQL null semantics, filters through refs, ordering with the primary-key
tie-break, read rules before filter and paging, hidden fields (not readable, filterable, sortable or writable), overlay
rows readable only with their base row, unique constraints (`data.unique`), `rowVersion` conflicts, native-write
refusal (`data.nativeWrite`), the `tx.call` / `ctx.call` rules (`runtime.ordinaryCommandInTx`,
`runtime.intentInTransaction`, `runtime.nestedTransaction`), write rules, rollback of events/notices/starts/signals.
These semantics are pinned by a shared conformance suite (`web/runtime/tests/conformance/cases.json`) that runs the
same package and cases on this host and on the Go host with PostgreSQL, so the two cannot drift apart. **What it does not do**: it does not check a contract's
`permission` (you call the handler directly; test the gate in the application's end-to-end suite), it does not verify
that a ref points at an existing row, and handlers see one logical process (no concurrency).

### 14.2 Notifications in tests

With any `templates` bound, a committed `tx.notify` is rendered and delivered, per recipient and channel, to the fakes:

* **address** recipients are used as given (`address.sms` → `fakes.sms`);
* **record** recipients go through `contacts`, else the `contactResolver` implementation bound for that entity;
* **user** recipients read the `core.user` fixture: `sms` → phone, `email` → e-mail, `inApp` and `push` → the user key, and
  `uiLanguage` becomes the template `locale`.

```ts
// example/src/server.test.ts
const agent = fixture(core.User, { firstName: "Agent", lastName: "A", email: "agent@example.uz", uiLanguage: "en" }, { key: "42" })
```

A native entity's fixture needs an explicit `key` and the fields the entity requires (`firstName`, `lastName`, `email` and
`uiLanguage` for `core.User`); `createdAt` and `updatedAt` default to `EPOCH` (`"2026-01-01T00:00:00Z"`, exported by the same
module) on every entity, whether or not it declares them as fields, so you never write them. Pass them to test time-dependent
code.

```ts
// example/src/server.test.ts
  // Rendered with the templates, delivered per recipient and channel to the fakes.
  assert.deepEqual(h.fakes.sms.delivered.map((m) => [m.to, m.text]), [["+998901112233", 'Your reward "Mug" is approved.']])
  assert.deepEqual(h.fakes.email.delivered.map((m) => m.to), ["ali@example.uz"])
  assert.deepEqual(h.deliveries.map((d) => [d.channel, d.status]), [["sms", "delivered"], ["email", "delivered"]])
```

Each `host.deliveries` entry is `{ notice, channel, recipient, target, status, reason?, content? }` with status
`delivered`, `skipped` (no renderer for the channel, no address, no contact data, a denied policy; `reason` says which) or
`failed` (the fake channel rejected it: `fakes.sms.failNext(new Error("down"))`, the template threw, or a policy failed).
Delivery happens after commit, so a rolled-back transaction delivers nothing. Without any templates, `tx.notify` is only
recorded in `host.notices`; with templates, a notice none of them covers is a `notify.noTemplates` error, as in the runtime.

**Every channel is tried for every recipient** (§11, mixed recipients). The rows tell you which pair had no target:

```ts
// example/src/server.test.ts
  assert.deepEqual(h.deliveries.map((d) => [d.recipient.kind, d.channel, d.status]), [
    ["record", "sms", "delivered"],
    ["record", "inApp", "skipped"], // in-app messages address ERP users, not client records
    ["user", "sms", "skipped"], // the core.User fixture has no phone
    ["user", "inApp", "delivered"],
  ])
```

**Delivery policies are evaluated.** For a record recipient, before a message is sent to a target over `sms`, `email` or
`push`, the host asks every `deliveryPolicy` implementation bound for the recipient's entity (list it in
`implementations`), with the request shape of §11: `notice.key` and `version` derived from the notice ID, `snapshot` from
the `snapshot` option, `target.address` the resolved phone or e-mail and `deviceId` null. Policies run in contract-ID order
with the rights of the test host's user and the first denial wins: the delivery is `skipped` with reason
`policy_denied:<reason>` (or `policy_denied`; a `reason` that is not `[a-z0-9_.-]{1,64}` is dropped as in production) and the
fake channel is not called, while the other targets and channels go on. A policy that throws, or answers anything but
`{ allow: boolean, reason?: string }`, makes the delivery `failed`: like the platform, it is never an implicit allow.
In-app, user and address recipients have no policy.

```ts
// example/src/server.test.ts
  const h = host({ fixtures: [mug, redemption, overlayFixture(c.ClientLoyalty, ali.ref, { smsOptOut: true })] })
  await h.transaction((tx) =>
    tx.notify(c.RedemptionResult, { subject: redemption.ref, payload: { reward: "Mug", approved: true }, to: [recipient.record(ali.ref)], channels: ["sms", "email"] }),
  )
  assert.deepEqual(h.fakes.sms.delivered, [])
  assert.deepEqual(h.fakes.email.delivered.map((m) => m.to), ["ali@example.uz"])
  assert.deepEqual(h.deliveries.map((d) => [d.channel, d.status, d.reason]), [["sms", "skipped", "policy_denied:sms_opt_out"], ["email", "delivered", undefined]])
```

What stays outside the host: the trigger-owner assignment (a policy with no owner is retried in production; the host just
runs it) and the application's Go-configured policy. Unit-test a policy in isolation by calling its handler with a request
object, as you would any query handler.

### 14.3 `fetch` and the fakes

Inside `iota-ext test` outbound network is blocked: `fetch`, sockets and DNS fail. The one way out is a **fake HTTP
route**: while a test host exists, the global `fetch` in the code under test is answered by `host.fakes.http`.

```ts
// example/src/server.test.ts
  h.fakes.http.route("GET", "https://api.partner.example/stock/Mug", { status: 200, body: { available: 3 } })
```

`route(method, url, response | (request) => response)` matches the method and the exact URL; an unrouted request throws
`UnmatchedRequestError`; `fakes.http.calls` records method, url, headers and parsed JSON body. The other channels record
every attempt in `calls` and every unique delivery in `delivered` (idempotency keys dedupe). Create the host *inside*
each test (the last host created owns the routes) and call `fakes.reset()` if you share one.

### 14.4 Assertions and doubles

`assert` (from the same module) needs no Node typings and prints diffs: `ok`, `equal` (`Object.is`), `notEqual`,
`deepEqual` (first differing path in the message), `partial` (the actual value contains these properties), `match`,
`throws`, `rejects`, `fail`. `assert.rejects(promiseOrFn, matcher)` takes a message substring or RegExp, an error class,
a **domain error contract** (`NotEnoughPoints`), or `{ code, kind, message, issue }`:

```ts
// example/src/server.test.ts
  await assert.rejects(h.transaction((tx) => tx.call(c.AdjustPoints, { client: ali.ref, delta: 20_000 })), { kind: "validation", issue: c.DeltaTooLarge })
```

You may use `node:assert/strict` instead (install `@types/node`). A native contract has no handler in a test host: bind a
double with `implement(crm.UpsertClient, async (tx, input) => …)` and pass it in `implementations`:

```ts
// example/src/server.test.ts
  const upsert = implement(crm.UpsertClient, async (_tx, input) => ({ client: input.client ?? ali.ref, created: input.client === undefined }))
```

A subscription, schedule or endpoint handler is a plain function: call `sub.handler(tx, event)` inside
`host.transaction`, or `host.deliver(sub, event)`; for a schedule or endpoint pass `{ ...host.command(), slot }` /
`{ ...host.command(), request }`.

### 14.5 What the offline tools do not cover: the client graph

`iota-ext test` runs your **server** code. There is no offline surface that mounts a Solid page, slot or cell renderer: that
needs a DOM, a browser build of Solid and the host's shared module registry, none of which the Node test host has, and a
stub would only hide differences from the browser. What you do have offline:

* `iota-ext check` / `build` typecheck the client graph (props, `useQuery` / `useCommand` inputs and results are typed from
  your contracts), enforce its rules (no server imports, no Node built-ins) and bundle it for the browser, so a screen that
  compiles calls your contracts correctly;
* every query and command a screen calls is a plain contract you can test on the host (§14.1), including its permission
  behaviour through read rules; keep screen logic that deserves a test in a pure `.ts` file and test that file;
* the loading, error and empty states and the markup are checked in the application: activate the package on a development
  instance, or render the component in your own Vitest + jsdom setup with `vite-plugin-solid`: wrap it in
  `RuntimeProvider` (exported by `@iota-uz/sdk/runtime/client`) with a fake transport,
  `{ invoke: async (kind, resource, input) => … }`, which is what `useQuery` and `useCommand` call.

## 15. Errors the tools print

Every diagnostic is `file:line:col code message` and points at **your** source.

| code | meaning |
|---|---|
| `ts.<n>` | a TypeScript error (the number is the compiler's). Messages for the usual mistakes name the fix: a write to a native entity → "use a published operation (`tx.call`)"; ordering by an enum → "cannot order by 'status'"; a range on a non-orderable field; a filter on a `json` field; `tx.call` of an ordinary command → "use `ctx.call`"; a keyless `tx.start` → "needs a start key" |
| `graph.impureContract` | the contracts graph imported `@iota-uz/sdk/runtime/server\|client`, Solid or a Node built-in |
| `graph.clientInServer` / `graph.serverInClient` | a graph reached the other one (the message names the file and, for npm, "through …") |
| `graph.unknownKit` | an `@iota-kit/<ns>` module is not in the kit: `iota-ext kit generate` or `kit pull` (reported once per file and module; the type errors it causes are hidden and counted by the `kit.followOn` warning) |
| `manifest.duplicateId` / `outsideNamespace` / `invalidId` | an ID is declared twice, outside the namespace, or malformed. Entities, routes and workflows share one ID space: give them different names |
| `manifest.undeclaredReference` | a resource refers to a contract that is not exported from the contracts graph |
| `manifest.wrongGraph` | a contract exported from the server graph, a server value from the client one, … |
| `manifest.duplicateImplementation` / `duplicateTemplates` | two exports implement one contract / one notice |
| `manifest.invalidDeclaration` | an invalid cron expression, endpoint path, guard target, … |
| `manifest.import` | importing a bundle failed (a top-level error in your code) |
| `build.unresolved` / `build.error` / `build.jsx` | a bundling error: a missing module, a JSX problem |
| `package.*` | `package.json` problems: missing `iota` section, a bad namespace, an entry that does not exist |
| `npm.lockfileMissing` | published packages with dependencies need `package-lock.json` |

Export resources as **top-level named exports**: a contract nested inside an exported object or array is an error
("export resources as top-level named exports"). A client export's implicit ID lowers the first letter of the export
name (`PolicyBadge` → `<namespace>.policyBadge`).

## 16. The `iota-ext` command

```text
iota-ext init <dir> --namespace <owner.module> [--name <npm name>] [--no-client] [--no-kit] [--force]
iota-ext kit generate [--catalog <file|sdk>]...        # offline; default: the SDK natives
iota-ext kit pull --server <url> [--token-env VAR] [--cookie-env VAR]
iota-ext kit status [--json]                            # verify .iota/kit: modules, files, hashes, tsconfig paths; exit 1 when damaged
iota-ext check [--json]
iota-ext build [--out <dir>] [--json]                   # default .iota/build: artifacts/ and manifest.json
iota-ext test [filter...] [--json]
```

All commands except `init` take `--dir <package dir>` (default: the current directory). Output is for people; `--json`
prints one JSON document for machines:

| command | `--json` document |
|---|---|
| `check` | `{ ok, diagnostics: [{ file, line?, column?, severity, code, message }], manifest }` |
| `build` | `{ ok, out, diagnostics, artifacts: [path], manifest }` |
| `test` | `{ ok, passed, failed, diagnostics, cases: [{ name, file, passed, durationMs, message?, location?: { file, line, column }, frames? }] }` |
| `kit generate` / `kit pull` | `{ ok, hash, modules, dir, editor }` (`editor`: `present`, `added`, `manual` or `absent`, the state of the tsconfig `paths` entry) |
| `kit status` | `{ ok, installed, dir, location, hash, source, modules: [{ name, state, problems }], editor }` (`state`: `ok`, `missing`, `modified`, `unrecorded`) |

**Exit codes**: `0` success; `1` the package has errors or a test failed; `2` usage (unknown option, bad namespace, not a
package directory); `3` environment (the network, the build worker, an unreadable catalog file, an HTTP 401/403 from
`kit pull`).

`check` typechecks, enforces the graph rules, builds all three graphs and extracts the manifest, writing nothing.
`build` does the same and writes `artifacts/` (`contracts.mjs`, `server.mjs`, `client/…`) and `manifest.json`; the
artifacts are what the platform would build from your sources, byte for byte. `test` compiles first and refuses to run a
package with type errors.

## 17. Publishing and operating

The CLI is for the loop on your machine. A package goes live through the application, by the configurator UI
(`/extensions`, with a Monaco editor, versions and workflows) or by MCP tools / the HTTP API under `/_ext/manage`.
Both do the same things:

1. **Create a draft** (`ext_package_create`) and put your files in it: write them (`ext_file_write`), upload the tree, or
   import a Git commit (`ext_import_git`). Include `package-lock.json` when you have dependencies.
2. **Check, test, build** (`ext_check`, `ext_test`, `ext_build`): the same worker and rules as the CLI, against the
   application's *real* kit.
3. **Publish** (`ext_publish`): an immutable version with source and artifact hashes, the manifest and the kit snapshot it
   resolved against. Foreign references are validated against that snapshot.
4. **Review** (`ext_review`): a reviewer approves a version, bound to its exact source and artifact hashes. A version runs
   only while its newest review is an approval.
5. **Activate** (`ext_activate`): the version becomes the live one for the whole application: its schema is applied
   (new fields and tables; incompatible changes are refused), its resources, subscriptions, schedules and endpoints
   start. Assign **trigger owners** for user-less triggers (`ext_owner_assign`) and **bind** replacement candidates
   (`ext_bind`: a native command, a screen, the shell).
6. **Operate**: `ext_workflow_*` (inspect, signal, decide tasks, terminate, restart, resolve), `ext_audit_list`,
   `ext_deactivate`, `ext_purge` (deletes the package's data).

Permissions: *read* (kit, versions), *author* (drafts), *review*, *publish* (activate, deactivate, purge, bind, owners), *operate* (execute, workflow actions, requeue).
Source control is yours: `ext_export` returns the draft as a tree to commit.

## 18. Reference: where to look

* The API by import path: `@iota-uz/sdk/runtime` (contracts, `t`, `field`, `entity`, `overlay`, `column`, `defineSlot`, `ref`, `$`,
  `href`, wire helpers), `@iota-uz/sdk/runtime/server` (`implement`, `subscribe`, `guard`, `precheck`, `writeRule`, `schedule`,
  `endpoint`, `templates`, `recipient`, context types), `@iota-uz/sdk/runtime/client` (`page`, `navigation`, `slot`, `Slot`, `cell`,
  `shellRenderer`, `useQuery`, `useCommand`, `isError`, `issuesOf`), `@iota-uz/sdk/runtime/testing` (everything in §14: the test
  host, `fixture`, `EPOCH`, `assert`, the fakes; **not** `@iota-uz/sdk/testing`, which is the Playwright package).
  `@iota-uz/sdk/runtime/kit` is internal to generated modules; never import it.
* The complete working package: [`example/`](example). More real packages (an overlay with columns, a custom cell, slot contributions and a slot of your own, a replacement
  client view, a campaign workflow with an SMS gateway endpoint, a replacement shell, versioned packages) are in `pkg/extruntime/e2e/testdata/`.
* The platform contract: [`../extension-runtime.md`](../extension-runtime.md), §9.1 onwards.
