import { useFilters, useTranslate } from '../runtime'
import { PeriodFilterControl } from './PeriodFilterControl'
import type { CalendarDate } from './model'

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
  const { filters } = useFilters()
  const translate = useTranslate()
  if (filters.length === 0) return null
  return (
    <div aria-label={translate('filter.bar.label', 'Dashboard filters')} className="lens-filter-bar" role="group">
      {filters.map((filter) => (
        filter.kind === 'period' && filter.period
          ? <PeriodFilterControl filter={filter} key={filter.id} today={today} />
          : null
      ))}
    </div>
  )
}
