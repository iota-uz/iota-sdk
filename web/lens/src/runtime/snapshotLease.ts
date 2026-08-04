interface SnapshotLease {
  consumers: number
  release: () => void
  timer?: ReturnType<typeof setTimeout>
}

// React StrictMode mounts an effect, cleans it up, and mounts it again. A
// module-level refcount with a one-task grace period prevents that probe (or a
// second runtime sharing the document) from releasing a live snapshot.
const snapshotLeases = new Map<string, SnapshotLease>()

export function acquireSnapshotLease(key: string, release: () => void): () => void {
  const current = snapshotLeases.get(key) ?? { consumers: 0, release }
  if (current.timer !== undefined) clearTimeout(current.timer)
  current.timer = undefined
  current.consumers += 1
  current.release = release
  snapshotLeases.set(key, current)
  return () => {
    const active = snapshotLeases.get(key)
    if (!active) return
    active.consumers = Math.max(0, active.consumers - 1)
    if (active.consumers > 0) return
    active.timer = setTimeout(() => {
      const pending = snapshotLeases.get(key)
      if (!pending || pending.consumers > 0) return
      snapshotLeases.delete(key)
      pending.release()
    }, 0)
  }
}
