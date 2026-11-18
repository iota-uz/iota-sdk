// The generated modules a client bundle reads for each shared specifier (see SHARED_SPECIFIERS in graphs.ts): the host
// publishes its instances in a registry (client-host/shared.ts) and the bundle's import is replaced by these shims.
import { readFileSync } from "node:fs";

import type { Layout } from "./layout.ts";

/** The SDK's Solid UI kit. Packages import this one specifier; there are no narrower subpaths. */
export const KIT_SPECIFIER = "@iota-uz/sdk/solid-ui";

/**
 * Shared specifiers the host provides lazily: the registry holds an async loader (`SHARED_LOADERS`), and the shim awaits it
 * (top-level await), so a host pays for the module only when a package version actually imports it.
 */
export const LAZY_SHARED: ReadonlySet<string> = new Set([KIT_SPECIFIER]);

const SHARED_REGISTRY = 'globalThis[Symbol.for("iota.runtime.shared")]';
const SHARED_LOADERS = 'globalThis[Symbol.for("iota.runtime.shared.loaders")]';

let sharedNames: Record<string, string[]> | undefined;

/** The export names of a shared specifier (`shared-exports.json`, generated from the host\'s sources by build-worker.mjs). */
export function sharedExportNames(layout: Layout, spec: string): string[] {
  sharedNames ??= JSON.parse(readFileSync(layout.sharedExports, "utf8")) as Record<string, string[]>;
  return sharedNames[spec] ?? [];
}

/** The shared runtime client: its `helpHref` is the one export a bundle gets bound to its own package rather than the host's page. */
const RUNTIME_CLIENT = "@iota-uz/sdk/runtime/client";

/**
 * The module a client bundle reads for a shared specifier. `namespace` is the package being built: the shim of the runtime
 * client exports a `helpHref` bound to it, so a slot or cell of package B rendered on package A's page (or on a native page)
 * links to B's documentation; an explicit `{ namespace }` argument still wins.
 */
export function sharedShim(layout: Layout, spec: string, namespace?: string): string {
  const bound = spec === RUNTIME_CLIENT && namespace !== undefined;
  const names = sharedExportNames(layout, spec).filter((n) => !(bound && n === "helpHref"));
  const exports = [
    names.length > 0 ? `export const { ${names.join(", ")} } = m;` : "",
    bound ? `export const helpHref = (path, options = {}) => m.helpHref(path, { ...options, namespace: options.namespace ?? ${JSON.stringify(namespace)} });` : "",
  ]
    .filter((line) => line !== "")
    .join("\n");
  if (LAZY_SHARED.has(spec)) {
    return [
      `const load = ${SHARED_LOADERS}?.[${JSON.stringify(spec)}];`,
      `if (load === undefined) throw new Error(${JSON.stringify(`iota runtime: the host does not provide "${spec}" (the UI kit ships with the host, at the version of the runtime release it was built for)`)});`,
      "const m = await load();",
      exports,
    ].join("\n");
  }
  return [
    `const m = ${SHARED_REGISTRY}?.[${JSON.stringify(spec)}];`,
    `if (m === undefined) throw new Error(${JSON.stringify(`iota runtime: the host did not publish shared module "${spec}"`)});`,
    exports,
  ].join("\n");
}

