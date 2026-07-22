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
 * already in hand. The cache is deliberately dumb: it never revalidates, a hit
 * is authoritative, a failed prefetch leaves no entry (the click path refetches
 * and keeps its own error handling), and only one fetch per URL is ever in
 * flight at a time.
 */
export class DocumentCache {
  private readonly capacity: number
  // Insertion order is age order, so the first key is always the oldest.
  private readonly entries = new Map<string, DashboardDocument>()
  private readonly inflight = new Map<string, Promise<DashboardDocument>>()
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
   * Warm a URL. Resolves once the fetch settles; a failure resolves (never
   * rejects) so a prefetch can never surface an error to the caller. A URL that
   * is already cached or already in flight is not fetched again.
   */
  prefetch(url: string): Promise<void> {
    const existing = this.inflight.get(url)
    if (existing) return existing.then(() => undefined, () => undefined)
    if (this.entries.has(url)) return Promise.resolve()
    const pending = fetchDocument(url, { fetcher: this.fetcher, csrf: this.csrf })
      .then((document) => {
        this.inflight.delete(url)
        this.store(url, document)
        return document
      })
      .catch((cause: unknown) => {
        // A failed prefetch is silently dropped: no entry, and the click path
        // stays authoritative for surfacing the error.
        this.inflight.delete(url)
        throw cause
      })
    this.inflight.set(url, pending)
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
