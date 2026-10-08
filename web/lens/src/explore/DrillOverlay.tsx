import { createEffect, createSignal, For, onCleanup, onMount, Show, type JSX } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { FieldFormat } from '../contract'
import { ArrowUpRight, CaretRight, Check, Copy, X } from '../icons'
import { useDrawer, useFormat, useTranslate } from '../runtime'
import { copyText } from '../runtime/clipboard'
import { rawValueText } from '../runtime/format'
import { useOverlayContainer } from '../runtime/overlayContainer'
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
  onPrefetchChild?: (childKey: string) => () => void
  onPerspective: (perspectiveId: string) => void
  onClose: () => void
}

function RowContent(props: {
  href?: string
  onActivate: () => void
  onIntent?: () => void | (() => void)
  children: JSX.Element
}): JSX.Element {
  let timer: ReturnType<typeof setTimeout> | undefined
  let cancelIntent: (() => void) | undefined
  let committed = false
  const cancel = () => {
    if (timer) clearTimeout(timer)
    timer = undefined
    if (!committed) cancelIntent?.()
    cancelIntent = undefined
  }
  const intent = () => {
    cancel()
    committed = false
    timer = setTimeout(() => {
      const cleanup = props.onIntent?.()
      cancelIntent = typeof cleanup === 'function' ? cleanup : undefined
    }, 120)
  }
  const activate = () => {
    if (timer) clearTimeout(timer)
    timer = undefined
    // Once intent turns into navigation, keep the speculative request alive so
    // the interactive activation can join and promote the same server job.
    committed = true
    props.onActivate()
  }
  onCleanup(cancel)
  const intentProps = {
    onBlur: cancel,
    onFocus: intent,
    onPointerEnter: intent,
    onPointerLeave: cancel,
  }
  if (props.href) {
    return (
      <a class="lens-drill-row" href={props.href} {...intentProps}>{props.children}</a>
    )
  }
  return (
    <button class="lens-drill-row" onClick={activate} type="button" {...intentProps}>
      {props.children}
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

export function DrillOverlay(props: DrillOverlayProps): JSX.Element | null {
  const drawer = useDrawer()
  const translate = useTranslate()
  const formatValue = useFormat(props.valueFormat)
  const formatShare = useFormat({ kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' })
  const anchorElement = () => props.anchorElement ?? null
  const container = useOverlayContainer(
    () => true,
    anchorElement,
    drawer.depth > 0 ? 'lens-overlay-root-over-drawer' : '',
    { dark: props.dark, theme: props.theme },
  )
  const [position, setPosition] = createSignal<Position>({ left: props.anchor.x, top: props.anchor.y, placement: 'right', caret: caretInset })
  const [copied, setCopied] = createSignal(false)
  const [dialogReady, setDialogReady] = createSignal(false)
  let dialogRef: HTMLDivElement | undefined
  let copiedTimer: ReturnType<typeof setTimeout> | undefined
  // Pop-in is decided once, at mount, and never under visual regression or
  // reduced motion: a screenshot taken mid-scale is non-deterministic, so the
  // VR flag routes the card straight to its resting state.
  const animated = !isVisualRegression() &&
    !globalThis.window?.matchMedia?.('(prefers-reduced-motion: reduce)').matches

  const reposition = () => {
    const dialog = dialogRef
    if (!dialog) return
    const element = anchorElement()
    const anchorRect = element?.getBoundingClientRect()
    const point = anchorRect && anchorRect.width > 0
      ? { x: anchorRect.left + anchorRect.width / 2, y: anchorRect.top + anchorRect.height / 2 }
      : props.anchor
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
  }

  createEffect(() => {
    if (!container() || !dialogReady()) return
    void props.target
    reposition()
  })

  onMount(() => {
    if (!container()) return
    // The first measurement runs against whatever layout exists at click time.
    // Anything that can still move it — the next two frames, web fonts, a
    // resize, the dialog growing — re-runs it, so the resting position never
    // depends on when the overlay happened to open.
    let frame = globalThis.requestAnimationFrame(() => {
      frame = globalThis.requestAnimationFrame(reposition)
    })
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(reposition)
    if (dialogRef) observer?.observe(dialogRef)
    if (anchorElement()) observer?.observe(anchorElement()!)
    globalThis.addEventListener('resize', reposition)
    const fonts = (globalThis.document as Document & { fonts?: FontFaceSet }).fonts
    void fonts?.ready.then(reposition)
    onCleanup(() => {
      globalThis.cancelAnimationFrame(frame)
      observer?.disconnect()
      globalThis.removeEventListener('resize', reposition)
    })
  })

  createEffect(() => {
    if (container() && dialogReady()) dialogRef?.focus()
  })

  onCleanup(() => {
    if (copiedTimer) clearTimeout(copiedTimer)
  })

  createEffect(() => {
    if (typeof document === 'undefined') return
    void props.onClose
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      props.onClose()
    }
    // Repositioning against a canvas mark during scroll is guesswork; the
    // popover is transient, so a scroll of the page underneath — which moves the
    // anchor it is pinned to — dismisses it. A scroll of the overlay's own body
    // (a long breakdown/structure list) must NOT close it, or the list is
    // unreachable: the segment can never be expanded.
    const onScroll = (event: Event) => {
      const node = event.target
      if (node instanceof globalThis.Node && dialogRef?.contains(node)) return
      props.onClose()
    }
    document.addEventListener('keydown', onKeyDown, true)
    globalThis.addEventListener('scroll', onScroll, true)
    onCleanup(() => {
      document.removeEventListener('keydown', onKeyDown, true)
      globalThis.removeEventListener('scroll', onScroll, true)
    })
  })

  // ArrowUp/ArrowDown walk every interactive element in the card — rows,
  // perspective options, the footer actions and the close button — as one
  // roving list, so the whole popover is reachable without a mouse. Esc and
  // Tab keep their existing behaviour.
  const moveFocus = (event: KeyboardEvent) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const dialog = dialogRef
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
  }

  const copyValue = async () => {
    // The raw machine value, so it pastes straight into a spreadsheet — not the
    // formatted display string («13.02 млрд UZS»). The on-screen figure keeps
    // its formatting.
    const text = rawValueText(props.target.value, props.valueFormat)
    if (text === undefined) return
    await copyText(text)
    setCopied(true)
    if (copiedTimer) clearTimeout(copiedTimer)
    copiedTimer = setTimeout(() => setCopied(false), 1500)
  }

  const target = props.target
  const choosable = target.perspectives.length > 1
  // Expanding into a perspective fork lands on a level that owns no data and
  // whose only content is the perspective choice — so when this overlay is
  // already showing that choice, offering the expansion too makes one click
  // ask the same question twice. A fork with a single perspective keeps the
  // action: nothing else here leads to it, and entering it resolves the sole
  // perspective on arrival.
  const expandable = Boolean(target.node && target.target) && !(target.expandsToFork && choosable)
  const empty = target.breakdown.length === 0 && !target.leafHref && !expandable && !choosable

  return (
    <Show when={container()}>
      {(mount) => (
        <Portal mount={mount()}>
          {/* A transparent catcher closes on any outside press without dimming the
              chart the popover is describing. */}
          <button aria-label={translate('drill.close', 'Close details')} class="lens-drill-scrim" onMouseDown={() => props.onClose()} tabIndex={-1} type="button" />
          <span aria-hidden="true" class="lens-drill-caret" data-placement={position().placement} style={{
            left: position().placement === 'left' ? `${position().left + overlayWidth}px` : `${position().left}px`,
            top: position().placement === 'below' ? `${position().top}px` : `${position().top + position().caret}px`,
          }} />
          {/* eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- focus movement is delegated from controls inside this programmatically focusable dialog. */}
          <div
            aria-label={props.target.label}
            aria-modal="false"
            class={`lens-drill-overlay${animated ? ' lens-drill-overlay-enter' : ''}`}
            data-placement={position().placement}
            onKeyDown={moveFocus}
            ref={(el) => { dialogRef = el; setDialogReady(true) }}
            role="dialog"
            style={{ left: `${position().left}px`, top: `${position().top}px`, width: `${overlayWidth}px` }}
            tabIndex={-1}
          >
            <header class="lens-drill-header">
              <div class="lens-drill-heading">
                <Show when={props.target.node}>
                  <p class="lens-drill-eyebrow">{translate('explore.segmentEyebrow', 'Segment')}</p>
                </Show>
                <p class="lens-drill-title">{props.target.label}</p>
                <Show when={props.target.value !== undefined}>
                  <p class="lens-drill-value">
                    <Show when={props.accentColor}>
                      <span aria-hidden="true" class="lens-drill-swatch" style={{ background: props.accentColor }} />
                    </Show>
                    <span class="lens-drill-value-figure">{formatValue(props.target.value)}</span>
                    <button
                      aria-label={copied() ? translate('explore.copied', 'Copied') : translate('explore.copyValue', 'Copy value')}
                      class="lens-icon-button lens-drill-copy"
                      data-copied={copied() ? 'true' : undefined}
                      onClick={() => { void copyValue() }}
                      title={copied() ? translate('explore.copied', 'Copied') : translate('explore.copyValue', 'Copy value')}
                      type="button"
                    >
                      {copied() ? <Check /> : <Copy />}
                    </button>
                  </p>
                </Show>
                <Show when={props.target.value !== undefined && props.target.share !== undefined}>
                  <p class="lens-drill-share">
                    {props.target.total !== undefined
                      ? translate('explore.shareOfTotal', '{share} of {total}', {
                        share: formatShare(props.target.share! * 100),
                        total: formatValue(props.target.total),
                      })
                      : formatShare(props.target.share! * 100)}
                  </p>
                </Show>
              </div>
              <button
                aria-label={translate('explore.close', 'Close')}
                class="lens-icon-button lens-drill-close"
                onClick={() => props.onClose()}
                type="button"
              >
                <X />
              </button>
            </header>

            <Show when={(props.path ?? []).length > 1}>
              <section class="lens-drill-section">
                <h4 class="lens-drill-section-label">{translate('explore.pathLabel', 'Path')}</h4>
                <ol class="lens-drill-path">
                  <For each={props.path}>
                    {(step, index) => (
                      <li>
                        <button
                          aria-current={step.current ? 'page' : undefined}
                          class="lens-drill-path-step"
                          disabled={step.current}
                          onClick={() => step.onSelect()}
                          style={{ 'padding-left': `${index() * 10}px` }}
                          type="button"
                        >
                          <Show when={index() > 0}>
                            <CaretRight />
                          </Show>
                          <span>{step.label}</span>
                        </button>
                      </li>
                    )}
                  </For>
                </ol>
              </section>
            </Show>

            <Show when={props.target.breakdown.length > 0}>
              <section class="lens-drill-section">
                <h4 class="lens-drill-section-label">{translate('explore.breakdown', 'Breakdown')}</h4>
                <ul class="lens-drill-rows">
                  <For each={props.target.breakdown}>
                    {(row) => (
                      <li>
                        {/* A child that is a record opens it; a child that is a level
                            drills into it. */}
                        <RowContent
                          href={row.href}
                          onActivate={() => props.onDrillChild(row.node.key)}
                          onIntent={row.href || !props.onPrefetchChild ? undefined : () => props.onPrefetchChild!(row.node.key)}
                        >
                          <span class="lens-drill-row-label">{row.label}</span>
                          <Show when={row.value !== undefined}>
                            <span class="lens-drill-row-value">{formatValue(row.value)}</span>
                          </Show>
                          <Show when={row.share !== undefined}>
                            <span class="lens-drill-row-share">{formatShare(row.share! * 100)}</span>
                          </Show>
                          <span aria-hidden="true" class="lens-drill-row-chevron">
                            {row.href ? <ArrowUpRight /> : <CaretRight />}
                          </span>
                          <Show when={row.share !== undefined}>
                            <span aria-hidden="true" class="lens-drill-row-bar" style={{ width: `${row.share! * 100}%` }} />
                          </Show>
                        </RowContent>
                      </li>
                    )}
                  </For>
                </ul>
              </section>
            </Show>

            <Show when={choosable}>
              <section class="lens-drill-section">
                <h4 class="lens-drill-section-label">{translate('explore.viewSegmentAs', 'View this segment as')}</h4>
                <div class="lens-drill-perspectives" role="listbox" aria-label={translate('explore.views', '{n} views', { n: target.perspectives.length })}>
                  <For each={target.perspectives}>
                    {(perspective) => (
                      <button
                        aria-selected={perspective.id === props.selectedPerspectiveId}
                        class="lens-drill-perspective"
                        onClick={() => props.onPerspective(perspective.id)}
                        role="option"
                        type="button"
                      >
                        {perspective.label}
                      </button>
                    )}
                  </For>
                </div>
              </section>
            </Show>

            <footer class="lens-drill-footer">
              <Show when={expandable}>
                <button class="lens-drill-action lens-drill-action-primary" onClick={() => props.onDrillInto(props.target)} type="button">
                  <span>{translate('explore.expandSegment', 'Expand segment')}</span>
                  <CaretRight />
                </button>
              </Show>
              <Show when={target.leafHref}>
                <a
                  class={`lens-drill-action lens-drill-action-leaf${expandable ? '' : ' lens-drill-action-primary'}`}
                  href={target.leafHref}
                >
                  <span>{translate('table.openRecord', 'Open record')}</span>
                  <ArrowUpRight />
                </a>
              </Show>
              <Show when={empty}>
                <p class="lens-drill-empty">{translate('explore.noDetail', 'No further detail')}</p>
              </Show>
            </footer>
          </div>
        </Portal>
      )}
    </Show>
  )
}
