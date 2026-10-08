import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * TurnBubble component (Layer 4 - Backward Compatible)
 * Container for a conversation turn (user message + assistant response)
 *
 * Renders both the user's message and the assistant's response in a single
 * visual grouping. If the assistant hasn't responded yet, only shows user message.
 *
 * For primitive-level control, use Turn from '@eai/chat-ui/primitives'
 */
import type { ConversationTurn } from '../types';
import { UserTurnView, type UserTurnViewProps } from './UserTurnView';
import { AssistantTurnView, type AssistantTurnViewProps } from './AssistantTurnView';
import type { UserMessageSlots, UserMessageClassNames } from './UserMessage';
import type { AssistantMessageSlots, AssistantMessageClassNames } from './AssistantMessage';
export interface TurnBubbleClassNames {
    /** Root container */
    root?: string;
    /** User turn wrapper */
    userTurn?: string;
    /** Assistant turn wrapper */
    assistantTurn?: string;
}
export interface TurnBubbleProps {
    /** The conversation turn containing user and optional assistant content */
    turn: ConversationTurn;
    /** When true, this turn is the last in the list (e.g. Regenerate shows only on last assistant message) */
    isLastTurn?: boolean;
    /** Custom render function for user turn (full control) */
    renderUserTurn?: (turn: ConversationTurn) => JSX.Element;
    /** Custom render function for assistant turn (full control) */
    renderAssistantTurn?: (turn: ConversationTurn) => JSX.Element;
    /** Props passed to UserTurnView (when not using custom renderer) */
    userTurnProps?: Omit<UserTurnViewProps, 'turn'>;
    /** Props passed to AssistantTurnView (when not using custom renderer) */
    assistantTurnProps?: Omit<AssistantTurnViewProps, 'turn'>;
    /** Slots for user message customization */
    userMessageSlots?: UserMessageSlots;
    /** Slots for assistant message customization */
    assistantMessageSlots?: AssistantMessageSlots;
    /** Class names for user message */
    userMessageClassNames?: UserMessageClassNames;
    /** Class names for assistant message */
    assistantMessageClassNames?: AssistantMessageClassNames;
    /** Class names for turn bubble container */
    classNames?: TurnBubbleClassNames;
    /** Whether assistant response is streaming */
    isStreaming?: boolean;
}
const defaultClassNames: Required<TurnBubbleClassNames> = {
    root: 'space-y-4 min-w-0',
    userTurn: '',
    assistantTurn: '',
};
export function TurnBubble(solidProps1Input: TurnBubbleProps) {
    const solidProps1 = mergeProps({ isLastTurn: false, isStreaming: false } as const, solidProps1Input);
    const classes = createMemo(() => ({
        root: solidProps1.classNames?.root ?? defaultClassNames.root,
        userTurn: solidProps1.classNames?.userTurn ?? defaultClassNames.userTurn,
        assistantTurn: solidProps1.classNames?.assistantTurn ?? defaultClassNames.assistantTurn,
    }));
    const userContent = createMemo(() => typeof solidProps1.turn.userTurn?.content === 'string' ? solidProps1.turn.userTurn.content : '');
    const isSystemSummaryTurn = createMemo(() => userContent().trim() === '' && solidProps1.turn.assistantTurn?.role === 'system');
    return (<div class={classes().root} data-turn-id={solidProps1.turn.id}>
      {/* User message */}
      {!isSystemSummaryTurn() && (<div class={classes().userTurn}>
          {solidProps1.renderUserTurn ? (solidProps1.renderUserTurn(solidProps1.turn)) : (<UserTurnView turn={solidProps1.turn} slots={solidProps1.userMessageSlots} classNames={solidProps1.userMessageClassNames} {...solidProps1.userTurnProps}/>)}
        </div>)}

      {/* Assistant response (if available) */}
      {solidProps1.turn.assistantTurn && (<div class={classes().assistantTurn}>
          {solidProps1.renderAssistantTurn ? (solidProps1.renderAssistantTurn(solidProps1.turn)) : (<AssistantTurnView turn={solidProps1.turn} isLastTurn={solidProps1.isLastTurn} isStreaming={solidProps1.isStreaming} slots={solidProps1.assistantMessageSlots} classNames={solidProps1.assistantMessageClassNames} {...solidProps1.assistantTurnProps}/>)}
        </div>)}
    </div>);
}
export default TurnBubble;
