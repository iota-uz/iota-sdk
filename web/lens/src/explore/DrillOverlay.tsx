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

export interface DrillOverlayProps {
  target: DrillTarget
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
  target, anchor, valueFormat, theme, dark = false, selectedPerspectiveId,
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

  useLayoutEffect(() => {
    if (!container || !dialogRef.current) return
    const rect = dialogRef.current.getBoundingClientRect()
    setPosition(positionOverlay(
      anchor,
      { width: rect.width || overlayWidth, height: rect.height },
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    ))
  }, [anchor, container, target])

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

  const expandable = Boolean(target.node && target.target)
  const empty = target.breakdown.length === 0 && !target.leafHref && !expandable

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

        {target.perspectives.length > 1 && (
          <section className="lens-drill-section">
            <h4 className="lens-drill-section-label">{translate('explore.viewSegmentAs', 'View this segment as')}</h4>
            <div className="lens-drill-perspectives" role="listbox" aria-label={translate('explore.views', '{n} views').replaceAll('{n}', String(target.perspectives.length))}>
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
