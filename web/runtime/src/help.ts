// Links from package pages to the package's own end-user documentation, which the Help Center serves at
// `<help base>/doc/apps/<namespace>/<document path>` (pkg/extruntime/pkgdocs). The document paths are the
// `docs/<locale>/<path>.md` files of the package without locale and extension; `iota-ext check` verifies every
// `helpHref("literal")` call against them.

/** Where the application mounts the Help Center. */
export const HELP_BASE = '/help'
/** The Help Center mount of package documentation (pkgdocs.Mount). */
export const HELP_APPS_MOUNT = 'apps'

const DOC_PATH = /^[a-z0-9][a-z0-9_-]*(\/[a-z0-9][a-z0-9_-]*)*$/
const NAMESPACE = /^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$/

export interface HelpHrefOptions {
  /** The package whose documentation to open. Default: the package the calling code belongs to. */
  namespace?: string
  /** Where the Help Center is mounted. Default: `/help`. */
  base?: string
}

/** Address of a document of `namespace` (`path` may end with `#section`). Throws on a malformed argument. */
export function helpDocHref(namespace: string, path: string, base: string = HELP_BASE): string {
  if (!NAMESPACE.test(namespace)) throw new Error(`helpHref: "${namespace}" is not a package namespace`)
  const [doc = '', ...fragment] = path.split('#')
  if (!DOC_PATH.test(doc)) {
    throw new Error(`helpHref: "${path}" is not a document path (the file docs/<locale>/<path>.md without the locale and .md, e.g. "getting-started")`)
  }
  const anchor = fragment.length > 0 ? `#${encodeURIComponent(fragment.join('#'))}` : ''
  return `${base.replace(/\/+$/, '')}/doc/${HELP_APPS_MOUNT}/${namespace}/${doc}${anchor}`
}

/** The package of the runtime page in the document: the host publishes it with the page's client context. */
function pageNamespace(): string | undefined {
  if (typeof document === 'undefined') return undefined
  try {
    const raw = document.getElementById('iota-client-context')?.textContent
    const initial = raw ? (JSON.parse(raw) as { initial?: { page?: { package?: unknown } } }).initial : undefined
    const name = initial?.page?.package
    return typeof name === 'string' && NAMESPACE.test(name) ? name : undefined
  } catch {
    return undefined
  }
}

/**
 * Link to a document of the package's own documentation, for `<a href={helpHref("getting-started")}>`.
 * `iota-ext check` reports a literal that names no `docs/en/<path>.md`.
 *
 * In a package bundle (anything built by `iota-ext build`) the default package is the one the bundle belongs to: the build binds
 * this function to the package namespace, so a slot or cell of package B rendered on package A's page, or on a native page, links
 * to B's own documents. An explicit `{ namespace }` names another package's. The fallback below (the package of the runtime page
 * being shown) serves code the build did not bind, such as the host itself.
 */
export function helpHref(path: string, options: HelpHrefOptions = {}): string {
  const namespace = options.namespace ?? pageNamespace()
  if (namespace === undefined) {
    throw new Error('helpHref: the package is unknown here; pass { namespace } (the page context names it only on runtime pages)')
  }
  return helpDocHref(namespace, path, options.base)
}
