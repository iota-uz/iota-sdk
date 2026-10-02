import { useLocation, useNavigate } from '@solidjs/router'
import Fuse from 'fuse.js'
import { createEffect, createMemo, createSignal, onCleanup, onMount, Show, type JSX } from 'solid-js'
import { useAppContext } from '../context/appContext'
import { useSessionEvents } from '../contexts/SessionEventContext'
import { useI18n } from '../i18n/i18n'
import { ArchiveIcon, PanelLeftIcon, PlusIcon, SearchIcon } from '../ui/icons'
import { useAppToast } from '../ui/toast'
import { createBichatRPCClient } from '../rpc/client'
import { groupSessionsByDate, type ChatSession } from '../utils/sessionGrouping'
import { toRPCErrorDisplay, type RPCErrorDisplay } from '../utils/rpcErrors'
import { SessionList } from './SessionList/SessionList'
import { SessionSkeleton } from './SessionSkeleton'

const STORAGE_KEY = 'bichat-sidebar-collapsed'
const LIST_LIMIT = 200
const POLL_INTERVAL_MS = 2000
const MAX_POLLS = 5
const UUID_LIKE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

type Tab = 'my' | 'all'

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

function writeCollapsed(value: boolean): void {
  try {
    localStorage.setItem(STORAGE_KEY, String(value))
  } catch {
    // storage unavailable (private mode) — collapse state stays in-memory
  }
}

function hasPlaceholderTitle(session: ChatSession): boolean {
  return !session.title || UUID_LIKE.test(session.title)
}

export function Sidebar(props: { onNavigate?: () => void }): JSX.Element {
  const { t } = useI18n()
  const navigate = useNavigate()
  const location = useLocation()
  const app = useAppContext()
  const toast = useAppToast()
  const events = useSessionEvents()
  const client = createBichatRPCClient(app)

  const [collapsed, setCollapsed] = createSignal(readCollapsed())
  const [tab, setTab] = createSignal<Tab>('my')
  const [query, setQuery] = createSignal('')
  const [loading, setLoading] = createSignal(true)
  const [sessions, setSessions] = createSignal<ChatSession[]>([])
  const [error, setError] = createSignal<RPCErrorDisplay | null>(null)
  let searchInput: HTMLInputElement | undefined
  let pollTimer: ReturnType<typeof setInterval> | undefined
  let loadToken = 0

  const setCollapsedPersisted = (value: boolean) => {
    setCollapsed(value)
    writeCollapsed(value)
  }

  const canReadAll = createMemo(() => {
    const permissions = app.user.permissions ?? []
    return permissions.includes('BiChat.ReadAll') || permissions.includes('AIChat.ReadAll')
  })

  const load = async (): Promise<void> => {
    const token = ++loadToken
    setLoading(true)
    try {
      const data =
        tab() === 'all'
          ? await client.call('bichat.session.listAll', {
              limit: LIST_LIMIT,
              offset: 0,
              includeArchived: false,
            })
          : await client.call('bichat.session.list', {
              limit: LIST_LIMIT,
              offset: 0,
              includeArchived: false,
            })
      if (token !== loadToken) return
      setSessions(data.sessions)
      setError(null)
    } catch (cause) {
      if (token !== loadToken) return
      setSessions([])
      setError(toRPCErrorDisplay(cause, 'Failed to load sessions'))
    } finally {
      if (token === loadToken) setLoading(false)
    }
  }

  createEffect(() => {
    void load()
  })

  const stopPlaceholderPolling = () => {
    if (pollTimer !== undefined) {
      clearInterval(pollTimer)
      pollTimer = undefined
    }
  }

  const startPlaceholderPolling = () => {
    if (pollTimer !== undefined) return
    let polls = 0
    pollTimer = setInterval(() => {
      polls += 1
      if (polls >= MAX_POLLS || !sessions().some(hasPlaceholderTitle)) {
        stopPlaceholderPolling()
        return
      }
      void load()
    }, POLL_INTERVAL_MS)
  }

  onMount(() => {
    const unsubscribe = events.onSessionCreated(() => {
      void load()
      startPlaceholderPolling()
    })
    onCleanup(unsubscribe)
  })

  onCleanup(stopPlaceholderPolling)

  createEffect(() => {
    if (tab() === 'all' && !canReadAll()) setTab('my')
  })

  onMount(() => {
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ expanded?: boolean }>).detail
      if (detail?.expanded && window.innerWidth < 768) setCollapsedPersisted(true)
    }
    window.addEventListener('bichat:artifacts-panel-expanded', handler)
    onCleanup(() => window.removeEventListener('bichat:artifacts-panel-expanded', handler))
  })

  onMount(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey)) return
      if (event.key === 'b' || event.key === 'B') {
        event.preventDefault()
        setCollapsedPersisted(!collapsed())
      } else if (event.key === 'k' || event.key === 'K') {
        event.preventDefault()
        if (collapsed()) setCollapsedPersisted(false)
        requestAnimationFrame(() => searchInput?.focus())
      }
    }
    document.addEventListener('keydown', onKeyDown)
    onCleanup(() => document.removeEventListener('keydown', onKeyDown))
  })

  const fuse = createMemo(() => new Fuse(sessions(), { keys: ['title'], threshold: 0.3 }))

  const filtered = createMemo(() => {
    const trimmed = query().trim()
    if (!trimmed) return sessions()
    return fuse()
      .search(trimmed)
      .map((result) => result.item)
  })

  const groups = createMemo(() => {
    const pinned = filtered().filter((session) => session.pinned)
    const dated = groupSessionsByDate(filtered().filter((session) => !session.pinned))
    if (pinned.length === 0) return dated
    return [{ name: 'Pinned', sessions: pinned }, ...dated]
  })

  const activeId = createMemo(() => location.pathname.match(/\/session\/([^/]+)/)?.[1])
  const isArchivedView = createMemo(() => location.pathname === '/archived')

  const openSession = (session: ChatSession) => {
    navigate(`/session/${session.id}`)
    props.onNavigate?.()
  }

  const newChat = () => {
    navigate('/')
    props.onNavigate?.()
  }

  const togglePin = async (session: ChatSession): Promise<void> => {
    try {
      if (session.pinned) {
        await client.call('bichat.session.unpin', { id: session.id })
        toast.success(t('sidebar.unpin'))
      } else {
        await client.call('bichat.session.pin', { id: session.id })
        toast.success(t('sidebar.pin'))
      }
      await load()
    } catch (cause) {
      toast.error(toRPCErrorDisplay(cause, 'Failed to update pin state').title)
    }
  }

  const renameSession = async (session: ChatSession, title: string): Promise<void> => {
    try {
      await client.call('bichat.session.updateTitle', { id: session.id, title })
      await load()
      toast.success(t('toast.sessionRenamed'))
    } catch (cause) {
      toast.error(toRPCErrorDisplay(cause, 'Failed to rename session').title)
    }
  }

  const archiveSession = async (session: ChatSession): Promise<void> => {
    try {
      await client.call('bichat.session.archive', { id: session.id })
      await load()
      toast.success(t('toast.sessionArchived'))
      if (activeId() === session.id) navigate('/')
    } catch (cause) {
      toast.error(toRPCErrorDisplay(cause, 'Failed to archive session').title)
    }
  }

  const footerButtonClass =
    'flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-xs font-medium text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-neutral-700'
  const tabButtonClass = (active: boolean) =>
    `cursor-pointer rounded-md px-2 py-1 text-xs font-medium transition-colors ${
      active ? 'bg-white text-neutral-900 shadow-sm' : 'text-neutral-500 hover:text-neutral-700'
    }`

  return (
    <aside
      role="navigation"
      aria-label={t('nav.chats')}
      class={`flex h-full flex-col overflow-hidden border-r border-neutral-200 bg-white transition-[width] duration-200 ease-in-out ${
        collapsed() ? 'w-14' : 'w-72'
      }`}
    >
      {/* Collapsed rail */}
      <Show when={collapsed()}>
        <div class="flex flex-col items-center gap-2 pt-3">
          <button
            type="button"
            onClick={newChat}
            title={t('nav.newChat')}
            aria-label={t('nav.newChat')}
            class="flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg bg-primary-600 text-white shadow-sm transition-colors hover:bg-primary-700"
          >
            <PlusIcon class="h-4 w-4" />
          </button>
        </div>
        <div class="mt-auto flex items-center justify-center border-t border-neutral-100 px-2 py-3">
          <button
            type="button"
            onClick={() => setCollapsedPersisted(false)}
            title={t('sidebar.expand')}
            aria-label={t('sidebar.expand')}
            class="flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-neutral-700"
          >
            <PanelLeftIcon class="h-4 w-4" />
          </button>
        </div>
      </Show>

      {/* Expanded */}
      <Show when={!collapsed()}>
        <div class="px-3 pt-3 pb-2">
          <button
            type="button"
            onClick={newChat}
            title={t('nav.newChat')}
            class="flex w-full cursor-pointer items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-2.5 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:-translate-y-0.5 hover:bg-primary-700"
          >
            <PlusIcon class="h-4 w-4" />
            <span>{t('nav.newChat')}</span>
          </button>
        </div>

        <div class="px-3 pb-2">
          <div class="relative">
            <SearchIcon class="pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2 text-neutral-400" />
            <input
              ref={searchInput}
              type="text"
              value={query()}
              onInput={(event) => setQuery(event.currentTarget.value)}
              placeholder={t('sidebar.search')}
              aria-label={t('sidebar.search')}
              class="w-full rounded-lg border border-neutral-200 bg-neutral-50 py-1.5 pr-2 pl-8 text-sm text-neutral-800 placeholder:text-neutral-400 focus:border-primary-400 focus:bg-white focus:ring-2 focus:ring-primary-100 focus:outline-none"
            />
          </div>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto px-2 pb-4">
          <Show when={!loading() || sessions().length > 0} fallback={<SessionSkeleton />}>
            <SessionList
              groups={groups}
              activeId={activeId}
              onSelect={(id) => {
                const session = filtered().find((item) => item.id === id)
                if (session) openSession(session)
              }}
              onPin={(session) => void togglePin(session)}
              onRename={(session, title) => void renameSession(session, title)}
              onArchive={(session) => void archiveSession(session)}
            />
          </Show>
          <Show when={!loading() && filtered().length === 0 && !error()}>
            <p class="px-3 py-6 text-center text-sm text-neutral-400">
              {query().trim() ? t('sidebar.emptySearch') : t('sidebar.empty')}
            </p>
          </Show>
          <Show when={error()}>
            {(display) => (
              <div
                class={`mx-2 mt-4 rounded-xl border p-3 ${
                  display().isPermissionDenied
                    ? 'border-amber-200 bg-amber-50'
                    : 'border-red-200 bg-red-50'
                }`}
              >
                <p
                  class={`text-xs font-medium ${
                    display().isPermissionDenied ? 'text-amber-700' : 'text-red-600'
                  }`}
                >
                  {display().title}
                </p>
                <p
                  class={`mt-1 text-xs ${
                    display().isPermissionDenied ? 'text-amber-600' : 'text-red-500'
                  }`}
                >
                  {display().description}
                </p>
              </div>
            )}
          </Show>
        </div>

        <div class="flex items-center justify-between gap-2 border-t border-neutral-100 px-3 py-2">
          <button
            type="button"
            onClick={() => {
              navigate('/archived')
              props.onNavigate?.()
            }}
            aria-current={isArchivedView() ? 'page' : undefined}
            class={footerButtonClass}
          >
            <ArchiveIcon class="h-4 w-4" />
            <span>{t('nav.archived')}</span>
          </button>
          <div class="flex items-center gap-1">
            <Show when={canReadAll()}>
              <div class="flex rounded-lg bg-neutral-100 p-0.5">
                <button type="button" onClick={() => setTab('my')} class={tabButtonClass(tab() === 'my')}>
                  {t('sidebar.myChats')}
                </button>
                <button type="button" onClick={() => setTab('all')} class={tabButtonClass(tab() === 'all')}>
                  {t('sidebar.allChats')}
                </button>
              </div>
            </Show>
            <button
              type="button"
              onClick={() => setCollapsedPersisted(true)}
              title={t('sidebar.collapse')}
              aria-label={t('sidebar.collapse')}
              class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-lg text-neutral-500 transition-colors hover:bg-neutral-100 hover:text-neutral-700"
            >
              <PanelLeftIcon class="h-4 w-4" />
            </button>
          </div>
        </div>
      </Show>
    </aside>
  )
}
