// The Test node panel (RV8): one real execution of the selected step. Its input is a form by the
// outputs of the steps before it (nodes.<id>.output), filled «From the last run»; the result is the
// outcome, output and logs, or the phrase of the error; a secret the step needs and the tenant lacks is
// marked «not set». A step with an external effect is confirmed first, focus on «Cancel», with the
// connection's name and «The effect is irreversible» when the call is not idempotent.
import { ConfirmationDialog } from '@iota-uz/sdk/solid-ui'
import { createMemo, createSignal, For, Show, type JSX } from 'solid-js'
import { catalogNode, findItem } from '../graph'
import { Icon } from '../icons'
import { describeItem } from '../names'
import { SchemaForm } from '../SchemaForm'
import { strings } from '../strings'
import type { AutomationGraph, EditorApi, EditorCatalog, EditorLocale, JsonSchema, Overlay, RunData, TestNodeResult } from '../types'
import { Value } from './Data'
import { missingSecret, testOverlay } from './model'
import { Logs } from './NodeDrawer'
import { runStrings } from './strings'
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

/** The steps a node can be reached from, nearest first. */
export function ancestors(graph: AutomationGraph, node: string): string[] {
  const out: string[] = []
  const seen = new Set([node])
  const queue = [node]
  while (queue.length > 0) {
    const current = queue.shift()!
    for (const e of graph.edges) {
      if (e.to !== current || seen.has(e.from.node)) continue
      seen.add(e.from.node)
      if (graph.nodes.some((n) => n.id === e.from.node)) out.push(e.from.node)
      queue.push(e.from.node)
    }
  }
  return out
}

const EXAMPLE_RUNS = 5

export function TestPanel(props: TestPanelProps): JSX.Element {
  const t = () => runStrings(props.locale).test
  const base = () => strings(props.locale)
  const ref = createMemo(() => findItem(props.graph, props.node))
  const type = () => (ref()?.item as { type?: string } | undefined)?.type ?? ''
  const spec = createMemo(() => catalogNode(props.catalog, type()))
  const params = () => ((ref()?.item as { params?: Record<string, unknown> } | undefined)?.params ?? {})
  const title = createMemo(() => describeItem(props.graph, props.catalog, props.node, props.locale).title)
  // The form: an object per earlier step with an output schema, keyed by the context path it overrides.
  const schema = createMemo<JsonSchema>(() => {
    const properties: Record<string, JsonSchema> = {}
    for (const id of ancestors(props.graph, props.node)) {
      const node = props.graph.nodes.find((n) => n.id === id)
      const output = node ? catalogNode(props.catalog, node.type)?.output : null
      if (output) properties[`nodes.${id}.output`] = { ...output, title: describeItem(props.graph, props.catalog, id, props.locale).title }
    }
    return { type: 'object', properties }
  })
  const hasFields = () => Object.keys((schema().properties ?? {}) as object).length > 0
  const [input, setInput] = createSignal<Record<string, unknown>>({})
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
    const context = Object.fromEntries(Object.entries(input()).filter(([, v]) => v !== undefined))
    void action.run(() => props.api.testNode({ graph, node_id: props.node, ...(Object.keys(context).length > 0 ? { context } : {}) })).then((res) => {
      setConfirming(false)
      if (!res) return
      setResult(res)
      props.onOverlay(testOverlay(props.node, res))
    }, () => setConfirming(false))
  }

  const fillFrom = (run: RunData) => {
    const next: Record<string, unknown> = {}
    for (const id of ancestors(props.graph, props.node)) {
      const step = [...run.steps].reverse().find((s) => s.node === id && !s.hidden && s.output !== undefined && s.output !== null)
      if (step && `nodes.${id}.output` in ((schema().properties ?? {}) as object)) next[`nodes.${id}.output`] = step.output
    }
    setInput(next)
  }
  const loadExample = () => {
    setExampleNote(null)
    void example.run(async () => {
      const reached = (r: RunData) => r.steps.some((s) => s.node === props.node || ancestors(props.graph, props.node).includes(s.node))
      const cached = [...props.cache.values()].sort((a, b) => b.created_at.localeCompare(a.created_at)).find(reached)
      if (cached) return cached
      if (!props.api.runs || !props.api.run || !props.automationId) return null
      const page = await props.api.runs({ automation_id: props.automationId, limit: EXAMPLE_RUNS })
      for (const row of page.items) {
        const run = await props.api.run(row.run_id)
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
                      <span>{missing() !== null ? t().secretMissing(missing() || '') : e().message}</span>
                    </p>
                  )}
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
