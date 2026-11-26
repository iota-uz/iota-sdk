// The list of a graph's runs (RV2, RV9): search by subject, state chips, a row per run with its state
// (dot and word), subject, node, version, time and the phrase of a failure; ↑ ↓ move between rows.
// Rows are keyed by run id, not by object: a re-read list updates the rows in place (P4) and only a run
// the list did not have before enters with a short rise (#1352).
import { createEffect, createMemo, createSignal, For, Match, on, onCleanup, Show, Switch, type JSX } from 'solid-js'
import { findItem } from '../graph'
import { Icon } from '../icons'
import { describeItem } from '../names'
import { strings } from '../strings'
import type { AutomationGraph, EditorCatalog, EditorLocale, RunState, RunSummary } from '../types'
import { Swap } from './Data'
import { ALL_STATES } from './model'
import { runStrings } from './strings'
import { absoluteTime, relativeTime, subjectName } from './util'

export const STATE_ICON: Record<RunState, string> = {
  running: 'running', waiting: 'waiting', blocked: 'error', failed: 'failed', done: 'passed', cancelled: 'cancelled',
}

export interface RunsListProps {
  items: RunSummary[] | undefined
  /** The list could not be read; with items it is a failed refresh (S4). */
  error: boolean
  refreshedAt: string | null
  selected: string | null
  states: RunState[]
  subject: string
  graph: AutomationGraph
  catalog: EditorCatalog
  locale: EditorLocale
  now: number
  more: boolean
  loadingMore: boolean
  onOpen(run: string): void
  onStates(states: RunState[]): void
  onSubject(subject: string): void
  onReset(): void
  onRetry(): void
  onMore(): void
  onStart?(): void
}

const SEARCH_DEBOUNCE_MS = 300

export function RunsList(props: RunsListProps): JSX.Element {
  const t = () => runStrings(props.locale)
  const filtered = () => props.states.length > 0 || props.subject.trim() !== ''
  const [query, setQuery] = createSignal(props.subject)
  let timer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(timer))
  // Back and Forward change the search: the box follows the URL.
  createEffect(on(() => props.subject, (subject) => {
    if (subject !== query().trim()) {
      clearTimeout(timer)
      setQuery(subject)
    }
  }, { defer: true }))
  const typed = (value: string) => {
    setQuery(value)
    clearTimeout(timer)
    timer = setTimeout(() => props.onSubject(value.trim()), SEARCH_DEBOUNCE_MS)
  }
  const toggle = (state: RunState) => {
    const next = props.states.includes(state) ? props.states.filter((s) => s !== state) : [...props.states, state]
    props.onStates(ALL_STATES.filter((s) => next.includes(s)))
  }
  const reset = () => {
    clearTimeout(timer)
    setQuery('')
    props.onReset()
  }
  let list: HTMLUListElement | undefined
  const onKey = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp' && event.key !== 'Home' && event.key !== 'End') return
    const items = props.items ?? []
    if (items.length === 0) return
    event.preventDefault()
    const at = items.findIndex((r) => r.run_id === props.selected)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1
      : Math.max(0, Math.min(items.length - 1, at + (event.key === 'ArrowDown' ? 1 : -1)))
    const run = items[next]!
    props.onOpen(run.run_id)
    queueMicrotask(() => list?.querySelector<HTMLElement>(`[data-run="${CSS.escape(run.run_id)}"]`)?.focus())
  }
  // The rows by run id: a list read again (Retry, a run's poll) keeps every row's element, and a new run is
  // the only row mounted. `seen` is the list as last drawn; null while there is none (first load, a new
  // filter), so the first rows of a list never animate.
  const ids = createMemo(() => (props.items ?? []).map((r) => r.run_id), [], {
    equals: (a, b) => a.length === b.length && a.every((id, i) => id === b[i]),
  })
  const byId = createMemo(() => new Map((props.items ?? []).map((r) => [r.run_id, r])))
  let seen: Set<string> | null = null
  createEffect(on(ids, (list) => { seen = props.items ? new Set(list) : null }))
  const nodeName = (id: string) => (findItem(props.graph, id) ? describeItem(props.graph, props.catalog, id, props.locale).title : null)

  return (
    <section class="ae-runlist" aria-label={t().list.label} data-testid="runs">
      <div class="ae-runlist__filters">
        <div class="ae-search">
          <Icon name="search" size={14} />
          <input
            type="search"
            class="form-control form-control-input ae-input"
            aria-label={t().list.search}
            placeholder={t().list.search}
            value={query()}
            onInput={(e) => typed(e.currentTarget.value)}
            data-testid="runs-search"
          />
        </div>
        <div class="ae-chips" role="group" aria-label={t().list.filter}>
          <For each={ALL_STATES}>
            {(state) => (
              <button type="button" class="ae-chip iota-press" aria-pressed={props.states.includes(state)} onClick={() => toggle(state)} data-state={state}>
                <span class={`ae-dot ae-dot--${state}`} aria-hidden="true" />{t().list.states[state]}
              </button>
            )}
          </For>
        </div>
      </div>
      <Show when={props.error && props.items}>
        <p class="ae-runlist__stale" role="alert">
          <Icon name="error" size={14} />
          <span>{t().list.stale(props.refreshedAt ? absoluteTime(props.refreshedAt, props.locale) : '')}</span>
          <button type="button" class="ae-link" onClick={() => props.onRetry()}>{retryText(props.locale)}</button>
        </p>
      </Show>
      <Switch>
        <Match when={props.error && !props.items}>
          <div class="ae-runlist__state iota-fade-in" role="alert" data-testid="runs-error">
            <p>{t().list.loadFailed}</p>
            <button type="button" class="ae-btn ae-btn--secondary" onClick={() => props.onRetry()}>{retryText(props.locale)}</button>
          </div>
        </Match>
        <Match when={!props.items}>
          <ul class="ae-runlist__rows" role="status" aria-busy="true" aria-label={t().list.loading}>
            <For each={[0, 1, 2, 3, 4]}>{() => <li class="ae-runrow ae-runrow--skeleton"><span class="ae-skeleton__bar" /><span class="ae-skeleton__bar" /></li>}</For>
          </ul>
        </Match>
        <Match when={props.items!.length === 0 && filtered()}>
          <div class="ae-runlist__state iota-fade-in" data-testid="runs-empty-filter">
            <p class="ae-runlist__title">{t().list.emptyFilter}</p>
            <button type="button" class="ae-btn ae-btn--secondary" onClick={reset}>{t().list.reset}</button>
          </div>
        </Match>
        <Match when={props.items!.length === 0}>
          <div class="ae-runlist__state iota-fade-in" data-testid="runs-empty">
            <p class="ae-runlist__title">{t().list.empty}</p>
            <p class="ae-muted">{t().list.emptyText}</p>
            <Show when={props.onStart}>
              <button type="button" class="btn btn-primary btn-normal" onClick={() => props.onStart?.()}><Icon name="manual" />{t().list.start}</button>
            </Show>
          </div>
        </Match>
        <Match when={props.items}>
          <ul ref={list} class="ae-runlist__rows iota-fade-in" role="listbox" aria-label={t().list.label} onKeyDown={onKey}>
            <For each={ids()}>
              {(id) => {
                // The row's data follows the list; it keeps its last value while the row is removed.
                const run = createMemo<RunSummary>((prev) => byId().get(id) ?? prev, byId().get(id)!)
                const fresh = seen !== null && !seen.has(id)
                const selected = () => id === props.selected
                const subject = () => subjectName(run().subject) ?? t().list.noSubject
                const node = () => {
                  const nid = run().current_nodes?.[0]
                  return nid ? nodeName(nid) : null
                }
                return (
                  <li
                    role="option"
                    class="ae-runrow"
                    classList={{ 'iota-enter': fresh }}
                    aria-selected={selected()}
                    tabindex={selected() || (!props.selected && id === ids()[0]) ? 0 : -1}
                    data-run={id}
                    data-testid="run-row"
                    onClick={() => props.onOpen(id)}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); props.onOpen(id) } }}
                  >
                    <div class="ae-runrow__top">
                      <Swap key={run().state}>
                        {(state) => (
                          <span class={`ae-runstate ae-runstate--${state}`}>
                            <span class={`ae-dot ae-dot--${state}`} aria-hidden="true" />{t().list.states[state as RunState]}
                          </span>
                        )}
                      </Swap>
                      <span class="ae-runrow__subject" title={subject()}>{subject()}</span>
                      <time class="ae-runrow__time ae-tabular" datetime={run().created_at} title={absoluteTime(run().created_at, props.locale)}>
                        {relativeTime(run().created_at, props.now, props.locale)}
                      </time>
                    </div>
                    <div class="ae-runrow__bottom">
                      <span class="ae-tag">v{run().version}</span>
                      <Show when={node()}>{(n) => <span class="ae-runrow__node" title={n()}>{n()}</span>}</Show>
                      <Show when={run().state === 'failed' && run().error}>
                        {(e) => <span class="ae-runrow__error" title={e()}>{e()}</span>}
                      </Show>
                    </div>
                  </li>
                )
              }}
            </For>
          </ul>
          <Show when={props.more}>
            <button type="button" class="ae-btn ae-btn--ghost ae-runlist__more" aria-busy={props.loadingMore} onClick={() => props.onMore()}>
              <Show when={props.loadingMore}><span class="ae-spinner" aria-hidden="true" /></Show>{t().list.more}
            </button>
          </Show>
        </Match>
      </Switch>
    </section>
  )
}

export const retryText = (locale: EditorLocale) => strings(locale).common.retry
