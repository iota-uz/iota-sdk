import { Portal as HostPortal } from '@iota-uz/sdk/solid';
import { cssLength } from "../utils/cssLength";
import { type JSX, splitProps, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Sidebar, ShareNetwork } from '../icons';
import { ChatSessionProvider, useChatSession, useChatMessaging, useChatInput } from '../context/ChatContext';
import { ChatDataSource, ConversationTurn, type SessionUser } from '../types';
import { RateLimiter } from '../utils/RateLimiter';
import { ChatHeader } from './ChatHeader';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import CompactionDoodle from './CompactionDoodle';
import WelcomeContent from './WelcomeContent';
import ArchiveBanner from './ArchiveBanner';
import { useTranslation } from '../hooks/useTranslation';
import { SessionArtifactsPanel } from './SessionArtifactsPanel';
import { SessionMembersModal } from './SessionMembersModal';
import Alert from './Alert';
import { StreamError } from './StreamError';
import { isOpenQuestionStatus } from '../machine/hitlLifecycle';
interface ChatSessionProps {
    dataSource: ChatDataSource;
    sessionId?: string;
    /** Optional rate limiter to throttle sendMessage */
    rateLimiter?: RateLimiter;
    /**
     * Called when a new session is created (e.g. on first message in a "new
     * chat"). Use this to navigate your SPA router to the new session URL.
     */
    onSessionCreated?: (sessionId: string) => void;
    /** Alias for isReadOnly (preferred) */
    readOnly?: boolean;
    isReadOnly?: boolean;
    /** Custom render function for user turns */
    renderUserTurn?: (turn: ConversationTurn) => JSX.Element;
    /** Custom render function for assistant turns */
    renderAssistantTurn?: (turn: ConversationTurn) => JSX.Element;
    className?: string;
    /** Custom content to display as header */
    headerSlot?: JSX.Element;
    /** Custom welcome screen component (replaces default WelcomeContent) */
    welcomeSlot?: JSX.Element;
    /**
     * Host-owned durable content rendered as part of the scrollable conversation
     * history after persisted turns (for example, a workflow or task artifact).
     */
    historyArtifactSlot?: JSX.Element;
    /** Custom logo for the header */
    logoSlot?: JSX.Element;
    /** Custom action buttons for the header */
    actionsSlot?: JSX.Element;
    /** Custom content rendered above the message input (e.g., model selector) */
    inputHeaderSlot?: JSX.Element;
    /** Callback when user navigates back */
    onBack?: () => void;
    /** Custom verbs for the typing indicator (e.g. ['Thinking', 'Analyzing', ...]) */
    thinkingVerbs?: string[];
    /** Callback invoked after an archived session is restored (e.g. to navigate or refresh) */
    onSessionRestored?: (sessionId: string) => void;
    /** Enables the built-in right-side artifacts panel for persisted session artifacts */
    showArtifactsPanel?: boolean;
    /** Initial expanded state for artifacts panel when no persisted preference exists */
    artifactsPanelDefaultExpanded?: boolean;
    /** localStorage key for artifacts panel expanded/collapsed state */
    artifactsPanelStorageKey?: string;
}
const ARTIFACTS_PANEL_WIDTH_DEFAULT = 352;
const ARTIFACTS_PANEL_WIDTH_MIN = 280;
const ARTIFACTS_PANEL_WIDTH_MAX = 560;
function ChatSessionCore(solidProps1Input: Omit<ChatSessionProps, 'sessionId'>) {
    const solidProps1 = mergeProps({ className: '', showArtifactsPanel: false, artifactsPanelDefaultExpanded: false, artifactsPanelStorageKey: 'bichat.artifacts-panel.expanded' } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const solidState3 = useChatSession();
    const solidState4 = useChatMessaging();
    const solidState5 = useChatInput();
    const [narrowArtifacts,setNarrowArtifacts]=createSignal(false);
    onMount(()=>{
      const media=window.matchMedia('(max-width: 1023px)');
      const update=()=>setNarrowArtifacts(media.matches);
      update();media.addEventListener('change',update);
      onCleanup(()=>media.removeEventListener('change',update));
    });
    const isArchived = createMemo(() => solidState3.session?.status === 'archived');
    const accessReadOnly = createMemo(() => solidState3.session?.access ? !solidState3.session.access.canWrite : false);
    const effectiveReadOnly = createMemo(() => Boolean(solidProps1.readOnly ?? solidProps1.isReadOnly) || isArchived() || accessReadOnly());
    const composerDisabled = createMemo(() => isOpenQuestionStatus(solidState4.pendingQuestion?.status));
    const [restoring, setRestoring] = createSignal(false);
    const handleRestore = async () => {
        if (!solidState3.session?.id) {
            return;
        }
        setRestoring(true);
        try {
            await solidProps1.dataSource.unarchiveSession(solidState3.session.id);
            solidState3.retryFetchSession();
            window.dispatchEvent(new CustomEvent('bichat:sessions-updated', {
                detail: { reason: 'restored', sessionId: solidState3.session.id },
            }));
            solidProps1.onSessionRestored?.(solidState3.session.id);
        }
        finally {
            setRestoring(false);
        }
    };
    const [artifactsPanelExpanded, setArtifactsPanelExpanded] = createSignal(solidProps1.artifactsPanelDefaultExpanded);
    const [membersModalOpen, setMembersModalOpen] = createSignal(false);
    const [headerMembers, setHeaderMembers] = createSignal<SessionUser[] | null>(null);
    const [artifactsPanelWidth, setArtifactsPanelWidth] = createSignal(ARTIFACTS_PANEL_WIDTH_DEFAULT);
    const [isResizingArtifactsPanel, setIsResizingArtifactsPanel] = createSignal(false);
    const layoutContainerRef = { current: null } as {
        current: HTMLDivElement | null;
    };
    createEffect(on(() => [solidProps1.artifactsPanelDefaultExpanded, solidProps1.artifactsPanelStorageKey, solidProps1.showArtifactsPanel], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.showArtifactsPanel) {
                return;
            }
            let nextValue = solidProps1.artifactsPanelDefaultExpanded;
            if (typeof window !== 'undefined') {
                const stored = window.localStorage.getItem(solidProps1.artifactsPanelStorageKey);
                if (stored !== null) {
                    nextValue = stored === 'true';
                }
            }
            setArtifactsPanelExpanded(nextValue);
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    createEffect(on(() => [solidProps1.artifactsPanelStorageKey, solidProps1.showArtifactsPanel], () => {
        const cleanup = untrack(() => {
            if (!solidProps1.showArtifactsPanel) {
                return;
            }
            if (typeof window === 'undefined') {
                return;
            }
            try {
                const raw = window.localStorage.getItem(`${solidProps1.artifactsPanelStorageKey}.width`);
                if (raw !== null) {
                    const n = Number.parseInt(raw, 10);
                    if (Number.isFinite(n) && n >= ARTIFACTS_PANEL_WIDTH_MIN && n <= ARTIFACTS_PANEL_WIDTH_MAX) {
                        setArtifactsPanelWidth(n);
                    }
                }
            }
            catch {
                // ignore
            }
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    // Load full member list for header avatar stack when session and listSessionMembers are available
    createEffect(on(() => [solidState3.session?.id, solidProps1.dataSource.listSessionMembers], () => {
        const cleanup = untrack(() => {
            if (!solidState3.session?.id || !solidProps1.dataSource.listSessionMembers) {
                setHeaderMembers(null);
                return;
            }
            let cancelled = false;
            solidProps1.dataSource.listSessionMembers(solidState3.session.id).then((members) => {
                if (!cancelled) {
                    setHeaderMembers(members.map((m) => m.user));
                }
            }).catch(() => {
                if (!cancelled) {
                    setHeaderMembers(null);
                }
            });
            return () => { cancelled = true; };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleArtifactsResizeStart = () => {
        setIsResizingArtifactsPanel(true);
    };
    const handleArtifactsResizeKeyDown = (e: KeyboardEvent) => {
        const step = e.shiftKey ? 40 : 20;
        let nextWidth: number | null = null;
        if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
            nextWidth = Math.min(ARTIFACTS_PANEL_WIDTH_MAX, artifactsPanelWidth() + step);
        }
        else if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
            nextWidth = Math.max(ARTIFACTS_PANEL_WIDTH_MIN, artifactsPanelWidth() - step);
        }
        else if (e.key === 'Home') {
            nextWidth = ARTIFACTS_PANEL_WIDTH_MIN;
        }
        else if (e.key === 'End') {
            nextWidth = ARTIFACTS_PANEL_WIDTH_MAX;
        }
        if (nextWidth !== null) {
            e.preventDefault();
            setArtifactsPanelWidth(nextWidth);
            try {
                window.localStorage.setItem(`${solidProps1.artifactsPanelStorageKey}.width`, String(nextWidth));
            }
            catch {
                // ignore
            }
        }
    };
    const lastPanelWidthRef = createMemo(() => ({ current: artifactsPanelWidth() }));
    lastPanelWidthRef().current = artifactsPanelWidth();
    createEffect(on(() => [isResizingArtifactsPanel(), solidProps1.artifactsPanelStorageKey], () => {
        const cleanup = untrack(() => {
            if (!isResizingArtifactsPanel()) {
                return;
            }
            const move = (e: MouseEvent) => {
                const el = layoutContainerRef.current;
                if (!el) {
                    return;
                }
                const rect = el.getBoundingClientRect();
                const w = rect.right - e.clientX;
                const clamped = Math.min(ARTIFACTS_PANEL_WIDTH_MAX, Math.max(ARTIFACTS_PANEL_WIDTH_MIN, w));
                setArtifactsPanelWidth(clamped);
            };
            const up = () => {
                setIsResizingArtifactsPanel(false);
                try {
                    if (typeof window !== 'undefined') {
                        window.localStorage.setItem(`${solidProps1.artifactsPanelStorageKey}.width`, String(lastPanelWidthRef().current));
                    }
                }
                catch {
                    // ignore
                }
            };
            document.addEventListener('mousemove', move, { passive: true });
            document.addEventListener('mouseup', up);
            document.body.style.cursor = 'col-resize';
            document.body.style.userSelect = 'none';
            return () => {
                document.removeEventListener('mousemove', move);
                document.removeEventListener('mouseup', up);
                document.body.style.cursor = '';
                document.body.style.userSelect = '';
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    return <Show when={!(solidState3.fetching && solidState4.turns.length === 0 && !solidState3.session)} fallback={<div class="flex h-full items-center justify-center"><div class="text-gray-500 dark:text-gray-400">{solidState2.t('BiChat.Input.Processing')}</div></div>}>{_visible => {
            // Show welcome screen for new sessions with no turns
            const showWelcome = createMemo(() => !solidState3.session && solidState4.turns.length === 0);
            const activeSessionId = createMemo(() => solidState3.session?.id ||
                (solidState3.currentSessionId && solidState3.currentSessionId !== 'new'
                    ? solidState3.currentSessionId : undefined));
            const supportsArtifactsPanel = createMemo(() => typeof solidProps1.dataSource.fetchSessionArtifacts === 'function');
            const showArtifactsControls = createMemo(() => Boolean(solidProps1.showArtifactsPanel && supportsArtifactsPanel() && activeSessionId()));
            const shouldRenderArtifactsPanel = createMemo(() => Boolean(showArtifactsControls() && artifactsPanelExpanded() && !showWelcome() && activeSessionId()));
            const handlePromptSelect = (prompt: string) => {
                solidState5.setMessage(prompt);
            };
            const handleToggleArtifactsPanel = () => {
                const nextValue = !artifactsPanelExpanded();
                setArtifactsPanelExpanded(nextValue);
                if (typeof window !== 'undefined') {
                    window.localStorage.setItem(solidProps1.artifactsPanelStorageKey, nextValue ? 'true' : 'false');
                    if (nextValue) {
                        window.dispatchEvent(new CustomEvent('bichat:artifacts-panel-expanded', { detail: { expanded: true } }));
                    }
                }
            };
            const canShowShareButton = createMemo(() => Boolean(solidState3.session?.access?.canManageMembers
                && solidProps1.dataSource.listUsers
                && solidProps1.dataSource.listSessionMembers
                && solidProps1.dataSource.addSessionMember
                && solidProps1.dataSource.updateSessionMemberRole
                && solidProps1.dataSource.removeSessionMember));
            const shareButton = createMemo(() => canShowShareButton() ? (<button type="button" onClick={() => setMembersModalOpen(true)} class="inline-flex cursor-pointer items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-all duration-150 hover:bg-gray-100 hover:text-gray-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200" aria-label={solidState2.t('BiChat.Share.Title')} title={solidState2.t('BiChat.Share.Title')}>
      <ShareNetwork className="h-4 w-4"/>
      {solidState2.t('BiChat.Share.Button')}
    </button>) : null);
            const headerActions = createMemo(() => (<>
      {shareButton()}
      {showArtifactsControls() && (<button type="button" onClick={handleToggleArtifactsPanel} class={[
                        'inline-flex cursor-pointer items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-all duration-150',
                        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50',
                        artifactsPanelExpanded() ? 'bg-primary-50 text-primary-700 hover:bg-primary-100 dark:bg-primary-950/30 dark:text-primary-300 dark:hover:bg-primary-900/40'
                            : 'text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200',
                    ].join(' ')} aria-label={artifactsPanelExpanded() ? solidState2.t('BiChat.Artifacts.ToggleHide') : solidState2.t('BiChat.Artifacts.ToggleShow')} aria-expanded={artifactsPanelExpanded()} title={artifactsPanelExpanded() ? solidState2.t('BiChat.Artifacts.ToggleHide') : solidState2.t('BiChat.Artifacts.ToggleShow')}>
          <Sidebar className="h-4 w-4" weight={artifactsPanelExpanded() ? 'duotone' : 'regular'}/>
          {solidState2.t('BiChat.Artifacts.Title')}
        </button>)}
      {solidProps1.actionsSlot}
    </>));
            return (<main data-testid="bichat-session" class={`flex min-w-0 min-h-0 flex-1 flex-col overflow-hidden bg-gray-50 dark:bg-gray-900 ${solidProps1.className}`}>
      {solidProps1.headerSlot || (<ChatHeader session={solidState3.session} onBack={solidProps1.onBack} readOnly={effectiveReadOnly()} logoSlot={solidProps1.logoSlot} actionsSlot={headerActions()} members={headerMembers() ?? (solidState3.session?.owner ? [solidState3.session.owner] : undefined)} onMembersClick={canShowShareButton() ? () => setMembersModalOpen(true) : undefined}/>)}
      {solidState3.error && (<Alert variant={solidState3.errorRetryable ? 'warning' : 'error'} title={solidState2.t('BiChat.Error.Generic')} message={solidState3.error} onDismiss={() => solidState3.setError(null)} onRetry={solidState3.errorRetryable ? solidState3.retryFetchSession : undefined}/>)}

      <div ref={element => layoutContainerRef.current = element} 
            // overflow-clip (not overflow-hidden): an overflow-hidden box is still a
            // scroll container, so focusing a control near the bottom (e.g. a radio
            // in the inline question form) lets the browser scroll this box to
            // reveal it — and with no scrollbar the chat is shoved off-screen with
            // no way back (blank until refresh). overflow-clip clips without
            // creating a scroll container, so it cannot be scrolled on focus.
            class="relative flex min-h-0 flex-1 overflow-clip">
        <div class="flex min-h-0 min-w-0 flex-1 flex-col">
          {showWelcome() ? (<div class="flex flex-1 flex-col overflow-auto">
              <div class="flex flex-1 items-center justify-center px-4 py-8">
                <div class="w-full max-w-5xl">
                  {solidProps1.welcomeSlot || (<WelcomeContent onPromptSelect={handlePromptSelect} disabled={solidState4.loading || composerDisabled()}/>)}
                  {solidState4.streamError && (<div class="px-6 pt-4">
                      <StreamError error={solidState4.streamError} compact onRetry={solidState4.streamErrorRetryable ? () => void solidState4.retryLastMessage() : undefined} onDismiss={solidState4.clearStreamError}/>
                    </div>)}
                  {!effectiveReadOnly() && solidProps1.inputHeaderSlot}
                  {!effectiveReadOnly() && (<MessageInput message={solidState5.message} loading={solidState4.loading} isStreaming={solidState4.isStreaming} fetching={solidState3.fetching} commandError={solidState5.inputError} onClearCommandError={() => solidState5.setInputError(null)} debugMode={solidState3.debugMode} debugSessionUsage={solidState3.sessionDebugUsage} debugLimits={solidState3.debugLimits} onMessageChange={solidState5.setMessage} onSubmit={solidState5.handleSubmit} messageQueue={solidState5.messageQueue} onUnqueue={solidState5.handleUnqueue} onRemoveQueueItem={solidState5.removeQueueItem} onUpdateQueueItem={solidState5.updateQueueItem} onCancelStreaming={solidState4.cancel} containerClassName="pt-6 px-6" formClassName="mx-auto" disabled={composerDisabled()} reasoningEffortOptions={solidState3.reasoningEffortOptions} reasoningEffort={solidState3.reasoningEffort} onReasoningEffortChange={solidState3.setReasoningEffort}/>)}
                  <p class="mt-4 pb-1 text-center text-xs text-gray-500 dark:text-gray-400">
                    {solidState2.t('BiChat.Welcome.Disclaimer')}
                  </p>
                </div>
              </div>
            </div>) : (<>
              {isArchived() && (<ArchiveBanner show onRestore={handleRestore} restoring={restoring()}/>)}
              <MessageList renderUserTurn={solidProps1.renderUserTurn} renderAssistantTurn={solidProps1.renderAssistantTurn} historyArtifactSlot={solidProps1.historyArtifactSlot} thinkingVerbs={solidProps1.thinkingVerbs} readOnly={effectiveReadOnly()}/>
              <>
                {solidState4.isCompacting && (<div class="flex justify-center px-4 pb-2">
                    <CompactionDoodle title={solidState2.t('BiChat.Slash.CompactingTitle')} subtitle={solidState2.t('BiChat.Slash.CompactingSubtitle')}/>
                  </div>)}
              </>
              {solidState4.streamError && (<div class="px-4 pb-2">
                  <StreamError error={solidState4.streamError} compact onRetry={solidState4.streamErrorRetryable ? () => void solidState4.retryLastMessage() : undefined} onDismiss={solidState4.clearStreamError}/>
                </div>)}
              {!effectiveReadOnly() && solidProps1.inputHeaderSlot}
              {!effectiveReadOnly() && (<MessageInput message={solidState5.message} loading={solidState4.loading} isStreaming={solidState4.isStreaming} fetching={solidState3.fetching} commandError={solidState5.inputError} onClearCommandError={() => solidState5.setInputError(null)} debugMode={solidState3.debugMode} debugSessionUsage={solidState3.sessionDebugUsage} debugLimits={solidState3.debugLimits} onMessageChange={solidState5.setMessage} onSubmit={solidState5.handleSubmit} messageQueue={solidState5.messageQueue} onUnqueue={solidState5.handleUnqueue} onRemoveQueueItem={solidState5.removeQueueItem} onUpdateQueueItem={solidState5.updateQueueItem} onCancelStreaming={solidState4.cancel} disabled={composerDisabled()} reasoningEffortOptions={solidState3.reasoningEffortOptions} reasoningEffort={solidState3.reasoningEffort} onReasoningEffortChange={solidState3.setReasoningEffort}/>)}
            </>)}
        </div>

        {/* Desktop: persistent slot with animated width so main content expands in sync */}
        <div class="hidden lg:flex lg:min-h-0 shrink-0 overflow-hidden">
          {shouldRenderArtifactsPanel() && activeSessionId() && (<div class="flex min-h-0" style={{ "width": cssLength(artifactsPanelWidth()) }}>
              <div role="separator" tabIndex={0} aria-label={solidState2.t('BiChat.Artifacts.Resize')} aria-orientation="vertical" aria-valuenow={artifactsPanelWidth()} aria-valuemin={ARTIFACTS_PANEL_WIDTH_MIN} aria-valuemax={ARTIFACTS_PANEL_WIDTH_MAX} onMouseDown={handleArtifactsResizeStart} onKeyDown={handleArtifactsResizeKeyDown} class="relative flex shrink-0 cursor-col-resize touch-none items-center justify-center w-2 transition-colors lg:flex group/resize after:absolute after:inset-y-0 after:left-0 after:w-0.5 after:bg-gray-300 dark:after:bg-gray-600 after:transition-colors group-hover/resize:after:bg-primary-400 dark:group-hover/resize:after:bg-primary-500 focus-visible:outline-none focus-visible:after:bg-primary-500 dark:focus-visible:after:bg-primary-400">
                <span class="absolute h-10 w-1.5 cursor-col-resize rounded-full bg-gray-400 transition-colors group-hover/resize:bg-primary-400 dark:bg-gray-500 dark:group-hover/resize:bg-primary-500"/>
              </div>
              <SessionArtifactsPanel dataSource={solidProps1.dataSource} sessionId={activeSessionId()!} isStreaming={solidState4.isStreaming} allowDrop={!effectiveReadOnly()} className="min-h-0 min-w-0 flex-1"/>
            </div>)}
        </div>

        
          {shouldRenderArtifactsPanel() && activeSessionId() && narrowArtifacts() && (<HostPortal surface="drawer" class="fixed inset-0 flex" label={solidState2.t('BiChat.Artifacts.Title')} onEscape={handleToggleArtifactsPanel}>
              <button type="button" class="cursor-pointer flex-1 bg-black/40" onClick={handleToggleArtifactsPanel} aria-label={solidState2.t('BiChat.Common.Close')}/>
              <SessionArtifactsPanel dataSource={solidProps1.dataSource} sessionId={activeSessionId()!} isStreaming={solidState4.isStreaming} allowDrop={!effectiveReadOnly()} className="flex h-full w-full max-w-sm min-h-0"/>
            </HostPortal>)}
        
      </div>
      {canShowShareButton() && (<SessionMembersModal isOpen={membersModalOpen()} sessionId={solidState3.session?.id} dataSource={solidProps1.dataSource} onClose={() => setMembersModalOpen(false)}/>)}
    </main>);
    }}</Show>;
}
export function ChatSession(props: ChatSessionProps) {
    const [local, coreProps] = splitProps(props, ['dataSource', 'sessionId', 'rateLimiter', 'onSessionCreated']);
    return (<ChatSessionProvider dataSource={local.dataSource} sessionId={local.sessionId} rateLimiter={local.rateLimiter} onSessionCreated={local.onSessionCreated}>
      <ChatSessionCore dataSource={local.dataSource} {...coreProps}/>
    </ChatSessionProvider>);
}
export type { ChatSessionProps };
