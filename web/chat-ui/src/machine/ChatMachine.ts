/**
 * ChatMachine — framework-agnostic state machine for BiChat.
 *
 * Owns all async logic that was previously in ChatContext.tsx:
 * - Session fetch (with race-condition guards)
 * - Message sending + streaming
 * - Slash commands (/debug, /clear, /compact)
 * - HITL question submission / rejection
 * - Message queue (auto-drain)
 * - Rate limiting
 * - Abort / cancel
 *
 * Zero React dependency. Designed for `useSyncExternalStore`:
 * three subscribe/getSnapshot pairs map to the existing 3-context split.
 */
import type { ChatDataSource, ConversationTurn, CodeOutput, Attachment, PendingQuestion, QuestionAnswers, SendMessageOptions, SessionDebugUsage, ActivityStep, } from "../types";
import { MessageRole } from "../types";
import type { RateLimiter } from "../utils/RateLimiter";
import { normalizeRPCError } from "../utils/errorDisplay";
import { loadQueue, saveQueue } from "../utils/queueStorage";
import { loadDebugMode, saveDebugMode } from "../utils/debugModeStorage";
import { clearReasoningEffort, loadReasoningEffort, saveReasoningEffort, } from "../utils/reasoningEffortStorage";
import { getSessionDebugUsage } from "../utils/debugTrace";
import { ARTIFACT_TOOL_NAMES, createPendingTurn, createCompactedSystemTurn, generateTempId, parseSlashCommand, readDebugLimitsFromGlobalContext, readReasoningEffortOptionsFromGlobalContext, type ParsedSlashCommand, } from "../context/chatHelpers";
import type { ChatMachineConfig, ChatMachineState, SessionSnapshot, MessagingSnapshot, InputSnapshot, LastSendAttempt, } from "./types";
import { deriveSessionSnapshot, deriveMessagingSnapshot, deriveInputSnapshot, deriveDebugMode, } from "./selectors";
import { applyTurnLifecycleForPendingQuestion, isOpenQuestionStatus, pendingQuestionFromInterrupt, } from "./hitlLifecycle";
// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type Listener = () => void;
class ResumeTransportError extends Error {
}
const MAX_QUEUE_SIZE = 5;
const RUN_MARKER_PREFIX = "bichat.run.";
/** Interval (ms) for polling stream status when another tab has an active generation. */
const PASSIVE_POLL_INTERVAL_MS = 2000;
function getRunMarkerKey(sessionId: string): string {
    return `${RUN_MARKER_PREFIX}${sessionId}`;
}
function setRunMarker(sessionId: string, runId: string): void {
    if (typeof window === "undefined" || !sessionId || sessionId === "new") {
        return;
    }
    try {
        window.sessionStorage.setItem(getRunMarkerKey(sessionId), runId);
    }
    catch {
        // ignore
    }
}
function getRunMarker(sessionId: string): string | null {
    if (typeof window === "undefined" || !sessionId || sessionId === "new") {
        return null;
    }
    try {
        return window.sessionStorage.getItem(getRunMarkerKey(sessionId));
    }
    catch {
        return null;
    }
}
function clearRunMarker(sessionId: string): void {
    if (typeof window === "undefined" || !sessionId || sessionId === "new") {
        return;
    }
    try {
        window.sessionStorage.removeItem(getRunMarkerKey(sessionId));
    }
    catch {
        // ignore
    }
}
// ---------------------------------------------------------------------------
// ChatMachine
// ---------------------------------------------------------------------------
export class ChatMachine {
    // ── Config ──────────────────────────────────────────────────────────────
    private dataSource: ChatDataSource;
    private rateLimiter: RateLimiter;
    private onSessionCreated?: (sessionId: string) => void;
    // ── Internal state ──────────────────────────────────────────────────────
    private state: ChatMachineState;
    // ── Refs (mutable, no subscription) ─────────────────────────────────────
    private abortController: AbortController | null = null;
    private lastSendAttempt: LastSendAttempt | null = null;
    /** Prevents fetchSession effect from clobbering state while stream is active. */
    private sendingSessionId: string | null = null;
    /**
     * Monotonic counter bumped on every real session switch. An async op (send
     * stream, post-stream sync, queue drain, resume, HITL) captures it at start
     * and re-checks before writing to the single shared `state.messaging`; a stale
     * epoch means the user navigated to a different session, so the write is
     * dropped instead of bleeding into / clobbering the now-visible session.
     */
    private viewEpoch = 0;
    private fetchCancelled = false;
    private disposed = false;
    private reasoningEffortOptions: string[] | null = null;
    private reasoningEffortOptionSet: Set<string> | null = null;
    /** Memoized sessionDebugUsage — avoids unnecessary session re-renders during streaming. */
    private lastSessionDebugUsage: SessionDebugUsage | null = null;
    /** Interval handle for passive polling when another tab has an active stream. */
    private passivePollingId: ReturnType<typeof setInterval> | null = null;
    // ── Listeners ───────────────────────────────────────────────────────────
    private sessionListeners = new Set<Listener>();
    private messagingListeners = new Set<Listener>();
    private inputListeners = new Set<Listener>();
    // ── Snapshot caches (for useSyncExternalStore identity stability) ───────
    private cachedSessionSnapshot: SessionSnapshot | null = null;
    private cachedMessagingSnapshot: MessagingSnapshot | null = null;
    private cachedInputSnapshot: InputSnapshot | null = null;
    private sessionSnapshotVersion = 0;
    private messagingSnapshotVersion = 0;
    private inputSnapshotVersion = 0;
    private lastSessionSnapshotVersion = -1;
    private lastMessagingSnapshotVersion = -1;
    private lastInputSnapshotVersion = -1;
    // ── Bound method references (stable identity) ──────────────────────────
    readonly setError: (error: string | null) => void;
    readonly retryFetchSession: () => void;
    readonly sendMessage: (content: string, attachments?: Attachment[]) => Promise<void>;
    readonly handleRegenerate: (turnId: string, model?: string) => Promise<void>;
    readonly handleEdit: (turnId: string, newContent: string) => Promise<void>;
    readonly handleCopy: (text: string) => Promise<void>;
    readonly handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void;
    readonly handleRejectPendingQuestion: () => Promise<void>;
    readonly retryLastMessage: () => Promise<void>;
    readonly clearStreamError: () => void;
    readonly cancel: () => void;
    readonly setCodeOutputs: (outputs: CodeOutput[]) => void;
    readonly setMessage: (message: string) => void;
    readonly setInputError: (error: string | null) => void;
    readonly handleSubmit: (e: {
        preventDefault: () => void;
    }, attachments?: Attachment[]) => void;
    readonly handleUnqueue: () => {
        content: string;
        attachments: Attachment[];
    } | null;
    readonly enqueueMessage: (content: string, attachments: Attachment[]) => boolean;
    readonly removeQueueItem: (index: number) => void;
    readonly updateQueueItem: (index: number, content: string) => void;
    readonly setReasoningEffort: (effort: string) => void;
    readonly setModel: (model: string | undefined) => void;
    constructor(config: ChatMachineConfig) {
        this.dataSource = config.dataSource;
        this.rateLimiter = config.rateLimiter;
        this.onSessionCreated = config.onSessionCreated;
        this.reasoningEffortOptions = this.buildReasoningEffortOptions();
        this.reasoningEffortOptionSet = this.reasoningEffortOptions
            ? new Set(this.reasoningEffortOptions)
            : null;
        const initialReasoningEffort = this.sanitizeReasoningEffort(loadReasoningEffort() || undefined);
        if (!initialReasoningEffort) {
            clearReasoningEffort();
        }
        this.state = {
            session: {
                currentSessionId: undefined,
                session: null,
                fetching: false,
                error: null,
                errorRetryable: false,
                debugModeBySession: {},
                debugLimits: readDebugLimitsFromGlobalContext(),
                reasoningEffort: initialReasoningEffort,
                reasoningEffortOptions: this.reasoningEffortOptions ?? undefined,
                model: undefined,
            },
            messaging: {
                turns: [],
                streamingContent: "",
                isStreaming: false,
                streamError: null,
                streamErrorRetryable: false,
                loading: false,
                pendingQuestion: null,
                codeOutputs: [],
                isCompacting: false,
                artifactsInvalidationTrigger: 0,
                thinkingContent: "",
                activeSteps: [],
                generationInProgress: false,
            },
            input: {
                message: "",
                inputError: null,
                messageQueue: [],
            },
        };
        // Bind all public methods once (stable references)
        this.setError = this._setError.bind(this);
        this.retryFetchSession = this._retryFetchSession.bind(this);
        this.sendMessage = this._sendMessage.bind(this);
        this.handleRegenerate = this._handleRegenerate.bind(this);
        this.handleEdit = this._handleEdit.bind(this);
        this.handleCopy = this._handleCopy.bind(this);
        this.handleSubmitQuestionAnswers =
            this._handleSubmitQuestionAnswers.bind(this);
        this.handleRejectPendingQuestion =
            this._handleRejectPendingQuestion.bind(this);
        this.retryLastMessage = this._retryLastMessage.bind(this);
        this.clearStreamError = this._clearStreamError.bind(this);
        this.cancel = this._cancel.bind(this);
        this.setCodeOutputs = this._setCodeOutputs.bind(this);
        this.setMessage = this._setMessage.bind(this);
        this.setInputError = this._setInputError.bind(this);
        this.handleSubmit = this._handleSubmit.bind(this);
        this.handleUnqueue = this._handleUnqueue.bind(this);
        this.enqueueMessage = this._enqueueMessage.bind(this);
        this.removeQueueItem = this._removeQueueItem.bind(this);
        this.updateQueueItem = this._updateQueueItem.bind(this);
        this.setReasoningEffort = this._setReasoningEffort.bind(this);
        this.setModel = this._setModel.bind(this);
    }
    private buildReasoningEffortOptions(): string[] | null {
        const options = readReasoningEffortOptionsFromGlobalContext();
        if (!options || options.length === 0) {
            return null;
        }
        return options;
    }
    // Keep outbound payloads constrained to server-declared options.
    private sanitizeReasoningEffort(effort: string | undefined): string | undefined {
        if (!effort ||
            !this.reasoningEffortOptionSet ||
            this.reasoningEffortOptionSet.size === 0) {
            return undefined;
        }
        return this.reasoningEffortOptionSet.has(effort) ? effort : undefined;
    }
    // =====================================================================
    // Lifecycle
    // =====================================================================
    /**
     * Set the active session ID. Triggers fetch when transitioning to a real
     * session, or resets state for 'new'/undefined.
     */
    setSessionId(id: string | undefined): void {
        if (this.disposed) {
            return;
        }
        const prev = this.state.session.currentSessionId;
        if (id === prev) {
            return;
        }
        // A different session is being shown. Invalidate any in-flight send/stream
        // for the previous session so its chunks, post-stream sync, and queue drain
        // can't write into this new view, and cancel the network stream + old poll.
        this.viewEpoch++;
        this.abortController?.abort();
        this.sendingSessionId = null;
        this._stopPassivePolling();
        // Clear the previous session's transient streaming state so it doesn't bleed
        // into the new view. Persisted turns/pendingQuestion are loaded by the fetch
        // below; a background run for the new session is re-detected by its status
        // check and re-sets these as needed.
        this._updateMessaging({
            streamingContent: "",
            thinkingContent: "",
            activeSteps: [],
            isStreaming: false,
            loading: false,
            generationInProgress: false,
            streamError: null,
            streamErrorRetryable: false,
        });
        this.state.session.currentSessionId = id;
        this._hydrateDebugModeForSession(id);
        this._notifySession();
        // Reset queue for new session
        if (!id || id === "new") {
            this._updateInput({ messageQueue: [] });
        }
        else {
            this._updateInput({ messageQueue: loadQueue(id) });
        }
        this._fetchSessionIfNeeded();
    }
    /**
     * Update mutable config that may change across parent re-renders.
     * Called from the React provider's useEffect to keep the machine in sync.
     */
    updateConfig(config: Pick<ChatMachineConfig, "dataSource" | "onSessionCreated">): void {
        this.dataSource = config.dataSource;
        this.onSessionCreated = config.onSessionCreated;
    }
    dispose(): void {
        this.disposed = true;
        this.fetchCancelled = true;
        this._stopPassivePolling();
        this.abortController?.abort();
        this.sessionListeners.clear();
        this.messagingListeners.clear();
        this.inputListeners.clear();
    }
    // =====================================================================
    // Subscribe / getSnapshot (for useSyncExternalStore)
    // =====================================================================
    subscribeSession = (listener: Listener): (() => void) => {
        this.sessionListeners.add(listener);
        return () => {
            this.sessionListeners.delete(listener);
        };
    };
    getSessionSnapshot = (): SessionSnapshot => {
        if (this.lastSessionSnapshotVersion !== this.sessionSnapshotVersion) {
            this.cachedSessionSnapshot = deriveSessionSnapshot(this.state, {
                setError: this.setError,
                retryFetchSession: this.retryFetchSession,
                setReasoningEffort: this.setReasoningEffort,
                setModel: this.setModel,
            });
            this.lastSessionSnapshotVersion = this.sessionSnapshotVersion;
        }
        return this.cachedSessionSnapshot!;
    };
    subscribeMessaging = (listener: Listener): (() => void) => {
        this.messagingListeners.add(listener);
        return () => {
            this.messagingListeners.delete(listener);
        };
    };
    getMessagingSnapshot = (): MessagingSnapshot => {
        if (this.lastMessagingSnapshotVersion !== this.messagingSnapshotVersion) {
            this.cachedMessagingSnapshot = deriveMessagingSnapshot(this.state, {
                sendMessage: this.sendMessage,
                handleRegenerate: this.handleRegenerate,
                handleEdit: this.handleEdit,
                handleCopy: this.handleCopy,
                handleSubmitQuestionAnswers: this.handleSubmitQuestionAnswers,
                handleRejectPendingQuestion: this.handleRejectPendingQuestion,
                retryLastMessage: this.retryLastMessage,
                clearStreamError: this.clearStreamError,
                cancel: this.cancel,
                setCodeOutputs: this.setCodeOutputs,
            });
            this.lastMessagingSnapshotVersion = this.messagingSnapshotVersion;
        }
        return this.cachedMessagingSnapshot!;
    };
    subscribeInput = (listener: Listener): (() => void) => {
        this.inputListeners.add(listener);
        return () => {
            this.inputListeners.delete(listener);
        };
    };
    getInputSnapshot = (): InputSnapshot => {
        if (this.lastInputSnapshotVersion !== this.inputSnapshotVersion) {
            this.cachedInputSnapshot = deriveInputSnapshot(this.state, {
                setMessage: this.setMessage,
                setInputError: this.setInputError,
                handleSubmit: this.handleSubmit,
                handleUnqueue: this.handleUnqueue,
                enqueueMessage: this.enqueueMessage,
                removeQueueItem: this.removeQueueItem,
                updateQueueItem: this.updateQueueItem,
            });
            this.lastInputSnapshotVersion = this.inputSnapshotVersion;
        }
        return this.cachedInputSnapshot!;
    };
    // =====================================================================
    // Private — state updates + notification
    // =====================================================================
    private _updateSession(patch: Partial<typeof this.state.session>): void {
        Object.assign(this.state.session, patch);
        this._notifySession();
    }
    private _notifySession(): void {
        this.sessionSnapshotVersion++;
        for (const fn of this.sessionListeners) {
            fn();
        }
    }
    private _updateMessaging(patch: Partial<typeof this.state.messaging>): void {
        Object.assign(this.state.messaging, patch);
        this._notifyMessaging();
        // sessionDebugUsage is derived from turns — only notify session listeners
        // when the derived usage actually changes (avoids re-renders during streaming).
        if ("turns" in patch) {
            const newUsage = getSessionDebugUsage(this.state.messaging.turns);
            if (!sessionDebugUsageEqual(this.lastSessionDebugUsage, newUsage)) {
                this.lastSessionDebugUsage = newUsage;
                this._notifySession();
            }
        }
    }
    private _notifyMessaging(): void {
        this.messagingSnapshotVersion++;
        for (const fn of this.messagingListeners) {
            fn();
        }
    }
    private _updateInput(patch: Partial<typeof this.state.input>): void {
        Object.assign(this.state.input, patch);
        // Only persist queue to sessionStorage when it actually changed
        if ("messageQueue" in patch) {
            this._persistQueue();
        }
        this._notifyInput();
    }
    private _notifyInput(): void {
        this.inputSnapshotVersion++;
        for (const fn of this.inputListeners) {
            fn();
        }
    }
    private _persistQueue(): void {
        const sid = this.state.session.currentSessionId;
        if (!sid || sid === "new") {
            return;
        }
        saveQueue(sid, this.state.input.messageQueue);
    }
    /**
     * True while `epoch` is still the active view — i.e. the machine has not been
     * disposed and no session switch has happened since `epoch` was captured.
     * Async ops gate their writes to the shared messaging state on this.
     */
    private _isCurrentEpoch(epoch: number): boolean {
        return !this.disposed && this.viewEpoch === epoch;
    }
    /** A send can't start while generating, mid-resume, or with an open question. */
    private _isBlockedForSend(): boolean {
        return (this.state.messaging.loading ||
            this.state.messaging.generationInProgress ||
            isOpenQuestionStatus(this.state.messaging.pendingQuestion?.status));
    }
    /**
     * Send the next queued message, but only if we still own the view (`epoch`)
     * and nothing is blocking (an in-flight generation or an open question). The
     * item is dequeued optimistically and restored to the front if it turns out we
     * can't send when the deferred tick fires — so a queued message is never
     * silently dropped.
     */
    private _drainQueueIfReady(epoch: number): void {
        if (!this._isCurrentEpoch(epoch)) {
            return;
        }
        const queue = this.state.input.messageQueue;
        if (queue.length === 0 || this._isBlockedForSend()) {
            return;
        }
        const next = queue[0];
        this._updateInput({ messageQueue: queue.slice(1) });
        // Defer to avoid a reentrant send within the current call stack.
        setTimeout(() => {
            if (!this._isCurrentEpoch(epoch)) {
                return;
            }
            if (this._isBlockedForSend()) {
                // Became blocked between dequeue and now — put it back, don't drop it.
                this._updateInput({
                    messageQueue: [next, ...this.state.input.messageQueue],
                });
                return;
            }
            this._sendMessageCore(next.content, next.attachments);
        }, 0);
    }
    private _setDebugModeForSession(sessionId: string, enabled: boolean): void {
        const nextDebugModeBySession = { ...this.state.session.debugModeBySession };
        if (enabled) {
            nextDebugModeBySession[sessionId] = true;
        }
        else {
            delete nextDebugModeBySession[sessionId];
        }
        this._updateSession({ debugModeBySession: nextDebugModeBySession });
        saveDebugMode(sessionId, enabled);
    }
    private _hydrateDebugModeForSession(sessionId: string | undefined): void {
        if (!sessionId || sessionId === "new") {
            return;
        }
        if (this.state.session.debugModeBySession[sessionId] === true) {
            return;
        }
        if (!loadDebugMode(sessionId)) {
            return;
        }
        this._updateSession({
            debugModeBySession: {
                ...this.state.session.debugModeBySession,
                [sessionId]: true,
            },
        });
    }
    private _setReasoningEffort(effort: string): void {
        const next = this.sanitizeReasoningEffort(effort);
        this._updateSession({ reasoningEffort: next });
        if (next) {
            saveReasoningEffort(next);
            return;
        }
        clearReasoningEffort();
    }
    private _setModel(model: string | undefined): void {
        this._updateSession({ model });
    }
    // =====================================================================
    // Private — session fetch
    // =====================================================================
    private _fetchSessionIfNeeded(): void {
        const id = this.state.session.currentSessionId;
        if (!id || id === "new") {
            this._updateSession({
                session: null,
                fetching: false,
                error: null,
                errorRetryable: false,
            });
            this._updateMessaging({
                turns: [],
                pendingQuestion: null,
            });
            this._updateInput({ inputError: null });
            return;
        }
        // Skip while stream is managing this session
        if (this.sendingSessionId === id) {
            return;
        }
        this.fetchCancelled = false;
        this._updateSession({ fetching: true, error: null, errorRetryable: false });
        this._updateInput({ inputError: null });
        const fetchId = id;
        this.dataSource
            .fetchSession(fetchId)
            .then((result) => {
            if (this.fetchCancelled || this.disposed) {
                return;
            }
            if (this.state.session.currentSessionId !== fetchId) {
                return;
            }
            if (this.sendingSessionId === fetchId) {
                return;
            }
            if (result) {
                this._updateSession({ session: result.session, fetching: false });
                this._setTurnsFromFetch(result.turns, result.pendingQuestion || null);
                this._checkStreamStatusAndResumeOrPoll(fetchId);
            }
            else {
                this._updateSession({ error: "Session not found", fetching: false });
            }
        })
            .catch((err) => {
            if (this.fetchCancelled || this.disposed) {
                return;
            }
            if (this.state.session.currentSessionId !== fetchId) {
                return;
            }
            if (this.sendingSessionId === fetchId) {
                return;
            }
            const normalized = normalizeRPCError(err, "Failed to load session");
            this._updateSession({
                error: normalized.userMessage,
                errorRetryable: normalized.retryable,
                fetching: false,
            });
        });
    }
    /**
     * Sets turns from fetch, preserving pending user-only turns if server hasn't caught up.
     * Applies `turns` + optional `pendingQuestion` in a single messaging update to avoid
     * transient intermediate UI states between separate notifications.
     */
    private _setTurnsFromFetch(fetchedTurns: ConversationTurn[], pendingQuestion?: PendingQuestion | null): void {
        if (!Array.isArray(fetchedTurns)) {
            console.warn("[ChatMachine] Ignoring malformed turns payload from fetchSession");
            return;
        }
        const prev = this.state.messaging.turns;
        const localPartialTurn = prev
            .slice()
            .reverse()
            .find((turn) => turn.assistantTurn?.id.startsWith("assistant-partial-"));
        if (localPartialTurn) {
            if (fetchedTurns.length === 0) {
                fetchedTurns = prev;
            }
            else {
                const correspondingIndex = fetchedTurns.findIndex((turn, index) => turn.userTurn.id === localPartialTurn.userTurn.id ||
                    (index === fetchedTurns.length - 1 &&
                        turn.userTurn.content === localPartialTurn.userTurn.content));
                if (correspondingIndex >= 0) {
                    if (!fetchedTurns[correspondingIndex].assistantTurn) {
                        fetchedTurns = fetchedTurns.map((turn, index) => index === correspondingIndex
                            ? { ...turn, assistantTurn: localPartialTurn.assistantTurn }
                            : turn);
                    }
                }
                else {
                    fetchedTurns = [...fetchedTurns, localPartialTurn];
                }
            }
        }
        const hasPendingUserOnly = prev.length > 0 && !prev[prev.length - 1].assistantTurn;
        const patch: Partial<typeof this.state.messaging> = {};
        const effectivePendingQuestion = pendingQuestion === undefined
            ? this.state.messaging.pendingQuestion
            : pendingQuestion;
        if (pendingQuestion !== undefined) {
            patch.pendingQuestion = pendingQuestion;
        }
        if (hasPendingUserOnly && (!fetchedTurns || fetchedTurns.length === 0)) {
            const lifecycleTurns = applyTurnLifecycleForPendingQuestion(prev, effectivePendingQuestion);
            if (lifecycleTurns !== prev) {
                patch.turns = lifecycleTurns;
            }
            if (Object.keys(patch).length > 0) {
                this._updateMessaging(patch);
            }
            return; // keep optimistic turn
        }
        patch.turns = applyTurnLifecycleForPendingQuestion(fetchedTurns ?? prev, effectivePendingQuestion);
        this._updateMessaging(patch);
    }
    /**
     * After fetch: if backend has an active stream, either resume (same-browser) or poll (passive).
     */
    private _checkStreamStatusAndResumeOrPoll(sessionId: string): void {
        if (!this.dataSource.getStreamStatus || !this.dataSource.resumeStream) {
            return;
        }
        this.dataSource
            .getStreamStatus(sessionId)
            .then((status) => {
            if (this.fetchCancelled || this.disposed) {
                return;
            }
            if (this.state.session.currentSessionId !== sessionId) {
                return;
            }
            if (this.sendingSessionId === sessionId) {
                return;
            }
            if (!status?.active || !status.runId) {
                this._updateMessaging({ generationInProgress: false });
                return;
            }
            const storedRunId = getRunMarker(sessionId);
            if (storedRunId === status.runId) {
                void this._resumeAcceptedRunOrPoll(sessionId, status.runId);
            }
            else {
                this._updateMessaging({ generationInProgress: true });
                this._startPassivePolling(sessionId);
            }
        })
            .catch(() => {
            this._updateMessaging({ generationInProgress: false });
        });
    }
    private _stopPassivePolling(): void {
        if (this.passivePollingId) {
            clearInterval(this.passivePollingId);
            this.passivePollingId = null;
        }
    }
    private _startPassivePolling(sessionId: string, onRecovered?: () => void): void {
        this._stopPassivePolling();
        this.passivePollingId = setInterval(() => {
            if (this.disposed || this.state.session.currentSessionId !== sessionId) {
                this._stopPassivePolling();
                return;
            }
            this.dataSource
                .getStreamStatus?.(sessionId)
                .then((status) => {
                if (!status?.active) {
                    this._stopPassivePolling();
                    this._updateMessaging({ generationInProgress: false });
                    this.dataSource
                        .fetchSession(sessionId)
                        .then((result) => {
                        if (this.state.session.currentSessionId === sessionId &&
                            result) {
                            this._setTurnsFromFetch(result.turns, result.pendingQuestion ?? null);
                            onRecovered?.();
                        }
                    })
                        .catch((err) => {
                        console.error("[ChatMachine] fetchSession after stream inactive:", err);
                    });
                }
            })
                .catch((err) => {
                console.error("[ChatMachine] getStreamStatus:", err);
            });
        }, PASSIVE_POLL_INTERVAL_MS);
    }
    private async _runResumeStream(sessionId: string, runId: string): Promise<void> {
        if (this.sendingSessionId !== null) {
            return;
        }
        this.sendingSessionId = sessionId;
        const controller = new AbortController();
        this.abortController = controller;
        const epoch = this.viewEpoch;
        this._updateMessaging({ isStreaming: true });
        let accumulatedContent = "";
        try {
            let terminalError: Error | null = null;
            await this.dataSource.resumeStream!(sessionId, runId, (chunk) => {
                // Stop writing resumed chunks into the view if the user navigated away.
                if (!this._isCurrentEpoch(epoch)) {
                    return;
                }
                if (chunk.type === "snapshot" &&
                    chunk.snapshot?.partialContent !== undefined) {
                    accumulatedContent = chunk.snapshot.partialContent;
                    this._updateMessaging({ streamingContent: accumulatedContent });
                }
                else if ((chunk.type === "content" || chunk.type === "chunk") &&
                    chunk.content) {
                    accumulatedContent += chunk.content;
                    this._updateMessaging({ streamingContent: accumulatedContent });
                }
                else if (chunk.type === "thinking" && chunk.content) {
                    this._handleThinkingChunk(chunk.content);
                }
                else if (chunk.type === "tool_start" && chunk.tool) {
                    this._handleToolStart(chunk.tool);
                }
                else if (chunk.type === "tool_end" && chunk.tool) {
                    this._handleToolEnd(chunk.tool);
                }
                else if (chunk.type === "error") {
                    terminalError =
                        chunk.errorSource === "transport"
                            ? new ResumeTransportError(chunk.error || "Stream error")
                            : new Error(chunk.error || "Stream error");
                }
            }, controller.signal);
            // Managed subscriptions resolve when closed on unmount. That is not a
            // terminal event: keep the run marker so a reload can reconnect.
            if (controller.signal.aborted) {
                throw new DOMException("Stream cancelled", "AbortError");
            }
            if (terminalError) {
                throw terminalError;
            }
            clearRunMarker(sessionId);
            await this._syncSessionFromServer(sessionId, true, epoch);
        }
        catch (err) {
            if (this._isCurrentEpoch(epoch) &&
                !(err instanceof Error && err.name === "AbortError")) {
                this._preservePartialAssistant(accumulatedContent);
                const normalized = normalizeRPCError(err, "Failed to resume stream");
                this._updateMessaging({
                    streamError: normalized.userMessage,
                    streamErrorRetryable: normalized.retryable,
                });
            }
            throw err;
        }
        finally {
            if (this.abortController === controller) {
                this.abortController = null;
            }
            if (this._isCurrentEpoch(epoch)) {
                this.sendingSessionId = null;
                this._updateMessaging({
                    isStreaming: false,
                    loading: false,
                    streamingContent: "",
                    thinkingContent: "",
                    activeSteps: [],
                    generationInProgress: false,
                });
            }
        }
    }
    private async _resumeAcceptedRunOrPoll(sessionId: string, runId: string): Promise<void> {
        setRunMarker(sessionId, runId);
        const epoch = this.viewEpoch;
        const previousAnswers = new Set(this.state.messaging.turns.map((turn) => turn.assistantTurn?.id));
        const clearRecoveredTransportError = () => {
            if (!this._isCurrentEpoch(epoch))
                return;
            const answer = this.state.messaging.turns.at(-1)?.assistantTurn;
            if (answer?.lifecycle === "complete" &&
                answer.content.trim() &&
                !previousAnswers.has(answer.id)) {
                this._updateMessaging({
                    streamError: null,
                    streamErrorRetryable: false,
                });
            }
        };
        try {
            await this._runResumeStream(sessionId, runId);
        }
        catch (err) {
            if (this.disposed ||
                !this._isCurrentEpoch(epoch) ||
                (err instanceof Error && err.name === "AbortError")) {
                return;
            }
            console.warn("[ChatMachine] resumeStream failed, switching to status polling fallback:", err);
            const getStreamStatus = this.dataSource.getStreamStatus;
            const status = getStreamStatus
                ? await getStreamStatus(sessionId).catch(() => null)
                : null;
            if (!this._isCurrentEpoch(epoch))
                return;
            if (status && !status.active) {
                clearRunMarker(sessionId);
                const synced = await this._syncSessionFromServer(sessionId, true, epoch).catch(() => false);
                if (!this._isCurrentEpoch(epoch))
                    return;
                if (synced && err instanceof ResumeTransportError) {
                    clearRecoveredTransportError();
                }
                this._updateMessaging({ generationInProgress: false });
                return;
            }
            setRunMarker(sessionId, status?.runId ?? runId);
            this._updateMessaging({
                isStreaming: false,
                loading: false,
                generationInProgress: true,
            });
            this._startPassivePolling(sessionId, err instanceof ResumeTransportError
                ? clearRecoveredTransportError
                : undefined);
        }
    }
    // =====================================================================
    // Private — actions
    // =====================================================================
    private _setError(error: string | null): void {
        this._updateSession({
            error,
            errorRetryable: false,
        });
    }
    private _retryFetchSession(): void {
        this._fetchSessionIfNeeded();
    }
    private _clearStreamError(): void {
        this._updateMessaging({
            streamError: null,
            streamErrorRetryable: false,
        });
    }
    private _notifySessionsUpdated(reason: string, sessionId?: string): void {
        if (typeof window === "undefined") {
            return;
        }
        window.dispatchEvent(new CustomEvent("bichat:sessions-updated", {
            detail: { reason, sessionId },
        }));
    }
    private _cancel(): void {
        if (this.abortController) {
            const sessionId = this.sendingSessionId ?? this.state.session.currentSessionId;
            if (sessionId && sessionId !== "new") {
                clearRunMarker(sessionId);
                if (this.dataSource.stopGeneration) {
                    this.dataSource.stopGeneration(sessionId).catch(() => {
                        // Non-fatal; local abort still stops the stream
                    });
                }
            }
            this.abortController.abort();
            this.abortController = null;
            this._updateMessaging({
                isStreaming: false,
                loading: false,
                streamingContent: "",
                thinkingContent: "",
                activeSteps: [],
            });
        }
    }
    private _setCodeOutputs(outputs: CodeOutput[]): void {
        this._updateMessaging({ codeOutputs: outputs });
    }
    private _setMessage(message: string): void {
        this._updateInput({ message });
    }
    private _setInputError(error: string | null): void {
        this._updateInput({ inputError: error });
    }
    // ── Slash commands ──────────────────────────────────────────────────────
    private async _executeSlashCommand(command: ParsedSlashCommand): Promise<boolean> {
        if (command.hasArgs) {
            this._updateInput({ inputError: "BiChat.Slash.ErrorNoArguments" });
            return true;
        }
        this._updateSession({ error: null, errorRetryable: false });
        this._updateInput({ inputError: null });
        this._clearStreamError();
        if (command.name === "/debug") {
            const debugMode = deriveDebugMode(this.state);
            const key = this.state.session.currentSessionId || "new";
            const nextDebugMode = !debugMode;
            this._setDebugModeForSession(key, nextDebugMode);
            if (nextDebugMode &&
                this.state.session.currentSessionId &&
                this.state.session.currentSessionId !== "new") {
                try {
                    const result = await this.dataSource.fetchSession(this.state.session.currentSessionId);
                    if (result) {
                        this._updateSession({ session: result.session });
                        this._setTurnsFromFetch(result.turns, result.pendingQuestion || null);
                    }
                }
                catch (err) {
                    console.error("Failed to refresh session for debug mode:", err);
                }
            }
            this._updateInput({ message: "" });
            return true;
        }
        const curSessionId = this.state.session.currentSessionId;
        if (!curSessionId || curSessionId === "new") {
            this._updateInput({ inputError: "BiChat.Slash.ErrorSessionRequired" });
            return true;
        }
        if (command.name === "/clear") {
            this._updateInput({ message: "" });
            this._updateMessaging({ loading: true, streamingContent: "" });
            try {
                await this.dataSource.clearSessionHistory(curSessionId);
                const result = await this.dataSource.fetchSession(curSessionId);
                if (result) {
                    this._updateSession({ session: result.session });
                    this._setTurnsFromFetch(result.turns, result.pendingQuestion || null);
                }
                else {
                    this._setTurnsFromFetch([], null);
                }
                this._updateMessaging({ codeOutputs: [] });
            }
            catch (err) {
                const normalized = normalizeRPCError(err, "Failed to clear session history");
                this._updateInput({ inputError: normalized.userMessage });
            }
            finally {
                this._updateMessaging({ loading: false, isStreaming: false });
            }
            return true;
        }
        if (command.name === "/compact") {
            this._updateInput({ message: "" });
            this._updateMessaging({
                loading: true,
                isCompacting: true,
                streamingContent: "",
            });
            try {
                const accepted = await this.dataSource.compactSessionHistory(curSessionId);
                if (!accepted.runId) {
                    throw new Error("Async compaction run metadata is missing");
                }
                this._updateMessaging({
                    turns: applyTurnLifecycleForPendingQuestion([
                        createCompactedSystemTurn(curSessionId, "Compacting conversation history..."),
                    ], null),
                    pendingQuestion: null,
                });
                await this._resumeAcceptedRunOrPoll(curSessionId, accepted.runId);
                if (!this.state.messaging.generationInProgress) {
                    const result = await this.dataSource.fetchSession(curSessionId);
                    if (result) {
                        this._updateSession({ session: result.session });
                        this._setTurnsFromFetch(result.turns, result.pendingQuestion || null);
                    }
                    else {
                        this._setTurnsFromFetch([], null);
                    }
                    this._updateMessaging({ codeOutputs: [] });
                }
            }
            catch (err) {
                const normalized = normalizeRPCError(err, "Failed to compact session history");
                this._updateInput({ inputError: normalized.userMessage });
            }
            finally {
                this._updateMessaging({
                    isCompacting: false,
                    loading: false,
                    isStreaming: false,
                });
            }
            return true;
        }
        this._updateInput({ inputError: "BiChat.Slash.ErrorUnknownCommand" });
        return true;
    }
    private _insertOptimisticTurn(prevTurns: ConversationTurn[], tempTurn: ConversationTurn, replaceFromMessageID?: string): void {
        if (!replaceFromMessageID) {
            this._updateMessaging({ turns: [...prevTurns, tempTurn] });
            return;
        }
        const idx = prevTurns.findIndex((turn) => turn.userTurn.id === replaceFromMessageID);
        if (idx === -1) {
            console.warn(`[ChatMachine] replaceFromMessageID "${replaceFromMessageID}" not found; appending as new turn`);
            this._updateMessaging({ turns: [...prevTurns, tempTurn] });
            return;
        }
        this._updateMessaging({ turns: [...prevTurns.slice(0, idx), tempTurn] });
    }
    private async _resolveSendSession(currentSessionId: string | undefined, debugMode: boolean): Promise<{
        activeSessionId: string | undefined;
        shouldNavigateAfter: boolean;
    }> {
        let activeSessionId = currentSessionId;
        let shouldNavigateAfter = false;
        if (!activeSessionId || activeSessionId === "new") {
            const result = await this.dataSource.createSession();
            if (result) {
                const createdSessionID = result.id;
                activeSessionId = createdSessionID;
                this._updateSession({ currentSessionId: createdSessionID });
                if (debugMode) {
                    this._setDebugModeForSession(createdSessionID, true);
                }
                shouldNavigateAfter = true;
            }
        }
        return { activeSessionId, shouldNavigateAfter };
    }
    private async _syncSessionFromServer(sessionId: string, allowEmptyTurns = false, epoch?: number): Promise<boolean> {
        const fetchResult = await this.dataSource.fetchSession(sessionId);
        if (!fetchResult) {
            return false;
        }
        // The user may have switched sessions while the fetch was in flight; don't
        // overwrite the now-visible session with this one's data.
        if (epoch !== undefined && !this._isCurrentEpoch(epoch)) {
            return false;
        }
        this._updateSession({ session: fetchResult.session });
        this._setTurnsFromFetch(allowEmptyTurns ? fetchResult.turns ?? [] : fetchResult.turns, fetchResult.pendingQuestion || null);
        return true;
    }
    private async _runSendStream(params: {
        activeSessionId: string | undefined;
        content: string;
        attachments: Attachment[];
        debugMode: boolean;
        replaceFromMessageID?: string;
        reasoningEffort?: string;
        model?: string;
        tempTurnId: string;
        controller: AbortController;
        epoch: number;
        onContent: (content: string) => void;
    }): Promise<{
        createdSessionId?: string;
        sessionFetched: boolean;
        stopped?: boolean;
    }> {
        const { activeSessionId, content, attachments, debugMode, replaceFromMessageID, reasoningEffort, model, tempTurnId, controller, epoch, onContent, } = params;
        let accumulatedContent = "";
        let createdSessionId: string | undefined;
        let sessionFetched = false;
        let currentRunId: string | undefined;
        this._updateMessaging({ isStreaming: true });
        for await (const chunk of this.dataSource.sendMessage(activeSessionId || "new", content, attachments, controller.signal, {
            debugMode,
            replaceFromMessageID,
            reasoningEffort,
            model,
        })) {
            // Stop if the send was cancelled or the user navigated to another session.
            if (controller.signal.aborted || !this._isCurrentEpoch(epoch)) {
                break;
            }
            if (chunk.type === "stream_started" && chunk.runId) {
                currentRunId = chunk.runId;
                if (activeSessionId && activeSessionId !== "new") {
                    setRunMarker(activeSessionId, chunk.runId);
                }
            }
            if ((chunk.type === "chunk" || chunk.type === "content") &&
                chunk.content) {
                accumulatedContent += chunk.content;
                onContent(accumulatedContent);
                this._updateMessaging({ streamingContent: accumulatedContent });
            }
            else if (chunk.type === "thinking" && chunk.content) {
                this._handleThinkingChunk(chunk.content);
            }
            else if (chunk.type === "tool_start" && chunk.tool) {
                this._handleToolStart(chunk.tool);
            }
            else if (chunk.type === "tool_end" && chunk.tool) {
                this._handleToolEnd(chunk.tool);
            }
            else if (chunk.type === "error") {
                throw new Error(chunk.error || "Stream error");
            }
            else if (chunk.type === "interrupt") {
                if (chunk.sessionId) {
                    createdSessionId = chunk.sessionId;
                }
                const pendingFromInterrupt = pendingQuestionFromInterrupt(chunk.interrupt, tempTurnId);
                if (pendingFromInterrupt) {
                    this._updateMessaging({
                        pendingQuestion: pendingFromInterrupt,
                        turns: applyTurnLifecycleForPendingQuestion(this.state.messaging.turns, pendingFromInterrupt),
                    });
                }
                if (!sessionFetched) {
                    sessionFetched = true;
                    const finalSessionId = createdSessionId || activeSessionId;
                    if (finalSessionId && finalSessionId !== "new") {
                        await this._syncSessionFromServer(finalSessionId, false, epoch);
                    }
                }
            }
            else if (chunk.type === "done") {
                if (chunk.sessionId) {
                    createdSessionId = chunk.sessionId;
                }
                if (!sessionFetched) {
                    sessionFetched = true;
                    const finalSessionId = createdSessionId || activeSessionId;
                    if (finalSessionId && finalSessionId !== "new") {
                        await this._syncSessionFromServer(finalSessionId, false, epoch);
                    }
                }
            }
            else if (chunk.type === "user_message" && chunk.sessionId) {
                createdSessionId = chunk.sessionId;
                if (currentRunId && createdSessionId) {
                    setRunMarker(createdSessionId, currentRunId);
                }
            }
        }
        const finalSessionId = createdSessionId || activeSessionId;
        if (finalSessionId && finalSessionId !== "new") {
            clearRunMarker(finalSessionId);
        }
        const stopped = controller.signal.aborted;
        return { createdSessionId, sessionFetched, stopped };
    }
    private async _ensureSessionSyncAfterStream(activeSessionId: string | undefined, createdSessionId: string | undefined, sessionFetched: boolean, epoch: number): Promise<void> {
        if (sessionFetched) {
            return;
        }
        const finalSessionId = createdSessionId || activeSessionId;
        if (!finalSessionId || finalSessionId === "new") {
            return;
        }
        try {
            await this._syncSessionFromServer(finalSessionId, true, epoch);
        }
        catch (fetchErr) {
            console.error("Failed to fetch session after stream:", fetchErr);
        }
    }
    private _finalizeSuccessfulSend(targetSessionId: string | undefined, shouldNavigateAfter: boolean): void {
        if (targetSessionId && targetSessionId !== "new") {
            this._notifySessionsUpdated("message_sent", targetSessionId);
        }
        if (shouldNavigateAfter && targetSessionId && targetSessionId !== "new") {
            this._notifySessionsUpdated("session_created", targetSessionId);
            if (this.onSessionCreated) {
                this.onSessionCreated(targetSessionId);
            }
        }
        this._clearStreamError();
        this.lastSendAttempt = null;
    }
    private _handleSendError(err: unknown, content: string, tempTurn: ConversationTurn, epoch: number, partialAssistantContent: string): boolean {
        // The user navigated to a different session while this send was failing —
        // none of the recovery writes (input restore, turn keep, error banner)
        // belong to the now-visible session, so drop them.
        if (!this._isCurrentEpoch(epoch)) {
            return false;
        }
        if (err instanceof Error && err.name === "AbortError") {
            // Soft-cancel (user stop / unmount): drop the optimistic turn and return
            // the prompt to the input so they can edit and resend.
            this._updateInput({ message: content });
            this._clearStreamError();
            this._updateMessaging({
                turns: this.state.messaging.turns.filter((turn) => turn.id !== tempTurn.id),
            });
            const sessionId = this.sendingSessionId ?? this.state.session.currentSessionId;
            if (sessionId && sessionId !== "new") {
                this._syncSessionFromServer(sessionId, true, epoch).catch(() => { });
            }
            return false;
        }
        const normalized = normalizeRPCError(err, "Failed to send message");
        // Genuine error: KEEP the optimistic user turn in history. Removing it would
        // collapse a brand-new chat back to the welcome screen (showWelcome is gated
        // on `!session && turns.length === 0`) and silently drop the message the user
        // just sent. Instead the turn stays, the error banner + Retry let them
        // re-send, and Retry replaces this turn (via replaceFromMessageID) rather
        // than appending a duplicate. The prompt lives in the turn, so the input is
        // cleared.
        if (this.lastSendAttempt) {
            this.lastSendAttempt = {
                ...this.lastSendAttempt,
                options: {
                    ...this.lastSendAttempt.options,
                    replaceFromMessageID: tempTurn.userTurn.id,
                },
            };
        }
        this._updateInput({ message: "", inputError: null });
        const turns = this._preservePartialAssistant(partialAssistantContent, tempTurn.id);
        this._updateMessaging({
            turns,
            streamError: normalized.userMessage,
            streamErrorRetryable: normalized.retryable,
        });
        console.error("Send message error:", err);
        return false;
    }
    private _preservePartialAssistant(content: string, targetTurnId?: string): ConversationTurn[] {
        if (content.trim() === "") {
            return this.state.messaging.turns;
        }
        let targetIndex = targetTurnId
            ? this.state.messaging.turns.findIndex((turn) => turn.id === targetTurnId)
            : -1;
        if (!targetTurnId) {
            for (let i = this.state.messaging.turns.length - 1; i >= 0; i--) {
                if (!this.state.messaging.turns[i].assistantTurn) {
                    targetIndex = i;
                    break;
                }
            }
        }
        if (targetIndex < 0) {
            return this.state.messaging.turns;
        }
        const turns = this.state.messaging.turns.map((turn, index) => index === targetIndex
            ? {
                ...turn,
                assistantTurn: {
                    id: generateTempId("assistant-partial"),
                    role: MessageRole.Assistant,
                    content,
                    citations: [],
                    artifacts: [],
                    codeOutputs: [],
                    lifecycle: "complete" as const,
                    createdAt: new Date().toISOString(),
                },
            }
            : turn);
        this._updateMessaging({ turns });
        return turns;
    }
    // ── Send message ────────────────────────────────────────────────────────
    /**
     * Public entry point (no options). Calls _sendMessageCore internally.
     */
    private async _sendMessage(content: string, attachments: Attachment[] = []): Promise<void> {
        return this._sendMessageCore(content, attachments);
    }
    /**
     * Internal entry point with options (for regenerate/edit).
     */
    private async _sendMessageDirect(content: string, attachments: Attachment[], options?: SendMessageOptions): Promise<void> {
        return this._sendMessageCore(content, attachments, options);
    }
    /**
     * Core send-message logic. Handles slash commands, rate limiting, streaming,
     * session creation, optimistic turns, and auto-queue-drain.
     */
    private async _sendMessageCore(content: string, attachments: Attachment[] = [], options?: SendMessageOptions): Promise<void> {
        if (this.disposed) {
            return;
        }
        if (!content.trim() ||
            this.state.messaging.loading ||
            isOpenQuestionStatus(this.state.messaging.pendingQuestion?.status)) {
            return;
        }
        const trimmedContent = content.trim();
        if (trimmedContent.startsWith("/")) {
            const maybeCommand = parseSlashCommand(content);
            if (!maybeCommand) {
                this._updateInput({ inputError: "BiChat.Slash.ErrorUnknownCommand" });
                return;
            }
            if (attachments.length > 0) {
                this._updateInput({ inputError: "BiChat.Slash.ErrorNoAttachments" });
                return;
            }
            await this._executeSlashCommand(maybeCommand);
            return;
        }
        if (!this.rateLimiter.canMakeRequest()) {
            const timeUntilNext = this.rateLimiter.getTimeUntilNextRequest();
            const seconds = Math.ceil(timeUntilNext / 1000);
            this._updateInput({
                inputError: `Rate limit exceeded. Please wait ${seconds} seconds before sending another message.`,
            });
            return;
        }
        this._updateInput({ message: "", inputError: null });
        this._updateSession({ error: null, errorRetryable: false });
        this._clearStreamError();
        this._updateMessaging({
            loading: true,
            streamingContent: "",
        });
        const controller = new AbortController();
        this.abortController = controller;
        const epoch = this.viewEpoch;
        const curSessionId = this.state.session.currentSessionId;
        const curDebugMode = deriveDebugMode(this.state);
        const replaceFromMessageID = options?.replaceFromMessageID;
        const tempTurn = createPendingTurn(curSessionId || "new", content, attachments);
        this.lastSendAttempt = { content, attachments, options };
        const prevTurns = this.state.messaging.turns;
        this._insertOptimisticTurn(prevTurns, tempTurn, replaceFromMessageID);
        let shouldDrainQueue = true;
        let partialAssistantContent = "";
        try {
            const { activeSessionId, shouldNavigateAfter } = await this._resolveSendSession(curSessionId, curDebugMode);
            // Lock: prevent fetch-session from clobbering state
            this.sendingSessionId = activeSessionId || null;
            const { createdSessionId, sessionFetched, stopped } = await this._runSendStream({
                activeSessionId,
                content,
                attachments,
                debugMode: curDebugMode,
                replaceFromMessageID,
                reasoningEffort: this.sanitizeReasoningEffort(this.state.session.reasoningEffort),
                model: this.state.session.model,
                tempTurnId: tempTurn.id,
                controller,
                epoch,
                onContent: (streamedContent) => {
                    partialAssistantContent = streamedContent;
                },
            });
            // If the user switched sessions mid-stream, every write below would land
            // in the now-visible session — skip them all (the stream is already
            // aborted; server state is intact and reloads when they return).
            if (!this._isCurrentEpoch(epoch)) {
                // nothing to do
            }
            else if (stopped) {
                this._updateMessaging({
                    turns: this.state.messaging.turns.filter((turn) => turn.id !== tempTurn.id),
                });
                this._updateInput({ message: content });
                this._clearStreamError();
                const syncId = createdSessionId || activeSessionId;
                if (syncId && syncId !== "new") {
                    await this._syncSessionFromServer(syncId, true, epoch).catch(() => { });
                }
            }
            else {
                await this._ensureSessionSyncAfterStream(activeSessionId, createdSessionId, sessionFetched, epoch);
                const targetSessionId = createdSessionId || activeSessionId;
                this._finalizeSuccessfulSend(targetSessionId, shouldNavigateAfter);
            }
        }
        catch (err) {
            shouldDrainQueue = this._handleSendError(err, content, tempTurn, epoch, partialAssistantContent);
        }
        finally {
            // Only reset the shared streaming state if we still own the view; a
            // session switch (or a newer send) must not have its state clobbered.
            if (this._isCurrentEpoch(epoch)) {
                this._updateMessaging({
                    loading: false,
                    streamingContent: "",
                    isStreaming: false,
                    thinkingContent: "",
                    activeSteps: [],
                });
                this.sendingSessionId = null;
            }
            // Don't null a newer send's controller.
            if (this.abortController === controller) {
                this.abortController = null;
            }
            // Auto-drain the queue — only into the same session/view this send
            // belonged to, and only when nothing is blocking.
            if (shouldDrainQueue) {
                this._drainQueueIfReady(epoch);
            }
        }
    }
    // ── Ephemeral activity trace helpers ─────────────────────────────────
    private _handleThinkingChunk(content: string): void {
        const THINKING_BUFFER_LIMIT = 500;
        const prev = this.state.messaging.thinkingContent;
        let updated = prev + content;
        if (updated.length > THINKING_BUFFER_LIMIT) {
            updated = updated.slice(-THINKING_BUFFER_LIMIT);
        }
        this._updateMessaging({ thinkingContent: updated });
        // Upsert a "thinking" step if not already active
        const steps = this.state.messaging.activeSteps;
        const existing = steps.find((s) => s.type === "thinking" && s.status === "active");
        if (!existing) {
            const step: ActivityStep = {
                id: `thinking-${Date.now()}`,
                type: "thinking",
                toolName: "thinking",
                status: "active",
                startedAt: Date.now(),
            };
            this._updateMessaging({ activeSteps: [...steps, step] });
        }
    }
    private _handleToolStart(tool: {
        callId?: string;
        name: string;
        arguments?: string;
        agentName?: string;
    }): void {
        // Mark any active thinking step as completed (model moved to tool execution)
        let steps = this.state.messaging.activeSteps.map((s) => s.type === "thinking" && s.status === "active"
            ? { ...s, status: "completed" as const, completedAt: Date.now() }
            : s);
        const isAgentDelegation = tool.name === "task";
        const step: ActivityStep = {
            id: tool.callId ||
                `tool-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
            type: isAgentDelegation ? "agent_delegation" : "tool",
            toolName: tool.name,
            arguments: tool.arguments,
            agentName: tool.agentName,
            status: "active",
            startedAt: Date.now(),
        };
        steps = [...steps, step];
        this._updateMessaging({ activeSteps: steps });
    }
    private _handleToolEnd(tool: {
        callId?: string;
        name: string;
        durationMs?: number;
        agentName?: string;
    }): void {
        const steps = [...this.state.messaging.activeSteps];
        const idx = steps.findIndex((s) => s.status === "active" && this._matchStep(s, tool));
        if (idx !== -1) {
            steps[idx] = {
                ...steps[idx],
                status: "completed" as const,
                completedAt: Date.now(),
                durationMs: tool.durationMs,
            };
        }
        this._updateMessaging({ activeSteps: steps });
        // Artifact invalidation
        if (tool.name && ARTIFACT_TOOL_NAMES.has(tool.name)) {
            this._updateMessaging({
                artifactsInvalidationTrigger: this.state.messaging.artifactsInvalidationTrigger + 1,
            });
        }
    }
    /** Match a step to a tool_end event. Use callId when present; fall back to name + agentName. */
    private _matchStep(step: ActivityStep, tool: {
        callId?: string;
        name: string;
        agentName?: string;
    }): boolean {
        // When the backend provides a callId, use it exclusively — no fallback.
        if (tool.callId) {
            return step.id === tool.callId;
        }
        return step.toolName === tool.name && step.agentName === tool.agentName;
    }
    // ── Retry ───────────────────────────────────────────────────────────────
    private async _retryLastMessage(): Promise<void> {
        const lastAttempt = this.lastSendAttempt;
        if (!lastAttempt || this.state.messaging.loading) {
            return;
        }
        this._clearStreamError();
        this._updateInput({ inputError: null });
        await this._sendMessageDirect(lastAttempt.content, lastAttempt.attachments, lastAttempt.options);
    }
    // ── Regenerate / Edit ───────────────────────────────────────────────────
    private async _handleRegenerate(turnId: string, model?: string): Promise<void> {
        const curSessionId = this.state.session.currentSessionId;
        if (!curSessionId || curSessionId === "new") {
            return;
        }
        const turn = this.state.messaging.turns.find((t) => t.id === turnId);
        if (!turn) {
            return;
        }
        if (model && model !== this.state.session.model) {
            this._setModel(model);
        }
        this._updateSession({ error: null, errorRetryable: false });
        // _sendMessageDirect delegates to _sendMessageCore which handles all errors internally
        await this._sendMessageDirect(turn.userTurn.content, turn.userTurn.attachments, {
            replaceFromMessageID: turn.userTurn.id,
        });
    }
    private async _handleEdit(turnId: string, newContent: string): Promise<void> {
        const curSessionId = this.state.session.currentSessionId;
        if (!curSessionId || curSessionId === "new") {
            this._updateInput({ message: newContent });
            this._updateMessaging({
                turns: this.state.messaging.turns.filter((t) => t.id !== turnId),
            });
            return;
        }
        const turn = this.state.messaging.turns.find((t) => t.id === turnId);
        if (!turn) {
            this._updateSession({
                error: "Failed to edit message",
                errorRetryable: false,
            });
            return;
        }
        this._updateSession({ error: null, errorRetryable: false });
        // _sendMessageDirect delegates to _sendMessageCore which handles all errors internally
        await this._sendMessageDirect(newContent, turn.userTurn.attachments, {
            replaceFromMessageID: turn.userTurn.id,
        });
    }
    private async _handleCopy(text: string): Promise<void> {
        if (typeof navigator === "undefined" || !navigator.clipboard) {
            return;
        }
        try {
            await navigator.clipboard.writeText(text);
        }
        catch {
            // Clipboard write can fail if page doesn't have focus (Permissions Policy)
        }
    }
    // ── HITL ────────────────────────────────────────────────────────────────
    private async _handleSubmitQuestionAnswers(answers: QuestionAnswers): Promise<void> {
        const curSessionId = this.state.session.currentSessionId;
        const curPendingQuestion = this.state.messaging.pendingQuestion;
        if (!curSessionId || !curPendingQuestion) {
            return;
        }
        const epoch = this.viewEpoch;
        this._updateMessaging({ loading: true });
        this._updateSession({ error: null, errorRetryable: false });
        const previousPendingQuestion = curPendingQuestion;
        try {
            const result = await this.dataSource.submitQuestionAnswers(curSessionId, previousPendingQuestion.id, answers);
            // Bail on dispose or a session switch — writes below belong to this session.
            if (!this._isCurrentEpoch(epoch)) {
                return;
            }
            if (result.success) {
                this._updateMessaging({
                    pendingQuestion: {
                        ...previousPendingQuestion,
                        status: "ANSWER_SUBMITTED",
                    },
                    turns: applyTurnLifecycleForPendingQuestion(this.state.messaging.turns, {
                        ...previousPendingQuestion,
                        status: "ANSWER_SUBMITTED",
                    }),
                });
                if (result.data) {
                    await this._resumeAcceptedRunOrPoll(curSessionId, result.data.runId);
                    if (!this.state.messaging.generationInProgress) {
                        const fetchResult = await this.dataSource.fetchSession(curSessionId);
                        if (!this._isCurrentEpoch(epoch)) {
                            return;
                        }
                        if (fetchResult) {
                            this._updateSession({ session: fetchResult.session });
                            this._setTurnsFromFetch(fetchResult.turns, fetchResult.pendingQuestion || null);
                        }
                        else {
                            this._updateSession({
                                error: "Failed to load updated session",
                                errorRetryable: true,
                            });
                        }
                    }
                }
                else if (curSessionId !== "new") {
                    const fetchResult = await this.dataSource.fetchSession(curSessionId);
                    if (!this._isCurrentEpoch(epoch)) {
                        return;
                    }
                    if (fetchResult) {
                        this._updateSession({ session: fetchResult.session });
                        this._setTurnsFromFetch(fetchResult.turns, fetchResult.pendingQuestion || null);
                    }
                    else {
                        this._updateSession({
                            error: "Failed to load updated session",
                            errorRetryable: true,
                        });
                    }
                }
            }
            else {
                this._updateSession({
                    error: result.error || "Failed to submit answers",
                    errorRetryable: false,
                });
            }
        }
        catch (err) {
            if (!this._isCurrentEpoch(epoch)) {
                return;
            }
            const normalized = normalizeRPCError(err, "Failed to submit answers");
            this._updateSession({
                error: normalized.userMessage,
                errorRetryable: normalized.retryable,
            });
        }
        finally {
            if (this._isCurrentEpoch(epoch)) {
                this._updateMessaging({ loading: false });
                // The question is resolved and generation finished — resume any queued
                // messages that were stranded while the question was open.
                if (!this.state.session.error) {
                    this._drainQueueIfReady(epoch);
                }
            }
        }
    }
    private async _handleRejectPendingQuestion(): Promise<void> {
        const curSessionId = this.state.session.currentSessionId;
        const curPendingQuestion = this.state.messaging.pendingQuestion;
        if (!curSessionId || !curPendingQuestion) {
            return;
        }
        const epoch = this.viewEpoch;
        try {
            const result = await this.dataSource.rejectPendingQuestion(curSessionId);
            if (!this._isCurrentEpoch(epoch)) {
                return;
            }
            if (result.success) {
                const submittedQuestion = {
                    ...curPendingQuestion,
                    status: "REJECT_SUBMITTED" as const,
                };
                this._updateMessaging({
                    pendingQuestion: submittedQuestion,
                    turns: applyTurnLifecycleForPendingQuestion(this.state.messaging.turns, submittedQuestion),
                });
                if (result.data) {
                    await this._resumeAcceptedRunOrPoll(curSessionId, result.data.runId);
                    if (!this.state.messaging.generationInProgress &&
                        curSessionId !== "new") {
                        const fetchResult = await this.dataSource.fetchSession(curSessionId);
                        if (!this._isCurrentEpoch(epoch)) {
                            return;
                        }
                        if (fetchResult) {
                            this._updateSession({ session: fetchResult.session });
                            this._setTurnsFromFetch(fetchResult.turns, fetchResult.pendingQuestion || null);
                        }
                        else {
                            this._updateSession({
                                error: "Failed to load updated session",
                                errorRetryable: true,
                            });
                        }
                    }
                }
                else if (curSessionId !== "new") {
                    const fetchResult = await this.dataSource.fetchSession(curSessionId);
                    if (!this._isCurrentEpoch(epoch)) {
                        return;
                    }
                    if (fetchResult) {
                        this._updateSession({ session: fetchResult.session });
                        this._setTurnsFromFetch(fetchResult.turns, fetchResult.pendingQuestion || null);
                    }
                }
            }
            else {
                this._updateSession({
                    error: result.error || "Failed to reject question",
                    errorRetryable: false,
                });
            }
        }
        catch (err) {
            if (!this._isCurrentEpoch(epoch)) {
                return;
            }
            const normalized = normalizeRPCError(err, "Failed to reject question");
            this._updateSession({
                error: normalized.userMessage,
                errorRetryable: normalized.retryable,
            });
        }
        finally {
            // Question resolved — resume any messages queued while it was open.
            if (this._isCurrentEpoch(epoch) && !this.state.session.error) {
                this._drainQueueIfReady(epoch);
            }
        }
    }
    // ── Input / queue ───────────────────────────────────────────────────────
    private _handleSubmit(e: {
        preventDefault: () => void;
    }, attachments: Attachment[] = []): void {
        e.preventDefault();
        const msg = this.state.input.message;
        if (!msg.trim() && attachments.length === 0) {
            return;
        }
        this._updateInput({ inputError: null });
        this._clearStreamError();
        const convertedAttachments: Attachment[] = attachments.map((att) => ({
            clientKey: att.clientKey || crypto.randomUUID(),
            id: att.id,
            uploadId: att.uploadId,
            filename: att.filename,
            mimeType: att.mimeType,
            sizeBytes: att.sizeBytes,
            base64Data: att.base64Data,
            url: att.url,
            preview: att.preview,
        }));
        // Queue if currently generating
        if (this.state.messaging.loading) {
            const ok = this._enqueueMessage(msg.trim(), convertedAttachments);
            if (ok) {
                this._updateInput({ message: "" });
            }
            return;
        }
        this._sendMessage(msg.trim(), convertedAttachments);
    }
    private _handleUnqueue(): {
        content: string;
        attachments: Attachment[];
    } | null {
        const queue = this.state.input.messageQueue;
        if (queue.length === 0) {
            return null;
        }
        const last = queue[queue.length - 1];
        this._updateInput({ messageQueue: queue.slice(0, -1) });
        return { content: last.content, attachments: last.attachments };
    }
    private _enqueueMessage(content: string, attachments: Attachment[]): boolean {
        if (this.state.input.messageQueue.length >= MAX_QUEUE_SIZE) {
            this._updateInput({ inputError: "BiChat.Input.QueueFull" });
            return false;
        }
        this._updateInput({
            messageQueue: [
                ...this.state.input.messageQueue,
                { content, attachments },
            ],
        });
        return true;
    }
    private _removeQueueItem(index: number): void {
        this._updateInput({
            messageQueue: this.state.input.messageQueue.filter((_, i) => i !== index),
        });
    }
    private _updateQueueItem(index: number, content: string): void {
        this._updateInput({
            messageQueue: this.state.input.messageQueue.map((item, i) => i === index ? { ...item, content } : item),
        });
    }
}
// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function sessionDebugUsageEqual(a: SessionDebugUsage | null, b: SessionDebugUsage): boolean {
    if (!a) {
        return false;
    }
    return (a.promptTokens === b.promptTokens &&
        a.completionTokens === b.completionTokens &&
        a.totalTokens === b.totalTokens &&
        a.turnsWithUsage === b.turnsWithUsage &&
        a.latestPromptTokens === b.latestPromptTokens &&
        a.latestCompletionTokens === b.latestCompletionTokens &&
        a.latestTotalTokens === b.latestTotalTokens);
}
