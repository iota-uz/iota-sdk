import type { RemoteFile } from './types'

/**
 * EditorStore is the framework-free state of the open draft files: buffers,
 * the server hash each buffer was based on, save conflicts and the guards that
 * keep late responses from overwriting newer state. Monaco and Solid only
 * render it.
 *
 * Every request the UI starts gets a token; a response is applied only when
 * its token is still the current one of that file and the package has not been
 * switched (reset) since. A response that lost the race is dropped and the
 * caller learns it by a false return.
 */

export type FileStatus = 'idle' | 'loading' | 'saving' | 'conflict' | 'error'

export interface FileEntry {
  path: string
  /** Server hash the buffer is based on; null while the file does not exist on the server. */
  baseHash: string | null
  /** Content at that hash. */
  baseContent: string
  buffer: string
  loaded: boolean
  status: FileStatus
  error?: string
  /** The server's version, known after a refused save or a conflict reload. */
  remote?: { hash: string; content: string }
}

export interface Token {
  readonly path: string
  readonly epoch: number
  readonly id: number
}

export interface SaveRequest {
  token: Token
  path: string
  content: string
  /** '' = the file must not exist yet; otherwise the hash it must still have. */
  expectHash: string
}

export type Listener = () => void

export class EditorStore {
  private files = new Map<string, FileEntry>()
  private pending = new Map<string, number>()
  private listeners = new Set<Listener>()
  private epoch = 0
  private seq = 0
  private version = 0

  /** Changes whenever any entry changes; usable as a signal source. */
  get revision(): number { return this.version }

  subscribe(fn: Listener): () => void {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }

  private touch(): void {
    this.version++
    for (const fn of [...this.listeners]) fn()
  }

  private token(path: string): Token {
    const id = ++this.seq
    this.pending.set(path, id)
    return { path, epoch: this.epoch, id }
  }

  private current(token: Token): FileEntry | undefined {
    if (token.epoch !== this.epoch || this.pending.get(token.path) !== token.id) return undefined
    return this.files.get(token.path)
  }

  /** Forgets everything and invalidates every request in flight (package switch). */
  reset(): void {
    this.epoch++
    this.files.clear()
    this.pending.clear()
    this.touch()
  }

  get(path: string): FileEntry | undefined { return this.files.get(path) }

  paths(): string[] { return [...this.files.keys()].sort() }

  entries(): FileEntry[] { return this.paths().map((p) => this.files.get(p) as FileEntry) }

  isDirty(path: string): boolean {
    const f = this.files.get(path)
    return !!f && (f.baseHash === null || f.buffer !== f.baseContent)
  }

  dirtyPaths(): string[] { return this.paths().filter((p) => this.isDirty(p)) }

  /**
   * applyListing merges the server's file list. Files that are gone from the
   * server disappear unless they hold unsaved work; files whose hash moved are
   * marked unloaded so the next open fetches them (a dirty buffer keeps its
   * edits and meets the change as a conflict on save).
   */
  applyListing(list: readonly RemoteFile[]): void {
    const remote = new Map(list.map((f) => [f.path, f]))
    for (const [path, entry] of this.files) {
      const r = remote.get(path)
      if (!r) {
        if (entry.baseHash !== null && !this.isDirty(path)) this.files.delete(path)
        continue
      }
      if (entry.baseHash !== r.hash && !this.isDirty(path)) {
        entry.baseHash = r.hash
        entry.loaded = false
      }
    }
    for (const r of list) {
      if (!this.files.has(r.path)) {
        this.files.set(r.path, { path: r.path, baseHash: r.hash, baseContent: '', buffer: '', loaded: false, status: 'idle' })
      }
    }
    this.touch()
  }

  /** create adds a new, unsaved file. It is refused when the path is taken. */
  create(path: string, content = ''): boolean {
    if (this.files.has(path)) return false
    this.files.set(path, { path, baseHash: null, baseContent: '', buffer: content, loaded: true, status: 'idle' })
    this.touch()
    return true
  }

  beginLoad(path: string): Token | undefined {
    const f = this.files.get(path)
    if (!f) return undefined
    if (f.status === 'idle' || f.status === 'error') f.status = f.loaded ? f.status : 'loading'
    this.touch()
    return this.token(path)
  }

  /**
   * finishLoad applies the server's content. First load fills the buffer; a
   * reload of a clean file refreshes it; during a conflict it only records the
   * server version; a file edited meanwhile keeps its buffer. Returns false
   * when the response was late or unwanted.
   */
  finishLoad(token: Token, data: { content: string; hash: string }): boolean {
    const f = this.current(token)
    if (!f) return false
    this.pending.delete(token.path)
    if (f.status === 'conflict') {
      f.remote = { hash: data.hash, content: data.content }
      this.touch()
      return true
    }
    if (f.loaded && f.buffer !== f.baseContent) {
      f.status = 'idle'
      this.touch()
      return false
    }
    f.baseHash = data.hash
    f.baseContent = data.content
    f.buffer = data.content
    f.loaded = true
    f.status = 'idle'
    delete f.error
    this.touch()
    return true
  }

  failLoad(token: Token, error: string): boolean {
    const f = this.current(token)
    if (!f) return false
    this.pending.delete(token.path)
    if (f.status !== 'conflict') f.status = 'error'
    f.error = error
    this.touch()
    return true
  }

  edit(path: string, content: string): void {
    const f = this.files.get(path)
    if (!f || f.buffer === content) return
    f.buffer = content
    this.touch()
  }

  /** beginSave starts a save of the current buffer, or returns undefined when there is nothing to do. */
  beginSave(path: string): SaveRequest | undefined {
    const f = this.files.get(path)
    if (!f || !f.loaded || f.status === 'saving' || f.status === 'conflict' || !this.isDirty(path)) return undefined
    f.status = 'saving'
    delete f.error
    const token = this.token(path)
    this.touch()
    return { token, path, content: f.buffer, expectHash: f.baseHash ?? '' }
  }

  /** finishSave records a successful save of req.content. Edits made while it ran stay dirty. */
  finishSave(req: SaveRequest, saved: { hash: string }): boolean {
    const f = this.current(req.token)
    if (!f) return false
    this.pending.delete(req.path)
    f.baseHash = saved.hash
    f.baseContent = req.content
    f.status = 'idle'
    delete f.error
    delete f.remote
    this.touch()
    return true
  }

  /** saveConflict marks a save refused because the server's file changed. */
  saveConflict(req: SaveRequest): boolean {
    const f = this.current(req.token)
    if (!f) return false
    this.pending.delete(req.path)
    f.status = 'conflict'
    delete f.remote
    this.touch()
    return true
  }

  failSave(req: SaveRequest, error: string): boolean {
    const f = this.current(req.token)
    if (!f) return false
    this.pending.delete(req.path)
    f.status = 'error'
    f.error = error
    this.touch()
    return true
  }

  /**
   * resolveConflict settles a conflict: 'mine' keeps the buffer and rebases it
   * on the server's version (the next save overwrites it knowingly), 'theirs'
   * takes the server's content. It needs the server version (finishLoad).
   */
  resolveConflict(path: string, choice: 'mine' | 'theirs'): boolean {
    const f = this.files.get(path)
    if (!f || f.status !== 'conflict' || !f.remote) return false
    f.baseHash = f.remote.hash
    f.baseContent = f.remote.content
    if (choice === 'theirs') f.buffer = f.remote.content
    f.status = 'idle'
    delete f.remote
    this.touch()
    return true
  }

  /** remove drops a file after the server deleted it (or a new file is discarded). */
  remove(path: string): void {
    this.pending.delete(path)
    if (this.files.delete(path)) this.touch()
  }

  /** renamed moves an entry after the server moved the file. */
  renamed(from: string, to: string): void {
    const f = this.files.get(from)
    if (!f || this.files.has(to)) return
    this.files.delete(from)
    this.pending.delete(from)
    this.files.set(to, { ...f, path: to })
    this.touch()
  }
}
