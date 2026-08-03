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
      this.active?.controller.abort()
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
