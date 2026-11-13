// Package stylesheets. A package's client graph styles itself with two mechanisms, both compiled here or by the
// client graph and both part of the immutable version artifact:
//
//   - Tailwind utilities over the SDK design tokens: `client/tailwind.css`, compiled from the classes the client sources
//     use, with the host's own token mapping (tailwind.config.js, copied into the worker at build time) and Tailwind's
//     theme and utilities layers only. No preflight and no base: the host page already has them.
//   - `*.module.css` imports, scoped by esbuild's local-css loader (graphs.ts) and emitted next to the client chunks.
//
// Tailwind runs in-process through its pure-JS compiler (no CLI subprocess, no native scanner): the candidates are the
// string-like tokens of the client graph's own source files, and the compiler drops everything that is not a utility.
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { compile } from "tailwindcss";
import type * as Esbuild from "esbuild";

import type { Layout } from "./layout.ts";
import type { Diagnostic } from "./protocol.ts";
import { errorMessage } from "./util.ts";

/** Output path of the utilities sheet inside the artifact. */
export const TAILWIND_OUTPUT = "client/tailwind.css";

const SCRIPT = /\.(?:[cm]?[jt]sx?)$/;
/** A candidate longer than this is a string, not a class. */
const MAX_CANDIDATE = 160;
// Browsers Tailwind v4 itself supports; the sheet is minified and nesting lowered for them.
const TARGET = ["chrome111", "safari16.4", "firefox128"];

/** True for the files whose text can carry class names. */
export const isScript = (file: string): boolean => SCRIPT.test(file);

/**
 * Class-name candidates of some source text: every whitespace-separated token of a string or template literal body
 * (so `"before:content-['*']"` stays whole), plus every token of the raw text split on quotes (class lists in
 * attributes, object keys, concatenations). Over-generation is harmless; the compiler ignores what is not a utility.
 */
export function candidatesOf(source: string, into: Set<string> = new Set()): Set<string> {
  const add = (token: string): void => {
    if (token.length > 0 && token.length <= MAX_CANDIDATE) into.add(token);
  };
  for (const literal of source.matchAll(/"((?:[^"\\\n]|\\.)*)"|'((?:[^'\\\n]|\\.)*)'|`((?:[^`\\]|\\.)*)`/g)) {
    const body = literal[1] ?? literal[2] ?? literal[3] ?? "";
    for (const token of body.split(/\s+/)) add(token);
  }
  for (const token of source.split(/[\s"'`]+/)) add(token);
  return into;
}

function loadConfig(layout: Layout): object {
  const path = join(layout.theme, "tailwind.config.js");
  // The canonical config is a CommonJS module with no requires; evaluating its text keeps the worker free of a module loader.
  const mod: { exports: object } = { exports: {} };
  new Function("module", "exports", readFileSync(path, "utf8"))(mod, mod.exports);
  return mod.exports;
}

export interface Stylesheet {
  /** The minified CSS; undefined when the sources use no utility. */
  css: string | undefined;
  diagnostics: Diagnostic[];
}

/**
 * Compiles the utilities the given client sources use, with the SDK theme. `files` are absolute paths of the
 * client graph's own files (node_modules and generated code excluded); non-script files are skipped.
 */
export async function tailwindStylesheet(esbuild: typeof Esbuild, layout: Layout, files: Iterable<string>): Promise<Stylesheet> {
  const candidates = new Set<string>();
  for (const file of files) {
    if (!isScript(file)) continue;
    try {
      candidatesOf(readFileSync(file, "utf8"), candidates);
    } catch (err) {
      return { css: undefined, diagnostics: [{ file, severity: "error", code: "css.tailwind", message: `cannot read ${file} for Tailwind: ${errorMessage(err)}` }] };
    }
  }
  if (candidates.size === 0) return { css: undefined, diagnostics: [] };
  try {
    const config = loadConfig(layout);
    const compiler = await compile(
      [
        '@config "tailwind.config.js";',
        "@layer theme, base, components, utilities;",
        '@import "tailwindcss/theme.css" layer(theme);',
        '@import "tailwindcss/utilities.css" layer(utilities);',
      ].join("\n"),
      {
        base: layout.theme,
        async loadStylesheet(id, base) {
          if (id !== "tailwindcss/theme.css" && id !== "tailwindcss/utilities.css") {
            throw new Error(`stylesheet "${id}" is not available to package styles (only the Tailwind theme and utilities layers are)`);
          }
          return { path: id, base, content: readFileSync(join(layout.theme, id.slice("tailwindcss/".length)), "utf8") };
        },
        async loadModule(id, base) {
          if (id !== "tailwind.config.js") throw new Error(`module "${id}" is not available to package styles`);
          return { path: join(layout.theme, id), base, module: config as never }; // the canonical config: Tailwind's UserConfig shape
        },
      },
    );
    const built = compiler.build([...candidates].sort());
    // No class of the sources is a utility: the compiler restates only the layer order, and there is nothing to ship.
    if (!built.includes("@layer utilities {")) return { css: undefined, diagnostics: [] };
    const out = await esbuild.transform(built, { loader: "css", minify: true, target: TARGET, legalComments: "none" });
    return { css: out.code, diagnostics: [] };
  } catch (err) {
    return { css: undefined, diagnostics: [{ file: "package.json", severity: "error", code: "css.tailwind", message: `Tailwind could not compile the package styles: ${errorMessage(err)}` }] };
  }
}

/**
 * The stylesheets of a version in link order: the Tailwind utilities first, then the scoped CSS modules. (Utilities live in a
 * cascade layer and module rules are unlayered, so a module rule wins over a utility whatever the order; the fixed order
 * only keeps the manifest, and so the artifact, deterministic.)
 */
export function stylesheetPaths(paths: Iterable<string>): string[] {
  const css = [...paths].filter((p) => p.startsWith("client/") && p.endsWith(".css")).sort();
  return [...css.filter((p) => p === TAILWIND_OUTPUT), ...css.filter((p) => p !== TAILWIND_OUTPUT)];
}
