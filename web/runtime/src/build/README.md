# Build worker

Node program behind `pkg/extruntime/build` (`NodeBuilder`). It is **not** the invocation pool: one
process per operation, one JSON request on stdin, one JSON response on stdout (anything a dependency
prints goes to stderr). Built by `scripts/build-worker.mjs` into `dist/build/` (shipped in
`@iota-uz/sdk` as `dist/runtime/build/`):

| file | role |
|---|---|
| `worker.mjs` | entry; `typescript` and `@babel/core` are bundled in, `esbuild` is resolved at run time (optional peer of `@iota-uz/sdk`, or `NODE_PATH`) |
| `extract.mjs` | child: imports the built bundles and runs `collectManifest` (isolated from the worker) |
| `test-child.mjs` | child: runs `*.test.ts` bundles with outbound network blocked |
| `vendor/` | TypeScript libs, `@types/node`, Solid (types + server build) the worker serves to packages |

Requires the host loader (`../host/register.mjs`, `../host/sdk/*.mjs`) next to it: artifacts keep
`@iota-uz/sdk/runtime*` as bare imports and every Node process that loads them (invocation worker,
extraction child, test child) registers `host/loader.ts` to resolve them.

## Operations

`{op, dir, outDir, kit}` where `dir` is the materialized tree (with `node_modules` when installed by
`npm ci --ignore-scripts`) and `kit = {hash, modules: {"@iota-kit/<ns>": {path: text}}}`.

* `check` / `build` — package.json (`iota.namespace|contracts|server|client|capabilities`) →
  TypeScript typecheck (`ts.<code>`, SDK typings and kit modules mounted at
  `node_modules/@iota-uz/sdk/runtime`, `node_modules/@iota-kit/*`) → three esbuild graphs →
  manifest extraction (`manifest.*`) → artifacts below `<outDir>/artifacts`:
  `contracts.mjs`, `server.mjs`, `client/index-<hash>.js`, `client/chunk-<hash>.js`,
  `client/manifest.json`. `check` runs the same pipeline and writes nothing.
* `test` — bundles every `*.test.ts` (SDK external, kit bundled), runs the cases registered with
  `test()` from `@iota-uz/sdk/runtime/testing` in `test-child.mjs`. `fetch`, `net` connections and `dns`
  are blocked; the only way out is a fake route of the host's `fakes.http`.
* `kit` — `{sources:[{namespace, resources, contractsModule?}]}` → `@iota-kit/<ns>` modules
  (`index.js`, `index.d.ts`, `package.json`) in dependency order.

## Graph rules (all diagnostics are `file:line:column` in the author's source)

| graph | rule | code |
|---|---|---|
| contracts | must not reach `@iota-uz/sdk/runtime/server|client`, `solid-js*`, Node builtins | `graph.impureContract` |
| server | must not reach `@iota-uz/sdk/runtime/client` | `graph.clientInServer` |
| client | must not reach the server entry, a module only the server graph reaches (not reachable from contracts; npm excluded), `@iota-uz/sdk/runtime/server`, Node builtins | `graph.serverInClient` |
| any | `@iota-kit/<ns>` not in the kit | `graph.unknownKit` |

Externals: `@iota-uz/sdk/runtime*` stay bare imports in every artifact. Kit modules and npm
dependencies are bundled (artifacts are self-contained and reproducible from tree + kit). Client chunks
additionally keep `solid-js`, `solid-js/web`, `solid-js/store` bare (listed in
`client/manifest.json` `imports`) so the host provides a single Solid instance.
