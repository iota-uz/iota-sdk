/**
 * EditableText Component
 * Inline editable text with double-click to edit
 * Features: auto-focus, auto-select, Enter to save, Escape to cancel
 * Can be triggered programmatically via ref.startEditing()
 */

import { useState, useRef, useEffect, useImperativeHandle, forwardRef, memo } from 'react'
import { CircleNotch } from '@phosphor-icons/react'

export interface EditableTextProps {
  /** Current text value */
  value: string
  /** Callback when text is saved */
  onSave: (newValue: string) => void
  /** Maximum character length */
  maxLength?: number
  /** Whether the component is in loading state */
  isLoading?: boolean
  /** Placeholder text when empty */
  placeholder?: string
  /** Additional CSS classes for the text display */
  className?: string
  /** Additional CSS classes for the input */
  inputClassName?: string
  /** Font size variant */
  size?: 'sm' | 'md' | 'lg'
}

export interface EditableTextRef {
  /** Programmatically start editing mode */
  startEditing: () => void
  /** Programmatically cancel editing */
  cancelEditing: () => void
}

const sizeClasses = {
  sm: 'text-sm',
  md: 'text-base',
  lg: 'text-lg',
}

const EditableText = forwardRef<EditableTextRef, EditableTextProps>(
  (
    {
      value,
      onSave,
      maxLength = 100,
      isLoading = false,
      placeholder = 'Untitled',
      className = '',
      inputClassName = '',
      size = 'sm',
    },
    ref
  ) => {
    const [isEditing, setIsEditing] = useState(false)
    const [editValue, setEditValue] = useState(value)
    const inputRef = useRef<HTMLInputElement>(null)

    // Expose methods via ref
    useImperativeHandle(ref, () => ({
      startEditing: () => {
        if (!isLoading) {
          setIsEditing(true)
        }
      },
      cancelEditing: () => {
        setEditValue(value)
        setIsEditing(false)
      },
    }))

    // Update edit value when value prop changes
    useEffect(() => {
      setEditValue(value)
    }, [value])

    // Auto-focus and select when entering edit mode
    useEffect(() => {
      if (isEditing && inputRef.current) {
        inputRef.current.focus()
        inputRef.current.select()
      }
    }, [isEditing])

    const handleSave = () => {
      const trimmed = editValue.trim()

      // Don't save if empty - revert to original
      if (!trimmed) {
        setEditValue(value)
        setIsEditing(false)
        return
      }

      // Only call onSave if value actually changed
      if (trimmed !== value) {
        onSave(trimmed)
      }

      setIsEditing(false)
    }

    const handleCancel = () => {
      setEditValue(value)
      setIsEditing(false)
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
      if (e.key === 'Enter') {
        e.preventDefault()
        handleSave()
      } else if (e.key === 'Escape') {
        e.preventDefault()
        handleCancel()
      }
    }

    const handleDoubleClick = () => {
      if (!isLoading) {
        setIsEditing(true)
      }
    }

    const handleBlur = () => {
      handleSave()
    }

    const sizeClass = sizeClasses[size]

    if (isEditing) {
      return (
        <div
          className="flex items-center gap-2 flex-1"
          onClick={(e) => e.preventDefault()}
        >
          <input
            ref={inputRef}
            type="text"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleBlur}
            maxLength={maxLength}
            placeholder={placeholder}
            className={`flex-1 px-2 py-1 ${sizeClass} bg-white dark:bg-gray-700 border border-primary-500 dark:border-primary-600 rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 dark:focus-visible:ring-primary-600/30 text-gray-900 dark:text-white ${inputClassName}`}
            aria-label="Edit text. Press Enter to save, Escape to cancel"
          />
        </div>
      )
    }

    const displayValue = value || placeholder

    return (
      <span
        onDoubleClick={handleDoubleClick}
        className={`${sizeClass} font-medium truncate flex-1 cursor-pointer select-none hover:text-primary-600 dark:hover:text-primary-400 transition-colors ${className}`}
        title="Double-click to edit"
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            handleDoubleClick()
          }
        }}
      >
        {isLoading ? (
          <span className="inline-flex items-center gap-2 text-gray-400 dark:text-gray-500">
            <CircleNotch size={12} className="animate-spin" />
            <span className="italic">{displayValue}</span>
          </span>
        ) : (
          <span className={!value ? 'text-gray-400 dark:text-gray-500 italic' : ''}>
            {displayValue}
          </span>
        )}
      </span>
    )
  }
)

EditableText.displayName = 'EditableText'

const MemoizedEditableText = memo(EditableText)

export { MemoizedEditableText as EditableText }
export default MemoizedEditableText
