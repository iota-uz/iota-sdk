import { createSignal } from 'solid-js'
import { fireEvent, render, screen } from '@solidjs/testing-library'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ConfirmationDialog, Dialog } from './Dialog'
import { ViewDrawer } from './Drawer'
import { Toast, ToastProvider, ToastRegion } from './Toast'

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () { this.open = true }
  HTMLDialogElement.prototype.close = function () { this.open = false; this.dispatchEvent(new Event('close')) }
})

describe('overlay primitives', () => {
  it('supports controlled dialog state, initial focus, focus wrapping and Escape', async () => {
    const escaped = vi.fn()
    const changed = vi.fn()
    render(() => {
      const [open, setOpen] = createSignal(true)
      return <Dialog open={open()} onOpenChange={(next) => { setOpen(next); changed(next) }} onEscape={escaped} aria-label="Editor"><button>First</button><button>Last</button></Dialog>
    })
    await Promise.resolve()
    const dialog = screen.getByRole('dialog', { name: 'Editor' }) as HTMLDialogElement
    const first = screen.getByRole('button', { name: 'First' })
    const last = screen.getByRole('button', { name: 'Last' })
    expect(dialog.open).toBe(true)
    expect(first).toHaveFocus()
    last.focus()
    await fireEvent.keyDown(last, { key: 'Tab' })
    expect(first).toHaveFocus()
    await fireEvent.keyDown(first, { key: 'Escape' })
    expect(escaped).toHaveBeenCalledOnce()
    expect(changed).toHaveBeenCalledWith(false)
    expect(dialog.open).toBe(false)
  })

  it('renders and closes the canonical uncontrolled confirmation dialog', async () => {
    const confirmed = vi.fn()
    render(() => <ConfirmationDialog defaultOpen heading="Delete product?" text="This cannot be undone" confirmText="Delete" onConfirm={confirmed} />)
    await Promise.resolve()
    const dialog = screen.getByRole('dialog') as HTMLDialogElement
    expect(dialog).toHaveClass('dialog', 'dialog-rounded', 'dialog-btt')
    await fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(confirmed).toHaveBeenCalledOnce()
    expect(dialog.open).toBe(false)
  })

  it('reopens after native close when a controlled owner refuses the change', async () => {
    const changed = vi.fn()
    render(() => <Dialog open onOpenChange={changed} aria-label="Pinned"><button>Inside</button></Dialog>)
    await Promise.resolve()
    const dialog = screen.getByRole('dialog', { name: 'Pinned' }) as HTMLDialogElement
    dialog.close()
    await Promise.resolve()
    expect(changed).toHaveBeenCalledWith(false)
    expect(dialog.open).toBe(true)
  })

  it('renders canonical view drawer and reports close', async () => {
    const changed = vi.fn()
    render(() => <ViewDrawer defaultOpen title="Product" onOpenChange={changed}>Body</ViewDrawer>)
    await Promise.resolve()
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveClass('dialog-rtl', 'w-full', 'h-full', 'flex', 'items-stretch')
    await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(changed).toHaveBeenCalledWith(false)
  })

  it('uses live-region roles and pauses automatic toast dismissal', async () => {
    vi.useFakeTimers()
    const dismissed = vi.fn()
    render(() => <ToastRegion><Toast variant="warning" title="Review" message="Missing field" duration={1000} onDismiss={dismissed} /></ToastRegion>)
    const alert = screen.getByRole('alert')
    expect(alert).toHaveClass('flex', 'items-start', 'gap-3', 'rounded-lg')
    await fireEvent.mouseEnter(alert.parentElement!)
    vi.advanceTimersByTime(1200)
    expect(dismissed).not.toHaveBeenCalled()
    await fireEvent.mouseLeave(alert.parentElement!)
    vi.advanceTimersByTime(1000)
    expect(dismissed).toHaveBeenCalledOnce()
    vi.useRealTimers()
  })

  it('owns an event-driven bounded toast queue and removes by event id', async () => {
    render(() => <ToastProvider max={2} defaultDuration={0} />)
    window.dispatchEvent(new CustomEvent('notify', { detail: { id: 'one', title: 'One', variant: 'success' } }))
    window.dispatchEvent(new CustomEvent('notify', { detail: { id: 'two', title: 'Two' } }))
    window.dispatchEvent(new CustomEvent('notify', { detail: { id: 'three', title: 'Three' } }))
    expect(screen.queryByText('One')).not.toBeInTheDocument()
    expect(screen.getByText('Two')).toBeVisible()
    window.dispatchEvent(new CustomEvent('remove-notification', { detail: 'two' }))
    expect(screen.queryByText('Two')).not.toBeInTheDocument()
  })
})
