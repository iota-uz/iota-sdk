import { isVisualRegression } from '../../visualRegression'
import { copyText } from '../../runtime/clipboard'
import type { EChartsTheme } from './theme'

/**
 * Tooltips render at `body` level, not inside the chart container: a panel
 * card clips its own overflow, so a tooltip anchored near the card edge was
 * cut off. From `body` ECharts flips it against the viewport instead, and the
 * pinned z-index keeps it above the expanded-panel dialog, which portals to
 * `body` too.
 */
export const tooltipZIndex = 2147483600

/**
 * Marks every tooltip node this runtime creates. Living outside the chart
 * container, a tooltip is no longer torn down with it — the adapter needs a way
 * to find the nodes it is responsible for without touching anything else the
 * host page appended to `body`.
 */
export const tooltipClassName = 'lens-echarts-tooltip'

const glyphAttributes = 'fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="16"'
/** The same two Phosphor paths `icons.tsx` draws, as markup ECharts can print. */
const copyGlyph = `<svg aria-hidden="true" focusable="false" height="13" width="13" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg"><rect x="88" y="88" width="128" height="128" rx="8" ${glyphAttributes}/><path d="M40,168H32a8,8,0,0,1-8-8V40a8,8,0,0,1,8-8H160a8,8,0,0,1,8,8v8" ${glyphAttributes}/></svg>`
const checkGlyph = `<svg aria-hidden="true" focusable="false" height="13" width="13" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg"><polyline points="216 72 104 184 48 128" ${glyphAttributes}/></svg>`

/**
 * Takes the amount it sits beside, unformatted, to the clipboard.
 *
 * The button is markup rather than a component because an ECharts tooltip is an
 * HTML string the chart owns; the click is answered by one delegate below, for
 * the same reason. `data-lens-copy` carries the machine value — plain digits in
 * the unit the figure is drawn in — so the delegate never has to parse a
 * formatted number back into one.
 */
export function tooltipCopyButton(raw: string | undefined, label: string): string {
  if (raw === undefined) return ''
  return `<button type="button" class="lens-tooltip-copy" data-lens-copy="${escape(raw)}" aria-label="${escape(label)}" title="${escape(label)}">${copyGlyph}</button>`
}

let delegateInstalled = false

/**
 * One listener for every chart's tooltips, installed on first mount.
 *
 * A tooltip is handed to `body` and rebuilt on every hover, so nothing inside it
 * can own a listener; and a listener per chart would answer one click as many
 * times as there are charts on the page. This is scoped to the class this
 * runtime puts on its own tooltip nodes, so it never sees a click the host
 * page's own DOM produced.
 */
export function installTooltipCopyDelegate(copiedLabel: () => string): void {
  if (delegateInstalled || typeof document === 'undefined') return
  delegateInstalled = true
  document.addEventListener('click', (event) => {
    const target = event.target
    if (!(target instanceof Element)) return
    const button = target.closest<HTMLElement>(`.${tooltipClassName} [data-lens-copy]`)
    if (!button) return
    // A tooltip stands over a mark that is often itself a drill target, and a
    // copy is not a drill.
    event.preventDefault()
    event.stopPropagation()
    // Confirmed on the act, not on the promise: an unfocused document can leave
    // `clipboard.writeText` pending indefinitely rather than rejecting, and a
    // button that never confirms reads as a broken one.
    void copyText(button.dataset.lensCopy ?? '')
    const label = copiedLabel()
    button.dataset.copied = 'true'
    button.setAttribute('aria-label', label)
    button.setAttribute('title', label)
    button.innerHTML = checkGlyph
  }, true)
}

function escape(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

/**
 * What clicking this mark does, at the foot of the card that names the mark.
 *
 * The same sentence used to be the accessible name of a glyph in the notes row
 * above the plot — the corner of the card furthest from where the reader is
 * pointing, and the one place a pointer never visits. Here it costs nothing at
 * rest, appears with the reading it applies to, and is already under the
 * cursor. Empty when the plot is inert: a tooltip must not offer a click that
 * does nothing.
 */
export function tooltipActionFooter(hint: string | undefined): string {
  if (!hint) return ''
  return `<div style="margin-top:6px;padding-top:6px;border-top:1px solid currentColor;opacity:0.5;font-size:11px">${escape(hint)}</div>`
}

/**
 * Tooltip settings shared by every chart kind.
 *
 * The chrome is the app's popover chrome, resolved from the same tokens: a
 * chart tooltip used to be a 4px box with a flat hairline and a minimal shadow,
 * beside facet popovers and info tips built on `--lens-radius-card` and
 * `--lens-shadow-popover`. Two floating surfaces on one page were two different
 * objects for no reason a reader could name.
 *
 * It lives in its own module because the distribution kinds need it too, and
 * they are reached *from* the option builder — importing it back out of that
 * file would close a cycle. Not importing it is what left histogram, boxplot
 * and heatmap tooltips clipped by the card for as long as those kinds existed.
 */
export function tooltipChrome(theme: EChartsTheme) {
  return {
    backgroundColor: theme.card,
    borderColor: theme.border,
    borderRadius: theme.cardRadius,
    padding: [8, 10] as [number, number],
    textStyle: { color: theme.text, fontSize: theme.type.base },
    appendTo: 'body',
    className: tooltipClassName,
    // Rendered at body level the tooltip escapes the card's clip, but it must
    // still stay on screen: confine clamps it to the window viewport so a wide
    // tooltip on a left-edge slice no longer overflows behind the sidebar
    // instead of merely flipping.
    confine: true,
    // ECharts paints the box itself, but the elevation has to be CSS: the node
    // lives on `body`, where `var(--lens-shadow-popover)` does not resolve, so
    // the theme hands over the value it read inside `.lens-root`.
    extraCssText: `z-index: ${tooltipZIndex}; box-shadow: ${theme.popoverShadow};`,
    // A moving tooltip is unscreenshotable; VR pins it in place.
    transitionDuration: isVisualRegression() ? 0 : undefined,
    // The card carries a copy button, so the pointer has to be able to reach
    // it: without `enterable` the tooltip is dismissed the moment the pointer
    // leaves the mark, and the extra hide delay is the travel time across the
    // gap ECharts leaves between the two.
    enterable: true,
    hideDelay: 240,
  }
}
