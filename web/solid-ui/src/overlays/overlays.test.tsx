import { createSignal } from 'solid-js'
import { fireEvent, render, screen } from '@solidjs/testing-library'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ConfirmationDialog, DEFAULT_BUSY_MESSAGE, Dialog } from './Dialog'
import { ViewDrawer } from './Drawer'
import { Toast, ToastProvider, ToastRegion, toastDuration } from './Toast'

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

  it('focuses the heading on phones even when showModal already focused a control', async () => {
    const matchMedia = window.matchMedia
    window.matchMedia = ((query: string) => ({ matches: query === '(max-width: 767px)', media: query })) as unknown as typeof window.matchMedia
    HTMLDialogElement.prototype.showModal = function () { this.open = true; this.querySelector<HTMLElement>('button')?.focus() }
    try {
      render(() => <ConfirmationDialog defaultOpen heading="Delete product?" confirmText="Delete" />)
      await Promise.resolve()
      expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Delete product?' }))
    } finally {
      window.matchMedia = matchMedia
    }
  })

  it('keeps native focus on desktop instead of focusing a control again', async () => {
    let focusSpy: ReturnType<typeof vi.spyOn> | undefined
    HTMLDialogElement.prototype.showModal = function () { this.open = true; const first = this.querySelector<HTMLElement>('button')!; first.focus(); focusSpy = vi.spyOn(first, 'focus') }
    render(() => <ConfirmationDialog defaultOpen heading="Delete product?" confirmText="Delete" />)
    await Promise.resolve()
    expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Close' }))
    expect(focusSpy).toBeDefined()
    expect(focusSpy).not.toHaveBeenCalled()
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
    expect(alert).toHaveClass('flex', 'items-start', 'gap-3', 'rounded-lg', 'bg-surface-300')
    expect(alert.closest('[data-toast-region]')).not.toHaveAttribute('aria-live')
    await fireEvent.mouseEnter(alert.parentElement!)
    vi.advanceTimersByTime(1200)
    expect(dismissed).not.toHaveBeenCalled()
    await fireEvent.mouseLeave(alert.parentElement!)
    vi.advanceTimersByTime(1000)
    expect(dismissed).toHaveBeenCalledOnce()
    vi.useRealTimers()
  })

  it('announces success and info politely and errors as alerts', () => {
    render(() => <ToastRegion><Toast variant="success" title="Saved" duration={0} /><Toast variant="info" title="Synced" duration={0} /><Toast variant="error" title="Failed" duration={0} /></ToastRegion>)
    const statuses = screen.getAllByRole('status')
    expect(statuses).toHaveLength(2)
    statuses.forEach((status) => expect(status).toHaveAttribute('aria-live', 'polite'))
    expect(screen.getByRole('alert')).not.toHaveAttribute('aria-live')
  })

  it('runs the action once and closes the toast', async () => {
    const onAction = vi.fn()
    const dismissed = vi.fn()
    render(() => <Toast variant="success" title="Row deleted" action={{ label: 'Undo', onAction }} onDismiss={dismissed} />)
    const undo = screen.getByRole('button', { name: 'Undo' })
    await fireEvent.click(undo)
    await fireEvent.click(undo)
    expect(onAction).toHaveBeenCalledOnce()
    expect(dismissed).toHaveBeenCalledOnce()
  })

  it('keeps a toast with an action at least 8 s and while it holds focus', async () => {
    vi.useFakeTimers()
    try {
      const dismissed = vi.fn()
      render(() => <Toast variant="error" title="Publish failed" duration={2000} action={{ label: 'Retry', onAction: () => {} }} onDismiss={dismissed} />)
      vi.advanceTimersByTime(7900)
      expect(dismissed).not.toHaveBeenCalled()
      const retry = screen.getByRole('button', { name: 'Retry' })
      retry.focus()
      await fireEvent.focusIn(retry)
      vi.advanceTimersByTime(60_000)
      expect(dismissed).not.toHaveBeenCalled()
      await fireEvent.focusOut(retry, { relatedTarget: document.body })
      vi.advanceTimersByTime(99)
      expect(dismissed).not.toHaveBeenCalled()
      vi.advanceTimersByTime(1)
      expect(dismissed).toHaveBeenCalledOnce()
    } finally {
      vi.useRealTimers()
    }
  })

  it('computes the effective toast duration', () => {
    expect(toastDuration(undefined, false)).toBe(8000)
    expect(toastDuration(3000, false)).toBe(3000)
    expect(toastDuration(3000, true)).toBe(8000)
    expect(toastDuration(12_000, true)).toBe(12_000)
    expect(toastDuration(0, true)).toBe(0)
  })

  it('passes a notification action through the provider', async () => {
    const onAction = vi.fn()
    render(() => <ToastProvider defaultDuration={0} />)
    window.dispatchEvent(new CustomEvent('notify', { detail: { id: 'undo', title: 'Archived', variant: 'success', action: { label: 'Undo', onAction } } }))
    await fireEvent.click(await screen.findByRole('button', { name: 'Undo' }))
    expect(onAction).toHaveBeenCalledOnce()
    expect(screen.queryByText('Archived')).not.toBeInTheDocument()
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

  describe('busy layers', () => {
    it('refuses every close of a busy confirmation dialog and explains why', async () => {
      const changed = vi.fn()
      const cancelled = vi.fn()
      const [busy, setBusy] = createSignal(true)
      render(() => {
        const [open, setOpen] = createSignal(true)
        return <ConfirmationDialog open={open()} onOpenChange={(next) => { setOpen(next); changed(next) }} busy={busy()} busyMessage="Still deleting" heading="Delete product?" confirmText="Delete" onCancel={cancelled} />
      })
      await Promise.resolve()
      const dialog = screen.getByRole('dialog') as HTMLDialogElement
      const status = dialog.querySelector('[data-dialog-busy-status]')!
      expect(status).toHaveAttribute('role', 'status')
      expect(status).toHaveTextContent('')
      expect(dialog).toHaveAttribute('aria-busy', 'true')
      expect(dialog.querySelector('[data-dialog-body]')).not.toBeNull()
      expect(screen.getByRole('button', { name: 'Delete' })).toHaveClass('btn-loading')

      await fireEvent.keyDown(dialog, { key: 'Escape' })
      await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
      await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
      await fireEvent.pointerDown(dialog)
      await fireEvent.click(dialog)
      dialog.dispatchEvent(new Event('cancel', { cancelable: true }))
      expect(changed).not.toHaveBeenCalled()
      expect(cancelled).not.toHaveBeenCalled()
      expect(dialog.open).toBe(true)
      expect(status).toHaveTextContent('Still deleting')

      setBusy(false)
      expect(status).toHaveTextContent('')
      expect(dialog).not.toHaveAttribute('aria-busy')
      await fireEvent.keyDown(dialog, { key: 'Escape' })
      expect(changed).toHaveBeenCalledWith(false)
      expect(dialog.open).toBe(false)
    })

    it('keeps the dialog open when onConfirm starts a request, then restores focus to the opener', async () => {
      const opener = document.createElement('button')
      opener.textContent = 'Open'
      document.body.append(opener)
      opener.focus()
      const [busy, setBusy] = createSignal(false)
      const [open, setOpen] = createSignal(true)
      render(() => <ConfirmationDialog open={open()} onOpenChange={setOpen} busy={busy()} heading="Publish?" confirmText="Publish" onConfirm={() => setBusy(true)} />)
      await Promise.resolve()
      const dialog = screen.getByRole('dialog') as HTMLDialogElement
      await fireEvent.click(screen.getByRole('button', { name: 'Publish' }))
      expect(open()).toBe(true)
      expect(dialog).toHaveAttribute('aria-busy', 'true')
      setBusy(false)
      await fireEvent.keyDown(dialog, { key: 'Escape' })
      expect(open()).toBe(false)
      expect(opener).toHaveFocus()
      opener.remove()
    })

    it('reserves the status line only for dialogs that can be busy', async () => {
      render(() => <ConfirmationDialog defaultOpen heading="Delete product?" confirmText="Delete" />)
      await Promise.resolve()
      expect(screen.getByRole('dialog').querySelector('[data-dialog-busy-status]')).toBeNull()
    })

    it('renders one fallback status line in a plain busy dialog', async () => {
      render(() => <Dialog open busy aria-label="Editor"><button>Save</button></Dialog>)
      await Promise.resolve()
      const dialog = screen.getByRole('dialog', { name: 'Editor' })
      expect(dialog.querySelectorAll('[data-dialog-busy-status]')).toHaveLength(1)
      await fireEvent.keyDown(dialog, { key: 'Escape' })
      expect(screen.getByRole('status')).toHaveTextContent(DEFAULT_BUSY_MESSAGE)
    })

    it('closes on a backdrop press unless busy or opted out', async () => {
      const changed = vi.fn()
      render(() => <Dialog open onOpenChange={changed} aria-label="Light"><button>Inside</button></Dialog>)
      await Promise.resolve()
      const dialog = screen.getByRole('dialog', { name: 'Light' })
      // A press that starts inside and ends on the backdrop is a drag, not a dismissal.
      await fireEvent.pointerDown(screen.getByRole('button', { name: 'Inside' }))
      await fireEvent.click(dialog)
      expect(changed).not.toHaveBeenCalled()
      await fireEvent.pointerDown(dialog)
      await fireEvent.click(dialog)
      expect(changed).toHaveBeenCalledWith(false)
    })

    it('refuses the view drawer close while busy', async () => {
      const changed = vi.fn()
      const [busy, setBusy] = createSignal(true)
      render(() => <ViewDrawer defaultOpen busy={busy()} title="Product" onOpenChange={changed}>Body</ViewDrawer>)
      await Promise.resolve()
      const dialog = screen.getByRole('dialog')
      await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
      await fireEvent.keyDown(dialog, { key: 'Escape' })
      expect(changed).not.toHaveBeenCalled()
      expect(screen.getByRole('status')).toHaveTextContent(DEFAULT_BUSY_MESSAGE)
      expect(dialog.querySelector('[data-dialog-body]')).toHaveTextContent('Body')
      setBusy(false)
      await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
      expect(changed).toHaveBeenCalledWith(false)
    })
  })
})
