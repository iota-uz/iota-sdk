/**
 * ArchivedChatList Component
 * Displays archived chat sessions with restore functionality.
 * Router-agnostic: uses callbacks for navigation instead of react-router-dom.
 */

import { useState, useEffect, useCallback, useMemo } from 'react'
import { motion } from 'framer-motion'
import { Archive, ArrowLeft } from '@phosphor-icons/react'
import SessionItem from './SessionItem'
import SearchInput from './SearchInput'
import DateGroupHeader from './DateGroupHeader'
import LoadingSpinner from './LoadingSpinner'
import ConfirmModal from './ConfirmModal'
import { EmptyState } from './EmptyState'
import { ToastContainer } from './ToastContainer'
import { useTranslation } from '../hooks/useTranslation'
import { useToast, type UseToastReturn } from '../hooks/useToast'
import { groupSessionsByDate } from '../utils/sessionGrouping'
import { staggerContainerVariants } from '../animations/variants'
import type { Session, ChatDataSource } from '../types'

export interface ArchivedChatListProps {
  dataSource: ChatDataSource
  onBack: () => void
  onSessionSelect: (sessionId: string) => void
  activeSessionId?: string
  className?: string
  toast?: UseToastReturn
}

export default function ArchivedChatList({
  dataSource,
  onBack,
  onSessionSelect,
  activeSessionId,
  className = '',
  toast: toastFromProps,
}: ArchivedChatListProps) {
  const { t } = useTranslation()
  const localToast = useToast()
  const toast = toastFromProps ?? localToast
  const shouldRenderToastContainer = !toastFromProps

  // Search state
  const [searchQuery, setSearchQuery] = useState('')

  // Session data
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)

  // Refresh key
  const [refreshKey, setRefreshKey] = useState(0)

  // Confirm modal state for restore action
  const [showConfirm, setShowConfirm] = useState(false)
  const [sessionToRestore, setSessionToRestore] = useState<string | null>(null)

  // Fetch archived sessions
  const fetchSessions = useCallback(async () => {
    try {
      setLoading(true)
      const result = await dataSource.listSessions({
        limit: 100,
        includeArchived: true,
      })
      setSessions(result.sessions.filter((s) => s.status === 'archived'))
    } catch (err) {
      console.error('Failed to load archived sessions:', err)
    } finally {
      setLoading(false)
    }
  }, [dataSource])

  useEffect(() => {
    fetchSessions()
  }, [fetchSessions, refreshKey])

  const handleRestoreRequest = (sessionId: string) => {
    setSessionToRestore(sessionId)
    setShowConfirm(true)
  }

  const confirmRestore = async () => {
    if (!sessionToRestore) return

    try {
      await dataSource.unarchiveSession(sessionToRestore)
      setRefreshKey((k) => k + 1)
      toast.success(t('archived.chatRestoredSuccessfully'))
    } catch (err) {
      console.error('Failed to restore session:', err)
      toast.error(t('archived.failedToRestoreChat'))
    } finally {
      setShowConfirm(false)
      setSessionToRestore(null)
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

  // Filter by search query
  const filteredSessions = useMemo(() => {
    if (!searchQuery.trim()) return sessions
    const q = searchQuery.toLowerCase()
    return sessions.filter((s) => s.title?.toLowerCase().includes(q))
  }, [sessions, searchQuery])

  // Group sessions by date
  const sessionGroups = useMemo(() => {
    const groups = groupSessionsByDate(filteredSessions, t)
    return Array.isArray(groups)
      ? groups.map((group) => ({
          ...group,
          sessions: Array.isArray(group.sessions) ? group.sessions : [],
        }))
      : []
  }, [filteredSessions, t])

  const isEmpty = sessions.length === 0
  const isEmptyAfterSearch = filteredSessions.length === 0 && !!searchQuery

  return (
    <div
      className={`flex-1 flex flex-col bg-gray-50 dark:bg-gray-900 ${className}`}
    >
      {/* Header */}
      <div className="border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-6 py-4">
        <div className="flex items-center gap-3 mb-4">
          <button
            onClick={onBack}
            className="inline-flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors text-gray-600 dark:text-gray-400"
            aria-label={t('archived.backToChats')}
          >
            <ArrowLeft size={20} className="w-5 h-5" />
            {t('common.back')}
          </button>
        </div>

        <div className="flex items-center gap-2 mb-4">
          <Archive
            size={24}
            className="w-6 h-6 text-gray-600 dark:text-gray-400"
          />
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            {t('archived.title')}
          </h1>
        </div>

        {/* Search */}
        <SearchInput
          value={searchQuery}
          onChange={setSearchQuery}
          placeholder={t('archived.searchArchivedChats')}
        />
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {loading && sessions.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <LoadingSpinner />
          </div>
        ) : isEmpty ? (
          <div className="flex items-center justify-center h-full px-6">
            <EmptyState
              icon={
                <Archive
                  size={48}
                  className="text-gray-400 dark:text-gray-500"
                />
              }
              title={t('archived.noArchivedChats')}
              description={t('archived.noArchivedChatsDescription')}
            />
          </div>
        ) : isEmptyAfterSearch ? (
          <div className="flex items-center justify-center h-full px-6">
            <EmptyState
              icon={
                <Archive
                  size={48}
                  className="text-gray-400 dark:text-gray-500"
                />
              }
              title={t('archived.noResults')}
              description={t('archived.noResultsDescription', {
                query: searchQuery,
              })}
            />
          </div>
        ) : (
          <motion.div
            className="px-4 py-4 space-y-4"
            variants={staggerContainerVariants}
            initial="hidden"
            animate="visible"
          >
            {sessionGroups.map((group) => (
              <div key={group.name}>
                <DateGroupHeader
                  groupName={group.name}
                  count={group.sessions.length}
                />
                <motion.ul className="space-y-1 mt-3 mb-4" role="list">
                  {group.sessions.map((session) => (
                    <motion.li key={session.id} className="opacity-70">
                      <SessionItem
                        session={session}
                        isActive={session.id === activeSessionId}
                        mode="archived"
                        testIdPrefix="archived"
                        onSelect={() => onSessionSelect(session.id)}
                        onRestore={() => handleRestoreRequest(session.id)}
                        onRename={(newTitle) =>
                          handleRenameSession(session.id, newTitle)
                        }
                      />
                    </motion.li>
                  ))}
                </motion.ul>
              </div>
            ))}
          </motion.div>
        )}
      </div>

      {/* Confirm Restore Modal */}
      <ConfirmModal
        isOpen={showConfirm}
        title={t('archived.restoreChat')}
        message={t('archived.restoreChatMessage')}
        confirmText={t('archived.restoreButton')}
        cancelText={t('common.cancel')}
        isDanger={false}
        onConfirm={confirmRestore}
        onCancel={() => {
          setShowConfirm(false)
          setSessionToRestore(null)
        }}
      />

      {/* Toast notifications */}
      {shouldRenderToastContainer && (
        <ToastContainer toasts={toast.toasts} onDismiss={toast.dismiss} />
      )}
    </div>
  )
}
