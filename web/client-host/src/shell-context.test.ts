// @vitest-environment jsdom
import { describe, expect, it } from 'vitest'
import { documentShellRegistry, readShellContext } from './shell-context'

function page(value?: unknown): Document {
  const doc = document.implementation.createHTMLDocument('page')
  if (value !== undefined) doc.body.innerHTML = `<script id="iota-shell-context" type="application/json">${JSON.stringify(value)}</script>`
  return doc
}

describe('shell context', () => {
  it('returns null when the page has no shell context', () => {
    const doc = page()
    expect(readShellContext(doc)).toBeNull()
    expect(documentShellRegistry(doc).navigation()).toEqual([])
  })

  it('reads the server navigation into the shared registry', () => {
    const doc = page({ version: '1.2.0', navigation: [
      { key: 'crm', title: 'CRM', order: 0, workspace: 'sales' },
      { key: '/clients', title: 'Clients', href: '/clients', parent: 'crm', order: 0 },
    ] })
    const registry = documentShellRegistry(doc)
    expect(registry.navigation()).toMatchObject([{ key: 'crm', workspace: 'sales', children: [{ key: '/clients', href: '/clients' }] }])
    expect(documentShellRegistry(doc)).toBe(registry)
  })

  it('rejects an incompatible major version and malformed items', () => {
    expect(() => readShellContext(page({ version: '2.0.0', navigation: [] }))).toThrow(/incompatible/)
    expect(() => readShellContext(page({ version: '1.0.0', navigation: [{ key: 1 }] }))).toThrow(/requires string key/)
  })
})
