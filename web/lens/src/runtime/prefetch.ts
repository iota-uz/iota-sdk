import type { DashboardDocument } from '../contract'
import { fetchDocument } from './document'

export interface DocumentCacheOptions {
  /** Maximum resolved documents kept before the oldest is evicted. */
  capacity?: number
  fetcher?: typeof fetch
  csrf?: string
}

/**
 * A tiny in-memory cache of drill-drawer documents, keyed by URL.
 *
 * The drawer used to fetch its document only on click. Prefetch warms the same
 * URL on hover/focus intent so the drawer opens against a document that is
 * already in hand. The cache never revalidates, a hit is authoritative, and a
 * failed prefetch leaves no entry. An interactive activation may overlap one
 * intent transport request so the shared server job can be promoted; it does
 * not duplicate datasource execution.
 */
export class DocumentCache {
  private readonly capacity: number
  // Insertion order is age order, so the first key is always the oldest.
  private readonly entries = new Map<string, DashboardDocument>()
  private readonly inflight = new Map<string, { promise: Promise<DashboardDocument>; speculative: boolean }>()
  private fetcher?: typeof fetch
  private csrf?: string

  constructor(options: DocumentCacheOptions = {}) {
    this.capacity = Math.max(1, Math.floor(options.capacity ?? 8))
    this.fetcher = options.fetcher
    this.csrf = options.csrf
  }

  /** Update the credentials used by later prefetches without dropping entries. */
  configure(options: { fetcher?: typeof fetch; csrf?: string }): void {
    this.fetcher = options.fetcher
    this.csrf = options.csrf
  }

  /** The cached document for a URL, or undefined on a miss. */
  get(url: string): DashboardDocument | undefined {
    return this.entries.get(url)
  }

  /**
   * Load a document through the shared cache/singleflight path.
   *
   * Unlike prefetch(), failures stay visible to the interactive caller. If a
   * speculative request is already running, opening the drawer promotes the
   * same server calculation through a separate activation request.
   */
  load(url: string): Promise<DashboardDocument> {
    const cached = this.entries.get(url)
    if (cached) return Promise.resolve(cached)
    const existing = this.inflight.get(url)
    if (existing && !existing.speculative) return existing.promise
    // A click sends a second transport activation while an intent request is
    // running. Both requests join the same server-side snapshot job; the
    // activation exists solely to promote that job to InteractiveCritical.
    const pending = fetchDocument(url, { fetcher: this.fetcher, csrf: this.csrf })
      .then((document) => {
        this.store(url, document)
        return document
      })
      .finally(() => {
        if (this.inflight.get(url)?.promise === pending) this.inflight.delete(url)
      })
    this.inflight.set(url, { promise: pending, speculative: false })
    return pending
  }

  /**
   * Warm a URL. Resolves once the fetch settles; a failure resolves (never
   * rejects) so a prefetch can never surface an error to the caller. A URL that
   * is already cached or already in flight is not fetched again.
   */
  prefetch(url: string, signal?: AbortSignal): Promise<void> {
    if (this.entries.has(url) || this.inflight.has(url)) return Promise.resolve()
    const pending = fetchDocument(url, { fetcher: this.fetcher, csrf: this.csrf, prefetch: true, signal })
      .then(async (document) => {
        if (!document.endpoints.panel || signal?.aborted) return document
        const { prefetchDocumentFirstRow } = await import('./prefetchPanels')
        return prefetchDocumentFirstRow(document, {
          csrf: this.csrf, fetcher: this.fetcher, signal,
        })
      })
      .then((document) => {
        this.store(url, document)
        return document
      })
      .finally(() => {
        if (this.inflight.get(url)?.promise === pending) this.inflight.delete(url)
      })
    this.inflight.set(url, { promise: pending, speculative: true })
    return pending.then(() => undefined, () => undefined)
  }

  private store(url: string, document: DashboardDocument): void {
    // Re-inserting refreshes recency; delete first so the key moves to the end.
    this.entries.delete(url)
    this.entries.set(url, document)
    while (this.entries.size > this.capacity) {
      const oldest = this.entries.keys().next().value
      if (oldest === undefined) break
      this.entries.delete(oldest)
    }
  }
}
