/**
 * UserFilter Component
 * Dropdown to filter chats by user
 * Uses @headlessui/react Menu for accessible dropdown
 */

import { memo } from 'react'
import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react'
import { CaretDown, X } from '@phosphor-icons/react'
import { UserAvatar } from './UserAvatar'
import { useTranslation } from '../hooks/useTranslation'
import type { SessionUser } from '../types'

interface UserFilterProps {
  users: SessionUser[]
  selectedUser: SessionUser | null
  onUserChange: (user: SessionUser | null) => void
  loading?: boolean
}

function UserFilter({ users, selectedUser, onUserChange, loading }: UserFilterProps) {
  const { t } = useTranslation()

  return (
    <div className="relative">
      <Menu>
        {({ open }) => (
          <>
            <MenuButton
              disabled={loading}
              className={`
                w-full px-3 py-2 bg-white dark:bg-gray-800
                border border-gray-300 dark:border-gray-600
                rounded-lg text-sm text-left
                hover:bg-gray-50 dark:hover:bg-gray-700
                focus:outline-none focus:ring-2 focus:ring-primary-500
                transition-smooth
                disabled:opacity-50 disabled:cursor-not-allowed
                flex items-center justify-between gap-2
              `}
              aria-label={t('allChats.allUsers')}
            >
              {selectedUser ? (
                <div className="flex items-center gap-2 min-w-0 flex-1">
                  <UserAvatar
                    firstName={selectedUser.firstName}
                    lastName={selectedUser.lastName}
                    initials={selectedUser.initials}
                    size="sm"
                  />
                  <span className="truncate text-gray-900 dark:text-gray-100">
                    {selectedUser.firstName} {selectedUser.lastName}
                  </span>
                </div>
              ) : (
                <span className="text-gray-500 dark:text-gray-400">
                  {loading ? t('allChats.loadingUsers') : t('allChats.allUsers')}
                </span>
              )}

              <div className="flex items-center gap-1 flex-shrink-0">
                {selectedUser && (
                  <button
                    onClick={(e) => {
                      e.preventDefault()
                      e.stopPropagation()
                      onUserChange(null)
                    }}
                    className="p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 transition-smooth"
                    aria-label={t('common.clear')}
                  >
                    <X size={14} className="w-3.5 h-3.5 text-gray-600 dark:text-gray-400" />
                  </button>
                )}
                <CaretDown
                  size={16}
                  className={`w-4 h-4 text-gray-600 dark:text-gray-400 transition-transform ${
                    open ? 'rotate-180' : ''
                  }`}
                />
              </div>
            </MenuButton>

            <MenuItems
              anchor="bottom start"
              className="
                w-[var(--button-width)]
                max-h-64 overflow-y-auto
                bg-white dark:bg-gray-800
                border border-gray-200 dark:border-gray-700
                rounded-lg shadow-lg
                z-50
                [--anchor-gap:4px]
                mt-1 p-1
              "
            >
              {/* "All users" option */}
              <MenuItem>
                {({ focus }) => (
                  <button
                    onClick={() => onUserChange(null)}
                    className={`
                      w-full text-left px-3 py-2 rounded-lg text-sm
                      transition-smooth
                      ${
                        focus
                          ? 'bg-gray-100 dark:bg-gray-700'
                          : 'hover:bg-gray-50 dark:hover:bg-gray-750'
                      }
                      ${
                        !selectedUser
                          ? 'text-primary-700 dark:text-primary-400 font-medium'
                          : 'text-gray-900 dark:text-gray-100'
                      }
                    `}
                  >
                    {t('allChats.allUsers')}
                  </button>
                )}
              </MenuItem>

              {/* Divider */}
              {users.length > 0 && (
                <div className="border-t border-gray-200 dark:border-gray-700 my-1" />
              )}

              {/* User options */}
              {users.map((user) => (
                <MenuItem key={user.id}>
                  {({ focus }) => (
                    <button
                      onClick={() => onUserChange(user)}
                      className={`
                        w-full text-left px-3 py-2 rounded-lg text-sm
                        transition-smooth
                        flex items-center gap-2
                        ${
                          focus
                            ? 'bg-gray-100 dark:bg-gray-700'
                            : 'hover:bg-gray-50 dark:hover:bg-gray-750'
                        }
                        ${
                          selectedUser?.id === user.id
                            ? 'text-primary-700 dark:text-primary-400 font-medium'
                            : 'text-gray-900 dark:text-gray-100'
                        }
                      `}
                    >
                      <UserAvatar
                        firstName={user.firstName}
                        lastName={user.lastName}
                        initials={user.initials}
                        size="sm"
                      />
                      <span className="truncate">
                        {user.firstName} {user.lastName}
                      </span>
                    </button>
                  )}
                </MenuItem>
              ))}

              {/* Empty state */}
              {users.length === 0 && (
                <div className="px-3 py-6 text-center">
                  <p className="text-sm text-gray-500 dark:text-gray-400">{t('allChats.noUsersFound')}</p>
                </div>
              )}
            </MenuItems>
          </>
        )}
      </Menu>
    </div>
  )
}

const MemoizedUserFilter = memo(UserFilter)
MemoizedUserFilter.displayName = 'UserFilter'

export { MemoizedUserFilter as UserFilter }
export default MemoizedUserFilter
