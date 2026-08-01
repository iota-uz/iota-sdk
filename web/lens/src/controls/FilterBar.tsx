import { useDashboard, useFilters, useTranslate } from '../runtime'
import { X } from '../icons'
import { CompareFilterControl } from './CompareFilterControl'
import { FacetFilterControl } from './FacetFilterControl'
import { PeriodFilterControl } from './PeriodFilterControl'
import type { CalendarDate } from './model'
import { clearRendererStateFromURL } from '../runtime/url'

export interface FilterBarProps {
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
}

/**
 * The declared dashboard controls, rendered in the header chrome. Empty when
 * the document declares none (and inside drawers, where the context hands out
 * no filters).
 */
export function FilterBar({ today }: FilterBarProps) {
  const { filters, applyURL } = useFilters()
  const { document } = useDashboard()
  const translate = useTranslate()
  if (filters.length === 0 && (document.activeFilters?.length ?? 0) === 0) return null
  const intercept = (url: string) => (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    event.preventDefault()
    applyURL(url)
  }
  const clearURL = filters.find((filter) =>
    filter.kind === 'facet' &&
    (filter.facet?.selections?.length ?? 0) > 0 &&
    Boolean(filter.facet?.clearUrl),
  )?.facet?.clearUrl
  const representedFacetDimensions = new Set(filters.flatMap((filter) =>
    filter.kind === 'facet' && filter.facet ? [filter.facet.dimension] : [],
  ))
  const extraActiveFilters = document.activeFilters?.filter((filter) =>
    !representedFacetDimensions.has(filter.dimension),
  )
  const resetURL = document.resetFiltersUrl
    ? clearRendererStateFromURL(document.resetFiltersUrl, new URL(window.location.href))
    : undefined
  return (
    <div aria-label={translate('filter.bar.label', 'Dashboard filters')} className="lens-filter-bar" role="group">
      {filters.map((filter) => (
        filter.kind === 'period' && filter.period
          ? <PeriodFilterControl filter={filter} key={filter.id} today={today} />
          : filter.kind === 'facet' && filter.facet
            ? <FacetFilterControl filter={filter} key={filter.id} />
            : filter.kind === 'compare' && filter.compare
              ? <CompareFilterControl filter={filter} key={filter.id} />
              : null
      ))}
      {extraActiveFilters?.map((filter) => (
        <a aria-label={`${translate('filter.facet.remove', 'Remove filter')}: ${filter.label}`} className="lens-facet-active-chip" href={filter.removeUrl} key={`${filter.dimension}:${filter.value}`} onClick={intercept(filter.removeUrl)}>
          <span>{filter.label}</span><X aria-hidden="true" />
        </a>
      ))}
      {resetURL && (document.activeFilters?.length ?? 0) > 0 && (
        <a className="lens-facet-clear" href={resetURL} onClick={intercept(resetURL)}>{translate('filter.facet.clearAll', 'Clear all')}</a>
      )}
      {clearURL && !document.resetFiltersUrl && (
        <a className="lens-facet-clear" href={clearURL}>
          {translate('filter.facet.clearAll', 'Clear all')}
        </a>
      )}
    </div>
  )
}
