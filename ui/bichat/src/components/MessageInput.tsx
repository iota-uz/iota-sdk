/**
 * MessageInput Component
 * Advanced input with file upload, drag-drop, keyboard shortcuts, and message queuing
 * Clean, professional design
 */

import { useState, useRef, useEffect, forwardRef, useImperativeHandle } from 'react'
import { Paperclip, PaperPlaneRight, X } from '@phosphor-icons/react'
import AttachmentGrid from './AttachmentGrid'
import { validateImageFile, validateFileCount, convertToBase64, createDataUrl } from '../utils/fileUtils'
import type { ImageAttachment, QueuedMessage } from '../types'

export interface MessageInputRef {
  focus: () => void
  clear: () => void
}

export interface MessageInputProps {
  message: string
  loading: boolean
  fetching?: boolean
  disabled?: boolean
  messageQueue?: QueuedMessage[]
  onMessageChange: (value: string) => void
  onSubmit: (e: React.FormEvent, attachments: ImageAttachment[]) => void
  onUnqueue?: () => { content: string; attachments: ImageAttachment[] } | null
  placeholder?: string
  maxFiles?: number
  maxFileSize?: number
  containerClassName?: string
}

const MAX_FILES_DEFAULT = 10
const MAX_FILE_SIZE_DEFAULT = 20 * 1024 * 1024 // 20MB
const MAX_HEIGHT = 192 // 12 lines approx

export const MessageInput = forwardRef<MessageInputRef, MessageInputProps>(
  (
    {
      message,
      loading,
      fetching = false,
      disabled = false,
      messageQueue = [],
      onMessageChange,
      onSubmit,
      onUnqueue,
      placeholder = 'Type a message...',
      maxFiles = MAX_FILES_DEFAULT,
      maxFileSize = MAX_FILE_SIZE_DEFAULT,
      containerClassName
    },
    ref
  ) => {
    const [attachments, setAttachments] = useState<ImageAttachment[]>([])
    const [isDragging, setIsDragging] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [isFocused, setIsFocused] = useState(false)

    const textareaRef = useRef<HTMLTextAreaElement>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const containerRef = useRef<HTMLDivElement>(null)

    useImperativeHandle(ref, () => ({
      focus: () => textareaRef.current?.focus(),
      clear: () => {
        onMessageChange('')
        setAttachments([])
        setError(null)
      }
    }))

    useEffect(() => {
      const textarea = textareaRef.current
      if (!textarea) return

      textarea.style.height = 'auto'
      const newHeight = Math.min(textarea.scrollHeight, MAX_HEIGHT)
      textarea.style.height = `${newHeight}px`
    }, [message])

    useEffect(() => {
      if (error) {
        const timer = setTimeout(() => setError(null), 5000)
        return () => clearTimeout(timer)
      }
    }, [error])

    const handleFileSelect = async (files: FileList | null) => {
      if (!files || files.length === 0) return

      try {
        validateFileCount(attachments.length, files.length, maxFiles)

        const newAttachments: ImageAttachment[] = []

        for (let i = 0; i < files.length; i++) {
          const file = files[i]
          validateImageFile(file, maxFileSize)
          const base64Data = await convertToBase64(file)
          const preview = createDataUrl(base64Data, file.type)

          newAttachments.push({
            filename: file.name,
            mimeType: file.type,
            sizeBytes: file.size,
            base64Data,
            preview
          })
        }

        setAttachments((prev) => [...prev, ...newAttachments])
        setError(null)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to process files')
      }
    }

    const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      handleFileSelect(e.target.files)
      e.target.value = ''
    }

    const handleRemoveAttachment = (index: number) => {
      setAttachments((prev) => prev.filter((_, i) => i !== index))
      setError(null)
    }

    const handleDragOver = (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setIsDragging(true)
    }

    const handleDragLeave = (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setIsDragging(false)
    }

    const handleDrop = async (e: React.DragEvent) => {
      e.preventDefault()
      e.stopPropagation()
      setIsDragging(false)
      await handleFileSelect(e.dataTransfer.files)
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        if (!loading && (message.trim() || attachments.length > 0)) {
          handleFormSubmit(e as unknown as React.FormEvent)
        }
      }

      if (e.key === 'Escape') {
        onMessageChange('')
        setAttachments([])
        setError(null)
      }

      if (e.key === 'ArrowUp' && !message.trim() && onUnqueue) {
        const unqueued = onUnqueue()
        if (unqueued) {
          onMessageChange(unqueued.content)
          setAttachments(unqueued.attachments)
        }
      }
    }

    const handleFormSubmit = (e: React.FormEvent) => {
      e.preventDefault()
      if (loading || disabled || (!message.trim() && attachments.length === 0)) {
        return
      }

      onSubmit(e, attachments)
      setAttachments([])
      setError(null)
    }

    const canSubmit = !loading && !disabled && (message.trim() || attachments.length > 0)
    const defaultContainerClassName = "sticky bottom-0 p-4 pb-6"

    return (
      <div
        ref={containerRef}
        className={containerClassName ?? defaultContainerClassName}
      >
        <form onSubmit={handleFormSubmit} className="max-w-4xl mx-auto">
          {/* Error display */}
          {error && (
            <div className="mb-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-600 dark:text-red-400 flex items-center justify-between">
              <span>{error}</span>
              <button
                type="button"
                onClick={() => setError(null)}
                className="ml-2 p-1 hover:bg-red-100 dark:hover:bg-red-800 rounded transition-colors"
                aria-label="Dismiss error"
              >
                <X size={14} />
              </button>
            </div>
          )}

          {/* Queue badge */}
          {messageQueue.length > 0 && (
            <div className="mb-3 text-xs text-gray-500 dark:text-gray-400">
              <span className="px-2.5 py-1 bg-purple-50 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400 rounded font-medium">
                {messageQueue.length} message{messageQueue.length > 1 ? 's' : ''} queued
              </span>
            </div>
          )}

          {/* Attachment preview */}
          {attachments.length > 0 && (
            <div className="mb-3">
              <AttachmentGrid attachments={attachments} onRemove={handleRemoveAttachment} />
            </div>
          )}

          {/* Input container with drag-drop */}
          <div
            className="relative"
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            {/* Drag overlay */}
            {isDragging && (
              <div className="absolute inset-0 z-10 bg-purple-50/95 dark:bg-purple-900/90 border-2 border-dashed border-purple-400 rounded-2xl flex items-center justify-center">
                <div className="flex flex-col items-center gap-2">
                  <div className="w-10 h-10 rounded-full bg-purple-100 dark:bg-purple-800 flex items-center justify-center">
                    <Paperclip size={20} className="text-purple-600 dark:text-purple-400" />
                  </div>
                  <span className="text-sm text-purple-700 dark:text-purple-300 font-medium">
                    Drop images here
                  </span>
                </div>
              </div>
            )}

            {/* Input container - using inline Tailwind classes */}
            <div
              className={`flex items-end gap-2 rounded-2xl p-2.5 bg-white dark:bg-gray-800 border shadow-sm transition-all duration-150 ${
                isFocused
                  ? 'border-purple-400 dark:border-purple-500 ring-3 ring-purple-500/10 dark:ring-purple-500/15'
                  : 'border-gray-200 dark:border-gray-700'
              }`}
            >
              {/* Attach button */}
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={loading || disabled || attachments.length >= maxFiles}
                className="flex-shrink-0 p-2 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                aria-label="Attach files"
                title="Attach images"
              >
                <Paperclip size={18} />
              </button>

              {/* Hidden file input */}
              <input
                ref={fileInputRef}
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif"
                multiple
                onChange={handleFileInputChange}
                className="hidden"
                aria-label="File input"
              />

              {/* Textarea */}
              <div className="flex-1">
                <textarea
                  ref={textareaRef}
                  value={message}
                  onChange={(e) => onMessageChange(e.target.value)}
                  onKeyDown={handleKeyDown}
                  onFocus={() => setIsFocused(true)}
                  onBlur={() => setIsFocused(false)}
                  placeholder={placeholder}
                  className="resize-none bg-transparent border-none outline-none px-1 py-2 w-full text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-[15px] leading-relaxed"
                  style={{ maxHeight: `${MAX_HEIGHT}px` }}
                  rows={1}
                  disabled={loading || disabled}
                  aria-label="Message input"
                />
              </div>

              {/* Send button - using inline Tailwind classes */}
              <button
                type="submit"
                disabled={!canSubmit}
                className="flex-shrink-0 p-2 rounded-lg bg-purple-600 hover:bg-purple-700 active:bg-purple-800 text-white shadow-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-purple-600"
                aria-label="Send message"
              >
                {loading ? (
                  <div className="w-[18px] h-[18px] border-2 border-white/60 border-t-transparent rounded-full animate-spin" />
                ) : (
                  <PaperPlaneRight size={18} weight="fill" />
                )}
              </button>
            </div>
          </div>

          {/* Loading indicator */}
          {(loading || fetching) && (
            <div className="mt-3 flex items-center justify-center gap-2">
              <div className="flex items-center gap-1.5">
                <div className="w-1.5 h-1.5 bg-gray-400 dark:bg-gray-500 rounded-full animate-pulse" />
                <div className="w-1.5 h-1.5 bg-gray-400 dark:bg-gray-500 rounded-full animate-pulse" style={{ animationDelay: '0.15s' }} />
                <div className="w-1.5 h-1.5 bg-gray-400 dark:bg-gray-500 rounded-full animate-pulse" style={{ animationDelay: '0.3s' }} />
              </div>
              <span className="text-sm text-gray-500 dark:text-gray-400">
                {loading ? 'AI is thinking...' : 'Processing...'}
              </span>
            </div>
          )}
        </form>
      </div>
    )
  }
)

MessageInput.displayName = 'MessageInput'
