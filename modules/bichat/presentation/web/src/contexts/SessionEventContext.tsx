import { createContext, useContext, type JSX, type Accessor } from 'solid-js'
import { createSignal } from 'solid-js'

export interface SessionEvents {
  /** Notifies listeners that a session was created (sidebar reload). */
  notifySessionCreated: (sessionId: string) => void
  /** Subscribes to session-created notifications; returns unsubscribe. */
  onSessionCreated: (listener: (sessionId: string) => void) => () => void
}

const SessionEventContext = createContext<SessionEvents>()

export function SessionEventProvider(props: { children: JSX.Element }): JSX.Element {
  const listeners = new Set<(sessionId: string) => void>()
  const events: SessionEvents = {
    notifySessionCreated(sessionId) {
      for (const listener of listeners) listener(sessionId)
    },
    onSessionCreated(listener) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
  return <SessionEventContext.Provider value={events}>{props.children}</SessionEventContext.Provider>
}

export function useSessionEvents(): SessionEvents {
  const events = useContext(SessionEventContext)
  if (!events) throw new Error('useSessionEvents must be used within SessionEventProvider')
  return events
}

export function createSessionCreatedSignal(): Accessor<string | undefined> {
  const events = useSessionEvents()
  const [last, setLast] = createSignal<string | undefined>(undefined)
  void events.onSessionCreated((sessionId) => setLast(sessionId))
  return last
}
