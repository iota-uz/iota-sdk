// Parent side of manifest extraction: spawns the isolated child and maps its outcome to diagnostics.
import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";
import { dirname, join } from "node:path";

import type { CapabilitiesJSON, ManifestJSON } from "../model.ts";
import type { Layout } from "./layout.ts";
import { locateExport, locateString } from "./locate.ts";
import type { Diagnostic } from "./protocol.ts";
import { inNodeModules, relPath } from "./util.ts";

export interface ExtractionInput {
  dir: string;
  layout: Layout;
  namespace: string;
  /** Absolute paths of the bundles to import. */
  contracts?: string;
  server?: string;
  client?: string;
  /** Source entry files, searched for the declaration of an offending export. */
  sources: string[];
  capabilities?: CapabilitiesJSON;
  dependencies?: Record<string, string>;
  timeoutMs: number;
}

export interface ExtractionResult {
  manifest?: ManifestJSON;
  diagnostics: Diagnostic[];
}

interface ChildOutcome {
  ok: boolean;
  kind?: "manifest" | "import";
  manifest?: ManifestJSON;
  problems?: string[];
  graph?: string;
  message?: string;
}

function runChild(input: ExtractionInput): Promise<ChildOutcome> {
  const script = join(dirname(input.layout.workerFile), "extract.mjs");
  const config = {
    loader: input.layout.loaderURL,
    namespace: input.namespace,
    contracts: input.contracts,
    server: input.server,
    client: input.client,
    capabilities: input.capabilities,
    dependencies: input.dependencies,
  };
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ["--no-warnings", script, JSON.stringify(config)], {
      cwd: input.dir,
      env: { PATH: process.env.PATH ?? "", NODE_ENV: "production", TZ: "UTC" },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (b: Buffer) => (stdout += b.toString("utf8")));
    child.stderr.on("data", (b: Buffer) => (stderr += b.toString("utf8")));
    const timer = setTimeout(() => child.kill("SIGKILL"), input.timeoutMs);
    child.on("error", reject);
    child.on("close", (code) => {
      clearTimeout(timer);
      const line = stdout.trim().split("\n").pop() ?? "";
      try {
        resolve(JSON.parse(line) as ChildOutcome);
      } catch {
        reject(new Error(`manifest extraction exited with ${String(code)}: ${(stderr || stdout).slice(0, 2000)}`));
      }
    });
  });
}

const PROBLEM_CODES: readonly (readonly [RegExp, string])[] = [
  [/is already declared by/, "manifest.duplicateId"],
  [/is outside namespace/, "manifest.outsideNamespace"],
  [/invalid id|invalid namespace/, "manifest.invalidId"],
  [/which is not exported by the contracts graph/, "manifest.undeclaredReference"],
  [/is already implemented by/, "manifest.duplicateImplementation"],
  [/already has templates/, "manifest.duplicateTemplates"],
  [/belongs to the client graph|must be declared and exported from the contracts graph|server value exported|contract exported from the server graph/, "manifest.wrongGraph"],
  [/cannot implement|guard cannot target|invalid cron|endpoint path/, "manifest.invalidDeclaration"],
];

/** `contracts export "Dup": id "x" is already declared by ...` -> where the offending export is declared. */
function locateProblem(input: ExtractionInput, problem: string): Diagnostic {
  const message = problem.trim();
  const code = PROBLEM_CODES.find(([re]) => re.test(message))?.[1] ?? "manifest.invalid";
  const sources = allSources(input);
  const at = (file: string, loc: { line: number; column: number }): Diagnostic => ({
    file: relPath(input.dir, file), line: loc.line, column: loc.column, severity: "error", code, message,
  });
  // Every export named in the message; the later declaration in source order is the duplicate.
  const named = [...message.matchAll(/export "([^"]+)"/g)].map((m) => m[1] ?? "");
  const found: { file: string; order: number; loc: { line: number; column: number } }[] = [];
  for (const name of named) {
    for (const [order, file] of sources.entries()) {
      const loc = locateExport(file, name);
      if (loc !== undefined) {
        found.push({ file, order, loc });
        break;
      }
    }
  }
  const later = found.sort((a, b) => a.order - b.order || a.loc.line - b.loc.line || a.loc.column - b.loc.column).pop();
  if (later !== undefined) return at(later.file, later.loc);
  const id = /id "([^"]+)"/.exec(message)?.[1];
  if (id !== undefined) {
    for (const file of sources) {
      const loc = locateString(file, id, /already declared/.test(message) ? 1 : 0) ?? locateString(file, id);
      if (loc !== undefined) return at(file, loc);
    }
  }
  return { file: "package.json", severity: "error", code, message };
}

function allSources(input: ExtractionInput): string[] {
  const files = new Set<string>(input.sources);
  const walk = (dir: string): void => {
    for (const e of readdirSync(dir, { withFileTypes: true })) {
      const abs = join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name !== "node_modules" && !e.name.startsWith(".")) walk(abs);
      } else if (/\.tsx?$/.test(e.name) && !inNodeModules(abs)) files.add(abs);
    }
  };
  walk(input.dir);
  return [...files];
}

export async function extractManifest(input: ExtractionInput): Promise<ExtractionResult> {
  const outcome = await runChild(input);
  if (outcome.ok && outcome.manifest !== undefined) return { manifest: outcome.manifest, diagnostics: [] };
  if (outcome.kind === "manifest") return { diagnostics: (outcome.problems ?? []).map((p) => locateProblem(input, p)) };
  return {
    diagnostics: [
      {
        file: "package.json",
        severity: "error",
        code: "manifest.import",
        message: `the ${outcome.graph ?? "package"} graph cannot be evaluated: ${(outcome.message ?? "").split("\n").slice(0, 6).join("\n")}`,
      },
    ],
  };
}
