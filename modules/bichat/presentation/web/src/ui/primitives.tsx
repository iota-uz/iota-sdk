import { createEffect, createSignal, For, Show, type JSX } from 'solid-js'
import { useI18n } from '../i18n/i18n'

export function Skeleton(props: { class?: string }): JSX.Element {
  return <div class={`animate-pulse rounded bg-neutral-200 ${props.class ?? ''}`} aria-hidden="true" />
}

export function ListItemSkeleton(): JSX.Element {
  return (
    <div class="flex items-center gap-3 px-3 py-2.5">
      <Skeleton class="h-8 w-8 shrink-0 rounded-full" />
      <div class="flex min-w-0 flex-1 flex-col gap-2">
        <Skeleton class="h-3 w-3/4" />
        <Skeleton class="h-3 w-1/2" />
      </div>
    </div>
  )
}

export function SkeletonGroup(props: { count: number }): JSX.Element {
  return (
    <div class="flex flex-col gap-1">
      <For each={Array.from({ length: props.count })}>{() => <ListItemSkeleton />}</For>
    </div>
  )
}

export function EditableText(props: {
  value: string
  onCommit: (value: string) => void
  onCancel?: () => void
  class?: string
  ariaLabel?: string
  editing?: boolean
  onEditStart?: () => void
}): JSX.Element {
  const [internalEditing, setInternalEditing] = createSignal(false)
  const editing = () => props.editing ?? internalEditing()
  let inputRef: HTMLInputElement | undefined

  createEffect(() => {
    if (editing()) {
      inputRef?.focus()
      inputRef?.select()
    }
  })

  const startEditing = () => {
    if (props.editing !== undefined) props.onEditStart?.()
    else setInternalEditing(true)
  }

  const commit = () => {
    if (!editing()) return
    const next = inputRef?.value.trim() ?? ''
    setInternalEditing(false)
    if (next.length > 0) props.onCommit(next)
    else props.onCancel?.()
  }

  const cancel = () => {
    if (!editing()) return
    setInternalEditing(false)
    props.onCancel?.()
  }

  return (
    <Show
      when={editing()}
      fallback={
        <button
          type="button"
          class={`cursor-text truncate rounded text-left ${props.class ?? ''}`}
          title={props.value}
          aria-label={props.ariaLabel}
          onClick={(event) => {
            event.stopPropagation()
            startEditing()
          }}
        >
          {props.value}
        </button>
      }
    >
      <input
        ref={inputRef}
        type="text"
        class={`w-full rounded border border-primary-400 bg-white px-1.5 py-0.5 text-sm text-neutral-900 outline-none focus:ring-2 focus:ring-primary-200 ${props.class ?? ''}`}
        value={props.value}
        aria-label={props.ariaLabel}
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            event.preventDefault()
            commit()
          } else if (event.key === 'Escape') {
            event.preventDefault()
            cancel()
          }
        }}
        onBlur={commit}
      />
    </Show>
  )
}

export function SkipLink(): JSX.Element {
  const { t } = useI18n()
  return (
    <a
      href="#bichat-main"
      class="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-[60] focus:rounded-lg focus:bg-neutral-900 focus:px-3 focus:py-2 focus:text-sm focus:font-medium focus:text-white focus:shadow-lg"
    >
      {t('nav.skipToChat')}
    </a>
  )
}
