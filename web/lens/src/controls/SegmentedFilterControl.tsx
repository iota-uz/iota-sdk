/* eslint-disable react/no-unknown-property -- Solid JSX uses `class`, the React-era rule expects `className`; the lint config migrates with the Solid port. */
import { For, Show } from 'solid-js'
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
export function SegmentedFilterControl(props: { filter: Filter }) {
  const segmented = props.filter.segmented
  const { values, setSegmented } = useFilters()
  const translate = useTranslate()
  return (
    <Show when={segmented}>
      <div class="lens-segmented-filter" data-filter-id={props.filter.id} data-testid={`lens-filter-${props.filter.id}`}>
        <Show when={props.filter.label}>
          <span class="lens-segmented-label">{props.filter.label}</span>
        </Show>
        <div aria-label={props.filter.label || translate('filter.bar.label', 'Dashboard filters')} class="lens-filter lens-segmented-track" role="group">
          <For each={segmented!.options}>
            {(option) => (
              <button
                aria-pressed={option.value === currentSegmentedValue(segmented!, values)}
                class="lens-filter-chip"
                onClick={() => setSegmented(props.filter, option.value)}
                type="button"
              >
                {option.label}
              </button>
            )}
          </For>
        </div>
      </div>
    </Show>
  )
}
