import { createSignal, For, Show } from 'solid-js'
import { Portal } from 'solid-js/web'
import type { EffectiveContext, GroupUnavailable } from '../contract/types'
import { DotsThree } from '../icons'
import { useMenuButton } from '../panels/useMenuButton'
import { useTranslate } from '../runtime'
import { reportPlacementMenu, useReportPlacementHost, type ReportPlacementAction } from './placementHost'

function actionLabel(action: ReportPlacementAction, translate: ReturnType<typeof useTranslate>): string {
  switch (action) {
    case 'open': return translate('report.action.open', 'Open report')
    case 'edit': return translate('report.action.edit', 'Edit report')
    case 'configure': return translate('report.action.configure', 'Configure card')
    case 'copy': return translate('report.action.copy', 'Make an independent copy')
    case 'remove': return translate('report.action.remove', 'Remove from dashboard')
  }
}

/** The placement that receives focus when the one acted on leaves the page. */
function focusSuccessor(section: HTMLElement | null | undefined): () => void {
  const all = [...globalThis.document.querySelectorAll<HTMLElement>('[data-report-placement]')]
  const index = section ? all.indexOf(section) : -1
  const candidates = index < 0 ? [] : [...all.slice(index + 1), ...all.slice(0, index).reverse()]
  const id = section?.dataset.reportPlacement
  return () => {
    const active = globalThis.document.activeElement
    if (active && active !== globalThis.document.body && active.isConnected) return
    const same = id ? globalThis.document.querySelector<HTMLElement>(`[data-report-placement="${CSS.escape(id)}"]`) : null
    const target = same ?? candidates.find((candidate) => candidate.isConnected)
    target?.focus()
  }
}

/**
 * The one actions menu of a report placement. Items are the host's handlers
 * that the source capabilities and the card's state allow; with none the
 * trigger is not rendered. Each item calls the host and nothing else: opening,
 * editing, copying and removing are host operations, never an implicit save.
 */
export function ReportPlacementMenu(props: {
  placementId: string
  title: string
  context?: EffectiveContext
  unavailable?: GroupUnavailable
}) {
  const host = useReportPlacementHost()
  const translate = useTranslate()
  const items = () => reportPlacementMenu(host, props.placementId, { context: props.context, unavailable: props.unavailable })
  const [pending, setPending] = createSignal<ReportPlacementAction>()
  const [failed, setFailed] = createSignal(false)
  const {
    close, container, itemRef, menu, menuPlacementProps, onMenuKeyDown, open, overlay, setOpen, trigger,
  } = useMenuButton('end')
  const label = () => translate('report.action.menu', 'Card actions: {title}', { title: props.title })

  const run = async (action: ReportPlacementAction) => {
    const handler = host?.actions?.[action]
    if (!handler || pending()) return
    const section = trigger.current?.closest<HTMLElement>('[data-report-placement]')
    const restore = focusSuccessor(section)
    close()
    trigger.current?.focus()
    setFailed(false)
    setPending(action)
    try {
      await handler(props.placementId)
    } catch (cause: unknown) {
      console.error(`[lens] placement ${props.placementId} action ${action} failed`, cause)
      setFailed(true)
    } finally {
      setPending(undefined)
      restore()
      requestAnimationFrame(restore)
    }
  }

  return (
    <Show when={items().length > 0}>
      <div class="lens-export-control lens-report-menu" ref={(element) => { container.current = element }}>
        <button
          aria-busy={pending() !== undefined || undefined}
          aria-expanded={open()}
          aria-haspopup="menu"
          aria-label={label()}
          class="lens-export-button lens-icon-button lens-report-menu-button"
          data-testid={`report-menu-${props.placementId}`}
          onClick={() => setOpen((current) => !current)}
          ref={(element) => { trigger.current = element }}
          title={label()}
          type="button"
        >
          <DotsThree />
        </button>
        <Show when={open() && overlay()}>
          <Portal mount={overlay()}>
            <div
              aria-label={label()}
              class="lens-export-menu lens-report-menu-list"
              data-align={menuPlacementProps().align}
              data-side={menuPlacementProps().side}
              onKeyDown={onMenuKeyDown}
              ref={(element) => { menu.current = element }}
              role="menu"
              style={menuPlacementProps().style}
              tabIndex={-1}
            >
              <For each={items()}>
                {(action) => (
                  <button
                    class="lens-export-menu-item"
                    data-action={action}
                    data-danger={action === 'remove' || undefined}
                    onClick={() => void run(action)}
                    ref={(element) => itemRef(action)(element)}
                    role="menuitem"
                    type="button"
                  >
                    {actionLabel(action, translate)}
                  </button>
                )}
              </For>
            </div>
          </Portal>
        </Show>
        <Show when={failed()}>
          <span class="lens-export-message lens-export-message-error" role="alert">
            {translate('report.action.failed', 'The action could not be completed.')}
          </span>
        </Show>
      </div>
    </Show>
  )
}
