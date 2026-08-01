import { describe, expect, it, vi } from 'vitest'
import { PanelClient } from './panel'

const frame = { columns: [{ name: 'value', type: 'number' as const }], rows: [[1]] }
const result = { frames: { 'panel:a': frame } }

function client(events: unknown[]) {
  const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(
    `${events.map((event) => JSON.stringify(event)).join('\n')}\n`,
    { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } },
  ))
  return { client: new PanelClient('/panels', { fetcher }), fetcher }
}

describe('PanelClient NDJSON protocol', () => {
  it('delivers validated results incrementally and accepts one terminal marker', async () => {
    const { client: panelClient, fetcher } = client([
      { panelId: 'a', result },
      { complete: true },
    ])
    const delivered = vi.fn()
    await expect(panelClient.loadBatch([{ snapshotId: 'snapshot', panelId: 'a', page: 3 }], { onResult: delivered }))
      .resolves.toEqual({ a: result })
    expect(delivered).toHaveBeenCalledWith('a', result)
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(JSON.parse(fetcher.mock.calls[0]?.[1]?.body as string)).toMatchObject({ panels: [{ panelId: 'a', page: 3 }] })
  })

  it.each([
    ['clean EOF before completion', [{ panelId: 'a', result }]],
    ['missing requested panel', [{ complete: true }]],
    ['duplicate panel', [{ panelId: 'a', result }, { panelId: 'a', result }, { complete: true }]],
    ['unrequested panel', [{ panelId: 'b', result }, { complete: true }]],
    ['mixed completion/result variant', [{ panelId: 'a', result, complete: true }]],
  ])('rejects %s', async (_name, events) => {
    const { client: panelClient } = client(events)
    await expect(panelClient.loadBatch([{ snapshotId: 'snapshot', panelId: 'a' }])).rejects.toThrow()
  })
})
