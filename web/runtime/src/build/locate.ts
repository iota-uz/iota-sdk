// Source locations for graph and manifest diagnostics: where a file imports a specifier, and where
// a name is exported. Parsed with the TypeScript scanner so positions match the author's text.
import { readFileSync } from "node:fs";

import ts from "typescript";

export interface Location {
  line: number;
  column: number;
}

const cache = new Map<string, ts.SourceFile | undefined>();

function parse(file: string): ts.SourceFile | undefined {
  if (!cache.has(file)) {
    try {
      const kind = file.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS;
      cache.set(file, ts.createSourceFile(file, readFileSync(file, "utf8"), ts.ScriptTarget.ES2022, true, kind));
    } catch {
      cache.set(file, undefined);
    }
  }
  return cache.get(file);
}

function at(sf: ts.SourceFile, node: ts.Node): Location {
  const p = sf.getLineAndCharacterOfPosition(node.getStart(sf));
  return { line: p.line + 1, column: p.character + 1 };
}

/** Position of the first import/export-from/dynamic import/require of `specifier` in `file`. */
export function locateImport(file: string, specifier: string): Location | undefined {
  const sf = parse(file);
  if (sf === undefined) return undefined;
  let found: Location | undefined;
  const visit = (node: ts.Node): void => {
    if (found !== undefined) return;
    if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier !== undefined &&
      ts.isStringLiteral(node.moduleSpecifier) &&
      node.moduleSpecifier.text === specifier
    ) {
      found = at(sf, node.moduleSpecifier);
      return;
    }
    if (
      ts.isCallExpression(node) &&
      (node.expression.kind === ts.SyntaxKind.ImportKeyword || (ts.isIdentifier(node.expression) && node.expression.text === "require")) &&
      node.arguments[0] !== undefined &&
      ts.isStringLiteralLike(node.arguments[0]) &&
      node.arguments[0].text === specifier
    ) {
      found = at(sf, node.arguments[0]);
      return;
    }
    if (ts.isImportEqualsDeclaration(node) && ts.isExternalModuleReference(node.moduleReference) && ts.isStringLiteral(node.moduleReference.expression) && node.moduleReference.expression.text === specifier) {
      found = at(sf, node.moduleReference.expression);
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
  return found;
}

/** Position of the declaration that exports `name` (`export const name`, `export function name`, `export { name }`). */
export function locateExport(file: string, name: string): Location | undefined {
  const sf = parse(file);
  if (sf === undefined) return undefined;
  for (const st of sf.statements) {
    if (ts.isVariableStatement(st) && st.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword)) {
      for (const d of st.declarationList.declarations) if (ts.isIdentifier(d.name) && d.name.text === name) return at(sf, d.name);
    } else if ((ts.isFunctionDeclaration(st) || ts.isClassDeclaration(st)) && st.name?.text === name && st.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword)) {
      return at(sf, st.name);
    } else if (ts.isExportDeclaration(st) && st.exportClause !== undefined && ts.isNamedExports(st.exportClause)) {
      for (const el of st.exportClause.elements) if (el.name.text === name) return at(sf, el.name);
    }
  }
  return undefined;
}

/** Position of the first string literal with exactly this text (resource IDs). */
export function locateString(file: string, text: string, skip = 0): Location | undefined {
  const sf = parse(file);
  if (sf === undefined) return undefined;
  let seen = 0;
  let found: Location | undefined;
  const visit = (node: ts.Node): void => {
    if (found !== undefined) return;
    if (ts.isStringLiteralLike(node) && node.text === text && seen++ >= skip) {
      found = at(sf, node);
      return;
    }
    ts.forEachChild(node, visit);
  };
  visit(sf);
  return found;
}
