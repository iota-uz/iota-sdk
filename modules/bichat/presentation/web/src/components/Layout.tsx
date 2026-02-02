/**
 * Layout Component
 * Main two-column layout (sidebar + content area)
 * Desktop-only version (no mobile sidebar toggle/overlay)
 */

import { Outlet, useNavigate } from 'react-router-dom'
import Sidebar from './Sidebar'

export default function Layout() {
  const navigate = useNavigate()

  // Handle new chat button - just navigate to home page
  const handleNewChat = () => {
    navigate('/')
  }

  return (
    <div className="flex flex-1 w-full min-h-screen overflow-hidden">
      {/* Sidebar - always visible (desktop-only) */}
      <Sidebar onNewChat={handleNewChat} creating={false} />

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-screen overflow-hidden">
        <Outlet />
      </main>
    </div>
  )
}
