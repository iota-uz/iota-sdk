// Where the worker finds its assets. The worker lives at <root>/build/<name>.mjs next to the SDK
// build output (<root>/*.js, <root>/*.d.ts), the host loader (<root>/host) and vendored typings
// (<root>/build/vendor).
import { existsSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import type * as Esbuild from "esbuild";

export interface Layout {
  /** SDK build output: .d.ts for typechecking. */
  sdkTypes: string;
  /** Host directory holding loader.mjs, register.mjs and sdk/*.mjs. */
  hostDir: string;
  /** Hook module that maps `@iota-uz/sdk/runtime*` to hostDir/sdk. */
  loaderURL: string;
  vendor: { lib: string; nodeTypes: string; solid: string; /** Directory whose node_modules holds solid-js (esbuild resolveDir). */ modules: string };
  workerFile: string;
  /**
   * Directory holding the SDK theme the package stylesheet is compiled with: `tailwind.config.js` (copied from the
   * repository's canonical token source at build time) and Tailwind's `theme.css` / `utilities.css`.
   */
  theme: string;
  /** Typings of `@iota-uz/sdk/solid-ui`, when this build ships them (next to the runtime typings or one level up). */
  solidUITypes: string | undefined;
  /** Generated: named exports of the modules the browser host shares (see graphs.ts). */
  sharedExports: string;
}

export function resolveLayout(moduleURL: string): Layout {
  const workerFile = fileURLToPath(moduleURL);
  const buildDir = dirname(workerFile);
  const root = dirname(buildDir);
  const hostDir = join(root, "host");
  return {
    sdkTypes: root,
    hostDir,
    loaderURL: pathToFileURL(join(hostDir, "register.mjs")).href,
    vendor: {
      lib: join(buildDir, "vendor", "lib"),
      nodeTypes: join(buildDir, "vendor", "node-types"),
      solid: join(buildDir, "vendor", "node_modules", "solid-js"),
      modules: join(buildDir, "vendor"),
    },
    workerFile,
    theme: join(buildDir, "theme"),
    // Emitted next to the runtime typings by `build-worker --emit-sdk`; in the assembled @iota-uz/sdk package it is the sibling dist/solid-ui.
    solidUITypes: [join(root, "solid-ui"), join(root, "..", "solid-ui")].find((d) => existsSync(join(d, "index.d.ts"))),
    sharedExports: join(buildDir, "shared-exports.json"),
  };
}

/** esbuild is a native dependency and stays outside the bundle (NODE_PATH or a sibling node_modules). */
export function loadEsbuild(moduleURL: string): typeof Esbuild {
  return createRequire(moduleURL)("esbuild") as typeof Esbuild;
}
