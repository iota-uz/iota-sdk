import { useDashboard, useFilters, useTranslate } from '../runtime'
import { X } from '../icons'
import { CompareFilterControl } from './CompareFilterControl'
import { FacetFilterMenu } from './FacetFilterMenu'
import { PeriodFilterControl } from './PeriodFilterControl'
import { SegmentedFilterControl } from './SegmentedFilterControl'
import type { CalendarDate } from './model'
import { clearRendererStateFromURL } from '../runtime/url'

export interface FilterBarProps {
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
}

interface ActiveChip {
  key: string
  label: string
  removeUrl: string
}

/**
 * The declared dashboard controls, rendered in the header chrome. Empty when
 * the document declares none (and inside drawers, where the context hands out
 * no filters).
 *
 * Two rows, and the first one never moves: the controls that open something
 * (period, comparison, filters) sit on the trigger row, and everything that is
 * currently *on* sits on the chip row below it. A chip used to be injected
 * between the triggers, so applying one filter re-wrapped the whole bar and the
 * next trigger the reader was aiming at had moved.
 */
export function FilterBar({ today }: FilterBarProps) {
  const { filters, applyURL } = useFilters()
  const { document } = useDashboard()
  const translate = useTranslate()
  if (filters.length === 0 && (document.activeFilters?.length ?? 0) === 0) return null
  const intercept = (url: string) => (event: React.MouseEvent<HTMLElement>) => {
    if ('metaKey' in event && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) return
    event.preventDefault()
    applyURL(url)
  }
  const facets = filters.filter((filter) => filter.kind === 'facet' && filter.facet)
  const clearURL = facets.find((filter) =>
    (filter.facet?.selections?.length ?? 0) > 0 && Boolean(filter.facet?.clearUrl),
  )?.facet?.clearUrl
  const representedFacetDimensions = new Set(facets.flatMap((filter) =>
    filter.facet ? [filter.facet.dimension] : [],
  ))
  // Every applied selection is a chip, wherever the document states it: the
  // facet's own `selections` and the document-level `activeFilters` that cover
  // dimensions no declared facet represents.
  const chips: Array<ActiveChip> = [
    ...facets.flatMap((filter) => (filter.facet?.selections ?? []).map((selection) => ({
      key: `${filter.id}:${selection.label}:${selection.removeUrl}`,
      label: selection.label,
      removeUrl: selection.removeUrl,
    }))),
    ...(document.activeFilters ?? [])
      .filter((filter) => !representedFacetDimensions.has(filter.dimension))
      .map((filter) => ({
        key: `${filter.dimension}:${filter.value}`,
        label: filter.label,
        removeUrl: filter.removeUrl,
      })),
  ]
  const resetURL = document.resetFiltersUrl
    ? clearRendererStateFromURL(document.resetFiltersUrl, new URL(window.location.href))
    : undefined
  const clearAllURL = resetURL && (document.activeFilters?.length ?? 0) > 0
    ? resetURL
    : !document.resetFiltersUrl ? clearURL : undefined

  return (
    <div aria-label={translate('filter.bar.label', 'Dashboard filters')} className="lens-filter-bar" role="group">
      <div className="lens-filter-bar-controls">
        {/* Declaration order is render order, and the row only wraps — it never
            reorders — so a control keeps the same neighbours at every width. */}
        {filters.map((filter) => (
          filter.kind === 'period' && filter.period
            ? <PeriodFilterControl filter={filter} key={filter.id} today={today} />
            : filter.kind === 'compare' && filter.compare
              ? <CompareFilterControl filter={filter} key={filter.id} />
              : filter.kind === 'segmented' && filter.segmented
                ? <SegmentedFilterControl filter={filter} key={filter.id} />
                : null
        ))}
        {facets.length > 0 && <FacetFilterMenu filters={facets} />}
      </div>
      {(chips.length > 0 || clearAllURL) && (
        <div className="lens-filter-bar-chips">
          {chips.map((chip) => (
            <a
              aria-label={`${translate('filter.facet.remove', 'Remove filter')}: ${chip.label}`}
              className="lens-facet-active-chip"
              href={chip.removeUrl}
              key={chip.key}
              onClick={intercept(chip.removeUrl)}
            >
              <span>{chip.label}</span><X aria-hidden="true" />
            </a>
          ))}
          {/* A button, not a link: it does not navigate anywhere a reader could
              bookmark, it dismisses the chips beside it. It also stays at the
              end of the chip row instead of flowing among the triggers. */}
          {clearAllURL && (
            <button className="lens-facet-clear" onClick={intercept(clearAllURL)} type="button">
              {translate('filter.facet.clearAll', 'Clear all')}
            </button>
          )}
        </div>
      )}
    </div>
  )
}
