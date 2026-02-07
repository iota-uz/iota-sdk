/**
 * LoadingSpinner Component
 * Displays animated loading indicators
 */

import { memo } from 'react'
import { CircleNotch } from '@phosphor-icons/react'

type SpinnerVariant = 'spinner' | 'dots' | 'pulse'

interface LoadingSpinnerProps {
  variant?: SpinnerVariant
  size?: 'sm' | 'md' | 'lg'
  message?: string
}

function SpinnerLoader({
  size = 'md',
  message,
}: {
  size: 'sm' | 'md' | 'lg'
  message?: string
}) {
  const sizeMap = {
    sm: 16,
    md: 32,
    lg: 48,
  }

  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  }

  return (
    <div className="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <CircleNotch
        size={sizeMap[size]}
        className={`${sizeClasses[size]} animate-spin motion-reduce:animate-none text-[var(--bichat-primary)]`}
      />
      {message && <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">{message}</p>}
    </div>
  )
}

function DotsLoader({
  size = 'md',
  message,
}: {
  size: 'sm' | 'md' | 'lg'
  message?: string
}) {
  const dotSizeClasses = {
    sm: 'w-1.5 h-1.5',
    md: 'w-2 h-2',
    lg: 'w-3 h-3',
  }

  const gapClasses = {
    sm: 'gap-0.5',
    md: 'gap-1',
    lg: 'gap-1.5',
  }

  return (
    <div className="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <div className={`flex ${gapClasses[size]}`}>
        {[0, 1, 2].map((index) => (
          <div
            key={index}
            className={`${dotSizeClasses[size]} bg-[var(--bichat-primary)] rounded-full animate-bounce motion-reduce:animate-none`}
            style={{ animationDelay: `${index * 0.15}s` }}
          />
        ))}
      </div>
      {message && <p className="mt-3 text-sm text-gray-600 dark:text-gray-400">{message}</p>}
    </div>
  )
}

function PulseLoader({
  size = 'md',
  message,
}: {
  size: 'sm' | 'md' | 'lg'
  message?: string
}) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  }

  return (
    <div className="flex flex-col items-center justify-center" role="status" aria-live="polite">
      <div className={`${sizeClasses[size]} bg-[var(--bichat-primary)] rounded-full animate-pulse motion-reduce:animate-none`} />
      {message && <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">{message}</p>}
    </div>
  )
}

function LoadingSpinner({ variant = 'spinner', size = 'md', message }: LoadingSpinnerProps) {
  switch (variant) {
    case 'dots':
      return <DotsLoader size={size} message={message} />
    case 'pulse':
      return <PulseLoader size={size} message={message} />
    case 'spinner':
    default:
      return <SpinnerLoader size={size} message={message} />
  }
}

const MemoizedLoadingSpinner = memo(LoadingSpinner)
MemoizedLoadingSpinner.displayName = 'LoadingSpinner'

export { MemoizedLoadingSpinner as LoadingSpinner }
export default MemoizedLoadingSpinner
