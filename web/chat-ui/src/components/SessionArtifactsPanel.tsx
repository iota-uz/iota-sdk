import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Paperclip, Plus } from '../icons';
import type { ChatDataSource, SessionArtifact } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { useOptionalChatMessaging } from '../context/ChatContext';
import { SessionArtifactList } from './SessionArtifactList';
import { SessionArtifactPreviewModal } from './SessionArtifactPreviewModal';
interface SessionArtifactsPanelProps {
    dataSource: ChatDataSource;
    sessionId: string;
    isStreaming: boolean;
    allowDrop?: boolean;
    className?: string;
    /** When provided, used instead of useChatMessaging().artifactsInvalidationTrigger (allows use outside SDK ChatSessionProvider). */
    artifactsInvalidationTrigger?: number;
}
const PAGE_SIZE = 50;
function mergeArtifacts(existing: SessionArtifact[], incoming: SessionArtifact[]): SessionArtifact[] {
    const merged = [...existing];
    const existingIds = new Set(existing.map((artifact) => artifact.id));
    for (const artifact of incoming) {
        if (existingIds.has(artifact.id)) {
            continue;
        }
        merged.push(artifact);
        existingIds.add(artifact.id);
    }
    return merged;
}
export function SessionArtifactsPanel(solidProps1Input: SessionArtifactsPanelProps) {
    const solidProps1 = mergeProps({ allowDrop: true, className: '' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const messaging = useOptionalChatMessaging();
    const artifactsInvalidationTrigger = createMemo(() => typeof solidProps1.artifactsInvalidationTrigger === 'number'
        ? solidProps1.artifactsInvalidationTrigger : messaging?.artifactsInvalidationTrigger ?? 0);
    const [fetching, setFetching] = createSignal(true);
    const [refreshing, setRefreshing] = createSignal(false);
    const [loadingMore, setLoadingMore] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const [artifacts, setArtifacts] = createSignal<SessionArtifact[]>([]);
    const [previewArtifactID, setPreviewArtifactID] = createSignal<string | null>(null);
    const [hasMore, setHasMore] = createSignal(false);
    const [isDragging, setIsDragging] = createSignal(false);
    const [dropSuccess, setDropSuccess] = createSignal(false);
    const requestSeq = { current: 0 };
    const hasLoadedRef = { current: false };
    const prevStreamingRef = createMemo(() => ({ current: solidProps1.isStreaming }));
    const artifactsRef = { current: [] } as {
        current: SessionArtifact[];
    };
    const nextOffsetRef = { current: 0 };
    const dragDepthRef = { current: 0 };
    const dropSuccessTimerRef = { current: null } as {
        current: (number | null) | null;
    };
    const canFetchArtifacts = createMemo(() => typeof solidProps1.dataSource.fetchSessionArtifacts === 'function');
    const canDropFiles = createMemo(() => solidProps1.allowDrop && typeof solidProps1.dataSource.uploadSessionArtifacts === 'function');
    const tRef = createMemo(() => ({ current: solidState2.t }));
    tRef().current = solidState2.t;
    const fetchArtifacts = async (opts: {
        reset: boolean;
        manual: boolean;
    }) => {
        if (!canFetchArtifacts() || !solidProps1.dataSource.fetchSessionArtifacts) {
            setFetching(false);
            setRefreshing(false);
            setLoadingMore(false);
            setArtifacts([]);
            setError(null);
            setHasMore(false);
            nextOffsetRef.current = 0;
            return;
        }
        const requestID = ++requestSeq.current;
        const offset = opts.reset ? 0 : nextOffsetRef.current;
        if (!hasLoadedRef.current || opts.reset) {
            if (opts.manual && hasLoadedRef.current) {
                setRefreshing(true);
            }
            else {
                setFetching(true);
            }
        }
        else {
            setLoadingMore(true);
        }
        setError(null);
        try {
            const response = await solidProps1.dataSource.fetchSessionArtifacts(solidProps1.sessionId, {
                limit: PAGE_SIZE,
                offset,
            });
            if (requestID !== requestSeq.current) {
                return;
            }
            const page = [...(response.artifacts || [])].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt));
            const nextList = opts.reset ? page : mergeArtifacts(artifactsRef.current, page);
            setArtifacts(nextList);
            artifactsRef.current = nextList;
            hasLoadedRef.current = true;
            const resolvedHasMore = Boolean(response.hasMore);
            const resolvedNextOffset = typeof response.nextOffset === 'number'
                ? response.nextOffset
                : offset + page.length;
            setHasMore(resolvedHasMore);
            nextOffsetRef.current = resolvedNextOffset;
        }
        catch (err) {
            if (requestID !== requestSeq.current) {
                return;
            }
            setError(err instanceof Error ? err.message : tRef().current('BiChat.Artifacts.FailedToLoad'));
        }
        finally {
            if (requestID === requestSeq.current) {
                setFetching(false);
                setRefreshing(false);
                setLoadingMore(false);
            }
        }
    };
    createEffect(on(() => [fetchArtifacts, solidProps1.sessionId], () => {
        const cleanup = untrack(() => {
            hasLoadedRef.current = false;
            setFetching(true);
            setRefreshing(false);
            setLoadingMore(false);
            setError(null);
            setArtifacts([]);
            artifactsRef.current = [];
            setPreviewArtifactID(null);
            setHasMore(false);
            nextOffsetRef.current = 0;
            void fetchArtifacts({ reset: true, manual: false });
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [fetchArtifacts, solidProps1.isStreaming], () => {
        const cleanup = untrack(() => {
            const wasStreaming = prevStreamingRef().current;
            if (wasStreaming && !solidProps1.isStreaming) {
                void fetchArtifacts({ reset: true, manual: false });
            }
            prevStreamingRef().current = solidProps1.isStreaming;
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [artifactsInvalidationTrigger(), solidProps1.sessionId, canFetchArtifacts(), fetchArtifacts], () => {
        const cleanup = untrack(() => {
            if (artifactsInvalidationTrigger() > 0 && solidProps1.sessionId && canFetchArtifacts()) {
                void fetchArtifacts({ reset: true, manual: false });
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const visibilityFetchRef = { current: fetchArtifacts };
    visibilityFetchRef.current = fetchArtifacts;
    const sessionIdRef = createMemo(() => ({ current: solidProps1.sessionId }));
    sessionIdRef().current = solidProps1.sessionId;
    const canFetchRef = createMemo(() => ({ current: canFetchArtifacts() }));
    canFetchRef().current = canFetchArtifacts();
    onMount(() => {
        const cleanup = untrack(() => {
            const handler = () => {
                if (document.visibilityState === 'visible' &&
                    sessionIdRef().current &&
                    canFetchRef().current) {
                    void visibilityFetchRef.current({ reset: true, manual: false });
                }
            };
            document.addEventListener('visibilitychange', handler);
            return () => document.removeEventListener('visibilitychange', handler);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const previewArtifact = createMemo(() => artifacts().find((artifact) => artifact.id === previewArtifactID()) ?? null);
    const clearDropSuccessTimer = () => {
        if (dropSuccessTimerRef.current === null) {
            return;
        }
        window.clearTimeout(dropSuccessTimerRef.current);
        dropSuccessTimerRef.current = null;
    };
    createEffect(on(() => [clearDropSuccessTimer], () => {
        const cleanup = untrack(() => {
            return () => {
                clearDropSuccessTimer();
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const setDropSuccessState = () => {
        setDropSuccess(true);
        clearDropSuccessTimer();
        dropSuccessTimerRef.current = window.setTimeout(() => {
            setDropSuccess(false);
            dropSuccessTimerRef.current = null;
        }, 1400);
    };
    const hasDragFiles = (e: DragEvent): boolean => {
        return Array.from((e.dataTransfer?.types ?? []) || []).includes('Files');
    };
    const handleDragEnter = (e: DragEvent) => {
        if (!canDropFiles() || !hasDragFiles(e)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        dragDepthRef.current += 1;
        setIsDragging(true);
    };
    const handleDragOver = (e: DragEvent) => {
        if (!canDropFiles() || !hasDragFiles(e)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        if (e.dataTransfer)
            e.dataTransfer.dropEffect = 'copy';
    };
    const handleDragLeave = (e: DragEvent) => {
        if (!canDropFiles() || !hasDragFiles(e)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
        if (dragDepthRef.current === 0) {
            setIsDragging(false);
        }
    };
    const handleDrop = async (e: DragEvent) => {
        if (!canDropFiles() || !solidProps1.dataSource.uploadSessionArtifacts || !hasDragFiles(e)) {
            return;
        }
        e.preventDefault();
        e.stopPropagation();
        dragDepthRef.current = 0;
        setIsDragging(false);
        const itemFiles = Array.from(e.dataTransfer?.items ?? [])
            .filter((item) => item.kind === 'file')
            .map((item) => item.getAsFile())
            .filter((file): file is File => file !== null);
        const droppedFiles = itemFiles.length > 0 ? itemFiles : Array.from(e.dataTransfer?.files ?? []);
        if (droppedFiles.length === 0) {
            return;
        }
        try {
            const result = await solidProps1.dataSource.uploadSessionArtifacts(solidProps1.sessionId, droppedFiles);
            if ((result.artifacts || []).length > 0) {
                setDropSuccessState();
                void fetchArtifacts({ reset: true, manual: false });
            }
            setError(null);
        }
        catch (err) {
            setError(err instanceof Error ? err.message : tRef().current('BiChat.Artifacts.FailedToLoad'));
        }
    };
    const canRenameArtifacts = createMemo(() => typeof solidProps1.dataSource.renameSessionArtifact === 'function');
    const canDeleteArtifacts = createMemo(() => typeof solidProps1.dataSource.deleteSessionArtifact === 'function');
    const handleRenameArtifact = async (artifact: SessionArtifact, name: string) => {
        if (!solidProps1.dataSource.renameSessionArtifact) {
            return;
        }
        const updatedArtifact = await solidProps1.dataSource.renameSessionArtifact(artifact.id, name, artifact.description || '');
        setArtifacts((prev) => {
            const next = prev.map((item) => (item.id === updatedArtifact.id ? updatedArtifact : item));
            artifactsRef.current = next;
            return next;
        });
    };
    const canDeleteArtifact = (artifact: SessionArtifact): boolean => {
        return artifact.type === 'attachment' && !artifact.messageId;
    };
    const handleDeleteArtifact = async (artifact: SessionArtifact) => {
        if (!solidProps1.dataSource.deleteSessionArtifact || !canDeleteArtifact(artifact)) {
            return;
        }
        await solidProps1.dataSource.deleteSessionArtifact(artifact.id);
        setArtifacts((prev) => {
            const next = prev.filter((item) => item.id !== artifact.id);
            artifactsRef.current = next;
            return next;
        });
        setPreviewArtifactID((current) => (current === artifact.id ? null : current));
    };
    const fileInputRef = { current: null } as {
        current: HTMLInputElement | null;
    };
    const handleAttachClick = () => {
        fileInputRef.current?.click();
    };
    const handleFileInputChange = async (e: Event & {
        currentTarget: HTMLInputElement;
    }) => {
        if (!solidProps1.dataSource.uploadSessionArtifacts || !e.currentTarget.files?.length) {
            return;
        }
        const files = Array.from(e.currentTarget.files);
        try {
            const result = await solidProps1.dataSource.uploadSessionArtifacts(solidProps1.sessionId, files);
            if ((result.artifacts || []).length > 0) {
                setDropSuccessState();
                void fetchArtifacts({ reset: true, manual: false });
            }
            setError(null);
        }
        catch (err) {
            setError(err instanceof Error ? err.message : tRef().current('BiChat.Artifacts.FailedToLoad'));
        }
        finally {
            // Reset input so same file can be re-selected
            e.currentTarget.value = '';
        }
    };
    return (<aside class={[
            'relative flex min-w-0 flex-1 flex-col border-l border-gray-200 bg-white dark:border-gray-700/80 dark:bg-gray-900',
            isDragging() ? 'bg-primary-50/40 dark:bg-primary-950/20' : '',
            solidProps1.className,
        ].join(' ')} aria-label={solidState2.t('BiChat.Artifacts.Title')} onDragEnter={handleDragEnter} onDragOver={handleDragOver} onDragLeave={handleDragLeave} onDrop={handleDrop}>
      {(isDragging() || dropSuccess()) && (<div class="pointer-events-none absolute inset-0 z-20 flex items-center justify-center bg-white/85 dark:bg-gray-900/85">
          <div class={[
                'mx-4 flex w-full max-w-xs flex-col items-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center',
                dropSuccess() ? 'border-emerald-400 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-950/30 dark:text-emerald-300'
                    : 'border-primary-400 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-950/30 dark:text-primary-300',
            ].join(' ')}>
            <Paperclip className="h-5 w-5" weight="bold"/>
            <span class="text-sm font-medium">
              {dropSuccess() ? solidState2.t('BiChat.Input.FilesAdded') : solidState2.t('BiChat.Input.DropFiles')}
            </span>
          </div>
        </div>)}

      <header class="flex items-center justify-between border-b border-gray-200 px-3 py-2 dark:border-gray-700/80">
        <div class="min-w-0 flex-1">
          <h2 class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">
            {solidState2.t('BiChat.Artifacts.Title')} ({artifacts().length})
          </h2>
        </div>
        {canDropFiles() && (<>
            <button type="button" onClick={handleAttachClick} class="ml-2 flex-shrink-0 rounded-md p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:text-gray-500 dark:hover:bg-gray-800 dark:hover:text-gray-300" aria-label={solidState2.t('BiChat.Artifacts.AttachFiles')} title={solidState2.t('BiChat.Artifacts.AttachFiles')}>
              <Plus className="h-4 w-4" weight="bold"/>
            </button>
            <input ref={element => fileInputRef.current = element} type="file" multiple class="hidden" onChange={handleFileInputChange} aria-label={solidState2.t('BiChat.Artifacts.AttachFiles')}/>
          </>)}
      </header>

      <div class="min-h-0 flex-1 overflow-y-auto px-3 py-3">
        {fetching() ? (<div class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            {solidState2.t('BiChat.Artifacts.Loading')}
          </div>) : error() ? (<div class="space-y-3 rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-900/70 dark:bg-red-950/30">
            <p class="text-sm font-medium text-red-800 dark:text-red-300">{solidState2.t('BiChat.Artifacts.FailedToLoad')}</p>
            <p class="text-xs text-red-700 dark:text-red-400">{error()}</p>
            <button type="button" onClick={() => {
                void fetchArtifacts({ reset: true, manual: true });
            }} class="cursor-pointer rounded-md border border-red-300 px-2 py-1 text-xs font-medium text-red-700 hover:bg-red-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-red-400/50 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-900/40">
              {solidState2.t('BiChat.Alert.Retry')}
            </button>
          </div>) : !canFetchArtifacts() ? (<div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-900/70 dark:bg-amber-950/30 dark:text-amber-200">
            {solidState2.t('BiChat.Artifacts.Unsupported')}
          </div>) : (<>
            <SessionArtifactList artifacts={artifacts()} selectedArtifactId={previewArtifactID() || undefined} onSelect={(artifact) => setPreviewArtifactID(artifact.id)}/>

            {hasMore() && (<div class="mt-3 flex justify-center">
                <button type="button" onClick={() => {
                    void fetchArtifacts({ reset: false, manual: true });
                }} disabled={loadingMore() || refreshing() || fetching()} class="cursor-pointer rounded-md border border-gray-200 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:text-gray-200 dark:hover:bg-gray-800">
                  {loadingMore() ? solidState2.t('BiChat.Artifacts.LoadingMore') : solidState2.t('BiChat.Artifacts.LoadMore')}
                </button>
              </div>)}
          </>)}
      </div>

      <SessionArtifactPreviewModal isOpen={previewArtifact() !== null} artifact={previewArtifact()} onClose={() => setPreviewArtifactID(null)} canRename={canRenameArtifacts()} canDelete={Boolean(previewArtifact() && canDeleteArtifacts() && canDeleteArtifact(previewArtifact()!))} onRename={canRenameArtifacts() ? handleRenameArtifact : undefined} onDelete={canDeleteArtifacts() ? handleDeleteArtifact : undefined}/>
    </aside>);
}
