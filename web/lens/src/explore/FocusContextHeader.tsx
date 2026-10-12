import { For, Show, type JSX } from 'solid-js'
import type { FieldFormat } from '../contract'
import { CaretRight } from '../icons'
import { StatusChip } from '../panels/StatPanel'
import { StatValueTicker } from '../panels/StatValueTicker'
import { useFormat, useTranslate } from '../runtime'
import { ContextMiniChart } from './ContextMiniChart'
import type { FocusContext } from './focusModel'
import type { ExploreBreadcrumb } from './model'

/**
 * The always-visible context block of a focus canvas: the breadcrumb back to
 * the metric's ancestors, the metric name with its counting value, the share
 * of the parent, the reporting period, the level's quality status, and the
 * parent mini-chart with the focused element highlighted.
 */
export interface FocusContextHeaderProps {
  context: FocusContext
  breadcrumbs: Array<ExploreBreadcrumb>
  valueFormat?: FieldFormat
  colorFor: (label: string, index: number) => string | undefined
  periodLabel?: string
  onCrumb: (pathIndex: number) => void
  onParent?: () => void
}

export function FocusContextHeader(props: FocusContextHeaderProps): JSX.Element {
  const translate = useTranslate()
  const formatValue = useFormat(props.valueFormat)
  const formatShare = useFormat({ kind: 'percent', minorUnits: false, precision: 1, decimalSeparator: '.' })
  const breadcrumbs = () => props.breadcrumbs
  const collapsible = () => breadcrumbs().length > 3
  const share = () => props.context.share !== undefined && props.context.total !== undefined
    ? translate('explore.shareOfTotal', '{share} of {total}', {
      share: formatShare(props.context.share * 100),
      total: formatValue(props.context.total),
    })
    : undefined

  return (
    <div class="lens-focus-header">
      <Show when={breadcrumbs().length > 1}>
        <nav aria-label={translate('focus.path', 'Drill path')} class="lens-focus-breadcrumb">
          <ol>
            <For each={breadcrumbs()}>
              {(crumb, index) => {
                // In a narrow container the middle of a long path collapses to a
                // static ellipsis: the first crumb keeps the origin, the last two
                // keep the immediate context, and the overlay still has the rest.
                const middle = collapsible() && index() > 0 && index() < breadcrumbs().length - 2
                return (
                  <>
                    <Show when={collapsible() && index() === breadcrumbs().length - 2}>
                      <li aria-hidden="true" class="lens-focus-crumb-gap">
                        <CaretRight className="lens-focus-crumb-sep" size={12} />
                        <span>…</span>
                      </li>
                    </Show>
                    <li class={middle ? 'lens-focus-crumb-middle' : undefined}>
                      <Show when={index() > 0}>
                        <CaretRight className="lens-focus-crumb-sep" size={12} />
                      </Show>
                      <Show
                        when={crumb.current}
                        fallback={(
                          <button class="lens-focus-crumb" onClick={() => props.onCrumb(crumb.pathIndex)} type="button">
                            {crumb.label}
                          </button>
                        )}
                      >
                        <span aria-current="page" class="lens-focus-crumb lens-focus-crumb-current">
                          {crumb.label}
                        </span>
                      </Show>
                    </li>
                  </>
                )
              }}
            </For>
          </ol>
        </nav>
      </Show>
      <div class="lens-focus-context">
        <div class="lens-focus-context-body">
          <span class="lens-focus-name" title={props.context.label}>{props.context.label}</span>
          <Show when={props.context.value !== undefined}>
            <span class="lens-focus-value">
              <StatValueTicker text={formatValue(props.context.value)} />
            </span>
          </Show>
          <Show when={share()}>
            <span class="lens-focus-share">{share()}</span>
          </Show>
          <Show when={props.periodLabel}>
            <span class="lens-focus-period">{props.periodLabel}</span>
          </Show>
          <Show when={props.context.status}>
            <StatusChip status={props.context.status!} />
          </Show>
        </div>
        <Show when={props.context.parent}>
          {(parent) => (
            <ContextMiniChart
              caption={props.onParent ? translate('focus.toParent', 'To parent') : undefined}
              centerLabel={props.context.share !== undefined ? `${Math.round(props.context.share * 100)}%` : undefined}
              colorFor={props.colorFor}
              label={translate('focus.backToParent', 'Back to {name}', { name: parent().label })}
              onClick={props.onParent}
              parent={parent()}
            />
          )}
        </Show>
      </div>
    </div>
  )
}
