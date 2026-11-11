// The three package graphs (contracts, server, client) plus the extraction/test bundles, built with
// esbuild. One plugin enforces the graph rules, resolves kit modules from memory, and runs the Solid
// JSX transform; violations are collected as source-located diagnostics instead of aborting the build.
import { builtinModules } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";

import babel from "@babel/core";
import solidPreset from "babel-preset-solid";
import type * as Esbuild from "esbuild";

import type { Layout } from "./layout.ts";
import { locateImport } from "./locate.ts";
import type { Diagnostic, KitPayload } from "./protocol.ts";
import { errorMessage, inNodeModules, relPath } from "./util.ts";

export type Mode = "contracts" | "server" | "client" | "extract" | "test";

export const SDK_RUNTIME = new Set([
  "@iota-uz/sdk/runtime",
  "@iota-uz/sdk/runtime/server",
  "@iota-uz/sdk/runtime/client",
  "@iota-uz/sdk/runtime/testing",
]);
const SOLID = /^solid-js(?:\/.*)?$/;
const BUILTINS = new Set(builtinModules);
const SKIP = Symbol("iota.skip");

/**
 * Specifiers client bundles never carry their own copy of: the browser host publishes its instances in
 * `globalThis[Symbol.for("iota.runtime.shared")]` (client-host/shared.ts) and bundles read them through a
 * generated shim. A second Solid or runtime client would break reactivity and the RuntimeProvider context.
 */
export const SHARED_SPECIFIERS: ReadonlySet<string> = new Set(["solid-js", "solid-js/web", "@iota-uz/sdk/runtime/client"]);
const SHARED_REGISTRY = 'globalThis[Symbol.for("iota.runtime.shared")]';

let sharedNames: Record<string, string[]> | undefined;

function sharedShim(layout: Layout, spec: string): string {
  sharedNames ??= JSON.parse(readFileSync(layout.sharedExports, "utf8")) as Record<string, string[]>;
  const names = sharedNames[spec] ?? [];
  return [
    `const m = ${SHARED_REGISTRY}?.[${JSON.stringify(spec)}];`,
    `if (m === undefined) throw new Error(${JSON.stringify(`iota runtime: the host did not publish shared module "${spec}"`)});`,
    names.length > 0 ? `export const { ${names.join(", ")} } = m;` : "",
  ].join("\n");
}

export interface GraphEnv {
  mode: Mode;
  dir: string;
  entry: string;
  layout: Layout;
  kit: KitPayload;
  /** Client: files reachable only from the server graph (relative to dir). */
  serverOnly?: ReadonlySet<string>;
  serverEntry?: string;
  /** Output base name of the entry (default: the mode). */
  outName?: string;
}

export interface GraphResult {
  diagnostics: Diagnostic[];
  /** Output files by path relative to the output directory. */
  files: Map<string, Uint8Array>;
  metafile: Esbuild.Metafile | undefined;
  /** Author files in the graph (relative to dir, node_modules and kit excluded). */
  inputs: Set<string>;
  kitModules: Set<string>;
  /** Files written for the entry output (relative path) and external bare imports. */
  entryOutput: string | undefined;
  /** Client: shared modules the bundle reads from the host registry. */
  shared: Set<string>;
}

const REQUIRE_SHIM =
  'import { createRequire as __iotaCreateRequire } from "node:module";\nconst require = __iotaCreateRequire(import.meta.url);';

interface Rules {
  platform: "node" | "browser";
  jsx: "dom" | "ssr";
  solid: "shared" | "vendor" | "forbid";
  sdkServer?: string;
  sdkClient?: string;
  builtin?: string;
}

const RULES: Record<Mode, Rules> = {
  contracts: { platform: "node", jsx: "ssr", solid: "forbid", sdkServer: "graph.impureContract", sdkClient: "graph.impureContract", builtin: "graph.impureContract" },
  server: { platform: "node", jsx: "ssr", solid: "vendor", sdkClient: "graph.clientInServer" },
  client: { platform: "browser", jsx: "dom", solid: "shared", sdkServer: "graph.serverInClient", builtin: "graph.serverInClient" },
  extract: { platform: "node", jsx: "ssr", solid: "vendor" },
  test: { platform: "node", jsx: "ssr", solid: "vendor" },
};

function isBuiltin(dir: string, spec: string): boolean {
  if (spec.startsWith("node:")) return true;
  const name = spec.split("/")[0] ?? spec;
  return BUILTINS.has(spec) && !existsSync(join(dir, "node_modules", name));
}

/** Compiles Solid JSX (and TS) to plain JS; `generate` selects DOM output (browser) or SSR output (Node). */
export async function transformJsx(esbuild: typeof Esbuild, file: string, source: string, generate: "dom" | "ssr"): Promise<string> {
  const lowered = await esbuild.transform(source, {
    loader: file.endsWith(".tsx") ? "tsx" : "jsx",
    jsx: "preserve",
    sourcefile: file,
    target: "esnext",
    tsconfigRaw: { compilerOptions: { jsx: "preserve" } },
  });
  const out = await babel.transformAsync(lowered.code, {
    filename: file,
    babelrc: false,
    configFile: false,
    sourceType: "module",
    comments: false,
    presets: [[solidPreset, { generate, hydratable: false }]],
  });
  if (out?.code == null) throw new Error(`${file}: JSX transform produced no output`);
  return out.code;
}

function toDiagnostics(dir: string, list: readonly Esbuild.Message[], severity: "error" | "warning"): Diagnostic[] {
  return list.map((m) => {
    const code = m.pluginName === "iota-solid" ? "build.jsx" : /^Could not resolve/.test(m.text) ? "build.unresolved" : "build.error";
    const file = m.location?.file ?? "package.json";
    return {
      file: file.startsWith("/") ? relPath(dir, file) : file,
      ...(m.location === null || m.location === undefined ? {} : { line: m.location.line, column: m.location.column + 1 }),
      severity,
      code,
      message: m.text,
    };
  });
}

function plugin(esbuild: typeof Esbuild, env: GraphEnv, sink: { diagnostics: Diagnostic[]; kitModules: Set<string>; shared: Set<string> }): Esbuild.Plugin {
  const rules = RULES[env.mode];
  const parents = new Map<string, { importer: string; spec: string }>();

  // Reports a graph violation at the author's file that (transitively) caused the import.
  const violate = (code: string, message: string, importer: string, spec: string): void => {
    let file = importer;
    let at = spec;
    let via = "";
    while (inNodeModules(file)) {
      const parent = parents.get(file);
      if (parent === undefined) break;
      via = via === "" ? ` (through ${relPath(env.dir, file)})` : via;
      at = parent.spec;
      file = parent.importer;
    }
    const loc = file.startsWith("/") ? locateImport(file, at) : undefined;
    sink.diagnostics.push({
      file: file.startsWith("/") ? relPath(env.dir, file) : file,
      ...(loc === undefined ? {} : { line: loc.line, column: loc.column }),
      severity: "error",
      code,
      message: message + via,
    });
  };

  return {
    name: "iota-graph",
    setup(build) {
      build.onResolve({ filter: /.*/ }, async (args) => {
        if (args.pluginData === SKIP || args.kind === "entry-point") return undefined;
        const spec = args.path;
        const importer = args.importer;

        if (env.mode === "client") {
          if (SHARED_SPECIFIERS.has(spec)) {
            sink.shared.add(spec);
            return { path: spec, namespace: "iota-shared" };
          }
          // The pure runtime has no state worth sharing (brands use the global symbol registry): bundled.
          if (spec === "@iota-uz/sdk/runtime") return { path: join(env.layout.hostDir, "sdk", "index.mjs") };
          if (spec === "@iota-uz/sdk/runtime/server") {
            violate("graph.serverInClient", `client graph imports "${spec}", which is server-only`, importer, spec);
            return { path: spec, external: true };
          }
        }

        if (SDK_RUNTIME.has(spec)) {
          if (spec === "@iota-uz/sdk/runtime/server" && rules.sdkServer !== undefined) {
            violate(rules.sdkServer, `${env.mode} graph imports "${spec}", which is server-only`, importer, spec);
          }
          if (spec === "@iota-uz/sdk/runtime/client" && rules.sdkClient !== undefined) {
            violate(rules.sdkClient, `${env.mode} graph imports "${spec}", which is client-only`, importer, spec);
          }
          return { path: spec, external: true };
        }

        if (spec.startsWith("@iota-kit/")) {
          const name = spec.split("/").slice(0, 2).join("/");
          if (env.kit.modules[name]?.["index.js"] === undefined) {
            violate("graph.unknownKit", `kit module "${name}" is not part of the developer kit this package builds against`, importer, spec);
            return { path: spec, external: true };
          }
          sink.kitModules.add(name);
          return { path: name, namespace: "iota-kit" };
        }

        if (isBuiltin(env.dir, spec)) {
          if (rules.builtin !== undefined) {
            violate(rules.builtin, `${env.mode} graph imports Node builtin "${spec}"`, importer, spec);
          }
          return { path: spec, external: true };
        }

        if (SOLID.test(spec)) {
          if (rules.solid === "forbid") {
            violate("graph.impureContract", `${env.mode} graph imports "${spec}"`, importer, spec);
            return { path: spec, external: true };
          }
          const r = await build.resolve(spec, { kind: args.kind, resolveDir: env.layout.vendor.modules, pluginData: SKIP });
          return r.errors.length > 0 ? { errors: r.errors } : { path: r.path };
        }

        const r = await build.resolve(spec, {
          importer,
          namespace: args.namespace,
          resolveDir: args.resolveDir,
          kind: args.kind,
          pluginData: SKIP,
        });
        if (r.errors.length > 0) return { errors: r.errors, warnings: r.warnings };
        if (importer.startsWith("/")) parents.set(r.path, { importer, spec });

        if (env.mode === "client" && !r.external && !inNodeModules(r.path)) {
          const rel = relPath(env.dir, r.path);
          if (rel === env.serverEntry) {
            violate("graph.serverInClient", `client graph imports the server entry "${rel}"`, importer, spec);
          } else if (env.serverOnly?.has(rel) === true) {
            violate("graph.serverInClient", `client graph imports "${rel}", which is reachable only from the server graph`, importer, spec);
          }
        }
        return { path: r.path, namespace: r.namespace, external: r.external, sideEffects: r.sideEffects, suffix: r.suffix };
      });

      build.onLoad({ filter: /.*/, namespace: "iota-shared" }, (args) => ({ contents: sharedShim(env.layout, args.path), loader: "js" }));

      build.onLoad({ filter: /.*/, namespace: "iota-kit" }, (args) => ({
        contents: env.kit.modules[args.path]?.["index.js"] ?? "",
        loader: "js",
        resolveDir: env.dir,
      }));

      build.onLoad({ filter: /\.[jt]sx$/, namespace: "file" }, async (args) => {
        if (inNodeModules(args.path)) return undefined;
        try {
          const contents = await transformJsx(esbuild, args.path, await readFile(args.path, "utf8"), rules.jsx);
          return { contents, loader: "js", resolveDir: dirname(args.path) };
        } catch (err) {
          // esbuild reports precise messages; Babel reports `loc`.
          const messages = (err as { errors?: Esbuild.Message[] }).errors;
          if (Array.isArray(messages) && messages.length > 0) {
            return { errors: messages.map((m) => ({ text: m.text, location: m.location })) };
          }
          const loc = (err as { loc?: { line: number; column: number } }).loc;
          const text = (errorMessage(err).split("\n")[0] ?? "JSX transform failed").replace(`${args.path}: `, "");
          return { errors: [{ text, location: { file: args.path, ...(loc === undefined ? {} : { line: loc.line, column: loc.column }) } }] };
        }
      });
    },
  };
}

export async function runGraph(esbuild: typeof Esbuild, env: GraphEnv): Promise<GraphResult> {
  const rules = RULES[env.mode];
  const sink = { diagnostics: [] as Diagnostic[], kitModules: new Set<string>(), shared: new Set<string>() };
  const client = env.mode === "client";
  const outdir = join(env.dir, ".iota-out");
  const options: Esbuild.BuildOptions = {
    absWorkingDir: env.dir,
    entryPoints: [{ in: env.entry, out: env.outName ?? (client ? "index" : env.mode) }],
    outdir,
    write: false,
    bundle: true,
    format: "esm",
    target: "es2022",
    platform: rules.platform,
    metafile: true,
    logLevel: "silent",
    legalComments: "none",
    charset: "utf8",
    splitting: client,
    entryNames: client ? "[name]-[hash]" : "[name]",
    chunkNames: "chunk-[hash]",
    assetNames: "asset-[hash]",
    outExtension: { ".js": client ? ".js" : ".mjs" },
    tsconfigRaw: { compilerOptions: {} },
    define: client ? { "process.env.NODE_ENV": '"production"' } : {},
    // Contracts are pure ESM (no Node shims); server and Node-side bundles may bundle CommonJS dependencies.
    ...(client || env.mode === "contracts" ? {} : { banner: { js: REQUIRE_SHIM } }),
    plugins: [plugin(esbuild, env, sink)],
  };

  const files = new Map<string, Uint8Array>();
  let metafile: Esbuild.Metafile | undefined;
  try {
    const result = await esbuild.build(options);
    metafile = result.metafile;
    for (const f of result.outputFiles ?? []) files.set(relPath(outdir, f.path), f.contents);
    sink.diagnostics.push(...toDiagnostics(env.dir, result.warnings, "warning"));
  } catch (err) {
    const failure = err as Partial<Esbuild.BuildFailure>;
    if (failure.errors === undefined) throw err;
    sink.diagnostics.push(...toDiagnostics(env.dir, failure.errors, "error"), ...toDiagnostics(env.dir, failure.warnings ?? [], "warning"));
  }

  const inputs = new Set<string>();
  let entryOutput: string | undefined;
  if (metafile !== undefined) {
    for (const input of Object.keys(metafile.inputs)) {
      if (!input.includes(":") && !inNodeModules(input)) inputs.add(input);
    }
    for (const [path, out] of Object.entries(metafile.outputs)) {
      const rel = relPath(outdir, join(env.dir, path));
      // Dynamic-import chunks also carry an entryPoint; only the package entry's output is the entry.
      if (out.entryPoint !== undefined && join(env.dir, out.entryPoint) === env.entry) entryOutput = rel;
    }
  }
  return { diagnostics: sink.diagnostics, files, metafile, inputs, kitModules: sink.kitModules, entryOutput, shared: sink.shared };
}
