/**
 * SessionMenu Component
 * Dropdown menu for session actions (pin, rename, regenerate title, delete)
 */

import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react'
import { DotsThree, Check, Bookmark, PencilSimple, Trash } from '@phosphor-icons/react'

interface SessionMenuProps {
  isPinned: boolean
  onPin: (e: React.MouseEvent) => void
  onRename: () => void
  onDelete: (e: React.MouseEvent) => void
}

export default function SessionMenu({ isPinned, onPin, onRename, onDelete }: SessionMenuProps) {
  return (
    <Menu>
      <MenuButton
        onClick={(e: React.MouseEvent) => {
          e.preventDefault()
          e.stopPropagation()
        }}
        className="opacity-0 group-hover:opacity-100 p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-all flex-shrink-0"
        aria-label="Chat options"
      >
        <DotsThree size={16} className="w-4 h-4" weight="bold" />
      </MenuButton>
      <MenuItems
        anchor="bottom start"
        className="w-52 bg-white/95 dark:bg-gray-900/95 backdrop-blur rounded-xl shadow-xl border border-gray-200 dark:border-gray-700 z-30 [--anchor-gap:8px] mt-1 p-2 space-y-1"
      >
        <MenuItem>
          {({ focus }) => (
            <button
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onPin(e)
              }}
              className={`flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 transition-colors ${
                focus
                  ? 'bg-gray-100 dark:bg-gray-800/70 ring-1 ring-gray-200/80 dark:ring-gray-700/80'
                  : 'hover:bg-gray-50 dark:hover:bg-gray-800'
              }`}
              aria-label={isPinned ? 'Unpin chat' : 'Pin chat'}
            >
              {isPinned ? (
                <Check size={16} className="w-4 h-4" />
              ) : (
                <Bookmark size={16} className="w-4 h-4" />
              )}
              {isPinned ? 'Unpin' : 'Pin'} Chat
            </button>
          )}
        </MenuItem>
        <MenuItem>
          {({ focus, close }) => (
            <button
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onRename()
                close()
              }}
              className={`flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 transition-colors ${
                focus
                  ? 'bg-gray-100 dark:bg-gray-800/70 ring-1 ring-gray-200/80 dark:ring-gray-700/80'
                  : 'hover:bg-gray-50 dark:hover:bg-gray-800'
              }`}
              aria-label="Rename chat"
            >
              <PencilSimple size={16} className="w-4 h-4" />
              Rename Chat
            </button>
          )}
        </MenuItem>
        {/* Regenerate title removed - users can manually rename sessions */}
        <MenuItem>
          {({ focus }) => (
            <button
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onDelete(e)
              }}
              className={`flex w-full items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium text-red-600 dark:text-red-400 transition-colors ${
                focus
                  ? 'bg-red-50 dark:bg-red-900/20 ring-1 ring-red-200/70 dark:ring-red-500/30'
                  : 'hover:bg-red-50/70 dark:hover:bg-red-900/10'
              }`}
              aria-label="Delete chat"
            >
              <Trash size={16} className="w-4 h-4" />
              Delete Chat
            </button>
          )}
        </MenuItem>
      </MenuItems>
    </Menu>
  )
}
