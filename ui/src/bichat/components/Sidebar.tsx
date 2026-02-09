/**
 * Sidebar Component
 * Main chat sidebar with session list, search, tabs, and session management.
 * Router-agnostic: uses callbacks for navigation instead of react-router-dom.
 */

import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { motion, useReducedMotion } from 'framer-motion'
import { X, Plus, Archive } from '@phosphor-icons/react'
import SessionSkeleton from './SessionSkeleton'
import SessionItem from './SessionItem'
import ConfirmModal from './ConfirmModal'
import SearchInput from './SearchInput'
import DateGroupHeader from './DateGroupHeader'
import { EmptyState } from './EmptyState'
import LoadingSpinner from './LoadingSpinner'
import TabBar from './TabBar'
import AllChatsList from './AllChatsList'
import { useTranslation } from '../hooks/useTranslation'
import { useToast } from '../hooks/useToast'
import { groupSessionsByDate } from '../utils/sessionGrouping'
import {
  staggerContainerVariants,
  buttonVariants,
} from '../animations/variants'
import type { Session, ChatDataSource } from '../types'
import { ToastContainer } from './ToastContainer'

type ActiveTab = 'my-chats' | 'all-chats'

export interface SidebarProps {
  dataSource: ChatDataSource
  onSessionSelect: (sessionId: string) => void
  onNewChat: () => void
  onArchivedView?: () => void
  activeSessionId?: string
  creating?: boolean
  showAllChatsTab?: boolean
  isOpen?: boolean
  onClose?: () => void
  headerSlot?: React.ReactNode
  footerSlot?: React.ReactNode
  className?: string
}

export default function Sidebar({
  dataSource,
  onSessionSelect,
  onNewChat,
  onArchivedView,
  activeSessionId,
  creating,
  showAllChatsTab,
  isOpen: _isOpen,
  onClose,
  headerSlot,
  footerSlot,
  className = '',
}: SidebarProps) {
  const { t } = useTranslation()
  const toast = useToast()
  const shouldReduceMotion = useReducedMotion()
  const sessionListRef = useRef<HTMLElement>(null)

  // Tab state
  const [activeTab, setActiveTab] = useState<ActiveTab>('my-chats')

  // Search state
  const [searchQuery, setSearchQuery] = useState('')

  // Session data
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Refresh key — bump to re-fetch sessions
  const [refreshKey, setRefreshKey] = useState(0)

  // Confirm modal state
  const [showConfirm, setShowConfirm] = useState(false)
  const [sessionToArchive, setSessionToArchive] = useState<string | null>(null)

  // Build tab items
  const tabs = useMemo(() => {
    const items = [{ id: 'my-chats', label: t('sidebar.myChats') }]
    if (showAllChatsTab) {
      items.push({ id: 'all-chats', label: t('sidebar.allChats') })
    }
    return items
  }, [showAllChatsTab, t])

  // Fetch sessions
  const fetchSessions = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const result = await dataSource.listSessions({ limit: 50 })
      setSessions(result.sessions)
    } catch (err) {
      console.error('Failed to load sessions:', err)
      setError(t('sidebar.failedToLoadSessions'))
    } finally {
      setLoading(false)
    }
  }, [dataSource, t])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions, refreshKey])

  // Poll for title updates on sessions with placeholder titles.
  // Use a ref to track whether polling should start, so that updating
  // sessions inside the interval does NOT re-trigger the effect (which
  // would reset the poll counter and create overlapping intervals).
  const sessionsRef = useRef(sessions)
  sessionsRef.current = sessions

  const hasPlaceholderTitles = useMemo(() => {
    const newChatLabel = t('chat.newChat')
    return sessions.some((s) => s && (!s.title || s.title === newChatLabel))
  }, [sessions, t])

  useEffect(() => {
    if (!hasPlaceholderTitles) return

    const pollInterval = 2000
    const maxPolls = 5
    let pollCount = 0

    const intervalId = setInterval(async () => {
      pollCount++
      try {
        const result = await dataSource.listSessions({ limit: 50 })
        setSessions(result.sessions)
      } catch {
        clearInterval(intervalId)
        return
      }
      if (pollCount >= maxPolls) {
        clearInterval(intervalId)
      }
    }, pollInterval)

    return () => clearInterval(intervalId)
  }, [hasPlaceholderTitles, dataSource])

  const handleArchiveRequest = (sessionId: string) => {
    setSessionToArchive(sessionId)
    setShowConfirm(true)
  }

  const confirmArchive = async () => {
    if (!sessionToArchive) return

    const wasCurrentSession = activeSessionId === sessionToArchive

    try {
      await dataSource.archiveSession(sessionToArchive)
      setRefreshKey((k) => k + 1)

      if (wasCurrentSession) {
        onSessionSelect('')
      }
    } catch (err) {
      console.error('Failed to archive session:', err)
      toast.error(t('sidebar.failedToArchiveChat'))
    } finally {
      setShowConfirm(false)
      setSessionToArchive(null)
    }
  }

  const handleTogglePin = async (
    sessionId: string,
    currentlyPinned: boolean
  ) => {
    try {
      if (currentlyPinned) {
        await dataSource.unpinSession(sessionId)
      } else {
        await dataSource.pinSession(sessionId)
      }
      setRefreshKey((k) => k + 1)
    } catch (err) {
      console.error('Failed to toggle pin:', err)
      toast.error(t('sidebar.failedToTogglePin'))
    }
  }

  const handleRenameSession = async (sessionId: string, newTitle: string) => {
    try {
      await dataSource.renameSession(sessionId, newTitle)
      toast.success(t('sidebar.chatRenamedSuccessfully'))
      setRefreshKey((k) => k + 1)
    } catch (err) {
      console.error('Failed to update session title:', err)
      toast.error(t('sidebar.failedToRenameChat'))
    }
  }

  const handleRegenerateTitle = async (sessionId: string) => {
    try {
      await dataSource.regenerateSessionTitle(sessionId)
      toast.success(t('sidebar.titleRegenerated'))
      setRefreshKey((k) => k + 1)
    } catch (err) {
      console.error('Failed to regenerate title:', err)
      toast.error(t('sidebar.failedToRegenerateTitle'))
    }
  }

  // Filter sessions by search
  const filteredSessions = useMemo(() => {
    if (!searchQuery.trim()) return sessions
    const q = searchQuery.toLowerCase()
    return sessions.filter((s) => s.title?.toLowerCase().includes(q))
  }, [sessions, searchQuery])

  // Separate pinned and unpinned
  const pinnedSessions = useMemo(
    () => filteredSessions.filter((s) => s.pinned),
    [filteredSessions]
  )
  const unpinnedSessions = useMemo(
    () => filteredSessions.filter((s) => !s.pinned),
    [filteredSessions]
  )

  // Group unpinned sessions by date
  const sessionGroups = useMemo(() => {
    const groups = groupSessionsByDate(unpinnedSessions, t)
    return Array.isArray(groups)
      ? groups.map((group) => ({
          ...group,
          sessions: Array.isArray(group.sessions) ? group.sessions : [],
        }))
      : []
  }, [unpinnedSessions, t])

  // Keyboard navigation for session list (WAI-ARIA listbox pattern)
  const handleSessionListKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLElement>) => {
      const nav = sessionListRef.current
      if (!nav) return

      const focusableItems = Array.from(
        nav.querySelectorAll<HTMLElement>('button[data-session-item]')
      )
      if (focusableItems.length === 0) return

      const currentIndex = focusableItems.indexOf(
        document.activeElement as HTMLElement
      )

      let nextIndex: number | null = null

      switch (e.key) {
        case 'ArrowDown':
          e.preventDefault()
          nextIndex =
            currentIndex < 0 ? 0 : Math.min(currentIndex + 1, focusableItems.length - 1)
          break
        case 'ArrowUp':
          e.preventDefault()
          nextIndex =
            currentIndex < 0
              ? focusableItems.length - 1
              : Math.max(currentIndex - 1, 0)
          break
        case 'Home':
          e.preventDefault()
          nextIndex = 0
          break
        case 'End':
          e.preventDefault()
          nextIndex = focusableItems.length - 1
          break
      }

      if (nextIndex !== null) {
        focusableItems[nextIndex].focus()
      }
    },
    []
  )

  return (
    <>
      <aside
        className={`w-64 bg-surface-300 dark:bg-gray-900 border-r border-gray-200 dark:border-gray-700 h-full min-h-0 flex flex-col overflow-hidden ${className}`}
        role="navigation"
        aria-label={t('sidebar.chatSessions')}
      >
        {/* Header */}
        <div className="p-4 border-b border-gray-200 dark:border-gray-700 flex items-center justify-between">
          {headerSlot}
          {onClose && (
            <motion.button
              onClick={onClose}
              className="cursor-pointer p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-smooth text-gray-600 dark:text-gray-400"
              title={t('sidebar.closeSidebar')}
              aria-label={t('sidebar.closeSidebar')}
              whileHover="hover"
              whileTap="tap"
              variants={buttonVariants}
            >
              <X size={20} className="w-5 h-5" />
            </motion.button>
          )}
        </div>

        {/* TabBar - Only visible if consumer passes showAllChatsTab */}
        {showAllChatsTab && (
          <TabBar
            tabs={tabs}
            activeTab={activeTab}
            onTabChange={(id) => setActiveTab(id as ActiveTab)}
          />
        )}

        {/* Conditional content based on active tab */}
        {activeTab === 'all-chats' && showAllChatsTab ? (
          <AllChatsList
            dataSource={dataSource}
            onSessionSelect={onSessionSelect}
            activeSessionId={activeSessionId}
          />
        ) : (
          <>
            {/* Search Input */}
            <div className="mt-3">
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder={t('sidebar.searchChats')}
              />
            </div>

            {/* New Chat Button */}
            <div className="p-4">
              <motion.button
                onClick={onNewChat}
                disabled={creating || loading}
                className="cursor-pointer w-full px-4 py-3 bg-primary-600 dark:bg-primary-700 text-white rounded-lg hover:bg-primary-700 dark:hover:bg-primary-800 transition-smooth font-medium disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center"
                title={t('chat.newChat')}
                aria-label={t('sidebar.createNewChat')}
                whileHover={shouldReduceMotion ? {} : { scale: 1.02 }}
                whileTap={shouldReduceMotion ? {} : { scale: 0.95 }}
              >
                {creating ? (
                  <LoadingSpinner variant="spinner" size="sm" />
                ) : (
                  <>
                    <Plus size={20} className="w-5 h-5" />
                    <span className="ml-2">{t('chat.newChat')}</span>
                  </>
                )}
              </motion.button>
            </div>

            {/* Archived Chats Link */}
            {onArchivedView && (
              <div className="px-4 pb-2">
                <button
                  onClick={onArchivedView}
                  className="cursor-pointer flex items-center gap-2 px-3 py-2 rounded-lg text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition-smooth text-sm font-medium w-full"
                  title={t('sidebar.archivedChats')}
                >
                  <Archive size={18} className="w-4.5 h-4.5" />
                  <span>{t('sidebar.archivedChats')}</span>
                </button>
              </div>
            )}

            {/* Chat History */}
            <nav
              ref={sessionListRef}
              className="flex-1 overflow-y-auto px-2 pb-4 hide-scrollbar"
              aria-label="Chat history"
              onKeyDown={handleSessionListKeyDown}
            >
              {loading && sessions.length === 0 ? (
                <SessionSkeleton count={5} />
              ) : (
                <>
                  {/* Pinned Sessions */}
                  {pinnedSessions.length > 0 && (
                    <div className="mb-4">
                      <DateGroupHeader
                        groupName={t('common.pinned')}
                        count={pinnedSessions.length}
                      />
                      <motion.div
                        className="space-y-1 mt-2"
                        variants={staggerContainerVariants}
                        initial="hidden"
                        animate="visible"
                        role="list"
                        aria-label={t('sidebar.pinnedChats')}
                      >
                        {pinnedSessions.map((session) => (
                          <SessionItem
                            key={session.id}
                            session={session}
                            isActive={session.id === activeSessionId}
                            onSelect={() => onSessionSelect(session.id)}
                            onArchive={() =>
                              handleArchiveRequest(session.id)
                            }
                            onPin={() =>
                              handleTogglePin(session.id, session.pinned)
                            }
                            onRename={(newTitle) =>
                              handleRenameSession(session.id, newTitle)
                            }
                            onRegenerateTitle={() =>
                              handleRegenerateTitle(session.id)
                            }
                          />
                        ))}
                      </motion.div>
                      <div className="border-b border-gray-200 dark:border-gray-700 my-3" />
                    </div>
                  )}

                  {/* Grouped Sessions by Date */}
                  {sessionGroups.map((group) => (
                    <div key={group.name} className="mb-4">
                      <DateGroupHeader
                        groupName={group.name}
                        count={group.sessions.length}
                      />
                      <motion.div
                        className="space-y-1 mt-2"
                        variants={staggerContainerVariants}
                        initial="hidden"
                        animate="visible"
                        role="list"
                        aria-label={`${group.name} chats`}
                      >
                        {group.sessions.map((session) => (
                          <SessionItem
                            key={session.id}
                            session={session}
                            isActive={session.id === activeSessionId}
                            onSelect={() => onSessionSelect(session.id)}
                            onArchive={() =>
                              handleArchiveRequest(session.id)
                            }
                            onPin={() =>
                              handleTogglePin(session.id, session.pinned)
                            }
                            onRename={(newTitle) =>
                              handleRenameSession(session.id, newTitle)
                            }
                            onRegenerateTitle={() =>
                              handleRegenerateTitle(session.id)
                            }
                          />
                        ))}
                      </motion.div>
                    </div>
                  ))}

                  {/* Empty State */}
                  {filteredSessions.length === 0 && !loading && (
                    <EmptyState
                      title={
                        searchQuery
                          ? t('sidebar.noChatsFound', { query: searchQuery })
                          : t('sidebar.noChatsYet')
                      }
                      description={
                        searchQuery
                          ? undefined
                          : t('sidebar.createOneToGetStarted')
                      }
                      action={
                        searchQuery ? (
                          <button
                            onClick={() => setSearchQuery('')}
                            className="cursor-pointer text-sm text-primary-600 dark:text-primary-400 hover:underline"
                          >
                            {t('common.clear')}
                          </button>
                        ) : undefined
                      }
                    />
                  )}
                </>
              )}

              {error && (
                <div className="mx-2 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
                  <p className="text-xs text-red-600 dark:text-red-400">
                    {error}
                  </p>
                </div>
              )}
            </nav>

            {/* Footer slot */}
            {footerSlot}
          </>
        )}
      </aside>

      {/* Confirm Archive Modal */}
      <ConfirmModal
        isOpen={showConfirm}
        title={t('sidebar.archiveChatSession')}
        message={t('sidebar.archiveChatMessage')}
        confirmText={t('sidebar.archiveButton')}
        cancelText={t('common.cancel')}
        isDanger={false}
        onConfirm={confirmArchive}
        onCancel={() => {
          setShowConfirm(false)
          setSessionToArchive(null)
        }}
      />

      {/* Toast notifications */}
      <ToastContainer toasts={toast.toasts} onDismiss={toast.dismiss} />
    </>
  )
}
