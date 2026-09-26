/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { createEffect, createSignal, onCleanup, onMount, Show } from 'solid-js'
import type { NodeKey } from '../contract'
import type { ChartActivation, ChartAdapter, ChartAnchor, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'
import { useTranslate } from '../runtime'

export interface ChartHostProps {
  input: ChartInput
  panelId?: string
  onSelect?: (key: NodeKey, anchor?: ChartAnchor, activation?: ChartActivation) => void
  onHover?: (key: NodeKey | null) => void
  adapter?: ChartAdapter
  label?: string
  drillable?: boolean
  resetZoomKey?: number
}

export function ChartHost(props: ChartHostProps) {
  let hostRef: HTMLDivElement | undefined
  let instance: ChartInstance | undefined
  const [loadError, setLoadError] = createSignal<Error>()
  const translate = useTranslate()

  const reportError = (cause: unknown, fallback: string): Error => {
    const error = cause instanceof Error ? cause : new Error(fallback)
    console.error(`[lens] chart panel ${props.panelId ?? '(unknown)'} failed to render`, error)
    return error
  }

  // Only the adapter drives remount; inputs and handlers are read through the
  // reactive props so a new frame updates in place instead of tearing down the
  // chart.
  onMount(() => {
    const events: ChartEvents = {
      onSelect: (key, anchor, activation) => props.onSelect?.(key, anchor, activation),
      onHover: (key) => props.onHover?.(key),
    }

    void (props.adapter ? Promise.resolve(props.adapter) : import('../charts').then(({ getChartAdapter }) => getChartAdapter(props.input.kind)))
      .then((resolved) => {
        if (!hostRef) return
        setLoadError(undefined)
        try {
          instance = resolved.mount(hostRef, props.input, events)
        } catch (cause: unknown) {
          setLoadError(reportError(cause, 'chart failed to render'))
        }
      })
      .catch((cause: unknown) => {
        setLoadError(reportError(cause, 'chart adapter failed to load'))
      })

    onCleanup(() => {
      instance?.dispose()
      instance = undefined
    })
  })

  createEffect(() => {
    const input = props.input
    if (!instance) return
    try {
      instance.update(input)
    } catch (cause: unknown) {
      setLoadError(reportError(cause, 'chart failed to update'))
    }
  })

  createEffect(() => {
    if ((props.resetZoomKey ?? 0) > 0) instance?.resetZoom?.()
  })

  return (
    <div
      class={`lens-chart-host${props.drillable ? ' lens-chart-host-drillable' : ''}`}
      aria-label={props.label}
      role="img"
      data-drillable={props.drillable || undefined}
    >
      <div ref={(el) => { hostRef = el }} class="lens-chart-canvas" />
      {loadError() && (
        <div class="lens-chart-load-error" role="alert">
          <span class="lens-chart-load-error-message">{translate('chart.error', 'Unable to render chart.')}</span>
          <Show when={loadError()?.message}>
            <span class="lens-chart-load-error-detail">{loadError()!.message}</span>
          </Show>
        </div>
      )}
    </div>
  )
}
