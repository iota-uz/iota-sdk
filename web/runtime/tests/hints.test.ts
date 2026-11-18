// Type errors an author is most likely to hit must say what to do instead. Each case compiles a small
// snippet against the real public types and checks the error text.
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

import ts from 'typescript'
import { describe, expect, it } from 'vitest'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')

function diagnose(body: string): string {
  const file = join(root, 'typetests', '__hints__.ts')
  const text = `
import { t, ref } from '../src/index.ts'
import type { Tx, QueryContext } from '../src/server.ts'
import * as c from './acme/contracts.ts'
import * as kit from './kit.ts'
declare const tx: Tx
declare const q: QueryContext
declare const clientRef: import('../src/index.ts').Ref<typeof kit.Client>
void t; void ref; void c; void kit; void tx; void q; void clientRef
${body}
`
  const options: ts.CompilerOptions = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    strict: true,
    noEmit: true,
    allowImportingTsExtensions: true,
    skipLibCheck: true,
    lib: ['lib.es2022.d.ts', 'lib.dom.d.ts'],
    types: [],
  }
  const host = ts.createCompilerHost(options)
  const read = host.readFile.bind(host)
  const exists = host.fileExists.bind(host)
  host.readFile = (f) => (f === file ? text : read(f))
  host.fileExists = (f) => f === file || exists(f)
  host.getSourceFile = ((f: string, lang: ts.ScriptTarget) => {
    const source = f === file ? text : read(f)
    return source === undefined ? undefined : ts.createSourceFile(f, source, lang, true)
  }) as ts.CompilerHost['getSourceFile']
  const program = ts.createProgram([file], options, host)
  return ts
    .getPreEmitDiagnostics(program, program.getSourceFile(file))
    .map((d) => ts.flattenDiagnosticMessageText(d.messageText, '\n'))
    .join('\n')
}

describe('readable type errors', () => {
  it('names the operation to use for a write to a native entity', () => {
    const msg = diagnose(`void tx.insert(kit.Client, { name: 'x' })`)
    expect(msg).toContain("'crm.client' belongs to the platform or another package")
    expect(msg).toContain('published operation (tx.call)')
    expect(diagnose(`void tx.update(kit.Client, clientRef, { name: 'x' })`)).toContain('not tx.insert/update/delete')
    expect(diagnose(`void tx.delete(kit.Client, clientRef)`)).toContain('published operation')
  })

  it('still accepts the package own entities', () => {
    expect(diagnose(`void tx.insert(c.Customer, { name: 'x' })`)).toBe('')
  })

  it('explains why an enum, bool or ref field cannot be ordered', () => {
    const msg = diagnose(`void q.find(c.Policy, { orderBy: [{ field: 'status' }] })`)
    expect(msg).toContain("cannot order by 'status': enum fields have no database order")
    expect(diagnose(`void q.find(c.Policy, { orderBy: [{ field: 'holder' }] })`)).toContain("cannot order by 'holder'")
    expect(diagnose(`void q.find(c.Policy, { orderBy: [{ field: 'expiresOn', desc: true }, { field: 'createdAt' }, { field: 'holder.name' }] })`)).toBe('')
  })

  it('explains range, like and json filters on the wrong kind', () => {
    expect(diagnose(`void q.find(c.Policy, { where: { status: { gt: 'draft' } } })`)).toContain('enum fields cannot be compared with lt/lte/gt/gte')
    expect(diagnose(`void q.find(c.Policy, { where: { expiresOn: { like: 'x' } } })`)).toContain('like needs a string field, not date')
    expect(diagnose(`void q.find(c.Policy, { where: { extra: { tags: [] } } })`)).toContain("json field 'extra' cannot be filtered")
    expect(diagnose(`void q.find(c.Policy, { where: { number: { like: 'A%' }, expiresOn: { gte: '2026-01-01' } as never } })`)).toBe('')
  })

  it('keeps the tx.call hints', () => {
    expect(diagnose(`void tx.call(c.RenewPolicy, { policy: ref(c.Policy, '00000000-0000-4000-8000-000000000000') })`)).toContain('ordinary command: use ctx.call')
  })
})
