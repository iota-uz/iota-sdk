/**
 * Every width at which the runtime changes what it draws, in one place.
 *
 * These are not the layout breakpoints — the stylesheet owns those, and a
 * responsive grid needs no help from JavaScript. These are the two widths where
 * a *component decides differently*: a decision that lives in TypeScript because
 * CSS cannot express it, and that therefore has to be named somewhere a reader
 * can find. Both were bare literals sitting a thousand lines apart, which is how
 * "compact" came to mean two different widths on one page.
 *
 * A value here is paired with the rule it protects. Change one and you are
 * changing that rule, so state why in the comment beside it.
 */

/**
 * The period popover shows two month panes side by side. Below this the pair no
 * longer fits between the viewport padding, so the calendar renders a single
 * pane and steps a month at a time.
 *
 * Mirrors the `@media (max-width: 664px)` block in `styles.css` that narrows
 * the popover itself: the pane count and the popover width have to change on
 * the same edge, or the card is sized for two panes while showing one.
 */
export const stackedCalendarMaxWidth = 664

/** `stackedCalendarMaxWidth` as a media query, for `matchMedia`. */
export const stackedCalendarMediaQuery = `(max-width: ${stackedCalendarMaxWidth}px)`

/**
 * Plot width below which a part-to-whole chart drops to its compact labels: the
 * slice's own name stops fitting inside the arc, so the label becomes the value
 * alone and the names move to the legend.
 *
 * This is the *plot box*, not the viewport — a 400px donut in a four-up row is
 * cramped on a 1440px screen. The mounted adapter watches the box and re-renders
 * when it crosses this edge, because ECharts label layout is decided at build
 * time and a resize alone will not revisit it.
 */
export const compactChartLabelWidth = 500
