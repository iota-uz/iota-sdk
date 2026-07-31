import { useCallback, useEffect, useRef, useState } from 'react'
import { CaretDown, CircleNotch, DownloadSimple } from '../icons'
import { downloadPanelImage, useExport, useTranslate, type PanelImageFormat } from '../runtime'

export function PanelExportMenu({ panelId, title }: { panelId: string; title: string }) {
  const exportState = useExport(panelId)
  const translate = useTranslate()
  const [open, setOpen] = useState(false)
  const [imagePending, setImagePending] = useState<PanelImageFormat>()
  const [imageError, setImageError] = useState<string>()
  const container = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const itemRefs = useRef<Array<HTMLButtonElement | null>>([])
  const label = translate('export.panel', 'Export panel')

  const closeAndFocusTrigger = useCallback(() => {
    setOpen(false)
    triggerRef.current?.focus()
  }, [])

  useEffect(() => {
    if (!open) return undefined
    itemRefs.current[0]?.focus()
    const close = (event: PointerEvent) => {
      if (event.target instanceof Node && !container.current?.contains(event.target)) setOpen(false)
    }
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.stopPropagation()
      closeAndFocusTrigger()
    }
    document.addEventListener('pointerdown', close, true)
    document.addEventListener('keydown', escape, true)
    return () => {
      document.removeEventListener('pointerdown', close, true)
      document.removeEventListener('keydown', escape, true)
    }
  }, [closeAndFocusTrigger, open])

  const image = async (format: PanelImageFormat) => {
    setOpen(false)
    setImageError(undefined)
    setImagePending(format)
    try {
      // React removes the menu after the click handler completes. Capturing in
      // the same task serializes the still-open menu into the panel image.
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
      await downloadPanelImage(panelId, title, format)
    } catch (cause: unknown) {
      setImageError(cause instanceof Error ? cause.message : translate('export.imageError', 'Image export failed'))
    } finally {
      setImagePending(undefined)
      requestAnimationFrame(() => triggerRef.current?.focus())
    }
  }

  const busy = exportState.status === 'pending' || imagePending !== undefined
  return (
    <div className="lens-export-control" ref={container}>
      <button
        aria-busy={busy}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={label}
        className="lens-export-button lens-icon-button"
        disabled={busy}
        onClick={() => setOpen((current) => !current)}
        ref={triggerRef}
        title={label}
        type="button"
      >
        {busy ? <CircleNotch className="lens-icon-spin" /> : <DownloadSimple />}
        {!busy && <CaretDown className="lens-export-caret" />}
      </button>
      {open && (
        <div
          className="lens-export-menu"
          onKeyDown={(event) => {
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
            event.preventDefault()
            const items = itemRefs.current.filter((item): item is HTMLButtonElement => item !== null)
            if (items.length === 0) return
            const current = items.indexOf(document.activeElement as HTMLButtonElement)
            const delta = event.key === 'ArrowDown' ? 1 : -1
            items[(current + delta + items.length) % items.length]?.focus()
          }}
          role="menu"
        >
          {exportState.available && (
            <button className="lens-export-menu-item" onClick={() => { closeAndFocusTrigger(); void exportState.run() }} ref={(element) => { itemRefs.current[0] = element }} role="menuitem" type="button">
              {translate('export.data', 'Data (XLSX)')}
            </button>
          )}
          <button className="lens-export-menu-item" onClick={() => { void image('png') }} ref={(element) => { itemRefs.current[exportState.available ? 1 : 0] = element }} role="menuitem" type="button">
            {translate('export.png', 'Image (PNG)')}
          </button>
          <button className="lens-export-menu-item" onClick={() => { void image('svg') }} ref={(element) => { itemRefs.current[exportState.available ? 2 : 1] = element }} role="menuitem" type="button">
            {translate('export.svg', 'Vector (SVG)')}
          </button>
        </div>
      )}
      {(imageError || exportState.message) && (
        <span className="lens-export-message lens-export-message-error" role="status">{imageError ?? exportState.message}</span>
      )}
    </div>
  )
}
