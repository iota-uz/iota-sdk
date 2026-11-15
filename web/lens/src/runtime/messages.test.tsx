import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { cleanup, render, screen } from '@solidjs/testing-library'
import { afterEach, describe, expect, it } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument } from '../contract'
import { DashboardRuntimeProvider, DocumentProvider, useTranslate } from './provider'
import { builtinMessage } from './messages'

afterEach(cleanup)

function Probe(props: { name: string; messageKey: string; fallback: string }) {
  const translate = useTranslate()
  return <p data-testid={props.name}>{translate(props.messageKey, props.fallback)}</p>
}

function renderProbe(locale: string, i18n: Record<string, string> = {}) {
  const document = parseDocument({ ...fixture, meta: { ...fixture.meta, locale }, i18n })
  render(() => (
    <DocumentProvider initialDocument={document}>
      <DashboardRuntimeProvider locale={locale}>{() => (
        <>
          <Probe fallback="Total" messageKey="panel.total" name="total" />
          <Probe fallback="Updated {time}" messageKey="dashboard.updated" name="updated" />
        </>
      )}</DashboardRuntimeProvider>
    </DocumentProvider>
  ))
}

describe('built-in messages', () => {
  it('resolves by the primary language subtag', () => {
    expect(builtinMessage('ru', 'panel.total')).toBe('Итого')
    expect(builtinMessage('ru-RU', 'panel.total')).toBe('Итого')
    expect(builtinMessage('uz', 'panel.total')).toBe('Jami')
    expect(builtinMessage('en', 'panel.total')).toBeUndefined()
    expect(builtinMessage(undefined, 'panel.total')).toBeUndefined()
  })

  it('does not hand a Cyrillic Uzbek reader the Latin catalogue', () => {
    expect(builtinMessage('uz-Cyrl', 'panel.total')).toBeUndefined()
  })

  it('translates chrome the host never sent', () => {
    renderProbe('ru')
    expect(screen.getByTestId('total')).toHaveTextContent('Итого')
    expect(screen.getByTestId('updated')).toHaveTextContent('Обновлено {time}')
  })

  it('lets the host catalogue win over the built-in one', () => {
    renderProbe('ru', { 'panel.total': 'Всего' })
    expect(screen.getByTestId('total')).toHaveTextContent('Всего')
  })

  it('keeps the English fallback for a language without a catalogue', () => {
    renderProbe('en')
    expect(screen.getByTestId('total')).toHaveTextContent('Total')
  })
})

// Keys whose wording belongs to the host: the report editor and the print
// report are administrative surfaces (see the note in messages.ts), and the
// pre-document strings have their own table.
const hostWorded = /^(report\.|print\.|runtime\.|dashboard\.edit|dashboard\.empty\.)/

function sourceFiles(directory: string): Array<string> {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return entry.name === 'test' || entry.name === 'node_modules' ? [] : sourceFiles(path)
    return /\.tsx?$/.test(entry.name) && !/\.(test|stories)\./.test(entry.name) ? [path] : []
  })
}

function runtimeKeys(): Array<string> {
  const keys = new Set<string>()
  for (const file of sourceFiles('src')) {
    const source = readFileSync(file, 'utf8')
    for (const match of source.matchAll(/(?:translate\(\s*|labelKey:\s*)'([a-z][A-Za-z]*(?:\.[A-Za-z_]+)+)'/g)) keys.add(match[1]!)
  }
  return [...keys].filter((key) => !hostWorded.test(key)).sort()
}

describe('built-in catalogue coverage', () => {
  it('translates every chrome string the runtime looks up into Russian', () => {
    const missing = runtimeKeys().filter((key) => builtinMessage('ru', key) === undefined)
    expect(missing).toEqual([])
  })

  it.each([
    'filter.period.previous', 'filter.period.next', 'filter.period.open', 'calendar.prevMonth', 'calendar.nextMonth',
    'table.openRecord', 'table.notSummable', 'table.scrollLeft', 'table.scrollRight', 'table.search', 'table.actions',
    'panel.openMetric', 'chart.openMark', 'cascade.openStage', 'explore.copyValue', 'filter.facet.remove',
    'availability.config_required', 'confidence.verified', 'export.failed',
  ])('translates the screen-reader label %s into Uzbek too', (key) => {
    expect(builtinMessage('uz', key)).toBeTruthy()
    expect(builtinMessage('uz', key)).not.toBe(builtinMessage('ru', key))
  })
})
