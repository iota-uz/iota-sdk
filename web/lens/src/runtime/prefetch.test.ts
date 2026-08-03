import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument } from '../contract'
import { DocumentCache, IdlePrefetchQueue } from './prefetch'

function requestUrl(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input
  if (input instanceof URL) return input.href
  return input.url
}

function documentResponse(id: string): Response {
  const document: DashboardDocument = {
    version: '1.0.0',
    snapshotId: `snapshot-${id}`,
    meta: { dashboardId: id, title: id, generatedAt: '2026-07-22T00:00:00Z', locale: 'en' },
    layout: { rows: [{ panels: [{ panelId: 'metric', span: 12 }] }] },
    panels: [{
      id: 'metric', kind: 'stat', semantics: 'series', title: id, frame: 'frame',
      encoding: { value: 'value' }, format: {}, actions: [], terminal: true,
    }],
    frames: { frame: { columns: [{ name: 'value', type: 'number' }], rows: [[1]] } },
    drill: { inlineDepth: 0, edges: {} },
    perspectives: [],
    endpoints: {},
    i18n: {},
    theme: { palette: {}, series: {} },
  }
  return new Response(JSON.stringify(document), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('IdlePrefetchQueue', () => {
  it('runs distinct candidates once, in registration order, through one worker', async () => {
    vi.useFakeTimers()
    const started: Array<string> = []
    const queue = new IdlePrefetchQueue({ capacity: 4, delayMs: 0 })
    const run = (key: string) => () => {
      started.push(key)
      return Promise.resolve()
    }

    queue.register('a', run('a'))
    queue.register('b', run('b'))
    queue.register('a', run('duplicate-a'))
    await vi.runAllTimersAsync()

    expect(started).toEqual(['a', 'b'])
  })

  it('bounds admitted work and aborts it on snapshot reset', async () => {
    vi.useFakeTimers()
    const queue = new IdlePrefetchQueue({ capacity: 1, delayMs: 0 })
    let aborted = false
    queue.register('a', (signal) => new Promise<void>((resolve) => {
      signal.addEventListener('abort', () => { aborted = true; resolve() }, { once: true })
    }))
    const ignored = vi.fn<() => Promise<void>>().mockResolvedValue()
    queue.register('b', ignored)
    await vi.advanceTimersByTimeAsync(0)

    queue.reset()
    await Promise.resolve()

    expect(aborted).toBe(true)
    expect(ignored).not.toHaveBeenCalled()
  })
})

describe('DocumentCache', () => {
  it('returns undefined on a miss and the document after a hit', async () => {
    const fetcher = vi.fn<typeof fetch>((input) => Promise.resolve(documentResponse(requestUrl(input))))
    const cache = new DocumentCache({ fetcher })

    expect(cache.get('/a')).toBeUndefined()
    await cache.prefetch('/a')
    expect(cache.get('/a')?.meta.dashboardId).toBe('/a')
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('dedupes concurrent prefetches of the same URL to one in-flight fetch', async () => {
    const fetcher = vi.fn<typeof fetch>((input) => Promise.resolve(documentResponse(requestUrl(input))))
    const cache = new DocumentCache({ fetcher })

    const first = cache.prefetch('/a')
    const second = cache.prefetch('/a')
    await Promise.all([first, second])

    expect(fetcher).toHaveBeenCalledTimes(1)
    expect(cache.get('/a')).toBeDefined()
  })

  it('joins an interactive load to an in-flight prefetch and surfaces its result', async () => {
    let resolveResponse: ((response: Response) => void) | undefined
    const fetcher = vi.fn<typeof fetch>(() => new Promise<Response>((resolve) => { resolveResponse = resolve }))
    const cache = new DocumentCache({ fetcher })

    const prefetch = cache.prefetch('/a')
    const load = cache.load('/a')
    expect(fetcher).toHaveBeenCalledTimes(1)

    resolveResponse?.(documentResponse('/a'))
    const loaded = await load
    await prefetch

    expect(loaded.meta.dashboardId).toBe('/a')
    expect(cache.get('/a')).toBe(loaded)
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('surfaces a failed shared prefetch to an interactive load', async () => {
    let resolveResponse: ((response: Response) => void) | undefined
    const fetcher = vi.fn<typeof fetch>(() => new Promise<Response>((resolve) => { resolveResponse = resolve }))
    const cache = new DocumentCache({ fetcher })

    const prefetch = cache.prefetch('/a')
    const load = cache.load('/a')
    resolveResponse?.(new Response('boom', { status: 503 }))

    await expect(prefetch).resolves.toBeUndefined()
    await expect(load).rejects.toThrow('503')
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('does not refetch a URL that is already cached', async () => {
    const fetcher = vi.fn<typeof fetch>((input) => Promise.resolve(documentResponse(requestUrl(input))))
    const cache = new DocumentCache({ fetcher })

    await cache.prefetch('/a')
    await cache.prefetch('/a')
    expect(fetcher).toHaveBeenCalledTimes(1)
  })

  it('evicts the oldest entry beyond capacity', async () => {
    const fetcher = vi.fn<typeof fetch>((input) => Promise.resolve(documentResponse(requestUrl(input))))
    const cache = new DocumentCache({ capacity: 2, fetcher })

    await cache.prefetch('/a')
    await cache.prefetch('/b')
    await cache.prefetch('/c')

    expect(cache.get('/a')).toBeUndefined()
    expect(cache.get('/b')).toBeDefined()
    expect(cache.get('/c')).toBeDefined()
  })

  it('drops a failed prefetch and never rejects, leaving the URL uncached', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(new Response('boom', { status: 503 }))
    const cache = new DocumentCache({ fetcher })

    await expect(cache.prefetch('/a')).resolves.toBeUndefined()
    expect(cache.get('/a')).toBeUndefined()

    // A later prefetch may retry because the failure left no in-flight entry.
    fetcher.mockResolvedValueOnce(documentResponse('/a'))
    await cache.prefetch('/a')
    expect(cache.get('/a')).toBeDefined()
    expect(fetcher).toHaveBeenCalledTimes(2)
  })
})
