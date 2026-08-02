import { useState } from 'react'
import { CaretDown, CircleNotch, DownloadSimple } from '../icons'
import { downloadPanelImage, useDashboard, useExport, useTranslate, type PanelImageFormat } from '../runtime'
import { useMenuButton } from './useMenuButton'

export function PanelExportMenu({ panelId, title }: { panelId: string; title: string }) {
  const exportState = useExport(panelId)
  const translate = useTranslate()
  const { document } = useDashboard()
  // The slice this panel is a picture of. The period comes from the document's
  // own filter state, so the name says which of two identically titled exports
  // is which.
  const period = document?.filters?.find((filter) => filter.kind === 'period')?.period?.value
  const scope = {
    dashboard: document?.meta?.dashboardId || document?.meta?.title,
    start: period?.start,
    end: period?.end,
  }
  const [imagePending, setImagePending] = useState<PanelImageFormat>()
  const [imageError, setImageError] = useState<string>()
  // An icon trigger rides the card's right edge, so its menu hangs from that
  // edge; a start-aligned menu would leave the card.
  const {
    closeAndFocusTrigger, container, itemRef, menu, menuPlacementProps, onMenuKeyDown, open, setOpen, trigger,
  } = useMenuButton('end')
  const label = translate('export.panel', 'Export panel')

  const image = async (format: PanelImageFormat) => {
    setOpen(false)
    setImageError(undefined)
    setImagePending(format)
    try {
      // React removes the menu after the click handler completes. Capturing in
      // the same task serializes the still-open menu into the panel image.
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
      await downloadPanelImage(panelId, title, format, scope)
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
          ref={menu}
          role="menu"
          tabIndex={-1}
          {...menuPlacementProps}
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
            {/* Both items produce a picture of this panel; only the file format
                differs. Naming one by its artefact and the other by its
                technique («Image» / «Vector») made them read as two kinds of
                thing. */}
            {translate('export.svg', 'Image (SVG)')}
          </button>
        </div>
      )}
      {(imageError || exportState.message) && (
        <span className="lens-export-message lens-export-message-error" role={imageError || exportState.status === 'error' ? 'alert' : 'status'}>{imageError ?? exportState.message}</span>
      )}
    </div>
  )
}
