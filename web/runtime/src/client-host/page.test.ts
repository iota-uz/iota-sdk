import { createEffect, lazy } from 'solid-js'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountSolidClientRoute, readClientBootstrap, type ClientRouteContext } from '@iota-uz/sdk/client-host'
import { documentPageData, mountRuntimePage, type RouteHost } from './page.ts'
import { deferred, pageData, pageDocument, tick } from './fixtures.ts'
import { useQuery } from './hooks.ts'
import { query } from '../contracts.ts'
import { t } from '../schema.ts'

const host: RouteHost<ClientRouteContext<unknown>> = {
  readContext: (doc) => readClientBootstrap(doc),
  mount: (options) => mountSolidClientRoute(options),
}

const transport = { invoke: vi.fn(async () => ({ n: 1 })) }
const mountEl = () => document.getElementById('iota-client-route-mount')!

let dispose: (() => void) | undefined
beforeEach(() => { vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => { dispose?.(); dispose = undefined; vi.restoreAllMocks(); document.body.innerHTML = '' })

describe('mountRuntimePage', () => {
  it('loads the module, mounts the page with typed params, locale and theme from the host', async () => {
    pageDocument()
    document.documentElement.dataset.theme = 'dark'
    const seen: { params?: unknown; language?: string; theme?: string } = {}
    const module = {
      policy: {
        component: (props: { params: unknown; route: ClientRouteContext<unknown> }) => {
          seen.params = props.params
          seen.language = props.route.locale?.language
          seen.theme = props.route.theme
          const el = document.createElement('section')
          el.id = 'policy-screen'
          el.textContent = 'Policy 7'
          return el
        },
      },
    }
    const importModule = vi.fn(async () => module)
    dispose = mountRuntimePage({ document, host, transport, importModule })

    expect(mountEl().querySelector('[data-runtime-page-loading]')).not.toBeNull()
    expect(mountEl().getAttribute('aria-busy')).toBe('true')
    await tick()

    expect(importModule).toHaveBeenCalledWith('/_ext/assets/h1h1h1h1/client/policy.js')
    expect(mountEl().querySelector('#policy-screen')?.textContent).toBe('Policy 7')
    expect(mountEl().querySelector('[data-runtime-page-loading]')).toBeNull()
    expect(mountEl().hasAttribute('aria-busy')).toBe(false)
    expect(seen.params).toEqual({ id: 7, flag: true, ref: 'abc' }) // int and bool decoded, uuid kept exact
    expect(seen.language).toBe('ru')
    expect(seen.theme).toBe('dark')
    expect(mountEl().dataset.iotaClientOwner).toBe('solid')
  })

  it('gives pages a working useQuery over the host transport', async () => {
    pageDocument(pageData({ params: {}, paramTypes: undefined }))
    const Count = query('acme.insurance.count', { input: t.object({}), output: t.object({ n: t.int() }) })
    const module = {
      policy: {
        component: () => {
          const q = useQuery(Count, {})
          const el = document.createElement('p')
          createEffect(() => { el.textContent = String(q.data()?.n ?? '…') })
          return el
        },
      },
    }
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => module })
    await new Promise((r) => setTimeout(r, 20))
    expect(mountEl().querySelector('p')?.textContent).toBe('1')
    expect(transport.invoke).toHaveBeenCalledWith('query', 'acme.insurance.count', {}, expect.anything())
  })

  it('disposes on navigation, releasing the mount for the next page', async () => {
    pageDocument()
    const module = { policy: { component: () => document.createElement('i') } }
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => module })
    await tick()
    expect(mountEl().querySelector('i')).not.toBeNull()
    document.dispatchEvent(new Event('iota:client-route-unmount'))
    expect(mountEl().childNodes.length).toBe(0)
    expect(mountEl().dataset.iotaClientOwner).toBeUndefined()
    // a second dispose is a no-op
    expect(() => dispose?.()).not.toThrow()
    // the document can host another runtime page afterwards
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => module })
    await tick()
    expect(mountEl().querySelector('i')).not.toBeNull()
  })

  it('ignores a module that arrives after the page was disposed', async () => {
    pageDocument()
    const late = deferred<Record<string, unknown>>()
    const component = vi.fn(() => document.createElement('b'))
    dispose = mountRuntimePage({ document, host, transport, importModule: () => late.promise })
    dispose()
    late.resolve({ policy: { component } })
    await tick()
    expect(component).not.toHaveBeenCalled()
    expect(mountEl().childNodes.length).toBe(0)
    expect(mountEl().dataset.iotaClientOwner).toBeUndefined()
  })

  it('shows the bundle failure with Retry that starts from a fresh document; a bad module has none', async () => {
    pageDocument()
    const reload = vi.fn()
    dispose = mountRuntimePage({ document, host, transport, reload, importModule: async () => { throw new TypeError('Failed to fetch dynamically imported module') } })
    await tick()
    const alert = mountEl().querySelector('[data-runtime-page-error]')
    expect(alert?.getAttribute('role')).toBe('alert')
    const retry = mountEl().querySelector<HTMLButtonElement>('[data-runtime-page-retry]')
    expect(retry?.textContent).toBe('Retry')
    expect(mountEl().dataset.iotaClientOwner).toBeUndefined()
    retry!.click()
    expect(reload).toHaveBeenCalledTimes(1)
    dispose()

    pageDocument()
    dispose = mountRuntimePage({ document, host, transport, reload, importModule: async () => ({ policy: {} }) })
    await tick()
    expect(mountEl().querySelector('[data-runtime-page-error]')).not.toBeNull()
    expect(mountEl().querySelector('[data-runtime-page-retry]')).toBeNull() // retry only helps loads
    expect(console.error).toHaveBeenCalled()
  })

  it('shows the failure of a lazy page chunk that cannot be fetched, with Retry, instead of an empty page', async () => {
    pageDocument()
    const reload = vi.fn()
    const chunk = deferred<{ default: () => HTMLElement }>()
    const module = { policy: { component: lazy(() => chunk.promise) } }
    dispose = mountRuntimePage({ document, host, transport, reload, importModule: async () => module })
    await tick()
    expect(mountEl().querySelector('[data-runtime-page-error]')).toBeNull() // still loading: nothing to report yet
    chunk.reject(new TypeError('Failed to fetch dynamically imported module'))
    await tick()
    const alert = mountEl().querySelector('[data-runtime-page-error]')
    expect(alert?.getAttribute('role')).toBe('alert')
    expect(alert?.textContent).toContain('The screen could not be loaded.')
    mountEl().querySelector<HTMLButtonElement>('[data-runtime-page-retry]')!.click()
    expect(reload).toHaveBeenCalledTimes(1)
    expect(console.error).toHaveBeenCalled()
    dispose()
    expect(mountEl().childNodes.length).toBe(0)
    expect(mountEl().dataset.iotaClientOwner).toBeUndefined()
  })

  it('reports a page that throws while rendering as an invalid screen (no Retry) and releases its root', async () => {
    pageDocument()
    const module = { policy: { component: () => { throw new Error('boom') } } }
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => module })
    await tick()
    const alert = mountEl().querySelector('[data-runtime-page-error]')
    expect(alert?.textContent).toContain('The screen module is invalid.')
    expect(mountEl().querySelector('[data-runtime-page-retry]')).toBeNull()
    expect(mountEl().dataset.iotaClientOwner).toBeUndefined()
    expect(console.error).toHaveBeenCalled()
  })

  it('uses the default export when the page names none, and rejects a foreign route', async () => {
    pageDocument(pageData({ page: { ...pageData().page, export: undefined }, params: {}, paramTypes: undefined }))
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => ({ default: { component: () => document.createElement('u') } }) })
    await tick()
    expect(mountEl().querySelector('u')).not.toBeNull()
    dispose()

    document.body.innerHTML = '<div id="iota-client-route-mount"></div>'
    expect(documentPageData(document)).toBeUndefined()
    expect(() => mountRuntimePage({ document, host: { ...host, readContext: () => ({ initial: { kind: 'other' }, theme: 'light' } as ClientRouteContext<unknown>) }, transport })).toThrow(/not a runtime page/)
  })

  it('rejects route params that violate the declared types', async () => {
    pageDocument(pageData({ params: { id: 'seven', flag: 'true', ref: 'abc' } }))
    const component = vi.fn(() => document.createElement('b'))
    dispose = mountRuntimePage({ document, host, transport, importModule: async () => ({ policy: { component } }) })
    await tick()
    expect(component).not.toHaveBeenCalled()
    expect(mountEl().querySelector('[data-runtime-page-error]')).not.toBeNull()
  })
})

