import { afterEach, describe, expect, it, vi } from 'vitest'
import type { DashboardDocument } from '../contract'
import { IdlePrefetchQueue } from './idlePrefetch'
import { DocumentCache } from './prefetch'

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

function progressiveDocumentResponse(id: string): Response {
  const document: DashboardDocument = {
    version: '1.0.0', snapshotId: `snapshot-${id}`,
    meta: { dashboardId: id, title: id, generatedAt: '2026-07-22T00:00:00Z', locale: 'en' },
    layout: { rows: [
      { panels: [{ panelId: 'hero', span: 12 }] },
      { panels: [{ panelId: 'lower', span: 12 }] },
    ] },
    panels: [
      { id: 'hero', kind: 'stat', semantics: 'series', title: 'Hero', frame: 'hero-frame', encoding: { value: 'value' }, format: {}, actions: [], terminal: true, deferred: true },
      { id: 'lower', kind: 'stat', semantics: 'series', title: 'Lower', frame: 'lower-frame', encoding: { value: 'value' }, format: {}, actions: [], terminal: true, deferred: true },
    ],
    frames: {}, drill: { inlineDepth: 0, edges: {} }, perspectives: [],
    endpoints: { panel: '/drawer/lens/panel' }, i18n: {}, theme: { palette: {}, series: {} },
  }
  return new Response(JSON.stringify(document), { status: 200, headers: { 'Content-Type': 'application/json' } })
}

function progressivePanelResponse(): Response {
  return new Response([
    JSON.stringify({ panelId: 'hero', result: { frames: { 'hero-frame': { columns: [{ name: 'value', type: 'number' }], rows: [[42]] } } } }),
    JSON.stringify({ complete: true }),
    '',
  ].join('\n'), { status: 200, headers: { 'Content-Type': 'application/x-ndjson' } })
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
  it('prefetches the first drawer row into the browser cache without loading lower rows', async () => {
    const fetcher = vi.fn<typeof fetch>((input, init) => {
      const url = requestUrl(input)
      if (url === '/drawer') {
        expect(new Headers(init?.headers).get('X-Lens-Prefetch')).toBe('intent')
        return Promise.resolve(progressiveDocumentResponse('drawer'))
      }
      expect(url).toBe('/drawer/lens/panel')
      expect(new Headers(init?.headers).get('X-Lens-Prefetch')).toBe('intent')
      expect(JSON.parse(String(init?.body))).toMatchObject({
        snapshotId: 'snapshot-drawer', panels: [{ panelId: 'hero' }],
      })
      return Promise.resolve(progressivePanelResponse())
    })
    const cache = new DocumentCache({ fetcher })

    await cache.prefetch('/drawer')

    const cached = cache.get('/drawer')
    expect(cached?.frames['hero-frame']?.rows).toEqual([[42]])
    expect(cached?.panels.find(({ id }) => id === 'hero')?.deferred).toBe(false)
    expect(cached?.panels.find(({ id }) => id === 'lower')?.deferred).toBe(true)
    expect(fetcher).toHaveBeenCalledTimes(2)

    const opened = await cache.load('/drawer')
    expect(opened.frames['hero-frame']?.rows).toEqual([[42]])
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('cancels document and first-row data warm-up with the speculative scope', async () => {
    const controller = new AbortController()
    let panelAborted = false
    const fetcher = vi.fn<typeof fetch>((input, init) => {
      if (requestUrl(input) === '/drawer') return Promise.resolve(progressiveDocumentResponse('drawer'))
      return new Promise<Response>((resolve) => {
        init?.signal?.addEventListener('abort', () => {
          panelAborted = true
          resolve(new Response('', { status: 499 }))
        }, { once: true })
      })
    })
    const cache = new DocumentCache({ fetcher })

    const pending = cache.prefetch('/drawer', controller.signal)
    await vi.waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2))
    controller.abort()
    await pending

    expect(panelAborted).toBe(true)
    expect(cache.get('/drawer')).toBeUndefined()
  })

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

  it('sends an interactive activation to promote an in-flight prefetch', async () => {
    const resolvers: Array<(response: Response) => void> = []
    const fetcher = vi.fn<typeof fetch>((_input, init) => new Promise<Response>((resolve) => {
      resolvers.push(resolve)
      if (resolvers.length === 1) expect(new Headers(init?.headers).get('X-Lens-Prefetch')).toBe('intent')
      if (resolvers.length === 2) expect(new Headers(init?.headers).get('X-Lens-Prefetch')).toBeNull()
    }))
    const cache = new DocumentCache({ fetcher })

    const prefetch = cache.prefetch('/a')
    const load = cache.load('/a')
    expect(fetcher).toHaveBeenCalledTimes(2)

    resolvers[0]?.(documentResponse('/a'))
    resolvers[1]?.(documentResponse('/a'))
    const loaded = await load
    await prefetch

    expect(loaded.meta.dashboardId).toBe('/a')
    expect(cache.get('/a')).toStrictEqual(loaded)
    expect(fetcher).toHaveBeenCalledTimes(2)
  })

  it('keeps an interactive activation independent from a failed prefetch transport', async () => {
    const resolvers: Array<(response: Response) => void> = []
    const fetcher = vi.fn<typeof fetch>(() => new Promise<Response>((resolve) => { resolvers.push(resolve) }))
    const cache = new DocumentCache({ fetcher })

    const prefetch = cache.prefetch('/a')
    const load = cache.load('/a')
    resolvers[0]?.(new Response('boom', { status: 503 }))
    resolvers[1]?.(documentResponse('/a'))

    await expect(prefetch).resolves.toBeUndefined()
    await expect(load).resolves.toMatchObject({ meta: { dashboardId: '/a' } })
    expect(fetcher).toHaveBeenCalledTimes(2)
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
