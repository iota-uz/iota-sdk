import type { JSX } from 'solid-js'

export function DateGroupHeader(props: { name: string; count: number }): JSX.Element {
  return (
    <div class="sticky top-0 z-10 border-b border-neutral-100 bg-white/95 px-3 py-2 backdrop-blur-sm">
      <div class="flex items-center justify-between">
        <span class="text-xs font-semibold tracking-wide text-neutral-500 uppercase">{props.name}</span>
        <span class="rounded-md bg-neutral-100 px-1.5 py-0.5 text-[10px] font-medium tabular-nums text-neutral-400">
          {props.count}
        </span>
      </div>
    </div>
  )
}
