import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * AvatarStack Component
 * Displays overlapping user avatars with an overflow "+N" indicator.
 * Used in ChatHeader and SessionItem for group chat visualization.
 */
import { UserAvatar } from './UserAvatar';
export interface AvatarStackProps {
    /** List of users to display */
    users: Array<{
        firstName: string;
        lastName: string;
        initials?: string;
    }>;
    /** Maximum avatars to show before "+N" (default: 3) */
    max?: number;
    /** Avatar size */
    size?: 'xs' | 'sm';
    /** Click handler — makes the stack interactive */
    onClick?: () => void;
    /** Additional CSS classes */
    className?: string;
}
const overlapClasses = {
    xs: '-ml-1.5',
    sm: '-ml-2',
} as const;
const badgeSizeClasses = {
    xs: 'w-6 h-6 text-[10px]',
    sm: 'w-8 h-8 text-xs',
} as const;
function AvatarStackInner(solidProps1Input: AvatarStackProps) {
    const solidProps1 = mergeProps({ max: 3, size: 'sm', className: '' } as const, solidProps1Input);
    const visible = createMemo(() => solidProps1.users.slice(0, solidProps1.max));
    const overflow = createMemo(() => solidProps1.users.length - solidProps1.max);
    const interactive = createMemo(() => typeof solidProps1.onClick === 'function');
    const overlap = createMemo(() => overlapClasses[solidProps1.size]);
    const badgeSize = createMemo(() => badgeSizeClasses[solidProps1.size]);
    const handleKeyDown = (e: KeyboardEvent) => {
        if (interactive() && (e.key === 'Enter' || e.key === ' ')) {
            e.preventDefault();
            solidProps1.onClick!();
        }
    };
    return (<div class={`inline-flex items-center ${interactive() ? 'cursor-pointer transition-opacity hover:opacity-80' : ''} ${solidProps1.className}`} onClick={interactive() ? solidProps1.onClick : undefined} onKeyDown={interactive() ? handleKeyDown : undefined} role={interactive() ? 'button' : undefined} tabIndex={interactive() ? 0 : undefined} aria-label={interactive() ? `${solidProps1.users.length} members` : undefined}>
      {visible().map((user, i) => (<div class={`${i > 0 ? overlap() : ''} ring-2 ring-white dark:ring-gray-900 rounded-full`} style={{ "z-index": visible().length - i }}>
          <UserAvatar firstName={user.firstName} lastName={user.lastName} initials={user.initials} size={solidProps1.size}/>
        </div>))}
      {overflow() > 0 && (<div class={`${overlap()} ${badgeSize()} rounded-full bg-gray-200 dark:bg-gray-700 text-gray-600 dark:text-gray-300 font-medium flex items-center justify-center flex-shrink-0 ring-2 ring-white dark:ring-gray-900`} style={{ "z-index": 0 }}>
          +{overflow()}
        </div>)}
    </div>);
}
const AvatarStack = AvatarStackInner;
AvatarStack; /* Solid components are named by their declarations. */
export { AvatarStack };
export default AvatarStack;
