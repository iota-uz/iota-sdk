/**
 * UserTurnView Component
 * Displays user messages with attachments, image modal, and actions
 */

import { useState } from 'react'
import { Copy, PencilSimple } from '@phosphor-icons/react'
import { formatDistanceToNow } from 'date-fns'
import AttachmentGrid from './AttachmentGrid'
import ImageModal from './ImageModal'
import { useChat } from '../context/ChatContext'
import type { Message, ImageAttachment } from '../types'

interface UserTurnViewProps {
  message: Message & {
    attachments?: ImageAttachment[]
  }
}

export function UserTurnView({ message }: UserTurnViewProps) {
  const { handleEdit, handleCopy } = useChat()
  const [selectedImageIndex, setSelectedImageIndex] = useState<number | null>(null)

  const handleCopyClick = async () => {
    if (handleCopy) {
      await handleCopy(message.content)
    } else {
      // Fallback to clipboard API
      try {
        await navigator.clipboard.writeText(message.content)
      } catch (err) {
        console.error('Failed to copy:', err)
      }
    }
  }

  const handleEditClick = () => {
    if (handleEdit) {
      const newContent = prompt('Edit message:', message.content)
      if (newContent && newContent !== message.content) {
        handleEdit(message.id, newContent)
      }
    }
  }

  return (
    <div className="flex gap-3 justify-end group">
      <div className="flex-1 flex flex-col items-end max-w-[70%]">
        {/* Attachments */}
        {message.attachments && message.attachments.length > 0 && (
          <div className="mb-2 w-full">
            <AttachmentGrid
              attachments={message.attachments}
              onView={(index) => setSelectedImageIndex(index)}
            />
          </div>
        )}

        {/* Message bubble */}
        {message.content && (
          <div className="rounded-2xl px-5 py-3 bg-primary-600 dark:bg-primary-700 text-white shadow-sm">
            <div className="text-base whitespace-pre-wrap break-words">{message.content}</div>
          </div>
        )}

        {/* Actions (visible on hover) */}
        <div className="flex items-center gap-2 mt-1 px-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <span className="text-xs text-gray-500 dark:text-gray-400">
            {formatDistanceToNow(new Date(message.createdAt), { addSuffix: true })}
          </span>

          {/* Copy button */}
          <button
            onClick={handleCopyClick}
            className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
            aria-label="Copy message"
            title="Copy"
          >
            <Copy size={14} />
          </button>

          {/* Edit button */}
          {handleEdit && (
            <button
              onClick={handleEditClick}
              className="p-1 text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 rounded transition-colors"
              aria-label="Edit message"
              title="Edit"
            >
              <PencilSimple size={14} />
            </button>
          )}
        </div>
      </div>

      {/* Avatar */}
      <div className="flex-shrink-0 w-8 h-8 rounded-full bg-primary-500 dark:bg-primary-600 flex items-center justify-center text-white font-semibold text-sm shadow-sm">
        U
      </div>

      {/* Image modal */}
      {selectedImageIndex !== null && message.attachments && (
        <ImageModal
          images={message.attachments}
          initialIndex={selectedImageIndex}
          onClose={() => setSelectedImageIndex(null)}
        />
      )}
    </div>
  )
}
