import { splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type BadgeVariant = 'pink' | 'yellow' | 'green' | 'blue' | 'purple' | 'gray'
export type BadgeSize = 'normal' | 'lg'

export interface BadgeProps extends JSX.HTMLAttributes<HTMLDivElement> {
  variant?: BadgeVariant
  size?: BadgeSize
}

const variants: Record<BadgeVariant, string> = {
  pink: 'border-pink bg-badge-pink text-pink',
  yellow: 'border-yellow bg-badge-yellow text-yellow',
  green: 'border-green bg-badge-green text-green',
  blue: 'border-blue bg-badge-blue text-blue',
  purple: 'border-purple bg-badge-purple text-purple',
  gray: 'border-subtle bg-badge-gray text-200',
}

export function Badge(props: BadgeProps) {
  const [local, native] = splitProps(props, ['children', 'class', 'variant', 'size'])
  return (
    <div {...native} class={classes('flex items-center justify-center rounded-lg text-sm font-medium border', variants[local.variant ?? 'pink'], local.size === 'lg' ? 'h-9' : 'h-8', local.class)}>
      {local.children}
    </div>
  )
}

