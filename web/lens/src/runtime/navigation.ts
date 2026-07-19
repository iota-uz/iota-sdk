import type { NodeKey, NodePath } from '../contract'

export interface NavigationView {
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
  | { type: 'switchPerspective'; perspectiveId: string; path?: NodePath }
  | { type: 'reset' }
  | { type: 'restore'; view: NavigationView; history?: Array<NavigationView> }

function cloneView(view: NavigationView): NavigationView {
  return {
    panelId: view.panelId,
    path: [...view.path],
    perspectiveId: view.perspectiveId,
  }
}

function samePath(left: NodePath, right: NodePath): boolean {
  return left.length === right.length && left.every((key, index) => key === right[index])
}

function sameView(left: NavigationView, right: NavigationView): boolean {
  return (
    left.panelId === right.panelId &&
    left.perspectiveId === right.perspectiveId &&
    samePath(left.path, right.path)
  )
}

function currentView(state: NavigationState): NavigationView {
  return cloneView(state)
}

function transition(state: NavigationState, view: NavigationView): NavigationState {
  if (sameView(state, view)) return state
  return { ...cloneView(view), history: [...state.history, currentView(state)] }
}

export function createNavigationState(view: Partial<NavigationView> = {}): NavigationState {
  return {
    panelId: view.panelId,
    path: [...(view.path ?? [])],
    perspectiveId: view.perspectiveId,
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
    case 'switchPerspective':
      return transition(state, {
        ...state,
        path: action.path ?? state.path,
        perspectiveId: action.perspectiveId,
      })
    case 'reset':
      return createNavigationState()
    case 'restore':
      return { ...cloneView(action.view), history: (action.history ?? []).map(cloneView) }
  }
}

export const navigationActions = {
  drillInto: (nodeKey: NodeKey, panelId?: string, path?: NodePath): NavigationAction => ({ type: 'drillInto', nodeKey, panelId, path }),
  back: (): NavigationAction => ({ type: 'back' }),
  jumpTo: (breadcrumbIndex: number): NavigationAction => ({ type: 'jumpTo', breadcrumbIndex }),
  switchPerspective: (perspectiveId: string, path?: NodePath): NavigationAction => ({ type: 'switchPerspective', perspectiveId, path }),
  reset: (): NavigationAction => ({ type: 'reset' }),
  restore: (view: NavigationView, history?: Array<NavigationView>): NavigationAction => ({ type: 'restore', view, history }),
}
