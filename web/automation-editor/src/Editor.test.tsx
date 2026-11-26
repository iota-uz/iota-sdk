import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { describe, expect, it, vi } from 'vitest'
import { catalog } from '../fixture/catalog'
import { connected } from '../fixture/graphs'
import { layoutCatalog, multiTrigger } from '../fixture/layoutGraphs'
import { Editor } from './Editor'
import { legacyTidyLayout } from './test/legacyTidy'
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

describe('Tidy button (ED2)', () => {
  const positions = (container: HTMLElement) =>
    [...container.querySelectorAll<HTMLElement>('.iota-flow__node')].map((n) => `${n.dataset.id}:${n.style.transform}`).sort().join('|')
  const start = () => setup({ graph: multiTrigger, catalog: layoutCatalog, layout: legacyTidyLayout(multiTrigger, layoutCatalog) })

  it('sits next to Fit, names its shortcut and does one undoable step', async () => {
    const { container } = start()
    const controls = container.querySelector('.ae-controls')!
    const buttons = [...controls.querySelectorAll('button')]
    const fit = buttons.findIndex((b) => b.getAttribute('aria-label') === 'Fit to screen')
    const tidy = buttons[fit + 1]!
    expect(tidy.getAttribute('aria-label')).toBe('Tidy up')
    expect(tidy.getAttribute('title')).toMatch(/Tidy up \((Ctrl\+Shift\+L|⇧⌘L)\)/)
    expect(tidy.getAttribute('aria-keyshortcuts')).toMatch(/Shift\+L$/)
    const before = positions(container)
    fireEvent.click(tidy)
    // The cards glide for --motion-slow, edges following: the canvas carries the class until they have landed.
    expect(container.querySelector('.ae-canvas--tidying')).not.toBeNull()
    await waitFor(() => expect(container.querySelector('.ae-canvas--tidying')).toBeNull())
    expect(positions(container)).not.toBe(before)
    // Undo glides them back the same way, as one step.
    fireEvent.click(screen.getByLabelText('Undo'))
    await waitFor(() => expect(positions(container)).toBe(before))
    expect(screen.getByLabelText('Undo')).toBeDisabled()
  })

  it('runs on Shift+Ctrl+L and stays out of fields', async () => {
    const { container } = start()
    const before = positions(container)
    fireEvent.keyDown(document.body, { key: 'L', ctrlKey: true, shiftKey: true })
    await waitFor(() => expect(positions(container)).not.toBe(before))
  })

  it('is absent from a read-only editor', () => {
    const { container } = setup({ readOnly: { reason: 'Old version' } })
    expect(container.querySelector('.ae-tidy')).toBeNull()
  })
})

describe('Editor dirty state', () => {
  it('tells the host when the draft starts and stops differing from the saved graph', async () => {
    const changes: boolean[] = []
    const { container } = setup({ onDirtyChange: (d) => changes.push(d) })
    await waitFor(() => expect(changes).toEqual([false]))
    const card = container.querySelector<HTMLElement>('.iota-flow__node[data-id="yes"]')!
    fireEvent.click(card)
    fireEvent.keyDown(card, { key: 'Delete' })
    await waitFor(() => expect(changes.at(-1)).toBe(true))
    fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true })
    await waitFor(() => expect(changes.at(-1)).toBe(false))
  })

  it('keeps the draft when it turns read only: undo and redo neither by button nor by shortcut', async () => {
    const changes: boolean[] = []
    const [readOnly, setReadOnly] = createSignal<{ reason: string } | undefined>()
    const { container } = render(() => (
      <Editor graph={connected} version={1} catalog={catalog} api={api} layout={{}} locale="en" enabled save={vi.fn()} reload={vi.fn()}
        onDirtyChange={(d) => changes.push(d)} {...(readOnly() ? { readOnly: readOnly()! } : {})} />
    ))
    const card = container.querySelector<HTMLElement>('.iota-flow__node[data-id="yes"]')!
    fireEvent.click(card)
    fireEvent.keyDown(card, { key: 'Delete' })
    await waitFor(() => expect(changes.at(-1)).toBe(true))
    // The host opens another version: the draft it drops stays as it is until the version arrives.
    setReadOnly({ reason: 'Opening v2' })
    expect(screen.getByLabelText('Undo')).toBeDisabled()
    expect(screen.getByLabelText('Redo')).toBeDisabled()
    const settle = () => new Promise((r) => setTimeout(r, 20))
    // Each shortcut is checked on its own: an unguarded ⌘Z would restore the saved graph here.
    fireEvent.click(screen.getByLabelText('Undo'))
    await settle()
    expect(changes.at(-1)).toBe(true)
    fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true })
    await settle()
    expect(changes.at(-1)).toBe(true)
    // Editable again (the switch was dropped): undo works as before and leaves a step to redo.
    setReadOnly(undefined)
    expect(screen.getByLabelText('Undo')).not.toBeDisabled()
    fireEvent.click(screen.getByLabelText('Undo'))
    await waitFor(() => expect(changes.at(-1)).toBe(false))
    expect(screen.getByLabelText('Redo')).not.toBeDisabled()
    // Read only with a step to redo: neither the button nor ⇧⌘Z nor ⌘Y redoes it.
    setReadOnly({ reason: 'Opening v2' })
    expect(screen.getByLabelText('Redo')).toBeDisabled()
    fireEvent.click(screen.getByLabelText('Redo'))
    await settle()
    expect(changes.at(-1)).toBe(false)
    fireEvent.keyDown(document.body, { key: 'z', ctrlKey: true, shiftKey: true })
    await settle()
    expect(changes.at(-1)).toBe(false)
    fireEvent.keyDown(document.body, { key: 'y', ctrlKey: true })
    await settle()
    expect(changes.at(-1)).toBe(false)
    setReadOnly(undefined)
    fireEvent.keyDown(document.body, { key: 'y', ctrlKey: true })
    await waitFor(() => expect(changes.at(-1)).toBe(true))
  })
})
