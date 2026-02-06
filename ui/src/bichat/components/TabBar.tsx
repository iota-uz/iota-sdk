/**
 * TabBar Component
 * Horizontal tabs with animated indicator for switching between views
 * Generic: accepts any set of tabs via props
 */

import { memo } from 'react'
import { motion } from 'framer-motion'

interface TabBarProps {
  tabs: Array<{ id: string; label: string }>
  activeTab: string
  onTabChange: (tabId: string) => void
}

function TabBar({ tabs, activeTab, onTabChange }: TabBarProps) {
  if (tabs.length === 0) {
    return null
  }

  return (
    <div
      className="flex gap-1 px-4 pt-4 pb-2 border-b border-gray-200 dark:border-gray-700"
      role="tablist"
    >
      {tabs.map((tab) => (
        <TabButton
          key={tab.id}
          id={tab.id}
          label={tab.label}
          isActive={activeTab === tab.id}
          onClick={() => onTabChange(tab.id)}
        />
      ))}
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
        relative px-4 py-2 rounded-t-lg text-sm font-medium transition-smooth focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50
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
          transition={{ duration: 0.2, ease: [0.4, 0, 0.2, 1] }}
        />
      )}
    </button>
  )
}

const MemoizedTabBar = memo(TabBar)
MemoizedTabBar.displayName = 'TabBar'

export { MemoizedTabBar as TabBar }
export default MemoizedTabBar
