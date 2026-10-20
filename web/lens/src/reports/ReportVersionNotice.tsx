import { createSignal, Show } from 'solid-js'
import type { LayoutGroup } from '../contract/types'
import { useDocumentState, useTranslate } from '../runtime'
import { useReportPlacementHost } from './placementHost'

/**
 * Update-available keeps the opened figures intact and offers the newer
 * version; incompatible explains why the placement cannot be shown.
 */
export function ReportVersionNotice(props: { placementId: string; unavailable?: LayoutGroup['unavailable'] }) {
  const host = useReportPlacementHost()
  const documentState = useDocumentState()
  const translate = useTranslate()
  const [kept, setKept] = createSignal(false)
  const [updating, setUpdating] = createSignal(false)
  const [failed, setFailed] = createSignal(false)
  const status = () => host?.status(props.placementId)
  let root: HTMLDivElement | undefined
  /** The notice removes its own buttons; hand focus to the placement rather than the document body. */
  const keepFocus = () => {
    const section = root?.closest<HTMLElement>('section')
    const id = props.placementId
    const restore = () => {
      const active = globalThis.document.activeElement
      if (active && active !== globalThis.document.body && active.isConnected) return
      const target = section?.isConnected ? section : globalThis.document.querySelector<HTMLElement>(`[data-report-placement="${CSS.escape(id)}"]`)
      target?.focus()
    }
    restore()
    requestAnimationFrame(restore)
  }
  const update = async () => {
    if (updating()) return
    setUpdating(true)
    setFailed(false)
    try {
      await host?.update?.(props.placementId)
      await documentState.refresh()
      keepFocus()
    } catch {
      setFailed(true)
    } finally {
      setUpdating(false)
    }
  }
  return <>
    <Show when={!props.unavailable && status() === 'update-available' && !kept()}>
      <div class="lens-report-version" data-testid={`report-version-${props.placementId}`} data-status="update-available" role="status" ref={(element) => { root = element }}>
        <p>{translate('report.version.updateAvailable', 'A newer version of this report is available.')}</p>
        <div class="lens-report-version-actions">
          <button type="button" class="lens-context-action lens-report-version-update" aria-disabled={updating() || undefined} onClick={() => void update()}>
            {translate('report.version.update', 'Update report')}
          </button>
          <button type="button" class="lens-context-action" aria-disabled={updating() || undefined} onClick={() => {
            if (updating()) return
            host?.keep?.(props.placementId)
            setKept(true)
            keepFocus()
          }}>{translate('report.version.keep', 'Keep this version')}</button>
        </div>
        <Show when={failed()}>
          <p class="lens-context-error" role="alert">{translate('report.version.updateFailed', 'The report could not be updated. The figures shown are still the opened version.')}</p>
        </Show>
      </div>
    </Show>
    <Show when={props.unavailable || status() === 'incompatible'}>
      <div class="lens-panel-state lens-report-unavailable" data-testid={`report-version-${props.placementId}`} data-status="incompatible" role="alert">
        <p>{translate('report.version.incompatible', 'This report is unavailable: its definition no longer matches the data source. Ask the report owner to update it.')}</p>
      </div>
    </Show>
  </>
}
