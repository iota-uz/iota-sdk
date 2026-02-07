/**
 * Chat header component
 * Displays session title and controls
 *
 * Supports customization via:
 * - logoSlot: Custom logo component
 * - actionsSlot: Custom action buttons
 * - Translations for "New Chat", "Archived", etc.
 */

import { ReactNode } from 'react'
import { Session } from '../types'
import { useTranslation } from '../hooks/useTranslation'
import { useBranding } from '../hooks/useBranding'

interface ChatHeaderProps {
  session: Session | null
  onBack?: () => void
  readOnly?: boolean
  /** Custom logo component to display */
  logoSlot?: ReactNode
  /** Custom action buttons */
  actionsSlot?: ReactNode
}

export function ChatHeader({ session, onBack, readOnly, logoSlot, actionsSlot }: ChatHeaderProps) {
  const { t } = useTranslation()
  const branding = useBranding()

  const BackButton = onBack ? (
    <button
      onClick={onBack}
      className="p-2 hover:bg-gray-100 dark:hover:bg-gray-700 active:bg-gray-200 dark:active:bg-gray-600 rounded-lg transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
      aria-label={t('chat.goBack')}
    >
      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
    </button>
  ) : null

  const Logo = logoSlot || (branding.logoUrl ? (
    <img src={branding.logoUrl} alt={branding.appName} className="h-6 w-auto" />
  ) : null)

  if (!session) {
    return (
      <header className="bichat-header border-b border-gray-200 dark:border-gray-700 px-4 py-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {BackButton}
            {Logo}
            <h1 className="text-lg font-semibold text-[var(--bichat-text)]">
              {t('chat.newChat')}
            </h1>
          </div>
          {actionsSlot && <div className="flex items-center gap-2">{actionsSlot}</div>}
        </div>
      </header>
    )
  }

  return (
    <header className="bichat-header border-b border-gray-200 dark:border-gray-700 px-4 py-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {BackButton}
          {Logo}
          <h1 className="text-lg font-semibold text-[var(--bichat-text)]">{session.title}</h1>
          {session.pinned && (
            <svg
              className="w-4 h-4 text-[var(--bichat-primary)]"
              fill="currentColor"
              viewBox="0 0 20 20"
              aria-label={t('chat.pinned')}
            >
              <path d="M10 2a1 1 0 011 1v1.323l3.954 1.582 1.599-.8a1 1 0 01.894 1.79l-1.233.616 1.738 5.42a1 1 0 01-.285 1.05A3.989 3.989 0 0115 15a3.989 3.989 0 01-2.667-1.019 1 1 0 01-.285-1.05l1.715-5.349L11 6.477V16h2a1 1 0 110 2H7a1 1 0 110-2h2V6.477L6.237 7.582l1.715 5.349a1 1 0 01-.285 1.05A3.989 3.989 0 015 15a3.989 3.989 0 01-2.667-1.019 1 1 0 01-.285-1.05l1.738-5.42-1.233-.617a1 1 0 01.894-1.788l1.599.799L9 4.323V3a1 1 0 011-1z" />
            </svg>
          )}
        </div>
        <div className="flex items-center gap-2">
          {readOnly && (
            <span className="px-2 py-1 text-xs bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-200 rounded">
              {t('chat.readOnly')}
            </span>
          )}
          {session.status === 'archived' && (
            <span className="px-2 py-1 text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded">
              {t('chat.archived')}
            </span>
          )}
          {actionsSlot}
        </div>
      </div>
    </header>
  )
}
