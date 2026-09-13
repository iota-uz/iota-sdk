import { createRoot } from 'solid-js'
import { describe, expect, it } from 'vitest'
import { createDraft } from './draft'
import { HostError } from './errors'

describe('Solid draft', () => {
  it('edits nested values without mutating the server snapshot and resets', () => createRoot((dispose) => {
    const server = { name: 'Base', terms: [{ rate: 10 }] }
    const draft = createDraft(server)
    draft.set('terms', 0, 'rate', 15)
    expect(server.terms[0]?.rate).toBe(10)
    expect(draft.dirty()).toBe(true)
    draft.reset()
    expect(draft.value.terms[0]?.rate).toBe(10)
    expect(draft.dirty()).toBe(false)
    dispose()
  }))

  it('keeps edits and field errors after a failed save', async () => {
    const { draft, dispose } = createRoot((dispose) => {
      const draft = createDraft({ name: 'Base' })
      return { draft, dispose }
    })
    draft.set('name', 'Edited')
    await expect(draft.save(async () => { throw new HostError('field_validation', 'Invalid', { name: 'Required' }) })).rejects.toThrow('Invalid')
    expect(draft.value.name).toBe('Edited')
    expect(draft.dirty()).toBe(true)
    expect(draft.fieldErrors()).toEqual({ name: 'Required' })
    dispose()
  })

  it('does not overwrite edits made while save is in flight', async () => {
    let finish!: (value: { name: string }) => void
    const { draft, dispose } = createRoot((dispose) => {
      const draft = createDraft({ name: 'Base' })
      return { draft, dispose }
    })
    draft.set('name', 'Saving')
    const saving = draft.save(() => new Promise((done) => { finish = done }))
    draft.set('name', 'New edit')
    finish({ name: 'Saved by server' })
    expect(await saving).toBe(false)
    expect(draft.value.name).toBe('New edit')
    expect(draft.dirty()).toBe(true)
    dispose()
  })

  it('defers a new server snapshot while dirty and registers a navigation guard', () => createRoot((dispose) => {
    let guard = () => true
    const draft = createDraft({ name: 'Base' }, { navigation: { guard: (check) => { guard = check; return () => undefined }, canNavigate: () => guard() } })
    draft.set('name', 'Edited')
    expect(draft.offerServerSnapshot({ name: 'Remote' })).toBe(false)
    expect(draft.value.name).toBe('Edited')
    expect(guard()).toBe(false)
    dispose()
  }))
})
