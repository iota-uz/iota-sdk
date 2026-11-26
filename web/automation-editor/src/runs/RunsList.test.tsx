import { render } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it } from 'vitest'
import { catalog } from '../../fixture/catalog'
import { sample } from '../../fixture/graphs'
import type { RunSummary } from '../types'
import { RunsList } from './RunsList'

const at = '2026-11-20T09:00:00Z'
const row = (id: string, state: RunSummary['state'] = 'running'): RunSummary => ({
  run_id: id, version: 3, state, subject: null, trigger: 'user_created', error: null, created_at: at, updated_at: at,
})

describe('RunsList (P4, #1352)', () => {
  it('keeps every row element when the list is read again and animates only a run it did not have', () => {
    const [items, setItems] = createSignal<RunSummary[] | undefined>([row('r1'), row('r2')])
    const view = render(() => (
      <RunsList
        items={items()} error={false} refreshedAt={null} selected="r1" states={[]} subject=""
        graph={sample} catalog={catalog} locale="en" now={Date.parse(at)} more={false} loadingMore={false}
        onOpen={() => {}} onStates={() => {}} onSubject={() => {}} onReset={() => {}} onRetry={() => {}} onMore={() => {}}
      />
    ))
    const el = (id: string) => view.container.querySelector<HTMLElement>(`[data-run="${id}"]`)
    const r1 = el('r1')!
    const r2 = el('r2')!
    // The first rows of a list do not animate.
    expect(r1.classList.contains('iota-enter')).toBe(false)
    expect(r2.classList.contains('iota-enter')).toBe(false)
    const state = r2.querySelector('.ae-runstate')!

    // A re-read: new objects for the same runs, r2 finished, a new run on top.
    setItems([row('r3'), row('r1'), row('r2', 'done')])
    expect(el('r1')).toBe(r1)
    expect(el('r2')).toBe(r2)
    expect(el('r3')!.classList.contains('iota-enter')).toBe(true)
    expect(r1.classList.contains('iota-enter')).toBe(false)
    // The changed state fades in place; the unchanged one is the same element.
    expect(r2.querySelector('.ae-runstate')!.textContent).toBe('Done')
    expect(r2.querySelector('.ae-swap--in')).not.toBeNull()
    expect(r2.querySelector('.ae-runstate')).not.toBe(state)
    expect(r1.querySelector('.ae-swap--in')).toBeNull()
    // The order follows the list.
    expect([...view.container.querySelectorAll('[data-run]')].map((e) => e.getAttribute('data-run'))).toEqual(['r3', 'r1', 'r2'])
  })
})
