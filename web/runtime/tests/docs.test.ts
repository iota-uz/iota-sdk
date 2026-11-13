import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterEach, describe, expect, it } from 'vitest'

import { collectDocs, scanLinks } from '../src/build/docs.ts'
import { sharedShim } from '../src/build/shared.ts'
import type { Layout } from '../src/build/layout.ts'
import { helpDocHref, helpHref } from '../src/help.ts'
import type { ManifestJSON } from '../src/model.ts'

const dirs: string[] = []
afterEach(() => {
  for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true })
})

const manifest: ManifestJSON = {
  schema: 'iota.manifest/1',
  namespace: 'acme.loyalty',
  resources: [
    { id: 'acme.loyalty.manage', kind: 'permission' },
    { id: 'acme.loyalty.cards', kind: 'route', path: '/loyalty/cards' },
    { id: 'acme.loyalty.cardRoute', kind: 'route', path: '/loyalty/cards/:id' },
  ],
}

function pkg(files: Record<string, string>): string {
  const dir = mkdtempSync(join(tmpdir(), 'docs-test-'))
  dirs.push(dir)
  for (const [path, text] of Object.entries(files)) {
    mkdirSync(dirname(join(dir, path)), { recursive: true })
    writeFileSync(join(dir, path), text)
  }
  return dir
}

const run = (files: Record<string, string>) => collectDocs({ dir: pkg(files), namespace: 'acme.loyalty', manifest })
const codes = (r: ReturnType<typeof run>) => r.diagnostics.map((d) => `${d.file}:${d.line ?? '-'} ${d.code}`)

describe('collectDocs', () => {
  it('collects documents, front matter and a plain body', () => {
    const r = run({
      'docs/en/getting-started.md': '---\ntitle: "Getting started"\norder: 2\naudience: Cashiers\n---\n\n# Welcome\n\nSee [setup](admin/setup.md#cards).\n',
      'docs/en/admin/setup.md': '---\ntitle: Setup # shown in the sidebar\npermission: acme.loyalty.manage\n---\nSetup body.\n',
      'docs/ru/getting-started.md': '# Начало работы\n\nТекст.\n',
    })
    expect(r.diagnostics).toEqual([])
    expect(r.docs).toEqual([
      { path: 'admin/setup', locale: 'en', title: 'Setup', permission: 'acme.loyalty.manage' },
      { path: 'getting-started', locale: 'en', title: 'Getting started', order: 2, audience: 'Cashiers' },
      { path: 'getting-started', locale: 'ru', title: 'Начало работы' },
    ])
    expect([...r.files.keys()].sort()).toEqual(['docs/en/admin/setup.md', 'docs/en/getting-started.md', 'docs/ru/getting-started.md'])
    expect(Buffer.from(r.files.get('docs/en/admin/setup.md')!).toString()).toBe('Setup body.\n')
    expect(Buffer.from(r.files.get('docs/en/getting-started.md')!).toString().startsWith('# Welcome')).toBe(true)
  })

  it('derives the title from the first heading, then from the file name', () => {
    const r = run({ 'docs/en/a.md': 'text\n\n# The A page ##\n', 'docs/en/some-thing_else.md': 'no heading\n' })
    expect(r.docs.map((d) => d.title)).toEqual(['The A page', 'Some thing else'])
  })

  it('has no docs and no files without a docs directory', () => {
    const r = run({ 'package.json': '{}' })
    expect(r).toEqual({ docs: [], files: new Map(), diagnostics: [] })
  })

  it('locates front matter problems', () => {
    const r = run({
      'docs/en/a.md': '---\ntitle: A\ncolour: red\norder: soon\ntitle: again\n  nested: x\npermission: "unterminated\n---\nBody\n',
      'docs/en/b.md': '---\ntitle: B\nno closing\n',
    })
    expect(codes(r)).toEqual([
      'docs/en/a.md:3 docs.frontMatter', // unknown key
      'docs/en/a.md:4 docs.frontMatter', // order
      'docs/en/a.md:5 docs.frontMatter', // duplicate title
      'docs/en/a.md:6 docs.frontMatter', // indented
      'docs/en/a.md:7 docs.frontMatter', // bad string
      'docs/en/b.md:1 docs.frontMatter', // not closed
    ])
  })

  it('refuses files outside the rules', () => {
    const r = run({
      'docs/README.md': '# x\n',
      'docs/en/logo.png': 'png',
      'docs/EN/a.md': '# a\n',
      'docs/en/Bad Name.md': '# a\n',
      'docs/en/ok.md': '# ok\n',
      'docs/en/deep/a/b/c/d/e.md': '# deep\n',
    })
    expect(codes(r)).toEqual(
      expect.arrayContaining([
        'docs/README.md:- docs.noLocale',
        'docs/en/logo.png:- docs.unsupportedFile',
        'docs/EN/a.md:- docs.badLocale',
        'docs/en/Bad Name.md:- docs.badPath',
        'docs/en/deep/a/b/c/d/e.md:- docs.badPath',
      ]),
    )
    expect(r.docs.map((d) => d.path)).toEqual(['ok'])
  })

  it('refuses a symbolic link and invalid UTF-8', () => {
    const dir = pkg({ 'docs/en/ok.md': '# ok\n', 'outside.md': '# out\n' })
    symlinkSync(join(dir, 'outside.md'), join(dir, 'docs/en/link.md'))
    writeFileSync(join(dir, 'docs/en/bad.md'), Buffer.from([0xff, 0xfe, 0x41]))
    const r = collectDocs({ dir, namespace: 'acme.loyalty', manifest })
    expect(codes(r)).toEqual(expect.arrayContaining(['docs/en/link.md:- docs.symlink', 'docs/en/bad.md:- docs.encoding']))
  })

  it('requires every document in the default locale and one permission per document', () => {
    const r = run({
      'docs/en/a.md': '---\npermission: acme.loyalty.manage\n---\nA\n',
      'docs/ru/a.md': 'A in Russian\n',
      'docs/ru/only-ru.md': 'Только по-русски\n',
    })
    expect(codes(r)).toEqual(['docs/ru/a.md:1 docs.permissionMismatch', 'docs/ru/only-ru.md:1 docs.missingDefault'])
  })

  it('validates permissions', () => {
    const r = run({
      'docs/en/a.md': '---\npermission: acme.loyalty.nope\n---\nA\n',
      'docs/en/b.md': '---\npermission: not a permission\n---\nB\n',
      'docs/en/c.md': '---\npermission: Client.Read\n---\nC\n',
      'docs/en/d.md': '---\npermission: core.users.read\n---\nD (foreign ID: the catalog decides at publish)\n',
    })
    expect(codes(r)).toEqual(['docs/en/a.md:2 docs.unknownPermission', 'docs/en/b.md:2 docs.badPermission'])
    expect(r.diagnostics[0]!.message).toContain('acme.loyalty.manage')
  })

  it('keeps titles plain text', () => {
    const r = run({
      'docs/en/a.md': '---\ntitle: "<img src=x onerror=alert(1)>"\n---\nA\n',
      'docs/en/b.md': '---\naudience: "<b>everyone</b>"\n---\nB\n',
      'docs/en/c.md': '# <script>alert(1)</script>\n',
    })
    expect(codes(r)).toEqual(['docs/en/a.md:2 docs.title', 'docs/en/b.md:2 docs.audience', 'docs/en/c.md:1 docs.title'])
  })

  it('checks links: broken, escaping, anchors, routes, schemes, images', () => {
    const r = run({
      'docs/en/index.md': '[ok](a.md) [ok2](./sub/b.md#x) [anchor](#top) [web](https://example.com) [mail](mailto:a@b.c) [abs](/crm/clients)\n',
      'docs/en/a.md': '[up](../index.md) [broken](nope.md) [escape](../../x.md) [ext](a.html) [q](a.md?x=1)\n',
      'docs/en/sub/b.md': '[back](../a.md) [route](route:acme.loyalty.cards) [badroute](route:acme.loyalty.missing) [param](route:acme.loyalty.cardRoute)\n',
      'docs/en/c.md': '[js](javascript:alert(1)) [data](data:text/html;base64,AAAA) ![logo](logo.png) ![ok](https://example.com/x.png) [pr](//evil.example/x)\n',
    })
    expect(codes(r)).toEqual([
      'docs/en/a.md:1 docs.linkEscapes', // ../index.md leaves the locale tree
      'docs/en/a.md:1 docs.brokenLink',
      'docs/en/a.md:1 docs.linkEscapes',
      'docs/en/a.md:1 docs.linkTarget',
      'docs/en/a.md:1 docs.linkTarget',
      'docs/en/c.md:1 docs.linkScheme',
      'docs/en/c.md:1 docs.linkScheme',
      'docs/en/c.md:1 docs.image',
      'docs/en/c.md:1 docs.linkScheme',
      'docs/en/sub/b.md:1 docs.unknownRoute',
      'docs/en/sub/b.md:1 docs.unknownRoute',
    ])
    const broken = r.diagnostics.find((d) => d.code === 'docs.brokenLink')!
    expect(broken.column).toBeGreaterThan(1)
    expect(broken.message).toContain('nope')
  })

  it('numbers lines of links after the front matter and ignores code', () => {
    const r = run({
      'docs/en/a.md': '---\ntitle: A\n---\n\nIntro\n\n```md\n[in fence](missing.md)\n```\n\nUse `[in span](missing.md)` and [real](missing.md).\n',
    })
    expect(codes(r)).toEqual(['docs/en/a.md:11 docs.brokenLink'])
  })

  it('checks literal helpHref calls against the package documents', () => {
    const r = run({
      'docs/en/getting-started.md': '# Start\n',
      'src/client.tsx': 'const a = helpHref("getting-started")\nconst b = helpHref("getting-started#cards")\nconst c = helpHref("nope")\nconst d = helpHref(dynamic)\nconst e = helpHref("other", { namespace: "x.y" })\n',
    })
    expect(codes(r)).toEqual(['src/client.tsx:3 docs.unknownHelpHref'])
    expect(r.diagnostics[0]!.message).toContain('getting-started')
    const none = run({ 'src/client.tsx': 'helpHref("start")\n' })
    expect(none.diagnostics[0]!.message).toContain('no documentation')
  })
})

describe('scanLinks', () => {
  it('finds inline links, images and definitions outside code', () => {
    const links = scanLinks('[a](x.md) ![i](y.png) <z.md>\n[ref]: w.md\n    \n~~~\n[no](v.md)\n~~~\n`[no](u.md)` [yes](<t u.md>)', 5)
    expect(links.map((l) => [l.dest, l.image, l.line])).toEqual([
      ['x.md', false, 5],
      ['y.png', true, 5],
      ['w.md', false, 6],
      ['t u.md', false, 11],
    ])
  })
})

describe('helpHref', () => {
  it('builds the Help Center address of a document', () => {
    expect(helpDocHref('acme.loyalty', 'getting-started')).toBe('/help/doc/apps/acme.loyalty/getting-started')
    expect(helpDocHref('acme.loyalty', 'admin/setup#card limits', '/help/')).toBe('/help/doc/apps/acme.loyalty/admin/setup#card%20limits')
    expect(helpHref('a', { namespace: 'acme.loyalty', base: '/docs' })).toBe('/docs/doc/apps/acme.loyalty/a')
  })

  it('refuses a malformed path or namespace, and an unknown package', () => {
    expect(() => helpDocHref('acme.loyalty', 'a.md')).toThrow(/not a document path/)
    expect(() => helpDocHref('acme.loyalty', '../x')).toThrow(/not a document path/)
    expect(() => helpDocHref('Acme', 'a')).toThrow(/namespace/)
    expect(() => helpHref('a')).toThrow(/package is unknown/)
  })
})

describe('helpHref in a package bundle is bound to the package', () => {
  const REGISTRY = Symbol.for('iota.runtime.shared')
  const CLIENT = '@iota-uz/sdk/runtime/client'

  /** Evaluates the shim the client graph generates for `@iota-uz/sdk/runtime/client` against the host's real helpHref. */
  async function load(namespace: string | undefined): Promise<{ helpHref: typeof helpHref; page: unknown }> {
    const dir = mkdtempSync(join(tmpdir(), 'shim-test-'))
    dirs.push(dir)
    const sharedExports = join(dir, 'shared-exports.json')
    writeFileSync(sharedExports, JSON.stringify({ [CLIENT]: ['page', 'helpHref', 'helpDocHref'] }))
    const code = sharedShim({ sharedExports } as Layout, CLIENT, namespace)
    ;(globalThis as Record<symbol, unknown>)[REGISTRY] = { [CLIENT]: { page: 'host-page', helpDocHref, helpHref } }
    return (await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}#${Math.random()}`)) as { helpHref: typeof helpHref; page: unknown }
  }
  afterEach(() => {
    delete (globalThis as Record<symbol, unknown>)[REGISTRY]
    delete (globalThis as { document?: unknown }).document
  })

  it("links to the bundle's own package whatever page is shown, and an explicit namespace still wins", async () => {
    // Package A's runtime page is on screen (its client context names A); the bundle is package B's.
    ;(globalThis as { document?: unknown }).document = { getElementById: () => ({ textContent: JSON.stringify({ initial: { page: { package: 'acme.pageowner' } } }) }) }
    const b = await load('acme.slotowner')
    expect(b.helpHref('getting-started')).toBe('/help/doc/apps/acme.slotowner/getting-started')
    expect(b.helpHref('getting-started', { namespace: 'acme.other' })).toBe('/help/doc/apps/acme.other/getting-started')
    expect(b.helpHref('limits#cards', { base: '/docs' })).toBe('/docs/doc/apps/acme.slotowner/limits#cards')
    expect(b.page).toBe('host-page') // everything else is the host's instance
    // The host's own function keeps following the page.
    expect(helpHref('getting-started')).toBe('/help/doc/apps/acme.pageowner/getting-started')
  })

  it('works where there is no page context at all (a native page)', async () => {
    const b = await load('acme.slotowner')
    expect(b.helpHref('getting-started')).toBe('/help/doc/apps/acme.slotowner/getting-started')
    expect(() => helpHref('getting-started')).toThrow(/package is unknown/)
  })

  it('leaves the host export alone when the build knows no namespace', async () => {
    const unbound = await load(undefined)
    expect(unbound.helpHref).toBe(helpHref)
  })
})

describe('the example package of the author guide', () => {
  it('ships documentation that passes every rule, with its page link and helpHref call', () => {
    const dir = fileURLToPath(new URL('../../../docs/extensions/example', import.meta.url))
    const r = collectDocs({
      dir,
      namespace: 'acme.loyalty',
      manifest: {
        ...manifest,
        resources: [
          { id: 'acme.loyalty.manage', kind: 'permission' },
          { id: 'acme.loyalty.rewards', kind: 'page', path: '/loyalty/rewards' },
        ],
      },
    })
    expect(r.diagnostics).toEqual([])
    expect(r.docs.map((d) => `${d.locale}/${d.path}${d.permission ? ` [${d.permission}]` : ''}`)).toEqual([
      'en/admin/limits [acme.loyalty.manage]',
      'en/getting-started',
      'ru/getting-started',
      'en/points',
    ])
  })
})
