import { ApiError } from './api'
import { DiagnosticsModel, type FileState } from './diagnostics'
import { EditorStore } from './editorState'
import type { BuildReport, CheckReport, Diagnostic, RemoteFile, TestReport } from './types'

/** The part of ManageApi a workspace needs (tests substitute a fake). */
export interface WorkspaceApi {
  listFiles(ns: string, signal?: AbortSignal): Promise<RemoteFile[]>
  readFile(ns: string, path: string, signal?: AbortSignal): Promise<{ content: string; hash: string }>
  writeFile(ns: string, path: string, content: string, expectHash: string | null, signal?: AbortSignal): Promise<RemoteFile>
  deleteFile(ns: string, path: string): Promise<void>
  moveFile(ns: string, from: string, to: string): Promise<void>
  check(ns: string, signal?: AbortSignal): Promise<CheckReport>
  build(ns: string, signal?: AbortSignal): Promise<BuildReport>
  test(ns: string, signal?: AbortSignal): Promise<TestReport>
}

export type RunKind = 'check' | 'build' | 'test'

export type RunResult =
  | { kind: 'check'; report: CheckReport }
  | { kind: 'build'; report: BuildReport }
  | { kind: 'test'; report: TestReport }

export type SaveOutcome = 'saved' | 'conflict' | 'error' | 'skipped'

export interface WorkspaceEvents {
  /** The state changed in a way the UI does not get from the store (runs, busy flag). */
  changed(): void
  /** A user-visible problem that is not a file state (a failed load, a refused action). */
  problem(message: string): void
}

const LOAD_CONCURRENCY = 4

/**
 * Workspace is everything the editor does for one package draft. It owns the
 * EditorStore and DiagnosticsModel, talks to the management API and applies
 * every response through the store's tokens, so a response that returns after
 * the package was closed (dispose) or after a newer request changes nothing.
 *
 * check/build/test run on the SAVED draft: they save the open files first and
 * refuse to run while a file is in conflict, so the diagnostics always describe
 * hashes the store knows.
 */
export class Workspace {
  readonly store = new EditorStore()
  readonly diagnostics = new DiagnosticsModel()
  private readonly abort = new AbortController()
  private disposed = false
  private busyCount = 0
  lastRun: (RunResult & { at: number }) | undefined

  constructor(
    private readonly api: WorkspaceApi,
    readonly namespace: string,
    private readonly events: WorkspaceEvents,
  ) {}

  get busy(): boolean { return this.busyCount > 0 }

  private begin(): void { this.busyCount++; this.events.changed() }
  private end(): void { this.busyCount--; if (!this.disposed) this.events.changed() }

  fileStates(): Map<string, FileState> {
    return new Map(this.store.entries().map((e) => [e.path, { baseHash: e.baseHash, dirty: this.store.isDirty(e.path) }]))
  }

  /** open lists the draft and loads every file, a few at a time. */
  async open(): Promise<void> {
    this.begin()
    try {
      const list = await this.api.listFiles(this.namespace, this.abort.signal)
      if (this.disposed) return
      this.store.applyListing(list)
      const queue = list.map((f) => f.path)
      const worker = async (): Promise<void> => {
        for (let path = queue.shift(); path !== undefined; path = queue.shift()) await this.load(path)
      }
      await Promise.all(Array.from({ length: Math.min(LOAD_CONCURRENCY, queue.length) }, worker))
    } catch (err) {
      if (!this.disposed) this.events.problem(messageOf(err))
    } finally {
      this.end()
    }
  }

  /** load fetches one file; the result is applied only if it is still wanted. */
  async load(path: string): Promise<boolean> {
    const token = this.store.beginLoad(path)
    if (!token) return false
    try {
      const r = await this.api.readFile(this.namespace, path, this.abort.signal)
      return this.store.finishLoad(token, r)
    } catch (err) {
      if (!this.disposed) this.store.failLoad(token, messageOf(err))
      return false
    }
  }

  /** save writes one file with the hash it was based on. */
  async save(path: string): Promise<SaveOutcome> {
    const req = this.store.beginSave(path)
    if (!req) return 'skipped'
    try {
      const saved = await this.api.writeFile(this.namespace, path, req.content, req.expectHash, this.abort.signal)
      return this.store.finishSave(req, saved) ? 'saved' : 'skipped'
    } catch (err) {
      if (this.disposed) return 'skipped'
      if (err instanceof ApiError && err.isConflict) {
        if (!this.store.saveConflict(req)) return 'skipped'
        await this.load(path) // fetch the server's version for the conflict view
        return 'conflict'
      }
      this.store.failSave(req, messageOf(err))
      return 'error'
    }
  }

  /** saveAll saves every dirty file. It is true when nothing unsaved or conflicting remains. */
  async saveAll(): Promise<boolean> {
    this.begin()
    try {
      await Promise.all(this.store.dirtyPaths().map((p) => this.save(p)))
      if (this.disposed) return false
      return this.store.entries().every((e) => !this.store.isDirty(e.path) && e.status !== 'conflict' && e.status !== 'error')
    } finally {
      this.end()
    }
  }

  private savedHashes(): Record<string, string | null> {
    const out: Record<string, string | null> = {}
    for (const e of this.store.entries()) if (!this.store.isDirty(e.path)) out[e.path] = e.baseHash
    return out
  }

  /** run saves the draft, then runs the server-side check, build or test on it. */
  async run(kind: RunKind): Promise<RunResult | undefined> {
    this.begin()
    try {
      if (!(await this.saveAll())) {
        if (!this.disposed) this.events.problem('unsaved')
        return undefined
      }
      const hashes = this.savedHashes()
      let result: RunResult
      switch (kind) {
        case 'check': result = { kind, report: await this.api.check(this.namespace, this.abort.signal) }; break
        case 'build': result = { kind, report: await this.api.build(this.namespace, this.abort.signal) }; break
        case 'test': result = { kind, report: await this.api.test(this.namespace, this.abort.signal) }; break
      }
      if (this.disposed) return undefined
      if (result.kind !== 'test') this.diagnostics.set(result.report.diagnostics, hashes)
      this.lastRun = { ...result, at: Date.now() }
      return result
    } catch (err) {
      if (this.disposed) return undefined
      this.recordDiagnosticsFrom(err)
      this.events.problem(messageOf(err))
      return undefined
    } finally {
      this.end()
    }
  }

  /** recordDiagnosticsFrom draws the diagnostics of a refused publish. */
  recordDiagnosticsFrom(err: unknown): void {
    if (this.disposed || !(err instanceof ApiError) || err.diagnostics.length === 0) return
    this.diagnostics.set(err.diagnostics as Diagnostic[], this.savedHashes())
    this.events.changed()
  }

  async createFile(path: string): Promise<boolean> {
    if (!validPath(path) || !this.store.create(path)) return false
    return (await this.save(path)) === 'saved'
  }

  async deleteFile(path: string): Promise<boolean> {
    const e = this.store.get(path)
    if (!e) return false
    if (e.baseHash === null) { // never saved: just discard
      this.store.remove(path)
      return true
    }
    this.begin()
    try {
      await this.api.deleteFile(this.namespace, path)
      if (this.disposed) return false
      this.store.remove(path)
      return true
    } catch (err) {
      if (!this.disposed) this.events.problem(messageOf(err))
      return false
    } finally {
      this.end()
    }
  }

  async rename(from: string, to: string): Promise<boolean> {
    const e = this.store.get(from)
    if (!e || !validPath(to) || this.store.get(to)) return false
    if (this.store.isDirty(from) && !(await this.save(from))) return false
    this.begin()
    try {
      await this.api.moveFile(this.namespace, from, to)
      if (this.disposed) return false
      this.store.renamed(from, to)
      return true
    } catch (err) {
      if (!this.disposed) this.events.problem(messageOf(err))
      return false
    } finally {
      this.end()
    }
  }

  /** dispose cancels requests in flight and makes every later response a no-op. */
  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.abort.abort()
    this.store.reset()
  }
}

export function validPath(path: string): boolean {
  return path !== '' && !path.startsWith('/') && !path.endsWith('/') && !path.split('/').some((s) => s === '' || s === '.' || s === '..')
}

function messageOf(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}
