/**
 * MessageInput Component
 * Advanced input with file upload, drag-drop, keyboard shortcuts, and message queuing
 * Clean, professional design
 */

import { useState, useRef, useEffect, forwardRef, useImperativeHandle, useMemo } from 'react'
import { Paperclip, PaperPlaneRight, X, Bug } from '@phosphor-icons/react'
import AttachmentGrid from './AttachmentGrid'
import { validateImageFile, validateFileCount, convertToBase64, createDataUrl } from '../utils/fileUtils'
import type { ImageAttachment, QueuedMessage, SessionDebugUsage } from '../types'
import { useTranslation } from '../hooks/useTranslation'

export interface MessageInputRef {
  focus: () => void
  clear: () => void
}

export interface MessageInputProps {
  message: string
  loading: boolean
  fetching?: boolean
  disabled?: boolean
  commandError?: string | null
  debugMode?: boolean
  debugSessionUsage?: SessionDebugUsage
  modelContextWindow?: number | null
  messageQueue?: QueuedMessage[]
  onClearCommandError?: () => void
  onMessageChange: (value: string) => void
  onSubmit: (e: React.FormEvent, attachments: ImageAttachment[]) => void
  onUnqueue?: () => { content: string; attachments: ImageAttachment[] } | null
  placeholder?: string
  maxFiles?: number
  maxFileSize?: number
  containerClassName?: string
  formClassName?: string
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
      commandError = null,
      debugMode = false,
      debugSessionUsage,
      modelContextWindow = null,
      messageQueue = [],
      onClearCommandError,
      onMessageChange,
      onSubmit,
      onUnqueue,
      placeholder: placeholderOverride,
      maxFiles = MAX_FILES_DEFAULT,
      maxFileSize = MAX_FILE_SIZE_DEFAULT,
      containerClassName,
      formClassName,
    },
    ref
  ) => {
    const { t } = useTranslation()
    const [attachments, setAttachments] = useState<ImageAttachment[]>([])
    const [isDragging, setIsDragging] = useState(false)
    const [error, setError] = useState<string | null>(null)
    const [isFocused, setIsFocused] = useState(false)
    const [commandListDismissed, setCommandListDismissed] = useState(false)
    const [activeCommandIndex, setActiveCommandIndex] = useState(0)
    const [isComposing, setIsComposing] = useState(false)
    const [dropSuccess, setDropSuccess] = useState(false)

    // Use override or translation
    const placeholder = placeholderOverride || t('input.placeholder')

    const textareaRef = useRef<HTMLTextAreaElement>(null)
    const fileInputRef = useRef<HTMLInputElement>(null)
    const containerRef = useRef<HTMLDivElement>(null)
    const formRef = useRef<HTMLFormElement>(null)
    const commandItemRefs = useRef<Array<HTMLLIElement | null>>([])
    const didAutoFocusRef = useRef(false)
    const isSlashMode = message.trimStart().startsWith('/')
    const commandQuery = message.trimStart().slice(1).split(/\s+/)[0]?.toLowerCase() || ''

    const slashCommands = useMemo(
      () => [
        { name: '/clear', description: t('slash.clearDescription') },
        { name: '/debug', description: t('slash.debugDescription') },
        { name: '/compact', description: t('slash.compactDescription') },
      ],
      [t]
    )
    const filteredCommands = useMemo(
      () =>
        slashCommands.filter((cmd) =>
          cmd.name.slice(1).startsWith(commandQuery)
        ),
      [commandQuery, slashCommands]
    )
    const isCommandListVisible = isSlashMode && !commandListDismissed && !loading && !disabled

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
      if (didAutoFocusRef.current || loading || disabled || fetching) return

      const frame = requestAnimationFrame(() => {
        textareaRef.current?.focus()
        didAutoFocusRef.current = true
      })

      return () => cancelAnimationFrame(frame)
    }, [loading, disabled, fetching])

    useEffect(() => {
      if (!error) return
      const timer = setTimeout(() => setError(null), 5000)
      return () => clearTimeout(timer)
    }, [error])

    useEffect(() => {
      if (isSlashMode) {
        setCommandListDismissed(false)
      }
    }, [isSlashMode, message])

    useEffect(() => {
      if (!isCommandListVisible) return

      const handleOutsideClick = (event: MouseEvent) => {
        if (!containerRef.current) return
        if (event.target instanceof Node && !containerRef.current.contains(event.target)) {
          setCommandListDismissed(true)
        }
      }

      document.addEventListener('mousedown', handleOutsideClick)
      return () => document.removeEventListener('mousedown', handleOutsideClick)
    }, [isCommandListVisible])

    useEffect(() => {
      setActiveCommandIndex(0)
    }, [commandQuery])

    useEffect(() => {
      if (filteredCommands.length === 0) {
        setActiveCommandIndex(0)
        return
      }

      setActiveCommandIndex((prev) => {
        if (prev < 0) return 0
        if (prev >= filteredCommands.length) return filteredCommands.length - 1
        return prev
      })
    }, [filteredCommands.length])

    useEffect(() => {
      if (!isCommandListVisible || filteredCommands.length === 0) return
      commandItemRefs.current[activeCommandIndex]?.scrollIntoView({
        block: 'nearest',
      })
    }, [activeCommandIndex, filteredCommands.length, isCommandListVisible])

    const handleFileSelect = async (files: FileList | null): Promise<boolean> => {
      if (!files || files.length === 0) return false

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
        return true
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to process files')
        return false
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
      const ok = await handleFileSelect(e.dataTransfer.files)
      if (ok) {
        setDropSuccess(true)
        setTimeout(() => setDropSuccess(false), 1500)
      }
    }

    const submitCommandSelection = (command: string) => {
      onMessageChange(command)
      setCommandListDismissed(true)
      setActiveCommandIndex(0)
      requestAnimationFrame(() => {
        formRef.current?.requestSubmit()
      })
    }

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (isComposing || e.nativeEvent.isComposing) {
        return
      }

      if (isCommandListVisible) {
        if (e.key === 'ArrowDown' || (e.key === 'Tab' && !e.shiftKey)) {
          e.preventDefault()
          if (filteredCommands.length > 0) {
            setActiveCommandIndex((prev) => (prev + 1) % filteredCommands.length)
          }
          return
        }

        if (e.key === 'ArrowUp' || (e.key === 'Tab' && e.shiftKey)) {
          e.preventDefault()
          if (filteredCommands.length > 0) {
            setActiveCommandIndex((prev) =>
              prev === 0 ? filteredCommands.length - 1 : prev - 1
            )
          }
          return
        }

        if (e.key === 'Escape') {
          e.preventDefault()
          setCommandListDismissed(true)
          return
        }

        if (e.key === 'Enter' && !e.shiftKey) {
          e.preventDefault()
          if (filteredCommands.length > 0) {
            submitCommandSelection(filteredCommands[activeCommandIndex].name)
            return
          }
          handleFormSubmit(e as unknown as React.FormEvent)
          return
        }
      }

      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault()
        if (!loading && (message.trim() || attachments.length > 0)) {
          handleFormSubmit(e as unknown as React.FormEvent)
        }
      }

      if (e.key === 'Escape') {
        if (isSlashMode) {
          setCommandListDismissed(true)
        } else {
          onMessageChange('')
          setAttachments([])
          setError(null)
        }
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
      if (isComposing) return
      if (loading || disabled || (!message.trim() && attachments.length === 0)) {
        return
      }

      setCommandListDismissed(true)
      onSubmit(e, attachments)
      setAttachments([])
      setError(null)
    }

    const canSubmit = !loading && !disabled && (message.trim() || attachments.length > 0)
    const visibleError = error || commandError
    const visibleErrorText = visibleError ? t(visibleError) : ''
    const defaultContainerClassName = "shrink-0 px-4 pt-4 pb-6"
    const formatTokens = (value: number): string => new Intl.NumberFormat().format(value)
    const latestPromptTokens = debugSessionUsage?.latestPromptTokens ?? 0
    const sessionTotalTokens = debugSessionUsage?.totalTokens ?? 0
    const sessionPromptTokens = debugSessionUsage?.promptTokens ?? 0
    const sessionCompletionTokens = debugSessionUsage?.completionTokens ?? 0
    const hasUsage = (debugSessionUsage?.turnsWithUsage ?? 0) > 0
    const contextPercent =
      modelContextWindow && latestPromptTokens > 0
        ? ((latestPromptTokens / modelContextWindow) * 100).toFixed(1)
        : null

    return (
      <div
        ref={containerRef}
        className={containerClassName ?? defaultContainerClassName}
      >
        <form ref={formRef} onSubmit={handleFormSubmit} className={formClassName ?? "mx-auto"}>
          {/* Error display */}
          {visibleError && (
            <div className="mb-3 p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-sm text-red-600 dark:text-red-400 flex items-center justify-between">
              <span>{visibleErrorText}</span>
              <button
                type="button"
                onClick={() => {
                  setError(null)
                  onClearCommandError?.()
                }}
                className="ml-2 p-1 hover:bg-red-100 dark:hover:bg-red-800 rounded transition-colors"
                aria-label={t('input.dismissError')}
              >
                <X size={14} />
              </button>
            </div>
          )}

          {/* Queue badge */}
          {messageQueue.length > 0 && (
            <div className="mb-3 text-xs text-gray-500 dark:text-gray-400">
              <span className="px-2.5 py-1 bg-primary-50 dark:bg-primary-900/30 text-primary-600 dark:text-primary-400 rounded font-medium">
                {t('input.messagesQueued', { count: messageQueue.length })}
              </span>
            </div>
          )}

          {debugMode && (
            <div className="mb-3 space-y-2 text-xs">
              {/* Debug Mode Badge */}
              <span className="inline-flex items-center gap-1 px-2.5 py-1 bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-300 rounded font-medium">
                <Bug size={12} />
                {t('slash.debugBadge')}
              </span>

              {/* Stats Container */}
              <div className="rounded-lg border border-gray-200/80 dark:border-gray-700/60 bg-white dark:bg-gray-800/60 px-3 py-2 space-y-3">
                {hasUsage ? (
                  <>
                    {/* Token Columns */}
                    <div className="flex items-center justify-between text-[11px]">
                      <div className="flex flex-col gap-0.5">
                        <span className="text-gray-500 dark:text-gray-400">
                          {t('slash.debugPromptTokens')}
                        </span>
                        <span className="font-mono font-medium text-gray-900 dark:text-gray-100">
                          {formatTokens(sessionPromptTokens)}
                        </span>
                      </div>
                      <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" aria-hidden="true" />
                      <div className="flex flex-col gap-0.5">
                        <span className="text-gray-500 dark:text-gray-400">
                          {t('slash.debugCompletionTokens')}
                        </span>
                        <span className="font-mono font-medium text-gray-900 dark:text-gray-100">
                          {formatTokens(sessionCompletionTokens)}
                        </span>
                      </div>
                      <span className="w-0.5 h-0.5 rounded-full bg-gray-300 dark:bg-gray-600" aria-hidden="true" />
                      <div className="flex flex-col gap-0.5">
                        <span className="text-gray-500 dark:text-gray-400">
                          {t('slash.debugTotalTokens')}
                        </span>
                        <span className="font-mono font-medium text-gray-900 dark:text-gray-100">
                          {formatTokens(sessionTotalTokens)}
                        </span>
                      </div>
                    </div>

                    {/* Context Progress Bar */}
                    {modelContextWindow && (
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between text-[11px]">
                          <span className="text-gray-500 dark:text-gray-400">
                            {t('slash.debugContextUsage')}
                          </span>
                          <span className="font-mono text-gray-600 dark:text-gray-300">
                            {formatTokens(latestPromptTokens)} / {formatTokens(modelContextWindow)}
                          </span>
                          {contextPercent && (
                            <span className={`font-medium ${
                              parseFloat(contextPercent) > 75
                                ? 'text-red-600 dark:text-red-400'
                                : parseFloat(contextPercent) > 50
                                ? 'text-amber-600 dark:text-amber-400'
                                : 'text-emerald-600 dark:text-emerald-400'
                            }`}>
                              {contextPercent}%
                            </span>
                          )}
                        </div>
                        <div className="h-1 rounded-full bg-gray-100 dark:bg-gray-700/50 overflow-hidden">
                          <div
                            className={`h-full rounded-full transition-all duration-500 ease-out ${
                              parseFloat(contextPercent || '0') > 75
                                ? 'bg-red-500'
                                : parseFloat(contextPercent || '0') > 50
                                ? 'bg-amber-500'
                                : 'bg-emerald-500'
                            }`}
                            style={{
                              width: contextPercent ? `${Math.min(parseFloat(contextPercent), 100)}%` : '0%'
                            }}
                          />
                        </div>
                      </div>
                    )}
                  </>
                ) : (
                  <p className="text-[11px] text-gray-500 dark:text-gray-400">
                    {t('slash.debugSessionUsageUnavailable')}
                  </p>
                )}

                {/* Context Window fallback (when no usage data) */}
                {!hasUsage && modelContextWindow && (
                  <p className="text-[11px] text-gray-600 dark:text-gray-300">
                    {t('slash.debugContextWindow')}:{' '}
                    <span className="font-mono font-medium">{formatTokens(modelContextWindow)}</span>
                  </p>
                )}
              </div>
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
              <div className="absolute inset-0 z-10 bg-primary-50/95 dark:bg-primary-900/90 border-2 border-dashed border-primary-400 rounded-2xl flex items-center justify-center">
                <div className="flex flex-col items-center gap-2">
                  <div className="w-10 h-10 rounded-full bg-primary-100 dark:bg-primary-800 flex items-center justify-center">
                    <Paperclip size={20} className="text-primary-600 dark:text-primary-400" />
                  </div>
                  <span className="text-sm text-primary-700 dark:text-primary-300 font-medium">
                    {t('input.dropImages')}
                  </span>
                </div>
              </div>
            )}

            {/* Drop success feedback */}
            {dropSuccess && (
              <div className="absolute inset-0 z-10 bg-green-50/95 dark:bg-green-900/90 border-2 border-green-400 rounded-2xl flex items-center justify-center animate-pulse pointer-events-none">
                <span className="text-sm text-green-700 dark:text-green-300 font-medium">
                  {t('input.filesAdded')}
                </span>
              </div>
            )}

            {/* Input container - using inline Tailwind classes */}
            <div
              className={`flex items-center gap-2 rounded-2xl p-1.5 sm:p-2.5 bg-white dark:bg-gray-800 border shadow-sm transition-all duration-150 ${
                isFocused
                  ? 'border-primary-400 dark:border-primary-500 ring-2 ring-primary-500/25 dark:ring-primary-500/30'
                  : 'border-gray-200 dark:border-gray-700 hover:border-gray-300 dark:hover:border-gray-600'
              }`}
            >
              {/* Attach button */}
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={loading || disabled || attachments.length >= maxFiles}
                className="cursor-pointer flex-shrink-0 self-center p-2 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                aria-label={t('input.attachFiles')}
                title={t('input.attachImages')}
              >
                <Paperclip size={18} className="cursor-pointer" />
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
              <div className="flex-1 self-stretch flex items-center">
                <textarea
                  ref={textareaRef}
                  value={message}
                  onChange={(e) => {
                    onMessageChange(e.target.value)
                    onClearCommandError?.()
                  }}
                  onKeyDown={handleKeyDown}
                  onCompositionStart={() => setIsComposing(true)}
                  onCompositionEnd={() => setIsComposing(false)}
                  onFocus={() => setIsFocused(true)}
                  onBlur={(e) => {
                    setIsFocused(false)
                    if (!containerRef.current) return
                    if (!e.relatedTarget || !containerRef.current.contains(e.relatedTarget)) {
                      setCommandListDismissed(true)
                    }
                  }}
                  placeholder={placeholder}
                  className="resize-none bg-transparent border-none outline-none px-1 py-2 w-full text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 text-sm leading-relaxed"
                  style={{ maxHeight: `${MAX_HEIGHT}px` }}
                  rows={1}
                  disabled={loading || disabled}
                  aria-busy={loading}
                  aria-label="Message input"
                />
              </div>

              {/* Send button - using inline Tailwind classes */}
              <button
                type="submit"
                disabled={!canSubmit}
                className="cursor-pointer flex-shrink-0 self-center p-2 rounded-lg bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm transition-colors disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-primary-600"
                aria-label={loading ? t('input.processing') : t('input.sendMessage')}
              >
                {loading ? (
                  <div className="w-[18px] h-[18px] border-2 border-white/60 border-t-transparent rounded-full animate-spin" />
                ) : (
                  <PaperPlaneRight size={18} weight="fill" />
                )}
              </button>
            </div>

            {isCommandListVisible && (
              <div className="absolute left-0 right-0 bottom-full mb-1.5 z-20 overflow-hidden rounded-lg border border-gray-200/70 bg-white/98 shadow-md backdrop-blur-xl dark:border-gray-700/70 dark:bg-gray-900/98 dark:shadow-black/20">
                {filteredCommands.length > 0 ? (
                  <ul role="listbox" aria-label={t('slash.commandsList')} className="py-1 px-1">
                    {filteredCommands.map((command, index) => {
                      const isActive = index === activeCommandIndex
                      return (
                        <li
                          key={command.name}
                          role="option"
                          aria-selected={isActive}
                          ref={(node) => {
                            commandItemRefs.current[index] = node
                          }}
                          onMouseEnter={() => setActiveCommandIndex(index)}
                          onMouseDown={(e) => {
                            e.preventDefault()
                            submitCommandSelection(command.name)
                          }}
                          className={`cursor-pointer flex items-baseline gap-2 rounded-md px-2 py-1.5 transition-colors duration-75 ${
                            isActive
                              ? 'bg-gray-100 dark:bg-gray-800'
                              : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'
                          }`}
                        >
                          <span className="text-xs font-medium font-mono text-gray-800 dark:text-gray-200 shrink-0">
                            <span className="text-gray-400 dark:text-gray-500">/</span>{command.name.slice(1)}
                          </span>
                          <span className="text-[11px] text-gray-400 dark:text-gray-500 truncate">
                            {command.description}
                          </span>
                        </li>
                      )
                    })}
                  </ul>
                ) : (
                  <div className="px-3 py-2.5 text-center">
                    <p className="text-[11px] text-gray-400 dark:text-gray-500">
                      {t('slash.noMatches')}
                    </p>
                  </div>
                )}
              </div>
            )}
          </div>

        </form>
      </div>
    )
  }
)

MessageInput.displayName = 'MessageInput'
