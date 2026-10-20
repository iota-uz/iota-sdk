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

const frame = () => new Promise((resolve) => requestAnimationFrame(resolve))

// False green: activating Apply without removing it (or without remounting) would never lose focus in the first place.
it('hands focus to the remounted editor after Apply removes itself', async () => {
  const june = { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }
  const all = { period: { mode: 'all' } }
  const [state, setState] = createStore<{ editor: ContextEditor }>({ editor: editor(june, june) })
  render(() => <ScopedFilterControls editor={state.editor} scope="dashboard" id="dashboard" />)
  fireEvent.click(screen.getByText('Stage all time'))
  const apply = screen.getByTestId('context-apply')
  apply.focus()
  fireEvent.click(apply)
  expect(screen.queryByTestId('context-apply')).toBeNull()
  setState('editor', 'value', 'dashboard', all)
  await frame()
  await frame()
  expect(screen.getByTestId('context-controls-dashboard-dashboard')).toContainElement(document.activeElement as HTMLElement)
})

// False green: a Restore that leaves Apply hidden would focus nothing and still pass a body check.
it('keeps focus in the editor after Restore removes itself', async () => {
  render(() => <ScopedFilterControls editor={editor({ period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }, { period: { mode: 'reports' } })} scope="dashboard" id="dashboard" />)
  const reset = screen.getByTestId('context-reset')
  reset.focus()
  fireEvent.click(reset)
  await frame()
  expect(document.activeElement).toBe(screen.getByTestId('context-apply'))
})

// False green: a natively disabled button would drop focus to the body while the save is pending.
it('keeps focus on Save filter defaults through a rejected save', async () => {
  let reject: (error: Error) => void = () => undefined
  const saveDefaults = vi.fn(() => new Promise<void>((_resolve, fail) => { reject = fail }))
  const june = { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }
  render(() => <ScopedFilterControls editor={editor(june, june)} scope="dashboard" id="dashboard" saveDefaults={saveDefaults} />)
  const save = screen.getByRole('button', { name: 'Save filter defaults' })
  save.focus()
  fireEvent.click(save)
  expect(save).toHaveAttribute('aria-disabled', 'true')
  expect(save).not.toBeDisabled()
  fireEvent.click(save)
  expect(saveDefaults).toHaveBeenCalledTimes(1)
  expect(document.activeElement).toBe(save)
  reject(new Error('rejected'))
  await screen.findByRole('alert')
  expect(save).not.toHaveAttribute('aria-disabled')
  expect(document.activeElement).toBe(save)
})
