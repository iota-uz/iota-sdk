import { useEffect, useId, useRef, useState } from 'react'
import type { Filter } from '../contract'
import { CaretDown, X } from '../icons'
import { useTranslate } from '../runtime'

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

function optionsURL(filter: Filter, search: string): string {
  const facet = filter.facet
  if (!facet) return ''
  const base = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const target = new URL(facet.optionsEndpoint, base)
  const param = facet.searchParam?.trim() || 'q'
  target.searchParams.delete(param)
  if (search.trim()) target.searchParams.set(param, search.trim())
  return `${target.pathname}${target.search}${target.hash}`
}

export function FacetFilterControl({ filter }: { filter: Filter }) {
  const facet = filter.facet
  const translate = useTranslate()
  const popoverID = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [options, setOptions] = useState<Array<FacetOption>>([])
  const [status, setStatus] = useState<'idle' | 'loading' | 'error'>('idle')

  useEffect(() => {
    if (!open || !facet) return undefined
    const controller = new AbortController()
    setStatus('loading')
    const timer = globalThis.setTimeout(() => {
      void fetch(optionsURL(filter, search), {
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
  }, [facet, filter, open, search])

  useEffect(() => {
    if (!open) return undefined
    searchRef.current?.focus()
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target
      if (!(target instanceof Node)) return
      const root = triggerRef.current?.closest('.lens-filter-facet')
      if (!root?.contains(target)) setOpen(false)
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
  }, [open])

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
      {open && (
        <div className="lens-facet-popover" id={popoverID} role="dialog" aria-label={filter.label}>
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
        </div>
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
