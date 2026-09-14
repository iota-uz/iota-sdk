import { For, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type AvatarVariant = 'round' | 'square'

const avatarColors = ['#695EFF', '#1CD6A5', '#544BCC', '#88CDFF', '#4A42B2', '#94A3B8', '#F6AE42', '#131313'] as const

export function avatarColor(initials: string): string {
  let hash = 0
  for (const byte of new TextEncoder().encode(initials)) hash = byte + ((hash * 32) - hash)
  return avatarColors[((hash % avatarColors.length) + avatarColors.length) % avatarColors.length]!
}

export interface AvatarProps extends JSX.HTMLAttributes<HTMLDivElement> {
  imageUrl?: string
  initials: string
  variant?: AvatarVariant
  imageAlt?: string
}

export function Avatar(props: AvatarProps) {
  const [local, native] = splitProps(props, ['imageUrl', 'initials', 'variant', 'imageAlt', 'class', 'children', 'style'])
  const shape = () => local.variant === 'square' ? 'rounded-lg' : 'rounded-full'
  const style = () => typeof local.style === 'string' ? `background-color:${avatarColor(local.initials)};${local.style}` : { 'background-color': avatarColor(local.initials), ...local.style }
  return (
    <div {...native} class={classes('w-9 h-9 font-medium flex items-center justify-center cursor-pointer text-white', shape(), local.class)} style={style()}>
      {local.imageUrl ? <img src={local.imageUrl} alt={local.imageAlt ?? 'Avatar'} class={classes('w-9 h-9 object-cover', shape())} /> : local.children ?? local.initials}
    </div>
  )
}

export interface AvatarGroupItem extends Omit<AvatarProps, 'class' | 'children'> {}
export interface AvatarGroupProps extends JSX.HTMLAttributes<HTMLDivElement> {
  avatars?: readonly AvatarGroupItem[]
  limit?: number
  overflowLabel?: (count: number) => string
}

export function AvatarGroup(props: AvatarGroupProps) {
  const [local, native] = splitProps(props, ['avatars', 'limit', 'overflowLabel', 'class', 'children'])
  const shown = () => (local.avatars ?? []).slice(0, local.limit ?? Number.POSITIVE_INFINITY)
  const overflow = () => Math.max(0, (local.avatars?.length ?? 0) - shown().length)
  return (
    <div {...native} role={native.role ?? 'group'} class={classes('flex -space-x-2', local.class)}>
      <For each={shown()}>{(avatar) => <Avatar {...avatar} class="ring-2 ring-surface-300" />}</For>
      {local.children}
      {overflow() > 0 && <span class="w-9 h-9 font-medium flex items-center justify-center rounded-full bg-surface-500 text-200 ring-2 ring-surface-300" aria-label={local.overflowLabel?.(overflow()) ?? `${overflow()} more`}>+{overflow()}</span>}
    </div>
  )
}
