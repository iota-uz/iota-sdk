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
})
