// Diagnostics for `@iota-kit/*` imports that do not resolve. TypeScript reports them as TS2307 and then fails
// on everything typed through them (implicit any, missing properties); the graph rules report them as
// `graph.unknownKit`. Both paths use the message below, which names the commands that install a kit module.
import type { Diagnostic, KitModules } from "./protocol.ts";

/** Codes TypeScript reports as a consequence of an unresolved import (the imported bindings are `any`). */
const FOLLOW_ON = new Set([7005, 7006, 7031, 7034, 2339, 2551, 18046]);

const KIT_IMPORT = /'(@iota-kit\/[^'/]+)(?:\/[^']*)?'/;

export function unknownKitMessage(name: string, kit: KitModules): string {
  const installed = Object.keys(kit).sort();
  const state = installed.length === 0 ? "no developer kit is installed" : `installed: ${installed.join(", ")}`;
  return (
    `kit module "${name}" is not part of the developer kit this package builds against (${state}). ` +
    "Run `iota-ext kit generate` (the SDK natives core and crm, offline), `iota-ext kit generate --catalog <file>` or `iota-ext kit pull --server <url>` (an application's kit)"
  );
}

/**
 * Replaces TS2307 for a missing kit module by the kit diagnostic and drops the follow-on type errors it causes
 * (the rest of the package cannot be judged until the module resolves); a warning says how many were hidden.
 * `diagnostics` are the converted TypeScript diagnostics of the package, `codes` their TypeScript codes in the same order.
 */
export function explainMissingKit(diagnostics: readonly Diagnostic[], codes: readonly number[], kit: KitModules): Diagnostic[] {
  const missing = new Set<string>();
  const out: Diagnostic[] = [];
  let hidden = 0;
  let firstFile: string | undefined;
  const converted: { d: Diagnostic; code: number }[] = diagnostics.map((d, i) => ({ d, code: codes[i] ?? 0 }));
  for (const { d, code } of converted) {
    if (code !== 2307) continue;
    const name = KIT_IMPORT.exec(d.message)?.[1];
    if (name !== undefined && kit[name]?.["index.js"] === undefined) {
      missing.add(name);
      firstFile ??= d.file;
    }
  }
  if (missing.size === 0) return [...diagnostics];
  for (const { d, code } of converted) {
    const name = code === 2307 ? KIT_IMPORT.exec(d.message)?.[1] : undefined;
    if (name !== undefined && missing.has(name)) {
      out.push({ ...d, code: "graph.unknownKit", message: unknownKitMessage(name, kit) });
    } else if (FOLLOW_ON.has(code) && d.severity === "error") {
      hidden++;
    } else {
      out.push(d);
    }
  }
  if (hidden > 0) {
    out.push({
      file: firstFile ?? "package.json",
      severity: "warning",
      code: "kit.followOn",
      message: `${hidden} follow-on type error${hidden === 1 ? "" : "s"} (implicit any, missing properties) are hidden until ${[...missing].sort().join(", ")} resolve${missing.size === 1 ? "s" : ""}`,
    });
  }
  return out;
}
