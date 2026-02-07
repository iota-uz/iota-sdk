/**
 * AllChatsList Component
 * Displays organization-wide chat sessions with infinite scroll
 * Uses ChatDataSource for data fetching (no GraphQL dependency)
 */

import { useState, useCallback, useEffect, useMemo } from 'react'
import { motion } from 'framer-motion'
import { Archive } from '@phosphor-icons/react'
import { UserAvatar } from './UserAvatar'
import { UserFilter } from './UserFilter'
import SessionSkeleton from './SessionSkeleton'
import { EmptyState } from './EmptyState'
import { staggerContainerVariants } from '../animations/variants'
import { useTranslation } from '../hooks/useTranslation'
import type { ChatDataSource, Session, SessionUser } from '../types'

interface AllChatsListProps {
  dataSource: ChatDataSource
  onSessionSelect: (sessionId: string) => void
  activeSessionId?: string
}

export default function AllChatsList({ dataSource, onSessionSelect, activeSessionId }: AllChatsListProps) {
  const { t } = useTranslation()

  // State
  const [includeArchived, setIncludeArchived] = useState(false)
  const [selectedUser, setSelectedUser] = useState<SessionUser | null>(null)
  const [offset, setOffset] = useState(0)
  const [fetching, setFetching] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [chats, setChats] = useState<Array<Session & { owner: SessionUser }>>([])
  const [totalCount, setTotalCount] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [users, setUsers] = useState<SessionUser[]>([])
  const [usersLoading, setUsersLoading] = useState(false)

  const limit = 20

  // Fetch users list
  useEffect(() => {
    if (!dataSource.listUsers) return

    let cancelled = false
    setUsersLoading(true)

    dataSource.listUsers().then((result) => {
      if (!cancelled) {
        setUsers(result)
        setUsersLoading(false)
      }
    }).catch(() => {
      if (!cancelled) {
        setUsersLoading(false)
      }
    })

    return () => { cancelled = true }
  }, [dataSource])

  // Fetch chats
  useEffect(() => {
    if (!dataSource.listAllSessions) return

    let cancelled = false
    setFetching(true)
    setError(null)

    dataSource.listAllSessions({
      limit,
      offset,
      includeArchived,
      userId: selectedUser?.id || null,
    }).then((result) => {
      if (!cancelled) {
        if (offset === 0) {
          setChats(result.sessions)
        } else {
          setChats((prev) => [...prev, ...result.sessions])
        }
        setTotalCount(result.total)
        setHasMore(result.hasMore)
        setFetching(false)
      }
    }).catch(() => {
      if (!cancelled) {
        setError(t('allChats.failedToLoad'))
        setFetching(false)
      }
    })

    return () => { cancelled = true }
  }, [dataSource, offset, includeArchived, selectedUser, t])

  // Reset offset when filter changes
  useEffect(() => {
    setOffset(0)
    setChats([])
  }, [includeArchived, selectedUser])

  // Load more handler
  const handleLoadMore = useCallback(() => {
    if (!fetching && hasMore) {
      setOffset((prev) => prev + limit)
    }
  }, [fetching, hasMore])

  // Infinite scroll observer
  const loadMoreRef = useCallback(
    (node: HTMLDivElement | null) => {
      if (!node || fetching || !hasMore) return

      const observer = new IntersectionObserver(
        (entries) => {
          if (entries[0].isIntersecting) {
            handleLoadMore()
          }
        },
        { threshold: 0.1 }
      )

      observer.observe(node)
      return () => observer.disconnect()
    },
    [fetching, hasMore, handleLoadMore]
  )

  // Derive unique users from chat data if listUsers is not available
  const derivedUsers = useMemo(() => {
    if (dataSource.listUsers) return users
    const userMap = new Map<string, SessionUser>()
    chats.forEach((chat) => {
      if (chat.owner && !userMap.has(chat.owner.id)) {
        userMap.set(chat.owner.id, chat.owner)
      }
    })
    return Array.from(userMap.values())
  }, [chats, users, dataSource.listUsers])

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Filter Controls */}
      <div className="px-4 py-3 space-y-3 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
        {/* User filter */}
        <UserFilter
          users={derivedUsers}
          selectedUser={selectedUser}
          onUserChange={setSelectedUser}
          loading={usersLoading || (fetching && chats.length === 0)}
        />

        {/* Include archived toggle */}
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={includeArchived}
            onChange={(e) => setIncludeArchived(e.target.checked)}
            className="
              w-4 h-4 rounded border-gray-300 dark:border-gray-600
              text-primary-600 focus:ring-primary-500 focus:ring-offset-0
              bg-white dark:bg-gray-800
              cursor-pointer
            "
          />
          <span className="text-sm text-gray-700 dark:text-gray-300 flex items-center gap-1.5">
            <Archive size={16} className="w-4 h-4" />
            {t('allChats.includeArchived')}
          </span>
        </label>

        {/* Results count */}
        {totalCount > 0 && (
          <p className="text-xs text-gray-500 dark:text-gray-400">
            {totalCount === 1
              ? t('allChats.chatFound', { count: totalCount })
              : t('allChats.chatsFound', { count: totalCount })}
          </p>
        )}
      </div>

      {/* Chat List */}
      <nav className="flex-1 overflow-y-auto px-2 pb-4 hide-scrollbar" aria-label={t('allChats.organizationChats')}>
        {fetching && chats.length === 0 ? (
          <SessionSkeleton count={5} />
        ) : (
          <>
            {chats.length > 0 ? (
              <motion.div
                className="space-y-1 mt-2"
                variants={staggerContainerVariants}
                initial="hidden"
                animate="visible"
                role="list"
                aria-label={t('allChats.organizationChatSessions')}
              >
                {chats.map((chat) => (
                  <motion.div
                    key={chat.id}
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                  >
                    <div
                      role="link"
                      tabIndex={0}
                      onClick={() => onSessionSelect(chat.id)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault()
                          onSessionSelect(chat.id)
                        }
                      }}
                      className={`
                        block px-3 py-2 rounded-lg transition-smooth group cursor-pointer
                        ${
                          chat.id === activeSessionId
                            ? 'bg-primary-50/50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 border-l-4 border-primary-400 dark:border-primary-600'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 border-l-4 border-transparent'
                        }
                      `}
                      aria-current={chat.id === activeSessionId ? 'page' : undefined}
                    >
                      <div className="flex items-start gap-2">
                        {/* Owner avatar */}
                        <UserAvatar
                          firstName={chat.owner.firstName}
                          lastName={chat.owner.lastName}
                          initials={chat.owner.initials}
                          size="sm"
                        />

                        {/* Chat info */}
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium truncate">
                            {chat.title || t('common.untitled')}
                          </p>
                          <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                            {chat.owner.firstName} {chat.owner.lastName}
                          </p>
                          {chat.status === 'archived' && (
                            <span className="inline-flex items-center gap-1 mt-1 px-2 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded-full text-xs">
                              <Archive size={12} className="w-3 h-3" />
                              {t('chat.archived')}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  </motion.div>
                ))}

                {/* Load more trigger */}
                {hasMore && (
                  <div ref={loadMoreRef} className="py-4 text-center">
                    {fetching ? (
                      <SessionSkeleton count={2} />
                    ) : (
                      <button
                        onClick={handleLoadMore}
                        className="text-sm text-primary-600 dark:text-primary-400 hover:underline"
                      >
                        {t('allChats.loadMore')}
                      </button>
                    )}
                  </div>
                )}
              </motion.div>
            ) : (
              <EmptyState
                title={t('allChats.noChatsFound')}
                description={
                  selectedUser
                    ? t('allChats.noChatsFromUser', { firstName: selectedUser.firstName, lastName: selectedUser.lastName })
                    : includeArchived
                    ? t('allChats.noChatsInOrg')
                    : t('allChats.noActiveChatsInOrg')
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
    </div>
  )
}
