/**
 * MessageInput component
 * Input field for sending messages with auto-resize and keyboard shortcuts
 */

import { useEffect, useRef } from 'react'
import { useChat } from '../context/ChatContext'

export function MessageInput() {
  const { message, setMessage, handleSubmit, loading } = useChat()
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = `${textareaRef.current.scrollHeight}px`
    }
  }, [message])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e as unknown as React.FormEvent)
    }

    if (e.key === 'Escape') {
      setMessage('')
    }
  }

  return (
    <div className="bichat-input border-t border-[var(--bichat-border)] p-4">
      <form onSubmit={handleSubmit} className="max-w-4xl mx-auto">
        <div className="flex gap-2 items-end">
          <div className="flex-1 relative">
            <textarea
              ref={textareaRef}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a message... (Enter to send, Shift+Enter for new line)"
              className="w-full px-4 py-3 pr-12 border border-[var(--bichat-border)] rounded-2xl focus:ring-2 focus:ring-[var(--bichat-primary)] focus:border-transparent resize-none max-h-40 overflow-y-auto"
              rows={1}
              disabled={loading}
              aria-label="Message input"
            />
            {loading && (
              <div className="absolute right-3 top-3">
                <div className="w-5 h-5 border-2 border-[var(--bichat-primary)] border-t-transparent rounded-full animate-spin" />
              </div>
            )}
          </div>
          <button
            type="submit"
            disabled={!message.trim() || loading}
            className="px-6 py-3 bg-[var(--bichat-primary)] text-white rounded-2xl hover:opacity-90 transition-opacity disabled:opacity-50 disabled:cursor-not-allowed"
            aria-label="Send message"
          >
            <svg
              className="w-5 h-5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
              />
            </svg>
          </button>
        </div>
        {loading && (
          <div className="mt-2 text-sm text-gray-500 flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-gray-400 border-t-transparent rounded-full animate-spin" />
            <span>AI is thinking...</span>
          </div>
        )}
      </form>
    </div>
  )
}
