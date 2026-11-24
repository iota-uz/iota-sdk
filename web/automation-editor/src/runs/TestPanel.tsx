// The Test node panel (RV8): one real execution of the selected step. Its input is the subject, the
// event of the trigger that leads to it (by the event's payload schema) and the outputs of the steps before
// it (nodes.<id>.output), filled «From the last run» (its subject and those outputs); the result is the
// outcome, output and logs, or the phrase of the error; a secret the step needs and the tenant lacks is
// marked «not set». A step with an external effect is confirmed first, focus on «Cancel», with the
// connection's name and «The effect is irreversible» when the call is not idempotent.
import { ConfirmationDialog } from '@iota-uz/sdk/solid-ui'
import { createEffect, createMemo, createSignal, For, on, Show, type JSX } from 'solid-js'
import { catalogEvent, catalogNode, findItem } from '../graph'
import { Icon } from '../icons'
import { describeItem } from '../names'
import { SchemaForm } from '../SchemaForm'
import { strings } from '../strings'
import type { AutomationGraph, EditorApi, EditorCatalog, EditorLocale, JsonSchema, Overlay, RunData, Subject, TestNodeResult } from '../types'
import { Value } from './Data'
import { isFinal, missingSecret, testOverlay } from './model'
import { Logs } from './NodeDrawer'
import { runStrings } from './strings'
import { SubjectPicker, subjectEntity } from './SubjectPicker'
import { createAction } from './util'

export interface TestPanelProps {
  graph: AutomationGraph
  catalog: EditorCatalog
  api: EditorApi
  locale: EditorLocale
  node: string
  automationId?: string
  /** Runs read in this editor; the example comes from the newest that reached the step. */
  cache: Map<string, RunData>
  connectionName?(id: string): string | undefined
  /** The result's overlay on the draft it ran on; null takes it off. */
  onOverlay(overlay: Overlay | null): void
  onClose(): void
}

/** The steps a node can be reached from, nearest first; with triggers: the triggers too. */
export function ancestors(graph: AutomationGraph, node: string, triggers = false): string[] {
  const out: string[] = []
  const seen = new Set([node])
  const queue = [node]
  const isTrigger = (id: string) => graph.triggers.some((t) => t.id === id)
  while (queue.length > 0) {
    const current = queue.shift()!
    for (const e of graph.edges) {
      if (e.to !== current || seen.has(e.from.node)) continue
      seen.add(e.from.node)
      if (graph.nodes.some((n) => n.id === e.from.node) || (triggers && isTrigger(e.from.node))) out.push(e.from.node)
      queue.push(e.from.node)
    }
  }
  return out
}

/** A generated key of a confirmed test. */
const newKey = () => (globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`)

const EXAMPLE_RUNS = 5

export function TestPanel(props: TestPanelProps): JSX.Element {
  const t = () => runStrings(props.locale).test
  const base = () => strings(props.locale)
  const ref = createMemo(() => findItem(props.graph, props.node))
  const type = () => (ref()?.item as { type?: string } | undefined)?.type ?? ''
  const spec = createMemo(() => catalogNode(props.catalog, type()))
  const params = () => ((ref()?.item as { params?: Record<string, unknown> } | undefined)?.params ?? {})
  const title = createMemo(() => describeItem(props.graph, props.catalog, props.node, props.locale).title)
  // The form: the event of the trigger that leads to the step (by its payload schema), and an object per
  // earlier step with an output schema, keyed by the context path it overrides.
  // The triggers that lead to the step; the author picks the one the test starts from (its event kind is
  // what event.kind reads).
  const triggers = createMemo(() => {
    const ids = new Set(ancestors(props.graph, props.node, true))
    return props.graph.triggers.filter((tr) => ids.has(tr.id))
  })
  const [triggerId, setTriggerId] = createSignal<string | null>(null)
  const trigger = () => triggers().find((tr) => tr.id === triggerId()) ?? triggers()[0]
  const triggerTitle = (id: string) => describeItem(props.graph, props.catalog, id, props.locale).title
  const schema = createMemo<JsonSchema>(() => {
    const properties: Record<string, JsonSchema> = {}
    const payload = trigger() ? catalogEvent(props.catalog, trigger()!.event)?.payload : undefined
    if (payload && Object.keys((payload.properties ?? {}) as object).length > 0) {
      properties.event = { ...payload, title: t().event(catalogEvent(props.catalog, trigger()!.event)?.title ?? trigger()!.event) }
    }
    for (const id of ancestors(props.graph, props.node)) {
      const node = props.graph.nodes.find((n) => n.id === id)
      const output = node ? catalogNode(props.catalog, node.type)?.output : null
      if (output) properties[`nodes.${id}.output`] = { ...output, title: describeItem(props.graph, props.catalog, id, props.locale).title }
    }
    return { type: 'object', properties }
  })
  const hasFields = () => Object.keys((schema().properties ?? {}) as object).length > 0
  const [input, setInput] = createSignal<Record<string, unknown>>({})
  const entity = createMemo(() => subjectEntity(props.graph, props.catalog))
  const [subject, setSubject] = createSignal<Subject | null>(null)
  // One key per test the author confirmed: a retry after a failure sends it again (the server answers with
  // the first test's result and calls nothing twice); a change of the input or a finished test drops it.
  let key: string | null = null
  createEffect(on([input, subject, triggerId], () => { key = null }, { defer: true }))
  const [result, setResult] = createSignal<TestNodeResult | null>(null)
  const [confirming, setConfirming] = createSignal(false)
  const [exampleNote, setExampleNote] = createSignal<string | null>(null)
  const action = createAction()
  const example = createAction()

  // An effect outside: confirmed first. Not idempotent (or unknown for an effect node): irreversible.
  const effect = () => spec()?.behavior === 'call' || spec()?.behavior === 'effect'
  const irreversible = () => effect() && spec()?.idempotent !== true
  const connection = () => {
    const id = typeof params().connection === 'string' ? (params().connection as string) : ''
    return (id && props.connectionName?.(id)) || spec()?.title || t().noConnection
  }
  const secrets = () => (Array.isArray(params().secrets) ? (params().secrets as unknown[]).filter((s): s is string => typeof s === 'string') : [])
  const missing = () => missingSecret(result()?.error)

  const start = () => {
    if (action.busy()) return
    if (effect()) setConfirming(true)
    else execute()
  }
  const execute = () => {
    const graph = props.graph
    const context: Record<string, unknown> = Object.fromEntries(Object.entries(input()).filter(([, v]) => v !== undefined))
    // The event carries its kind: the chosen trigger's event, as a run's event does.
    const tr = trigger()
    if (tr) context.event = { ...((context.event as Record<string, unknown> | undefined) ?? {}), kind: tr.event }
    key ??= newKey()
    const sent = key
    const who = subject()
    void action.run(() => props.api.testNode({
      graph, node_id: props.node, idempotency_key: sent,
      ...(who ? { subject: { entity: who.entity, key: who.key } } : {}),
      ...(Object.keys(context).length > 0 ? { context } : {}),
    })).then((res) => {
      setConfirming(false)
      if (!res) return
      if (key === sent) key = null
      setResult(res)
      props.onOverlay(testOverlay(props.node, res))
    }, () => setConfirming(false))
  }

  // The run's subject, its event (as the server projected it for the author) and the outputs of the steps
  // before this one; what the run does not carry keeps what was typed.
  const fillFrom = (run: RunData) => {
    if (run.subject && run.subject.entity === entity()) setSubject(run.subject)
    const next: Record<string, unknown> = { ...input() }
    const ev = run.event
    if (ev && typeof ev.kind === 'string') {
      const tr = triggers().find((x) => x.id === run.trigger && x.event === ev.kind) ?? triggers().find((x) => x.event === ev.kind)
      if (tr) {
        setTriggerId(tr.id)
        const { kind: _kind, ...payload } = ev
        if ('event' in ((schema().properties ?? {}) as object)) next.event = payload
      }
    }
    for (const id of ancestors(props.graph, props.node)) {
      const step = [...run.steps].reverse().find((s) => s.node === id && !s.hidden && s.output !== undefined && s.output !== null)
      if (step && `nodes.${id}.output` in ((schema().properties ?? {}) as object)) next[`nodes.${id}.output`] = step.output
    }
    setInput(next)
  }
  // The newest run that reached this very step: the list is read fresh, a run is read once (a finished
  // run from the editor's cache is final).
  const loadExample = () => {
    setExampleNote(null)
    void example.run(async () => {
      const reached = (r: RunData) => r.steps.some((s) => s.node === props.node && !s.hidden)
      if (!props.api.runs || !props.api.run || !props.automationId) {
        return [...props.cache.values()].sort((a, b) => b.created_at.localeCompare(a.created_at)).find(reached) ?? null
      }
      const page = await props.api.runs({ automation_id: props.automationId, limit: EXAMPLE_RUNS })
      for (const row of page.items) {
        const known = props.cache.get(row.run_id)
        const run = known && isFinal(known.state) ? known : await props.api.run(row.run_id)
        props.cache.set(run.run_id, run)
        if (reached(run)) return run
      }
      return null
    }).then((run) => {
      if (run) fillFrom(run)
      else setExampleNote(t().exampleNone)
    }, () => setExampleNote(t().failed))
  }

  return (
    <section class="ae-dock ae-test" aria-labelledby="ae-test-title" data-testid="test-node">
      <header class="ae-validation__head">
        <h2 class="ae-validation__title" id="ae-test-title">{t().title(title())}</h2>
        <span class="ae-validation__summary">{t().note}</span>
        <button type="button" class="ae-iconbtn" aria-label={t().close} title={t().close} onClick={() => { props.onOverlay(null); props.onClose() }}><Icon name="close" /></button>
      </header>
      <div class="ae-dock__body">
        <form class="ae-dock__form" onSubmit={(e) => { e.preventDefault(); start() }}>
          <div class="ae-run-node__h">
            <h3 class="ae-panel__h3">{t().input}</h3>
            <Show when={props.api.run || props.cache.size > 0}>
              <button type="button" class="ae-btn ae-btn--ghost ae-btn--sm" aria-busy={example.busy()} onClick={loadExample} data-testid="test-example">
                <Show when={example.busy()} fallback={<Icon name="undo" size={14} />}><span class="ae-spinner" aria-hidden="true" /></Show>{t().example}
              </button>
            </Show>
          </div>
          <Show when={exampleNote()}>{(note) => <p class="ae-muted" role="status">{note()}</p>}</Show>
          <Show when={triggers().length > 1}>
            <fieldset class="ae-field">
              <legend class="ae-field__label">{t().trigger}</legend>
              <div class="ae-segmented" role="radiogroup" aria-label={t().trigger}>
                <For each={triggers()}>
                  {(tr) => (
                    <label class="ae-segmented__item">
                      <input type="radio" name="ae-test-trigger" value={tr.id} checked={trigger()?.id === tr.id} disabled={action.busy()}
                        onChange={() => { setTriggerId(tr.id); setInput(({ event: _event, ...rest }) => rest) }} />
                      <span>{triggerTitle(tr.id)}</span>
                    </label>
                  )}
                </For>
              </div>
            </fieldset>
          </Show>
          <SubjectPicker entity={entity()} api={props.api} locale={props.locale} value={subject()} onChange={setSubject} disabled={action.busy()} />
          <Show when={hasFields()} fallback={<p class="ae-muted">{t().noInput}</p>}>
            <SchemaForm
              schema={schema()}
              value={input()}
              onChange={setInput}
              readOnly={action.busy()}
              locale={props.locale}
              nodeType={type()}
              nodeId={props.node}
              graph={props.graph}
              expressions={[]}
              templates={[]}
              completions={() => []}
            />
          </Show>
          <Show when={secrets().length > 0}>
            <div class="ae-field">
              <span class="ae-field__label">{t().secrets}</span>
              <ul class="ae-secrets">
                <For each={secrets()}>
                  {(name) => (
                    <li>
                      <code>{name}</code>
                      <Show when={missing() === name}><span class="ae-tag ae-tag--danger" data-testid="secret-unset">{t().secretUnset}</span></Show>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          </Show>
          <div class="ae-dock__actions">
            <button type="submit" class="btn btn-primary btn-normal" aria-busy={action.busy()} data-testid="test-start">
              <Show when={action.busy()}><span class="ae-spinner" aria-hidden="true" /></Show>
              {action.busy() ? t().running : action.state().status === 'failed' ? base().common.retry : t().run}
            </button>
          </div>
          <Show when={action.state().status === 'busy' && (action.state() as { slow: boolean }).slow}><p role="status" class="ae-muted" data-testid="test-slow">{t().stillWaiting}</p></Show>
          <Show when={action.state().status === 'failed'}><p role="alert" class="ae-danger">{t().failed}</p></Show>
        </form>
        <div class="ae-dock__result" aria-live="polite" data-testid="test-result">
          <Show when={result()}>
            {(r) => (
              <>
                <h3 class="ae-panel__h3">{t().result}</h3>
                <p class="ae-facts">
                  <span>{t().outcome}: <span class="ae-outcome">{r().outcome}</span></span>
                  <Show when={r().duration_ms !== undefined}><span class="ae-muted ae-tabular">{t().duration(r().duration_ms!)}</span></Show>
                </p>
                <Show when={r().error}>
                  {(e) => (
                    <p class="ae-run-node__error" data-testid="test-error">
                      <Icon name="error" size={14} />
                      <span>{missing() !== null ? t().secretMissing(missing() || '') : t().reasons[(['timeout', 'network_blocked', 'connection_unavailable', 'exception', 'outcome_unknown'] as const).find((r) => r === e().reason) ?? 'other']}</span>
                    </p>
                  )}
                </Show>
                <Show when={r().error && missing() === null && r().error!.message}>
                  <details class="ae-answer"><summary>{t().answer}</summary><pre class="ae-value__text">{r().error!.message}</pre></details>
                </Show>
                <Show when={r().output !== undefined && r().output !== null}>
                  <section class="ae-panel__section">
                    <h3 class="ae-panel__h3">{t().output}</h3>
                    <Value value={r().output} locale={props.locale} label={t().output} />
                  </section>
                </Show>
                <Show when={(r().logs?.length ?? 0) > 0}><Logs lines={r().logs!} locale={props.locale} title={t().logs} /></Show>
              </>
            )}
          </Show>
        </div>
      </div>
      <ConfirmationDialog
        open={confirming()}
        onOpenChange={(open) => { if (!open && !action.busy()) setConfirming(false) }}
        heading={t().confirmTitle}
        text={t().confirmText(connection())}
        confirmText={action.busy() ? t().running : t().confirm}
        cancelText={t().cancel}
        closeLabel={base().common.close}
        danger={irreversible()}
        busy={action.busy()}
        initialFocus={() => document.querySelector<HTMLElement>('dialog[open] [data-test-id="dialog-cancel-btn"]') ?? undefined}
        onConfirm={execute}
        data-testid="test-confirm"
      >
        <Show when={irreversible()}>
          <p class="ae-danger" data-testid="test-irreversible"><Icon name="error" size={14} /> {t().irreversible}</p>
        </Show>
        <Show when={action.state().status === 'busy' && (action.state() as { slow: boolean }).slow}><p role="status">{t().stillWaiting}</p></Show>
      </ConfirmationDialog>
    </section>
  )
}
