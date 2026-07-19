import { describe, expect, it, vi } from 'vitest'
import fixture from '../../fixtures/small.json'
import { parseDocument, type DashboardDocument, type QueryResponse } from '../contract'
import { SnapshotGoneError } from './query'
import { queryWithSnapshotRecovery } from './recovery'

function runtimeDocument(snapshotId: string, includeDetail = true): DashboardDocument {
  return parseDocument({
    ...fixture,
    snapshotId,
    panels: [{ ...fixture.panels[0], drillRoot: 'root' }],
    drill: {
      inlineDepth: 0,
      edges: {
        root: {
          path: ['root'], label: 'Root', perspectives: [],
          children: includeDetail ? [{ key: 'detail', path: ['root', 'detail'], label: 'Detail', target: 'detail' }] : [],
        },
        ...(includeDetail ? { detail: { path: ['root', 'detail'], label: 'Detail', children: [], perspectives: [] } } : {}),
      },
    },
  })
}

const response: QueryResponse = { frames: { detail: { columns: [], rows: [] } } }

describe('queryWithSnapshotRecovery', () => {
  it('silently refreshes the document and replays a resolvable path', async () => {
    const fresh = runtimeDocument('fresh')
    const loadDocument = vi.fn().mockResolvedValue(fresh)
    const query = vi.fn()
      .mockRejectedValueOnce(new SnapshotGoneError())
      .mockResolvedValueOnce(response)

    const result = await queryWithSnapshotRecovery({
      request: { snapshotId: 'expired', path: ['root', 'detail'] },
      navigation: { panelId: 'total', path: ['root', 'detail'] },
      loadDocument,
      query,
    })

    expect(result).toEqual(expect.objectContaining({ document: fresh, reset: false, response }))
    expect(loadDocument).toHaveBeenCalledTimes(1)
    expect(query).toHaveBeenLastCalledWith(expect.objectContaining({ snapshotId: 'fresh', path: ['root', 'detail'] }))
  })

  it('resets to the panel root when the path disappeared', async () => {
    const fresh = runtimeDocument('fresh', false)
    const result = await queryWithSnapshotRecovery({
      request: { snapshotId: 'expired', path: ['root', 'detail'] },
      navigation: { panelId: 'total', path: ['root', 'detail'] },
      loadDocument: vi.fn().mockResolvedValue(fresh),
      query: vi.fn().mockRejectedValueOnce(new SnapshotGoneError()),
    })

    expect(result).toEqual({ document: fresh, navigation: { panelId: 'total', path: ['root'] }, reset: true })
  })

  it('does not refresh for ordinary query errors', async () => {
    const loadDocument = vi.fn()
    await expect(queryWithSnapshotRecovery({
      request: { snapshotId: 'current', path: ['root'] },
      navigation: { path: ['root'] },
      loadDocument,
      query: vi.fn().mockRejectedValue(new Error('network failed')),
    })).rejects.toThrow('network failed')
    expect(loadDocument).not.toHaveBeenCalled()
  })
})
