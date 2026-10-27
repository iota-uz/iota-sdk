import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * UserFilter Component
 * Dropdown to filter chats by user
 * Uses @headlessui/react Menu for accessible dropdown
 */
import { Menu, MenuButton, MenuItem, MenuItems } from './Menu';
import { CaretDown, X } from '../icons';
import { UserAvatar } from './UserAvatar';
import { useTranslation } from '../hooks/useTranslation';
import type { SessionUser } from '../types';
interface UserFilterProps {
    users: SessionUser[];
    selectedUser: SessionUser | null;
    onUserChange: (user: SessionUser | null) => void;
    loading?: boolean;
}
function UserFilter(solidProps1: UserFilterProps) {
    const solidState2 = useTranslation();
    return (<div class="relative">
      <Menu>
        {(solidProps3) => (<>
            <MenuButton disabled={solidProps1.loading} className={`
                cursor-pointer w-full px-3 py-2 bg-white dark:bg-gray-800
                border border-gray-300 dark:border-gray-600
                rounded-lg text-sm text-left
                hover:bg-gray-50 dark:hover:bg-gray-700
                focus:outline-none focus:ring-2 focus:ring-primary-500
                transition-smooth
                disabled:opacity-50 disabled:cursor-not-allowed
                flex items-center justify-between gap-2
              `} aria-label={solidState2.t('BiChat.AllChats.AllUsers')}>
              {solidProps1.selectedUser ? (<div class="flex items-center gap-2 min-w-0 flex-1">
                  <UserAvatar firstName={solidProps1.selectedUser.firstName} lastName={solidProps1.selectedUser.lastName} initials={solidProps1.selectedUser.initials} size="sm"/>
                  <span class="truncate text-gray-900 dark:text-gray-100">
                    {solidProps1.selectedUser.firstName} {solidProps1.selectedUser.lastName}
                  </span>
                </div>) : (<span class="text-gray-500 dark:text-gray-400">
                  {solidProps1.loading ? solidState2.t('BiChat.AllChats.LoadingUsers') : solidState2.t('BiChat.AllChats.AllUsers')}
                </span>)}

              <div class="flex items-center gap-1 flex-shrink-0">
                {solidProps1.selectedUser && (<button onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    solidProps1.onUserChange?.(null);
                }} class="cursor-pointer p-1 rounded hover:bg-gray-200 dark:hover:bg-gray-600 transition-smooth" aria-label={solidState2.t('BiChat.Common.Clear')}>
                    <X size={14} className="w-3.5 h-3.5 text-gray-600 dark:text-gray-400"/>
                  </button>)}
                <CaretDown size={16} className={`w-4 h-4 text-gray-600 dark:text-gray-400 transition-transform ${solidProps3.open ? 'rotate-180' : ''}`}/>
              </div>
            </MenuButton>

            <MenuItems anchor="bottom start" className="
                w-[var(--button-width)]
                max-h-64 overflow-y-auto
                bg-white dark:bg-gray-800
                border border-gray-200 dark:border-gray-700
                rounded-lg shadow-lg
                z-50
                [--anchor-gap:4px]
                mt-1 p-1
              ">
              {/* "All users" option */}
              <MenuItem>
                {(solidProps4) => (<button onClick={() => solidProps1.onUserChange?.(null)} class={`
                      cursor-pointer w-full text-left px-3 py-2 rounded-lg text-sm
                      transition-smooth
                      ${solidProps4.focus ? 'bg-gray-100 dark:bg-gray-700'
                    : 'hover:bg-gray-50 dark:hover:bg-gray-750'}
                      ${!solidProps1.selectedUser
                    ? 'text-primary-700 dark:text-primary-400 font-medium'
                    : 'text-gray-900 dark:text-gray-100'}
                    `}>
                    {solidState2.t('BiChat.AllChats.AllUsers')}
                  </button>)}
              </MenuItem>

              {/* Divider */}
              {solidProps1.users.length > 0 && (<div class="border-t border-gray-200 dark:border-gray-700 my-1"/>)}

              {/* User options */}
              {solidProps1.users.map((user) => (<MenuItem>
                  {(solidProps5) => (<button onClick={() => solidProps1.onUserChange?.(user)} class={`
                        cursor-pointer w-full text-left px-3 py-2 rounded-lg text-sm
                        transition-smooth
                        flex items-center gap-2
                        ${solidProps5.focus ? 'bg-gray-100 dark:bg-gray-700'
                        : 'hover:bg-gray-50 dark:hover:bg-gray-750'}
                        ${solidProps1.selectedUser?.id === user.id
                        ? 'text-primary-700 dark:text-primary-400 font-medium'
                        : 'text-gray-900 dark:text-gray-100'}
                      `}>
                      <UserAvatar firstName={user.firstName} lastName={user.lastName} initials={user.initials} size="sm"/>
                      <span class="truncate">
                        {user.firstName} {user.lastName}
                      </span>
                    </button>)}
                </MenuItem>))}

              {/* Empty state */}
              {solidProps1.users.length === 0 && (<div class="px-3 py-6 text-center">
                  <p class="text-sm text-gray-700 dark:text-gray-300">{solidState2.t('BiChat.AllChats.NoUsersFound')}</p>
                </div>)}
            </MenuItems>
          </>)}
      </Menu>
    </div>);
}
const MemoizedUserFilter = UserFilter;
MemoizedUserFilter; /* Solid components are named by their declarations. */
export { MemoizedUserFilter as UserFilter };
export default MemoizedUserFilter;
