import type {
  ActivationResult, AuditEntry, BindingInfo, BuildReport, CheckReport, Diagnostic, KitInfo, PackageInfo, RemoteFile,
  Review, TestReport, TriggerOwner, TypeFiles, VersionInfo, WorkflowHistory, WorkflowInstance,
} from './types'

/** ApiError is a failure answered by the management API. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    readonly diagnostics: Diagnostic[] = [],
  ) {
    super(message)
    this.name = 'ApiError'
  }

  get isConflict(): boolean { return this.status === 409 }
  get isDenied(): boolean { return this.status === 403 || this.status === 401 }
}

export interface ApiOptions {
  /** Management API prefix, for example /_ext/manage. */
  base: string
  /** CSRF token of the session; read per request so a refreshed token is used. */
  csrf?: () => string | undefined
  fetch?: typeof fetch
}

interface Init {
  method?: string
  body?: BodyInit | null
  headers?: Record<string, string>
  signal?: AbortSignal | undefined
}

const enc = encodeURIComponent
const encPath = (p: string): string => p.split('/').map(enc).join('/')

/**
 * ManageApi is the browser client of /_ext/manage. It adds nothing to the
 * service: rights, validation and audit are server-side.
 */
export class ManageApi {
  private readonly doFetch: typeof fetch

  constructor(private readonly opts: ApiOptions) {
    this.doFetch = opts.fetch ?? ((input, init) => fetch(input, init))
  }

  private async raw(path: string, init: Init = {}): Promise<Response> {
    const headers: Record<string, string> = { ...init.headers }
    const csrf = this.opts.csrf?.()
    if (csrf) headers['x-csrf-token'] = csrf
    const res = await this.doFetch(this.opts.base + path, {
      method: init.method ?? 'GET', credentials: 'same-origin', headers, body: init.body ?? null, signal: init.signal ?? null,
    })
    if (res.ok) return res
    let code = 'error'
    let message = `HTTP ${res.status}`
    let diagnostics: Diagnostic[] = []
    try {
      const body = await res.json() as { error?: { code?: string; message?: string; diagnostics?: Diagnostic[] } }
      code = body.error?.code ?? code
      message = body.error?.message ?? message
      diagnostics = Array.isArray(body.error?.diagnostics) ? body.error.diagnostics : []
    } catch { /* non-JSON failure: keep the status text */ }
    throw new ApiError(res.status, code, message, diagnostics)
  }

  private async json<T>(path: string, init: Init = {}): Promise<T> {
    const res = await this.raw(path, init)
    if (res.status === 204) return undefined as T
    return await res.json() as T
  }

  private post<T>(path: string, body?: unknown, signal?: AbortSignal): Promise<T> {
    return this.json<T>(path, {
      method: 'POST', signal,
      ...(body === undefined ? {} : { body: JSON.stringify(body), headers: { 'content-type': 'application/json' } }),
    })
  }

  private pkg(ns: string): string { return `/packages/${enc(ns)}` }

  listPackages(signal?: AbortSignal): Promise<PackageInfo[]> { return this.json('/packages', { signal }) }
  createPackage(namespace: string): Promise<PackageInfo> { return this.post('/packages', { namespace }) }
  getPackage(ns: string, signal?: AbortSignal): Promise<PackageInfo> { return this.json(this.pkg(ns), { signal }) }

  listFiles(ns: string, signal?: AbortSignal): Promise<RemoteFile[]> { return this.json(`${this.pkg(ns)}/files`, { signal }) }

  async readFile(ns: string, path: string, signal?: AbortSignal): Promise<{ content: string; hash: string }> {
    const res = await this.raw(`${this.pkg(ns)}/files/${encPath(path)}`, { signal })
    const hash = (res.headers.get('ETag') ?? '').replace(/"/g, '')
    return { content: await res.text(), hash }
  }

  /**
   * writeFile saves a file. expectHash null overwrites without a check,
   * '' requires a new file, otherwise the current hash must match (409).
   */
  writeFile(ns: string, path: string, content: string, expectHash: string | null, signal?: AbortSignal): Promise<RemoteFile> {
    const headers: Record<string, string> = {}
    if (expectHash === '') headers['If-None-Match'] = '*'
    else if (expectHash !== null) headers['If-Match'] = `"${expectHash}"`
    return this.json(`${this.pkg(ns)}/files/${encPath(path)}`, { method: 'PUT', body: content, headers, signal })
  }

  deleteFile(ns: string, path: string): Promise<void> { return this.json(`${this.pkg(ns)}/files/${encPath(path)}`, { method: 'DELETE' }) }
  moveFile(ns: string, from: string, to: string): Promise<void> { return this.post(`${this.pkg(ns)}/move`, { from, to }) }

  check(ns: string, signal?: AbortSignal): Promise<CheckReport> { return this.post(`${this.pkg(ns)}/check`, undefined, signal) }
  build(ns: string, signal?: AbortSignal): Promise<BuildReport> { return this.post(`${this.pkg(ns)}/build`, undefined, signal) }
  test(ns: string, signal?: AbortSignal): Promise<TestReport> { return this.post(`${this.pkg(ns)}/test`, undefined, signal) }
  publish(ns: string, version?: string): Promise<VersionInfo> { return this.post(`${this.pkg(ns)}/publish`, version ? { version } : undefined) }

  listVersions(ns: string, signal?: AbortSignal): Promise<VersionInfo[]> { return this.json(`${this.pkg(ns)}/versions`, { signal }) }
  review(ns: string, version: string, approve: boolean, comment: string, v: Pick<VersionInfo, 'sourceHash' | 'artifactHash'>): Promise<Review> {
    return this.post(`${this.pkg(ns)}/versions/${enc(version)}/review`, {
      approve, comment, expectSourceHash: v.sourceHash, expectArtifactHash: v.artifactHash,
    })
  }
  activate(ns: string, version: string): Promise<ActivationResult> { return this.post(`${this.pkg(ns)}/versions/${enc(version)}/activate`) }
  deactivate(ns: string): Promise<void> { return this.post(`${this.pkg(ns)}/deactivate`) }

  importGit(repoUrl: string, commit: string, subdir: string): Promise<PackageInfo> { return this.post('/import/git', { repoUrl, commit, subdir }) }
  exportUrl(ns: string, version?: string): string {
    return `${this.opts.base}${this.pkg(ns)}/export?format=zip${version ? `&version=${enc(version)}` : ''}`
  }

  listBindings(signal?: AbortSignal): Promise<BindingInfo[]> { return this.json('/bindings', { signal }) }
  bind(slotKind: string, targetId: string, pkg: string, version: string, exportName: string): Promise<BindingInfo> {
    return this.post('/bindings', { slotKind, targetId, package: pkg, version, export: exportName })
  }
  unbind(slotKind: string, targetId: string): Promise<void> { return this.json(`/bindings/${enc(slotKind)}/${enc(targetId)}`, { method: 'DELETE' }) }

  listOwners(signal?: AbortSignal): Promise<TriggerOwner[]> { return this.json('/owners', { signal }) }
  assignOwner(triggerId: string, userId: number): Promise<TriggerOwner> {
    return this.json(`/triggers/${enc(triggerId)}/owner`, { method: 'PUT', body: JSON.stringify({ userId }), headers: { 'content-type': 'application/json' } })
  }
  removeOwner(triggerId: string): Promise<void> { return this.json(`/triggers/${enc(triggerId)}/owner`, { method: 'DELETE' }) }

  kit(signal?: AbortSignal): Promise<KitInfo> { return this.json('/kit', { signal }) }
  audit(namespace: string, signal?: AbortSignal): Promise<AuditEntry[]> { return this.json(`/audit?namespace=${enc(namespace)}&limit=50`, { signal }) }

  listWorkflows(status: string, signal?: AbortSignal): Promise<WorkflowInstance[]> {
    return this.json(`/workflows${status ? `?status=${enc(status)}` : ''}`, { signal })
  }
  getWorkflow(id: string, signal?: AbortSignal): Promise<WorkflowHistory> { return this.json(`/workflows/${enc(id)}`, { signal }) }
  signalWorkflow(id: string, name: string, payload: unknown): Promise<void> { return this.post(`/workflows/${enc(id)}/signal`, { name, payload }) }
  terminateWorkflow(id: string): Promise<void> { return this.post(`/workflows/${enc(id)}/terminate`) }
  startWorkflowV2(id: string, version?: string): Promise<{ instance: string }> { return this.post(`/workflows/${enc(id)}/start-v2`, version ? { version } : undefined) }
  resolveOccurrence(id: string, name: string, n: number, result: unknown): Promise<void> { return this.post(`/workflows/${enc(id)}/resolve`, { name, n, result }) }
  retryOccurrence(id: string, name: string, n: number): Promise<void> { return this.post(`/workflows/${enc(id)}/retry`, { name, n }) }
  resumeWorkflow(id: string): Promise<void> { return this.post(`/workflows/${enc(id)}/resume`) }
  decideTask(taskId: string, decision: unknown): Promise<void> { return this.post(`/tasks/${enc(taskId)}/decide`, { decision }) }
}

/** fetchTypes loads Monaco's type information (kit or SDK typings). */
export async function fetchTypes(url: string, signal?: AbortSignal, fetchImpl: typeof fetch = (i, n) => fetch(i, n)): Promise<TypeFiles> {
  const res = await fetchImpl(url, { credentials: 'same-origin', signal: signal ?? null })
  if (!res.ok) throw new ApiError(res.status, 'types', `HTTP ${res.status}`)
  return await res.json() as TypeFiles
}
