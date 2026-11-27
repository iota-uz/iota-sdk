// The Configure form of a node: one normal control per JSON Schema field. CEL and template params get
// a code field with client-side completion; the host can replace any top-level field through fieldSlot.
// JSON text appears only for a schema without a type (`{}`), labelled "any JSON".
import { Button, Combobox, Switch, ViewDrawer, type ComboboxOption } from '@iota-uz/sdk/solid-ui'
import { createEffect, createMemo, createSignal, createUniqueId, For, Index, Match, on, Show, Switch as Choose, type JSX } from 'solid-js'
import { completeAt, type CompletionPath } from './completion'
import { pick } from './locale'
import { offeredParticipants } from './graph'
import type { AutomationGraph, CatalogEvent, EditorApi, EditorLocale, ExpressionType, FieldSlot, JsonSchema } from './types'

const STRINGS: Record<EditorLocale, {
  anyJson: string; invalidJson: string; clear: string; add: string; remove: string; up: string; down: string
  openInPanel: string; connectionFromHost: string; item: (n: number) => string; seconds: string; minutes: string; hours: string; days: string
  durationAmount: string; durationUnit: string; empty: string; suggestions: string; cel: string; template: string; done: string; choose: string
  notFound: string; sourceList: string; sourceManual: string; sourceExpression: string; sourceGroups: { private: string; group: string }
  manualPlaceholder: string; manualHint: string; sourceModes: string; noList: string
}> = {
  en: {
    anyJson: 'any JSON', invalidJson: 'Not valid JSON: the value is kept as it was.', clear: 'Clear', add: 'Add', remove: 'Remove',
    up: 'Move up', down: 'Move down', openInPanel: 'Open in panel', connectionFromHost: 'The connection picker comes from the host page.',
    item: (n) => `Item ${n}`, seconds: 'seconds', minutes: 'minutes', hours: 'hours', days: 'days', durationAmount: 'Amount',
    durationUnit: 'Unit', empty: 'Nothing yet', suggestions: 'Suggestions', cel: 'CEL expression', template: 'Text with {{ }} placeholders',
    done: 'Done', choose: 'Choose…', notFound: 'Nothing matches.',
    sourceList: 'From chats', sourceManual: 'Id or @username', sourceExpression: 'Expression', sourceGroups: { private: 'Private', group: 'Groups' },
    manualPlaceholder: '-1001234567890 or @channel', manualHint: 'A chat id, or the @username of a public group or channel.', sourceModes: 'How the value is given', noList: 'The list is not available here: give an id or an expression.',
  },
  ru: {
    anyJson: 'любой JSON', invalidJson: 'Это не JSON: значение осталось прежним.', clear: 'Очистить', add: 'Добавить', remove: 'Удалить',
    up: 'Выше', down: 'Ниже', openInPanel: 'Открыть в панели', connectionFromHost: 'Выбор подключения предоставляет страница хоста.',
    item: (n) => `Элемент ${n}`, seconds: 'секунды', minutes: 'минуты', hours: 'часы', days: 'дни', durationAmount: 'Количество',
    durationUnit: 'Единица', empty: 'Пока пусто', suggestions: 'Подсказки', cel: 'Выражение CEL', template: 'Текст с подстановками {{ }}',
    done: 'Готово', choose: 'Выберите…', notFound: 'Ничего не подходит.',
    sourceList: 'Из чатов', sourceManual: 'Id или @username', sourceExpression: 'Выражение', sourceGroups: { private: 'Личные', group: 'Группы' },
    manualPlaceholder: '-1001234567890 или @channel', manualHint: 'Id чата или @username публичной группы или канала.', sourceModes: 'Как задано значение', noList: 'Список здесь недоступен: укажите id или выражение.',
  },
  uz: {
    anyJson: 'istalgan JSON', invalidJson: 'Bu JSON emas: qiymat oʻzgarmadi.', clear: 'Tozalash', add: 'Qoʻshish', remove: 'Oʻchirish',
    up: 'Yuqoriga', down: 'Pastga', openInPanel: 'Panelda ochish', connectionFromHost: 'Ulanishni tanlash xost sahifasidan keladi.',
    item: (n) => `Element ${n}`, seconds: 'soniya', minutes: 'daqiqa', hours: 'soat', days: 'kun', durationAmount: 'Miqdor',
    durationUnit: 'Birlik', empty: 'Hozircha boʻsh', suggestions: 'Takliflar', cel: 'CEL ifodasi', template: '{{ }} oʻrinbosarli matn',
    done: 'Tayyor', choose: 'Tanlang…', notFound: 'Hech narsa mos kelmadi.',
    sourceList: 'Chatlardan', sourceManual: 'Id yoki @username', sourceExpression: 'Ifoda', sourceGroups: { private: 'Shaxsiy', group: 'Guruhlar' },
    manualPlaceholder: '-1001234567890 yoki @channel', manualHint: 'Chat id yoki ochiq guruh yoki kanalning @username.', sourceModes: 'Qiymat qanday berilgan', noList: 'Roʻyxat bu yerda mavjud emas: id yoki ifoda kiriting.',
  },
  'uz-Cyrl': {
    anyJson: 'исталган JSON', invalidJson: 'Бу JSON эмас: қиймат ўзгармади.', clear: 'Тозалаш', add: 'Қўшиш', remove: 'Ўчириш',
    up: 'Юқорига', down: 'Пастга', openInPanel: 'Панелда очиш', connectionFromHost: 'Уланишни танлаш хост саҳифасидан келади.',
    item: (n) => `Элемент ${n}`, seconds: 'сония', minutes: 'дақиқа', hours: 'соат', days: 'кун', durationAmount: 'Миқдор',
    durationUnit: 'Бирлик', empty: 'Ҳозирча бўш', suggestions: 'Таклифлар', cel: 'CEL ифодаси', template: '{{ }} ўринбосарли матн',
    done: 'Тайёр', choose: 'Танланг…', notFound: 'Ҳеч нарса мос келмади.',
    sourceList: 'Чатлардан', sourceManual: 'Id ёки @username', sourceExpression: 'Ифода', sourceGroups: { private: 'Шахсий', group: 'Гуруҳлар' },
    manualPlaceholder: '-1001234567890 ёки @channel', manualHint: 'Чат id ёки очиқ гуруҳ ёки канал @username.', sourceModes: 'Қиймат қандай берилган', noList: 'Рўйхат бу ерда мавжуд эмас: id ёки ифода киритинг.',
  },
}

export type SchemaFormProps = {
  schema: JsonSchema
  value: Record<string, unknown>
  onChange(next: Record<string, unknown>): void
  readOnly: boolean
  locale: EditorLocale
  nodeType: string
  nodeId: string
  graph: AutomationGraph
  /** The catalog's events: a field marked `x-event-participants` is offered only where the trigger events that reach the node have participants. Left out, every field is shown. */
  events?: CatalogEvent[]
  fieldSlot?: FieldSlot
  /** Choices of a field with `x-options-source` (the host's `EditorApi.searchOptions`). */
  searchOptions?: EditorApi['searchOptions']
  expressions: { param: string; type: ExpressionType }[]
  templates: string[]
  completions: () => CompletionPath[]
  focusPath?: string | null
  errors?: { path: string; message: string }[]
}

export type FieldKind = 'source' | 'cel' | 'template' | 'duration' | 'enum' | 'boolean' | 'number' | 'string' | 'list' | 'object' | 'json' | 'connection'

const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === 'object' && value !== null && !Array.isArray(value)

export function schemaTypes(schema: JsonSchema): string[] {
  const raw = schema.type
  return (Array.isArray(raw) ? raw : raw === undefined ? [] : [raw]).filter((t): t is string => typeof t === 'string')
}

export function isNullable(schema: JsonSchema): boolean {
  return schemaTypes(schema).includes('null') || (Array.isArray(schema.enum) && schema.enum.includes(null))
}

/** The control a field gets. `path` is the field's path from the params' root; `name` is its top-level param. */
export function fieldKind(schema: JsonSchema, path: string, ctx: { expressions: string[]; templates: string[] }): FieldKind {
  if (path === 'connection') return 'connection'
  if (typeof schema['x-options-source'] === 'string') return 'source'
  if (ctx.expressions.includes(path)) return 'cel'
  if (ctx.templates.includes(path)) return 'template'
  if ('x-max-ms' in schema) return 'duration'
  if (Array.isArray(schema.enum)) return 'enum'
  const types = schemaTypes(schema).filter((t) => t !== 'null')
  const type = types[0]
  if (!type) return schema.properties ? 'object' : schema.items ? 'list' : 'json'
  switch (type) {
    case 'boolean': return 'boolean'
    case 'integer':
    case 'number': return 'number'
    case 'string': return 'string'
    case 'array': return 'list'
    case 'object': return schema.properties ? 'object' : 'json'
    default: return 'json'
  }
}

/** `missed_after` → "Missed after". */
export function humanize(name: string): string {
  const words = name.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[_\-.]+/g, ' ').trim().toLowerCase()
  return words.charAt(0).toUpperCase() + words.slice(1)
}

/** A blank value for a new list item or a cleared required field. */
export function emptyValue(schema: JsonSchema): unknown {
  if ('default' in schema) return structuredClone(schema.default)
  switch (fieldKind(schema, '', { expressions: [], templates: [] })) {
    case 'boolean': return false
    case 'number': return 0
    case 'list': return []
    case 'object': {
      const out: Record<string, unknown> = {}
      for (const [k, s] of Object.entries((schema.properties ?? {}) as Record<string, JsonSchema>)) {
        if ('default' in s) out[k] = structuredClone(s.default)
      }
      return out
    }
    case 'enum': return (schema.enum as unknown[])[0] ?? null
    default: return ''
  }
}

const DURATION = /^([1-9][0-9]{0,5})(s|m|h|d)$/
type Unit = 's' | 'm' | 'h' | 'd'

export function parseDuration(value: unknown): { amount: string; unit: Unit } {
  if (typeof value === 'number' && Number.isFinite(value)) {
    const ms = value
    for (const [unit, size] of [['d', 86_400_000], ['h', 3_600_000], ['m', 60_000]] as const) if (ms % size === 0 && ms >= size) return { amount: String(ms / size), unit }
    return { amount: String(Math.max(1, Math.round(ms / 1000))), unit: 's' }
  }
  const match = typeof value === 'string' ? DURATION.exec(value) : null
  return match ? { amount: match[1]!, unit: match[2] as Unit } : { amount: '', unit: 'm' }
}

function joinPath(base: string, key: string | number): string {
  if (typeof key === 'number') return `${base}[${key}]`
  return base ? `${base}.${key}` : key
}

/** Sets or removes a key (undefined removes). */
function withKey(obj: Record<string, unknown>, key: string, value: unknown): Record<string, unknown> {
  const next = { ...obj }
  if (value === undefined) delete next[key]
  else next[key] = value
  return next
}

/**
 * A field the catalog marks `x-event-participants` (the participants of a notify node) is hidden while no
 * trigger event that reaches the node offers any, unless it already holds a value: that one stays visible so
 * the author can clear what the server would refuse.
 */
export function fieldHidden(props: Pick<SchemaFormProps, 'events' | 'graph' | 'nodeId'>, schema: JsonSchema, value: unknown): boolean {
  if (schema['x-event-participants'] !== true || !props.events) return false
  if (Array.isArray(value) && value.length > 0) return false
  return offeredParticipants(props.graph, { events: props.events }, props.nodeId).length === 0
}

type Ctx = {
  props: SchemaFormProps
  s: () => (typeof STRINGS)['en']
  errorAt(path: string): string | undefined
}

export function SchemaForm(props: SchemaFormProps) {
  let root!: HTMLDivElement
  const s = () => pick(STRINGS, props.locale)
  const ctx: Ctx = {
    props,
    s,
    errorAt: (path) => props.errors?.find((e) => e.path === path)?.message,
  }
  const entries = createMemo(() => Object.entries((props.schema.properties ?? {}) as Record<string, JsonSchema>))
  const required = createMemo(() => new Set(Array.isArray(props.schema.required) ? (props.schema.required as string[]) : []))
  createEffect(on(() => props.focusPath, (path) => {
    if (!path) return
    queueMicrotask(() => {
      const wrapper = root?.querySelector<HTMLElement>(`[data-param="${CSS.escape(path)}"]`)
      if (!wrapper) return
      wrapper.scrollIntoView?.({ block: 'center' })
      wrapper.querySelector<HTMLElement>('input:not([type=hidden]):not(.hidden),textarea,button,[tabindex="0"]')?.focus()
    })
  }))
  return (
    <div class="ae-form" ref={root}>
      <Show when={entries().length > 0} fallback={<Show when={Object.keys(props.schema).length === 0 || !props.schema.properties}>
        <Show when={fieldKind(props.schema, '', { expressions: [], templates: [] }) === 'json'}>
          <JsonField ctx={ctx} path="" schema={props.schema} value={props.value} onChange={(v) => props.onChange(isRecord(v) ? v : {})} label={<span />} />
        </Show>
      </Show>}>
        <For each={entries()}>{([name, schema]) => {
          const onChange = (v: unknown) => props.onChange(withKey(props.value, name, v))
          const slot = createMemo(() => props.fieldSlot?.({
            nodeType: props.nodeType, nodeId: props.nodeId, param: name, value: props.value[name], onChange,
            params: props.value, onParamsChange: props.onChange, readOnly: props.readOnly, graph: props.graph,
          }))
          return (
            <Show when={!fieldHidden(props, schema, props.value[name])}>
              <Show when={slot() === undefined} fallback={<div class="ae-form-field" data-param={name}>{slot()}<FieldError ctx={ctx} path={name} /></div>}>
                <FieldView ctx={ctx} name={name} path={name} schema={schema} required={required().has(name)} value={props.value[name]} onChange={onChange} />
              </Show>
            </Show>
          )
        }}</For>
      </Show>
    </div>
  )
}

function FieldError(props: { ctx: Ctx; path: string }) {
  return <Show when={props.ctx.errorAt(props.path)}>{(message) => <p class="ae-form-error" role="alert">{message()}</p>}</Show>
}

function FieldLabel(props: { name: string; schema: JsonSchema; for?: string; required?: boolean; id?: string }) {
  const title = () => (typeof props.schema.title === 'string' && props.schema.title) || humanize(props.name)
  return (
    <span class="ae-form-label-row">
      <Show when={props.for} fallback={<span class="ae-form-label" id={props.id}>{title()}</span>}>
        <label class="ae-form-label" for={props.for} id={props.id}>{title()}</label>
      </Show>
      <Show when={props.required}><span class="ae-form-required" aria-hidden="true">*</span></Show>
      <Show when={/^[A-Za-z_][A-Za-z0-9_]*$/.test(props.name)}><code class="ae-form-param">{props.name}</code></Show>
    </span>
  )
}

function Description(props: { schema: JsonSchema; id: string }) {
  return <Show when={typeof props.schema.description === 'string' && props.schema.description}><p class="ae-form-description" id={props.id}>{props.schema.description as string}</p></Show>
}

type FieldViewProps = { ctx: Ctx; name: string; path: string; schema: JsonSchema; required: boolean; value: unknown; onChange(v: unknown): void }

function FieldView(props: FieldViewProps) {
  const kind = createMemo(() => fieldKind(props.schema, props.path, {
    expressions: props.ctx.props.expressions.map((e) => e.param),
    templates: props.ctx.props.templates,
  }))
  const id = createUniqueId()
  const descId = `${id}-desc`
  const readOnly = () => props.ctx.props.readOnly
  // An optional field cleared to empty is left out (or null when the schema allows null).
  const clearedValue = () => (isNullable(props.schema) ? null : props.required ? '' : undefined)
  const label = (forId?: string) => <FieldLabel name={props.name} schema={props.schema} for={forId} required={props.required} id={`${id}-label`} />
  return (
    <div class="ae-form-field" data-param={props.path} data-kind={kind()}>
      <Choose>
        <Match when={kind() === 'connection'}>
          {label()}
          <p class="ae-form-note">{props.ctx.s().connectionFromHost}</p>
        </Match>
        <Match when={kind() === 'source'}>
          <SourceField ctx={props.ctx} id={id} label={label()} title={(typeof props.schema.title === 'string' && props.schema.title) || humanize(props.name)}
            source={props.schema['x-options-source'] as string} value={props.value} onChange={props.onChange} readOnly={readOnly()} describedBy={descId} />
        </Match>
        <Match when={kind() === 'cel' || kind() === 'template'}>
          <CodeField
            ctx={props.ctx} id={id} label={label(id)} title={(typeof props.schema.title === 'string' && props.schema.title) || humanize(props.name)}
            mode={kind() === 'cel' ? 'cel' : 'template'} value={typeof props.value === 'string' ? props.value : ''}
            onChange={(v) => props.onChange(v === '' ? clearedValue() : v)} readOnly={readOnly()} describedBy={descId}
          />
        </Match>
        <Match when={kind() === 'duration'}>
          <DurationField ctx={props.ctx} id={id} label={label()} value={props.value} onChange={props.onChange} readOnly={readOnly()} cleared={clearedValue()} />
        </Match>
        <Match when={kind() === 'enum'}>
          <EnumField ctx={props.ctx} id={id} label={label()} schema={props.schema} value={props.value} onChange={props.onChange} readOnly={readOnly()} />
        </Match>
        <Match when={kind() === 'boolean'}>
          <div class="ae-form-switch">
            <Switch id={id} checked={props.value === true} disabled={readOnly()} onChange={(e) => props.onChange(e.currentTarget.checked)} aria-labelledby={`${id}-label`} aria-describedby={descId} />
            {label()}
          </div>
        </Match>
        <Match when={kind() === 'number'}>
          {label(id)}
          <input
            id={id} type="number" class="form-control form-control-input ae-form-input" readOnly={readOnly()} aria-describedby={descId}
            min={typeof props.schema.minimum === 'number' ? props.schema.minimum : undefined}
            max={typeof props.schema.maximum === 'number' ? props.schema.maximum : undefined}
            step={schemaTypes(props.schema).includes('integer') ? 1 : 'any'}
            value={typeof props.value === 'number' ? String(props.value) : ''}
            onInput={(e) => {
              const raw = e.currentTarget.value
              if (raw === '') return props.onChange(clearedValue() === '' ? undefined : clearedValue())
              const n = Number(raw)
              if (Number.isFinite(n)) props.onChange(schemaTypes(props.schema).includes('integer') ? Math.trunc(n) : n)
            }}
          />
        </Match>
        <Match when={kind() === 'string'}>
          <StringField ctx={props.ctx} id={id} label={label(id)} title={(typeof props.schema.title === 'string' && props.schema.title) || humanize(props.name)} schema={props.schema} value={props.value} onChange={(v) => props.onChange(v === '' ? clearedValue() : v)} readOnly={readOnly()} describedBy={descId} />
        </Match>
        <Match when={kind() === 'list'}>
          <ListField ctx={props.ctx} label={label()} schema={props.schema} path={props.path} value={props.value} onChange={props.onChange} readOnly={readOnly()} />
        </Match>
        <Match when={kind() === 'object'}>
          <ObjectField ctx={props.ctx} label={label()} schema={props.schema} path={props.path} value={props.value} onChange={props.onChange} readOnly={readOnly()} nullable={isNullable(props.schema) || !props.required} />
        </Match>
        <Match when={kind() === 'json'}>
          <JsonField ctx={props.ctx} path={props.path} schema={props.schema} value={props.value} onChange={props.onChange} label={label(id)} id={id} />
        </Match>
      </Choose>
      <Description schema={props.schema} id={descId} />
      <FieldError ctx={props.ctx} path={props.path} />
    </div>
  )
}

function StringField(props: { ctx: Ctx; id: string; label: JSX.Element; title: string; schema: JsonSchema; value: unknown; onChange(v: string): void; readOnly: boolean; describedBy: string }) {
  const long = () => (typeof props.schema.maxLength === 'number' ? props.schema.maxLength > 200 : true) && props.schema.format !== 'date-time' && !props.schema.pattern
  const [drawer, setDrawer] = createSignal(false)
  const text = () => (typeof props.value === 'string' ? props.value : '')
  return (
    <>
      <div class="ae-form-label-line">
        {props.label}
        <Show when={long()}>
          <button type="button" class="ae-form-link" onClick={() => setDrawer(true)}>{props.ctx.s().openInPanel}</button>
        </Show>
      </div>
      <Show when={long()} fallback={
        <input id={props.id} type="text" class="form-control form-control-input ae-form-input" readOnly={props.readOnly} aria-describedby={props.describedBy}
          maxLength={typeof props.schema.maxLength === 'number' ? props.schema.maxLength : undefined}
          value={text()} onInput={(e) => props.onChange(e.currentTarget.value)} />
      }>
        <textarea id={props.id} rows={2} class="form-control form-control-input ae-form-textarea" readOnly={props.readOnly} aria-describedby={props.describedBy}
          maxLength={typeof props.schema.maxLength === 'number' ? props.schema.maxLength : undefined}
          value={text()} onInput={(e) => props.onChange(e.currentTarget.value)} />
        <ViewDrawer open={drawer()} onOpenChange={setDrawer} title={props.title} closeLabel={props.ctx.s().done}>
          <textarea class="form-control form-control-input ae-form-textarea ae-form-textarea-large" readOnly={props.readOnly} aria-label={props.title}
            value={text()} onInput={(e) => props.onChange(e.currentTarget.value)} />
        </ViewDrawer>
      </Show>
    </>
  )
}

/** A monospace editor for CEL or `{{ }}` templates with a completion listbox. */
export function CodeField(props: {
  ctx: Ctx; id: string; label: JSX.Element; title: string; mode: 'cel' | 'template'; value: string
  onChange(v: string): void; readOnly: boolean; describedBy?: string
}) {
  const [drawer, setDrawer] = createSignal(false)
  return (
    <>
      <div class="ae-form-label-line">
        {props.label}
        <span class="ae-form-badge">{props.mode === 'cel' ? 'CEL' : '{{ }}'}</span>
        <button type="button" class="ae-form-link" onClick={() => setDrawer(true)}>{props.ctx.s().openInPanel}</button>
      </div>
      <CodeArea ctx={props.ctx} id={props.id} mode={props.mode} value={props.value} onChange={props.onChange} readOnly={props.readOnly} describedBy={props.describedBy} label={props.title} rows={props.mode === 'cel' ? 2 : 3} />
      <ViewDrawer open={drawer()} onOpenChange={setDrawer} title={props.title} closeLabel={props.ctx.s().done}>
        <p class="ae-form-description">{props.mode === 'cel' ? props.ctx.s().cel : props.ctx.s().template}</p>
        <CodeArea ctx={props.ctx} id={`${props.id}-drawer`} mode={props.mode} value={props.value} onChange={props.onChange} readOnly={props.readOnly} label={props.title} rows={16} />
      </ViewDrawer>
    </>
  )
}

function CodeArea(props: { ctx: Ctx; id: string; mode: 'cel' | 'template'; value: string; onChange(v: string): void; readOnly: boolean; describedBy?: string; label: string; rows: number }) {
  let area!: HTMLTextAreaElement
  const [state, setState] = createSignal<{ from: number; to: number; items: CompletionPath[] } | null>(null)
  const [active, setActive] = createSignal(0)
  const listId = `${props.id}-completions`
  const refresh = () => {
    if (props.readOnly) return setState(null)
    const caret = area.selectionStart ?? area.value.length
    const result = completeAt(area.value, caret, props.mode, props.ctx.props.completions())
    const typed = area.value.slice(result.from, caret)
    const show = result.items.length > 0 && (typed.length > 0 || (props.mode === 'template' && area.value.slice(0, caret).trimEnd().endsWith('{{')))
    setState(show ? result : null)
    setActive(0)
  }
  const apply = (item: CompletionPath) => {
    const current = state()
    if (!current) return
    const next = area.value.slice(0, current.from) + item.path + area.value.slice(current.to)
    props.onChange(next)
    area.value = next
    const caret = current.from + item.path.length
    area.setSelectionRange(caret, caret)
    setState(null)
    area.focus()
  }
  const onKeyDown = (event: KeyboardEvent) => {
    const current = state()
    if (!current) return
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const n = current.items.length
      setActive((i) => (i + (event.key === 'ArrowDown' ? 1 : -1) + n) % n)
    } else if (event.key === 'Enter' || event.key === 'Tab') {
      const item = current.items[active()]
      if (item) {
        event.preventDefault()
        apply(item)
      }
    } else if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      setState(null)
    }
  }
  return (
    <div class="ae-form-code">
      <textarea
        ref={area} id={props.id} rows={props.rows} spellcheck={false} autocomplete="off"
        class="form-control form-control-input ae-form-textarea ae-form-mono" readOnly={props.readOnly}
        aria-describedby={props.describedBy} aria-label={props.label}
        role="combobox" aria-autocomplete="list" aria-expanded={state() !== null} aria-controls={listId}
        aria-activedescendant={state() ? `${listId}-${active()}` : undefined}
        value={props.value}
        onInput={(e) => { props.onChange(e.currentTarget.value); refresh() }}
        onKeyDown={onKeyDown}
        onBlur={() => setTimeout(() => setState(null), 120)}
      />
      <Show when={state()}>{(current) => (
        <ul class="ae-form-completions" role="listbox" id={listId} aria-label={props.ctx.s().suggestions}>
          <Index each={current().items}>{(item, index) => (
            <li
              id={`${listId}-${index}`} role="option" aria-selected={active() === index}
              class="ae-form-completion" classList={{ 'ae-form-completion-active': active() === index }}
              onPointerDown={(e) => { e.preventDefault(); apply(item()) }}
            >
              <span class="ae-form-mono">{item().path}</span>
              <span class="ae-form-completion-type">{item().type}</span>
            </li>
          )}</Index>
        </ul>
      )}</Show>
    </div>
  )
}

function DurationField(props: { ctx: Ctx; id: string; label: JSX.Element; value: unknown; onChange(v: unknown): void; readOnly: boolean; cleared: unknown }) {
  const parsed = createMemo(() => parseDuration(props.value))
  const [unit, setUnit] = createSignal<Unit>(parsed().unit)
  createEffect(on(parsed, (p) => { if (p.amount) setUnit(p.unit) }))
  const emit = (amount: string, u: Unit) => {
    if (amount === '') return props.onChange(props.cleared === '' ? undefined : props.cleared)
    const n = Math.trunc(Number(amount))
    if (Number.isFinite(n) && n > 0) props.onChange(`${n}${u}`)
  }
  const units = (): [Unit, string][] => [['s', props.ctx.s().seconds], ['m', props.ctx.s().minutes], ['h', props.ctx.s().hours], ['d', props.ctx.s().days]]
  return (
    <>
      {props.label}
      <div class="ae-form-duration" role="group" aria-labelledby={`${props.id}-label`}>
        <input id={props.id} type="number" min="1" step="1" class="form-control form-control-input ae-form-input ae-form-duration-amount"
          aria-label={props.ctx.s().durationAmount} readOnly={props.readOnly} value={parsed().amount}
          onInput={(e) => emit(e.currentTarget.value, unit())} />
        {/* Four units: a native select is allowed (K5 applies above five options). */}
        <select class="form-control form-control-input ae-form-input ae-form-duration-unit" aria-label={props.ctx.s().durationUnit}
          disabled={props.readOnly} value={unit()}
          onChange={(e) => { const u = e.currentTarget.value as Unit; setUnit(u); emit(parsed().amount, u) }}>
          <For each={units()}>{([value, label]) => <option value={value}>{label}</option>}</For>
        </select>
      </div>
    </>
  )
}

function EnumField(props: { ctx: Ctx; id: string; label: JSX.Element; schema: JsonSchema; value: unknown; onChange(v: unknown): void; readOnly: boolean }) {
  const values = () => (props.schema.enum as unknown[]).filter((v) => v !== null)
  const names = () => (Array.isArray(props.schema['x-enum-titles']) ? (props.schema['x-enum-titles'] as string[]) : [])
  // `x-ref: "event"`: the values are event keys; they read as the catalog's titles, grouped by module.
  const events = () => (props.schema['x-ref'] === 'event' ? new Map((props.ctx.props.events ?? []).map((e) => [e.key, e])) : null)
  const labelOf = (v: unknown, i: number) => names()[i] ?? (typeof v === 'string' ? events()?.get(v)?.title : undefined) ?? (v === '' ? '—' : String(v))
  const options = () => {
    const known = events()
    const plain = values().map((v, i) => ({ value: String(i), label: labelOf(v, i) }))
    if (!known) return plain
    const groups = new Map<string, typeof plain>()
    values().forEach((v, i) => {
      const group = (typeof v === 'string' && known.get(v)?.group) || ''
      const list = groups.get(group) ?? []
      list.push(plain[i]!)
      groups.set(group, list)
    })
    return [...groups].flatMap(([group, list]) => (group ? [{ value: `group:${group}`, label: group, groupHeader: true }, ...list] : list))
  }
  const index = () => values().findIndex((v) => JSON.stringify(v) === JSON.stringify(props.value))
  return (
    <>
      {props.label}
      <Show when={values().length > 5} fallback={
        <div class="ae-form-segmented" role="radiogroup" aria-labelledby={`${props.id}-label`}>
          <For each={values()}>{(v, i) => (
            <button type="button" role="radio" aria-checked={index() === i()} disabled={props.readOnly}
              class="ae-form-segment" classList={{ 'ae-form-segment-active': index() === i() }}
              onClick={() => props.onChange(v)}>{labelOf(v, i())}</button>
          )}</For>
        </div>
      }>
        <Combobox
          id={props.id} aria-labelledby={`${props.id}-label`} searchable readOnly={props.readOnly} placeholder={props.ctx.s().choose}
          labels={{ notFound: props.ctx.s().notFound }}
          options={options()}
          value={index() >= 0 ? String(index()) : ''}
          onValueChange={(v) => { const i = Number(Array.isArray(v) ? v[0] : v); if (Number.isInteger(i) && values()[i] !== undefined) props.onChange(values()[i]) }}
        />
      </Show>
    </>
  )
}

function ListField(props: { ctx: Ctx; label: JSX.Element; schema: JsonSchema; path: string; value: unknown; onChange(v: unknown): void; readOnly: boolean }) {
  const items = () => (Array.isArray(props.value) ? props.value : [])
  const itemSchema = () => (isRecord(props.schema.items) ? (props.schema.items as JsonSchema) : {})
  const set = (next: unknown[]) => props.onChange(next)
  const move = (i: number, d: number) => {
    const next = [...items()]
    const j = i + d
    if (j < 0 || j >= next.length) return
    ;[next[i], next[j]] = [next[j], next[i]]
    set(next)
  }
  return (
    <fieldset class="ae-form-group">
      <legend class="ae-form-legend">{props.label}</legend>
      <Show when={items().length > 0} fallback={<p class="ae-form-note">{props.ctx.s().empty}</p>}>
        <ol class="ae-form-list">
          <Index each={items()}>{(item, i) => (
            <li class="ae-form-list-item">
              <div class="ae-form-list-body">
                <FieldView ctx={props.ctx} name={props.ctx.s().item(i + 1)} path={joinPath(props.path, i)} schema={itemSchema()} required
                  value={item()} onChange={(v) => { const next = [...items()]; next[i] = v === undefined ? emptyValue(itemSchema()) : v; set(next) }} />
              </div>
              <Show when={!props.readOnly}>
                <div class="ae-form-list-actions">
                  <button type="button" class="ae-form-icon" aria-label={`${props.ctx.s().up}: ${props.ctx.s().item(i + 1)}`} disabled={i === 0} onClick={() => move(i, -1)}>↑</button>
                  <button type="button" class="ae-form-icon" aria-label={`${props.ctx.s().down}: ${props.ctx.s().item(i + 1)}`} disabled={i === items().length - 1} onClick={() => move(i, 1)}>↓</button>
                  <button type="button" class="ae-form-icon ae-form-icon-danger" aria-label={`${props.ctx.s().remove}: ${props.ctx.s().item(i + 1)}`} onClick={() => set(items().filter((_, j) => j !== i))}>×</button>
                </div>
              </Show>
            </li>
          )}</Index>
        </ol>
      </Show>
      <Show when={!props.readOnly}>
        <Button type="button" variant="secondary" size="sm" onClick={() => set([...items(), emptyValue(itemSchema())])}>{props.ctx.s().add}</Button>
      </Show>
    </fieldset>
  )
}

function ObjectField(props: { ctx: Ctx; label: JSX.Element; schema: JsonSchema; path: string; value: unknown; onChange(v: unknown): void; readOnly: boolean; nullable: boolean }) {
  const obj = () => (isRecord(props.value) ? props.value : {})
  const entries = () => Object.entries((props.schema.properties ?? {}) as Record<string, JsonSchema>)
  const required = () => new Set(Array.isArray(props.schema.required) ? (props.schema.required as string[]) : [])
  const hasValue = () => isRecord(props.value) && Object.keys(props.value).length > 0
  return (
    <fieldset class="ae-form-group">
      <legend class="ae-form-legend">{props.label}</legend>
      <For each={entries()}>{([name, schema]) => (
        <Show when={!fieldHidden(props.ctx.props, schema, obj()[name])}>
          <FieldView ctx={props.ctx} name={name} path={joinPath(props.path, name)} schema={schema} required={required().has(name)}
            value={obj()[name]} onChange={(v) => props.onChange(withKey(obj(), name, v))} />
        </Show>
      )}</For>
      <Show when={props.nullable && !props.readOnly && hasValue()}>
        <Button type="button" variant="ghost" size="sm" onClick={() => props.onChange(isNullable(props.schema) ? null : undefined)}>{props.ctx.s().clear}</Button>
      </Show>
    </fieldset>
  )
}

function JsonField(props: { ctx: Ctx; path: string; schema: JsonSchema; value: unknown; onChange(v: unknown): void; label: JSX.Element; id?: string }) {
  const id = props.id ?? createUniqueId()
  const [text, setText] = createSignal(props.value === undefined ? '' : JSON.stringify(props.value, null, 2))
  const [invalid, setInvalid] = createSignal(false)
  createEffect(on(() => props.value, (v) => {
    let current: unknown
    try { current = text().trim() === '' ? undefined : JSON.parse(text()) } catch { return }
    if (JSON.stringify(current) !== JSON.stringify(v)) setText(v === undefined ? '' : JSON.stringify(v, null, 2))
  }, { defer: true }))
  return (
    <>
      <div class="ae-form-label-line">
        {props.label}
        <span class="ae-form-badge">{props.ctx.s().anyJson}</span>
      </div>
      <textarea id={id} rows={4} spellcheck={false} class="form-control form-control-input ae-form-textarea ae-form-mono"
        aria-label={props.ctx.s().anyJson} aria-invalid={invalid() || undefined} readOnly={props.ctx.props.readOnly}
        value={text()}
        onInput={(e) => {
          const raw = e.currentTarget.value
          setText(raw)
          if (raw.trim() === '') { setInvalid(false); return props.onChange(undefined) }
          try { props.onChange(JSON.parse(raw)); setInvalid(false) } catch { setInvalid(true) }
        }} />
      <Show when={invalid()}><p class="ae-form-error" role="alert">{props.ctx.s().invalidJson}</p></Show>
    </>
  )
}

type SourceMode = 'list' | 'manual' | 'expr'

const QUOTED_USERNAME = /^"(@[A-Za-z][A-Za-z0-9_]{4,31})"$/
const CHAT_ID = /^-?[0-9]{1,18}$/
/** A plain string literal, the form in which the option of a source that is not a chat is stored. */
const OPTION_LITERAL = /^"(?:[^"\\]|\\.)*"$/

/** How a stored value reads: a number is an id (picked or typed), `"@name"` a typed username, any other text an expression. */
export function sourceModeOf(value: unknown): SourceMode {
  if (typeof value === 'string' && value !== '') {
    if (QUOTED_USERNAME.test(value)) return 'manual'
    return OPTION_LITERAL.test(value) ? 'list' : 'expr'
  }
  return 'list'
}

/** The option a stored value stands for in the picker: a chat id, or the declared value of another source's option. */
export function optionOf(value: unknown): string {
  if (typeof value === 'number') return String(value)
  if (typeof value !== 'string' || QUOTED_USERNAME.test(value) || !OPTION_LITERAL.test(value)) return ''
  try {
    const decoded: unknown = JSON.parse(value) // the literal is a JSON string: escapes are decoded
    return typeof decoded === 'string' ? decoded : ''
  } catch {
    return ''
  }
}

/** The value a Combobox selection means: a chat id, undefined when the selection was removed, null when it is not a chat. */
export function chatOfSelection(v: string | readonly string[] | undefined): number | undefined | null {
  const raw = Array.isArray(v) ? v[0] : v
  if (raw === undefined || raw === '') return undefined
  return typeof raw === 'string' && CHAT_ID.test(raw) ? Number(raw) : null
}

/** The value of a Combobox selection that is not a chat id: the option's own value as a CEL string literal (the
 *  way a typed username is stored), undefined for a group header, which is no choice. */
export function literalOfSelection(v: string | readonly string[] | undefined): string | undefined {
  const raw = Array.isArray(v) ? v[0] : v
  if (typeof raw !== 'string' || raw === '' || raw.startsWith('group:')) return undefined
  return JSON.stringify(raw)
}

/** The source whose options are Telegram chats: their values are numeric ids. */
export const CHAT_SOURCE = 'telegram.chats'

/**
 * What a Combobox selection stores for a source: undefined removes the value; a chat id for the chat source; the
 * option's own value, as a string literal that keeps its form ("00123" stays "00123"), for any other source. null:
 * nothing is chosen (a group header, or a value the source cannot hold).
 */
export function selectionOf(source: string, v: string | readonly string[] | undefined): { value: unknown } | null {
  const raw = Array.isArray(v) ? v[0] : v
  if (raw === undefined || raw === '') return { value: undefined }
  if (typeof raw === 'string' && raw.startsWith('group:')) return null
  if (source === CHAT_SOURCE) {
    const chat = chatOfSelection(raw)
    return chat === null ? null : { value: chat }
  }
  const literal = literalOfSelection(raw)
  return literal === undefined ? null : { value: literal }
}

/** What the "id or @username" input stores: a number for an id, a CEL string literal for a username. */
export function manualValue(text: string): number | string | undefined {
  const t = text.trim()
  if (t === '') return undefined
  if (CHAT_ID.test(t) && Number.isSafeInteger(Number(t))) return Number(t)
  return JSON.stringify(t)
}

/** What the "id or @username" input shows for a stored value. */
export function manualText(value: unknown): string {
  if (typeof value === 'number') return String(value)
  const m = typeof value === 'string' ? QUOTED_USERNAME.exec(value) : null
  return m ? m[1]! : typeof value === 'string' ? value.replace(/^"|"$/g, '') : ''
}

/** A field whose value is a chat (or another choice of the host's source): picked from the known ones,
 *  typed as an id or @username, or given as an expression. */
function SourceField(props: {
  ctx: Ctx; id: string; label: JSX.Element; title: string; source: string; value: unknown
  onChange(v: unknown): void; readOnly: boolean; describedBy: string
}) {
  const st = () => props.ctx.s()
  const [mode, setMode] = createSignal<SourceMode>(sourceModeOf(props.value))
  // The value this field wrote last. A value that comes back equal to it is the field's own edit (a half-typed
  // "-" or "@" reads as an expression, but it is still the id or username being typed); any other value is
  // from outside (undo, redo, another edit), and when it no longer fits the mode on screen the mode moves to
  // the one it reads as.
  let wrote: { value: unknown } | undefined
  const emit = (value: unknown) => { wrote = { value }; props.onChange(value) }
  createEffect(on(() => props.value, (value) => {
    if (wrote && wrote.value === value) return
    wrote = undefined
    const fits = mode() === 'expr' ? (value === undefined || typeof value === 'string')
      : mode() === 'manual' ? (value === undefined || typeof value === 'number' || sourceModeOf(value) === 'manual')
        : (value === undefined || typeof value === 'number' || sourceModeOf(value) === 'list')
    if (!fits) setMode(sourceModeOf(value))
  }, { defer: true }))
  const modes = (): [SourceMode, string][] => [['list', st().sourceList], ['manual', st().sourceManual], ['expr', st().sourceExpression]]
  const search = props.ctx.props.searchOptions
  const groupLabel = (g: string | undefined) => (g === 'private' ? st().sourceGroups.private : g === 'group' ? st().sourceGroups.group : g ?? '')
  const load = async (query: string): Promise<ComboboxOption[]> => {
    if (!search) return []
    const found = await search(props.source, query, { nodeType: props.ctx.props.nodeType, params: props.ctx.props.value })
    const out: ComboboxOption[] = []
    let last: string | undefined
    for (const o of found) {
      if (o.group !== last) { out.push({ value: `group:${o.group ?? ''}`, label: groupLabel(o.group), groupHeader: true }); last = o.group }
      out.push({ value: o.value, label: o.label })
    }
    return out
  }
  const current = () => optionOf(props.value)
  // The stored id is shown even before the list loads (or when the chat is not among the known ones).
  const seed = (): ComboboxOption[] => (current() ? [{ value: current(), label: current() }] : [])
  // The known chats are offered as the list opens; typing narrows them through the host.
  const [known, setKnown] = createSignal<ComboboxOption[] | undefined>()
  // The chats are those of the step's connection: another connection reloads them.
  const connection = () => { const c = (props.ctx.props.value as Record<string, unknown>)?.connection; return typeof c === 'string' ? c : '' }
  createEffect(on([mode, connection], ([m]) => {
    if (m !== 'list' || !search) return
    setKnown(undefined)
    const asked = connection()
    void load('').then((o) => { if (asked === connection()) setKnown(o) }).catch(() => { if (asked === connection()) setKnown([]) })
  }))
  const offered = (): ComboboxOption[] => {
    const list = known() ?? []
    return current() && !list.some((o) => o.value === current()) ? [...seed(), ...list] : list.length ? list : seed()
  }
  return (
    <>
      {props.label}
      <div class="ae-form-segmented" role="radiogroup" aria-label={st().sourceModes}>
        <For each={modes()}>{([m, text]) => (
          <button type="button" role="radio" aria-checked={mode() === m} disabled={props.readOnly}
            class="ae-form-segment" classList={{ 'ae-form-segment-active': mode() === m }} onClick={() => setMode(m)}>{text}</button>
        )}</For>
      </div>
      <Choose>
        <Match when={mode() === 'list'}>
          <Show when={search} fallback={<p class="ae-form-note">{st().noList}</p>}>
            {/* Keyed by the connection: another connection is another picker, with none of the earlier one's
                remembered search results and none of its searches still on the way. */}
            <For each={[connection()]}>{() => (
              <Combobox
                id={props.id} label={<span class="ae-sr-only">{props.title}</span>} searchable readOnly={props.readOnly} placeholder={st().choose}
                labels={{ notFound: st().notFound }} options={offered()} value={current()}
                loadOptions={(query) => load(query)}
                onValueChange={(v) => {
                  // The chat source stores a number; any other source keeps the value its option declared.
                  const chosen = selectionOf(props.source, v)
                  if (chosen) emit(chosen.value)
                }}
              />
            )}</For>
          </Show>
        </Match>
        <Match when={mode() === 'manual'}>
          <input id={props.id} type="text" class="form-control form-control-input ae-form-input" readOnly={props.readOnly} autocomplete="off" spellcheck={false}
            placeholder={st().manualPlaceholder} aria-describedby={props.describedBy} aria-label={props.title}
            value={manualText(props.value)} onInput={(e) => emit(manualValue(e.currentTarget.value))} />
          <p class="ae-form-note">{st().manualHint}</p>
        </Match>
        <Match when={mode() === 'expr'}>
          <CodeField ctx={props.ctx} id={props.id} label={<span />} title={props.title} mode="cel"
            value={typeof props.value === 'string' ? props.value : ''} onChange={(v) => emit(v === '' ? undefined : v)}
            readOnly={props.readOnly} describedBy={props.describedBy} />
        </Match>
      </Choose>
    </>
  )
}
