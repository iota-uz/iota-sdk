/**
 * Citation Processing Utility
 *
 * Transforms OpenAI citations from raw markers (e.g., ≡cite≡turn0search2≡)
 * to formatted inline citation markers (e.g., [1], [2]).
 *
 * Process:
 * 1. Sort citations by startIndex (descending) to process from end to start
 * 2. Replace character ranges [startIndex, endIndex] with numbered markers
 * 3. Return processed content with clean inline citation references
 */

import type { Citation } from '../types'

export interface ProcessedContent {
  /** Content with citation markers replaced by [1], [2], etc. */
  content: string
  /** Citations array with their display indices */
  citations: Array<Citation & { displayIndex: number }>
}

/**
 * Process message content to replace raw citation markers with formatted inline citations
 *
 * @param content - Raw message content with potential citation markers
 * @param citations - Array of citations with startIndex/endIndex positions
 * @returns Processed content with clean citation markers and indexed citations
 *
 * @example
 * ```ts
 * const result = processCitations(
 *   "Tesla reported $28B ≡cite≡turn0search2≡ revenue",
 *   [{ startIndex: 20, endIndex: 42, title: "...", url: "...", type: "url_citation" }]
 * )
 * // result.content = "Tesla reported $28B [1] revenue"
 * // result.citations = [{ ..., displayIndex: 1 }]
 * ```
 */
export function processCitations(
  content: string,
  citations: Citation[] | null | undefined
): ProcessedContent {
  // If no citations, return content as-is
  if (!citations || citations.length === 0) {
    return { content, citations: [] }
  }

  // Sort citations by startIndex (descending) to process from end to start
  // This prevents index shifting issues when replacing text
  const sortedCitations = [...citations].sort((a, b) => b.startIndex - a.startIndex)

  let processedContent = content
  const processedCitations: Array<Citation & { displayIndex: number }> = []

  // Process each citation from end to start
  sortedCitations.forEach((citation, index) => {
    const { startIndex, endIndex } = citation

    // Validate indices
    if (startIndex < 0 || endIndex > processedContent.length || startIndex >= endIndex) {
      console.warn('[citationProcessor] Invalid citation indices:', {
        startIndex,
        endIndex,
        contentLength: processedContent.length,
      })
      return
    }

    // Calculate display index (1-based, in original order)
    const displayIndex = citations.length - index

    // Replace the citation marker range with [N]
    const before = processedContent.slice(0, startIndex)
    const after = processedContent.slice(endIndex)
    processedContent = `${before}[${displayIndex}]${after}`

    // Store citation with its display index
    processedCitations.unshift({
      ...citation,
      displayIndex,
    })
  })

  return {
    content: processedContent,
    citations: processedCitations,
  }
}
