import {
  useCallback, useEffect, useLayoutEffect, useRef, useState,
  type KeyboardEvent, type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import type { FieldFormat } from '../contract'
import { ArrowUpRight, CaretRight, X } from '../icons'
import { useFormat, useTranslate } from '../runtime'
import type { DrillTarget } from './model'

export interface DrillOverlayAnchor {
  x: number
  y: number
}

export interface DrillPathStep {
  label: string
  current: boolean
  onSelect: () => void
}

export interface DrillOverlayProps {
  target: DrillTarget
  /**
   * Live source of the anchor point, when the overlay was opened from an
   * element rather than a pointer. The element is re-measured whenever the
   * layout can still move (web fonts landing, the expanded-panel dialog
   * mounting, a resize), because a rect read at click time is a snapshot of a
   * layout that has not settled yet.
   */
  anchorElement?: HTMLElement | null
  /** Ancestors of the current level; the header only shows the last one. */
  path?: Array<DrillPathStep>
  anchor: DrillOverlayAnchor
  valueFormat?: FieldFormat
  theme?: string
  dark?: boolean
  selectedPerspectiveId?: string
  onDrillInto: (target: DrillTarget) => void
  onDrillChild: (childKey: string) => void
  onPerspective: (perspectiveId: string) => void
  onClose: () => void
}

function RowContent({ href, onActivate, onKeyDown, register, children }: {
  href?: string
  onActivate: () => void
  onKeyDown: (event: KeyboardEvent<HTMLElement>) => void
  register: (element: HTMLElement | null) => void
  children: ReactNode
}) {
  if (href) {
    return (
      <a className="lens-drill-row" href={href} onKeyDown={onKeyDown} ref={register}>{children}</a>
    )
  }
  return (
    <button className="lens-drill-row" onClick={onActivate} onKeyDown={onKeyDown} ref={register} type="button">
      {children}
    </button>
  )
}

const overlayWidth = 320
const overlayGap = 12
const viewportPadding = 12

interface Position {
  left: number
  top: number
  placement: 'right' | 'left' | 'below'
}

/**
 * Places the popover beside the mark it describes, never on top of it, and
 * keeps it inside the viewport. Preference order matches how people read the
 * chart: to the right of the mark, then left, then below.
 */
export function positionOverlay(
  anchor: DrillOverlayAnchor,
  size: { width: number; height: number },
  viewport: { width: number; height: number },
): Position {
  const fitsRight = anchor.x + overlayGap + size.width + viewportPadding <= viewport.width
  const fitsLeft = anchor.x - overlayGap - size.width - viewportPadding >= 0
  const placement: Position['placement'] = fitsRight ? 'right' : fitsLeft ? 'left' : 'below'
  const left = placement === 'right'
    ? anchor.x + overlayGap
    : placement === 'left'
      ? anchor.x - overlayGap - size.width
      : Math.min(Math.max(viewportPadding, anchor.x - size.width / 2), viewport.width - size.width - viewportPadding)
  const rawTop = placement === 'below' ? anchor.y + overlayGap : anchor.y - size.height / 2
  const top = Math.min(Math.max(viewportPadding, rawTop), Math.max(viewportPadding, viewport.height - size.height - viewportPadding))
  return { left: Math.max(viewportPadding, left), top, placement }
}

export function DrillOverlay({
  target, path = [], anchor, anchorElement, valueFormat, theme, dark = false, selectedPerspectiveId,
  onDrillInto, onDrillChild, onPerspective, onClose,
}: DrillOverlayProps) {
  const translate = useTranslate()
  const formatValue = useFormat(valueFormat)
  const formatShare = useFormat({ kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' })
  const [container, setContainer] = useState<HTMLElement>()
  const [position, setPosition] = useState<Position>({ left: anchor.x, top: anchor.y, placement: 'right' })
  const dialogRef = useRef<HTMLDivElement>(null)
  const rowRefs = useRef<Array<HTMLElement | null>>([])

  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const element = document.createElement('div')
    // Same rule as the expanded panel: leaving the dashboard subtree means the
    // host has to re-declare the Lens root context, and living at the end of
    // body means no ancestor stacking context can bury it.
    element.className = `lens-root lens-overlay-root${dark ? ' dark' : ''}`
    if (theme) element.dataset.theme = theme
    document.body.appendChild(element)
    setContainer(element)
    return () => {
      element.remove()
      setContainer(undefined)
    }
  }, [dark, theme])

  const anchorRef = useRef(anchor)
  anchorRef.current = anchor
  const anchorElementRef = useRef(anchorElement)
  anchorElementRef.current = anchorElement

  const reposition = useCallback(() => {
    const dialog = dialogRef.current
    if (!dialog) return
    const element = anchorElementRef.current
    const anchorRect = element?.getBoundingClientRect()
    const point = anchorRect && anchorRect.width > 0
      ? { x: anchorRect.left + anchorRect.width / 2, y: anchorRect.top + anchorRect.height / 2 }
      : anchorRef.current
    const rect = dialog.getBoundingClientRect()
    const next = positionOverlay(
      point,
      { width: rect.width || overlayWidth, height: rect.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    )
    setPosition((current) => (
      current.left === next.left && current.top === next.top && current.placement === next.placement
        ? current
        : next
    ))
  }, [])

  useLayoutEffect(() => {
    if (!container) return
    reposition()
  }, [container, reposition, target])

  useEffect(() => {
    if (!container) return undefined
    // The first measurement runs against whatever layout exists at click time.
    // Anything that can still move it — the next two frames, web fonts, a
    // resize, the dialog growing — re-runs it, so the resting position never
    // depends on when the overlay happened to open.
    let frame = globalThis.requestAnimationFrame(() => {
      frame = globalThis.requestAnimationFrame(reposition)
    })
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (dialogRef.current) observer?.observe(dialogRef.current)
    if (anchorElementRef.current) observer?.observe(anchorElementRef.current)
    globalThis.addEventListener('resize', reposition)
    const fonts = (globalThis.document as Document & { fonts?: FontFaceSet }).fonts
    void fonts?.ready.then(reposition)
    return () => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
    }
  }, [container, reposition])

  useEffect(() => {
    if (container) dialogRef.current?.focus()
  }, [container])

  useEffect(() => {
    if (typeof document === 'undefined') return undefined
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      onClose()
    }
    // Repositioning against a canvas mark during scroll is guesswork; the
    // popover is transient, so scrolling dismisses it instead.
    const onScroll = () => onClose()
    document.addEventListener('keydown', onKeyDown, true)
    globalThis.addEventListener('scroll', onScroll, true)
    return () => {
      document.removeEventListener('keydown', onKeyDown, true)
      globalThis.removeEventListener('scroll', onScroll, true)
    }
  }, [onClose])

  const moveRowFocus = useCallback((event: KeyboardEvent<HTMLElement>, index: number) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const count = target.breakdown.length
    let next = index
    if (event.key === 'ArrowDown') next = (index + 1) % count
    if (event.key === 'ArrowUp') next = (index - 1 + count) % count
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = count - 1
    rowRefs.current[next]?.focus()
  }, [target.breakdown.length])

  if (!container) return null

  const choosable = target.perspectives.length > 1
  // Expanding into a perspective fork lands on a level that owns no data and
  // whose only content is the perspective choice — so when this overlay is
  // already showing that choice, offering the expansion too makes one click
  // ask the same question twice. A fork with a single perspective keeps the
  // action: nothing else here leads to it, and entering it resolves the sole
  // perspective on arrival.
  const expandable = Boolean(target.node && target.target) && !(target.expandsToFork && choosable)
  const empty = target.breakdown.length === 0 && !target.leafHref && !expandable && !choosable

  return createPortal(
    <>
      {/* A transparent catcher closes on any outside press without dimming the
          chart the popover is describing. */}
      <div className="lens-drill-scrim" onMouseDown={onClose} />
      <div
        aria-label={target.label}
        aria-modal="false"
        className="lens-drill-overlay"
        data-placement={position.placement}
        ref={dialogRef}
        role="dialog"
        style={{ left: `${position.left}px`, top: `${position.top}px`, width: `${overlayWidth}px` }}
        tabIndex={-1}
      >
        <header className="lens-drill-header">
          <div className="lens-drill-heading">
            <p className="lens-drill-label">{target.label}</p>
            {target.value !== undefined && (
              <p className="lens-drill-value">
                {formatValue(target.value)}
                {target.share !== undefined && (
                  <span className="lens-drill-share">· {formatShare(target.share * 100)}</span>
                )}
              </p>
            )}
          </div>
          <button
            aria-label={translate('explore.close', 'Close')}
            className="lens-icon-button lens-drill-close"
            onClick={onClose}
            type="button"
          >
            <X />
          </button>
        </header>

        {path.length > 1 && (
          <section className="lens-drill-section">
            <h4 className="lens-drill-section-label">{translate('explore.pathLabel', 'Path')}</h4>
            <ol className="lens-drill-path">
              {path.map((step, index) => (
                <li key={`${step.label}-${index}`}>
                  <button
                    aria-current={step.current ? 'page' : undefined}
                    className="lens-drill-path-step"
                    disabled={step.current}
                    onClick={step.onSelect}
                    style={{ paddingLeft: `${index * 10}px` }}
                    type="button"
                  >
                    {index > 0 && <CaretRight />}
                    <span>{step.label}</span>
                  </button>
                </li>
              ))}
            </ol>
          </section>
        )}

        {target.breakdown.length > 0 && (
          <section className="lens-drill-section">
            <h4 className="lens-drill-section-label">{translate('explore.breakdown', 'Breakdown')}</h4>
            <ul className="lens-drill-rows">
              {target.breakdown.map((row, index) => (
                <li key={row.node.key}>
                  {/* A child that is a record opens it; a child that is a level
                      drills into it. */}
                  <RowContent
                    href={row.href}
                    onActivate={() => onDrillChild(row.node.key)}
                    onKeyDown={(event) => moveRowFocus(event, index)}
                    register={(element) => { rowRefs.current[index] = element }}
                  >
                    <span className="lens-drill-row-label">{row.label}</span>
                    {row.value !== undefined && <span className="lens-drill-row-value">{formatValue(row.value)}</span>}
                    {row.share !== undefined && <span className="lens-drill-row-share">{formatShare(row.share * 100)}</span>}
                    {row.href ? <ArrowUpRight /> : <CaretRight />}
                    {row.share !== undefined && (
                      <span aria-hidden="true" className="lens-drill-row-bar" style={{ width: `${row.share * 100}%` }} />
                    )}
                  </RowContent>
                </li>
              ))}
            </ul>
          </section>
        )}

        {choosable && (
          <section className="lens-drill-section">
            <h4 className="lens-drill-section-label">{translate('explore.viewSegmentAs', 'View this segment as')}</h4>
            <div className="lens-drill-perspectives" role="listbox" aria-label={translate('explore.views', '{n} views', { n: target.perspectives.length })}>
              {target.perspectives.map((perspective) => (
                <button
                  aria-selected={perspective.id === selectedPerspectiveId}
                  className="lens-drill-perspective"
                  key={perspective.id}
                  onClick={() => onPerspective(perspective.id)}
                  role="option"
                  type="button"
                >
                  {perspective.label}
                </button>
              ))}
            </div>
          </section>
        )}

        <footer className="lens-drill-footer">
          {expandable && (
            <button className="lens-drill-action" onClick={() => onDrillInto(target)} type="button">
              <span>{translate('explore.expandSegment', 'Expand segment')}</span>
              <CaretRight />
            </button>
          )}
          {target.leafHref && (
            <a className="lens-drill-action lens-drill-action-leaf" href={target.leafHref}>
              <span>{translate('table.openRecord', 'Open record')}</span>
              <ArrowUpRight />
            </a>
          )}
          {empty && <p className="lens-drill-empty">{translate('explore.noDetail', 'No further detail')}</p>}
        </footer>
      </div>
    </>,
    container,
  )
}
