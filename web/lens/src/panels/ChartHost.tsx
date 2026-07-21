import { useCallback, useEffect, useRef, useState } from 'react'
import type { NodeKey } from '../contract'
import type { ChartAdapter, ChartAnchor, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'
import { useTranslate } from '../runtime'

export interface ChartHostProps {
  input: ChartInput
  panelId?: string
  onSelect?: (key: NodeKey, anchor?: ChartAnchor) => void
  onHover?: (key: NodeKey | null) => void
  adapter?: ChartAdapter
  label?: string
  drillable?: boolean
}

export function ChartHost({ input, panelId, onSelect, onHover, adapter, label, drillable = false }: ChartHostProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const instanceRef = useRef<ChartInstance>()
  const inputRef = useRef(input)
  const eventsRef = useRef({ onSelect, onHover })
  const [loadError, setLoadError] = useState<Error>()
  const translate = useTranslate()
  inputRef.current = input
  eventsRef.current = { onSelect, onHover }

  const reportError = useCallback((cause: unknown, fallback: string): Error => {
    const error = cause instanceof Error ? cause : new Error(fallback)
    console.error(`[lens] chart panel ${panelId ?? '(unknown)'} failed to render`, error)
    return error
  }, [panelId])

  useEffect(() => {
    let active = true
    const events: ChartEvents = {
      onSelect: (key, anchor) => eventsRef.current.onSelect?.(key, anchor),
      onHover: (key) => eventsRef.current.onHover?.(key),
    }

    void (adapter ? Promise.resolve(adapter) : import('../charts').then(({ getChartAdapter }) => getChartAdapter()))
      .then((resolved) => {
        if (!active || !hostRef.current) return
        setLoadError(undefined)
        try {
          instanceRef.current = resolved.mount(hostRef.current, inputRef.current, events)
        } catch (cause: unknown) {
          setLoadError(reportError(cause, 'chart failed to render'))
        }
      })
      .catch((cause: unknown) => {
        if (active) setLoadError(reportError(cause, 'chart adapter failed to load'))
      })

    return () => {
      active = false
      instanceRef.current?.dispose()
      instanceRef.current = undefined
    }
    // Only the adapter drives remount; inputs and handlers are read through
    // refs so a new frame updates in place instead of tearing down the chart.
  }, [adapter, reportError])

  useEffect(() => {
    if (!instanceRef.current) return
    try {
      instanceRef.current.update(input)
    } catch (cause: unknown) {
      setLoadError(reportError(cause, 'chart failed to update'))
    }
  }, [input, reportError])

  return (
    <div
      className={`lens-chart-host${drillable ? ' lens-chart-host-drillable' : ''}`}
      aria-label={label}
      data-drillable={drillable || undefined}
      onMouseDown={drillable ? (event) => event.currentTarget.focus() : undefined}
      tabIndex={drillable ? 0 : undefined}
    >
      <div ref={hostRef} className="lens-chart-canvas" />
      {loadError && (
        <div className="lens-chart-load-error" role="alert">
          <span className="lens-chart-load-error-message">{translate('chart.error', 'Unable to render chart.')}</span>
          {loadError.message && <span className="lens-chart-load-error-detail">{loadError.message}</span>}
        </div>
      )}
    </div>
  )
}
