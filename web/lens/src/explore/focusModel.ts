import type {
  DashboardDocument,
  Encoding,
  Frame,
  Level,
  Node,
  NodePath,
  Panel,
  PanelKind,
  PanelStatus,
} from '../contract'
import { levelForPath } from '../runtime'
import { drillTargetForLevel, drillTargetForNode, labelForNode, rowForNode } from './model'

/**
 * The chart kinds a drill level may claim as its own visualization. Mirrors the
 * Go-side `validLevelView`: table lives behind `Level.source`, and stat/metric
 * kinds carry configs a level cannot supply. An unknown value degrades to the
 * semantics-derived view instead of an unsupported-panel card.
 */
const levelViewKinds: ReadonlySet<PanelKind> = new Set([
  'pie', 'donut', 'bar', 'hbar', 'line', 'area', 'cascade', 'coverage',
])

/** The view a level declares for itself, when it is a renderable chart kind. */
export function exploreViewForLevel(level: Level | undefined): PanelKind | undefined {
  return level?.view && levelViewKinds.has(level.view) ? level.view : undefined
}

/**
 * Whether this explore host opts into the focus-canvas chrome: the drill root
 * declares it once for the whole exploration, or an individual level carries
 * it. Documents that never set `presentation.focus` render exactly as before.
 */
export function isFocusCanvas(rootLevel: Level | undefined, level: Level | undefined): boolean {
  return rootLevel?.presentation?.focus === 'canvas' || level?.presentation?.focus === 'canvas'
}

export type FocusMiniView = 'donut' | 'cascade'

export interface FocusSlice {
  key: string
  label: string
  value: number
  selected: boolean
}

/**
 * The parent context the focus header renders beside the metric: the figure
 * the share is taken against, the mini-chart slices with the focused one
 * marked, and the breadcrumb index that returns to it.
 */
export interface FocusParent {
  label: string
  /** Sum of the parent's visible children — what the share is against. */
  value?: number
  /** `jumpTo` breadcrumb index that returns to the parent level. */
  pathIndex: number
  view: FocusMiniView
  slices: Array<FocusSlice>
  selectedIndex: number
}

/**
 * Everything the focus chrome states about the current level: the metric name,
 * its value and share of the parent, the parent mini-chart, and the level's
 * quality status. Pure projection over the document + navigation path; every
 * field except `label` is optional so a deep link whose parent frame has not
 * loaded degrades to a name-only header instead of crashing.
 */
export interface FocusContext {
  label: string
  value?: number
  share?: number
  total?: number
  parent?: FocusParent
  status?: PanelStatus
}

function numeric(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return undefined
}

function rowValue(frame: Frame | undefined, row: Array<unknown> | undefined, field: string | undefined): unknown {
  if (!frame || !row || !field) return undefined
  const index = frame.columns.findIndex(({ name }) => name === field)
  return index < 0 ? undefined : row[index]
}

function sliceValue(node: Node, level: Level, frame: Frame | undefined, encoding: Encoding | undefined): number {
  const row = rowForNode(node, level, frame, encoding)
  return numeric(rowValue(frame, row, encoding?.value)) ?? 0
}

function parentSlices(
  document: DashboardDocument,
  panel: Panel,
  parent: Level,
  frame: Frame | undefined,
  selectedKey: string,
): { slices: Array<FocusSlice>; selectedIndex: number } {
  const encoding = parent.encoding ?? panel.encoding
  const slices = parent.children.map((child) => ({
    key: child.key,
    label: labelForNode(child, parent, document, frame, encoding),
    value: sliceValue(child, parent, frame, encoding),
    selected: child.key === selectedKey,
  }))
  return { slices, selectedIndex: slices.findIndex((slice) => slice.selected) }
}

/**
 * Projects the focus-header model for the level the navigation path addresses.
 * The value/share/total come through the same `drillTargetForNode` resolution
 * the overlay uses, so the header can never disagree with the popover; when
 * the path sits at the drill root there is no parent and the header carries
 * the level's own total.
 */
export function focusContextForLevel(
  document: DashboardDocument,
  panel: Panel,
  path: NodePath,
  level: Level,
): FocusContext {
  const status = level.status
  const parentPath = path.slice(0, -1)
  const parentLevel = parentPath.length > 0 ? levelForPath(document, parentPath) : undefined
  const selectedKey = path.at(-1)
  const node = parentLevel && selectedKey
    ? parentLevel.children.find((child) => child.key === selectedKey)
    : undefined

  if (!parentLevel || !node) {
    // Drill root (or an unresolvable parent): the header describes the level
    // itself. A missing frame leaves the value out rather than guessing.
    const frame = level.frame ? document.frames[level.frame] : undefined
    const target = drillTargetForLevel(document, panel, level, frame)
    return { label: target.label, value: target.total, total: target.total, status }
  }

  const parentFrame = parentLevel.frame ? document.frames[parentLevel.frame] : undefined
  const levelFrame = level.frame ? document.frames[level.frame] : undefined
  const target = drillTargetForNode(document, parentLevel, node, parentFrame, levelFrame, panel)
  const { slices, selectedIndex } = parentSlices(document, panel, parentLevel, parentFrame, node.key)
  const hasSliceValues = slices.some((slice) => slice.value !== 0)
  const parent: FocusParent = {
    label: parentLevel.label.trim() || panel.title,
    value: target.total,
    pathIndex: parentPath.length - 1,
    view: exploreViewForLevel(parentLevel) === 'cascade' ? 'cascade' : 'donut',
    // A parent whose frame has not loaded (deep link) yields all-zero slices;
    // an empty list tells the header to drop the mini-chart entirely.
    slices: hasSliceValues ? slices : [],
    selectedIndex,
  }

  return {
    label: level.label.trim() || target.label,
    value: target.value,
    share: target.share,
    total: target.total,
    parent,
    status,
  }
}
