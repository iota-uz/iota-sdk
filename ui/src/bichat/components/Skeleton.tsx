/**
 * Skeleton Component
 * Reusable loading skeleton with multiple variants
 */

import { memo } from 'react'

export interface SkeletonProps {
  /** Skeleton variant */
  variant?: 'text' | 'circular' | 'rectangular' | 'rounded'
  /** Width (CSS value or number for pixels) */
  width?: string | number
  /** Height (CSS value or number for pixels) */
  height?: string | number
  /** Additional CSS classes */
  className?: string
  /** Enable animation */
  animate?: boolean
}

export interface SkeletonGroupProps {
  /** Number of skeleton items to render */
  count?: number
  /** Gap between items */
  gap?: 'sm' | 'md' | 'lg'
  /** Additional CSS classes for the container */
  className?: string
  /** Render function for each skeleton item */
  children?: (index: number) => React.ReactNode
}

const variantClasses = {
  text: 'rounded',
  circular: 'rounded-full',
  rectangular: 'rounded-none',
  rounded: 'rounded-lg',
}

const gapClasses = {
  sm: 'space-y-1',
  md: 'space-y-2',
  lg: 'space-y-3',
}

function Skeleton({
  variant = 'text',
  width,
  height,
  className = '',
  animate = true,
}: SkeletonProps) {
  const variantClass = variantClasses[variant]

  const style: React.CSSProperties = {
    width: typeof width === 'number' ? `${width}px` : width,
    height: typeof height === 'number' ? `${height}px` : height,
  }

  return (
    <div
      className={`bg-gray-200 dark:bg-gray-700 ${variantClass} ${animate ? 'animate-pulse' : ''} ${className}`}
      style={style}
      aria-hidden="true"
    />
  )
}

/**
 * SkeletonGroup - Renders multiple skeleton items
 */
export function SkeletonGroup({
  count = 3,
  gap = 'md',
  className = '',
  children,
}: SkeletonGroupProps) {
  const gapClass = gapClasses[gap]

  return (
    <div className={`${gapClass} ${className}`} aria-hidden="true">
      {Array.from({ length: count }).map((_, index) =>
        children ? (
          <div key={index}>{children(index)}</div>
        ) : (
          <Skeleton key={index} variant="text" height={16} />
        )
      )}
    </div>
  )
}

/**
 * SkeletonText - Text line skeleton with configurable width
 */
export function SkeletonText({
  lines = 1,
  className = '',
}: {
  lines?: number
  className?: string
}) {
  const widths = ['100%', '90%', '80%', '95%', '85%']

  return (
    <div className={`space-y-2 ${className}`} aria-hidden="true">
      {Array.from({ length: lines }).map((_, index) => (
        <Skeleton
          key={index}
          variant="text"
          width={widths[index % widths.length]}
          height={14}
        />
      ))}
    </div>
  )
}

/**
 * SkeletonAvatar - Circular avatar skeleton
 */
export function SkeletonAvatar({
  size = 40,
  className = '',
}: {
  size?: number
  className?: string
}) {
  return (
    <Skeleton
      variant="circular"
      width={size}
      height={size}
      className={className}
    />
  )
}

/**
 * SkeletonCard - Card-shaped skeleton
 */
export function SkeletonCard({
  width,
  height = 120,
  className = '',
}: {
  width?: string | number
  height?: string | number
  className?: string
}) {
  return (
    <Skeleton
      variant="rounded"
      width={width}
      height={height}
      className={className}
    />
  )
}

/**
 * ListItemSkeleton - Common list item skeleton with icon and text
 */
export function ListItemSkeleton({ className = '' }: { className?: string }) {
  return (
    <div className={`flex items-center gap-3 px-3 py-2 ${className}`}>
      <Skeleton variant="rounded" width={20} height={20} />
      <Skeleton variant="text" height={16} className="flex-1" />
    </div>
  )
}

export default memo(Skeleton)
