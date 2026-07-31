import { useEffect, useState } from 'react'
import type { CompareMode, Filter } from '../contract'
import { useFilters, useTranslate } from '../runtime'

export function CompareFilterControl({ filter }: { filter: Filter }) {
  const comparison = filter.compare
  const { setCompare } = useFilters()
  const translate = useTranslate()
  const serverMode = comparison?.value.mode ?? 'off'
  const [mode, setMode] = useState<CompareMode>(serverMode)
  const [start, setStart] = useState(comparison?.value.start ?? '')
  const [end, setEnd] = useState(comparison?.value.end ?? '')
  useEffect(() => setMode(serverMode), [serverMode])
  if (!comparison) return null
  const select = (mode: CompareMode) => {
    setMode(mode)
    if (mode !== 'custom') setCompare(filter, { mode })
  }
  return (
    <div className="lens-compare-filter">
      <label>
        <span>{filter.label}</span>
        <select onChange={(event) => select(event.currentTarget.value as CompareMode)} value={mode}>
          <option value="off">{translate('filter.compare.off', 'Comparison off')}</option>
          <option value="previous_period">{translate('filter.compare.previous', 'Previous period')}</option>
          <option value="year_ago">{translate('filter.compare.yearAgo', 'Year ago')}</option>
          <option value="custom">{translate('filter.compare.custom', 'Custom interval')}</option>
        </select>
      </label>
      {mode === 'custom' && (
        <span className="lens-compare-custom">
          <input aria-label={translate('filter.compare.start', 'Comparison start')} onChange={(event) => setStart(event.currentTarget.value)} type="date" value={start} />
          <input aria-label={translate('filter.compare.end', 'Comparison end')} onChange={(event) => setEnd(event.currentTarget.value)} type="date" value={end} />
          <button disabled={!start || !end || end < start} onClick={() => setCompare(filter, { mode: 'custom', start, end })} type="button">
            {translate('filter.period.apply', 'Apply')}
          </button>
        </span>
      )}
    </div>
  )
}
