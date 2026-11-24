// Values of a run as text or a JSON tree. Nothing is interpreted as HTML: every value is a text node.
// Output above 256 KB is folded until «Load in full»; «Copy» is announced through aria-live.
import { createMemo, createSignal, For, Match, onCleanup, Show, Switch, type JSX } from 'solid-js'
import { Icon } from '../icons'
import type { EditorLocale } from '../types'
import { byteSize, OUTPUT_FOLD_BYTES } from './model'
import { runStrings } from './strings'
import { formatBytes, jsonText } from './util'

const OPEN_DEPTH = 2
const MAX_CHILDREN = 200

function Leaf(props: { value: unknown }): JSX.Element {
  const v = () => props.value
  return (
    <Switch fallback={<span class="ae-json__str">{String(v())}</span>}>
      <Match when={v() === null}><span class="ae-json__null">null</span></Match>
      <Match when={typeof v() === 'string'}><span class="ae-json__str">"{v() as string}"</span></Match>
      <Match when={typeof v() === 'number'}><span class="ae-json__num">{String(v())}</span></Match>
      <Match when={typeof v() === 'boolean'}><span class="ae-json__bool">{String(v())}</span></Match>
    </Switch>
  )
}

function Node(props: { name?: string; value: unknown; depth: number; all: boolean; locale: EditorLocale }): JSX.Element {
  const [more, setMore] = createSignal(false)
  const shown = () => (props.all || more() ? entries() : entries().slice(0, MAX_CHILDREN))
  const isObject = () => props.value !== null && typeof props.value === 'object'
  const entries = createMemo<[string, unknown][]>(() => {
    if (!isObject()) return []
    return Array.isArray(props.value) ? props.value.map((v, i) => [String(i), v]) : Object.entries(props.value as Record<string, unknown>)
  })
  const label = () => (props.name !== undefined ? <span class="ae-json__key">{props.name}: </span> : null)
  return (
    <Show when={isObject()} fallback={<div class="ae-json__row">{label()}<Leaf value={props.value} /></div>}>
      <Show when={entries().length > 0} fallback={<div class="ae-json__row">{label()}<span class="ae-json__punct">{Array.isArray(props.value) ? '[]' : '{}'}</span></div>}>
        <details class="ae-json__branch" open={props.depth < OPEN_DEPTH}>
          <summary class="ae-json__row">
            {label()}
            <span class="ae-json__punct">{Array.isArray(props.value) ? `[${entries().length}]` : `{${entries().length}}`}</span>
          </summary>
          <div class="ae-json__children">
            <For each={shown()}>{([k, v]) => <Node name={k} value={v} depth={props.depth + 1} all={props.all} locale={props.locale} />}</For>
            <Show when={shown().length < entries().length}>
              <button type="button" class="ae-link ae-json__more" onClick={() => setMore(true)}>{runStrings(props.locale).node.showAll(entries().length)}</button>
            </Show>
          </div>
        </details>
      </Show>
    </Show>
  )
}

/** A value: a string as text, anything else as a JSON tree; folded above 256 KB. */
export function Value(props: { value: unknown; locale: EditorLocale; label: string }): JSX.Element {
  const t = () => runStrings(props.locale).node
  const size = createMemo(() => byteSize(props.value))
  const [full, setFull] = createSignal(false)
  const folded = () => !full() && size() > OUTPUT_FOLD_BYTES
  return (
    <div class="ae-value" role="group" aria-label={props.label}>
      <Show when={!folded()} fallback={
        <div class="ae-value__folded">
          <p class="ae-panel__note">{t().folded(formatBytes(size(), props.locale))}</p>
          <pre class="ae-value__text ae-value__text--preview">{jsonText(props.value).slice(0, 2048)}…</pre>
          <button type="button" class="ae-btn ae-btn--secondary" onClick={() => setFull(true)}>{t().loadFull}</button>
        </div>
      }>
        <Show when={typeof props.value === 'string'} fallback={<div class="ae-json"><Node value={props.value} depth={0} all={full()} locale={props.locale} /></div>}>
          <pre class="ae-value__text">{props.value as string}</pre>
        </Show>
      </Show>
    </div>
  )
}

/** «Copy» with its result announced politely (RV5). */
export function CopyText(props: { text: string; locale: EditorLocale; label?: string }): JSX.Element {
  const t = () => runStrings(props.locale).node
  const [said, setSaid] = createSignal('')
  let timer: ReturnType<typeof setTimeout> | undefined
  onCleanup(() => clearTimeout(timer))
  const copy = async () => {
    try {
      if (!navigator.clipboard) throw new Error('no clipboard')
      await navigator.clipboard.writeText(props.text)
      setSaid(t().copied)
    } catch (error) {
      console.warn('automation-editor: copy failed', error)
      setSaid('')
    }
    clearTimeout(timer)
    timer = setTimeout(() => setSaid(''), 2000)
  }
  return (
    <span class="ae-copy">
      <button type="button" class="ae-btn ae-btn--ghost ae-btn--sm" onClick={() => void copy()} aria-label={props.label ? `${t().copy}: ${props.label}` : t().copy}>
        <Icon name={said() ? 'check' : 'copy'} size={14} /><span>{said() || t().copy}</span>
      </button>
      <span class="ae-sr" role="status" aria-live="polite">{said()}</span>
    </span>
  )
}
