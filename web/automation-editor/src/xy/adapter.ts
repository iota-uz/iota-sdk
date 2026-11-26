// The only module that imports @xyflow/system (pinned to 1.0.0). It speaks the editor's own types;
// no xyflow type leaves this file.
import {
  ConnectionMode,
  getBezierPath,
  getViewportForBounds,
  PanOnScrollMode,
  Position,
  XYDrag,
  XYHandle,
  XYPanZoom,
  type HandleBounds,
  type InternalNodeBase,
  type NodeBase,
  type Transform,
} from '@xyflow/system'
import type { XY } from '../types'
import { LIB, NODRAG, NOPAN, NOWHEEL } from './dom'
import { motionMs } from '../motion'

export type Viewport = { x: number; y: number; zoom: number }

export const MIN_ZOOM = 0.2
export const MAX_ZOOM = 2

// ——— node lookup ———

export type LookupHandle = { id: string; kind: 'source' | 'target'; x: number; y: number; size: number }
export type LookupItem = {
  id: string
  position: XY
  width: number
  height: number
  selected: boolean
  draggable: boolean
  handles: LookupHandle[]
}

type Internal = InternalNodeBase<NodeBase>

/** The internal node map xyflow's drag and handle code read; handles are declared from geometry. */
export class NodeLookup {
  readonly map = new Map<string, Internal>()

  set(items: LookupItem[]): void {
    const seen = new Set<string>()
    for (const item of items) {
      seen.add(item.id)
      const handleBounds = {
        source: [] as HandleBounds[],
        target: [] as HandleBounds[],
      }
      for (const h of item.handles) {
        const bounds: HandleBounds = {
          id: h.id,
          nodeId: item.id,
          x: h.x - h.size / 2,
          y: h.y - h.size / 2,
          width: h.size,
          height: h.size,
          position: h.kind === 'source' ? Position.Bottom : Position.Left,
          type: h.kind,
        }
        handleBounds[h.kind].push(bounds)
      }
      const existing = this.map.get(item.id)
      const userNode: NodeBase = existing?.internals.userNode ?? { id: item.id, position: item.position, data: {} }
      userNode.position = item.position
      userNode.selected = item.selected
      const node: Internal = {
        id: item.id,
        position: { ...item.position },
        data: {},
        selected: item.selected,
        draggable: item.draggable,
        measured: { width: item.width, height: item.height },
        width: item.width,
        height: item.height,
        internals: {
          positionAbsolute: { ...item.position },
          z: 0,
          userNode,
          handleBounds,
          isParent: false,
        },
      }
      this.map.set(item.id, node)
    }
    for (const id of [...this.map.keys()]) if (!seen.has(id)) this.map.delete(id)
  }

  move(id: string, position: XY): void {
    const node = this.map.get(id)
    if (!node) return
    node.position = { ...position }
    node.internals.positionAbsolute = { ...position }
  }
}

// ——— pan & zoom ———

export type PanZoomOptions = {
  viewport: Viewport
  onTransform(viewport: Viewport): void
  onStart?(): void
  onEnd?(viewport: Viewport): void
  onContextMenu?(event: MouseEvent): void
}

export type PanZoomUpdate = { panOnDrag: boolean; selecting: boolean; connecting: boolean }

export type PanZoom = {
  update(options: PanZoomUpdate): void
  set(viewport: Viewport, duration?: number): void
  get(): Viewport
  scaleBy(factor: number): void
  scaleTo(zoom: number): void
  destroy(): void
}

export function createPanZoom(domNode: Element, options: PanZoomOptions): PanZoom {
  const instance = XYPanZoom({
    domNode,
    minZoom: MIN_ZOOM,
    maxZoom: MAX_ZOOM,
    translateExtent: [[-Infinity, -Infinity], [Infinity, Infinity]],
    viewport: options.viewport,
    onDraggingChange: () => undefined,
    onPanZoomStart: () => options.onStart?.(),
    onPanZoom: (_event, viewport) => options.onTransform(viewport),
    onPanZoomEnd: (_event, viewport) => options.onEnd?.(viewport),
  })
  const apply = (update: PanZoomUpdate) => instance.update({
    noWheelClassName: NOWHEEL,
    noPanClassName: NOPAN,
    onPaneContextMenu: options.onContextMenu,
    preventScrolling: true,
    panOnScroll: false,
    panOnDrag: update.panOnDrag && !update.selecting,
    panOnScrollMode: PanOnScrollMode.Free,
    panOnScrollSpeed: 0.5,
    userSelectionActive: update.selecting,
    zoomOnPinch: true,
    zoomOnScroll: true,
    zoomOnDoubleClick: false,
    zoomActivationKeyPressed: false,
    lib: LIB,
    onTransformChange: (transform: Transform) => options.onTransform({ x: transform[0], y: transform[1], zoom: transform[2] }),
    connectionInProgress: update.connecting,
    paneDragThreshold: 1,
  })
  apply({ panOnDrag: true, selecting: false, connecting: false })
  return {
    update: apply,
    set: (viewport, duration) => void instance.setViewport(viewport, duration ? { duration } : undefined),
    get: () => instance.getViewport(),
    scaleBy: (factor) => void instance.scaleBy(factor, { duration: motionMs('fast') }),
    scaleTo: (zoom) => void instance.scaleTo(zoom, { duration: motionMs('fast') }),
    destroy: () => instance.destroy(),
  }
}

export function viewportForBounds(bounds: { x: number; y: number; width: number; height: number }, width: number, height: number, padding = 0.1): Viewport {
  return getViewportForBounds(bounds, width, height, MIN_ZOOM, 1, padding)
}

// ——— node drag ———

export type DragCallbacks = {
  lookup: NodeLookup
  container(): Element | null
  viewport(): Viewport
  snap(): boolean
  grid: number
  enabled(): boolean
  panBy(delta: XY): Promise<boolean>
  /** The node was pressed: select it (unless it is already in a multi selection). */
  onPress(id: string): void
  onStart(ids: string[]): void
  onMove(positions: Map<string, XY>): void
  onEnd(positions: Map<string, XY>): void
}

export type NodeDrag = { bind(element: Element, id: string): void; destroy(): void }

export function createNodeDrag(callbacks: DragCallbacks): () => NodeDrag {
  const positionsOf = (items: Map<string, { position: XY }>) => {
    const out = new Map<string, XY>()
    for (const [id, item] of items) out.set(id, { x: item.position.x, y: item.position.y })
    return out
  }
  return () => {
    const drag = XYDrag({
      onNodeMouseDown: (id) => callbacks.onPress(id),
      getStoreItems: () => {
        const vp = callbacks.viewport()
        return {
          nodes: [],
          nodeLookup: callbacks.lookup.map,
          edges: [],
          nodeExtent: [[-Infinity, -Infinity], [Infinity, Infinity]],
          snapGrid: [callbacks.grid, callbacks.grid],
          snapToGrid: callbacks.snap(),
          nodeOrigin: [0, 0],
          multiSelectionActive: false,
          domNode: callbacks.container(),
          transform: [vp.x, vp.y, vp.zoom],
          autoPanOnNodeDrag: true,
          nodesDraggable: callbacks.enabled(),
          selectNodesOnDrag: true,
          nodeDragThreshold: 2,
          panBy: callbacks.panBy,
          unselectNodesAndEdges: () => undefined,
          updateNodePositions: (items, dragging) => {
            const positions = positionsOf(items as Map<string, { position: XY }>)
            for (const [id, p] of positions) callbacks.lookup.move(id, p)
            if (dragging) callbacks.onMove(positions)
          },
        }
      },
      onDragStart: (_event, items) => callbacks.onStart([...items.keys()]),
      onDragStop: (_event, items) => callbacks.onEnd(positionsOf(items)),
    })
    return {
      bind: (element, id) => drag.update({ domNode: element, nodeId: id, isSelectable: true, noDragClassName: NODRAG }),
      destroy: () => drag.destroy(),
    }
  }
}

// ——— connections ———

export type ConnectionState = { from: XY; to: XY; fromNode: string; fromHandle: string; overTarget: string | null; valid: boolean | null }

export type ConnectParams = {
  container: HTMLDivElement
  lookup: NodeLookup
  flowId: string
  nodeId: string
  handleId: string
  handleElement: Element
  viewport(): Viewport
  panBy(delta: XY): Promise<boolean>
  isValid(target: string): boolean
  onUpdate(state: ConnectionState | null): void
  /** Released on a valid target handle. */
  onConnect(target: string): void
  /** Released without a valid handle: the caller decides (node body, empty pane, outside). */
  onRelease(point: { clientX: number; clientY: number }, flow: XY): void
}

export function startConnection(event: MouseEvent | TouchEvent, params: ConnectParams): void {
  let connected = false
  XYHandle.onPointerDown(event, {
    autoPanOnConnect: true,
    connectionMode: ConnectionMode.Strict,
    connectionRadius: 24,
    domNode: params.container,
    handleId: params.handleId,
    nodeId: params.nodeId,
    isTarget: false,
    nodeLookup: params.lookup.map,
    lib: LIB,
    flowId: params.flowId,
    updateConnection: (state) => {
      if (!state.inProgress) {
        params.onUpdate(null)
        return
      }
      params.onUpdate({
        from: state.from,
        // `to` is in pane pixels (or a target handle's pixels); the editor draws in flow coordinates.
        to: toFlow(state.to, params.viewport()),
        fromNode: params.nodeId,
        fromHandle: params.handleId,
        overTarget: state.toNode?.id ?? null,
        valid: state.isValid,
      })
    },
    panBy: params.panBy,
    cancelConnection: () => params.onUpdate(null),
    onConnect: (connection) => {
      connected = true
      params.onConnect(connection.target)
    },
    onConnectEnd: (endEvent, state) => {
      if (connected) return
      const point = 'changedTouches' in endEvent ? endEvent.changedTouches[0] : (endEvent as MouseEvent)
      if (!point) return
      const bounds = params.container.getBoundingClientRect()
      params.onRelease({ clientX: point.clientX, clientY: point.clientY }, toFlow({ x: point.clientX - bounds.left, y: point.clientY - bounds.top }, params.viewport()))
      void state
    },
    isValidConnection: (connection) => params.isValid(connection.target),
    getTransform: () => {
      const vp = params.viewport()
      return [vp.x, vp.y, vp.zoom]
    },
    getFromHandle: () => params.lookup.map.get(params.nodeId)?.internals.handleBounds?.source?.find((h) => h.id === params.handleId) ?? null,
    dragThreshold: 1,
    handleDomNode: params.handleElement,
  })
}

// ——— paths & coordinates ———

/** A forward edge leaves an outcome downwards and enters the target from the left. */
export function edgePath(source: XY, target: XY): string {
  const [path] = getBezierPath({
    sourceX: source.x, sourceY: source.y, sourcePosition: Position.Bottom,
    targetX: target.x, targetY: target.y, targetPosition: Position.Top,
    curvature: 0.3,
  })
  return path
}

export function toFlow(point: XY, viewport: Viewport): XY {
  return { x: (point.x - viewport.x) / viewport.zoom, y: (point.y - viewport.y) / viewport.zoom }
}

export function toBox(flow: XY, viewport: Viewport): XY {
  return { x: flow.x * viewport.zoom + viewport.x, y: flow.y * viewport.zoom + viewport.y }
}
