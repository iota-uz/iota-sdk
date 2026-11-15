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
