import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Archive, ArrowLeft } from '../icons';
import SessionItem from './SessionItem';
import SearchInput from './SearchInput';
import DateGroupHeader from './DateGroupHeader';
import LoadingSpinner from './LoadingSpinner';
import ConfirmModal from './ConfirmModal';
import { EmptyState } from './EmptyState';
import { ToastContainer } from './ToastContainer';
import { useTranslation } from '../hooks/useTranslation';
import { useToast, type UseToastReturn } from '../hooks/useToast';
import { groupSessionsByDate } from '../utils/sessionGrouping';
import { staggerContainerVariants } from '../animations/variants';
import type { Session, ChatDataSource } from '../types';
export interface ArchivedChatListProps {
    dataSource: ChatDataSource;
    onBack: () => void;
    onSessionSelect: (sessionId: string) => void;
    activeSessionId?: string;
    className?: string;
    toast?: UseToastReturn;
}
export default function ArchivedChatList(solidProps1Input: ArchivedChatListProps) {
    const solidProps1 = mergeProps({ className: '' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const localToast = useToast();
    const toast = createMemo(() => solidProps1.toast ?? localToast);
    const shouldRenderToastContainer = createMemo(() => !solidProps1.toast);
    // Search state
    const [searchQuery, setSearchQuery] = createSignal('');
    // Session data
    const [sessions, setSessions] = createSignal<Session[]>([]);
    const [loading, setLoading] = createSignal(true);
    // Refresh key
    const [refreshKey, setRefreshKey] = createSignal(0);
    // Confirm modal state for restore action
    const [showConfirm, setShowConfirm] = createSignal(false);
    const [sessionToRestore, setSessionToRestore] = createSignal<string | null>(null);
    // Fetch archived sessions
    const fetchSessions = async () => {
        try {
            setLoading(true);
            const result = await solidProps1.dataSource.listSessions({
                limit: 100,
                includeArchived: true,
            });
            setSessions(result.sessions.filter((s) => s.status === 'archived'));
        }
        catch (err) {
            console.error('Failed to load archived sessions:', err);
        }
        finally {
            setLoading(false);
        }
    };
    createEffect(on(() => [fetchSessions, refreshKey()], () => {
        const cleanup = untrack(() => {
            fetchSessions();
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleRestoreRequest = (sessionId: string) => {
        setSessionToRestore(sessionId);
        setShowConfirm(true);
    };
    const confirmRestore = async () => {
        const restoredSessionID = sessionToRestore();
        if (!restoredSessionID) {
            return;
        }
        try {
            await solidProps1.dataSource.unarchiveSession(restoredSessionID);
            window.dispatchEvent(new CustomEvent('bichat:sessions-updated', {
                detail: { reason: 'restored', sessionId: restoredSessionID },
            }));
            setRefreshKey((k) => k + 1);
            toast().success(solidState2.t('BiChat.Archived.ChatRestoredSuccessfully'));
        }
        catch (err) {
            console.error('Failed to restore session:', err);
            toast().error(solidState2.t('BiChat.Archived.FailedToRestoreChat'));
        }
        finally {
            setShowConfirm(false);
            setSessionToRestore(null);
        }
    };
    const handleRenameSession = async (sessionId: string, newTitle: string) => {
        try {
            await solidProps1.dataSource.renameSession(sessionId, newTitle);
            toast().success(solidState2.t('BiChat.Sidebar.ChatRenamedSuccessfully'));
            setRefreshKey((k) => k + 1);
        }
        catch (err) {
            console.error('Failed to update session title:', err);
            toast().error(solidState2.t('BiChat.Sidebar.FailedToRenameChat'));
        }
    };
    // Filter by search query
    const filteredSessions = createMemo(() => {
        if (!searchQuery().trim()) {
            return sessions();
        }
        const q = createMemo(() => searchQuery().toLowerCase());
        return sessions().filter((s) => s.title?.toLowerCase().includes(q()));
    });
    // Group sessions by date
    const sessionGroups = createMemo(() => {
        const groups = createMemo(() => groupSessionsByDate(filteredSessions(), solidState2.t));
        return Array.isArray(groups())
            ? groups().map((group) => ({
                ...group,
                sessions: Array.isArray(group.sessions) ? group.sessions : [],
            }))
            : [];
    });
    const isEmpty = createMemo(() => sessions().length === 0);
    const isEmptyAfterSearch = createMemo(() => filteredSessions().length === 0 && !!searchQuery());
    return (<div class={`flex-1 flex flex-col bg-gray-50 dark:bg-gray-900 ${solidProps1.className}`}>
      {/* Header */}
      <div class="border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-6 py-4">
        <div class="flex items-center gap-3 mb-4">
          <button onClick={solidProps1.onBack} class="inline-flex items-center gap-2 px-3 py-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors text-gray-600 dark:text-gray-400" aria-label={solidState2.t('BiChat.Archived.BackToChats')}>
            <ArrowLeft size={20} className="w-5 h-5"/>
            {solidState2.t('BiChat.Common.Back')}
          </button>
        </div>

        <div class="flex items-center gap-2 mb-4">
          <Archive size={24} className="w-6 h-6 text-gray-600 dark:text-gray-400"/>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {solidState2.t('BiChat.Archived.Title')}
          </h1>
        </div>

        {/* Search */}
        <SearchInput value={searchQuery()} onChange={setSearchQuery} placeholder={solidState2.t('BiChat.Archived.SearchArchivedChats')}/>
      </div>

      {/* Content */}
      <div class="flex-1 overflow-y-auto">
        {loading() && sessions().length === 0 ? (<div class="flex items-center justify-center h-full">
            <LoadingSpinner />
          </div>) : isEmpty() ? (<div class="flex items-center justify-center h-full px-6">
            <EmptyState icon={<Archive size={48} className="text-gray-400 dark:text-gray-500"/>} title={solidState2.t('BiChat.Archived.NoArchivedChats')} description={solidState2.t('BiChat.Archived.NoArchivedChatsDescription')}/>
          </div>) : isEmptyAfterSearch() ? (<div class="flex items-center justify-center h-full px-6">
            <EmptyState icon={<Archive size={48} className="text-gray-400 dark:text-gray-500"/>} title={solidState2.t('BiChat.Archived.NoResults')} description={solidState2.t('BiChat.Archived.NoResultsDescription', {
                query: searchQuery(),
            })}/>
          </div>) : (<div class="px-4 py-4 space-y-4">
            {sessionGroups().map((group) => (<div>
                <DateGroupHeader groupName={group.name} count={group.sessions.length}/>
                <ul class="space-y-1 mt-3 mb-4" role="list">
                  {group.sessions.map((session) => (<li class="opacity-70">
                      <SessionItem session={session} isActive={session.id === solidProps1.activeSessionId} mode="archived" testIdPrefix="archived" onSelect={() => solidProps1.onSessionSelect?.(session.id)} onRestore={() => handleRestoreRequest(session.id)} onRename={(newTitle) => handleRenameSession(session.id, newTitle)}/>
                    </li>))}
                </ul>
              </div>))}
          </div>)}
      </div>

      {/* Confirm Restore Modal */}
      <ConfirmModal isOpen={showConfirm()} title={solidState2.t('BiChat.Archived.RestoreChat')} message={solidState2.t('BiChat.Archived.RestoreChatMessage')} confirmText={solidState2.t('BiChat.Archived.RestoreButton')} cancelText={solidState2.t('BiChat.Common.Cancel')} isDanger={false} onConfirm={confirmRestore} onCancel={() => {
            setShowConfirm(false);
            setSessionToRestore(null);
        }}/>

      {/* Toast notifications */}
      {shouldRenderToastContainer() && (<ToastContainer toasts={toast().toasts} onDismiss={toast().dismiss}/>)}
    </div>);
}
