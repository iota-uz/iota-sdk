import { createEffect, createSignal, onCleanup } from 'solid-js'
import type { Filter } from '../contract'
import { useDashboard } from '../runtime'

export interface FacetOption {
  label: string
  value: string
  count?: number
  selected?: boolean
  toggleUrl: string
}

export interface FacetOptionsResponse {
  options: Array<FacetOption>
  /** Producer-owned URL with this dimension removed and every other query value preserved. */
  applyUrl: string
}

export type FacetOptionsStatus = 'idle' | 'loading' | 'error'

export function optionsURL(optionsEndpoint: string, searchParam: string | undefined, search: string): string {
  const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const target = new URL(optionsEndpoint, base)
  const param = searchParam?.trim() || 'q'
  target.searchParams.delete(param)
  if (search.trim()) target.searchParams.set(param, search.trim())
  return `${target.pathname}${target.search}${target.hash}`
}

export function relativeURL(target: URL): string {
  return `${target.pathname}${target.search}${target.hash}`
}

/**
 * The option list behind one facet dimension: fetched on demand, debounced per
 * search term, and cached per resolved URL for the lifetime of the mount.
 *
 * It is a hook rather than a component's private state because the options list
 * is now rendered inside the shared filter menu, where the mounted pane changes
 * as the reader walks the dimensions, while the request/cache contract with the
 * producer stays exactly what the standalone facet control had.
 */
export function useFacetOptions(facet: NonNullable<Filter['facet']> | undefined) {
  const { document: runtimeDocument } = useDashboard()
  const optionsEndpoint = facet?.optionsEndpoint ?? ''
  const searchParam = facet?.searchParam
  const cache = new Map<string, FacetOptionsResponse>()
  const inFlight = new Map<string, Promise<FacetOptionsResponse>>()
  const [search, setSearch] = createSignal('')
  const [options, setOptions] = createSignal<Array<FacetOption>>([])
  const [applyTarget, setApplyTarget] = createSignal('')
  const [status, setStatus] = createSignal<FacetOptionsStatus>('idle')

  const requestOptions = (query: string): Promise<FacetOptionsResponse> => {
    const target = optionsURL(optionsEndpoint, searchParam, query)
    const cached = cache.get(target)
    if (cached) return Promise.resolve(cached)
    const pending = inFlight.get(target)
    if (pending) return pending
    const request = fetch(target, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    }).then(async (response) => {
      if (!response.ok) throw new Error(`facet options failed with ${response.status}`)
      return response.json() as Promise<FacetOptionsResponse>
    }).then((payload) => {
      cache.set(target, payload)
      return payload
    }).finally(() => inFlight.delete(target))
    inFlight.set(target, request)
    return request
  }

  createEffect(() => {
    cache.clear()
    inFlight.clear()
    setOptions([])
    setApplyTarget('')
    setSearch('')
  })

  createEffect(() => {
    if (!optionsEndpoint) return
    const query = search()
    let current = true
    const target = optionsURL(optionsEndpoint, searchParam, query)
    setStatus(cache.has(target) ? 'idle' : 'loading')
    const timer = globalThis.setTimeout(() => {
      void requestOptions(query).then((payload) => {
        if (!current) return
        setOptions(payload.options ?? [])
        setApplyTarget(payload.applyUrl)
        setStatus('idle')
      }).catch(() => {
        if (current) setStatus('error')
      })
    }, query.trim() ? (runtimeDocument.theme.debounceMs ?? 500) : 0)
    onCleanup(() => {
      current = false
      globalThis.clearTimeout(timer)
    })
  })

  return { applyTarget, options, search, setSearch, status }
}
