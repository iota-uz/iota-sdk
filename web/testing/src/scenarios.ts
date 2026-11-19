import type { ScenarioControl, WaitOptions } from './index.js'
export type ScenarioInput = { name: string; version: string; seed: string; now: string; params: Record<string, unknown> }
export type ScenarioResult<Data = Record<string, unknown>> = { scopeId: string; name: string; version: string; seed: string; now: string; refs: Record<string, { kind: string; id: string }>; data: Data }
export class ScenarioControlError extends Error {
  constructor(readonly code: string, readonly status: number, message: string) { super(message); this.name = 'ScenarioControlError' }
}
export function createHttpScenarioControl<Data = Record<string, unknown>>(environment: { environmentId: string; baseURL: string }, token: string, options: WaitOptions = {}): ScenarioControl<ScenarioInput, ScenarioResult<Data>> {
  const timeoutMs = options.timeoutMs ?? 10_000
  if (!Number.isInteger(timeoutMs) || timeoutMs <= 0 || token.length < 32) throw new Error('A positive timeout and a control credential of at least 32 characters are required')
  const invoke = async (pathname: string, method: string, body?: unknown): Promise<unknown> => {
    const response = await fetch(new URL(pathname, environment.baseURL), { method, redirect: 'manual', signal: AbortSignal.timeout(timeoutMs),
      headers: { Authorization: `Bearer ${token}`, ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) })
    if (!response.ok) {
      const text = await response.text()
      let error: { code?: string; message?: string } = {}
      try { error = JSON.parse(text) } catch {}
      throw new ScenarioControlError(error.code ?? `http_${response.status}`, response.status, error.message ?? `Scenario control returned HTTP ${response.status}`)
    }
    if (response.headers.get('X-Test-Environment-ID') !== environment.environmentId) throw new ScenarioControlError('environment_mismatch', response.status, 'Scenario endpoint belongs to another environment')
    return response.json()
  }
  return {
    prepare: async (input, scopeId) => {
      await invoke('/__test__/scopes', 'POST', { scopeId })
      return await invoke('/__test__/scenarios/prepare', 'POST', { ...input, scopeId }) as ScenarioResult<Data>
    },
    dispose: async scopeId => { await invoke(`/__test__/scopes/${encodeURIComponent(scopeId)}`, 'DELETE') },
  }
}

export type EntityRef = { kind: string; id: string }
export type ExecutionStatus = { executionId: string; scopeId: string; job: string; state: 'running' | 'succeeded' | 'failed' | 'canceled'; error?: string; startedAt: string; finishedAt?: string }
export type WaitResult = { outcome: 'reached' | 'failed' | 'timeout' | 'canceled'; condition: string; execution: ExecutionStatus; checks: number; lastConditionError?: string; elapsedMs: number }
export type ProviderStep = { operation?: string; outcome: 'success' | 'refusal' | 'timeout'; hold?: string; response?: Record<string, unknown> }
export type ProviderCall = { sequence: number; operation: string; key: string; outcome: string; correlation: string; hold?: string }
export type ScenarioManifest = { format: string; environment: Record<string, string>; scenario: ScenarioInput & { scopeId: string }; result: ScenarioResult; controls: { clock?: string; resources?: Record<string, unknown> } }

/** Typed failure of a job wait; `result` carries the execution diagnostics. */
export class JobWaitError extends ScenarioControlError {
  constructor(code: string, status: number, message: string, readonly result: WaitResult) { super(code, status, message); this.name = 'JobWaitError' }
}

/**
 * Scoped jobs, provider fakes, the dedicated clock and replay manifests of an
 * isolated environment. Every call names its physical scope; the server checks
 * that the scope belongs to this environment and credential.
 */
export function createHttpTestControls(environment: { environmentId: string; baseURL: string }, token: string, options: WaitOptions = {}) {
  const timeoutMs = options.timeoutMs ?? 10_000
  if (!Number.isInteger(timeoutMs) || timeoutMs <= 0 || token.length < 32) throw new Error('A positive timeout and a control credential of at least 32 characters are required')
  const invoke = async <T>(pathname: string, method: string, body?: unknown, requestTimeoutMs = timeoutMs): Promise<T> => {
    const response = await fetch(new URL(pathname, environment.baseURL), { method, redirect: 'manual', signal: AbortSignal.timeout(requestTimeoutMs),
      headers: { Authorization: `Bearer ${token}`, ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) })
    const text = await response.text()
    let payload: Record<string, unknown> = {}
    let parsed = false
    try { payload = JSON.parse(text); parsed = payload !== null && typeof payload === 'object' } catch {}
    if (!response.ok) {
      const code = typeof payload.code === 'string' ? payload.code : `http_${response.status}`
      const message = typeof payload.message === 'string' ? payload.message : `Test control returned HTTP ${response.status}`
      if (payload.details && typeof payload.details === 'object' && 'outcome' in payload.details) throw new JobWaitError(code, response.status, message, payload.details as WaitResult)
      throw new ScenarioControlError(code, response.status, message)
    }
    if (response.headers.get('X-Test-Environment-ID') !== environment.environmentId) throw new ScenarioControlError('environment_mismatch', response.status, 'Test control endpoint belongs to another environment')
    // A successful control answer is always a JSON object; anything else is a broken endpoint, not an empty result.
    if (!parsed) throw new ScenarioControlError('invalid_response', response.status, 'Test control returned a malformed JSON body')
    return payload as T
  }
  const scope = (scopeId: string) => `/__test__/scopes/${encodeURIComponent(scopeId)}`
  const provider = (scopeId: string, name: string) => `${scope(scopeId)}/providers/${encodeURIComponent(name)}`
  const boundedWait = (ms: number) => { if (!Number.isInteger(ms) || ms < 1 || ms > 120_000) throw new Error('timeoutMs must be an integer from 1 to 120000'); return ms }
  return {
    describe: () => invoke<Record<string, unknown>>('/__test__/controls', 'GET'),
    jobs: {
      run: async (scopeId: string, job: string, refs: Record<string, EntityRef> = {}) => (await invoke<{ executionId: string }>(`${scope(scopeId)}/jobs`, 'POST', { job, refs })).executionId,
      status: (scopeId: string, executionId: string) => invoke<ExecutionStatus>(`${scope(scopeId)}/executions/${encodeURIComponent(executionId)}`, 'GET'),
      /** Resolves when reached; rejects with JobWaitError on failed or timeout. */
      wait: (scopeId: string, executionId: string, condition: string, waitMs: number) => invoke<WaitResult>(`${scope(scopeId)}/executions/${encodeURIComponent(executionId)}/wait`, 'POST', { condition, timeoutMs: boundedWait(waitMs) }, boundedWait(waitMs) + timeoutMs),
    },
    providers: {
      script: async (scopeId: string, name: string, steps: ProviderStep[]) => { await invoke(`${provider(scopeId, name)}/scripts`, 'POST', { steps }) },
      calls: async (scopeId: string, name: string) => (await invoke<{ calls: ProviderCall[] }>(`${provider(scopeId, name)}/calls`, 'GET')).calls,
      waitHeld: async (scopeId: string, name: string, gate: string, waitMs: number) => { await invoke(`${provider(scopeId, name)}/gates/${encodeURIComponent(gate)}/wait`, 'POST', { timeoutMs: boundedWait(waitMs) }, boundedWait(waitMs) + timeoutMs) },
      release: async (scopeId: string, name: string, gate: string) => { await invoke(`${provider(scopeId, name)}/gates/${encodeURIComponent(gate)}/release`, 'POST') },
      forget: async (scopeId: string) => { await invoke(`${scope(scopeId)}/providers`, 'DELETE') },
    },
    clock: {
      /** Server time; pass it to Playwright's page.clock so browser and scenario time agree. */
      now: async () => new Date((await invoke<{ now: string }>('/__test__/clock', 'GET')).now),
      advance: async (scopeId: string, durationMs: number) => new Date((await invoke<{ now: string }>(`${scope(scopeId)}/clock/advance`, 'POST', { durationMs })).now),
    },
    manifest: (scopeId: string) => invoke<ScenarioManifest>(`${scope(scopeId)}/manifest`, 'GET'),
    replay: async (scopeId: string, manifest: ScenarioManifest) => {
      await invoke('/__test__/scopes', 'POST', { scopeId })
      return await invoke<ScenarioResult>(`${scope(scopeId)}/replay`, 'POST', manifest)
    },
  }
}
