import { differenceInDays, startOfDay } from 'date-fns'
import type { Session, SessionGroup } from '../types'

/**
 * Groups chat sessions by date relative to today
 * Categories: Today, Yesterday, Last 7 Days, Last 30 Days, Older
 * Sessions within each group are sorted by updatedAt (most recent first)
 */
export function groupSessionsByDate(sessions: Session[], t?: (key: string) => string): SessionGroup[] {
  // Ensure sessions is always an array
  const safeSessions = Array.isArray(sessions) ? sessions : []

  const now = new Date()
  const today = startOfDay(now)

  // Use translation function if provided, otherwise use key as fallback
  const translate = t || ((key: string) => key)

  // Map internal keys to translation keys
  const dateLabels: Record<string, string> = {
    'Today': translate('dateGroup.today'),
    'Yesterday': translate('dateGroup.yesterday'),
    'Last 7 Days': translate('dateGroup.last7Days'),
    'Last 30 Days': translate('dateGroup.last30Days'),
    'Older': translate('dateGroup.older'),
  }

  // Initialize all groups in the desired order
  const groupMap = new Map<string, Session[]>([
    ['Today', []],
    ['Yesterday', []],
    ['Last 7 Days', []],
    ['Last 30 Days', []],
    ['Older', []],
  ])

  // Categorize each session
  safeSessions.forEach((session) => {
    const sessionDate = new Date(session.updatedAt)
    const daysDiff = differenceInDays(today, startOfDay(sessionDate))

    if (daysDiff === 0) {
      groupMap.get('Today')!.push(session)
    } else if (daysDiff === 1) {
      groupMap.get('Yesterday')!.push(session)
    } else if (daysDiff <= 7) {
      groupMap.get('Last 7 Days')!.push(session)
    } else if (daysDiff <= 30) {
      groupMap.get('Last 30 Days')!.push(session)
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

  // Convert to array and filter out empty groups, translating names
  const groups: SessionGroup[] = []
  groupMap.forEach((sessions, internalName) => {
    if (sessions.length > 0) {
      groups.push({ name: dateLabels[internalName] || internalName, sessions })
    }
  })

  return groups
}
