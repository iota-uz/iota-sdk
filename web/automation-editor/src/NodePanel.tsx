// The panel of the selected item: Configure (the params form from the catalog, the trigger's event,
// filter and params, or the schedule editor) and Data (the output schema and copyable paths).
import { Combobox, CopyButton, type ComboboxOption } from '@iota-uz/sdk/solid-ui'
import { createMemo, createSignal, For, Match, Show, Switch, type JSX } from 'solid-js'
import { contextPaths, outputPaths, schemaPaths, type CompletionPath } from './completion'
import { catalogEvent, catalogNode, findItem, nodeSpec, updateNode, updateTrigger } from './graph'
import { Icon } from './icons'
import { describeItem } from './names'
import { ScheduleEditor } from './ScheduleEditor'
import { SchemaForm } from './SchemaForm'
import { pick as pickLocale } from './locale'
import { strings } from './strings'
import type { AutomationGraph, EditorApi, EditorCatalog, EditorLocale, FieldSlot, GraphError, JsonSchema } from './types'
import { errorTarget } from './validation'

export interface NodePanelProps {
  graph: AutomationGraph
  catalog: EditorCatalog
  id: string
  locale: EditorLocale
  readOnly: boolean
  fieldSlot?: FieldSlot
  searchOptions?: EditorApi['searchOptions']
  errors: GraphError[]
  focusField?: string | null
  /** `key` groups keystrokes of one field into one history step. */
  onChange(graph: AutomationGraph, key: string): void
  onClose(): void
}

// The action picker's words: only this lazy panel uses them, so they stay out of the package entry.
const ACTION_STRINGS: Record<EditorLocale, { action: string; placeholder: string; missing(key: string): string; pick: string }> = {
  en: { action: 'Action', placeholder: 'Choose an action', missing: (key) => `The action “${key}” is not available: its module may be off or you lack the permission.`, pick: 'Choose the action this step runs; its fields appear here.' },
  ru: { action: 'Действие', placeholder: 'Выберите действие', missing: (key) => `Действие «${key}» недоступно: возможно, модуль выключен или нет прав.`, pick: 'Выберите действие, которое выполняет шаг; его поля появятся здесь.' },
  uz: { action: 'Amal', placeholder: 'Amalni tanlang', missing: (key) => `«${key}» amali mavjud emas: modul oʻchirilgan yoki ruxsat yoʻq boʻlishi mumkin.`, pick: 'Qadam bajaradigan amalni tanlang; uning maydonlari shu yerda chiqadi.' },
  'uz-Cyrl': { action: 'Амал', placeholder: 'Амални танланг', missing: (key) => `«${key}» амали мавжуд эмас: модул ўчирилган ёки рухсат йўқ бўлиши мумкин.`, pick: 'Қадам бажарадиган амални танланг; унинг майдонлари шу ерда чиқади.' },
}

const FILTER_SCHEMA = (title: string, description: string): JsonSchema => ({
  type: 'object',
  properties: { filter: { type: ['string', 'null'], title, description, maxLength: 2000 } },
})

export function NodePanel(props: NodePanelProps): JSX.Element {
  const t = () => strings(props.locale)
  const [tab, setTab] = createSignal<'configure' | 'data'>('configure')
  const ref = createMemo(() => findItem(props.graph, props.id))
  const name = createMemo(() => describeItem(props.graph, props.catalog, props.id, props.locale))
  const completions = createMemo<CompletionPath[]>(() => contextPaths({ catalog: props.catalog, graph: props.graph, nodeId: ref()?.kind === 'node' ? props.id : null }))
  const fieldErrors = createMemo(() => props.errors
    .map((error) => ({ error, target: errorTarget(props.graph, error.path) }))
    .filter((e) => e.target.id === props.id)
    .map((e) => ({ path: e.target.field ?? '', message: e.error.message })))
  const tabId = `ae-panel-${props.id}`

  const eventOptions = createMemo<ComboboxOption[]>(() => {
    const groups = new Map<string, ComboboxOption[]>()
    for (const event of props.catalog.events) {
      const list = groups.get(event.group) ?? []
      list.push({
        value: event.key,
        label: event.connected === false ? `${event.title} · ${t().palette.noConnection}` : event.title,
        disabled: event.connected === false,
      })
      groups.set(event.group, list)
    }
    const out: ComboboxOption[] = []
    for (const [group, options] of groups) {
      out.push({ value: `group:${group}`, label: group, groupHeader: true })
      out.push(...options)
    }
    return out
  })

  const outputList = createMemo<CompletionPath[]>(() => {
    const r = ref()
    if (!r) return []
    if (r.kind === 'node') return outputPaths(props.id, nodeSpec(props.catalog, r.item)?.output ?? null)
    return schemaPaths('event', catalogEvent(props.catalog, r.item.event)?.payload ?? null, 'trigger')
  })

  const onTabKey = (event: KeyboardEvent) => {
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      event.preventDefault()
      const next = tab() === 'configure' ? 'data' : 'configure'
      setTab(next)
      document.getElementById(`${tabId}-${next}`)?.focus()
    }
  }

  return (
    <aside class="ae-panel" role="complementary" aria-label={t().panel.label}>
      <header class="ae-panel__head">
        <span class="ae-panel__icon"><Icon name={name().icon ?? (name().kind === 'trigger' ? 'event' : catalogNode(props.catalog, (ref()?.item as { type?: string } | undefined)?.type ?? '')?.behavior ?? 'node')} /></span>
        <div class="ae-panel__titles">
          <h2 class="ae-panel__title" title={name().title}>{name().title}</h2>
          <p class="ae-panel__sub" title={name().subtitle}>{name().kind === 'trigger' ? t().canvas.trigger : catalogNode(props.catalog, (ref()?.item as { type?: string })?.type ?? '')?.description ?? ''}</p>
        </div>
        <button type="button" class="ae-iconbtn" aria-label={t().panel.close} title={t().panel.close} onClick={() => props.onClose()}>
          <Icon name="close" />
        </button>
      </header>
      <div class="ae-tabs" role="tablist" aria-label={t().panel.label} onKeyDown={onTabKey}>
        <For each={['configure', 'data'] as const}>
          {(value) => (
            <button
              type="button"
              role="tab"
              id={`${tabId}-${value}`}
              class="ae-tab"
              aria-selected={tab() === value}
              aria-controls={`${tabId}-${value}-panel`}
              tabindex={tab() === value ? 0 : -1}
              onClick={() => setTab(value)}
            >
              {value === 'configure' ? t().panel.configure : t().panel.data}
            </button>
          )}
        </For>
      </div>
      <div class="ae-panel__body" role="tabpanel" id={`${tabId}-${tab()}-panel`} aria-labelledby={`${tabId}-${tab()}`}>
        <Show when={tab() === 'configure'} fallback={<DataTab paths={outputList()} locale={props.locale} />}>
          <Switch>
            <Match when={ref()?.kind === 'node' && ref()}>
              {(r) => {
                const node = () => r().item as AutomationGraph['nodes'][number]
                const own = () => catalogNode(props.catalog, node().type)
                // A front type (the generic "action"): a picker over the types it can name, then the named
                // type's own form. The node keeps its type and `{action, args}`.
                const front = () => own()?.dispatch
                const picked = () => {
                  const rule = front()
                  const key = rule ? node().params[rule.param] : undefined
                  return typeof key === 'string' ? key : ''
                }
                const target = () => (front() ? (picked() ? catalogNode(props.catalog, front()!.prefix + picked()) : undefined) : own())
                const actionOptions = createMemo<ComboboxOption[]>(() => {
                  const rule = front()
                  if (!rule) return []
                  const groups = new Map<string, ComboboxOption[]>()
                  for (const n of props.catalog.nodes) {
                    if (n === own() || !n.type.startsWith(rule.prefix)) continue
                    const key = n.type.slice(rule.prefix.length)
                    const list = groups.get(n.group) ?? []
                    list.push({ value: key, label: `${n.title} · ${key}`, disabled: n.connected === false })
                    groups.set(n.group, list)
                  }
                  const out: ComboboxOption[] = []
                  for (const [group, options] of groups) out.push({ value: `group:${group}`, label: group, groupHeader: true }, ...options)
                  return out
                })
                const pick = (key: string) => {
                  const rule = front()
                  if (!rule || !key || key === picked()) return
                  // Args the new action also takes are kept; the others go.
                  const next = catalogNode(props.catalog, rule.prefix + key)
                  const argsSchema = (next?.params.properties as Record<string, JsonSchema> | undefined)?.args
                  const known = new Set(Object.keys((argsSchema?.properties as Record<string, unknown> | undefined) ?? {}))
                  const old = (node().params.args ?? {}) as Record<string, unknown>
                  const args = Object.fromEntries(Object.entries(old).filter(([name]) => known.has(name)))
                  props.onChange(updateNode(props.graph, props.id, { ...node().params, [rule.param]: key, args }), `${rule.param}:${props.id}`)
                }
                const spec = () => target() ?? (front() ? undefined : own())
                return (
                  <div class="ae-panel__stack">
                  <Show when={front()}>
                    {(rule) => (
                      <div class="ae-field" data-param={rule().param}>
                        <Combobox
                          label={pickLocale(ACTION_STRINGS, props.locale).action}
                          options={actionOptions()}
                          value={picked() || undefined}
                          searchable
                          readOnly={props.readOnly}
                          placeholder={pickLocale(ACTION_STRINGS, props.locale).placeholder}
                          labels={{ notFound: t().palette.nothing('') }}
                          onValueChange={(value) => pick(Array.isArray(value) ? value[0] ?? '' : value)}
                        />
                        <Show when={picked() && !target()}>
                          <p class="ae-panel__note" role="alert">{pickLocale(ACTION_STRINGS, props.locale).missing(picked())}</p>
                        </Show>
                        <Show when={!picked()}>
                          <p class="ae-panel__note">{pickLocale(ACTION_STRINGS, props.locale).pick}</p>
                        </Show>
                      </div>
                    )}
                  </Show>
                  <Show when={spec()} fallback={<Show when={!front()}><p class="ae-panel__note" role="alert">{t().panel.unknownType(node().type)}</p></Show>}>
                    {(s) => (
                      <SchemaForm
                        schema={s().params}
                        value={node().params}
                        onChange={(params) => {
                          const rule = front()
                          // The named type's form has no `action` field: the picked key stays on the node.
                          const next = rule ? { ...params, [rule.param]: picked() } : params
                          props.onChange(updateNode(props.graph, props.id, next), `params:${props.id}`)
                        }}
                        readOnly={props.readOnly}
                        locale={props.locale}
                        nodeType={s().type}
                        nodeId={props.id}
                        graph={props.graph}
                        events={props.catalog.events}
                        fieldSlot={props.fieldSlot}
                        searchOptions={props.searchOptions}
                        expressions={s().expressions}
                        templates={s().templates}
                        completions={completions}
                        focusPath={props.focusField ?? null}
                        errors={fieldErrors()}
                      />
                    )}
                  </Show>
                  </div>
                )
              }}
            </Match>
            <Match when={ref()?.kind === 'trigger' && ref()}>
              {(r) => {
                const trigger = () => r().item as AutomationGraph['triggers'][number]
                const event = () => catalogEvent(props.catalog, trigger().event)
                return (
                  <div class="ae-panel__stack">
                    <div class="ae-field" data-param="event">
                      <Combobox
                        label={t().panel.event}
                        options={eventOptions()}
                        value={trigger().event}
                        searchable
                        readOnly={props.readOnly}
                        placeholder={t().panel.eventPlaceholder}
                        labels={{ notFound: t().palette.nothing('') }}
                        onValueChange={(value) => {
                          const key = Array.isArray(value) ? value[0] : value
                          if (!key || key === trigger().event) return
                          const next = catalogEvent(props.catalog, key)
                          props.onChange(updateTrigger(props.graph, props.id, { event: key, params: undefined, filter: next?.schedule || next?.manual ? undefined : trigger().filter }), `event:${props.id}`)
                        }}
                      />
                      <Show when={!event()}>
                        <p class="ae-panel__note" role="alert">{t().panel.eventMissing}</p>
                      </Show>
                    </div>
                    <Switch>
                      <Match when={event()?.schedule}>
                        <section class="ae-panel__section" aria-label={t().panel.schedule}>
                          <ScheduleEditor
                            params={trigger().params ?? {}}
                            onChange={(params) => props.onChange(updateTrigger(props.graph, props.id, { params }), `params:${props.id}`)}
                            readOnly={props.readOnly}
                            locale={props.locale}
                          />
                        </section>
                      </Match>
                      <Match when={event()}>
                        {(e) => (
                          <>
                            <Show when={e().trigger_params}>
                              {(schema) => (
                                <section class="ae-panel__section" aria-label={t().panel.params}>
                                  <SchemaForm
                                    schema={schema()}
                                    value={trigger().params ?? {}}
                                    onChange={(params) => props.onChange(updateTrigger(props.graph, props.id, { params }), `params:${props.id}`)}
                                    readOnly={props.readOnly}
                                    locale={props.locale}
                                    nodeType={trigger().event}
                                    nodeId={props.id}
                                    graph={props.graph}
                                    fieldSlot={props.fieldSlot}
                        searchOptions={props.searchOptions}
                                    expressions={[]}
                                    templates={[]}
                                    completions={completions}
                                    focusPath={props.focusField ?? null}
                                    errors={fieldErrors()}
                                  />
                                </section>
                              )}
                            </Show>
                            {/* A filter only for event triggers, not for schedules or manual starts. */}
                            <Show when={!e().manual}>
                              <SchemaForm
                                schema={FILTER_SCHEMA(t().panel.filter, t().panel.filterHint)}
                                value={trigger().filter ? { filter: trigger().filter } : {}}
                                onChange={(next) => props.onChange(updateTrigger(props.graph, props.id, { filter: typeof next.filter === 'string' ? next.filter : undefined }), `filter:${props.id}`)}
                                readOnly={props.readOnly}
                                locale={props.locale}
                                nodeType={trigger().event}
                                nodeId={props.id}
                                graph={props.graph}
                                expressions={[{ param: 'filter', type: 'bool' }]}
                                templates={[]}
                                completions={completions}
                                focusPath={props.focusField ?? null}
                                errors={fieldErrors()}
                              />
                            </Show>
                          </>
                        )}
                      </Match>
                    </Switch>
                  </div>
                )
              }}
            </Match>
          </Switch>
        </Show>
      </div>
      <footer class="ae-panel__foot">
        <span class="ae-panel__id">{t().panel.id} · <code>{props.id}</code></span>
        <CopyButton text={props.id} variant="minimal" labels={{ copy: t().panel.copy, copied: t().panel.copied }} aria-label={`${t().panel.copy}: ${props.id}`} />
      </footer>
    </aside>
  )
}

function DataTab(props: { paths: CompletionPath[]; locale: EditorLocale }): JSX.Element {
  const t = () => strings(props.locale)
  return (
    <Show when={props.paths.length > 0} fallback={<p class="ae-panel__note">{t().panel.noOutput}</p>}>
      <h3 class="ae-panel__h3">{t().panel.output}</h3>
      <ul class="ae-paths">
        <For each={props.paths}>
          {(path) => (
            <li class="ae-paths__row">
              <code class="ae-paths__path" title={path.description}>{path.path}</code>
              <span class="ae-paths__type">{path.type}</span>
              <CopyButton text={path.path} variant="minimal" labels={{ copy: t().panel.copy, copied: t().panel.copied }} aria-label={`${t().panel.copy}: ${path.path}`} />
            </li>
          )}
        </For>
      </ul>
    </Show>
  )
}
