// The graph editor: toolbar, canvas, step panel, Validation dock, ⌘K palette, history, copy/paste,
// Tidy, touch controls and the read-only outline on phones. The package knows nothing of routes,
// storage or permissions: everything it does to the outside goes through the host's props.
import { ConfirmationDialog, Switch as Toggle, ToastProvider, useToast } from '@iota-uz/sdk/solid-ui'
import { batch, createEffect, createMemo, createSignal, lazy, on, onCleanup, onMount, Show, Suspense, type JSX } from 'solid-js'
import { Canvas } from './Canvas'
import { copyItems, parseClip, pasteClip, serializeClip, type Clip } from './clipboard'
import type { MenuItem } from './ContextMenu'
import { snap } from './geometry'
import {
  addNode,
  addTrigger,
  catalogEvent,
  catalogNode,
  connect,
  disconnect,
  duplicateItems,
  edgeOf,
  findItem,
  graphsEqual,
  IGNORE,
  itemOutcomes,
  parentOf,
  removeItems,
  setIgnore,
} from './graph'
import { amend, canRedo, canUndo, createHistory, record, redo, undo, type History, type Snapshot } from './history'
import { Icon } from './icons'
import { ensureLayout, placeNew, tidied, tidyLayout } from './layout'
import { isMac } from './locale'
import { Outline } from './Outline'
import type { PaletteChoice, PaletteMode } from './Palette'
import { keyLabel, strings } from './strings'
import type { AutomationGraph, AutomationLayout, CanvasApi, CanvasMenu, EditorProps, GraphError, XY } from './types'
import { localErrors } from './validation'
import { rebaseErrors, type ValidationState } from './validation'
import { isEditableTarget } from './xy/dom'

// The panel's forms (JSON Schema fields, CEL and template editors, the schedule editor) are their own
// lazy chunk: the canvas chunk stays within its budget and the first paint does not wait for them.
const loadPalette = () => import('./Palette')
const loadMenu = () => import('./ContextMenu')
const loadPanel = () => import('./NodePanel')
const NodePanel = lazy(() => loadPanel().then((m) => ({ default: m.NodePanel })))
const Palette = lazy(() => loadPalette().then((m) => ({ default: m.Palette })))
const ContextMenu = lazy(() => loadMenu().then((m) => ({ default: m.ContextMenu })))
const SettingsDialog = lazy(() => import('./SettingsDialog').then((m) => ({ default: m.SettingsDialog })))
const ValidationPanel = lazy(() => import('./ValidationPanel').then((m) => ({ default: m.ValidationPanel })))
const KeyMap = lazy(() => import('./KeyMap').then((m) => ({ default: m.KeyMap })))

export const VALIDATE_DEBOUNCE_MS = 800
export const LAYOUT_DEBOUNCE_MS = 500
export const FIELD_COALESCE_MS = 1000
const STILL_WAITING_MS = 4000

type SaveState = { status: 'idle' } | { status: 'saving'; slow: boolean } | { status: 'failed' }
type LayoutState = 'idle' | 'saving' | 'saved' | 'failed'

const layoutsEqual = (a: AutomationLayout, b: AutomationLayout) => {
  const ka = Object.keys(a)
  if (ka.length !== Object.keys(b).length) return false
  return ka.every((k) => b[k] && a[k]!.x === b[k]!.x && a[k]!.y === b[k]!.y)
}

export function Editor(props: EditorProps): JSX.Element {
  return (
    <ToastProvider notifyEvent="iota-automation-editor:notify" removeEvent="iota-automation-editor:remove">
      <EditorBody {...props} />
    </ToastProvider>
  )
}

function EditorBody(props: EditorProps): JSX.Element {
  const t = () => strings(props.locale)
  const toast = useToast()
  const mac = isMac()
  const readOnly = () => Boolean(props.readOnly)

  // ——— draft & history ———
  const initial: Snapshot = { graph: props.graph, layout: ensureLayout(props.graph, props.catalog, props.layout) }
  const [history, setHistory] = createSignal<History>(createHistory(initial))
  const draft = () => history().present.graph
  const layout = () => history().present.layout
  const [saved, setSaved] = createSignal<AutomationGraph>(props.graph)
  const [base, setBase] = createSignal(props.version)
  const [persistedLayout, setPersistedLayout] = createSignal<AutomationLayout>(initial.layout)
  const dirty = createMemo(() => !graphsEqual(draft(), saved()))

  let lastKey: string | null = null
  let lastAt = 0
  const commit = (next: Snapshot, key?: string) => {
    // A phone shows the read-only outline: no path may edit there (a layer left open, a shortcut).
    if (mobile()) return
    const now = Date.now()
    if (key && key === lastKey && now - lastAt < FIELD_COALESCE_MS) setHistory((h) => amend(h, next))
    else setHistory((h) => record(h, next))
    lastKey = key ?? null
    lastAt = now
  }
  const commitGraph = (graph: AutomationGraph, key?: string) => commit({ graph, layout: ensureLayout(graph, props.catalog, layout()) }, key)

  // Positions the host read from storage (another tab moved steps): they replace the positions on screen
  // and become the saved layout, without a history step or a layout save.
  createEffect(on(() => props.remoteLayout, (remote) => {
    if (!remote) return
    const next = ensureLayout(draft(), props.catalog, { ...layout(), ...remote.positions })
    batch(() => {
      setHistory((h) => amend(h, { graph: h.present.graph, layout: next }))
      setPersistedLayout(next)
    })
  }, { defer: true }))

  // ——— selection, layers ———
  const [selected, setSelectedSignal] = createSignal<string | null>(props.selected ?? null)
  const setSelected = (id: string | null) => {
    setSelectedSignal(id)
    if (id !== (props.selected ?? null)) props.onSelect?.(id)
  }
  createEffect(on(() => props.selected, (id) => {
    if (id !== undefined && id !== selected()) setSelectedSignal(id)
  }, { defer: true }))
  const [focusField, setFocusField] = createSignal<string | null>(null)
  const [palette, setPalette] = createSignal<PaletteMode | null>(null)
  const [menu, setMenu] = createSignal<CanvasMenu | null>(null)
  const [settingsOpen, setSettingsOpen] = createSignal(false)
  const [keymapOpen, setKeymapOpen] = createSignal(false)
  const [validationOpen, setValidationOpen] = createSignal(false)
  const [selecting, setSelecting] = createSignal(false)
  const [snapOn, setSnapOn] = createSignal(true)
  const [mobile, setMobile] = createSignal(false)
  const [coarse, setCoarse] = createSignal(false)
  const [online, setOnline] = createSignal(true)
  const [areaSize, setAreaSize] = createSignal({ width: 0, height: 0 })
  const [versionsMenu, setVersionsMenu] = createSignal<XY | null>(null)
  let canvasApi: CanvasApi | null = null
  let area: HTMLDivElement | undefined
  let root: HTMLDivElement | undefined
  let panelRef: HTMLDivElement | undefined
  let internalClip: Clip | null = null

  createEffect(on(draft, (graph) => {
    const id = selected()
    if (id && !findItem(graph, id)) setSelected(null)
  }))

  // Crossing to the phone width closes every editing layer; the outline is read-only (T7).
  createEffect(on(mobile, (phone) => {
    if (!phone) return
    batch(() => {
      setPalette(null)
      setMenu(null)
      setVersionsMenu(null)
      setSettingsOpen(false)
      setKeymapOpen(false)
      setSelected(null)
      setValidationOpen(false)
    })
  }, { defer: true }))

  // ——— validation ———
  // Each result belongs to the snapshot it checked. Shown errors are rebased onto the current draft by
  // item id; a result for an older snapshot never decides whether the current one may be saved.
  const [validation, setValidation] = createSignal<ValidationState>({ status: 'idle' })
  const local = createMemo(() => localErrors(draft(), (type) => catalogNode(props.catalog, type), {
    unknown: (type) => t().validation.unavailable(type),
    noConnection: (title) => t().validation.noConnection(title),
  }))
  const checked = createMemo(() => {
    const state = validation()
    if (state.status === 'idle') return { errors: [] as GraphError[], warnings: [] as GraphError[] }
    return { errors: rebaseErrors(state.errors, state.graph, draft()), warnings: rebaseErrors(state.warnings, state.graph, draft()) }
  })
  const allErrors = createMemo<GraphError[]>(() => [...local(), ...checked().errors])
  /** The server answered for exactly this snapshot. */
  const validFor = (graph: AutomationGraph) => {
    const state = validation()
    return state.status === 'done' && state.graph === graph
  }
  let validateSeq = 0
  let validateTimer: ReturnType<typeof setTimeout> | undefined
  /** Checks a snapshot; the result is shown only while that snapshot is still the draft. */
  const validate = async (graph: AutomationGraph = draft()): Promise<{ errors: GraphError[]; warnings: GraphError[] } | null> => {
    clearTimeout(validateTimer)
    const seq = ++validateSeq
    const previous = validation()
    const keep = previous.status === 'idle' ? { errors: [], warnings: [], graph } : previous
    setValidation({ ...keep, status: 'checking' })
    try {
      const result = await props.api.validate(graph)
      const issues = { errors: result.errors ?? [], warnings: result.warnings ?? [] }
      if (seq === validateSeq && draft() === graph) setValidation({ status: 'done', ...issues, graph })
      return issues
    } catch (error) {
      console.warn('automation-editor: validate failed', error)
      if (seq === validateSeq) {
        setValidation({ ...keep, status: 'failed' })
        toast.notify({ variant: 'error', title: t().validation.failed, action: { label: t().common.retry, onAction: () => void validate() } })
      }
      return null
    }
  }
  createEffect(on(draft, () => {
    // An edit invalidates the check in flight; the next one starts after a pause.
    validateSeq += 1
    clearTimeout(validateTimer)
    validateTimer = setTimeout(() => void validate(), VALIDATE_DEBOUNCE_MS)
  }, { defer: true }))

  // ——— writes ———
  // Save, layout saves and Load latest change the same stored layout: they run one at a time, and after
  // each one the layout on screen is compared with the stored one and the latest snapshot is sent.
  let writes: Promise<unknown> = Promise.resolve()
  let writing = 0
  const exclusive = <T,>(fn: () => Promise<T>): Promise<T> => {
    writing += 1
    const run = writes.then(fn, fn)
    writes = run.then(() => undefined, () => undefined)
    return run.finally(() => {
      writing -= 1
      queueMicrotask(checkLayout)
    })
  }

  // ——— layout autosave (AS1): never a version ———
  const [layoutState, setLayoutState] = createSignal<LayoutState>('idle')
  let layoutTimer: ReturnType<typeof setTimeout> | undefined
  let layoutQueued = false
  // A refused snapshot stays refused ("Not saved · Retry") until the person retries or moves something;
  // it is never resent on its own. Nothing is scheduled after unmount.
  let failedLayout: AutomationLayout | null = null
  let disposed = false
  onCleanup(() => {
    disposed = true
    clearTimeout(layoutTimer)
  })
  const retryLayout = () => {
    failedLayout = null
    void saveLayoutNow()
  }
  const saveLayoutNow = async () => {
    clearTimeout(layoutTimer)
    layoutTimer = undefined
    const save = props.saveLayout
    if (!save || readOnly() || layoutQueued || disposed) return
    layoutQueued = true
    await exclusive(async () => {
      layoutQueued = false
      for (;;) {
        const next = layout()
        if (layoutsEqual(next, persistedLayout())) {
          setLayoutState((state) => (state === 'idle' ? 'idle' : 'saved'))
          return
        }
        setLayoutState('saving')
        try {
          await save(next)
          setPersistedLayout(next)
          failedLayout = null
        } catch (error) {
          console.warn('automation-editor: saveLayout failed', error)
          failedLayout = next
          setLayoutState('failed')
          return
        }
      }
    })
  }
  /** After any write, or when the layout changes: schedule a layout save if the screen and storage differ. */
  function checkLayout() {
    if (disposed || !props.saveLayout || readOnly() || writing > 0 || layoutQueued || layoutTimer !== undefined) return
    if (failedLayout && layoutsEqual(layout(), failedLayout)) return
    if (layoutsEqual(layout(), persistedLayout())) {
      if (layoutState() === 'saving') setLayoutState('saved')
      return
    }
    layoutTimer = setTimeout(() => void saveLayoutNow(), LAYOUT_DEBOUNCE_MS)
  }
  createEffect(on(layout, () => {
    if (writing === 0 && !layoutQueued) {
      clearTimeout(layoutTimer)
      layoutTimer = undefined
    }
    checkLayout()
  }, { defer: true }))

  // ——— save ———
  const [saveState, setSaveState] = createSignal<SaveState>({ status: 'idle' })
  const [pendingReview, setPendingReview] = createSignal(false)
  const [conflict, setConflict] = createSignal<{ by: string; version: number } | null>(null)
  const [overwrite, setOverwrite] = createSignal<number | null>(null)
  const [confirmReload, setConfirmReload] = createSignal(false)
  const [reloading, setReloading] = createSignal(false)
  const editingBlocked = () => readOnly() || mobile()
  /** Save is blocked by errors known for this very draft; a draft not checked yet is checked on Save. */
  const knownErrors = () => local().length > 0 || (validFor(draft()) && checked().errors.length > 0)
  const canSave = () => !editingBlocked() && saveState().status !== 'saving' && !knownErrors() && (dirty() || props.version === 0 && base() === 0)

  const save = async () => {
    if (!canSave()) {
      if (knownErrors()) setValidationOpen(true)
      return
    }
    const graph = draft()
    const sentBase = base()
    setSaveState({ status: 'saving', slow: false })
    const slow = setTimeout(() => setSaveState((s) => (s.status === 'saving' ? { status: 'saving', slow: true } : s)), STILL_WAITING_MS)
    if (!validFor(graph)) {
      // The snapshot that is sent is the one that was checked.
      // validate() publishes the result only while this snapshot is still the draft; a newer check owns
      // the panel otherwise.
      const issues = await validate(graph)
      if (!issues || issues.errors.length > 0) {
        clearTimeout(slow)
        setSaveState({ status: 'idle' })
        if (issues && draft() === graph) setValidationOpen(true)
        return
      }
    }
    let positions = layout()
    try {
      const result = await exclusive(() => {
        // The layout sent is the one on screen when the write starts, after any layout save before it.
        positions = layout()
        return props.save(graph, positions, sentBase)
      })
      clearTimeout(slow)
      if ('ok' in result && result.ok) {
        batch(() => {
          setSaveState({ status: 'idle' })
          setBase(result.version)
          setSaved(result.graph)
          setPersistedLayout(positions)
          setConflict(null)
          setOverwrite(null)
          setPendingReview(Boolean(result.pending_review))
          // The server may normalise the graph (defaults); adopt it when the draft did not move meanwhile.
          if (graphsEqual(draft(), graph) && !graphsEqual(graph, result.graph)) setHistory((h) => amend(h, { graph: result.graph, layout: h.present.layout }))
        })
        toast.notify({ variant: 'success', title: t().save.saved(result.version) })
        return
      }
      if ('conflict' in result) {
        setSaveState({ status: 'idle' })
        setConflict(result.conflict)
        return
      }
      setSaveState({ status: 'idle' })
      const rejected = result as { errors?: GraphError[]; warnings?: GraphError[] }
      // The server's errors are about the sent snapshot: shown only while it is still the draft.
      if (draft() === graph) {
        validateSeq += 1
        setValidation({ status: 'done', errors: rejected.errors ?? [], warnings: rejected.warnings ?? [], graph })
        setValidationOpen(true)
      }
    } catch (error) {
      clearTimeout(slow)
      console.warn('automation-editor: save failed', error)
      setSaveState({ status: 'failed' })
      if (typeof navigator !== 'undefined' && navigator.onLine === false) setOnline(false)
      // The retry sends the same base (F3): base only moves on success.
      toast.notify({ variant: 'error', title: t().save.failed, action: { label: t().save.retry, onAction: () => void save() } })
    }
  }

  const loadLatest = async () => {
    setReloading(true)
    try {
      const latest = await exclusive(() => props.reload())
      // Draft, layout, saved baseline and version change together; this is not a user edit, so the phone
      // guard of commit() does not apply (the width may change while the reload runs).
      batch(() => {
        const nextLayout = ensureLayout(latest.graph, props.catalog, latest.layout)
        setHistory((h) => record(h, { graph: latest.graph, layout: nextLayout }))
        lastKey = null
        setSaved(latest.graph)
        setBase(latest.version)
        setPersistedLayout(nextLayout)
        setConflict(null)
        setOverwrite(null)
        setConfirmReload(false)
      })
    } catch (error) {
      console.warn('automation-editor: reload failed', error)
      toast.notify({ variant: 'error', title: t().save.reloadFailed, action: { label: t().common.retry, onAction: () => void loadLatest() } })
    } finally {
      setReloading(false)
    }
  }

  const stay = () => {
    const c = conflict()
    if (!c) return
    batch(() => {
      setBase(c.version)
      setOverwrite(c.version)
      setConflict(null)
    })
  }

  // ——— graph actions ———
  const addFromPalette = (choice: PaletteChoice) => {
    const mode = palette()
    setPalette(null)
    if (!mode || readOnly()) return
    let graph = draft()
    if (choice.kind === 'target') {
      if (mode.kind !== 'connect') return
      graph = choice.id ? connect(graph, mode.from, choice.id) : disconnect(graph, mode.from)
      commitGraph(graph)
      canvasApi?.focus(mode.from.node)
      return
    }
    let id: string
    if (choice.kind === 'node') {
      const spec = catalogNode(props.catalog, choice.type)
      if (!spec) return
      const added = addNode(graph, spec)
      graph = added.graph
      id = added.id
    } else {
      const added = addTrigger(graph, catalogEvent(props.catalog, choice.event), choice.event)
      graph = added.graph
      id = added.id
    }
    const from = mode.kind === 'add' ? mode.from ?? null : null
    if (from) graph = connect(graph, from, id)
    const nextLayout = { ...layout() }
    const flow = mode.kind === 'add' ? mode.flow : null
    // A drop point is the person's own; a place the editor picks (under the source, right of the triggers,
    // the middle of the view) carries the mark, so a later automatic layout may lay its group out again.
    nextLayout[id] = flow && choice.kind === 'node'
      ? placeNew(graph, props.catalog, nextLayout, id, { near: { x: snap(flow.x), y: snap(flow.y) } })
      : tidied(placeNew(graph, props.catalog, nextLayout, id, { source: from?.node ?? null, near: from || choice.kind === 'trigger' ? null : centerNear() }))
    commit({ graph, layout: ensureLayout(graph, props.catalog, nextLayout) })
    setSelected(id)
    queueMicrotask(() => canvasApi?.focus(id))
  }

  const centerNear = (): XY | null => {
    if (draft().nodes.length + draft().triggers.length <= 1) return null
    const c = canvasApi?.center()
    return c ? { x: snap(c.x - 112), y: snap(c.y - 48) } : null
  }

  const selection = () => {
    const ids = canvasApi?.selection() ?? []
    const id = selected()
    return ids.length > 0 ? ids : id ? [id] : []
  }

  const deleteItems = (ids: string[]) => {
    if (readOnly() || ids.length === 0) return
    const parent = parentOf(draft(), ids[0]!)
    commitGraph(removeItems(draft(), ids))
    const focusTarget = parent && !ids.includes(parent) ? parent : null
    setSelected(focusTarget)
    if (focusTarget) queueMicrotask(() => canvasApi?.focus(focusTarget))
  }

  const duplicate = (ids: string[]) => {
    if (readOnly() || ids.length === 0) return
    const result = duplicateItems(draft(), ids)
    const nextLayout = { ...layout() }
    for (const [from, to] of result.map) {
      const p = nextLayout[from]
      if (p) nextLayout[to] = { x: Math.round(p.x) + 32, y: Math.round(p.y) + 32 }
    }
    commit({ graph: result.graph, layout: ensureLayout(result.graph, props.catalog, nextLayout) })
    setSelected(result.ids[0] ?? null)
  }

  const copy = (ids: string[]): string | null => {
    const clip = copyItems(draft(), layout(), ids)
    if (!clip) return null
    internalClip = clip
    return serializeClip(clip)
  }

  const paste = (clip: Clip | null, at?: XY | null) => {
    if (!clip || readOnly()) return
    const point = at ?? canvasApi?.center() ?? { x: 0, y: 0 }
    const result = pasteClip(draft(), layout(), clip, { x: point.x + 32, y: point.y + 32 })
    commit({ graph: result.graph, layout: ensureLayout(result.graph, props.catalog, result.layout) })
    setSelected(result.ids[0] ?? null)
  }

  // Tidy is one history step: the whole layout is replaced at once, and the cards glide there (200 ms).
  const [layoutTransition, setLayoutTransition] = createSignal(0)
  const tidy = () => {
    if (readOnly() || mobile()) return
    batch(() => {
      setLayoutTransition((n) => n + 1)
      commit({ graph: draft(), layout: tidyLayout(draft(), props.catalog) })
    })
    queueMicrotask(() => canvasApi?.fit())
  }

  const doUndo = () => setHistory((h) => undo(h))
  const doRedo = () => setHistory((h) => redo(h))

  const openPanelFocus = (id: string) => {
    setSelected(id)
    queueMicrotask(() => panelRef?.querySelector<HTMLElement>('input, textarea, button[role="combobox"], [role="combobox"], select, button')?.focus())
  }

  const pickIssue = (target: { id: string | null; field: string | null }) => {
    if (!target.id) {
      if (target.field === 'name' || target.field === 'run_as') setSettingsOpen(true)
      return
    }
    setSelected(target.id)
    setFocusField(target.field)
    canvasApi?.focus(target.id)
  }

  // ——— menus ———
  const menuItems = createMemo<MenuItem[]>(() => {
    const m = menu()
    if (!m) return []
    const items: MenuItem[] = []
    const editable = !readOnly()
    if (m.item) {
      const id = m.item
      const ids = selection().includes(id) ? selection() : [id]
      const many = ids.length > 1
      if (!many) items.push({ key: 'open', label: t().menu.open, shortcut: 'Enter', run: () => openPanelFocus(id) })
      if (editable && !many) {
        for (const outcome of itemOutcomes(draft(), props.catalog, id)) {
          items.push({ key: `connect:${outcome}`, label: t().menu.connectTo(outcome), run: () => setPalette({ kind: 'connect', from: { node: id, outcome }, anchor: { x: m.x, y: m.y } }) })
          const ignored = edgeOf(draft(), id, outcome)?.to === IGNORE
          items.push({
            key: `ignore:${outcome}`,
            label: ignored ? t().menu.unignore(outcome) : t().menu.ignore(outcome),
            run: () => commitGraph(setIgnore(draft(), { node: id, outcome }, !ignored)),
          })
        }
      }
      if (editable) items.push({ key: 'duplicate', label: t().menu.duplicate, shortcut: '⌘D', run: () => duplicate(ids) })
      items.push({ key: 'copy', label: t().menu.copy, shortcut: '⌘C', run: () => writeClipboard(copy(ids)) })
      if (editable) items.push({ key: 'delete', label: many ? t().menu.deleteMany(ids.length) : t().menu.delete, shortcut: 'Delete', danger: true, run: () => deleteItems(ids) })
      return items
    }
    if (editable) {
      items.push({ key: 'add', label: t().menu.addHere, run: () => setPalette({ kind: 'add', anchor: { x: m.x, y: m.y }, flow: m.flow }) })
      items.push({ key: 'paste', label: t().menu.paste, shortcut: '⌘V', run: () => void readClipboard().then((clip) => paste(clip, m.flow)) })
    }
    items.push({ key: 'select-all', label: t().menu.selectAll, shortcut: '⌘A', run: () => canvasApi?.selectAll() })
    items.push({ key: 'fit', label: t().canvas.fit, shortcut: '⇧1', run: () => canvasApi?.fit() })
    if (editable) items.push({ key: 'tidy', label: t().canvas.tidy, shortcut: '⇧⌘L', run: tidy })
    if (editable && coarse()) items.push({ key: 'snap', label: t().menu.snap, checked: snapOn(), run: () => setSnapOn((v) => !v) })
    return items
  })

  const writeClipboard = (text: string | null) => {
    if (!text) return
    void navigator.clipboard?.writeText(text).catch(() => undefined)
  }
  const readClipboard = async (): Promise<Clip | null> => {
    try {
      const text = await navigator.clipboard?.readText()
      return (text && parseClip(text)) || internalClip
    } catch {
      return internalClip
    }
  }

  // ——— keyboard (ED5, K6) ———
  const inEditor = (target: EventTarget | null) => {
    if (!root) return false
    if (target === document.body || target === document.documentElement || target === null) return true
    return target instanceof Node && root.contains(target)
  }
  const layerOpen = () => palette() !== null || menu() !== null || versionsMenu() !== null || settingsOpen() || keymapOpen() || confirmReload()

  const onKeyDown = (event: KeyboardEvent) => {
    if (!inEditor(event.target) || mobile() || props.tab === 'runs') return
    const target = event.target as HTMLElement | null
    if (target?.closest?.('dialog[open]')) return
    const mod = mac ? event.metaKey : event.ctrlKey
    const key = event.key.toLowerCase()
    if (mod && key === 's') {
      event.preventDefault()
      void save()
      return
    }
    if (isEditableTarget(event.target) || layerOpen()) return
    if (mod && key === 'k') {
      event.preventDefault()
      if (!readOnly()) setPalette({ kind: 'add' })
    } else if (mod && event.shiftKey && key === 'l') {
      event.preventDefault()
      tidy()
    } else if (mod && key === 'z') {
      event.preventDefault()
      if (event.shiftKey) doRedo()
      else doUndo()
    } else if (mod && key === 'y') {
      event.preventDefault()
      doRedo()
    } else if (mod && event.key === 'Enter') {
      event.preventDefault()
      setValidationOpen(true)
      void validate()
    } else if (mod && key === 'd') {
      event.preventDefault()
      duplicate(selection())
    } else if (mod && key === 'a') {
      event.preventDefault()
      canvasApi?.selectAll()
    } else if (event.shiftKey && (event.code === 'Digit1' || event.key === '!')) {
      event.preventDefault()
      canvasApi?.fit()
    } else if (event.shiftKey && (event.code === 'Digit0' || event.key === ')')) {
      event.preventDefault()
      canvasApi?.zoomReset()
    } else if (event.key === '?') {
      event.preventDefault()
      setKeymapOpen(true)
    } else if ((event.key === 'Delete' || event.key === 'Backspace') && !(target?.closest?.('.iota-flow__node'))) {
      const ids = selection()
      if (ids.length > 0) {
        event.preventDefault()
        deleteItems(ids)
      }
    }
  }

  const onCopy = (event: ClipboardEvent) => {
    if (!inEditor(event.target) || isEditableTarget(event.target) || mobile()) return
    const ids = selection()
    const text = ids.length > 0 ? copy(ids) : null
    if (!text) return
    event.preventDefault()
    event.clipboardData?.setData('text/plain', text)
  }
  const onPaste = (event: ClipboardEvent) => {
    if (!inEditor(event.target) || isEditableTarget(event.target) || readOnly() || mobile()) return
    const clip = parseClip(event.clipboardData?.getData('text/plain') ?? '') ?? null
    if (!clip) return
    event.preventDefault()
    paste(clip)
  }

  onMount(() => {
    const phone = window.matchMedia('(max-width: 767px)')
    const pointer = window.matchMedia('(pointer: coarse)')
    const update = () => {
      setMobile(phone.matches)
      setCoarse(pointer.matches)
    }
    update()
    phone.addEventListener('change', update)
    pointer.addEventListener('change', update)
    setOnline(navigator.onLine !== false)
    const goOnline = () => {
      setOnline(true)
      void validate()
    }
    const goOffline = () => setOnline(false)
    const beforeUnload = (event: BeforeUnloadEvent) => {
      if (!dirty() || readOnly()) return
      event.preventDefault()
      event.returnValue = t().save.leave
    }
    window.addEventListener('online', goOnline)
    window.addEventListener('offline', goOffline)
    window.addEventListener('beforeunload', beforeUnload)
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('copy', onCopy)
    document.addEventListener('paste', onPaste)
    const observer = new ResizeObserver(() => {
      if (area) setAreaSize({ width: area.clientWidth, height: area.clientHeight })
    })
    if (area) observer.observe(area)
    onCleanup(() => {
      phone.removeEventListener('change', update)
      pointer.removeEventListener('change', update)
      window.removeEventListener('online', goOnline)
      window.removeEventListener('offline', goOffline)
      window.removeEventListener('beforeunload', beforeUnload)
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('copy', onCopy)
      document.removeEventListener('paste', onPaste)
      observer.disconnect()
      clearTimeout(validateTimer)
      clearTimeout(layoutTimer)
    })
    void validate()
    // The palette, menu and panel are their own chunks; fetch them while the browser is idle so their
    // first use does not wait.
    const idle = (window as Window & { requestIdleCallback?: (cb: () => void) => number }).requestIdleCallback ?? ((cb: () => void) => window.setTimeout(cb, 200))
    idle(() => {
      void loadPalette()
      void loadMenu()
      void loadPanel()
    })
  })

  // ——— render ———
  const errorsShown = () => allErrors()
  // The panel shows the issues rebased onto the current draft, plus the editor's own.
  const validationView = createMemo<ValidationState>(() => {
    const state = validation()
    if (state.status === 'idle' && local().length === 0) return state
    return { status: state.status === 'idle' ? 'done' : state.status, errors: allErrors(), warnings: checked().warnings, graph: draft() }
  })
  const issueCount = () => allErrors().length + checked().warnings.length
  const empty = () => draft().triggers.length === 0 && draft().nodes.length === 0
  const saveLabel = () => (saveState().status === 'saving' ? t().toolbar.saving : overwrite() ? t().save.saveOver(overwrite()!) : t().toolbar.save)

  const toolbar = () => (
    <header class="ae-toolbar" role="toolbar" aria-label={t().toolbar.label}>
      <div class="ae-toolbar__path">
        <Show when={props.crumb}><span class="ae-toolbar__crumb">{props.crumb}</span></Show>
        <button type="button" class="ae-toolbar__name" title={draft().name || t().toolbar.newGraph} onClick={() => setSettingsOpen(true)} disabled={mobile()}>
          {draft().name || t().toolbar.newGraph}
        </button>
        <Show when={base() > 0}>
          <span class="ae-tag">v{base()}</span>
        </Show>
        <Show when={dirty() && !readOnly()}>
          <span class="ae-tag ae-tag--dirty" title={t().toolbar.unsaved}><Icon name="dot" size={12} /><span class="ae-sr">{t().toolbar.unsaved}</span></span>
        </Show>
      </div>
      <Show when={props.onTab}>
        <div class="ae-toolbar__tabs" role="tablist" aria-label={t().toolbar.label}>
          <button type="button" role="tab" class="ae-tab" aria-selected={(props.tab ?? 'edit') === 'edit'} onClick={() => props.onTab?.('edit')}>{t().toolbar.edit}</button>
          <button type="button" role="tab" class="ae-tab" aria-selected={props.tab === 'runs'} onClick={() => props.onTab?.('runs')}>
            {t().toolbar.runs}
            <Show when={props.runsCount !== undefined}><span class="ae-count">{props.runsCount}</span></Show>
          </button>
        </div>
      </Show>
      <div class="ae-toolbar__actions">
        <Show when={!mobile()}>
          <button type="button" class="ae-btn ae-btn--ghost" onClick={() => { setValidationOpen(true); void validate() }} aria-keyshortcuts={mac ? 'Meta+Enter' : 'Control+Enter'}>
            <Show when={validation().status === 'checking'} fallback={<Icon name={allErrors().length > 0 ? 'error' : 'check'} />}><span class="ae-spinner" aria-hidden="true" /></Show>
            <span>{validation().status === 'checking' ? t().toolbar.validating : t().toolbar.validate}</span>
            <Show when={issueCount() > 0}><span class="ae-count ae-count--danger">{issueCount()}</span></Show>
          </button>
          <button type="button" class="ae-iconbtn" aria-label={t().toolbar.undo} title={`${t().toolbar.undo} (${keyLabel('⌘Z', mac)})`} disabled={!canUndo(history())} onClick={doUndo}><Icon name="undo" /></button>
          <button type="button" class="ae-iconbtn" aria-label={t().toolbar.redo} title={`${t().toolbar.redo} (${keyLabel('⇧⌘Z', mac)})`} disabled={!canRedo(history())} onClick={doRedo}><Icon name="redo" /></button>
          <Show when={!readOnly()}>
            <button type="button" class="ae-btn ae-btn--ghost" onClick={() => setPalette({ kind: 'add' })} aria-keyshortcuts={mac ? 'Meta+K' : 'Control+K'}>
              <Icon name="plus" /><span>{t().toolbar.add}</span><Show when={!coarse()}><kbd class="ae-kbd">{keyLabel('⌘K', mac)}</kbd></Show>
            </button>
          </Show>
          <button type="button" class="ae-iconbtn" aria-label={t().toolbar.settings} title={t().toolbar.settings} onClick={() => setSettingsOpen(true)}><Icon name="settings" /></button>
          <button type="button" class="ae-iconbtn" aria-label={t().toolbar.keys} title={t().toolbar.keys} onClick={() => setKeymapOpen(true)}><Icon name="keys" /></button>
          <Show when={props.versions && props.versions.length > 0 && props.onOpenVersion}>
            <button
              type="button"
              class="ae-btn ae-btn--ghost"
              aria-haspopup="menu"
              aria-expanded={versionsMenu() !== null}
              onClick={(event) => {
                const r = event.currentTarget.getBoundingClientRect()
                setVersionsMenu({ x: r.left, y: r.bottom + 4 })
              }}
            >
              {t().toolbar.versions}
            </button>
          </Show>
          {props.actions}
          <Show when={!readOnly()}>
            <button
              type="button"
              class="btn btn-primary btn-normal ae-save"
              aria-disabled={!canSave()}
              aria-busy={saveState().status === 'saving'}
              title={allErrors().length > 0 ? t().save.hasErrors : undefined}
              onClick={() => void save()}
            >
              <Show when={saveState().status === 'saving'}><span class="ae-spinner" aria-hidden="true" /></Show>
              {saveLabel()}
            </button>
          </Show>
        </Show>
        <Show when={props.toggleEnabled}>
          <Toggle checked={props.enabled} label={t().toolbar.enabled} onChange={(event) => { event.currentTarget.checked = props.enabled; props.toggleEnabled?.() }} />
        </Show>
      </div>
    </header>
  )

  const banners = () => (
    <div class="ae-banners">
      <Show when={props.readOnly}>
        {(r) => <div class="ae-banner" role="status"><Icon name="dot" /><span><strong>{t().states.readOnly}:</strong> {r().reason}</span></div>}
      </Show>
      <Show when={!online()}>
        <div class="ae-banner ae-banner--warn" role="alert">
          <Icon name="error" /><span>{t().save.offline}</span>
          <button type="button" class="ae-btn ae-btn--secondary" onClick={() => { setOnline(navigator.onLine !== false); void validate() }}>{t().save.retry}</button>
        </div>
      </Show>
      <Show when={conflict()}>
        {(c) => (
          <div class="ae-banner ae-banner--warn" role="alert">
            <Icon name="error" /><span>{t().save.conflict(c().by, c().version)}</span>
            <button type="button" class="ae-btn ae-btn--secondary" onClick={() => setConfirmReload(true)}>{t().save.loadLatest}</button>
            <button type="button" class="ae-btn ae-btn--ghost" onClick={stay}>{t().save.stay}</button>
          </div>
        )}
      </Show>
      <Show when={pendingReview()}>
        <div class="ae-banner" role="status">
          <Icon name="dot" /><span>{t().save.pendingReview}</span>
          {props.banner}
          <button type="button" class="ae-iconbtn" aria-label={t().common.close} onClick={() => setPendingReview(false)}><Icon name="close" /></button>
        </div>
      </Show>
      <Show when={!pendingReview() && props.banner}>
        <div class="ae-banner" role="status">{props.banner}</div>
      </Show>
      <Show when={saveState().status === 'saving' && (saveState() as { slow: boolean }).slow}>
        <div class="ae-banner" role="status"><span class="ae-spinner" aria-hidden="true" /><span>{t().save.stillWaiting}</span></div>
      </Show>
    </div>
  )

  return (
    <div ref={root} class="ae-editor" classList={{ 'ae-editor--mobile': mobile(), 'ae-editor--coarse': coarse() }} lang={props.locale}>
      {toolbar()}
      {banners()}
      <Show when={mobile()}>
        <div class="ae-mobile">
          <p class="ae-mobile__note" role="note">{t().states.mobile}</p>
          <Outline graph={draft()} catalog={props.catalog} overlay={props.overlay} locale={props.locale} />
        </div>
      </Show>
      <Show when={props.tab === 'runs' && !mobile()}>
        <div class="ae-runs">{props.runsBody}</div>
      </Show>
      <Show when={!mobile()}>
        <div class="ae-work" classList={{ 'ae-work--hidden': props.tab === 'runs' }}>
          <div class="ae-main">
            <div ref={area} class="ae-area">
              <Canvas
                graph={draft()}
                catalog={props.catalog}
                layout={layout()}
                errors={errorsShown()}
                overlay={props.overlay}
                selected={selected()}
                onSelect={(id) => {
                  setSelected(id)
                  setFocusField(null)
                }}
                onChange={readOnly() ? undefined : (graph) => commitGraph(graph)}
                onMove={readOnly() ? undefined : (positions) => commit({ graph: draft(), layout: { ...layout(), ...positions } })}
                onDrop={readOnly() ? undefined : (drop) => setPalette({ kind: 'add', from: drop.from, anchor: { x: drop.x, y: drop.y }, flow: drop.flow })}
                onMenu={(m) => setMenu(m)}
                onReady={(api) => { canvasApi = api }}
                onTidy={readOnly() ? undefined : tidy}
                layoutTransition={layoutTransition()}
                onOpen={openPanelFocus}
                onEscape={() => false}
                selecting={selecting()}
                snap={snapOn()}
                minimap
                locale={props.locale}
              />
              <Show when={empty()}>
                <div class="ae-empty">
                  <h2 class="ae-empty__title">{t().states.emptyTitle}</h2>
                  <p class="ae-empty__text">{t().states.emptyText}</p>
                  <Show when={!readOnly()}>
                    <button type="button" class="btn btn-primary btn-normal" onClick={() => setPalette({ kind: 'trigger' })}>{t().states.addTrigger}</button>
                  </Show>
                </div>
              </Show>
              <Show when={coarse() && !readOnly()}>
                <div class="ae-touchbar nopan">
                  <button type="button" class="ae-btn ae-btn--secondary" onClick={() => setPalette({ kind: 'add' })}><Icon name="plus" /><span>{t().canvas.add}</span></button>
                  <button type="button" class="ae-btn ae-btn--secondary" aria-pressed={selecting()} onClick={() => setSelecting((v) => !v)}><Icon name="select" /><span>{t().canvas.selectMode}</span></button>
                </div>
              </Show>
              <Show when={props.saveLayout && !readOnly() && layoutState() !== 'idle'}>
                <div class="ae-quiet" role="status" aria-live="polite">
                  <Show when={layoutState() === 'failed'} fallback={layoutState() === 'saving' ? t().save.layoutSaving : t().save.layoutSaved}>
                    <span>{t().save.layoutFailed}</span>
                    <button type="button" class="ae-link" onClick={retryLayout}>{t().save.retry}</button>
                  </Show>
                </div>
              </Show>
              <Show when={palette()}>
                {(mode) => <Suspense><Palette mode={mode()} catalog={props.catalog} graph={draft()} locale={props.locale} bounds={areaSize()} onPick={addFromPalette} onClose={() => setPalette(null)} /></Suspense>}
              </Show>
              <Show when={menu()}>
                {(m) => <Suspense><ContextMenu x={m().x} y={m().y} bounds={areaSize()} items={menuItems()} locale={props.locale} onClose={() => setMenu(null)} /></Suspense>}
              </Show>
            </div>
            <Show when={validationOpen()}>
              <Suspense>
              <ValidationPanel
                state={validationView()}
                graph={draft()}
                catalog={props.catalog}
                locale={props.locale}
                onPick={pickIssue}
                onRetry={() => void validate()}
                onClose={() => setValidationOpen(false)}
              />
              </Suspense>
            </Show>
            {props.panels}
          </div>
          <Show when={selected()}>
            {(id) => (
              <div
                ref={panelRef}
                class="ae-side"
                onKeyDown={(event) => {
                  if (event.key === 'Escape' && !event.defaultPrevented && !(event.target as HTMLElement).closest('[role="listbox"], [aria-expanded="true"]')) {
                    event.preventDefault()
                    const current = id()
                    setSelected(null)
                    canvasApi?.focus(current)
                  }
                }}
              >
                <Suspense fallback={<div class="ae-panel ae-panel--loading" role="status" aria-busy="true" aria-label={t().states.loading} />}>
                <NodePanel
                  graph={draft()}
                  catalog={props.catalog}
                  id={id()}
                  locale={props.locale}
                  readOnly={readOnly()}
                  fieldSlot={props.fieldSlot}
                  errors={allErrors()}
                  focusField={focusField()}
                  onChange={(graph, key) => commitGraph(graph, key)}
                  onClose={() => {
                    const current = id()
                    setSelected(null)
                    canvasApi?.focus(current)
                  }}
                />
                </Suspense>
              </div>
            )}
          </Show>
        </div>
      </Show>
      <Show when={settingsOpen()}>
        <Suspense>
          <SettingsDialog
        open={settingsOpen()}
        graph={draft()}
        locale={props.locale}
        readOnly={editingBlocked()}
        onChange={(patch, key) => commitGraph({ ...draft(), ...patch }, key)}
        onClose={() => setSettingsOpen(false)}
      />
        </Suspense>
      </Show>
      <Show when={versionsMenu()}>
        {(p) => (
          <Suspense>
          <ContextMenu
            x={p().x}
            y={p().y}
            bounds={{ width: window.innerWidth, height: window.innerHeight }}
            locale={props.locale}
            onClose={() => setVersionsMenu(null)}
            items={(props.versions ?? []).map((v) => ({
              key: `v${v.version}`,
              label: `${t().toolbar.version(v.version)}${v.enabled ? ` · ${t().toolbar.enabled}` : ''}${v.note ? ` · ${v.note}` : ''}${v.by ? ` · ${v.by}` : ''}`,
              checked: v.version === base(),
              run: () => props.onOpenVersion?.(v.version),
            }))}
          />
          </Suspense>
        )}
      </Show>
      <Show when={keymapOpen()}>
        <Suspense>
          <KeyMap open locale={props.locale} onClose={() => setKeymapOpen(false)} />
        </Suspense>
      </Show>
      <ConfirmationDialog
        open={confirmReload()}
        onOpenChange={(open) => { if (!open) setConfirmReload(false) }}
        heading={t().save.replaceTitle}
        text={t().save.replaceText(conflict()?.version ?? base())}
        confirmText={t().save.replace}
        cancelText={t().save.cancel}
        closeLabel={t().common.close}
        danger
        busy={reloading()}
        onConfirm={() => void loadLatest()}
      />
    </div>
  )
}

