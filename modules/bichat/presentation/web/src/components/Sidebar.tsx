/**
 * Sidebar Component
 * Left sidebar with session list.
 * Used as a desktop sidebar and as a mobile drawer panel (Layout controls the overlay/drawer).
 *
 * Collapse UX matches the SDK sidebar pattern:
 * - Click empty space to toggle
 * - Cursor hints (e-resize / w-resize)
 * - localStorage persistence
 * - Keyboard shortcuts: Cmd+B toggle, Cmd+K focus search
 */

import React, { useState, useMemo, useEffect, useCallback, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react'
import { Plus, CaretLineLeft, CaretLineRight, Archive, Gear, Users, List } from '@phosphor-icons/react'
import { SearchInput, EmptyState, AllChatsList } from '@iota-uz/sdk/bichat'
import { createAppletRPCClient } from '@iota-uz/sdk'
import type { BichatRPC } from '@iota-uz/sdk/bichat'
import SessionList from './SessionList/SessionList'
import SessionSkeleton from './SessionSkeleton'
import { groupSessionsByDate } from '../utils/sessionGrouping'
import type { ChatSession } from '../utils/sessionGrouping'
import { useIotaContext } from '../contexts/IotaContext'
import { toRPCErrorDisplay, type RPCErrorDisplay } from '../utils/rpcErrors'
import { useBiChatDataSource } from '../data/bichatDataSource'
import { useAppToast } from '../contexts/ToastContext'
import { useSessionEvents } from '../contexts/SessionEventContext'

const STORAGE_KEY = 'bichat-sidebar-collapsed'

function useSidebarCollapse() {
  const [isCollapsed, setIsCollapsed] = useState(() => {
    try {
      return localStorage.getItem(STORAGE_KEY) === 'true'
    } catch {
      return false
    }
  })

  const isCollapsedRef = useRef(isCollapsed)
  useEffect(() => {
    isCollapsedRef.current = isCollapsed
  }, [isCollapsed])

  const toggle = useCallback(() => {
    setIsCollapsed((prev) => {
      const next = !prev
      try {
        localStorage.setItem(STORAGE_KEY, String(next))
      } catch { /* noop */ }
      return next
    })
  }, [])

  const expand = useCallback(() => {
    setIsCollapsed(false)
    try {
      localStorage.setItem(STORAGE_KEY, 'false')
    } catch { /* noop */ }
  }, [])

  const collapse = useCallback(() => {
    setIsCollapsed(true)
    try {
      localStorage.setItem(STORAGE_KEY, 'true')
    } catch { /* noop */ }
  }, [])

  return { isCollapsed, isCollapsedRef, toggle, expand, collapse }
}

type ActiveTab = 'my-chats' | 'all-chats'

interface SidebarProps {
  onNewChat: () => void
  creating?: boolean
  onClose?: () => void
}

// Simple search filter (case-insensitive title match)
function useSessionSearch(sessions: ChatSession[], query: string): ChatSession[] {
  return useMemo(() => {
    if (!query.trim()) return sessions
    const lowerQuery = query.toLowerCase()
    return sessions.filter((s) => s.title?.toLowerCase().includes(lowerQuery))
  }, [sessions, query])
}

export default function Sidebar({ onNewChat, creating, onClose }: SidebarProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const { config, user } = useIotaContext()
  const toast = useAppToast()
  const sessionEvents = useSessionEvents()
  const { isCollapsed, isCollapsedRef, toggle, expand, collapse } = useSidebarCollapse()
  const searchContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent<{ expanded: boolean }>).detail
      if (detail?.expanded) collapse()
    }
    window.addEventListener('bichat:artifacts-panel-expanded', handler)
    return () => window.removeEventListener('bichat:artifacts-panel-expanded', handler)
  }, [collapse])

  // Permission checks
  const canReadAllChats =
    user.permissions.includes('BiChat.ReadAll') || user.permissions.includes('AIChat.ReadAll')

  // Tab state
  const [activeTab, setActiveTab] = useState<ActiveTab>('my-chats')

  // Search state
  const [searchQuery, setSearchQuery] = useState('')

  // Extract session ID from pathname (e.g., /session/123 -> 123)
  const activeSessionId = location.pathname.match(/\/session\/([^/]+)/)?.[1]
  const isArchivedView = location.pathname === '/archived'

  const dataSource = useBiChatDataSource((sessionId: string) => navigate(`/session/${sessionId}`))

  useEffect(() => {
    if (!canReadAllChats && activeTab === 'all-chats') {
      setActiveTab('my-chats')
    }
  }, [activeTab, canReadAllChats])

  const rpc = useMemo(
    () => createAppletRPCClient({ endpoint: config.rpcUIEndpoint }),
    [config.rpcUIEndpoint]
  )
  const callRPC = useCallback(
    <TMethod extends keyof BichatRPC & string>(
      method: TMethod,
      params: BichatRPC[TMethod]['params']
    ) => rpc.callTyped<BichatRPC, TMethod>(method, params),
    [rpc]
  )

  const [fetching, setFetching] = useState(true)
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [loadError, setLoadError] = useState<RPCErrorDisplay | null>(null)
  const [actionError, setActionError] = useState<RPCErrorDisplay | null>(null)
  const accessDenied = loadError?.isPermissionDenied === true

  const reloadSessions = useMemo(() => {
    return async () => {
      setFetching(true)
      try {
        const data = await callRPC(
          'bichat.session.list',
          { limit: 200, offset: 0, includeArchived: false }
        )
        setSessions(data.sessions || [])
        setLoadError(null)
        setActionError(null)
      } catch (error) {
        console.error('Failed to load sessions:', error)
        setSessions([])
        setLoadError(toRPCErrorDisplay(error, 'Failed to load sessions'))
      } finally {
        setFetching(false)
      }
    }
  }, [callRPC])

  useEffect(() => {
    void reloadSessions()
  }, [reloadSessions])

  useEffect(() => {
    return sessionEvents.onSessionCreated(() => {
      void reloadSessions()
    })
  }, [reloadSessions, sessionEvents])

  // Poll for title updates on sessions with placeholder titles
  const sessionsKey = useMemo(
    () => sessions.map((s) => `${s.id}:${s.title || ''}`).join(','),
    [sessions]
  )

  useEffect(() => {
    // Check if any session has a placeholder title (null or empty)
    const sessionsWithPlaceholderTitles = sessions.filter(
      (s: ChatSession) => !s.title
    )

    if (sessionsWithPlaceholderTitles.length === 0) {
      return
    }

    // Poll every 2 seconds for up to 10 seconds
    const pollInterval = 2000
    const maxPolls = 5
    let pollCount = 0

    const intervalId = setInterval(() => {
      pollCount++
      void reloadSessions()

      if (pollCount >= maxPolls) {
        clearInterval(intervalId)
      }
    }, pollInterval)

    return () => clearInterval(intervalId)
  }, [reloadSessions, sessions, sessionsKey])

  // Get all sessions and apply search filter
  const filteredSessions = useSessionSearch(sessions, searchQuery)

  // Separate pinned and unpinned sessions from filtered results
  const pinnedSessions = filteredSessions.filter((s) => s.pinned)
  const unpinnedSessions = filteredSessions.filter((s) => !s.pinned)

  // Group unpinned sessions by date
  const sessionGroups = groupSessionsByDate(unpinnedSessions)

  const handleTogglePin = async (sessionId: string, currentlyPinned: boolean, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    try {
      if (currentlyPinned) {
        await callRPC('bichat.session.unpin', { id: sessionId })
      } else {
        await callRPC('bichat.session.pin', { id: sessionId })
      }
      setActionError(null)
      await reloadSessions()
      toast.success(currentlyPinned ? 'Chat unpinned' : 'Chat pinned')
    } catch (error) {
      console.error('Failed to toggle pin:', error)
      const display = toRPCErrorDisplay(error, 'Failed to update pin state')
      setActionError(display)
      toast.error(display.title)
    }
  }

  const handleRenameSession = async (sessionId: string, newTitle: string) => {
    try {
      await callRPC('bichat.session.updateTitle', {
        id: sessionId,
        title: newTitle,
      })
      setActionError(null)
      await reloadSessions()
      toast.success('Chat renamed')
    } catch (error) {
      console.error('Failed to update session title:', error)
      const display = toRPCErrorDisplay(error, 'Failed to rename session')
      setActionError(display)
      toast.error(display.title)
    }
  }

  const handleArchiveSession = async (sessionId: string, e?: React.MouseEvent) => {
    e?.preventDefault?.()
    e?.stopPropagation?.()
    try {
      await callRPC('bichat.session.archive', { id: sessionId })
      setActionError(null)
      await reloadSessions()
      toast.success('Chat archived')
      const currentPath = location.pathname
      if (currentPath === `/session/${sessionId}`) {
        navigate('/')
      }
    } catch (error) {
      console.error('Failed to archive session:', error)
      const display = toRPCErrorDisplay(error, 'Failed to archive session')
      setActionError(display)
      toast.error(display.title)
    }
  }

  // Click-on-empty-space to toggle (same pattern as SDK sidebar)
  const handleSidebarClick = useCallback(
    (e: React.MouseEvent<HTMLElement>) => {
      const interactive = 'a, button, input, summary, [role="button"]'
      if ((e.target as HTMLElement).closest(interactive)) return
      toggle()
    },
    [toggle]
  )

  // Focus the search input (expanding sidebar first if needed)
  const focusSearch = useCallback(() => {
    if (isCollapsedRef.current) {
      expand()
      // Wait for expanded content to become interactive (opacity transition)
      setTimeout(() => {
        searchContainerRef.current?.querySelector('input')?.focus()
      }, 250)
    } else {
      searchContainerRef.current?.querySelector('input')?.focus()
    }
  }, [expand, isCollapsedRef])

  // Keyboard shortcuts: Cmd+B (toggle), Cmd+K (focus search)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't intercept when inside editable fields (except for our shortcuts)
      const isMod = e.metaKey || e.ctrlKey

      if (isMod && e.key === 'b') {
        e.preventDefault()
        toggle()
      }

      if (isMod && e.key === 'k') {
        e.preventDefault()
        focusSearch()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [toggle, focusSearch])

  return (
    <aside
      onClick={handleSidebarClick}
      className={`relative h-full flex flex-col overflow-hidden border-r border-gray-200/60 dark:border-gray-800/60 transition-[width] duration-300 ease-in-out ${
        isCollapsed
          ? 'w-16 cursor-e-resize bg-gray-50/80 dark:bg-gray-900'
          : 'w-64 cursor-w-resize bg-white dark:bg-gray-900'
      }`}
      style={{ willChange: 'width' }}
      role="navigation"
      aria-label="Chat sessions"
    >
      {/* Collapsed overlay — absolutely positioned, fades in after width shrinks */}
      <div
        className={`absolute inset-x-0 top-0 bottom-0 z-10 flex flex-col items-center pt-3 gap-3 transition-opacity ${
          isCollapsed
            ? 'opacity-100 duration-150 delay-100'
            : 'opacity-0 pointer-events-none duration-100'
        }`}
      >
        <div className="group/tooltip relative">
          <motion.button
            onClick={(e) => {
              e.stopPropagation()
              onNewChat()
              onClose?.()
            }}
            disabled={creating || fetching || accessDenied}
            className="w-10 h-10 rounded-lg bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm flex items-center justify-center disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors focus-visible:ring-2 focus-visible:ring-primary-400/50"
            title="New chat"
            aria-label="Create new chat"
            whileTap={{ scale: 0.95 }}
          >
            {creating ? (
              <div className="w-4 h-4 border-2 border-white/50 border-t-transparent rounded-full animate-spin" />
            ) : (
              <Plus size={18} weight="bold" />
            )}
          </motion.button>
          <span className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/tooltip:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
            New chat
          </span>
        </div>

      </div>

      {/* Expanded content — fades out before width shrinks, prevents text compression */}
      <div
        className={`flex flex-col flex-1 min-h-0 w-64 shrink-0 transition-opacity ${
          isCollapsed
            ? 'opacity-0 pointer-events-none duration-100'
            : 'opacity-100 duration-150 delay-[200ms]'
        }`}
      >
        {/* New Chat Button - only in My Chats view */}
        {activeTab === 'my-chats' && (
          <div className="px-4 pt-3 pb-2">
            <motion.button
              onClick={(e) => {
                e.stopPropagation()
                onNewChat()
                onClose?.()
              }}
              disabled={creating || fetching || accessDenied}
              className="w-full px-4 py-2.5 rounded-lg font-medium bg-primary-600 hover:bg-primary-700 hover:-translate-y-0.5 active:bg-primary-800 text-white shadow-sm transition-all duration-150 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer flex items-center justify-center gap-2 focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900"
              title={accessDenied ? 'Missing permission for BiChat' : 'New chat'}
              aria-label="Create new chat"
              whileHover={{ y: -1 }}
              whileTap={{ scale: 0.98 }}
            >
              {creating ? (
                <>
                  <div className="w-4 h-4 border-2 border-white/50 border-t-transparent rounded-full animate-spin" />
                  <span>Creating...</span>
                </>
              ) : (
                <>
                  <Plus size={16} weight="bold" />
                  <span>New Chat</span>
                </>
              )}
            </motion.button>
          </div>
        )}

        {/* Search (My Chats) or title (All Chats) */}
        <div className="px-4 pb-2">
          {activeTab === 'my-chats' ? (
            <div ref={searchContainerRef} className="cursor-pointer">
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Search chats..."
              />
            </div>
          ) : (
            <div className="text-sm font-medium text-gray-700 dark:text-gray-300 truncate py-2">
              All Chats
            </div>
          )}
        </div>

        {/* My Chats: Chat History */}
        {activeTab === 'my-chats' && (
            <nav className="flex-1 overflow-y-auto scrollbar-thin px-2 pb-4" aria-label="Chat history">
              {fetching && sessions.length === 0 ? (
                <SessionSkeleton count={5} />
              ) : (
                <>
                  <SessionList
                    groups={sessionGroups}
                    pinnedSessions={pinnedSessions}
                    activeSessionId={activeSessionId}
                    onTogglePin={handleTogglePin}
                    onRename={handleRenameSession}
                    onArchive={handleArchiveSession}
                    onNavigate={onClose}
                  />

                  {/* Empty State - refined */}
                  {filteredSessions.length === 0 && !fetching && !loadError && (
                    <EmptyState
                      title={searchQuery ? `No results for "${searchQuery}"` : 'No chats yet'}
                      description={
                        searchQuery
                          ? undefined
                          : 'Start a conversation to begin'
                      }
                      action={
                        searchQuery ? (
                          <button
                            onClick={() => setSearchQuery('')}
                            className="mt-2 text-sm text-primary-600 dark:text-primary-400 hover:text-primary-700 dark:hover:text-primary-300 font-medium transition-colors"
                          >
                            Clear search
                          </button>
                        ) : undefined
                      }
                    />
                  )}
                </>
              )}

              {loadError && (
                <div
                  className={
                    loadError.isPermissionDenied
                      ? 'mx-2 mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl'
                      : 'mx-2 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl'
                  }
                >
                  <p
                    className={
                      loadError.isPermissionDenied
                        ? 'text-xs text-amber-700 dark:text-amber-300 font-medium'
                        : 'text-xs text-red-600 dark:text-red-400 font-medium'
                    }
                  >
                    {loadError.title}
                  </p>
                  <p
                    className={
                      loadError.isPermissionDenied
                        ? 'mt-1 text-xs text-amber-600 dark:text-amber-400'
                        : 'mt-1 text-xs text-red-500 dark:text-red-300'
                    }
                  >
                    {loadError.description}
                  </p>
                </div>
              )}

              {actionError && !loadError && (
                <div
                  className={
                    actionError.isPermissionDenied
                      ? 'mx-2 mt-4 p-3 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl'
                      : 'mx-2 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl'
                  }
                >
                  <p
                    className={
                      actionError.isPermissionDenied
                        ? 'text-xs text-amber-700 dark:text-amber-300 font-medium'
                        : 'text-xs text-red-600 dark:text-red-400 font-medium'
                    }
                  >
                    {actionError.title}
                  </p>
                  <p
                    className={
                      actionError.isPermissionDenied
                        ? 'mt-1 text-xs text-amber-600 dark:text-amber-400'
                        : 'mt-1 text-xs text-red-500 dark:text-red-300'
                    }
                  >
                    {actionError.description}
                  </p>
                </div>
              )}
            </nav>
        )}

        {/* All Chats View */}
        {activeTab === 'all-chats' && (
          dataSource.listAllSessions ? (
            <AllChatsList
              dataSource={dataSource}
              onSessionSelect={(sessionId: string) => {
                navigate(`/session/${sessionId}`)
                onClose?.()
              }}
              activeSessionId={activeSessionId}
            />
          ) : (
            <div className="flex-1 flex items-center justify-center p-4">
              <EmptyState
                title="All Chats"
                description="This app does not expose organization-wide chat sessions."
              />
            </div>
          )
        )}
      </div>

      {/* Footer — settings (left) when expanded, collapse/expand (right or centered) */}
      <div
        className={`mt-auto border-t border-gray-100 dark:border-gray-800/80 transition-all duration-300 flex items-center ${
          isCollapsed ? 'px-2 py-3 justify-center' : 'px-4 py-3 justify-between'
        }`}
      >
        {!isCollapsed && (
        <Menu>
          <MenuButton
            onClick={(e: React.MouseEvent) => {
              e.stopPropagation()
            }}
            disabled={fetching || accessDenied}
            className="flex items-center justify-center rounded-lg text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 p-2"
            aria-label="Settings"
            title="Settings"
          >
            <Gear size={20} />
          </MenuButton>
          <MenuItems
            anchor="top start"
            className="w-48 bg-white/95 dark:bg-gray-900/95 backdrop-blur-lg rounded-xl shadow-lg border border-gray-200/80 dark:border-gray-700/60 z-30 [--anchor-gap:8px] mb-1 p-1.5"
          >
            <MenuItem>
              {({ focus, close }) => (
                <button
                  onClick={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                    navigate('/archived')
                    onClose?.()
                    close()
                  }}
                  className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${
                    focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''
                  }`}
                  aria-label="Archived chats"
                  aria-current={isArchivedView ? 'page' : undefined}
                >
                  <Archive size={16} className="text-gray-400 dark:text-gray-500" />
                  Archived
                </button>
              )}
            </MenuItem>
            {canReadAllChats && activeTab !== 'all-chats' && (
              <MenuItem>
                {({ focus, close }) => (
                  <button
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      setActiveTab('all-chats')
                      close()
                    }}
                    className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${
                      focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''
                    }`}
                    aria-label="All chats (read-only)"
                  >
                    <Users size={16} className="text-gray-400 dark:text-gray-500" />
                    All Chats
                  </button>
                )}
              </MenuItem>
            )}
            {canReadAllChats && activeTab === 'all-chats' && (
              <MenuItem>
                {({ focus, close }) => (
                  <button
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      setActiveTab('my-chats')
                      close()
                    }}
                    className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${
                      focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''
                    }`}
                    aria-label="My chats"
                  >
                    <List size={16} className="text-gray-400 dark:text-gray-500" />
                    My Chats
                  </button>
                )}
              </MenuItem>
            )}
          </MenuItems>
        </Menu>
        )}

        <div className="group/tooltip relative">
          <button
            onClick={(e) => {
              e.stopPropagation()
              toggle()
            }}
            className={`flex items-center gap-2 rounded-lg text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors cursor-pointer ${
              isCollapsed ? 'w-10 h-10 justify-center' : 'px-3 py-2'
            }`}
            title={isCollapsed ? 'Expand sidebar (⌘B)' : 'Collapse sidebar (⌘B)'}
            aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {isCollapsed ? (
              <CaretLineRight size={16} />
            ) : (
              <>
                <CaretLineLeft size={16} />
                <span className="text-xs font-medium">Collapse</span>
              </>
            )}
          </button>
          {isCollapsed && (
            <span className="pointer-events-none absolute left-full ml-2 top-1/2 -translate-y-1/2 rounded-md bg-gray-900 dark:bg-gray-100 px-2 py-1 text-xs font-medium text-white dark:text-gray-900 opacity-0 group-hover/tooltip:opacity-100 transition-opacity whitespace-nowrap shadow-lg">
              Expand
            </span>
          )}
        </div>
      </div>
    </aside>
  )
}
