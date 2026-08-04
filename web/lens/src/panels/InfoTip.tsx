import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type MouseEvent as ReactMouseEvent } from 'react'
import { createPortal } from 'react-dom'
import { useTranslate } from '../runtime'
import { hoverBridgeDelay, useOverlayContainer } from '../runtime/overlayContainer'
import { Info } from '../icons'

export interface InfoTipProps {
  /** Already-localized note. Newlines separate paragraphs. */
  text: string
  /**
   * What the note is about — the metric's own name. It names the trigger for a
   * reader who cannot see what it sits beside; it is deliberately *not* drawn
   * in the bubble. A heading there restated, in bold caps, the label rendered
   * four pixels above it, on every panel of every dashboard — the tail already
   * says which glyph the note belongs to.
   */
  subject?: string
  /**
   * Compact form for a metric strip cell, whose label is 10px type: the
   * header glyph's 28px hit box would tower over it. Also lifts the control
   * above a card-wide navigate anchor, which otherwise swallows the click.
   */
  inline?: boolean
}

interface FloatingPosition {
  left: number
  top: number
  /** Which side of the trigger the bubble took, so the tail can point back. */
  side: 'above' | 'below'
}

const popoverGap = 6
const viewportGutter = 8
/** Half the tail's width plus the bubble's corner radius. */
const tailInset = 14

/**
 * Keeps a note inside the viewport while preferring the familiar
 * below-and-left-aligned placement. The note flips above the trigger when the
 * remaining space below would clip it.
 */
export function positionInfoTip(
  anchor: Pick<DOMRect, 'left' | 'top' | 'bottom'>,
  bubble: Pick<DOMRect, 'width' | 'height'>,
  viewport: { width: number; height: number },
): FloatingPosition {
  const maxLeft = Math.max(viewportGutter, viewport.width - bubble.width - viewportGutter)
  const left = Math.min(Math.max(anchor.left, viewportGutter), maxLeft)
  const below = anchor.bottom + popoverGap
  const above = anchor.top - bubble.height - popoverGap
  const fitsBelow = below + bubble.height <= viewport.height - viewportGutter || above < viewportGutter
  const top = fitsBelow
    ? Math.min(Math.max(below, viewportGutter), Math.max(viewportGutter, viewport.height - bubble.height - viewportGutter))
    : above
  return { left, top, side: fitsBelow ? 'below' : 'above' }
}

/** Where the tail sits along the bubble's edge, so it points at its trigger. */
export function infoTipTailOffset(anchorCenter: number, bubbleLeft: number, bubbleWidth: number): number {
  return Math.min(Math.max(anchorCenter - bubbleLeft, tailInset), Math.max(tailInset, bubbleWidth - tailInset))
}

/**
 * The ⓘ affordance in a panel's header.
 *
 * A panel's supporting note explains how a figure is obtained and what it
 * excludes — a paragraph most readers need once. Rendered as chrome it costs
 * a permanent band above every card and pushes the plot itself below the
 * fold; behind this glyph it costs nothing until asked for. Print keeps the
 * same text as the figure's footnote, where there is nothing to ask.
 *
 * Hover and focus open it; a click pins it open so the text can be read on a
 * touch screen and selected with the pointer.
 */
export function InfoTip({ text, subject, inline }: InfoTipProps) {
  const translate = useTranslate()
  const [pinned, setPinned] = useState(false)
  const [hovered, setHovered] = useState(false)
  const [position, setPosition] = useState<FloatingPosition>()
  const [tailOffset, setTailOffset] = useState(tailInset)
  const wrapperRef = useRef<HTMLSpanElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const bubbleRef = useRef<HTMLSpanElement>(null)
  const closeTimer = useRef<ReturnType<typeof globalThis.setTimeout>>()
  const bubbleId = useId()
  const label = translate('panel.info', 'About this metric')
  const accessibleLabel = inline
    ? text
    : subject ? translate('panel.infoAbout', 'About {name}', { name: subject }) : label
  const open = pinned || hovered
  const container = useOverlayContainer(open, wrapperRef, 'lens-info-tip-overlay-root')

  useEffect(() => {
    if (!open) setPosition(undefined)
  }, [open])

  const reposition = useCallback(() => {
    const anchor = buttonRef.current?.getBoundingClientRect()
    const bubble = bubbleRef.current?.getBoundingClientRect()
    if (!anchor || !bubble) return
    const next = positionInfoTip(
      anchor,
      bubble,
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    )
    setPosition((current) => current?.left === next.left && current.top === next.top && current.side === next.side
      ? current
      : next)
    setTailOffset(infoTipTailOffset((anchor.left + anchor.right) / 2, next.left, bubble.width))
  }, [])

  useLayoutEffect(() => {
    if (container) reposition()
  }, [container, reposition])

  useEffect(() => {
    if (!container) return undefined
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (bubbleRef.current) observer?.observe(bubbleRef.current)
    globalThis.addEventListener('resize', reposition)
    // Capture scrolls from the dashboard's own scroll container as well as the
    // document; a fixed portal must continue to follow its trigger.
    globalThis.addEventListener('scroll', reposition, true)
    return () => {
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    }
  }, [container, reposition])

  // An open bubble is dismissed the way every other transient surface in the
  // runtime is: Escape, or a click that lands anywhere else.
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setPinned(false)
        setHovered(false)
      }
    }
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node
      if (!wrapperRef.current?.contains(target) && !bubbleRef.current?.contains(target)) setPinned(false)
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('pointerdown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('pointerdown', onPointerDown)
    }
  }, [open])

  const paragraphs = text.split(/\n{2,}/).map((paragraph) => paragraph.trim()).filter(Boolean)
  const cancelClose = () => {
    if (closeTimer.current !== undefined) globalThis.clearTimeout(closeTimer.current)
    closeTimer.current = undefined
  }
  const leavingSurface = (event: ReactMouseEvent, other: HTMLElement | null) => {
    const next = event.relatedTarget
    if (next instanceof Node && other?.contains(next)) return
    cancelClose()
    closeTimer.current = globalThis.setTimeout(() => setHovered(false), hoverBridgeDelay)
  }

  useEffect(() => () => cancelClose(), [])

  const bubbleStyle: CSSProperties = {
    left: position?.left ?? 0,
    top: position?.top ?? 0,
    pointerEvents: 'auto',
    visibility: position ? 'visible' : 'hidden',
    '--lens-info-tip-tail': `${tailOffset}px`,
  } as CSSProperties

  return (
    <span
      // Class names stay literal: Tailwind's content scan cannot see an
      // interpolated modifier and would drop the rule.
      className={inline ? 'lens-info-tip lens-info-tip-inline' : 'lens-info-tip'}
      ref={wrapperRef}
      onMouseEnter={() => { cancelClose(); setHovered(true) }}
      onMouseLeave={(event) => leavingSurface(event, bubbleRef.current)}
    >
      <button
        aria-describedby={open ? bubbleId : undefined}
        aria-expanded={open}
        aria-label={accessibleLabel}
        // An open bubble has to say which glyph opened it: a strip of four
        // metrics with four ⓘ and one floating note left nothing on screen
        // tying the two together. Class names stay literal for Tailwind's scan.
        className={inline
          ? `lens-export-button lens-icon-button lens-info-tip-button lens-info-tip-button-inline${open ? ' lens-info-tip-button-open' : ''}`
          : `lens-export-button lens-icon-button lens-info-tip-button${open ? ' lens-info-tip-button-open' : ''}`}
        onBlur={() => setHovered(false)}
        onClick={() => setPinned((current) => !current)}
        onFocus={() => setHovered(true)}
        ref={buttonRef}
        type="button"
      >
        <Info />
      </button>
      {open && container && createPortal(
        <span
          className="lens-info-tip-bubble"
          data-side={position?.side ?? 'below'}
          id={bubbleId}
          onMouseEnter={() => { cancelClose(); setHovered(true) }}
          onMouseLeave={(event) => leavingSurface(event, wrapperRef.current)}
          ref={bubbleRef}
          role="tooltip"
          style={bubbleStyle}
        >
          <span aria-hidden="true" className="lens-info-tip-tail" />
          {/* The scroll box is inside the bubble, not the bubble itself: a
              scroll container clips on both axes, and the tail hangs over the
              edge it points from. */}
          <span className="lens-info-tip-body">
            {paragraphs.map((paragraph, index) => (
              <span className="lens-info-tip-text" key={index}>{paragraph}</span>
            ))}
          </span>
        </span>,
        container,
      )}
    </span>
  )
}
