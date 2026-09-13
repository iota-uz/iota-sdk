export type HostErrorCode =
  | 'unauthenticated'
  | 'permission_denied'
  | 'validation'
  | 'field_validation'
  | 'transient'
  | 'protocol'
  | 'unknown'

export class HostError extends Error {
  readonly fieldErrors: Readonly<Record<string, string>>

  constructor(
    readonly code: HostErrorCode,
    message: string,
    readonly details?: unknown,
    fieldErrors?: Readonly<Record<string, string>>,
  ) {
    super(message)
    this.name = 'HostError'
    this.fieldErrors = fieldErrors ?? (
      code === 'field_validation' && details && typeof details === 'object'
        ? details as Record<string, string>
        : {}
    )
  }

  get retryable(): boolean { return this.code === 'transient' }
}

export interface HostErrorHandlers {
  unauthenticated?(error: HostError): void
  permissionDenied?(error: HostError): void
  validation?(error: HostError): void
}

export function asHostError(input: unknown): HostError {
  if (input instanceof HostError) return input
  const candidate = input as { code?: unknown; message?: unknown; details?: unknown; data?: unknown }
  const details = candidate?.details ?? candidate?.data
  const detailRecord = details && typeof details === 'object' ? details as Record<string, unknown> : undefined
  const raw = String(detailRecord?.code ?? candidate?.code ?? 'unknown')
  const code: HostErrorCode = raw === 'unauthenticated' || raw === 'unauthorized'
    ? 'unauthenticated'
    : raw === 'forbidden' || raw === 'permission_denied'
      ? 'permission_denied'
      : raw === 'validation'
        ? 'validation'
        : raw === 'field_validation'
          ? 'field_validation'
          : raw === 'timeout' || raw === 'unavailable' || raw === 'transient'
            ? 'transient'
            : raw === 'protocol'
              ? 'protocol'
              : 'unknown'
  const fields = detailRecord?.fieldErrors ?? detailRecord?.fields ?? details
  const fieldErrors = code === 'field_validation' && fields && typeof fields === 'object'
    ? fields as Record<string, string>
    : {}
  return new HostError(code, String(candidate?.message ?? input ?? 'Unknown host error'), details, fieldErrors)
}

export function dispatchHostError(error: HostError, handlers?: HostErrorHandlers): void {
  if (error.code === 'unauthenticated') handlers?.unauthenticated?.(error)
  if (error.code === 'permission_denied') handlers?.permissionDenied?.(error)
  if (error.code === 'validation' || error.code === 'field_validation') handlers?.validation?.(error)
}
