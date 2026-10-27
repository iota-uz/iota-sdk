
import { createEffect, createSignal, createUniqueId, For, onCleanup, Show, type JSX } from 'solid-js'
import { useDashboard, useFilters, useTranslate } from '../runtime'
import { DotsThree, X } from '../icons'
import { CompareFilterControl } from './CompareFilterControl'
import { FacetFilterMenu } from './FacetFilterMenu'
import { PeriodFilterControl } from './PeriodFilterControl'
import { SegmentedFilterControl } from './SegmentedFilterControl'
import { ScopedFilterControls } from './ScopedFilterControls'
import type { CalendarDate } from './model'
import { clearRendererStateFromURL } from '../runtime/url'
import type { Filter } from '../contract'
import { ToolbarOverflowContext } from './toolbarOverflow'

export interface FilterBarProps {
  /** Fixed "today" for deterministic stories and visual regression. */
  today?: CalendarDate
  /**
   * The producer's own scope line, printed after the controls. It states what
   * the controls cannot — a data cut-off, a scope caveat — so it reads as the
   * tail of the same sentence rather than as a second heading.
   */
  subtitle?: string
  /**
   * Keep the period and Filters on the scope row and fold the secondary verbs
   * (report periods, restoring saved filters, filter defaults, comparison)
   * into one "More" menu.
   */
  compact?: boolean
}

const focusable = 'button:not(:disabled), a[href], input:not(:disabled), [tabindex]:not([tabindex="-1"])'

/**
 * The compact bar's "More" disclosure. Its panel is always mounted, hidden
 * while closed, so the controls folded into it keep their state; the trigger
 * is shown only while the panel holds something to operate.
 */
function ToolbarOverflow(props: { panel: (element: HTMLElement) => void; children?: JSX.Element }) {
  const translate = useTranslate()
  const id = createUniqueId()
  const [open, setOpen] = createSignal(false)
  const [filled, setFilled] = createSignal(false)
  let container: HTMLDivElement | undefined
  let trigger: HTMLButtonElement | undefined
  let panel: HTMLDivElement | undefined
  const close = (focusTrigger: boolean) => {
    setOpen(false)
    if (focusTrigger) trigger?.focus()
  }
  createEffect(() => {
    const element = panel
    if (!element || typeof MutationObserver === 'undefined') return
    const measure = () => setFilled(Boolean(element.querySelector(focusable)))
    measure()
    const observer = new MutationObserver(measure)
    observer.observe(element, { childList: true, subtree: true })
    onCleanup(() => observer.disconnect())
  })
  createEffect(() => {
    if (!open()) return
    requestAnimationFrame(() => panel?.querySelector<HTMLElement>(focusable)?.focus())
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target instanceof Element ? event.target : undefined
      if (!target || container?.contains(target) || target.closest('.lens-overlay-root')) return
      close(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    onCleanup(() => document.removeEventListener('pointerdown', onPointerDown))
  })
  return (
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions -- Escape closes the disclosure from any control inside it.
    <div
      class="lens-toolbar-overflow"
      data-filled={filled() || undefined}
      onKeyDown={(event) => {
        if (event.key !== 'Escape' || !open()) return
        if (event.target instanceof Element && event.target.closest('.lens-overlay-root')) return
        event.preventDefault()
        event.stopPropagation()
        close(true)
      }}
      ref={(element) => { container = element }}
    >
      <button
        aria-controls={id}
        aria-expanded={open()}
        aria-label={translate('filter.more', 'More filter options')}
        class="lens-facet-trigger lens-toolbar-overflow-trigger"
        data-testid="toolbar-overflow"
        hidden={!filled()}
        onClick={() => setOpen(!open())}
        ref={(element) => { trigger = element }}
        title={translate('filter.more', 'More filter options')}
        type="button"
      >
        <DotsThree />
      </button>
      <div
        aria-label={translate('filter.more', 'More filter options')}
        class="lens-toolbar-overflow-panel"
        data-testid="toolbar-overflow-panel"
        hidden={!open()}
        id={id}
        ref={(element) => { panel = element; props.panel(element) }}
        role="group"
      >
        {props.children}
      </div>
    </div>
  )
}

interface ActiveChip {
  key: string
  label: string
  removeUrl: string
}

export function FilterControls(props: { filters: Filter[]; today?: CalendarDate }) {
  const facets = () => props.filters.filter((filter) => filter.kind === 'facet' && filter.facet)
  return (
    <>
      <For each={props.filters}>
        {(filter) => (
          filter.kind === 'period' && filter.period
            ? <PeriodFilterControl filter={filter} today={props.today} />
            : filter.kind === 'compare' && filter.compare
              ? <CompareFilterControl filter={filter} />
              : filter.kind === 'segmented' && filter.segmented
                ? <SegmentedFilterControl filter={filter} />
                : null
        )}
      </For>
      {facets().length > 0 && <FacetFilterMenu filters={facets()} />}
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
export function FilterBar(props: FilterBarProps) {
  const { filters, applyURL } = useFilters()
  const { document } = useDashboard()
  const translate = useTranslate()
  if (document.contextEditor?.readOnly) return null
  const globalFilters = filters.filter((filter) => !filter.placement && !filter.scope)
  if (!document.contextEditor && globalFilters.length === 0 && (document.activeFilters?.length ?? 0) === 0 && !props.subtitle) return null
  const intercept = (url: string) => (event: MouseEvent) => {
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
  const [overflow, setOverflow] = createSignal<HTMLElement>()
  const secondary = (filter: Filter) => Boolean(props.compact) && filter.kind === 'compare'
  const inlineFilters = (list: Filter[]) => list.filter((filter) => !secondary(filter))
  const overflowFilters = (list: Filter[]) => list.filter(secondary)
  const contextFilters = globalFilters.filter((filter) => filter.kind !== 'period' && filter.kind !== 'facet')

  return (
    <div aria-label={translate('filter.bar.label', 'Dashboard filters')} class="lens-filter-bar" role="group">
      <div class="lens-dashboard-scope">
        {/* Declaration order is render order, and the row only wraps — it never
            reorders — so a control keeps the same neighbours at every width. */}
        <ToolbarOverflowContext.Provider value={props.compact ? overflow : undefined}>
          <Show when={document.contextEditor} fallback={<FilterControls filters={inlineFilters(globalFilters)} today={props.today} />}>{(editor) => <><ScopedFilterControls editor={editor()} scope="dashboard" id="dashboard" facets={facets} /><FilterControls filters={inlineFilters(contextFilters)} today={props.today} /></>}</Show>
        </ToolbarOverflowContext.Provider>
        <Show when={props.compact}>
          <ToolbarOverflow panel={setOverflow}>
            <FilterControls filters={overflowFilters(document.contextEditor ? contextFilters : globalFilters)} today={props.today} />
          </ToolbarOverflow>
        </Show>
        {/* Not a heading and not a control: the part of the scope no control
            owns. It sits last so the editable segments stay together at the
            left edge, where the eye lands. */}
        {props.subtitle && <p class="lens-dashboard-subtitle">{props.subtitle}</p>}
      </div>
      <Show when={chips.length > 0 || clearAllURL}>
        <div class="lens-filter-bar-chips">
          <For each={chips}>
            {(chip) => (
              <a
                aria-label={`${translate('filter.facet.remove', 'Remove filter')}: ${chip.label}`}
                class="lens-facet-active-chip"
                href={chip.removeUrl}
                onClick={intercept(chip.removeUrl)}
              >
                <span>{chip.label}</span><X aria-hidden="true" />
              </a>
            )}
          </For>
          {/* A button, not a link: it does not navigate anywhere a reader could
              bookmark, it dismisses the chips beside it. It also stays at the
              end of the chip row instead of flowing among the triggers. */}
          {clearAllURL && (
            <button class="lens-facet-clear" onClick={intercept(clearAllURL)} type="button">
              {translate('filter.facet.clearAll', 'Clear all')}
            </button>
          )}
        </div>
      </Show>
    </div>
  )
}
