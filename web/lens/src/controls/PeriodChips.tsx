import { FilterBuilder, type FilterBuilderField, type FilterCondition } from '@iota-uz/sdk/solid-ui'
import kitStyles from '@iota-uz/sdk/solid-ui/embedded-css'
import { currentPeriodValue, useFilters, useTranslate } from '../runtime'
import type { Filter, PeriodValue } from '../contract'
import { formatDisplayDate, localToday, periodChoices } from './PeriodFilterControl'
import { parseISODate, type CalendarDate } from './model'

/**
 * The shared filter-chip kit's utilities, installed once per page. The kit
 * expects a host carrying the SDK stylesheet (tokens and base), which is what
 * an embedded dashboard sits on; the sheet adds only the utilities the chip
 * components use, so a host whose lists filter by chips and its dashboard
 * render the same chip.
 *
 * The sheet goes first in <head>, ahead of the host's own. Both are Tailwind
 * builds, and their `@layer utilities` blocks merge into one layer where the
 * later rule wins on equal specificity: appended, the kit's plain `.hidden`
 * landed after the host's `lg:block` and hid the host's desktop sidebar on
 * every page with a dashboard period chip. First in the head, the kit only
 * fills in what the host does not define. Its layer order (properties, theme,
 * base, components, utilities) is Tailwind's own, so declaring the layers
 * first does not reorder the host's.
 */
export function installKitStyles(sheet: string = kitStyles): void {
  if (typeof document === 'undefined' || document.querySelector('style[data-lens-solid-ui]')) return
  const style = document.createElement('style')
  style.dataset.lensSolidUi = ''
  style.textContent = sheet
  document.head.prepend(style)
}

/** "01.07.2026 – 30.09.2026"; a half-open range keeps its open side empty. */
function displayRange(value: PeriodValue): string {
  const start = parseISODate(value.start)
  const end = parseISODate(value.end)
  if (!start && !end) return ''
  return `${start ? formatDisplayDate(start) : '…'} – ${end ? formatDisplayDate(end) : '…'}`
}

/**
 * The range a chip edit may write: ordered, inside the period's min / max
 * (clamped, so the URL never carries a range the server would reject), and
 * closed unless the period allows open ends. Undefined rejects the edit.
 */
export function chipRange(period: NonNullable<Filter['period']>, start: string, end: string): PeriodValue | undefined {
  if (start && end && end < start) [start, end] = [end, start]
  if (!period.allowEmpty && (!start || !end)) return undefined
  if (!start && !end) return undefined
  if (period.min && start && start < period.min) start = period.min
  if (period.max && end && end > period.max) end = period.max
  if (period.max && start && start > period.max) return undefined
  if (period.min && end && end < period.min) return undefined
  return { start, end }
}

export interface PeriodChipsProps {
  /** Period filters declared with the filter_chips component. */
  filters: Filter[]
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
}

/**
 * Period filters as chips of the shared filter builder (solid-ui
 * FilterBuilder): the same chip, editor and keyboard handling the host's
 * lists use. A chip reads «Период · Этот квартал» or «Период · 01.07.2026 –
 * 30.09.2026»; opening it offers the period's presets and a custom range.
 * State goes through the filters context exactly like the calendar control —
 * the document URL's start/end parameters — so links, refresh, back, drill
 * and export keep the period. A period that cannot be empty is a required
 * chip: it has no remove button and "Clear all" keeps it.
 */
export function PeriodChips(props: PeriodChipsProps) {
  const { values, setPeriod } = useFilters()
  const translate = useTranslate()
  installKitStyles()
  const today = () => props.today ?? localToday()
  const periods = () => props.filters.filter((filter) => filter.kind === 'period' && filter.period)
  const choices = (filter: Filter) => periodChoices(filter.period!, today(), translate)
  const fields = (): FilterBuilderField[] => periods().map((filter) => ({
    key: filter.id,
    label: filter.label ?? translate('filter.period.open', 'Change period'),
    type: 'date',
    operators: ['between'],
    required: !filter.period!.allowEmpty,
    presetOptions: choices(filter).map((choice) => ({ id: choice.id, label: choice.label, values: [choice.value.start, choice.value.end] })),
  }))
  const conditions = (): FilterCondition[] => periods().flatMap((filter) => {
    const value = currentPeriodValue(filter.period!, values)
    // A half-open range stays a chip: dropping it would read as "removed"
    // the next time another period is applied.
    if (!value.start && !value.end) return []
    const preset = choices(filter).find((choice) => choice.value.start === value.start && choice.value.end === value.end)
    return [{ field: filter.id, operator: 'between', values: [value.start, value.end], ...(preset ? { preset: preset.id } : {}) }]
  })
  const apply = (next: FilterCondition[]) => {
    for (const filter of periods()) {
      const condition = next.find((item) => item.field === filter.id)
      const current = currentPeriodValue(filter.period!, values)
      if (condition && condition.values.length === 2) {
        const [start, end] = condition.values as [string, string]
        if (start === current.start && end === current.end) continue
        const range = chipRange(filter.period!, start, end)
        if (range && (range.start !== current.start || range.end !== current.end)) setPeriod(filter, range)
      } else if (!condition && filter.period!.allowEmpty && (current.start || current.end)) {
        setPeriod(filter, { start: '', end: '' })
      }
    }
  }
  return (
    <FilterBuilder
      class="lens-period-chips"
      data-testid="period-chips"
      fields={fields()}
      conditions={conditions()}
      onConditionsChange={apply}
      operatorLabel={() => ''}
      addLabel={translate('filter.chips.add', 'Add filter')}
      addDialogLabel={translate('filter.chips.addDialog', 'Add filter')}
      searchPlaceholder={translate('filter.chips.searchFields', 'Search fields')}
      formatSummary={(field, condition) => {
        const preset = condition.preset ? field.presetOptions?.find((item) => item.id === condition.preset) : undefined
        return preset?.label ?? displayRange({ start: condition.values[0] ?? '', end: condition.values[1] ?? '' })
      }}
      applyLabel={translate('filter.period.apply', 'Apply')}
      removeLabel={translate('filter.facet.remove', 'Remove filter')}
      clearLabel={translate('filter.facet.clearAll', 'Clear all')}
    />
  )
}
