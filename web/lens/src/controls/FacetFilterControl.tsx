import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Filter } from '../contract'
import { CaretDown, X } from '../icons'
import { cubeFilterParam, useDashboard, useFilters, useTranslate } from '../runtime'
import { useFocusTrap } from '../runtime/focusTrap'
import { useOverlayContainer } from '../runtime/overlayContainer'
import { positionPopover } from './PeriodFilterControl'

const minimumFacetBarPercent = 3

interface FacetOption {
  label: string
  value: string
  count?: number
  selected?: boolean
  toggleUrl: string
}

interface FacetOptionsResponse {
  options: Array<FacetOption>
  /** Producer-owned URL with this dimension removed and every other query value preserved. */
  applyUrl: string
}

function optionsURL(optionsEndpoint: string, searchParam: string | undefined, search: string): string {
  const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const target = new URL(optionsEndpoint, base)
  const param = searchParam?.trim() || 'q'
  target.searchParams.delete(param)
  if (search.trim()) target.searchParams.set(param, search.trim())
  return `${target.pathname}${target.search}${target.hash}`
}

function relativeURL(target: URL): string {
  return `${target.pathname}${target.search}${target.hash}`
}

export function FacetFilterControl({ filter }: { filter: Filter }) {
  const facet = filter.facet
  const translate = useTranslate()
  const { document: runtimeDocument } = useDashboard()
  const { applyURL } = useFilters()
  const popoverID = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const cache = useRef(new Map<string, FacetOptionsResponse>())
  const inFlight = useRef(new Map<string, Promise<FacetOptionsResponse>>())
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [options, setOptions] = useState<Array<FacetOption>>([])
  const [applyTarget, setApplyTarget] = useState('')
  const [draft, setDraft] = useState<Set<string>>()
  const [status, setStatus] = useState<'idle' | 'loading' | 'error'>('idle')
  const [position, setPosition] = useState({ left: 0, top: 0 })
  const optionsEndpoint = facet?.optionsEndpoint ?? ''
  const searchParam = facet?.searchParam
  const closePopover = useCallback(() => setOpen(false), [])
  const container = useOverlayContainer(open, triggerRef)
  useFocusTrap(popoverRef, open && Boolean(container), closePopover, searchRef, triggerRef)

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

  const warmOptions = useCallback(() => {
    if (optionsEndpoint) void requestOptions('').catch(() => undefined)
  }, [optionsEndpoint, requestOptions])

  useEffect(() => {
    cache.current.clear()
    inFlight.current.clear()
    setOptions([])
    setApplyTarget('')
    setDraft(undefined)
    setSearch('')
  }, [optionsEndpoint])

  useEffect(() => {
    if (!open || !optionsEndpoint) return undefined
    let current = true
    const target = optionsURL(optionsEndpoint, searchParam, search)
    setStatus(cache.current.has(target) ? 'idle' : 'loading')
    const timer = globalThis.setTimeout(() => {
      void requestOptions(search).then((payload) => {
        if (!current) return
        setOptions(payload.options ?? [])
        setApplyTarget(payload.applyUrl)
        setDraft((existing) => existing ?? new Set(
          (payload.options ?? []).filter((option) => option.selected).map((option) => option.value),
        ))
        setStatus('idle')
      }).catch(() => {
        if (current) setStatus('error')
      })
    }, search.trim() ? (runtimeDocument.theme.debounceMs ?? 500) : 0)
    return () => {
      current = false
      globalThis.clearTimeout(timer)
    }
  }, [open, optionsEndpoint, requestOptions, runtimeDocument.theme.debounceMs, search, searchParam])

  useEffect(() => {
    if (open) return
    setSearch('')
    setDraft(undefined)
  }, [open])

  const reposition = useCallback(() => {
    const trigger = triggerRef.current
    const popover = popoverRef.current
    if (!trigger || !popover) return
    const anchor = trigger.getBoundingClientRect()
    const size = popover.getBoundingClientRect()
    setPosition(positionPopover(
      anchor,
      { width: size.width, height: size.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    ))
  }, [])

  useLayoutEffect(() => {
    if (container) reposition()
  }, [container, reposition])

  useEffect(() => {
    if (!container) return undefined
    const frame = globalThis.requestAnimationFrame(reposition)
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (popoverRef.current) observer?.observe(popoverRef.current)
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    return () => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    }
  }, [container, reposition])

  useEffect(() => {
    if (!open) return undefined
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      const root = triggerRef.current?.closest('.lens-filter-facet')
      if (!root?.contains(target) && !popoverRef.current?.contains(target)) setOpen(false)
    }
    globalThis.document.addEventListener('pointerdown', onPointerDown)
    return () => {
      globalThis.document.removeEventListener('pointerdown', onPointerDown)
    }
  }, [container, open])

  if (!facet) return null
  const selectedCount = facet.selections?.length ?? 0
  const selected = draft ?? new Set(options.filter((option) => option.selected).map((option) => option.value))
  const ordered = options
  const maxCount = Math.max(0, ...options.map(({ count }) => count ?? 0))

  const toggleDraft = (value: string) => {
    setDraft((current) => {
      const next = new Set(current ?? selected)
      if (next.has(value)) next.delete(value)
      else next.add(value)
      return next
    })
  }
  const applyDraft = (values: ReadonlySet<string>) => {
    const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
    const target = new URL(applyTarget || window.location.href, base)
    target.searchParams.delete(cubeFilterParam)
    // applyTarget is producer-owned and already preserves every unrelated
    // cube filter. Re-add those, then append this dimension's staged OR-set.
    const producer = new URL(applyTarget || window.location.href, base)
    for (const value of producer.searchParams.getAll(cubeFilterParam)) target.searchParams.append(cubeFilterParam, value)
    for (const value of values) target.searchParams.append(cubeFilterParam, `${facet.dimension}:${value}`)
    setOpen(false)
    applyURL(relativeURL(target))
  }

  return (
    <div className="lens-filter-facet">
      <button
        aria-controls={popoverID}
        aria-expanded={open}
        className="lens-facet-trigger"
        onClick={() => setOpen((value) => !value)}
        onFocus={warmOptions}
        onPointerEnter={warmOptions}
        ref={triggerRef}
        type="button"
      >
        <span>{filter.label}</span>
        {selectedCount > 0 && <span className="lens-facet-count">{selectedCount}</span>}
        <CaretDown aria-hidden="true" />
      </button>
      {open && container && createPortal(
        <div
          aria-label={filter.label}
          aria-modal="true"
          className="lens-facet-popover"
          id={popoverID}
          ref={popoverRef}
          role="dialog"
          style={{ left: position.left, top: position.top }}
        >
          <input
            aria-label={translate('filter.facet.search', 'Search options')}
            className="lens-facet-search"
            onChange={(event) => setSearch(event.target.value)}
            placeholder={translate('filter.facet.search', 'Search options')}
            ref={searchRef}
            type="search"
            value={search}
          />
          <div className="lens-facet-options">
            {status === 'loading' ? (
              <div className="lens-facet-state">{translate('filter.facet.loading', 'Loading options…')}</div>
            ) : status === 'error' ? (
              <div className="lens-facet-state lens-facet-error">{translate('filter.facet.error', 'Options could not be loaded')}</div>
            ) : ordered.length === 0 ? (
              <div className="lens-facet-state">{translate('filter.facet.empty', 'No options')}</div>
            ) : ordered.map((option) => {
              const checked = selected.has(option.value)
              const magnitude = maxCount > 0 && (option.count ?? 0) > 0
                ? Math.max(minimumFacetBarPercent, Math.floor((option.count ?? 0) * 100 / maxCount))
                : 0
              return (
                <label className="lens-facet-option" key={`${option.value}-${option.toggleUrl}`}>
                  {magnitude > 0 && <span aria-hidden="true" className="lens-facet-option-bar" style={{ width: `${magnitude}%` }} />}
                  <input
                    checked={checked}
                    onChange={() => toggleDraft(option.value)}
                    type="checkbox"
                  />
                  <span className="lens-facet-option-label">{option.label}</span>
                  {(option.count ?? 0) > 0 && <span className="lens-facet-option-count">{option.count}</span>}
                </label>
              )
            })}
          </div>
          <div className="lens-facet-actions">
            <button onClick={() => setDraft(new Set())} type="button">{translate('filter.facet.clear', 'Clear')}</button>
            <button className="lens-facet-apply" disabled={!applyTarget} onClick={() => applyDraft(selected)} type="button">
              {translate('filter.facet.apply', 'Apply')}
            </button>
          </div>
        </div>,
        container,
      )}
      {facet.selections?.map((selection) => (
        <a
          aria-label={`${translate('filter.facet.remove', 'Remove filter')}: ${selection.label}`}
          className="lens-facet-active-chip"
          href={selection.removeUrl}
          onClick={(event) => {
            if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
            event.preventDefault()
            applyURL(selection.removeUrl)
          }}
          key={`${selection.label}-${selection.removeUrl}`}
        >
          <span>{selection.label}</span>
          <X aria-hidden="true" />
        </a>
      ))}
    </div>
  )
}
