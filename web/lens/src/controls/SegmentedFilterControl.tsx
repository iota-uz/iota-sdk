import type { Filter } from '../contract'
import { useFilters, useTranslate } from '../runtime'
import { currentSegmentedValue } from '../runtime/filters'

/**
 * A labelled segmented control: one closed set of mutually exclusive values,
 * all of them visible, the applied one raised.
 *
 * It is deliberately built from the period control's tray (`.lens-filter`) and
 * its chips (`.lens-filter-chip`), not from a second lookalike of them. The
 * header already states "one of these is applied" in exactly that language —
 * a recessed track with a single raised member — and the runtime's control
 * vocabulary has room for four weights, not five. So the only thing this file
 * adds visually is the leading label, in the same small-caps voice the
 * comparison trigger uses for its own name.
 *
 * A tabstrip was the alternative, and it is the wrong object: tabs switch
 * between panels of one view, this switches the whole page's data. It is also
 * why the control is one of the three `--lens-control-h-*` heights rather than
 * the taller underlined strip a tabstrip wears.
 */
export function SegmentedFilterControl({ filter }: { filter: Filter }) {
  const segmented = filter.segmented
  const { values, setSegmented } = useFilters()
  const translate = useTranslate()
  if (!segmented) return null
  const label = filter.label || translate('filter.bar.label', 'Dashboard filters')
  const active = currentSegmentedValue(segmented, values)
  return (
    <div className="lens-segmented-filter" data-filter-id={filter.id} data-testid={`lens-filter-${filter.id}`}>
      {filter.label && <span className="lens-segmented-label">{filter.label}</span>}
      <div aria-label={label} className="lens-filter lens-segmented-track" role="group">
        {segmented.options.map((option) => (
          <button
            aria-pressed={option.value === active}
            className="lens-filter-chip"
            key={option.value}
            onClick={() => setSegmented(filter, option.value)}
            type="button"
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  )
}
