import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Archive, CaretRight } from '../icons';
import { UserAvatar } from './UserAvatar';
import { UserFilter } from './UserFilter';
import SessionSkeleton from './SessionSkeleton';
import { EmptyState } from './EmptyState';
import { staggerContainerVariants } from '../animations/variants';
import { useTranslation } from '../hooks/useTranslation';
import type { ChatDataSource, Session, SessionUser } from '../types';
interface AllChatsListProps {
    dataSource: ChatDataSource;
    onSessionSelect: (sessionId: string) => void;
    activeSessionId?: string;
}
export default function AllChatsList(solidProps1: AllChatsListProps) {
    const solidState2 = useTranslation();
    // State
    const [includeArchived, setIncludeArchived] = createSignal(false);
    const [selectedUser, setSelectedUser] = createSignal<SessionUser | null>(null);
    const [offset, setOffset] = createSignal(0);
    const [fetching, setFetching] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [chats, setChats] = createSignal<Session[]>([]);
    const [totalCount, setTotalCount] = createSignal(0);
    const [hasMore, setHasMore] = createSignal(false);
    const [users, setUsers] = createSignal<SessionUser[]>([]);
    const [usersLoading, setUsersLoading] = createSignal(false);
    const limit = 20;
    // Fetch users list
    createEffect(on(() => [solidProps1.dataSource], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.dataSource.listUsers) {
                return;
            }
            let cancelled = false;
            setUsersLoading(true);
            solidProps1.dataSource.listUsers().then((result) => {
                if (!cancelled) {
                    setUsers(result);
                    setUsersLoading(false);
                }
            }).catch(() => {
                if (!cancelled) {
                    setUsersLoading(false);
                }
            });
            return () => { cancelled = true; };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Fetch chats
    createEffect(on(() => [solidProps1.dataSource, offset(), includeArchived(), selectedUser(), solidState2.t], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.dataSource.listAllSessions) {
                return;
            }
            let cancelled = false;
            setFetching(true);
            setError(null);
            solidProps1.dataSource.listAllSessions({
                limit,
                get offset() {
                    return offset();
                },
                get includeArchived() {
                    return includeArchived();
                },
                userId: selectedUser()?.id || null,
            }).then((result) => {
                if (!cancelled) {
                    if (offset() === 0) {
                        setChats(result.sessions);
                    }
                    else {
                        setChats((prev) => [...prev, ...result.sessions]);
                    }
                    setTotalCount(result.total);
                    setHasMore(result.hasMore);
                    setFetching(false);
                }
            }).catch(() => {
                if (!cancelled) {
                    setError(solidState2.t('BiChat.AllChats.FailedToLoad'));
                    setFetching(false);
                }
            });
            return () => { cancelled = true; };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Reset offset when filter changes
    createEffect(on(() => [includeArchived(), selectedUser()], () => {
        const cleanup = untrack(() => {
            setOffset(0);
            setChats([]);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Load more handler
    const handleLoadMore = () => {
        if (!fetching() && hasMore()) {
            setOffset((prev) => prev + limit);
        }
    };
    const loadMoreNodeRef = { current: null } as {
        current: (HTMLDivElement | null) | null;
    };
    const loadMoreRef = (node: HTMLDivElement | null) => {
        loadMoreNodeRef.current = node;
    };
    // Infinite scroll observer
    createEffect(on(() => [fetching(), hasMore(), handleLoadMore], () => {
        const cleanup = untrack(() => {
            const node = loadMoreNodeRef.current;
            if (!node || fetching() || !hasMore()) {
                return;
            }
            const observer = new IntersectionObserver((entries) => {
                if (entries[0].isIntersecting) {
                    handleLoadMore();
                }
            }, { threshold: 0.1 });
            observer.observe(node);
            return () => observer.disconnect();
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Derive unique users from chat data if listUsers is not available
    const derivedUsers = createMemo(() => {
        if (solidProps1.dataSource.listUsers) {
            return users();
        }
        const userMap = new Map<string, SessionUser>();
        chats().forEach((chat) => {
            if (chat.owner && !userMap.has(chat.owner.id)) {
                userMap.set(chat.owner.id, chat.owner);
            }
        });
        return Array.from(userMap.values());
    });
    // Expanded state for user groups (tracks expanded owner IDs; collapsed by default)
    const [expandedGroups, setExpandedGroups] = createSignal<Set<string>>(new Set());
    const toggleGroup = (ownerId: string) => {
        setExpandedGroups((prev) => {
            const next = new Set(prev);
            if (next.has(ownerId)) {
                next.delete(ownerId);
            }
            else {
                next.add(ownerId);
            }
            return next;
        });
    };
    // Auto-expand the group containing the active session (e.g. deep-link)
    createEffect(on(() => [solidProps1.activeSessionId, chats(), selectedUser()], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.activeSessionId || selectedUser()) {
                return;
            }
            const chat = chats().find(c => c.id === solidProps1.activeSessionId);
            if (chat?.owner?.id) {
                setExpandedGroups(prev => {
                    if (prev.has(chat.owner!.id)) {
                        return prev;
                    }
                    return new Set([...prev, chat.owner!.id]);
                });
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Group chats by owner when no specific user is selected
    const groupedChats = createMemo(() => {
        if (selectedUser()) {
            return null;
        } // flat list when user is selected
        const groupMap = new Map<string, {
            owner: SessionUser;
            chats: Session[];
            latestUpdatedAt: string;
        }>();
        chats().forEach((chat) => {
            const owner = chat.owner ?? {
                id: '__unknown__',
                firstName: solidState2.t('BiChat.Common.Untitled'),
                lastName: '',
                initials: '?',
            };
            const ownerId = owner.id;
            if (!groupMap.has(ownerId)) {
                groupMap.set(ownerId, {
                    get owner() {
                        return owner;
                    },
                    chats: [],
                    latestUpdatedAt: chat.updatedAt,
                });
            }
            const group = groupMap.get(ownerId)!;
            group.chats.push(chat);
            // Track the most recent updatedAt for sorting groups
            if (chat.updatedAt > group.latestUpdatedAt) {
                group.latestUpdatedAt = chat.updatedAt;
            }
        });
        // Sort groups by most recently active first
        return Array.from(groupMap.values()).sort((a, b) => b.latestUpdatedAt.localeCompare(a.latestUpdatedAt));
    });
    return (<div class="flex flex-col h-full overflow-hidden" onClick={(e) => e.stopPropagation()} onPointerDown={(e) => e.stopPropagation()}>
      {/* Filter Controls */}
      <div class="px-4 py-3 space-y-3 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
        {/* User filter */}
        <UserFilter users={derivedUsers()} selectedUser={selectedUser()} onUserChange={setSelectedUser} loading={usersLoading() || (fetching() && chats().length === 0)}/>

        {/* Include archived toggle */}
        <label class="flex items-center gap-2 cursor-pointer select-none">
          <input type="checkbox" checked={includeArchived()} onChange={(e) => setIncludeArchived(e.target.checked)} class="
              w-4 h-4 rounded border-gray-300 dark:border-gray-600
              text-primary-600 focus:ring-primary-500 focus:ring-offset-0
              bg-white dark:bg-gray-800
              cursor-pointer
            "/>
          <span class="text-sm text-gray-700 dark:text-gray-300 flex items-center gap-1.5">
            <Archive size={16} className="w-4 h-4"/>
            {solidState2.t('BiChat.AllChats.IncludeArchived')}
          </span>
        </label>

        {/* Results count */}
        {totalCount() > 0 && (<p class="text-xs text-gray-500 dark:text-gray-400">
            {totalCount() === 1
                ? solidState2.t('BiChat.AllChats.ChatFound', { count: totalCount() })
                : solidState2.t('BiChat.AllChats.ChatsFound', { count: totalCount() })}
          </p>)}
      </div>

      {/* Chat List */}
      <nav class="flex-1 overflow-y-auto px-2 pb-4 hide-scrollbar" aria-label={solidState2.t('BiChat.AllChats.OrganizationChats')}>
        {fetching() && chats().length === 0 ? (<SessionSkeleton count={5}/>) : (<>
            {chats().length > 0 ? (<div class="space-y-1 mt-2" role="list" aria-label={solidState2.t('BiChat.AllChats.OrganizationChatSessions')}>
                {groupedChats() ? (
                /* ── Grouped view (no user selected) ── */
                groupedChats()!.map((group) => {
                    const ownerId = group.owner.id;
                    const ownerName = [group.owner.firstName, group.owner.lastName].filter(Boolean).join(' ');
                    const isCollapsed = createMemo(() => !expandedGroups().has(ownerId));
                    return (<div class="mb-1">
                        {/* Group header */}
                        <div role="button" tabIndex={0} onClick={() => toggleGroup(ownerId)} onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault();
                                toggleGroup(ownerId);
                            }
                        }} class="flex items-center gap-2 px-3 py-2 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-800 rounded-lg transition-smooth select-none" aria-expanded={!isCollapsed()}>
                          <CaretRight size={14} weight="bold" className={`shrink-0 text-gray-500 dark:text-gray-400 transition-transform duration-150 ${isCollapsed() ? '' : 'rotate-90'}`}/>
                          <UserAvatar firstName={group.owner.firstName} lastName={group.owner.lastName} initials={group.owner.initials} size="sm"/>
                          <span class="text-sm font-medium text-gray-700 dark:text-gray-300 truncate flex-1 min-w-0">
                            {ownerName || solidState2.t('BiChat.Common.Untitled')}
                          </span>
                          <span class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800 px-2 py-0.5 rounded-full flex-shrink-0">
                            {group.chats.length}
                          </span>
                        </div>

                        {/* Group items */}
                        
                          {!isCollapsed() && (<div class="overflow-hidden">
                              <div class="space-y-0.5 pl-6">
                                {group.chats.map((chat) => (<div>
                                    <div role="link" tabIndex={0} onClick={() => solidProps1.onSessionSelect?.(chat.id)} onKeyDown={(e) => {
                                    if (e.key === 'Enter' || e.key === ' ') {
                                        e.preventDefault();
                                        solidProps1.onSessionSelect?.(chat.id);
                                    }
                                }} class={`
                                        block px-3 py-2 rounded-lg transition-smooth group cursor-pointer
                                        ${chat.id === solidProps1.activeSessionId
                                    ? 'bg-primary-50/50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 border-l-4 border-primary-400 dark:border-primary-600'
                                    : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 border-l-4 border-transparent'}
                                      `} aria-current={chat.id === solidProps1.activeSessionId ? 'page' : undefined}>
                                      <div class="flex items-center gap-2 min-w-0">
                                        <p class="text-sm truncate flex-1 min-w-0">
                                          {chat.title || solidState2.t('BiChat.Common.Untitled')}
                                        </p>
                                        <div class="flex items-center gap-1.5 flex-shrink-0">
                                          {chat.isGroup && chat.memberCount && chat.memberCount > 1 && (<span class="text-xs text-gray-400 dark:text-gray-500">
                                              {chat.memberCount}
                                            </span>)}
                                          {chat.status === 'archived' && (<span class="inline-flex items-center gap-1 px-2 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded-full text-xs">
                                              <Archive size={12} className="w-3 h-3"/>
                                              {solidState2.t('BiChat.Chat.Archived')}
                                            </span>)}
                                        </div>
                                      </div>
                                    </div>
                                  </div>))}
                              </div>
                            </div>)}
                        
                      </div>);
                })) : (
                /* ── Flat view (user selected) ── */
                chats().map((chat) => {
                    const owner = chat.owner ?? {
                        id: '',
                        firstName: '',
                        lastName: '',
                        initials: 'U',
                    };
                    const ownerName = [owner.firstName, owner.lastName].filter(Boolean).join(' ');
                    return (<div>
                        <div role="link" tabIndex={0} onClick={() => solidProps1.onSessionSelect?.(chat.id)} onKeyDown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault();
                                solidProps1.onSessionSelect?.(chat.id);
                            }
                        }} class={`
                            block px-3 py-2 rounded-lg transition-smooth group cursor-pointer
                            ${chat.id === solidProps1.activeSessionId
                            ? 'bg-primary-50/50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-400 border-l-4 border-primary-400 dark:border-primary-600'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 border-l-4 border-transparent'}
                          `} aria-current={chat.id === solidProps1.activeSessionId ? 'page' : undefined}>
                          <div class="flex items-start gap-2">
                            {/* Owner avatar */}
                            <UserAvatar firstName={owner.firstName} lastName={owner.lastName} initials={owner.initials} size="sm"/>

                            {/* Chat info */}
                            <div class="flex-1 min-w-0">
                              <p class="text-sm font-medium truncate">
                                {chat.title || solidState2.t('BiChat.Common.Untitled')}
                              </p>
                              {ownerName && (<p class="text-xs text-gray-500 dark:text-gray-400 truncate">
                                  {ownerName}
                                </p>)}
                              {chat.status === 'archived' && (<span class="inline-flex items-center gap-1 mt-1 px-2 py-0.5 bg-gray-100 dark:bg-gray-800 text-gray-600 dark:text-gray-400 rounded-full text-xs">
                                  <Archive size={12} className="w-3 h-3"/>
                                  {solidState2.t('BiChat.Chat.Archived')}
                                </span>)}
                            </div>
                          </div>
                        </div>
                      </div>);
                }))}

                {/* Load more trigger */}
                {hasMore() && (<div ref={loadMoreRef} class="py-4 text-center">
                    {fetching() ? (<SessionSkeleton count={2}/>) : (<button onClick={handleLoadMore} class="text-sm text-primary-600 dark:text-primary-400 hover:underline">
                        {solidState2.t('BiChat.AllChats.LoadMore')}
                      </button>)}
                  </div>)}
              </div>) : (<EmptyState title={solidState2.t('BiChat.AllChats.NoChatsFound')} description={selectedUser() ? solidState2.t('BiChat.AllChats.NoChatsFromUser', { firstName: selectedUser()!.firstName, lastName: selectedUser()!.lastName })
                    : includeArchived() ? solidState2.t('BiChat.AllChats.NoChatsInOrg')
                        : solidState2.t('BiChat.AllChats.NoActiveChatsInOrg')}/>)}
          </>)}

        {error() && (<div class="mx-2 mt-4 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
            <p class="text-xs text-red-600 dark:text-red-400">
              {error()}
            </p>
          </div>)}
      </nav>
    </div>);
}
