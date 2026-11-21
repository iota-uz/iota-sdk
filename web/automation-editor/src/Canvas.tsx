// The canvas: a controlled Solid wrapper over @xyflow/system (through ./xy/adapter). It never mutates
// `graph` or `layout`; it emits intents (onChange, onMove, onDrop, onMenu, onSelect). One edge per
// (node, outcome): a new connection replaces the old one. The DOM is read only after mount.
import { batch, createEffect, createMemo, createSignal, For, on, onCleanup, onMount, Show, type Accessor, type JSX } from 'solid-js'
import { createStore, reconcile } from 'solid-js/store'
import { cardGeometry, GRID, HANDLE_SIZE, normalizeRect, OUTCOME_ROW_Y, rectsOverlap, snap, TARGET_HANDLE, type CardGeometry } from './geometry'
import { connect, edgeKey, FIRED, findItem, IGNORE, itemOutcomes, parentOf, pathOf, removeItems } from './graph'
import { Icon } from './icons'
import { backEdges, ensureLayout } from './layout'
import { describeItem } from './names'
import { strings } from './strings'
import type { AutomationLayout, CanvasApi, CanvasItem, CanvasProps, ItemState, XY } from './types'
import { errorsByItem } from './validation'
import {
  createMinimap,
  createNodeDrag,
  createPanZoom,
  edgePath,
  loopPath,
  NodeLookup,
  startConnection,
  toBox,
  toFlow,
  viewportForBounds,
  type ConnectionState,
  type LookupHandle,
  type NodeDrag,
  type PanZoom,
  type Viewport,
} from './xy/adapter'
import { FLOW_CLASS, handleAttributes, isPaneAt, NODE_CLASS, nodeIdAt, NODRAG, NOPAN, NOWHEEL, PANE_CLASS } from './xy/dom'

let flowCounter = 0

export const LONG_PRESS_MS = 550
const LONG_PRESS_SLOP = 10
const HOVER_PATH_MS = 150

type Box = { start: XY; end: XY }

function sameItem(a: CanvasItem | undefined, b: CanvasItem | undefined): boolean {
  if (a === b) return true
  if (!a || !b) return false
  return a.id === b.id && a.kind === b.kind && a.type === b.type && a.state === b.state && a.error === b.error &&
    a.outs.length === b.outs.length && a.outs.every((o, i) => {
      const other = b.outs[i]!
      return o.outcome === other.outcome && o.to === other.to && o.ignore === other.ignore
    })
}

const STATE_ICON: Record<ItemState, string> = {
  passed: 'passed', running: 'running', waiting: 'waiting', failed: 'failed', skipped: 'skipped', cancelled: 'cancelled',
}

export function Canvas(props: CanvasProps): JSX.Element {
  const flowId = `ae${++flowCounter}`
  const t = () => strings(props.locale)
  const editable = () => props.onChange !== undefined
  const movable = () => props.onMove !== undefined

  let container: HTMLDivElement | undefined
  let viewportEl: HTMLDivElement | undefined
  let minimapEl: SVGSVGElement | undefined
  let panZoom: PanZoom | null = null
  const lookup = new NodeLookup()
  const nodeElements = new Map<string, HTMLDivElement>()

  const [viewport, setViewport] = createSignal<Viewport>({ x: 48, y: 48, zoom: 1 })
  const [size, setSize] = createSignal({ width: 0, height: 0 })
  const [multi, setMulti] = createSignal<ReadonlySet<string>>(new Set())
  const [connection, setConnection] = createSignal<ConnectionState | null>(null)
  const [dragging, setDragging] = createSignal(false)
  const [box, setBox] = createSignal<Box | null>(null)
  const [hovered, setHovered] = createSignal<string | null>(null)
  const [press, setPress] = createSignal<XY | null>(null)
  const [altSnapOff, setAltSnapOff] = createSignal(false)
  const [mounted, setMounted] = createSignal(false)

  // ——— derived graph view ———

  const layout = createMemo(() => ensureLayout(props.graph, props.catalog, props.layout))
  const [positions, setPositions] = createStore<AutomationLayout>({})
  createEffect(on(layout, (next) => setPositions(reconcile(structuredClone(next)))))

  const errorMap = createMemo(() => errorsByItem(props.graph, props.errors))

  const itemMap = createMemo(() => {
    const graph = props.graph
    const map = new Map<string, CanvasItem>()
    const edgeIndex = new Map<string, string>()
    for (const e of graph.edges) edgeIndex.set(edgeKey(e.from.node, e.from.outcome), e.to)
    const add = (id: string, kind: 'trigger' | 'node', type: string) => {
      const outs = itemOutcomes(graph, props.catalog, id).map((outcome) => {
        const to = edgeIndex.get(edgeKey(id, outcome)) ?? null
        return { outcome, to: to === IGNORE ? null : to, ignore: to === IGNORE }
      })
      map.set(id, { id, kind, type, outs, state: props.overlay?.states.get(id), error: errorMap().get(id) })
    }
    for (const trigger of graph.triggers) add(trigger.id, 'trigger', trigger.event)
    for (const node of graph.nodes) add(node.id, 'node', node.type)
    return map
  })

  const ids = createMemo<string[]>((previous) => {
    const next = [...itemMap().keys()]
    return previous && previous.length === next.length && previous.every((id, i) => id === next[i]) ? previous : next
  })

  const geometryMap = createMemo(() => {
    const map = new Map<string, CardGeometry>()
    for (const [id, item] of itemMap()) map.set(id, cardGeometry(item.outs.map((o) => o.outcome), item.kind === 'node'))
    return map
  })

  const back = createMemo(() => backEdges(props.graph))

  const edgeList = createMemo<string[]>((previous) => {
    const known = itemMap()
    const next = props.graph.edges
      .filter((e) => e.to !== IGNORE && known.has(e.from.node) && known.has(e.to))
      .map((e) => `${edgeKey(e.from.node, e.from.outcome)}\u0000${e.to}`)
    return previous && previous.length === next.length && previous.every((k, i) => k === next[i]) ? previous : next
  })

  const isSelected = (id: string) => props.selected === id || multi().has(id)
  const selection = () => {
    const out = new Set(multi())
    if (props.selected) out.add(props.selected)
    return [...out].filter((id) => itemMap().has(id))
  }

  createEffect(on(() => props.selected, (id) => {
    if (id && !multi().has(id)) setMulti(new Set([id]))
    if (!id && multi().size <= 1) setMulti(new Set<string>())
  }))

  // Path highlight (ED6): the selected item, or the hovered one after ~150 ms; the run overlay wins.
  let hoverTimer: ReturnType<typeof setTimeout> | undefined
  const highlight = createMemo(() => {
    if (props.overlay || connection() || dragging()) return null
    const id = hovered() ?? props.selected
    if (!id || !itemMap().has(id)) return null
    return pathOf(props.graph, id)
  })

  // ——— lookup for xyflow ———

  const syncLookup = () => {
    const geometries = geometryMap()
    lookup.set(ids().map((id) => {
      const geometry = geometries.get(id)!
      const handles: LookupHandle[] = geometry.outcomes.map((slot) => ({ id: slot.outcome, kind: 'source' as const, x: slot.handle.x, y: slot.handle.y, size: HANDLE_SIZE }))
      if (geometry.target) handles.push({ id: TARGET_HANDLE, kind: 'target' as const, x: geometry.target.x, y: geometry.target.y, size: HANDLE_SIZE })
      return {
        id,
        position: positions[id] ?? { x: 0, y: 0 },
        width: geometry.width,
        height: geometry.height,
        selected: isSelected(id),
        draggable: movable(),
        handles,
      }
    }))
  }

  const panBy = async (delta: XY) => {
    if (!panZoom) return false
    const vp = panZoom.get()
    panZoom.set({ x: vp.x - delta.x, y: vp.y - delta.y, zoom: vp.zoom })
    return true
  }

  const snapOn = () => (props.snap ?? true) && !altSnapOff()

  const dragFactory = createNodeDrag({
    lookup,
    container: () => container ?? null,
    viewport: () => viewport(),
    snap: snapOn,
    grid: GRID,
    enabled: movable,
    panBy,
    onPress: (id) => {
      if (!multi().has(id)) {
        setMulti(new Set([id]))
        props.onSelect(id)
      }
    },
    onStart: () => setDragging(true),
    onMove: (moved) => {
      batch(() => {
        for (const [id, p] of moved) setPositions(id, { x: p.x, y: p.y })
      })
    },
    onEnd: (moved) => {
      setDragging(false)
      const out: AutomationLayout = {}
      let changed = false
      for (const [id, p] of moved) {
        const next = snapOn() ? { x: snap(p.x), y: snap(p.y) } : { x: Math.round(p.x), y: Math.round(p.y) }
        out[id] = next
        const before = layout()[id]
        if (!before || before.x !== next.x || before.y !== next.y) changed = true
      }
      if (changed) props.onMove?.(out)
      setPositions(reconcile(structuredClone(layout())))
    },
  })

  // ——— viewport ———

  const applyViewport = (vp: Viewport) => {
    setViewport(vp)
    if (viewportEl) viewportEl.style.transform = `translate(${vp.x}px, ${vp.y}px) scale(${vp.zoom})`
    if (container) {
      container.style.setProperty('--ae-grid', `${GRID * vp.zoom}px`)
      container.style.setProperty('--ae-grid-x', `${vp.x}px`)
      container.style.setProperty('--ae-grid-y', `${vp.y}px`)
    }
  }

  const bounds = () => {
    const geometries = geometryMap()
    const list = ids()
    if (list.length === 0) return null
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
    for (const id of list) {
      const p = positions[id]
      const g = geometries.get(id)
      if (!p || !g) continue
      minX = Math.min(minX, p.x)
      minY = Math.min(minY, p.y)
      maxX = Math.max(maxX, p.x + g.width)
      maxY = Math.max(maxY, p.y + g.height + 24)
    }
    return { x: minX, y: minY, width: maxX - minX, height: maxY - minY }
  }

  const fit = () => {
    const b = bounds()
    const { width, height } = size()
    if (!b || !panZoom || width === 0) return
    panZoom.set(viewportForBounds(b, width, height, 0.12), 0)
  }

  const center = (): XY => {
    const { width, height } = size()
    return toFlow({ x: width / 2, y: height / 2 }, viewport())
  }

  const focusItem = (id: string) => {
    const element = nodeElements.get(id)
    if (!element) return
    element.focus({ preventScroll: true })
    const p = positions[id]
    const g = geometryMap().get(id)
    const { width, height } = size()
    if (!p || !g || !panZoom || width === 0) return
    const vp = viewport()
    const topLeft = toBox(p, vp)
    const bottomRight = toBox({ x: p.x + g.width, y: p.y + g.height }, vp)
    if (topLeft.x < 0 || topLeft.y < 0 || bottomRight.x > width || bottomRight.y > height) {
      panZoom.set({ x: width / 2 - (p.x + g.width / 2) * vp.zoom, y: height / 2 - (p.y + g.height / 2) * vp.zoom, zoom: vp.zoom }, 150)
    }
  }

  const api: CanvasApi = {
    center,
    fit,
    zoomIn: () => panZoom?.scaleBy(1.2),
    zoomOut: () => panZoom?.scaleBy(1 / 1.2),
    zoomReset: () => panZoom?.scaleTo(1),
    selectAll: () => setMulti(new Set(ids())),
    selection,
    focus: focusItem,
    toBox: (flow) => toBox(flow, viewport()),
  }

  // ——— mount ———

  onMount(() => {
    if (!container) return
    panZoom = createPanZoom(container, {
      viewport: viewport(),
      onTransform: applyViewport,
      onContextMenu: (event) => openMenu(event.clientX, event.clientY, null, event),
    })
    applyViewport(viewport())
    const resize = new ResizeObserver(() => {
      if (!container) return
      setSize({ width: container.clientWidth, height: container.clientHeight })
    })
    resize.observe(container)
    setSize({ width: container.clientWidth, height: container.clientHeight })
    fit()
    const onKey = (event: KeyboardEvent) => setAltSnapOff(event.altKey)
    const onPointer = (event: MouseEvent) => setAltSnapOff(event.altKey)
    window.addEventListener('keydown', onKey, true)
    window.addEventListener('keyup', onKey, true)
    window.addEventListener('mousemove', onPointer, true)
    setMounted(true)
    props.onReady?.(api)
    onCleanup(() => {
      resize.disconnect()
      window.removeEventListener('keydown', onKey, true)
      window.removeEventListener('keyup', onKey, true)
      window.removeEventListener('mousemove', onPointer, true)
      panZoom?.destroy()
      panZoom = null
    })
  })

  createEffect(() => {
    panZoom?.update({ panOnDrag: true, selecting: Boolean(props.selecting) || box() !== null, connecting: connection() !== null })
  })

  // Minimap and "Fit" keep working after edits (ED11).
  createEffect(() => {
    if (!mounted() || !props.minimap || !minimapEl || !panZoom) return
    const map = createMinimap(minimapEl, panZoom, () => {
      const b = bounds()
      return b ? Math.max(b.width / 200, b.height / 140, 1) : 1
    })
    map.update(size().width, size().height)
    onCleanup(() => map.destroy())
  })

  // ——— menus, long press, box selection ———

  const openMenu = (clientX: number, clientY: number, item: string | null, event?: Event) => {
    event?.preventDefault()
    if (!props.onMenu || !container) return
    const rect = container.getBoundingClientRect()
    const x = clientX - rect.left
    const y = clientY - rect.top
    if (item && !isSelected(item)) {
      setMulti(new Set([item]))
      props.onSelect(item)
    }
    props.onMenu({ x, y, flow: toFlow({ x, y }, viewport()), item })
  }

  let longPress: { timer: ReturnType<typeof setTimeout>; start: XY; pointer: number } | null = null
  const cancelLongPress = () => {
    if (longPress) clearTimeout(longPress.timer)
    longPress = null
    setPress(null)
  }
  const activePointers = new Set<number>()
  let downAt: XY | null = null

  const onPointerDown = (event: PointerEvent) => {
    activePointers.add(event.pointerId)
    downAt = { x: event.clientX, y: event.clientY }
    syncLookup()
    if (activePointers.size > 1) {
      cancelLongPress()
      return
    }
    const target = event.target as Element
    const item = target.closest(`.${NODE_CLASS}`)?.getAttribute('data-id') ?? null
    if (event.pointerType === 'touch' && !target.closest(`.ae-handle, .ae-controls, .ae-minimap`)) {
      const rect = container!.getBoundingClientRect()
      setPress({ x: event.clientX - rect.left, y: event.clientY - rect.top })
      const start = { x: event.clientX, y: event.clientY }
      longPress = {
        start,
        pointer: event.pointerId,
        timer: setTimeout(() => {
          setPress(null)
          longPress = null
          openMenu(start.x, start.y, item)
        }, LONG_PRESS_MS),
      }
    }
    const onPane = !item && !target.closest(`.${NOPAN}`)
    if (onPane && (event.shiftKey || props.selecting) && event.button === 0) {
      const rect = container!.getBoundingClientRect()
      const p = { x: event.clientX - rect.left, y: event.clientY - rect.top }
      setBox({ start: p, end: p })
      container!.setPointerCapture(event.pointerId)
      cancelLongPress()
    }
  }

  const onPointerMove = (event: PointerEvent) => {
    if (longPress && longPress.pointer === event.pointerId) {
      const dx = event.clientX - longPress.start.x
      const dy = event.clientY - longPress.start.y
      if (dx * dx + dy * dy > LONG_PRESS_SLOP * LONG_PRESS_SLOP) cancelLongPress()
    }
    const current = box()
    if (current) {
      const rect = container!.getBoundingClientRect()
      setBox({ start: current.start, end: { x: event.clientX - rect.left, y: event.clientY - rect.top } })
    }
  }

  const onPointerUp = (event: PointerEvent) => {
    activePointers.delete(event.pointerId)
    cancelLongPress()
    const current = box()
    if (!current) return
    setBox(null)
    const vp = viewport()
    const a = toFlow(current.start, vp)
    const b = toFlow(current.end, vp)
    const area = normalizeRect(a, b)
    if (area.width < 4 && area.height < 4) return
    const hit = ids().filter((id) => {
      const p = positions[id]
      const g = geometryMap().get(id)
      return p && g && rectsOverlap(area, { x: p.x, y: p.y, width: g.width, height: g.height })
    })
    const next = new Set(event.shiftKey ? [...multi(), ...hit] : hit)
    setMulti(next)
    props.onSelect(hit.length === 1 ? hit[0]! : next.size > 0 ? (props.selected && next.has(props.selected) ? props.selected : null) : null)
  }

  const onPaneClick = (event: MouseEvent) => {
    if (downAt && Math.hypot(event.clientX - downAt.x, event.clientY - downAt.y) > 4) return
    const target = event.target as Element
    if (target.closest(`.${NODE_CLASS}, .${NOPAN}`)) return
    setMulti(new Set<string>())
    props.onSelect(null)
  }

  // ——— node interactions ———

  const selectFromClick = (id: string, event: MouseEvent) => {
    if (event.shiftKey || event.metaKey || event.ctrlKey) {
      const next = new Set(multi())
      if (next.has(id)) next.delete(id)
      else next.add(id)
      setMulti(next)
      props.onSelect(next.has(id) ? id : null)
      return
    }
    setMulti(new Set([id]))
    props.onSelect(id)
  }

  let keyMove: Set<string> | null = null
  let keyMoveTimer: ReturnType<typeof setTimeout> | undefined
  const commitKeyMove = () => {
    clearTimeout(keyMoveTimer)
    if (!keyMove) return
    const out: AutomationLayout = {}
    for (const id of keyMove) {
      const p = positions[id]
      if (p) out[id] = { x: p.x, y: p.y }
    }
    keyMove = null
    props.onMove?.(out)
    setPositions(reconcile(structuredClone(layout())))
  }

  const nearest = (id: string, key: string): string | null => {
    const p = positions[id]
    if (!p) return null
    const g = geometryMap().get(id)!
    const from = { x: p.x + g.width / 2, y: p.y + g.height / 2 }
    let best: string | null = null
    let bestScore = Infinity
    for (const other of ids()) {
      if (other === id) continue
      const q = positions[other]
      const h = geometryMap().get(other)
      if (!q || !h) continue
      const dx = q.x + h.width / 2 - from.x
      const dy = q.y + h.height / 2 - from.y
      const forward = key === 'ArrowRight' ? dx : key === 'ArrowLeft' ? -dx : key === 'ArrowDown' ? dy : -dy
      const side = key === 'ArrowRight' || key === 'ArrowLeft' ? Math.abs(dy) : Math.abs(dx)
      if (forward <= 0) continue
      const score = forward + side * 2
      if (score < bestScore) {
        bestScore = score
        best = other
      }
    }
    return best
  }

  const onNodeKeyDown = (id: string, event: KeyboardEvent) => {
    if (event.target !== event.currentTarget) return
    const key = event.key
    if (key.startsWith('Arrow') && event.altKey && movable()) {
      event.preventDefault()
      const delta = { ArrowLeft: { x: -GRID, y: 0 }, ArrowRight: { x: GRID, y: 0 }, ArrowUp: { x: 0, y: -GRID }, ArrowDown: { x: 0, y: GRID } }[key]!
      const moving = selection().includes(id) ? selection() : [id]
      keyMove = new Set([...(keyMove ?? []), ...moving])
      // One run of Alt+arrows is one step: it is committed when Alt is released, focus leaves, or after a pause.
      clearTimeout(keyMoveTimer)
      keyMoveTimer = setTimeout(commitKeyMove, 1000)
      batch(() => {
        for (const m of moving) {
          const p = positions[m]
          if (p) setPositions(m, { x: p.x + delta.x, y: p.y + delta.y })
        }
      })
      return
    }
    if (key.startsWith('Arrow') && !event.altKey && !event.metaKey && !event.ctrlKey) {
      event.preventDefault()
      const next = nearest(id, key)
      if (next) {
        setMulti(new Set([next]))
        props.onSelect(next)
        focusItem(next)
      }
      return
    }
    if (key === 'Enter') {
      event.preventDefault()
      props.onOpen?.(id)
      return
    }
    if ((key === 'Delete' || key === 'Backspace') && editable()) {
      event.preventDefault()
      const gone = selection().includes(id) ? selection() : [id]
      const parent = parentOf(props.graph, id)
      const graph = removeItems(props.graph, gone)
      setMulti(new Set<string>())
      props.onChange?.(graph)
      const focusTarget = parent && !gone.includes(parent) ? parent : null
      props.onSelect(focusTarget)
      if (focusTarget) queueMicrotask(() => focusItem(focusTarget))
      else container?.focus()
      return
    }
    if (key === 'ContextMenu' || (key === 'F10' && event.shiftKey)) {
      event.preventDefault()
      const element = nodeElements.get(id)
      const rect = element?.getBoundingClientRect()
      if (rect) openMenu(rect.left + rect.width / 2, rect.top + rect.height / 2, id)
    }
  }

  const onNodeKeyUp = (event: KeyboardEvent) => {
    if (keyMove && event.key === 'Alt') commitKeyMove()
  }

  const onRootKeyDown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') {
      if (props.onEscape?.()) {
        event.preventDefault()
        return
      }
      if (multi().size > 0 || props.selected) {
        event.preventDefault()
        setMulti(new Set<string>())
        props.onSelect(null)
      }
    }
    if ((event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey)) && event.target === container) {
      event.preventDefault()
      const rect = container.getBoundingClientRect()
      openMenu(rect.left + rect.width / 2, rect.top + rect.height / 2, null)
    }
  }

  // ——— connections ———

  const connectingKey = () => {
    const c = connection()
    return c ? edgeKey(c.fromNode, c.fromHandle) : null
  }

  const onHandleDown = (id: string, outcome: string, event: MouseEvent | TouchEvent) => {
    if (!editable() || !container) return
    if ('button' in event && event.button !== 0) return
    event.stopPropagation()
    syncLookup()
    const from = { node: id, outcome }
    startConnection(event, {
      container,
      lookup,
      flowId,
      nodeId: id,
      handleId: outcome,
      handleElement: event.currentTarget as Element,
      viewport: () => viewport(),
      panBy,
      isValid: (target) => findItem(props.graph, target)?.kind === 'node',
      onUpdate: setConnection,
      onConnect: (target) => props.onChange?.(connect(props.graph, from, target)),
      onRelease: (point, flow) => {
        const root = container!.getRootNode() as Document | ShadowRoot
        const target = nodeIdAt(root, point.clientX, point.clientY)
        if (target) {
          if (findItem(props.graph, target)?.kind === 'node') props.onChange?.(connect(props.graph, from, target))
          return
        }
        if (isPaneAt(root, container!, point.clientX, point.clientY)) {
          const rect = container!.getBoundingClientRect()
          props.onDrop?.({ from, x: point.clientX - rect.left, y: point.clientY - rect.top, flow })
        }
      },
    })
  }

  const connectionHint = createMemo(() => {
    const c = connection()
    if (!c) return null
    const title = (id: string) => describeItem(props.graph, props.catalog, id, props.locale ?? 'en').title
    const from = `${title(c.fromNode)}.${c.fromHandle}`
    const target = c.overTarget && c.valid ? title(c.overTarget) : '…'
    const old = props.graph.edges.find((e) => e.from.node === c.fromNode && e.from.outcome === c.fromHandle)
    if (!old) return t().canvas.connect(from, target)
    const was = old.to === IGNORE ? t().canvas.ignored : findItem(props.graph, old.to) ? title(old.to) : t().canvas.nowhere
    return t().canvas.rewire(from, target, was)
  })

  // ——— rendering ———

  const hint = () => {
    if (connectionHint()) return connectionHint()
    if (!editable()) return t().canvas.hintReadOnly
    const coarse = typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches
    return coarse ? t().canvas.hintTouch : t().canvas.hintMouse
  }

  const edgeView = (key: string) => {
    const [source, to] = key.split('\u0000') as [string, string]
    const dot = source.lastIndexOf('.')
    const node = source.slice(0, dot)
    const outcome = source.slice(dot + 1)
    const path = createMemo(() => {
      const p = positions[node]
      const q = positions[to]
      const g = geometryMap().get(node)
      const slot = g?.outcomes.find((o) => o.outcome === outcome)
      const target = geometryMap().get(to)?.target
      if (!p || !q || !slot || !target) return ''
      const a = { x: p.x + slot.handle.x, y: p.y + slot.handle.y }
      const b = { x: q.x + target.x, y: q.y + target.y }
      return back().has(`${node}->${to}`) || b.x < a.x - GRID ? loopPath(a, b) : edgePath(a, b)
    })
    const taken = () => props.overlay?.taken.has(source) ?? false
    const dim = () => {
      const h = highlight()
      if (props.overlay) return !taken()
      return h ? !h.edges.has(source) : false
    }
    return (
      <path
        class="ae-edge"
        classList={{ 'ae-edge--hidden': connectingKey() === source, 'ae-edge--taken': taken(), 'ae-dim': dim(), 'ae-edge--loop': back().has(`${node}->${to}`) }}
        d={path()}
        data-edge={source}
        data-to={to}
      />
    )
  }

  const NodeCard = (cardProps: { id: string }) => {
    const id = cardProps.id
    const item = createMemo(() => itemMap().get(id), undefined, { equals: sameItem }) as Accessor<CanvasItem>
    const geometry = createMemo(() => geometryMap().get(id) ?? cardGeometry([], true))
    const name = createMemo(() => describeItem(props.graph, props.catalog, id, props.locale ?? 'en'))
    const position = () => positions[id] ?? { x: 0, y: 0 }
    const selected = createMemo(() => isSelected(id))
    const dim = () => {
      if (props.overlay) return !props.overlay.states.has(id)
      const h = highlight()
      return h ? !h.items.has(id) : false
    }
    const unavailable = () => item()?.kind === 'node' && !props.catalog.nodes.some((n) => n.type === item()?.type)
    let element: HTMLDivElement | undefined
    let drag: NodeDrag | null = null

    onMount(() => {
      if (!element) return
      nodeElements.set(id, element)
      drag = dragFactory()
      drag.bind(element, id)
    })
    onCleanup(() => {
      drag?.destroy()
      if (nodeElements.get(id) === element) nodeElements.delete(id)
    })

    const outLabel = (out: CanvasItem['outs'][number]) => {
      if (out.ignore) return t().canvas.ignored
      if (!out.to) return t().canvas.notConnected
      return describeItem(props.graph, props.catalog, out.to, props.locale ?? 'en').title
    }

    return (
      <div
        ref={element}
        class={`${NODE_CLASS} ${NOPAN} ae-card`}
        classList={{
          'ae-card--trigger': item()?.kind === 'trigger',
          'ae-card--selected': selected(),
          'ae-card--error': Boolean(item()?.error) || unavailable(),
          'ae-dim': dim(),
          [`ae-card--${item()?.state ?? 'idle'}`]: Boolean(item()?.state),
        }}
        style={{
          transform: `translate(${position().x}px, ${position().y}px)`,
          width: `${geometry().width}px`,
          height: `${geometry().height}px`,
        }}
        data-id={id}
        role="group"
        tabindex="0"
        aria-label={item()?.kind === 'trigger' ? `${t().canvas.trigger}: ${name().title}` : t().canvas.node(name().title)}
        aria-current={selected() ? 'true' : undefined}
        onClick={(event) => selectFromClick(id, event)}
        onDblClick={() => props.onOpen?.(id)}
        onKeyDown={(event) => onNodeKeyDown(id, event)}
        onKeyUp={onNodeKeyUp}
        onFocusOut={() => commitKeyMove()}
        onContextMenu={(event) => {
          event.stopPropagation()
          openMenu(event.clientX, event.clientY, id, event)
        }}
        onPointerEnter={(event) => {
          if (event.pointerType !== 'mouse') return
          clearTimeout(hoverTimer)
          hoverTimer = setTimeout(() => setHovered(id), HOVER_PATH_MS)
        }}
        onPointerLeave={() => {
          clearTimeout(hoverTimer)
          if (hovered() === id) setHovered(null)
        }}
      >
        <Show when={geometry().target}>
          {(target) => (
            <div
              {...handleAttributes(flowId, id, TARGET_HANDLE, 'target', editable())}
              class={`${handleAttributes(flowId, id, TARGET_HANDLE, 'target', editable()).class} ae-handle ae-handle--target`}
              style={{ left: `${target().x}px`, top: `${target().y}px` }}
              aria-hidden="true"
            />
          )}
        </Show>
        <Show when={props.renderNode} fallback={
          <div class="ae-card__body">
            <div class="ae-card__head">
              <span class="ae-card__icon"><Icon name={name().icon ?? (item()?.kind === 'trigger' ? 'event' : props.catalog.nodes.find((n) => n.type === item()?.type)?.behavior ?? 'node')} /></span>
              <span class="ae-card__title" title={name().title}>{name().title}</span>
              <Show when={item()?.state}>
                {(state) => (
                  <span class={`ae-state ae-state--${state()}`} role="img" aria-label={t().canvas.states[state()]}>
                    <Icon name={STATE_ICON[state()]} size={12} />
                    <span class="ae-state__text">{t().canvas.states[state()]}</span>
                  </span>
                )}
              </Show>
              <Show when={props.overlay?.badges.get(id)}>
                {(badge) => <span class="ae-badge">{badge()}</span>}
              </Show>
              <Show when={item()?.error || unavailable()}>
                <span class="ae-badge ae-badge--error" role="img" aria-label={unavailable() ? t().canvas.unavailable : t().canvas.error} title={item()?.error ?? t().canvas.unavailable}>
                  <Icon name="error" size={12} />
                  <span>{unavailable() ? t().canvas.unavailable : '!'}</span>
                </span>
              </Show>
            </div>
            <div class="ae-card__sub" title={name().subtitle}>{name().subtitle}</div>
          </div>
        }>
          {(render) => <div class="ae-card__body">{render()(item)}</div>}
        </Show>
        <div class="ae-card__outs">
          <For each={geometry().outcomes}>
            {(slot) => {
              const out = () => item()?.outs.find((o) => o.outcome === slot.outcome)
              const taken = () => props.overlay?.taken.has(edgeKey(id, slot.outcome)) ?? false
              const attributes = () => handleAttributes(flowId, id, slot.outcome, 'source', editable())
              return (
                <>
                  <span
                    class="ae-out"
                    classList={{ 'ae-out--open': !out()?.to && !out()?.ignore, 'ae-out--ignore': Boolean(out()?.ignore), 'ae-out--taken': taken() }}
                    style={{ left: `${slot.x}px`, width: `${slot.width}px`, top: `${OUTCOME_ROW_Y}px` }}
                    title={t().canvas.outcome(slot.outcome, out() ? outLabel(out()!) : '')}
                  >
                    {slot.outcome === FIRED && item()?.kind === 'trigger' ? '→' : slot.outcome}
                  </span>
                  <div
                    {...attributes()}
                    class={`${attributes().class} ae-handle ae-handle--source`}
                    classList={{ 'ae-handle--open': !out()?.to && !out()?.ignore }}
                    style={{ left: `${slot.handle.x}px`, top: `${slot.handle.y}px` }}
                    role={editable() ? 'button' : undefined}
                    aria-label={t().canvas.outcome(slot.outcome, out() ? outLabel(out()!) : '')}
                    onMouseDown={(event) => onHandleDown(id, slot.outcome, event)}
                    onTouchStart={(event) => onHandleDown(id, slot.outcome, event)}
                  />
                  <Show when={!out()?.to}>
                    <span class={`ae-out-note ${NODRAG}`} style={{ left: `${slot.handle.x}px`, top: `${slot.handle.y + 10}px` }}>
                      {out()?.ignore ? `× ${t().canvas.ignored}` : t().canvas.notConnected}
                    </span>
                  </Show>
                </>
              )
            }}
          </For>
        </div>
      </div>
    )
  }

  const connectionPath = () => {
    const c = connection()
    if (!c) return ''
    return edgePath(c.from, c.to)
  }

  const boxRect = () => {
    const b = box()
    return b ? normalizeRect(b.start, b.end) : null
  }

  const minimapView = createMemo(() => {
    const b = bounds()
    if (!b) return null
    const pad = 80
    return { x: b.x - pad, y: b.y - pad, width: b.width + pad * 2, height: b.height + pad * 2 }
  })

  const viewportRect = () => {
    const { width, height } = size()
    const vp = viewport()
    const a = toFlow({ x: 0, y: 0 }, vp)
    return { x: a.x, y: a.y, width: width / vp.zoom, height: height / vp.zoom }
  }

  return (
    <div
      ref={container}
      class={`${FLOW_CLASS} ae-canvas`}
      classList={{ 'ae-canvas--readonly': !editable(), 'ae-canvas--connecting': connection() !== null, 'ae-canvas--selecting': Boolean(props.selecting) }}
      role="application"
      aria-label={props.label ?? t().canvas.label}
      aria-roledescription="canvas"
      tabindex="-1"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onClick={onPaneClick}
      onKeyDown={onRootKeyDown}
      onContextMenu={(event) => {
        if ((event.target as Element).closest(`.${NODE_CLASS}`)) return
        openMenu(event.clientX, event.clientY, null, event)
      }}
    >
      <div class={`${PANE_CLASS} ae-pane`} />
      <div ref={viewportEl} class="ae-viewport">
        <svg class="ae-edges" aria-hidden="true">
          <For each={edgeList()}>{(key) => edgeView(key)}</For>
          <Show when={connection()}>
            <path class="ae-edge ae-edge--draft" classList={{ 'ae-edge--valid': connection()?.valid === true }} d={connectionPath()} />
          </Show>
        </svg>
        <For each={ids()}>{(id) => <NodeCard id={id} />}</For>
      </div>
      <Show when={boxRect()}>
        {(rect) => <div class="ae-box" style={{ left: `${rect().x}px`, top: `${rect().y}px`, width: `${rect().width}px`, height: `${rect().height}px` }} />}
      </Show>
      <Show when={press()}>
        {(p) => <div class="ae-press" style={{ left: `${p().x}px`, top: `${p().y}px` }} aria-hidden="true" />}
      </Show>
      <div class={`ae-hint ${NOPAN}`} classList={{ 'ae-hint--active': connection() !== null }} role="status" aria-live="polite">
        {hint()}
      </div>
      <div class={`ae-controls ${NOPAN} ${NOWHEEL}`} role="toolbar" aria-label={t().canvas.label}>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.zoomIn} title={t().canvas.zoomIn} onClick={() => api.zoomIn()}><Icon name="plus" /></button>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.zoomOut} title={t().canvas.zoomOut} onClick={() => api.zoomOut()}><Icon name="minus" /></button>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.fit} title={t().canvas.fit} onClick={() => api.fit()}><Icon name="fit" /></button>
        <button type="button" class="ae-iconbtn ae-iconbtn--text" aria-label={t().canvas.zoomReset} title={t().canvas.zoomReset} onClick={() => api.zoomReset()}>
          {Math.round(viewport().zoom * 100)}%
        </button>
      </div>
      <Show when={props.minimap}>
        <svg
          ref={minimapEl}
          class={`ae-minimap ${NOPAN} ${NOWHEEL}`}
          width="200"
          height="140"
          viewBox={minimapView() ? `${minimapView()!.x} ${minimapView()!.y} ${minimapView()!.width} ${minimapView()!.height}` : '0 0 200 140'}
          preserveAspectRatio="xMidYMid meet"
          role="img"
          aria-label={t().canvas.minimap}
        >
          <For each={ids()}>
            {(id) => {
              const g = () => geometryMap().get(id)
              return <rect class="ae-minimap__node" classList={{ 'ae-minimap__node--selected': isSelected(id) }} x={positions[id]?.x ?? 0} y={positions[id]?.y ?? 0} width={g()?.width ?? 0} height={g()?.height ?? 0} rx="8" />
            }}
          </For>
          <rect class="ae-minimap__view" x={viewportRect().x} y={viewportRect().y} width={viewportRect().width} height={viewportRect().height} />
        </svg>
      </Show>
    </div>
  )
}
