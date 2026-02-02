import EmptyState from '../components/EmptyState'
import { ChatCircle } from '@phosphor-icons/react'

/**
 * SessionsPage - Welcome/Empty state when no session is selected
 * The sidebar handles session creation and navigation
 */
export default function SessionsPage() {
  return (
    <div className="flex-1 flex items-center justify-center bg-gray-50 dark:bg-gray-950">
      <EmptyState
        icon={<ChatCircle size={64} className="text-gray-400 dark:text-gray-600" weight="thin" />}
        title="Welcome to BiChat"
        description="Select a chat from the sidebar or create a new one to get started"
      />
    </div>
  )
}
