import { describe, expect, it, vi } from 'vitest'
import type { QueryRequest } from '../contract'
import { QueryClient, SnapshotGoneError, queryCacheKey } from './query'

const request: QueryRequest = {
  snapshotId: 'snapshot-1',
  path: ['root', 'detail'],
  perspective: 'composition',
  page: 2,
}

const response = {
  frames: {
    detail: {
      columns: [{ name: 'value', type: 'number' as const }],
      rows: [[42]],
    },
  },
  page: { number: 2, size: 50 },
}

describe('QueryClient', () => {
  it('deduplicates in-flight requests and returns cache hits without fetching', async () => {
    let resolveResponse: ((value: Response) => void) | undefined
    const fetcher = vi.fn(() => new Promise<Response>((resolve) => { resolveResponse = resolve }))
    const client = new QueryClient('/lens/query', { csrf: 'token', fetcher })

    const first = client.query(request)
    const second = client.query({ ...request, path: [...request.path] })
    expect(fetcher).toHaveBeenCalledTimes(1)
    resolveResponse?.(new Response(JSON.stringify(response), { status: 200 }))

    await expect(first).resolves.toEqual(response)
    await expect(second).resolves.toEqual(response)
    await expect(client.query(request)).resolves.toEqual(response)
    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(fetcher).toHaveBeenCalledWith('/lens/query', expect.objectContaining({
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': 'token' },
      body: JSON.stringify(request),
    }))
  })

  it('keys every snapshot-scoped dimension', () => {
    const keys = [
      request,
      { ...request, snapshotId: 'snapshot-2' },
      { ...request, path: ['root'] },
      { ...request, perspective: 'evidence' },
      { ...request, page: 3 },
    ].map(queryCacheKey)
    expect(new Set(keys)).toHaveLength(keys.length)
  })

  it('recognizes only the exact 410 snapshot_gone protocol', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response(JSON.stringify({
      error: 'snapshot_gone', message: 'expired',
    }), { status: 410 }))
    const client = new QueryClient('/lens/query', { fetcher })
    await expect(client.query(request)).rejects.toEqual(expect.objectContaining<Partial<SnapshotGoneError>>({
      name: 'SnapshotGoneError', code: 'snapshot_gone', status: 410,
    }))
  })

  it('aborts pending work when disposed', () => {
    let signal: AbortSignal | undefined
    const fetcher = vi.fn<typeof fetch>().mockImplementation((_input, init) => {
      signal = init?.signal as AbortSignal
      return new Promise<Response>(() => undefined)
    })
    const client = new QueryClient('/lens/query', { fetcher })
    void client.query(request)
    client.dispose()
    expect(signal?.aborted).toBe(true)
  })
})
