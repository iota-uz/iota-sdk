// Package documentation: `docs/<locale>/<path>.md` in the package tree becomes the manifest's `docs` and the
// files `docs/<locale>/<path>.md` of the artifact (front matter removed). Every rule here is mirrored by
// `manifest.Manifest.Validate` / `manifest.ValidateDocFiles` in Go, which a hand-made artifact does not pass
// through the build for. Diagnostics point at the author's file, line and column.
import { existsSync, lstatSync, readdirSync, readFileSync } from "node:fs";
import { join, posix } from "node:path";

import type { DocJSON, ManifestJSON } from "../model.ts";
import type { Diagnostic } from "./protocol.ts";
import { relPath } from "./util.ts";

export const DOCS_DIR = "docs";
export const DEFAULT_DOC_LOCALE = "en";

const MAX_DOCS = 200;
const MAX_DOC_BYTES = 256 << 10;
const MAX_DOCS_BYTES = 4 << 20;
const MAX_PATH_BYTES = 120;
const MAX_SEGMENTS = 5;
const MAX_TITLE = 120;
const MAX_AUDIENCE = 60;
const MAX_ORDER = 10_000;

const SEGMENT = /^[a-z0-9][a-z0-9_-]*$/;
const LOCALE = /^[a-z]{2,3}(-[a-z0-9]{2,8}){0,2}$/;
const PERMISSION_ID = /^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$/;
const PERMISSION_NATIVE = /^[A-Z][A-Za-z0-9]*\.[A-Z][A-Za-z0-9]*$/;
const SCHEME = /^[a-zA-Z][a-zA-Z0-9+.-]*:/;
const SAFE_SCHEMES = new Set(["http:", "https:", "mailto:", "tel:"]);
const FRONT_KEYS = ["title", "order", "audience", "permission"] as const;

export interface DocsResult {
  docs: DocJSON[];
  /** Artifact files: `docs/<locale>/<path>.md` -> body. */
  files: Map<string, Uint8Array>;
  diagnostics: Diagnostic[];
}

interface Parsed {
  file: string;
  locale: string;
  path: string;
  meta: { title?: string; order?: number; audience?: string; permission?: string };
  /** 1-based line of each front matter key. */
  lines: Record<string, number>;
  body: string;
  /** 1-based line of the first body line in the source file. */
  bodyLine: number;
}

/** One plain text line: no control characters, no angle brackets (mirror of manifest.PlainText). */
export function isPlainText(s: string, max: number): boolean {
  if (s === "" || s.trim() !== s || [...s].length > max) return false;
  return !/[\p{Cc}<>\u2028\u2029]/u.test(s);
}

export function isDocPath(p: string): boolean {
  if (p === "" || p.length > MAX_PATH_BYTES) return false;
  const segments = p.split("/");
  return segments.length <= MAX_SEGMENTS && segments.every((s) => SEGMENT.test(s));
}

function humanize(path: string): string {
  const last = path.slice(path.lastIndexOf("/") + 1).replace(/[-_]+/g, " ").trim();
  return last === "" ? path : last.charAt(0).toUpperCase() + last.slice(1);
}

/** Splits flat `key: value` front matter. Returns the number of source lines it occupies (0 when there is none). */
function frontMatter(lines: string[], diag: (d: Omit<Diagnostic, "file" | "severity">) => void): { meta: Parsed["meta"]; at: Record<string, number>; end: number } {
  const meta: Parsed["meta"] = {};
  const at: Record<string, number> = {};
  if (lines[0]?.trimEnd() !== "---") return { meta, at, end: 0 };
  let close = -1;
  for (let i = 1; i < lines.length; i++) {
    if (lines[i]!.trimEnd() === "---") {
      close = i;
      break;
    }
  }
  if (close < 0) {
    diag({ line: 1, column: 1, code: "docs.frontMatter", message: "front matter is not closed: add a line containing only --- after it" });
    return { meta, at, end: lines.length };
  }
  for (let i = 1; i < close; i++) {
    const raw = lines[i]!;
    const text = raw.trim();
    if (text === "" || text.startsWith("#")) continue;
    const m = /^([A-Za-z_][\w-]*)\s*:(.*)$/.exec(raw);
    if (m === null || /^\s/.test(raw)) {
      diag({ line: i + 1, column: 1, code: "docs.frontMatter", message: `unsupported front matter line "${text}": use flat "key: value" lines (${FRONT_KEYS.join(", ")})` });
      continue;
    }
    const key = m[1]!;
    if (!(FRONT_KEYS as readonly string[]).includes(key)) {
      diag({ line: i + 1, column: 1, code: "docs.frontMatter", message: `unknown front matter key "${key}" (one of ${FRONT_KEYS.join(", ")})` });
      continue;
    }
    if (key in at) {
      diag({ line: i + 1, column: 1, code: "docs.frontMatter", message: `front matter key "${key}" is set twice` });
      continue;
    }
    const value = scalar(m[2]!.trim());
    if (value === undefined) {
      diag({ line: i + 1, column: raw.indexOf(":") + 2, code: "docs.frontMatter", message: `the value of "${key}" is not a valid string (quote it with "…" when it contains # or starts with a quote)` });
      continue;
    }
    at[key] = i + 1;
    if (key === "order") {
      if (!/^-?\d+$/.test(value) || Math.abs(Number(value)) > MAX_ORDER) {
        diag({ line: i + 1, column: raw.indexOf(":") + 2, code: "docs.frontMatter", message: `"order" must be an integer between -${MAX_ORDER} and ${MAX_ORDER}` });
        continue;
      }
      meta.order = Number(value);
    } else {
      meta[key as "title" | "audience" | "permission"] = value;
    }
  }
  return { meta, at, end: close + 1 };
}

/** A YAML-flavoured scalar: "double", 'single' or plain (a trailing ` # comment` is dropped). */
function scalar(raw: string): string | undefined {
  if (raw.startsWith('"')) {
    const m = /^("(?:[^"\\]|\\.)*")\s*(#.*)?$/.exec(raw);
    if (m === null) return undefined;
    try {
      return JSON.parse(m[1]!) as string;
    } catch {
      return undefined;
    }
  }
  if (raw.startsWith("'")) {
    const m = /^'((?:[^']|'')*)'\s*(#.*)?$/.exec(raw);
    return m === null ? undefined : m[1]!.replace(/''/g, "'");
  }
  return raw.replace(/\s+#.*$/, "").trim();
}

interface Link {
  dest: string;
  image: boolean;
  line: number;
  column: number;
}

/** Links and images of a markdown body outside code. Lines are 1-based in the source file. */
export function scanLinks(body: string, firstLine: number): Link[] {
  const out: Link[] = [];
  let fence: string | undefined;
  body.split("\n").forEach((raw, i) => {
    const line = firstLine + i;
    const f = /^ {0,3}(`{3,}|~{3,})/.exec(raw);
    if (fence !== undefined) {
      if (f !== null && f[1]![0] === fence[0] && f[1]!.length >= fence.length && raw.trim() === f[1]) fence = undefined;
      return;
    }
    if (f !== null) {
      fence = f[1]!;
      return;
    }
    // Code spans hide what looks like a link; blanking them keeps the columns.
    const text = raw.replace(/(`+)[^`]*?\1/g, (m) => " ".repeat(m.length));
    for (const m of text.matchAll(/(!?)\[(?:[^\[\]\\]|\\.)*\]\(\s*(<[^>]*>|[^\s)]*)/g)) {
      const dest = m[2]!.startsWith("<") ? m[2]!.slice(1, -1) : m[2]!;
      out.push({ dest, image: m[1] === "!", line, column: (m.index ?? 0) + 1 });
    }
    const def = /^ {0,3}\[[^\]]+\]:\s*(<[^>]*>|\S+)/.exec(text);
    if (def !== null) out.push({ dest: def[1]!.startsWith("<") ? def[1]!.slice(1, -1) : def[1]!, image: false, line, column: text.indexOf(def[1]!) + 1 });
  });
  return out;
}

/** First ATX heading outside code, as plain text (no inline markup). */
function firstHeading(body: string): string | undefined {
  let fence: string | undefined;
  for (const raw of body.split("\n")) {
    const f = /^ {0,3}(`{3,}|~{3,})/.exec(raw);
    if (fence !== undefined) {
      if (f !== null && raw.trim() === f[1]) fence = undefined;
      continue;
    }
    if (f !== null) {
      fence = f[1]!;
      continue;
    }
    const m = /^ {0,3}#{1,6}[ \t]+(.*?)[ \t]*#*[ \t]*$/.exec(raw);
    if (m !== null) return m[1];
  }
  return undefined;
}

interface Inputs {
  dir: string;
  namespace: string;
  manifest: ManifestJSON;
}

/** Walks `docs/` strictly: only `docs/<locale>/…/<name>.md` regular files are allowed. */
function walk(dir: string, diagnostics: Diagnostic[]): string[] {
  const root = join(dir, DOCS_DIR);
  if (!existsSync(root)) return [];
  const out: string[] = [];
  const visit = (current: string): void => {
    for (const entry of readdirSync(current, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
      const abs = join(current, entry.name);
      const rel = relPath(dir, abs);
      if (lstatSync(abs).isSymbolicLink()) {
        diagnostics.push({ file: rel, severity: "error", code: "docs.symlink", message: "documentation may not contain symbolic links" });
      } else if (entry.isDirectory()) visit(abs);
      else if (entry.isFile()) out.push(rel);
    }
  };
  visit(root);
  return out;
}

/** Validates and collects the documentation of the package at `dir`. */
export function collectDocs(input: Inputs): DocsResult {
  const diagnostics: Diagnostic[] = [];
  const files = new Map<string, Uint8Array>();
  const err = (file: string, code: string, message: string, line?: number, column?: number): void => {
    diagnostics.push({ file, ...(line === undefined ? {} : { line }), ...(column === undefined ? {} : { column }), severity: "error", code, message });
  };

  const parsed: Parsed[] = [];
  let total = 0;
  for (const rel of walk(input.dir, diagnostics)) {
    const inside = rel.slice(DOCS_DIR.length + 1);
    const segments = inside.split("/");
    if (!rel.endsWith(".md")) {
      err(rel, "docs.unsupportedFile", `only markdown (.md) files are allowed under ${DOCS_DIR}/ (images and other files are not supported in package documentation)`);
      continue;
    }
    if (segments.length < 2) {
      err(rel, "docs.noLocale", `documents live in a locale directory: ${DOCS_DIR}/${DEFAULT_DOC_LOCALE}/${inside}`);
      continue;
    }
    const locale = segments[0]!;
    const path = segments.slice(1).join("/").replace(/\.md$/, "");
    if (!LOCALE.test(locale)) {
      err(rel, "docs.badLocale", `"${locale}" is not a locale directory name (lowercase language tag such as en, ru, uz or pt-br)`);
      continue;
    }
    if (!isDocPath(path)) {
      err(rel, "docs.badPath", `invalid document path "${path}": up to ${MAX_SEGMENTS} segments of lowercase letters, digits, - and _`);
      continue;
    }
    const bytes = readFileSync(join(input.dir, rel));
    if (bytes.length > MAX_DOC_BYTES) {
      err(rel, "docs.tooLarge", `the document is ${bytes.length} bytes, the limit is ${MAX_DOC_BYTES}`);
      continue;
    }
    let text: string;
    try {
      text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
    } catch {
      err(rel, "docs.encoding", "the document is not valid UTF-8");
      continue;
    }
    if (text.includes("\u0000")) {
      err(rel, "docs.encoding", "the document contains a NUL character");
      continue;
    }
    total += bytes.length;
    const lines = text.replace(/^﻿/, "").replace(/\r\n?/g, "\n").split("\n");
    const fm = frontMatter(lines, (d) => err(rel, d.code, d.message, d.line, d.column));
    const rest = lines.slice(fm.end);
    const lead = rest.findIndex((l) => l.trim() !== "");
    const skipped = lead < 0 ? rest.length : lead;
    parsed.push({ file: rel, locale, path, meta: fm.meta, lines: fm.at, body: rest.slice(skipped).join("\n"), bodyLine: fm.end + skipped + 1 });
  }
  if (parsed.length > MAX_DOCS) err(`${DOCS_DIR}`, "docs.tooMany", `${parsed.length} documents, the limit is ${MAX_DOCS}`);
  if (total > MAX_DOCS_BYTES) err(`${DOCS_DIR}`, "docs.tooLarge", `the documentation is ${total} bytes, the limit is ${MAX_DOCS_BYTES}`);

  const defaults = new Set(parsed.filter((p) => p.locale === DEFAULT_DOC_LOCALE).map((p) => p.path));
  const declared = new Set(input.manifest.resources.filter((r) => r.kind === "permission").map((r) => r.id));
  const routes = new Map(input.manifest.resources.filter((r) => (r.kind === "route" || r.kind === "page") && typeof r.path === "string").map((r) => [r.id, r.path as string]));
  const permissionOf = new Map<string, { permission: string; file: string }>();
  const docs: DocJSON[] = [];

  for (const p of parsed) {
    const at = (key: string): number => p.lines[key] ?? 1;
    if (!defaults.has(p.path)) {
      err(p.file, "docs.missingDefault", `there is no ${DOCS_DIR}/${DEFAULT_DOC_LOCALE}/${p.path}.md: every document exists in ${DEFAULT_DOC_LOCALE}, which readers without a translation get`, 1, 1);
    }
    const title = p.meta.title ?? firstHeading(p.body) ?? humanize(p.path);
    if (!isPlainText(title, MAX_TITLE)) {
      err(p.file, "docs.title", `invalid title "${title}": one line of plain text, 1 to ${MAX_TITLE} characters, no < or > (set "title" in the front matter)`, at("title"), 1);
    }
    const audience = p.meta.audience;
    if (audience !== undefined && audience !== "" && !isPlainText(audience, MAX_AUDIENCE)) {
      err(p.file, "docs.audience", `invalid audience "${audience}": one line of plain text, up to ${MAX_AUDIENCE} characters, no < or >`, at("audience"), 1);
    }
    const permission = p.meta.permission ?? "";
    if (p.meta.permission !== undefined) {
      if (permission === "" || permission.length > 120 || !(PERMISSION_ID.test(permission) || PERMISSION_NATIVE.test(permission))) {
        err(p.file, "docs.badPermission", `"${permission}" is not a permission ID (a declared one such as ${input.namespace}.manage, or a native one such as Client.Read)`, at("permission"), 1);
      } else if (permission.startsWith(`${input.namespace}.`) && !declared.has(permission)) {
        err(p.file, "docs.unknownPermission", `permission "${permission}" is not declared by the package${declared.size > 0 ? ` (declared: ${[...declared].sort().join(", ")})` : ""}`, at("permission"), 1);
      }
    }
    const prior = permissionOf.get(p.path);
    if (prior !== undefined && prior.permission !== permission) {
      err(p.file, "docs.permissionMismatch", `the permission of ${p.path} differs from ${prior.file} ("${permission}" and "${prior.permission}"): all translations of a document share one`, at("permission"), 1);
    }
    permissionOf.set(p.path, prior ?? { permission, file: p.file });

    for (const link of scanLinks(p.body, p.bodyLine)) {
      const problem = checkLink(link, p, defaults, routes);
      if (problem !== undefined) err(p.file, problem.code, problem.message, link.line, link.column);
    }

    docs.push({
      path: p.path, locale: p.locale, title,
      ...(p.meta.order === undefined || p.meta.order === 0 ? {} : { order: p.meta.order }),
      ...(audience === undefined || audience === "" ? {} : { audience }),
      ...(permission === "" ? {} : { permission }),
    });
    files.set(`${DOCS_DIR}/${p.locale}/${p.path}.md`, Buffer.from(p.body.endsWith("\n") || p.body === "" ? p.body : p.body + "\n"));
  }
  docs.sort((a, b) => (a.path < b.path ? -1 : a.path > b.path ? 1 : a.locale < b.locale ? -1 : 1));

  diagnostics.push(...checkHelpHrefs(input.dir, defaults, parsed.length > 0));
  return { docs, files, diagnostics };
}

function checkLink(link: Link, from: Parsed, docs: ReadonlySet<string>, routes: ReadonlyMap<string, string>): { code: string; message: string } | undefined {
  const dest = link.dest;
  if (dest === "" || dest.startsWith("#")) return undefined;
  if (link.image && !SCHEME.test(dest)) {
    return { code: "docs.image", message: `image "${dest}": package documentation has no images (an image file cannot be shipped with it); remove it or link an absolute https URL` };
  }
  if (dest.startsWith("route:")) {
    const id = dest.slice("route:".length).split("#")[0]!;
    const path = routes.get(id);
    if (path === undefined) return { code: "docs.unknownRoute", message: `route link "${dest}": "${id}" is not a route or page of this package` };
    if (/[:{*]/.test(path)) return { code: "docs.unknownRoute", message: `route link "${dest}": "${path}" takes parameters, so it has no fixed address to link` };
    return undefined;
  }
  const scheme = SCHEME.exec(dest);
  if (scheme !== null) {
    return SAFE_SCHEMES.has(scheme[0].toLowerCase()) ? undefined : { code: "docs.linkScheme", message: `link "${dest}": the ${scheme[0]} scheme is not allowed (http, https, mailto, tel and route: are)` };
  }
  if (dest.startsWith("//")) return { code: "docs.linkScheme", message: `link "${dest}": a protocol-relative link is not allowed; use https://` };
  if (dest.startsWith("/")) return undefined;
  const target = dest.split("#")[0]!;
  if (target.includes("?")) return { code: "docs.linkTarget", message: `link "${dest}": a link to another document has no query string` };
  let decoded: string;
  try {
    decoded = decodeURIComponent(target);
  } catch {
    return { code: "docs.linkTarget", message: `link "${dest}" is not a valid address` };
  }
  if (!decoded.endsWith(".md")) {
    return { code: "docs.linkTarget", message: `link "${dest}": a relative link points at another document of the package by its file name, with .md (docs/<locale>/ files), or use route:<id> for a page` };
  }
  const resolved = posix.normalize(posix.join(posix.dirname(from.path), decoded)).replace(/\.md$/, "");
  if (resolved.startsWith("..") || resolved.startsWith("/")) {
    return { code: "docs.linkEscapes", message: `link "${dest}" leaves the ${from.locale}/ documentation tree: links stay inside the documents of the package` };
  }
  if (!docs.has(resolved)) {
    return { code: "docs.brokenLink", message: `link "${dest}": there is no document "${resolved}" (documents: ${[...docs].sort().join(", ")})` };
  }
  return undefined;
}

const SKIP_SOURCE = new Set(["node_modules", ".iota", ".git", DOCS_DIR]);

/** `helpHref("literal")` calls (one argument: the package's own documents) must name an existing document. */
function checkHelpHrefs(dir: string, docs: ReadonlySet<string>, haveDocs: boolean): Diagnostic[] {
  const out: Diagnostic[] = [];
  const visit = (current: string): void => {
    for (const entry of readdirSync(current, { withFileTypes: true })) {
      if (SKIP_SOURCE.has(entry.name)) continue;
      const abs = join(current, entry.name);
      if (entry.isDirectory()) visit(abs);
      else if (entry.isFile() && /\.(?:[cm]?[jt]s|[jt]sx)$/.test(entry.name) && !entry.name.endsWith(".d.ts")) scanSource(abs);
    }
  };
  const scanSource = (abs: string): void => {
    const text = readFileSync(abs, "utf8");
    if (!text.includes("helpHref")) return;
    const lines = text.split("\n");
    lines.forEach((line, i) => {
      for (const m of line.matchAll(/\bhelpHref\(\s*(["'`])([^"'`$\\]*)\1\s*\)/g)) {
        const target = m[2]!.split("#")[0]!;
        if (target !== "" && docs.has(target)) continue;
        const known = [...docs].sort();
        out.push({
          file: relPath(dir, abs), line: i + 1, column: (m.index ?? 0) + 1, severity: "error", code: "docs.unknownHelpHref",
          message: haveDocs
            ? `helpHref("${m[2]!}"): the package has no document "${target}" (documents: ${known.join(", ")})`
            : `helpHref("${m[2]!}"): the package has no documentation; add docs/${DEFAULT_DOC_LOCALE}/${target || "index"}.md`,
        });
      }
    });
  };
  visit(dir);
  return out;
}
