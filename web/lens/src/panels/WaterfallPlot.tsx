import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
  type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { hoverBridgeDelay, useOverlayContainer } from '../runtime/overlayContainer'
import { CopyValueButton } from './CopyValueButton'
import type { WaterfallItem, WaterfallModel } from './CascadePanel'

/** Chrome a host wraps around a single column, today: activation for a drill. */
export type WaterfallInteraction = (item: WaterfallItem, index: number) => Record<string, unknown> | undefined

/**
 * When the split band names itself. The band's own colour already says a part of
 * the movement differs in kind; its chip answers a follow-up question — how much,
 * and called what — and standing permanently beside the bar it spends the plot's
 * scarcest space (the gutter the neighbouring columns print their totals in) on
 * an answer nobody asked for yet. `hover` floats it over the column as a
 * tooltip, the way every other panel on the board answers a pointer. A sheet of
 * paper has no pointer, so the printed report keeps the chip standing in-plot.
 */
export type WaterfallSplitCallout = 'hover' | 'always'

export interface WaterfallPlotProps {
  model: WaterfallModel
  label: string
  interaction?: WaterfallInteraction
  /** A grouping role when the columns are activatable; an image otherwise. */
  role?: 'group' | 'img'
  /** Defaults to `hover`; a static rendering must pass `always`. */
  splitCallout?: WaterfallSplitCallout
  /** The unit the ticks no longer repeat, stated once at the head of the axis. */
  axisUnit?: string
  /** What activating a column does, printed at the foot of its tooltip. */
  actionHint?: string
  children?: ReactNode
}

const tipGap = 8
const viewportGutter = 8

export interface WaterfallTipPosition {
  left: number
  top: number
  /** Which side of the band the tip took, so the tail can point back. */
  side: 'above' | 'below'
}

/**
 * Centres the tip over the band it explains and keeps it on screen. It prefers
 * to sit above: the band is at the top of its bar, and below it would cover the
 * rest of the movement it is a part of.
 */
export function positionWaterfallTip(
  anchor: Pick<DOMRect, 'left' | 'width' | 'top' | 'bottom'>,
  tip: Pick<DOMRect, 'width' | 'height'>,
  viewport: { width: number; height: number },
): WaterfallTipPosition {
  const maxLeft = Math.max(viewportGutter, viewport.width - tip.width - viewportGutter)
  const left = Math.min(Math.max(anchor.left + anchor.width / 2 - tip.width / 2, viewportGutter), maxLeft)
  const above = anchor.top - tip.height - tipGap
  const fitsAbove = above >= viewportGutter
  return {
    left: Math.round(left),
    top: Math.round(fitsAbove ? above : anchor.bottom + tipGap),
    side: fitsAbove ? 'above' : 'below',
  }
}

interface WaterfallTipProps {
  anchor: RefObject<HTMLElement | null>
  open: boolean
  item: WaterfallItem
  actionHint?: string
  onMouseEnter: () => void
  onMouseLeave: () => void
}

/**
 * What this step is, in full, floated over the plot rather than parked in it.
 *
 * The plot itself is compact by design — «−41,20 млрд UZS» over a bar four
 * pixels tall — and a reader who points at a column is asking to look closer.
 * So the card carries the unabbreviated figure, the part of the movement that
 * differs in kind when there is one, a way to take either number with you, and
 * what a click would do. In-plot none of that had anywhere to stand that was
 * not over a neighbouring column. A body-level portal is what the board's other
 * answers to a pointer (every ECharts tooltip) already use, and no card clips
 * it.
 */
function WaterfallTip({ anchor, open, item, actionHint, onMouseEnter, onMouseLeave }: WaterfallTipProps) {
  const container = useOverlayContainer(open, anchor, 'lens-waterfall-tip-overlay-root')
  const tip = useRef<HTMLSpanElement>(null)
  const [position, setPosition] = useState<WaterfallTipPosition>()

  const reposition = useCallback(() => {
    const anchored = anchor.current?.getBoundingClientRect()
    const box = tip.current?.getBoundingClientRect()
    if (!anchored || !box) return
    // The column prints its own total just above the bar, and the band is at the
    // bar's top — so a tip placed off the band alone lands squarely on that
    // figure. Clearing whichever of the two sits higher keeps both readable.
    const printed = anchor.current?.closest('.lens-waterfall-bar')?.querySelector('strong')
    const top = Math.min(anchored.top, printed?.getBoundingClientRect().top ?? anchored.top)
    const next = positionWaterfallTip(
      { left: anchored.left, width: anchored.width, top, bottom: anchored.bottom },
      box,
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    )
    setPosition((current) => current?.left === next.left && current.top === next.top && current.side === next.side
      ? current
      : next)
  }, [anchor])

  useLayoutEffect(() => {
    if (container) reposition()
  }, [container, reposition])

  useEffect(() => {
    if (!open) setPosition(undefined)
  }, [open])

  useEffect(() => {
    if (!container) return undefined
    // A fixed portal has to follow its anchor through the dashboard's own
    // scroll container as well as the document's.
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    return () => {
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    }
  }, [container, reposition])

  if (!open || !container) return null
  const amount = item.exactValue ?? item.formattedValue
  const split = item.exactSplit ?? item.formattedSplit
  return createPortal(
    <span
      className="lens-waterfall-tip"
      data-side={position?.side ?? 'above'}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      ref={tip}
      role="tooltip"
      style={{
        left: position?.left ?? 0,
        top: position?.top ?? 0,
        visibility: position ? 'visible' : 'hidden',
      }}
    >
      <span aria-hidden="true" className="lens-waterfall-tip-tail" />
      <span className="lens-waterfall-tip-head">{item.label}</span>
      <span className="lens-waterfall-tip-amount">
        <strong>{amount}</strong>
        {item.rawValue !== undefined && <CopyValueButton raw={item.rawValue} />}
      </span>
      {split !== undefined && (
        <>
          {item.splitLabel && <span className="lens-waterfall-tip-label">{item.splitLabel}</span>}
          <span className="lens-waterfall-tip-amount" data-split="true">
            <strong>{split}</strong>
            {item.rawSplit !== undefined && <CopyValueButton raw={item.rawSplit} />}
          </span>
        </>
      )}
      {actionHint && <span className="lens-waterfall-tip-foot">{actionHint}</span>}
    </span>,
    container,
  )
}

interface WaterfallColumnProps {
  item: WaterfallItem
  index: number
  count: number
  chrome?: Record<string, unknown>
  splitCallout: WaterfallSplitCallout
  actionHint?: string
}

function WaterfallColumn({ item, index, count, chrome, splitCallout, actionHint }: WaterfallColumnProps) {
  const [pointed, setPointed] = useState(false)
  const bar = useRef<HTMLDivElement>(null)
  const closeTimer = useRef<ReturnType<typeof setTimeout>>()
  // The card carries buttons, so it has to survive the pointer leaving the
  // column to reach them: it lives in a body portal with a deliberate visual
  // gap, and closing on the column's own mouseleave would unmount it mid-travel.
  const show = () => {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    setPointed(true)
  }
  const hide = () => {
    if (closeTimer.current) clearTimeout(closeTimer.current)
    closeTimer.current = setTimeout(() => setPointed(false), hoverBridgeDelay)
  }
  useEffect(() => () => { if (closeTimer.current) clearTimeout(closeTimer.current) }, [])
  // The callout leans away from the nearer plot edge, so a split on the last
  // columns does not run off a printed chart.
  const calloutSide = index * 2 >= count ? 'start' : 'end'
  const splitText = `${item.splitLabel ? `${item.splitLabel} ` : ''}${item.formattedSplit ?? ''}`
  return (
    <div
      className="lens-waterfall-column"
      onBlur={hide}
      onFocus={show}
      onMouseEnter={show}
      onMouseLeave={hide}
      {...chrome}
    >
      {item.underlayHeight !== undefined && (
        <span
          aria-hidden="true"
          className="lens-waterfall-underlay"
          style={{
            top: `${item.top + item.height}%`,
            height: `${item.underlayHeight}%`,
          }}
        />
      )}
      {index < count - 1 && (
        <span
          className="lens-waterfall-connector"
          style={{ top: `${item.connectorTop}%` }}
        />
      )}
      <div
        className="lens-waterfall-bar"
        ref={bar}
        data-checkpoint={item.checkpoint}
        data-kind={item.kind}
        data-label-row={index % 2}
        data-no-movement={item.noMovement}
        data-terminal={!chrome || undefined}
        data-tone={item.tone}
        style={{
          top: `${item.top}%`,
          height: `${item.height}%`,
        }}
      >
        <strong>{item.formattedValue}</strong>
        {item.splitHeight !== undefined && (
          <span
            className="lens-waterfall-bar-split"
            style={{ height: `${item.splitHeight}%` }}
          >
            {/* The amount stays in the accessibility tree whether or not a
                pointer exists: the tooltip only ever exists while one hovers. */}
            {splitCallout === 'hover'
              ? <span className="lens-sr-only">{splitText}</span>
              : (
                <span
                  className="lens-waterfall-split-callout"
                  data-reveal="always"
                  data-side={calloutSide}
                >
                  {splitText}
                </span>
              )}
          </span>
        )}
      </div>
      {splitCallout === 'hover' && (
        <WaterfallTip
          actionHint={chrome ? actionHint : undefined}
          anchor={bar}
          item={item}
          onMouseEnter={show}
          onMouseLeave={hide}
          open={pointed}
        />
      )}
    </div>
  )
}

/**
 * The waterfall's markup, shared by the interactive panel and the printed
 * report. Everything positional lives in the model, so the same DOM serves a
 * clickable dashboard column and an inert printed one — the printed report gets
 * the real bridge instead of a table of the same numbers.
 */
export function WaterfallPlot({
  model,
  label,
  interaction,
  role = 'img',
  splitCallout = 'hover',
  axisUnit,
  actionHint,
  children,
}: WaterfallPlotProps) {
  return (
    <div
      aria-label={label}
      className="lens-waterfall"
      data-lens-waterfall
      role={role}
      style={{
        '--lens-waterfall-count': model.items.length,
        '--lens-waterfall-zero': `${model.zero}%`,
      } as CSSProperties}
    >
      <div className="lens-waterfall-chart">
        <div className="lens-waterfall-axis" aria-hidden="true">
          {/* Stated once, at the head of the axis, rather than on all eight
              gridlines — the same convention every ECharts axis here follows. */}
          {axisUnit && <span className="lens-waterfall-axis-unit">{axisUnit}</span>}
          {model.ticks.map((tick) => (
            <span key={tick.value} style={{ top: `${tick.top}%` }}>{tick.label}</span>
          ))}
        </div>
        <div className="lens-waterfall-plot">
          {model.ticks.map((tick) => (
            <span
              className="lens-waterfall-gridline"
              key={`grid-${tick.value}`}
              style={{ top: `${tick.top}%` }}
            />
          ))}
          <div className="lens-waterfall-zero" />
          {model.items.map((item, index) => (
            <WaterfallColumn
              // The whole column is the target, not the bar: a step worth 1,07
              // of a 201 opening draws four pixels tall, and nobody should have
              // to aim at four pixels to open it.
              actionHint={actionHint}
              chrome={interaction?.(item, index)}
              count={model.items.length}
              index={index}
              item={item}
              key={`${item.label}-${index}`}
              splitCallout={splitCallout}
            />
          ))}
        </div>
      </div>
      <div className="lens-waterfall-labels">
        {model.items.map((item, index) => (
          <span className="lens-waterfall-label" key={`${item.label}-label-${index}`}>
            {/* Clamped to two lines rather than wrapped at any character: the
                band was uniform because every label broke mid-word, so
                «Исходящее перестрахование» read as three lines ending in an
                orphaned «е». The full name is on the element, for the readers
                the clamp costs it. */}
            <span title={item.label}>{item.label}</span>
            {item.annotation && (
              <small className="lens-waterfall-annotation">{item.annotation}</small>
            )}
          </span>
        ))}
      </div>
      {children}
    </div>
  )
}
