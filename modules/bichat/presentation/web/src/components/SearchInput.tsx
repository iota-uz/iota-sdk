import { useEffect, useRef } from 'react'
import { MagnifyingGlass, X } from '@phosphor-icons/react'

interface SearchInputProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
  autoFocus?: boolean
}

export default function SearchInput({
  value,
  onChange,
  placeholder = 'Search chats...',
  autoFocus = false,
}: SearchInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (autoFocus && inputRef.current) {
      inputRef.current.focus()
    }
  }, [autoFocus])

  const handleClear = () => {
    onChange('')
    inputRef.current?.focus()
  }

  return (
    <div role="search">
      <div className="relative w-full">
        {/* Search Icon */}
        <span className="absolute inset-y-0 left-3 flex items-center pointer-events-none">
          <MagnifyingGlass size={16} weight="bold" className="text-gray-400 dark:text-gray-500" aria-hidden="true" />
        </span>

        {/* Input Field - refined styling */}
        <input
          ref={inputRef}
          type="search"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="w-full pl-10 pr-10 py-2.5 text-sm bg-gray-50 dark:bg-gray-800/50 border border-gray-200 dark:border-gray-700/50 rounded-xl focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:focus:ring-primary-500/20 focus:border-primary-400 dark:focus:border-primary-600 text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 transition-all duration-200"
          aria-label="Search chat sessions"
        />

        {/* Clear Button */}
        {value && (
          <button
            onClick={handleClear}
            className="absolute inset-y-0 right-2 flex items-center p-1.5 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-700 transition-all duration-200 text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300"
            aria-label="Clear search"
            title="Clear search"
          >
            <X size={14} weight="bold" />
          </button>
        )}
      </div>
    </div>
  )
}
