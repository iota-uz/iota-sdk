import type { JSX } from 'solid-js'

export function ChatHeader(props: { title: string }): JSX.Element {
  return (
    <header class="sticky top-0 z-10 shrink-0 border-b border-neutral-200 bg-white/90 backdrop-blur">
      <div class="flex items-center px-4 py-3">
        <h1 class="truncate text-base font-semibold text-neutral-900">{props.title}</h1>
      </div>
    </header>
  )
}
