import { spawn } from 'node:child_process'
import { createInterface } from 'node:readline'
import { randomUUID } from 'node:crypto'
import type { EnvironmentDescriptor, EnvironmentLifecycle } from './index.js'

export type DriverDescriptor = EnvironmentDescriptor & {
  environmentId: string; buildRevision: string; schemaFingerprint: string; baselineFingerprint: string
  capabilities: readonly string[]; artifactDirectory: string
}
/** One persistent, shell-free driver owns every environment it creates. */
export function createProcessEnvironmentDriver<Spec>(options: {
  command: string; args?: readonly string[]; cwd?: string; env?: Record<string, string | undefined>
  spec(workerIndex: number): Spec; timeoutMs?: number
}) {
  const timeoutMs = options.timeoutMs ?? 120_000
  if (!Number.isInteger(timeoutMs) || timeoutMs <= 0) throw new Error('timeoutMs must be a positive integer')
  const child = spawn(options.command, [...options.args ?? []], { cwd: options.cwd, env: options.env, stdio: ['pipe', 'pipe', 'pipe'] })
  const pending = new Map<string, { resolve(value: any): void; reject(error: Error): void; timer: ReturnType<typeof setTimeout> }>()
  let stderr = ''
  let closed = false
  child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-16_384) })
  const rejectAll = (error: Error) => {
    closed = true
    for (const request of pending.values()) { clearTimeout(request.timer); request.reject(error) }
    pending.clear()
  }
  const exit = new Promise<void>((resolve, reject) => {
    child.once('error', error => { rejectAll(error); reject(error) })
    child.once('exit', (code, signal) => {
      const error = new Error(`Test environment driver exited (${code ?? signal}): ${stderr}`)
      rejectAll(error)
      if (code === 0) resolve(); else reject(error)
    })
  })
  // Keep unexpected process exits observed even before the caller closes the driver.
  void exit.catch(() => {})
  const lines = createInterface({ input: child.stdout })
  lines.on('line', line => {
    let response: any
    try { response = JSON.parse(line) } catch { rejectAll(new Error('Test environment driver emitted invalid JSON')); child.kill('SIGINT'); return }
    const request = pending.get(response.id)
    if (!request) return
    pending.delete(response.id); clearTimeout(request.timer)
    if (response.error) request.reject(new Error(`${response.error.code}: ${response.error.message}`))
    else request.resolve(response.descriptor)
  })
  const invoke = (request: Record<string, unknown>): Promise<any> => {
    if (closed) return Promise.reject(new Error('Test environment driver is closed'))
    const id = randomUUID()
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id); reject(new Error('Test environment driver request timed out')); child.kill('SIGINT')
      }, timeoutMs)
      pending.set(id, { resolve, reject, timer })
      child.stdin.write(`${JSON.stringify({ id, ...request })}\n`, error => {
        if (error) { clearTimeout(timer); pending.delete(id); reject(error) }
      })
    })
  }
  const lifecycle: EnvironmentLifecycle<DriverDescriptor> = {
    start: async workerIndex => {
      const descriptor = await invoke({ operation: 'start', spec: options.spec(workerIndex) })
      if (!descriptor?.environmentId || !descriptor?.baseURL || !descriptor?.artifactDirectory) { child.kill('SIGINT'); throw new Error('Incomplete test environment descriptor') }
      return { ...descriptor, runId: descriptor.environmentId, artifactDir: descriptor.artifactDirectory }
    },
    // Driver start returns only after the configured real readiness probe succeeds.
    ready: async () => {},
    stop: async environment => { await invoke({ operation: 'stop', environmentId: environment.environmentId }) },
  }
  return { lifecycle, close: async () => { child.stdin.end(); await exit; lines.close() } }
}
