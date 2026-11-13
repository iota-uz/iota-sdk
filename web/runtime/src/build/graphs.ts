// The three package graphs (contracts, server, client) plus the extraction/test bundles, built with
// esbuild. One plugin enforces the graph rules, resolves kit modules from memory, and runs the Solid
// JSX transform; violations are collected as source-located diagnostics instead of aborting the build.
import { builtinModules } from "node:module";
import { existsSync, readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { basename, dirname, join } from "node:path";

import babel from "@babel/core";
import solidPreset from "babel-preset-solid";
import type * as Esbuild from "esbuild";

import { unknownKitMessage } from "./kitdiag.ts";
import type { Layout } from "./layout.ts";
import { locateImport } from "./locate.ts";
import type { Diagnostic, KitPayload } from "./protocol.ts";
import { errorMessage, inNodeModules, relPath, sha256 } from "./util.ts";

export type Mode = "contracts" | "server" | "client" | "extract" | "test";

export const SDK_RUNTIME = new Set([
  "@iota-uz/sdk/runtime",
  "@iota-uz/sdk/runtime/server",
  "@iota-uz/sdk/runtime/client",
  "@iota-uz/sdk/runtime/kit",
  "@iota-uz/sdk/runtime/testing",
]);
const SOLID = /^solid-js(?:\/.*)?$/;
const BUILTINS = new Set(builtinModules);
const SKIP = Symbol("iota.skip");

/** The SDK's Solid UI kit. Packages import this one specifier; there are no narrower subpaths. */
export const KIT_SPECIFIER = "@iota-uz/sdk/solid-ui";

/**
 * Specifiers client bundles never carry their own copy of: the browser host publishes its instances in
 * `globalThis[Symbol.for("iota.runtime.shared")]` (client-host/shared.ts) and bundles read them through a
 * generated shim. A second Solid or runtime client would break reactivity and the RuntimeProvider context, and a
 * second UI kit would not share the host's theme.
 */
export const SHARED_SPECIFIERS: ReadonlySet<string> = new Set(["solid-js", "solid-js/web", "solid-js/store", "@iota-uz/sdk/runtime/client", KIT_SPECIFIER]);
/**
 * Shared specifiers the host provides lazily: the registry holds an async loader (`SHARED_LOADERS`), and the shim awaits it
 * (top-level await), so a host pays for the module only when a package version actually imports it.
 */
export const LAZY_SHARED: ReadonlySet<string> = new Set([KIT_SPECIFIER]);
const SHARED_REGISTRY = 'globalThis[Symbol.for("iota.runtime.shared")]';
const SHARED_LOADERS = 'globalThis[Symbol.for("iota.runtime.shared.loaders")]';

let sharedNames: Record<string, string[]> | undefined;

function sharedShim(layout: Layout, spec: string): string {
  sharedNames ??= JSON.parse(readFileSync(layout.sharedExports, "utf8")) as Record<string, string[]>;
  const names = sharedNames[spec] ?? [];
  const exports = names.length > 0 ? `export const { ${names.join(", ")} } = m;` : "";
  if (LAZY_SHARED.has(spec)) {
    return [
      `const load = ${SHARED_LOADERS}?.[${JSON.stringify(spec)}];`,
      `if (load === undefined) throw new Error(${JSON.stringify(`iota runtime: the host does not provide "${spec}" (the UI kit ships with the host, at the version of the runtime release it was built for)`)});`,
      "const m = await load();",
      exports,
    ].join("\n");
  }
  return [
    `const m = ${SHARED_REGISTRY}?.[${JSON.stringify(spec)}];`,
    `if (m === undefined) throw new Error(${JSON.stringify(`iota runtime: the host did not publish shared module "${spec}"`)});`,
    exports,
  ].join("\n");
}

/**
 * Stands in for the UI kit where a bundle is evaluated in Node (manifest extraction, package tests): the module can be
 * imported and its exports referenced, but nothing renders. Calling, constructing or reading a member of an export is inert.
 */
function kitStub(layout: Layout): string {
  sharedNames ??= JSON.parse(readFileSync(layout.sharedExports, "utf8")) as Record<string, string[]>;
  const names = sharedNames[KIT_SPECIFIER] ?? [];
  return [
    "const stub = (name) => new Proxy(function () {}, { get: (_t, key) => (key === 'name' ? name : stub(name + '.' + String(key))), apply: () => undefined, construct: () => ({}) });",
    ...names.map((n) => `export const ${n} = stub(${JSON.stringify(n)});`),
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
  /** Client: the contracts entry (relative to dir), named by the message of a server-only import. */
  contractsEntry?: string;
  /** Output base name of the entry (default: the mode). */
  outName?: string;
  /** The package namespace: scopes the class names of `*.module.css` files so packages sharing a page cannot collide. */
  namespace?: string;
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
  /** Graphs that cannot carry stylesheets (the diagnostic code), as opposed to client code and its Node evaluation. */
  css?: string;
}

const RULES: Record<Mode, Rules> = {
  contracts: { platform: "node", jsx: "ssr", solid: "forbid", sdkServer: "graph.impureContract", sdkClient: "graph.impureContract", builtin: "graph.impureContract", css: "graph.cssOutsideClient" },
  server: { platform: "node", jsx: "ssr", solid: "vendor", sdkClient: "graph.clientInServer", css: "graph.cssOutsideClient" },
  client: { platform: "browser", jsx: "dom", solid: "shared", sdkServer: "graph.serverInClient", builtin: "graph.serverInClient" },
  extract: { platform: "node", jsx: "ssr", solid: "vendor" },
  test: { platform: "node", jsx: "ssr", solid: "vendor" },
};

const CSS_FILE = /\.css$/i;
const CSS_MODULE = /\.module\.css$/i;

const GLOBAL_CSS_MESSAGE =
  "global stylesheets are not allowed in package client code: every package of an installation shares one page, so a global rule leaks into the host and into other packages. " +
  "Use Tailwind utility classes (compiled with the SDK theme) or a scoped CSS module (import styles from './name.module.css')";

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

/** Position of the first occurrence of `text` in a CSS file (`@import`, `composes: ... from`). */
function locateText(file: string, text: string): { line: number; column: number } | undefined {
  try {
    const lines = readFileSync(file, "utf8").split("\n");
    for (let i = 0; i < lines.length; i++) {
      const column = (lines[i] ?? "").indexOf(text);
      if (column >= 0) return { line: i + 1, column: column + 1 };
    }
  } catch {
    // unreadable: the diagnostic still names the file
  }
  return undefined;
}

function toDiagnostics(dir: string, list: readonly Esbuild.Message[], severity: "error" | "warning", cssFiles: ReadonlyMap<string, string>): Diagnostic[] {
  return list.map((m) => {
    // A scoped stylesheet is a virtual module (see the plugin); report its real file. esbuild only warns about malformed
    // CSS and still emits what it could read, which would ship a broken stylesheet in an immutable version: an error here.
    const virtual = m.location?.file.startsWith("iota-css:") === true ? m.location.file.slice("iota-css:".length) : undefined;
    const real = virtual === undefined ? undefined : cssFiles.get(virtual);
    const css = virtual !== undefined || /\.css$/i.test(m.location?.file ?? "");
    const code = m.pluginName === "iota-solid" ? "build.jsx" : /^Could not resolve/.test(m.text) ? "build.unresolved" : css ? "css.syntax" : "build.error";
    const file = real ?? m.location?.file ?? "package.json";
    return {
      file: file.startsWith("/") ? relPath(dir, file) : file,
      ...(m.location === null || m.location === undefined ? {} : { line: m.location.line, column: m.location.column + 1 }),
      severity: css && code === "css.syntax" ? "error" : severity,
      code,
      message: m.text,
    };
  });
}

function plugin(esbuild: typeof Esbuild, env: GraphEnv, sink: { diagnostics: Diagnostic[]; kitModules: Set<string>; shared: Set<string>; cssFiles: Map<string, string> }): Esbuild.Plugin {
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
    const loc = file.startsWith("/") ? (CSS_FILE.test(file) ? locateText(file, at) : locateImport(file, at)) : undefined;
    sink.diagnostics.push({
      file: file.startsWith("/") ? relPath(env.dir, file) : file,
      ...(loc === undefined ? {} : { line: loc.line, column: loc.column }),
      severity: "error",
      code,
      message: message + via,
    });
  };

  // An unresolved import, located at the import in the author's source: JSX files are transformed before esbuild reads them,
  // so the location esbuild reports for them is a line of the transformed output.
  const unresolved = (importer: string, spec: string, text: string, original?: Esbuild.PartialMessage): Esbuild.PartialMessage => {
    const loc = importer.startsWith("/") && !inNodeModules(importer) ? locateImport(importer, spec) : undefined;
    if (loc === undefined) return original === undefined ? { text } : { ...original, text };
    return { text, location: { file: importer, line: loc.line, column: loc.column - 1 } };
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
          if (spec === "@iota-uz/sdk/runtime/kit") return { path: join(env.layout.hostDir, "sdk", "kit.mjs") };
          if (spec === "@iota-uz/sdk/runtime/server") {
            violate("graph.serverInClient", `client graph imports "${spec}", which is server-only`, importer, spec);
            return { path: spec, external: true };
          }
        }

        if (spec.startsWith(`${KIT_SPECIFIER}/`)) {
          return { errors: [unresolved(importer, spec, `Could not resolve "${spec}": UI kit subpaths are not importable, only "${KIT_SPECIFIER}" is; import the component from it (import { Button } from "${KIT_SPECIFIER}")`)] };
        }

        if (spec === KIT_SPECIFIER && env.mode !== "client") {
          if (rules.sdkClient !== undefined) {
            violate(rules.sdkClient, `${env.mode} graph imports "${spec}", which is client-only`, importer, spec);
            return { path: spec, external: true };
          }
          return { path: spec, namespace: "iota-kit-stub" };
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
            violate("graph.unknownKit", unknownKitMessage(name, env.kit.modules), importer, spec);
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
        if (r.errors.length > 0) return { errors: r.errors.map((e) => unresolved(importer, spec, e.text, e)), warnings: r.warnings };
        if (importer.startsWith("/")) parents.set(r.path, { importer, spec });

        if (r.namespace === "file" && CSS_FILE.test(r.path) && args.kind !== "url-token") {
          // A stylesheet imported by author code, or by another stylesheet (its pluginData names the real file).
          const from = (args.pluginData as { file?: string } | undefined)?.file ?? importer;
          if (rules.css !== undefined) {
            violate(rules.css, `${env.mode} graph imports the stylesheet "${spec}"; stylesheets belong to the client graph`, from, spec);
            return { path: spec, external: true };
          }
          if (!inNodeModules(r.path) && CSS_MODULE.test(r.path)) {
            const text = readFileSync(r.path, "utf8");
            const stem = basename(r.path).replace(CSS_MODULE, "").replace(/[^A-Za-z0-9_-]/g, "_");
            // esbuild derives the class prefix from the (virtual) file name: it carries a digest of the package, path and
            // content, so two packages' `Card.module.css` (or two versions of one package on a page) never share a class.
            const digest = sha256(`${env.namespace ?? ""}\0${relPath(env.dir, r.path)}\0${text}`).slice(0, 8);
            const virtual = `${/^[A-Za-z]/.test(stem) ? stem : `_${stem}`}-${digest}.module.css`;
            sink.cssFiles.set(virtual, r.path);
            return { path: virtual, namespace: "iota-css", pluginData: { file: r.path, text } };
          }
          if (env.mode === "client") violate("graph.globalCss", `${GLOBAL_CSS_MESSAGE} (imported "${spec}")`, from, spec);
          return { path: r.path, namespace: "iota-empty" };
        }

        if (env.mode === "client" && !r.external && !inNodeModules(r.path)) {
          const rel = relPath(env.dir, r.path);
          if (rel === env.serverEntry) {
            violate("graph.serverInClient", `client graph imports the server entry "${rel}"`, importer, spec);
          } else if (env.serverOnly?.has(rel) === true) {
            const shared = env.contractsEntry ?? "the contracts entry";
            violate(
              "graph.serverInClient",
              `client graph imports "${rel}", which is reachable only from the server graph. The contracts graph (${shared}) is the pure layer client and server share: ` +
                `if "${rel}" is pure (no Node, no server SDK), export it from ${shared} or import it there, so the contracts graph reaches it; otherwise keep it out of client code`,
              importer,
              spec,
            );
          }
        }
        return { path: r.path, namespace: r.namespace, external: r.external, sideEffects: r.sideEffects, suffix: r.suffix };
      });

      build.onLoad({ filter: /.*/, namespace: "iota-css" }, (args) => {
        const data = args.pluginData as { file: string; text: string };
        return { contents: data.text, loader: "local-css", resolveDir: dirname(data.file), pluginData: data };
      });

      build.onLoad({ filter: /.*/, namespace: "iota-empty" }, () => ({ contents: "", loader: "js" }));

      build.onLoad({ filter: /.*/, namespace: "iota-kit-stub" }, () => ({ contents: kitStub(env.layout), loader: "js" }));

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
  const sink = { diagnostics: [] as Diagnostic[], kitModules: new Set<string>(), shared: new Set<string>(), cssFiles: new Map<string, string>() };
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
    // Test bundles carry a source map so a failing assertion is reported at the author's .ts line.
    ...(env.mode === "test" ? { sourcemap: "external" as const, sourcesContent: false } : {}),
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
    sink.diagnostics.push(...toDiagnostics(env.dir, result.warnings, "warning", sink.cssFiles));
  } catch (err) {
    const failure = err as Partial<Esbuild.BuildFailure>;
    if (failure.errors === undefined) throw err;
    sink.diagnostics.push(...toDiagnostics(env.dir, failure.errors, "error", sink.cssFiles), ...toDiagnostics(env.dir, failure.warnings ?? [], "warning", sink.cssFiles));
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
    // esbuild bundles the CSS of every module the entry reaches (lazy chunks included) into the entry's stylesheet and also
    // emits a per-chunk copy; only the entry's is linked, so the copies are dropped from the artifact.
    const entryMeta = Object.entries(metafile.outputs).find(([, out]) => out.entryPoint !== undefined && join(env.dir, out.entryPoint) === env.entry)?.[1];
    if (client && entryMeta !== undefined) {
      const keep = entryMeta.cssBundle === undefined ? undefined : relPath(outdir, join(env.dir, entryMeta.cssBundle));
      for (const name of [...files.keys()]) if (name.endsWith(".css") && name !== keep) files.delete(name);
    }
  }
  return { diagnostics: sink.diagnostics, files, metafile, inputs, kitModules: sink.kitModules, entryOutput, shared: sink.shared };
}
