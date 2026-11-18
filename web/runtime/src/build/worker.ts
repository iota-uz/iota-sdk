// Build worker entry: `node worker.mjs` reads one JSON request on stdin and writes one JSON response
// on stdout (check | build | test | kit). Not the invocation pool: no long-lived state.
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

import { generateKit } from "./kit.ts";
import { loadEsbuild, resolveLayout } from "./layout.ts";
import { runBuild } from "./pipeline.ts";
import type { BuildResponse, PackageRequest, Request, Response } from "./protocol.ts";
import { runTests } from "./testrun.ts";
import { errorMessage, hasErrors } from "./util.ts";

async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of process.stdin) chunks.push(chunk as Buffer);
  return Buffer.concat(chunks).toString("utf8");
}

async function handle(req: Request): Promise<Response> {
  const layout = resolveLayout(import.meta.url);
  if (req.op === "kit") return generateKit(layout, req.sources);
  const esbuild = loadEsbuild(import.meta.url);
  if (req.op === "test") return runTests(esbuild, layout, req);
  const outcome = await runBuild(esbuild, layout, req);
  const response: BuildResponse = { diagnostics: outcome.diagnostics, artifacts: [] };
  if (outcome.manifest !== undefined) response.manifest = outcome.manifest;
  if (req.op === "build" && !hasErrors(outcome.diagnostics)) {
    const root = join((req as PackageRequest).outDir, "artifacts");
    for (const [path, bytes] of outcome.files) {
      const target = join(root, path);
      mkdirSync(dirname(target), { recursive: true });
      writeFileSync(target, bytes);
    }
    response.artifacts = outcome.artifacts;
  }
  return response;
}

// Dependencies may print; stdout carries the protocol only.
const log = (...args: unknown[]): void => void process.stderr.write(args.map(String).join(" ") + "\n");
console.log = console.info = console.debug = console.warn = console.error = log;

try {
  const response = await handle(JSON.parse(await readStdin()) as Request);
  process.stdout.write(JSON.stringify(response) + "\n", () => process.exit(0));
} catch (err) {
  process.stderr.write(`build worker: ${errorMessage(err)}\n${err instanceof Error ? (err.stack ?? "") : ""}\n`);
  process.exit(1);
}
