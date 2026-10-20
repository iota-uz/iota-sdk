import { createEffect, createMemo, createUniqueId, For, onCleanup, onMount, Show } from 'solid-js'
import { unwrap } from 'solid-js/store'
import type { ContextBoard, ContextCalendar, ContextEditor, ContextFilters, ContextOrigin, ContextPeriod, EffectiveContext, Filter, PeriodValue } from '../contract/types'
import { useDashboard, useFilters, useTranslate } from '../runtime'
import { CaretDown, FunnelSimple } from '../icons'
import { createFilterContextDraft, filterBoardURL } from '../reports/filterDraft'
import { PeriodFilterControl } from './PeriodFilterControl'
import { FacetFilterMenu } from './FacetFilterMenu'
import { parseISODate, addDays, addMonths, compactRangeLabel, formatISODate } from './model'
import { useFilterDefaultsHost } from '../reports/filterHost'

type Scope = 'dashboard' | 'card' | 'section'
function scopeValue(board: ContextBoard, scope: Scope, id: string): ContextFilters {
  return scope === 'dashboard' ? board.dashboard : scope === 'card' ? board.cards?.[id] ?? {} : board.sections?.[id] ?? {}
}

type ScopedFilterProps = { editor: ContextEditor; scope: Scope; id: string; effective?: EffectiveContext; facets?: Filter[]; saveDefaults?: (filters: ContextFilters) => Promise<void> }

const focusableControl = 'button:not(:disabled):not([aria-disabled="true"]), input:not(:disabled), a[href]'

/** Every selection applies at once and remounts the editor, so focus is handed back to the control the reader operated. */
function focusLost(root: HTMLElement) {
  const active = root.ownerDocument.activeElement
  return !active || active === root.ownerDocument.body || !active.isConnected
}

const periodTrigger = '.lens-filter-trigger'
const facetTrigger = '.lens-facet-trigger'

function focusEditor(root: HTMLElement, target: string) {
  if (!root.isConnected || !focusLost(root)) return
  ;(root.querySelector<HTMLElement>(target) ?? root.querySelector<HTMLElement>(focusableControl))?.focus()
}

export function ScopedFilterControls(props: ScopedFilterProps) {
  const { document } = useDashboard()
  const identity = createMemo(() => JSON.stringify([document.snapshotId, props.scope, props.id, props.editor, props.effective?.identity]))
  const focus: { pending?: string } = {}
  return <Show when={identity()} keyed>{(identity) => identity && <ScopedFilterDraftControls {...props} focus={focus} />}</Show>
}

function ScopedFilterDraftControls(props: ScopedFilterProps & { focus: { pending?: string } }) {
  let root: HTMLDivElement | undefined
  onMount(() => {
    const target = props.focus.pending
    if (!target || !root) return
    const element = root
    requestAnimationFrame(() => {
      if (!element.isConnected) return
      props.focus.pending = undefined
      focusEditor(element, target)
    })
  })
  const keepFocus = (target: string) => {
    props.focus.pending = target
    const element = root
    if (element) requestAnimationFrame(() => focusEditor(element, target))
  }
  const { applyURL } = useFilters()
  const translate = useTranslate()
  const host = useFilterDefaultsHost()
  const origin = { scope: props.scope, id: props.id }
  const saveDefaults = props.saveDefaults ?? (host?.canSave(origin) ? (filters: ContextFilters) => host.saveDefaults(origin, filters) : undefined)
  const draft = createFilterContextDraft(scopeValue(props.editor.value, props.scope, props.id), scopeValue(props.editor.saved, props.scope, props.id), {
    apply(value) {
      const board = structuredClone(unwrap(props.editor.value))
      if (props.scope === 'dashboard') board.dashboard = value
      else if (props.scope === 'card') board.cards = { ...board.cards, [props.id]: value }
      else board.sections = { ...board.sections, [props.id]: value }
      applyURL(filterBoardURL(props.editor.url, board))
    },
    saveDefaults,
  })
  const inherited = () => inheritsPeriod(draft.value().period)
  const inheritText = () => props.scope === 'dashboard'
    ? translate('filter.context.reportPeriods', 'Report periods')
    : translate('filter.context.useDashboard', 'Use dashboard period')
  const selectedRange = (): PeriodValue => periodRange(draft.value().period, props.editor.value.calendar)
  const filter: Filter = { id: `context-${props.scope}-${props.id}`, kind: 'period', period: {
    startParam: 'context-start', endParam: 'context-end', allowEmpty: true,
    get value() { return selectedRange() },
  } }
  const hostToday = () => calendarToday(props.editor.value.calendar)
  const commit = (change: (value: ContextFilters) => ContextFilters, focusTarget: string) => {
    draft.update(change)
    if (!draft.dirty()) return
    draft.apply()
    keepFocus(focusTarget)
  }
  const onPeriod = (value: PeriodValue, preset?: string) => commit((current) => ({ ...current, period:
    preset === 'last30days' ? { mode: 'relative', days: 30 }
      : preset === 'last12months' ? { mode: 'relative', months: 12 }
        : value.start && value.end ? { mode: 'fixed', start: value.start, end: value.end } : { mode: 'all' },
  }), periodTrigger)
  const applyFacets = (_url: string, selections: ReadonlyMap<string, ReadonlySet<string>>) => {
    commit((current) => {
      const constraints = new Map((current.constraints ?? []).map((constraint) => [constraint.dimension, constraint.values]))
      for (const [dimension, values] of selections) {
        if (values.size) constraints.set(dimension, [...values])
        else constraints.delete(dimension)
      }
      return { ...current, constraints: [...constraints].map(([dimension, values]) => ({ dimension, values })) }
    }, facetTrigger)
  }
  const constraints = createMemo(() => draft.value().constraints ?? [])
  return <div class="lens-scoped-controls" data-testid={`context-controls-${props.scope}-${props.id}`} ref={(element) => { root = element }}>
    <PeriodFilterControl filter={filter} today={hostToday()} inheritLabel={inherited() ? inheritText() : undefined} onChange={onPeriod} />
    <Show when={!inherited()}><button type="button" class="lens-context-action" data-testid="context-inherit" onClick={() => commit((value) => ({ ...value, period: props.scope === 'dashboard' ? { mode: 'reports' } : undefined }), periodTrigger)}>{inheritText()}</button></Show>
    <Show when={props.facets?.length}><FacetFilterMenu filters={props.facets!} onApplyURL={applyFacets} /></Show>
    <For each={constraints()}>{(constraint) => <span class="lens-active-filter-chip"><span>{constraintLabel(props.editor, constraint)}</span></span>}</For>
    <Show when={draft.restorable()}>
      <button type="button" class="lens-context-action" data-testid="context-reset" onClick={() => { draft.reset(); if (draft.dirty()) draft.apply(); keepFocus(periodTrigger) }}>{translate('filter.context.reset', 'Restore saved filters')}</button>
    </Show>
    <Show when={saveDefaults}><button type="button" class="lens-context-action" aria-disabled={draft.saving() || undefined} onClick={() => { if (!draft.saving()) void draft.saveDefaults().catch(() => undefined) }}>{translate('filter.context.save', 'Save filter defaults')}</button></Show>
    <Show when={draft.error()}><p class="lens-context-error" role="alert">{translate('filter.context.saveFailed', 'Filter defaults could not be saved')}</p></Show>
  </div>
}

function constraintLabel(editor: ContextEditor, constraint: { dimension: string; values: string[] }) {
  const values = constraint.values.map((value) => editor.valueLabels?.[constraint.dimension]?.[value] || value).join(', ')
  return `${editor.dimensionLabels?.[constraint.dimension] || constraint.dimension}: ${values}`
}

const calendarToday = (calendar: ContextCalendar) =>
  parseISODate(new Intl.DateTimeFormat('en-CA', { timeZone: calendar.timezone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(calendar.asOf)))

/** A selection's dates on the report calendar, the same way the host resolves a relative period. */
function periodRange(period: ContextPeriod | undefined, calendar: ContextCalendar): PeriodValue {
  if (period?.mode === 'fixed') return { start: period.start ?? '', end: period.end ?? '' }
  if (period?.mode === 'relative') {
    const today = calendarToday(calendar)
    if (!today) return { start: '', end: '' }
    const start = period.months ? addDays(addMonths(today, -period.months), 1) : addDays(today, 1 - (period.days ?? 1))
    return { start: formatISODate(start), end: formatISODate(today) }
  }
  return { start: '', end: '' }
}

const inheritsPeriod = (period?: ContextPeriod) => !period || period.mode === 'reports'

/** A group context carries something the dashboard bar does not already say; dashboard restrictions are the bar's own chips. */
export function contextDiffersFromDashboard(context: EffectiveContext): boolean {
  return context.period.origin.scope !== 'dashboard'
    || context.restrictions.some((restriction) => restriction.origin.scope !== 'dashboard')
    || context.conflicts.length > 0
    || context.unsupported.length > 0
    || Object.values(context.unsupportedOperations ?? {}).some((restrictions) => restrictions.length > 0)
}

export interface LocalOverride { key: string; label: string; origin: string }

/** Every period and constraint a section or card applies on top of the dashboard. */
export function localOverrides(editor: ContextEditor, context: EffectiveContext, id: string, translate: ReturnType<typeof useTranslate>): LocalOverride[] {
  const section = sectionOf(context)
  const scopes: Array<{ scope: 'section' | 'card'; id: string; origin: string }> = [
    ...(section ? [{ scope: 'section' as const, id: section, origin: `${translate('filter.context.scope.section', 'Section')}: ${editor.originLabels?.[`section:${section}`] || section}` }] : []),
    { scope: 'card', id, origin: translate('filter.context.scope.card', 'Card') },
  ]
  return scopes.flatMap(({ scope, id: scopeID, origin }) => {
    const filters = (scope === 'section' ? editor.value.sections : editor.value.cards)?.[scopeID] ?? {}
    const prefix = scope === 'section' ? `${translate('filter.context.scope.section', 'Section')} · ` : ''
    const period = inheritsPeriod(filters.period) ? [] : [{ key: `${scope}:period`, label: prefix + periodLabel(filters.period!, context, { scope, id: scopeID }, translate), origin }]
    return [...period, ...(filters.constraints ?? []).filter((constraint) => constraint.values.length > 0).map((constraint) => ({
      key: `${scope}:${constraint.dimension}`, label: prefix + constraintLabel(editor, constraint), origin,
    }))]
  })
}

function periodLabel(period: ContextPeriod, context: EffectiveContext, origin: ContextOrigin, translate: ReturnType<typeof useTranslate>) {
  const resolved = context.period.origin.scope === origin.scope && context.period.origin.id === origin.id
    ? { start: context.period.start, end: context.period.end }
    : periodRange(period, context.calendar)
  const start = resolved.start ? parseISODate(resolved.start) : undefined
  const end = resolved.end ? parseISODate(resolved.end) : undefined
  if (start && end) return compactRangeLabel(start, end)
  return period.mode === 'all' ? translate('filter.period.allTime', 'All time') : translate('filter.period.custom', 'Custom range')
}

const sectionOf = (context: EffectiveContext) => context.input.layers.find((layer) => layer.origin.scope === 'section')?.origin.id

export function FilterContextInfo(props: { context: EffectiveContext; labels?: Record<string, string>; origins?: Record<string, string>; values?: Record<string, Record<string, string>>; class?: string }) {
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
  const originLabel = (origin: ContextOrigin) => {
    const name = props.origins?.[`${origin.scope}:${origin.id}`]
    return name ? `${label(origin.scope)}: ${name}` : label(origin.scope)
  }
  return <div class={`lens-context-info${props.class ? ` ${props.class}` : ''}`} data-testid="effective-context" data-context-id={props.context.identity}>
    <p>{props.context.period.start ? `${props.context.period.start} – ${props.context.period.end}` : translate('filter.period.allTime', 'All time')} · {originLabel(props.context.period.origin)} · {props.context.calendar.timezone}</p>
    <For each={props.context.restrictions}>{(restriction) => <p>{props.labels?.[restriction.dimension] || restriction.dimension}: {restriction.values.map((value) => props.values?.[restriction.dimension]?.[value] || value).join(', ')} · {originLabel(restriction.origin)}</p>}</For>
    <Show when={props.context.conflicts.length}><p role="status" data-testid="context-conflict">{translate('filter.context.conflict', 'No data for this combination of filters')}</p></Show>
    <Show when={props.context.unsupportedOperations?.drill?.length}><p role="status">{translate('filter.context.drillUnavailable', 'Detailed data is unavailable for these filters')}</p></Show>
    <Show when={props.context.unsupportedOperations?.export?.length}><p role="status">{translate('filter.context.exportUnavailable', 'Export is unavailable for these filters')}</p></Show>
    <Show when={props.context.unsupported.length}><p role="status" data-testid="context-unsupported">{translate('filter.context.unsupported', 'This report does not support these filters')}: {props.context.unsupported.map((value) => props.labels?.[value.dimension] || value.dimension).join(', ')}</p></Show>
  </div>
}

export interface ContextGroupDisclosure {
  open: () => boolean
  setOpen: (open: boolean) => void
  panelID: string
  trigger: { current?: HTMLButtonElement }
}

export function createContextGroupDisclosure(open: () => boolean, setOpen: (open: boolean) => void): ContextGroupDisclosure {
  return { open, setOpen, panelID: createUniqueId(), trigger: {} }
}

/** Whether the group header has anything to carry for this context. */
export function hasContextGroupToolbar(editor: ContextEditor | undefined, context: EffectiveContext, id: string): boolean {
  if (!editor) return false
  return !editor.readOnly
    || context.input.layers.some((layer) => layer.origin.scope === 'group')
    || localOverrides(editor, context, id, (_key, fallback) => fallback).length > 0
}

/**
 * The group header's share of the scoped filters: what this group narrows on
 * top of the dashboard, as chips, and one trigger that discloses the editors.
 * Two always-open filter rows per section used to stand between every heading
 * and its figures.
 */
export function ContextGroupToolbar(props: { id: string; context: EffectiveContext; disclosure: ContextGroupDisclosure }) {
  const { document } = useDashboard()
  const { applyURL } = useFilters()
  const translate = useTranslate()
  const editor = () => document.contextEditor
  const overrides = createMemo(() => editor() ? localOverrides(editor()!, props.context, props.id, translate) : [])
  const hasGroupLayer = () => props.context.input.layers.some((layer) => layer.origin.scope === 'group')
  return <Show when={editor()}>{(value) => <div class="lens-context-toolbar">
    <Show when={hasGroupLayer()}>
      {(() => {
        const backURL = () => {
          const board = structuredClone(unwrap(value().value))
          const id = props.context.input.layers.find((layer) => layer.origin.scope === 'card')?.origin.id ?? props.id
          const groups = board.groups?.[id] ?? []
          board.groups = { ...board.groups, [id]: groups.slice(0, -1) }
          return filterBoardURL(value().url, board)
        }
        return <a class="lens-context-action" data-testid="context-back" href={backURL()} onClick={(event) => {
          if (value().readOnly || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
          event.preventDefault()
          applyURL(backURL())
        }}>{translate('filter.context.back', 'Back to previous selection')}</a>
      })()}
    </Show>
    <For each={overrides()}>{(override) => <span class="lens-active-filter-chip" title={override.origin}><span>{override.label}</span></span>}</For>
    <Show when={!value().readOnly}>
      <button
        aria-controls={props.disclosure.panelID}
        aria-expanded={props.disclosure.open()}
        class="lens-facet-trigger lens-context-trigger"
        data-testid={`context-local-${props.id}`}
        onClick={() => props.disclosure.setOpen(!props.disclosure.open())}
        ref={(element) => { props.disclosure.trigger.current = element }}
        type="button"
      >
        <FunnelSimple aria-hidden="true" />
        <span>{translate('filter.context.local', 'Filters')}</span>
        <Show when={overrides().length > 0}><span class="lens-facet-count">{overrides().length}</span></Show>
        <CaretDown aria-hidden="true" />
      </button>
    </Show>
  </div>}</Show>
}

/**
 * The group body's share: the effective context, only when it says more than
 * the dashboard bar does, and the disclosed section and card editors.
 */
export function ContextGroupDetails(props: { id: string; context: EffectiveContext; disclosure: ContextGroupDisclosure }) {
  const { document } = useDashboard()
  const translate = useTranslate()
  const editor = () => document.contextEditor
  const facets = (scope: Scope, id: string) => (document.filters ?? []).filter((filter) => filter.kind === 'facet' && filter.scope?.scope === scope && filter.scope.id === id)
  const section = () => sectionOf(props.context)
  const close = () => {
    props.disclosure.setOpen(false)
    props.disclosure.trigger.current?.focus()
  }
  let panel: HTMLDivElement | undefined
  createEffect(() => {
    if (!props.disclosure.open()) return
    const frame = requestAnimationFrame(() => {
      if (panel && !panel.contains(panel.ownerDocument.activeElement)) panel.querySelector<HTMLElement>(focusableControl)?.focus()
    })
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !(event.target instanceof Node) || !panel?.contains(event.target)) return
      event.stopPropagation()
      close()
    }
    globalThis.document.addEventListener('keydown', onKeyDown)
    onCleanup(() => {
      cancelAnimationFrame(frame)
      globalThis.document.removeEventListener('keydown', onKeyDown)
    })
  })
  return <>
    <Show when={contextDiffersFromDashboard(props.context)}>
      <FilterContextInfo class="lens-context-info-live" context={props.context} labels={editor()?.dimensionLabels} origins={editor()?.originLabels} values={editor()?.valueLabels} />
    </Show>
    <Show when={props.disclosure.open() && editor() && !editor()!.readOnly ? editor() : undefined}>{(value) => (
      <div
        aria-label={translate('filter.context.local', 'Filters')}
        class="lens-context-panel"
        data-testid={`context-panel-${props.id}`}
        id={props.disclosure.panelID}
        ref={(element) => { panel = element }}
        role="region"
      >
        <Show when={section()}>{(id) => (
          <div class="lens-context-editor">
            <p class="lens-context-editor-title">{translate('filter.context.section', 'Whole section')}: {value().originLabels?.[`section:${id()}`] || id()}</p>
            <ScopedFilterControls editor={value()} scope="section" id={id()} effective={props.context} facets={facets('section', id())} />
          </div>
        )}</Show>
        <div class="lens-context-editor">
          <p class="lens-context-editor-title">{translate('filter.context.card', 'This card')}</p>
          <ScopedFilterControls editor={value()} scope="card" id={props.id} effective={props.context} facets={facets('card', props.id)} />
        </div>
      </div>
    )}</Show>
  </>
}
