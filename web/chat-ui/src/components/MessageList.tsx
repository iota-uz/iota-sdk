import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * MessageList — displays conversation turns with auto-scroll and grouping.
 *
 * Uses turn-based architecture where each ConversationTurn groups
 * a user message with its assistant response.
 */
import { useChatSession, useChatMessaging } from '../context/ChatContext';
import { ConversationTurn } from '../types';
import { TurnBubble } from './TurnBubble';
import { TypingIndicator } from './TypingIndicator';
import { ActivityTrace } from './ActivityTrace';
import StreamingCursor from './StreamingCursor';
import ScrollToBottomButton from './ScrollToBottomButton';
import { DateSeparator } from './DateSeparator';
import { normalizeStreamingMarkdown } from '../utils/markdownStream';
import { useMessageListScroll } from '../hooks/useMessageListScroll';
import { useTranslation } from '../hooks/useTranslation';
import { isSameDay } from 'date-fns';
// Eagerly start loading the chunk so it's ready before streaming begins
const markdownImport = import('./MarkdownRenderer');
const MarkdownRenderer = lazy(() => markdownImport.then((m) => ({ default: m.MarkdownRenderer })));
// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------
function MessageListSkeleton() {
    return (<div class="space-y-6" aria-hidden="true">
      <div class="flex justify-end">
        <div class="w-3/5 max-w-md rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
          <div class="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
          <div class="h-3 w-4/5 rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
        </div>
      </div>
      <div class="flex gap-3">
        <div class="w-8 h-8 rounded-full bg-gray-200 dark:bg-gray-700 animate-pulse shrink-0"/>
        <div class="w-4/5 max-w-lg rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
          <div class="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
          <div class="h-3 w-5/6 rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
          <div class="h-3 w-3/5 rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
        </div>
      </div>
      <div class="flex justify-end">
        <div class="w-2/5 max-w-xs rounded-2xl bg-gray-100 dark:bg-gray-800 p-4 space-y-2">
          <div class="h-3 w-full rounded bg-gray-200 dark:bg-gray-700 animate-pulse"/>
        </div>
      </div>
    </div>);
}
function StreamingBubble(solidProps1: {
    content: string;
    normalizedContent: string;
}) {
    return (<div class="flex min-w-0 gap-3">
      <div class="flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-xs">
        AI
      </div>
      <div class="flex-1 min-w-0 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-2xl rounded-bl-sm px-4 py-3 text-gray-900 dark:text-gray-100" style={{ "max-width": 'var(--bichat-bubble-assistant-max-width, 85%)' }}>
        <Suspense fallback={<div class="prose prose-sm max-w-none dark:prose-invert whitespace-pre-wrap">
              {solidProps1.content}
            </div>}>
          <MarkdownRenderer content={solidProps1.normalizedContent} sendDisabled/>
        </Suspense>
        <StreamingCursor />
      </div>
    </div>);
}
// ---------------------------------------------------------------------------
// MessageList
// ---------------------------------------------------------------------------
interface MessageListProps {
    renderUserTurn?: (turn: ConversationTurn) => JSX.Element;
    renderAssistantTurn?: (turn: ConversationTurn) => JSX.Element;
    /** Host-owned workflow/artifact placed in the scrollable conversation history. */
    historyArtifactSlot?: JSX.Element;
    thinkingVerbs?: string[];
    readOnly?: boolean;
}
export function MessageList(solidProps2: MessageListProps) {
    const solidState3 = useTranslation();
    const solidState4 = useChatSession();
    const solidState5 = useChatMessaging();
    const solidState6 = useMessageListScroll({ get currentSessionId() {
            return solidState4.currentSessionId;
        }, get fetching() {
            return solidState4.fetching;
        }, get turnsLength() { return solidState5.turns.length; }, get streamingContent() {
            return solidState5.streamingContent;
        } });
    const normalizedStreaming = createMemo(() => (solidState5.streamingContent ? normalizeStreamingMarkdown(solidState5.streamingContent) : ''));
    const showAuthorNames = createMemo(() => Boolean(solidState4.session?.isGroup));
    const showEphemeral = createMemo(() => solidState5.showActivityTrace || solidState5.showTypingIndicator);
    return (<div class="relative flex-1 min-w-0 min-h-0">
      <div ref={element => solidState6.containerRef.current = element} data-testid="bichat-message-list" class="h-full overflow-y-auto overflow-x-hidden px-4 py-6">
        <div class="mx-auto w-full min-w-0 space-y-6">
          {solidState4.fetching && solidState5.turns.length === 0 && <MessageListSkeleton />}

          {solidState5.turns.map((turn, index) => {
            const turnDate = new Date(turn.createdAt);
            const prevDate = createMemo(() => index > 0 ? new Date(solidState5.turns[index - 1].createdAt) : null);
            const showDateSeparator = createMemo(() => !!prevDate() && !isSameDay(turnDate, prevDate()!));
            const isLast = createMemo(() => index === solidState5.turns.length - 1);
            const userTurnProps = createMemo(() => ({
                allowEdit: solidProps2.readOnly ? false : isLast(),
                showAuthorName: showAuthorNames(),
            }));
            return (<>
                {showDateSeparator() && <DateSeparator date={turnDate}/>}
                <TurnBubble turn={turn} isLastTurn={isLast()} renderUserTurn={solidProps2.renderUserTurn} renderAssistantTurn={solidProps2.renderAssistantTurn} userTurnProps={userTurnProps()} assistantTurnProps={solidProps2.readOnly ? { allowRegenerate: false } : undefined}/>
              </>);
        })}

          {solidProps2.historyArtifactSlot && (<div class="min-w-0" data-bichat-history-artifact>
              {solidProps2.historyArtifactSlot}
            </div>)}

          {solidState5.isStreaming && solidState5.streamingContent && (<StreamingBubble content={solidState5.streamingContent} normalizedContent={normalizedStreaming()}/>)}

          
            {showEphemeral() && (<div>
                {solidState5.showActivityTrace && (<ActivityTrace thinkingContent={solidState5.thinkingContent} activeSteps={solidState5.activeSteps}/>)}
                {solidState5.showTypingIndicator && <TypingIndicator verbs={solidProps2.thinkingVerbs}/>}
              </div>)}
          

          <div ref={element => solidState6.messagesEndRef.current = element}/>
        </div>
      </div>

      <ScrollToBottomButton show={solidState6.showScrollButton} onClick={solidState6.handleScrollToBottom} unreadCount={solidState6.unreadCount} label={solidState5.isStreaming && solidState6.showScrollButton ? solidState3.t('BiChat.ScrollToBottom.NewMessages') : undefined}/>
    </div>);
}
