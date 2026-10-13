import { createRoot } from 'solid-js'
import { createStore } from 'solid-js/store'
import { describe, expect, it, vi } from 'vitest'
import { createFilterContextDraft, encodeFilterBoard, filterBoardURL } from './filterDraft'
import type { ContextBoard } from '../contract/types'

// False green: observing only draft fields would miss accidental host writes on Apply.
describe('scoped filter drafts', () => {
  it('stages, applies temporarily, and restores only acknowledged saved defaults', async () => {
    const apply = vi.fn()
    const saveDefaults = vi.fn().mockRejectedValueOnce(new Error('Conflict')).mockResolvedValue(undefined)
    let dispose = () => {}
    const draft = createRoot((cleanup) => {
      dispose = cleanup
      return createFilterContextDraft({ period: { mode: 'all' } }, { period: { mode: 'relative', days: 30 } }, { apply, saveDefaults })
    })
    try {
      draft.update(() => ({ period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } }))
      expect(apply).not.toHaveBeenCalled()
      expect(saveDefaults).not.toHaveBeenCalled()
      draft.apply()
      expect(apply).toHaveBeenCalledTimes(1)
      expect(saveDefaults).not.toHaveBeenCalled()
      await expect(draft.saveDefaults()).rejects.toThrow('Conflict')
      expect(draft.value().period?.mode).toBe('fixed')
      draft.reset()
      expect(draft.value().period).toEqual({ mode: 'relative', days: 30 })
      await draft.saveDefaults()
      draft.update(() => ({ period: { mode: 'all' } }))
      draft.reset()
      expect(draft.value().period).toEqual({ mode: 'relative', days: 30 })
      expect(apply).toHaveBeenCalledTimes(1)
    } finally { dispose() }
  })

  // False green: plain objects would not reproduce browser DataCloneError from a Solid store.
  it('takes immutable copies from a nested Solid store without mutating its document', () => {
    const [store] = createStore({ cards: { june: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } } } })
    createRoot((dispose) => {
      const draft = createFilterContextDraft(store.cards.june, {}, { apply: () => undefined })
      draft.update(() => ({ period: { mode: 'all' } }))
      expect(store.cards.june.period.mode).toBe('fixed')
      expect(draft.value().period?.mode).toBe('all')
      dispose()
    })
  })

  it('keeps independent scopes and relative selections in the transport', () => {
    const board: ContextBoard = {
      calendar: { timezone: 'Asia/Tashkent', asOf: '2026-07-01T00:00:00Z', defaultPeriod: { mode: 'relative', days: 30 } },
      dashboard: { period: { mode: 'reports' } },
      cards: { june: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } } },
      sections: { insurance: { constraints: [{ dimension: 'product', values: ['osago'] }] } },
    }
    const encoded = encodeFilterBoard(board)
    expect(JSON.parse(atob(encoded.replaceAll('-', '+').replaceAll('_', '/')))).toEqual(board)
    const url = new URL(filterBoardURL('/sales?_f=product:travel&lens-state=old&compare=off', board), window.location.href)
    expect(url.searchParams.has('_f')).toBe(false)
    expect(url.searchParams.has('lens-state')).toBe(false)
    expect(url.searchParams.get('compare')).toBe('off')
    expect(url.searchParams.get('lens-filters')).toBe(encoded)
  })
})
