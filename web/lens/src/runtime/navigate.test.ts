import { afterEach, describe, expect, it, vi } from 'vitest'
import { navigateTo } from './navigate'

const here = 'http://lens.test/analytics/profitability?ActualRangeStart=2025-01-01'

function stubLocation(href = here) {
  const assign = vi.fn()
  const url = new URL(href)
  vi.stubGlobal('location', {
    assign,
    href: url.href,
    origin: url.origin,
    pathname: url.pathname,
    search: url.search,
  })
  return assign
}

afterEach(() => {
  vi.unstubAllGlobals()
  document.body.innerHTML = ''
})

describe('navigateTo', () => {
  it('scrolls to a section on this page instead of reloading it', () => {
    const assign = stubLocation()
    const replaceState = vi.fn()
    vi.stubGlobal('history', { replaceState })
    const section = document.createElement('section')
    section.id = 'result'
    const scrollIntoView = vi.fn()
    section.scrollIntoView = scrollIntoView
    document.body.append(section)

    navigateTo('#result')

    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
    expect(replaceState).toHaveBeenCalledWith(null, '', '#result')
    // The whole point: the reader stays on the document they are reading.
    expect(assign).not.toHaveBeenCalled()
  })

  it('leaves a fragment alone when the section is not on the page', () => {
    const assign = stubLocation()

    navigateTo('#missing')

    expect(assign).toHaveBeenCalledWith('#missing')
  })

  it('navigates when the fragment belongs to a different report', () => {
    const assign = stubLocation()
    // Same path, different filter — a different document, so the server has to
    // answer for it even though the fragment matches.
    const target = 'http://lens.test/analytics/profitability?ActualRangeStart=2024-01-01#result'

    navigateTo(target)

    expect(assign).toHaveBeenCalledWith(target)
  })

  it('navigates for an ordinary URL', () => {
    const assign = stubLocation()

    navigateTo('/analytics/drill/earned_premium')

    expect(assign).toHaveBeenCalledWith('/analytics/drill/earned_premium')
  })

  it('navigates normally when a fragment has malformed encoding', () => {
    const assign = stubLocation()

    navigateTo('#%E0%A4%A')

    expect(assign).toHaveBeenCalledWith('#%E0%A4%A')
  })
})
