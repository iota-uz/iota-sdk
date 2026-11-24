import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { catalog } from '../fixture/catalog'
import { connected } from '../fixture/graphs'
import { Editor } from './Editor'
import type { EditorApi, EditorProps } from './types'

const api: EditorApi = {
  validate: vi.fn(async () => ({ ok: true, errors: [] })),
  dryRun: vi.fn(async () => ({ trace: [] })),
  testNode: vi.fn(async () => ({ outcome: 'ok' })),
}

function setup(overrides: Partial<EditorProps> = {}) {
  const save = vi.fn(async (graph) => ({ ok: true as const, graph, version: 2 }))
  const utils = render(() => (
    <Editor graph={connected} version={1} catalog={catalog} api={api} layout={{}} locale="en" enabled save={save} reload={vi.fn()} {...overrides} />
  ))
  return { ...utils, save }
}

describe('Editor keyboard (K6)', () => {
  it('shortcuts do not fire while focus is in a field; ⌘Z there stays the field’s own undo', async () => {
    const { container } = setup()
    const card = container.querySelector<HTMLElement>('.iota-flow__node[data-id="yes"]')!
    fireEvent.click(card)
    const name = await screen.findByText('Delay', { selector: '.ae-panel__title' })
    expect(name).toBeTruthy()
    const input = container.querySelector<HTMLInputElement>('.ae-panel input, .ae-panel textarea')!
    input.focus()
    fireEvent.keyDown(input, { key: 'k', ctrlKey: true })
    expect(container.querySelector('.ae-palette')).toBeNull()
    fireEvent.keyDown(input, { key: 'Delete' })
    expect(container.querySelector('.iota-flow__node[data-id="yes"]')).not.toBeNull()
    // Outside a field the same key opens the palette.
    fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true })
    // The palette is its own lazy chunk.
    await waitFor(() => expect(container.querySelector('.ae-palette')).not.toBeNull())
  })

  it('a read-only editor renders no save and no connectable handles', () => {
    const { container } = setup({ readOnly: { reason: 'Old version' } })
    expect(container.querySelector('.ae-save')).toBeNull()
    expect(container.querySelector('.iota-flow__handle.connectable')).toBeNull()
    expect(container.textContent).toContain('Old version')
  })
})

describe('Editor remote layout', () => {
  it('positions another tab saved replace those on screen without a layout save', async () => {
    const saveLayout = vi.fn(async () => undefined)
    const [remote, setRemote] = createSignal<{ positions: Record<string, { x: number; y: number }> } | null>(null)
    const { container } = render(() => (
      <Editor graph={connected} version={1} catalog={catalog} api={api} layout={{}} locale="en" enabled save={vi.fn()} reload={vi.fn()} saveLayout={saveLayout} remoteLayout={remote()} />
    ))
    const card = () => container.querySelector<HTMLElement>('.iota-flow__node[data-id="yes"]')!
    await waitFor(() => expect(card()).not.toBeNull())
    const before = card().style.transform
    setRemote({ positions: { yes: { x: 960, y: 640 } } })
    await waitFor(() => expect(card().style.transform).not.toBe(before))
    expect(card().style.transform).toContain('960')
    await new Promise((r) => setTimeout(r, 700))
    expect(saveLayout).not.toHaveBeenCalled()
  })
})
