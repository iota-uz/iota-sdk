import { describe, expect, it } from 'vitest'
import { DiagnosticsModel, MarkerSeverity, location, normalizePath, toMarker } from './diagnostics'
import type { Diagnostic } from './types'

const diag = (over: Partial<Diagnostic>): Diagnostic => ({ file: 'src/a.ts', line: 2, column: 7, severity: 'error', code: 'ts.2322', message: 'Type mismatch', ...over })
const lines = ['import x from "./x"', 'const value: string = 1', 'export {}']
const text = (n: number) => lines[n - 1]

describe('location and path normalisation', () => {
  it('renders file:line:column and tolerates missing positions', () => {
    expect(location(diag({}))).toBe('src/a.ts:2:7')
    expect(location(diag({ column: undefined }))).toBe('src/a.ts:2')
    expect(location(diag({ line: undefined, column: undefined }))).toBe('src/a.ts')
    expect(location(diag({ file: './src/a.ts' }))).toBe('src/a.ts:2:7')
  })

  it('normalises separators and leading dots so server and editor paths agree', () => {
    expect(normalizePath('./src/a.ts')).toBe('src/a.ts')
    expect(normalizePath('/src/a.ts')).toBe('src/a.ts')
    expect(normalizePath('src\\a.ts')).toBe('src/a.ts')
  })
})

describe('markers', () => {
  it('maps a diagnostic to the identifier at its line and column', () => {
    const m = toMarker(diag({}), text, lines.length)
    expect(m).toMatchObject({ startLineNumber: 2, startColumn: 7, endLineNumber: 2, endColumn: 12, severity: MarkerSeverity.Error, code: 'ts.2322', source: 'iota' })
  })

  it('maps warnings and clamps positions into the buffer', () => {
    const m = toMarker(diag({ severity: 'warning', line: 99, column: 500 }), text, lines.length)
    expect(m.severity).toBe(MarkerSeverity.Warning)
    expect(m.startLineNumber).toBe(3)
    expect(m.startColumn).toBe(lines[2]!.length + 1)
    expect(m.endColumn).toBeGreaterThanOrEqual(m.startColumn)
  })

  it('marks the whole line when only the line is known and the first character when the buffer is empty', () => {
    const whole = toMarker(diag({ column: undefined }), text, lines.length)
    expect(whole).toMatchObject({ startColumn: 1, endColumn: lines[1]!.length + 1 })
    const empty = toMarker(diag({ line: 1, column: undefined }), () => '', 1)
    expect(empty).toMatchObject({ startColumn: 1, endColumn: 2 })
  })
})

describe('DiagnosticsModel', () => {
  const report = [
    diag({ file: 'src/a.ts' }),
    diag({ file: 'src/b.ts', severity: 'warning', line: 1, column: 1, code: 'lint.x' }),
    diag({ file: './src/a.ts', line: 1, column: 1, code: 'ts.1' }),
    diag({ file: 'manifest', line: undefined, column: undefined, code: 'manifest.duplicateId' }),
  ]

  it('draws only the diagnostics of the requested file', () => {
    const m = new DiagnosticsModel()
    m.set(report, { 'src/a.ts': 'h1', 'src/b.ts': 'h1' })
    const a = m.markersFor('src/a.ts', { baseHash: 'h1', dirty: false }, text, lines.length)
    expect(a.map((x) => x.code).sort()).toEqual(['ts.1', 'ts.2322'])
    expect(m.markersFor('src/b.ts', { baseHash: 'h1', dirty: false }, text, lines.length)).toHaveLength(1)
    expect(m.markersFor('src/c.ts', { baseHash: 'h1', dirty: false }, text, lines.length)).toHaveLength(0)
  })

  it('stops drawing diagnostics once the file has unsaved edits or a new hash, but keeps listing them as stale', () => {
    const m = new DiagnosticsModel()
    m.set(report, { 'src/a.ts': 'h1', 'src/b.ts': 'h1' })
    expect(m.markersFor('src/a.ts', { baseHash: 'h1', dirty: true }, text, lines.length)).toEqual([])
    expect(m.markersFor('src/a.ts', { baseHash: 'h2', dirty: false }, text, lines.length)).toEqual([])
    const problems = m.problems({ 'src/a.ts': { baseHash: 'h1', dirty: true }, 'src/b.ts': { baseHash: 'h1', dirty: false } })
    const byFile = (f: string) => problems.filter((p) => p.path === f).every((p) => p.stale)
    expect(byFile('src/a.ts')).toBe(true)
    expect(problems.find((p) => p.path === 'src/b.ts')?.stale).toBe(false)
  })

  it('lists problems errors first, then by file and position, and treats unknown files as stale', () => {
    const m = new DiagnosticsModel()
    m.set(report, { 'src/a.ts': 'h1', 'src/b.ts': 'h1' })
    const problems = m.problems(new Map([['src/a.ts', { baseHash: 'h1', dirty: false }], ['src/b.ts', { baseHash: 'h1', dirty: false }]]))
    expect(problems.map((p) => p.diagnostic.code)).toEqual(['manifest.duplicateId', 'ts.1', 'ts.2322', 'lint.x'])
    expect(problems[0]).toMatchObject({ path: 'manifest', stale: true })
  })

  it('forgets everything on clear', () => {
    const m = new DiagnosticsModel()
    m.set(report, { 'src/a.ts': 'h1' })
    m.clear()
    expect(m.problems({})).toEqual([])
  })
})
