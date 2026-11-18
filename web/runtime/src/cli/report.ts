// Human-readable output: diagnostics as `file:line:col code message`, test results grouped by file.
import type { Diagnostic, TestCaseResult } from '../build/protocol.ts'

export interface Style {
  red(text: string): string
  green(text: string): string
  yellow(text: string): string
  dim(text: string): string
  bold(text: string): string
}

export function style(enabled: boolean): Style {
  const wrap = (open: number, close: number) => (text: string): string => (enabled ? `\u001b[${open}m${text}\u001b[${close}m` : text)
  return { red: wrap(31, 39), green: wrap(32, 39), yellow: wrap(33, 39), dim: wrap(2, 22), bold: wrap(1, 22) }
}

const indent = (text: string, by: string): string =>
  text
    .split('\n')
    .map((l) => (l === '' ? l : by + l))
    .join('\n')

/** `src/server.ts:12:5 ts.2322 message`; warnings read `... code warning: message`. Continuation lines are indented. */
export function formatDiagnostic(d: Diagnostic, s: Style): string {
  const where = d.line === undefined ? d.file : `${d.file}:${d.line}:${d.column ?? 1}`
  const [first = '', ...rest] = d.message.split('\n')
  const tag = d.severity === 'warning' ? `${s.yellow('warning:')} ` : ''
  const head = `${s.bold(where)} ${s.dim(d.code)} ${tag}${first}`
  return rest.length === 0 ? head : `${head}\n${indent(rest.join('\n'), '    ')}`
}

export function formatDiagnostics(list: readonly Diagnostic[], s: Style): string {
  return list.map((d) => formatDiagnostic(d, s)).join('\n')
}

export function formatCase(c: TestCaseResult, s: Style): string {
  if (c.passed) return `  ${s.green('✓')} ${c.name} ${s.dim(`(${c.durationMs}ms)`)}`
  const lines = [`  ${s.red('✗')} ${c.name} ${s.dim(`(${c.durationMs}ms)`)}`]
  if (c.message !== undefined && c.message !== '') lines.push(indent(c.message, '      '))
  if (c.location !== undefined) lines.push(`      ${s.dim('at')} ${c.location.file}:${c.location.line}:${c.location.column}`)
  const frames = (c.frames ?? []).filter((f) => !f.includes(`${c.location?.file ?? '\u0000'}:${c.location?.line ?? 0}:${c.location?.column ?? 0}`))
  for (const f of frames.slice(0, 4)) lines.push(`      ${s.dim(`at ${f}`)}`)
  return lines.join('\n')
}

export function formatTests(cases: readonly TestCaseResult[], s: Style): string {
  const out: string[] = []
  let file: string | undefined
  for (const c of cases) {
    if (c.file !== file) {
      file = c.file
      out.push(s.bold(c.file))
    }
    out.push(formatCase(c, s))
  }
  return out.join('\n')
}

export const plural = (n: number, word: string): string => `${n} ${word}${n === 1 ? '' : 's'}`
