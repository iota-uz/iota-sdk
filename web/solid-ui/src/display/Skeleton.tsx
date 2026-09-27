import { For, splitProps, type JSX } from 'solid-js'
import { classes } from '../internal/classes'

export type SkeletonVariant = 'lines' | 'text' | 'card' | 'table'

export interface SkeletonProps extends JSX.HTMLAttributes<HTMLDivElement> {
  variant?: SkeletonVariant
  skeletonClass?: string
  lines?: number
}

export function Skeleton(props: SkeletonProps) {
  const [local, native] = splitProps(props, ['class', 'variant', 'skeletonClass', 'lines'])
  const variant = () => local.variant ?? 'lines'
  const count = () => local.lines && local.lines > 0 ? local.lines : variant() === 'text' ? 2 : variant() === 'table' ? 5 : 3

  return (
    <div {...native} class={classes('animate-pulse', variant() === 'text' && 'space-y-2.5', local.class)} aria-hidden="true">
      {variant() === 'card' ? (
        <div class="flex items-center space-x-3">
          <svg class="w-10 h-10 text-gray-200" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 20 20">
            <path d="M10 0a10 10 0 1 0 10 10A10.011 10.011 0 0 0 10 0Zm0 5a3 3 0 1 1 0 6 3 3 0 0 1 0-6Zm0 13a8.949 8.949 0 0 1-4.951-1.488A3.987 3.987 0 0 1 9 13h2a3.987 3.987 0 0 1 3.951 3.512A8.949 8.949 0 0 1 10 18Z" />
          </svg>
          <div class="flex-1">
            <div class={classes('h-2.5 bg-gray-200 rounded-full w-32 mb-2', local.skeletonClass)} />
            <div class={classes('h-2 bg-gray-200 rounded-full w-48', local.skeletonClass)} />
          </div>
        </div>
      ) : variant() === 'table' ? (
        <For each={Array.from({ length: count() })}>{() => (
          <div class="flex items-center space-x-4 mb-4 last:mb-0">
            <div class={classes('h-2.5 bg-gray-200 rounded w-16', local.skeletonClass)} />
            <div class={classes('h-2.5 bg-gray-200 rounded flex-1', local.skeletonClass)} />
            <div class={classes('h-2.5 bg-gray-200 rounded w-20', local.skeletonClass)} />
            <div class={classes('h-2.5 bg-gray-200 rounded w-12', local.skeletonClass)} />
          </div>
        )}</For>
      ) : (
        <For each={Array.from({ length: count() })}>{(_, index) => (
          <div class={classes(
            variant() === 'text' ? 'h-2 bg-gray-200 rounded' : 'h-2.5 bg-gray-200 rounded-full mb-2.5 last:mb-0',
            variant() === 'text' && (index() === count() - 1 ? 'w-3/4' : 'w-full'),
            local.skeletonClass,
          )} />
        )}</For>
      )}
    </div>
  )
}

export type SkeletonPresetProps = Omit<SkeletonProps, 'variant'>
export const SkeletonText = (props: SkeletonPresetProps) => <Skeleton {...props} variant="text" />
export const SkeletonCard = (props: SkeletonPresetProps) => <Skeleton {...props} variant="card" />
export const SkeletonTable = (props: SkeletonPresetProps) => <Skeleton {...props} variant="table" />

