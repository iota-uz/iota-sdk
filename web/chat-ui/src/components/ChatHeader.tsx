import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Chat header component
 * Displays session title, controls, and group chat indicators.
 *
 * Supports customization via:
 * - logoSlot: Custom logo component
 * - actionsSlot: Custom action buttons
 * - members / onMembersClick: Avatar stack for group chats
 * - Translations for "New Chat", "Archived", etc.
 */
import { Session } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import { useBranding } from '../hooks/useBranding';
import { AvatarStack } from './AvatarStack';
interface ChatHeaderProps {
    session: Session | null;
    onBack?: () => void;
    readOnly?: boolean;
    /** Custom logo component to display */
    logoSlot?: JSX.Element;
    /** Custom action buttons */
    actionsSlot?: JSX.Element;
    /** Members to display in avatar stack for group chats */
    members?: Array<{
        firstName: string;
        lastName: string;
        initials?: string;
    }>;
    /** Callback when avatar stack is clicked (to open members modal) */
    onMembersClick?: () => void;
}
export function ChatHeader(solidProps1: ChatHeaderProps) {
    const solidState2 = useTranslation();
    const branding = useBranding();
    const BackButton = solidProps1.onBack ? (<button type="button" onClick={solidProps1.onBack} class="cursor-pointer p-2 hover:bg-gray-100 dark:hover:bg-gray-700 active:bg-gray-200 dark:active:bg-gray-600 rounded-lg transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50" aria-label={solidState2.t('BiChat.Chat.GoBack')}>
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width={2} d="M15 19l-7-7 7-7"/>
      </svg>
    </button>) : null;
    const Logo = solidProps1.logoSlot || (branding.logoUrl ? (<img src={branding.logoUrl} alt={branding.appName} class="h-6 w-auto"/>) : null);
    return <>{createMemo(() => {
            if (!solidProps1.session) {
                return (<header class="bichat-header border-b border-gray-200 dark:border-gray-700 px-4 py-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            {BackButton}
            {Logo}
            <h1 class="text-lg font-semibold text-[var(--bichat-text)]">
              {solidState2.t('BiChat.Chat.NewChat')}
            </h1>
          </div>
          {solidProps1.actionsSlot && <div class="flex items-center gap-2">{solidProps1.actionsSlot}</div>}
        </div>
      </header>);
            }
            const resolvedSessionTitle = createMemo(() => solidProps1.session?.title?.trim() || solidState2.t('BiChat.Chat.NewChat'));
            const isGroupSession = Boolean(solidProps1.session.isGroup || (solidProps1.session.memberCount && solidProps1.session.memberCount > 1));
            const memberCount = solidProps1.session.memberCount ?? 0;
            // Avatar stack: use provided members or empty array
            const stackUsers = solidProps1.members && solidProps1.members.length > 0 ? solidProps1.members : [];
            return (<header class="bichat-header border-b border-gray-200 dark:border-gray-700 px-4 py-3">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3 min-w-0">
          {BackButton}
          {Logo}
          <div class="min-w-0">
            <div class="flex items-center gap-2.5">
              <h1 class="text-lg font-semibold text-[var(--bichat-text)] truncate">{resolvedSessionTitle()}</h1>
              {solidProps1.session.pinned && (<svg class="w-4 h-4 text-[var(--bichat-primary)] flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" role="img" aria-label={solidState2.t('BiChat.Chat.Pinned')}>
                  <path d="M10 2a1 1 0 011 1v1.323l3.954 1.582 1.599-.8a1 1 0 01.894 1.79l-1.233.616 1.738 5.42a1 1 0 01-.285 1.05A3.989 3.989 0 0115 15a3.989 3.989 0 01-2.667-1.019 1 1 0 01-.285-1.05l1.715-5.349L11 6.477V16h2a1 1 0 110 2H7a1 1 0 110-2h2V6.477L6.237 7.582l1.715 5.349a1 1 0 01-.285 1.05A3.989 3.989 0 015 15a3.989 3.989 0 01-2.667-1.019 1 1 0 01-.285-1.05l1.738-5.42-1.233-.617a1 1 0 01.894-1.788l1.599.799L9 4.323V3a1 1 0 011-1z"/>
                </svg>)}
              {isGroupSession && stackUsers.length > 0 && (<AvatarStack users={stackUsers} max={3} size="xs" onClick={solidProps1.onMembersClick} className="flex-shrink-0"/>)}
            </div>
            {isGroupSession && memberCount > 0 && (<p class="text-xs text-gray-600 dark:text-gray-400 mt-0.5">
                {memberCount === 1
                        ? solidState2.t('BiChat.Chat.OneMember')
                        : solidState2.t('BiChat.Chat.MemberCount').replace('{{count}}', String(memberCount))}
              </p>)}
          </div>
        </div>
        <div class="flex items-center gap-2 flex-shrink-0">
          {solidProps1.readOnly && (<span class="px-2 py-1 text-xs bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-200 rounded">
              {solidState2.t('BiChat.Chat.ReadOnly')}
            </span>)}
          {solidProps1.session.status === 'archived' && (<span class="px-2 py-1 text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded">
              {solidState2.t('BiChat.Chat.Archived')}
            </span>)}
          {solidProps1.actionsSlot}
        </div>
      </div>
    </header>);
        })}</>;
}
