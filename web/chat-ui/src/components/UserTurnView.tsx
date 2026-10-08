import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * UserTurnView Component (Layer 4 - Backward Compatible)
 * Displays user messages with attachments, image modal, and actions
 *
 * Uses turn-based architecture - receives a ConversationTurn and displays
 * the userTurn content.
 *
 * For more customization, use the UserMessage component directly with slots.
 */
import { useChatMessaging } from '../context/ChatContext';
import { UserMessage, type UserMessageSlots, type UserMessageClassNames } from './UserMessage';
import type { ConversationTurn } from '../types';
export interface UserTurnViewProps {
    /** The conversation turn containing the user message */
    turn: ConversationTurn;
    /** Slot overrides for customization */
    slots?: UserMessageSlots;
    /** Class name overrides */
    classNames?: UserMessageClassNames;
    /** User initials for avatar */
    initials?: string;
    /** Hide avatar */
    hideAvatar?: boolean;
    /** Hide actions */
    hideActions?: boolean;
    /** Hide timestamp */
    hideTimestamp?: boolean;
    /** Whether edit action should be available */
    allowEdit?: boolean;
    /** Show sender identity label above the message bubble */
    showAuthorName?: boolean;
}
export function UserTurnView(solidProps1Input: UserTurnViewProps) {
    const solidProps1 = mergeProps({ showAuthorName: false } as const, solidProps1Input);
    const solidState2 = useChatMessaging();
    const author = createMemo(() => solidProps1.turn.userTurn.author);
    const fullName = createMemo(() => [author()?.firstName || '', author()?.lastName || ''].join(' ').trim());
    const authorName = createMemo(() => solidProps1.showAuthorName && fullName().length > 0 ? fullName() : undefined);
    const resolvedInitials = createMemo(() => solidProps1.initials ?? author()?.initials ?? 'U');
    return (<UserMessage turn={solidProps1.turn.userTurn} turnId={solidProps1.turn.id} initials={resolvedInitials()} authorName={authorName()} slots={solidProps1.slots} classNames={solidProps1.classNames} onCopy={solidState2.handleCopy} onEdit={solidState2.handleEdit} hideAvatar={solidProps1.hideAvatar} hideActions={solidProps1.hideActions} hideTimestamp={solidProps1.hideTimestamp} allowEdit={solidProps1.allowEdit}/>);
}
export default UserTurnView;
