import type { Diagnostic } from './types'

/** Monaco's MarkerSeverity values (monaco.MarkerSeverity). */
export const MarkerSeverity = { Hint: 1, Info: 2, Warning: 4, Error: 8 } as const

export interface Marker {
  startLineNumber: number
  startColumn: number
  endLineNumber: number
  endColumn: number
  severity: number
  message: string
  code: string
  source: string
}

/** normalizePath makes a diagnostic's file comparable with draft paths. */
export function normalizePath(file: string): string {
  let p = file.replace(/\\/g, '/')
  while (p.startsWith('./')) p = p.slice(2)
  return p.replace(/^\/+/, '')
}

/** location renders file:line:column the way editors and MCP clients jump to it. */
export function location(d: Diagnostic): string {
  const file = normalizePath(d.file)
  if (d.line && d.line > 0) return d.column && d.column > 0 ? `${file}:${d.line}:${d.column}` : `${file}:${d.line}`
  return file
}

export interface FileState {
  /** Server hash of the file now. */
  baseHash: string | null
  /** Unsaved edits. */
  dirty: boolean
}

export interface Problem {
  diagnostic: Diagnostic
  path: string
  /** The diagnostic was computed from a saved version that is no longer the buffer. */
  stale: boolean
}

/**
 * DiagnosticsModel maps the server's check/build diagnostics to files and
 * lines. A check always runs on the saved draft, so a diagnostic is exact only
 * while the file still has the hash the check saw and holds no unsaved edits;
 * after that it stays in the problem list marked stale and is not drawn in the
 * editor (a drifted line number would point at the wrong code).
 */
export class DiagnosticsModel {
  private diagnostics: Diagnostic[] = []
  private hashes = new Map<string, string | null>()

  /** set replaces the report. hashes are the draft files' server hashes at the time of the run. */
  set(diagnostics: readonly Diagnostic[], hashes: Record<string, string | null>): void {
    this.diagnostics = [...diagnostics]
    this.hashes = new Map(Object.entries(hashes).map(([p, h]) => [normalizePath(p), h]))
  }

  clear(): void {
    this.diagnostics = []
    this.hashes.clear()
  }

  isStale(path: string, state: FileState | undefined): boolean {
    const p = normalizePath(path)
    if (!state) return true
    return state.dirty || !this.hashes.has(p) || this.hashes.get(p) !== state.baseHash
  }

  /** problems lists every diagnostic, errors first, then by file and position. */
  problems(files: ReadonlyMap<string, FileState> | Record<string, FileState>): Problem[] {
    const lookup = (p: string): FileState | undefined => (files instanceof Map ? files.get(p) : (files as Record<string, FileState>)[p])
    const out = this.diagnostics.map((d) => {
      const path = normalizePath(d.file)
      return { diagnostic: d, path, stale: this.isStale(path, lookup(path)) }
    })
    out.sort((a, b) =>
      Number(b.diagnostic.severity === 'error') - Number(a.diagnostic.severity === 'error') ||
      a.path.localeCompare(b.path) || (a.diagnostic.line ?? 0) - (b.diagnostic.line ?? 0) || (a.diagnostic.column ?? 0) - (b.diagnostic.column ?? 0))
    return out
  }

  /**
   * markersFor returns the editor markers of one open file: exact diagnostics
   * only. lineText returns the 1-based line of the buffer, or undefined past
   * its end; positions are clamped into the buffer.
   */
  markersFor(path: string, state: FileState | undefined, lineText: (line: number) => string | undefined, lineCount: number): Marker[] {
    const p = normalizePath(path)
    if (this.isStale(p, state)) return []
    const out: Marker[] = []
    for (const d of this.diagnostics) {
      if (normalizePath(d.file) !== p) continue
      out.push(toMarker(d, lineText, lineCount))
    }
    return out
  }
}

export function toMarker(d: Diagnostic, lineText: (line: number) => string | undefined, lineCount: number): Marker {
  const line = Math.min(Math.max(d.line && d.line > 0 ? d.line : 1, 1), Math.max(lineCount, 1))
  const text = lineText(line) ?? ''
  const startColumn = Math.min(Math.max(d.column && d.column > 0 ? d.column : 1, 1), text.length + 1)
  // underline the identifier at the position, else one character, else the line
  let end = startColumn
  while (end <= text.length && /[\w$]/.test(text.charAt(end - 1))) end++
  if (end === startColumn) end = Math.min(startColumn + 1, text.length + 1)
  if (!d.column && text.length > 0) end = text.length + 1
  return {
    startLineNumber: line, startColumn, endLineNumber: line, endColumn: Math.max(end, startColumn + (text.length === 0 ? 1 : 0)),
    severity: d.severity === 'error' ? MarkerSeverity.Error : MarkerSeverity.Warning,
    message: d.message, code: d.code, source: 'iota',
  }
}
