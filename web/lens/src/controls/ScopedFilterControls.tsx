import { createMemo, For, Show } from 'solid-js'
import type { ContextBoard, ContextEditor, ContextFilters, EffectiveContext, Filter, PeriodValue } from '../contract/types'
import { useDashboard, useFilters, useTranslate } from '../runtime'
import { createFilterContextDraft, filterBoardURL } from '../reports/filterDraft'
import { PeriodFilterControl } from './PeriodFilterControl'
import { FacetFilterMenu } from './FacetFilterMenu'
import { parseISODate } from './model'

type Scope = 'dashboard' | 'card' | 'section'
function scopeValue(board: ContextBoard, scope: Scope, id: string): ContextFilters {
  return scope === 'dashboard' ? board.dashboard : scope === 'card' ? board.cards?.[id] ?? {} : board.sections?.[id] ?? {}
}

export function ScopedFilterControls(props: { editor: ContextEditor; scope: Scope; id: string; effective?: EffectiveContext; facets?: Filter[]; saveDefaults?: (filters: ContextFilters) => Promise<void> }) {
  const { applyURL } = useFilters()
  const translate = useTranslate()
  const draft = createFilterContextDraft(scopeValue(props.editor.value, props.scope, props.id), scopeValue(props.editor.saved, props.scope, props.id), {
    apply(value) {
      const board = structuredClone(props.editor.value)
      if (props.scope === 'dashboard') board.dashboard = value
      else if (props.scope === 'card') board.cards = { ...board.cards, [props.id]: value }
      else board.sections = { ...board.sections, [props.id]: value }
      applyURL(filterBoardURL(props.editor.url, board))
    },
    saveDefaults: props.saveDefaults,
  })
  const inherited = () => !draft.value().period || draft.value().period?.mode === 'reports'
  const inheritText = () => props.scope === 'dashboard'
    ? translate('filter.context.reportPeriods', 'Report periods')
    : translate('filter.context.useDashboard', 'Use dashboard period')
  const selectedRange = (): PeriodValue => {
    const period = draft.value().period
    if (period?.mode === 'fixed') return { start: period.start ?? '', end: period.end ?? '' }
    if (period?.mode === 'relative') return { start: props.effective?.period.start ?? '', end: props.effective?.period.end ?? '' }
    return { start: '', end: '' }
  }
  const filter: Filter = { id: `context-${props.scope}-${props.id}`, kind: 'period', period: {
    startParam: 'context-start', endParam: 'context-end', allowEmpty: true,
    get value() { return selectedRange() },
  } }
  const hostToday = () => parseISODate(new Intl.DateTimeFormat('en-CA', { timeZone: props.editor.value.calendar.timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(props.editor.value.calendar.asOf)))
  const onPeriod = (value: PeriodValue, preset?: string) => draft.update((current) => ({ ...current, period:
    preset === 'last30days' ? { mode: 'relative', days: 30 }
      : preset === 'last12months' ? { mode: 'relative', months: 12 }
        : value.start && value.end ? { mode: 'fixed', start: value.start, end: value.end } : { mode: 'all' },
  }))
  const stageFacets = (_url: string, selections: ReadonlyMap<string, ReadonlySet<string>>) => {
    draft.update((current) => {
      const constraints = new Map((current.constraints ?? []).map((constraint) => [constraint.dimension, constraint.values]))
      for (const [dimension, values] of selections) {
        if (values.size) constraints.set(dimension, [...values])
        else constraints.delete(dimension)
      }
      return { ...current, constraints: [...constraints].map(([dimension, values]) => ({ dimension, values })) }
    })
  }
  const constraints = createMemo(() => draft.value().constraints ?? [])
  return <div class="lens-scoped-controls" data-testid={`context-controls-${props.scope}-${props.id}`}>
    <Show when={props.scope === 'section'}><strong>{translate('filter.context.section', 'Only this section')}: {props.id}</strong></Show>
    <PeriodFilterControl filter={filter} today={hostToday()} inheritLabel={inherited() ? inheritText() : undefined} onChange={onPeriod} />
    <Show when={!inherited()}><button type="button" class="lens-context-action" data-testid="context-inherit" onClick={() => draft.update((value) => ({ ...value, period: props.scope === 'dashboard' ? { mode: 'reports' } : undefined }))}>{inheritText()}</button></Show>
    <Show when={props.facets?.length}><FacetFilterMenu filters={props.facets!} onApplyURL={stageFacets} /></Show>
    <For each={constraints()}>{(constraint) => <span class="lens-active-filter-chip">{props.editor.dimensionLabels?.[constraint.dimension] || constraint.dimension}: {constraint.values.join(', ')}</span>}</For>
    <button type="button" class="lens-context-action" data-testid="context-apply" disabled={!draft.dirty()} onClick={() => draft.apply()}>{translate('filter.context.apply', 'Apply')}</button>
    <button type="button" class="lens-context-action" data-testid="context-reset" onClick={() => draft.reset()}>{translate('filter.context.reset', 'Restore saved filters')}</button>
    <Show when={draft.dirty()}><span role="status">{translate('filter.context.staged', 'Changes are not applied')}</span></Show>
    <Show when={props.saveDefaults}><button type="button" class="lens-context-action" disabled={draft.saving()} onClick={() => void draft.saveDefaults().catch(() => undefined)}>{translate('filter.context.save', 'Save filter defaults')}</button></Show>
    <Show when={draft.error()}><p role="alert">{translate('filter.context.saveFailed', 'Filter defaults could not be saved')}</p></Show>
  </div>
}

export function FilterContextInfo(props: { context: EffectiveContext; labels?: Record<string, string> }) {
  const translate = useTranslate()
  const label = (scope: string) => {
    switch (scope) {
      case 'report': return translate('filter.context.scope.report', 'Report')
      case 'dashboard': return translate('filter.context.scope.dashboard', 'Dashboard')
      case 'section': return translate('filter.context.scope.section', 'Section')
      case 'card': return translate('filter.context.scope.card', 'Card')
      case 'group': return translate('filter.context.scope.group', 'Exploration')
      default: return translate('filter.context.scope.host', 'Host default')
    }
  }
  return <div class="lens-context-info" data-testid="effective-context" data-context-id={props.context.identity}>
    <p>{props.context.period.start ? `${props.context.period.start} – ${props.context.period.end}` : translate('filter.period.allTime', 'All time')} · {label(props.context.period.origin.scope)} · {props.context.calendar.timezone}</p>
    <For each={props.context.restrictions}>{(restriction) => <p>{props.labels?.[restriction.dimension] || restriction.dimension}: {restriction.values.join(', ')} · {label(restriction.origin.scope)} ({restriction.origin.id})</p>}</For>
    <Show when={props.context.conflicts.length}><p role="status" data-testid="context-conflict">{translate('filter.context.conflict', 'No data for this combination of filters')}</p></Show>
    <Show when={props.context.unsupported.length}><p role="status" data-testid="context-unsupported">{translate('filter.context.unsupported', 'This report does not support these filters')}: {props.context.unsupported.map((value) => props.labels?.[value.dimension] || value.dimension).join(', ')}</p></Show>
  </div>
}

export function ContextGroupControls(props: { id: string; context: EffectiveContext }) {
  const { document } = useDashboard()
  const editor = () => document.contextEditor
  const section = () => props.context.input.layers.find((layer) => layer.origin.scope === 'section')?.origin.id
  const firstSectionCard = () => {
    const groups = document.layout.rows.flatMap((row) => row.panels.flatMap((item) => item.groups ?? []))
    return groups.find((group) => group.filterContext?.input.layers.some((layer) => layer.origin.scope === 'section' && layer.origin.id === section()))?.id
  }
  return <>
    <FilterContextInfo context={props.context} labels={editor()?.dimensionLabels} />
    <Show when={editor()}>{(value) => <>
      <Show when={section() && firstSectionCard() === props.id}><ScopedFilterControls editor={value()} scope="section" id={section()!} effective={props.context} /></Show>
      <ScopedFilterControls editor={value()} scope="card" id={props.id} effective={props.context} />
    </>}</Show>
  </>
}
