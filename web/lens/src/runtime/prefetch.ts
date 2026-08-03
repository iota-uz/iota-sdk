import type { DashboardDocument } from '../contract'
import { fetchDocument } from './document'

export interface DocumentCacheOptions {
  /** Maximum resolved documents kept before the oldest is evicted. */
  capacity?: number
  fetcher?: typeof fetch
  csrf?: string
}

export interface IdlePrefetchQueueOptions {
  /** Total distinct candidates admitted for one snapshot. */
  capacity?: number
  /** Small grace period that lets visible work and user input go first. */
  delayMs?: number
}

interface IdlePrefetchTask {
  key: string
  consumers: number
  run: (signal: AbortSignal) => Promise<void>
}

/**
 * One-slot, bounded speculative queue shared by every panel in a runtime.
 * Registration order is DOM/layout reveal order. A concrete target is
 * deduplicated across panels, and reset() is the snapshot cancellation gate.
 */
export class IdlePrefetchQueue {
  private readonly capacity: number
  private readonly delayMs: number
  private readonly tasks = new Map<string, IdlePrefetchTask>()
  private readonly completed = new Set<string>()
  private pending: Array<IdlePrefetchTask> = []
  private active?: { task: IdlePrefetchTask; controller: AbortController }
  private timer?: ReturnType<typeof setTimeout>
  private paused = false

  constructor(options: IdlePrefetchQueueOptions = {}) {
    this.capacity = Math.max(1, Math.floor(options.capacity ?? 8))
    this.delayMs = Math.max(0, Math.floor(options.delayMs ?? 250))
  }

  register(key: string, run: (signal: AbortSignal) => Promise<void>): () => void {
    if (this.completed.has(key)) return () => undefined
    const existing = this.tasks.get(key)
    if (existing) {
      existing.consumers += 1
      return () => this.detach(existing)
    }
    if (this.tasks.size + this.completed.size >= this.capacity) return () => undefined
    const task: IdlePrefetchTask = { key, consumers: 1, run }
    this.tasks.set(key, task)
    this.pending.push(task)
    this.pump()
    return () => this.detach(task)
  }

  setPaused(paused: boolean): void {
    if (this.paused === paused) return
    this.paused = paused
    if (paused) {
      if (this.timer !== undefined) clearTimeout(this.timer)
      this.timer = undefined
      if (this.active) this.active.controller.abort()
      return
    }
    this.pump()
  }

  reset(): void {
    if (this.timer !== undefined) clearTimeout(this.timer)
    this.timer = undefined
    for (const task of this.tasks.values()) task.consumers = 0
    this.active?.controller.abort()
    this.active = undefined
    this.pending = []
    this.tasks.clear()
    this.completed.clear()
    this.paused = false
  }

  private detach(task: IdlePrefetchTask): void {
    task.consumers = Math.max(0, task.consumers - 1)
    if (task.consumers > 0) return
    this.tasks.delete(task.key)
    this.pending = this.pending.filter((candidate) => candidate !== task)
    if (this.active?.task === task) this.active.controller.abort()
  }

  private pump(): void {
    if (this.paused || this.active || this.timer !== undefined) return
    const task = this.pending.shift()
    if (!task) return
    if (task.consumers === 0) {
      this.tasks.delete(task.key)
      this.pump()
      return
    }
    this.timer = setTimeout(() => {
      this.timer = undefined
      if (this.paused || task.consumers === 0) {
        if (task.consumers > 0) this.pending.unshift(task)
        this.pump()
        return
      }
      const controller = new AbortController()
      this.active = { task, controller }
      void task.run(controller.signal).then(() => {
        if (!controller.signal.aborted) this.completed.add(task.key)
      }, () => undefined).finally(() => {
        if (this.active?.task === task) this.active = undefined
        if (controller.signal.aborted && task.consumers > 0 && !this.completed.has(task.key)) {
          this.pending.unshift(task)
        } else {
          this.tasks.delete(task.key)
        }
        this.pump()
      })
    }, this.delayMs)
  }
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
   * Load a document through the shared cache/singleflight path.
   *
   * Unlike prefetch(), failures stay visible to the interactive caller. If a
   * speculative request is already running, opening the drawer joins that
   * exact promise instead of starting a second document calculation.
   */
  load(url: string): Promise<DashboardDocument> {
    const cached = this.entries.get(url)
    if (cached) return Promise.resolve(cached)
    const existing = this.inflight.get(url)
    if (existing) return existing
    const pending = fetchDocument(url, { fetcher: this.fetcher, csrf: this.csrf })
      .then((document) => {
        this.store(url, document)
        return document
      })
      .finally(() => {
        if (this.inflight.get(url) === pending) this.inflight.delete(url)
      })
    this.inflight.set(url, pending)
    return pending
  }

  /**
   * Warm a URL. Resolves once the fetch settles; a failure resolves (never
   * rejects) so a prefetch can never surface an error to the caller. A URL that
   * is already cached or already in flight is not fetched again.
   */
  prefetch(url: string): Promise<void> {
    return this.load(url).then(() => undefined, () => undefined)
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
