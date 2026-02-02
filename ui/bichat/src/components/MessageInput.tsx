/**
 * MessageInput Component - Complete Rewrite
 * Advanced input with file upload, drag-drop, keyboard shortcuts, and message queuing
 * Desktop-only version (no mobile/touch optimizations)
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
  /** Override container classes (default: sticky bottom with border) */
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

    const textareaRef = useRef<HTMLTextAreaElement>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const containerRef = useRef<HTMLDivElement>(null)

    // Expose methods via ref
    useImperativeHandle(ref, () => ({
      focus: () => textareaRef.current?.focus(),
      clear: () => {
        onMessageChange('')
        setAttachments([])
        setError(null)
      }
    }))

    // Auto-resize textarea
    useEffect(() => {
      const textarea = textareaRef.current
      if (!textarea) return

      textarea.style.height = 'auto'
      const newHeight = Math.min(textarea.scrollHeight, MAX_HEIGHT)
      textarea.style.height = `${newHeight}px`
    }, [message])

    // Clear error after 5 seconds
    useEffect(() => {
      if (error) {
        const timer = setTimeout(() => setError(null), 5000)
        return () => clearTimeout(timer)
      }
    }, [error])

    const handleFileSelect = async (files: FileList | null) => {
      if (!files || files.length === 0) return

      try {
        // Validate count
        validateFileCount(attachments.length, files.length, maxFiles)

        const newAttachments: ImageAttachment[] = []

        for (let i = 0; i < files.length; i++) {
          const file = files[i]

          // Validate file
          validateImageFile(file, maxFileSize)

          // Convert to base64
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
      // Reset input so same file can be selected again
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

      const files = e.dataTransfer.files
      await handleFileSelect(files)
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      // Submit on Enter (without Shift)
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        if (!loading && (message.trim() || attachments.length > 0)) {
          handleFormSubmit(e as unknown as React.FormEvent)
        }
      }

      // Clear on Escape
      if (e.key === 'Escape') {
        onMessageChange('')
        setAttachments([])
        setError(null)
      }

      // Unqueue on Arrow Up (if input is empty and unqueue function exists)
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
      // Clear attachments after submit
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
            <div className="mb-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-xl text-sm text-red-600 dark:text-red-400 flex items-center justify-between shadow-sm">
              <span>{error}</span>
              <button
                type="button"
                onClick={() => setError(null)}
                className="ml-2 p-1.5 hover:bg-red-100 dark:hover:bg-red-800 rounded-lg transition-colors"
                aria-label="Dismiss error"
              >
                <X size={14} />
              </button>
            </div>
          )}

          {/* Queue badge */}
          {messageQueue.length > 0 && (
            <div className="mb-3 text-xs text-gray-500 dark:text-gray-400 flex items-center gap-2">
              <span className="px-3 py-1.5 bg-primary-50 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 rounded-full font-medium border border-primary-200 dark:border-primary-800">
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

          {/* Input container with drag-drop overlay */}
          <div
            className="relative"
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            {/* Drag overlay */}
            {isDragging && (
              <div className="absolute inset-0 z-10 bg-primary-50/95 dark:bg-primary-900/95 border-2 border-dashed border-primary-400 dark:border-primary-500 rounded-3xl flex items-center justify-center backdrop-blur-sm">
                <div className="flex flex-col items-center gap-2">
                  <div className="w-12 h-12 rounded-full bg-primary-100 dark:bg-primary-800 flex items-center justify-center">
                    <Paperclip size={24} weight="duotone" className="text-primary-600 dark:text-primary-400" />
                  </div>
                  <span className="text-primary-700 dark:text-primary-300 font-medium">
                    Drop images here
                  </span>
                </div>
              </div>
            )}

            {/* Premium input container */}
            <div className="input-premium flex items-end gap-2 rounded-3xl p-3">
              {/* Attach button */}
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={loading || disabled || attachments.length >= maxFiles}
                className="flex-shrink-0 p-2.5 text-gray-500 dark:text-gray-400 hover:text-primary-600 dark:hover:text-primary-400 hover:bg-gray-100 dark:hover:bg-gray-700/50 rounded-xl transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed"
                aria-label="Attach files"
                title="Attach images"
              >
                <Paperclip size={20} weight="duotone" />
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
              <div className="flex-1 relative">
                <textarea
                  ref={textareaRef}
                  value={message}
                  onChange={(e) => onMessageChange(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder={placeholder}
                  className="flex-1 resize-none bg-transparent border-none outline-none px-2 py-2.5 w-full text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-[15px] leading-relaxed"
                  style={{ maxHeight: `${MAX_HEIGHT}px` }}
                  rows={1}
                  disabled={loading || disabled}
                  aria-label="Message input"
                />
              </div>

              {/* Send button - premium gradient */}
              <button
                type="submit"
                disabled={!canSubmit}
                className="btn-primary flex-shrink-0 p-2.5 rounded-xl disabled:opacity-40 disabled:cursor-not-allowed disabled:transform-none"
                aria-label="Send message"
              >
                {loading ? (
                  <div className="w-5 h-5 border-2 border-white/80 border-t-transparent rounded-full animate-spin" />
                ) : (
                  <PaperPlaneRight size={20} weight="fill" />
                )}
              </button>
            </div>
          </div>

          {/* Loading indicator - refined */}
          {(loading || fetching) && (
            <div className="mt-4 flex items-center justify-center gap-3">
              <div className="flex items-center gap-2 px-4 py-2 bg-gray-100 dark:bg-gray-800 rounded-full">
                <div className="w-2 h-2 bg-primary-500 rounded-full animate-pulse" />
                <div className="w-2 h-2 bg-primary-500 rounded-full animate-pulse" style={{ animationDelay: '0.2s' }} />
                <div className="w-2 h-2 bg-primary-500 rounded-full animate-pulse" style={{ animationDelay: '0.4s' }} />
              </div>
              <span className="text-sm text-gray-500 dark:text-gray-400 font-medium">
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
