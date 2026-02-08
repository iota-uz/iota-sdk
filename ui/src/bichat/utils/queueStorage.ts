import type { QueuedMessage } from '../types'

const STORAGE_PREFIX = 'bichat.queue.'
const MAX_BYTES = 512 * 1024 // 512KB safety limit for sessionStorage payloads

function key(sessionId: string): string {
  return `${STORAGE_PREFIX}${sessionId}`
}

function safeSerialize(queue: QueuedMessage[]): string | null {
  try {
    const json = JSON.stringify(queue)
    if (json.length > MAX_BYTES) return null
    return json
  } catch {
    return null
  }
}

export function saveQueue(sessionId: string, queue: QueuedMessage[]) {
  if (typeof window === 'undefined') return
  if (!sessionId || sessionId === 'new') return

  try {
    if (!queue || queue.length === 0) {
      window.sessionStorage.removeItem(key(sessionId))
      return
    }

    const serialized = safeSerialize(queue)
    if (!serialized) return

    window.sessionStorage.setItem(key(sessionId), serialized)
  } catch {
    // ignore storage errors (quota, privacy mode)
  }
}

export function loadQueue(sessionId: string): QueuedMessage[] {
  if (typeof window === 'undefined') return []
  if (!sessionId || sessionId === 'new') return []

  try {
    const raw = window.sessionStorage.getItem(key(sessionId))
    if (!raw) return []

    const parsed = JSON.parse(raw) as unknown
    if (!Array.isArray(parsed)) return []

    // Minimal validation
    return parsed
      .map((item) => {
        const obj = item as any
        return {
          content: typeof obj?.content === 'string' ? obj.content : '',
          attachments: Array.isArray(obj?.attachments) ? obj.attachments : [],
        } satisfies QueuedMessage
      })
      .filter((m) => m.content.trim() !== '' || (m.attachments?.length ?? 0) > 0)
  } catch {
    return []
  }
}

export function clearQueue(sessionId: string) {
  if (typeof window === 'undefined') return
  if (!sessionId || sessionId === 'new') return
  try {
    window.sessionStorage.removeItem(key(sessionId))
  } catch {
    // ignore
  }
}

