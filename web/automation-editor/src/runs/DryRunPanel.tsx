// The Dry run panel (RV7): runs the draft over a subject and events without effects. The path goes on
// the canvas as a run's overlay; the panel lists it, says which condition stopped a trigger, how many
// decisions were made live, and what would have been sent (captured effects with rendered params).
import { Combobox, type ComboboxOption } from '@iota-uz/sdk/solid-ui'
import { createMemo, createSignal, For, Show, type JSX } from 'solid-js'
import { catalogEvent, findItem } from '../graph'
import { Icon } from '../icons'
import { describeItem } from '../names'
import { strings } from '../strings'
import type { AutomationGraph, DecisionMode, EditorApi, EditorCatalog, EditorLocale, Subject, TraceStep } from '../types'
import { Value } from './Data'
import { runStrings } from './strings'
import { createAction } from './util'

export interface DryRunPanelProps {
  graph: AutomationGraph
  catalog: EditorCatalog
  api: EditorApi
  locale: EditorLocale
  onTrace(trace: TraceStep[] | null): void
  onPick(id: string): void
  onClose(): void
}

const MODES: DecisionMode[] = ['cached', 'live', 'scripted']

/** The subject entity the graph's triggers are about ("none" when none is). */
function subjectEntity(graph: AutomationGraph, catalog: EditorCatalog): string {
  for (const trigger of graph.triggers) {
    const entity = catalogEvent(catalog, trigger.event)?.subject
    if (entity && entity !== 'none') return entity
  }
  return 'none'
}

export function DryRunPanel(props: DryRunPanelProps): JSX.Element {
  const t = () => runStrings(props.locale).dry
  const entity = createMemo(() => subjectEntity(props.graph, props.catalog))
  const firstEvent = () => props.graph.triggers[0]?.event ?? ''
  const [events, setEvents] = createSignal(JSON.stringify([{ kind: firstEvent() }], null, 2))
  const [context, setContext] = createSignal('{}')
  const [mode, setMode] = createSignal<DecisionMode>('cached')
  const [subject, setSubject] = createSignal<Subject | null>(null)
  const [options, setOptions] = createSignal<ComboboxOption[]>([])
  const [trace, setTrace] = createSignal<TraceStep[] | null>(null)
  const [invalid, setInvalid] = createSignal<'events' | 'context' | null>(null)
  const action = createAction()
  const labels = new Map<string, Subject>()

  const loadSubjects = async (query: string): Promise<ComboboxOption[]> => {
    if (!props.api.searchSubjects) return []
    const found = await props.api.searchSubjects(entity(), query)
    const list = found.map((f) => {
      labels.set(`${f.subject.entity}:${f.subject.key}`, { ...f.subject, label: f.label })
      return { value: `${f.subject.entity}:${f.subject.key}`, label: f.note ? `${f.label} · ${f.note}` : f.label }
    })
    setOptions(list)
    return list
  }

  const parse = (): { events: Record<string, unknown>[]; context: Record<string, unknown> } | null => {
    let ev: unknown
    let ctx: unknown = {}
    try {
      ev = JSON.parse(events())
    } catch {
      setInvalid('events')
      return null
    }
    if (!Array.isArray(ev) || ev.some((e) => !e || typeof e !== 'object' || Array.isArray(e))) {
      setInvalid('events')
      return null
    }
    if (mode() === 'scripted' && context().trim()) {
      try {
        ctx = JSON.parse(context())
      } catch {
        setInvalid('context')
        return null
      }
      if (!ctx || typeof ctx !== 'object' || Array.isArray(ctx)) {
        setInvalid('context')
        return null
      }
    }
    setInvalid(null)
    return { events: ev as Record<string, unknown>[], context: ctx as Record<string, unknown> }
  }

  const run = () => {
    const input = parse()
    if (!input) return
    const graph = props.graph
    void action.run(() => props.api.dryRun({
      graph, events: input.events, decisions: mode(),
      subject: subject() ?? { entity: entity() === 'none' ? 'none' : entity(), key: '' },
      ...(Object.keys(input.context).length > 0 ? { context: input.context } : {}),
    })).then((res) => {
      if (!res) return
      setTrace(res.trace)
      props.onTrace(res.trace)
    }, () => undefined)
  }

  const name = (id: string) => (id && findItem(props.graph, id) ? describeItem(props.graph, props.catalog, id, props.locale).title : id)
  const captured = createMemo(() => (trace() ?? []).filter((s) => s.status === 'captured' && (s.effects?.length ?? 0) > 0))
  const live = createMemo(() => (trace() ?? []).filter((s) => s.status === 'executed').length)
  const stepText = (s: TraceStep) => {
    switch (s.kind) {
      case 'not_triggered': return t().notTriggered
      case 'wait': return t().waits(name(s.node))
      case 'ended': return t().ended
      case 'error': return s.path && s.message?.includes('missing') ? t().missing(s.path) : s.message ?? t().error
      default: return s.outcome ?? s.kind
    }
  }
  const icon = (s: TraceStep) => (s.kind === 'error' ? 'failed' : s.kind === 'not_triggered' ? 'skipped' : s.kind === 'wait' ? 'waiting' : s.kind === 'cancelled' ? 'cancelled' : 'passed')

  return (
    <section class="ae-dock ae-dryrun" aria-labelledby="ae-dryrun-title" data-testid="dry-run">
      <header class="ae-validation__head">
        <h2 class="ae-validation__title" id="ae-dryrun-title">{t().title}</h2>
        <span class="ae-validation__summary"><Icon name="check" size={14} /> {t().note}</span>
        <button type="button" class="ae-iconbtn" aria-label={t().close} title={t().close} onClick={() => { props.onTrace(null); props.onClose() }}><Icon name="close" /></button>
      </header>
      <div class="ae-dock__body">
        <form class="ae-dock__form" onSubmit={(e) => { e.preventDefault(); run() }}>
          <Show when={entity() !== 'none' && props.api.searchSubjects} fallback={<p class="ae-muted">{t().subject}: {t().subjectNone}</p>}>
            <Combobox
              label={t().subject}
              placeholder={t().subjectSearch}
              searchable
              options={options()}
              loadOptions={(q) => loadSubjects(q)}
              value={subject() ? `${subject()!.entity}:${subject()!.key}` : ''}
              onValueChange={(v) => setSubject(labels.get(typeof v === 'string' ? v : v[0] ?? '') ?? null)}
            />
          </Show>
          <label class="ae-field">
            <span class="ae-field__label">{t().events}</span>
            <textarea class="form-control form-control-input ae-input ae-code" rows={5} spellcheck={false} value={events()} onInput={(e) => setEvents(e.currentTarget.value)}
              aria-invalid={invalid() === 'events'} aria-describedby="ae-dry-events-hint" data-testid="dry-events" />
            <span class="ae-field__hint" id="ae-dry-events-hint">{invalid() === 'events' ? t().invalidJson : t().eventsHint}</span>
          </label>
          <fieldset class="ae-field">
            <legend class="ae-field__label">{t().decisions}</legend>
            <div class="ae-segmented" role="radiogroup" aria-label={t().decisions}>
              <For each={MODES}>
                {(m) => (
                  <label class="ae-segmented__item">
                    <input type="radio" name="ae-dry-mode" value={m} checked={mode() === m} onChange={() => setMode(m)} />
                    <span>{t().modes[m]}</span>
                  </label>
                )}
              </For>
            </div>
            <span class="ae-field__hint">{t().modeHints[mode()]}</span>
          </fieldset>
          <Show when={mode() === 'scripted'}>
            <label class="ae-field">
              <span class="ae-field__label">{t().context}</span>
              <textarea class="form-control form-control-input ae-input ae-code" rows={3} spellcheck={false} value={context()} onInput={(e) => setContext(e.currentTarget.value)} aria-invalid={invalid() === 'context'} />
              <span class="ae-field__hint">{invalid() === 'context' ? runStrings(props.locale).test.invalidJson : t().contextHint}</span>
            </label>
          </Show>
          <div class="ae-dock__actions">
            <button type="submit" class="btn btn-primary btn-normal" aria-busy={action.busy()} data-testid="dry-run-start">
              <Show when={action.busy()}><span class="ae-spinner" aria-hidden="true" /></Show>
              {action.busy() ? t().running : action.state().status === 'failed' ? strings(props.locale).common.retry : t().run}
            </button>
          </div>
          <Show when={action.state().status === 'busy' && (action.state() as { slow: boolean }).slow}><p role="status" class="ae-muted">{t().stillWaiting}</p></Show>
          <Show when={action.state().status === 'failed'}><p role="alert" class="ae-danger">{t().failed}</p></Show>
        </form>
        <div class="ae-dock__result" aria-live="polite">
          <Show when={trace()} fallback={<p class="ae-muted">{t().empty}</p>}>
            {(steps) => (
              <>
                <h3 class="ae-panel__h3">{t().path}<Show when={live() > 0}><span class="ae-tag">{t().live(live())}</span></Show></h3>
                <ol class="ae-trace" data-testid="dry-trace">
                  <For each={steps()}>
                    {(s) => (
                      <li class="ae-trace__step" classList={{ 'ae-trace__step--bad': s.kind === 'error' || s.kind === 'not_triggered' }}>
                        <Icon name={icon(s)} size={14} />
                        <button type="button" class="ae-link" onClick={() => s.node && props.onPick(s.node)} disabled={!s.node || !findItem(props.graph, s.node)}>{name(s.node)}</button>
                        <span>{stepText(s)}</span>
                        <Show when={s.status}><span class="ae-tag">{s.status === 'executed' ? t().executed : t().captured}</span></Show>
                        <Show when={s.kind === 'not_triggered' && (s.path || s.message)}>
                          <span class="ae-trace__cond" data-testid="dry-condition">{t().condition}: <code>{s.path || ''}</code>{s.path && s.message ? ' · ' : ''}{s.message ?? ''}</span>
                        </Show>
                      </li>
                    )}
                  </For>
                </ol>
                <h3 class="ae-panel__h3">{t().wouldSend}</h3>
                <Show when={captured().length > 0} fallback={<p class="ae-muted">{t().nothingSent}</p>}>
                  <ul class="ae-effects" data-testid="dry-effects">
                    <For each={captured()}>
                      {(s) => (
                        <li class="ae-effects__item">
                          <span class="ae-effects__node">{name(s.node)}</span>
                          <For each={s.effects ?? []}>{(e) => <Value value={e} locale={props.locale} label={name(s.node)} />}</For>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </>
            )}
          </Show>
        </div>
      </div>
    </section>
  )
}
