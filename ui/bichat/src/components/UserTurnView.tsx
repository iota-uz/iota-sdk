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
      <div className="flex-1 flex flex-col items-end max-w-[75%]">
        {/* Attachments */}
        {message.attachments && message.attachments.length > 0 && (
          <div className="mb-2 w-full">
            <AttachmentGrid
              attachments={message.attachments}
              onView={(index) => setSelectedImageIndex(index)}
            />
          </div>
        )}

        {/* Message bubble - premium gradient with inner highlight */}
        {message.content && (
          <div className="bubble-user rounded-2xl rounded-br-md px-5 py-3">
            <div className="text-[15px] whitespace-pre-wrap break-words leading-relaxed relative z-10">
              {message.content}
            </div>
          </div>
        )}

        {/* Actions (visible on hover) - refined */}
        <div className="flex items-center gap-1.5 mt-2 px-1 opacity-0 group-hover:opacity-100 transition-all duration-200">
          <span className="text-xs text-gray-400 dark:text-gray-500 mr-1">
            {formatDistanceToNow(new Date(message.createdAt), { addSuffix: true })}
          </span>

          {/* Copy button */}
          <button
            onClick={handleCopyClick}
            className="p-1.5 text-gray-400 dark:text-gray-500 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-all duration-200"
            aria-label="Copy message"
            title="Copy"
          >
            <Copy size={14} weight="bold" />
          </button>

          {/* Edit button */}
          {handleEdit && (
            <button
              onClick={handleEditClick}
              className="p-1.5 text-gray-400 dark:text-gray-500 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-all duration-200"
              aria-label="Edit message"
              title="Edit"
            >
              <PencilSimple size={14} weight="bold" />
            </button>
          )}
        </div>
      </div>

      {/* Avatar - premium gradient */}
      <div className="avatar-primary flex-shrink-0 w-8 h-8 rounded-full flex items-center justify-center text-white font-semibold text-sm">
        <span className="relative z-10">U</span>
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
