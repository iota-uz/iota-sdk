// The docked Validation panel: errors and warnings with their item and field; a click selects the item
// and focuses the field. A network failure keeps the draft and offers a retry.
import { For, Show, type JSX } from 'solid-js'
import { Icon } from './icons'
import { describeItem } from './names'
import { strings } from './strings'
import type { AutomationGraph, EditorCatalog, EditorLocale, GraphError } from './types'
import { applyFix, removablePath } from './fix'
import { errorTarget, issuesOf, type ValidationState } from './validation'

export interface ValidationPanelProps {
  state: ValidationState
  graph: AutomationGraph
  catalog: EditorCatalog
  locale: EditorLocale
  onPick(target: { id: string | null; field: string | null }): void
  /** Takes the graph with an issue's fix applied (absent: fixes are not offered, e.g. read only). */
  onFix?(graph: AutomationGraph): void
  onRetry(): void
  onClose(): void
}

export function ValidationPanel(props: ValidationPanelProps): JSX.Element {
  const t = () => strings(props.locale)
  const issues = () => issuesOf(props.state)
  const row = (error: GraphError, kind: 'error' | 'warning') => {
    const target = errorTarget(props.graph, error.path)
    const where = target.id ? describeItem(props.graph, props.catalog, target.id, props.locale).title : t().validation.graph
    return (
      <li>
        <button type="button" class="ae-issue" classList={{ 'ae-issue--warning': kind === 'warning' }} onClick={() => props.onPick(target)}>
          <Icon name={kind === 'error' ? 'error' : 'dot'} class="ae-issue__icon" label={kind === 'error' ? t().validation.errors(1) : t().validation.warnings(1)} />
          <span class="ae-issue__where">{where}{target.field ? ` · ${target.field}` : ''}</span>
          <span class="ae-issue__text">{error.message}</span>
        </button>
        <Show when={error.fix && props.onFix && removablePath(error.fix.remove)}>
          <button type="button" class="ae-btn ae-btn--secondary ae-issue__fix" onClick={() => { const next = applyFix(props.graph, error.fix!.remove); if (next) props.onFix?.(next) }}>{error.fix!.label}</button>
        </Show>
      </li>
    )
  }
  return (
    <section class="ae-validation" aria-label={t().validation.title}>
      <header class="ae-validation__head">
        <h2 class="ae-validation__title">{t().validation.title}</h2>
        <span class="ae-validation__summary" role="status" aria-live="polite">
          <Show when={props.state.status === 'checking'}>{t().validation.checking}</Show>
          <Show when={props.state.status === 'idle'}>{t().validation.notChecked}</Show>
          <Show when={props.state.status === 'done' || props.state.status === 'failed'}>
            {issues().errors.length === 0 && issues().warnings.length === 0 && props.state.status === 'done'
              ? t().validation.none
              : [issues().errors.length > 0 ? t().validation.errors(issues().errors.length) : '', issues().warnings.length > 0 ? t().validation.warnings(issues().warnings.length) : ''].filter(Boolean).join(' · ')}
          </Show>
        </span>
        <button type="button" class="ae-iconbtn" aria-label={t().validation.close} title={t().validation.close} onClick={() => props.onClose()}>
          <Icon name="close" />
        </button>
      </header>
      <Show when={props.state.status === 'failed'}>
        <div class="ae-validation__failed" role="alert">
          <span>{t().validation.failed}</span>
          <button type="button" class="ae-btn ae-btn--secondary" onClick={() => props.onRetry()}>{t().validation.retry}</button>
        </div>
      </Show>
      <ul class="ae-validation__list">
        <For each={issues().errors}>{(error) => row(error, 'error')}</For>
        <For each={issues().warnings}>{(warning) => row(warning, 'warning')}</For>
      </ul>
    </section>
  )
}
