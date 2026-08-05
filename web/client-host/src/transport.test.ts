import { describe, expect, it } from 'vitest'
import { ManagedSession } from './transport'

describe('ManagedSession', () => {
  it('deduplicates concurrent refresh and swaps the snapshot atomically', async () => {
    let calls = 0
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    const session = new ManagedSession({ csrf: 'old' }, async () => { calls += 1; await gate; return { csrf: 'new' } })
    const signal = new AbortController().signal
    const first = session.refresh(signal)
    const second = session.refresh(signal)
    expect(session.snapshot().csrf).toBe('old')
    release()
    await Promise.all([first, second])
    expect(calls).toBe(1)
    expect(session.snapshot().csrf).toBe('new')
  })
})
