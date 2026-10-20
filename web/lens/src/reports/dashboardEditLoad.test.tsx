import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library'
import { createSignal } from 'solid-js'
import { afterEach, expect, it, vi } from 'vitest'
import { LensDashboard } from '../LensDashboard'
import { editComposition, editDocument } from '../stories/dashboardEdit'
import { createReportDraft } from './draft'

// The first import of the edit chunk fails, as after a deploy that replaced it;
// every later import resolves to the real module.
const chunk = vi.hoisted(() => ({ failures: 1 }))
vi.mock('./dashboardEdit', async (importOriginal) => {
  if (chunk.failures > 0) {
    chunk.failures -= 1
    throw new Error('chunk failed to load')
  }
  return importOriginal()
})

afterEach(() => {
  cleanup()
  document.body.replaceChildren()
})

// False green: logging the failure alone leaves the board read-only with no way back into edit mode.
it('offers a retry when the edit chunk fails to load and enters edit mode after it', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => undefined)
  render(() => {
    const draft = createReportDraft(editComposition(), { save: (value) => Promise.resolve(value) })
    const [editing, setEditing] = createSignal(false)
    return <LensDashboard edit={{ draft, editing, setEditing }} fetcher={() => new Promise<Response>(() => undefined)} initialDocument={editDocument()} />
  })
  const alert = await screen.findByTestId('dashboard-edit-load-failed')
  expect(screen.queryByTestId('dashboard-edit')).toBeNull()
  fireEvent.click(alert.querySelector('button')!)
  expect(await screen.findByTestId('dashboard-edit')).toBeInTheDocument()
  expect(screen.queryByTestId('dashboard-edit-load-failed')).toBeNull()
})
