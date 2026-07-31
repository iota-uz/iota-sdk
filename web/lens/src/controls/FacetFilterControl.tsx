import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Filter } from '../contract'
import { CaretDown, X } from '../icons'
import { useTranslate } from '../runtime'
import { positionPopover } from './PeriodFilterControl'

const searchDelay = 250

interface FacetOption {
  label: string
  count?: number
  selected?: boolean
  toggleUrl: string
}

interface FacetOptionsResponse {
  options: Array<FacetOption>
}

function optionsURL(optionsEndpoint: string, searchParam: string | undefined, search: string): string {
  const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const target = new URL(optionsEndpoint, base)
  const param = searchParam?.trim() || 'q'
  target.searchParams.delete(param)
  if (search.trim()) target.searchParams.set(param, search.trim())
  return `${target.pathname}${target.search}${target.hash}`
}

export function FacetFilterControl({ filter }: { filter: Filter }) {
  const facet = filter.facet
  const translate = useTranslate()
  const popoverID = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const popoverRef = useRef<HTMLDivElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [options, setOptions] = useState<Array<FacetOption>>([])
  const [status, setStatus] = useState<'idle' | 'loading' | 'error'>('idle')
  const [container, setContainer] = useState<HTMLElement>()
  const [position, setPosition] = useState({ left: 0, top: 0 })
  const optionsEndpoint = facet?.optionsEndpoint ?? ''
  const searchParam = facet?.searchParam

  useEffect(() => {
    if (!open || !optionsEndpoint) return undefined
    const controller = new AbortController()
    setStatus('loading')
    const timer = globalThis.setTimeout(() => {
      void fetch(optionsURL(optionsEndpoint, searchParam, search), {
        credentials: 'same-origin',
        headers: { Accept: 'application/json' },
        signal: controller.signal,
      }).then(async (response) => {
        if (!response.ok) throw new Error(`facet options failed with ${response.status}`)
        return response.json() as Promise<FacetOptionsResponse>
      }).then((payload) => {
        if (controller.signal.aborted) return
        setOptions(payload.options ?? [])
        setStatus('idle')
      }).catch(() => {
        if (!controller.signal.aborted) setStatus('error')
      })
    }, searchDelay)
    return () => {
      globalThis.clearTimeout(timer)
      controller.abort()
    }
  }, [open, optionsEndpoint, search, searchParam])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const element = document.createElement('div')
    const root = triggerRef.current?.closest('.lens-root')
    element.className = `lens-root lens-overlay-root${root?.classList.contains('dark') ? ' dark' : ''}`
    if (root instanceof HTMLElement && root.dataset.theme) element.dataset.theme = root.dataset.theme
    document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
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
    searchRef.current?.focus()
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      const root = triggerRef.current?.closest('.lens-filter-facet')
      if (!root?.contains(target) && !popoverRef.current?.contains(target)) setOpen(false)
    }
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setOpen(false)
      triggerRef.current?.focus()
    }
    globalThis.document.addEventListener('pointerdown', onPointerDown)
    globalThis.document.addEventListener('keydown', onKeyDown)
    return () => {
      globalThis.document.removeEventListener('pointerdown', onPointerDown)
      globalThis.document.removeEventListener('keydown', onKeyDown)
    }
  }, [container, open])

  if (!facet) return null
  const selectedCount = facet.selections?.length ?? 0
  return (
    <div className="lens-filter-facet">
      <button
        aria-controls={popoverID}
        aria-expanded={open}
        className="lens-facet-trigger"
        onClick={() => setOpen((value) => !value)}
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
          <div aria-live="polite" className="lens-facet-options">
            {status === 'loading' ? (
              <div className="lens-facet-state">{translate('filter.facet.loading', 'Loading options…')}</div>
            ) : status === 'error' ? (
              <div className="lens-facet-state lens-facet-error">{translate('filter.facet.error', 'Options could not be loaded')}</div>
            ) : options.length === 0 ? (
              <div className="lens-facet-state">{translate('filter.facet.empty', 'No options')}</div>
            ) : options.map((option) => (
              <a
                aria-current={option.selected ? 'true' : undefined}
                className="lens-facet-option"
                href={option.toggleUrl}
                key={`${option.label}-${option.toggleUrl}`}
              >
                <span className="lens-facet-check" aria-hidden="true">{option.selected ? '✓' : ''}</span>
                <span className="lens-facet-option-label">{option.label}</span>
                {(option.count ?? 0) > 0 && <span className="lens-facet-option-count">{option.count}</span>}
              </a>
            ))}
          </div>
        </div>,
        container,
      )}
      {facet.selections?.map((selection) => (
        <a
          aria-label={`${translate('filter.facet.remove', 'Remove filter')}: ${selection.label}`}
          className="lens-facet-active-chip"
          href={selection.removeUrl}
          key={`${selection.label}-${selection.removeUrl}`}
        >
          <span>{selection.label}</span>
          <X aria-hidden="true" />
        </a>
      ))}
    </div>
  )
}
