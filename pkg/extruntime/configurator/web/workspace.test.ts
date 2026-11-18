import { describe, expect, it, vi } from 'vitest'
import { ApiError } from './api'
import { Workspace, validPath, type WorkspaceApi } from './workspace'
import type { RemoteFile } from './types'

const file = (path: string, hash: string): RemoteFile => ({ path, hash, size: 1, updatedBy: 1, updatedAt: '' })

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

/** An in-memory server with optimistic concurrency, like the management API. */
function fakeServer(initial: Record<string, string>) {
  const files = new Map(Object.entries(initial))
  let n = 0
  const hashes = new Map<string, string>([...files.keys()].map((p) => [p, `h${++n}`]))
  const calls = { check: 0, build: 0, test: 0, writes: [] as string[] }
  const api: WorkspaceApi = {
    listFiles: async () => [...files.keys()].map((p) => file(p, hashes.get(p)!)),
    readFile: async (_ns, path) => ({ content: files.get(path)!, hash: hashes.get(path)! }),
    writeFile: async (_ns, path, content, expect) => {
      if (expect === '' && files.has(path)) throw new ApiError(409, 'conflict', 'exists')
      if (expect && expect !== hashes.get(path)) throw new ApiError(409, 'conflict', 'changed by someone else')
      files.set(path, content)
      hashes.set(path, `h${++n}`)
      calls.writes.push(path)
      return file(path, hashes.get(path)!)
    },
    deleteFile: async (_ns, path) => { files.delete(path); hashes.delete(path) },
    moveFile: async (_ns, from, to) => { files.set(to, files.get(from)!); hashes.set(to, hashes.get(from)!); files.delete(from); hashes.delete(from) },
    check: async () => { calls.check++; return { diagnostics: [{ file: 'a.ts', line: 1, column: 1, severity: 'error', code: 'ts.1', message: 'bad' }], hasErrors: true, kitHash: 'k' } },
    build: async () => { calls.build++; return { diagnostics: [], hasErrors: false, kitHash: 'k', sourceHash: 's', artifactHash: 'a' } },
    test: async () => { calls.test++; return { passed: true, cases: [] } },
  }
  return { api, files, hashes, calls, bump: (path: string, content: string) => { files.set(path, content); hashes.set(path, `h${++n}`) } }
}

const events = () => ({ changed: vi.fn(), problem: vi.fn() })

describe('Workspace open and save', () => {
  it('lists and loads every file', async () => {
    const s = fakeServer({ 'a.ts': 'one', 'b.ts': 'two', 'c.ts': 'three', 'd.ts': 'four', 'e.ts': 'five' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    expect(w.store.entries().every((e) => e.loaded)).toBe(true)
    expect(w.store.get('e.ts')?.buffer).toBe('five')
  })

  it('saves with the base hash and rebases on the new hash', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    w.store.edit('a.ts', 'two')
    expect(await w.save('a.ts')).toBe('saved')
    expect(s.files.get('a.ts')).toBe('two')
    expect(w.store.isDirty('a.ts')).toBe(false)
    expect(w.store.get('a.ts')?.baseHash).toBe(s.hashes.get('a.ts'))
  })

  it('turns a refused save into a conflict that keeps the buffer and fetches the server version', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    w.store.edit('a.ts', 'mine')
    s.bump('a.ts', 'theirs') // someone else saved meanwhile
    expect(await w.save('a.ts')).toBe('conflict')
    expect(w.store.get('a.ts')).toMatchObject({ status: 'conflict', buffer: 'mine', remote: { content: 'theirs' } })
    expect(s.files.get('a.ts')).toBe('theirs')
    w.store.resolveConflict('a.ts', 'mine')
    expect(await w.save('a.ts')).toBe('saved')
    expect(s.files.get('a.ts')).toBe('mine')
  })

  it('creates a new file with create-only semantics and refuses an existing path', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    expect(await w.createFile('src/new.ts')).toBe(true)
    expect(s.files.has('src/new.ts')).toBe(true)
    expect(await w.createFile('a.ts')).toBe(false)
    expect(await w.createFile('../escape.ts')).toBe(false)
    s.bump('taken.ts', 'x') // appears on the server after the listing
    expect(await w.createFile('taken.ts')).toBe(false)
    expect(w.store.get('taken.ts')?.status).toBe('conflict')
  })

  it('deletes and renames on the server and in the store', async () => {
    const s = fakeServer({ 'a.ts': 'one', 'b.ts': 'two' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    expect(await w.deleteFile('a.ts')).toBe(true)
    expect(s.files.has('a.ts')).toBe(false)
    w.store.edit('b.ts', 'two!')
    expect(await w.rename('b.ts', 'src/b.ts')).toBe(true)
    expect(s.files.get('src/b.ts')).toBe('two!') // saved before moving
    expect(w.store.get('src/b.ts')).toBeDefined()
    w.store.create('draft.ts')
    expect(await w.deleteFile('draft.ts')).toBe(true) // never saved: discarded locally
  })
})

describe('Workspace check, build and test', () => {
  it('saves unsaved edits first, then checks and records diagnostics against the saved hashes', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    w.store.edit('a.ts', 'two')
    const result = await w.run('check')
    expect(result?.kind).toBe('check')
    expect(s.calls.writes).toEqual(['a.ts'])
    expect(s.calls.check).toBe(1)
    const problems = w.diagnostics.problems(w.fileStates())
    expect(problems).toHaveLength(1)
    expect(problems[0]).toMatchObject({ path: 'a.ts', stale: false })
    // editing afterwards makes them stale
    w.store.edit('a.ts', 'three')
    expect(w.diagnostics.problems(w.fileStates())[0]?.stale).toBe(true)
  })

  it('does not run while a file is in conflict', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const ev = events()
    const w = new Workspace(s.api, 'acme.x', ev)
    await w.open()
    w.store.edit('a.ts', 'mine')
    s.bump('a.ts', 'theirs')
    expect(await w.run('build')).toBeUndefined()
    expect(s.calls.build).toBe(0)
    expect(ev.problem).toHaveBeenCalledWith('unsaved')
  })

  it('records a test report without touching the diagnostics', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    await w.run('check')
    const before = w.diagnostics.problems(w.fileStates()).length
    const r = await w.run('test')
    expect(r).toMatchObject({ kind: 'test', report: { passed: true } })
    expect(w.diagnostics.problems(w.fileStates())).toHaveLength(before)
    expect(w.lastRun?.kind).toBe('test')
  })

  it('draws the diagnostics of a refused publish', async () => {
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace(s.api, 'acme.x', events())
    await w.open()
    w.recordDiagnosticsFrom(new ApiError(409, 'failedprecondition', 'build failed', [
      { file: 'a.ts', line: 3, column: 2, severity: 'error', code: 'ts.2345', message: 'nope' },
    ]))
    expect(w.diagnostics.problems(w.fileStates())[0]).toMatchObject({ path: 'a.ts', stale: false })
  })
})

describe('Workspace late responses', () => {
  it('applies nothing that returns after dispose', async () => {
    const read = deferred<{ content: string; hash: string }>()
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace({ ...s.api, readFile: () => read.promise }, 'acme.x', events())
    const opening = w.open()
    await new Promise((r) => setTimeout(r, 0))
    w.dispose()
    read.resolve({ content: 'late', hash: 'x' })
    await opening
    expect(w.store.entries()).toEqual([])
  })

  it('ignores a save response that returns after a package switch (dispose)', async () => {
    const write = deferred<RemoteFile>()
    const s = fakeServer({ 'a.ts': 'one' })
    const ev = events()
    const w = new Workspace({ ...s.api, writeFile: () => write.promise }, 'acme.x', ev)
    await w.open()
    w.store.edit('a.ts', 'two')
    const saving = w.save('a.ts')
    w.dispose()
    write.resolve(file('a.ts', 'h9'))
    expect(await saving).toBe('skipped')
    expect(w.store.get('a.ts')).toBeUndefined()
  })

  it('does not report a failure of a request that was cancelled by dispose', async () => {
    const write = deferred<RemoteFile>()
    const s = fakeServer({ 'a.ts': 'one' })
    const ev = events()
    const w = new Workspace({ ...s.api, writeFile: () => write.promise }, 'acme.x', ev)
    await w.open()
    w.store.edit('a.ts', 'two')
    const saving = w.save('a.ts')
    w.dispose()
    write.reject(new DOMException('aborted', 'AbortError'))
    expect(await saving).toBe('skipped')
    expect(ev.problem).not.toHaveBeenCalled()
  })

  it('does not draw check results that return after dispose', async () => {
    const check = deferred<{ diagnostics: never[]; hasErrors: boolean; kitHash: string }>()
    const s = fakeServer({ 'a.ts': 'one' })
    const w = new Workspace({ ...s.api, check: () => check.promise }, 'acme.x', events())
    await w.open()
    const running = w.run('check')
    await new Promise((r) => setTimeout(r, 0))
    w.dispose()
    check.resolve({ diagnostics: [], hasErrors: false, kitHash: 'k' })
    expect(await running).toBeUndefined()
    expect(w.lastRun).toBeUndefined()
  })
})

describe('validPath', () => {
  it('accepts relative draft paths only', () => {
    expect(validPath('src/a.ts')).toBe(true)
    for (const bad of ['', '/a.ts', 'a/', 'a//b', './a.ts', '../a.ts', 'a/../b']) expect(validPath(bad)).toBe(false)
  })
})
