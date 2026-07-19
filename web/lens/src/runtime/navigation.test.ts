import { describe, expect, it } from 'vitest'
import { createNavigationState, navigationActions, navigationReducer } from './navigation'

describe('navigationReducer', () => {
  it('drills by NodeKey and records the previous view', () => {
    const initial = createNavigationState({ panelId: 'total', path: ['root'], perspectiveId: 'composition' })
    const next = navigationReducer(initial, navigationActions.drillInto('detail'))

    expect(next).toEqual({
      panelId: 'total', path: ['root', 'detail'], perspectiveId: 'composition',
      history: [{ panelId: 'total', path: ['root'], perspectiveId: 'composition' }],
    })
    expect(initial.path).toEqual(['root'])
  })

  it('backs through history and is stable at the boundary', () => {
    const root = createNavigationState({ path: ['root'] })
    const detail = navigationReducer(root, navigationActions.drillInto('detail'))
    const leaf = navigationReducer(detail, navigationActions.drillInto('leaf'))

    const backToDetail = navigationReducer(leaf, navigationActions.back())
    const backToRoot = navigationReducer(backToDetail, navigationActions.back())
    expect(backToDetail.path).toEqual(['root', 'detail'])
    expect(backToRoot.path).toEqual(['root'])
    expect(navigationReducer(backToRoot, navigationActions.back())).toBe(backToRoot)
  })

  it.each([
    { index: 0, expected: ['root'] },
    { index: 1, expected: ['root', 'detail'] },
  ])('jumps to breadcrumb $index and records history', ({ index, expected }) => {
    const state = createNavigationState({ path: ['root', 'detail', 'leaf'] })
    const next = navigationReducer(state, navigationActions.jumpTo(index))
    expect(next.path).toEqual(expected)
    expect(next.history).toHaveLength(1)
  })

  it.each([-1, 2, 3])('ignores breadcrumb edge index %s', (index) => {
    const state = createNavigationState({ path: ['root', 'detail', 'leaf'] })
    expect(navigationReducer(state, navigationActions.jumpTo(index))).toBe(state)
  })

  it('switches perspective without changing path and ignores an identical switch', () => {
    const initial = createNavigationState({ path: ['root'], perspectiveId: 'old' })
    const next = navigationReducer(initial, navigationActions.switchPerspective('new'))
    expect(next.path).toEqual(['root'])
    expect(next.perspectiveId).toBe('new')
    expect(next.history).toHaveLength(1)
    expect(navigationReducer(next, navigationActions.switchPerspective('new'))).toBe(next)
  })

  it('resets the view and history', () => {
    const state = navigationReducer(createNavigationState({ path: ['root'] }), navigationActions.drillInto('detail'))
    expect(navigationReducer(state, navigationActions.reset())).toEqual({ path: [], history: [] })
  })

  it('restores an external view without retaining internal history', () => {
    const state = navigationReducer(createNavigationState({ path: ['root'] }), navigationActions.drillInto('detail'))
    expect(navigationReducer(state, navigationActions.restore({ path: ['external'], perspectiveId: 'p' }))).toEqual({
      path: ['external'], perspectiveId: 'p', history: [], panelId: undefined,
    })
  })
})
