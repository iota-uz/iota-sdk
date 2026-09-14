import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { render } from 'solid-js/web'
import { buildLazyLoadURL, LazyLoad, type LazyLoadObserver } from './LazyLoad'

let dispose: (() => void) | undefined
const host = document.createElement('div')

beforeEach(() => document.body.append(host))

afterEach(() => {
  dispose?.()
  dispose = undefined
  host.replaceChildren()
  host.remove()
})

describe('LazyLoad', () => {
  it('builds an encoded endpoint and replaces the canonical wrapper with the remote fragment', async () => {
    const adapter = vi.fn(async ({ url }: { url: string }) => `<article data-url="${url}">Loaded</article>`)
    dispose = render(() => <LazyLoad endpoint="/rows" params={{ page: 2, q: 'new item' }} adapter={adapter}>Waiting</LazyLoad>, host)
    expect(host.querySelector('.lazy-load')!.textContent).toBe('Waiting')
    await vi.waitFor(() => expect(host.querySelector('article')?.textContent).toBe('Loaded'))
    expect(buildLazyLoadURL('/rows?sort=name', { page: 2 })).toBe('/rows?sort=name&page=2')
    expect(host.querySelector('.lazy-load')).toBeNull()
    expect(adapter.mock.calls[0]![0].url).toBe('/rows?page=2&q=new+item')
  })

  it('waits for intersection, exposes retry on failure, and succeeds without hard-coded transport', async () => {
    let callback!: IntersectionObserverCallback
    const observer: LazyLoadObserver = { observe: vi.fn(), disconnect: vi.fn() }
    const observerFactory = vi.fn((next: IntersectionObserverCallback) => { callback = next; return observer })
    const adapter = vi.fn()
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce(<p>Recovered</p>)
    dispose = render(() => <LazyLoad endpoint="/panel" trigger="visible" observerFactory={observerFactory} adapter={adapter} />, host)
    expect(adapter).not.toHaveBeenCalled()
    callback([{ isIntersecting: true } as IntersectionObserverEntry], observer as IntersectionObserver)
    await vi.waitFor(() => expect(host.querySelector('[role="alert"]')).not.toBeNull())
    ;(host.querySelector('button') as HTMLButtonElement).click()
    await vi.waitFor(() => expect(host.textContent).toContain('Recovered'))
    expect(observer.disconnect).toHaveBeenCalled()
  })

  it('aborts an in-flight request when disposed', async () => {
    let signal!: AbortSignal
    dispose = render(() => <LazyLoad endpoint="/slow" adapter={({ signal: next }) => { signal = next; return new Promise(() => undefined) }} />, host)
    await vi.waitFor(() => expect(signal).toBeDefined())
    dispose()
    dispose = undefined
    expect(signal.aborted).toBe(true)
  })

  it('reports an unavailable intersection observer through the accessible error state', async () => {
    const onError = vi.fn()
    dispose = render(() => <LazyLoad endpoint="/visible" trigger="visible" onError={onError} />, host)
    await vi.waitFor(() => expect(host.querySelector('[role="alert"]')).not.toBeNull())
    expect(onError).toHaveBeenCalledOnce()
  })
})
