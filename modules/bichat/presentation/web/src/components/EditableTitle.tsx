/**
 * EditableTitle Component
 * Wrapper around EditableText from @iota-uz/bichat-ui for backward compatibility
 */
import { forwardRef, memo } from 'react'
import { EditableText, type EditableTextRef } from '@iota-uz/bichat-ui'

interface EditableTitleProps {
  title: string
  onSave: (newTitle: string) => void
  maxLength?: number
  isLoading?: boolean
}

export interface EditableTitleRef {
  startEditing: () => void
}

const EditableTitle = forwardRef<EditableTitleRef, EditableTitleProps>(
  ({ title, onSave, maxLength = 60, isLoading = false }, ref) => {
    return (
      <EditableText
        ref={ref as React.Ref<EditableTextRef>}
        value={title}
        onSave={onSave}
        maxLength={maxLength}
        isLoading={isLoading}
        placeholder="Untitled Chat"
        size="sm"
      />
    )
  }
)

EditableTitle.displayName = 'EditableTitle'

export default memo(EditableTitle)
