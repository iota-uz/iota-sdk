import { describe, expect, it } from 'vitest'
import { drawerNavigationFromSource, hiddenSeriesFromURL, hiddenSeriesToURL, navigationFromURL, navigationToURL } from './url'

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

  it('reads a drawer document initial view from the document source', () => {
    const current = new URL('https://example.test/analytics?tenant=kept')
    const source = '/analytics/family/lens/document?family=expenses&path=expenses&path=expenses%2Facquisition&perspective=expenses%2Facquisition%2Fcomposition'

    expect(drawerNavigationFromSource(source, current)).toEqual({
      path: ['expenses', 'expenses/acquisition'],
      perspectiveId: 'expenses/acquisition/composition',
    })
  })
})

describe('legend URL sync', () => {
  it('round-trips hidden series per panel while preserving filters and other panels', () => {
    let url = new URL('https://example.test/dashboard?start=2026-07-01')
    url = hiddenSeriesToURL(url, 'revenue', new Set(['КАСКО', 'A/B']))
    url = hiddenSeriesToURL(url, 'count', new Set(['ОСАГО']))

    expect(hiddenSeriesFromURL(url, 'revenue')).toEqual(new Set(['A/B', 'КАСКО']))
    expect(hiddenSeriesFromURL(url, 'count')).toEqual(new Set(['ОСАГО']))
    expect(url.searchParams.get('start')).toBe('2026-07-01')

    url = hiddenSeriesToURL(url, 'revenue', new Set())
    expect(hiddenSeriesFromURL(url, 'revenue').size).toBe(0)
    expect(hiddenSeriesFromURL(url, 'count')).toEqual(new Set(['ОСАГО']))
  })

  it('ignores malformed shared URL entries', () => {
    const url = new URL('https://example.test/dashboard?lensHidden=broken&lensHidden=%5B%22panel%22%5D')
    expect(hiddenSeriesFromURL(url, 'panel').size).toBe(0)
  })
})
