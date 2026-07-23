import {
  useCallback, useEffect, useLayoutEffect, useRef, useState,
  type CSSProperties, type KeyboardEvent, type ReactNode,
} from 'react'
import { createPortal } from 'react-dom'
import type { FieldFormat } from '../contract'
import { ArrowUpRight, CaretRight, Check, Copy, X } from '../icons'
import { useFormat, useTranslate } from '../runtime'
import { isVisualRegression } from '../visualRegression'
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
  /**
   * The clicked segment's series color, resolved through the same path the
   * chart and legend use (`seriesColorResolver`). Absent for a level card,
   * which describes no single mark.
   */
  accentColor?: string
  theme?: string
  dark?: boolean
  selectedPerspectiveId?: string
  onDrillInto: (target: DrillTarget) => void
  onDrillChild: (childKey: string) => void
  onPerspective: (perspectiveId: string) => void
  onClose: () => void
}

function RowContent({ href, onActivate, children }: {
  href?: string
  onActivate: () => void
  children: ReactNode
}) {
  if (href) {
    return (
      <a className="lens-drill-row" href={href}>{children}</a>
    )
  }
  return (
    <button className="lens-drill-row" onClick={onActivate} type="button">
      {children}
    </button>
  )
}

const overlayWidth = 320
const overlayGap = 12
const viewportPadding = 12
const caretInset = 14

interface Position {
  left: number
  top: number
  placement: 'right' | 'left' | 'below'
  /**
   * Offset of the caret tip along the card edge it sits on, measured from the
   * card's top-left. Clamped inside the edge so the caret keeps pointing at the
   * mark even after the card is nudged back inside the viewport.
   */
  caret: number
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), Math.max(min, max))
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
  const clampedLeft = Math.max(viewportPadding, left)
  const caret = placement === 'below'
    ? clamp(anchor.x - clampedLeft, caretInset, size.width - caretInset)
    : clamp(anchor.y - top, caretInset, size.height - caretInset)
  return { left: clampedLeft, top, placement, caret }
}

export function DrillOverlay({
  target, path = [], anchor, anchorElement, valueFormat, accentColor, theme, dark = false, selectedPerspectiveId,
  onDrillInto, onDrillChild, onPerspective, onClose,
}: DrillOverlayProps) {
  const translate = useTranslate()
  const formatValue = useFormat(valueFormat)
  const formatShare = useFormat({ kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' })
  const [container, setContainer] = useState<HTMLElement>()
  const [position, setPosition] = useState<Position>({ left: anchor.x, top: anchor.y, placement: 'right', caret: caretInset })
  const [copied, setCopied] = useState(false)
  const dialogRef = useRef<HTMLDivElement>(null)
  const copiedTimer = useRef<ReturnType<typeof setTimeout>>()
  // Pop-in is decided once, at mount, and never under visual regression or
  // reduced motion: a screenshot taken mid-scale is non-deterministic, so the
  // VR flag routes the card straight to its resting state.
  const [animate] = useState(() => {
    if (isVisualRegression()) return false
    return !globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  })

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
      current.left === next.left && current.top === next.top &&
      current.placement === next.placement && current.caret === next.caret
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

  useEffect(() => () => {
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
  }, [])

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

  // ArrowUp/ArrowDown walk every interactive element in the card — rows,
  // perspective options, the footer actions and the close button — as one
  // roving list, so the whole popover is reachable without a mouse. Esc and
  // Tab keep their existing behaviour.
  const moveFocus = useCallback((event: KeyboardEvent<HTMLElement>) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const dialog = dialogRef.current
    if (!dialog) return
    const focusables = Array.from(dialog.querySelectorAll<HTMLElement>(
      'button:not([disabled]), a[href], [tabindex]:not([tabindex="-1"])',
    ))
    if (focusables.length === 0) return
    event.preventDefault()
    const active = document.activeElement as HTMLElement | null
    const index = active ? focusables.indexOf(active) : -1
    let next = index
    if (event.key === 'ArrowDown') next = index < 0 ? 0 : (index + 1) % focusables.length
    if (event.key === 'ArrowUp') next = index < 0 ? focusables.length - 1 : (index - 1 + focusables.length) % focusables.length
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = focusables.length - 1
    focusables[next]?.focus()
  }, [])

  const copyValue = useCallback(async () => {
    if (target.value === undefined) return
    // Copy the raw machine value (plain digits, no thousands separators, no unit
    // or compact abbreviation) so it pastes straight into a spreadsheet — not the
    // formatted display string («13.02 млрд UZS»). The on-screen figure keeps its
    // formatting.
    const text = String(target.value)
    const clipboard = globalThis.navigator?.clipboard
    let done = false
    try {
      if (clipboard?.writeText) {
        await clipboard.writeText(text)
        done = true
      }
    } catch {
      done = false
    }
    if (!done) {
      // No async clipboard (insecure context) or a rejected write: fall back to
      // the legacy selection copy, and if even that is unavailable stay silent
      // rather than throw out of an event handler.
      try {
        const field = document.createElement('textarea')
        field.value = text
        field.setAttribute('readonly', '')
        field.style.position = 'fixed'
        field.style.opacity = '0'
        document.body.appendChild(field)
        field.select()
        document.execCommand('copy')
        field.remove()
      } catch {
        // Silent no-op: the value is still on screen to read.
      }
    }
    setCopied(true)
    if (copiedTimer.current) clearTimeout(copiedTimer.current)
    copiedTimer.current = setTimeout(() => setCopied(false), 1500)
  }, [target.value])

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
  const hasValue = target.value !== undefined
  const caretStyle: CSSProperties = position.placement === 'below'
    ? { left: `${position.left + position.caret}px`, top: `${position.top}px` }
    : position.placement === 'left'
      ? { left: `${position.left + overlayWidth}px`, top: `${position.top + position.caret}px` }
      : { left: `${position.left}px`, top: `${position.top + position.caret}px` }

  return createPortal(
    <>
      {/* A transparent catcher closes on any outside press without dimming the
          chart the popover is describing. */}
      <div className="lens-drill-scrim" onMouseDown={onClose} />
      <span aria-hidden="true" className="lens-drill-caret" data-placement={position.placement} style={caretStyle} />
      <div
        aria-label={target.label}
        aria-modal="false"
        className={`lens-drill-overlay${animate ? ' lens-drill-overlay-enter' : ''}`}
        data-placement={position.placement}
        onKeyDown={moveFocus}
        ref={dialogRef}
        role="dialog"
        style={{ left: `${position.left}px`, top: `${position.top}px`, width: `${overlayWidth}px` }}
        tabIndex={-1}
      >
        <header className="lens-drill-header">
          <div className="lens-drill-heading">
            {target.node && (
              <p className="lens-drill-eyebrow">{translate('explore.segmentEyebrow', 'Segment')}</p>
            )}
            <p className="lens-drill-title">{target.label}</p>
            {hasValue && (
              <p className="lens-drill-value">
                {accentColor && (
                  <span aria-hidden="true" className="lens-drill-swatch" style={{ background: accentColor }} />
                )}
                <span className="lens-drill-value-figure">{formatValue(target.value!)}</span>
                <button
                  aria-label={copied ? translate('explore.copied', 'Copied') : translate('explore.copyValue', 'Copy value')}
                  className="lens-icon-button lens-drill-copy"
                  data-copied={copied ? 'true' : undefined}
                  onClick={() => { void copyValue() }}
                  title={copied ? translate('explore.copied', 'Copied') : translate('explore.copyValue', 'Copy value')}
                  type="button"
                >
                  {copied ? <Check /> : <Copy />}
                </button>
              </p>
            )}
            {hasValue && target.share !== undefined && (
              <p className="lens-drill-share">
                {target.total !== undefined
                  ? translate('explore.shareOfTotal', '{share} of {total}', {
                    share: formatShare(target.share * 100),
                    total: formatValue(target.total),
                  })
                  : formatShare(target.share * 100)}
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
              {target.breakdown.map((row) => (
                <li key={row.node.key}>
                  {/* A child that is a record opens it; a child that is a level
                      drills into it. */}
                  <RowContent
                    href={row.href}
                    onActivate={() => onDrillChild(row.node.key)}
                  >
                    <span className="lens-drill-row-label">{row.label}</span>
                    {row.value !== undefined && <span className="lens-drill-row-value">{formatValue(row.value)}</span>}
                    {row.share !== undefined && <span className="lens-drill-row-share">{formatShare(row.share * 100)}</span>}
                    <span aria-hidden="true" className="lens-drill-row-chevron">
                      {row.href ? <ArrowUpRight /> : <CaretRight />}
                    </span>
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
            <button className="lens-drill-action lens-drill-action-primary" onClick={() => onDrillInto(target)} type="button">
              <span>{translate('explore.expandSegment', 'Expand segment')}</span>
              <CaretRight />
            </button>
          )}
          {target.leafHref && (
            <a
              className={`lens-drill-action lens-drill-action-leaf${expandable ? '' : ' lens-drill-action-primary'}`}
              href={target.leafHref}
            >
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
