import { createEffect, createSignal, For, Match, on, onCleanup, Show, Switch, type JSX } from 'solid-js'
import type { EffectiveContext } from '../contract/types'
import { LensMessagesProvider, useTranslate, type LensThemeMode } from '../runtime'
import type { Destination } from './contract/types'
import { destinationConfirmationId, type ReportDestinationChoice, type ReportDestinationConfirmation, type ReportDestinationHost } from './picker'
import { ReportDialog } from './ReportDialog'

export interface ReportDestinationPickerProps {
  /** The opened report being added. */
  report: { id: string; title: string }
  host: ReportDestinationHost
  /** The slice shown in the opened analysis; it travels to the host unchanged. */
  context?: EffectiveContext
  /** Hands the choice to the host. The dialog closes only after it resolves. */
  onConfirm: (confirmation: ReportDestinationConfirmation) => Promise<void> | void
  onClose: () => void
  restoreFocus?: HTMLElement
  theme?: LensThemeMode
  messages?: Record<string, string>
}

const searchDelay = 200

/** Chooses the dashboard an opened report is added to, or a new one. */
export function ReportDestinationPicker(props: ReportDestinationPickerProps): JSX.Element {
  return (
    <LensMessagesProvider messages={props.messages}>
      <DestinationDialog {...props} />
    </LensMessagesProvider>
  )
}

function DestinationDialog(props: ReportDestinationPickerProps): JSX.Element {
  const translate = useTranslate()
  const [search, setSearch] = createSignal('')
  const [query, setQuery] = createSignal('')
  const [destinations, setDestinations] = createSignal<Destination[]>([])
  const [state, setState] = createSignal<'loading' | 'ready' | 'error'>('loading')
  const [reload, setReload] = createSignal(0)
  const [choice, setChoice] = createSignal<string>()
  const [confirming, setConfirming] = createSignal(false)
  const [failed, setFailed] = createSignal(false)
  let searchRef: HTMLInputElement | undefined
  let pending: { choice: string; id: string } | undefined
  const known = new Map<string, Destination>()
  const name = 'lens-report-destination'

  createEffect(() => {
    const value = search()
    const timer = setTimeout(() => setQuery(value.trim()), searchDelay)
    onCleanup(() => clearTimeout(timer))
  })

  createEffect(on([query, reload], ([value]) => {
    const controller = new AbortController()
    setState('loading')
    props.host.destinations({ reportId: props.report.id, search: value }, { signal: controller.signal }).then((list) => {
      if (controller.signal.aborted) return
      for (const destination of list) known.set(destination.id, destination)
      setDestinations(list)
      setState('ready')
    }, (cause: unknown) => {
      if (controller.signal.aborted) return
      console.error('[lens] report destinations failed', cause)
      setState('error')
    })
    onCleanup(() => controller.abort())
  }))

  const resolvedChoice = (): ReportDestinationChoice | undefined => {
    const value = choice()
    if (value === undefined) return undefined
    if (value === '') return { kind: 'new' }
    const destination = known.get(value)
    return destination ? { kind: 'existing', destination: { ...destination } } : undefined
  }

  const confirm = async () => {
    const selected = resolvedChoice()
    if (!selected || confirming()) return
    const key = choice()!
    if (pending?.choice !== key) pending = { choice: key, id: destinationConfirmationId() }
    setConfirming(true)
    setFailed(false)
    try {
      await props.onConfirm({ id: pending.id, reportId: props.report.id, choice: selected, context: props.context ? structuredClone(props.context) : undefined })
      pending = undefined
      props.onClose()
    } catch (cause: unknown) {
      console.error('[lens] adding the report to a dashboard failed', cause)
      setFailed(true)
    } finally {
      setConfirming(false)
    }
  }

  const footer = (
    <div class="lens-report-picker-footer">
      <div class="lens-report-picker-summary">
        <Show when={failed()}>
          <p class="lens-report-picker-error" role="alert">
            {translate('report.destination.failed', 'The report could not be added. Your choice is kept; try again.')}
          </p>
        </Show>
      </div>
      <div class="lens-report-picker-actions">
        <button class="lens-context-action" disabled={confirming()} onClick={() => props.onClose()} type="button">
          {translate('report.picker.cancel', 'Cancel')}
        </button>
        <button
          aria-busy={confirming() || undefined}
          class="lens-context-action lens-report-primary"
          data-testid="report-destination-confirm"
          disabled={!resolvedChoice() || confirming()}
          onClick={() => void confirm()}
          type="button"
        >
          {translate('report.destination.confirm', 'Add to dashboard')}
        </button>
      </div>
    </div>
  )

  return (
    <ReportDialog
      description={translate('report.destination.description', 'The card keeps the period shown now. The dashboard changes only after you save it.')}
      footer={footer}
      initialFocus={() => searchRef}
      onClose={() => { if (!confirming()) props.onClose() }}
      restoreFocus={props.restoreFocus}
      testId="report-destination-picker"
      theme={props.theme}
      title={translate('report.destination.title', 'Add “{title}” to a dashboard', { title: props.report.title })}
    >
      <div class="lens-report-picker">
        <div class="lens-report-picker-filters">
          <input
            aria-label={translate('report.destination.search', 'Search dashboards')}
            class="lens-report-picker-search"
            onInput={(event) => setSearch(event.currentTarget.value)}
            placeholder={translate('report.destination.search', 'Search dashboards')}
            ref={(element) => { searchRef = element }}
            type="search"
            value={search()}
          />
        </div>
        <fieldset class="lens-report-destination-list">
          <legend class="lens-report-destination-legend">{translate('report.destination.list', 'Dashboards')}</legend>
          <div class="lens-report-picker-item" data-selected={choice() === '' || undefined}>
            <input checked={choice() === ''} class="lens-report-picker-check" id={`${name}-new`} name={name} onChange={() => setChoice('')} type="radio" value="" />
            <label class="lens-report-picker-name" for={`${name}-new`}>{translate('report.destination.new', 'New dashboard')}</label>
          </div>
          <Switch>
            <Match when={state() === 'loading'}>
              <p class="lens-report-picker-state" role="status">{translate('report.destination.loading', 'Loading dashboards…')}</p>
            </Match>
            <Match when={state() === 'error'}>
              <div class="lens-report-picker-state" data-state="error" role="alert">
                <p>{translate('report.destination.loadFailed', 'The dashboards could not be loaded.')}</p>
                <button class="lens-context-action" onClick={() => setReload((value) => value + 1)} type="button">
                  {translate('report.picker.retry', 'Try again')}
                </button>
              </div>
            </Match>
            <Match when={destinations().length === 0}>
              <p class="lens-report-picker-state" role="status">{translate('report.destination.empty', 'No dashboards match this search.')}</p>
            </Match>
            <Match when={true}>
              <For each={destinations()}>
                {(destination) => (
                  <div class="lens-report-picker-item" data-selected={choice() === destination.id || undefined}>
                    <input
                      checked={choice() === destination.id}
                      class="lens-report-picker-check"
                      id={`${name}-${destination.id}`}
                      name={name}
                      onChange={() => setChoice(destination.id)}
                      type="radio"
                      value={destination.id}
                    />
                    <div class="lens-report-picker-text">
                      <label class="lens-report-picker-name" for={`${name}-${destination.id}`}>{destination.title}</label>
                      <Show when={(destination.placed ?? 0) > 0}>
                        <p class="lens-report-picker-meta">
                          <span class="lens-report-picker-badge" data-tone="placed">{translate('report.destination.placed', 'Already has this report')}</span>
                        </p>
                      </Show>
                    </div>
                  </div>
                )}
              </For>
            </Match>
          </Switch>
        </fieldset>
      </div>
    </ReportDialog>
  )
}
