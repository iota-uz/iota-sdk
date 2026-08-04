import { describe, expect, it } from 'vitest'
import type { Panel } from '../contract'
import { retargetPanel } from './kinds'

describe('retargetPanel', () => {
  it('drops configuration owned by the previous renderer kind', () => {
    const table: Panel = {
      id: 'evidence', kind: 'table', title: 'Evidence', semantics: 'evidence', frame: 'evidence:root',
      encoding: { label: 'name', value: 'value' }, format: {}, actions: [],
      columns: [{ field: 'name', label: 'Name', cell: { kind: 'plain' } }],
      table: { searchable: true }, terminal: true,
    }

    const retargeted = retargetPanel(table, 'bar')
    expect(retargeted).not.toHaveProperty('columns')
    expect(retargeted).not.toHaveProperty('table')
  })

  it('preserves configuration when the renderer kind remains compatible', () => {
    const gauge: Panel = {
      id: 'progress', kind: 'gauge', title: 'Progress', semantics: 'series', frame: 'progress:root',
      encoding: { value: 'value' }, format: {}, actions: [], radial: { mode: 'progress', max: 100 }, terminal: true,
    }

    expect(retargetPanel(gauge, 'radial')).toMatchObject({ kind: 'radial', radial: gauge.radial })
  })
})
