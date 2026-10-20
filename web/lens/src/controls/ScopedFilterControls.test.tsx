import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library'
import { createStore } from 'solid-js/store'
import { afterEach, expect, it, vi } from 'vitest'
import type { ContextEditor } from '../contract/types'
import { ScopedFilterControls } from './ScopedFilterControls'

vi.mock('../runtime', () => ({
  useDashboard: () => ({ document: { revision: 'initial' } }),
  useFilters: () => ({ applyURL: vi.fn() }),
  useTranslate: () => (_key: string, fallback: string) => fallback,
}))
vi.mock('./PeriodFilterControl', () => ({
  PeriodFilterControl: (props: { onChange: (value: { start: string; end: string }) => void }) =>
    <button onClick={() => props.onChange({ start: '', end: '' })}>Stage all time</button>,
}))
afterEach(cleanup)

// False green: remounting the test component would conceal stale mounted scope snapshots.
it.each(['dashboard', 'card', 'section'] as const)('replaces mounted %s drafts with the active document', (scope) => {
  const initial = { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }
  const next = { period: { mode: 'relative', days: 30 } }
  const [state, setState] = createStore<{ editor: ContextEditor }>({ editor: {
    url: '/sales',
    value: { calendar: { timezone: 'Asia/Tashkent', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } }, dashboard: initial, cards: { test: initial }, sections: { test: initial } },
    saved: { calendar: { timezone: 'Asia/Tashkent', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } }, dashboard: initial, cards: { test: initial }, sections: { test: initial } },
  } })
  render(() => <ScopedFilterControls editor={state.editor} scope={scope} id="test" />)
  fireEvent.click(screen.getByText('Stage all time'))
  expect(screen.getByTestId('context-apply')).toBeInTheDocument()
  setState('editor', 'value', { dashboard: next, cards: { test: next }, sections: { test: next } })
  setState('editor', 'saved', { dashboard: next, cards: { test: next }, sections: { test: next } })
  expect(screen.queryByTestId('context-apply')).toBeNull()
  fireEvent.click(screen.getByText('Stage all time'))
  fireEvent.click(screen.getByTestId('context-reset'))
  expect(screen.queryByTestId('context-apply')).toBeNull()
})

const calendar = { timezone: 'Asia/Tashkent', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } }
const editor = (value: ContextEditor['value']['dashboard'], saved: ContextEditor['value']['dashboard']): ContextEditor => ({
  url: '/sales', value: { calendar, dashboard: value }, saved: { calendar, dashboard: saved },
})

// False green: checking only that Apply is enabled would pass with the always-rendered ghost button.
it('offers Apply and its status only while a change is staged', () => {
  const june = { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }
  render(() => <ScopedFilterControls editor={editor(june, june)} scope="dashboard" id="dashboard" />)
  expect(screen.queryByTestId('context-apply')).toBeNull()
  expect(screen.queryByRole('status')).toBeNull()
  expect(screen.queryByTestId('context-reset')).toBeNull()
  fireEvent.click(screen.getByText('Stage all time'))
  expect(screen.getByTestId('context-apply')).toHaveAttribute('data-variant', 'primary')
  expect(screen.getByRole('status')).toHaveTextContent('Changes are not applied')
  expect(screen.getByTestId('context-reset')).toBeInTheDocument()
  fireEvent.click(screen.getByTestId('context-reset'))
  expect(screen.queryByTestId('context-apply')).toBeNull()
  expect(screen.queryByTestId('context-reset')).toBeNull()
})

// False green: a draft-only dirty check would hide Restore for an applied selection that differs from saved defaults.
it('offers Restore whenever the shown selection differs from the saved one, and staging it offers Apply', () => {
  render(() => <ScopedFilterControls editor={editor({ period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }, { period: { mode: 'reports' } })} scope="dashboard" id="dashboard" />)
  expect(screen.queryByTestId('context-apply')).toBeNull()
  fireEvent.click(screen.getByTestId('context-reset'))
  expect(screen.queryByTestId('context-reset')).toBeNull()
  expect(screen.getByTestId('context-apply')).toBeInTheDocument()
})
