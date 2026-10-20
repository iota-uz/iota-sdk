import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library'
import { createStore } from 'solid-js/store'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type { ContextEditor, ContextFilters } from '../contract/types'
import { decodeFilterBoard, filterContextParam } from '../reports/filterDraft'
import { ScopedFilterControls } from './ScopedFilterControls'

const applyURL = vi.hoisted(() => vi.fn<(url: string) => void>())
vi.mock('../runtime', () => ({
  useDashboard: () => ({ document: { revision: 'initial' } }),
  useFilters: () => ({ applyURL }),
  useTranslate: () => (_key: string, fallback: string) => fallback,
}))
vi.mock('./PeriodFilterControl', () => ({
  PeriodFilterControl: (props: { onChange: (value: { start: string; end: string }) => void }) =>
    <button class="lens-filter-trigger" onClick={() => props.onChange({ start: '', end: '' })}>Choose all time</button>,
}))
beforeEach(() => applyURL.mockReset())
afterEach(cleanup)

const calendar = { timezone: 'Asia/Tashkent', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'all' } }
const june = { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }
const editor = (value: ContextFilters, saved: ContextFilters): ContextEditor => ({
  url: '/sales', value: { calendar, dashboard: value }, saved: { calendar, dashboard: saved },
})
const appliedBoard = (call = 0) => decodeFilterBoard(new URL(applyURL.mock.calls[call]![0], 'http://localhost').searchParams.get(filterContextParam)!)
const frame = () => new Promise((resolve) => requestAnimationFrame(resolve))

// False green: remounting the test component would conceal stale mounted scope snapshots.
it.each(['dashboard', 'card', 'section'] as const)('applies %s selections against the active document', (scope) => {
  const next = { period: { mode: 'relative', days: 30 } }
  const [state, setState] = createStore<{ editor: ContextEditor }>({ editor: {
    url: '/sales',
    value: { calendar, dashboard: june, cards: { test: june }, sections: { test: june } },
    saved: { calendar, dashboard: june, cards: { test: june }, sections: { test: june } },
  } })
  render(() => <ScopedFilterControls editor={state.editor} scope={scope} id="test" />)
  setState('editor', 'value', { dashboard: next, cards: { test: next }, sections: { test: next } })
  fireEvent.click(screen.getByText('Choose all time'))
  expect(applyURL).toHaveBeenCalledTimes(1)
  const board = appliedBoard()
  const scoped = scope === 'dashboard' ? board.dashboard : scope === 'card' ? board.cards?.test : board.sections?.test
  expect(scoped).toEqual({ period: { mode: 'all' } })
  const untouched = scope === 'card' ? board.dashboard : board.cards?.test
  expect(untouched).toEqual(next)
})

// False green: asserting only that applyURL ran would pass with a leftover staged Apply step beside it.
it('applies a period selection at once and leaves nothing staged', () => {
  render(() => <ScopedFilterControls editor={editor(june, june)} scope="dashboard" id="dashboard" />)
  fireEvent.click(screen.getByText('Choose all time'))
  expect(applyURL).toHaveBeenCalledTimes(1)
  expect(appliedBoard().dashboard).toEqual({ period: { mode: 'all' } })
  expect(screen.queryByTestId('context-apply')).toBeNull()
  expect(screen.queryByRole('status')).toBeNull()
})

// False green: a selection equal to the applied one must not refetch the document.
it('does not apply a selection that changes nothing', () => {
  render(() => <ScopedFilterControls editor={editor({ period: { mode: 'all' } }, june)} scope="dashboard" id="dashboard" />)
  fireEvent.click(screen.getByText('Choose all time'))
  expect(applyURL).not.toHaveBeenCalled()
})

// False green: a Restore that only stages would leave the saved selection unapplied with no way to apply it.
it('applies the inherit choice and Restore at once', () => {
  render(() => <ScopedFilterControls editor={editor(june, { period: { mode: 'reports' } })} scope="dashboard" id="dashboard" />)
  fireEvent.click(screen.getByTestId('context-reset'))
  expect(applyURL).toHaveBeenCalledTimes(1)
  expect(appliedBoard().dashboard).toEqual({ period: { mode: 'reports' } })
  cleanup()
  render(() => <ScopedFilterControls editor={editor(june, june)} scope="dashboard" id="dashboard" />)
  expect(screen.queryByTestId('context-reset')).toBeNull()
  fireEvent.click(screen.getByTestId('context-inherit'))
  expect(applyURL).toHaveBeenCalledTimes(2)
  expect(appliedBoard(1).dashboard).toEqual({ period: { mode: 'reports' } })
})

// False green: without the remount the operated control never leaves the DOM, so focus could not be lost.
it('returns focus to the operated control after the applied selection remounts the editor', async () => {
  const [state, setState] = createStore<{ editor: ContextEditor }>({ editor: editor(june, june) })
  render(() => <ScopedFilterControls editor={state.editor} scope="dashboard" id="dashboard" />)
  const trigger = screen.getByText('Choose all time')
  trigger.focus()
  fireEvent.click(trigger)
  setState('editor', 'value', 'dashboard', { period: { mode: 'all' } })
  expect(trigger.isConnected).toBe(false)
  await frame()
  await frame()
  expect(document.activeElement).toBe(screen.getByText('Choose all time'))
})

// False green: a natively disabled button would drop focus to the body while the save is pending.
it('keeps focus on Save filter defaults through a rejected save', async () => {
  let reject: (error: Error) => void = () => undefined
  const saveDefaults = vi.fn(() => new Promise<void>((_resolve, fail) => { reject = fail }))
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
