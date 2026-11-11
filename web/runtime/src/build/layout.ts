// Where the worker finds its assets. The worker lives at <root>/build/<name>.mjs next to the SDK
// build output (<root>/*.js, <root>/*.d.ts), the host loader (<root>/host) and vendored typings
// (<root>/build/vendor).
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
  };
}

/** esbuild is a native dependency and stays outside the bundle (NODE_PATH or a sibling node_modules). */
export function loadEsbuild(moduleURL: string): typeof Esbuild {
  return createRequire(moduleURL)("esbuild") as typeof Esbuild;
}
