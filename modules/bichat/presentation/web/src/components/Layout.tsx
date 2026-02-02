/**
 * Layout Component
 * Main two-column layout (sidebar + content area)
 * Desktop-only version (no mobile sidebar toggle/overlay)
 */

import { useState } from 'react'
import { Outlet, useNavigate } from 'react-router-dom'
import { useMutation } from 'urql'
import Sidebar from './Sidebar'

const CREATE_SESSION_MUTATION = `
  mutation CreateSession {
    createSession {
      id
      title
    }
  }
`

export default function Layout() {
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)
  const [, createSession] = useMutation(CREATE_SESSION_MUTATION)

  // Handle new chat button
  const handleNewChat = async () => {
    setCreating(true)
    try {
      const result = await createSession({})
      if (result.data?.createSession) {
        navigate(`/session/${result.data.createSession.id}`)
      }
    } catch (error) {
      console.error('Failed to create session:', error)
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="flex flex-1 w-full min-h-screen overflow-hidden">
      {/* Sidebar - always visible (desktop-only) */}
      <Sidebar onNewChat={handleNewChat} creating={creating} />

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-screen overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
