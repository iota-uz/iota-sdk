import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * SessionMembersModal
 * Polished sharing dialog for managing session members.
 * Uses InlineDialog, UserAvatars, custom search dropdown, and segmented role controls.
 */
import { InlineDialog, InlineDialogBackdrop, InlineDialogPanel, InlineDialogTitle, } from './InlineDialog';
import { UserPlus, Trash, Crown, UsersThree, MagnifyingGlass, X } from '../icons';
import type { ChatDataSource, SessionMember, SessionUser } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { UserAvatar } from './UserAvatar';
import { ConfirmModal } from './ConfirmModal';
export interface SessionMembersModalProps {
    isOpen: boolean;
    sessionId?: string;
    dataSource: ChatDataSource;
    onClose: () => void;
}
// ---------------------------------------------------------------------------
// RoleSegmentedControl
// ---------------------------------------------------------------------------
const ROLES: readonly [
    'editor',
    'viewer'
] = ['editor', 'viewer'];
function RoleSegmentedControl(solidProps1Input: {
    value: 'editor' | 'viewer';
    onChange: (role: 'editor' | 'viewer') => void;
    disabled?: boolean;
    size?: 'sm' | 'md';
    t: (key: string) => string;
}) {
    const solidProps1 = mergeProps({ size: 'md' } as const, solidProps1Input);
    const btnBase = createMemo(() => solidProps1.size === 'sm'
        ? 'px-2 py-0.5 text-[11px]'
        : 'px-3 py-1 text-xs');
    const currentIndex = createMemo(() => ROLES.indexOf(solidProps1.value));
    const handleKeyDown = (e: KeyboardEvent & {
        currentTarget: HTMLDivElement;
    }) => {
        if (solidProps1.disabled) {
            return;
        }
        let nextIndex: number | null = null;
        if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
            e.preventDefault();
            nextIndex = currentIndex() <= 0 ? ROLES.length - 1 : currentIndex() - 1;
        }
        else if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
            e.preventDefault();
            nextIndex = currentIndex() >= ROLES.length - 1 ? 0 : currentIndex() + 1;
        }
        if (nextIndex !== null) {
            const nextRole = ROLES[nextIndex];
            solidProps1.onChange?.(nextRole);
            const target = e.currentTarget.querySelector(`[data-role="${nextRole}"]`) as HTMLButtonElement | null;
            target?.focus();
        }
    };
    return (<div role="radiogroup" aria-label={solidProps1.t('BiChat.Share.RoleLabel')} class="inline-flex rounded-lg border border-gray-200 dark:border-gray-700 p-0.5 bg-gray-50 dark:bg-gray-800/50" onKeyDown={handleKeyDown}>
      {ROLES.map((role) => (<button type="button" role="radio" aria-checked={solidProps1.value === role} tabIndex={solidProps1.value === role ? 0 : -1} data-role={role} disabled={solidProps1.disabled} onClick={() => solidProps1.onChange?.(role)} class={`${btnBase()} cursor-pointer rounded-md font-medium transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 disabled:opacity-50 disabled:cursor-not-allowed ${solidProps1.value === role
                ? 'bg-primary-600 text-white shadow-sm'
                : 'text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-gray-100'}`}>
          {role === 'editor' ? solidProps1.t('BiChat.Share.RoleEditor') : solidProps1.t('BiChat.Share.RoleViewer')}
        </button>))}
    </div>);
}
// ---------------------------------------------------------------------------
// Skeleton
// ---------------------------------------------------------------------------
function MemberSkeleton() {
    return (<div class="flex items-center gap-3 rounded-xl px-3 py-2.5" aria-hidden="true">
      <div class="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700 animate-pulse flex-shrink-0"/>
      <div class="flex-1 space-y-1.5">
        <div class="h-3 w-28 rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
        <div class="h-2.5 w-16 rounded bg-gray-100 dark:bg-gray-800 animate-pulse"/>
      </div>
      <div class="h-6 w-16 rounded-lg bg-gray-200 dark:bg-gray-700 animate-pulse"/>
    </div>);
}
// ---------------------------------------------------------------------------
// SessionMembersModal
// ---------------------------------------------------------------------------
export function SessionMembersModal(solidProps2: SessionMembersModalProps) {
    const headingId = createUniqueId();
    const solidState3 = useTranslation();
    const statusTimerRef = { current: undefined } as {
        current: (ReturnType<typeof setTimeout> | undefined);
    };
    const [loading, setLoading] = createSignal(false);
    const [saving, setSaving] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [users, setUsers] = createSignal<SessionUser[]>([]);
    const [members, setMembers] = createSignal<SessionMember[]>([]);
    const [selectedUser, setSelectedUser] = createSignal<SessionUser | null>(null);
    const [selectedRole, setSelectedRole] = createSignal<'editor' | 'viewer'>('editor');
    const [query, setQuery] = createSignal('');
    const [confirmRemove, setConfirmRemove] = createSignal<SessionMember | null>(null);
    const [statusMessage, setStatusMessage] = createSignal<string | null>(null);
    const [dropdownOpen, setDropdownOpen] = createSignal(false);
    const [dropdownHighlightIndex, setDropdownHighlightIndex] = createSignal(0);
    const dropdownOptionRefs = { current: [] } as {
        current: (HTMLButtonElement | null)[];
    };
    const canManageMembers = Boolean(solidProps2.dataSource.listUsers
        && solidProps2.dataSource.listSessionMembers
        && solidProps2.dataSource.addSessionMember
        && solidProps2.dataSource.updateSessionMemberRole
        && solidProps2.dataSource.removeSessionMember);
    const refresh = async () => {
        if (!solidProps2.sessionId || !canManageMembers) {
            return;
        }
        setLoading(true);
        setError(null);
        try {
            const [usersData, membersData] = await Promise.all([
                solidProps2.dataSource.listUsers!(),
                solidProps2.dataSource.listSessionMembers!(solidProps2.sessionId),
            ]);
            setUsers(usersData);
            setMembers(membersData);
        }
        catch {
            setError(solidState3.t('BiChat.Share.LoadFailed'));
        }
        finally {
            setLoading(false);
        }
    };
    createEffect(on(() => [solidProps2.isOpen, refresh], () => {
        const cleanup = untrack(() => {
            if (!solidProps2.isOpen) {
                return;
            }
            void refresh();
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Reset state when modal closes
    createEffect(on(() => [solidProps2.isOpen], () => {
        const cleanup = untrack(() => {
            if (!solidProps2.isOpen) {
                setQuery('');
                setSelectedUser(null);
                setSelectedRole('editor');
                setError(null);
                setConfirmRemove(null);
                setStatusMessage(null);
                setDropdownOpen(false);
                setDropdownHighlightIndex(0);
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const memberIDs = createMemo(() => new Set(members().map((m) => m.user.id)));
    const availableUsers = createMemo(() => users().filter((user) => !memberIDs().has(user.id)));
    const filteredUsers = createMemo(() => {
        if (!query().trim()) {
            return availableUsers();
        }
        const q = createMemo(() => query().toLowerCase());
        return availableUsers().filter((u) => u.firstName.toLowerCase().includes(q())
            || u.lastName.toLowerCase().includes(q())
            || `${u.firstName} ${u.lastName}`.toLowerCase().includes(q()));
    });
    // Reset dropdown highlight when filtered list changes
    createEffect(on(() => [filteredUsers().length], () => {
        const cleanup = untrack(() => {
            setDropdownHighlightIndex((i) => Math.min(Math.max(0, i), Math.max(0, filteredUsers().length - 1)));
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    onMount(() => {
        const cleanup = untrack(() => () => clearTimeout(statusTimerRef.current));
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const flashStatus = (msg: string) => {
        clearTimeout(statusTimerRef.current);
        setStatusMessage(msg);
        statusTimerRef.current = setTimeout(() => setStatusMessage(null), 3000);
    };
    const handleAdd = async () => {
        const user = selectedUser();
        if (!solidProps2.sessionId || !user || !solidProps2.dataSource.addSessionMember) {
            return;
        }
        setSaving(true);
        setError(null);
        try {
            await solidProps2.dataSource.addSessionMember(solidProps2.sessionId, user.id, selectedRole());
            setSelectedUser(null);
            setQuery('');
            flashStatus(solidState3.t('BiChat.Share.MemberAdded'));
            await refresh();
        }
        catch {
            setError(solidState3.t('BiChat.Share.AddFailed'));
        }
        finally {
            setSaving(false);
        }
    };
    const handleUpdateRole = async (userId: string, role: 'editor' | 'viewer') => {
        if (!solidProps2.sessionId || !solidProps2.dataSource.updateSessionMemberRole) {
            return;
        }
        setSaving(true);
        setError(null);
        try {
            await solidProps2.dataSource.updateSessionMemberRole(solidProps2.sessionId, userId, role);
            await refresh();
        }
        catch {
            setError(solidState3.t('BiChat.Share.UpdateFailed'));
        }
        finally {
            setSaving(false);
        }
    };
    const handleRemove = async (userId: string) => {
        if (!solidProps2.sessionId || !solidProps2.dataSource.removeSessionMember) {
            return;
        }
        setSaving(true);
        setError(null);
        try {
            await solidProps2.dataSource.removeSessionMember(solidProps2.sessionId, userId);
            flashStatus(solidState3.t('BiChat.Share.MemberRemoved'));
            await refresh();
        }
        catch {
            setError(solidState3.t('BiChat.Share.RemoveFailed'));
        }
        finally {
            setSaving(false);
        }
    };
    return (<>
      <InlineDialog open={solidProps2.isOpen} onClose={solidProps2.onClose} className="relative z-40">
        <InlineDialogBackdrop className="fixed inset-0 bg-black/40 dark:bg-black/60 backdrop-blur-sm transition-opacity duration-200"/>

        <div class="fixed inset-0 flex items-center justify-center z-50 p-4">
          <InlineDialogPanel aria-labelledby={headingId} className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl dark:shadow-2xl dark:shadow-black/30 max-w-md w-full">
            {/* Header */}
            <div class="flex items-center justify-between px-6 pt-5 pb-4">
              <div class="flex items-center gap-3">
                <div class="flex items-center justify-center w-9 h-9 rounded-xl bg-primary-50 dark:bg-primary-950/40 border border-primary-200/60 dark:border-primary-800/40">
                  <UsersThree size={18} weight="duotone" className="text-primary-600 dark:text-primary-400"/>
                </div>
                <InlineDialogTitle id={headingId} className="text-base font-semibold text-gray-900 dark:text-gray-100">
                  {solidState3.t('BiChat.Share.Title')}
                </InlineDialogTitle>
              </div>
              <button type="button" onClick={solidProps2.onClose} class="rounded-lg p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 dark:hover:text-gray-300 dark:hover:bg-gray-700 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={solidState3.t('BiChat.Common.Close')}>
                <X size={18}/>
              </button>
            </div>

            {/* Body */}
            <div class="px-6 pb-5 space-y-4">
              {!canManageMembers && (<div class="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300">
                  {solidState3.t('BiChat.Share.Unsupported')}
                </div>)}

              {error() && (<div class="rounded-xl border border-red-200 bg-red-50 px-3 py-2.5 text-xs text-red-700 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-300">
                  {error()}
                </div>)}

              {/* Status announcements for screen readers */}
              <div aria-live="polite" aria-atomic="true" class="sr-only">
                {statusMessage()}
              </div>

              {/* Members list */}
              <div>
                <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {solidState3.t('BiChat.Share.Members')}{!loading() && members().length > 0 ? ` (${members().length})` : ''}
                </h3>

                <div class="max-h-64 overflow-y-auto -mx-1 px-1 space-y-1">
                  {loading() ? (<>
                      <MemberSkeleton />
                      <MemberSkeleton />
                      <MemberSkeleton />
                    </>) : members().length === 0 ? (<div class="flex flex-col items-center justify-center py-8 text-gray-400 dark:text-gray-500">
                      <UsersThree size={32} weight="thin" className="mb-2"/>
                      <p class="text-sm">{solidState3.t('BiChat.Share.Empty')}</p>
                    </div>) : (members().map((member) => (<div class="flex items-center gap-3 rounded-xl px-3 py-2 transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/40">
                        <UserAvatar firstName={member.user.firstName} lastName={member.user.lastName} initials={member.user.initials} size="sm"/>
                        <div class="flex-1 min-w-0">
                          <div class="truncate text-sm font-medium text-gray-900 dark:text-gray-100">
                            {member.user.firstName} {member.user.lastName}
                          </div>
                        </div>

                        {member.role === 'owner' ? (<span class="inline-flex items-center gap-1 rounded-full bg-amber-50 dark:bg-amber-900/20 border border-amber-200/60 dark:border-amber-800/40 px-2.5 py-0.5 text-[11px] font-medium text-amber-700 dark:text-amber-300">
                            <Crown size={12} weight="duotone"/>
                            {solidState3.t('BiChat.Share.RoleOwner')}
                          </span>) : (<div class="flex items-center gap-2 flex-shrink-0">
                            <RoleSegmentedControl value={member.role} onChange={(role) => handleUpdateRole(member.user.id, role)} disabled={saving()} size="sm" t={solidState3.t}/>
                            <button type="button" disabled={saving()} onClick={() => setConfirmRemove(member)} class="cursor-pointer rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400 disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-500/50" aria-label={`${solidState3.t('BiChat.Share.Remove')} ${member.user.firstName} ${member.user.lastName}`}>
                              <Trash size={14}/>
                            </button>
                          </div>)}
                      </div>)))}
                </div>
              </div>

              {/* Add member */}
              {canManageMembers && (<div class="rounded-xl border border-gray-200 dark:border-gray-700 p-3 space-y-3">
                  <h3 class="text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                    {solidState3.t('BiChat.Share.AddMember')}
                  </h3>

                  {/* User search + dropdown — outer container owns blur-to-close so keyboard nav does not close dropdown */}
                  <div class="relative" onBlur={(e: FocusEvent & {
                currentTarget: HTMLDivElement;
            }) => {
                if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                    setDropdownOpen(false);
                }
            }}>
                    <div class="relative">
                      <MagnifyingGlass size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"/>
                      <input type="text" class="w-full rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60 pl-8 pr-3 py-2 text-sm text-gray-900 dark:text-gray-100 placeholder-gray-400 dark:placeholder-gray-500 transition-colors focus:border-primary-400 dark:focus:border-primary-600 focus:outline-none focus:ring-2 focus:ring-primary-500/20" placeholder={solidState3.t('BiChat.Share.SearchUsers')} value={selectedUser() ? `${selectedUser()!.firstName} ${selectedUser()!.lastName}` : query()} onFocus={() => {
                setDropdownOpen(true);
                setDropdownHighlightIndex(0);
                if (selectedUser()) {
                    setSelectedUser(null);
                    setQuery('');
                }
            }} onInput={(e) => { setQuery(e.target.value); setSelectedUser(null); setDropdownOpen(true); setDropdownHighlightIndex(0); }} onKeyDown={(e) => {
                if (!dropdownOpen() || filteredUsers().length === 0) {
                    if (e.key === 'Escape') {
                        setDropdownOpen(false);
                    }
                    return;
                }
                if (e.key === 'Escape') {
                    e.preventDefault();
                    setDropdownOpen(false);
                    setDropdownHighlightIndex(0);
                    return;
                }
                if (e.key === 'ArrowDown') {
                    e.preventDefault();
                    const next = createMemo(() => (dropdownHighlightIndex() + 1) % filteredUsers().length);
                    setDropdownHighlightIndex(next());
                    setTimeout(() => dropdownOptionRefs.current[next()]?.focus(), 0);
                    return;
                }
                if (e.key === 'ArrowUp') {
                    e.preventDefault();
                    const next = createMemo(() => dropdownHighlightIndex() <= 0 ? filteredUsers().length - 1 : dropdownHighlightIndex() - 1);
                    setDropdownHighlightIndex(next());
                    setTimeout(() => dropdownOptionRefs.current[next()]?.focus(), 0);
                    return;
                }
                if (e.key === 'Enter') {
                    e.preventDefault();
                    const user = createMemo(() => filteredUsers()[dropdownHighlightIndex()]);
                    if (user()) {
                        setSelectedUser(user());
                        setQuery('');
                        setDropdownOpen(false);
                    }
                }
            }}/>
                    </div>

                    {dropdownOpen() && !selectedUser() && (<div class="absolute z-10 mt-1 max-h-48 w-full overflow-auto rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 shadow-lg py-1">
                        {filteredUsers().length === 0 ? (<div class="px-3 py-2 text-sm text-gray-500 dark:text-gray-400">
                            {availableUsers().length === 0
                        ? solidState3.t('BiChat.Share.NoUsersAvailable')
                        : solidState3.t('BiChat.Share.NoSearchResults')}
                          </div>) : (filteredUsers().map((user, index) => (<button type="button" ref={(el) => { dropdownOptionRefs.current[index] = el; }} class={`flex w-full items-center gap-2.5 px-3 py-2 cursor-pointer transition-colors hover:bg-primary-50 dark:hover:bg-primary-900/20 ${index === dropdownHighlightIndex() ? 'bg-primary-50 dark:bg-primary-900/20' : ''}`} onMouseDown={(e) => e.preventDefault()} onClick={() => { setSelectedUser(user); setQuery(''); setDropdownOpen(false); }}>
                              <UserAvatar firstName={user.firstName} lastName={user.lastName} initials={user.initials} size="xs"/>
                              <span class="text-sm text-gray-900 dark:text-gray-100">
                                {user.firstName} {user.lastName}
                              </span>
                            </button>)))}
                      </div>)}
                  </div>

                  {/* Role selector + Add button */}
                  <div class="flex items-center justify-between">
                    <RoleSegmentedControl value={selectedRole()} onChange={setSelectedRole} disabled={saving()} t={solidState3.t}/>
                    <button type="button" onClick={handleAdd} disabled={saving() || !selectedUser()} class="inline-flex cursor-pointer items-center gap-1.5 rounded-xl bg-primary-600 px-3.5 py-2 text-sm font-medium text-white shadow-sm transition-all duration-150 hover:bg-primary-700 hover:shadow active:bg-primary-800 disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-800">
                      <UserPlus size={14}/>
                      {solidState3.t('BiChat.Share.Add')}
                    </button>
                  </div>
                </div>)}
            </div>
          </InlineDialogPanel>
        </div>
      </InlineDialog>

      {/* Remove confirmation */}
      <ConfirmModal isOpen={!!confirmRemove()} isDanger title={solidState3.t('BiChat.Share.RemoveConfirmTitle')} message={confirmRemove() ? solidState3.t('BiChat.Share.RemoveConfirmMessage').replace('{{name}}', `${confirmRemove()!.user.firstName} ${confirmRemove()!.user.lastName}`)
            : ''} confirmText={solidState3.t('BiChat.Share.Remove')} onConfirm={() => {
            if (confirmRemove()) {
                void handleRemove(confirmRemove()!.user.id);
            }
            setConfirmRemove(null);
        }} onCancel={() => setConfirmRemove(null)}/>
    </>);
}
export default SessionMembersModal;
