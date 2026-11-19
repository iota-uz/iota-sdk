import { createSignal, For, Match, onCleanup, Show, Switch, type JSX } from 'solid-js'
import { originLabel, slicePeriodLabel } from './provenanceLabels'
import { ReportDestinationPicker } from '../reports/ReportDestinationPicker'
import { ReportDialog } from '../reports/ReportDialog'
import { useTranslate } from '../runtime'
import type { ReportDestinationConfirmation } from '../reports/picker'
import {
  investigationProposal, suggestedInvestigationTitle,
  type InvestigationHost, type InvestigationSaveIntent, type InvestigationSlice, type SavedInvestigation,
} from './investigation'

let saveSequence = 0
function saveId(): string {
  saveSequence += 1
  const random = globalThis.crypto?.randomUUID?.()
  return random ? `investigation-${random}` : `investigation-${Date.now().toString(36)}-${saveSequence}`
}

export interface SaveInvestigationProps {
  slice: InvestigationSlice
  host: InvestigationHost
  dashboardId: string
  dimensionLabels?: Record<string, string>
  originNames?: Record<string, string>
  theme?: string
  restoreFocus?: HTMLElement
  onClose: () => void
}

type Phase =
  | { kind: 'form' }
  | { kind: 'saving'; intent: InvestigationSaveIntent; add: boolean }
  | { kind: 'failed'; intent: InvestigationSaveIntent; add: boolean }
  | { kind: 'saved'; report: SavedInvestigation; intent: InvestigationSaveIntent }
  | { kind: 'choosing'; report: SavedInvestigation }
  | { kind: 'add-failed'; report: SavedInvestigation }
  | { kind: 'added'; report: SavedInvestigation }

/**
 * Saves the explored slice as a report through the host. The form already
 * holds the suggested name, the source, the period and every constraint with
 * its origin. A save is one host call whose id is stable across retries, so
 * a retry after a failure never creates a second report; once the report
 * exists, "add to a dashboard" is retried on its own.
 */
export function SaveInvestigation(props: SaveInvestigationProps): JSX.Element {
  const translate = useTranslate()
  const [title, setTitle] = createSignal(suggestedInvestigationTitle(props.slice))
  const [phase, setPhase] = createSignal<Phase>({ kind: 'form' })
  let pending: { key: string; id: string } | undefined
  let controller: AbortController | undefined
  let nameRef: HTMLInputElement | undefined
  onCleanup(() => controller?.abort())

  const canAdd = () => Boolean(props.host.destinations && props.host.place)
  const busy = () => phase().kind === 'saving'
  const valid = () => title().trim().length > 0

  const save = async (intent: InvestigationSaveIntent, add: boolean) => {
    if (busy() || !valid()) return
    const proposal = investigationProposal(props.slice, { title: title(), dashboardId: props.dashboardId })
    const key = JSON.stringify([intent, proposal])
    if (pending?.key !== key) pending = { key, id: saveId() }
    controller = new AbortController()
    setPhase({ kind: 'saving', intent, add })
    try {
      const report = await props.host.save({ id: pending.id, intent, proposal }, { signal: controller.signal })
      pending = undefined
      setPhase(add ? { kind: 'choosing', report } : { kind: 'saved', report, intent })
    } catch (cause: unknown) {
      if (controller.signal.aborted) return
      console.error('[lens] saving the investigation failed', cause)
      setPhase({ kind: 'failed', intent, add })
    }
  }

  const place = async (report: SavedInvestigation, confirmation: ReportDestinationConfirmation) => {
    await props.host.place!({ id: confirmation.id, report: { ...report }, choice: confirmation.choice, context: confirmation.context })
    setPhase({ kind: 'added', report })
  }

  const report = (): SavedInvestigation | undefined => {
    const current = phase()
    return 'report' in current ? current.report : undefined
  }

  const reportLink = (value: SavedInvestigation) => (
    <Show when={value.href} fallback={<strong>{value.title}</strong>}>
      <a class="lens-investigation-link" data-testid="investigation-report-link" href={value.href}>{value.title}</a>
    </Show>
  )

  const footer = (): JSX.Element => (
    <div class="lens-report-picker-footer">
      <div class="lens-report-picker-summary">
        <Show when={phase().kind === 'failed'}>
          <p class="lens-report-picker-error" role="alert">
            {translate('report.investigation.saveFailed', 'The report was not saved. Your form is kept; try again.')}
          </p>
        </Show>
      </div>
      <div class="lens-report-picker-actions">
        <Switch>
          <Match when={report()}>
            <button class="lens-context-action lens-report-primary" data-testid="investigation-done" onClick={() => props.onClose()} type="button">
              {translate('report.investigation.done', 'Done')}
            </button>
          </Match>
          <Match when={true}>
            <button class="lens-context-action" disabled={busy()} onClick={() => props.onClose()} type="button">
              {translate('report.picker.cancel', 'Cancel')}
            </button>
            <button
              aria-busy={(phase().kind === 'saving' && (phase() as { intent: string }).intent === 'draft') || undefined}
              class="lens-context-action"
              data-testid="investigation-save-draft"
              disabled={busy() || !valid()}
              onClick={() => void save('draft', false)}
              type="button"
            >
              {translate('report.investigation.saveDraft', 'Save draft')}
            </button>
            <button
              aria-busy={(phase().kind === 'saving' && !(phase() as { add: boolean }).add && (phase() as { intent: string }).intent === 'publish') || undefined}
              class={`lens-context-action${canAdd() ? '' : ' lens-report-primary'}`}
              data-testid="investigation-publish"
              disabled={busy() || !valid()}
              onClick={() => void save('publish', false)}
              type="button"
            >
              {translate('report.investigation.publish', 'Publish')}
            </button>
            <Show when={canAdd()}>
              <button
                aria-busy={(phase().kind === 'saving' && (phase() as { add: boolean }).add) || undefined}
                class="lens-context-action lens-report-primary"
                data-testid="investigation-publish-add"
                disabled={busy() || !valid()}
                onClick={() => void save('publish', true)}
                type="button"
              >
                {translate('report.investigation.publishAdd', 'Publish and add…')}
              </button>
            </Show>
          </Match>
        </Switch>
      </div>
    </div>
  )

  return (
    <Switch>
      <Match when={phase().kind === 'choosing' ? report() : undefined}>
        {(value) => (
          <ReportDestinationPicker
            context={props.slice.context}
            host={props.host.destinations!}
            onClose={() => {
              // Closing the picker without a confirmed destination keeps the
              // published report and offers adding again; nothing is re-saved.
              if (phase().kind === 'choosing') setPhase({ kind: 'add-failed', report: value() })
            }}
            onConfirm={(confirmation) => place(value(), confirmation)}
            report={{ id: value().reportId, title: value().title }}
            restoreFocus={props.restoreFocus}
            theme={props.theme as never}
          />
        )}
      </Match>
      <Match when={true}>
        <ReportDialog
          description={translate('report.investigation.description', 'Saves this exploration as a report. Its figures are recalculated every time it is opened; the dashboard you explored does not change.')}
          footer={footer()}
          initialFocus={() => nameRef}
          onClose={() => { if (!busy()) props.onClose() }}
          restoreFocus={props.restoreFocus}
          testId="save-investigation"
          theme={props.theme}
          title={translate('report.investigation.title', 'Save investigation')}
          variant="drawer"
          willClose={() => !busy()}
        >
          <div class="lens-investigation" data-phase={phase().kind}>
            <Switch>
              <Match when={phase().kind === 'saved' || phase().kind === 'added' || phase().kind === 'add-failed' ? report() : undefined}>
                {(value) => (
                  <div class="lens-investigation-result" role="status" data-testid="investigation-result">
                    <p>
                      <Switch>
                        <Match when={phase().kind === 'saved' && (phase() as { intent: string }).intent === 'draft'}>
                          {translate('report.investigation.draftSaved', 'Draft saved:')}{' '}
                        </Match>
                        <Match when={true}>{translate('report.investigation.published', 'Published:')}{' '}</Match>
                      </Switch>
                      {reportLink(value())}
                    </p>
                    <Show when={phase().kind === 'added'}>
                      <p>{translate('report.investigation.added', 'Added to the dashboard draft. The draft opens for review; nothing is published there yet.')}</p>
                    </Show>
                    <Show when={phase().kind === 'add-failed'}>
                      <p class="lens-report-picker-error" role="alert" data-testid="investigation-add-failed">
                        {translate('report.investigation.addPending', 'The report is published but was not added to a dashboard.')}
                      </p>
                      <button
                        class="lens-context-action lens-report-primary"
                        data-testid="investigation-add-retry"
                        onClick={() => setPhase({ kind: 'choosing', report: value() })}
                        type="button"
                      >
                        {translate('report.investigation.addRetry', 'Add to a dashboard')}
                      </button>
                    </Show>
                  </div>
                )}
              </Match>
            </Switch>
            <Show when={!report()}>
              <label class="lens-investigation-field">
                <span class="lens-investigation-label">{translate('report.investigation.name', 'Name')}</span>
                <input
                  class="lens-investigation-input"
                  data-testid="investigation-name"
                  disabled={busy()}
                  onInput={(event) => setTitle(event.currentTarget.value)}
                  ref={(element) => { nameRef = element }}
                  required
                  type="text"
                  value={title()}
                />
              </label>
            </Show>
            <dl class="lens-investigation-summary" data-testid="investigation-summary">
              <dt>{translate('report.investigation.source', 'Source')}</dt>
              <dd>{props.slice.reportTitle} · {props.slice.measure.title}</dd>
              <dt>{translate('report.investigation.period', 'Period')}</dt>
              <dd>
                {slicePeriodLabel(props.slice.period, translate)}
                <span class="lens-investigation-origin"> · {originLabel(props.slice.period.origin, translate, props.originNames)} · {props.slice.calendar.timezone}</span>
              </dd>
              <dt>{translate('report.investigation.constraints', 'Constraints')}</dt>
              <dd>
                <Show when={props.slice.constraints.length > 0} fallback={translate('report.investigation.noConstraints', 'None: the whole source')}>
                  <ul class="lens-investigation-constraints">
                    <For each={props.slice.constraints}>
                      {(constraint) => (
                        <li>
                          <span>{props.dimensionLabels?.[constraint.dimension] || constraint.dimension}: {constraint.labels.join(', ')}</span>
                          <span class="lens-investigation-origin"> · {originLabel(constraint.origin, translate, props.originNames)}</span>
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </dd>
            </dl>
            <p class="lens-investigation-note">
              {translate('report.investigation.refreshNote', 'The report stores this definition, not the figures shown now: it is recalculated whenever it is opened.')}
            </p>
          </div>
        </ReportDialog>
      </Match>
    </Switch>
  )
}
