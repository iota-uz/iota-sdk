import { createSignal, For, type Accessor, type JSX } from 'solid-js'
import { useI18n } from '../../i18n/i18n'
import type { ChatSession, SessionGroup } from '../../utils/sessionGrouping'
import { DateGroupHeader } from './DateGroupHeader'
import { SessionItem } from './SessionItem'

const GROUP_I18N_KEYS: Record<string, string> = {
  Pinned: 'sidebar.pinned',
  Today: 'sidebar.today',
  Yesterday: 'sidebar.yesterday',
  'Previous 7 Days': 'sidebar.previous7Days',
  'Previous 30 Days': 'sidebar.previous30Days',
  Older: 'sidebar.older',
}

export function SessionList(props: {
  groups: Accessor<SessionGroup[]>
  activeId?: () => string | undefined
  onSelect: (id: string) => void
  onPin: (session: ChatSession) => void
  onRename: (session: ChatSession, title: string) => void
  onArchive: (session: ChatSession) => void
}): JSX.Element {
  const { t } = useI18n()
  const [openMenuId, setOpenMenuId] = createSignal<string | null>(null)

  const groupName = (name: string): string => {
    const key = GROUP_I18N_KEYS[name]
    return key ? t(key) : name
  }

  return (
    <For each={props.groups()}>
      {(group) => (
        <div class="mb-4">
          <DateGroupHeader name={groupName(group.name)} count={group.sessions.length} />
          <div role="list" aria-label={groupName(group.name)} class="mt-2 space-y-1">
            <For each={group.sessions}>
              {(session) => (
                <div role="listitem">
                  <SessionItem
                    session={session}
                    active={props.activeId?.() === session.id}
                    onSelect={() => props.onSelect(session.id)}
                    onPin={() => props.onPin(session)}
                    onRename={(title) => props.onRename(session, title)}
                    onArchive={() => props.onArchive(session)}
                    menuOpen={openMenuId() === session.id}
                    onMenuToggle={() =>
                      setOpenMenuId((current) => (current === session.id ? null : session.id))
                    }
                  />
                </div>
              )}
            </For>
          </div>
        </div>
      )}
    </For>
  )
}
