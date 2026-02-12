import { differenceInDays, startOfDay } from 'date-fns'

/**
 * Chat session type for BiChat
 */
export interface ChatSession {
  id: string
  title: string | null
  createdAt: string
  updatedAt: string
  pinned?: boolean
}

/**
 * Represents a group of chat sessions organized by date
 */
export interface SessionGroup {
  name: string
  sessions: ChatSession[]
}

/**
 * Groups chat sessions by date relative to today
 * Categories: Today, Yesterday, Previous 7 Days, Previous 30 Days, Older
 * Sessions within each group are sorted by updatedAt (most recent first)
 */
export function groupSessionsByDate(sessions: ChatSession[]): SessionGroup[] {
  const now = new Date()
  const today = startOfDay(now)

  // Initialize all groups in the desired order
  const groupMap = new Map<string, ChatSession[]>([
    ['Today', []],
    ['Yesterday', []],
    ['Previous 7 Days', []],
    ['Previous 30 Days', []],
    ['Older', []],
  ])

  // Categorize each session
  sessions.forEach((session) => {
    const sessionDate = new Date(session.updatedAt)
    const daysDiff = differenceInDays(today, startOfDay(sessionDate))

    if (daysDiff === 0) {
      groupMap.get('Today')!.push(session)
    } else if (daysDiff === 1) {
      groupMap.get('Yesterday')!.push(session)
    } else if (daysDiff <= 7) {
      groupMap.get('Previous 7 Days')!.push(session)
    } else if (daysDiff <= 30) {
      groupMap.get('Previous 30 Days')!.push(session)
    } else {
      groupMap.get('Older')!.push(session)
    }
  })

  // Sort sessions within each group by updatedAt (most recent first)
  groupMap.forEach((sessions) => {
    sessions.sort((a, b) => {
      return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
    })
  })

  // Convert to array and filter out empty groups
  const groups: SessionGroup[] = []
  groupMap.forEach((sessions, name) => {
    if (sessions.length > 0) {
      groups.push({ name, sessions })
    }
  })

  return groups
}
