import { describe, expect, it } from 'vitest'
import type { Frame, Panel } from '../contract'
import { inlineTableSummary } from './tableSummary'

const frame: Frame = {
  columns: [{ name: 'id', type: 'string' }, { name: 'amount', type: 'number' }, { name: 'count', type: 'number' }],
  rows: [['a', -200, 1], ['b', -1.5, 2], ['c', null, 3]],
}

function table(overrides: Record<string, unknown> = {}): Panel {
  return {
    id: 'ops', kind: 'table', title: 'Operations', semantics: 'evidence', frame: 'ops:root',
    encoding: { id: 'id' }, format: {}, actions: [],
    columns: [
      { field: 'id', label: 'Id', cell: { kind: 'plain' } },
      { field: 'amount', label: 'Amount', cell: { kind: 'plain' }, total: true },
      { field: 'count', label: 'Count', cell: { kind: 'plain' } },
    ],
    ...overrides,
  } as Panel
}

describe('inlineTableSummary', () => {
  it('sums every total column of a complete inline frame', () => {
    expect(inlineTableSummary(table(), frame)).toEqual({ values: { amount: -201.5 }, filteredRows: 3, totalRows: 3 })
  })

  it('leaves tables without a total column, deferred panels and missing frames alone', () => {
    const noTotals = table({ columns: [{ field: 'id', label: 'Id', cell: { kind: 'plain' } }] })
    expect(inlineTableSummary(noTotals, frame)).toBeUndefined()
    expect(inlineTableSummary(table({ deferred: true }), frame)).toBeUndefined()
    expect(inlineTableSummary(table(), undefined)).toBeUndefined()
    expect(inlineTableSummary(table({ kind: 'bar' }), frame)).toBeUndefined()
  })
})
