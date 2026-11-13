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
| `../cli/iota-ext.mjs` | the author CLI (`src/cli`): drives this worker; see `docs/extensions/authoring.md` |
| `vendor/` | TypeScript libs, `@types/node`, Solid (types + server build) the worker serves to packages |

Requires the host loader (`../host/register.mjs`, `../host/sdk/*.mjs`) next to it: artifacts keep
`@iota-uz/sdk/runtime*` as bare imports and every Node process that loads them (invocation worker,
extraction child, test child) registers `host/loader.ts` to resolve them.

## Operations

`{op, dir, outDir, kit}` where `dir` is the materialized tree (with `node_modules` when installed by
`npm ci --ignore-scripts`) and `kit = {hash, modules: {"@iota-kit/<ns>": {path: text}}}`.

* `check` / `build` — package.json (`iota.namespace|contracts|server|client|capabilities`) →
  TypeScript typecheck (`ts.<code>`, SDK typings and kit modules mounted at
  `node_modules/@iota-uz/sdk/runtime`, `node_modules/@iota-kit/*`: virtual paths, the request payload is the only source of the
  kit, so where the CLI keeps it on disk (`.iota/kit`) is not the worker's concern; an `@iota-kit/*` import the payload lacks is
  one `graph.unknownKit` diagnostic naming the kit commands, and the TS errors that follow from it are dropped) → three esbuild graphs →
  manifest extraction (`manifest.*`) → artifacts below `<outDir>/artifacts`:
  `contracts.mjs`, `server.mjs`, `client/index-<hash>.js`, `client/chunk-<hash>.js`,
  `client/tailwind.css` and `client/index-<hash>.css` (the version's stylesheets, see "Package styles"),
  `client/manifest.json`. `check` runs the same pipeline and writes nothing.
* `test` — typechecks the package first (`typecheck: false` skips it; type errors are returned as `diagnostics` and
  nothing runs), then bundles every `*.test.ts` (optionally only those matching `filter`; SDK external, kit bundled, with
  an external source map) and runs the cases registered with `test()` from `@iota-uz/sdk/runtime/testing` in
  `test-child.mjs`. A failing case carries `message`, the source-mapped `location` in the author's file and the
  `frames` (platform frames removed), never the temp bundle path. `fetch`, `net` connections and `dns`
  are blocked; the only way out is a fake route of the host's `fakes.http`.
* `kit` — `{sources:[{namespace, resources, contractsModule?, lists?}]}` (a catalog snapshot's sources) → `@iota-kit/<ns>` modules
  (`index.js`, `index.d.ts`, `package.json`) in dependency order; native entities carry their `keyType`.

## Graph rules (all diagnostics are `file:line:column` in the author's source)

| graph | rule | code |
|---|---|---|
| contracts | must not reach `@iota-uz/sdk/runtime/server|client`, `solid-js*`, Node builtins | `graph.impureContract` |
| server | must not reach `@iota-uz/sdk/runtime/client` | `graph.clientInServer` |
| client | must not reach the server entry, a module only the server graph reaches (not reachable from contracts; npm excluded), `@iota-uz/sdk/runtime/server`, Node builtins | `graph.serverInClient` |
| contracts, server | must not import a stylesheet (`.css`, `.module.css`) | `graph.cssOutsideClient` |
| contracts, server | must not import `@iota-uz/sdk/solid-ui` (the UI kit is client-only) | `graph.impureContract`, `graph.clientInServer` |
| client | must not import a global (non-module) stylesheet, from code or from another stylesheet | `graph.globalCss` |
| any | `@iota-kit/<ns>` not in the kit | `graph.unknownKit` |

Externals: `@iota-uz/sdk/runtime*` stay bare imports in server/contracts artifacts (resolved by the host
loader). Kit modules and npm dependencies are bundled (artifacts are self-contained and reproducible from
tree + kit). Client chunks never carry Solid or the runtime client: `solid-js`, `solid-js/web`,
`solid-js/store` and `@iota-uz/sdk/runtime/client` are replaced by generated shims that read the host's registry
`globalThis[Symbol.for("iota.runtime.shared")]` (`client-host/shared.ts`; the shim export names are generated
into `shared-exports.json` from the same sources). The pure `@iota-uz/sdk/runtime` is bundled. `client/manifest.json` lists the registry modules used
under `shared`.

## Package styles

A client graph styles itself with two mechanisms, both artifacts of the version (covered by its hash) and both listed in
`client/manifest.json` under `css`, utilities first:

* **Tailwind utilities** (`css.ts`): the pure-JS Tailwind v4 compiler runs in the worker over the author files of the client
  graph (the esbuild metafile's inputs, so exactly the files that ship), with the host's own token mapping (`build/theme/tailwind.config.js`,
  copied verbatim from the repository root by `build-worker.mjs`, so the host's pipeline and the packages cannot drift) and Tailwind's `theme`
  and `utilities` layers only. No preflight or base: the host page has them. Candidates are the string-like tokens of the sources; the compiler
  drops what is not a utility. The result is minified with esbuild and written to `client/tailwind.css`; no utility, no file.
* **Scoped CSS modules**: `*.module.css` imports go through esbuild's `local-css` loader as virtual modules named after the file, the package
  namespace and a digest of path and content, so the class names (`Board_1a2b3c4d_panel`) are unique to the package and to the version. esbuild bundles
  the CSS of the whole entry graph (lazy chunks included) into the entry's stylesheet; the per-chunk copies are dropped. A malformed stylesheet is
  `css.syntax`, located in the author's file.

`@iota-uz/sdk/solid-ui` is a shared specifier like Solid, but lazy: its shim awaits the host's loader
(`globalThis[Symbol.for("iota.runtime.shared.loaders")]`), so only hosts running a version that imports it pay for the kit. It is the
host's instance at the host's release version. Where a bundle is only evaluated in Node (manifest extraction, package tests) the specifier is an inert stub with the same exports.
`build-worker.mjs` records the kit's export names in `shared-exports.json` and, with `--emit-sdk`, emits its declarations next to the runtime ones (the assembled package
ships them as `dist/solid-ui`).
