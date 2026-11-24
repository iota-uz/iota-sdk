// The node's drawer in a run (RV5): its waits, its steps (attempts and for each items), and of the chosen
// step the outcome, error, input, output and logs. Logs arrive masked from the server; they are filtered
// here and shown as text.
import { createEffect, createMemo, createSignal, For, on, Show, type JSX } from 'solid-js'
import { catalogNode, findItem } from '../graph'
import { Icon } from '../icons'
import { describeItem } from '../names'
import { strings } from '../strings'
import type { AutomationGraph, EditorCatalog, EditorLocale, ItemState, RunData, RunStep } from '../types'
import { CopyText, Value } from './Data'
import { hasData, stepsOf } from './model'
import { runStrings } from './strings'
import { absoluteTime, duration, jsonText, shortTime } from './util'

export interface NodeDrawerProps {
  graph: AutomationGraph
  catalog: EditorCatalog
  run: RunData
  node: string
  state?: ItemState
  locale: EditorLocale
  onClose(): void
  /** A bottom sheet on phones. */
  sheet?: boolean
}

export function NodeDrawer(props: NodeDrawerProps): JSX.Element {
  const t = () => runStrings(props.locale)
  const base = () => strings(props.locale)
  const name = createMemo(() => describeItem(props.graph, props.catalog, props.node, props.locale))
  const steps = createMemo(() => stepsOf(props.run, props.node))
  const waits = createMemo(() => props.run.waits.filter((w) => w.node === props.node))
  const [picked, setPicked] = createSignal<number | null>(null)
  createEffect(on(() => props.node, () => setPicked(null), { defer: true }))
  // The chosen step; by default the last one (the latest attempt of the latest item).
  const step = createMemo<RunStep | undefined>(() => {
    const list = steps()
    const i = picked()
    return i !== null && i < list.length ? list[i] : list[list.length - 1]
  })
  const multiple = () => steps().length > 1
  const items = createMemo(() => {
    const tokens = [...new Set(steps().map((s) => s.token_id))]
    return new Map(tokens.map((token, i) => [token, i + 1]))
  })
  const stepLabel = (s: RunStep) => {
    const parts: string[] = []
    if (items().size > 1) parts.push(t().node.item(items().get(s.token_id) ?? 1))
    if (s.attempt > 1 || steps().some((o) => o.token_id === s.token_id && o.attempt > 1)) parts.push(t().node.attempt(s.attempt))
    return parts.join(' · ') || shortTime(s.started_at, props.locale)
  }
  const icon = () => name().icon ?? (findItem(props.graph, props.node)?.kind === 'trigger' ? 'event' : catalogNode(props.catalog, (findItem(props.graph, props.node)?.item as { type?: string } | undefined)?.type ?? '')?.behavior ?? 'node')
  const titleId = `ae-run-node-${props.node}`

  return (
    <section class="ae-panel ae-run-node" classList={{ 'ae-run-node--sheet': props.sheet }} aria-labelledby={titleId} data-testid="run-node">
      <header class="ae-panel__head">
        <span class="ae-panel__icon"><Icon name={icon()} /></span>
        <div class="ae-panel__titles">
          <h2 class="ae-panel__title" id={titleId} title={name().title}>{name().title}</h2>
          <Show when={name().subtitle}><p class="ae-panel__sub">{name().subtitle}</p></Show>
        </div>
        <Show when={props.state}>
          {(s) => <span class={`ae-state ae-state--${s()}`}><Icon name={s()} size={12} /><span class="ae-state__text">{base().canvas.states[s()]}</span></span>}
        </Show>
        <button type="button" class="ae-iconbtn" aria-label={t().node.close} title={t().node.close} onClick={() => props.onClose()}><Icon name="close" /></button>
      </header>
      <div class="ae-panel__body">
        <div class="ae-panel__stack">
          <For each={waits()}>
            {(w) => (
              <p class="ae-run-node__wait" role="status">
                <Icon name="waiting" size={14} />
                <span>{t().node.waiting[w.kind]}</span>
                <Show when={w.timeout_at}>{(at) => <time class="ae-muted" datetime={at()}>{t().node.until(absoluteTime(at(), props.locale))}</time>}</Show>
              </p>
            )}
          </For>
          <Show when={steps().length > 0} fallback={<Show when={waits().length === 0}><p class="ae-panel__note" data-testid="run-node-empty">{t().node.notReached}</p></Show>}>
            <Show when={multiple()}>
              <div class="ae-chips" role="tablist" aria-label={t().node.steps(steps().length)}>
                <For each={steps()}>
                  {(s, i) => (
                    <button type="button" role="tab" class="ae-chip" aria-selected={step() === s} onClick={() => setPicked(i())}
                      classList={{ 'ae-chip--danger': Boolean(s.error) }}>
                      {stepLabel(s)}
                    </button>
                  )}
                </For>
              </div>
            </Show>
            <Show when={step()}>{(s) => <StepView step={s()} locale={props.locale} />}</Show>
          </Show>
        </div>
      </div>
    </section>
  )
}

function StepView(props: { step: RunStep; locale: EditorLocale }): JSX.Element {
  const t = () => runStrings(props.locale).node
  const took = () => duration(props.step.started_at, props.step.ended_at, props.locale)
  const present = (v: unknown) => v !== undefined && v !== null && !(typeof v === 'object' && Object.keys(v as object).length === 0)
  return (
    <div class="ae-panel__stack" data-testid="run-step">
      <p class="ae-facts">
        <Show when={props.step.outcome}>
          {(o) => <span>{t().outcome}: <span class="ae-outcome">{o()}</span></span>}
        </Show>
        <span class="ae-muted">
          <time datetime={props.step.started_at} title={absoluteTime(props.step.started_at, props.locale)}>{shortTime(props.step.started_at, props.locale)}</time>
          <Show when={took()}>{(d) => <span> · {t().duration(d())}</span>}</Show>
        </span>
      </p>
      <Show when={props.step.error}>
        {(e) => <p class="ae-run-node__error"><Icon name="error" size={14} /><span>{e().message}</span></p>}
      </Show>
      <Show when={!props.step.hidden} fallback={<p class="ae-panel__note">{t().hidden}</p>}>
        <Show when={hasData(props.step)} fallback={<p class="ae-panel__note" data-testid="run-node-nodata">{t().noData}</p>}>
          <Show when={present(props.step.input)}>
            <section class="ae-panel__section">
              <div class="ae-run-node__h"><h3 class="ae-panel__h3">{t().input}</h3><CopyText text={jsonText(props.step.input)} label={t().input} locale={props.locale} /></div>
              <Value value={props.step.input} locale={props.locale} label={t().input} />
            </section>
          </Show>
          <Show when={present(props.step.output)}>
            <section class="ae-panel__section">
              <div class="ae-run-node__h"><h3 class="ae-panel__h3">{t().output}</h3><CopyText text={jsonText(props.step.output)} label={t().output} locale={props.locale} /></div>
              <Value value={props.step.output} locale={props.locale} label={t().output} />
            </section>
          </Show>
          <Show when={props.step.logs.length > 0}>
            <Logs lines={props.step.logs} locale={props.locale} />
          </Show>
        </Show>
      </Show>
    </div>
  )
}

/** Log lines with a filter; an empty match has its own state. */
export function Logs(props: { lines: string[]; locale: EditorLocale; title?: string }): JSX.Element {
  const t = () => runStrings(props.locale).node
  const [query, setQuery] = createSignal('')
  const shown = createMemo(() => {
    const q = query().trim().toLowerCase()
    return q ? props.lines.filter((l) => l.toLowerCase().includes(q)) : props.lines
  })
  const id = `ae-logs-${Math.random().toString(36).slice(2, 8)}`
  return (
    <section class="ae-panel__section" data-testid="run-logs">
      <div class="ae-run-node__h">
        <h3 class="ae-panel__h3">{props.title ?? t().logs}</h3>
        <CopyText text={shown().join('\n')} label={props.title ?? t().logs} locale={props.locale} />
      </div>
      <div class="ae-logs__filter">
        <label class="ae-sr" for={id}>{t().filterLogs}</label>
        <Icon name="search" size={14} />
        <input id={id} type="search" class="form-control form-control-input ae-input" placeholder={t().filterLogs} value={query()} onInput={(e) => setQuery(e.currentTarget.value)} />
        <span class="ae-muted ae-tabular" aria-live="polite">{query().trim() ? t().logCount(shown().length, props.lines.length) : ''}</span>
      </div>
      <Show when={shown().length > 0} fallback={<p class="ae-panel__note" data-testid="logs-nomatch">{t().noLogMatch(query().trim())}</p>}>
        <pre class="ae-logs">{shown().join('\n')}</pre>
      </Show>
    </section>
  )
}
