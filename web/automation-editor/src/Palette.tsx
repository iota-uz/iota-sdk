// ⌘K palette of steps and triggers: catalog groups, search by title and description with a counter and
// highlighted matches, keyboard navigation (K5). It also serves "connect <outcome> to…" (ED4) as a list
// of targets with "not connected". At a released outcome it opens at the release point (ED3).
import { createMemo, createSignal, For, Show, type JSX } from 'solid-js'
import { describeItem } from './names'
import { Icon } from './icons'
import { isCoarse, useDismiss, useFocusTrap } from './layer'
import { strings } from './strings'
import type { AutomationGraph, EditorCatalog, EditorLocale, XY } from './types'

export type PaletteMode =
  | { kind: 'add'; from?: { node: string; outcome: string } | null; anchor?: XY | null; flow?: XY | null }
  | { kind: 'trigger'; anchor?: XY | null }
  | { kind: 'connect'; from: { node: string; outcome: string }; anchor?: XY | null }

export type PaletteChoice =
  | { kind: 'node'; type: string }
  | { kind: 'trigger'; event: string }
  | { kind: 'target'; id: string | null }

type Entry = {
  key: string
  group: string
  section: 'triggers' | 'steps' | 'targets'
  title: string
  description: string
  icon: string
  disabled: boolean
  note?: string
  /** Extra text the search matches but the list does not show (an event key). */
  hidden?: string
  choice: PaletteChoice
}

export interface PaletteProps {
  /** Closing: it plays its leaving animation, inert (#1352). */
  leaving?: boolean
  mode: PaletteMode
  catalog: EditorCatalog
  graph: AutomationGraph
  locale: EditorLocale
  /** The container the anchor is relative to, for clamping. */
  bounds?: { width: number; height: number }
  onPick(choice: PaletteChoice): void
  onClose(): void
}

const fold = (text: string) => text.toLocaleLowerCase()

export function highlight(text: string, query: string): JSX.Element {
  const q = query.trim()
  if (!q) return text
  const index = fold(text).indexOf(fold(q))
  if (index < 0) return text
  return (
    <>
      {text.slice(0, index)}
      <mark class="ae-mark">{text.slice(index, index + q.length)}</mark>
      {text.slice(index + q.length)}
    </>
  )
}

export function Palette(props: PaletteProps): JSX.Element {
  const t = () => strings(props.locale)
  const coarse = isCoarse()
  const [query, setQuery] = createSignal('')
  const [active, setActive] = createSignal(0)
  let root: HTMLDivElement | undefined
  let list: HTMLDivElement | undefined
  const listId = `ae-palette-${Math.random().toString(36).slice(2, 8)}`

  useDismiss(() => root, () => props.onClose(), { leaving: () => props.leaving })
  // On touch the search is not focused: the keyboard would cover the list.
  useFocusTrap(() => root, { autofocus: !coarse, leaving: () => props.leaving })

  const entries = createMemo<Entry[]>(() => {
    const mode = props.mode
    if (mode.kind === 'connect') {
      const out: Entry[] = [{
        key: 'none', group: '', section: 'targets', title: t().palette.notConnected, description: '', icon: 'ignore', disabled: false,
        choice: { kind: 'target', id: null },
      }]
      for (const node of props.graph.nodes) {
        const name = describeItem(props.graph, props.catalog, node.id, props.locale)
        out.push({ key: `t:${node.id}`, group: t().palette.existing, section: 'targets', title: name.title, description: name.subtitle, icon: 'node', disabled: false, choice: { kind: 'target', id: node.id } })
      }
      return out
    }
    const out: Entry[] = []
    if (mode.kind === 'trigger' || (mode.kind === 'add' && !mode.from)) {
      for (const event of props.catalog.events) {
        out.push({
          key: `e:${event.key}`,
          group: event.group,
          section: 'triggers',
          title: event.title,
          description: '',
          hidden: event.key,
          icon: event.schedule ? 'schedule' : event.manual ? 'manual' : 'event',
          disabled: event.connected === false,
          note: event.connected === false ? t().palette.noConnection : undefined,
          choice: { kind: 'trigger', event: event.key },
        })
      }
    }
    if (mode.kind === 'add') {
      for (const node of props.catalog.nodes) {
        // A front type (the generic "action") is not offered: each type it can name is its own entry
        // ("Block user" under its module), so nobody has to type a key.
        if (node.dispatch && props.catalog.nodes.some((n) => n !== node && n.type.startsWith(node.dispatch!.prefix))) continue
        out.push({
          key: `n:${node.type}`,
          hidden: node.type,
          group: node.group,
          section: 'steps',
          title: node.title,
          description: node.description,
          icon: node.icon ?? node.behavior,
          disabled: node.connected === false,
          note: node.connected === false ? t().palette.noConnection : undefined,
          choice: { kind: 'node', type: node.type },
        })
      }
    }
    return out
  })

  const filtered = createMemo(() => {
    const q = fold(query().trim())
    if (!q) return entries()
    return entries().filter((e) => fold(e.title).includes(q) || fold(e.description).includes(q) || fold(e.group).includes(q) || fold(e.hidden ?? '').includes(q))
  })

  const selectable = createMemo(() => filtered().filter((e) => !e.disabled))

  // Filtering reflows the list without a jump: a group and an entry that stay in the results keep their
  // elements (the group objects are cached by key, the entries come from one memo), only new ones fade in.
  type Group = { key: string; section: Entry['section']; group: string }
  const groupCache = new Map<string, Group>()
  const grouped = createMemo(() => {
    const items = new Map<string, Entry[]>()
    const order: Group[] = []
    for (const entry of filtered()) {
      const key = `${entry.section}\u0000${entry.group}`
      let list = items.get(key)
      if (!list) {
        list = []
        items.set(key, list)
        let group = groupCache.get(key)
        if (!group) {
          group = { key, section: entry.section, group: entry.group }
          groupCache.set(key, group)
        }
        order.push(group)
      }
      list.push(entry)
    }
    return { order, items }
  })
  const groups = () => grouped().order
  const itemsOf = (group: Group) => grouped().items.get(group.key) ?? []

  const activeEntry = () => selectable()[Math.min(active(), selectable().length - 1)]

  const move = (delta: number) => {
    const count = selectable().length
    if (count === 0) return
    setActive((i) => (i + delta + count) % count)
    queueMicrotask(() => list?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }))
  }

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      move(1)
    } else if (event.key === 'ArrowUp') {
      event.preventDefault()
      move(-1)
    } else if (event.key === 'Enter') {
      const entry = activeEntry()
      if (entry) {
        event.preventDefault()
        props.onPick(entry.choice)
      }
    } else if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      props.onClose()
    }
  }

  const title = () => {
    const mode = props.mode
    if (mode.kind === 'connect') return t().palette.titleConnect(mode.from.outcome)
    if (mode.kind === 'trigger') return t().states.addTrigger
    return mode.from ? t().palette.titleFor(mode.from.outcome) : t().palette.title
  }

  const position = (): JSX.CSSProperties => {
    if (coarse) return {}
    const anchor = props.mode.anchor
    if (!anchor) return {}
    const width = 360
    const height = 420
    const b = props.bounds ?? { width: window.innerWidth, height: window.innerHeight }
    const left = Math.max(8, Math.min(anchor.x, b.width - width - 8))
    const top = Math.max(8, Math.min(anchor.y, b.height - height - 8))
    return { left: `${left}px`, top: `${top}px` }
  }

  const noEvents = () => (props.mode.kind === 'trigger' || (props.mode.kind === 'add' && !props.mode.from)) && props.catalog.events.length === 0

  return (
    <div
      ref={root}
      class="ae-palette nopan nowheel"
      classList={{ 'ae-palette--anchored': Boolean(props.mode.anchor) && !coarse, 'ae-palette--sheet': coarse }}
      style={position()}
      role="dialog"
      aria-modal="true"
      aria-label={title()}
      tabindex="-1"
      onKeyDown={onKeyDown}
    >
      <div class="ae-palette__head">
        <span class="ae-palette__title">{title()}</span>
        <Show when={coarse}>
          <button type="button" class="ae-btn ae-btn--ghost" onClick={() => props.onClose()}>{t().palette.close}</button>
        </Show>
      </div>
      <div class="ae-palette__search">
        <Icon name="search" />
        <input
          data-autofocus
          type="search"
          role="combobox"
          aria-expanded="true"
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={activeEntry() ? `${listId}-${activeEntry()!.key}` : undefined}
          aria-label={t().palette.search}
          placeholder={t().palette.search}
          value={query()}
          onInput={(event) => {
            setQuery(event.currentTarget.value)
            setActive(0)
          }}
        />
        <span class="ae-palette__count" aria-live="polite">{t().palette.count(selectable().length, entries().filter((e) => !e.disabled).length)}</span>
      </div>
      <div ref={list} id={listId} class="ae-palette__list" role="listbox" aria-label={title()}>
        <Show when={noEvents()}>
          <p class="ae-palette__empty">{t().palette.noEvents}</p>
        </Show>
        <Show when={!noEvents() && filtered().length === 0}>
          <p class="ae-palette__empty">{t().palette.nothing(query().trim())}</p>
        </Show>
        <For each={groups()}>
          {(group) => (
            <div role="group" aria-label={group.group || (group.section === 'triggers' ? t().palette.triggers : t().palette.steps)}>
              <Show when={group.group}>
                <div class="ae-palette__group" aria-hidden="true">
                  {group.section === 'triggers' ? `${t().palette.triggers} · ` : ''}{group.group}
                </div>
              </Show>
              <For each={itemsOf(group)}>
                {(entry) => {
                  const selected = () => activeEntry()?.key === entry.key
                  return (
                    <div
                      id={`${listId}-${entry.key}`}
                      class="ae-palette__item"
                      classList={{ 'ae-palette__item--active': selected(), 'ae-palette__item--disabled': entry.disabled }}
                      role="option"
                      aria-selected={selected()}
                      aria-disabled={entry.disabled}
                      data-choice={entry.key}
                      onPointerMove={() => {
                        const index = selectable().indexOf(entry)
                        if (index >= 0 && index !== active()) setActive(index)
                      }}
                      onClick={() => {
                        if (!entry.disabled) props.onPick(entry.choice)
                      }}
                    >
                      <span class="ae-palette__icon"><Icon name={entry.icon} /></span>
                      <span class="ae-palette__text">
                        <span class="ae-palette__name">{highlight(entry.title, query())}</span>
                        <Show when={entry.description}>
                          <span class="ae-palette__desc">{highlight(entry.description, query())}</span>
                        </Show>
                      </span>
                      <Show when={entry.note}>
                        <span class="ae-palette__note">{entry.note}</span>
                      </Show>
                    </div>
                  )
                }}
              </For>
            </div>
          )}
        </For>
      </div>
    </div>
  )
}
