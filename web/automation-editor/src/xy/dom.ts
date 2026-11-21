// The DOM contract of @xyflow/system in one place: class names and data attributes its handle and
// drag code look up. Nothing else in the editor spells these strings.
export const LIB = 'iota'

export const FLOW_CLASS = `${LIB}-flow`
export const NODE_CLASS = `${LIB}-flow__node`
export const HANDLE_CLASS = `${LIB}-flow__handle`
export const PANE_CLASS = `${LIB}-flow__pane`
/** Elements that never start a pan. */
export const NOPAN = 'nopan'
/** Elements that never start a node drag. */
export const NODRAG = 'nodrag'
/** Elements whose wheel does not zoom. */
export const NOWHEEL = 'nowheel'

export type HandleKind = 'source' | 'target'

/** data-id of a handle as XYHandle builds it: `${flowId}-${nodeId}-${handleId}-${type}`. */
export function handleDataId(flowId: string, nodeId: string, handleId: string, kind: HandleKind): string {
  return `${flowId}-${nodeId}-${handleId}-${kind}`
}

/** Attributes and classes of a handle element. */
export function handleAttributes(flowId: string, nodeId: string, handleId: string, kind: HandleKind, connectable: boolean) {
  const classes = [HANDLE_CLASS, kind, NOPAN, NODRAG]
  if (connectable) {
    classes.push('connectable')
    // Only targets accept the end of a connection: an outcome is never connected to an outcome.
    if (kind === 'target') classes.push('connectableend')
  }
  return {
    class: classes.join(' '),
    'data-id': handleDataId(flowId, nodeId, handleId, kind),
    'data-nodeid': nodeId,
    'data-handleid': handleId,
    'data-handlepos': kind === 'source' ? 'bottom' : 'left',
  }
}

export function nodeAttributes(id: string) {
  return { 'data-id': id }
}

/** The item id of the node element under a point, or null (pane, outside, a control). */
export function nodeIdAt(root: Document | ShadowRoot, x: number, y: number): string | null {
  const element = root.elementFromPoint(x, y)
  const node = element?.closest(`.${NODE_CLASS}`)
  return node?.getAttribute('data-id') ?? null
}

/** Whether a point is over the canvas pane itself (not a node, control or overlay). */
export function isPaneAt(root: Document | ShadowRoot, container: Element, x: number, y: number): boolean {
  const element = root.elementFromPoint(x, y)
  if (!element || !container.contains(element)) return false
  return !element.closest(`.${NODE_CLASS}, .${HANDLE_CLASS}, .${NOPAN}`) || element.classList.contains(PANE_CLASS)
}

export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.closest('[contenteditable="true"], [role="textbox"], [role="combobox"]') !== null
}
