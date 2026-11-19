import type { ContextOrigin } from '../contract'
import { compactRangeLabel, parseISODate } from '../controls/model'
import type { useTranslate } from '../runtime'
import type { InvestigationSlice } from './investigation'

type Translate = ReturnType<typeof useTranslate>

/** The period of a slice as the reader saw it, in the calendar's own dates. */
export function slicePeriodLabel(period: InvestigationSlice['period'], translate: Translate): string {
  const start = period.start ? parseISODate(period.start) : undefined
  const end = period.end ? parseISODate(period.end) : undefined
  if (start && end) return compactRangeLabel(start, end)
  return period.selection.mode === 'all' ? translate('filter.period.allTime', 'All time') : translate('filter.period.custom', 'Custom range')
}

/** Where a filter came from, in the reader's words. */
export function originLabel(origin: ContextOrigin, translate: Translate, names?: Record<string, string>): string {
  if (origin.scope === 'drill') return translate('explore.origin.drill', 'Exploration: {level}', { level: origin.id })
  const scope = (() => {
    switch (origin.scope) {
      case 'report': return translate('filter.context.scope.report', 'Report')
      case 'dashboard': return translate('filter.context.scope.dashboard', 'Dashboard')
      case 'section': return translate('filter.context.scope.section', 'Section')
      case 'card': return translate('filter.context.scope.card', 'Card')
      case 'group': return translate('filter.context.scope.group', 'Exploration')
      default: return translate('filter.context.scope.host', 'Host default')
    }
  })()
  const name = names?.[`${origin.scope}:${origin.id}`]
  return name ? `${scope}: ${name}` : scope
}

