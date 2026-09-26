/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, onCleanup, Show, type JSX } from 'solid-js'
import { For, children as useChildren } from 'solid-js'
import { Portal } from 'solid-js/web'
import { ArrowUpRight } from '../icons'
import { hoverBridgeDelay, useOverlayContainer } from './overlayContainer'
import { CopyValueButton } from './CopyValueButton'
import { useIsClamped } from './useIsClamped'
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
  /**
   * What a column with no amount says out loud, since its figure is an em dash.
   * Absent in a rendering with no runtime to translate it (the printed report),
   * where the dash and the stage's own badge carry the meaning.
   */
  unknownLabel?: string
  children?: JSX.Element
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
/* eslint-disable react-refresh/only-export-components */
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
  anchor: () => HTMLElement | null | undefined
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
function WaterfallTip(props: WaterfallTipProps) {
  const container = useOverlayContainer(() => props.open, props.anchor, 'lens-waterfall-tip-overlay-root')
  let tip: HTMLSpanElement | undefined
  const [position, setPosition] = createSignal<WaterfallTipPosition>()

  const reposition = () => {
    const anchored = props.anchor()?.getBoundingClientRect()
    const box = tip?.getBoundingClientRect()
    if (!anchored || !box) return
    // The column prints its own total just above the bar, and the band is at the
    // bar's top — so a tip placed off the band alone lands squarely on that
    // figure. Clearing whichever of the two sits higher keeps both readable.
    const printed = props.anchor()?.closest('.lens-waterfall-bar')?.querySelector('strong')
    const top = Math.min(anchored.top, printed?.getBoundingClientRect().top ?? anchored.top)
    const next = positionWaterfallTip(
      { left: anchored.left, width: anchored.width, top, bottom: anchored.bottom },
      box,
      { width: globalThis.innerWidth || 1024, height: globalThis.innerHeight || 768 },
    )
    setPosition((current) => current?.left === next.left && current.top === next.top && current.side === next.side
      ? current
      : next)
  }

  createEffect(() => {
    if (container()) reposition()
  })

  createEffect(() => {
    if (!props.open) setPosition(undefined)
  })

  createEffect(() => {
    if (!container()) return
    // A fixed portal has to follow its anchor through the dashboard's own
    // scroll container as well as the document's.
    globalThis.addEventListener('resize', reposition)
    globalThis.addEventListener('scroll', reposition, true)
    onCleanup(() => {
      globalThis.removeEventListener('resize', reposition)
      globalThis.removeEventListener('scroll', reposition, true)
    })
  })

  return (
    <Show when={props.open && container()}>
      <Portal mount={container()}>
        <span
          class="lens-waterfall-tip"
          data-side={position()?.side ?? 'above'}
          onMouseEnter={props.onMouseEnter}
          onMouseLeave={props.onMouseLeave}
          ref={(el) => { tip = el }}
          role="tooltip"
          style={{
            left: `${position()?.left ?? 0}px`,
            top: `${position()?.top ?? 0}px`,
            visibility: position() ? 'visible' : 'hidden',
          }}
        >
          <span aria-hidden="true" class="lens-waterfall-tip-tail" />
          <span class="lens-waterfall-tip-head">{props.item.label}</span>
          <span class="lens-waterfall-tip-amount">
            <strong>{props.item.exactValue ?? props.item.formattedValue}</strong>
            <Show when={props.item.rawValue !== undefined}>
              <CopyValueButton raw={props.item.rawValue!} />
            </Show>
          </span>
          <Show when={(props.item.exactSplit ?? props.item.formattedSplit) !== undefined}>
            <>
              <Show when={props.item.splitLabel}>
                <span class="lens-waterfall-tip-label">{props.item.splitLabel}</span>
              </Show>
              <span class="lens-waterfall-tip-amount" data-split="true">
                <strong>{props.item.exactSplit ?? props.item.formattedSplit}</strong>
                <Show when={props.item.rawSplit !== undefined}>
                  <CopyValueButton raw={props.item.rawSplit!} />
                </Show>
              </span>
            </>
          </Show>
          <Show when={props.actionHint}>
            <span class="lens-waterfall-tip-foot">{props.actionHint}</span>
          </Show>
        </span>
      </Portal>
    </Show>
  )
}

interface WaterfallColumnProps {
  item: WaterfallItem
  index: number
  count: number
  chrome?: Record<string, unknown>
  splitCallout: WaterfallSplitCallout
  actionHint?: string
  unknownLabel?: string
}

/**
 * One step of the bridge, from its bar down to the name under it.
 *
 * The name used to live in a second grid below the plot, aligned to this one by
 * a matching column count and a matching left margin — so a column that opened
 * a drill was activatable over its bar and inert over the word naming it, which
 * is the part of a step a hand actually goes for. Nothing about the plate, the
 * cursor or the focus ring reached past the bar, because none of them had
 * anything to reach: the label was not in this element.
 *
 * It is now, and the two bands stay in step through `subgrid` — every column
 * takes its plot height and its label height from the chart's rows rather than
 * from its own content, so a two-line name under one step cannot shorten that
 * step's bar and make it un-comparable with the one beside it.
 */

function WaterfallColumn(props: WaterfallColumnProps) {
  const [pointed, setPointed] = createSignal(false)
  let bar: HTMLDivElement | undefined
  let label: HTMLSpanElement | undefined
  const labelClamped = useIsClamped(() => label)
  let closeTimer: ReturnType<typeof setTimeout> | undefined
  // The card carries buttons, so it has to survive the pointer leaving the
  // column to reach them: it lives in a body portal with a deliberate visual
  // gap, and closing on the column's own mouseleave would unmount it mid-travel.
  const show = () => {
    if (closeTimer) clearTimeout(closeTimer)
    setPointed(true)
  }
  const hide = () => {
    if (closeTimer) clearTimeout(closeTimer)
    closeTimer = setTimeout(() => setPointed(false), hoverBridgeDelay)
  }
  onCleanup(() => { if (closeTimer) clearTimeout(closeTimer) })
  // The callout leans away from the nearer plot edge, so a split on the last
  // columns does not run off a printed chart.
  const calloutSide = props.index * 2 >= props.count ? 'start' : 'end'
  const splitText = `${props.item.splitLabel ? `${props.item.splitLabel} ` : ''}${props.item.formattedSplit ?? ''}`
  const chrome = props.chrome ?? {}
  return (
    <div
      class="lens-waterfall-column"
      onBlur={hide}
      onFocus={show}
      onMouseEnter={show}
      onMouseLeave={hide}
      {...chrome}
    >
      <div class="lens-waterfall-column-plot">
        <Show when={props.item.underlayHeight !== undefined}>
          <span
            aria-hidden="true"
            class="lens-waterfall-underlay"
            style={{
              top: `${props.item.top + props.item.height}%`,
              height: `${props.item.underlayHeight}%`,
            }}
          />
        </Show>
        <Show when={props.index < props.count - 1}>
          <span
            class="lens-waterfall-connector"
            style={{ top: `${props.item.connectorTop}%` }}
          />
        </Show>
        <div
          class="lens-waterfall-bar"
          ref={(el) => { bar = el }}
          data-checkpoint={props.item.checkpoint}
          data-kind={props.item.kind}
          data-label-row={props.index % 2}
          data-no-movement={props.item.noMovement}
          data-terminal={!props.chrome || undefined}
          data-tone={props.item.tone}
          data-unknown={props.item.unknown}
          style={{
            top: `${props.item.top}%`,
            height: `${props.item.height}%`,
          }}
        >
          <strong>
            {props.item.formattedValue}
            <Show when={props.item.unknown && props.unknownLabel}>
              <span class="lens-sr-only">{props.unknownLabel}</span>
            </Show>
          </strong>
          <Show when={props.item.splitHeight !== undefined}>
            <span
              class="lens-waterfall-bar-split"
              style={{ height: `${props.item.splitHeight}%` }}
            >
              {/* The amount stays in the accessibility tree whether or not a
                  pointer exists: the tooltip only ever exists while one hovers. */}
              <Show when={props.splitCallout === 'hover'} fallback={
                <span
                  class="lens-waterfall-split-callout"
                  data-reveal="always"
                  data-side={calloutSide}
                >
                  {splitText}
                </span>
              }>
                <span class="lens-sr-only">{splitText}</span>
              </Show>
            </span>
          </Show>
        </div>
        <Show when={props.splitCallout === 'hover'}>
          <WaterfallTip
            actionHint={props.chrome ? props.actionHint : undefined}
            anchor={() => bar}
            item={props.item}
            onMouseEnter={show}
            onMouseLeave={hide}
            open={pointed()}
          />
        </Show>
      </div>
      <span class="lens-waterfall-label">
        {/* Clamped to two lines rather than wrapped at any character: the band
            was uniform because every label broke mid-word, so «Исходящее
            перестрахование» read as three lines ending in an orphaned «е». The
            native tooltip is the full name behind that clamp — so it is here
            only while there is a clamp to see behind, which is a question about
            the rendered box (a name that fits at 1400px is cut at 900px) and
            not about the string. */}
        <span ref={(el) => { label = el }} title={labelClamped() ? props.item.label : undefined}>{props.item.label}</span>
        <Show when={props.item.annotation}>
          <small class="lens-waterfall-annotation">
            {props.item.annotation}
            {/* The badge is the only solid shape in a column whose bar may be an
                empty dashed gap, so it is what a reader aims at — and a chip
                that looks the same whether or not the step opens anything is
                what made «Настроить» unfindable. The arrow is the claim that
                there is somewhere to go, and it is drawn on the same condition
                a drill cell's pill draws it: a destination actually resolved
                (`chrome`), never merely a status worth stating. */}
            <Show when={props.chrome}><ArrowUpRight size={10} /></Show>
          </small>
        </Show>
      </span>
    </div>
  )
}

/**
 * The waterfall's markup, shared by the interactive panel and the printed
 * report. Everything positional lives in the model, so the same DOM serves a
 * clickable dashboard column and an inert printed one — the printed report gets
 * the real bridge instead of a table of the same numbers.
 */
export function WaterfallPlot(props: WaterfallPlotProps) {
  const slotted = useChildren(() => props.children)
  return (
    <div
      aria-label={props.label}
      class="lens-waterfall"
      data-lens-waterfall=""
      role={props.role ?? 'img'}
      style={{
        '--lens-waterfall-count': props.model.items.length,
        '--lens-waterfall-zero': `${props.model.zero}%`,
      } as JSX.CSSProperties}
    >
      <div class="lens-waterfall-chart">
        <div class="lens-waterfall-axis" aria-hidden="true">
          {/* Stated once, at the head of the axis, rather than on all eight
              gridlines — the same convention every ECharts axis here follows. */}
          <Show when={props.axisUnit}>
            <span class="lens-waterfall-axis-unit">{props.axisUnit}</span>
          </Show>
          <For each={props.model.ticks}>
            {(tick) => <span style={{ top: `${tick.top}%` }}>{tick.label}</span>}
          </For>
        </div>
        {/* The rules are the chart's own layer rather than children of the plot:
            the plot spans the label band now, so a `top: 40%` measured against
            it would draw the gridline through the names. This box takes the
            plot row alone — the height those percentages have always meant —
            and the plot, which comes after it, draws over it. */}
        <div aria-hidden="true" class="lens-waterfall-rules">
          <For each={props.model.ticks}>
            {(tick) => (
              <span
                class="lens-waterfall-gridline"
                style={{ top: `${tick.top}%` }}
              />
            )}
          </For>
          <div class="lens-waterfall-zero" />
        </div>
        <div class="lens-waterfall-plot">
          <For each={props.model.items}>
            {(item, index) => (
              <WaterfallColumn
                // The whole column is the target, not the bar: a step worth 1,07
                // of a 201 opening draws four pixels tall, and nobody should have
                // to aim at four pixels to open it.
                actionHint={props.actionHint}
                chrome={props.interaction?.(item, index())}
                count={props.model.items.length}
                index={index()}
                item={item}
                splitCallout={props.splitCallout ?? 'hover'}
                unknownLabel={props.unknownLabel}
              />
            )}
          </For>
        </div>
      </div>
      {slotted()}
    </div>
  )
}
