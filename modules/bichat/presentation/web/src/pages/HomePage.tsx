/**
 * HomePage - Landing page with example queries
 * Shown at root when no session is selected
 */

import { useNavigate } from 'react-router-dom'
import { useMutation } from 'urql'
import { WelcomeContent } from '@iota-uz/bichat-ui'

const CREATE_SESSION_MUTATION = `
  mutation CreateSession {
    createSession {
      id
      title
    }
  }
`

export default function HomePage() {
  const navigate = useNavigate()
  const [, createSession] = useMutation(CREATE_SESSION_MUTATION)

  const handlePromptSelect = async (promptText: string) => {
    try {
      // Create a new session
      const result = await createSession({})
      if (result.data?.createSession) {
        // Navigate to the new session with the prompt as state
        navigate(`/session/${result.data.createSession.id}`, {
          state: { initialMessage: promptText }
        })
      }
    } catch (error) {
      console.error('Failed to create session:', error)
    }
  }

  return (
    <div className="flex-1 flex items-center justify-center overflow-y-auto bg-gray-50 dark:bg-gray-950">
      <WelcomeContent onPromptSelect={handlePromptSelect} />
    </div>
  )
}
