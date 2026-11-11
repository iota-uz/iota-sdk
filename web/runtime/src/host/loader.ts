// Module resolution hooks (node:module register) that make artifacts self-sufficient:
// `server.mjs` / `contracts.mjs` keep `@iota-uz/sdk/runtime*` as bare imports and the worker
// resolves them to the SDK modules it was built with (./sdk/*.mjs), so every artifact and the
// host share one SDK instance per worker.
import type { InitializeHook, ResolveHook } from "node:module";

let specifiers: Readonly<Record<string, string>> = {};

export const initialize: InitializeHook<{ sdkDir: string }> = (data) => {
  const at = (file: string) => new URL(file, data.sdkDir).href;
  specifiers = {
    "@iota-uz/sdk/runtime": at("index.mjs"),
    "@iota-uz/sdk/runtime/server": at("server.mjs"),
    "@iota-uz/sdk/runtime/client": at("client.mjs"),
    "@iota-uz/sdk/runtime/testing": at("testing.mjs"),
  };
};

export const resolve: ResolveHook = (specifier, context, next) => {
  const url = specifiers[specifier];
  return url === undefined ? next(specifier, context) : { url, shortCircuit: true };
};
