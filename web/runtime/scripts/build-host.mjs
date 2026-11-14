#!/usr/bin/env node
// Builds the Node host worker: strips TypeScript types (esbuild transform,
// per file, no bundling) from src/host/*.ts and from the SDK
// modules the worker imports at runtime, and writes
//   <out>/worker.mjs, <out>/*.mjs        host modules
//   <out>/sdk/*.mjs                      SDK modules (entity.ts, schema.ts, ...)
// Default output: web/runtime/dist/host. Sources must use erasable TypeScript
// syntax only (no enums, namespaces or parameter properties).
import { mkdir, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { transform } from "esbuild";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const src = join(root, "src");
const hostSrc = join(src, "host");
const out = process.argv[2] ? resolve(process.argv[2]) : join(root, "dist", "host");

const isSource = (f) => f.endsWith(".ts") && !f.endsWith(".d.ts") && !f.endsWith(".test.ts") && !f.endsWith(".check.ts");
const hostEntries = (await readdir(hostSrc)).filter(isSource).map((f) => join(hostSrc, f));
// Public SDK modules artifacts import as `@iota-uz/sdk/runtime*` (resolved by host/loader.ts).
const publicEntries = ["index", "server", "client", "kit", "testing"].map((name) => join(src, `${name}.ts`));
const entries = [...hostEntries, ...publicEntries];

// Output path of a source file: host modules flat, everything else under sdk/.
const outPath = (file) =>
  file.startsWith(hostSrc + "/") ? join(out, relative(hostSrc, file)).replace(/\.ts$/, ".mjs") : join(out, "sdk", relative(src, file)).replace(/\.ts$/, ".mjs");

const specifier = /(\bfrom\s+|\bimport\s*\(\s*|\bimport\s+)(["'])(\.{1,2}\/[^"']+\.ts)\2/g;

await rm(out, { recursive: true, force: true });
const done = new Set();
const queue = [...entries];
while (queue.length > 0) {
  const file = queue.pop();
  if (done.has(file)) continue;
  done.add(file);
  const { code: stripped } = await transform(await readFile(file, "utf8"), { loader: "ts", format: "esm", target: "node20", sourcefile: file });
  const target = outPath(file);
  const rewritten = stripped.replace(specifier, (_all, lead, quote, spec) => {
    const dep = resolve(dirname(file), spec);
    queue.push(dep);
    let rel = relative(dirname(target), outPath(dep));
    if (!rel.startsWith(".")) rel = "./" + rel;
    return `${lead}${quote}${rel}${quote}`;
  });
  await mkdir(dirname(target), { recursive: true });
  await writeFile(target, rewritten);
}
console.log(`built ${done.size} modules into ${out}`);
