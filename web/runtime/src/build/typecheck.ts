// TypeScript typecheck of a package tree against the bundled SDK typings and kit modules.
import { posix } from "node:path";

import ts from "typescript";

import { explainMissingKit } from "./kitdiag.ts";
import type { Layout } from "./layout.ts";
import type { Diagnostic, KitPayload } from "./protocol.ts";
import { inNodeModules, isInside, listFiles, relPath } from "./util.ts";
import { VirtualFS } from "./vfs.ts";

const SOURCE = /\.(?:[cm]?tsx?)$/;

export function compilerOptions(dir: string): ts.CompilerOptions {
  return {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    strict: true,
    jsx: ts.JsxEmit.Preserve,
    jsxImportSource: "solid-js",
    lib: ["lib.es2022.d.ts", "lib.dom.d.ts", "lib.dom.iterable.d.ts"],
    types: ["node"],
    typeRoots: [posix.join(dir, "node_modules/@types")],
    allowImportingTsExtensions: true,
    resolveJsonModule: true,
    esModuleInterop: true,
    forceConsistentCasingInFileNames: true,
    skipLibCheck: true,
    noEmit: true,
  };
}

export function createHost(vfs: VirtualFS, layout: Layout, cache = new Map<string, ts.SourceFile | undefined>()): ts.CompilerHost {
  return {
    getSourceFile(fileName, languageVersionOrOptions) {
      if (cache.has(fileName)) return cache.get(fileName);
      const text = vfs.readFile(fileName);
      const file = text === undefined ? undefined : ts.createSourceFile(fileName, text, languageVersionOrOptions, false);
      cache.set(fileName, file);
      return file;
    },
    getDefaultLibFileName: () => posix.join(layout.vendor.lib, "lib.d.ts"),
    getDefaultLibLocation: () => layout.vendor.lib,
    writeFile: () => undefined,
    getCurrentDirectory: () => vfs.dir,
    getCanonicalFileName: (f) => f,
    useCaseSensitiveFileNames: () => true,
    getNewLine: () => "\n",
    fileExists: (f) => vfs.fileExists(f),
    readFile: (f) => vfs.readFile(f),
    directoryExists: (d) => vfs.directoryExists(d),
    getDirectories: (d) => vfs.getDirectories(d),
    realpath: (p) => p,
  };
}

export function sourceFiles(dir: string): string[] {
  return listFiles(dir, (rel) => rel.split("/").some((s) => s === "node_modules" || s.startsWith(".")))
    .filter((f) => SOURCE.test(f))
    .map((f) => posix.join(dir, f));
}

/**
 * Arity errors cannot carry a type-level message, so the ones authors hit are explained here:
 * `tx.start(W, input)` outside a subscription or workflow lacks its required start key.
 */
function hintFor(d: ts.Diagnostic): string | undefined {
  if (d.code !== 2554 || d.file === undefined || d.start === undefined) return undefined;
  const at = d.start;
  let hint: string | undefined;
  const visit = (node: ts.Node): void => {
    if (hint !== undefined || at < node.getFullStart() || at > node.getEnd()) return;
    ts.forEachChild(node, visit);
    if (
      hint === undefined &&
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      node.expression.name.text === "start" &&
      node.arguments.length === 2
    ) {
      hint = "hint: tx.start needs a start key outside subscriptions and workflows: tx.start(Workflow, input, { key })";
    }
  };
  visit(d.file);
  return hint;
}

export function toDiagnostic(dir: string, d: ts.Diagnostic): Diagnostic | undefined {
  const hint = hintFor(d);
  const message = ts.flattenDiagnosticMessageText(d.messageText, "\n") + (hint === undefined ? "" : `\n${hint}`);
  const severity = d.category === ts.DiagnosticCategory.Error ? "error" : d.category === ts.DiagnosticCategory.Warning ? "warning" : undefined;
  if (severity === undefined) return undefined;
  if (d.file === undefined || d.start === undefined) {
    return { file: "package.json", severity, code: `ts.${d.code}`, message };
  }
  if (!isInside(dir, d.file.fileName) || inNodeModules(d.file.fileName)) return undefined;
  const pos = d.file.getLineAndCharacterOfPosition(d.start);
  return { file: relPath(dir, d.file.fileName), line: pos.line + 1, column: pos.character + 1, severity, code: `ts.${d.code}`, message };
}

export function typecheck(dir: string, layout: Layout, kit: KitPayload): Diagnostic[] {
  const vfs = new VirtualFS(dir, layout, kit);
  const options = compilerOptions(dir);
  const program = ts.createProgram({ rootNames: sourceFiles(dir), options, host: createHost(vfs, layout) });
  const out: Diagnostic[] = [];
  const codes: number[] = [];
  for (const d of ts.getPreEmitDiagnostics(program)) {
    const diagnostic = toDiagnostic(dir, d);
    if (diagnostic === undefined) continue;
    out.push(diagnostic);
    codes.push(d.code);
  }
  return explainMissingKit(out, codes, kit.modules);
}
