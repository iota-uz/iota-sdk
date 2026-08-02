import { isVisualRegression } from '../../visualRegression'
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
  }
}
