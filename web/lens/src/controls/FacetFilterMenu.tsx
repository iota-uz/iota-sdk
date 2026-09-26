/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, createUniqueId, onCleanup, For, Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { Filter } from '../contract'
import { CaretDown, FunnelSimple } from '../icons'
import { searchableListEntries } from '../listSearch'
import { cubeFilterParam, useFilters, useTranslate } from '../runtime'
import { useMenuButton } from '../panels/useMenuButton'
import { relativeURL, useFacetOptions } from './facetOptions'

const minimumFacetBarPercent = 3

/**
 * The URL one Apply produces for every dimension the reader staged.
 *
 * `base` is a producer-owned apply URL: the current slice with one dimension
 * removed and every other query value preserved. Rebuilding from it means the
 * menu never has to know which of the page's parameters are filters — it drops
 * the values of the dimensions it is restating and appends the staged sets.
 */
/* eslint-disable react-refresh/only-export-components */
export function stagedFilterURL(base: string, drafts: ReadonlyMap<string, ReadonlySet<string>>): string {
  const href = typeof window === 'undefined' ? 'http://localhost/' : window.location.href
  const source = new URL(base || href, href)
  const target = new URL(source.href)
  target.searchParams.delete(cubeFilterParam)
  for (const value of source.searchParams.getAll(cubeFilterParam)) {
    const separator = value.indexOf(':')
    const dimension = separator < 0 ? value : value.slice(0, separator)
    if (drafts.has(dimension)) continue
    target.searchParams.append(cubeFilterParam, value)
  }
  for (const [dimension, values] of drafts) {
    for (const value of values) target.searchParams.append(cubeFilterParam, `${dimension}:${value}`)
  }
  return relativeURL(target)
}

/**
 * One dimension's option list.
 *
 * It names the dimension it edits — the popover used to be an anonymous list of
 * checkboxes hanging somewhere near the bar, so a reader who lost track of which
 * trigger they pressed had nothing on screen to tell them — and it states the
 * dimension in its search placeholder for the same reason.
 */
function FacetPane(props: {
  filter: Filter
  draft: ReadonlySet<string> | undefined
  onToggle: (dimension: string, value: string, from: ReadonlySet<string>) => void
  onTarget: (dimension: string, applyUrl: string) => void
}) {
  const translate = useTranslate()
  const facet = props.filter.facet
  const { applyTarget, options, search, setSearch, status } = useFacetOptions(facet)
  const dimension = facet?.dimension ?? ''
  let paneRef: HTMLDivElement | undefined

  createEffect(() => {
    const target = applyTarget()
    if (target) props.onTarget(dimension, target)
  })

  createEffect(() => {
    if (status() === 'loading') return
    const frame = globalThis.requestAnimationFrame(() => {
      paneRef?.querySelector<HTMLElement>('input, button')?.focus()
    })
    onCleanup(() => globalThis.cancelAnimationFrame(frame))
  })

  // The applied set is what the pane shows until the reader touches it; from
  // then on the draft is the truth, including the empty draft that Clear makes.
  const selected = () => props.draft ?? new Set(options().filter((option) => option.selected).map((option) => option.value))
  const maxCount = () => Math.max(0, ...options().map(({ count }) => count ?? 0))
  // A list too short to scroll is too short to search: the box could only ever
  // filter the one visible row away. Same threshold as the chart legend.
  const searchable = () => options().length >= searchableListEntries || search().trim() !== ''

  return (
    <div class="lens-filter-menu-pane" ref={(el) => { paneRef = el }}>
      <p class="lens-filter-menu-pane-title">{props.filter.label}</p>
      <Show when={searchable()}>
        <input
          aria-label={translate('filter.facet.search', 'Search options')}
          class="lens-facet-search"
          onChange={(event) => setSearch(event.target.value)}
          placeholder={translate('filter.facet.searchIn', 'Search {facet}', { facet: props.filter.label ?? '' })}
          type="search"
          value={search()}
        />
      </Show>
      <div class="lens-facet-options">
        <Show when={status() !== 'loading' && status() !== 'error' && options().length > 0} fallback={
          <div class={`lens-facet-state${status() === 'error' ? ' lens-facet-error' : ''}`}>
            {status() === 'loading'
              ? translate('filter.facet.loading', 'Loading options…')
              : status() === 'error'
                ? translate('filter.facet.error', 'Options could not be loaded')
                : translate('filter.facet.empty', 'No options')}
          </div>
        }>
          <For each={options()}>
            {(option) => {
              const checked = () => selected().has(option.value)
              const magnitude = maxCount() > 0 && (option.count ?? 0) > 0
                ? Math.max(minimumFacetBarPercent, Math.floor((option.count ?? 0) * 100 / maxCount()))
                : 0
              return (
                <label class="lens-facet-option">
                  {magnitude > 0 && <span aria-hidden="true" class="lens-facet-option-bar" style={{ width: `${magnitude}%` }} />}
                  <input
                    checked={checked()}
                    onChange={() => props.onToggle(dimension, option.value, selected())}
                    type="checkbox"
                  />
                  <span class="lens-facet-option-label" title={option.label}>{option.label}</span>
                  {(option.count ?? 0) > 0 && <span class="lens-facet-option-count">{option.count}</span>}
                </label>
              )
            }}
          </For>
        </Show>
      </div>
    </div>
  )
}

/**
 * Every facet the dashboard declares, behind one trigger.
 *
 * Nine sibling dropdowns in the header was a Hick's-law problem with a layout
 * problem on top: the row wrapped raggedly into three, and adding a chip
 * reflowed all of them. One trigger states how many filters are on, one popover
 * holds the dimensions in a rail beside their options, and one Apply commits
 * every dimension the reader staged. A dashboard with a single facet names it on
 * the trigger and drops the rail — a menu of one is a question with no answer.
 *
 * Placement is `menuPlacement()`'s, the same measured rule the panel and export
 * menus use. Like an ordinary dropdown it starts at the trigger's leading edge;
 * it flips or re-aligns only when the viewport says it must.
 */
export function FacetFilterMenu(props: { filters: Array<Filter> }) {
  const translate = useTranslate()
  const { applyURL } = useFilters()
  const menuID = createUniqueId()
  const { close, container, menu, menuPlacementProps, open, overlay, setOpen, trigger } = useMenuButton('start')
  const [activeID, setActiveID] = createSignal(props.filters[0]?.id ?? '')
  const [drafts, setDrafts] = createSignal(new Map<string, ReadonlySet<string>>())
  const [targets, setTargets] = createSignal(new Map<string, string>())
  const [touched, setTouched] = createSignal(new Set<string>())

  const rememberTarget = (dimension: string, applyUrl: string) => {
    setTargets((current) => (current.get(dimension) === applyUrl ? current : new Map(current).set(dimension, applyUrl)))
  }

  const active = () => props.filters.find((filter) => filter.id === activeID()) ?? props.filters[0]
  const appliedCount = () => props.filters.reduce((sum, filter) => sum + (filter.facet?.selections?.length ?? 0), 0)

  // Closing drops the staging, so a popover reopened after a mis-click can never
  // present a selection the dashboard is not actually showing.
  createEffect(() => {
    if (open()) return
    setDrafts(new Map())
    setTouched(new Set<string>())
  })

  const setDraft = (dimension: string, values: ReadonlySet<string>) => {
    setDrafts((current) => new Map(current).set(dimension, values))
    setTouched((current) => { const next = new Set(current); next.add(dimension); return next })
  }
  // The pane hands over the set it is displaying — the staged one if there is
  // one, the applied one otherwise — so the first toggle stages the applied
  // selection plus the click instead of a set of one.
  const toggle = (dimension: string, value: string, from: ReadonlySet<string>) => {
    const next = new Set(from)
    if (next.has(value)) next.delete(value)
    else next.add(value)
    setDraft(dimension, next)
  }

  // Only a dimension the reader actually touched is restated; the rest keep the
  // slice they already have.
  const staged = () => new Map([...drafts()].filter(([dimension]) => touched().has(dimension)))
  const stagedCount = () => [...staged().values()].reduce((sum, values) => sum + values.size, 0)
  const base = () => [...staged().keys()].map((dimension) => targets().get(dimension)).find(Boolean) ?? ''
  const apply = () => {
    if (staged().size === 0 || !base()) return
    close()
    applyURL(stagedFilterURL(base(), staged()))
  }

  const single = props.filters.length === 1
  const label = single ? (props.filters[0]?.label ?? '') : translate('filter.facet.filters', 'Filters')
  // Clear removes a selection, so it is only there when there is one to remove.
  // A facet holding a single option used to render it (and a search box) beside
  // an untouched list: two controls that could not do anything.
  const activeDimension = () => active()?.facet?.dimension ?? ''
  const clearable = () => touched().has(activeDimension())
    ? (drafts().get(activeDimension())?.size ?? 0) > 0
    : (active()?.facet?.selections?.length ?? 0) > 0

  return (
    <Show when={props.filters.length > 0}>
      <div class="lens-filter-menu" ref={(el) => { container.current = el }}>
        <button
          aria-controls={menuID}
          aria-expanded={open()}
          aria-haspopup="dialog"
          class="lens-facet-trigger"
          onClick={() => setOpen((current) => !current)}
          ref={(el) => { trigger.current = el }}
          type="button"
        >
          <FunnelSimple aria-hidden="true" />
          <span>{label}</span>
          {appliedCount() > 0 && <span class="lens-facet-count">{appliedCount()}</span>}
          <CaretDown aria-hidden="true" />
        </button>
        <Show when={open() && overlay()}>
          <Portal mount={overlay()}>
            <div
              aria-label={label}
              class="lens-filter-menu-popover"
              id={menuID}
              ref={(el) => { menu.current = el }}
              role="dialog"
              data-align={menuPlacementProps().align}
              data-side={menuPlacementProps().side}
              style={menuPlacementProps().style}
            >
              <div class="lens-filter-menu-body">
                {!single && (
                  <div aria-label={label} class="lens-filter-menu-rail" role="tablist">
                    <For each={props.filters}>
                      {(filter) => {
                        const dimension = filter.facet?.dimension ?? ''
                        const count = drafts().has(dimension) && touched().has(dimension)
                          ? (drafts().get(dimension)?.size ?? 0)
                          : (filter.facet?.selections?.length ?? 0)
                        return (
                          <button
                            aria-selected={filter.id === active()?.id}
                            class="lens-filter-menu-rail-item"
                            onClick={() => setActiveID(filter.id)}
                            role="tab"
                            type="button"
                          >
                            <span class="lens-filter-menu-rail-label">{filter.label}</span>
                            {count > 0 && <span class="lens-facet-count">{count}</span>}
                          </button>
                        )
                      }}
                    </For>
                  </div>
                )}
                <div class="lens-filter-menu-pane-host">
                  {active() && (
                    <FacetPane
                      draft={touched().has(active()!.facet?.dimension ?? '') ? drafts().get(active()!.facet?.dimension ?? '') : undefined}
                      filter={active()!}
                      onTarget={rememberTarget}
                      onToggle={toggle}
                    />
                  )}
                </div>
              </div>
              {/* The footer states the size of what Apply is about to do. It used to
                say nothing until after the request came back, so the only way to
                check a staged multi-select was to apply it. */}
              <div class="lens-facet-actions">
                <span class="lens-facet-staged" data-staged={touched().size > 0 || undefined}>
                  {translate('filter.facet.staged', 'Selected: {count}', { count: touched().size > 0 ? stagedCount() : appliedCount() })}
                </span>
                {clearable() && (
                  <button onClick={() => setDraft(activeDimension(), new Set())} type="button">
                    {translate('filter.facet.clear', 'Clear')}
                  </button>
                )}
                <button class="lens-facet-apply" disabled={staged().size === 0 || !base()} onClick={apply} type="button">
                  {translate('filter.facet.apply', 'Apply')}
                </button>
              </div>
            </div>
          </Portal>
        </Show>
      </div>
    </Show>
  )
}
