#!/usr/bin/env node
// Builds the Node build worker (docs/extension-runtime.md §4) into <out>/build:
//   worker.mjs, extract.mjs, test-child.mjs   bundled with esbuild; typescript and babel are inlined,
//                                             esbuild itself (native binary) is resolved at run time
//   vendor/                                   typings and Solid runtime the worker serves to packages
// and, with --emit-sdk, the SDK itself (<out>/*.js + *.d.ts) and the host worker (<out>/host), so a
// bare output directory is a complete build runtime. `pnpm build` runs this after tsc and build-host.
// Usage: node scripts/build-worker.mjs [--emit-sdk] [out]   (default out: web/runtime/dist)
import { spawnSync } from "node:child_process";
import { cp, mkdir, readdir, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import { build } from "esbuild";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const require = createRequire(import.meta.url);
const args = process.argv.slice(2);
const emitSdk = args.includes("--emit-sdk");
const out = resolve(args.find((a) => !a.startsWith("--")) ?? join(root, "dist"));
const buildDir = join(out, "build");

if (emitSdk) {
  const tsc = require.resolve("typescript/bin/tsc");
  const r = spawnSync(process.execPath, [tsc, "-p", join(root, "tsconfig.build.json"), "--outDir", out], { cwd: root, stdio: "inherit" });
  if (r.status !== 0) process.exit(r.status ?? 1);
  const h = spawnSync(process.execPath, ["--no-warnings", join(root, "scripts/build-host.mjs"), join(out, "host")], { cwd: root, stdio: "inherit" });
  if (h.status !== 0) process.exit(h.status ?? 1);
}

await rm(buildDir, { recursive: true, force: true });
await mkdir(buildDir, { recursive: true });

// typescript and @babel/core are CommonJS: give the bundle the CJS globals they expect.
const banner = [
  'import { createRequire as __iotaCreateRequire } from "node:module";',
  'import { fileURLToPath as __iotaFileURLToPath } from "node:url";',
  'import { dirname as __iotaDirname } from "node:path";',
  "const require = __iotaCreateRequire(import.meta.url);",
  "const __filename = __iotaFileURLToPath(import.meta.url);",
  "const __dirname = __iotaDirname(__filename);",
].join("\n");

await build({
  entryPoints: {
    worker: join(root, "src/build/worker.ts"),
    extract: join(root, "src/build/extract.ts"),
    "test-child": join(root, "src/build/test-child.ts"),
  },
  outdir: buildDir,
  outExtension: { ".js": ".mjs" },
  bundle: true,
  platform: "node",
  format: "esm",
  target: "node22",
  external: ["esbuild", "@babel/preset-typescript"],
  banner: { js: banner },
  legalComments: "none",
  logLevel: "warning",
  tsconfig: join(root, "src/build/tsconfig.json"),
});

// Vendored assets: TypeScript libs, Node typings, Solid (types for typecheck, server build for extraction).
const vendor = join(buildDir, "vendor");
const pkgDir = (name) => dirname(require.resolve(`${name}/package.json`));
const tsLib = join(pkgDir("typescript"), "lib");
await mkdir(join(vendor, "lib"), { recursive: true });
for (const f of await readdir(tsLib)) {
  if (/^lib\.(es\d+|esnext|dom|decorators)[a-z0-9.]*\.d\.ts$/.test(f) || f === "lib.d.ts") await cp(join(tsLib, f), join(vendor, "lib", f));
}
const nodeTypes = join(root, "node_modules/@types/node");
await cp(nodeTypes, join(vendor, "node-types"), { recursive: true, dereference: true, filter: (src) => !src.endsWith(".md") && !src.endsWith("LICENSE") });
const solid = pkgDir("solid-js");
const solidOut = join(vendor, "node_modules", "solid-js");
await mkdir(solidOut, { recursive: true });
for (const entry of ["package.json", "jsx-runtime.d.ts", "dist", "types", "web", "store"]) {
  await cp(join(solid, entry), join(solidOut, entry), {
    recursive: true,
    filter: (src) => !/\.(map|md)$/.test(src) && !/[\\/]dist[\\/]dev\./.test(src) && !/[\\/]jsx-dev-runtime/.test(src),
  }).catch(() => undefined);
}
// Solid's own dependencies (resolved from Solid's location, as a package manager would).
for (const dep of ["seroval", "seroval-plugins", "csstype"]) {
  const depDir = join(dirname(solid), dep);
  await cp(depDir, join(vendor, "node_modules", dep), { recursive: true, filter: (src) => !/\.(map|md)$/.test(src) });
}
// Named exports of the modules the browser host shares with package bundles (client-host/shared.ts):
// the build worker generates one shim per specifier that re-exports from the host's registry.
const sharedExports = {};
const exportsOf = async (name, options) => {
  const result = await build({ bundle: true, write: false, format: "esm", platform: "browser", metafile: true, logLevel: "silent", outdir: "out", ...options });
  const output = Object.values(result.metafile.outputs)[0];
  sharedExports[name] = (output?.exports ?? []).filter((e) => e !== "default").sort();
};
await exportsOf("solid-js", { stdin: { contents: 'export * from "solid-js"', resolveDir: join(vendor) } });
await exportsOf("solid-js/web", { stdin: { contents: 'export * from "solid-js/web"', resolveDir: join(vendor) } });
await exportsOf("solid-js/store", { stdin: { contents: 'export * from "solid-js/store"', resolveDir: join(vendor) } });
await exportsOf("@iota-uz/sdk/runtime/client", {
  stdin: {
    contents: ["client.ts", "client-host/hooks.ts", "client-host/transport.ts"].map((f) => `export * from ${JSON.stringify(join(root, "src", f))};`).join("\n"),
    resolveDir: root,
    loader: "ts",
  },
  external: ["solid-js", "solid-js/web", "solid-js/store"],
});
await writeFile(join(buildDir, "shared-exports.json"), JSON.stringify(sharedExports, null, 2) + "\n");
await writeFile(join(buildDir, "README.txt"), "Generated by scripts/build-worker.mjs. See docs/extension-runtime.md section 4.\n");
console.log(`built build worker into ${buildDir}`);
