import { useMemo } from 'react'
import Fuse from 'fuse.js'
import type { ChatSession } from '../utils/sessionGrouping'

/**
 * Hook for fuzzy searching chat sessions
 * Uses Fuse.js for flexible, typo-tolerant search
 *
 * Optimized with split memoization:
 * - Fuse instance is memoized based on sessions only
 * - Search results are memoized separately based on query
 */
export function useSessionSearch(sessions: ChatSession[], searchQuery: string): ChatSession[] {
  // Memoize Fuse instance based on sessions only
  // This prevents unnecessary Fuse recreation on every query change
  const fuse = useMemo(() => {
    return new Fuse(sessions, {
      keys: [
        {
          name: 'title',
          weight: 1.0, // Title is most important
        },
      ],
      threshold: 0.3, // 0 = perfect match, 1 = match anything
      includeScore: true,
      ignoreLocation: true, // Search anywhere in the string
      minMatchCharLength: 1,
    })
  }, [sessions])

  // Use the memoized Fuse instance for searching
  return useMemo(() => {
    const trimmedQuery = searchQuery.trim()

    // Return all sessions if no search query
    if (!trimmedQuery) {
      return sessions
    }

    // Perform search and extract items
    const results = fuse.search(trimmedQuery)
    return results.map((result) => result.item)
  }, [fuse, searchQuery, sessions])
}
