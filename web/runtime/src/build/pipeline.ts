// check/build pipeline: package.json -> typecheck -> contracts/server/client graphs -> artifacts ->
// manifest extraction (in an isolated child) -> client chunk metadata.
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";

import type * as Esbuild from "esbuild";

import type { CapabilitiesJSON, ManifestJSON } from "../model.ts";
import { collectDocs } from "./docs.ts";
import { stylesheetPaths, tailwindStylesheet, TAILWIND_OUTPUT } from "./css.ts";
import { extractManifest } from "./extraction.ts";
import { runGraph, type GraphResult } from "./graphs.ts";
import type { Layout } from "./layout.ts";
import type { BuildResponse, Diagnostic, PackageRequest } from "./protocol.ts";
import { typecheck } from "./typecheck.ts";
import { errorMessage, hasErrors, normalizeDiagnostics, relPath } from "./util.ts";

interface IotaSection {
  namespace?: unknown;
  contracts?: unknown;
  server?: unknown;
  client?: unknown;
  capabilities?: unknown;
}

export interface PackageInfo {
  namespace: string;
  contracts?: string;
  server?: string;
  client?: string;
  capabilities?: CapabilitiesJSON;
}

const NAMESPACE = /^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$/;

function strings(value: unknown): string[] | undefined {
  return Array.isArray(value) && value.every((v) => typeof v === "string") ? (value as string[]) : undefined;
}

/** Reads package.json `iota`; entries are absolute paths of existing files. */
export function readPackage(dir: string): { info?: PackageInfo; diagnostics: Diagnostic[] } {
  const bad = (message: string, code = "package.invalid"): { diagnostics: Diagnostic[] } => ({
    diagnostics: [{ file: "package.json", severity: "error", code, message }],
  });
  const path = join(dir, "package.json");
  if (!existsSync(path)) return bad("package.json is missing", "package.missing");
  let json: { iota?: IotaSection };
  try {
    json = JSON.parse(readFileSync(path, "utf8")) as { iota?: IotaSection };
  } catch (err) {
    return bad(`package.json is not valid JSON: ${errorMessage(err)}`);
  }
  const iota = json.iota;
  if (iota === undefined || typeof iota !== "object") return bad('package.json has no "iota" section');
  if (typeof iota.namespace !== "string" || !NAMESPACE.test(iota.namespace)) {
    return bad('iota.namespace must be a dotted lowercase namespace such as "acme.insurance"');
  }
  const diagnostics: Diagnostic[] = [];
  const info: PackageInfo = { namespace: iota.namespace };
  for (const graph of ["contracts", "server", "client"] as const) {
    const value = iota[graph];
    if (value === undefined) continue;
    if (typeof value !== "string") {
      diagnostics.push({ file: "package.json", severity: "error", code: "package.invalid", message: `iota.${graph} must be a path` });
      continue;
    }
    const abs = resolve(dir, value);
    if (relPath(dir, abs).startsWith("..") || !existsSync(abs)) {
      diagnostics.push({ file: "package.json", severity: "error", code: "package.missingEntry", message: `iota.${graph} entry "${value}" does not exist in the package` });
      continue;
    }
    info[graph] = abs;
  }
  if (info.contracts === undefined && info.server === undefined && info.client === undefined && diagnostics.length === 0) {
    diagnostics.push({ file: "package.json", severity: "error", code: "package.invalid", message: "iota declares no contracts, server or client entry" });
  }
  const caps = iota.capabilities;
  if (caps !== undefined && typeof caps === "object" && caps !== null) {
    const c = caps as { network?: unknown; secrets?: unknown };
    const capabilities: CapabilitiesJSON = {};
    const network = strings(c.network);
    const secrets = strings(c.secrets);
    if (network !== undefined) capabilities.network = network;
    if (secrets !== undefined) capabilities.secrets = secrets;
    info.capabilities = capabilities;
  }
  return diagnostics.length > 0 ? { diagnostics } : { info, diagnostics };
}

const kitNamespace = (module: string): string => module.slice("@iota-kit/".length);

export interface BuildOutcome extends BuildResponse {
  /** Artifact bytes by path (the worker writes them below outDir/artifacts). */
  files: Map<string, Uint8Array>;
}

const EXTRACT_TIMEOUT_MS = 60_000;

export async function runBuild(esbuild: typeof Esbuild, layout: Layout, req: PackageRequest): Promise<BuildOutcome> {
  const diagnostics: Diagnostic[] = [];
  const files = new Map<string, Uint8Array>();
  const done = (manifest?: ManifestJSON): BuildOutcome => ({
    diagnostics: normalizeDiagnostics(diagnostics),
    ...(manifest === undefined ? {} : { manifest }),
    artifacts: [...files.keys()].sort(),
    files,
  });

  const pkg = readPackage(req.dir);
  diagnostics.push(...pkg.diagnostics);
  if (pkg.info === undefined) return done();
  const info = pkg.info;

  try {
    diagnostics.push(...typecheck(req.dir, layout, req.kit));
  } catch (err) {
    diagnostics.push({ file: "package.json", severity: "error", code: "typecheck.failed", message: errorMessage(err) });
  }

  const env = { dir: req.dir, layout, kit: req.kit, namespace: info.namespace };
  const graphs: Record<"contracts" | "server" | "client", GraphResult | undefined> = { contracts: undefined, server: undefined, client: undefined };
  if (info.contracts !== undefined) graphs.contracts = await runGraph(esbuild, { ...env, mode: "contracts", entry: info.contracts });
  if (info.server !== undefined) graphs.server = await runGraph(esbuild, { ...env, mode: "server", entry: info.server });
  if (info.client !== undefined) {
    const serverOnly = new Set<string>();
    for (const f of graphs.server?.inputs ?? []) if (!(graphs.contracts?.inputs.has(f) ?? false)) serverOnly.add(f);
    graphs.client = await runGraph(esbuild, {
      ...env,
      mode: "client",
      entry: info.client,
      serverOnly,
      ...(info.server === undefined ? {} : { serverEntry: relPath(req.dir, info.server) }),
    });
  }
  for (const g of Object.values(graphs)) if (g !== undefined) diagnostics.push(...g.diagnostics);
  // Tailwind utilities for the classes the client graph's own sources use (css.ts).
  let utilities: string | undefined;
  if (graphs.client !== undefined && !hasErrors(graphs.client.diagnostics)) {
    const sheet = await tailwindStylesheet(esbuild, layout, [...graphs.client.inputs].map((f) => join(req.dir, f)));
    diagnostics.push(...sheet.diagnostics);
    utilities = sheet.css;
  }
  if (hasErrors(diagnostics.filter((d) => !d.code.startsWith("ts.")))) return done();

  // Artifacts.
  const clientEntry = graphs.client?.entryOutput;
  if (graphs.contracts !== undefined) addOutput(files, "contracts.mjs", graphs.contracts.files.get("contracts.mjs"));
  if (graphs.server !== undefined) addOutput(files, "server.mjs", graphs.server.files.get("server.mjs"));
  if (graphs.client !== undefined && clientEntry !== undefined) {
    for (const [path, bytes] of graphs.client.files) files.set(`client/${path}`, bytes);
    if (utilities !== undefined) files.set(TAILWIND_OUTPUT, Buffer.from(utilities));
  }

  // Manifest extraction imports the real server/contracts artifacts; the client graph is rebuilt for
  // Node (SSR JSX, vendored Solid) because DOM output cannot be evaluated outside a browser.
  const scratch = mkdtempSync(join(req.outDir, "extract-"));
  try {
    const bundle = (name: string, bytes: Uint8Array | undefined): string | undefined => {
      if (bytes === undefined) return undefined;
      const path = join(scratch, name);
      mkdirSync(dirname(path), { recursive: true });
      writeFileSync(path, bytes);
      return path;
    };
    let client: string | undefined;
    if (info.client !== undefined) {
      const x = await runGraph(esbuild, { ...env, mode: "extract", entry: info.client, outName: "client-extract" });
      diagnostics.push(...x.diagnostics.filter((d) => d.severity === "error"));
      client = bundle("client-extract.mjs", x.files.get("client-extract.mjs"));
    }
    const extraction = await extractManifest({
      dir: req.dir,
      layout,
      namespace: info.namespace,
      ...optional("contracts", bundle("contracts.mjs", files.get("contracts.mjs"))),
      ...optional("server", bundle("server.mjs", files.get("server.mjs"))),
      ...optional("client", client),
      sources: [info.contracts, info.server, info.client].filter((p): p is string => p !== undefined),
      ...(info.capabilities === undefined ? {} : { capabilities: info.capabilities }),
      dependencies: dependencies(req, graphs),
      timeoutMs: EXTRACT_TIMEOUT_MS,
    });
    diagnostics.push(...extraction.diagnostics);
    let manifest = extraction.manifest;
    if (manifest !== undefined && graphs.client !== undefined && clientEntry !== undefined) {
      manifest = withChunks(manifest, `client/${clientEntry}`);
      files.set("client/manifest.json", Buffer.from(clientManifest(manifest, graphs.client, clientEntry, stylesheetPaths(files.keys()))));
    }
    if (manifest !== undefined) {
      // End-user documentation: docs/<locale>/<path>.md joins the manifest and the artifact.
      const documentation = collectDocs({ dir: req.dir, namespace: info.namespace, manifest });
      diagnostics.push(...documentation.diagnostics);
      for (const [path, bytes] of documentation.files) files.set(path, bytes);
      if (documentation.docs.length > 0) manifest = { ...manifest, docs: documentation.docs };
    }
    return done(manifest);
  } catch (err) {
    diagnostics.push({ file: "package.json", severity: "error", code: "manifest.failed", message: errorMessage(err) });
    return done();
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
}

function optional<K extends string>(key: K, value: string | undefined): Partial<Record<K, string>> {
  return value === undefined ? {} : ({ [key]: value } as Record<K, string>);
}

function addOutput(files: Map<string, Uint8Array>, name: string, bytes: Uint8Array | undefined): void {
  if (bytes !== undefined) files.set(name, bytes);
}

/** Foreign namespaces the package actually imports, each pinned to the kit snapshot it resolved against. */
function dependencies(req: PackageRequest, graphs: Record<string, GraphResult | undefined>): Record<string, string> {
  const used = new Set<string>();
  for (const g of Object.values(graphs)) for (const m of g?.kitModules ?? []) used.add(m);
  return Object.fromEntries([...used].sort().map((m) => [kitNamespace(m), req.kit.hash]));
}

/** Client resources live in the entry chunk; the host imports it and takes `export`. */
function withChunks(manifest: ManifestJSON, chunk: string): ManifestJSON {
  return {
    ...manifest,
    resources: manifest.resources.map((r) =>
      r.export !== undefined && r.kind !== "navigation" ? { ...r, chunk } : r,
    ),
  };
}

/** `client/manifest.json`: the entry chunk, lazy chunks, stylesheets and the shared modules the bundle reads from the host registry. */
function clientManifest(manifest: ManifestJSON, client: GraphResult, entry: string, css: string[]): string {
  const all = [...client.files.keys()].sort();
  const exported = manifest.resources.filter((r) => r.export !== undefined).map((r) => r.export as string).sort();
  const doc = {
    schema: "iota.client/1",
    namespace: manifest.namespace,
    entry: `client/${entry}`,
    chunks: all.filter((f) => f.endsWith(".js") && f !== entry).map((f) => `client/${f}`),
    css,
    shared: [...client.shared].sort(),
    exports: [...new Set(exported)],
  };
  return JSON.stringify(doc, null, 2) + "\n";
}
