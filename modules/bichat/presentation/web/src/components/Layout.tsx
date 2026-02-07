/**
 * Layout Component
 * Main layout (sidebar + content area).
 * Responsive: desktop sidebar + mobile drawer with focus trap and swipe-to-close.
 */

import { useEffect, useRef } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { AnimatePresence, motion, type PanInfo } from 'framer-motion'
import { List } from '@phosphor-icons/react'
import { SkipLink, useFocusTrap, useKeyboardShortcuts } from '@iota-uz/sdk/bichat'
import Sidebar from './Sidebar'
import { useSidebarState } from '../hooks/useSidebarState'

export default function Layout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { isMobile, isMobileOpen, openMobile, closeMobile } = useSidebarState()
  const drawerRef = useRef<HTMLDivElement>(null)
  const menuButtonRef = useRef<HTMLButtonElement>(null)

  // Handle new chat button - just navigate to home page
  const handleNewChat = () => {
    navigate('/')
  }

  useKeyboardShortcuts([
    {
      key: 'n',
      ctrl: true,
      description: 'New chat',
      callback: () => navigate('/'),
    },
  ])

  useFocusTrap(drawerRef, isMobile && isMobileOpen, menuButtonRef.current)

  useEffect(() => {
    if (!isMobile || !isMobileOpen) return

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        closeMobile()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [closeMobile, isMobile, isMobileOpen])

  const handleDrawerDragEnd = (_: MouseEvent | TouchEvent | PointerEvent, info: PanInfo) => {
    // Swipe-left to close
    if (info.offset.x < -80) {
      closeMobile()
    }
  }

  return (
    <div className="relative flex flex-1 w-full h-full min-h-0 overflow-hidden">
      <SkipLink />

      {/* Sidebar - desktop */}
      <div className="hidden md:block">
        <Sidebar onNewChat={handleNewChat} creating={false} />
      </div>

      {/* Sidebar - mobile drawer */}
      <AnimatePresence>
        {isMobile && isMobileOpen && (
          <>
            <motion.div
              className="fixed inset-0 z-40 bg-black/40"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={closeMobile}
              aria-hidden="true"
            />
            <motion.div
              className="fixed inset-y-0 left-0 z-50 w-[18rem] max-w-[85vw] shadow-2xl"
              initial={{ x: '-100%' }}
              animate={{ x: 0 }}
              exit={{ x: '-100%' }}
              transition={{ type: 'spring', stiffness: 320, damping: 32 }}
              drag="x"
              dragDirectionLock
              dragConstraints={{ left: -120, right: 0 }}
              dragElastic={{ left: 0.2, right: 0 }}
              onDragEnd={handleDrawerDragEnd}
              onClick={(e) => e.stopPropagation()}
            >
              <div ref={drawerRef} className="h-full bg-white dark:bg-gray-900">
                <Sidebar onNewChat={handleNewChat} creating={false} onClose={closeMobile} />
              </div>
            </motion.div>
          </>
        )}
      </AnimatePresence>

      {/* Main Content */}
      <main id="main-content" className="relative flex-1 flex flex-col min-h-0 overflow-hidden">
        {/* Mobile menu button */}
        {isMobile && !isMobileOpen && (
          <button
            ref={menuButtonRef}
            onClick={openMobile}
            className="md:hidden absolute top-3 left-3 z-30 w-10 h-10 rounded-xl bg-white/90 dark:bg-gray-900/90 text-gray-700 dark:text-gray-200 border border-gray-200/60 dark:border-gray-800/80 shadow-sm flex items-center justify-center hover:bg-white dark:hover:bg-gray-900 transition-colors cursor-pointer focus-visible:ring-2 focus-visible:ring-primary-400/50"
            aria-label="Open sidebar"
            title="Open sidebar"
          >
            <List size={20} weight="bold" />
          </button>
        )}
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={location.pathname}
            className="flex flex-1 min-h-0"
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.15, ease: 'easeOut' }}
          >
            <Outlet />
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  )
}
