/**
 * When a list of toggleable rows earns a search field.
 *
 * Two components render such a list — the chart legend and the facet filter —
 * and they used to answer this question separately: the legend showed its
 * search above six entries, the facet showed one unconditionally, so a facet
 * holding a single region rendered a search box that could only ever filter
 * that one row down to nothing. The threshold lives here so the two cannot
 * disagree again.
 */
export const searchableListEntries = 6
