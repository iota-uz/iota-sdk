export type QueryKey = readonly [namespace: string, method: string, request: string]

export interface QueryCache {
  get<T>(key: QueryKey): T | undefined
  set<T>(key: QueryKey, value: T): void
  invalidate(namespace: string, methods: readonly string[]): void
}

function cacheKey(key: QueryKey): string { return JSON.stringify(key) }

export class MemoryQueryCache implements QueryCache {
  private readonly values = new Map<string, unknown>()

  get<T>(key: QueryKey): T | undefined { return this.values.get(cacheKey(key)) as T | undefined }
  set<T>(key: QueryKey, value: T): void { this.values.set(cacheKey(key), value) }
  invalidate(namespace: string, methods: readonly string[]): void {
    const families = new Set(methods)
    for (const encoded of this.values.keys()) {
      const [entryNamespace, method] = JSON.parse(encoded) as QueryKey
      if (entryNamespace === namespace && families.has(method)) this.values.delete(encoded)
    }
  }
}

export function stableSerialize(value: unknown): string {
  if (value === undefined) return 'null'
  if (value === null || typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(stableSerialize).join(',')}]`
  const object = value as Record<string, unknown>
  return `{${Object.keys(object).sort().map((key) => `${JSON.stringify(key)}:${stableSerialize(object[key])}`).join(',')}}`
}

export function queryKey(namespace: string, method: string, request: unknown): QueryKey {
  return [namespace, method, stableSerialize(request)]
}
