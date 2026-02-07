/**
 * SessionMenu Component
 * Dropdown menu for session actions (pin, rename, regenerate title, delete)
 */

import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react'
import { DotsThreeVertical, Check, Bookmark, PencilSimple, Trash } from '@phosphor-icons/react'

interface SessionMenuProps {
  isPinned: boolean
  onPin: (e: React.MouseEvent) => void
  onRename: () => void
  onDelete: (e?: React.MouseEvent) => void
}

export default function SessionMenu({ isPinned, onPin, onRename, onDelete }: SessionMenuProps) {
  return (
    <Menu>
      <MenuButton
        onClick={(e: React.MouseEvent) => {
          e.preventDefault()
          e.stopPropagation()
        }}
        className="opacity-0 group-hover:opacity-100 p-1 rounded cursor-pointer hover:bg-gray-200 dark:hover:bg-gray-700 active:bg-gray-300 dark:active:bg-gray-600 transition-all duration-150 flex-shrink-0 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
        aria-label="Chat options"
      >
        <DotsThreeVertical size={16} className="w-4 h-4" weight="bold" />
      </MenuButton>
      <MenuItems
        anchor="bottom start"
        className="w-48 bg-white/95 dark:bg-gray-900/95 backdrop-blur-lg rounded-xl shadow-lg border border-gray-200/80 dark:border-gray-700/60 z-30 [--anchor-gap:8px] mt-1 p-1.5"
      >
        <MenuItem>
          {({ focus }) => (
            <button
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onPin(e)
              }}
              className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${
                focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''
              }`}
              aria-label={isPinned ? 'Unpin chat' : 'Pin chat'}
            >
              {isPinned ? (
                <Check size={15} className="text-gray-400 dark:text-gray-500" />
              ) : (
                <Bookmark size={15} className="text-gray-400 dark:text-gray-500" />
              )}
              {isPinned ? 'Unpin' : 'Pin'}
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
              className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-gray-600 dark:text-gray-300 transition-colors ${
                focus ? 'bg-gray-100 dark:bg-gray-800/70' : ''
              }`}
              aria-label="Rename chat"
            >
              <PencilSimple size={15} className="text-gray-400 dark:text-gray-500" />
              Rename
            </button>
          )}
        </MenuItem>
        <MenuItem>
          {({ focus }) => (
            <button
              onClick={(e) => {
                e.preventDefault()
                e.stopPropagation()
                onDelete(e)
              }}
              className={`flex w-full items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] text-red-500 dark:text-red-400 transition-colors ${
                focus ? 'bg-red-50 dark:bg-red-900/20' : ''
              }`}
              aria-label="Delete chat"
            >
              <Trash size={15} className="text-red-400 dark:text-red-500" />
              Delete
            </button>
          )}
        </MenuItem>
      </MenuItems>
    </Menu>
  )
}
