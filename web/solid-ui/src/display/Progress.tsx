import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface ProgressProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'children'> {
  value: number
  target: number
  valueLabel?: JSX.Element
  targetLabel?: JSX.Element
  /**
   * Accessible name ("Collected premiums"). With it the bar is a
   * role="progressbar" whose value is the visible percentage (0-100, clamped
   * like the fill) with "value / target" as text; without it the bar is purely
   * visual, as the templ Progress.
   */
  label?: string
}

export function Progress(props: ProgressProps) {
  const [local, native] = splitProps(props, ['class', 'value', 'target', 'valueLabel', 'targetLabel', 'label'])
  const percentage = () => local.target > 0 ? Math.min(Math.max(local.value / local.target * 100, 0), 100) : 0
  return (
    <div {...native} class={classes('flex gap-2 items-center text-sm font-medium', local.class)}>
      <div class="bg-surface-100 flex-1 relative rounded-md">
        <div
          class="bg-success text-on-success text-right rounded-md p-1"
          style={{ width: `${percentage()}%` }}
          role={local.label ? 'progressbar' : undefined}
          aria-label={local.label || undefined}
          aria-valuemin={local.label ? 0 : undefined}
          aria-valuemax={local.label ? 100 : undefined}
          aria-valuenow={local.label ? Math.round(percentage()) : undefined}
          aria-valuetext={local.label ? `${local.value} / ${local.target}` : undefined}
        >
          {local.valueLabel ?? local.value}
        </div>
      </div>
      <span class="shrink-0">{local.targetLabel ?? local.target}</span>
    </div>
  )
}

