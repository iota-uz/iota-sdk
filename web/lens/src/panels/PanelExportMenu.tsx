/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createSignal } from 'solid-js'
import { Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import { CaretDown, CircleNotch, DownloadSimple } from '../icons'
import { downloadPanelImage, useDashboard, useExport, useTranslate, type PanelImageFormat } from '../runtime'
import { useMenuButton } from './useMenuButton'

export function PanelExportMenu(props: { panelId: string; title: string }) {
  const exportState = useExport(props.panelId)
  const translate = useTranslate()
  const { document } = useDashboard()
  // The slice this panel is a picture of. The period comes from the document's
  // own filter state, so the name says which of two identically titled exports
  // is which.
  const period = () => document.filters?.find((filter) => filter.kind === 'period')?.period?.value
  const [imagePending, setImagePending] = createSignal<PanelImageFormat>()
  const [imageError, setImageError] = createSignal<string>()
  // An icon trigger rides the card's right edge, so its menu hangs from that
  // edge; a start-aligned menu would leave the card.
  const {
    closeAndFocusTrigger, container, itemRef, menu, menuPlacementProps, onMenuKeyDown, open, overlay, setOpen, trigger,
  } = useMenuButton('end')
  const label = translate('export.panel', 'Export panel')

  const image = async (format: PanelImageFormat) => {
    setOpen(false)
    setImageError(undefined)
    setImagePending(format)
    try {
      // The menu is removed after the click handler completes. Capturing in
      // the same task serializes the still-open menu into the panel image.
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
      await downloadPanelImage(props.panelId, props.title, format, {
        dashboard: document.meta?.dashboardId || document.meta?.title,
        start: period()?.start,
        end: period()?.end,
      })
    } catch (cause: unknown) {
      console.error(`[lens] panel ${props.panelId} image export failed`, cause)
      setImageError(translate('export.imageError', 'Image export failed'))
    } finally {
      setImagePending(undefined)
      requestAnimationFrame(() => trigger.current?.focus())
    }
  }

  const busy = () => exportState.status === 'pending' || imagePending() !== undefined
  return (
    <div class="lens-export-control" ref={(el) => { container.current = el }}>
      <button
        aria-busy={busy()}
        aria-expanded={open()}
        aria-haspopup="menu"
        aria-label={label}
        class="lens-export-button lens-icon-button lens-panel-export-button"
        disabled={busy()}
        onClick={() => setOpen((current) => !current)}
        ref={(el) => { trigger.current = el }}
        title={label}
        type="button"
      >
        {busy() ? <CircleNotch className="lens-icon-spin" /> : <DownloadSimple />}
        {!busy() && <CaretDown className="lens-export-caret" />}
      </button>
      <Show when={open() && overlay()}>
        <Portal mount={overlay()}>
          <div
            class="lens-export-menu"
            onKeyDown={onMenuKeyDown}
            ref={(el) => { menu.current = el }}
            role="menu"
            tabIndex={-1}
            data-align={menuPlacementProps().align}
            data-side={menuPlacementProps().side}
            style={menuPlacementProps().style}
          >
            <Show when={exportState.available}>
              <button
                class="lens-export-menu-item"
                onClick={() => { closeAndFocusTrigger(); void exportState.run() }}
                ref={(el) => itemRef('data')(el)}
                role="menuitem"
                type="button"
              >
                {translate('export.data', 'Data (XLSX)')}
              </button>
            </Show>
            <button class="lens-export-menu-item" onClick={() => { void image('png') }} ref={(el) => itemRef('png')(el)} role="menuitem" type="button">
              {translate('export.png', 'Image (PNG)')}
            </button>
            <button class="lens-export-menu-item" onClick={() => { void image('svg') }} ref={(el) => itemRef('svg')(el)} role="menuitem" type="button">
              {/* Both items produce a picture of this panel; only the file format
                  differs. Naming one by its artefact and the other by its
                  technique («Image» / «Vector») made them read as two kinds of
                  thing. */}
              {translate('export.svg', 'Image (SVG)')}
            </button>
          </div>
        </Portal>
      </Show>
      {(imageError() || exportState.message) && (
        <span class="lens-export-message lens-export-message-error" role={imageError() || exportState.status === 'error' ? 'alert' : 'status'}>{imageError() ?? exportState.message}</span>
      )}
    </div>
  )
}
