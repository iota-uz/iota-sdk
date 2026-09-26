import { createSignal, Show, type JSX } from 'solid-js'
import { useI18n } from '../../i18n/i18n'
import { DotsVerticalIcon, PinIcon } from '../../ui/icons'
import { EditableText } from '../../ui/primitives'
import type { ChatSession } from '../../utils/sessionGrouping'
import { SessionMenu } from './SessionMenu'

const UUID_LIKE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

function isPlaceholderTitle(title: string | null | undefined): boolean {
  return !title || UUID_LIKE.test(title)
}

export function SessionItem(props: {
  session: ChatSession
  active: boolean
  onSelect: () => void
  onPin: () => void
  onRename: (title: string) => void
  onArchive: () => void
  menuOpen: boolean
  onMenuToggle: () => void
}): JSX.Element {
  const { t } = useI18n()
  const [renaming, setRenaming] = createSignal(false)

  const displayTitle = () => {
    const title = props.session.title
    return isPlaceholderTitle(title) ? t('sidebar.generatingTitle') : (title ?? '')
  }

  return (
    <div
      role="button"
      tabindex={0}
      data-session-id={props.session.id}
      class={`group relative block cursor-pointer rounded-xl px-3 py-2.5 transition-colors duration-200 ${
        props.active
          ? 'bg-primary-50 text-primary-700'
          : 'text-neutral-700 hover:bg-neutral-100'
      }`}
      onClick={() => props.onSelect()}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault()
          props.onSelect()
        }
      }}
    >
      {props.active && (
        <div class="absolute top-1/2 left-0 h-6 w-1 -translate-y-1/2 rounded-full bg-primary-500" />
      )}
      <div class="flex items-center justify-between gap-2">
        <div class="min-w-0 flex-1" onClick={(event) => event.stopPropagation()}>
          <EditableText
            value={displayTitle()}
            editing={renaming()}
            onEditStart={() => setRenaming(true)}
            onCommit={(title) => {
              setRenaming(false)
              props.onRename(title)
            }}
            onCancel={() => setRenaming(false)}
            ariaLabel={t('sidebar.rename')}
            class="text-sm"
          />
        </div>
        <Show when={props.session.pinned}>
          <PinIcon class="h-3.5 w-3.5 shrink-0 text-primary-500" />
        </Show>
        <button
          type="button"
          class="shrink-0 rounded-md p-1 text-neutral-400 opacity-0 transition-all duration-150 hover:bg-neutral-200 hover:text-neutral-600 focus-visible:opacity-100 group-hover:opacity-100"
          aria-label={t('sidebar.sessionMenu')}
          aria-haspopup="menu"
          aria-expanded={props.menuOpen}
          onClick={(event) => {
            event.stopPropagation()
            props.onMenuToggle()
          }}
        >
          <DotsVerticalIcon class="h-4 w-4" />
        </button>
      </div>
      <Show when={props.menuOpen}>
        <SessionMenu
          pinned={props.session.pinned ?? false}
          onPin={props.onPin}
          onRename={() => setRenaming(true)}
          onArchive={props.onArchive}
          onClose={() => props.onMenuToggle()}
        />
      </Show>
    </div>
  )
}
