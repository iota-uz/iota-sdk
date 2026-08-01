import { useState } from 'react'
import { CaretDown, CircleNotch, DownloadSimple } from '../icons'
import { downloadPanelImage, useExport, useTranslate, type PanelImageFormat } from '../runtime'
import { useMenuButton } from './useMenuButton'

export function PanelExportMenu({ panelId, title }: { panelId: string; title: string }) {
  const exportState = useExport(panelId)
  const translate = useTranslate()
  const [imagePending, setImagePending] = useState<PanelImageFormat>()
  const [imageError, setImageError] = useState<string>()
  const { closeAndFocusTrigger, container, itemRef, onMenuKeyDown, open, setOpen, trigger } = useMenuButton()
  const label = translate('export.panel', 'Export panel')

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
      console.error(`[lens] panel ${panelId} image export failed`, cause)
      setImageError(translate('export.imageError', 'Image export failed'))
    } finally {
      setImagePending(undefined)
      requestAnimationFrame(() => trigger.current?.focus())
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
        ref={trigger}
        title={label}
        type="button"
      >
        {busy ? <CircleNotch className="lens-icon-spin" /> : <DownloadSimple />}
        {!busy && <CaretDown className="lens-export-caret" />}
      </button>
      {open && (
        <div
          className="lens-export-menu"
          onKeyDown={onMenuKeyDown}
          role="menu"
          tabIndex={-1}
        >
          {exportState.available && (
            <button className="lens-export-menu-item" onClick={() => { closeAndFocusTrigger(); void exportState.run() }} ref={itemRef('data')} role="menuitem" type="button">
              {translate('export.data', 'Data (XLSX)')}
            </button>
          )}
          <button className="lens-export-menu-item" onClick={() => { void image('png') }} ref={itemRef('png')} role="menuitem" type="button">
            {translate('export.png', 'Image (PNG)')}
          </button>
          <button className="lens-export-menu-item" onClick={() => { void image('svg') }} ref={itemRef('svg')} role="menuitem" type="button">
            {translate('export.svg', 'Vector (SVG)')}
          </button>
        </div>
      )}
      {(imageError || exportState.message) && (
        <span className="lens-export-message lens-export-message-error" role={imageError || exportState.status === 'error' ? 'alert' : 'status'}>{imageError ?? exportState.message}</span>
      )}
    </div>
  )
}
