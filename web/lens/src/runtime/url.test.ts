import { describe, expect, it } from 'vitest'
import { navigationFromURL, navigationToURL } from './url'

describe('navigation URL sync', () => {
  it('round-trips NodeKeys without treating slashes or labels as path syntax', () => {
    const view = { path: ['explorer/branch', 'node with spaces', 'child&value'], perspectiveId: 'composition/all' }
    const url = navigationToURL(view, new URL('https://example.test/dashboard?host=kept#section'))

    expect(url.searchParams.get('host')).toBe('kept')
    expect(url.hash).toBe('#section')
    expect(navigationFromURL(url)).toEqual(view)
  })

  it('removes stale Lens parameters for the root view', () => {
    const url = navigationToURL({ path: [] }, new URL('https://example.test/?path=old&perspective=old&host=kept'))
    expect(url.searchParams.toString()).toBe('host=kept')
    expect(navigationFromURL(url)).toEqual({ path: [], perspectiveId: undefined })
  })

  it('round-trips the drawer document and its active Lens view', () => {
    const view = {
      panelId: 'dashboard-panel',
      path: ['dashboard/root'],
      drawer: {
        src: '/analytics/drill/loss/lens/document?token=signed',
        panelId: 'drawer-panel',
        path: ['drawer/root', 'claims'],
        perspectiveId: 'evidence',
      },
    }
    const url = navigationToURL(view, new URL('https://example.test/analytics?tenant=kept'))

    expect(url.pathname).toBe('/analytics')
    expect(url.searchParams.get('tenant')).toBe('kept')
    expect(url.searchParams.get('drawer')).toBe('/analytics/drill/loss/lens/document?token=signed')
    expect(navigationFromURL(url)).toEqual({ path: ['dashboard/root'], perspectiveId: undefined, drawer: view.drawer })
  })

  it('rejects a cross-origin drawer source from a shared URL', () => {
    const url = new URL('https://example.test/dashboard?drawer=https%3A%2F%2Fevil.test%2Fdocument')
    expect(navigationFromURL(url)).toEqual({ path: [], perspectiveId: undefined })
  })
})
