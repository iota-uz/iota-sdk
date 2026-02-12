import { createContext, useContext, useMemo, useRef, type ReactNode } from 'react'

type SessionCreatedCallback = (sessionId: string) => void

export interface SessionEventContextValue {
  notifySessionCreated: (sessionId: string) => void
  onSessionCreated: (cb: SessionCreatedCallback) => () => void
}

const SessionEventContext = createContext<SessionEventContextValue | null>(null)

export function SessionEventProvider({ children }: { children: ReactNode }) {
  const listenersRef = useRef(new Set<SessionCreatedCallback>())

  const value = useMemo<SessionEventContextValue>(() => {
    return {
      notifySessionCreated: (sessionId: string) => {
        listenersRef.current.forEach((cb) => cb(sessionId))
      },
      onSessionCreated: (cb: SessionCreatedCallback) => {
        listenersRef.current.add(cb)
        return () => {
          listenersRef.current.delete(cb)
        }
      },
    }
  }, [])

  return (
    <SessionEventContext.Provider value={value}>
      {children}
    </SessionEventContext.Provider>
  )
}

export function useSessionEvents(): SessionEventContextValue {
  const ctx = useContext(SessionEventContext)
  if (!ctx) {
    throw new Error('useSessionEvents must be used within SessionEventProvider')
  }
  return ctx
}

