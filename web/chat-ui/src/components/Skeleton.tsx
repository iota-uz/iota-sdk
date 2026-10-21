import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * Skeleton Component
 * Reusable loading skeleton with multiple variants
 */
export interface SkeletonProps {
    /** Skeleton variant */
    variant?: 'text' | 'circular' | 'rectangular' | 'rounded';
    /** Width (CSS value or number for pixels) */
    width?: string | number;
    /** Height (CSS value or number for pixels) */
    height?: string | number;
    /** Additional CSS classes */
    className?: string;
    /** Enable animation */
    animate?: boolean;
}
export interface SkeletonGroupProps {
    /** Number of skeleton items to render */
    count?: number;
    /** Gap between items */
    gap?: 'sm' | 'md' | 'lg';
    /** Additional CSS classes for the container */
    className?: string;
    /** Render function for each skeleton item */
    children?: (index: number) => JSX.Element;
}
const variantClasses = {
    text: 'rounded',
    circular: 'rounded-full',
    rectangular: 'rounded-none',
    rounded: 'rounded-lg',
};
const gapClasses = {
    sm: 'space-y-1',
    md: 'space-y-2',
    lg: 'space-y-3',
};
function Skeleton(solidProps1Input: SkeletonProps) {
    const solidProps1 = mergeProps({ variant: 'text', className: '', animate: true } as const, solidProps1Input);
    const variantClass = createMemo(() => variantClasses[solidProps1.variant]);
    const style = createMemo<JSX.CSSProperties>(() => ({
        width: typeof solidProps1.width === 'number' ? `${solidProps1.width}px` : solidProps1.width,
        height: typeof solidProps1.height === 'number' ? `${solidProps1.height}px` : solidProps1.height,
    }));
    return (<div class={`bg-gray-200 dark:bg-gray-700 ${variantClass()} ${solidProps1.animate ? 'animate-pulse' : ''} ${solidProps1.className}`} style={style()} aria-hidden="true"/>);
}
/**
 * SkeletonGroup - Renders multiple skeleton items
 */
export function SkeletonGroup(solidProps2Input: SkeletonGroupProps) {
    const solidProps2 = mergeProps({ count: 3, gap: 'md', className: '' } as const, solidProps2Input);
    const gapClass = createMemo(() => gapClasses[solidProps2.gap]);
    return (<div class={`${gapClass()} ${solidProps2.className}`} aria-hidden="true">
      {Array.from({ length: solidProps2.count }).map((_, index) => solidProps2.children ? (<div>{solidProps2.children(index)}</div>) : (<Skeleton variant="text" height={16}/>))}
    </div>);
}
/**
 * SkeletonText - Text line skeleton with configurable width
 */
export function SkeletonText(solidProps3Input: {
    lines?: number;
    className?: string;
}) {
    const solidProps3 = mergeProps({ lines: 1, className: '' } as const, solidProps3Input);
    const widths = ['100%', '90%', '80%', '95%', '85%'];
    return (<div class={`space-y-2 ${solidProps3.className}`} aria-hidden="true">
      {Array.from({ length: solidProps3.lines }).map((_, index) => (<Skeleton variant="text" width={widths[index % widths.length]} height={14}/>))}
    </div>);
}
/**
 * SkeletonAvatar - Circular avatar skeleton
 */
export function SkeletonAvatar(solidProps4Input: {
    size?: number;
    className?: string;
}) {
    const solidProps4 = mergeProps({ size: 40, className: '' } as const, solidProps4Input);
    return (<Skeleton variant="circular" width={solidProps4.size} height={solidProps4.size} className={solidProps4.className}/>);
}
/**
 * SkeletonCard - Card-shaped skeleton
 */
export function SkeletonCard(solidProps5Input: {
    width?: string | number;
    height?: string | number;
    className?: string;
}) {
    const solidProps5 = mergeProps({ height: 120, className: '' } as const, solidProps5Input);
    return (<Skeleton variant="rounded" width={solidProps5.width} height={solidProps5.height} className={solidProps5.className}/>);
}
/**
 * ListItemSkeleton - Common list item skeleton with icon and text
 */
export function ListItemSkeleton(solidProps6Input: {
    className?: string;
}) {
    const solidProps6 = mergeProps({ className: '' } as const, solidProps6Input);
    return (<div class={`flex items-center gap-3 px-3 py-2 ${solidProps6.className}`}>
      <Skeleton variant="rounded" width={20} height={20}/>
      <Skeleton variant="text" height={16} className="flex-1"/>
    </div>);
}
const MemoizedSkeleton = Skeleton;
MemoizedSkeleton; /* Solid components are named by their declarations. */
export { MemoizedSkeleton as Skeleton };
export default MemoizedSkeleton;
