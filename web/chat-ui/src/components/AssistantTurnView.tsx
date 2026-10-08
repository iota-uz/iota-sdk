import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * AssistantTurnView Component (Layer 4 - Backward Compatible)
 * Displays assistant messages with markdown, charts, sources, downloads, code outputs, and streaming cursor
 *
 * Uses turn-based architecture - receives a ConversationTurn and displays
 * the assistantTurn content.
 *
 * For more customization, use the AssistantMessage component directly with slots.
 */
import { useChatSession, useChatMessaging } from '../context/ChatContext';
import { useIotaContext } from '../context/IotaContext';
import { AssistantMessage, type AssistantMessageSlots, type AssistantMessageClassNames, type RegenerateModelOption, } from './AssistantMessage';
import { SystemMessage } from './SystemMessage';
import type { ConversationTurn } from '../types';
export interface AssistantTurnViewProps {
    /** The conversation turn containing the assistant response */
    turn: ConversationTurn;
    /** When true, this is the last turn in the list (Regenerate button shown only on last assistant message) */
    isLastTurn?: boolean;
    /** Whether the response is currently being streamed */
    isStreaming?: boolean;
    /** Slot overrides for customization */
    slots?: AssistantMessageSlots;
    /** Class name overrides */
    classNames?: AssistantMessageClassNames;
    /** Hide avatar */
    hideAvatar?: boolean;
    /** Hide actions */
    hideActions?: boolean;
    /** Hide timestamp */
    hideTimestamp?: boolean;
    /** Whether regenerate action should be available */
    allowRegenerate?: boolean;
}
export function AssistantTurnView(solidProps1Input: AssistantTurnViewProps) {
    const solidProps1 = mergeProps({ isLastTurn: false, isStreaming: false, allowRegenerate: true } as const, solidProps1Input);
    const solidState2 = useChatSession();
    const solidState3 = useChatMessaging();
    const iotaContext = useIotaContext();
    const regenerateModels = createMemo<RegenerateModelOption[] | undefined>(() => {
        const models = iotaContext.extensions?.llm?.models;
        if (!models || models.length < 2) {
            return undefined;
        }
        return models.map((m) => ({ id: m.id, label: m.label }));
    });
    const assistantTurn = createMemo(() => solidProps1.turn.assistantTurn);
    return <Show when={!(!assistantTurn())}>{_visible => {
            return <Show when={assistantTurn()!.role === 'system'} fallback={<AssistantMessage turn={assistantTurn()!} turnId={solidProps1.turn.id} isLastTurn={solidProps1.isLastTurn} isStreaming={solidProps1.isStreaming} pendingQuestion={solidState3.pendingQuestion} slots={solidProps1.slots} classNames={solidProps1.classNames} onCopy={solidState3.handleCopy} onRegenerate={solidProps1.allowRegenerate ? solidState3.handleRegenerate : undefined} regenerateModels={solidProps1.allowRegenerate ? regenerateModels() : undefined} onSendMessage={solidState3.sendMessage} sendDisabled={solidState3.loading || solidProps1.isStreaming} hideAvatar={solidProps1.hideAvatar} hideActions={solidProps1.hideActions} hideTimestamp={solidProps1.hideTimestamp} showDebug={solidState2.debugMode}/>}>
            <SystemMessage content={assistantTurn()!.content} createdAt={assistantTurn()!.createdAt} onCopy={solidState3.handleCopy} hideActions={solidProps1.hideActions} hideTimestamp={solidProps1.hideTimestamp}/></Show>;
        }}</Show>;
}
export default AssistantTurnView;
