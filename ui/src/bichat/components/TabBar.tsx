/**
 * TabBar Component
 * Horizontal tabs with animated indicator for switching between views
 * Generic: accepts any set of tabs via props
 */

import { memo, useRef, useCallback } from 'react'
import { motion } from 'framer-motion'

interface TabBarProps {
  tabs: Array<{ id: string; label: string }>
  activeTab: string
  onTabChange: (tabId: string) => void
}

function TabBar({ tabs, activeTab, onTabChange }: TabBarProps) {
  const tablistRef = useRef<HTMLDivElement>(null)

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      const currentIndex = tabs.findIndex((tab) => tab.id === activeTab)
      if (currentIndex < 0) return

      let nextIndex: number | null = null

      switch (e.key) {
        case 'ArrowRight':
          e.preventDefault()
          nextIndex = (currentIndex + 1) % tabs.length
          break
        case 'ArrowLeft':
          e.preventDefault()
          nextIndex = (currentIndex - 1 + tabs.length) % tabs.length
          break
        case 'Home':
          e.preventDefault()
          nextIndex = 0
          break
        case 'End':
          e.preventDefault()
          nextIndex = tabs.length - 1
          break
      }

      if (nextIndex !== null) {
        onTabChange(tabs[nextIndex].id)
        // Focus the newly activated tab button
        const tablist = tablistRef.current
        if (tablist) {
          const buttons = tablist.querySelectorAll<HTMLElement>('[role="tab"]')
          buttons[nextIndex]?.focus()
        }
      }
    },
    [tabs, activeTab, onTabChange]
  )

  if (tabs.length === 0) {
    return null
  }

  return (
    <div
      ref={tablistRef}
      className="flex gap-1 px-4 pt-4 pb-2 border-b border-gray-200 dark:border-gray-700"
      role="tablist"
      onKeyDown={handleKeyDown}
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
      tabIndex={isActive ? 0 : -1}
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
