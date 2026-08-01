import { useCallback, useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'

/** Shared focus, dismissal, and arrow navigation for Lens menu buttons. */
export function useMenuButton() {
  const [open, setOpen] = useState(false)
  const container = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const items = useRef(new Map<string, HTMLButtonElement>())
  const close = useCallback(() => setOpen(false), [])
  const closeAndFocusTrigger = useCallback(() => {
    close()
    trigger.current?.focus()
  }, [close])

  useEffect(() => {
    if (!open || typeof document === 'undefined') return undefined
    const focusFrame = requestAnimationFrame(() => [...items.current.values()][0]?.focus())
    const pointerdown = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) close()
    }
    const keydown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      closeAndFocusTrigger()
    }
    document.addEventListener('pointerdown', pointerdown, true)
    document.addEventListener('keydown', keydown, true)
    return () => {
      cancelAnimationFrame(focusFrame)
      document.removeEventListener('pointerdown', pointerdown, true)
      document.removeEventListener('keydown', keydown, true)
    }
  }, [close, closeAndFocusTrigger, open])

  const itemRef = useCallback((key: string) => (element: HTMLButtonElement | null) => {
    if (element) items.current.set(key, element)
    else items.current.delete(key)
  }, [])
  const onMenuKeyDown = useCallback((event: ReactKeyboardEvent) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const activeItems = [...items.current.values()].filter((item) => item.isConnected)
    if (activeItems.length === 0) return
    if (event.key === 'Home') {
      activeItems[0]?.focus()
      return
    }
    if (event.key === 'End') {
      activeItems[activeItems.length - 1]?.focus()
      return
    }
    const current = activeItems.indexOf(document.activeElement as HTMLButtonElement)
    const delta = event.key === 'ArrowDown' ? 1 : -1
    activeItems[(current + delta + activeItems.length) % activeItems.length]?.focus()
  }, [])

  return { close, closeAndFocusTrigger, container, itemRef, onMenuKeyDown, open, setOpen, trigger }
}
