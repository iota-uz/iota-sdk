import { spawn } from 'node:child_process'
import { createInterface } from 'node:readline'
import { randomUUID } from 'node:crypto'
import type { EnvironmentDescriptor, EnvironmentLifecycle } from './index.js'

export type DriverDescriptor = EnvironmentDescriptor & {
  environmentId: string; buildRevision: string; schemaFingerprint: string; baselineFingerprint: string
  capabilities: readonly string[]; artifactDirectory: string
}
export class DriverError extends Error {
  readonly code: string
  readonly operation: string
  readonly environmentId?: string
  readonly artifactDirectory?: string
  readonly descriptor?: Partial<DriverDescriptor>
  readonly causes: readonly DriverError[]
  constructor(message: string, details: { code: string; operation: string; environmentId?: string; artifactDirectory?: string; descriptor?: Partial<DriverDescriptor>; causes?: readonly DriverError[]; cause?: unknown }) {
    super(message, { cause: details.cause })
    this.name = 'DriverError'
    this.code = details.code
    this.operation = details.operation
    this.environmentId = details.environmentId
    this.artifactDirectory = details.artifactDirectory
    this.descriptor = details.descriptor
    this.causes = details.causes ?? []
  }
}
/** One persistent, shell-free driver owns every environment it creates. */
export function createProcessEnvironmentDriver<Spec>(options: {
  command: string; args?: readonly string[]; cwd?: string; env?: Record<string, string | undefined>
  spec(workerIndex: number): Spec; timeoutMs?: number; closeTimeoutMs?: number; killTimeoutMs?: number
}) {
  const timeoutMs = options.timeoutMs ?? 120_000
  const closeTimeoutMs = options.closeTimeoutMs ?? 35_000
  const killTimeoutMs = options.killTimeoutMs ?? 10_000
  for (const [name, value] of Object.entries({ timeoutMs, closeTimeoutMs, killTimeoutMs })) {
    if (!Number.isInteger(value) || value <= 0) throw new Error(`${name} must be a positive integer`)
  }
  const child = spawn(options.command, [...options.args ?? []], { cwd: options.cwd, env: options.env, stdio: ['pipe', 'pipe', 'pipe'] })
  type Request = { operation: string; environmentId?: string }
  const pending = new Map<string, { request: Request; resolve(value: any): void; reject(error: DriverError): void; timer: ReturnType<typeof setTimeout> }>()
  const environments = new Map<string, DriverDescriptor>()
  let stderr = ''
  let closed = false
  let closing: Promise<void> | undefined
  child.stderr.on('data', chunk => { stderr = (stderr + chunk).slice(-16_384) })
  const failure = (request: Request, code: string, message: string, cause?: unknown) => new DriverError(message, {
    code, operation: request.operation, environmentId: request.environmentId,
    artifactDirectory: request.environmentId ? environments.get(request.environmentId)?.artifactDirectory : undefined, cause,
  })
  const rejectAll = (code: string, message: string, cause?: unknown) => {
    closed = true
    for (const item of pending.values()) { clearTimeout(item.timer); item.reject(failure(item.request, code, message, cause)) }
    pending.clear()
  }
  const exit = new Promise<void>((resolve, reject) => {
    child.once('error', error => {
      rejectAll('process_error', error.message, error)
      reject(failure({ operation: 'close' }, 'process_error', error.message, error))
    })
    child.once('exit', (code, signal) => {
      const message = `Test environment driver exited (${code ?? signal}): ${stderr}`
      rejectAll('process_exit', message)
      if (code === 0) resolve(); else reject(failure({ operation: 'close' }, 'process_exit', message))
    })
  })
  void exit.catch(() => {})
  const lines = createInterface({ input: child.stdout })
  const close = () => closing ??= (async () => {
    closed = true
    child.stdin.end()
    let timer: ReturnType<typeof setTimeout> | undefined
    const settled = async (ms: number): Promise<boolean> => {
      try {
        return await Promise.race([exit.then(() => true), new Promise<false>(resolve => { timer = setTimeout(() => resolve(false), ms) })])
      } finally { if (timer) clearTimeout(timer) }
    }
    try {
      if (await settled(closeTimeoutMs)) return
      child.kill('SIGINT')
      if (await settled(killTimeoutMs)) return
      child.kill('SIGKILL')
      // Only this spawned driver receives the hard kill; never await its exit
      // beyond the shutdown deadline. Its rejection remains observed above.
      throw failure({ operation: 'close' }, 'close_timeout', 'Test environment driver did not close before its shutdown deadline')
    } finally { lines.close() }
  })()
  lines.on('line', line => {
    let response: any
    const validError = (error: any): boolean => error !== null && typeof error === 'object' && typeof error.code === 'string' && typeof error.message === 'string' && (error.causes === undefined || Array.isArray(error.causes) && error.causes.every(validError))
    try {
      response = JSON.parse(line)
      if (response === null || typeof response !== 'object' || typeof response.id !== 'string' || (response.error !== undefined && !validError(response.error))) throw new Error('Invalid response envelope')
    } catch {
      rejectAll('invalid_response', 'Test environment driver emitted invalid JSON')
      void close().catch(() => {})
      return
    }
    const item = pending.get(response.id)
    if (!item) return
    pending.delete(response.id); clearTimeout(item.timer)
    if (response.error) {
      const convert = (error: any): DriverError => new DriverError(`${error.code}: ${error.message}`, {
        code: error.code, operation: error.operation ?? item.request.operation,
        environmentId: error.environmentId ?? response.descriptor?.environmentId ?? item.request.environmentId,
        artifactDirectory: error.artifactDirectory ?? response.descriptor?.artifactDirectory ?? (item.request.environmentId ? environments.get(item.request.environmentId)?.artifactDirectory : undefined),
        descriptor: response.descriptor, causes: error.causes?.map(convert),
      })
      item.reject(convert(response.error))
    } else item.resolve(response.descriptor)
  })
  child.stdin.on('error', error => {
    rejectAll('write_failed', error.message, error)
    void close().catch(() => {})
  })
  const invoke = (request: Request & { spec?: Spec }): Promise<any> => {
    if (closed) return Promise.reject(failure(request, 'driver_closed', 'Test environment driver is closed'))
    const id = randomUUID()
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        pending.delete(id); reject(failure(request, 'request_timeout', 'Test environment driver request timed out'))
        void close().catch(() => {})
      }, timeoutMs)
      pending.set(id, { request, resolve, reject, timer })
      child.stdin.write(`${JSON.stringify({ id, ...request })}\n`, error => {
        if (error) { clearTimeout(timer); pending.delete(id); reject(failure(request, 'write_failed', error.message, error)) }
      })
    })
  }
  const lifecycle: EnvironmentLifecycle<DriverDescriptor> = {
    start: async workerIndex => {
      const descriptor = await invoke({ operation: 'start', spec: options.spec(workerIndex) })
      if (!descriptor?.environmentId || !descriptor?.baseURL || !descriptor?.artifactDirectory) {
        void close().catch(() => {})
        throw new DriverError('Incomplete test environment descriptor', { code: 'invalid_descriptor', operation: 'start', environmentId: descriptor?.environmentId, artifactDirectory: descriptor?.artifactDirectory, descriptor })
      }
      const environment = { ...descriptor, runId: descriptor.environmentId, artifactDir: descriptor.artifactDirectory }
      environments.set(environment.environmentId, environment)
      return environment
    },
    ready: async () => {},
    stop: async environment => { await invoke({ operation: 'stop', environmentId: environment.environmentId }); environments.delete(environment.environmentId) },
  }
  return { lifecycle, close }
}
