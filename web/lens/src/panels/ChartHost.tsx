import { useEffect, useRef, useState } from 'react'
import type { NodeKey } from '../contract'
import type { ChartAdapter, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'
import { useTranslate } from '../runtime'

export interface ChartHostProps {
  input: ChartInput
  panelId?: string
  onSelect?: (key: NodeKey) => void
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

  const reportError = (cause: unknown, fallback: string): Error => {
    const error = cause instanceof Error ? cause : new Error(fallback)
    console.error(`[lens] chart panel ${panelId ?? '(unknown)'} failed to render`, error)
    return error
  }

  useEffect(() => {
    let active = true
    const events: ChartEvents = {
      onSelect: (key) => eventsRef.current.onSelect?.(key),
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
    // reportError is a stable closure over panelId; adapter drives remount.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [adapter])

  useEffect(() => {
    if (!instanceRef.current) return
    try {
      instanceRef.current.update(input)
    } catch (cause: unknown) {
      setLoadError(reportError(cause, 'chart failed to update'))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [input])

  return (
    <div
      className={`lens-chart-host${drillable ? ' lens-chart-host-drillable' : ''}`}
      aria-label={label}
      data-drillable={drillable || undefined}
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
