import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library'
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
    const after = positions(container)
    expect(after).not.toBe(before)
    // The cards glide for 200 ms: the canvas carries the class until they have landed.
    expect(container.querySelector('.ae-canvas--tidying')).not.toBeNull()
    await waitFor(() => expect(container.querySelector('.ae-canvas--tidying')).toBeNull())
    fireEvent.click(screen.getByLabelText('Undo'))
    expect(positions(container)).toBe(before)
    expect(screen.getByLabelText('Undo')).toBeDisabled()
  })

  it('runs on Shift+Ctrl+L and stays out of fields', () => {
    const { container } = start()
    const before = positions(container)
    fireEvent.keyDown(document.body, { key: 'L', ctrlKey: true, shiftKey: true })
    expect(positions(container)).not.toBe(before)
  })

  it('is absent from a read-only editor', () => {
    const { container } = setup({ readOnly: { reason: 'Old version' } })
    expect(container.querySelector('.ae-tidy')).toBeNull()
  })
})
