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
      headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body) })
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
