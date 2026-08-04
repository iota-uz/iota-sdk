import { useCallback, useEffect, useRef, useState } from 'react'
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
  const cache = useRef(new Map<string, FacetOptionsResponse>())
  const inFlight = useRef(new Map<string, Promise<FacetOptionsResponse>>())
  const [search, setSearch] = useState('')
  const [options, setOptions] = useState<Array<FacetOption>>([])
  const [applyTarget, setApplyTarget] = useState('')
  const [status, setStatus] = useState<FacetOptionsStatus>('idle')

  const requestOptions = useCallback((query: string): Promise<FacetOptionsResponse> => {
    const target = optionsURL(optionsEndpoint, searchParam, query)
    const cached = cache.current.get(target)
    if (cached) return Promise.resolve(cached)
    const pending = inFlight.current.get(target)
    if (pending) return pending
    const request = fetch(target, {
      credentials: 'same-origin',
      headers: { Accept: 'application/json' },
    }).then(async (response) => {
      if (!response.ok) throw new Error(`facet options failed with ${response.status}`)
      return response.json() as Promise<FacetOptionsResponse>
    }).then((payload) => {
      cache.current.set(target, payload)
      return payload
    }).finally(() => inFlight.current.delete(target))
    inFlight.current.set(target, request)
    return request
  }, [optionsEndpoint, searchParam])

  useEffect(() => {
    cache.current.clear()
    inFlight.current.clear()
    setOptions([])
    setApplyTarget('')
    setSearch('')
  }, [optionsEndpoint])

  useEffect(() => {
    if (!optionsEndpoint) return undefined
    let current = true
    const target = optionsURL(optionsEndpoint, searchParam, search)
    setStatus(cache.current.has(target) ? 'idle' : 'loading')
    const timer = globalThis.setTimeout(() => {
      void requestOptions(search).then((payload) => {
        if (!current) return
        setOptions(payload.options ?? [])
        setApplyTarget(payload.applyUrl)
        setStatus('idle')
      }).catch(() => {
        if (current) setStatus('error')
      })
    }, search.trim() ? (runtimeDocument.theme.debounceMs ?? 500) : 0)
    return () => {
      current = false
      globalThis.clearTimeout(timer)
    }
  }, [optionsEndpoint, requestOptions, runtimeDocument.theme.debounceMs, search, searchParam])

  return { applyTarget, options, search, setSearch, status }
}
