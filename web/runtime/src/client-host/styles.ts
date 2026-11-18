// Version stylesheets. A package version's client build records its stylesheets (Tailwind utilities, scoped CSS
// modules) in the immutable artifact; the server hands the URLs to every place that mounts the version's code (page,
// slot, cell, shell). This module makes sure each URL is linked once in the document, and resolved before the first render.
//
// The links belong to the document's <head>, not to the island or fragment that asked for them: an HTMX swap that removes
// an island must not take the stylesheet of its neighbours with it, and a second island of the same version must not link
// it again. A link the server already wrote (a runtime page links its version's sheet in the HTML) is reused wherever it is.

/** Marks the stylesheet links this module creates. */
export const STYLE_ATTR = 'data-iota-ext-css'

/** How long a stylesheet may take before the first render stops waiting for it (the page renders unstyled rather than never). */
const STYLE_TIMEOUT_MS = 10_000

function find(doc: Document, href: string): HTMLLinkElement | undefined {
  const absolute = new URL(href, doc.baseURI).href
  return Array.from(doc.querySelectorAll<HTMLLinkElement>('link[rel~="stylesheet"]')).find((link) => link.href === absolute)
}

// What this module has seen of each link (a browser sets `sheet` for a loaded stylesheet; the record also covers a link
// whose load event fired before the next island asked, and failed ones, which do not make the next island wait again).
const outcome = new WeakMap<HTMLLinkElement, boolean>()

function settled(link: HTMLLinkElement, win: Window): Promise<boolean> {
  // `sheet` is set once the stylesheet has loaded and parsed; a link the parser created is usually done already.
  if (link.sheet !== null) return Promise.resolve(true)
  const known = outcome.get(link)
  if (known !== undefined) return Promise.resolve(known)
  return new Promise((resolve) => {
    const finish = (ok: boolean) => {
      win.clearTimeout(timer)
      link.removeEventListener('load', onLoad)
      link.removeEventListener('error', onError)
      outcome.set(link, ok)
      resolve(ok)
    }
    const onLoad = () => finish(true)
    const onError = () => finish(false)
    const timer = win.setTimeout(() => finish(false), STYLE_TIMEOUT_MS)
    link.addEventListener('load', onLoad)
    link.addEventListener('error', onError)
  })
}

/**
 * Links `hrefs` in `doc` (those not linked yet) and resolves once all of them loaded. Never rejects: a stylesheet that
 * fails to load is reported to the console and the component renders without it.
 */
export async function ensureStyles(doc: Document | undefined, hrefs: readonly string[] | undefined): Promise<void> {
  const win = doc?.defaultView
  if (!doc || !win || !hrefs || hrefs.length === 0) return
  await Promise.all([...new Set(hrefs)].map(async (href) => {
    let link = find(doc, href)
    if (!link) {
      link = doc.createElement('link')
      link.rel = 'stylesheet'
      link.href = href
      link.setAttribute(STYLE_ATTR, '')
      doc.head.append(link)
    }
    if (!(await settled(link, win))) win.console.error(`Extension stylesheet ${href} could not be loaded; its components render without it`)
  }))
}
