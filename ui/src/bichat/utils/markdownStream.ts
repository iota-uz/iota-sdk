/**
 * Normalizes partially-streamed markdown so that react-markdown
 * can render it without layout artifacts (e.g. an unclosed code fence
 * turning the rest of the message into a code block).
 *
 * Called on every streaming content update — kept intentionally cheap (O(lines)).
 */
export function normalizeStreamingMarkdown(text: string): string {
  // Track unclosed code fences.
  // A code fence opens with ≥3 backticks at line start and closes with
  // the same (or more) backticks on its own line.
  const lines = text.split('\n')
  let inCodeBlock = false
  let fenceTicks = ''

  for (const line of lines) {
    const trimmed = line.trimStart()
    const fenceMatch = trimmed.match(/^(`{3,})/)
    if (fenceMatch) {
      if (!inCodeBlock) {
        inCodeBlock = true
        fenceTicks = fenceMatch[1]
      } else if (trimmed.startsWith(fenceTicks) && trimmed.slice(fenceTicks.length).trim() === '') {
        inCodeBlock = false
        fenceTicks = ''
      }
    }
  }

  if (inCodeBlock) {
    text += '\n' + fenceTicks
  }

  return text
}
