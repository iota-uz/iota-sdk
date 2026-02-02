/**
 * HomePage - Landing page with example queries and chat input
 * Shown at root when no session is selected
 * Matches shyona's ChatGPT-style centered layout
 */

import { useState, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation } from 'urql'
import { WelcomeContent, MessageInput, type MessageInputRef } from '@iota-uz/bichat-ui'

const CREATE_SESSION_MUTATION = `
  mutation CreateSession {
    createSession {
      id
      title
    }
  }
`

const SEND_MESSAGE_MUTATION = `
  mutation SendMessage($sessionId: ID!, $content: String!) {
    sendMessage(sessionId: $sessionId, content: $content) {
      id
    }
  }
`

export default function HomePage() {
  const navigate = useNavigate()
  const [, createSession] = useMutation(CREATE_SESSION_MUTATION)
  const [, sendMessage] = useMutation(SEND_MESSAGE_MUTATION)

  const [message, setMessage] = useState('')
  const [loading, setLoading] = useState(false)
  const inputRef = useRef<MessageInputRef>(null)

  // Submit message - creates session and sends message
  const submitMessage = useCallback(async (content: string) => {
    if (!content.trim() || loading) return

    setLoading(true)
    try {
      // Create a new session
      const result = await createSession({})
      if (result.data?.createSession) {
        const sessionId = result.data.createSession.id

        // Send the message
        await sendMessage({ sessionId, content: content.trim() })

        // Navigate to the session
        navigate(`/session/${sessionId}`)
      }
    } catch (error) {
      console.error('Failed to create session:', error)
    } finally {
      setLoading(false)
    }
  }, [createSession, sendMessage, navigate, loading])

  // Handle example prompt click
  const handlePromptSelect = useCallback((promptText: string) => {
    submitMessage(promptText)
  }, [submitMessage])

  // Handle form submit
  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault()
    submitMessage(message)
  }, [message, submitMessage])

  return (
    <div className="flex-1 flex flex-col items-center justify-center overflow-y-auto px-4 bg-gradient-to-b from-gray-50 to-gray-100 dark:from-gray-950 dark:to-gray-900">
      {/* Welcome content with example prompts */}
      <WelcomeContent
        onPromptSelect={handlePromptSelect}
        disabled={loading}
      />

      {/* Centered input - premium floating design */}
      <div className="w-full max-w-4xl pb-10 relative z-10">
        <MessageInput
          ref={inputRef}
          message={message}
          loading={loading}
          disabled={loading}
          onMessageChange={setMessage}
          onSubmit={handleSubmit}
          placeholder="Ask BiChat about your business data..."
          containerClassName="p-4"
        />

        {/* AI disclaimer - refined */}
        <div className="mt-4 text-center">
          <p className="text-xs text-gray-400 dark:text-gray-500">
            BiChat is powered by AI. Responses may not always be accurate.
          </p>
        </div>
      </div>
    </div>
  )
}
