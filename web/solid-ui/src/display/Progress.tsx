import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export interface ProgressProps extends Omit<JSX.HTMLAttributes<HTMLDivElement>, 'children'> {
  value: number
  target: number
  valueLabel?: JSX.Element
  targetLabel?: JSX.Element
}

export function Progress(props: ProgressProps) {
  const [local, native] = splitProps(props, ['class', 'value', 'target', 'valueLabel', 'targetLabel'])
  const percentage = () => local.target > 0 ? Math.min(Math.max(local.value / local.target * 100, 0), 100) : 0
  return (
    <div {...native} class={classes('flex gap-2 items-center text-sm font-medium', local.class)}>
      <div class="bg-surface-100 flex-1 relative rounded-md">
        <div
          class="bg-success text-on-success text-right rounded-md p-1"
          style={{ width: `${percentage()}%` }}
          role="progressbar"
          aria-valuemin="0"
          aria-valuemax={local.target}
          aria-valuenow={local.value}
        >
          {local.valueLabel ?? local.value}
        </div>
      </div>
      <span class="shrink-0">{local.targetLabel ?? local.target}</span>
    </div>
  )
}

