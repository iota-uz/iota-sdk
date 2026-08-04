/**
 * Full-page navigation seam. Panel-level actions are plain document
 * navigations; routing them through one function keeps the host page able to
 * intercept them and makes the behaviour testable.
 *
 * A target inside the current document is the exception: a summary card whose
 * subject is expanded by a section further down the page links to that section,
 * and reloading the page to reach a heading it is already showing would be
 * absurd. Those scroll instead.
 */
export function navigateTo(url: string): void {
  const anchor = sameDocumentAnchor(url)
  if (anchor) {
    scrollToAnchor(anchor)
    return
  }
  globalThis.location.assign(url)
}

/** The element id `url` points at, when it points inside the current page. */
function sameDocumentAnchor(url: string): string | undefined {
  const here = globalThis.location
  if (!here) return undefined
  let target: URL
  try {
    target = new URL(url, here.href)
  } catch {
    return undefined
  }
  if (!target.hash || target.hash === '#') return undefined
  // Same document means same everything-but-the-fragment. A different query is
  // a different report, so it has to go through the server.
  if (target.origin !== here.origin || target.pathname !== here.pathname || target.search !== here.search) {
    return undefined
  }
  try {
    return decodeURIComponent(target.hash.slice(1))
  } catch {
    return undefined
  }
}

function scrollToAnchor(id: string): void {
  const element = globalThis.document?.getElementById(id)
  if (!element) {
    // The section is not on this page after all — let the browser resolve the
    // fragment the ordinary way rather than swallowing the click.
    globalThis.location.assign(`#${id}`)
    return
  }
  const reduced = globalThis.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  element.scrollIntoView({ behavior: reduced ? 'auto' : 'smooth', block: 'start' })
  // Leave the fragment in the address bar so the view is shareable, without
  // spending a history entry on every click of the same card.
  globalThis.history?.replaceState(null, '', `#${id}`)
}
