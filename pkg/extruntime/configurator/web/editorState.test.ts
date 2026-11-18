import { describe, expect, it, vi } from 'vitest'
import { EditorStore } from './editorState'
import type { RemoteFile } from './types'

const remote = (path: string, hash: string): RemoteFile => ({ path, hash, size: 1, updatedBy: 1, updatedAt: '2026-01-01T00:00:00Z' })

function loaded(store: EditorStore, path: string, content: string, hash: string): void {
  const t = store.beginLoad(path)
  if (!t) throw new Error('no entry ' + path)
  expect(store.finishLoad(t, { content, hash })).toBe(true)
}

describe('EditorStore dirty tracking', () => {
  it('is clean after load, dirty after an edit, clean again when the edit is undone', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    expect(s.isDirty('a.ts')).toBe(false)
    loaded(s, 'a.ts', 'one', 'h1')
    expect(s.isDirty('a.ts')).toBe(false)
    s.edit('a.ts', 'two')
    expect(s.dirtyPaths()).toEqual(['a.ts'])
    s.edit('a.ts', 'one')
    expect(s.isDirty('a.ts')).toBe(false)
  })

  it('treats a new file as dirty until its first save, even when empty', () => {
    const s = new EditorStore()
    expect(s.create('src/new.ts')).toBe(true)
    expect(s.create('src/new.ts')).toBe(false)
    expect(s.isDirty('src/new.ts')).toBe(true)
    const req = s.beginSave('src/new.ts')
    expect(req?.expectHash).toBe('')
    expect(s.finishSave(req!, { hash: 'n1' })).toBe(true)
    expect(s.isDirty('src/new.ts')).toBe(false)
    expect(s.get('src/new.ts')?.baseHash).toBe('n1')
  })

  it('notifies subscribers and exposes a revision', () => {
    const s = new EditorStore()
    const fn = vi.fn()
    const off = s.subscribe(fn)
    s.applyListing([remote('a.ts', 'h1')])
    expect(fn).toHaveBeenCalled()
    const rev = s.revision
    off()
    s.edit('a.ts', 'x')
    expect(fn).toHaveBeenCalledTimes(1)
    expect(s.revision).toBeGreaterThan(rev) // state still changes after unsubscribing
  })
})

describe('EditorStore saving', () => {
  it('saves against the base hash and keeps edits made while the save ran', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'one', 'h1')
    s.edit('a.ts', 'two')
    const req = s.beginSave('a.ts')!
    expect(req).toMatchObject({ path: 'a.ts', content: 'two', expectHash: 'h1' })
    expect(s.get('a.ts')?.status).toBe('saving')
    expect(s.beginSave('a.ts')).toBeUndefined() // no second save in flight
    s.edit('a.ts', 'three')
    expect(s.finishSave(req, { hash: 'h2' })).toBe(true)
    expect(s.get('a.ts')).toMatchObject({ baseHash: 'h2', baseContent: 'two', buffer: 'three', status: 'idle' })
    expect(s.isDirty('a.ts')).toBe(true)
    const next = s.beginSave('a.ts')!
    expect(next.expectHash).toBe('h2')
  })

  it('has nothing to save for a clean file', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'one', 'h1')
    expect(s.beginSave('a.ts')).toBeUndefined()
  })

  it('keeps the buffer when a save fails and allows a retry', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'one', 'h1')
    s.edit('a.ts', 'two')
    const req = s.beginSave('a.ts')!
    expect(s.failSave(req, 'permission denied')).toBe(true)
    expect(s.get('a.ts')).toMatchObject({ status: 'error', error: 'permission denied', buffer: 'two' })
    expect(s.beginSave('a.ts')).toBeDefined()
  })
})

describe('EditorStore save conflicts', () => {
  function conflicted() {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'one', 'h1')
    s.edit('a.ts', 'mine')
    const req = s.beginSave('a.ts')!
    expect(s.saveConflict(req)).toBe(true)
    return s
  }

  it('blocks further saves until the conflict is settled and never loses the buffer', () => {
    const s = conflicted()
    expect(s.get('a.ts')).toMatchObject({ status: 'conflict', buffer: 'mine' })
    expect(s.beginSave('a.ts')).toBeUndefined()
    expect(s.resolveConflict('a.ts', 'mine')).toBe(false) // server version not fetched yet
  })

  it('keep-mine rebases on the server version so the next save overwrites it knowingly', () => {
    const s = conflicted()
    const t = s.beginLoad('a.ts')!
    expect(s.finishLoad(t, { content: 'theirs', hash: 'h9' })).toBe(true)
    expect(s.get('a.ts')?.remote).toEqual({ hash: 'h9', content: 'theirs' })
    expect(s.get('a.ts')?.buffer).toBe('mine')
    expect(s.resolveConflict('a.ts', 'mine')).toBe(true)
    expect(s.get('a.ts')).toMatchObject({ status: 'idle', baseHash: 'h9', buffer: 'mine' })
    expect(s.beginSave('a.ts')).toMatchObject({ expectHash: 'h9', content: 'mine' })
  })

  it('take-theirs replaces the buffer with the server version', () => {
    const s = conflicted()
    s.finishLoad(s.beginLoad('a.ts')!, { content: 'theirs', hash: 'h9' })
    expect(s.resolveConflict('a.ts', 'theirs')).toBe(true)
    expect(s.get('a.ts')).toMatchObject({ buffer: 'theirs', baseContent: 'theirs', baseHash: 'h9', status: 'idle' })
    expect(s.isDirty('a.ts')).toBe(false)
  })
})

describe('EditorStore late responses', () => {
  it('ignores a load that was superseded by a newer one', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    const first = s.beginLoad('a.ts')!
    const second = s.beginLoad('a.ts')!
    expect(s.finishLoad(second, { content: 'new', hash: 'h2' })).toBe(true)
    expect(s.finishLoad(first, { content: 'old', hash: 'h1' })).toBe(false)
    expect(s.get('a.ts')).toMatchObject({ buffer: 'new', baseHash: 'h2' })
  })

  it('does not clobber edits with a load that returns after the user typed', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'one', 'h1')
    const reload = s.beginLoad('a.ts')!
    s.edit('a.ts', 'typing')
    expect(s.finishLoad(reload, { content: 'server', hash: 'h2' })).toBe(false)
    expect(s.get('a.ts')).toMatchObject({ buffer: 'typing', baseHash: 'h1' })
  })

  it('ignores every response that arrives after a package switch', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    const load = s.beginLoad('a.ts')!
    loaded(s, 'a.ts', 'x', 'h1')
    s.edit('a.ts', 'y')
    const save = s.beginSave('a.ts')!
    s.reset()
    s.applyListing([remote('a.ts', 'other')]) // same path in the next package
    expect(s.finishLoad(load, { content: 'stale', hash: 'h1' })).toBe(false)
    expect(s.finishSave(save, { hash: 'h2' })).toBe(false)
    expect(s.saveConflict(save)).toBe(false)
    expect(s.failSave(save, 'x')).toBe(false)
    expect(s.get('a.ts')).toMatchObject({ baseHash: 'other', loaded: false, buffer: '' })
  })

  it('ignores a save response after the file was removed or a newer save began', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'x', 'h1')
    s.edit('a.ts', 'y')
    const save = s.beginSave('a.ts')!
    s.remove('a.ts')
    expect(s.finishSave(save, { hash: 'h2' })).toBe(false)
    expect(s.get('a.ts')).toBeUndefined()
  })

  it('ignores a failed load that was superseded', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    const first = s.beginLoad('a.ts')!
    const second = s.beginLoad('a.ts')!
    expect(s.failLoad(first, 'boom')).toBe(false)
    expect(s.failLoad(second, 'boom')).toBe(true)
    expect(s.get('a.ts')).toMatchObject({ status: 'error', error: 'boom' })
  })
})

describe('EditorStore listing', () => {
  it('drops files deleted on the server but keeps unsaved work', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1'), remote('b.ts', 'h1'), remote('c.ts', 'h1')])
    loaded(s, 'b.ts', 'b', 'h1')
    s.edit('b.ts', 'b2')
    s.create('new.ts')
    s.applyListing([remote('c.ts', 'h1')])
    expect(s.paths()).toEqual(['b.ts', 'c.ts', 'new.ts'])
  })

  it('marks a clean file whose hash moved as unloaded, and keeps a dirty one', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1'), remote('b.ts', 'h1')])
    loaded(s, 'a.ts', 'a', 'h1')
    loaded(s, 'b.ts', 'b', 'h1')
    s.edit('b.ts', 'b2')
    s.applyListing([remote('a.ts', 'h2'), remote('b.ts', 'h2')])
    expect(s.get('a.ts')).toMatchObject({ loaded: false, baseHash: 'h2' })
    expect(s.get('b.ts')).toMatchObject({ loaded: true, baseHash: 'h1', buffer: 'b2' })
  })

  it('moves an entry on rename without losing its state', () => {
    const s = new EditorStore()
    s.applyListing([remote('a.ts', 'h1')])
    loaded(s, 'a.ts', 'a', 'h1')
    s.edit('a.ts', 'a2')
    s.renamed('a.ts', 'src/a.ts')
    expect(s.get('a.ts')).toBeUndefined()
    expect(s.get('src/a.ts')).toMatchObject({ path: 'src/a.ts', buffer: 'a2' })
  })
})
