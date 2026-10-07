/**
 * Snapshot derivation helpers for the ChatMachine.
 *
 * These are pure functions that derive public snapshots from internal state.
 */
import { getSessionDebugUsage } from '../utils/debugTrace';
import type { ChatMachineState, SessionSnapshot, MessagingSnapshot, InputSnapshot } from './types';
export function deriveDebugMode(state: ChatMachineState): boolean {
    const key = state.session.currentSessionId || 'new';
    return state.session.debugModeBySession[key] ?? false;
}
export function deriveSessionSnapshot(state: ChatMachineState, methods: Pick<SessionSnapshot, 'setError' | 'retryFetchSession' | 'setReasoningEffort' | 'setModel'>): SessionSnapshot {
    return {
        session: state.session.session,
        currentSessionId: state.session.currentSessionId,
        fetching: state.session.fetching,
        error: state.session.error,
        errorRetryable: state.session.errorRetryable,
        debugMode: deriveDebugMode(state),
        sessionDebugUsage: getSessionDebugUsage(state.messaging.turns),
        debugLimits: state.session.debugLimits,
        reasoningEffort: state.session.reasoningEffort,
        reasoningEffortOptions: state.session.reasoningEffortOptions,
        model: state.session.model,
        setError: methods.setError,
        retryFetchSession: methods.retryFetchSession,
        setReasoningEffort: methods.setReasoningEffort,
        setModel: methods.setModel,
    };
}
export function deriveMessagingSnapshot(state: ChatMachineState, methods: Pick<MessagingSnapshot, 'sendMessage' | 'handleRegenerate' | 'handleEdit' | 'handleCopy' | 'handleSubmitQuestionAnswers' | 'handleRejectPendingQuestion' | 'retryLastMessage' | 'clearStreamError' | 'cancel' | 'setCodeOutputs'>): MessagingSnapshot {
    const { turns, loading, isStreaming, streamingContent, isCompacting, streamError, streamErrorRetryable, pendingQuestion, codeOutputs, artifactsInvalidationTrigger, thinkingContent, activeSteps, generationInProgress, } = state.messaging;
    // ---------------------------------------------------------------------------
    // Activity indicator visibility
    //
    // UX phases during a streaming response:
    //
    //   Phase              | ActivityTrace | TypingIndicator
    //   -------------------|:---:|:---:
    //   Waiting            |  -  | visible    (message sent, no events yet)
    //   Tool execution     | vis | visible    (tool events arriving, no content)
    //   Streaming          |  -  |  -         (content flowing, all tools done)
    //   Streaming + tools  | vis |  -         (content flowing, new tool starts)
    // ---------------------------------------------------------------------------
    const hasAnySteps = activeSteps.length > 0;
    const hasActiveSteps = activeSteps.some(s => s.status === 'active');
    // Before content: show if any steps exist (including completed).
    // After content starts: show only if a tool is still actively running.
    const showActivityTrace = !isCompacting && isStreaming
        && (streamingContent ? hasActiveSteps : hasAnySteps);
    const showTypingIndicator = !isCompacting && !thinkingContent && !streamingContent
        && (loading || (isStreaming && hasAnySteps));
    return {
        turns,
        streamingContent,
        isStreaming,
        streamError,
        streamErrorRetryable,
        loading,
        pendingQuestion,
        codeOutputs,
        isCompacting,
        compactionSummary: null,
        artifactsInvalidationTrigger,
        thinkingContent,
        activeSteps,
        generationInProgress,
        showActivityTrace,
        showTypingIndicator,
        ...methods,
    };
}
export function deriveInputSnapshot(state: ChatMachineState, methods: Pick<InputSnapshot, 'setMessage' | 'setInputError' | 'handleSubmit' | 'handleUnqueue' | 'enqueueMessage' | 'removeQueueItem' | 'updateQueueItem'>): InputSnapshot {
    return {
        message: state.input.message,
        inputError: state.input.inputError,
        messageQueue: state.input.messageQueue,
        ...methods,
    };
}
