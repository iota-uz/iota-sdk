import type { DashboardDocument } from '../contract'
import { fetchDocument } from './document'
import { PanelClient } from './panel'

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
      .then((document) => this.prefetchFirstRow(document, signal))
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

  /**
   * Carry useful child data all the way into the browser cache. A prefetched
   * drawer document used to contain only its layout, so opening it still
   * triggered the same first-row panel request as a cold click. Warming just
   * the first content row gives the user immediate figures while preserving a
   * bounded one-surface budget; lower rows remain progressive after open.
   */
  private async prefetchFirstRow(document: DashboardDocument, signal?: AbortSignal): Promise<DashboardDocument> {
    const endpoint = document.endpoints.panel
    const firstRow = document.layout.rows.find(({ panels }) => panels.length > 0)
    if (!endpoint || !firstRow || signal?.aborted) return document
    const ids = new Set(firstRow.panels.map(({ panelId }) => panelId))
    const candidates = document.panels.filter(({ id, deferred }) => deferred === true && ids.has(id))
    if (candidates.length === 0) return document

    const client = new PanelClient(endpoint, { csrf: this.csrf, fetcher: this.fetcher, prefetch: true })
    try {
      const results = await client.loadBatch(candidates.map(({ id }) => ({
        snapshotId: document.snapshotId,
        panelId: id,
      })), { signal })
      const frames = { ...document.frames }
      const hydrated = new Set<string>()
      for (const panel of candidates) {
        const result = results[panel.id]
        if (!result?.frames || result.error) continue
        const loaded = result.frames[panel.frame] ?? Object.values(result.frames)[0]
        if (!loaded) continue
        frames[panel.frame] = loaded
        hydrated.add(panel.id)
      }
      if (hydrated.size === 0) return document
      return {
        ...document,
        frames,
        panels: document.panels.map((panel) => hydrated.has(panel.id) ? { ...panel, deferred: false } : panel),
      }
    } finally {
      client.dispose()
    }
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
