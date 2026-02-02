/**
 * Sidebar Component
 * Left sidebar with session list (desktop-only, no mobile features)
 */

import React, { useState, useMemo, useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { motion } from 'framer-motion'
import { useQuery, useMutation } from 'urql'
import { Plus } from '@phosphor-icons/react'
import { SearchInput, EmptyState } from '@iota-uz/bichat-ui'
import TabBar from './TabBar'
import SessionList from './SessionList/SessionList'
import SessionSkeleton from './SessionSkeleton'
import { groupSessionsByDate } from '../utils/sessionGrouping'
import type { ChatSession } from '../utils/sessionGrouping'

// GraphQL queries
const GET_SESSIONS_QUERY = `
  query Sessions {
    sessions {
      id
      title
      pinned
      createdAt
      updatedAt
    }
  }
`

const DELETE_SESSION_MUTATION = `
  mutation DeleteSession($id: UUID!) {
    deleteSession(id: $id)
  }
`

const UPDATE_SESSION_TITLE_MUTATION = `
  mutation UpdateSessionTitle($id: UUID!, $title: String!) {
    updateSessionTitle(id: $id, title: $title) {
      id
      title
      updatedAt
    }
  }
`

const PIN_SESSION_MUTATION = `
  mutation PinSession($id: UUID!) {
    pinSession(id: $id) {
      id
      pinned
      updatedAt
    }
  }
`

const UNPIN_SESSION_MUTATION = `
  mutation UnpinSession($id: UUID!) {
    unpinSession(id: $id) {
      id
      pinned
      updatedAt
    }
  }
`

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

  // GraphQL queries
  const [result, reexecuteQuery] = useQuery({
    query: GET_SESSIONS_QUERY,
  })

  const [, deleteSession] = useMutation(DELETE_SESSION_MUTATION)
  const [, updateSessionTitle] = useMutation(UPDATE_SESSION_TITLE_MUTATION)
  const [, pinSession] = useMutation(PIN_SESSION_MUTATION)
  const [, unpinSession] = useMutation(UNPIN_SESSION_MUTATION)

  // Poll for title updates on sessions with placeholder titles
  const currentSessions = useMemo(
    () => result.data?.sessions || [],
    [result.data?.sessions]
  )
  const sessionsKey = useMemo(
    () => currentSessions.map((s: ChatSession) => `${s.id}:${s.title || ''}`).join(','),
    [currentSessions]
  )

  useEffect(() => {
    // Check if any session has a placeholder title (null or empty)
    const sessionsWithPlaceholderTitles = currentSessions.filter(
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
      reexecuteQuery({ requestPolicy: 'network-only' })

      if (pollCount >= maxPolls) {
        clearInterval(intervalId)
      }
    }, pollInterval)

    return () => clearInterval(intervalId)
  }, [sessionsKey, reexecuteQuery, currentSessions])

  // Get all sessions and apply search filter
  const sessions: ChatSession[] = result.data?.sessions || []
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
      await deleteSession({ id: sessionId })
      reexecuteQuery({ requestPolicy: 'network-only' })
    } catch (error) {
      console.error('Failed to delete session:', error)
    }
  }

  const handleTogglePin = async (sessionId: string, currentlyPinned: boolean, e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()

    try {
      if (currentlyPinned) {
        await unpinSession({ id: sessionId })
      } else {
        await pinSession({ id: sessionId })
      }
      reexecuteQuery({ requestPolicy: 'network-only' })
    } catch (error) {
      console.error('Failed to toggle pin:', error)
    }
  }

  const handleRenameSession = async (sessionId: string, newTitle: string) => {
    try {
      await updateSessionTitle({ id: sessionId, title: newTitle })
      reexecuteQuery({ requestPolicy: 'network-only' })
    } catch (error) {
      console.error('Failed to update session title:', error)
    }
  }

  // Regenerate title functionality removed for now
  // Users can manually rename sessions via the rename option

  return (
    <aside
      className="w-64 h-screen flex flex-col overflow-hidden bg-white dark:bg-gray-900 border-r border-gray-200 dark:border-gray-800"
      role="navigation"
      aria-label="Chat sessions"
    >
      {/* Header */}
      <div className="px-5 py-4 border-b border-gray-200 dark:border-gray-800">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-primary-600 flex items-center justify-center">
            <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
            </svg>
          </div>
          <h2 className="text-base font-semibold text-gray-900 dark:text-white">BiChat</h2>
        </div>
      </div>

      {/* TabBar - Only visible if user has ReadAll permission */}
      <TabBar activeTab={activeTab} onTabChange={setActiveTab} showAllChats={canReadAllChats} />

      {/* My Chats View */}
      {activeTab === 'my-chats' && (
        <>
          {/* New Chat Button */}
          <div className="px-4 pt-4 pb-2">
            <motion.button
              onClick={onNewChat}
              disabled={creating || result.fetching}
              className="w-full px-4 py-2.5 rounded-lg font-medium bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center justify-center gap-2"
              title="New chat"
              aria-label="Create new chat"
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

          {/* Search Input */}
          <div className="px-4 pb-3">
            <SearchInput
              value={searchQuery}
              onChange={setSearchQuery}
              placeholder="Search chats..."
            />
          </div>

          {/* Chat History */}
          <nav className="flex-1 overflow-y-auto scrollbar-thin px-2 pb-4" aria-label="Chat history">
            {result.fetching && sessions.length === 0 ? (
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
                {filteredSessions.length === 0 && !result.fetching && (
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

            {result.error && (
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
