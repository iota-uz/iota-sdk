import { AppletRPCException } from '@iota-uz/sdk'

export interface RPCErrorDisplay {
  title: string
  description: string
  isPermissionDenied: boolean
}

function extractHTTPStatus(details: unknown): number | null {
  if (!details || typeof details !== 'object') {
    return null
  }
  const status = (details as { status?: unknown }).status
  return typeof status === 'number' ? status : null
}

export function isPermissionDeniedError(error: unknown): boolean {
  if (error instanceof AppletRPCException) {
    if (error.code === 'forbidden') {
      return true
    }
    if (error.code === 'http_error' && extractHTTPStatus(error.details) === 403) {
      return true
    }
  }

  if (error instanceof Error) {
    const msg = error.message.toLowerCase()
    return msg.includes('permission denied') || msg.includes('forbidden')
  }

  if (typeof error === 'string') {
    const msg = error.toLowerCase()
    return msg.includes('permission denied') || msg.includes('forbidden')
  }

  return false
}

export function toRPCErrorDisplay(error: unknown, fallbackTitle: string): RPCErrorDisplay {
  if (isPermissionDeniedError(error)) {
    return {
      title: 'Access denied',
      description: 'Your account does not have permission to use this BiChat action.',
      isPermissionDenied: true,
    }
  }

  if (error instanceof AppletRPCException && error.code === 'http_error') {
    const status = extractHTTPStatus(error.details)
    return {
      title: fallbackTitle,
      description: status ? `Request failed with HTTP ${status}.` : 'Request failed.',
      isPermissionDenied: false,
    }
  }

  if (error instanceof Error && error.message.trim().length > 0) {
    return {
      title: fallbackTitle,
      description: error.message,
      isPermissionDenied: false,
    }
  }

  if (typeof error === 'string' && error.trim().length > 0) {
    return {
      title: fallbackTitle,
      description: error,
      isPermissionDenied: false,
    }
  }

  return {
    title: fallbackTitle,
    description: 'Unexpected error. Please try again.',
    isPermissionDenied: false,
  }
}
