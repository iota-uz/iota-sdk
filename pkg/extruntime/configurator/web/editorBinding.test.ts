import { describe, expect, it, vi } from 'vitest'
import { DiagnosticsModel } from './diagnostics'
import { EditorBinding, MARKER_OWNER, type EditorLike, type ModelLike, type MonacoLike } from './editorBinding'
import { EditorStore } from './editorState'
import type { Marker } from './diagnostics'

class FakeModel implements ModelLike {
  disposed = false
  private listeners = new Set<() => void>()
  constructor(public value: string, readonly language: string, readonly uri: string) {}
  getValue() { return this.value }
  setValue(v: string) { this.value = v; this.fire() }
  getLineContent(n: number) { return this.value.split('\n')[n - 1] ?? '' }
  getLineCount() { return this.value.split('\n').length }
  onDidChangeContent(l: () => void) { this.listeners.add(l); return { dispose: () => this.listeners.delete(l) } }
  dispose() { this.disposed = true }
  type(v: string) { this.value = v; this.fire() }
  fire() { for (const l of [...this.listeners]) l() }
  get listenerCount() { return this.listeners.size }
}

class FakeEditor implements EditorLike {
  model: FakeModel | null = null
  disposed = false
  position: { lineNumber: number; column: number } | undefined
  options: Record<string, unknown> = {}
  setModel(m: ModelLike | null) { this.model = m as FakeModel | null }
  focus() {}
  layout() {}
  revealPositionInCenter() {}
  setPosition(p: { lineNumber: number; column: number }) { this.position = p }
  updateOptions(o: Record<string, unknown>) { Object.assign(this.options, o) }
  dispose() { this.disposed = true }
}

function setup() {
  const models: FakeModel[] = []
  const markers = new Map<string, Marker[]>()
  let editor!: FakeEditor
  let theme = ''
  const monaco: MonacoLike<FakeModel, FakeEditor> = {
    editor: {
      create: (_c, o) => { editor = new FakeEditor(); editor.options = o; return editor },
      createModel: (value, language, uri) => { const m = new FakeModel(value, language, String(uri)); models.push(m); return m },
      setModelMarkers: (m, owner, ms) => { if (owner === MARKER_OWNER) markers.set(m.uri, ms) },
      setTheme: (n) => { theme = n },
    },
    Uri: { parse: (s) => s },
  }
  const store = new EditorStore()
  const diagnostics = new DiagnosticsModel()
  const load = (path: string, content: string, hash = 'h1') => {
    store.applyListing([...store.entries().map((e) => ({ path: e.path, hash: e.baseHash ?? '', size: 1, updatedBy: 1, updatedAt: '' })), { path, hash, size: 1, updatedBy: 1, updatedAt: '' }])
    store.finishLoad(store.beginLoad(path)!, { content, hash })
  }
  const binding = new EditorBinding(monaco, document.createElement('div'), store, diagnostics, { theme: 'light', readOnly: false })
  return { monaco, models, markers, store, diagnostics, binding, load, editor: () => editor, theme: () => theme }
}

describe('EditorBinding', () => {
  it('creates a model for every loaded file, so imports between draft files resolve', () => {
    const t = setup()
    t.load('src/contracts.ts', 'export const a = 1')
    t.load('src/page.tsx', 'import { a } from "./contracts"')
    expect(t.models.map((m) => m.uri).sort()).toEqual(['file:///src/contracts.ts', 'file:///src/page.tsx'])
    expect(t.models.find((m) => m.uri.endsWith('page.tsx'))?.language).toBe('typescript')
    expect(t.editor().model).toBeNull() // nothing opened yet
    t.binding.open('src/page.tsx')
    expect(t.editor().model?.uri).toBe('file:///src/page.tsx')
  })

  it('moves typing into the store, and store changes into the model without echoing', () => {
    const t = setup()
    t.load('a.ts', 'one')
    t.binding.open('a.ts')
    const model = t.models[0]!
    model.type('two')
    expect(t.store.get('a.ts')?.buffer).toBe('two')
    expect(t.store.isDirty('a.ts')).toBe(true)

    const edits = vi.spyOn(t.store, 'edit')
    t.store.edit('a.ts', 'three') // e.g. "take theirs"
    edits.mockClear()
    expect(model.value).toBe('three')
    expect(edits).not.toHaveBeenCalled()
  })

  it('draws diagnostics of the saved file and withdraws them as soon as it is edited', () => {
    const t = setup()
    t.load('a.ts', 'const x: string = 1\n', 'h1')
    t.diagnostics.set([{ file: 'a.ts', line: 1, column: 7, severity: 'error', code: 'ts.2322', message: 'bad' }], { 'a.ts': 'h1' })
    t.binding.sync()
    expect(t.markers.get('file:///a.ts')).toHaveLength(1)
    t.models[0]!.type('const x: string = 1\n\n')
    expect(t.markers.get('file:///a.ts')).toEqual([])
  })

  it('drops the model of a file that disappeared', () => {
    const t = setup()
    t.load('a.ts', 'one')
    t.binding.open('a.ts')
    const model = t.models[0]!
    t.store.remove('a.ts')
    expect(model.disposed).toBe(true)
    expect(model.listenerCount).toBe(0)
    expect(t.editor().model).toBeNull()
  })

  it('reveals a diagnostic position in its file', () => {
    const t = setup()
    t.load('a.ts', 'line1\nline2')
    t.binding.reveal('a.ts', 2, 3)
    expect(t.editor().model?.uri).toBe('file:///a.ts')
    expect(t.editor().position).toEqual({ lineNumber: 2, column: 3 })
  })

  it('applies the host theme and read-only mode', () => {
    const t = setup()
    t.binding.setTheme('dark')
    expect(t.theme()).toBe('vs-dark')
    t.binding.setReadOnly(true)
    expect(t.editor().options.readOnly).toBe(true)
  })

  it('disposes the editor, every model and every listener, and ignores later calls', () => {
    const t = setup()
    t.load('a.ts', 'one')
    t.load('b.ts', 'two')
    t.binding.open('a.ts')
    const spy = vi.spyOn(t.monaco.editor, 'createModel')
    t.binding.dispose()
    expect(t.editor().disposed).toBe(true)
    for (const m of t.models) {
      expect(m.disposed).toBe(true)
      expect(m.listenerCount).toBe(0)
    }
    expect(t.markers.get('file:///a.ts')).toEqual([])
    t.binding.dispose() // idempotent
    t.load('c.ts', 'three') // store changes after dispose must not create models or throw
    t.binding.open('c.ts')
    t.binding.reveal('c.ts', 1, 1)
    expect(spy).not.toHaveBeenCalled()
  })
})
