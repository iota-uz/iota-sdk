/**
 * UserAvatar Component
 * Displays user initials with deterministic color from a color palette
 */

import { memo } from 'react'

export interface UserAvatarProps {
  /** User's first name */
  firstName: string
  /** User's last name */
  lastName: string
  /** Override initials (defaults to first letters of first and last name) */
  initials?: string
  /** Avatar size */
  size?: 'sm' | 'md' | 'lg'
  /** Additional CSS classes */
  className?: string
}

/**
 * Generate a consistent color index from a string
 * Uses simple hash function for deterministic color selection
 */
function hashString(str: string): number {
  let hash = 0
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i)
    hash = (hash << 5) - hash + char
    hash = hash & hash // Convert to 32bit integer
  }
  return Math.abs(hash)
}

/**
 * Color palette using Tailwind colors
 * Selected for good contrast with white text
 */
const colorPalette = [
  { bg: 'bg-blue-500', text: 'text-white' },
  { bg: 'bg-green-500', text: 'text-white' },
  { bg: 'bg-purple-500', text: 'text-white' },
  { bg: 'bg-pink-500', text: 'text-white' },
  { bg: 'bg-indigo-500', text: 'text-white' },
  { bg: 'bg-teal-500', text: 'text-white' },
  { bg: 'bg-orange-500', text: 'text-white' },
  { bg: 'bg-cyan-500', text: 'text-white' },
  { bg: 'bg-amber-500', text: 'text-white' },
  { bg: 'bg-lime-500', text: 'text-white' },
]

/**
 * Size configurations
 */
const sizeClasses = {
  sm: 'w-8 h-8 text-xs',
  md: 'w-10 h-10 text-sm',
  lg: 'w-12 h-12 text-base',
}

function UserAvatar({
  firstName,
  lastName,
  initials: providedInitials,
  size = 'md',
  className = '',
}: UserAvatarProps) {
  // Generate initials if not provided
  const derivedInitials = (() => {
    const firstChar = firstName?.trim()?.charAt(0) || ''
    const lastChar = lastName?.trim()?.charAt(0) || ''
    const combined = `${firstChar}${lastChar}`.trim()
    return combined || 'U'
  })()

  const initials = (providedInitials?.trim() || derivedInitials).toUpperCase()

  // Select color based on full name hash (deterministic)
  const fullName = `${firstName}${lastName}`
  const colorIndex = hashString(fullName) % colorPalette.length
  const colors = colorPalette[colorIndex]

  return (
    <div
      className={`
        ${sizeClasses[size]}
        ${colors.bg}
        ${colors.text}
        ${className}
        rounded-full
        flex
        items-center
        justify-center
        font-semibold
        flex-shrink-0
        select-none
      `}
      aria-label={`${firstName} ${lastName}`}
      title={`${firstName} ${lastName}`}
    >
      {initials}
    </div>
  )
}

const MemoizedUserAvatar = memo(UserAvatar)
MemoizedUserAvatar.displayName = 'UserAvatar'

export { MemoizedUserAvatar as UserAvatar }
export default MemoizedUserAvatar
