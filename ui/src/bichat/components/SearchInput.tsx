/**
 * SearchInput Component
 * Reusable search input with icon, clear button, and keyboard shortcuts
 */

import { useEffect, useRef, memo, type KeyboardEvent } from 'react'
import { MagnifyingGlass, X } from '@phosphor-icons/react'

export interface SearchInputProps {
  /** Current search value */
  value: string
  /** Callback when value changes */
  onChange: (value: string) => void
  /** Placeholder text */
  placeholder?: string
  /** Auto-focus on mount */
  autoFocus?: boolean
  /** Callback when Enter is pressed */
  onSubmit?: (value: string) => void
  /** Callback when Escape is pressed */
  onEscape?: () => void
  /** Additional CSS classes for the container */
  className?: string
  /** Size variant */
  size?: 'sm' | 'md' | 'lg'
  /** Disable the input */
  disabled?: boolean
  /** ARIA label for accessibility */
  ariaLabel?: string
}

const sizeClasses = {
  sm: {
    container: 'py-1.5 pl-8 pr-8 text-xs',
    icon: 14,
    clearBtn: 'p-1',
  },
  md: {
    container: 'py-2.5 pl-10 pr-10 text-sm',
    icon: 16,
    clearBtn: 'p-1.5',
  },
  lg: {
    container: 'py-3 pl-12 pr-12 text-base',
    icon: 18,
    clearBtn: 'p-2',
  },
}

function SearchInput({
  value,
  onChange,
  placeholder = 'Search...',
  autoFocus = false,
  onSubmit,
  onEscape,
  className = '',
  size = 'md',
  disabled = false,
  ariaLabel = 'Search',
}: SearchInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const sizes = sizeClasses[size]

  useEffect(() => {
    if (autoFocus && inputRef.current) {
      inputRef.current.focus()
    }
  }, [autoFocus])

  const handleClear = () => {
    onChange('')
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && onSubmit) {
      e.preventDefault()
      onSubmit(value)
    } else if (e.key === 'Escape') {
      e.preventDefault()
      if (value && !onEscape) {
        // Default behavior: clear on Escape if no handler provided
        handleClear()
      } else if (onEscape) {
        onEscape()
      }
    }
  }

  return (
    <div className={`relative w-full ${className}`} role="search">
      {/* Search Icon */}
      <span className="absolute inset-y-0 left-3 flex items-center pointer-events-none">
        <MagnifyingGlass
          size={sizes.icon}
          weight="bold"
          className="text-gray-400 dark:text-gray-500"
          aria-hidden="true"
        />
      </span>

      {/* Input Field */}
      <input
        ref={inputRef}
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        className={`w-full ${sizes.container} bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:focus:ring-primary-500/20 focus:border-primary-400 dark:focus:border-primary-600 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed`}
        aria-label={ariaLabel}
      />

      {/* Clear Button */}
      {value && !disabled && (
        <button
          type="button"
          onClick={handleClear}
          className={`absolute inset-y-0 right-2 flex items-center ${sizes.clearBtn} rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-all duration-200 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300`}
          aria-label="Clear search"
          title="Clear search"
        >
          <X size={sizes.icon - 2} weight="bold" />
        </button>
      )}
    </div>
  )
}

const MemoizedSearchInput = memo(SearchInput)
MemoizedSearchInput.displayName = 'SearchInput'

export { MemoizedSearchInput as SearchInput }
export default MemoizedSearchInput
