import { useDashboard, useFilters, useTranslate } from '../runtime'
import { X } from '../icons'
import { CompareFilterControl } from './CompareFilterControl'
import { FacetFilterMenu } from './FacetFilterMenu'
import { PeriodFilterControl } from './PeriodFilterControl'
import { SegmentedFilterControl } from './SegmentedFilterControl'
import type { CalendarDate } from './model'
import { clearRendererStateFromURL } from '../runtime/url'
import type { Filter } from '../contract'

export interface FilterBarProps {
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
  /**
   * The producer's own scope line, printed after the controls. It states what
   * the controls cannot — a data cut-off, a scope caveat — so it reads as the
   * tail of the same sentence rather than as a second heading.
   */
  subtitle?: string
}

interface ActiveChip {
  key: string
  label: string
  removeUrl: string
}

export function FilterControls({ filters, today }: { filters: Filter[], today?: CalendarDate }) {
  const facets = filters.filter((filter) => filter.kind === 'facet' && filter.facet)
  return (
    <>
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
    </>
  )
}

/**
 * The declared dashboard controls, rendered in the header chrome. Empty when
 * the document declares none, carries no applied filter and states no scope
 * line of its own (and inside drawers, where the context hands out no filters).
 *
 * Two rows, and the first one never moves: the controls that open something
 * (period, comparison, filters) sit on the scope row, and everything that is
 * currently *on* sits on the chip row below it. A chip used to be injected
 * between the triggers, so applying one filter re-wrapped the whole bar and the
 * next trigger the reader was aiming at had moved.
 *
 * The scope row reads as a sentence about what is on screen — «1 янв — 4 авг
 * 2026 · Без сравнения · данные по 4 авг» — with each segment editable in
 * place. It was a row of bordered boxes at the same weight as Экспорт and
 * Пересчитать beside it, which gave a reader no way to tell the controls that
 * change the figures from the ones that carry them away.
 */
export function FilterBar({ subtitle, today }: FilterBarProps) {
  const { filters, applyURL } = useFilters()
  const { document } = useDashboard()
  const translate = useTranslate()
  const globalFilters = filters.filter((filter) => !filter.placement)
  if (globalFilters.length === 0 && (document.activeFilters?.length ?? 0) === 0 && !subtitle) return null
  const intercept = (url: string) => (event: React.MouseEvent<HTMLElement>) => {
    if ('metaKey' in event && (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey)) return
    event.preventDefault()
    applyURL(url)
  }
  const facets = globalFilters.filter((filter) => filter.kind === 'facet' && filter.facet)
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
      <div className="lens-dashboard-scope">
        {/* Declaration order is render order, and the row only wraps — it never
            reorders — so a control keeps the same neighbours at every width. */}
        <FilterControls filters={globalFilters} today={today} />
        {/* Not a heading and not a control: the part of the scope no control
            owns. It sits last so the editable segments stay together at the
            left edge, where the eye lands. */}
        {subtitle && <p className="lens-dashboard-subtitle">{subtitle}</p>}
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
