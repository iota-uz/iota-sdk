import { createHash } from "node:crypto";
import { readdirSync, readFileSync } from "node:fs";
import { join, relative, sep } from "node:path";

import type { Diagnostic } from "./protocol.ts";

export const sha256 = (data: string | Uint8Array): string => createHash("sha256").update(data).digest("hex");

export const toPosix = (path: string): string => (sep === "/" ? path : path.split(sep).join("/"));

export const relPath = (dir: string, abs: string): string => toPosix(relative(dir, abs));

export function isInside(dir: string, abs: string): boolean {
  const r = relative(dir, abs);
  return r !== "" && !r.startsWith("..") && !r.startsWith(sep);
}

export const inNodeModules = (path: string): boolean => toPosix(path).split("/").includes("node_modules");

/** All files below `dir` as sorted relative posix paths; `skip` prunes directories/files by relative path. */
export function listFiles(dir: string, skip: (rel: string) => boolean = () => false): string[] {
  const out: string[] = [];
  const walk = (current: string): void => {
    for (const entry of readdirSync(current, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      const abs = join(current, entry.name);
      const rel = relPath(dir, abs);
      if (skip(rel)) continue;
      if (entry.isDirectory()) walk(abs);
      else if (entry.isFile()) out.push(rel);
    }
  };
  walk(dir);
  return out;
}

export const readText = (path: string): string => readFileSync(path, "utf8");

export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

const severityRank = { error: 0, warning: 1 } as const;

/** Stable order, duplicates dropped. */
export function normalizeDiagnostics(list: readonly Diagnostic[]): Diagnostic[] {
  const seen = new Set<string>();
  const out: Diagnostic[] = [];
  for (const d of list) {
    const key = `${d.file}\u0000${d.line ?? 0}\u0000${d.column ?? 0}\u0000${d.code}\u0000${d.message}`;
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(d);
  }
  return out.sort(
    (a, b) =>
      severityRank[a.severity] - severityRank[b.severity] ||
      (a.file < b.file ? -1 : a.file > b.file ? 1 : 0) ||
      (a.line ?? 0) - (b.line ?? 0) ||
      (a.column ?? 0) - (b.column ?? 0) ||
      (a.code < b.code ? -1 : a.code > b.code ? 1 : 0),
  );
}

export const hasErrors = (list: readonly Diagnostic[]): boolean => list.some((d) => d.severity === "error");
