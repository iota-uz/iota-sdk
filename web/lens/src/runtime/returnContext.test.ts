import { describe, expect, it } from 'vitest'
import fixture from '../../fixtures/return-context.json'
import {
  backURL, decodeHostState, decodeReturnContext, encodeHostState, encodeReturnContext,
  hostStateParam, readReturnContext, returnParam, withReturnContext,
  type LensReturnContext,
} from './returnContext'

// The same fixture is read by pkg/lens/drillreturn: Go and the runtime must
// produce and accept identical tokens.
describe('drill return protocol', () => {
  it('matches the Go tokens of the shared fixture', () => {
    expect(fixture.param).toBe(returnParam)
    expect(fixture.hostParam).toBe(hostStateParam)
    for (const item of fixture.contexts) {
      expect(encodeReturnContext(item.context as LensReturnContext)).toBe(item.token)
      expect(decodeReturnContext(item.token)).toEqual(item.context)
    }
    for (const item of fixture.hostStates) {
      expect(encodeHostState(item.state as { leaf: string; state: string })).toBe(item.token)
      expect(decodeHostState(item.token)).toEqual(item.state)
    }
  })

  it('rejects every invalid token of the shared fixture', () => {
    for (const token of fixture.invalid) expect(decodeReturnContext(token)).toBeUndefined()
  })

  it('decorates a host link with the exploration and returns the host state only to its leaf', () => {
    const lens = new URL('https://erp.test/sales?path=root&path=root%2Fnorth&period=2026-06')
    const first = new URL(withReturnContext('/records?branch=north', lens, { title: 'Sales', trail: ['Region', 'North'] }))
    const context = readReturnContext(first)!
    expect(context).toEqual({ v: 1, href: '/sales?path=root&path=root%2Fnorth&period=2026-06', title: 'Sales', trail: ['Region', 'North'] })
    expect(first.searchParams.get('branch')).toBe('north')

    const back = new URL(backURL(context, '/records', '{"page":3}')!, lens)
    expect(back.searchParams.getAll('path')).toEqual(['root', 'root/north'])
    expect(back.searchParams.get('period')).toBe('2026-06')

    const again = readReturnContext(new URL(withReturnContext('/records?branch=north', back, { title: 'Sales', trail: ['Region', 'North'] })))!
    expect(again.host).toBe('{"page":3}')
    expect(again.href).not.toContain(hostStateParam)
    const other = readReturnContext(new URL(withReturnContext('/invoices', back, { title: 'Sales', trail: [] })))!
    expect(other.host).toBeUndefined()
  })

  it('leaves foreign and drawer links alone and needs a context to decorate', () => {
    const lens = new URL('https://erp.test/sales?path=a')
    expect(withReturnContext('https://elsewhere.test/x', lens, { title: 'Sales', trail: [] })).toBe('https://elsewhere.test/x')
    expect(withReturnContext('/sales#lens-drawer=k', lens, { title: 'Sales', trail: [] })).toBe('/sales#lens-drawer=k')
    expect(withReturnContext('/records', lens, undefined)).toBe('/records')
  })
})
