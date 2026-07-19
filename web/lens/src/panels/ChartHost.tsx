import { useEffect, useRef, useState } from 'react'
import type { NodeKey } from '../contract'
import type { ChartAdapter, ChartEvents, ChartInput, ChartInstance } from '../charts/adapter'

export interface ChartHostProps {
  input: ChartInput
  onSelect?: (key: NodeKey) => void
  onHover?: (key: NodeKey | null) => void
  adapter?: ChartAdapter
  label?: string
  drillable?: boolean
}

export function ChartHost({ input, onSelect, onHover, adapter, label, drillable = false }: ChartHostProps) {
  const hostRef = useRef<HTMLDivElement>(null)
  const instanceRef = useRef<ChartInstance>()
  const inputRef = useRef(input)
  const eventsRef = useRef({ onSelect, onHover })
  const [loadError, setLoadError] = useState<Error>()
  inputRef.current = input
  eventsRef.current = { onSelect, onHover }

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
        instanceRef.current = resolved.mount(hostRef.current, inputRef.current, events)
      })
      .catch((cause: unknown) => {
        if (active) setLoadError(cause instanceof Error ? cause : new Error('chart adapter failed to load'))
      })

    return () => {
      active = false
      instanceRef.current?.dispose()
      instanceRef.current = undefined
    }
  }, [adapter])

  useEffect(() => {
    instanceRef.current?.update(input)
  }, [input])

  return (
    <div
      className={`lens-chart-host${drillable ? ' lens-chart-host-drillable' : ''}`}
      aria-label={label}
      data-drillable={drillable || undefined}
    >
      <div ref={hostRef} className="lens-chart-canvas" />
      {loadError && <div className="lens-chart-load-error" role="alert">Unable to render chart.</div>}
    </div>
  )
}
