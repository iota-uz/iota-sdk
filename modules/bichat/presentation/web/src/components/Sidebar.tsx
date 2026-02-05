/**
 * Sidebar Component
 * Left sidebar with session list (desktop-only, no mobile features)
 */

import React, { useState, useMemo, useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { Plus, Sparkle } from '@phosphor-icons/react'
import { SearchInput, EmptyState } from '@iota-uz/sdk/bichat'
import { createAppletRPCClient } from '@iota-uz/sdk'
import TabBar from './TabBar'
import SessionList from './SessionList/SessionList'
import SessionSkeleton from './SessionSkeleton'
import { groupSessionsByDate } from '../utils/sessionGrouping'
import type { ChatSession } from '../utils/sessionGrouping'
import { useIotaContext } from '../contexts/IotaContext'

// Note: RegenerateSessionTitle will be implemented later if needed
// For now, users can manually rename sessions

type ActiveTab = 'my-chats' | 'all-chats'

interface SidebarProps {
  onNewChat: () => void
  creating?: boolean
}

// Simple search filter (case-insensitive title match)
function useSessionSearch(sessions: ChatSession[], query: string): ChatSession[] {
  return useMemo(() => {
    if (!query.trim()) return sessions
    const lowerQuery = query.toLowerCase()
    return sessions.filter((s) => s.title?.toLowerCase().includes(lowerQuery))
  }, [sessions, query])
}

export default function Sidebar({ onNewChat, creating }: SidebarProps) {
  const location = useLocation()

  // Permission checks (placeholder - TODO: integrate with IotaContext when available)
  const canReadAllChats = false // For now, disable "All Chats" tab

  // Tab state
  const [activeTab, setActiveTab] = useState<ActiveTab>('my-chats')

  // Search state
  const [searchQuery, setSearchQuery] = useState('')

  // Extract session ID from pathname (e.g., /session/123 -> 123)
  const activeSessionId = location.pathname.match(/\/session\/([^/]+)/)?.[1]

  const rpc = useMemo(
    () => createAppletRPCClient({ endpoint: config.rpcUIEndpoint }),
    [config.rpcUIEndpoint]
  )

  const [fetching, setFetching] = useState(true)
  const [sessions, setSessions] = useState<ChatSession[]>([])
  const [loadError, setLoadError] = useState(false)

  const reloadSessions = useMemo(() => {
    return async () => {
      setFetching(true)
      try {
        const data = await rpc.call<{ limit: number; offset: number }, { sessions: ChatSession[] }>(
          'bichat.session.list',
          { limit: 200, offset: 0 }
        )
        setSessions(data.sessions || [])
        setLoadError(false)
      } catch (error) {
        console.error('Failed to load sessions:', error)
        setSessions([])
        setLoadError(true)
      } finally {
        setFetching(false)
      }
    }
  }, [rpc])

  useEffect(() => {
    void reloadSessions()
  }, [reloadSessions])

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

  const handleDeleteSession = async (sessionId: string, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    if (!confirm('Delete this chat session?')) return

    try {
      await rpc.call<{ id: string }, { ok: boolean }>('bichat.session.delete', { id: sessionId })
      await reloadSessions()
    } catch (error) {
      console.error('Failed to delete session:', error)
    }
  }

  const handleTogglePin = async (sessionId: string, currentlyPinned: boolean, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    try {
      if (currentlyPinned) {
        await rpc.call<{ id: string }, { session: ChatSession }>('bichat.session.unpin', { id: sessionId })
      } else {
        await rpc.call<{ id: string }, { session: ChatSession }>('bichat.session.pin', { id: sessionId })
      }
      await reloadSessions()
    } catch (error) {
      console.error('Failed to toggle pin:', error)
    }
  }

  const handleRenameSession = async (sessionId: string, newTitle: string) => {
    try {
      await rpc.call<{ id: string; title: string }, { session: ChatSession }>('bichat.session.updateTitle', {
        id: sessionId,
        title: newTitle,
      })
      await reloadSessions()
    } catch (error) {
      console.error('Failed to update session title:', error)
    }
  }

  // Regenerate title functionality removed for now
  // Users can manually rename sessions via the rename option

  return (
    <aside
      className="w-64 h-full flex flex-col overflow-hidden bg-white dark:bg-gray-900 border-r border-gray-100 dark:border-gray-800/80"
      role="navigation"
      aria-label="Chat sessions"
    >
      {/* TabBar - Only visible if user has ReadAll permission */}
      <TabBar activeTab={activeTab} onTabChange={setActiveTab} showAllChats={canReadAllChats} />

      {/* My Chats View */}
      {activeTab === 'my-chats' && (
        <>
          {/* New Chat Button */}
          <div className="px-4 pt-3 pb-2">
            <motion.button
              onClick={onNewChat}
              disabled={creating || fetching}
              className="w-full px-4 py-2.5 rounded-lg font-medium bg-primary-600 hover:bg-primary-700 hover:-translate-y-0.5 active:bg-primary-800 text-white shadow-sm transition-all duration-150 disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2 focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900"
              title="New chat"
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
                  <Sparkle size={12} weight="fill" className="text-white/70" />
                </>
              )}
            </motion.button>
          </div>

          {/* Search Input */}
          <div className="px-4 pb-2">
            <SearchInput
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder="Search chats..."
            />
          </div>

          {/* Chat History */}
          <nav className="flex-1 overflow-y-auto scrollbar-thin px-2 pb-4" aria-label="Chat history">
            {fetching && sessions.length === 0 ? (
              <SessionSkeleton count={5} />
            ) : (
              <>
                <SessionList
                  groups={sessionGroups}
                  pinnedSessions={pinnedSessions}
                  activeSessionId={activeSessionId}
                  onDelete={handleDeleteSession}
                  onTogglePin={handleTogglePin}
                  onRename={handleRenameSession}
                />

                {/* Empty State - refined */}
                {filteredSessions.length === 0 && !fetching && (
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
              <div className="mx-2 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl">
                <p className="text-xs text-red-600 dark:text-red-400 font-medium">Failed to load sessions</p>
              </div>
            )}
          </nav>
        </>
      )}

      {/* All Chats View - Placeholder */}
      {activeTab === 'all-chats' && (
        <div className="flex-1 flex items-center justify-center p-4">
          <EmptyState
            title="All Chats"
            description="This feature is not yet implemented"
          />
        </div>
      )}
    </aside>
  )
}
