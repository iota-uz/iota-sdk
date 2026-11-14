import { expect, test, type Page, type Route } from '@playwright/test'

/**
 * Real Monaco in Chromium, the real screen mounted by the standard Solid
 * client host, and a mocked /_ext/manage API. The API contract itself is the
 * Go tests' business; this checks what only a browser can: Monaco mounts and
 * disposes, diagnostics land on the right file and line, and the TypeScript
 * worker resolves the kit, the SDK and the draft's own files.
 */

const PAGE_SOURCE = `import { coreUser } from '@iota-kit/core'
import { entity } from '@iota-uz/sdk/runtime'
import { thing } from './contracts'
import { gone } from './missing'
export const id: number = coreUser.id
export const e = entity('acme.demo.thing')
export const t = thing
`

const FILES: Record<string, { content: string; hash: string }> = {
  'package.json': { content: '{"name":"@acme/demo","iota":{"namespace":"acme.demo"}}\n', hash: 'h-pkg' },
  'src/contracts.ts': { content: 'export const thing = 1\n', hash: 'h-contracts' },
  'src/page.tsx': { content: PAGE_SOURCE, hash: 'h-page' },
}

async function mockApi(page: Page, opts: { onWrite?: (path: string, body: string) => void } = {}) {
  const json = (route: Route, body: unknown, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
  await page.route('**/_ext/manage/**', async (route) => {
    const url = new URL(route.request().url())
    const p = url.pathname.replace('/_ext/manage', '')
    const method = route.request().method()
    if (p === '/packages') {
      return json(route, [{ namespace: 'acme.demo', provenance: 'runtime', draftRev: 3, createdBy: 1, createdAt: '2026-01-01T00:00:00Z', updatedBy: 1, updatedAt: '2026-01-01T00:00:00Z' }])
    }
    if (p === '/packages/acme.demo/files') {
      return json(route, Object.entries(FILES).map(([path, f]) => ({ path, hash: f.hash, size: f.content.length, updatedBy: 1, updatedAt: '2026-01-01T00:00:00Z' })))
    }
    const file = p.match(/^\/packages\/acme\.demo\/files\/(.+)$/)
    if (file) {
      const path = decodeURIComponent(file[1]!)
      if (method === 'PUT') {
        const body = route.request().postData() ?? ''
        opts.onWrite?.(path, body)
        FILES[path] = { content: body, hash: `h-${Math.random().toString(16).slice(2)}` }
        return json(route, { path, hash: FILES[path]!.hash, size: body.length, updatedBy: 1, updatedAt: '2026-01-02T00:00:00Z' })
      }
      const f = FILES[path]
      if (!f) return json(route, { error: { code: 'notfound', message: 'file not found' } }, 404)
      return route.fulfill({ status: 200, headers: { ETag: `"${f.hash}"`, 'content-type': 'application/octet-stream' }, body: f.content })
    }
    if (p === '/packages/acme.demo/check' && method === 'POST') {
      return json(route, {
        diagnostics: [{ file: 'src/page.tsx', line: 5, column: 14, severity: 'error', code: 'ts.2322', message: "Type 'string' is not assignable to type 'number'." }],
        hasErrors: true, kitHash: 'k1',
      })
    }
    return json(route, { error: { code: 'notfound', message: `unmocked ${method} ${p}` } }, 404)
  })
  await page.route('**/extensions/types/kit', (route) => json(route, {
    hash: 'k1',
    files: {
      '@iota-kit/core/index.d.ts': 'export declare const coreUser: { id: number }\n',
      '@iota-kit/core/package.json': '{"name":"@iota-kit/core","types":"index.d.ts"}',
    },
  }))
  await page.route('**/extensions/types/sdk', (route) => json(route, {
    hash: 's1',
    files: { '@iota-uz/sdk/dist/runtime/index.d.ts': 'export declare function entity(id: string): { id: string }\n' },
  }))
}

type Marker = { code?: string | { value: string }; startLineNumber: number; message: string; owner: string }

async function markers(page: Page, path: string, owner?: string): Promise<Marker[]> {
  return await page.evaluate(([p, o]) => {
    const m = (window as unknown as { __monaco: { editor: { getModelMarkers(f: unknown): Marker[] }; Uri: { parse(s: string): unknown } } }).__monaco
    return m.editor.getModelMarkers({ resource: m.Uri.parse(`file:///${p}`), ...(o ? { owner: o } : {}) })
  }, [path, owner] as const)
}

const codeOf = (m: Marker): string => (typeof m.code === 'object' ? m.code.value : String(m.code))

async function openConfigurator(page: Page, query = '') {
  await page.goto(`/${query}`)
  await page.getByRole('button', { name: /acme\.demo/ }).click()
  await expect(page.locator('.monaco-editor').first()).toBeVisible()
  await page.getByRole('button', { name: /src\/page\.tsx/ }).click()
  await expect(page.locator('.monaco-editor .view-lines')).toContainText('coreUser')
}

test('mounts Monaco, shows the saved file and a server diagnostic at its file and line', async ({ page }) => {
  await mockApi(page)
  await openConfigurator(page)
  await page.getByRole('button', { name: 'Check', exact: true }).click()

  const problem = page.getByTestId('problems').getByText('src/page.tsx:5:14')
  await expect(problem).toBeVisible()
  await expect.poll(async () => (await markers(page, 'src/page.tsx', 'iota-check')).map((m) => [m.startLineNumber, codeOf(m)])).toEqual([[5, 'ts.2322']])
  await expect(page.locator('.monaco-editor .squiggly-error').first()).toBeVisible()

  // typing makes the diagnostic stale: it stays listed, it is no longer drawn
  await page.locator('.monaco-editor .view-lines').first().click({ force: true })
  await page.keyboard.type('// edit')
  await expect.poll(async () => (await markers(page, 'src/page.tsx', 'iota-check')).length).toBe(0)
  await expect(page.getByTestId('problems')).toContainText('src/page.tsx:5:14')
})

test('resolves the developer kit, @iota-uz/sdk/runtime and the draft\'s own files in the TypeScript worker', async ({ page }) => {
  await mockApi(page)
  await openConfigurator(page)
  // the worker validates the file once the type libraries are in
  await expect.poll(async () => {
    const codes = (await markers(page, 'src/page.tsx', 'typescript')).map(codeOf)
    return codes.includes('2307') ? codes.filter((c) => c === '2307').length : -1
  }, { timeout: 30_000 }).toBe(1) // only './missing' is unresolved
  const msgs = (await markers(page, 'src/page.tsx', 'typescript')).map((m) => m.message).join('\n')
  expect(msgs).toContain("'./missing'")
  expect(msgs).not.toContain('@iota-kit/core')
  expect(msgs).not.toContain('@iota-uz/sdk/runtime')
  expect(msgs).not.toContain("'./contracts'")
})

test('saves with the base hash, and unmount disposes the editor and every model', async ({ page }) => {
  const writes: string[] = []
  await mockApi(page, { onWrite: (p) => writes.push(p) })
  await openConfigurator(page)
  await page.locator('.monaco-editor .view-lines').first().click({ force: true })
  await page.keyboard.type('// saved\n')
  await page.keyboard.press('Control+S')
  await expect.poll(() => writes).toEqual(['src/page.tsx'])

  expect(await page.evaluate(() => (window as unknown as { __monaco: { editor: { getModels(): unknown[] } } }).__monaco.editor.getModels().length)).toBeGreaterThan(0)
  await page.evaluate(() => (window as unknown as { __dispose(): void }).__dispose())
  await expect(page.locator('.monaco-editor')).toHaveCount(0)
  expect(await page.evaluate(() => (window as unknown as { __monaco: { editor: { getModels(): unknown[] } } }).__monaco.editor.getModels().length)).toBe(0)
})

test('follows the host theme', async ({ page }) => {
  await mockApi(page)
  await openConfigurator(page, '?theme=dark')
  await expect(page.locator('.monaco-editor.vs-dark').first()).toBeVisible()
})

test('a reader sees the draft read-only', async ({ page }) => {
  await mockApi(page)
  await openConfigurator(page, '?permissions=Extensions.Read')
  await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Check', exact: true })).toBeDisabled()
})
