/**
 * Internal types for the ChatMachine.
 *
 * These are implementation details — consumers use the existing
 * ChatSessionStateValue / ChatMessagingStateValue / ChatInputStateValue
 * types via the public hooks.
 */
import type { Session, ConversationTurn, PendingQuestion, CodeOutput, QueuedMessage, Attachment, ActivityStep, ChatDataSource, SessionDebugUsage, DebugLimits, SendMessageOptions, QuestionAnswers, } from '../types';
import type { RateLimiter } from '../utils/RateLimiter';
// ---------------------------------------------------------------------------
// Machine configuration
// ---------------------------------------------------------------------------
export interface ChatMachineConfig {
    dataSource: ChatDataSource;
    rateLimiter: RateLimiter;
    onSessionCreated?: (sessionId: string) => void;
}
// ---------------------------------------------------------------------------
// Internal state
// ---------------------------------------------------------------------------
export interface SessionState {
    currentSessionId: string | undefined;
    session: Session | null;
    fetching: boolean;
    error: string | null;
    errorRetryable: boolean;
    debugModeBySession: Record<string, boolean>;
    debugLimits: DebugLimits | null;
    reasoningEffort: string | undefined;
    reasoningEffortOptions?: string[];
    model: string | undefined;
}
export interface MessagingState {
    turns: ConversationTurn[];
    streamingContent: string;
    isStreaming: boolean;
    streamError: string | null;
    streamErrorRetryable: boolean;
    loading: boolean;
    pendingQuestion: PendingQuestion | null;
    codeOutputs: CodeOutput[];
    isCompacting: boolean;
    artifactsInvalidationTrigger: number;
    /** Ephemeral: accumulated reasoning/thinking content during streaming. */
    thinkingContent: string;
    /** Ephemeral: ordered list of activity steps (thinking, tools, delegations). */
    activeSteps: ActivityStep[];
    /** True when another tab/device has an active stream; show "generation in progress" and poll. */
    generationInProgress: boolean;
}
export interface InputState {
    message: string;
    inputError: string | null;
    messageQueue: QueuedMessage[];
}
export interface ChatMachineState {
    session: SessionState;
    messaging: MessagingState;
    input: InputState;
}
// ---------------------------------------------------------------------------
// Snapshot types — satisfy existing context value interfaces
// ---------------------------------------------------------------------------
/**
 * Mirrors ChatSessionStateValue. Methods are stable (bound once on
 * machine construction) so they never trigger re-renders by identity change.
 */
export interface SessionSnapshot {
    session: Session | null;
    currentSessionId?: string;
    fetching: boolean;
    error: string | null;
    errorRetryable: boolean;
    debugMode: boolean;
    sessionDebugUsage: SessionDebugUsage;
    debugLimits: DebugLimits | null;
    reasoningEffort: string | undefined;
    reasoningEffortOptions: string[] | undefined;
    model: string | undefined;
    setError: (error: string | null) => void;
    retryFetchSession: () => void;
    setReasoningEffort: (effort: string) => void;
    setModel: (model: string | undefined) => void;
}
/** Superset of ChatMessagingStateValue used internally by the state machine; includes internal-only fields such as generationInProgress. */
export interface MessagingSnapshot {
    turns: ConversationTurn[];
    streamingContent: string;
    isStreaming: boolean;
    streamError: string | null;
    streamErrorRetryable: boolean;
    loading: boolean;
    pendingQuestion: PendingQuestion | null;
    codeOutputs: CodeOutput[];
    isCompacting: boolean;
    compactionSummary: string | null;
    artifactsInvalidationTrigger: number;
    thinkingContent: string;
    activeSteps: ActivityStep[];
    generationInProgress: boolean;
    showActivityTrace: boolean;
    showTypingIndicator: boolean;
    sendMessage: (content: string, attachments?: Attachment[]) => Promise<void>;
    handleRegenerate?: (turnId: string, model?: string) => Promise<void>;
    handleEdit?: (turnId: string, newContent: string) => Promise<void>;
    handleCopy: (text: string) => Promise<void>;
    handleSubmitQuestionAnswers: (answers: QuestionAnswers) => void;
    handleRejectPendingQuestion: () => Promise<void>;
    retryLastMessage: () => Promise<void>;
    clearStreamError: () => void;
    cancel: () => void;
    setCodeOutputs: (outputs: CodeOutput[]) => void;
}
/** Mirrors ChatInputStateValue. */
export interface InputSnapshot {
    message: string;
    inputError: string | null;
    messageQueue: QueuedMessage[];
    setMessage: (message: string) => void;
    setInputError: (error: string | null) => void;
    handleSubmit: (e: {
        preventDefault: () => void;
    }, attachments?: Attachment[]) => void;
    handleUnqueue: () => {
        content: string;
        attachments: Attachment[];
    } | null;
    enqueueMessage: (content: string, attachments: Attachment[]) => boolean;
    removeQueueItem: (index: number) => void;
    updateQueueItem: (index: number, content: string) => void;
}
// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------
export interface LastSendAttempt {
    content: string;
    attachments: Attachment[];
    options?: SendMessageOptions;
}
