import type { NodeKey, NodePath } from '../contract'

export interface NavigationView {
  panelId?: string
  path: NodePath
  perspectiveId?: string
  drawer?: DrawerNavigationView
}

export interface DrawerNavigationView {
  src: string
  panelId?: string
  path: NodePath
  perspectiveId?: string
}

export interface NavigationState extends NavigationView {
  history: Array<NavigationView>
}

export type NavigationAction =
  | { type: 'drillInto'; nodeKey: NodeKey; panelId?: string; path?: NodePath }
  | { type: 'back' }
  | { type: 'jumpTo'; breadcrumbIndex: number }
  /**
   * `enterKey` folds "enter this segment, then show it as X" into one step.
   * Picking a view for a segment is one user action, and charging it two
   * transitions puts a level nobody asked to stand on between the chart and
   * the answer — the level it enters is a fork whose only content is that
   * same choice.
   */
  | {
    type: 'switchPerspective'
    perspectiveId: string
    path?: NodePath
    replace?: boolean
    enterKey?: NodeKey
    panelId?: string
  }
  | { type: 'reset' }
  | { type: 'openDrawer'; src: string }
  | { type: 'updateDrawer'; view: Omit<DrawerNavigationView, 'src'> }
  | { type: 'closeDrawer' }
  | { type: 'restore'; view: NavigationView; history?: Array<NavigationView> }

function cloneDrawer(view: DrawerNavigationView | undefined): DrawerNavigationView | undefined {
  return view ? { src: view.src, panelId: view.panelId, path: [...view.path], perspectiveId: view.perspectiveId } : undefined
}

function cloneView(view: NavigationView): NavigationView {
  return {
    panelId: view.panelId,
    path: [...view.path],
    perspectiveId: view.perspectiveId,
    ...(view.drawer ? { drawer: cloneDrawer(view.drawer) } : {}),
  }
}

function samePath(left: NodePath, right: NodePath): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index])
}

function sameView(left: NavigationView, right: NavigationView): boolean {
  const sameDrawer = left.drawer === undefined && right.drawer === undefined || (
    left.drawer !== undefined && right.drawer !== undefined &&
    left.drawer.src === right.drawer.src &&
    left.drawer.panelId === right.drawer.panelId &&
    left.drawer.perspectiveId === right.drawer.perspectiveId &&
    samePath(left.drawer.path, right.drawer.path)
  )
  return (
    left.panelId === right.panelId &&
    left.perspectiveId === right.perspectiveId &&
    samePath(left.path, right.path) && sameDrawer
  )
}

function currentView(state: NavigationState): NavigationView {
  return cloneView(state)
}

function transition(state: NavigationState, view: NavigationView): NavigationState {
  if (sameView(state, view)) return state
  return { ...cloneView(view), history: [...state.history, currentView(state)] }
}

function replace(state: NavigationState, view: NavigationView): NavigationState {
  if (sameView(state, view)) return state
  return { ...cloneView(view), history: state.history }
}

export function createNavigationState(view: Partial<NavigationView> = {}): NavigationState {
  return {
    panelId: view.panelId,
    path: [...(view.path ?? [])],
    perspectiveId: view.perspectiveId,
    ...(view.drawer ? { drawer: cloneDrawer(view.drawer) } : {}),
    history: [],
  }
}

export function navigationReducer(state: NavigationState, action: NavigationAction): NavigationState {
  switch (action.type) {
    case 'drillInto':
      return transition(state, {
        panelId: action.panelId ?? state.panelId,
        path: action.path ?? [...state.path, action.nodeKey],
        perspectiveId: state.perspectiveId,
      })
    case 'back': {
      const previous = state.history.at(-1)
      if (!previous) return state
      return { ...cloneView(previous), history: state.history.slice(0, -1) }
    }
    case 'jumpTo': {
      if (action.breadcrumbIndex < 0 || action.breadcrumbIndex >= state.path.length - 1) return state
      return transition(state, { ...state, path: state.path.slice(0, action.breadcrumbIndex + 1) })
    }
    case 'switchPerspective': {
      const next = {
        ...state,
        panelId: action.panelId ?? state.panelId,
        path: action.path ?? state.path,
        perspectiveId: action.perspectiveId,
      }
      return action.replace ? replace(state, next) : transition(state, next)
    }
    case 'reset':
      return createNavigationState()
    case 'openDrawer':
      if (state.drawer) return state
      return transition(state, {
        ...state,
        drawer: { src: action.src, path: [] },
      })
    case 'updateDrawer':
      if (!state.drawer) return state
      return transition(state, {
        ...state,
        drawer: { src: state.drawer.src, ...action.view, path: [...action.view.path] },
      })
    case 'closeDrawer':
      if (!state.drawer) return state
      return replace(state, { panelId: state.panelId, path: state.path, perspectiveId: state.perspectiveId })
    case 'restore':
      return { ...cloneView(action.view), history: (action.history ?? []).map(cloneView) }
  }
}

export const navigationActions = {
  drillInto: (nodeKey: NodeKey, panelId?: string, path?: NodePath): NavigationAction => ({ type: 'drillInto', nodeKey, panelId, path }),
  back: (): NavigationAction => ({ type: 'back' }),
  jumpTo: (breadcrumbIndex: number): NavigationAction => ({ type: 'jumpTo', breadcrumbIndex }),
  switchPerspective: (
    perspectiveId: string,
    path?: NodePath,
    replace?: boolean,
    enterKey?: NodeKey,
    panelId?: string,
  ): NavigationAction => ({ type: 'switchPerspective', perspectiveId, path, replace, enterKey, panelId }),
  reset: (): NavigationAction => ({ type: 'reset' }),
  openDrawer: (src: string): NavigationAction => ({ type: 'openDrawer', src }),
  updateDrawer: (view: Omit<DrawerNavigationView, 'src'>): NavigationAction => ({ type: 'updateDrawer', view }),
  closeDrawer: (): NavigationAction => ({ type: 'closeDrawer' }),
  restore: (view: NavigationView, history?: Array<NavigationView>): NavigationAction => ({ type: 'restore', view, history }),
}
