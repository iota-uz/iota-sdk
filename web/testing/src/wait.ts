export type WaitOptions = { timeoutMs?: number }

export function timeout(options: WaitOptions = {}): number {
  const value = options.timeoutMs ?? 10_000
  if (!Number.isInteger(value) || value <= 0) throw new Error('timeoutMs must be a positive integer')
  return value
}

/** One budget for a multi-step wait: later steps receive what the earlier ones left. */
export function deadline(options: WaitOptions = {}): () => number {
  const end = Date.now() + timeout(options)
  return () => Math.max(1, end - Date.now())
}

export async function waitForCondition(condition: () => Promise<boolean>, timeoutMs: number, diagnostic: () => Promise<Error> | Error): Promise<void> {
  const end = Date.now() + timeoutMs
  while (!(await condition())) {
    if (Date.now() >= end) throw await diagnostic()
    await new Promise(resolve => setTimeout(resolve, Math.min(16, Math.max(1, end - Date.now()))))
  }
}
