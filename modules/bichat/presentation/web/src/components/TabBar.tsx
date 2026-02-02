/**
 * TabBar Component
 * Horizontal tabs for switching between "My Chats" and "All Chats"
 */

import { memo } from 'react'
import { motion } from 'framer-motion'

type Tab = 'my-chats' | 'all-chats'

interface TabBarProps {
  activeTab: Tab
  onTabChange: (tab: Tab) => void
  showAllChats: boolean
}

function TabBar({ activeTab, onTabChange, showAllChats }: TabBarProps) {
  // If user doesn't have ReadAll permission, don't show tabs at all
  if (!showAllChats) {
    return null
  }

  return (
    <div
      className="flex gap-1 px-4 pt-4 pb-2 border-b border-gray-200 dark:border-gray-700"
      role="tablist"
      aria-label="Chat view selector"
    >
      <TabButton
        id="my-chats"
        label="My Chats"
        isActive={activeTab === 'my-chats'}
        onClick={() => onTabChange('my-chats')}
      />
      <TabButton
        id="all-chats"
        label="All Chats"
        isActive={activeTab === 'all-chats'}
        onClick={() => onTabChange('all-chats')}
      />
    </div>
  )
}

interface TabButtonProps {
  id: string
  label: string
  isActive: boolean
  onClick: () => void
}

function TabButton({ id, label, isActive, onClick }: TabButtonProps) {
  return (
    <button
      id={id}
      role="tab"
      aria-selected={isActive}
      aria-controls={`${id}-panel`}
      onClick={onClick}
      className={`
        relative px-4 py-2 rounded-t-lg text-sm font-medium transition-colors
        ${
          isActive
            ? 'text-primary-700 dark:text-primary-400'
            : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200'
        }
      `}
    >
      {label}

      {/* Active indicator */}
      {isActive && (
        <motion.div
          layoutId="activeTab"
          className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary-600 dark:bg-primary-500"
          transition={{ type: 'spring', stiffness: 380, damping: 30 }}
        />
      )}
    </button>
  )
}

export default memo(TabBar)
