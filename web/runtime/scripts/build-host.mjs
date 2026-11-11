#!/usr/bin/env node
// Builds the Node host worker: strips TypeScript types from src/host/*.ts and
// writes dist/host/*.mjs (entry: dist/host/worker.mjs). Uses only Node
// built-ins, so Go tests and CI can build the worker without installing
// dependencies. Sources must use erasable TypeScript syntax only.
import { mkdir, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { stripTypeScriptTypes } from "node:module";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const src = join(root, "src", "host");
const out = process.argv[2] ? resolve(process.argv[2]) : join(root, "dist", "host");

const sources = (await readdir(src)).filter((f) => f.endsWith(".ts") && !f.endsWith(".d.ts") && !f.endsWith(".test.ts"));
await rm(out, { recursive: true, force: true });
await mkdir(out, { recursive: true });

for (const file of sources) {
  const code = stripTypeScriptTypes(await readFile(join(src, file), "utf8"), { mode: "strip" });
  const rewritten = code.replace(/(from\s+|import\s*\(\s*)(["'])(\.{1,2}\/[^"']+)\.ts\2/g, "$1$2$3.mjs$2");
  await writeFile(join(out, file.replace(/\.ts$/, ".mjs")), rewritten);
}
console.log(`built ${sources.length} host modules into ${out}`);
