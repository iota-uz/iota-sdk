// Developer kit generation: `@iota-kit/<namespace>` modules for a catalog snapshot. The JS is pure —
// references built with the public SDK builders (nativeEntity, command, event, ...), never handlers —
// and the .d.ts comes from TypeScript declaration emit over that JS, so kit types are exactly what the
// builders infer. A source that ships its pure contracts module is used as the module JS as-is.
import { posix } from "node:path";

import ts from "typescript";

import type { FieldJSON, Json, ResourceJSON, TypeJSON } from "../model.ts";
import type { Layout } from "./layout.ts";
import type { Diagnostic, KitModules, KitResponse, KitSourcePayload } from "./protocol.ts";
import { compilerOptions, createHost } from "./typecheck.ts";
import { sha256 } from "./util.ts";
import { VirtualFS } from "./vfs.ts";

const KIT_ROOT = "/__kit__";
const moduleName = (ns: string): string => `@iota-kit/${ns}`;
const alias = (ns: string): string => `kit_${ns.replace(/[^A-Za-z0-9]/g, "_")}`;
const q = (value: unknown): string => JSON.stringify(value);

interface Entry {
  ns: string;
  exportName: string;
  resource: ResourceJSON;
}

const SKIPPED_KINDS = new Set(["page", "navigation", "cell", "shell"]);

/** A slot resource without a target declares a slot others contribute to; with a target it is a contribution. */
const isSlotDeclaration = (r: ResourceJSON): boolean => r.kind === "slot" && r.target === undefined;
const isPublic = (r: ResourceJSON): boolean => !SKIPPED_KINDS.has(r.kind) && (r.kind !== "slot" || isSlotDeclaration(r));

const pascal = (s: string): string => s.slice(0, 1).toUpperCase() + s.slice(1);

/** Export names: PascalCase of the last non-version id segment; colliding names take more segments. */
function exportNames(ids: readonly string[]): Map<string, string> {
  const segments = (id: string): string[] => {
    const all = id.split(".");
    if (all.length > 2 && /^v\d+$/.test(all[all.length - 1] ?? "")) all.pop();
    return all.slice(1);
  };
  const depth = new Map(ids.map((id) => [id, 1]));
  const name = (id: string): string =>
    segments(id)
      .slice(-(depth.get(id) ?? 1))
      .map(pascal)
      .join("");
  for (let round = 0; round < 12; round++) {
    const groups = new Map<string, string[]>();
    for (const id of ids) groups.set(name(id), [...(groups.get(name(id)) ?? []), id]);
    const clashing = [...groups.values()].filter((g) => g.length > 1);
    if (clashing.length === 0) break;
    for (const group of clashing) for (const id of group) depth.set(id, (depth.get(id) ?? 1) + 1);
  }
  const out = new Map<string, string>();
  const used = new Set<string>();
  for (const id of ids) {
    const base = name(id) || "Resource";
    let n = base;
    for (let i = 2; used.has(n); i++) n = `${base}${i}`;
    used.add(n);
    out.set(id, n);
  }
  return out;
}

function typeRefs(type: TypeJSON | undefined, out: string[]): void {
  if (type === undefined) return;
  if (type.kind === "ref" && type.entity !== undefined) out.push(type.entity);
  type.properties?.forEach((p) => typeRefs(p.type, out));
  typeRefs(type.items, out);
}

function dependencies(r: ResourceJSON): string[] {
  const out: string[] = [];
  r.fields?.forEach((f) => typeRefs(f.type, out));
  for (const t of [r.input, r.output, r.params, r.payload]) typeRefs(t, out);
  Object.values(r.signals ?? {}).forEach((t) => typeRefs(t, out));
  out.push(...(r.errors ?? []));
  for (const id of [r.base, r.subject, r.permission, r.overlay]) if (id !== undefined && id !== "") out.push(id);
  return out;
}

class Generator {
  readonly used = new Set<string>();
  readonly imports = new Set<string>();
  readonly diagnostics: Diagnostic[] = [];
  private readonly lines: string[] = [];
  private readonly done = new Set<string>();
  private readonly stack = new Set<string>();
  private readonly local = new Map<string, Entry>();

  constructor(
    private readonly ns: string,
    private readonly resources: readonly ResourceJSON[],
    private readonly index: ReadonlyMap<string, Entry>,
    private readonly ownedLists: readonly string[],
  ) {
    for (const r of resources) {
      const e = index.get(r.id);
      if (e !== undefined) this.local.set(r.id, e);
    }
  }

  private fail(r: string, message: string): string {
    this.diagnostics.push({ file: moduleName(this.ns), severity: "error", code: "kit.unresolved", message: `${r}: ${message}` });
    return "undefined";
  }

  private use(name: string): string {
    this.used.add(name);
    return name;
  }

  /** Expression naming an already generated contract (same module) or another kit module's export. */
  private expr(from: string, id: string): string {
    const e = this.index.get(id);
    if (e === undefined) return this.fail(from, `references "${id}", which no kit source declares`);
    if (e.ns === this.ns) return this.done.has(id) ? e.exportName : this.fail(from, `references "${id}" before it is declared`);
    this.imports.add(e.ns);
    return `${alias(e.ns)}.${e.exportName}`;
  }

  /** Entity argument of t.ref/field.ref; a back edge of a ref cycle degrades to an id-only reference. */
  private entityArg(from: string, id: string): string {
    const e = this.index.get(id);
    if (e !== undefined && e.ns === this.ns && !this.done.has(id)) return `{ id: ${q(id)} }`;
    return this.expr(from, id);
  }

  schema(from: string, type: TypeJSON): string {
    let s: string;
    switch (type.kind) {
      case "string":
        s = `t.string(${type.maxLength ? `{ maxLength: ${type.maxLength} }` : ""})`;
        break;
      case "decimal":
        s = `t.decimal({ precision: ${type.precision ?? 0}, scale: ${type.scale ?? 0} })`;
        break;
      case "enum":
        s = `t.enum(${q(type.values ?? [])})`;
        break;
      case "ref":
        s = `t.ref(${this.entityArg(from, type.entity ?? "")})`;
        break;
      case "object":
        s = `t.object({ ${(type.properties ?? []).map((p) => `${q(p.name)}: ${this.schema(from, p.type)}`).join(", ")} })`;
        break;
      case "array":
        s = `t.array(${this.schema(from, type.items ?? { kind: "json" })})`;
        break;
      default:
        s = `t.${type.kind}()`;
    }
    this.use("t");
    if (type.optional === true) s += ".optional()";
    if (type.nullable === true && type.optional !== true) s += ".nullable()";
    return s;
  }

  private field(from: string, f: FieldJSON): string {
    const type = f.type;
    let s: string;
    switch (type.kind) {
      case "string":
        s = `field.text(${type.maxLength ? `{ maxLength: ${type.maxLength} }` : ""})`;
        break;
      case "decimal":
        s = `field.decimal({ precision: ${type.precision ?? 0}, scale: ${type.scale ?? 0} })`;
        break;
      case "enum":
        s = `field.enum(${q(type.values ?? [])})`;
        break;
      case "ref":
        s = `field.ref(${this.entityArg(from, type.entity ?? "")})`;
        break;
      case "object":
      case "array":
        this.diagnostics.push({ file: moduleName(this.ns), severity: "error", code: "kit.unsupportedField", message: `${from}: field "${f.id}" is ${type.kind}; entity fields are scalars (use json)` });
        s = "field.json()";
        break;
      default:
        s = `field.${type.kind}()`;
    }
    this.use("field");
    if (type.optional === true) s += ".optional()";
    else if (type.nullable === true) s += ".nullable()";
    if (f.unique === true) s += ".unique()";
    if (f.index === true) s += ".index()";
    if (f.default !== undefined) s += `.default(${q(f.default as Json)})`;
    if (f.renamedFrom !== undefined && f.renamedFrom.length > 0) s += `.renamedFrom(${f.renamedFrom.map((k) => q(k)).join(", ")})`;
    if (f.label !== undefined) s += `.label(${q(f.label)})`;
    return s;
  }

  private fields(from: string, fields: readonly FieldJSON[] | undefined): string {
    return `{ ${(fields ?? []).map((f) => `${q(f.id)}: ${this.field(from, f)}`).join(", ")} }`;
  }

  private callable(r: ResourceJSON, builder: string): string {
    const props: string[] = [];
    if (r.atomic === true) props.push("atomic: true");
    props.push(`input: ${this.schema(r.id, r.input ?? { kind: "object" })}`);
    if (r.output !== undefined) props.push(`output: ${this.schema(r.id, r.output)}`);
    if (r.errors !== undefined && r.errors.length > 0) props.push(`errors: [${r.errors.map((e) => this.expr(r.id, e)).join(", ")}]`);
    if (r.permission !== undefined) props.push(`permission: ${this.expr(r.id, r.permission)}`);
    if (r.label !== undefined) props.push(`label: ${q(r.label)}`);
    if (r.description !== undefined) props.push(`description: ${q(r.description)}`);
    return `${this.use(builder)}(${q(r.id)}, { ${props.join(", ")} })`;
  }

  private build(r: ResourceJSON): string | undefined {
    const subject = (): string => (r.subject === undefined ? "" : `subject: ${this.expr(r.id, r.subject)}, `);
    switch (r.kind) {
      case "entity":
        return `${this.use("nativeEntity")}(${q(r.id)}, ${this.fields(r.id, r.fields)}, { key: "string" })`;
      case "overlay": {
        const opts = [r.label !== undefined ? `label: ${q(r.label)}` : "", r.description !== undefined ? `description: ${q(r.description)}` : ""].filter(Boolean);
        return `foreign(${this.use("overlay")}(${q(r.id)}, ${this.expr(r.id, r.base ?? "")}, ${this.fields(r.id, r.fields)}${opts.length > 0 ? `, { ${opts.join(", ")} }` : ""}))`;
      }
      case "command":
        return `foreign(${this.callable(r, "command")})`;
      case "operation":
        return `foreign(${this.callable(r, "operation")})`;
      case "query":
        return `foreign(${this.callable(r, "query")})`;
      case "event":
      case "notice": {
        const desc = r.description !== undefined ? `, description: ${q(r.description)}` : "";
        return `foreign(${this.use(r.kind)}(${q(r.id)}, { ${subject()}payload: ${this.schema(r.id, r.payload ?? { kind: "object" })}${desc} }))`;
      }
      case "workflow": {
        const props = [`input: ${this.schema(r.id, r.input ?? { kind: "object" })}`];
        if (r.output !== undefined) props.push(`output: ${this.schema(r.id, r.output)}`);
        const signals = Object.entries(r.signals ?? {});
        if (signals.length > 0) props.push(`signals: { ${signals.map(([k, v]) => `${q(k)}: ${this.schema(r.id, v)}`).join(", ")} }`);
        if (r.description !== undefined) props.push(`description: ${q(r.description)}`);
        return `foreign(${this.use("workflow")}(${q(r.id)}, { ${props.join(", ")} }))`;
      }
      case "domainError":
      case "issue":
        return `foreign(${this.use(r.kind)}(${q(r.id)}, ${this.schema(r.id, r.params ?? { kind: "object" })}))`;
      case "permission":
        return `foreign(${this.use("permission")}(${q(r.id)}, { label: ${q(r.label ?? { en: r.id })} }))`;
      case "route":
        return `foreign(${this.use("route")}(${q(r.id)}, { path: ${q(r.path ?? "")}, params: ${this.schema(r.id, r.params ?? { kind: "object" })} }))`;
      case "contactResolver":
      case "deliveryPolicy":
        return `foreign(${this.use(r.kind)}(${q(r.id)}, ${this.expr(r.id, r.subject ?? "")}))`;
      case "column": {
        const overlayName = this.expr(r.id, r.overlay ?? "");
        const props = [`id: ${q(r.id)}`, `field: ${overlayName}.fields[${q(r.field ?? "")}]`, `label: ${q(r.label ?? { en: r.id })}`];
        for (const flag of ["filterable", "sortable", "exportable"] as const) if (r[flag] === true) props.push(`${flag}: true`);
        return `foreign(${this.use("column")}(${this.expr(r.id, `list:${r.list ?? ""}`)}, { ${props.join(", ")} }))`;
      }
      case "slot":
        return `${this.use("slotRef")}(${q(r.id)})`;
      default:
        return undefined;
    }
  }

  private emit(id: string): void {
    if (this.done.has(id) || this.stack.has(id)) return;
    const r = this.resources.find((x) => x.id === id);
    const entry = this.local.get(id);
    if (r === undefined || entry === undefined) return;
    this.stack.add(id);
    for (const dep of dependencies(r)) if (this.local.has(dep)) this.emit(dep);
    this.stack.delete(id);
    const value = this.build(r);
    this.done.add(id);
    if (value !== undefined) this.lines.push(`export const ${entry.exportName} = ${value};`);
  }

  generate(): string {
    // Native lists are not resources of their own: they are named by the columns that extend them.
    for (const l of this.ownedLists) {
      const id = `list:${l}`;
      const entry = this.index.get(id);
      if (entry !== undefined) {
        this.done.add(id);
        this.lines.push(`export const ${entry.exportName} = ${this.use("nativeList")}(${q(l)});`);
      }
    }
    for (const r of [...this.resources].sort((a, b) => (a.id < b.id ? -1 : 1))) this.emit(r.id);
    const sdk = [...this.used].sort();
    const head = [
      "// Generated developer kit module: pure references, no handlers.",
      ...(this.used.size > 0 ? [`import { ${[...new Set([...sdk, "foreign"])].sort().join(", ")} } from "@iota-uz/sdk/runtime";`] : []),
      ...[...this.imports].sort().map((ns) => `import * as ${alias(ns)} from ${q(moduleName(ns))};`),
    ];
    return `${head.join("\n")}\n\n${this.lines.join("\n")}\n`;
  }
}

/** id -> export name of `export const X = builder("id", ...)` in a shipped pure module. */
function scanExports(js: string): Map<string, string> {
  const sf = ts.createSourceFile("index.js", js, ts.ScriptTarget.ES2022, true, ts.ScriptKind.JS);
  const local = new Map<string, string>();
  const out = new Map<string, string>();
  const exported = new Map<string, string>();
  for (const st of sf.statements) {
    if (ts.isVariableStatement(st)) {
      const isExport = st.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword) === true;
      for (const d of st.declarationList.declarations) {
        if (!ts.isIdentifier(d.name) || d.initializer === undefined) continue;
        const call = unwrap(d.initializer);
        const first = call?.arguments[0];
        if (first !== undefined && ts.isStringLiteralLike(first)) {
          local.set(d.name.text, first.text);
          if (isExport) out.set(first.text, d.name.text);
        }
      }
    } else if (ts.isExportDeclaration(st) && st.exportClause !== undefined && ts.isNamedExports(st.exportClause) && st.moduleSpecifier === undefined) {
      for (const el of st.exportClause.elements) exported.set(el.propertyName?.text ?? el.name.text, el.name.text);
    }
  }
  for (const [localName, exportName] of exported) {
    const id = local.get(localName);
    if (id !== undefined && !out.has(id)) out.set(id, exportName);
  }
  return out;
}

/** `foreign(builder("id", ...))` and `builder("id", ...)` both yield the inner builder call. */
function unwrap(node: ts.Expression): ts.CallExpression | undefined {
  if (!ts.isCallExpression(node)) return undefined;
  const first = node.arguments[0];
  if (ts.isIdentifier(node.expression) && node.expression.text === "foreign" && first !== undefined && ts.isCallExpression(first)) return first;
  return node;
}

function shippedImports(js: string): string[] {
  const sf = ts.createSourceFile("index.js", js, ts.ScriptTarget.ES2022, false, ts.ScriptKind.JS);
  const out: string[] = [];
  for (const st of sf.statements) {
    if ((ts.isImportDeclaration(st) || ts.isExportDeclaration(st)) && st.moduleSpecifier !== undefined && ts.isStringLiteral(st.moduleSpecifier) && st.moduleSpecifier.text.startsWith("@iota-kit/")) {
      out.push(st.moduleSpecifier.text.slice("@iota-kit/".length));
    }
  }
  return out;
}

function packageJSON(ns: string, hash: string): string {
  return (
    JSON.stringify(
      {
        name: moduleName(ns),
        version: "0.0.0",
        private: true,
        type: "module",
        main: "./index.js",
        types: "./index.d.ts",
        exports: { ".": { types: "./index.d.ts", import: "./index.js" } },
        iota: { kit: { namespace: ns, hash } },
      },
      null,
      2,
    ) + "\n"
  );
}

function declarations(layout: Layout, ns: string, js: string, built: KitModules, cache: Map<string, ts.SourceFile | undefined>): { dts: string; diagnostics: Diagnostic[] } {
  const vfs = new VirtualFS(KIT_ROOT, layout, { hash: "", modules: built });
  const root = posix.join(KIT_ROOT, "src", `${ns}.js`);
  vfs.addFile(root, js);
  const options: ts.CompilerOptions = {
    ...compilerOptions(KIT_ROOT),
    allowJs: true,
    checkJs: false,
    declaration: true,
    emitDeclarationOnly: true,
    noEmit: false,
    types: [],
    lib: ["lib.es2022.d.ts"],
    outDir: posix.join(KIT_ROOT, "out"),
  };
  const host = createHost(vfs, layout, cache);
  let dts = "";
  host.writeFile = (name, text) => {
    if (name.endsWith(".d.ts")) dts = text;
  };
  const program = ts.createProgram({ rootNames: [root], options, host });
  const diagnostics: Diagnostic[] = [];
  const sf = program.getSourceFile(root);
  for (const d of [...program.getSyntacticDiagnostics(sf), ...program.getDeclarationDiagnostics(sf)]) {
    diagnostics.push({ file: moduleName(ns), severity: "error", code: `kit.declaration.${d.code}`, message: ts.flattenDiagnosticMessageText(d.messageText, "\n") });
  }
  program.emit(sf, undefined, undefined, true);
  return { dts, diagnostics };
}

export function generateKit(layout: Layout, sources: readonly KitSourcePayload[]): KitResponse {
  const diagnostics: Diagnostic[] = [];
  const fail = (code: string, message: string): KitResponse => ({
    diagnostics: [...diagnostics, { file: "kit", severity: "error", code, message }],
    hash: "",
    modules: {},
  });

  const seen = new Set<string>();
  for (const s of sources) {
    if (seen.has(s.namespace)) return fail("kit.duplicateNamespace", `namespace "${s.namespace}" appears twice`);
    seen.add(s.namespace);
  }
  const ordered = [...sources].sort((a, b) => (a.namespace < b.namespace ? -1 : 1));
  const hash = sha256(JSON.stringify(ordered.map((s) => [s.namespace, [...s.resources].sort((a, b) => (a.id < b.id ? -1 : 1)), s.contractsModule ?? null])));

  // Index of every public resource and native list with its export name in its module.
  const index = new Map<string, Entry>();
  const publicOf = new Map<string, ResourceJSON[]>();
  const shippedNames = new Map<string, Map<string, string>>();
  for (const s of ordered) {
    publicOf.set(s.namespace, s.resources.filter(isPublic));
    if (s.contractsModule !== undefined) shippedNames.set(s.namespace, scanExports(s.contractsModule));
  }
  // A native list is owned by the namespace its id extends (longest prefix), else by the first declaring column.
  const listOwner = new Map<string, string>();
  for (const s of ordered) {
    for (const r of publicOf.get(s.namespace) ?? []) {
      if (r.kind !== "column" || !r.list) continue;
      const prefixed = ordered.filter((o) => r.list?.startsWith(`${o.namespace}.`) === true).sort((a, b) => b.namespace.length - a.namespace.length)[0];
      if (!listOwner.has(r.list)) listOwner.set(r.list, prefixed?.namespace ?? s.namespace);
    }
  }
  const ownedLists = new Map<string, string[]>();
  for (const [list, ns] of [...listOwner].sort()) ownedLists.set(ns, [...(ownedLists.get(ns) ?? []), list]);
  for (const s of ordered) {
    const resources = publicOf.get(s.namespace) ?? [];
    const lists = ownedLists.get(s.namespace) ?? [];
    const ids = [...resources.map((r) => r.id), ...lists];
    const names = exportNames(ids);
    const shipped = shippedNames.get(s.namespace);
    for (const bare of ids) {
      const isList = lists.includes(bare);
      const id = isList ? `list:${bare}` : bare;
      const resource = resources.find((r) => r.id === bare) ?? ({ id: bare, kind: "slot" } as ResourceJSON);
      const exportName = shipped === undefined ? names.get(bare) : shipped.get(bare);
      if (exportName !== undefined) index.set(id, { ns: s.namespace, exportName, resource });
    }
  }

  // Modules in dependency order; a cycle between kit modules cannot be initialized.
  const needs = new Map<string, Set<string>>();
  for (const s of ordered) {
    const deps = new Set<string>();
    if (s.contractsModule !== undefined) shippedImports(s.contractsModule).forEach((d) => deps.add(d));
    else for (const r of publicOf.get(s.namespace) ?? []) {
      for (const id of dependencies(r)) {
        const e = index.get(id);
        if (e !== undefined && e.ns !== s.namespace) deps.add(e.ns);
      }
      if (r.list !== undefined) {
        const e = index.get(`list:${r.list}`);
        if (e !== undefined && e.ns !== s.namespace) deps.add(e.ns);
      }
    }
    needs.set(s.namespace, deps);
  }
  const order: KitSourcePayload[] = [];
  const state = new Map<string, "visiting" | "done">();
  let cycle: string | undefined;
  const visit = (ns: string): void => {
    if (state.get(ns) === "done" || cycle !== undefined) return;
    if (state.get(ns) === "visiting") {
      cycle = ns;
      return;
    }
    state.set(ns, "visiting");
    for (const d of [...(needs.get(ns) ?? [])].sort()) if (seen.has(d)) visit(d);
    state.set(ns, "done");
    const src = ordered.find((s) => s.namespace === ns);
    if (src !== undefined) order.push(src);
  };
  for (const s of ordered) visit(s.namespace);
  if (cycle !== undefined) return fail("kit.cycle", `kit modules form a dependency cycle through "${cycle}"`);

  const modules: Record<string, Record<string, string>> = {};
  const cache = new Map<string, ts.SourceFile | undefined>();
  for (const s of order) {
    let js: string;
    if (s.contractsModule !== undefined) js = s.contractsModule;
    else {
      const gen = new Generator(s.namespace, publicOf.get(s.namespace) ?? [], index, ownedLists.get(s.namespace) ?? []);
      js = gen.generate();
      diagnostics.push(...gen.diagnostics);
    }
    const emitted = declarations(layout, s.namespace, js, modules, cache);
    diagnostics.push(...emitted.diagnostics);
    // A shipped module declares its entities with `entity()`. To every other package they are foreign:
    // typed like native entities, so generic writes (tx.insert/update/delete) are a type error.
    if (s.contractsModule !== undefined) emitted.dts = emitted.dts.replace(/\}, "own">/g, '}, "native">');
    modules[moduleName(s.namespace)] = {
      "index.js": js,
      "index.d.ts": emitted.dts,
      "package.json": packageJSON(s.namespace, hash),
    };
  }
  const failed = diagnostics.some((d) => d.severity === "error");
  return { diagnostics, hash, modules: failed ? {} : modules };
}
