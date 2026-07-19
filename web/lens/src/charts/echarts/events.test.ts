import { describe, expect, it } from 'vitest'
import { nodeKeyFromEvent } from './events'

describe('nodeKeyFromEvent', () => {
  it('emits only the stable NodeKey attached to chart data', () => {
    expect(nodeKeyFromEvent({ data: { nodeKey: 'region/central', value: 42 } })).toBe('region/central')
  })

  it('never falls back to an index or localized label', () => {
    expect(nodeKeyFromEvent({ data: { value: 42 }, dataIndex: 3, name: 'Central' } as never)).toBeUndefined()
    expect(nodeKeyFromEvent({ data: { nodeKey: 3 } })).toBeUndefined()
  })
})
