import { createRoot } from 'solid-js'
import { describe, expect, it } from 'vitest'
import { copyReportDefinition, createReportDraft, duplicateReportPlacement, parseReportDefinition } from './draft'
import type { ReportDefinition } from './draft'

const definition: ReportDefinition = {
  id: 'revenue', revision: '1', title: 'Revenue', source: { id: 'sales', version: 1 },
  view: 'summary', parameters: { period: { mode: 'relative', duration: '30d' } },
}

// False green: local mutations update the saved object before the host acknowledges a write.
describe('report draft boundary', () => {
  it('keeps a draft private, rolls back all fields, and saves only through the host', async () => {
    let dispose = () => {}
    const writes: unknown[] = []
    const draft = createRoot((cleanup) => {
      dispose = cleanup
      return createReportDraft(definition, { save(value, options) { writes.push({ value, options }); return Promise.resolve({ ...value, revision: '2' }) } })
    })
    try {
      draft.update((value) => ({ ...value, title: 'June', parameters: { period: { mode: 'fixed', start: '2026-06-01', end: '2026-06-30' } } }))
      expect(writes).toHaveLength(0)
      expect(draft.saved()).toEqual(definition)
      expect(draft.dirty()).toBe(true)
      draft.cancel()
      expect(draft.value()).toEqual(definition)
      draft.update((value) => ({ ...value, title: 'Saved June' }))
      await draft.save('publish')
      expect(writes).toEqual([{ value: { ...definition, title: 'Saved June' }, options: { expectedRevision: '1', intent: 'publish' } }])
      expect(draft.saved().revision).toBe('2')
      expect(draft.dirty()).toBe(false)
    } finally { dispose() }
  })

  it('retains local edits on rejection and while an earlier save is in flight', async () => {
    let dispose = () => {}
    let reject = true
    let acknowledge!: (value: ReportDefinition) => void
    const draft = createRoot((cleanup) => {
      dispose = cleanup
      return createReportDraft(definition, { save(value) {
        if (reject) return Promise.reject(new Error('Conflict'))
        return new Promise((resolve) => { acknowledge = (ack) => resolve({ ...value, ...ack }) })
      } })
    })
    try {
      draft.update((value) => ({ ...value, title: 'Edited' }))
      await expect(draft.save()).rejects.toThrow('Conflict')
      expect(draft.value().title).toBe('Edited')
      expect(draft.saved()).toEqual(definition)
      reject = false
      const save = draft.save()
      expect(draft.saving()).toBe(true)
      draft.update((value) => ({ ...value, title: 'Later edit' }))
      acknowledge({ ...definition, title: 'Edited', revision: '2' })
      await save
      expect(draft.saved().title).toBe('Edited')
      expect(draft.value().title).toBe('Later edit')
      expect(draft.value().revision).toBe('2')
      expect(draft.dirty()).toBe(true)
    } finally { dispose() }
  })

  it('distinguishes a placement duplicate from an independent definition copy', () => {
    const placement = { id: 'a', reportId: definition.id, parameters: { product: ['osago'] } }
    const duplicate = duplicateReportPlacement(placement, 'b')
    const copied = copyReportDefinition(definition, 'independent', '1')
    const duplicateProducts = duplicate.parameters!.product as string[]
    duplicateProducts.push('travel')
    copied.title = 'Independent'
    expect(duplicate.reportId).toBe(placement.reportId)
    expect(placement.parameters.product).toEqual(['osago'])
    expect(copied.id).not.toBe(definition.id)
    expect(definition.title).toBe('Revenue')
  })

  it('rejects runtime snapshots as saved report definitions', () => {
    expect(() => parseReportDefinition({ ...definition, snapshotId: 'expired' })).toThrow()
  })
})
