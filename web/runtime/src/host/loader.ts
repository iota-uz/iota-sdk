// Module resolution hooks (node:module register) that make artifacts self-sufficient:
// `server.mjs` / `contracts.mjs` keep `@iota-uz/sdk/runtime*` as bare imports and the worker
// resolves them to the SDK modules it was built with (./sdk/*.mjs), so every artifact and the
// host share one SDK instance per worker.
import type { InitializeHook, ResolveHook } from "node:module";

let specifiers: Readonly<Record<string, string>> = {};
let sdkDir = "";
let vendorDir = "";

const SOLID = /^solid-js(?:\/|$)/;

export const initialize: InitializeHook<{ sdkDir: string; vendorDir?: string }> = (data) => {
  sdkDir = data.sdkDir;
  vendorDir = data.vendorDir ?? "";
  const at = (file: string) => new URL(file, data.sdkDir).href;
  specifiers = {
    "@iota-uz/sdk/runtime": at("index.mjs"),
    "@iota-uz/sdk/runtime/server": at("server.mjs"),
    "@iota-uz/sdk/runtime/client": at("client.mjs"),
    "@iota-uz/sdk/runtime/kit": at("kit.mjs"),
    "@iota-uz/sdk/runtime/testing": at("testing.mjs"),
  };
};

export const resolve: ResolveHook = (specifier, context, next) => {
  const url = specifiers[specifier];
  if (url !== undefined) return { url, shortCircuit: true };
  // The runtime client (client-host hooks) imports Solid; the worker has no node_modules next to the SDK
  // modules, so Solid resolves from the build worker's vendored copy (manifest extraction of client graphs).
  if (vendorDir !== "" && SOLID.test(specifier) && context.parentURL?.startsWith(sdkDir) === true) {
    return next(specifier, { ...context, parentURL: new URL("resolve.mjs", vendorDir).href });
  }
  return next(specifier, context);
};
