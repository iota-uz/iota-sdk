import type { DiagnosticsModel, Marker } from './diagnostics'
import type { EditorStore } from './editorState'
import { languageOf, themeName } from './monaco'
import { modelUri } from './monacoTypes'

/** The slice of Monaco this binding uses; the real API satisfies it, tests fake it. */
export interface Disposable { dispose(): void }

export interface ModelLike {
  getValue(): string
  setValue(value: string): void
  getLineContent(line: number): string
  getLineCount(): number
  onDidChangeContent(listener: () => void): Disposable
  dispose(): void
}

export interface EditorLike {
  setModel(model: ModelLike | null): void
  focus(): void
  layout(): void
  revealPositionInCenter(position: { lineNumber: number; column: number }): void
  setPosition(position: { lineNumber: number; column: number }): void
  updateOptions(options: Record<string, unknown>): void
  dispose(): void
}

export interface MonacoLike<M extends ModelLike = ModelLike, E extends EditorLike = EditorLike> {
  editor: {
    create(container: HTMLElement, options: Record<string, unknown>): E
    createModel(value: string, language: string, uri: unknown): M
    setModelMarkers(model: M, owner: string, markers: Marker[]): void
    setTheme(name: string): void
  }
  Uri: { parse(uri: string): unknown }
}

export const MARKER_OWNER = 'iota-check'

export interface BindingOptions {
  theme: 'light' | 'dark'
  readOnly: boolean
}

/**
 * EditorBinding keeps one Monaco editor and one model per loaded draft file in
 * step with the EditorStore, and draws the server's diagnostics as markers.
 * Models exist for every loaded file, not only the visible one, so the
 * TypeScript worker resolves imports between draft files (and local contracts)
 * before anything was ever activated. dispose() releases the editor, every
 * model, all listeners and the markers; afterwards the binding ignores calls.
 */
export class EditorBinding<M extends ModelLike = ModelLike, E extends EditorLike = EditorLike> {
  private readonly editor: E
  private readonly models = new Map<string, { model: M; listener: Disposable }>()
  private readonly unsubscribe: () => void
  private applying = false
  private disposed = false
  private active: string | undefined

  constructor(
    private readonly monaco: MonacoLike<M, E>,
    container: HTMLElement,
    private readonly store: EditorStore,
    private readonly diagnostics: DiagnosticsModel,
    options: BindingOptions,
  ) {
    this.editor = monaco.editor.create(container, {
      automaticLayout: true, readOnly: options.readOnly, theme: themeName(options.theme), minimap: { enabled: false },
      scrollBeyondLastLine: false, tabSize: 2, fontSize: 13, model: null,
    })
    this.unsubscribe = store.subscribe(() => this.sync())
    this.sync()
  }

  get activePath(): string | undefined { return this.active }

  /** open shows the file, if its content is loaded. */
  open(path: string): void {
    if (this.disposed) return
    this.active = path
    this.sync()
  }

  /** sync reconciles models, buffers and markers with the store. */
  sync(): void {
    if (this.disposed) return
    this.applying = true
    try {
      const entries = new Map(this.store.entries().filter((e) => e.loaded).map((e) => [e.path, e]))
      for (const [path, h] of [...this.models]) {
        if (!entries.has(path)) {
          this.monaco.editor.setModelMarkers(h.model, MARKER_OWNER, [])
          h.listener.dispose()
          h.model.dispose()
          this.models.delete(path)
        }
      }
      for (const [path, entry] of entries) {
        let h = this.models.get(path)
        if (!h) {
          const model = this.monaco.editor.createModel(entry.buffer, languageOf(path), this.monaco.Uri.parse(modelUri(path)))
          h = { model, listener: model.onDidChangeContent(() => this.onModelChange(path)) }
          this.models.set(path, h)
        } else if (h.model.getValue() !== entry.buffer) {
          h.model.setValue(entry.buffer)
        }
        this.monaco.editor.setModelMarkers(h.model, MARKER_OWNER, this.markers(path, h.model))
      }
      const shown = this.active ? this.models.get(this.active) : undefined
      this.editor.setModel(shown ? shown.model : null)
    } finally {
      this.applying = false
    }
  }

  private markers(path: string, model: M): Marker[] {
    const entry = this.store.get(path)
    if (!entry) return []
    return this.diagnostics.markersFor(path, { baseHash: entry.baseHash, dirty: this.store.isDirty(path) }, (n) => (n >= 1 && n <= model.getLineCount() ? model.getLineContent(n) : undefined), model.getLineCount())
  }

  private onModelChange(path: string): void {
    if (this.disposed || this.applying) return
    const h = this.models.get(path)
    if (h) this.store.edit(path, h.model.getValue())
  }

  /** reveal moves the cursor to a diagnostic's position, opening its file. */
  reveal(path: string, line: number, column: number): void {
    if (this.disposed) return
    this.open(path)
    const position = { lineNumber: Math.max(line, 1), column: Math.max(column, 1) }
    this.editor.revealPositionInCenter(position)
    this.editor.setPosition(position)
    this.editor.focus()
  }

  setTheme(theme: 'light' | 'dark'): void {
    if (!this.disposed) this.monaco.editor.setTheme(themeName(theme))
  }

  setReadOnly(readOnly: boolean): void {
    if (!this.disposed) this.editor.updateOptions({ readOnly })
  }

  layout(): void {
    if (!this.disposed) this.editor.layout()
  }

  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.unsubscribe()
    this.editor.setModel(null)
    for (const { model, listener } of this.models.values()) {
      this.monaco.editor.setModelMarkers(model, MARKER_OWNER, [])
      listener.dispose()
      model.dispose()
    }
    this.models.clear()
    this.editor.dispose()
  }
}
