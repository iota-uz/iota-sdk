// `test` operation: bundles every *.test.ts of the package (SDK external, kit bundled) and runs them in
// an isolated child with outbound network blocked; see test-child.ts.
import { spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

import type * as Esbuild from "esbuild";

import { runGraph } from "./graphs.ts";
import type { Layout } from "./layout.ts";
import type { Diagnostic, PackageRequest, TestCaseResult, TestResponse } from "./protocol.ts";
import { errorMessage, listFiles, normalizeDiagnostics } from "./util.ts";

const TEST_FILE = /\.test\.(?:[cm]?ts|[cm]?js)$/;
const CASE_TIMEOUT_MS = 10_000;
const RUN_TIMEOUT_MS = 120_000;

interface ChildReport {
  cases: TestCaseResult[];
}

function runChild(layout: Layout, cwd: string, files: { file: string; path: string }[]): Promise<ChildReport> {
  const script = join(dirname(layout.workerFile), "test-child.mjs");
  const config = { loader: layout.loaderURL, files, caseTimeoutMs: CASE_TIMEOUT_MS };
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, ["--no-warnings", script, JSON.stringify(config)], {
      cwd,
      env: { PATH: process.env.PATH ?? "", NODE_ENV: "test", TZ: "UTC" },
      stdio: ["ignore", "pipe", "pipe"],
    });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (b: Buffer) => (stdout += b.toString("utf8")));
    child.stderr.on("data", (b: Buffer) => (stderr += b.toString("utf8")));
    const timer = setTimeout(() => child.kill("SIGKILL"), RUN_TIMEOUT_MS);
    child.on("error", reject);
    child.on("close", (code) => {
      clearTimeout(timer);
      try {
        resolve(JSON.parse(stdout.trim().split("\n").pop() ?? "") as ChildReport);
      } catch {
        reject(new Error(`test runner exited with ${String(code)}: ${(stderr || stdout).slice(0, 2000)}`));
      }
    });
  });
}

export async function runTests(esbuild: typeof Esbuild, layout: Layout, req: PackageRequest): Promise<TestResponse> {
  const diagnostics: Diagnostic[] = [];
  const cases: TestCaseResult[] = [];
  const found = listFiles(req.dir, (rel) => rel.split("/").some((s) => s === "node_modules" || s.startsWith("."))).filter((f) => TEST_FILE.test(f));
  const scratch = mkdtempSync(join(req.outDir, "tests-"));
  try {
    const bundles: { file: string; path: string }[] = [];
    for (const [i, file] of found.entries()) {
      const out = `test-${i}`;
      const graph = await runGraph(esbuild, { mode: "test", dir: req.dir, layout, kit: req.kit, entry: join(req.dir, file), outName: out });
      const errors = graph.diagnostics.filter((d) => d.severity === "error");
      const bytes = graph.files.get(`${out}.mjs`);
      if (errors.length > 0 || bytes === undefined) {
        diagnostics.push(...errors);
        cases.push({ name: file, file, passed: false, message: errors.map((d) => `${d.file}:${d.line ?? 0}: ${d.message}`).join("\n") || "bundling failed", durationMs: 0 });
        continue;
      }
      const path = join(scratch, `${out}.mjs`);
      mkdirSync(dirname(path), { recursive: true });
      writeFileSync(path, bytes);
      bundles.push({ file, path });
    }
    if (bundles.length > 0) {
      try {
        cases.push(...(await runChild(layout, req.dir, bundles)).cases);
      } catch (err) {
        diagnostics.push({ file: "package.json", severity: "error", code: "test.runner", message: errorMessage(err) });
      }
    }
  } finally {
    rmSync(scratch, { recursive: true, force: true });
  }
  return { diagnostics: normalizeDiagnostics(diagnostics), cases };
}
