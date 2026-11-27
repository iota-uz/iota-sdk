// The canvas: a controlled Solid wrapper over @xyflow/system (through ./xy/adapter). It never mutates
// `graph` or `layout`; it emits intents (onChange, onMove, onDrop, onMenu, onSelect). One edge per
// (node, outcome): a new connection replaces the old one. The DOM is read only after mount.
import { batch, createEffect, createMemo, createSignal, For, on, onCleanup, onMount, Show, untrack, type Accessor, type JSX } from 'solid-js'
import { createStore, reconcile } from 'solid-js/store'
import { cardGeometry, GRID, HANDLE_SIZE, normalizeRect, OUTCOME_ROW_Y, rectsOverlap, snap, TARGET_HANDLE, type CardGeometry, type Rect } from './geometry'
import { connect, disconnect, edgeKey, FIRED, findItem, IGNORE, nodeOutcomes, parentOf, pathOf, removeItems } from './graph'
import { Icon } from './icons'
import { backEdges, ensureLayout, unmarked } from './layout'
import { isMac } from './locale'
import { describeAll, type ItemName } from './names'
import { ARROW_GAP, createRouter, directPath, fitRoute, routePath, trimEnd, type Point, type RouteLink, type Router } from './route'
import { toneOf, type Tone } from './tone'
import { motionMs } from './motion'
import { keyLabel, strings } from './strings'
import type { AutomationLayout, CanvasApi, CanvasItem, CanvasProps, ItemState, XY } from './types'
import { connectionLost, errorsByItem } from './validation'
import {
  createNodeDrag,
  createPanZoom,
  edgePath,
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
import { FLOW_CLASS, HANDLE_CLASS, handleDataId, isPaneAt, NODE_CLASS, nodeIdAt, NODRAG, NOPAN, NOWHEEL, PANE_CLASS } from './xy/dom'

let flowCounter = 0

export const LONG_PRESS_MS = 550
const LONG_PRESS_SLOP = 10
const HOVER_PATH_MS = 150

type Box = { start: XY; end: XY }

const EMPTY_GEOMETRY = cardGeometry([], true)

function sameItem(a: CanvasItem | undefined, b: CanvasItem | undefined): boolean {
  if (a === b) return true
  if (!a || !b) return false
  return a.id === b.id && a.kind === b.kind && a.type === b.type && a.state === b.state && a.error === b.error && a.warning === b.warning &&
    a.outs.length === b.outs.length && a.outs.every((o, i) => {
      const other = b.outs[i]!
      return o.outcome === other.outcome && o.to === other.to && o.ignore === other.ignore
    })
}

const STATE_ICON: Record<ItemState, string> = {
  passed: 'passed', running: 'running', waiting: 'waiting', failed: 'failed', skipped: 'skipped', cancelled: 'cancelled',
}

const coarsePointer = () => typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches

export function Canvas(props: CanvasProps): JSX.Element {
  const flowId = `ae${++flowCounter}`
  const t = () => strings(props.locale)
  const editable = () => props.onChange !== undefined
  const movable = () => props.onMove !== undefined

  let container: HTMLDivElement | undefined
  let viewportEl: HTMLDivElement | undefined
  let gridEl: HTMLDivElement | undefined
  let gridStep = 0
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
  // An edge picked by a click (Delete removes it) and the edge under the pointer, by `node.outcome`.
  const [selectedEdge, setSelectedEdge] = createSignal<string | null>(null)
  const [hoveredEdge, setHoveredEdge] = createSignal<string | null>(null)
  const [press, setPress] = createSignal<XY | null>(null)
  const [altSnapOff, setAltSnapOff] = createSignal(false)
  const [tidying, setTidying] = createSignal(false)
  // Motion (#1352): an edge just connected (it flashes) and a draft line pulling back into its handle. The
  // lift, landing and click of a card are classes set on its element directly, not state every card reads:
  // a drop on a 200-card canvas must not recompute every card (#1331 benchmark).
  const [flashEdge, setFlashEdge] = createSignal<string | null>(null)
  let lifted: string[] = []
  const mark = (ids: Iterable<string>, name: string, on: boolean) => {
    for (const id of ids) nodeElements.get(id)?.classList.toggle(name, on)
  }
  const [retract, setRetract] = createSignal<{ d: string; x: number; y: number } | null>(null)
  const motionTimers = new Set<ReturnType<typeof setTimeout>>()
  const later = (ms: number, run: () => void) => {
    const timer = setTimeout(() => {
      motionTimers.delete(timer)
      run()
    }, ms)
    motionTimers.add(timer)
  }
  onCleanup(() => motionTimers.forEach((timer) => clearTimeout(timer)))
  const click = (id: string) => {
    const element = nodeElements.get(id)
    if (!element) return
    element.classList.remove('ae-card--click')
    // A rendered frame without the class, so the animation starts over.
    requestAnimationFrame(() => element.classList.add('ae-card--click'))
    later(motionMs('fast') + 32, () => element.classList.remove('ae-card--click'))
  }

  // ——— derived graph view ———

  // The routes found so far: the ends they join and the path; and which router found each. `routeTick`
  // moves once per slice of routing.
  const [routed, setRouted] = createStore<Record<string, { points: Point[]; clear?: number }>>({})
  const routeOwner = new Map<string, object>()
  // The box of every route in `routed`, read plainly by the visibility check (`routeTick` says when it changed).
  const routeBoxes = new Map<string, Rect>()
  const [routeTick, setRouteTick] = createSignal(0)

  const layout = createMemo(() => ensureLayout(props.graph, props.catalog, props.layout))
  // What is drawn and routed: whole numbers, without the mark Tidy leaves in a position.
  const shown = createMemo(() => unmarked(layout()))
  const [positions, setPositions] = createStore<AutomationLayout>({})
  // `committed` is the layout as the host holds it, `positions` what is on screen (it follows a drag). Both are
  // stores that reconcile, so an edge only re-reads them when a position it uses really changed.
  const [committed, setCommitted] = createStore<AutomationLayout>({})
  // Routes that cross a card that moved are drawn again before anything else: until then they show a plain path.
  const stale = new Set<string>()
  createEffect(on(shown, (next) => {
    const keys = [...routeBoxes.keys()]
    if (keys.length > 0) {
      const geometries = geometryMap()
      const moved: Rect[] = []
      for (const [id, p] of Object.entries(next)) {
        const before = committed[id]
        const g = geometries.get(id)
        if (g && (!before || before.x !== p.x || before.y !== p.y)) moved.push({ x: p.x - 8, y: p.y - 8, width: g.width + 16, height: g.height + 16 })
      }
      if (moved.length > 0) {
        batch(() => {
          for (const key of keys) {
            const found = routeBoxes.get(key)
            if (!found || !moved.some((r) => rectsOverlap(r, found))) continue
            setRouted(key, undefined as never)
            routeBoxes.delete(key)
            routeOwner.delete(key)
            stale.add(key)
          }
        })
      }
    }
    // A layout change that should animate (Tidy, and undo or redo of one) glides the cards to their new
    // places in one --motion-slow tween, and the edges follow them frame by frame (they read `positions`).
    const token = untrack(() => props.layoutTransition)
    const animate = token !== undefined && token !== appliedTransition && measured()
    appliedTransition = token
    const from: AutomationLayout | null = animate ? untrack(() => Object.fromEntries(Object.entries(positions).map(([id, p]) => [id, { x: p.x, y: p.y }]))) : null
    setCommitted(reconcile(structuredClone(next)))
    // The token is read only for a change that animates: reading it costs a style recalculation.
    const ms = from ? motionMs('slow') : 0
    if (!from || ms <= 0) {
      cancelAnimationFrame(tweenFrame)
      setTidying(false)
      setPositions(reconcile(structuredClone(next)))
      return
    }
    tween(from, next, ms)
  }))

  let appliedTransition = props.layoutTransition
  let tweenFrame = 0
  // --ease-standard, cubic-bezier(0.2, 0, 0.2, 1), sampled for the tween.
  const easeStandard = (t: number) => {
    let lo = 0, hi = 1, u = t
    for (let i = 0; i < 12; i++) {
      u = (lo + hi) / 2
      const x = 3 * (1 - u) * (1 - u) * u * 0.2 + 3 * (1 - u) * u * u * 0.2 + u * u * u
      if (x < t) lo = u
      else hi = u
    }
    return 3 * (1 - u) * u * u + u * u * u
  }
  const tween = (from: AutomationLayout, to: AutomationLayout, ms: number) => {
    cancelAnimationFrame(tweenFrame)
    // Only cards on screen at the start or the end glide; the others take their place at once (the glide
    // costs what is seen, as rendering does).
    const rect = untrack(visibleRect)
    const geometries = untrack(geometryMap)
    const seen = (p: XY | undefined, id: string) => {
      const g = geometries.get(id)
      return rect === null || !p || !g || rectsOverlap(rect, { x: p.x, y: p.y, width: g.width, height: g.height + 24 })
    }
    const moving = Object.keys(to).filter((id) => from[id] && (from[id]!.x !== to[id]!.x || from[id]!.y !== to[id]!.y) && (seen(from[id], id) || seen(to[id], id)))
    if (moving.length === 0) {
      setPositions(reconcile(structuredClone(to)))
      return
    }
    // Cards that do not move (and new ones) take their place at once.
    setPositions(reconcile(Object.fromEntries(Object.entries(to).map(([id, p]) => [id, moving.includes(id) ? { ...from[id]! } : { ...p }]))))
    setTidying(true)
    const start = performance.now()
    // The frame's own clock, not the rAF timestamp: some hosts start that one elsewhere.
    const frame = () => {
      const k = Math.min(1, (performance.now() - start) / ms)
      const e = easeStandard(k)
      batch(() => {
        for (const id of moving) {
          const a = from[id]!
          const b = to[id]!
          setPositions(id, { x: Math.round(a.x + (b.x - a.x) * e), y: Math.round(a.y + (b.y - a.y) * e) })
        }
      })
      if (k < 1) tweenFrame = requestAnimationFrame(frame)
      else {
        setTidying(false)
        setPositions(reconcile(structuredClone(to)))
      }
    }
    tweenFrame = requestAnimationFrame(frame)
  }
  onCleanup(() => cancelAnimationFrame(tweenFrame))
  // A drag or a keyboard move takes the positions over: a glide in flight lands at once, so its next frame
  // cannot overwrite the move.
  const stopTween = () => {
    if (!untrack(tidying)) return
    cancelAnimationFrame(tweenFrame)
    setTidying(false)
    setPositions(reconcile(structuredClone(untrack(() => shown()))))
  }

  const errorMap = createMemo(() => errorsByItem(props.graph, props.errors))
  const lost = createMemo(() => connectionLost(props.graph, props.errors))

  const itemMap = createMemo(() => {
    const graph = props.graph
    const map = new Map<string, CanvasItem>()
    const edgeIndex = new Map<string, string>()
    const extra = new Map<string, string[]>()
    for (const e of graph.edges) {
      edgeIndex.set(edgeKey(e.from.node, e.from.outcome), e.to)
      const list = extra.get(e.from.node)
      if (list) list.push(e.from.outcome)
      else extra.set(e.from.node, [e.from.outcome])
    }
    const specs = new Map(props.catalog.nodes.map((n) => [n.type, n]))
    const errors = errorMap()
    const warnings = errorsByItem(props.graph, props.warnings)
    const states = props.overlay?.states
    const add = (id: string, kind: 'trigger' | 'node', type: string, outcomes: string[]) => {
      for (const outcome of extra.get(id) ?? []) if (!outcomes.includes(outcome)) outcomes.push(outcome)
      const outs = outcomes.map((outcome) => {
        const to = edgeIndex.get(edgeKey(id, outcome)) ?? null
        return { outcome, to: to === IGNORE ? null : to, ignore: to === IGNORE }
      })
      map.set(id, { id, kind, type, outs, state: states?.get(id), error: errors.get(id), warning: warnings.get(id) })
    }
    for (const trigger of graph.triggers) add(trigger.id, 'trigger', trigger.event, [FIRED])
    for (const node of graph.nodes) add(node.id, 'node', node.type, nodeOutcomes(node, specs.get(node.type)))
    return map
  })

  const names = createMemo(() => describeAll(props.graph, props.catalog, props.locale ?? 'en'), undefined, {
    equals: (a, b) => a.size === b.size && [...a].every(([id, n]) => {
      const m = b.get(id)
      return m !== undefined && m.title === n.title && m.subtitle === n.subtitle && m.icon === n.icon
    }),
  })
  const nameOf = (id: string): ItemName => names().get(id) ?? { title: id, subtitle: '', kind: 'node' }

  const ids = createMemo<string[]>((previous) => {
    const next = [...itemMap().keys()]
    return previous && previous.length === next.length && previous.every((id, i) => id === next[i]) ? previous : next
  })

  // The same outcomes give the same geometry object, so replacing the graph (undo) does not re-create
  // the outcome chips of unchanged cards.
  const geometryCache = new Map<string, CardGeometry>()
  const geometryMap = createMemo(() => {
    const map = new Map<string, CardGeometry>()
    for (const [id, item] of itemMap()) {
      const key = `${item.kind}\u0000${item.outs.map((o) => o.outcome).join('\u0000')}`
      let geometry = geometryCache.get(key)
      if (!geometry) {
        geometry = cardGeometry(item.outs.map((o) => o.outcome), item.kind === 'node')
        if (geometryCache.size > 500) geometryCache.clear()
        geometryCache.set(key, geometry)
      }
      map.set(id, geometry)
    }
    return map
  })

  const back = createMemo(() => backEdges(props.graph))

  // Edges are keyed by their source (one edge per node.outcome); the target is read reactively, so a
  // rewired edge updates its path instead of being re-created.
  const edgeTargets = createMemo(() => {
    const known = itemMap()
    const map = new Map<string, string>()
    for (const e of props.graph.edges) {
      if (e.to !== IGNORE && known.has(e.from.node) && known.has(e.to)) map.set(edgeKey(e.from.node, e.from.outcome), e.to)
    }
    return map
  })
  // Orthogonal routes through the gaps between the cards, for the layout as committed. The memo only notes
  // that its inputs changed; the router itself is built when the first route is asked for, after the frame.
  // A card being dragged is not in the layout yet: its edges take a plain orthogonal path until it lands.
  const routing = createMemo(() => {
    shown()
    geometryMap()
    edgeTargets()
    let built: Router | null = null
    return {
      router: (): Router => {
        if (built) return built
        const base = shown()
        const geometries = geometryMap()
        const rects = new Map<string, Rect>()
        for (const [id, g] of geometries) {
          const p = base[id]
          if (p) rects.set(id, { x: p.x, y: p.y, width: g.width, height: g.height })
        }
        const links: RouteLink[] = []
        for (const [key, to] of edgeTargets()) {
          const node = key.slice(0, key.lastIndexOf('.'))
          const outcome = key.slice(key.lastIndexOf('.') + 1)
          const p = base[node]
          const q = base[to]
          const slot = geometries.get(node)?.outcomes.find((o) => o.outcome === outcome)
          const handle = geometries.get(to)?.target
          if (!p || !q || !slot || !handle) continue
          links.push({ key, from: node, to, port: { x: p.x + slot.handle.x, y: p.y + slot.handle.y }, entry: { x: q.x + handle.x, y: q.y + handle.y } })
        }
        built = createRouter(rects, links)
        return built
      },
    }
  })
  createEffect(() => {
    const key = selectedEdge()
    if (key && !edgeTargets().has(key)) setSelectedEdge(null)
  })
  const edgeList = createMemo<string[]>((previous) => {
    const next = [...edgeTargets().keys()]
    return previous && previous.length === next.length && previous.every((k, i) => k === next[i]) ? previous : next
  })


  // Only what is inside the viewport (plus a margin) is in the DOM, as xyflow's "only render visible
  // elements": a 200-step graph costs the cards on screen, not all of them. Selected items stay mounted.
  const VISIBLE_MARGIN = 64
  // Before mount nothing is measured (and on the server nothing renders); a container without a size
  // after mount (hidden, or jsdom) renders everything.
  const [measured, setMeasured] = createSignal(false)
  const visibleRect = createMemo(() => {
    const { width, height } = size()
    const vp = viewport()
    if (!measured()) return { x: 0, y: 0, width: 0, height: 0 }
    if (width === 0) return null
    const margin = VISIBLE_MARGIN / vp.zoom
    const a = toFlow({ x: 0, y: 0 }, vp)
    return { x: a.x - margin, y: a.y - margin, width: width / vp.zoom + margin * 2, height: height / vp.zoom + margin * 2 }
  }, null, { equals: (a, b) => a === b || (a !== null && b !== null && a.x === b.x && a.y === b.y && a.width === b.width && a.height === b.height) })
  const keep = (id: string) => props.selected === id || multi().has(id)
  // While cards glide (Tidy, undo) visibility is decided once, from where they land, and kept with what was
  // on screen: it does not re-run over every card on every frame.
  const visibleIds = createMemo<string[]>((previous) => {
    const rect = visibleRect()
    const geometries = geometryMap()
    const gliding = tidying()
    const next = rect === null ? ids() : ids().filter((id) => {
      if (keep(id)) return true
      if (gliding && previous?.includes(id)) return true
      const p = gliding ? committed[id] : positions[id]
      const g = geometries.get(id)
      return p && g ? rectsOverlap(rect, { x: p.x, y: p.y, width: g.width, height: g.height + 24 }) : false
    })
    return previous && previous.length === next.length && previous.every((id, i) => id === next[i]) ? previous : next
  })
  const visibleEdges = createMemo<string[]>((previous) => {
    const rect = visibleRect()
    routeTick()
    const targets = edgeTargets()
    const gliding = tidying()
    const place = gliding ? committed : positions
    const next = rect === null ? edgeList() : edgeList().filter((source) => {
      if (gliding && previous?.includes(source)) return true
      const node = source.slice(0, source.lastIndexOf('.'))
      const p = place[node]
      const q = place[targets.get(source) ?? '']
      if (!p || !q) return false
      const x = Math.min(p.x, q.x) - 64
      const y = Math.min(p.y, q.y) - 64
      if (rectsOverlap(rect, { x, y, width: Math.abs(p.x - q.x) + 640, height: Math.abs(p.y - q.y) + 224 })) return true
      // A route found earlier may run where its ends do not; routeTick says when one is published.
      const box = routeBoxes.get(source)
      return box !== undefined && rectsOverlap(rect, box)
    })
    return previous && previous.length === next.length && previous.every((k, i) => k === next[i]) ? previous : next
  })

  // Routes are found a few at a time after the frame that needs them, so mounting or replacing a big graph
  // does not wait for them: an edge shows its previous route (when its ends did not move) or a plain
  // orthogonal one until its own is ready.
  const ROUTE_SLICE_MS = 6
  let routeTimer: ReturnType<typeof setTimeout> | undefined
  createEffect(() => {
    const current = routing()
    // Edges that are gone leave the cache, and with them the router they were routed by.
    const targets = edgeTargets()
    for (const key of [...routeOwner.keys()]) {
      if (targets.has(key)) continue
      routeOwner.delete(key)
      routeBoxes.delete(key)
      setRouted(key, undefined as never)
    }
    clearTimeout(routeTimer)
    // While cards glide their edges take direct paths: routing the layout they land in waits for the landing,
    // so its slices do not take frames from the glide.
    if (tidying()) return
    const wanted = visibleEdges().filter((key) => routeOwner.get(key) !== current).sort((a, b) => Number(stale.has(b)) - Number(stale.has(a)))
    if (wanted.length === 0) return
    let todo: string[] | null = null
    const run = () => {
      const router = current.router()
      // One link per step, in port order, so the slice deadline is checked between links.
      todo ??= router.plan(wanted)
      const started = performance.now()
      batch(() => {
        while (todo!.length > 0 && performance.now() - started < ROUTE_SLICE_MS) {
          const key = todo!.shift()!
          const route = router.route(key)
          routeOwner.set(key, current)
          if (!route || route.points.length === 0) continue
          const xs = route.points.map((point) => point.x)
          const ys = route.points.map((point) => point.y)
          const x = Math.min(...xs)
          const y = Math.min(...ys)
          stale.delete(key)
          routeBoxes.set(key, { x, y, width: Math.max(...xs) - x, height: Math.max(...ys) - y })
          setRouted(key, { points: route.points.map((point) => ({ x: point.x, y: point.y })), clear: route.clear })
        }
        // One note per slice, so visibility looks at the boxes just published once.
        setRouteTick((n) => n + 1)
      })
      if (todo.length > 0) routeTimer = setTimeout(run, 0)
    }
    routeTimer = setTimeout(run, 0)
  })
  onCleanup(() => clearTimeout(routeTimer))

  const isSelected = (id: string) => props.selected === id || multi().has(id)
  const selection = () => {
    const out = new Set(multi())
    if (props.selected) out.add(props.selected)
    return [...out].filter((id) => itemMap().has(id))
  }

  createEffect(on(() => props.selected, (id) => {
    if (id) setSelectedEdge(null)
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
        if (lifted.length === 0) {
          lifted = [...moved.keys()]
          mark(lifted, 'ae-card--lifted', true)
          if (coarsePointer()) for (const id of lifted) click(id)
        }
        for (const [id, p] of moved) setPositions(id, { x: p.x, y: p.y })
      })
    },
    onEnd: (moved) => {
      setDragging(false)
      // The cards land on the grid with a short glide instead of a jump.
      const landed = [...moved.keys()]
      mark(lifted, 'ae-card--lifted', false)
      lifted = []
      mark(landed, 'ae-card--landing', true)
      later(motionMs('fast'), () => mark(landed, 'ae-card--landing', false))
      const out: AutomationLayout = {}
      let changed = false
      for (const [id, p] of moved) {
        const next = snapOn() ? { x: snap(p.x), y: snap(p.y) } : { x: Math.round(p.x), y: Math.round(p.y) }
        out[id] = next
        const before = shown()[id]
        if (!before || before.x !== next.x || before.y !== next.y) changed = true
      }
      if (changed) props.onMove?.(out)
      setPositions(reconcile(structuredClone(shown())))
    },
  })

  // ——— viewport ———

  const applyViewport = (vp: Viewport) => {
    setViewport(vp)
    if (viewportEl) viewportEl.style.transform = `translate(${vp.x}px, ${vp.y}px) scale(${vp.zoom})`
    // The dot grid is its own layer: a pan only translates it (compositing, no repaint); a zoom resizes it.
    if (gridEl) {
      const step = GRID * vp.zoom
      if (gridStep !== step) {
        gridStep = step
        gridEl.style.backgroundSize = `${step}px ${step}px`
      }
      const mod = (v: number) => ((v % step) + step) % step
      gridEl.style.transform = `translate(${Math.round(mod(vp.x) - step)}px, ${Math.round(mod(vp.y) - step)}px)`
    }
  }

  // `settledOnly` reads the committed layout: Fit right after Tidy frames where the cards land, not where
  // the glide starts.
  const bounds = (only?: string[], settledOnly = false) => {
    const geometries = geometryMap()
    const list = only ?? ids()
    if (list.length === 0) return null
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity
    for (const id of list) {
      const p = settledOnly ? committed[id] ?? positions[id] : positions[id]
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
    const b = bounds(undefined, true)
    const { width, height } = size()
    if (!b || !panZoom || width === 0) return
    panZoom.set(viewportForBounds(b, width, height, 0.12), 0)
  }

  // A small graph is fitted; a big one that would need a small zoom opens at its start (triggers) at
  // 100 %, so the first paint costs the cards on screen.
  const FIT_ITEMS = 40
  const INITIAL_MIN_ZOOM = 1
  const initialView = () => {
    const b = bounds()
    const { width, height } = size()
    if (!b || !panZoom || width === 0) return
    const fitted = viewportForBounds(b, width, height, 0.12)
    if (fitted.zoom >= INITIAL_MIN_ZOOM || ids().length <= FIT_ITEMS) {
      panZoom.set(fitted, 0)
      return
    }
    const zoom = INITIAL_MIN_ZOOM
    // It opens at the triggers (the top left of the first one), not at the left edge of a wide shared row.
    const triggers = [...itemMap()].filter(([, item]) => item.kind === 'trigger').map(([id]) => id)
    const start = bounds(triggers) ?? b
    panZoom.set({ x: 48 - start.x * zoom, y: 48 - start.y * zoom, zoom }, 0)
  }

  const center = (): XY => {
    const { width, height } = size()
    return toFlow({ x: width / 2, y: height / 2 }, viewport())
  }

  const focusItem = (id: string) => {
    const element = nodeElements.get(id)
    if (!element) {
      // Off screen, so not mounted: bring it into view, then focus it once it renders.
      const p = positions[id]
      const g = geometryMap().get(id)
      const { width, height } = size()
      if (!p || !g || !panZoom || width === 0) return
      const vp = viewport()
      panZoom.set({ x: width / 2 - (p.x + g.width / 2) * vp.zoom, y: height / 2 - (p.y + g.height / 2) * vp.zoom, zoom: vp.zoom })
      queueMicrotask(() => nodeElements.get(id)?.focus({ preventScroll: true }))
      return
    }
    element.focus({ preventScroll: true })
    const p = positions[id]
    const g = geometryMap().get(id)
    const { width, height } = size()
    if (!p || !g || !panZoom || width === 0) return
    const vp = viewport()
    const topLeft = toBox(p, vp)
    const bottomRight = toBox({ x: p.x + g.width, y: p.y + g.height }, vp)
    if (topLeft.x < 0 || topLeft.y < 0 || bottomRight.x > width || bottomRight.y > height) {
      panZoom.set({ x: width / 2 - (p.x + g.width / 2) * vp.zoom, y: height / 2 - (p.y + g.height / 2) * vp.zoom, zoom: vp.zoom }, motionMs('fast'))
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
    initialView()
    setMeasured(true)
    // The motion tokens are read here, once per page, while the first paint needs a style pass anyway: read
    // later, at the end of a drag, the read forces a style recalculation over every card (a long task in the
    // #1331 benchmark).
    for (const token of ['instant', 'fast', 'base', 'slow'] as const) motionMs(token)
    const onKey = (event: KeyboardEvent) => setAltSnapOff(event.altKey)
    const onPointer = (event: MouseEvent) => setAltSnapOff(event.altKey)
    window.addEventListener('keydown', onKey, true)
    window.addEventListener('keyup', onKey, true)
    window.addEventListener('mousemove', onPointer, true)
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

  // Minimap (ED11): a press or drag on it centres the viewport on that point; it follows every edit.
  const onMinimapPointer = (event: PointerEvent) => {
    if (!minimapEl || !panZoom) return
    if (event.type === 'pointerdown') {
      event.stopPropagation()
      minimapEl.setPointerCapture(event.pointerId)
    } else if (!minimapEl.hasPointerCapture(event.pointerId)) {
      return
    }
    const matrix = minimapEl.getScreenCTM()
    if (!matrix) return
    const point = new DOMPoint(event.clientX, event.clientY).matrixTransform(matrix.inverse())
    const vp = viewport()
    const { width, height } = size()
    panZoom.set({ x: width / 2 - point.x * vp.zoom, y: height / 2 - point.y * vp.zoom, zoom: vp.zoom })
  }

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
    stopTween()
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
          // The finger lifting after a long press must not click the menu item that opened under it.
          const swallow = (e: Event) => {
            e.preventDefault()
            e.stopPropagation()
          }
          window.addEventListener('click', swallow, { capture: true, once: true })
          window.addEventListener('touchend', swallow, { capture: true, once: true })
          setTimeout(() => {
            window.removeEventListener('click', swallow, { capture: true })
            window.removeEventListener('touchend', swallow, { capture: true })
          }, 800)
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
    setSelectedEdge(null)
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
    setPositions(reconcile(structuredClone(shown())))
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
      stopTween()
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
    const edge = selectedEdge()
    if (edge && (event.key === 'Delete' || event.key === 'Backspace') && editable() && event.target === container) {
      event.preventDefault()
      const dot = edge.lastIndexOf('.')
      setSelectedEdge(null)
      props.onChange?.(disconnect(props.graph, { node: edge.slice(0, dot), outcome: edge.slice(dot + 1) }))
      return
    }
    if (event.key === 'Escape' && edge) {
      event.preventDefault()
      setSelectedEdge(null)
      return
    }
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
    let last: ConnectionState | null = null
    const key = edgeKey(id, outcome)
    // Released on a target: the new edge flashes and the target answers with a click.
    const connected = (target: string) => {
      props.onChange?.(connect(props.graph, from, target))
      setFlashEdge(key)
      click(target)
      later(motionMs('slow'), () => setFlashEdge((current) => (current === key ? null : current)))
    }
    // Released off a target: the line pulls back into its handle.
    const missed = () => {
      const state = last
      if (!state || motionMs('fast') <= 0) return
      setRetract({ d: edgePath(state.from, state.to), x: state.from.x, y: state.from.y })
      later(motionMs('fast'), () => setRetract(null))
    }
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
      onUpdate: (state) => {
        if (state) last = state
        setConnection(state)
      },
      onConnect: (target) => connected(target),
      onRelease: (point, flow) => {
        const root = container!.getRootNode() as Document | ShadowRoot
        const target = nodeIdAt(root, point.clientX, point.clientY)
        if (target) {
          if (findItem(props.graph, target)?.kind === 'node') connected(target)
          else missed()
          return
        }
        if (isPaneAt(root, container!, point.clientX, point.clientY)) {
          const rect = container!.getBoundingClientRect()
          props.onDrop?.({ from, x: point.clientX - rect.left, y: point.clientY - rect.top, flow })
          return
        }
        missed()
      },
    })
  }

  const connectionHint = createMemo(() => {
    const c = connection()
    if (!c) return null
    const title = (id: string) => nameOf(id).title
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
    return coarsePointer() ? t().canvas.hintTouch : t().canvas.hintMouse
  }

  const edgeView = (source: string) => {
    const dot = source.lastIndexOf('.')
    const node = source.slice(0, dot)
    const outcome = source.slice(dot + 1)
    const to = () => edgeTargets().get(source) ?? ''
    const loop = () => back().has(`${node}->${to()}`)
    const tone: Tone = toneOf(outcome)
    // The route found for the committed layout, fitted onto the handles where the cards stand now; while a
    // card is dragged further than that, a rounded step (directPath) until it lands and is routed again.
    const path = createMemo(() => {
      const target = to()
      const p = positions[node]
      const q = positions[target]
      const g = geometryMap().get(node)
      const slot = g?.outcomes.find((o) => o.outcome === outcome)
      const handle = geometryMap().get(target)?.target
      if (!p || !q || !slot || !handle) return ''
      const a = { x: p.x + slot.handle.x, y: p.y + slot.handle.y }
      const b = { x: q.x + handle.x, y: q.y + handle.y }
      const found = routed[source]
      const fit = found ? fitRoute(found.points, a, b) : null
      if (fit) return routePath(trimEnd(fit), 10, found?.clear)
      return directPath(a, { x: b.x, y: b.y - ARROW_GAP })
    })
    const taken = () => props.overlay?.taken.has(source) ?? false
    const active = () => taken() && props.overlay?.states.get(to()) === 'waiting'
    const chosen = () => selectedEdge() === source
    const className = createMemo(() => {
      let c = `ae-edge ae-edge--${tone}`
      if (connectingKey() === source) c += ' ae-edge--rewiring'
      if (flashEdge() === source) c += ' ae-edge--flash'
      if (taken()) c += ' ae-edge--taken'
      if (active()) c += ' ae-edge--waiting'
      if (loop()) c += ' ae-edge--loop'
      if (chosen()) c += ' ae-edge--selected'
      if (hoveredEdge() === source) c += ' ae-edge--hover'
      const h = highlight()
      if (props.overlay ? !taken() : h ? !h.edges.has(source) : false) c += ' ae-dim'
      else if (h && !props.overlay) c += ' ae-edge--lit'
      return c
    })
    // The arrow takes the edge's colour: one marker per tone and state, drawn once per canvas.
    const marker = () => `url(#${flowId}-arrow-${chosen() || flashEdge() === source ? 'active' : taken() ? 'active' : tone})`
    // The run's path lights up edge by edge in the order of its steps.
    const step = () => {
      const taken = props.overlay?.taken
      if (!taken?.has(source)) return undefined
      let i = 0
      for (const k of taken) {
        if (k === source) return `--ae-step:${i}`
        i++
      }
      return undefined
    }
    return (
      <g class="ae-edge-group" data-edge={source} data-to={to()}>
        <path class={className()} d={path()} style={step()} marker-end={marker()} />
        {/* A wide transparent stroke over the line: hover and click on it, as on the reference editor's edges. */}
        <path
          class={`ae-edge-hit ${NOPAN}`}
          d={path()}
          onPointerEnter={() => setHoveredEdge(source)}
          onPointerLeave={() => setHoveredEdge((current) => (current === source ? null : current))}
          onClick={(event) => {
            event.stopPropagation()
            setSelectedEdge(source)
            setMulti(new Set<string>())
            props.onSelect(null)
            container?.focus({ preventScroll: true })
          }}
          onContextMenu={(event) => {
            event.stopPropagation()
            event.preventDefault()
            setSelectedEdge(source)
          }}
        />
      </g>
    )
  }

  const NodeCard = (cardProps: { id: string }) => {
    const id = cardProps.id
    const item = createMemo(() => itemMap().get(id), undefined, { equals: sameItem }) as Accessor<CanvasItem>
    const geometry = createMemo(() => geometryMap().get(id) ?? EMPTY_GEOMETRY)
    const name = createMemo(() => nameOf(id), undefined, { equals: (a, b) => a.title === b.title && a.subtitle === b.subtitle && a.icon === b.icon })
    const selected = createMemo(() => isSelected(id))
    const unavailable = createMemo(() => item()?.kind === 'node' && !props.catalog.nodes.some((n) => n.type === item()?.type))
    const className = createMemo(() => {
      const it = item()
      let c = `${NODE_CLASS} ${NOPAN} ae-card`
      if (it?.kind === 'trigger') c += ' ae-card--trigger'
      if (selected()) c += ' ae-card--selected'
      if (it?.error || unavailable()) c += ' ae-card--error'
      if (it?.state) c += ` ae-card--${it.state}`
      const over = connection()
      if (over && over.overTarget === id && over.valid) c += ' ae-card--drop'
      const h = highlight()
      if (props.overlay ? !props.overlay.states.has(id) : h ? !h.items.has(id) : false) c += ' ae-dim'
      return c
    })
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

    const outLabel = (out: CanvasItem['outs'][number] | undefined) => {
      if (!out || (!out.to && !out.ignore)) return t().canvas.notConnected
      if (out.ignore) return t().canvas.ignored
      return nameOf(out.to!).title
    }
    // Outcomes without an edge: one note under its handle, or one line for the card when there are several.
    const notes = createMemo(() => (item()?.outs ?? []).filter((o) => !o.to).map((o) =>
      `${o.outcome === FIRED ? '→' : o.outcome}: ${o.ignore ? t().canvas.ignored : t().canvas.notConnected}`), undefined, { equals: (a, b) => a.join('\u0000') === b.join('\u0000') })
    const badgeLabel = () => unavailable() ? t().canvas.unavailable : lost().has(id) ? `${t().canvas.noConnection}: ${item()?.error ?? ''}` : t().canvas.error
    const icon = () => name().icon ?? (item()?.kind === 'trigger' ? 'event' : props.catalog.nodes.find((n) => n.type === item()?.type)?.behavior ?? 'node')

    return (
      <div
        ref={element}
        class={className()}
        style={{ transform: `translate(${positions[id]?.x ?? 0}px,${positions[id]?.y ?? 0}px)`, width: `${geometry().width}px`, height: `${geometry().height}px` }}
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
              class={`${HANDLE_CLASS} target ${NOPAN} ${NODRAG} ae-handle ae-handle--target${editable() ? ' connectable connectableend' : ''}`}
              data-id={handleDataId(flowId, id, TARGET_HANDLE, 'target')}
              data-nodeid={id}
              data-handleid={TARGET_HANDLE}
              data-handlepos="top"
              style={`left:${target().x}px;top:${target().y}px`}
              aria-hidden="true"
            />
          )}
        </Show>
        <Show when={props.renderNode} fallback={
          <div class="ae-card__body">
            <div class="ae-card__head">
              <span class="ae-card__icon"><Icon name={icon()} /></span>
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
                <span
                  class="ae-badge ae-badge--error"
                  role="img"
                  aria-label={badgeLabel()}
                  title={item()?.error ?? t().canvas.unavailable}
                >
                  <Icon name="error" size={12} />
                  <span>{unavailable() ? t().canvas.unavailable : lost().has(id) ? t().canvas.noConnection : '!'}</span>
                </span>
              </Show>
              <Show when={!item()?.error && item()?.warning}><span class="ae-badge ae-badge--warning" title={item()?.warning}>!</span></Show>
            </div>
            <Show when={props.overlay?.notes?.get(id)} fallback={<div class="ae-card__sub" title={name().subtitle}>{name().subtitle}</div>}>
              {(note) => <div class="ae-card__sub ae-card__note" title={note()}>{note()}</div>}
            </Show>
          </div>
        }>
          {(render) => <div class="ae-card__body">{render()(item)}</div>}
        </Show>
        <For each={geometry().outcomes}>
          {(slot) => {
            // The outcome chip is the source handle itself: one element, its dot drawn by CSS at the
            // declared handle position (bottom centre).
            const out = () => item()?.outs.find((o) => o.outcome === slot.outcome)
            const open = () => !out()?.to && !out()?.ignore
            const chipClass = () => {
              let c = `${HANDLE_CLASS} source ${NOPAN} ${NODRAG} ae-out ae-out--${toneOf(slot.outcome)}`
              if (editable()) c += ' connectable'
              if (open()) c += ' ae-out--open'
              if (out()?.ignore) c += ' ae-out--ignore'
              if (props.overlay?.taken.has(edgeKey(id, slot.outcome))) c += ' ae-out--taken'
              return c
            }
            return (
              <>
                <div
                  class={chipClass()}
                  data-id={handleDataId(flowId, id, slot.outcome, 'source')}
                  data-nodeid={id}
                  data-handleid={slot.outcome}
                  data-handlepos="bottom"
                  style={`left:${slot.x}px;width:${slot.width}px;top:${OUTCOME_ROW_Y}px`}
                  title={t().canvas.outcome(slot.outcome, outLabel(out()))}
                  aria-hidden="true"
                  onMouseDown={(event) => onHandleDown(id, slot.outcome, event)}
                  onTouchStart={(event) => onHandleDown(id, slot.outcome, event)}
                >
                  <span class="ae-out__label">{slot.outcome === FIRED && item()?.kind === 'trigger' ? '→' : slot.outcome}</span>
                </div>
                <Show when={!out()?.to && notes().length === 1}>
                  <span class="ae-out-note" style={`left:${slot.handle.x}px;top:${slot.handle.y + 10}px`}>
                    {out()?.ignore ? t().canvas.ignored : t().canvas.notConnected}
                  </span>
                </Show>
              </>
            )
          }}
        </For>
        <Show when={notes().length > 1}>
          <span class="ae-out-note ae-out-note--card" style={`top:${geometry().height + 10}px`} title={notes().join(' · ')}>
            {notes().join(' · ')}
          </span>
        </Show>
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
    const b = bounds(undefined, tidying())
    if (!b) return null
    const pad = 80
    return { x: Math.round(b.x - pad), y: Math.round(b.y - pad), width: Math.round(b.width + pad * 2), height: Math.round(b.height + pad * 2) }
  })

  // The minimap draws every card as one path (two with the selection): one element to restyle, not 200.
  const minimapPaths = createMemo(() => {
    const geometries = geometryMap()
    // While cards glide the minimap shows where they land (one path per frame for 200 cards is not free).
    const place = tidying() ? committed : positions
    let rest = ''
    let selected = ''
    for (const id of ids()) {
      const p = place[id]
      const g = geometries.get(id)
      if (!p || !g) continue
      const rect = `M${p.x} ${p.y}h${g.width}v${g.height}h${-g.width}z`
      if (isSelected(id)) selected += rect
      else rest += rect
    }
    return { rest, selected }
  })

  const viewportRect = () => {
    const { width, height } = size()
    const vp = viewport()
    const a = toFlow({ x: 0, y: 0 }, vp)
    // Whole flow units: the frame does not shimmer with sub-pixel viewport values.
    return { x: Math.round(a.x), y: Math.round(a.y), width: Math.round(width / vp.zoom), height: Math.round(height / vp.zoom) }
  }

  return (
    <div
      ref={container}
      class={`${FLOW_CLASS} ae-canvas`}
      classList={{ 'ae-canvas--readonly': !editable(), 'ae-canvas--connecting': connection() !== null, 'ae-canvas--selecting': Boolean(props.selecting), 'ae-canvas--tidying': tidying() }}
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
      <div ref={gridEl} class="ae-grid" aria-hidden="true" />
      <div class={`${PANE_CLASS} ae-pane`} />
      <div ref={viewportEl} class="ae-viewport">
        <svg class="ae-edges" aria-hidden="true">
          <defs>
            <For each={['neutral', 'ok', 'warn', 'bad', 'active'] as const}>
              {(name) => (
                <marker id={`${flowId}-arrow-${name}`} class={`ae-arrow ae-arrow--${name}`} viewBox="-10 -10 20 20" markerWidth="24" markerHeight="24" markerUnits="userSpaceOnUse" orient="auto-start-reverse" refX="0" refY="0">
                  <polyline points="-5,-4 0,0 -5,4 -5,-4" />
                </marker>
              )}
            </For>
          </defs>
          <For each={visibleEdges()}>{(key) => edgeView(key)}</For>
          <Show when={connection()}>
            <path class="ae-edge ae-edge--draft" classList={{ 'ae-edge--valid': connection()?.valid === true }} d={connectionPath()} />
          </Show>
          <Show when={retract()}>
            {(r) => <path class="ae-edge ae-edge--draft ae-edge--retract" d={r().d} style={`transform-origin:${r().x}px ${r().y}px`} />}
          </Show>
        </svg>
        <For each={visibleIds()}>{(id) => <NodeCard id={id} />}</For>
      </div>
      <Show when={boxRect()}>
        {(rect) => <div class="ae-box" style={{ left: `${rect().x}px`, top: `${rect().y}px`, width: `${rect().width}px`, height: `${rect().height}px` }} />}
      </Show>
      <Show when={press()}>
        {(p) => (
          // The ring fills over the long-press time; a full static ring under reduced motion.
          <svg class="ae-press" viewBox="0 0 56 56" style={{ left: `${p().x}px`, top: `${p().y}px`, '--ae-long-press': `${LONG_PRESS_MS}ms` }} aria-hidden="true">
            <circle class="ae-press__track" cx="28" cy="28" r="25" />
            <circle class="ae-press__fill" cx="28" cy="28" r="25" pathLength="100" />
          </svg>
        )}
      </Show>
      <div class={`ae-hint ${NOPAN}`} classList={{ 'ae-hint--active': connection() !== null }} role="status" aria-live="polite">
        {hint()}
      </div>
      <div class={`ae-controls ${NOPAN} ${NOWHEEL}`} role="toolbar" aria-label={t().canvas.label}>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.zoomIn} title={t().canvas.zoomIn} onClick={() => api.zoomIn()}><Icon name="plus" /></button>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.zoomOut} title={t().canvas.zoomOut} onClick={() => api.zoomOut()}><Icon name="minus" /></button>
        <button type="button" class="ae-iconbtn" aria-label={t().canvas.fit} title={t().canvas.fit} onClick={() => api.fit()}><Icon name="fit" /></button>
        <Show when={props.onTidy}>
          <button
            type="button"
            class="ae-iconbtn ae-tidy"
            aria-label={t().canvas.tidy}
            aria-keyshortcuts={isMac() ? 'Meta+Shift+L' : 'Control+Shift+L'}
            title={`${t().canvas.tidy} (${keyLabel('⇧⌘L', isMac())})`}
            onClick={() => props.onTidy?.()}
          >
            <Icon name="broom" />
          </button>
        </Show>
        <button type="button" class="ae-iconbtn ae-iconbtn--text" aria-label={`${Math.round(viewport().zoom * 100)}% · ${t().canvas.zoomReset}`} title={t().canvas.zoomReset} onClick={() => api.zoomReset()}>
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
          onPointerDown={onMinimapPointer}
          onPointerMove={onMinimapPointer}
          role="img"
          aria-label={t().canvas.minimap}
        >
          <path class="ae-minimap__node" d={minimapPaths().rest} />
          <path class="ae-minimap__node ae-minimap__node--selected" d={minimapPaths().selected} />
          <rect class="ae-minimap__view" x={viewportRect().x} y={viewportRect().y} width={viewportRect().width} height={viewportRect().height} />
        </svg>
      </Show>
    </div>
  )
}
