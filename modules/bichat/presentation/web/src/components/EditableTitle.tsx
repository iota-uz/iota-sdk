import { useState, useRef, useEffect, useImperativeHandle, forwardRef } from 'react'
import { CircleNotch } from '@phosphor-icons/react'

interface EditableTitleProps {
  title: string
  onSave: (newTitle: string) => void
  maxLength?: number
  isLoading?: boolean
}

export interface EditableTitleRef {
  startEditing: () => void
}

/**
 * Editable title component with double-click to edit
 * Features: auto-focus, auto-select, Enter to save, Escape to cancel
 * Can be triggered programmatically via ref.startEditing()
 */
const EditableTitle = forwardRef<EditableTitleRef, EditableTitleProps>(({
  title,
  onSave,
  maxLength = 60,
  isLoading = false,
}, ref) => {
  const [isEditing, setIsEditing] = useState(false)
  const [editValue, setEditValue] = useState(title)
  const inputRef = useRef<HTMLInputElement>(null)

  // Expose startEditing method via ref
  useImperativeHandle(ref, () => ({
    startEditing: () => {
      if (!isLoading) {
        setIsEditing(true)
      }
    },
  }))

  // Update edit value when title prop changes
  useEffect(() => {
    setEditValue(title)
  }, [title])

  // Auto-focus and select when entering edit mode
  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus()
      inputRef.current.select()
    }
  }, [isEditing])

  const handleSave = () => {
    const trimmed = editValue.trim()

    // Don't save if empty or unchanged
    if (!trimmed) {
      setEditValue(title)
      setIsEditing(false)
      return
    }

    if (trimmed !== title) {
      onSave(trimmed)
    }

    setIsEditing(false)
  }

  const handleCancel = () => {
    setEditValue(title)
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
    // Only save on blur if we have changes
    handleSave()
  }

  if (isEditing) {
    return (
      <div className="flex items-center gap-2 flex-1" onClick={(e) => e.preventDefault()}>
        <input
          ref={inputRef}
          type="text"
          value={editValue}
          onChange={(e) => setEditValue(e.target.value)}
          onKeyDown={handleKeyDown}
          onBlur={handleBlur}
          maxLength={maxLength}
          className="flex-1 px-2 py-1 text-sm bg-white dark:bg-gray-700 border border-primary-500 dark:border-primary-600 rounded focus:outline-none focus:ring-2 focus:ring-primary-500 dark:focus:ring-primary-600 text-gray-900 dark:text-white"
          aria-label="Edit chat title. Press Enter to save, Escape to cancel"
        />
      </div>
    )
  }

  return (
    <span
      onDoubleClick={handleDoubleClick}
      className="text-sm font-medium truncate flex-1 cursor-pointer select-none"
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
        <span className="inline-flex items-center gap-2">
          <CircleNotch size={12} className="animate-spin" />
          {title}
        </span>
      ) : (
        title
      )}
    </span>
  )
})

EditableTitle.displayName = 'EditableTitle'

export default EditableTitle
