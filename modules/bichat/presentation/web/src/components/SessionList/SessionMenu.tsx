import { onCleanup, onMount, type JSX } from 'solid-js'
import { useI18n } from '../../i18n/i18n'
import { ArchiveIcon, PencilIcon, PinIcon } from '../../ui/icons'

export function SessionMenu(props: {
  pinned: boolean
  onPin: () => void
  onRename: () => void
  onArchive: () => void
  onClose: () => void
}): JSX.Element {
  const { t } = useI18n()
  let rootRef: HTMLDivElement | undefined

  const itemClass =
    'flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-[13px] text-neutral-600 transition-colors hover:bg-neutral-100 hover:text-neutral-900'

  onMount(() => {
    const onPointerDown = (event: PointerEvent) => {
      if (rootRef && !rootRef.contains(event.target as Node)) props.onClose()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') props.onClose()
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    onCleanup(() => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    })
  })

  return (
    <div
      ref={rootRef}
      role="menu"
      class="absolute top-full right-0 z-20 mt-1 w-44 rounded-xl border border-neutral-200 bg-white py-1 shadow-lg"
    >
      <button
        type="button"
        role="menuitem"
        class={itemClass}
        onClick={(event) => {
          event.stopPropagation()
          props.onPin()
          props.onClose()
        }}
      >
        <PinIcon class="h-4 w-4 text-neutral-400" />
        {props.pinned ? t('sidebar.unpin') : t('sidebar.pin')}
      </button>
      <button
        type="button"
        role="menuitem"
        class={itemClass}
        onClick={(event) => {
          event.stopPropagation()
          props.onRename()
          props.onClose()
        }}
      >
        <PencilIcon class="h-4 w-4 text-neutral-400" />
        {t('sidebar.rename')}
      </button>
      <button
        type="button"
        role="menuitem"
        class={itemClass}
        onClick={(event) => {
          event.stopPropagation()
          props.onArchive()
          props.onClose()
        }}
      >
        <ArchiveIcon class="h-4 w-4 text-neutral-400" />
        {t('sidebar.archive')}
      </button>
    </div>
  )
}
