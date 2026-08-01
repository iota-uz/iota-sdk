export const fallbackSeries = [
  '#2563eb', '#0d9488', '#d97706', '#7c3aed', '#dc2626',
  '#0284c7', '#db2777', '#65a30d', '#9333ea', '#64748b',
] as const

export function stablePaletteIndex(key: string, size: number): number {
  if (size <= 0) return 0
  let hash = 0x811c9dc5
  for (let index = 0; index < key.length; index += 1) {
    hash ^= key.charCodeAt(index)
    hash = Math.imul(hash, 0x01000193)
  }
  return (hash >>> 0) % size
}
